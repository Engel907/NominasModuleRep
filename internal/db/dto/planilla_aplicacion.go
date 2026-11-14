package dto

import (
	"context"

	"github.com/shopspring/decimal"
)

type PrestamoPorAplicarRow struct {
	DetallePrestamoID int64
	PrestamoID        int64
	MontoRetenido     decimal.Decimal
}

// ListarPrestamosPorAplicarPeriodo: cuotas retenidas del período que aún no
// se aplicaron al saldo (aplicado_en IS NULL), bloqueadas FOR UPDATE, en
// orden de préstamo para bloquear siempre en el mismo orden.
func (q *Queries) ListarPrestamosPorAplicarPeriodo(ctx context.Context, periodoID int64) ([]PrestamoPorAplicarRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT pdp.id, pdp.prestamo_id, pdp.monto_retenido
FROM planilla_detalle_prestamo pdp
JOIN planilla_detalle_empleado pde ON pde.id = pdp.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ?
  AND pdp.aplicado_en IS NULL
ORDER BY pdp.prestamo_id ASC, pdp.id ASC
FOR UPDATE`, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrestamoPorAplicarRow
	for rows.Next() {
		var r PrestamoPorAplicarRow
		if err := rows.Scan(&r.DetallePrestamoID, &r.PrestamoID, &r.MontoRetenido); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarcarPrestamoAplicado registra que la cuota ya bajó el saldo. El
// `aplicado_en IS NULL` lo hace idempotente (trg_pdpre_inmutable_bu solo
// permite NULL → valor en un período aprobado).
func (q *Queries) MarcarPrestamoAplicado(ctx context.Context, detallePrestamoID int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE planilla_detalle_prestamo SET aplicado_en = NOW()
WHERE id = ? AND aplicado_en IS NULL`, detallePrestamoID)
	return err
}
