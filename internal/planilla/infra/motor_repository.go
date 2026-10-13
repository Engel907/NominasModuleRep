package infra

import (
	"context"
	"time"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
)

type motorQueries interface {
	ObtenerContratoVigenteEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (dto.ContratoVigenteEnFechaRow, error)
	ListarConceptosAplicablesEnFecha(ctx context.Context, regimenLaboralID int64, fecha time.Time) ([]dto.ConceptoAplicableRow, error)
	ListarParametrosHorasExtraEnFecha(ctx context.Context, fecha time.Time) ([]dto.ParametroHoraExtraRow, error)
}

var _ motorQueries = (*dto.Queries)(nil)

type motorRepository struct{ q motorQueries }

// NewMotorRepository: planilla.NewMotorRepository(dto.New(sqlDB)).
func NewMotorRepository(q motorQueries) planilla.MotorRepository { return &motorRepository{q: q} }

func (r *motorRepository) ObtenerContratoVigenteEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (planilla.ContratoVigente, error) {
	x, err := r.q.ObtenerContratoVigenteEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return planilla.ContratoVigente{}, err
	}
	return planilla.ContratoVigente(x), nil
}

func (r *motorRepository) ListarConceptosAplicablesEnFecha(ctx context.Context, regimenLaboralID int64, fecha time.Time) ([]planilla.ConceptoAplicableFila, error) {
	rows, err := r.q.ListarConceptosAplicablesEnFecha(ctx, regimenLaboralID, fecha)
	if err != nil {
		return nil, err
	}
	out := make([]planilla.ConceptoAplicableFila, 0, len(rows))
	for _, x := range rows {
		f := planilla.ConceptoAplicableFila{
			ConceptoOrdenCalculo: planilla.ConceptoOrdenCalculo{
				ConceptoSalarialID:   x.ConceptoSalarialID,
				Codigo:               x.Codigo,
				OrdenCalculo:         x.OrdenCalculo,
				AfectaBaseCCSS:       x.AfectaBaseCCSS,
				AfectaBaseRenta:      x.AfectaBaseRenta,
				AfectaBaseAguinaldo:  x.AfectaBaseAguinaldo,
				AfectaBaseCesantia:   x.AfectaBaseCesantia,
				AfectaBaseVacaciones: x.AfectaBaseVacaciones,
			},
			Naturaleza:       x.Naturaleza,
			TipoCalculo:      x.TipoCalculo,
			FormulaID:        x.FormulaID,
			ExpresionFormula: x.ExpresionFormula,
		}
		if x.MontoFijo.Valid {
			m := x.MontoFijo.Decimal
			f.MontoFijo = &m
		}
		out = append(out, f)
	}
	return out, nil
}

func (r *motorRepository) ListarParametrosHorasExtraEnFecha(ctx context.Context, fecha time.Time) ([]planilla.ParametroHoraExtra, error) {
	rows, err := r.q.ListarParametrosHorasExtraEnFecha(ctx, fecha)
	if err != nil {
		return nil, err
	}
	out := make([]planilla.ParametroHoraExtra, 0, len(rows))
	for _, x := range rows {
		out = append(out, planilla.ParametroHoraExtra{TipoHoraExtraID: x.TipoHoraExtraID, FactorRecargo: x.FactorRecargo})
	}
	return out, nil
}
