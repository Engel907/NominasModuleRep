package dto

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type UsuarioAdministracion struct {
	ID                     int64         `json:"id"`
	NombreUsuario          string        `json:"nombre_usuario"`
	NombreCompleto         string        `json:"nombre_completo"`
	Correo                 string        `json:"correo"`
	EmpleadoID             sql.NullInt64 `json:"-"`
	EmpleadoIDJSON         *int64        `json:"empleado_id"`
	Activo                 bool          `json:"activo"`
	RequiereCambioPassword bool          `json:"requiere_cambio_password"`
	PermisosVersion        uint32        `json:"-"`
	Roles                  []string      `json:"roles"`
	CreatedAt              time.Time     `json:"created_at"`
	UpdatedAt              time.Time     `json:"updated_at"`
}

func usuarioAdminDesdeRow(row interface{ Scan(...any) error }) (UsuarioAdministracion, error) {
	var u UsuarioAdministracion
	var roles sql.NullString
	if err := row.Scan(
		&u.ID, &u.NombreUsuario, &u.NombreCompleto, &u.Correo, &u.EmpleadoID,
		&u.Activo, &u.RequiereCambioPassword, &u.PermisosVersion, &u.CreatedAt, &u.UpdatedAt, &roles,
	); err != nil {
		return UsuarioAdministracion{}, err
	}
	if u.EmpleadoID.Valid {
		id := u.EmpleadoID.Int64
		u.EmpleadoIDJSON = &id
	}
	if roles.Valid && roles.String != "" {
		for _, rol := range strings.Split(roles.String, ",") {
			u.Roles = append(u.Roles, rol)
		}
	}
	if u.Roles == nil {
		u.Roles = []string{}
	}
	return u, nil
}

const usuarioAdminSelect = `SELECT u.id, u.nombre_usuario, u.nombre_completo, u.correo,
       u.empleado_id, u.activo, u.requiere_cambio_password, u.permisos_version,
       u.created_at, u.updated_at, GROUP_CONCAT(DISTINCT r.codigo ORDER BY r.codigo SEPARATOR ',')
FROM usuarios u
LEFT JOIN usuario_roles ur ON ur.usuario_id = u.id AND ur.es_vigente = 1
LEFT JOIN roles r ON r.id = ur.rol_id AND r.activo = 1`

func (q *Queries) CrearUsuarioAdministracion(ctx context.Context, nombreUsuario, nombreCompleto, correo string, empleadoID sql.NullInt64, passwordHash string) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash, empleado_id, activo, requiere_cambio_password)
VALUES (?, ?, ?, ?, ?, 1, 1)`, nombreUsuario, nombreCompleto, correo, passwordHash, empleadoID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) ActualizarUsuarioAdministracion(ctx context.Context, id int64, nombreCompleto, correo *string, empleadoID **int64) (bool, error) {
	var existe int
	if err := q.db.QueryRowContext(ctx, `SELECT 1 FROM usuarios WHERE id = ? FOR UPDATE`, id).Scan(&existe); err != nil {
		return false, err
	}
	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if nombreCompleto != nil {
		sets = append(sets, "nombre_completo = ?")
		args = append(args, *nombreCompleto)
	}
	if correo != nil {
		sets = append(sets, "correo = ?")
		args = append(args, *correo)
	}
	if empleadoID != nil {
		sets = append(sets, "empleado_id = ?")
		if *empleadoID == nil {
			args = append(args, nil)
		} else {
			args = append(args, **empleadoID)
		}
	}
	if len(sets) == 0 {
		return false, nil
	}
	args = append(args, id)
	_, err := q.db.ExecContext(ctx, `UPDATE usuarios SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	return err == nil, err
}

func (q *Queries) RestablecerPasswordTemporal(ctx context.Context, usuarioID int64, hash string) error {
	res, err := q.db.ExecContext(ctx, `
UPDATE usuarios
SET password_hash = ?, requiere_cambio_password = 1, fecha_ultimo_cambio_password = NOW(),
    intentos_fallidos = 0, bloqueado_hasta = NULL
WHERE id = ?`, hash, usuarioID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (q *Queries) UsuarioAdministracionPorID(ctx context.Context, id int64) (UsuarioAdministracion, error) {
	return usuarioAdminDesdeRow(q.db.QueryRowContext(ctx, usuarioAdminSelect+`
WHERE u.id = ?
GROUP BY u.id`, id))
}

type FiltroUsuarios struct {
	Texto  string
	Activo *bool
	Rol    string
	Limit  int32
	Offset int32
}

func (q *Queries) ListarUsuariosAdministracion(ctx context.Context, f FiltroUsuarios) ([]UsuarioAdministracion, int64, error) {
	where := make([]string, 0, 3)
	args := make([]any, 0, 5)
	if f.Texto != "" {
		where = append(where, "(u.nombre_usuario LIKE ? OR u.nombre_completo LIKE ? OR u.correo LIKE ?)")
		needle := "%" + f.Texto + "%"
		args = append(args, needle, needle, needle)
	}
	if f.Activo != nil {
		where = append(where, "u.activo = ?")
		args = append(args, *f.Activo)
	}
	if f.Rol != "" {
		where = append(where, `EXISTS (
SELECT 1 FROM usuario_roles fur
JOIN roles fr ON fr.id = fur.rol_id
WHERE fur.usuario_id = u.id AND fur.es_vigente = 1 AND fr.activo = 1 AND fr.codigo = ?)`)
		args = append(args, f.Rol)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int64
	if err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usuarios u`+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	rows, err := q.db.QueryContext(ctx, usuarioAdminSelect+clause+`
GROUP BY u.id
ORDER BY u.id
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var usuarios []UsuarioAdministracion
	for rows.Next() {
		u, err := usuarioAdminDesdeRow(rows)
		if err != nil {
			return nil, 0, err
		}
		usuarios = append(usuarios, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if usuarios == nil {
		usuarios = []UsuarioAdministracion{}
	}
	return usuarios, total, nil
}

type RolAdministracion struct {
	ID          int64  `json:"id"`
	Codigo      string `json:"codigo"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
	Activo      bool   `json:"activo"`
}

type PermisoAdministracion struct {
	Codigo string `json:"codigo"`
	Nombre string `json:"nombre"`
	Modulo string `json:"modulo"`
}

func (q *Queries) ListarRolesAdministracion(ctx context.Context) ([]RolAdministracion, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, codigo, nombre, COALESCE(descripcion, ''), activo
FROM roles ORDER BY codigo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RolAdministracion{}
	for rows.Next() {
		var r RolAdministracion
		if err := rows.Scan(&r.ID, &r.Codigo, &r.Nombre, &r.Descripcion, &r.Activo); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ListarPermisosDeRol(ctx context.Context, rolID int64) ([]PermisoAdministracion, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT p.codigo, p.nombre, p.modulo
FROM roles r
JOIN rol_permisos rp ON rp.rol_id = r.id
JOIN permisos p ON p.id = rp.permiso_id
WHERE r.id = ?
ORDER BY p.modulo, p.codigo`, rolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PermisoAdministracion{}
	for rows.Next() {
		var p PermisoAdministracion
		if err := rows.Scan(&p.Codigo, &p.Nombre, &p.Modulo); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (q *Queries) ContarAdministradoresActivos(ctx context.Context) (int64, error) {
	var n int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(DISTINCT u.id)
FROM usuarios u
JOIN usuario_roles ur ON ur.usuario_id = u.id AND ur.es_vigente = 1
JOIN roles r ON r.id = ur.rol_id AND r.codigo = 'ADMINISTRADOR_SISTEMA' AND r.activo = 1
WHERE u.activo = 1`).Scan(&n)
	return n, err
}

func (q *Queries) BloquearRolPorID(ctx context.Context, rolID int64) (Rol, error) {
	var rol Rol
	err := q.db.QueryRowContext(ctx, `
SELECT id, codigo, descripcion, activo
FROM roles WHERE id = ? FOR UPDATE`, rolID).Scan(&rol.ID, &rol.Codigo, &rol.Descripcion, &rol.Activo)
	return rol, err
}

func (q *Queries) UsuarioTieneRolVigente(ctx context.Context, usuarioID, rolID int64) (bool, error) {
	var id int64
	err := q.db.QueryRowContext(ctx, `
SELECT id FROM usuario_roles
WHERE usuario_id = ? AND rol_id = ? AND es_vigente = 1
LIMIT 1 FOR UPDATE`, usuarioID, rolID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (q *Queries) EmpleadoIDDeUsuario(ctx context.Context, usuarioID int64) (int64, error) {
	var empleadoID sql.NullInt64
	err := q.db.QueryRowContext(ctx, `SELECT empleado_id FROM usuarios WHERE id = ? AND activo = 1`, usuarioID).Scan(&empleadoID)
	if err != nil {
		return 0, err
	}
	if !empleadoID.Valid {
		return 0, sql.ErrNoRows
	}
	return empleadoID.Int64, nil
}

type RegistroAuditoria struct {
	ID            int64           `json:"id"`
	TablaAfectada string          `json:"tabla_afectada"`
	RegistroID    int64           `json:"registro_id"`
	UsuarioID     sql.NullInt64   `json:"-"`
	UsuarioIDJSON *int64          `json:"usuario_id"`
	NombreUsuario sql.NullString  `json:"-"`
	Usuario       *string         `json:"usuario"`
	Accion        string          `json:"accion"`
	ValorAnterior json.RawMessage `json:"valor_anterior"`
	ValorNuevo    json.RawMessage `json:"valor_nuevo"`
	FechaCambio   time.Time       `json:"fecha_cambio"`
}

type FiltroAuditoria struct {
	Tabla      string
	RegistroID *int64
	UsuarioID  *int64
	Desde      *time.Time
	Hasta      *time.Time
	Limit      int32
	Offset     int32
}

func (q *Queries) ListarAuditoria(ctx context.Context, f FiltroAuditoria) ([]RegistroAuditoria, int64, error) {
	where := make([]string, 0, 5)
	args := make([]any, 0, 7)
	if f.Tabla != "" {
		where = append(where, "a.tabla_afectada = ?")
		args = append(args, f.Tabla)
	}
	if f.RegistroID != nil {
		where = append(where, "a.registro_id = ?")
		args = append(args, *f.RegistroID)
	}
	if f.UsuarioID != nil {
		where = append(where, "a.usuario_id = ?")
		args = append(args, *f.UsuarioID)
	}
	if f.Desde != nil {
		where = append(where, "a.fecha_cambio >= ?")
		args = append(args, *f.Desde)
	}
	if f.Hasta != nil {
		where = append(where, "a.fecha_cambio <= ?")
		args = append(args, *f.Hasta)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int64
	if err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auditoria_cambios a`+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	rows, err := q.db.QueryContext(ctx, `
SELECT a.id, a.tabla_afectada, a.registro_id, a.usuario_id, u.nombre_usuario, a.accion,
       CAST(a.valor_anterior AS CHAR), CAST(a.valor_nuevo AS CHAR), a.fecha_cambio
FROM auditoria_cambios a
LEFT JOIN usuarios u ON u.id = a.usuario_id`+clause+`
ORDER BY a.fecha_cambio DESC, a.id DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []RegistroAuditoria{}
	for rows.Next() {
		var r RegistroAuditoria
		var anterior, nuevo sql.NullString
		if err := rows.Scan(&r.ID, &r.TablaAfectada, &r.RegistroID, &r.UsuarioID, &r.NombreUsuario,
			&r.Accion, &anterior, &nuevo, &r.FechaCambio); err != nil {
			return nil, 0, err
		}
		if r.UsuarioID.Valid {
			id := r.UsuarioID.Int64
			r.UsuarioIDJSON = &id
		}
		if r.NombreUsuario.Valid {
			nombre := r.NombreUsuario.String
			r.Usuario = &nombre
		}
		if anterior.Valid {
			r.ValorAnterior = json.RawMessage(anterior.String)
		}
		if nuevo.Valid {
			r.ValorNuevo = json.RawMessage(nuevo.String)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
