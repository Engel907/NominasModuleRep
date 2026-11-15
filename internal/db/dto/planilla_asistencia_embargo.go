package dto

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

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
