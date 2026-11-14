package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaHorasExtra (horas_extra): la hora ordinaria sale del contrato vigente a
// la fecha efectiva: salario mensual ÷ horas mensuales de la JORNADA
// (parametros_jornada), salario diario ÷ horas diarias de la jornada, o la
// tarifa por hora (valorHoraOrdinaria).
type reglaHorasExtra struct{}

func (reglaHorasExtra) Aplicar(rc *ContextoRegla) error {
	rc.St.usados[formulaHorasExtra] = true
	if len(rc.In.HorasExtra) == 0 {
		return nil
	}
	salario, divisor, err := valorHoraOrdinaria(rc.Contrato, rc.P)
	if err != nil {
		return fmt.Errorf("horas extra: %w", err)
	}
	monto, horas, err := CalcularHorasExtra(salario, divisor, rc.In.HorasExtra, rc.Factores)
	if err != nil {
		return err
	}
	if monto.IsPositive() {
		rc.St.agregar(rc.Concepto, monto, horas)
	}
	return nil
}

// CalcularHorasExtra: función pura. Monto = salario × Σ(horas × factor) ÷
// divisor de la hora (horas del mes, del día, o 1 si el salario es por hora).
func CalcularHorasExtra(
	salarioMensual, horasMensualesOrdinarias decimal.Decimal,
	horas []HoraExtraInput,
	factores map[int64]ParametroHoraExtra,
) (decimal.Decimal, decimal.Decimal, error) {
	if len(horas) == 0 {
		return decimal.Zero, decimal.Zero, nil
	}
	if !horasMensualesOrdinarias.IsPositive() {
		return decimal.Zero, decimal.Zero, errors.New("planilla: horas mensuales ordinarias (divisor de la hora) no definidas para calcular horas extra")
	}
	if salarioMensual.IsNegative() {
		return decimal.Zero, decimal.Zero, errors.New("planilla: salario mensual negativo al calcular horas extra")
	}

	horasPonderadas := decimal.Zero
	horasTotales := decimal.Zero
	for _, h := range horas {
		if h.CantidadHoras.IsNegative() {
			return decimal.Zero, decimal.Zero, fmt.Errorf("planilla: horas extra negativas para tipo_hora_extra_id=%d", h.TipoHoraExtraID)
		}
		p, ok := factores[h.TipoHoraExtraID]
		if !ok {
			return decimal.Zero, decimal.Zero, fmt.Errorf("planilla: no hay parametro vigente de horas extra para tipo_hora_extra_id=%d", h.TipoHoraExtraID)
		}
		horasPonderadas = horasPonderadas.Add(h.CantidadHoras.Mul(p.FactorRecargo))
		horasTotales = horasTotales.Add(h.CantidadHoras)
	}
	monto := salarioMensual.Mul(horasPonderadas).DivRound(horasMensualesOrdinarias, 6).Round(dosDecimales)
	return monto, horasTotales, nil
}
