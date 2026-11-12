package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaSalarioBase (referencia_directa): SAL_BASE a partir de
// contratos.salario_pactado (D5), por segmentos de vigencia (L10):
//
//	Σ(salario del segmento × días comerciales del segmento) ÷ DivisorDiasMes
//
// con un solo redondeo al final. Ingreso, cese y aumentos a mitad de período
// quedan prorrateados por sus días; los días de incapacidad se descuentan
// (L11: no se pagan dos veces).
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
	if total := decimal.NewFromInt(int64(DiasSegmentos(in.Segmentos))); total.GreaterThan(p.DiasPeriodoCompleto) {
		return fmt.Errorf("los segmentos suman %s días comerciales, más que el período (%s)", total, p.DiasPeriodoCompleto)
	}
	// L11: los días de incapacidad no se pagan como salario (los cubre SUB_INCAP).
	netos, err := SegmentosNetos(in.Segmentos, in.Incapacidades, divisor)
	if err != nil {
		return err
	}
	dias := decimal.NewFromInt(int64(DiasSegmentos(netos)))
	if !in.DiasTrabajados.Equal(dias) {
		return fmt.Errorf("DiasTrabajados=%s no coincide con los días con contrato sin incapacidad (%s)", in.DiasTrabajados, dias)
	}
	monto := SalarioBasePeriodo(netos, p.DivisorDiasMes)
	rc.St.salarioBase = monto
	rc.St.agregar(rc.Concepto, monto, dias)
	return nil
}
