package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

// AsignacionVigenteRow: asignación que el motor aplica a la fecha.
type AsignacionVigenteRow struct {
	ID                 int64
	ConceptoSalarialID int64
	Modo               string          // monto | porcentaje
	Valor              decimal.Decimal // porcentaje en 0-100
}

// ListarAsignacionesVigentesEnFecha: asignaciones del empleado vigentes a la fecha.
func (q *Queries) ListarAsignacionesVigentesEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]AsignacionVigenteRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, concepto_salarial_id, modo, valor
FROM empleado_conceptos_asignados
WHERE empleado_id = ?
  AND fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY concepto_salarial_id, id`, empleadoID, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AsignacionVigenteRow
	for rows.Next() {
		var r AsignacionVigenteRow
		if err := rows.Scan(&r.ID, &r.ConceptoSalarialID, &r.Modo, &r.Valor); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AsignacionRow: asignación con los nombres para mostrarla.
type AsignacionRow struct {
	ID                  int64
	EmpleadoID          int64
	ConceptoSalarialID  int64
	ConceptoCodigo      string
	ConceptoNombre      string
	Naturaleza          string
	Modo                string
	Valor               decimal.Decimal
	TerceroID           *int64
	TerceroNombre       *string
	Referencia          *string
	Observacion         *string
	FechaInicioVigencia time.Time
	FechaFinVigencia    *time.Time
	CreadoPor           int64
	CreatedAt           time.Time
}

func scanAsignacion(s rowScanner) (AsignacionRow, error) {
	var r AsignacionRow
	var tercero sql.NullInt64
	var terceroNombre, ref, obs sql.NullString
	var fin sql.NullTime
	if err := s.Scan(&r.ID, &r.EmpleadoID, &r.ConceptoSalarialID, &r.ConceptoCodigo, &r.ConceptoNombre, &r.Naturaleza,
		&r.Modo, &r.Valor, &tercero, &terceroNombre, &ref, &obs, &r.FechaInicioVigencia, &fin, &r.CreadoPor, &r.CreatedAt); err != nil {
		return AsignacionRow{}, err
	}
	if tercero.Valid {
		r.TerceroID = &tercero.Int64
	}
	if terceroNombre.Valid {
		r.TerceroNombre = &terceroNombre.String
	}
	if ref.Valid {
		r.Referencia = &ref.String
	}
	if obs.Valid {
		r.Observacion = &obs.String
	}
	if fin.Valid {
		r.FechaFinVigencia = &fin.Time
	}
	return r, nil
}

// ListarAsignacionesEmpleado: historial completo del empleado, la más reciente primero.
func (q *Queries) ListarAsignacionesEmpleado(ctx context.Context, empleadoID int64) ([]AsignacionRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT a.id, a.empleado_id, a.concepto_salarial_id, cs.codigo, cs.nombre, cs.naturaleza, a.modo, a.valor,
       a.tercero_id, t.nombre, a.referencia, a.observacion, a.fecha_inicio_vigencia, a.fecha_fin_vigencia,
       a.creado_por, a.created_at
FROM empleado_conceptos_asignados a
JOIN conceptos_salariales cs ON cs.id = a.concepto_salarial_id
LEFT JOIN terceros t ON t.id = a.tercero_id
WHERE a.empleado_id = ?
ORDER BY a.fecha_inicio_vigencia DESC, a.id DESC`, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AsignacionRow
	for rows.Next() {
		r, err := scanAsignacion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ObtenerAsignacionParaActualizar lee una asignación bloqueándola.
func (q *Queries) ObtenerAsignacionParaActualizar(ctx context.Context, id int64) (AsignacionRow, error) {
	return scanAsignacion(q.db.QueryRowContext(ctx, `
SELECT a.id, a.empleado_id, a.concepto_salarial_id, cs.codigo, cs.nombre, cs.naturaleza, a.modo, a.valor,
       a.tercero_id, t.nombre, a.referencia, a.observacion, a.fecha_inicio_vigencia, a.fecha_fin_vigencia,
       a.creado_por, a.created_at
FROM empleado_conceptos_asignados a
JOIN conceptos_salariales cs ON cs.id = a.concepto_salarial_id
LEFT JOIN terceros t ON t.id = a.tercero_id
WHERE a.id = ? FOR UPDATE`, id))
}

// ObtenerAsignacionVigenteDeConcepto: la asignación vigente (sin fin) del
// empleado para el concepto; sql.ErrNoRows si no hay.
func (q *Queries) ObtenerAsignacionVigenteDeConcepto(ctx context.Context, empleadoID, conceptoID int64) (AsignacionRow, error) {
	return scanAsignacion(q.db.QueryRowContext(ctx, `
SELECT a.id, a.empleado_id, a.concepto_salarial_id, cs.codigo, cs.nombre, cs.naturaleza, a.modo, a.valor,
       a.tercero_id, t.nombre, a.referencia, a.observacion, a.fecha_inicio_vigencia, a.fecha_fin_vigencia,
       a.creado_por, a.created_at
FROM empleado_conceptos_asignados a
JOIN conceptos_salariales cs ON cs.id = a.concepto_salarial_id
LEFT JOIN terceros t ON t.id = a.tercero_id
WHERE a.empleado_id = ? AND a.concepto_salarial_id = ? AND a.fecha_fin_vigencia IS NULL FOR UPDATE`, empleadoID, conceptoID))
}

// ConceptoAsignable: concepto activo cuya fórmula vigente a la fecha es de
// tipo asignacion_empleado.
func (q *Queries) ConceptoAsignable(ctx context.Context, conceptoID int64, fecha time.Time) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM conceptos_salariales cs
JOIN concepto_salarial_formula f ON f.concepto_salarial_id = cs.id
WHERE cs.id = ? AND cs.activo = 1
  AND JSON_UNQUOTE(JSON_EXTRACT(f.expresion_formula, '$.tipo')) = 'asignacion_empleado'
  AND f.fecha_inicio_vigencia <= ?
  AND (f.fecha_fin_vigencia IS NULL OR f.fecha_fin_vigencia >= ?)`, conceptoID, fecha, fecha).Scan(&n)
	return n > 0, err
}

type InsertarAsignacionParams struct {
	EmpleadoID          int64
	ConceptoSalarialID  int64
	Modo                string
	Valor               decimal.Decimal
	TerceroID           *int64
	Referencia          *string
	Observacion         *string
	FechaInicioVigencia time.Time
	CreadoPor           int64
}

func (q *Queries) InsertarAsignacion(ctx context.Context, a InsertarAsignacionParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleado_conceptos_asignados
  (empleado_id, concepto_salarial_id, modo, valor, tercero_id, referencia, observacion, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.EmpleadoID, a.ConceptoSalarialID, a.Modo, a.Valor, a.TerceroID, a.Referencia, a.Observacion, a.FechaInicioVigencia, a.CreadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CerrarAsignacion fija fecha_fin_vigencia de una asignación vigente.
func (q *Queries) CerrarAsignacion(ctx context.Context, id int64, fin time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`UPDATE empleado_conceptos_asignados SET fecha_fin_vigencia = ? WHERE id = ? AND fecha_fin_vigencia IS NULL`, fin, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
