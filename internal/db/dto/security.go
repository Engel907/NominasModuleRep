package dto

import (
	"context"
	"database/sql"
	"time"
)

// ============================================================================
// NOTA DE MIGRACIÓN (Fase 5B, ver internal/db/queries/security.sql)
//
// Las queries anotadas de security.sql se escribieron antes de que
// db/schema.sql tuviera su forma final (varias columnas se agregaron o
// renombraron en 000001_seguridad_reforzada.up.sql). Este archivo implementa
// la INTENCIÓN de cada query (mismo nombre, mismo :one/:many/:exec), pero
// contra las columnas REALES de db/schema.sql, confirmadas ahí antes de
// escribir cada Scan(). Diferencias puntuales:
//
//   - refresh_tokens: la tabla real usa `fecha_emision` / `fecha_expiracion`,
//     no `creado_en` / `expira_en` como decía la query original.
//   - tokens_reset_password: igual, la tabla real usa `fecha_creacion` /
//     `fecha_expiracion`, no `creado_en` / `expira_en`.
//   - usuario_roles.es_vigente es una COLUMNA GENERADA (STORED, calculada a
//     partir de fecha_fin_vigencia) — MySQL rechaza cualquier INSERT/UPDATE
//     que le pase un valor explícito. CerrarUsuarioRolVigente e
//     InsertarUsuarioRolVigente de abajo NUNCA escriben es_vigente: solo
//     escriben fecha_fin_vigencia (NULL = vigente) y dejan que el motor
//     recalcule es_vigente solo. Sí se puede filtrar por es_vigente en el
//     WHERE (es una columna normal para lectura), y así se hace.
//
// Todos los IDs de tablas con PK BIGINT UNSIGNED se manejan acá como int64
// (decisión D1, igual en todo internal/db/dto): simplifica las firmas y el
// Scan(), y ningún ID de este sistema se acerca remotamente a superar el
// rango de int64.
// ============================================================================

// ============================================================================
// USUARIOS / LOGIN / FUERZA BRUTA
// ============================================================================

// Usuario refleja las columnas de `usuarios` que security.sql necesita para
// login y control de fuerza bruta (no todas las columnas de la tabla, p.ej.
// no incluye nombre_completo/correo/empleado_id porque ninguna query de
// security.sql las pide).
type Usuario struct {
	ID                        int64
	NombreUsuario             string
	PasswordHash              string
	Activo                    bool
	IntentosFallidos          uint16
	BloqueadoHasta            sql.NullTime
	RequiereCambioPassword    bool
	FechaUltimoCambioPassword sql.NullTime
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

func scanUsuario(row *sql.Row) (Usuario, error) {
	var u Usuario
	err := row.Scan(
		&u.ID, &u.NombreUsuario, &u.PasswordHash, &u.Activo,
		&u.IntentosFallidos, &u.BloqueadoHasta,
		&u.RequiereCambioPassword, &u.FechaUltimoCambioPassword,
		&u.CreatedAt, &u.UpdatedAt,
	)
	return u, err
}

const getUsuarioColumnas = `id, nombre_usuario, password_hash, activo,
       intentos_fallidos, bloqueado_hasta,
       requiere_cambio_password, fecha_ultimo_cambio_password,
       created_at, updated_at`

// GetUsuarioPorNombreUsuario busca la cuenta por login. Devuelve
// sql.ErrNoRows si no existe.
func (q *Queries) GetUsuarioPorNombreUsuario(ctx context.Context, nombreUsuario string) (Usuario, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT `+getUsuarioColumnas+`
FROM usuarios
WHERE nombre_usuario = ?
LIMIT 1`, nombreUsuario)
	return scanUsuario(row)
}

// GetUsuarioPorID busca la cuenta por ID. Devuelve sql.ErrNoRows si no existe.
func (q *Queries) GetUsuarioPorID(ctx context.Context, id int64) (Usuario, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT `+getUsuarioColumnas+`
FROM usuarios
WHERE id = ?
LIMIT 1`, id)
	return scanUsuario(row)
}

// IncrementarIntentosFallidos suma 1 al contador tras un password incorrecto.
// El umbral (N) y los minutos de bloqueo son política de internal/auth
// (ADR-000 punto c), no viven acá.
func (q *Queries) IncrementarIntentosFallidos(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET intentos_fallidos = intentos_fallidos + 1
WHERE id = ?`, id)
	return err
}

// BloquearUsuarioTemporalmente se invoca desde el servicio cuando
// intentos_fallidos alcanza el umbral N.
func (q *Queries) BloquearUsuarioTemporalmente(ctx context.Context, id int64, bloqueadoHasta time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET bloqueado_hasta = ?
WHERE id = ?`, bloqueadoHasta, id)
	return err
}

// ResetearIntentosFallidos se invoca tras un login exitoso.
func (q *Queries) ResetearIntentosFallidos(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET intentos_fallidos = 0,
    bloqueado_hasta = NULL
WHERE id = ?`, id)
	return err
}

// ActualizarPasswordHash se usa tanto para el re-hash perezoso bcrypt→argon2id
// post-login como para confirmar un reset de contraseña.
// requiere_cambio_password se limpia explícitamente porque un cambio de
// password siempre lo satisface.
func (q *Queries) ActualizarPasswordHash(ctx context.Context, id int64, passwordHash string) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET password_hash = ?,
    requiere_cambio_password = 0,
    fecha_ultimo_cambio_password = NOW()
WHERE id = ?`, passwordHash, id)
	return err
}

// MarcarRequiereCambioPassword implementa el flujo administrativo de
// "forzar cambio de password" (ADR-000 punto d).
func (q *Queries) MarcarRequiereCambioPassword(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET requiere_cambio_password = 1
WHERE id = ?`, id)
	return err
}

// ============================================================================
// REFRESH TOKENS (sesiones persistidas/revocables — ADR-000 punto b)
// ============================================================================

// RefreshToken refleja `refresh_tokens`. ip_origen/user_agent existen en la
// tabla real pero ninguna query de security.sql los lee, así que no se
// incluyen acá (agregar si una query futura los necesita).
type RefreshToken struct {
	ID              int64
	UsuarioID       int64
	TokenHash       string
	FechaEmision    time.Time
	FechaExpiracion time.Time
	Revocado        bool
	FechaRevocacion sql.NullTime
}

// CrearRefreshToken inserta una nueva sesión. token_hash es SHA-256 del
// refresh token PASETO en claro — nunca se persiste el token en claro
// (ADR-000 punto b). Devuelve el ID autogenerado.
func (q *Queries) CrearRefreshToken(ctx context.Context, usuarioID int64, tokenHash string, fechaExpiracion time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO refresh_tokens (usuario_id, token_hash, fecha_expiracion, fecha_emision)
VALUES (?, ?, ?, NOW())`, usuarioID, tokenHash, fechaExpiracion)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetRefreshTokenPorHash busca la sesión por el hash del token presentado.
// Devuelve sql.ErrNoRows si no existe.
func (q *Queries) GetRefreshTokenPorHash(ctx context.Context, tokenHash string) (RefreshToken, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, usuario_id, token_hash, fecha_emision, fecha_expiracion, revocado, fecha_revocacion
FROM refresh_tokens
WHERE token_hash = ?
LIMIT 1`, tokenHash)

	var rt RefreshToken
	err := row.Scan(
		&rt.ID, &rt.UsuarioID, &rt.TokenHash,
		&rt.FechaEmision, &rt.FechaExpiracion,
		&rt.Revocado, &rt.FechaRevocacion,
	)
	return rt, err
}

// RevocarRefreshTokenPorHash se usa en logout de una sola sesión y en
// rotación de refresh token.
func (q *Queries) RevocarRefreshTokenPorHash(ctx context.Context, tokenHash string) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE refresh_tokens
SET revocado = 1,
    fecha_revocacion = NOW()
WHERE token_hash = ?
  AND revocado = 0`, tokenHash)
	return err
}

// RevocarTodosLosRefreshTokensDeUsuario se usa tras un reset de contraseña o
// una acción administrativa de "cerrar todas las sesiones" del usuario.
func (q *Queries) RevocarTodosLosRefreshTokensDeUsuario(ctx context.Context, usuarioID int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE refresh_tokens
SET revocado = 1,
    fecha_revocacion = NOW()
WHERE usuario_id = ?
  AND revocado = 0`, usuarioID)
	return err
}

// ============================================================================
// RESET DE CONTRASEÑA (ADR-000 punto d)
// ============================================================================

// TokenResetPassword refleja `tokens_reset_password`. ip_origen existe en la
// tabla real pero ninguna query de security.sql lo lee.
type TokenResetPassword struct {
	ID              int64
	UsuarioID       int64
	TokenHash       string
	FechaCreacion   time.Time
	FechaExpiracion time.Time
	Usado           bool
}

// CrearTokenResetPassword inserta un token de un solo uso. token_hash es
// SHA-256 del token plano enviado al usuario (por correo u otro canal, fuera
// del alcance de este paquete). Devuelve el ID autogenerado.
func (q *Queries) CrearTokenResetPassword(ctx context.Context, usuarioID int64, tokenHash string, fechaExpiracion time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO tokens_reset_password (usuario_id, token_hash, fecha_expiracion, usado, fecha_creacion)
VALUES (?, ?, ?, 0, NOW())`, usuarioID, tokenHash, fechaExpiracion)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetTokenResetPasswordPorHash busca el token por su hash. Devuelve
// sql.ErrNoRows si no existe.
func (q *Queries) GetTokenResetPasswordPorHash(ctx context.Context, tokenHash string) (TokenResetPassword, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, usuario_id, token_hash, fecha_expiracion, usado, fecha_creacion
FROM tokens_reset_password
WHERE token_hash = ?
LIMIT 1`, tokenHash)

	var t TokenResetPassword
	err := row.Scan(&t.ID, &t.UsuarioID, &t.TokenHash, &t.FechaExpiracion, &t.Usado, &t.FechaCreacion)
	return t, err
}

// MarcarTokenResetPasswordUsado marca el token como consumido; un token
// usado nunca vuelve a aceptarse.
func (q *Queries) MarcarTokenResetPasswordUsado(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE tokens_reset_password
SET usado = 1
WHERE id = ?`, id)
	return err
}

// InvalidarTokensResetPasswordPendientesDeUsuario invalida los tokens no
// usados anteriores del usuario al emitir uno nuevo, para que solo el más
// reciente sea válido.
func (q *Queries) InvalidarTokensResetPasswordPendientesDeUsuario(ctx context.Context, usuarioID int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE tokens_reset_password
SET usado = 1
WHERE usuario_id = ?
  AND usado = 0`, usuarioID)
	return err
}

// ============================================================================
// usuario_roles — tabla versionada (ADR-000 puntos f, i; §2)
//
// Regla de escritura de 2 pasos, SIEMPRE dentro de la misma Tx (nunca una
// sin la otra fuera de internal/shared/vigencia):
//   1) CerrarUsuarioRolVigente
//   2) InsertarUsuarioRolVigente
//
// La unicidad real es por (usuario_id, rol_id, es_vigente) — permite N roles
// vigentes simultáneos por usuario, a diferencia de las otras tablas con
// patrón de vigencia que son únicas por su entidad principal.
// ============================================================================

// CerrarUsuarioRolVigente cierra la asignación vigente de `rolID` para
// `usuarioID`, si existe, fijando su fecha_fin_vigencia. NO toca es_vigente:
// es una columna generada (STORED) que MySQL recalcula solo a partir de
// fecha_fin_vigencia, y rechaza cualquier escritura explícita sobre ella.
func (q *Queries) CerrarUsuarioRolVigente(ctx context.Context, usuarioID, rolID int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE usuario_roles
SET fecha_fin_vigencia = ?
WHERE usuario_id = ?
  AND rol_id = ?
  AND es_vigente = 1`, fechaFinVigencia, usuarioID, rolID)
	return err
}

// InsertarUsuarioRolVigente otorga `rolID` a `usuarioID` como asignación
// vigente (fecha_fin_vigencia queda NULL; es_vigente se recalcula solo).
// creadoPor va NULL solo en cargas de migración inicial sin operador
// identificable. Devuelve el ID autogenerado de la fila.
func (q *Queries) InsertarUsuarioRolVigente(ctx context.Context, usuarioID, rolID int64, fechaInicioVigencia time.Time, creadoPor sql.NullInt64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO usuario_roles
    (usuario_id, rol_id, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por, fecha_asignacion)
VALUES (?, ?, ?, NULL, ?, NOW())`, usuarioID, rolID, fechaInicioVigencia, creadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RolVigenteDeUsuario es el resultado de GetRolesVigentesDeUsuario: una fila
// de usuario_roles vigente, unida a su rol.
type RolVigenteDeUsuario struct {
	ID                  int64
	RolID               int64
	Codigo              string
	Descripcion         sql.NullString
	FechaInicioVigencia time.Time
}

// GetRolesVigentesDeUsuario lista los roles vigentes (es_vigente = 1) y
// activos (roles.activo = 1) del usuario.
func (q *Queries) GetRolesVigentesDeUsuario(ctx context.Context, usuarioID int64) ([]RolVigenteDeUsuario, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT ur.id, ur.rol_id, r.codigo, r.descripcion, ur.fecha_inicio_vigencia
FROM usuario_roles ur
JOIN roles r ON r.id = ur.rol_id
WHERE ur.usuario_id = ?
  AND ur.es_vigente = 1
  AND r.activo = 1`, usuarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resultado []RolVigenteDeUsuario
	for rows.Next() {
		var rv RolVigenteDeUsuario
		if err := rows.Scan(&rv.ID, &rv.RolID, &rv.Codigo, &rv.Descripcion, &rv.FechaInicioVigencia); err != nil {
			return nil, err
		}
		resultado = append(resultado, rv)
	}
	return resultado, rows.Err()
}

// Rol refleja las columnas de `roles` que GetRolPorID necesita (no incluye
// `nombre`, que la query original tampoco selecciona).
type Rol struct {
	ID          int64
	Codigo      string
	Descripcion sql.NullString
	Activo      bool
}

// GetRolPorID busca un rol por ID. Devuelve sql.ErrNoRows si no existe.
func (q *Queries) GetRolPorID(ctx context.Context, id int64) (Rol, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, codigo, descripcion, activo
FROM roles
WHERE id = ?
LIMIT 1`, id)

	var r Rol
	err := row.Scan(&r.ID, &r.Codigo, &r.Descripcion, &r.Activo)
	return r, err
}

// ============================================================================
// RESOLUCIÓN DE PERMISOS (ADR-000 §3 — autorización por permiso, no por rol)
//
// Esta es LA query que arma el payload del access token, y la que usaría el
// middleware/servicio si se decide resolver contra BD en cada request en vez
// de confiar en el payload del token.
// ============================================================================

// GetPermisosVigentesDeUsuario devuelve los códigos de permiso (sin
// duplicados) que el usuario tiene por sus roles vigentes y activos,
// solo si la cuenta misma está activa.
func (q *Queries) GetPermisosVigentesDeUsuario(ctx context.Context, usuarioID int64) ([]string, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT DISTINCT p.codigo
FROM usuarios u
JOIN usuario_roles ur ON ur.usuario_id = u.id AND ur.es_vigente = 1
JOIN roles r ON r.id = ur.rol_id AND r.activo = 1
JOIN rol_permisos rp ON rp.rol_id = r.id
JOIN permisos p ON p.id = rp.permiso_id
WHERE u.id = ?
  AND u.activo = 1`, usuarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codigos []string
	for rows.Next() {
		var codigo string
		if err := rows.Scan(&codigo); err != nil {
			return nil, err
		}
		codigos = append(codigos, codigo)
	}
	return codigos, rows.Err()
}