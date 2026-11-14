package dto

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type DiaAsistenciaRow struct {
	Fecha time.Time
	Horas decimal.Decimal
}

// ListarAsistenciaPresenteEnRango: días con asistencia 'presente' y horas
// ordinarias > 0 del empleado en [desde, hasta] (base de los salarios diario
// y por hora pagados por asistencia).
func (q *Queries) ListarAsistenciaPresenteEnRango(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]DiaAsistenciaRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT fecha, horas_ordinarias
FROM registro_asistencia_detalle
WHERE empleado_id = ? AND fecha >= ? AND fecha <= ?
  AND estado = 'presente' AND horas_ordinarias > 0
ORDER BY fecha`, empleadoID, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DiaAsistenciaRow
	for rows.Next() {
		var r DiaAsistenciaRow
		if err := rows.Scan(&r.Fecha, &r.Horas); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListarFeriadosPagoObligatorioEnRango: fechas de feriados de pago
// obligatorio del calendario laboral en [desde, hasta].
func (q *Queries) ListarFeriadosPagoObligatorioEnRango(ctx context.Context, desde, hasta time.Time) ([]time.Time, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT c.fecha
FROM calendario_laboral_dia c
JOIN feriados f ON f.id = c.feriado_id
WHERE c.es_feriado = 1 AND f.pago_obligatorio = 1
  AND c.fecha >= ? AND c.fecha <= ?
ORDER BY c.fecha`, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var f time.Time
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// EmbargoMesRow: lo que un embargo ya pidió y retuvo en el mes.
type EmbargoMesRow struct {
	EmbargoID int64
	Calculado decimal.Decimal
	Aplicado  decimal.Decimal
}

// EmbargosMesRow: líquido embargable y embargos ya aplicados al empleado en
// los períodos aprobados, pagados o contabilizados con fecha_fin en
// [desde, hasta], salvo excluirPeriodoID.
type EmbargosMesRow struct {
	BaseEmbargo decimal.Decimal
	PorEmbargo  []EmbargoMesRow
}

func (q *Queries) SumarEmbargosMesEmpleado(ctx context.Context, empleadoID, excluirPeriodoID int64, desde, hasta time.Time) (EmbargosMesRow, error) {
	var out EmbargosMesRow
	err := q.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(pde.base_embargo), 0)
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
WHERE pde.empleado_id = ?
  AND pp.id <> ?
  AND pp.estado IN ('aprobada', 'pagada', 'contabilizada')
  AND pp.fecha_fin >= ?
  AND pp.fecha_fin <= ?`, empleadoID, excluirPeriodoID, desde, hasta).Scan(&out.BaseEmbargo)
	if err != nil {
		return out, err
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT pdemb.embargo_id, SUM(pdemb.monto_calculado), SUM(pdemb.monto)
FROM planilla_detalle_embargo pdemb
JOIN planilla_detalle_empleado pde ON pde.id = pdemb.planilla_detalle_empleado_id
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
WHERE pde.empleado_id = ?
  AND pp.id <> ?
  AND pp.estado IN ('aprobada', 'pagada', 'contabilizada')
  AND pp.fecha_fin >= ?
  AND pp.fecha_fin <= ?
GROUP BY pdemb.embargo_id
ORDER BY pdemb.embargo_id`, empleadoID, excluirPeriodoID, desde, hasta)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var r EmbargoMesRow
		if err := rows.Scan(&r.EmbargoID, &r.Calculado, &r.Aplicado); err != nil {
			return out, err
		}
		out.PorEmbargo = append(out.PorEmbargo, r)
	}
	return out, rows.Err()
}
