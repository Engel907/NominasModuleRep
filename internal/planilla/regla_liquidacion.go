package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaLiquidacion (formula 'liquidacion'): el monto del concepto LIQ_*
// es el rubro de la liquidación ya calculado por internal/liquidacion y
// persistido al procesarla (expresion_formula.rubro). No recalcula nada: las
// deducciones (CCSS, renta, embargo, préstamo) las aplican sus reglas
// normales, y el rubro respeta las banderas afecta_base_* de su concepto.
type reglaLiquidacion struct{}

func (reglaLiquidacion) Aplicar(rc *ContextoRegla) error {
	rc.St.usados[formulaLiquidacion] = true
	if rc.In.Liquidacion == nil {
		return errors.New("el período no tiene liquidación para este empleado (concepto LIQ_* fuera de una planilla de liquidación)")
	}
	if rc.Concepto.Rubro == "" {
		return errors.New("expresion_formula de tipo liquidacion sin 'rubro'")
	}
	monto, ok := rc.In.Liquidacion.Rubros[rc.Concepto.Rubro]
	if !ok {
		return fmt.Errorf("rubro de liquidación desconocido %q", rc.Concepto.Rubro)
	}
	if monto.IsNegative() {
		return fmt.Errorf("rubro %q negativo (%s) en la liquidación %d", rc.Concepto.Rubro, monto, rc.In.Liquidacion.LiquidacionID)
	}
	if monto.IsPositive() {
		rc.St.agregar(rc.Concepto, monto.Round(dosDecimales), decimal.Zero)
	}
	return nil
}
