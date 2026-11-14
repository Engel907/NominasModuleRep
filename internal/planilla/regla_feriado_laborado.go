package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// Claves de parametros_generales del feriado laborado (decimal, veces la
// hora ordinaria que se agrega por hora ordinaria trabajada en el feriado).
const (
	ClaveFeriadoFactorObligatorio   = "FERIADO_LABORADO_FACTOR_OBLIGATORIO"
	ClaveFeriadoFactorNoObligatorio = "FERIADO_LABORADO_FACTOR_NO_OBLIGATORIO"
)

// FeriadoLaboradoInput: horas ordinarias trabajadas en feriados del período,
// agrupadas por si el feriado es de pago obligatorio.
type FeriadoLaboradoInput struct {
	PagoObligatorio bool
	Horas           decimal.Decimal
}

// reglaFeriadoLaborado (feriado_laborado, CT arts. 148-149): el salario ya
// remunera el feriado (el mensual y el diario por días comerciales lo
// incluyen; los pagados por asistencia lo cuentan como día trabajado o, si es
// de pago obligatorio y no se trabajó, como día pagado); trabajarlo suma
// factor × horas ordinarias a la hora ordinaria del contrato.
type reglaFeriadoLaborado struct{}

func (reglaFeriadoLaborado) Aplicar(rc *ContextoRegla) error {
	rc.St.usados[formulaFeriadoLaborado] = true
	if len(rc.In.FeriadosLaborados) == 0 {
		return nil
	}
	salario, divisor, err := valorHoraOrdinaria(rc.Contrato, rc.P)
	if err != nil {
		return fmt.Errorf("feriado laborado: %w", err)
	}
	monto, horas, err := CalcularFeriadoLaborado(salario, divisor, rc.In.FeriadosLaborados,
		rc.P.FactorFeriadoObligatorio, rc.P.FactorFeriadoNoObligatorio)
	if err != nil {
		return err
	}
	if monto.IsPositive() {
		rc.St.agregar(rc.Concepto, monto, horas)
	}
	return nil
}

// CalcularFeriadoLaborado: función pura.
func CalcularFeriadoLaborado(salarioMensual, divisorHoras decimal.Decimal, feriados []FeriadoLaboradoInput,
	factorObligatorio, factorNoObligatorio *decimal.Decimal) (decimal.Decimal, decimal.Decimal, error) {
	if !divisorHoras.IsPositive() {
		return decimal.Zero, decimal.Zero, errors.New("planilla: divisor de horas de la jornada no definido para el feriado laborado")
	}
	ponderadas, totales := decimal.Zero, decimal.Zero
	for _, f := range feriados {
		if f.Horas.IsNegative() {
			return decimal.Zero, decimal.Zero, errors.New("planilla: horas de feriado laborado negativas")
		}
		if f.Horas.IsZero() {
			continue
		}
		factor, clave := factorNoObligatorio, ClaveFeriadoFactorNoObligatorio
		if f.PagoObligatorio {
			factor, clave = factorObligatorio, ClaveFeriadoFactorObligatorio
		}
		if factor == nil {
			return decimal.Zero, decimal.Zero, fmt.Errorf("%w: %s (hay horas trabajadas en un feriado de ese tipo)", ErrParametroFaltante, clave)
		}
		ponderadas = ponderadas.Add(f.Horas.Mul(*factor))
		totales = totales.Add(f.Horas)
	}
	monto := salarioMensual.Mul(ponderadas).DivRound(divisorHoras, 6).Round(dosDecimales)
	return monto, totales, nil
}
