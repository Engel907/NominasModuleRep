package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaSalarioBase (referencia_directa): SAL_BASE a partir de
// contratos.salario_pactado, por segmentos de vigencia y según la modalidad de
// cada segmento:
//   - mensual: salario × días comerciales ÷ DIVISOR_DIAS_MES;
//   - quincenal: salario × 2 × días comerciales ÷ DIVISOR_DIAS_MES;
//   - semanal: salario × días naturales ÷ 7.
//
// Los días de incapacidad y, en modo disfrute, los de vacaciones no se pagan
// aquí (los pagan SUB_INCAP y PAGO_VACACIONES). Un solo redondeo al final.
type reglaSalarioBase struct{}

func (reglaSalarioBase) Aplicar(rc *ContextoRegla) error {
	p, in := rc.P, rc.In
	if !p.DivisorDiasMes.IsPositive() || !p.DiasPeriodoCompleto.IsPositive() {
		return errors.New("faltan DivisorDiasMes/DiasPeriodoCompleto para calcular el salario base")
	}
	if len(in.Segmentos) == 0 {
		return errors.New("el empleado no tiene segmentos de contrato en el período")
	}
	divisor, err := divisorEntero(p.DivisorDiasMes)
	if err != nil {
		return err
	}
	for _, s := range in.Segmentos {
		if in.FechaInicioPeriodo.IsZero() || in.FechaFinPeriodo.IsZero() {
			break
		}
		if s.Inicio.Before(soloFecha(in.FechaInicioPeriodo)) || s.Fin.After(soloFecha(in.FechaFinPeriodo)) {
			return fmt.Errorf("el segmento del contrato %d (%s a %s) sale del período", s.ContratoID,
				s.Inicio.Format("2006-01-02"), s.Fin.Format("2006-01-02"))
		}
	}
	netos, err := SegmentosNetos(in.Segmentos, in.Incapacidades, divisor)
	if err != nil {
		return err
	}
	dias := decimal.NewFromInt(int64(DiasSegmentos(netos)))
	if !in.DiasTrabajados.Equal(dias) {
		return fmt.Errorf("DiasTrabajados=%s no coincide con los días con contrato sin incapacidad (%s)", in.DiasTrabajados, dias)
	}
	pagados, err := segmentosSinVacaciones(netos, in, p, divisor)
	if err != nil {
		return err
	}
	monto, diasPagados, err := salarioBaseSegmentos(pagados, p)
	if err != nil {
		return err
	}
	rc.St.salarioBase = monto
	rc.St.agregar(rc.Concepto, monto, decimal.NewFromInt(int64(diasPagados)))
	return nil
}

// salarioBaseSegmentos: monto (redondeado a 2 decimales) y días pagados de
// los segmentos ya netos de incapacidad y vacaciones.
func salarioBaseSegmentos(segs []SegmentoSalario, p ParametrosCalculo) (decimal.Decimal, int, error) {
	total, dias := decimal.Zero, 0
	for _, s := range segs {
		m, err := s.montoSegmento(p)
		if err != nil {
			return decimal.Zero, 0, err
		}
		total = total.Add(m)
		dias += s.Dias
	}
	return total.Round(dosDecimales), dias, nil
}
