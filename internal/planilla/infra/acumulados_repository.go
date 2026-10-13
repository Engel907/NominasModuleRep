package infra

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
)

type acumuladosQueries interface {
	ListarEmpleadosActivosEnRango(ctx context.Context, desde, hasta time.Time) ([]int64, error)
	SumarBaseAguinaldoPorEmpleado(ctx context.Context, desde, hasta time.Time) ([]dto.BaseAcumuladaEmpleadoRow, error)
	SumarBaseSalarioEscolarPorEmpleado(ctx context.Context, desde, hasta time.Time) ([]dto.BaseAcumuladaEmpleadoRow, error)
	ListarEmpleadosConAguinaldoEnAnio(ctx context.Context, anio int32) (map[int64]bool, error)
	ListarEmpleadosConSalarioEscolarEnAnio(ctx context.Context, anio int32) (map[int64]bool, error)
	ObtenerPorcentajeSalarioEscolarEnFecha(ctx context.Context, fecha time.Time) (decimal.Decimal, error)
	CrearAguinaldo(ctx context.Context, arg dto.CrearAguinaldoParams) (int64, error)
	CrearSalarioEscolar(ctx context.Context, arg dto.CrearSalarioEscolarParams) (int64, error)
}

var _ acumuladosQueries = (*dto.Queries)(nil)

type acumuladosRepository struct{ q acumuladosQueries }

func NewAcumuladosRepository(q acumuladosQueries) planilla.AcumuladosRepository {
	return &acumuladosRepository{q: q}
}

func toMap(rows []dto.BaseAcumuladaEmpleadoRow) map[int64]planilla.BaseAcumulada {
	m := make(map[int64]planilla.BaseAcumulada, len(rows))
	for _, r := range rows {
		m[r.EmpleadoID] = planilla.BaseAcumulada{Total: r.TotalBase, PlanillasCount: r.PlanillasCount}
	}
	return m
}

func (r *acumuladosRepository) ListarEmpleadosActivosEnRango(ctx context.Context, d, h time.Time) ([]int64, error) {
	return r.q.ListarEmpleadosActivosEnRango(ctx, d, h)
}
func (r *acumuladosRepository) SumarBaseAguinaldo(ctx context.Context, d, h time.Time) (map[int64]planilla.BaseAcumulada, error) {
	rows, err := r.q.SumarBaseAguinaldoPorEmpleado(ctx, d, h)
	return toMap(rows), err
}
func (r *acumuladosRepository) SumarBaseSalarioEscolar(ctx context.Context, d, h time.Time) (map[int64]planilla.BaseAcumulada, error) {
	rows, err := r.q.SumarBaseSalarioEscolarPorEmpleado(ctx, d, h)
	return toMap(rows), err
}
func (r *acumuladosRepository) EmpleadosConAguinaldo(ctx context.Context, anio int32) (map[int64]bool, error) {
	return r.q.ListarEmpleadosConAguinaldoEnAnio(ctx, anio)
}
func (r *acumuladosRepository) EmpleadosConSalarioEscolar(ctx context.Context, anio int32) (map[int64]bool, error) {
	return r.q.ListarEmpleadosConSalarioEscolarEnAnio(ctx, anio)
}
func (r *acumuladosRepository) PorcentajeSalarioEscolarEnFecha(ctx context.Context, f time.Time) (decimal.Decimal, error) {
	return r.q.ObtenerPorcentajeSalarioEscolarEnFecha(ctx, f)
}
func (r *acumuladosRepository) GuardarAguinaldo(ctx context.Context, x planilla.RegistroAcumulado) error {
	_, err := r.q.CrearAguinaldo(ctx, dto.CrearAguinaldoParams{
		EmpleadoID: x.EmpleadoID, Anio: x.Anio, PeriodoDesde: x.PeriodoDesde, PeriodoHasta: x.PeriodoHasta,
		TotalSalariosBrutos: x.TotalBase, MontoAguinaldo: x.Monto, ProcesadoPor: x.ProcesadoPor,
	})
	return err
}
func (r *acumuladosRepository) GuardarSalarioEscolar(ctx context.Context, x planilla.RegistroAcumulado) error {
	_, err := r.q.CrearSalarioEscolar(ctx, dto.CrearSalarioEscolarParams{
		EmpleadoID: x.EmpleadoID, Anio: x.Anio, PeriodoDesde: x.PeriodoDesde, PeriodoHasta: x.PeriodoHasta,
		PorcentajeAplicado: x.Porcentaje, TotalSalariosBrutos: x.TotalBase, MontoSalarioEscolar: x.Monto,
		ProcesadoPor: x.ProcesadoPor,
	})
	return err
}
