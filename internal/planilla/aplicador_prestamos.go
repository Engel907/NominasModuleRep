package planilla

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"planillas-ccdr/internal/db/dto"
)

// ErrPrestamoNoAplicable: el préstamo cambió (ya no está activo o su saldo es
// menor que lo retenido) entre el cálculo y la aprobación: hay que recalcular.
var ErrPrestamoNoAplicable = &ErrorCiclo{"prestamo_no_aplicable", "un préstamo del período cambió después del cálculo; recalcule antes de aprobar", 409}

type aplicadorPrestamos struct{}

// NewAplicadorPrestamos: gancho AlAprobar que aplica las cuotas retenidas al
// saldo de los préstamos.
func NewAplicadorPrestamos() GanchoAlAprobar { return aplicadorPrestamos{} }

// AlAprobar: por cada planilla_detalle_prestamo del período con
// aplicado_en IS NULL → préstamo FOR UPDATE, saldo -= monto_retenido (error si
// quedaría < 0), aplicado_en = NOW. Idempotente: una cuota ya aplicada no se
// vuelve a descontar. trg_prestamos_saldado_bu pasa a 'pagado' el préstamo
// que llega a saldo 0.
func (aplicadorPrestamos) AlAprobar(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow) error {
	pendientes, err := q.ListarPrestamosPorAplicarPeriodo(ctx, periodo.ID)
	if err != nil {
		return fmt.Errorf("planilla: leyendo préstamos por aplicar del período %d: %w", periodo.ID, err)
	}
	for _, x := range pendientes {
		if x.MontoRetenido.IsPositive() {
			p, err := q.GetPrestamoParaActualizar(ctx, x.PrestamoID)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPrestamoNoAplicable
			}
			if err != nil {
				return fmt.Errorf("planilla: bloqueando préstamo %d: %w", x.PrestamoID, err)
			}
			if p.Estado != "activo" || x.MontoRetenido.GreaterThan(p.SaldoPendiente) {
				return &ErrorCiclo{Codigo: ErrPrestamoNoAplicable.Codigo, HTTP: ErrPrestamoNoAplicable.HTTP,
					Mensaje: fmt.Sprintf("el préstamo %d está %s con saldo %s y el período retiene %s; recalcule antes de aprobar",
						x.PrestamoID, p.Estado, p.SaldoPendiente.StringFixed(dosDecimales), x.MontoRetenido.StringFixed(dosDecimales))}
			}
			if err := q.AplicarAbonoPrestamo(ctx, dto.AplicarAbonoPrestamoParams{SaldoPendiente: x.MontoRetenido, ID: x.PrestamoID}); err != nil {
				return fmt.Errorf("planilla: descontando saldo del préstamo %d: %w", x.PrestamoID, err)
			}
		}
		if err := q.MarcarPrestamoAplicado(ctx, x.DetallePrestamoID); err != nil {
			return fmt.Errorf("planilla: marcando aplicado el detalle de préstamo %d: %w", x.DetallePrestamoID, err)
		}
	}
	return nil
}
