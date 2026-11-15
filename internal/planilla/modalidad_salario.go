package planilla

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Valores de contratos.modalidad_salario: período que remunera el monto de
// salario_pactado.
const (
	ModalidadMensual   = "mensual"   // monto del mes; se devenga por días comerciales
	ModalidadQuincenal = "quincenal" // monto de la quincena (medio mes); por días comerciales
	ModalidadSemanal   = "semanal"   // monto de la semana (7 días naturales); por días naturales
)

// diasSemana: días naturales que remunera un salario semanal (incluido el
// descanso semanal, como el salario mensual incluye los descansos del mes).
const diasSemana = 7

// ModalidadSalarioValida: mensual | quincenal | semanal.
func ModalidadSalarioValida(m string) bool {
	return m == ModalidadMensual || m == ModalidadQuincenal || m == ModalidadSemanal
}

// BaseDias: cómo se cuentan los días de un segmento. Los salarios mensual y
// quincenal se devengan por días comerciales (cada mes vale Divisor días);
// el semanal, por días naturales.
type BaseDias struct {
	Divisor   int
	Naturales bool
}

// Contar: días de [desde, hasta] (ambos inclusive) en esta base.
func (b BaseDias) Contar(desde, hasta time.Time) int {
	if b.Naturales {
		desde, hasta = soloFecha(desde), soloFecha(hasta)
		if hasta.Before(desde) {
			return 0
		}
		return int(diasEntre(desde, hasta)) + 1
	}
	return DiasComerciales(desde, hasta, b.Divisor)
}

// baseDiasModalidad: base de días con que se devenga una modalidad.
func baseDiasModalidad(modalidad string, divisor int) BaseDias {
	return BaseDias{Divisor: divisor, Naturales: modalidad == ModalidadSemanal}
}

// Base: base de días del segmento.
func (s SegmentoSalario) Base(divisor int) BaseDias { return baseDiasModalidad(s.Modalidad, divisor) }

// EquivalenteMensual: el salario llevado al mes. Quincenal × 2; semanal ÷ 7 ×
// DIVISOR_DIAS_MES (el día de un salario semanal vale lo mismo en la planilla y
// en el mes equivalente).
func EquivalenteMensual(salario decimal.Decimal, modalidad string, divisorDiasMes decimal.Decimal) (decimal.Decimal, error) {
	switch modalidad {
	case ModalidadMensual, "":
		return salario, nil
	case ModalidadQuincenal:
		return salario.Mul(decimal.NewFromInt(2)), nil
	case ModalidadSemanal:
		if !divisorDiasMes.IsPositive() {
			return decimal.Zero, errors.New("DIVISOR_DIAS_MES no definido para llevar el salario semanal al mes")
		}
		return salario.Mul(divisorDiasMes).DivRound(decimal.NewFromInt(diasSemana), 6), nil
	}
	return decimal.Zero, fmt.Errorf("modalidad de salario desconocida %q", modalidad)
}

// ValorDia: lo que vale un día de salario del segmento. Mensual y quincenal:
// equivalente mensual ÷ DIVISOR_DIAS_MES (día comercial); semanal: salario ÷ 7
// (día natural).
func (s SegmentoSalario) ValorDia(p ParametrosCalculo) (decimal.Decimal, error) {
	return valorDiaModalidad(s.Salario, s.Modalidad, s.ContratoID, p)
}

func valorDiaModalidad(salario decimal.Decimal, modalidad string, contratoID int64, p ParametrosCalculo) (decimal.Decimal, error) {
	switch modalidad {
	case ModalidadMensual, ModalidadQuincenal, "":
		if !p.DivisorDiasMes.IsPositive() {
			return decimal.Zero, errors.New("DIVISOR_DIAS_MES no definido para valorar el día")
		}
		m, err := EquivalenteMensual(salario, modalidad, p.DivisorDiasMes)
		if err != nil {
			return decimal.Zero, err
		}
		return m.Div(p.DivisorDiasMes), nil
	case ModalidadSemanal:
		return salario.Div(decimal.NewFromInt(diasSemana)), nil
	}
	return decimal.Zero, fmt.Errorf("modalidad de salario desconocida %q (contrato %d)", modalidad, contratoID)
}

// montoSegmento: lo que devenga el segmento por sus días (sin redondear).
func (s SegmentoSalario) montoSegmento(p ParametrosCalculo) (decimal.Decimal, error) {
	switch s.Modalidad {
	case ModalidadMensual, ModalidadQuincenal, "":
		m, err := EquivalenteMensual(s.Salario, s.Modalidad, p.DivisorDiasMes)
		if err != nil {
			return decimal.Zero, err
		}
		return m.Mul(decimal.NewFromInt(int64(s.Dias))).DivRound(p.DivisorDiasMes, 6), nil
	case ModalidadSemanal:
		return s.Salario.Mul(decimal.NewFromInt(int64(s.Dias))).DivRound(decimal.NewFromInt(diasSemana), 6), nil
	}
	return decimal.Zero, fmt.Errorf("el contrato %d tiene una modalidad de salario desconocida %q", s.ContratoID, s.Modalidad)
}

// valorHoraOrdinaria: salario mensual equivalente y divisor de horas
// mensuales de la jornada (parametros_jornada) con que se valora la hora
// ordinaria (monto = salario × horas ÷ divisor).
func valorHoraOrdinaria(c ContratoVigente, p ParametrosCalculo) (salario, divisor decimal.Decimal, err error) {
	if c.JornadaID == nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("el contrato %d no tiene jornada del catálogo", c.ContratoID)
	}
	d, ok := p.DivisorPorJornada[*c.JornadaID]
	if !ok {
		return decimal.Zero, decimal.Zero, fmt.Errorf("la jornada %q (id %d) no tiene parametros_jornada vigente", c.Jornada, *c.JornadaID)
	}
	m, err := EquivalenteMensual(c.SalarioPactado, c.ModalidadSalario, p.DivisorDiasMes)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("contrato %d: %w", c.ContratoID, err)
	}
	return m, d, nil
}
