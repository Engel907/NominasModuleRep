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
// quedan prorrateados por sus días.
type reglaSalarioBase struct{}

func (reglaSalarioBase) Aplicar(rc *ContextoRegla) error {
	p, in := rc.P, rc.In
	if !p.DivisorDiasMes.IsPositive() || !p.DiasPeriodoCompleto.IsPositive() {
		return errors.New("faltan DivisorDiasMes/DiasPeriodoCompleto para calcular el salario base")
	}
	if len(in.Segmentos) == 0 {
		return errors.New("el empleado no tiene segmentos de contrato en el período")
	}
	dias := decimal.NewFromInt(int64(DiasSegmentos(in.Segmentos)))
	if dias.GreaterThan(p.DiasPeriodoCompleto) {
		return fmt.Errorf("los segmentos suman %s días comerciales, más que el período (%s)", dias, p.DiasPeriodoCompleto)
	}
	if in.DiasTrabajados.IsNegative() || in.DiasTrabajados.GreaterThan(dias) {
		return fmt.Errorf("DiasTrabajados=%s fuera de rango [0, %s]", in.DiasTrabajados, dias)
	}
	monto := SalarioBasePeriodo(in.Segmentos, p.DivisorDiasMes)
	rc.St.salarioBase = monto
	rc.St.agregar(rc.Concepto, monto, dias)
	return nil
}
