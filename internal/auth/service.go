// Package auth es la capa de servicio del módulo de seguridad: orquesta
// internal/security (hashing + PASETO) y las funciones de acceso a datos de
// internal/db/dto (escritas a mano con database/sql) para implementar login,
// refresh, logout, cambio/reset de contraseña y asignación/revocación de rol.
//
// Convenciones:
//   - Todos los IDs son int64 de punta a punta (dto, security.Payload,
//     handlers, middleware): decisión D1, sin conversiones intermedias.
//   - Toda escritura sobre tablas auditadas pasa por db.WithAuditContext
//     (ejecutarTx), que necesita el *sql.DB (de ahí que Servicio lo guarde).
//     Dentro de una transacción el servicio habla con un Store (en producción,
//     el *dto.Queries de la Tx) — por eso Store, y no *dto.Queries, es la
//     interfaz que usa la lógica y la que reemplazan los tests.
//
// F14 — autenticación reforzada (S1, S2, S5, S6, S11):
//   - Login: respuesta única ante usuario inexistente/inactivo/bloqueado/mala
//     contraseña, tiempo igualado con un hash señuelo, contador atómico.
//   - Refresh: rotación atómica con detección de reutilización por familia.
//   - CambiarPassword / reset: política de contraseñas, un solo uso, revocación
//     de sesiones, y claim rcp para obligar el cambio inicial.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/security"
)

// Store es el subconjunto de los métodos de *dto.Queries que este servicio
// necesita. Las firmas corresponden EXACTAMENTE (nombre y tipos) a los métodos
// de internal/db/dto/security.go, de modo que *dto.Queries satisface Store.
type Store interface {
	GetUsuarioPorNombreUsuario(ctx context.Context, nombreUsuario string) (dto.Usuario, error)
	GetUsuarioPorID(ctx context.Context, id int64) (dto.Usuario, error)
	RegistrarIntentoFallidoAtomico(ctx context.Context, id int64, maxIntentos int, bloqueadoHasta time.Time) (dto.EstadoIntentos, error)
	ResetearIntentosFallidos(ctx context.Context, id int64) error
	ActualizarPasswordHash(ctx context.Context, id int64, passwordHash string) error
	RehashPasswordHash(ctx context.Context, id int64, hashAnterior, hashNuevo string) error

	CrearRefreshTokenConFamilia(ctx context.Context, usuarioID int64, tokenHash string, fechaExpiracion time.Time, familiaID, ip, userAgent string) (int64, error)
	GetRefreshTokenPorHash(ctx context.Context, tokenHash string) (dto.RefreshToken, error)
	RevocarRefreshTokenPorHash(ctx context.Context, tokenHash string) error
	RevocarRefreshTokenSiVigente(ctx context.Context, id int64) (bool, error)
	RevocarFamiliaRefreshTokens(ctx context.Context, familiaID string) (int64, error)
	RevocarTodosLosRefreshTokensDeUsuario(ctx context.Context, usuarioID int64) error

	CrearTokenResetPassword(ctx context.Context, usuarioID int64, tokenHash string, fechaExpiracion time.Time) (int64, error)
	GetTokenResetPasswordPorHash(ctx context.Context, tokenHash string) (dto.TokenResetPassword, error)
	ConsumirTokenResetPassword(ctx context.Context, id int64, ahora time.Time) (bool, error)
	InvalidarTokensResetPasswordPendientesDeUsuario(ctx context.Context, usuarioID int64) error

	GetRolPorID(ctx context.Context, id int64) (dto.Rol, error)

	GetPermisosVigentesDeUsuario(ctx context.Context, usuarioID int64) ([]string, error)

	// F15: revocación efectiva de access tokens.
	IncrementarPermisosVersion(ctx context.Context, usuarioID int64) (uint32, error)
	IncrementarPermisosVersionPorRol(ctx context.Context, rolID int64) error
	SetUsuarioActivo(ctx context.Context, usuarioID int64, activo bool) (bool, error)
	SetRolActivo(ctx context.Context, rolID int64, activo bool) (bool, error)
	ReemplazarPermisosRol(ctx context.Context, rolID int64, codigos []string) error
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
	sqlDB       *sql.DB
	store       Store
	tokenMaker  security.TokenMaker
	politica    Politica
	notificador Notificador
	logger      *slog.Logger

	// Puntos de inyección para pruebas; nil/por defecto en producción.
	txRunner  func(ctx context.Context, actorID int64, fn func(context.Context, Store) error) error
	verificar func(password, hash string) (necesitaRehash bool, err error)

	// Envío de correos de reset fuera del request (ver enviarResetAsync).
	envios   chan struct{}
	enviosWG sync.WaitGroup
}

// maxEnviosSimultaneos acota las goroutines de envío de correo: sin tope, un
// atacante podría disparar miles de resets y agotar memoria/conexiones SMTP.
const maxEnviosSimultaneos = 16

// NuevoServicio recibe el *sql.DB porque las escrituras auditadas abren su
// propia transacción vía db.WithAuditContext; `store` se usa para lecturas y
// escrituras simples. Precalcula el hash señuelo de Login (cuesta ~un hash
// argon2id) para que ningún request pague ese costo.
//
// El notificador del reset se inyecta con ConNotificador; sin él, el reset
// registra un error y no envía nada (nunca falla "hacia afuera").
func NuevoServicio(store Store, sqlDB *sql.DB, tokenMaker security.TokenMaker, politica Politica) *Servicio {
	hashSenuelo()
	return &Servicio{
		sqlDB:      sqlDB,
		store:      store,
		tokenMaker: tokenMaker,
		politica:   politica,
		logger:     slog.Default(),
		verificar:  security.VerifyPassword,
		envios:     make(chan struct{}, maxEnviosSimultaneos),
	}
}

// ConNotificador fija el canal de entrega del token de reset (S2).
func (s *Servicio) ConNotificador(n Notificador) *Servicio {
	s.notificador = n
	return s
}

// ConLogger reemplaza el slog.Default() usado para el log interno de seguridad.
func (s *Servicio) ConLogger(l *slog.Logger) *Servicio {
	if l != nil {
		s.logger = l
	}
	return s
}

// EsperarEnvios bloquea hasta que terminen los envíos de correo en curso.
// Para el apagado ordenado del servidor y para los tests.
func (s *Servicio) EsperarEnvios() { s.enviosWG.Wait() }

// ejecutarTx corre fn dentro de una transacción con contexto de auditoría
// (actorID queda como autor en los triggers). fn recibe un Store ligado a la Tx.
func (s *Servicio) ejecutarTx(ctx context.Context, actorID int64, fn func(context.Context, Store) error) error {
	if s.txRunner != nil {
		return s.txRunner(ctx, actorID, fn)
	}
	return db.WithAuditContext(ctx, s.sqlDB, actorID, func(ctx context.Context, q *dto.Queries) error {
		return fn(ctx, q)
	})
}

// ============================================================
// LOGIN
// ============================================================

// Login (F14 / S5). Contrato:
//   - Usuario inexistente, contraseña mala, usuario inactivo y usuario
//     bloqueado devuelven el MISMO error (ErrCredencialesInvalidas); el motivo
//     real solo va al log interno.
//   - Todos esos caminos pagan una verificación argon2id (la del usuario, o
//     la del hash señuelo si no existe) para que el tiempo no delate nada.
//     Solo un error de infraestructura (BD caída) devuelve otro error.
//   - El contador de fallos es atómico en SQL (RegistrarIntentoFallidoAtomico).
//   - Un intento sobre una cuenta ya bloqueada o inactiva NO suma al contador
//     ni extiende el bloqueo: si no, cualquiera podría mantener bloqueada a una
//     víctima indefinidamente.
func (s *Servicio) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	usuario, err := s.store.GetUsuarioPorNombreUsuario(ctx, req.NombreUsuario)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err // fallo de infraestructura: no se enmascara como credenciales inválidas
		}
		_, _ = s.verificar(req.Password, hashSenuelo()) // igualar tiempo
		s.logFalloLogin(ctx, "usuario_inexistente", 0, req.NombreUsuario, req.Origen)
		return nil, ErrCredencialesInvalidas
	}

	necesitaRehash, verr := s.verificar(req.Password, usuario.PasswordHash)
	if verr != nil && !errors.Is(verr, security.ErrPasswordInvalida) {
		// Hash con formato desconocido o corrupto: es un problema de datos que
		// hay que ver en los logs, pero hacia el cliente es "credenciales inválidas".
		s.logger.ErrorContext(ctx, "auth: no se pudo verificar el hash del usuario",
			"usuario_id", usuario.ID, "error", verr)
	}
	passwordOK := verr == nil

	switch {
	case !usuario.Activo:
		s.logFalloLogin(ctx, ErrUsuarioInactivo.Error(), usuario.ID, usuario.NombreUsuario, req.Origen)
		return nil, ErrCredencialesInvalidas
	case estaBloqueado(usuario):
		s.logFalloLogin(ctx, ErrUsuarioBloqueado.Error(), usuario.ID, usuario.NombreUsuario, req.Origen)
		return nil, ErrCredencialesInvalidas
	case !passwordOK:
		s.registrarIntentoFallido(ctx, usuario, req.Origen, "password_incorrecto")
		return nil, ErrCredencialesInvalidas
	}

	// Login exitoso: resetear contador de intentos fallidos.
	if err := s.store.ResetearIntentosFallidos(ctx, usuario.ID); err != nil {
		return nil, err
	}

	// Re-hash perezoso bcrypt→argon2id. No bloquea ni falla el login, pero un
	// error SÍ se registra. Usa RehashPasswordHash, que no toca
	// requiere_cambio_password: antes, este re-hash borraba el flag por efecto
	// colateral y un usuario obligado a cambiar la clave quedaba libre.
	if necesitaRehash {
		s.rehashPerezoso(ctx, usuario, req.Password)
	}

	permisos, err := s.store.GetPermisosVigentesDeUsuario(ctx, usuario.ID)
	if err != nil {
		return nil, err
	}

	// Cada login abre una familia de refresh tokens nueva.
	return s.emitirParDeTokens(ctx, s.store, usuario.ID, usuario.NombreUsuario, permisos,
		usuario.RequiereCambioPassword, usuario.PermisosVersion, uuid.NewString(), req.Origen)
}

func estaBloqueado(u dto.Usuario) bool {
	return u.BloqueadoHasta.Valid && u.BloqueadoHasta.Time.After(time.Now())
}

func (s *Servicio) rehashPerezoso(ctx context.Context, usuario dto.Usuario, passwordPlano string) {
	nuevoHash, err := security.HashPassword(passwordPlano)
	if err != nil {
		s.logger.WarnContext(ctx, "auth: no se pudo calcular el re-hash argon2id", "usuario_id", usuario.ID, "error", err)
		return
	}
	if err := s.store.RehashPasswordHash(ctx, usuario.ID, usuario.PasswordHash, nuevoHash); err != nil {
		s.logger.WarnContext(ctx, "auth: no se pudo guardar el re-hash argon2id", "usuario_id", usuario.ID, "error", err)
	}
}

// registrarIntentoFallido suma un fallo de forma atómica (y bloquea si llega
// al umbral). Un error de BD acá se registra pero no cambia la respuesta al
// cliente: seguir devolviendo "credenciales inválidas" es lo uniforme.
func (s *Servicio) registrarIntentoFallido(ctx context.Context, usuario dto.Usuario, origen OrigenSolicitud, motivo string) {
	hasta := time.Now().Add(s.politica.MinutosBloqueo)
	estado, err := s.store.RegistrarIntentoFallidoAtomico(ctx, usuario.ID, s.politica.MaxIntentosFallidos, hasta)
	if err != nil {
		s.logger.ErrorContext(ctx, "auth: no se pudo registrar el intento fallido",
			"usuario_id", usuario.ID, "error", err)
		return
	}
	s.logger.WarnContext(ctx, "auth: intento fallido",
		"motivo", motivo,
		"usuario_id", usuario.ID,
		"intentos_fallidos", estado.IntentosFallidos,
		"bloqueado", estado.BloqueadoHasta.Valid && estado.BloqueadoHasta.Time.After(time.Now()),
		"ip", origen.IP,
		"user_agent", origen.UserAgent,
	)
}

// logFalloLogin deja en el log interno el motivo real de un login rechazado
// (el cliente solo ve ErrCredencialesInvalidas).
func (s *Servicio) logFalloLogin(ctx context.Context, motivo string, usuarioID int64, nombreUsuario string, origen OrigenSolicitud) {
	s.logger.WarnContext(ctx, "auth: login rechazado",
		"motivo", motivo,
		"usuario_id", usuarioID,
		"nombre_usuario", truncar(nombreUsuario, 60),
		"ip", origen.IP,
		"user_agent", origen.UserAgent,
	)
}

func truncar(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

// El hash señuelo se calcula UNA vez por proceso, con los mismos parámetros
// argon2id que un hash real (security.HashPassword), y lo usa Login cuando el
// usuario no existe para gastar el mismo tiempo que con un usuario real.
var (
	senueloOnce sync.Once
	senueloHash string
)

func hashSenuelo() string {
	senueloOnce.Do(func() {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			panic("auth: no se pudo generar el secreto del hash señuelo: " + err.Error())
		}
		h, err := security.HashPassword(hex.EncodeToString(buf))
		if err != nil {
			panic("auth: no se pudo precalcular el hash señuelo: " + err.Error())
		}
		senueloHash = h
	})
	return senueloHash
}

// ============================================================
// REFRESH
// ============================================================

// Refresh (F14 / S6) rota el refresh token:
//   - La revocación del token usado y el alta del nuevo ocurren en UNA
//     transacción, y la revocación es condicional (revocado = 0) con
//     verificación de filas afectadas: de dos refresh concurrentes con el
//     mismo token, solo uno gana.
//   - Un token ya revocado que reaparece (o el perdedor de esa carrera) es una
//     reutilización: se revoca TODA su familia de sesiones y se registra el
//     evento; el cliente solo ve ErrRefreshTokenInvalido.
//   - Se verifica que la fila pertenezca al usuario del payload y que el
//     usuario siga activo.
func (s *Servicio) Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error) {
	payload, err := s.tokenMaker.VerificarToken(req.RefreshToken, security.TipoTokenRefresco)
	if err != nil {
		return nil, ErrRefreshTokenInvalido
	}

	fila, err := s.store.GetRefreshTokenPorHash(ctx, hashToken(req.RefreshToken))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenInvalido
		}
		return nil, err
	}

	if fila.UsuarioID != payload.UsuarioID {
		s.logger.WarnContext(ctx, "auth: refresh token cuya fila pertenece a otro usuario",
			"usuario_payload", payload.UsuarioID, "usuario_fila", fila.UsuarioID, "token_id", fila.ID, "ip", req.Origen.IP)
		return nil, ErrRefreshTokenInvalido
	}

	if fila.Revocado {
		s.manejarReutilizacion(ctx, fila, req.Origen)
		return nil, ErrRefreshTokenInvalido
	}
	if fila.FechaExpiracion.Before(time.Now()) {
		return nil, ErrRefreshTokenInvalido
	}

	usuario, err := s.store.GetUsuarioPorID(ctx, fila.UsuarioID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenInvalido
		}
		return nil, err
	}
	if !usuario.Activo {
		return nil, ErrRefreshTokenInvalido
	}

	// Permisos re-resueltos contra BD (no se copian del access token vencido)
	// para que un refresh sí refleje cambios de rol recientes.
	permisos, err := s.store.GetPermisosVigentesDeUsuario(ctx, usuario.ID)
	if err != nil {
		return nil, err
	}

	familia := uuid.NewString() // sesiones previas a 000114 (familia NULL) arrancan una familia
	if fila.FamiliaID.Valid && fila.FamiliaID.String != "" {
		familia = fila.FamiliaID.String
	}

	var tokens *LoginResponse
	err = s.ejecutarTx(ctx, usuario.ID, func(ctx context.Context, st Store) error {
		gano, err := st.RevocarRefreshTokenSiVigente(ctx, fila.ID)
		if err != nil {
			return err
		}
		if !gano {
			return errReutilizacionRefresh // otro request lo revocó entre la lectura y acá
		}
		tokens, err = s.emitirParDeTokens(ctx, st, usuario.ID, usuario.NombreUsuario, permisos,
			usuario.RequiereCambioPassword, usuario.PermisosVersion, familia, req.Origen)
		return err
	})
	if errors.Is(err, errReutilizacionRefresh) {
		// La Tx ya hizo rollback; la revocación de la familia va en la suya.
		s.manejarReutilizacion(ctx, fila, req.Origen)
		return nil, ErrRefreshTokenInvalido
	}
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

// manejarReutilizacion responde a un refresh token ya revocado que reaparece:
// revoca toda la familia (o, en sesiones sin familia, todas las del usuario) y
// deja registro. La revocación corre con el usuario como actor de auditoría,
// así que los triggers de refresh_tokens también la dejan en la bitácora.
func (s *Servicio) manejarReutilizacion(ctx context.Context, fila dto.RefreshToken, origen OrigenSolicitud) {
	var revocadas int64
	err := s.ejecutarTx(ctx, fila.UsuarioID, func(ctx context.Context, st Store) error {
		if fila.FamiliaID.Valid && fila.FamiliaID.String != "" {
			n, err := st.RevocarFamiliaRefreshTokens(ctx, fila.FamiliaID.String)
			revocadas = n
			return err
		}
		return st.RevocarTodosLosRefreshTokensDeUsuario(ctx, fila.UsuarioID)
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "auth: reutilización de refresh token y FALLÓ la revocación de la familia",
			"usuario_id", fila.UsuarioID, "token_id", fila.ID, "error", err)
		return
	}
	s.logger.WarnContext(ctx, "auth: REUTILIZACIÓN de refresh token detectada; familia revocada",
		"usuario_id", fila.UsuarioID,
		"token_id", fila.ID,
		"familia_id", fila.FamiliaID.String,
		"sesiones_revocadas", revocadas,
		"ip", origen.IP,
		"user_agent", origen.UserAgent,
	)
}

// ============================================================
// LOGOUT
// ============================================================

// Logout revoca únicamente la sesión (refresh token) presentada.
//
// Logout es público (no exige access token) y no comprueba que el refresh
// token pertenezca a quien llama; basta con poseerlo. Los access tokens
// siguen válidos hasta expirar o hasta que cambie permisos_version.
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
// F15: además de los refresh tokens sube permisos_version, de modo que los
// access tokens vivos de ese usuario dejan de valer en <= AUTH_CACHE_TTL.
func (s *Servicio) LogoutTodasLasSesiones(ctx context.Context, usuarioID int64) error {
	return s.RevocarSesionesUsuario(ctx, usuarioID, usuarioID)
}

// RevocarSesionesUsuario permite a un operador autorizado invalidar las
// sesiones de otro usuario; el actor de auditoría y el titular son distintos.
func (s *Servicio) RevocarSesionesUsuario(ctx context.Context, usuarioID, actorID int64) error {
	return s.ejecutarTx(ctx, actorID, func(ctx context.Context, st Store) error {
		if err := st.RevocarTodosLosRefreshTokensDeUsuario(ctx, usuarioID); err != nil {
			return err
		}
		_, err := st.IncrementarPermisosVersion(ctx, usuarioID)
		return err
	})
}

// ============================================================
// CAMBIO DE CONTRASEÑA (autenticado)
// ============================================================

// CambiarPassword (F14 / S1): valida la contraseña actual, aplica la política
// a la nueva, guarda el hash argon2id (limpia requiere_cambio_password y
// actualiza fecha_ultimo_cambio_password), limpia el bloqueo, REVOCA todas las
// sesiones del usuario y todo en una transacción con auditoría.
//
// Devuelve un par de tokens NUEVO (rcp=false, familia nueva): la sesión que
// hizo el cambio se revoca junto con las demás, y así el cliente no queda
// deslogueado ni con el access token restringido (rcp=true) del primer login.
//
// Una contraseña actual incorrecta cuenta como intento fallido (misma política
// de bloqueo que el login): si no, un access token robado serviría para
// adivinar la contraseña sin límite.
func (s *Servicio) CambiarPassword(ctx context.Context, usuarioID int64, req CambiarPasswordRequest) (*LoginResponse, error) {
	usuario, err := s.store.GetUsuarioPorID(ctx, usuarioID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCredencialesInvalidas
		}
		return nil, err
	}
	if !usuario.Activo || estaBloqueado(usuario) {
		return nil, ErrCredencialesInvalidas
	}

	if _, err := s.verificar(req.PasswordActual, usuario.PasswordHash); err != nil {
		if errors.Is(err, security.ErrPasswordInvalida) {
			s.registrarIntentoFallido(ctx, usuario, req.Origen, "cambio_password_actual_incorrecta")
			return nil, ErrPasswordActualIncorrecta
		}
		return nil, err
	}

	if err := ValidarPassword(req.NuevaPassword, usuario.NombreUsuario, usuario.Correo); err != nil {
		return nil, err
	}
	if req.NuevaPassword == req.PasswordActual { // la actual ya quedó verificada arriba
		return nil, errPolitica(MotivoIgualActual, "La nueva contraseña debe ser distinta de la actual.")
	}

	nuevoHash, err := security.HashPassword(req.NuevaPassword)
	if err != nil {
		return nil, err
	}
	permisos, err := s.store.GetPermisosVigentesDeUsuario(ctx, usuario.ID)
	if err != nil {
		return nil, err
	}

	var tokens *LoginResponse
	err = s.ejecutarTx(ctx, usuario.ID, func(ctx context.Context, st Store) error {
		if err := st.ActualizarPasswordHash(ctx, usuario.ID, nuevoHash); err != nil {
			return err
		}
		if err := st.ResetearIntentosFallidos(ctx, usuario.ID); err != nil {
			return err
		}
		if err := st.RevocarTodosLosRefreshTokensDeUsuario(ctx, usuario.ID); err != nil {
			return err
		}
		// F15: invalida los access tokens emitidos antes del cambio (otras sesiones).
		pv, err := st.IncrementarPermisosVersion(ctx, usuario.ID)
		if err != nil {
			return err
		}
		tokens, err = s.emitirParDeTokens(ctx, st, usuario.ID, usuario.NombreUsuario, permisos,
			false, pv, uuid.NewString(), req.Origen)
		return err
	})
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

// ============================================================
// ASIGNACIÓN / REVOCACIÓN DE ROL (usuario_roles versionada — ADR-000 §2)
// ============================================================

// CambiarEstadoUsuario cambia el estado de una cuenta e invalida los tokens
// existentes dentro de la misma transacción auditada.
func (s *Servicio) CambiarEstadoUsuario(ctx context.Context, usuarioID int64, activo bool, cambiadoPor int64) error {
	if !activo && usuarioID == cambiadoPor {
		return ErrAutoDesactivacion
	}
	return s.ejecutarTx(ctx, cambiadoPor, func(ctx context.Context, st Store) error {
		cambio, err := st.SetUsuarioActivo(ctx, usuarioID, activo)
		if err != nil || !cambio {
			return err
		}
		if !activo {
			if err := st.RevocarTodosLosRefreshTokensDeUsuario(ctx, usuarioID); err != nil {
				return err
			}
		}
		_, err = st.IncrementarPermisosVersion(ctx, usuarioID)
		return err
	})
}

// CambiarEstadoRol activa o desactiva un rol e invalida los tokens de todos
// sus usuarios vigentes dentro de la misma transacción.
func (s *Servicio) CambiarEstadoRol(ctx context.Context, rolID int64, activo bool, cambiadoPor int64) error {
	return s.ejecutarTx(ctx, cambiadoPor, func(ctx context.Context, st Store) error {
		cambio, err := st.SetRolActivo(ctx, rolID, activo)
		if err != nil || !cambio {
			return err
		}
		return st.IncrementarPermisosVersionPorRol(ctx, rolID)
	})
}

// ReemplazarPermisosRol actualiza el conjunto completo de permisos del rol.
// La modificación y la invalidación de sesiones se confirman o revierten juntas.
func (s *Servicio) ReemplazarPermisosRol(ctx context.Context, rolID int64, codigos []string, cambiadoPor int64) error {
	vistos := make(map[string]struct{}, len(codigos))
	for _, codigo := range codigos {
		if !authz.Existe(codigo) {
			return ErrPermisoInvalido
		}
		if _, duplicado := vistos[codigo]; duplicado {
			return ErrPermisoDuplicado
		}
		vistos[codigo] = struct{}{}
	}

	return s.ejecutarTx(ctx, cambiadoPor, func(ctx context.Context, st Store) error {
		if err := st.ReemplazarPermisosRol(ctx, rolID, codigos); err != nil {
			if errors.Is(err, dto.ErrPermisoNoEncontrado) {
				return ErrPermisoNoEncontrado
			}
			return err
		}
		return st.IncrementarPermisosVersionPorRol(ctx, rolID)
	})
}

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
		if _, err = q.InsertarUsuarioRolVigente(ctx, usuarioID, req.RolID, fechaEfectiva, sql.NullInt64{Int64: creadoPor, Valid: true}); err != nil {
			return err
		}
		// F15 (S7): en la misma Tx, para que el cambio de permisos rija en <= AUTH_CACHE_TTL.
		_, err = q.IncrementarPermisosVersion(ctx, usuarioID)
		return err
	})
}

// RevocarRol cierra la fila vigente de (usuario, rol) sin abrir una nueva
// (solo el paso 1 del patrón de 2 pasos).
func (s *Servicio) RevocarRol(ctx context.Context, usuarioID int64, req RevocarRolRequest, revocadoPor int64) error {
	return s.RevocarRolSeguro(ctx, usuarioID, req, revocadoPor)
}

// ============================================================
// RESET DE CONTRASEÑA (ADR-000 punto d; F14 / S2)
// ============================================================

// SolicitarResetPassword genera un token de un solo uso, persiste SOLO su
// hash SHA-256 y lo entrega al correo del usuario vía Notificador.
//
// Devuelve nil exista o no el usuario, esté activo o no, y también ante
// errores internos (se registran en el log): el handler responde 202 con el
// mismo mensaje en todos los casos, sin revelar si la cuenta existe. El envío
// del correo corre en segundo plano para que su latencia (SMTP) no distinga
// "existe" de "no existe"; queda una diferencia de pocos ms por las escrituras
// en BD, que solo el rate limiting (pendiente) cierra del todo.
func (s *Servicio) SolicitarResetPassword(ctx context.Context, req SolicitarResetPasswordRequest) error {
	usuario, err := s.store.GetUsuarioPorNombreUsuario(ctx, req.NombreUsuario)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.logger.ErrorContext(ctx, "auth: reset de contraseña: error consultando el usuario", "error", err)
		}
		return nil
	}
	if !usuario.Activo {
		return nil
	}
	if s.notificador == nil {
		s.logger.ErrorContext(ctx, "auth: reset de contraseña sin Notificador configurado; no se genera token",
			"usuario_id", usuario.ID)
		return nil
	}

	tokenPlano, err := generarTokenAleatorio()
	if err != nil {
		s.logger.ErrorContext(ctx, "auth: reset de contraseña: no se pudo generar el token", "error", err)
		return nil
	}
	expira := time.Now().Add(s.politica.DuracionTokenReset)

	// Invalidar los pendientes y crear el nuevo, en una sola transacción:
	// o queda exactamente un token vigente o queda el anterior intacto.
	err = s.ejecutarTx(ctx, usuario.ID, func(ctx context.Context, st Store) error {
		if err := st.InvalidarTokensResetPasswordPendientesDeUsuario(ctx, usuario.ID); err != nil {
			return err
		}
		_, err := st.CrearTokenResetPassword(ctx, usuario.ID, hashToken(tokenPlano), expira)
		return err
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "auth: reset de contraseña: no se pudo persistir el token",
			"usuario_id", usuario.ID, "error", err)
		return nil
	}

	s.enviarResetAsync(usuario.Correo, usuario.NombreUsuario, tokenPlano, expira)
	return nil
}

func (s *Servicio) enviarResetAsync(correo, nombre, token string, expira time.Time) {
	select {
	case s.envios <- struct{}{}:
	default:
		s.logger.Error("auth: demasiados envíos de reset en curso; no se envió este correo")
		return
	}
	s.enviosWG.Add(1)
	go func() {
		defer s.enviosWG.Done()
		defer func() { <-s.envios }()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("auth: pánico enviando el correo de reset", "panico", r)
			}
		}()

		// El contexto del request ya se canceló cuando se respondió el 202.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.notificador.EnviarResetPassword(ctx, correo, nombre, token, expira); err != nil {
			s.logger.Error("auth: no se pudo enviar el correo de reset", "error", err)
		}
	}()
}

// ConfirmarResetPassword valida el token, aplica la política de contraseñas y,
// en UNA transacción: consume el token (condicional: gana solo una de varias
// confirmaciones concurrentes), guarda el nuevo hash, limpia el bloqueo por
// intentos fallidos y revoca todas las sesiones del usuario.
//
// La política se valida ANTES de consumir el token: una contraseña rechazada
// no lo quema, el usuario puede reintentar con el mismo enlace.
func (s *Servicio) ConfirmarResetPassword(ctx context.Context, req ConfirmarResetPasswordRequest) error {
	fila, err := s.store.GetTokenResetPasswordPorHash(ctx, hashToken(req.Token))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTokenResetInvalido
		}
		return err
	}
	if fila.Usado || fila.FechaExpiracion.Before(time.Now()) {
		return ErrTokenResetInvalido
	}

	usuario, err := s.store.GetUsuarioPorID(ctx, fila.UsuarioID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTokenResetInvalido
		}
		return err
	}
	if !usuario.Activo {
		return ErrTokenResetInvalido
	}

	if err := ValidarPassword(req.NuevaPassword, usuario.NombreUsuario, usuario.Correo); err != nil {
		return err
	}
	nuevoHash, err := security.HashPassword(req.NuevaPassword)
	if err != nil {
		return err
	}

	return s.ejecutarTx(ctx, usuario.ID, func(ctx context.Context, st Store) error {
		consumido, err := st.ConsumirTokenResetPassword(ctx, fila.ID, time.Now())
		if err != nil {
			return err
		}
		if !consumido {
			return ErrTokenResetInvalido
		}
		if err := st.ActualizarPasswordHash(ctx, usuario.ID, nuevoHash); err != nil {
			return err
		}
		if err := st.ResetearIntentosFallidos(ctx, usuario.ID); err != nil {
			return err
		}
		if err := st.RevocarTodosLosRefreshTokensDeUsuario(ctx, usuario.ID); err != nil {
			return err
		}
		_, err = st.IncrementarPermisosVersion(ctx, usuario.ID) // F15: corta access tokens vivos
		return err
	})
}

// ============================================================
// Helpers internos
// ============================================================

// emitirParDeTokens crea el par access/refresh y persiste el refresh dentro de
// `familiaID`. `st` es s.store (login) o el Store de una transacción (rotación,
// cambio de contraseña). rcp va al claim "rcp" del access token.
func (s *Servicio) emitirParDeTokens(ctx context.Context, st Store, usuarioID int64, nombreUsuario string, permisos []string, rcp bool, pv uint32, familiaID string, origen OrigenSolicitud) (*LoginResponse, error) {
	accessToken, accessPayload, err := s.tokenMaker.CrearAccessToken(usuarioID, nombreUsuario, permisos, rcp, pv, s.politica.DuracionAccessToken)
	if err != nil {
		return nil, err
	}

	refreshToken, refreshPayload, err := s.tokenMaker.CrearRefreshToken(usuarioID, s.politica.DuracionRefreshToken)
	if err != nil {
		return nil, err
	}

	if _, err := st.CrearRefreshTokenConFamilia(ctx, usuarioID, hashToken(refreshToken), refreshPayload.ExpiraEn, familiaID, origen.IP, origen.UserAgent); err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:            accessToken,
		AccessTokenExpiraEn:    accessPayload.ExpiraEn,
		RefreshToken:           refreshToken,
		RefreshTokenExpiraEn:   refreshPayload.ExpiraEn,
		RequiereCambioPassword: rcp,
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
