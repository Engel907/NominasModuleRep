package planilla

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// ClaveRentaMetodoPeriodo: parametros_generales (texto). El impuesto al
// salario se liquida por mes; esta clave decide cómo se retiene en un período
// que cubre una fracción del mes (quincena, semana).
const ClaveRentaMetodoPeriodo = "RENTA_METODO_PERIODO"

const (
	// RentaAcumuladoMes: cada período grava el total del mes (lo de este
	// período más lo de los períodos ya aprobados del mes) y retiene la
	// diferencia con lo ya retenido. Exacto al cierre del mes.
	RentaAcumuladoMes = "acumulado_mes"
	// RentaProyeccion: cada período se grava por separado con los tramos y
	// créditos proporcionales a su fracción del mes.
	RentaProyeccion = "proyeccion"
)

// MetodoRentaValido indica si el valor es un método admitido.
func MetodoRentaValido(m string) bool { return m == RentaAcumuladoMes || m == RentaProyeccion }

// reglaRenta (renta): impuesto sobre la renta por tramos sobre la base de
// renta de los ingresos ya calculados, menos créditos fiscales.
type reglaRenta struct{}

func (reglaRenta) Aplicar(rc *ContextoRegla) error {
	st := rc.St
	creditos, err := ResolverCreditosFiscales(rc.In.CreditosFiscales, rc.P.CreditosFiscales)
	if err != nil {
		return err
	}
	r, parcial, err := CalcularRentaPeriodo(CalcularBaseRenta(st.ingresos), rc.In.RentaMesPrevia, creditos, rc.P)
	if err != nil {
		return err
	}
	st.res.ImpuestoRentaBruto = r.ImpuestoBruto
	st.res.CreditosFiscalesAplicados = r.CreditosAplicados
	st.rentaNeta = r.ImpuestoNeto
	if parcial {
		st.res.Advertencias = append(st.res.Advertencias, fmt.Sprintf(
			"créditos fiscales aplicados parcialmente: solicitados %s, aplicados %s en el mes",
			r.CreditosSolicitados.StringFixed(dosDecimales), r.CreditosAplicados.StringFixed(dosDecimales)))
	}
	if r.ImpuestoNeto.IsPositive() {
		st.agregar(rc.Concepto, r.ImpuestoNeto, decimal.Zero)
	}
	return nil
}

// CalcularRentaPeriodo devuelve el impuesto que corresponde retener en el
// período y si los créditos del mes no alcanzaron a aplicarse completos.
//   - Período mensual o de liquidación (FraccionMensualPeriodo 0 o 1), o
//     RENTA_METODO_PERIODO = acumulado_mes: impuesto del mes sobre
//     base + previa.Base, menos lo ya retenido (nunca negativo).
//   - RENTA_METODO_PERIODO = proyeccion: tramos y créditos × fracción.
func CalcularRentaPeriodo(base decimal.Decimal, previa RentaAcumuladaMes, creditos []CreditoFiscalResuelto, p ParametrosCalculo) (ResultadoImpuestoRenta, bool, error) {
	f := p.FraccionMensualPeriodo
	fraccionado := f.IsPositive() && f.LessThan(decimal.NewFromInt(1))
	if fraccionado {
		switch p.RentaMetodoPeriodo {
		case RentaProyeccion:
			r, err := CalcularImpuestoRenta(base, escalarTramos(p.TramosRenta, f), escalarCreditos(creditos, f))
			return r, r.CreditosAplicados.LessThan(r.CreditosSolicitados), err
		case RentaAcumuladoMes:
		default:
			return ResultadoImpuestoRenta{}, false, fmt.Errorf("%s = %q inválido para un período de fracción %s (%s | %s)",
				ClaveRentaMetodoPeriodo, p.RentaMetodoPeriodo, f, RentaAcumuladoMes, RentaProyeccion)
		}
	}
	mes, err := CalcularImpuestoRenta(base.Add(previa.Base), p.TramosRenta, creditos)
	if err != nil {
		return ResultadoImpuestoRenta{}, false, err
	}
	neta := decimal.Max(mes.ImpuestoNeto.Sub(previa.Neta), decimal.Zero)
	bruta := decimal.Max(mes.ImpuestoBruto.Sub(previa.Bruta), neta)
	return ResultadoImpuestoRenta{
		ImpuestoBruto:       bruta,
		CreditosSolicitados: mes.CreditosSolicitados,
		CreditosAplicados:   bruta.Sub(neta),
		ImpuestoNeto:        neta,
	}, mes.CreditosAplicados.LessThan(mes.CreditosSolicitados), nil
}

// escalarTramos: los límites de cada tramo × f (equivale a gravar base / f y
// multiplicar el impuesto por f).
func escalarTramos(tramos []TramoRenta, f decimal.Decimal) []TramoRenta {
	out := make([]TramoRenta, len(tramos))
	for i, t := range tramos {
		t.MontoDesde = t.MontoDesde.Mul(f).Round(dosDecimales)
		if !t.Abierto {
			t.MontoHasta = t.MontoHasta.Mul(f).Round(dosDecimales)
		}
		out[i] = t
	}
	return out
}

func escalarCreditos(creditos []CreditoFiscalResuelto, f decimal.Decimal) []CreditoFiscalResuelto {
	out := make([]CreditoFiscalResuelto, len(creditos))
	for i, c := range creditos {
		c.MontoTotal = c.MontoTotal.Mul(f).Round(dosDecimales)
		out[i] = c
	}
	return out
}
