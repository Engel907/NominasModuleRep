package planilla

import (
	"errors"
	"fmt"
)

// reglaSalarioBase (referencia_directa): SAL_BASE a partir de
// contratos.salario_pactado del contrato vigente (D5), prorrateado:
//
//	salario * FraccionMensualPeriodo * DiasTrabajados / DiasPeriodoCompleto
type reglaSalarioBase struct{}

func (reglaSalarioBase) Aplicar(rc *ContextoRegla) error {
	p, in := rc.P, rc.In
	if !p.FraccionMensualPeriodo.IsPositive() || !p.DiasPeriodoCompleto.IsPositive() {
		return errors.New("faltan FraccionMensualPeriodo/DiasPeriodoCompleto para prorratear el salario base")
	}
	if in.DiasTrabajados.IsNegative() || in.DiasTrabajados.GreaterThan(p.DiasPeriodoCompleto) {
		return fmt.Errorf("DiasTrabajados=%s fuera de rango [0, %s]", in.DiasTrabajados, p.DiasPeriodoCompleto)
	}
	monto := rc.Contrato.SalarioPactado.Mul(p.FraccionMensualPeriodo).Mul(in.DiasTrabajados).
		DivRound(p.DiasPeriodoCompleto, 6).Round(dosDecimales)
	rc.St.salarioBase = monto
	rc.St.agregar(rc.Concepto, monto, in.DiasTrabajados)
	return nil
}
