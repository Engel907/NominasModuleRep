package dto

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// ErrLiquidacionNoDisponible: la liquidación ya la tomó otro período.
var ErrLiquidacionNoDisponible = errors.New("dto: la liquidación ya está ligada a otro período")

// ListarEmpleadosConLiquidacionParaPeriodo: empleados con liquidaciones
// 'procesada' con salida hasta `hasta` que no tiene otro período (libres o ya
// ligadas a este). Ordenados por id.
func (q *Queries) ListarEmpleadosConLiquidacionParaPeriodo(ctx context.Context, periodoID int64, hasta time.Time) ([]int64, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT DISTINCT empleado_id
FROM liquidaciones
WHERE estado = 'procesada'
  AND fecha_salida <= ?
  AND (periodo_planilla_id IS NULL OR periodo_planilla_id = ?)
ORDER BY empleado_id`, hasta, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// LiquidacionParaPeriodoRow: montos por rubro de una liquidación procesada.
type LiquidacionParaPeriodoRow struct {
	ID                  int64
	FechaSalida         time.Time
	MontoPreaviso       decimal.Decimal
	MontoCesantia       decimal.Decimal
	MontoVacaciones     decimal.Decimal
	MontoAguinaldo      decimal.Decimal
	MontoSalarioEscolar decimal.Decimal
}

// ListarLiquidacionesParaPeriodo: liquidaciones del empleado que paga el
// período (mismo filtro que ListarEmpleadosConLiquidacionParaPeriodo).
func (q *Queries) ListarLiquidacionesParaPeriodo(ctx context.Context, periodoID, empleadoID int64, hasta time.Time) ([]LiquidacionParaPeriodoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, fecha_salida, monto_preaviso, monto_cesantia, monto_vacaciones_proporcionales,
       monto_aguinaldo_proporcional, monto_salario_escolar_proporcional
FROM liquidaciones
WHERE empleado_id = ?
  AND estado = 'procesada'
  AND fecha_salida <= ?
  AND (periodo_planilla_id IS NULL OR periodo_planilla_id = ?)
ORDER BY id`, empleadoID, hasta, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LiquidacionParaPeriodoRow
	for rows.Next() {
		var r LiquidacionParaPeriodoRow
		if err := rows.Scan(&r.ID, &r.FechaSalida, &r.MontoPreaviso, &r.MontoCesantia, &r.MontoVacaciones,
			&r.MontoAguinaldo, &r.MontoSalarioEscolar); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LigarLiquidacionPeriodo liga la liquidación al período que la paga.
// Idempotente para el mismo período; ligada a otro → ErrLiquidacionNoDisponible.
func (q *Queries) LigarLiquidacionPeriodo(ctx context.Context, liquidacionID, periodoID int64) error {
	res, err := q.db.ExecContext(ctx, `
UPDATE liquidaciones SET periodo_planilla_id = ?
WHERE id = ? AND estado = 'procesada' AND periodo_planilla_id IS NULL`, periodoID, liquidacionID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 1 {
		return err
	}
	var actual sql.NullInt64
	err = q.db.QueryRowContext(ctx, `SELECT periodo_planilla_id FROM liquidaciones WHERE id = ? FOR UPDATE`, liquidacionID).Scan(&actual)
	if err != nil {
		return err
	}
	if actual.Valid && actual.Int64 == periodoID {
		return nil
	}
	return ErrLiquidacionNoDisponible
}

// DesligarLiquidacionesPeriodo: descartar/recalcular libera las liquidaciones.
func (q *Queries) DesligarLiquidacionesPeriodo(ctx context.Context, periodoID int64) error {
	_, err := q.db.ExecContext(ctx, `UPDATE liquidaciones SET periodo_planilla_id = NULL WHERE periodo_planilla_id = ?`, periodoID)
	return err
}

// ExisteLiquidacionProcesadaDesde: el empleado ya tiene una liquidación
// procesada con salida >= desde (su fecha de ingreso vigente).
func (q *Queries) ExisteLiquidacionProcesadaDesde(ctx context.Context, empleadoID int64, desde time.Time) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM liquidaciones
WHERE empleado_id = ? AND estado = 'procesada' AND fecha_salida >= ?`, empleadoID, desde).Scan(&n)
	return n > 0, err
}

// LiquidacionPorCerrarRow: salida programada cuyo cierre está pendiente.
type LiquidacionPorCerrarRow struct {
	ID          int64
	EmpleadoID  int64
	FechaSalida time.Time
}

// ListarLiquidacionesPorCerrar: procesadas sin cierre con salida <= hasta.
func (q *Queries) ListarLiquidacionesPorCerrar(ctx context.Context, hasta time.Time) ([]LiquidacionPorCerrarRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, empleado_id, fecha_salida FROM liquidaciones
WHERE estado = 'procesada' AND cierre_aplicado_en IS NULL AND fecha_salida <= ?
ORDER BY fecha_salida, id`, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LiquidacionPorCerrarRow
	for rows.Next() {
		var r LiquidacionPorCerrarRow
		if err := rows.Scan(&r.ID, &r.EmpleadoID, &r.FechaSalida); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarcarCierreLiquidacion: registra el cierre. Devuelve false si otro proceso
// ya lo registró (idempotente).
func (q *Queries) MarcarCierreLiquidacion(ctx context.Context, liquidacionID int64) (bool, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE liquidaciones SET cierre_aplicado_en = NOW()
WHERE id = ? AND cierre_aplicado_en IS NULL`, liquidacionID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// ObtenerLiquidacionPorCerrarParaActualizar bloquea la fila y dice si el
// cierre sigue pendiente.
func (q *Queries) ObtenerLiquidacionPorCerrarParaActualizar(ctx context.Context, liquidacionID int64) (bool, error) {
	var cierre sql.NullTime
	err := q.db.QueryRowContext(ctx, `SELECT cierre_aplicado_en FROM liquidaciones WHERE id = ? FOR UPDATE`, liquidacionID).Scan(&cierre)
	return !cierre.Valid, err
}

// PeriodoPorTipoFechasRow: período existente de un tipo con esas fechas.
type PeriodoPorTipoFechasRow struct {
	ID     int64
	Estado string
}

// ObtenerPeriodoPorTipoFechas (uk_pp_tipo_fechas). sql.ErrNoRows si no existe.
func (q *Queries) ObtenerPeriodoPorTipoFechas(ctx context.Context, tipoID int64, inicio, fin time.Time) (PeriodoPorTipoFechasRow, error) {
	var r PeriodoPorTipoFechasRow
	err := q.db.QueryRowContext(ctx, `
SELECT id, estado FROM periodos_planilla
WHERE tipo_planilla_id = ? AND fecha_inicio = ? AND fecha_fin = ?
FOR UPDATE`, tipoID, inicio, fin).Scan(&r.ID, &r.Estado)
	return r, err
}
