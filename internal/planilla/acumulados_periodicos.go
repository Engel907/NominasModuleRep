package planilla

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
)

// CalcularAguinaldo (pura): base acumulada / AGUINALDO_MESES, un solo
// redondeo a 2 decimales. Los meses siempre llegan como parámetro.
func CalcularAguinaldo(totalBase, meses decimal.Decimal) (decimal.Decimal, error) {
	if totalBase.IsNegative() {
		return decimal.Zero, errors.New("planilla: base de aguinaldo negativa")
	}
	if !meses.IsPositive() {
		return decimal.Zero, fmt.Errorf("planilla: AGUINALDO_MESES inválido (%s)", meses)
	}
	return totalBase.DivRound(meses, dosDecimales), nil
}

// CalcularSalarioEscolar (pura): base acumulada × porcentaje vigente
// (fracción, ej. 0.0833), un solo redondeo a 2 decimales.
func CalcularSalarioEscolar(totalBase, porcentaje decimal.Decimal) (decimal.Decimal, error) {
	if totalBase.IsNegative() {
		return decimal.Zero, errors.New("planilla: base de salario escolar negativa")
	}
	if !porcentaje.IsPositive() || porcentaje.GreaterThan(decimal.NewFromInt(1)) {
		return decimal.Zero, fmt.Errorf("planilla: porcentaje de salario escolar inválido (%s); se espera una fracción en (0, 1]", porcentaje)
	}
	return totalBase.Mul(porcentaje).Round(dosDecimales), nil
}

var (
	ErrFueraDeVentana       = &ErrorCiclo{"fuera_de_ventana", "la fecha actual está fuera de la ventana legal de ejecución", 409}
	ErrAcumuladoYaEjecutado = &ErrorCiclo{"acumulado_ya_ejecutado", "ya existe un período para esta ventana de cálculo", 409}
	ErrAcumuladoYaProcesado = &ErrorCiclo{"acumulado_ya_procesado", "un empleado ya tiene el acumulado de este año procesado por otro período", 409}
	ErrClaseNoAcumulable    = errors.New("planilla: la clase no es aguinaldo ni salario_escolar")
)

// TipoPorClaseLector: lectura del único tipo de una clase no regular.
type TipoPorClaseLector interface {
	ObtenerTipoPlanillaPorClase(ctx context.Context, clase string) (dto.TipoPlanillaRow, error)
}

// EjecutorAcumulados crea los períodos de aguinaldo y salario escolar.
type EjecutorAcumulados struct {
	periodos *PeriodoService
	tipos    TipoPorClaseLector
	ventanas *Ventanas
	ahora    func() time.Time
}

func NewEjecutorAcumulados(periodos *PeriodoService, tipos TipoPorClaseLector, ventanas *Ventanas) *EjecutorAcumulados {
	return &EjecutorAcumulados{periodos: periodos, tipos: tipos, ventanas: ventanas, ahora: time.Now}
}

// Ejecutar valida la ventana de ejecución (fuera → 409, sin opción de
// forzar) y crea el período de la clase con las fechas de la ventana de
// cálculo. Una segunda ejecución del mismo año → 409.
func (e *EjecutorAcumulados) Ejecutar(ctx context.Context, usuarioID int64, clase ClaseTipoPlanilla, anio int) (dto.PeriodoCicloRow, error) {
	hoy := soloFecha(e.ahora().UTC())
	v, err := e.ventanas.Ventana(ctx, clase, anio, hoy)
	if err != nil {
		return dto.PeriodoCicloRow{}, err
	}
	if !v.EnEjecucion(hoy) {
		return dto.PeriodoCicloRow{}, &ErrorCiclo{Codigo: ErrFueraDeVentana.Codigo, HTTP: ErrFueraDeVentana.HTTP,
			Mensaje: fmt.Sprintf("%s %d se ejecuta del %s al %s (hoy es %s)", clase, anio,
				v.EjecucionDesde.Format("2006-01-02"), v.EjecucionHasta.Format("2006-01-02"), hoy.Format("2006-01-02"))}
	}
	tipo, err := e.tipos.ObtenerTipoPlanillaPorClase(ctx, string(clase))
	if errors.Is(err, sql.ErrNoRows) {
		return dto.PeriodoCicloRow{}, ErrTipoPlanillaNoEncontrado
	}
	if err != nil {
		return dto.PeriodoCicloRow{}, fmt.Errorf("planilla: tipo de planilla de clase %s: %w", clase, err)
	}
	p, err := e.periodos.Crear(ctx, usuarioID, tipo.ID, v.CalculoDesde, v.CalculoHasta)
	if errors.Is(err, ErrPeriodoTraslapado) {
		return dto.PeriodoCicloRow{}, ErrAcumuladoYaEjecutado
	}
	return p, err
}

// errorParametrosVentana traduce un parámetro de ventana faltante a 422.
func errorParametrosVentana(err error) error {
	var nc parametros.ErrParametroNoConfigurado
	if errors.As(err, &nc) {
		return &ErrorCiclo{Codigo: ErrParametrosIncompletos.Codigo, Mensaje: err.Error(), HTTP: ErrParametrosIncompletos.HTTP}
	}
	return err
}

// soloFecha descarta la hora y deja solo la fecha en UTC.
func soloFecha(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
