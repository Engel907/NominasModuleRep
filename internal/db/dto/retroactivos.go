package dto

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// RetroactivoRow: retroactivo registrado para un período de pago.
type RetroactivoRow struct {
	ID                int64
	PeriodoPlanillaID int64
	EmpleadoID        int64
	Desde, Hasta      time.Time
	Motivo            string
	CreadoPor         int64
	CreatedAt         time.Time
}

const columnasRetroactivo = `id, periodo_planilla_id, empleado_id, desde, hasta, motivo, creado_por, created_at`

func scanRetroactivo(s interface{ Scan(...any) error }) (RetroactivoRow, error) {
	var r RetroactivoRow
	err := s.Scan(&r.ID, &r.PeriodoPlanillaID, &r.EmpleadoID, &r.Desde, &r.Hasta, &r.Motivo, &r.CreadoPor, &r.CreatedAt)
	return r, err
}

type InsertarRetroactivoParams struct {
	PeriodoPlanillaID int64
	EmpleadoID        int64
	Desde, Hasta      time.Time
	Motivo            string
	CreadoPor         int64
}

func (q *Queries) InsertarRetroactivo(ctx context.Context, p InsertarRetroactivoParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO retroactivos (periodo_planilla_id, empleado_id, desde, hasta, motivo, creado_por)
VALUES (?, ?, ?, ?, ?, ?)`, p.PeriodoPlanillaID, p.EmpleadoID, p.Desde, p.Hasta, p.Motivo, p.CreadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ObtenerRetroactivo: sql.ErrNoRows si no existe en el período.
func (q *Queries) ObtenerRetroactivo(ctx context.Context, periodoID, id int64) (RetroactivoRow, error) {
	return scanRetroactivo(q.db.QueryRowContext(ctx,
		`SELECT `+columnasRetroactivo+` FROM retroactivos WHERE periodo_planilla_id = ? AND id = ?`, periodoID, id))
}

func (q *Queries) ContarRetroactivos(ctx context.Context, periodoID int64, empleadoID *int64) (int64, error) {
	var n int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM retroactivos
WHERE periodo_planilla_id = ? AND (? IS NULL OR empleado_id = ?)`, periodoID, empleadoID, empleadoID).Scan(&n)
	return n, err
}

// ListarRetroactivos: retroactivos del período (opcionalmente de un
// empleado), por empleado. limit <= 0 = todos.
func (q *Queries) ListarRetroactivos(ctx context.Context, periodoID int64, empleadoID *int64, limit, offset int32) ([]RetroactivoRow, error) {
	if limit <= 0 {
		limit, offset = 1<<30, 0
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasRetroactivo+` FROM retroactivos
WHERE periodo_planilla_id = ? AND (? IS NULL OR empleado_id = ?)
ORDER BY empleado_id, id
LIMIT ? OFFSET ?`, periodoID, empleadoID, empleadoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RetroactivoRow
	for rows.Next() {
		r, err := scanRetroactivo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) EliminarRetroactivo(ctx context.Context, periodoID, id int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `DELETE FROM retroactivos WHERE periodo_planilla_id = ? AND id = ?`, periodoID, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PeriodoOrigenRetroactivoRow: período regular aprobado que se recalcula.
type PeriodoOrigenRetroactivoRow struct {
	ID             int64
	TipoPlanillaID int64
	Codigo         string // tipos_planilla.codigo
	FechaInicio    time.Time
	FechaFin       time.Time
}

// ListarPeriodosOrigenRetroactivo: períodos regulares
// aprobada|pagada|contabilizada con fecha_fin en [desde, hasta] en los que el
// empleado tiene detalle, salvo excluirPeriodoID, por fecha.
func (q *Queries) ListarPeriodosOrigenRetroactivo(ctx context.Context, empleadoID int64, desde, hasta time.Time, excluirPeriodoID int64) ([]PeriodoOrigenRetroactivoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT pp.id, pp.tipo_planilla_id, tp.codigo, pp.fecha_inicio, pp.fecha_fin
FROM periodos_planilla pp
JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
JOIN planilla_detalle_empleado pde ON pde.periodo_planilla_id = pp.id AND pde.empleado_id = ?
WHERE tp.clase = 'regular'
  AND pp.estado IN ('aprobada', 'pagada', 'contabilizada')
  AND pp.fecha_fin >= ? AND pp.fecha_fin <= ?
  AND pp.id <> ?
ORDER BY pp.fecha_fin, pp.id`, empleadoID, desde, hasta, excluirPeriodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PeriodoOrigenRetroactivoRow
	for rows.Next() {
		var r PeriodoOrigenRetroactivoRow
		if err := rows.Scan(&r.ID, &r.TipoPlanillaID, &r.Codigo, &r.FechaInicio, &r.FechaFin); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// IngresoPagadoRow: lo pagado de un concepto de ingreso por un período.
// LineaID: primera línea propia del período con ese concepto (0 = ninguna).
type IngresoPagadoRow struct {
	ConceptoSalarialID int64
	Monto              decimal.Decimal
	LineaID            int64
}

// SumarIngresosPagadosPeriodo: por concepto de ingreso, lo que el período
// pagó al empleado en sus propias líneas más las diferencias de retroactivos
// de ese período que ya calcularon otros períodos (salvo excluirPeriodoID).
func (q *Queries) SumarIngresosPagadosPeriodo(ctx context.Context, empleadoID, periodoID, excluirPeriodoID int64) ([]IngresoPagadoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT t.concepto_salarial_id, SUM(t.monto), COALESCE(MIN(t.linea_id), 0) FROM (
  SELECT pdc.concepto_salarial_id, pdc.monto, pdc.id AS linea_id
  FROM planilla_detalle_concepto pdc
  JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
  JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
  WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ?
    AND pdc.retroactivo_periodo_origen_id IS NULL
    AND cs.naturaleza = 'ingreso'
  UNION ALL
  SELECT pdc.concepto_salarial_id, pdc.monto, NULL
  FROM planilla_detalle_concepto pdc
  JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
  WHERE pdc.retroactivo_periodo_origen_id = ? AND pde.empleado_id = ?
    AND pde.periodo_planilla_id <> ?
) t
GROUP BY t.concepto_salarial_id
ORDER BY t.concepto_salarial_id`, periodoID, empleadoID, periodoID, empleadoID, excluirPeriodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IngresoPagadoRow
	for rows.Next() {
		var r IngresoPagadoRow
		if err := rows.Scan(&r.ConceptoSalarialID, &r.Monto, &r.LineaID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListarCompensacionesVacacionesPagadasPorPeriodo: compensaciones que pagó
// el período (para recalcularlo tal como se pagó).
func (q *Queries) ListarCompensacionesVacacionesPagadasPorPeriodo(ctx context.Context, empleadoID, periodoID int64) ([]CompensacionVacacionesRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, ABS(dias)
FROM vacaciones_movimientos
WHERE empleado_id = ? AND tipo_movimiento = 'compensacion' AND periodo_planilla_id = ?
ORDER BY fecha_efectiva ASC, id ASC`, empleadoID, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CompensacionVacacionesRow
	for rows.Next() {
		var r CompensacionVacacionesRow
		if err := rows.Scan(&r.MovimientoID, &r.Dias); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PeriodoEsRegular: el período es de un tipo de planilla de clase regular.
func (q *Queries) PeriodoEsRegular(ctx context.Context, periodoID int64) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM periodos_planilla pp JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
WHERE pp.id = ? AND tp.clase = 'regular'`, periodoID).Scan(&n)
	return n > 0, err
}
