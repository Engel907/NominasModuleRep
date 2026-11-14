package planilla

import "testing"

// Tramos de prueba (no son los legales): 0 % hasta 900 000, 10 % hasta
// 1 300 000, 15 % el resto.
func paramsRenta(fraccion, metodo string) ParametrosCalculo {
	return ParametrosCalculo{
		TramosRenta: []TramoRenta{
			{OrdenTramo: 1, MontoDesde: d("0"), MontoHasta: d("900000"), Porcentaje: d("0")},
			{OrdenTramo: 2, MontoDesde: d("900000"), MontoHasta: d("1300000"), Porcentaje: d("0.10")},
			{OrdenTramo: 3, MontoDesde: d("1300000"), Porcentaje: d("0.15"), Abierto: true},
		},
		FraccionMensualPeriodo: d(fraccion),
		RentaMetodoPeriodo:     metodo,
	}
}

// Dos quincenas de 600 000: el mes grava 1 200 000 → 30 000. Por separado,
// con los tramos mensuales completos, cada quincena quedaría exenta.
func TestRentaQuincenalAcumulada(t *testing.T) {
	p := paramsRenta("0.5", RentaAcumuladoMes)
	q1, _, err := CalcularRentaPeriodo(d("600000"), RentaAcumuladaMes{}, nil, p)
	if err != nil || !q1.ImpuestoNeto.IsZero() {
		t.Fatalf("q1 = %s, %v; esperaba 0", q1.ImpuestoNeto, err)
	}
	previa := RentaAcumuladaMes{Base: d("600000"), Bruta: q1.ImpuestoBruto, Neta: q1.ImpuestoNeto}
	q2, _, err := CalcularRentaPeriodo(d("600000"), previa, nil, p)
	if err != nil || !q2.ImpuestoNeto.Equal(d("30000")) {
		t.Fatalf("q2 = %s, %v; esperaba 30000", q2.ImpuestoNeto, err)
	}
}

func TestRentaQuincenalProyeccion(t *testing.T) {
	p := paramsRenta("0.5", RentaProyeccion)
	creditos := []CreditoFiscalResuelto{{TipoCreditoFiscalID: 1, MontoTotal: d("2000")}}
	r, _, err := CalcularRentaPeriodo(d("600000"), RentaAcumuladaMes{}, creditos, p)
	// tramos × 0.5: 0 % hasta 450 000, 10 % hasta 650 000 → 15 000 − 1 000 de crédito.
	if err != nil || !r.ImpuestoBruto.Equal(d("15000")) || !r.ImpuestoNeto.Equal(d("14000")) {
		t.Fatalf("bruto=%s neto=%s err=%v", r.ImpuestoBruto, r.ImpuestoNeto, err)
	}
}

func TestRentaQuincenalSinMetodo(t *testing.T) {
	if _, _, err := CalcularRentaPeriodo(d("600000"), RentaAcumuladaMes{}, nil, paramsRenta("0.5", "")); err == nil {
		t.Fatal("esperaba error: período de media fracción sin RENTA_METODO_PERIODO")
	}
}

// La planilla de liquidación (fracción 0) acumula con la regular del mes, y
// los créditos ya aplicados en el mes no se aplican dos veces.
func TestRentaLiquidacionAcumulaConElMes(t *testing.T) {
	p := paramsRenta("0", "")
	creditos := []CreditoFiscalResuelto{{TipoCreditoFiscalID: 1, MontoTotal: d("2000")}}
	previa := RentaAcumuladaMes{Base: d("1000000"), Bruta: d("10000"), Creditos: d("2000"), Neta: d("8000")}
	r, _, err := CalcularRentaPeriodo(d("200000"), previa, creditos, p)
	// mes 1 200 000 → 30 000 − 2 000 = 28 000; ya retenidos 8 000.
	if err != nil || !r.ImpuestoNeto.Equal(d("20000")) || !r.CreditosAplicados.IsZero() {
		t.Fatalf("neto=%s créditos=%s err=%v", r.ImpuestoNeto, r.CreditosAplicados, err)
	}
}

// Nunca se devuelve renta en planilla: si lo ya retenido supera el impuesto
// del mes, el período retiene 0.
func TestRentaNoNegativa(t *testing.T) {
	previa := RentaAcumuladaMes{Base: d("100000"), Bruta: d("50000"), Neta: d("50000")}
	r, _, err := CalcularRentaPeriodo(d("100000"), previa, nil, paramsRenta("1", ""))
	if err != nil || !r.ImpuestoNeto.IsZero() || r.ImpuestoBruto.IsNegative() {
		t.Fatalf("neto=%s bruto=%s err=%v", r.ImpuestoNeto, r.ImpuestoBruto, err)
	}
}
