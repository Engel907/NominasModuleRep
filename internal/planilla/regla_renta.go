package planilla

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaRenta (renta): impuesto sobre la renta por tramos sobre la base de
// renta de los ingresos ya calculados, menos créditos fiscales.
type reglaRenta struct{}

func (reglaRenta) Aplicar(rc *ContextoRegla) error {
	st := rc.St
	creditos, err := ResolverCreditosFiscales(rc.In.CreditosFiscales, rc.P.CreditosFiscales)
	if err != nil {
		return err
	}
	r, err := CalcularImpuestoRenta(CalcularBaseRenta(st.ingresos), rc.P.TramosRenta, creditos)
	if err != nil {
		return err
	}
	st.res.ImpuestoRentaBruto = r.ImpuestoBruto
	st.res.CreditosFiscalesAplicados = r.CreditosAplicados
	st.rentaNeta = r.ImpuestoNeto
	if r.CreditosAplicados.LessThan(r.CreditosSolicitados) {
		st.res.Advertencias = append(st.res.Advertencias, fmt.Sprintf(
			"créditos fiscales aplicados parcialmente: solicitados %s, aplicados %s",
			r.CreditosSolicitados.StringFixed(dosDecimales), r.CreditosAplicados.StringFixed(dosDecimales)))
	}
	if r.ImpuestoNeto.IsPositive() {
		st.agregar(rc.Concepto, r.ImpuestoNeto, decimal.Zero)
	}
	return nil
}
