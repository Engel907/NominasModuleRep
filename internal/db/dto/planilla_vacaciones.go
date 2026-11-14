package dto

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type CompensacionVacacionesRow struct {
	MovimientoID int64
	Dias         decimal.Decimal // positivo (el movimiento guarda dias < 0)
}

// ListarCompensacionesVacacionesPendientes: movimientos 'compensacion' del
// empleado sin pagar, con fecha_efectiva <= hasta, libres o ya ligados a este
// período (recalcular).
func (q *Queries) ListarCompensacionesVacacionesPendientes(ctx context.Context, empleadoID, periodoID int64, hasta time.Time) ([]CompensacionVacacionesRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, ABS(dias)
FROM vacaciones_movimientos
WHERE empleado_id = ?
  AND tipo_movimiento = 'compensacion'
  AND pagado_en IS NULL
  AND (periodo_planilla_id IS NULL OR periodo_planilla_id = ?)
  AND fecha_efectiva <= ?
ORDER BY fecha_efectiva ASC, id ASC`, empleadoID, periodoID, hasta)
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

type VacacionesAprobadasRow struct {
	SolicitudID int64
	FechaInicio time.Time
	FechaFin    time.Time
}

// ListarVacacionesAprobadasEnRango: solicitudes aprobadas del empleado que se
// solapan con [desde, hasta].
func (q *Queries) ListarVacacionesAprobadasEnRango(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]VacacionesAprobadasRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, fecha_inicio, fecha_fin
FROM vacaciones_solicitudes
WHERE empleado_id = ?
  AND estado = 'aprobada'
  AND fecha_inicio <= ?
  AND fecha_fin >= ?
ORDER BY fecha_inicio ASC, id ASC`, empleadoID, hasta, desde)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VacacionesAprobadasRow
	for rows.Next() {
		var r VacacionesAprobadasRow
		if err := rows.Scan(&r.SolicitudID, &r.FechaInicio, &r.FechaFin); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ErrCompensacionNoDisponible: la compensación ya está pagada o ligada a otro período.
var ErrCompensacionNoDisponible = errors.New("dto: la compensación de vacaciones ya no está disponible para este período")

// LigarCompensacionVacaciones liga el movimiento al período que lo paga.
func (q *Queries) LigarCompensacionVacaciones(ctx context.Context, movimientoID, periodoID int64) error {
	res, err := q.db.ExecContext(ctx, `
UPDATE vacaciones_movimientos SET periodo_planilla_id = ?
WHERE id = ? AND tipo_movimiento = 'compensacion' AND pagado_en IS NULL
  AND (periodo_planilla_id IS NULL OR periodo_planilla_id = ?)`, periodoID, movimientoID, periodoID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		var ok int
		if err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM vacaciones_movimientos WHERE id = ? AND periodo_planilla_id = ? AND pagado_en IS NULL`, movimientoID, periodoID).Scan(&ok); err != nil {
			return err
		}
		if ok == 0 {
			return ErrCompensacionNoDisponible
		}
	}
	return nil
}

// DesligarCompensacionesVacacionesPeriodo: al descartar/recalcular, las
// compensaciones no pagadas del período vuelven a quedar libres.
func (q *Queries) DesligarCompensacionesVacacionesPeriodo(ctx context.Context, periodoID int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE vacaciones_movimientos SET periodo_planilla_id = NULL
WHERE periodo_planilla_id = ? AND pagado_en IS NULL`, periodoID)
	return err
}

// MarcarCompensacionesVacacionesPagadas: al aprobar (idempotente).
func (q *Queries) MarcarCompensacionesVacacionesPagadas(ctx context.Context, periodoID int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE vacaciones_movimientos SET pagado_en = NOW()
WHERE periodo_planilla_id = ? AND pagado_en IS NULL`, periodoID)
	return err
}

type EmpleadoIngresoRow struct {
	EmpleadoID   int64
	FechaIngreso time.Time
}

// ListarEmpleadosActivosIngreso: empleados activos con su fecha de ingreso
// (job acumular-vacaciones).
func (q *Queries) ListarEmpleadosActivosIngreso(ctx context.Context) ([]EmpleadoIngresoRow, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT id, fecha_ingreso FROM empleados WHERE activo = 1 ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EmpleadoIngresoRow
	for rows.Next() {
		var r EmpleadoIngresoRow
		if err := rows.Scan(&r.EmpleadoID, &r.FechaIngreso); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
