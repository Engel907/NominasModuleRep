package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaHorasExtra (horas_extra): la base es el salario MENSUAL completo del
// contrato vigente a la fecha efectiva, y el divisor de la hora ordinaria es
// el de la JORNADA del contrato (parametros_jornada, L12).
type reglaHorasExtra struct{}

func (reglaHorasExtra) Aplicar(rc *ContextoRegla) error {
	rc.St.usados[formulaHorasExtra] = true
	if len(rc.In.HorasExtra) == 0 {
		return nil
	}
	if rc.Contrato.JornadaID == nil {
		return fmt.Errorf("el contrato %d no tiene jornada del catálogo (jornada %q)", rc.Contrato.ContratoID, rc.Contrato.Jornada)
	}
	divisor, ok := rc.P.DivisorPorJornada[*rc.Contrato.JornadaID]
	if !ok {
		return fmt.Errorf("la jornada %q (id %d) no tiene parametros_jornada vigente para calcular horas extra", rc.Contrato.Jornada, *rc.Contrato.JornadaID)
	}
	monto, horas, err := CalcularHorasExtra(rc.Contrato.SalarioPactado, divisor, rc.In.HorasExtra, rc.Factores)
	if err != nil {
		return err
	}
	if monto.IsPositive() {
		rc.St.agregar(rc.Concepto, monto, horas)
	}
	return nil
}

// CalcularHorasExtra: función pura. Monto =
//
//	salarioMensual / horasMensualesOrdinarias * Σ(horas_i * factor_i)
//
// factor_recargo (parametros_horas_extra) es el multiplicador total sobre la
// hora ordinaria (1.5 = 150%); NO se hardcodea ningún factor: si falta el
// parámetro vigente de algún tipo, error. Retorna (monto, horasTotales).
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
