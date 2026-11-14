package dto

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Usuario refleja las columnas de `usuarios` que security.sql necesita para
// login y control de fuerza bruta (no todas las columnas de la tabla, p.ej.
// No incluye nombre_completo/empleado_id). agrega `correo`: lo necesitan
// el envío del reset y la política de contraseñas (no contener el correo).
type Usuario struct {
	ID                        int64
	NombreUsuario             string
	Correo                    string
	PasswordHash              string
	Activo                    bool
	IntentosFallidos          uint16
	BloqueadoHasta            sql.NullTime
	RequiereCambioPassword    bool
	FechaUltimoCambioPassword sql.NullTime
	PermisosVersion           uint32 // sube con cada cambio que invalide tokens emitidos
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

func scanUsuario(row *sql.Row) (Usuario, error) {
	var u Usuario
	err := row.Scan(
		&u.ID, &u.NombreUsuario, &u.Correo, &u.PasswordHash, &u.Activo,
		&u.IntentosFallidos, &u.BloqueadoHasta,
		&u.RequiereCambioPassword, &u.FechaUltimoCambioPassword,
		&u.PermisosVersion,
		&u.CreatedAt, &u.UpdatedAt,
	)
	return u, err
}

const getUsuarioColumnas = `id, nombre_usuario, correo, password_hash, activo,
       intentos_fallidos, bloqueado_hasta,
       requiere_cambio_password, fecha_ultimo_cambio_password,
       permisos_version,
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

// EstadoIntentos es el estado de fuerza bruta de una cuenta tras registrar un fallo.
type EstadoIntentos struct {
	IntentosFallidos uint16
	BloqueadoHasta   sql.NullTime
}

// RegistrarIntentoFallidoAtomico suma 1 al contador y, si el nuevo
// valor alcanza maxIntentos, fija bloqueado_hasta — todo en un solo UPDATE, de
// modo que dos intentos concurrentes nunca se pisan ni se saltan el umbral.
func (q *Queries) RegistrarIntentoFallidoAtomico(ctx context.Context, id int64, maxIntentos int, bloqueadoHasta time.Time) (EstadoIntentos, error) {
	if _, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET bloqueado_hasta   = IF(intentos_fallidos + 1 >= ?, ?, bloqueado_hasta),
    intentos_fallidos = LEAST(intentos_fallidos + 1, 65535)
WHERE id = ?`, maxIntentos, bloqueadoHasta.UTC(), id); err != nil {
		return EstadoIntentos{}, err
	}

	var e EstadoIntentos
	err := q.db.QueryRowContext(ctx, `
SELECT intentos_fallidos, bloqueado_hasta
FROM usuarios
WHERE id = ?`, id).Scan(&e.IntentosFallidos, &e.BloqueadoHasta)
	return e, err
}

// RehashPasswordHash es el re-hash perezoso bcrypt→argon2id tras un login
// exitoso. A diferencia de ActualizarPasswordHash no toca
// requiere_cambio_password ni fecha_ultimo_cambio_password (un re-hash
// transparente no es un cambio de contraseña del usuario), y solo escribe si
// el hash en BD sigue siendo el que se verificó: si la contraseña cambió entre
// la lectura y este UPDATE, no se pisa con el hash viejo.
func (q *Queries) RehashPasswordHash(ctx context.Context, id int64, hashAnterior, hashNuevo string) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET password_hash = ?
WHERE id = ?
  AND password_hash = ?`, hashNuevo, id, hashAnterior)
	return err
}

// IncrementarIntentosFallidos suma 1 al contador tras un password incorrecto.
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

// ActualizarPasswordHash fija una contraseña NUEVA elegida por el usuario
// (cambio autenticado o confirmación de reset). requiere_cambio_password se
// limpia porque un cambio de password siempre lo satisface.
// No usar para el re-hash perezoso post-login: ver RehashPasswordHash.
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
// "forzar cambio de password".
func (q *Queries) MarcarRequiereCambioPassword(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET requiere_cambio_password = 1
WHERE id = ?`, id)
	return err
}

// RefreshToken refleja `refresh_tokens`. ip_origen/user_agent existen en la
// tabla real pero ninguna query de security.sql los lee, así que no se
// incluyen acá (agregar si una query futura los necesita).
type RefreshToken struct {
	ID              int64
	UsuarioID       int64
	FamiliaID       sql.NullString // NULL en sesiones anteriores a la migración 000114
	TokenHash       string
	FechaEmision    time.Time
	FechaExpiracion time.Time
	Revocado        bool
	FechaRevocacion sql.NullTime
}

// CrearRefreshToken inserta una nueva sesión. token_hash es SHA-256 del
// refresh token; nunca se persiste el token en claro. Devuelve el ID
// autogenerado.
func (q *Queries) CrearRefreshToken(ctx context.Context, usuarioID int64, tokenHash string, fechaExpiracion time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO refresh_tokens (usuario_id, token_hash, fecha_expiracion, fecha_emision)
VALUES (?, ?, ?, NOW())`, usuarioID, tokenHash, fechaExpiracion)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CrearRefreshTokenConFamilia inserta una sesión dentro de una
// familia de rotación (UUID) y registra desde dónde se emitió. Un login abre
// una familia nueva; cada rotación hereda la de la fila que reemplaza.
// ip y userAgent vacíos se guardan como NULL; se truncan al tamaño de la columna.
func (q *Queries) CrearRefreshTokenConFamilia(ctx context.Context, usuarioID int64, tokenHash string, fechaExpiracion time.Time, familiaID, ip, userAgent string) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO refresh_tokens (usuario_id, familia_id, token_hash, fecha_expiracion, fecha_emision, ip_origen, user_agent)
VALUES (?, ?, ?, ?, NOW(), ?, ?)`,
		usuarioID, familiaID, tokenHash, fechaExpiracion,
		nullSiVacio(truncarRunas(ip, 45)), nullSiVacio(truncarRunas(userAgent, 255)))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func nullSiVacio(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func truncarRunas(s string, max int) string {
	if len(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// GetRefreshTokenPorHash busca la sesión por el hash del token presentado.
// Devuelve sql.ErrNoRows si no existe.
func (q *Queries) GetRefreshTokenPorHash(ctx context.Context, tokenHash string) (RefreshToken, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, usuario_id, familia_id, token_hash, fecha_emision, fecha_expiracion, revocado, fecha_revocacion
FROM refresh_tokens
WHERE token_hash = ?
LIMIT 1`, tokenHash)

	var rt RefreshToken
	err := row.Scan(
		&rt.ID, &rt.UsuarioID, &rt.FamiliaID, &rt.TokenHash,
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

// RevocarRefreshTokenSiVigente revoca la fila por ID solo si sigue sin
// revocar, y informa si fue ESTE llamador quien la revocó. Es el paso atómico
// de la rotación: de dos refresh concurrentes con el mismo token, exactamente
// uno obtiene true; el otro (false) es una reutilización.
func (q *Queries) RevocarRefreshTokenSiVigente(ctx context.Context, id int64) (bool, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE refresh_tokens
SET revocado = 1,
    fecha_revocacion = NOW()
WHERE id = ?
  AND revocado = 0`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// RevocarFamiliaRefreshTokens revoca todas las sesiones aún vigentes de una
// cadena de rotación (respuesta a una reutilización detectada).
func (q *Queries) RevocarFamiliaRefreshTokens(ctx context.Context, familiaID string) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE refresh_tokens
SET revocado = 1,
    fecha_revocacion = NOW()
WHERE familia_id = ?
  AND revocado = 0`, familiaID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
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

// ConsumirTokenResetPassword marca el token como usado solo si todavía
// está sin usar y sin expirar, y devuelve true únicamente al llamador que lo
// consumió, de modo que dos confirmaciones concurrentes con el mismo token no
// pasan ambas. `ahora` va en UTC.
func (q *Queries) ConsumirTokenResetPassword(ctx context.Context, id int64, ahora time.Time) (bool, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE tokens_reset_password
SET usado = 1,
    fecha_uso = ?
WHERE id = ?
  AND usado = 0
  AND fecha_expiracion > ?`, ahora.UTC(), id, ahora.UTC())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
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

// CerrarUsuarioRolVigente cierra la asignación vigente de `rolID` para
// `usuarioID`, si existe, fijando su fecha_fin_vigencia. No toca es_vigente:
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

var ErrPermisoNoEncontrado = errors.New("dto: el permiso no existe")

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

// SetUsuarioActivo cambia el estado de una cuenta dentro de la transacción
// recibida. Devuelve false si no hubo cambio; sql.ErrNoRows si el usuario no existe.
func (q *Queries) SetUsuarioActivo(ctx context.Context, usuarioID int64, activo bool) (bool, error) {
	var actual bool
	if err := q.db.QueryRowContext(ctx, `
SELECT activo
FROM usuarios
WHERE id = ?
FOR UPDATE`, usuarioID).Scan(&actual); err != nil {
		return false, err
	}
	if actual == activo {
		return false, nil
	}
	if _, err := q.db.ExecContext(ctx, `UPDATE usuarios SET activo = ? WHERE id = ?`, activo, usuarioID); err != nil {
		return false, err
	}
	return true, nil
}

// SetRolActivo cambia el estado de un rol y bloquea su fila para serializar
// cambios concurrentes con la edición de sus permisos.
func (q *Queries) SetRolActivo(ctx context.Context, rolID int64, activo bool) (bool, error) {
	var actual bool
	if err := q.db.QueryRowContext(ctx, `
SELECT activo
FROM roles
WHERE id = ?
FOR UPDATE`, rolID).Scan(&actual); err != nil {
		return false, err
	}
	if actual == activo {
		return false, nil
	}
	if _, err := q.db.ExecContext(ctx, `UPDATE roles SET activo = ? WHERE id = ?`, activo, rolID); err != nil {
		return false, err
	}
	return true, nil
}

// ReemplazarPermisosRol reemplaza el conjunto completo de permisos de un rol.
// La fila del rol se bloquea para serializar actualizaciones concurrentes;
// los permisos inexistentes abortan la transacción sin dejar cambios parciales.
func (q *Queries) ReemplazarPermisosRol(ctx context.Context, rolID int64, codigos []string) error {
	var existe int64
	if err := q.db.QueryRowContext(ctx, `SELECT id FROM roles WHERE id = ? FOR UPDATE`, rolID).Scan(&existe); err != nil {
		return err
	}

	ids := make([]int64, 0, len(codigos))
	vistos := make(map[string]struct{}, len(codigos))
	for _, codigo := range codigos {
		if _, duplicado := vistos[codigo]; duplicado {
			return errors.New("dto: código de permiso duplicado")
		}
		vistos[codigo] = struct{}{}
		var permisoID int64
		err := q.db.QueryRowContext(ctx, `SELECT id FROM permisos WHERE codigo = ?`, codigo).Scan(&permisoID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPermisoNoEncontrado
		}
		if err != nil {
			return err
		}
		ids = append(ids, permisoID)
	}

	if _, err := q.db.ExecContext(ctx, `DELETE FROM rol_permisos WHERE rol_id = ?`, rolID); err != nil {
		return err
	}
	for _, permisoID := range ids {
		if _, err := q.db.ExecContext(ctx, `
INSERT INTO rol_permisos (rol_id, permiso_id)
VALUES (?, ?)`, rolID, permisoID); err != nil {
			return err
		}
	}
	return nil
}

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

// IncrementarPermisosVersion sube permisos_version del usuario y devuelve el
// valor nuevo. Debe llamarse DENTRO de la misma Tx que el cambio que invalida
// los tokens (asignar/revocar rol, activar/desactivar, cambio de contraseña).
func (q *Queries) IncrementarPermisosVersion(ctx context.Context, usuarioID int64) (uint32, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET permisos_version = permisos_version + 1
WHERE id = ?`, usuarioID)
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return 0, err
	} else if n == 0 {
		return 0, sql.ErrNoRows
	}
	var v uint32
	err = q.db.QueryRowContext(ctx, `SELECT permisos_version FROM usuarios WHERE id = ?`, usuarioID).Scan(&v)
	return v, err
}

// IncrementarPermisosVersionPorRol sube la versión de todos los usuarios con
// ese rol vigente. Usar al cambiar rol_permisos o desactivar un rol (afecta a
// muchos usuarios a la vez).
func (q *Queries) IncrementarPermisosVersionPorRol(ctx context.Context, rolID int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE usuarios u
JOIN usuario_roles ur ON ur.usuario_id = u.id AND ur.es_vigente = 1
SET u.permisos_version = u.permisos_version + 1
WHERE ur.rol_id = ?`, rolID)
	return err
}

// EstadoSesion es lo mínimo que el middleware necesita por request.
type EstadoSesion struct {
	Activo          bool
	PermisosVersion uint32
}

// GetEstadoSesion devuelve activo y permisos_version del usuario. Devuelve
// sql.ErrNoRows si no existe.
func (q *Queries) GetEstadoSesion(ctx context.Context, usuarioID int64) (EstadoSesion, error) {
	var e EstadoSesion
	err := q.db.QueryRowContext(ctx, `
SELECT activo, permisos_version
FROM usuarios
WHERE id = ?
LIMIT 1`, usuarioID).Scan(&e.Activo, &e.PermisosVersion)
	return e, err
}
