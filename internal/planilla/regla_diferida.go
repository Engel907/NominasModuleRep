package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaDiferida (aguinaldo, salario_escolar, vacaciones).
type reglaDiferida struct{}

func (reglaDiferida) Aplicar(rc *ContextoRegla) error {
	var (
		monto decimal.Decimal
		err   error
	)
	switch rc.Concepto.TipoFormula {
	case formulaAguinaldo:
		monto, err = CalcularAguinaldo(rc.In.BaseAguinaldoAcumulada, rc.P.AguinaldoMeses)
	case formulaSalarioEscolar:
		if !rc.P.PorcentajeSalarioEscolar.IsPositive() {
			return errors.New("no hay parametros_salario_escolar vigente")
		}
		monto, err = CalcularSalarioEscolar(rc.In.BaseSalarioEscolarAcumulada, rc.P.PorcentajeSalarioEscolar)
	case formulaVacaciones:
		rc.St.usados[formulaVacaciones] = true
		monto, dias, movs, err := calcularPagoVacaciones(rc.In, rc.P)
		if err != nil {
			return err
		}
		rc.St.vacacionesCompensadas = movs
		if monto.IsPositive() {
			rc.St.agregar(rc.Concepto, monto, dias)
		}
		return nil
	default:
		return fmt.Errorf("regla diferida sin tipo conocido %q", rc.Concepto.TipoFormula)
	}
	if err != nil {
		return err
	}
	if monto.IsPositive() {
		rc.St.agregar(rc.Concepto, monto, decimal.Zero)
	}
	return nil
}
