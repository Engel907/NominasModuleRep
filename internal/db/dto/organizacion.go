// internal/db/dto/organizacion.go
//
// Fase 3B — RRHH: catálogos organizacionales (clasificaciones de puesto,
// programas presupuestarios, unidades organizativas, puestos).
//
// NOTA DE MIGRACIÓN (mismo criterio que security.go de la Fase 5A): las
// columnas de abajo se verificaron contra db/schema.sql, NO contra
// db/queries/empleados_puestos.sql — ese archivo de queries fue escrito
// contra un esquema supuesto que en varios puntos no coincide con las
// tablas reales. Diferencias puntuales para estos 4 catálogos:
//
//   - clasificaciones_puesto y programas_presupuestarios NO tienen columnas
//     created_at/updated_at en el schema real (sí tienen `activo`). Los
//     Create/Update de abajo no las escriben porque no existen.
//   - puestos NO tiene columna `descripcion` en el schema real. Se quitó el
//     campo Descripcion de PuestoInput/PuestoUpdateInput en
//     internal/services/organizacion_service.go — no hay dónde persistirlo.
//   - unidades_organizativas NO tiene columna programa_presupuestario_id en
//     el schema real (ese campo vive en empleado_datos_laborales, no en la
//     unidad). Se quitó ProgramaPresupuestarioID de UnidadOrganizativaInput/
//     UpdateInput/ListRequest en internal/services/organizacion_service.go.
//   - unidades_organizativas SÍ tiene created_at/updated_at, a diferencia de
//     los otros 3 catálogos de este archivo.
package dto

import (
	"context"
	"database/sql"
	"time"
)

// ============================================================================
// CLASIFICACIONES_PUESTO
// ============================================================================

type ClasificacionesPuesto struct {
	ID     int64
	Codigo string
	Nombre string
	Activo bool
}

func scanClasificacionPuesto(row *sql.Row) (ClasificacionesPuesto, error) {
	var c ClasificacionesPuesto
	err := row.Scan(&c.ID, &c.Codigo, &c.Nombre, &c.Activo)
	return c, err
}

func (q *Queries) CreateClasificacionPuesto(ctx context.Context, codigo, nombre string) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO clasificaciones_puesto (codigo, nombre, activo)
VALUES (?, ?, 1)`, codigo, nombre)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) GetClasificacionPuestoByID(ctx context.Context, id int64) (ClasificacionesPuesto, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, codigo, nombre, activo
FROM clasificaciones_puesto
WHERE id = ?
LIMIT 1`, id)
	return scanClasificacionPuesto(row)
}

func (q *Queries) UpdateClasificacionPuesto(ctx context.Context, id int64, nombre string, activo bool) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE clasificaciones_puesto
SET nombre = ?, activo = ?
WHERE id = ?`, nombre, activo, id)
	return err
}

func (q *Queries) ListClasificacionesPuesto(ctx context.Context, activo *bool, nombre *string, limit, offset int32) ([]ClasificacionesPuesto, error) {
	var activoArg, nombreArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if nombre != nil {
		nombreArg = *nombre
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT id, codigo, nombre, activo
FROM clasificaciones_puesto
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR nombre LIKE CONCAT('%', ?, '%'))
ORDER BY nombre ASC
LIMIT ? OFFSET ?`, activoArg, activoArg, nombreArg, nombreArg, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ClasificacionesPuesto
	for rows.Next() {
		var c ClasificacionesPuesto
		if err := rows.Scan(&c.ID, &c.Codigo, &c.Nombre, &c.Activo); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (q *Queries) CountClasificacionesPuesto(ctx context.Context, activo *bool, nombre *string) (int64, error) {
	var activoArg, nombreArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if nombre != nil {
		nombreArg = *nombre
	}
	row := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM clasificaciones_puesto
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR nombre LIKE CONCAT('%', ?, '%'))`, activoArg, activoArg, nombreArg, nombreArg)
	var total int64
	err := row.Scan(&total)
	return total, err
}

// ============================================================================
// PROGRAMAS_PRESUPUESTARIOS
// ============================================================================

type ProgramasPresupuestario struct {
	ID     int64
	Codigo string
	Nombre string
	Activo bool
	Notas  sql.NullString
}

func scanProgramaPresupuestario(row *sql.Row) (ProgramasPresupuestario, error) {
	var p ProgramasPresupuestario
	err := row.Scan(&p.ID, &p.Codigo, &p.Nombre, &p.Activo, &p.Notas)
	return p, err
}

func (q *Queries) CreateProgramaPresupuestario(ctx context.Context, codigo, nombre string) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO programas_presupuestarios (codigo, nombre, activo)
VALUES (?, ?, 1)`, codigo, nombre)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) GetProgramaPresupuestarioByID(ctx context.Context, id int64) (ProgramasPresupuestario, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, codigo, nombre, activo, notas
FROM programas_presupuestarios
WHERE id = ?
LIMIT 1`, id)
	return scanProgramaPresupuestario(row)
}

func (q *Queries) UpdateProgramaPresupuestario(ctx context.Context, id int64, nombre string, activo bool) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE programas_presupuestarios
SET nombre = ?, activo = ?
WHERE id = ?`, nombre, activo, id)
	return err
}

func (q *Queries) ListProgramasPresupuestarios(ctx context.Context, activo *bool, nombre *string, limit, offset int32) ([]ProgramasPresupuestario, error) {
	var activoArg, nombreArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if nombre != nil {
		nombreArg = *nombre
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT id, codigo, nombre, activo, notas
FROM programas_presupuestarios
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR nombre LIKE CONCAT('%', ?, '%'))
ORDER BY nombre ASC
LIMIT ? OFFSET ?`, activoArg, activoArg, nombreArg, nombreArg, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProgramasPresupuestario
	for rows.Next() {
		var p ProgramasPresupuestario
		if err := rows.Scan(&p.ID, &p.Codigo, &p.Nombre, &p.Activo, &p.Notas); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (q *Queries) CountProgramasPresupuestarios(ctx context.Context, activo *bool, nombre *string) (int64, error) {
	var activoArg, nombreArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if nombre != nil {
		nombreArg = *nombre
	}
	row := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM programas_presupuestarios
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR nombre LIKE CONCAT('%', ?, '%'))`, activoArg, activoArg, nombreArg, nombreArg)
	var total int64
	err := row.Scan(&total)
	return total, err
}

// ============================================================================
// UNIDADES_ORGANIZATIVAS
// ============================================================================

type UnidadesOrganizativa struct {
	ID            int64
	Codigo        string
	Nombre        string
	UnidadPadreID sql.NullInt64
	Activo        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func scanUnidadOrganizativa(row *sql.Row) (UnidadesOrganizativa, error) {
	var u UnidadesOrganizativa
	err := row.Scan(&u.ID, &u.Codigo, &u.Nombre, &u.UnidadPadreID, &u.Activo, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func nullInt64FromPtr(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func (q *Queries) CreateUnidadOrganizativa(ctx context.Context, codigo, nombre string, unidadPadreID *int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO unidades_organizativas (codigo, nombre, unidad_padre_id, activo, created_at, updated_at)
VALUES (?, ?, ?, 1, NOW(), NOW())`, codigo, nombre, nullInt64FromPtr(unidadPadreID))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) GetUnidadOrganizativaByID(ctx context.Context, id int64) (UnidadesOrganizativa, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, codigo, nombre, unidad_padre_id, activo, created_at, updated_at
FROM unidades_organizativas
WHERE id = ?
LIMIT 1`, id)
	return scanUnidadOrganizativa(row)
}

func (q *Queries) UpdateUnidadOrganizativa(ctx context.Context, id int64, nombre string, unidadPadreID *int64, activo bool) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE unidades_organizativas
SET nombre = ?, unidad_padre_id = ?, activo = ?, updated_at = NOW()
WHERE id = ?`, nombre, nullInt64FromPtr(unidadPadreID), activo, id)
	return err
}

func (q *Queries) ListUnidadesOrganizativas(ctx context.Context, activo *bool, nombre *string, unidadPadreID *int64, limit, offset int32) ([]UnidadesOrganizativa, error) {
	var activoArg, nombreArg, padreArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if nombre != nil {
		nombreArg = *nombre
	}
	if unidadPadreID != nil {
		padreArg = *unidadPadreID
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT id, codigo, nombre, unidad_padre_id, activo, created_at, updated_at
FROM unidades_organizativas
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR nombre LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR unidad_padre_id = ?)
ORDER BY nombre ASC
LIMIT ? OFFSET ?`, activoArg, activoArg, nombreArg, nombreArg, padreArg, padreArg, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UnidadesOrganizativa
	for rows.Next() {
		var u UnidadesOrganizativa
		if err := rows.Scan(&u.ID, &u.Codigo, &u.Nombre, &u.UnidadPadreID, &u.Activo, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (q *Queries) CountUnidadesOrganizativas(ctx context.Context, activo *bool, nombre *string, unidadPadreID *int64) (int64, error) {
	var activoArg, nombreArg, padreArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if nombre != nil {
		nombreArg = *nombre
	}
	if unidadPadreID != nil {
		padreArg = *unidadPadreID
	}
	row := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM unidades_organizativas
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR nombre LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR unidad_padre_id = ?)`, activoArg, activoArg, nombreArg, nombreArg, padreArg, padreArg)
	var total int64
	err := row.Scan(&total)
	return total, err
}

// ============================================================================
// PUESTOS
// ============================================================================

type Puesto struct {
	ID                    int64
	Codigo                string
	Nombre                string
	UnidadOrganizativaID  int64
	ClasificacionPuestoID sql.NullInt64
	Activo                bool
}

func scanPuesto(row *sql.Row) (Puesto, error) {
	var p Puesto
	err := row.Scan(&p.ID, &p.Codigo, &p.Nombre, &p.UnidadOrganizativaID, &p.ClasificacionPuestoID, &p.Activo)
	return p, err
}

func (q *Queries) CreatePuesto(ctx context.Context, codigo, nombre string, clasificacionPuestoID, unidadOrganizativaID int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO puestos (codigo, nombre, unidad_organizativa_id, clasificacion_puesto_id, activo)
VALUES (?, ?, ?, ?, 1)`, codigo, nombre, unidadOrganizativaID, clasificacionPuestoID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) GetPuestoByID(ctx context.Context, id int64) (Puesto, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, codigo, nombre, unidad_organizativa_id, clasificacion_puesto_id, activo
FROM puestos
WHERE id = ?
LIMIT 1`, id)
	return scanPuesto(row)
}

func (q *Queries) UpdatePuesto(ctx context.Context, id int64, nombre string, clasificacionPuestoID, unidadOrganizativaID int64, activo bool) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE puestos
SET nombre = ?, unidad_organizativa_id = ?, clasificacion_puesto_id = ?, activo = ?
WHERE id = ?`, nombre, unidadOrganizativaID, clasificacionPuestoID, activo, id)
	return err
}

func (q *Queries) ListPuestos(ctx context.Context, activo *bool, nombre *string, unidadOrganizativaID, clasificacionPuestoID *int64, limit, offset int32) ([]Puesto, error) {
	var activoArg, nombreArg, unidadArg, clasifArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if nombre != nil {
		nombreArg = *nombre
	}
	if unidadOrganizativaID != nil {
		unidadArg = *unidadOrganizativaID
	}
	if clasificacionPuestoID != nil {
		clasifArg = *clasificacionPuestoID
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT id, codigo, nombre, unidad_organizativa_id, clasificacion_puesto_id, activo
FROM puestos
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR nombre LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR unidad_organizativa_id = ?)
  AND (? IS NULL OR clasificacion_puesto_id = ?)
ORDER BY nombre ASC
LIMIT ? OFFSET ?`, activoArg, activoArg, nombreArg, nombreArg, unidadArg, unidadArg, clasifArg, clasifArg, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Puesto
	for rows.Next() {
		var p Puesto
		if err := rows.Scan(&p.ID, &p.Codigo, &p.Nombre, &p.UnidadOrganizativaID, &p.ClasificacionPuestoID, &p.Activo); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (q *Queries) CountPuestos(ctx context.Context, activo *bool, nombre *string, unidadOrganizativaID, clasificacionPuestoID *int64) (int64, error) {
	var activoArg, nombreArg, unidadArg, clasifArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if nombre != nil {
		nombreArg = *nombre
	}
	if unidadOrganizativaID != nil {
		unidadArg = *unidadOrganizativaID
	}
	if clasificacionPuestoID != nil {
		clasifArg = *clasificacionPuestoID
	}
	row := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM puestos
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR nombre LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR unidad_organizativa_id = ?)
  AND (? IS NULL OR clasificacion_puesto_id = ?)`, activoArg, activoArg, nombreArg, nombreArg, unidadArg, unidadArg, clasifArg, clasifArg)
	var total int64
	err := row.Scan(&total)
	return total, err
}