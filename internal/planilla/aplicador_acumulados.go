package planilla

import (
	"context"
	"fmt"

	"planillas-ccdr/internal/db/dto"
)

type aplicadorAcumulados struct{}

// NewAplicadorAcumulados: gancho AlAprobar que pasa a 'procesado' las
// filas de aguinaldos / salarios_escolares que el período dejó en
// 'proyectado'. Idempotente; en un período regular no toca nada.
func NewAplicadorAcumulados() GanchoAlAprobar { return aplicadorAcumulados{} }

func (aplicadorAcumulados) AlAprobar(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow) error {
	if err := q.ProcesarAcumuladosPeriodo(ctx, periodo.ID, periodo.AprobadoPorUsuarioID); err != nil {
		return fmt.Errorf("planilla: procesando acumulados del período %d: %w", periodo.ID, err)
	}
	return nil
}
