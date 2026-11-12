package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaEmbargo (embargo): embargos activos sobre el salario líquido
// (ingresos - CCSS obrero - renta neta, lo deducido antes en el flujo).
type reglaEmbargo struct{}

func (reglaEmbargo) Aplicar(rc *ContextoRegla) error {
	st := rc.St
	st.usados[formulaEmbargo] = true
	if len(rc.In.Embargos) == 0 {
		return nil
	}
	if !rc.P.SalarioMinimoInembargable.IsPositive() {
		return errors.New("falta SalarioMinimoInembargable vigente para calcular embargos")
	}
	ingresos := decimal.Zero
	for _, l := range st.ingresos {
		ingresos = ingresos.Add(l.Monto)
	}
	liquido := ingresos.Sub(st.ccssObrero).Sub(st.rentaNeta)
	aplicados := CalcularEmbargos(rc.In.Embargos, liquido, rc.P.SalarioMinimoInembargable)
	st.res.EmbargosAplicados = aplicados
	suma := decimal.Zero
	for _, e := range aplicados {
		suma = suma.Add(e.MontoAplicado)
		if e.SaldoPendienteInsatisfecho.IsPositive() {
			st.res.Advertencias = append(st.res.Advertencias, fmt.Sprintf(
				"embargo_id=%d insatisfecho por %s (límite legal / remanente)",
				e.EmbargoID, e.SaldoPendienteInsatisfecho.StringFixed(dosDecimales)))
		}
	}
	if suma.IsPositive() {
		st.agregar(rc.Concepto, suma.Round(dosDecimales), decimal.Zero)
	}
	return nil
}
