package infra

import (
	"context"
	"time"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
)

type parametrosQueries interface {
	ListarCargasSocialesEnFecha(context.Context, time.Time) ([]dto.CargaSocialVigenteRow, error)
	ListarTramosRentaEnFecha(context.Context, time.Time) ([]dto.TramoRentaVigenteRow, error)
	ListarCreditosFiscalesEnFecha(context.Context, time.Time) ([]dto.CreditoFiscalVigenteRow, error)
	ListarParametrosHorasExtraEnFecha(context.Context, time.Time) ([]dto.ParametroHoraExtraRow, error)
	ListarTramosSubsidioIncapacidadEnFecha(context.Context, time.Time) ([]dto.TramoSubsidioIncapacidadRow, error)
	ListarEscalaEmbargoEnFecha(context.Context, time.Time) ([]dto.TramoEscalaEmbargoVigenteRow, error)
	ListarBaseMinimaContributivaEnFecha(context.Context, time.Time) ([]dto.BaseMinimaContributivaVigenteRow, error)
}

var _ parametrosQueries = (*dto.Queries)(nil)

type fuenteParametros struct{ q parametrosQueries }

func NewFuenteParametros(q parametrosQueries) planilla.FuenteParametros {
	return &fuenteParametros{q: q}
}

func (f *fuenteParametros) ListarCargasSocialesEnFecha(ctx context.Context, fecha time.Time) ([]dto.CargaSocialVigenteRow, error) {
	return f.q.ListarCargasSocialesEnFecha(ctx, fecha)
}

func (f *fuenteParametros) ListarTramosRentaEnFecha(ctx context.Context, fecha time.Time) ([]dto.TramoRentaVigenteRow, error) {
	return f.q.ListarTramosRentaEnFecha(ctx, fecha)
}

func (f *fuenteParametros) ListarCreditosFiscalesEnFecha(ctx context.Context, fecha time.Time) ([]dto.CreditoFiscalVigenteRow, error) {
	return f.q.ListarCreditosFiscalesEnFecha(ctx, fecha)
}

func (f *fuenteParametros) ListarParametrosHorasExtraEnFecha(ctx context.Context, fecha time.Time) ([]dto.ParametroHoraExtraRow, error) {
	return f.q.ListarParametrosHorasExtraEnFecha(ctx, fecha)
}

func (f *fuenteParametros) ListarTramosSubsidioIncapacidadEnFecha(ctx context.Context, fecha time.Time) ([]dto.TramoSubsidioIncapacidadRow, error) {
	return f.q.ListarTramosSubsidioIncapacidadEnFecha(ctx, fecha)
}

func (f *fuenteParametros) ListarEscalaEmbargoEnFecha(ctx context.Context, fecha time.Time) ([]planilla.TramoEscalaEmbargo, error) {
	rows, err := f.q.ListarEscalaEmbargoEnFecha(ctx, fecha)
	if err != nil {
		return nil, err
	}
	out := make([]planilla.TramoEscalaEmbargo, 0, len(rows))
	for i, row := range rows {
		out = append(out, planilla.TramoEscalaEmbargo{
			OrdenTramo:   int32(i + 1),
			SalarioDesde: row.DesdeMultiplo,
			SalarioHasta: row.HastaMultiplo,
			Porcentaje:   row.PorcentajeMaximo,
		})
	}
	return out, nil
}

func (f *fuenteParametros) ListarBaseMinimaContributivaEnFecha(ctx context.Context, fecha time.Time) ([]planilla.BaseMinimaContributivaFila, error) {
	rows, err := f.q.ListarBaseMinimaContributivaEnFecha(ctx, fecha)
	if err != nil {
		return nil, err
	}
	out := make([]planilla.BaseMinimaContributivaFila, 0, len(rows))
	for _, row := range rows {
		out = append(out, planilla.BaseMinimaContributivaFila{
			RegimenCCSSID: row.RegimenCCSSID,
			Monto:         row.Monto,
		})
	}
	return out, nil
}
