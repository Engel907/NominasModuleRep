// Package auth es la capa de servicio del módulo de seguridad: orquesta
// internal/security (hashing + PASETO) y las funciones de acceso a datos de
// internal/db/dto (escritas a mano con database/sql) para implementar login,
// refresh, logout, asignación/revocación de rol y reset de contraseña.
//
// Convenciones:
//   - Todos los IDs son int64 de punta a punta (dto, security.Payload,
//     handlers, middleware): decisión D1, sin conversiones intermedias.
//   - Las funciones de dto reciben argumentos posicionales (no structs
//     XxxParams). CrearRefreshToken, CrearTokenResetPassword e
//     InsertarUsuarioRolVigente devuelven (int64, error); este servicio no
//     necesita el ID autogenerado y lo descarta con `_`.
//   - Toda escritura sobre tablas auditadas pasa por db.WithAuditContext,
//     que necesita el *sql.DB (de ahí que Servicio lo guarde).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/security"
)

// Store es el subconjunto de los métodos de *dto.Queries que este servicio
// necesita. Se declara como interfaz local (en vez de depender del
// *Queries completo) para que los tests puedan pasar un fake sin implementar
// todo el catálogo de queries del proyecto.
//
// Las firmas corresponden EXACTAMENTE (nombre y tipos) a los métodos de
// internal/db/dto/security.go, de modo que *dto.Queries satisface Store.
type Store interface {
	GetUsuarioPorNombreUsuario(ctx context.Context, nombreUsuario string) (dto.Usuario, error)
	GetUsuarioPorID(ctx context.Context, id int64) (dto.Usuario, error)
	IncrementarIntentosFallidos(ctx context.Context, id int64) error
	BloquearUsuarioTemporalmente(ctx context.Context, id int64, bloqueadoHasta time.Time) error
	ResetearIntentosFallidos(ctx context.Context, id int64) error
	ActualizarPasswordHash(ctx context.Context, id int64, passwordHash string) error

	CrearRefreshToken(ctx context.Context, usuarioID int64, tokenHash string, fechaExpiracion time.Time) (int64, error)
	GetRefreshTokenPorHash(ctx context.Context, tokenHash string) (dto.RefreshToken, error)
	RevocarRefreshTokenPorHash(ctx context.Context, tokenHash string) error
	RevocarTodosLosRefreshTokensDeUsuario(ctx context.Context, usuarioID int64) error

	CrearTokenResetPassword(ctx context.Context, usuarioID int64, tokenHash string, fechaExpiracion time.Time) (int64, error)
	GetTokenResetPasswordPorHash(ctx context.Context, tokenHash string) (dto.TokenResetPassword, error)
	MarcarTokenResetPasswordUsado(ctx context.Context, id int64) error
	InvalidarTokensResetPasswordPendientesDeUsuario(ctx context.Context, usuarioID int64) error

	GetRolPorID(ctx context.Context, id int64) (dto.Rol, error)

	GetPermisosVigentesDeUsuario(ctx context.Context, usuarioID int64) ([]string, error)
}

// Politica agrupa los parámetros de negocio que el ADR-000 marca
// explícitamente como "no van en SQL, van en capa de servicio Go"
// (fuerza bruta, punto c) más las duraciones de token.
//
// Los valores vienen SOLO de internal/config (config.AuthConfig); este
// paquete no define defaults propios. El cableado (cmd/api/main.go) mapea:
//
//	MaxIntentosFallidos  <- cfg.Auth.MaxFailedLogins
//	MinutosBloqueo       <- cfg.Auth.LockoutDuration
//	DuracionAccessToken  <- cfg.Auth.AccessTokenDuration
//	DuracionRefreshToken <- cfg.Auth.RefreshTokenDuration
//	DuracionTokenReset   <- cfg.Auth.PasswordResetTokenDuration
type Politica struct {
	MaxIntentosFallidos  int
	MinutosBloqueo       time.Duration
	DuracionAccessToken  time.Duration
	DuracionRefreshToken time.Duration
	DuracionTokenReset   time.Duration
}

type Servicio struct {
	sqlDB      *sql.DB
	store      Store
	tokenMaker security.TokenMaker
	politica   Politica
}

// NuevoServicio recibe el *sql.DB porque las escrituras auditadas
// (AsignarRol / RevocarRol) abren su propia transacción vía
// db.WithAuditContext; `store` se usa para las demás operaciones.
func NuevoServicio(store Store, sqlDB *sql.DB, tokenMaker security.TokenMaker, politica Politica) *Servicio {
	return &Servicio{sqlDB: sqlDB, store: store, tokenMaker: tokenMaker, politica: politica}
}

// ============================================================
// LOGIN
// ============================================================

func (s *Servicio) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	usuario, err := s.store.GetUsuarioPorNombreUsuario(ctx, req.NombreUsuario)
	if err != nil {
		// No distinguimos "no existe" de "password incorrecto": mismo error
		// genérico para no filtrar qué usuarios existen (user enumeration).
		//
		// F14: este retorno temprano es mucho más rápido que el camino con
		// argon2id, así que el tiempo de respuesta todavía delata si el
		// usuario existe. Igualar tiempos (hash ficticio) es trabajo de F14.
		// También: un error de BD distinto de sql.ErrNoRows se enmascara
		// como credenciales inválidas.
		return nil, ErrCredencialesInvalidas
	}

	// F14: estos dos chequeos ocurren ANTES de verificar la contraseña, así
	// que ErrUsuarioInactivo / ErrUsuarioBloqueado revelan el estado de la
	// cuenta a quien solo conoce el nombre de usuario.
	if !usuario.Activo {
		return nil, ErrUsuarioInactivo
	}

	if usuario.BloqueadoHasta.Valid && usuario.BloqueadoHasta.Time.After(time.Now()) {
		return nil, ErrUsuarioBloqueado
	}

	necesitaRehash, err := security.VerifyPassword(req.Password, usuario.PasswordHash)
	if err != nil {
		if errors.Is(err, security.ErrPasswordInvalida) {
			return nil, s.registrarIntentoFallido(ctx, usuario)
		}
		return nil, err
	}

	// Login exitoso: resetear contador de intentos fallidos.
	if err := s.store.ResetearIntentosFallidos(ctx, usuario.ID); err != nil {
		return nil, err
	}

	// Re-hash perezoso bcrypt→argon2id (no bloquea el login si falla; se
	// reintentará en el próximo login exitoso).
	//
	// F14: el error de ActualizarPasswordHash se descarta en silencio (ni
	// siquiera se loguea). Además ActualizarPasswordHash marca
	// requiere_cambio_password = 0 y fecha_ultimo_cambio_password = NOW():
	// un re-hash perezoso limpia por efecto colateral el flag de "debe
	// cambiar la contraseña", y la respuesta de este login (abajo) ya lee
	// el valor viejo de `usuario`.
	if necesitaRehash {
		if nuevoHash, hashErr := security.HashPassword(req.Password); hashErr == nil {
			_ = s.store.ActualizarPasswordHash(ctx, usuario.ID, nuevoHash)
		}
	}

	permisos, err := s.store.GetPermisosVigentesDeUsuario(ctx, usuario.ID)
	if err != nil {
		return nil, err
	}

	return s.emitirParDeTokens(ctx, usuario.ID, usuario.NombreUsuario, permisos, usuario.RequiereCambioPassword)
}

// registrarIntentoFallido incrementa el contador y, de alcanzar el umbral,
// bloquea temporalmente al usuario (ADR-000 punto c: política en Go, no en SQL).
//
// F14: el conteo se calcula en Go sobre la lectura previa
// (usuario.IntentosFallidos + 1), no sobre el valor ya incrementado en BD:
// intentos concurrentes pueden quedar por debajo del umbral. El error de
// BloquearUsuarioTemporalmente también se descarta.
func (s *Servicio) registrarIntentoFallido(ctx context.Context, usuario dto.Usuario) error {
	if err := s.store.IncrementarIntentosFallidos(ctx, usuario.ID); err != nil {
		return err
	}
	nuevoConteo := int(usuario.IntentosFallidos) + 1
	if nuevoConteo >= s.politica.MaxIntentosFallidos {
		_ = s.store.BloquearUsuarioTemporalmente(ctx, usuario.ID, time.Now().Add(s.politica.MinutosBloqueo))
	}
	return ErrCredencialesInvalidas
}

// ============================================================
// REFRESH
// ============================================================

func (s *Servicio) Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error) {
	payload, err := s.tokenMaker.VerificarToken(req.RefreshToken, security.TipoTokenRefresco)
	if err != nil {
		return nil, ErrRefreshTokenInvalido
	}

	hash := hashToken(req.RefreshToken)
	fila, err := s.store.GetRefreshTokenPorHash(ctx, hash)
	if err != nil {
		return nil, ErrRefreshTokenInvalido
	}
	if fila.Revocado || fila.FechaExpiracion.Before(time.Now()) {
		// F15: presentar un refresh token YA revocado (reutilización de un
		// token rotado) no dispara ninguna reacción: no hay detección de
		// reuso ni revocación de la familia de sesiones.
		return nil, ErrRefreshTokenInvalido
	}

	usuario, err := s.store.GetUsuarioPorID(ctx, payload.UsuarioID)
	if err != nil || !usuario.Activo {
		return nil, ErrRefreshTokenInvalido
	}

	// Rotación: se revoca el refresh token usado y se emite uno nuevo, para
	// que un token robado y reutilizado sea detectable/uso único.
	//
	// F15: la rotación NO es atómica. RevocarRefreshTokenPorHash no informa
	// filas afectadas, así que dos refresh concurrentes con el mismo token
	// pueden pasar ambos el chequeo de arriba y emitir dos pares válidos.
	// Revocar + emitir tampoco van en una misma transacción. Tampoco se
	// compara fila.UsuarioID con payload.UsuarioID.
	if err := s.store.RevocarRefreshTokenPorHash(ctx, hash); err != nil {
		return nil, err
	}

	// Permisos re-resueltos contra BD (no se copian del access token
	// vencido) para que un refresh sí refleje cambios de rol recientes.
	permisos, err := s.store.GetPermisosVigentesDeUsuario(ctx, usuario.ID)
	if err != nil {
		return nil, err
	}

	tokens, err := s.emitirParDeTokens(ctx, usuario.ID, usuario.NombreUsuario, permisos, usuario.RequiereCambioPassword)
	if err != nil {
		return nil, err
	}
	return &RefreshResponse{
		AccessToken:          tokens.AccessToken,
		AccessTokenExpiraEn:  tokens.AccessTokenExpiraEn,
		RefreshToken:         tokens.RefreshToken,
		RefreshTokenExpiraEn: tokens.RefreshTokenExpiraEn,
	}, nil
}

// ============================================================
// LOGOUT
// ============================================================

// Logout revoca únicamente la sesión (refresh token) presentada.
//
// F15: Logout es público (no exige access token) y no comprueba que el
// refresh token pertenezca a quien llama; basta con poseerlo.
func (s *Servicio) Logout(ctx context.Context, req LogoutRequest) error {
	if _, err := s.tokenMaker.VerificarToken(req.RefreshToken, security.TipoTokenRefresco); err != nil {
		// Un token ya inválido/expirado no necesita revocación explícita.
		return nil
	}
	return s.store.RevocarRefreshTokenPorHash(ctx, hashToken(req.RefreshToken))
}

// LogoutTodasLasSesiones revoca TODAS las sesiones del usuario — usada tras
// un reset de contraseña o por un administrador.
//
// F15: solo revoca refresh tokens; los access tokens ya emitidos siguen
// siendo válidos hasta expirar (15-30 min, ver security.Payload).
func (s *Servicio) LogoutTodasLasSesiones(ctx context.Context, usuarioID int64) error {
	return s.store.RevocarTodosLosRefreshTokensDeUsuario(ctx, usuarioID)
}

// ============================================================
// ASIGNACIÓN / REVOCACIÓN DE ROL (usuario_roles versionada — ADR-000 §2)
// ============================================================

// AsignarRol implementa la transacción de 2 pasos sobre usuario_roles.
// db.WithAuditContext envuelve la Tx para que los triggers de auditoría
// registren `creadoPor` como actor, en vez del defecto registro_id=0
// documentado en ADR-000 §1.
//
// Devuelve ErrRolYaVigente si el usuario ya tiene ese rol vigente (el
// handler lo traduce a 409); la verificación corre DENTRO de la misma Tx.
func (s *Servicio) AsignarRol(ctx context.Context, usuarioID int64, req AsignarRolRequest, creadoPor int64) error {
	rol, err := s.store.GetRolPorID(ctx, req.RolID)
	if err != nil {
		return err
	}
	if !rol.Activo {
		return ErrRolInactivo
	}

	fechaEfectiva := time.Now()
	if req.FechaEfectiva != nil {
		fechaEfectiva = *req.FechaEfectiva
	}

	return db.WithAuditContext(ctx, s.sqlDB, creadoPor, func(ctx context.Context, q *dto.Queries) error {
		vigentes, err := q.GetRolesVigentesDeUsuario(ctx, usuarioID)
		if err != nil {
			return err
		}
		for _, rv := range vigentes {
			if rv.RolID == req.RolID {
				return ErrRolYaVigente
			}
		}

		// Paso 1: cierra la fila vigente anterior para el mismo (usuario,
		// rol), si existe. Tras la verificación de arriba normalmente
		// afecta 0 filas (no es error); se conserva para respetar el patrón
		// de 2 pasos de ADR-000 §2.
		if err := q.CerrarUsuarioRolVigente(ctx, usuarioID, req.RolID, fechaEfectiva); err != nil {
			return err
		}
		// Paso 2: abre la nueva fila vigente.
		//
		// F14: verificar-y-luego-insertar no es atómico: dos asignaciones
		// concurrentes podrían pasar ambas la verificación; en ese caso
		// ganaría la restricción única (usuario_id, rol_id, es_vigente) y
		// el segundo INSERT fallaría con error de duplicado (1062) en vez
		// de ErrRolYaVigente.
		_, err = q.InsertarUsuarioRolVigente(ctx, usuarioID, req.RolID, fechaEfectiva, sql.NullInt64{Int64: creadoPor, Valid: true})
		return err
	})
}

// RevocarRol cierra la fila vigente de (usuario, rol) sin abrir una nueva
// (solo el paso 1 del patrón de 2 pasos).
func (s *Servicio) RevocarRol(ctx context.Context, usuarioID int64, req RevocarRolRequest, revocadoPor int64) error {
	return db.WithAuditContext(ctx, s.sqlDB, revocadoPor, func(ctx context.Context, q *dto.Queries) error {
		return q.CerrarUsuarioRolVigente(ctx, usuarioID, req.RolID, time.Now())
	})
}

// ============================================================
// RESET DE CONTRASEÑA (ADR-000 punto d)
// ============================================================

// SolicitarResetPassword genera un token de un solo uso y devuelve el
// valor EN CLARO para que el llamador (handler) lo envíe por el canal que
// corresponda (correo, etc.) — este servicio nunca envía notificaciones,
// solo persiste el hash SHA-256.
//
// Devuelve (token, nil) incluso si el usuario no existe/está inactivo,
// pero en ese caso el token no se persiste — el handler debe responder
// igual (mensaje genérico "si el usuario existe, se envió un correo") para
// no filtrar existencia de cuentas. Ver comentario en el handler.
//
// F15: el camino "usuario inexistente" retorna de inmediato mientras el
// otro hace varias escrituras en BD, así que el tiempo de respuesta
// delata si la cuenta existe. Además, invalidar los tokens pendientes y
// crear el nuevo no están en una misma transacción.
func (s *Servicio) SolicitarResetPassword(ctx context.Context, req SolicitarResetPasswordRequest) (tokenPlano string, huboUsuario bool, err error) {
	usuario, err := s.store.GetUsuarioPorNombreUsuario(ctx, req.NombreUsuario)
	if err != nil || !usuario.Activo {
		return "", false, nil
	}

	tokenPlano, err = generarTokenAleatorio()
	if err != nil {
		return "", false, err
	}

	if err := s.store.InvalidarTokensResetPasswordPendientesDeUsuario(ctx, usuario.ID); err != nil {
		return "", false, err
	}

	if _, err := s.store.CrearTokenResetPassword(ctx, usuario.ID, hashToken(tokenPlano), time.Now().Add(s.politica.DuracionTokenReset)); err != nil {
		return "", false, err
	}

	return tokenPlano, true, nil
}

// ConfirmarResetPassword valida el token, actualiza el password y revoca
// todas las sesiones activas del usuario (buena práctica: un reset de
// contraseña debe invalidar cualquier sesión que pudiera estar
// comprometida).
//
// F15: los cuatro pasos de abajo (leer token, actualizar password, marcar
// el token usado, revocar sesiones) NO van en una transacción ni usan una
// marca de uso condicional: dos confirmaciones concurrentes con el mismo
// token pueden pasar ambas la validación, y un fallo a mitad de camino deja
// el password cambiado con el token aún utilizable o las sesiones activas.
// F14: tampoco se limpia el bloqueo por intentos fallidos tras el reset.
func (s *Servicio) ConfirmarResetPassword(ctx context.Context, req ConfirmarResetPasswordRequest) error {
	fila, err := s.store.GetTokenResetPasswordPorHash(ctx, hashToken(req.Token))
	if err != nil {
		return ErrTokenResetInvalido
	}
	if fila.Usado || fila.FechaExpiracion.Before(time.Now()) {
		return ErrTokenResetInvalido
	}

	nuevoHash, err := security.HashPassword(req.NuevaPassword)
	if err != nil {
		return err
	}

	if err := s.store.ActualizarPasswordHash(ctx, fila.UsuarioID, nuevoHash); err != nil {
		return err
	}

	if err := s.store.MarcarTokenResetPasswordUsado(ctx, fila.ID); err != nil {
		return err
	}

	return s.store.RevocarTodosLosRefreshTokensDeUsuario(ctx, fila.UsuarioID)
}

// ============================================================
// Helpers internos
// ============================================================

func (s *Servicio) emitirParDeTokens(ctx context.Context, usuarioID int64, nombreUsuario string, permisos []string, requiereCambioPassword bool) (*LoginResponse, error) {
	accessToken, accessPayload, err := s.tokenMaker.CrearAccessToken(usuarioID, nombreUsuario, permisos, s.politica.DuracionAccessToken)
	if err != nil {
		return nil, err
	}

	refreshToken, refreshPayload, err := s.tokenMaker.CrearRefreshToken(usuarioID, s.politica.DuracionRefreshToken)
	if err != nil {
		return nil, err
	}

	if _, err := s.store.CrearRefreshToken(ctx, usuarioID, hashToken(refreshToken), refreshPayload.ExpiraEn); err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:            accessToken,
		AccessTokenExpiraEn:    accessPayload.ExpiraEn,
		RefreshToken:           refreshToken,
		RefreshTokenExpiraEn:   refreshPayload.ExpiraEn,
		RequiereCambioPassword: requiereCambioPassword,
	}, nil
}

// hashToken es SHA-256 en hex — nunca se persiste un token en claro
// (ADR-000 punto b, mismo criterio para tokens_reset_password).
func hashToken(token string) string {
	suma := sha256.Sum256([]byte(token))
	return hex.EncodeToString(suma[:])
}

// generarTokenAleatorio produce 32 bytes de aleatoriedad criptográfica en
// hex (64 chars) para usar como token de reset de contraseña en claro.
func generarTokenAleatorio() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
