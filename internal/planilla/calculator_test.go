package planilla

import (
	"testing"

	"github.com/shopspring/decimal"
)

// dec es un atajo para construir decimal.Decimal desde un literal string en
// los tests, evitando por completo decimal.NewFromFloat (que arrastra
// imprecisión binaria).
func dec(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

// tramosRenta2026 reproduce la escala mensual vigente desde el 1° de enero
// de 2026 según el Decreto Ejecutivo N° 45333-H (Gaceta N° 229 del 5 de
// diciembre de 2025): exento hasta ¢918.000; 10% de ¢918.000 a ¢1.347.000;
// 15% de ¢1.347.000 a ¢2.364.000; 20% de ¢2.364.000 a ¢4.727.000; 25%
// sobre el exceso de ¢4.727.000. En producción estos tramos siempre se
// resuelven desde parametros_tramos_renta (BD) — aquí se fijan como
// fixture de test para reproducir la ley vigente de forma determinística.
func tramosRenta2026() []TramoRenta {
	return []TramoRenta{
		{OrdenTramo: 1, MontoDesde: dec("0"), MontoHasta: dec("918000"), Porcentaje: dec("0")},
		{OrdenTramo: 2, MontoDesde: dec("918000"), MontoHasta: dec("1347000"), Porcentaje: dec("0.10")},
		{OrdenTramo: 3, MontoDesde: dec("1347000"), MontoHasta: dec("2364000"), Porcentaje: dec("0.15")},
		{OrdenTramo: 4, MontoDesde: dec("2364000"), MontoHasta: dec("4727000"), Porcentaje: dec("0.20")},
		{OrdenTramo: 5, MontoDesde: dec("4727000"), Porcentaje: dec("0.25"), Abierto: true},
	}
}

// cargaSocial2026 reproduce el porcentaje obrero y patronal de 2026
// (10.83 % obrero, 26.83 % patronal).
func cargaSocial2026() ParametroCargaSocial {
	return ParametroCargaSocial{
		ConceptoSalarialID: 1,
		PorcentajeObrero:   dec("0.1083"),
		PorcentajePatronal: dec("0.2683"),
	}
}

// salarioMinimoInembargable2026 reproduce el monto vigente desde el 1° de
// enero de 2026 (Decreto de Salarios N° 45303-MTSS) usado como base
// protegida del Art. 172 del Código de Trabajo.
const salarioMinimoInembargable2026 = "268711.27"

func TestCalcularImpuestoRenta_TramoExento(t *testing.T) {
	resultado, err := CalcularImpuestoRenta(dec("918000"), tramosRenta2026(), nil)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !resultado.ImpuestoBruto.Equal(dec("0")) {
		t.Errorf("ImpuestoBruto = %s, esperado 0 (salario exactamente en el tope exento)", resultado.ImpuestoBruto)
	}
	if !resultado.ImpuestoNeto.Equal(dec("0")) {
		t.Errorf("ImpuestoNeto = %s, esperado 0", resultado.ImpuestoNeto)
	}
}

func TestCalcularImpuestoRenta_Tramos(t *testing.T) {
	casos := []struct {
		nombre                string
		baseRenta             decimal.Decimal
		creditos              []CreditoFiscalResuelto
		impuestoBrutoEsperado decimal.Decimal
		impuestoNetoEsperado  decimal.Decimal
	}{
		{
			nombre:                "tramo 10%",
			baseRenta:             dec("1200000"),
			impuestoBrutoEsperado: dec("28200.00"),
			impuestoNetoEsperado:  dec("28200.00"),
		},
		{
			nombre:                "tramo 15%",
			baseRenta:             dec("1800000"),
			impuestoBrutoEsperado: dec("110850.00"),
			impuestoNetoEsperado:  dec("110850.00"),
		},
		{
			nombre:                "tramo 20%",
			baseRenta:             dec("3000000"),
			impuestoBrutoEsperado: dec("322650.00"),
			impuestoNetoEsperado:  dec("322650.00"),
		},
		{
			nombre:    "tramo 25% con creditos de 2 hijos y conyuge",
			baseRenta: dec("5000000"),
			creditos: []CreditoFiscalResuelto{
				{TipoCreditoFiscalID: 1, MontoTotal: dec("3420.00")},
				{TipoCreditoFiscalID: 2, MontoTotal: dec("2590.00")},
			},
			impuestoBrutoEsperado: dec("736300.00"),
			impuestoNetoEsperado:  dec("730290.00"),
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			resultado, err := CalcularImpuestoRenta(c.baseRenta, tramosRenta2026(), c.creditos)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !resultado.ImpuestoBruto.Equal(c.impuestoBrutoEsperado) {
				t.Errorf("ImpuestoBruto = %s, esperado %s", resultado.ImpuestoBruto, c.impuestoBrutoEsperado)
			}
			if !resultado.ImpuestoNeto.Equal(c.impuestoNetoEsperado) {
				t.Errorf("ImpuestoNeto = %s, esperado %s", resultado.ImpuestoNeto, c.impuestoNetoEsperado)
			}
		})
	}
}

func TestCalcularImpuestoRenta_SinTramosVigentesRetornaError(t *testing.T) {
	_, err := CalcularImpuestoRenta(dec("1000000"), nil, nil)
	if err == nil {
		t.Fatal("se esperaba error explícito por falta de tramos vigentes, se obtuvo nil")
	}
}

func TestCalcularImpuestoRenta_CeroNoImplicaTramoAbierto(t *testing.T) {
	tramos := tramosRenta2026()
	tramos[len(tramos)-1].Abierto = false
	if _, err := CalcularImpuestoRenta(dec("5000000"), tramos, nil); err == nil {
		t.Fatal("se esperaba error por límite superior inválido; MontoHasta cero no debe marcar tramo abierto")
	}
}

// escalaHistorica: la escala sembrada en parametros_embargo_escala (000118),
// ya en fracción como la entrega el cargador: excedente hasta 3 mínimos al
// 12.5 %, el resto al 25 %.
func escalaHistorica() []TramoEscalaEmbargo {
	return []TramoEscalaEmbargo{
		{OrdenTramo: 1, SalarioDesde: dec("0"), SalarioHasta: decimal.NewNullDecimal(dec("3")), Porcentaje: dec("0.125")},
		{OrdenTramo: 2, SalarioDesde: dec("3"), Porcentaje: dec("0.25")},
	}
}

// pension / judicial: tipos como los sembrados (prioridad legal 1 y 2; la
// pensión con tope del 50 % del líquido).
func pension(id int64, pct string) EmbargoInput {
	return EmbargoInput{EmbargoID: id, TipoEmbargoID: 1, Prioridad: 1, PrioridadLegal: 1, EsPension: true, TopePctLiquido: dec("0.5"), Porcentaje: dec(pct)}
}

func judicial(id int64, pct string) EmbargoInput {
	return EmbargoInput{EmbargoID: id, TipoEmbargoID: 2, Prioridad: 2, PrioridadLegal: 2, Porcentaje: dec(pct)}
}

func calcularEmbargos(t *testing.T, emb []EmbargoInput, liquido string) map[int64]EmbargoAplicadoOutput {
	t.Helper()
	res, err := CalcularEmbargos(emb, dec(liquido), dec(salarioMinimoInembargable2026), escalaHistorica())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != len(emb) {
		t.Fatalf("se esperaban %d resultados, se obtuvieron %d", len(emb), len(res))
	}
	out := map[int64]EmbargoAplicadoOutput{}
	for _, r := range res {
		if !r.SaldoPendienteInsatisfecho.Equal(r.MontoCalculado.Sub(r.MontoAplicado)) || r.MontoAplicado.IsNegative() {
			t.Errorf("embargo %d con montos inconsistentes: %+v", r.EmbargoID, r)
		}
		out[r.EmbargoID] = r
	}
	return out
}

func TestCalcularEmbargos(t *testing.T) {
	t.Run("embargo general queda en cero si el liquido no supera el minimo inembargable", func(t *testing.T) {
		r := calcularEmbargos(t, []EmbargoInput{judicial(1, "0.20")}, "200000")[1]
		if !r.MontoAplicado.Equal(dec("0")) || !r.SaldoPendienteInsatisfecho.Equal(dec("40000")) {
			t.Errorf("aplicado %s, insatisfecho %s; esperado 0 y 40000", r.MontoAplicado, r.SaldoPendienteInsatisfecho)
		}
	})

	t.Run("una pension respeta el tope del 50% de su tipo", func(t *testing.T) {
		r := calcularEmbargos(t, []EmbargoInput{pension(2, "0.60")}, "800000")[2]
		if !r.MontoAplicado.Equal(dec("400000")) || !r.SaldoPendienteInsatisfecho.Equal(dec("80000")) {
			t.Errorf("aplicado %s, insatisfecho %s; esperado 400000 (tope 50 %%) y 80000", r.MontoAplicado, r.SaldoPendienteInsatisfecho)
		}
	})

	t.Run("pension sobre liquido bajo el minimo se practica igual", func(t *testing.T) {
		r := calcularEmbargos(t, []EmbargoInput{pension(2, "0.30")}, "200000")[2]
		if !r.MontoAplicado.Equal(dec("60000")) {
			t.Errorf("aplicado %s, esperado 60000", r.MontoAplicado)
		}
	})

	t.Run("dos pensiones que juntas pasan el tope: el tope es acumulado", func(t *testing.T) {
		r := calcularEmbargos(t, []EmbargoInput{pension(6, "0.30"), pension(5, "0.30")}, "1000000")
		if !r[5].MontoAplicado.Equal(dec("300000")) || !r[6].MontoAplicado.Equal(dec("200000")) || !r[6].SaldoPendienteInsatisfecho.Equal(dec("100000")) {
			t.Errorf("pensiones: id5 %s, id6 %s (insatisfecho %s); esperado 300000 / 200000 / 100000",
				r[5].MontoAplicado, r[6].MontoAplicado, r[6].SaldoPendienteInsatisfecho)
		}
	})

	t.Run("judicial y pension: la pension primero y el judicial toma el fondo de la escala", func(t *testing.T) {
		r := calcularEmbargos(t, []EmbargoInput{judicial(3, "0.50"), pension(4, "0.30")}, "1000000")
		if !r[4].MontoAplicado.Equal(dec("300000")) {
			t.Errorf("pension aplicado %s, esperado 300000", r[4].MontoAplicado)
		}
		if !r[3].MontoAplicado.Equal(dec("53911.09")) || !r[3].SaldoPendienteInsatisfecho.Equal(dec("446088.91")) {
			t.Errorf("judicial aplicado %s, insatisfecho %s; esperado 53911.09 / 446088.91", r[3].MontoAplicado, r[3].SaldoPendienteInsatisfecho)
		}
	})

	t.Run("excedente en el segundo tramo de la escala", func(t *testing.T) {
		r := calcularEmbargos(t, []EmbargoInput{judicial(1, "0.50")}, "1500000")[1]
		if !r.MontoAplicado.Equal(dec("207055.46")) {
			t.Errorf("aplicado %s, esperado 207055.46", r.MontoAplicado)
		}
	})

	t.Run("orden por prioridad legal y luego prioridad del embargo; el fondo se agota", func(t *testing.T) {
		hacienda := EmbargoInput{EmbargoID: 10, TipoEmbargoID: 3, Prioridad: 3, PrioridadLegal: 3, MontoFijo: dec("50000")}
		r := calcularEmbargos(t, []EmbargoInput{hacienda, judicial(11, "0.03")}, "1000000")
		if !r[11].MontoAplicado.Equal(dec("30000")) || !r[10].MontoAplicado.Equal(dec("50000")) {
			t.Errorf("judicial %s, hacienda %s", r[11].MontoAplicado, r[10].MontoAplicado)
		}
		hacienda.MontoFijo = dec("70000")
		r = calcularEmbargos(t, []EmbargoInput{hacienda, judicial(11, "0.03")}, "1000000")
		if !r[10].MontoAplicado.Equal(dec("61411.09")) {
			t.Errorf("hacienda debería quedar en el resto del fondo (61411.09), fue %s", r[10].MontoAplicado)
		}
	})

	t.Run("tope mensual propio del embargo", func(t *testing.T) {
		e := judicial(1, "0.50")
		e.TopeMensual = dec("20000")
		r := calcularEmbargos(t, []EmbargoInput{e}, "1000000")[1]
		if !r.MontoCalculado.Equal(dec("20000")) || !r.MontoAplicado.Equal(dec("20000")) {
			t.Errorf("calculado %s, aplicado %s; esperado 20000", r.MontoCalculado, r.MontoAplicado)
		}
	})

	t.Run("errores explicitos", func(t *testing.T) {
		emb := []EmbargoInput{judicial(1, "0.10")}
		if _, err := CalcularEmbargos(emb, dec("1000000"), decimal.Zero, escalaHistorica()); err == nil {
			t.Error("sin SALARIO_MINIMO_INEMBARGABLE debe dar error")
		}
		smi := dec(salarioMinimoInembargable2026)
		if _, err := CalcularEmbargos(emb, dec("1000000"), smi, nil); err == nil {
			t.Error("sin escala debe dar error")
		}
		hueco := escalaHistorica()
		hueco[1].SalarioDesde = dec("4")
		if _, err := CalcularEmbargos(emb, dec("1000000"), smi, hueco); err == nil {
			t.Error("escala con hueco debe dar error")
		}
		abierto := escalaHistorica()
		abierto[0].SalarioHasta = decimal.NullDecimal{}
		if _, err := CalcularEmbargos(emb, dec("1000000"), smi, abierto); err == nil {
			t.Error("tramo abierto que no es el último debe dar error")
		}
	})
}

func TestBalanceIngresosDeduccionesNeto(t *testing.T) {
	devengos := []ConceptoCalculadoOutput{
		{
			ConceptoSalarialID: 1,
			Codigo:             "SAL_BASE",
			OrdenCalculo:       1,
			Monto:              dec("1500000.00"),
			AfectaBaseCCSS:     true,
			AfectaBaseRenta:    true,
		},
	}

	baseCCSS := CalcularBaseCCSS(devengos)
	baseRenta := CalcularBaseRenta(devengos)

	cuotaObrero, cuotaPatronal := CalcularCCSS(baseCCSS, cargaSocial2026())
	_ = cuotaPatronal

	creditos := []CreditoFiscalResuelto{
		{TipoCreditoFiscalID: 1, MontoTotal: dec("1710.00")},
	}
	resultadoRenta, err := CalcularImpuestoRenta(baseRenta, tramosRenta2026(), creditos)
	if err != nil {
		t.Fatalf("error inesperado calculando renta: %v", err)
	}

	prestamos := CalcularPrestamos([]PrestamoDeduccionInput{
		{PrestamoID: 1, CuotaPeriodica: dec("50000.00"), SaldoPendiente: dec("200000.00")},
	})

	totalIngresos := baseCCSS
	totalDeducciones := cuotaObrero.Add(resultadoRenta.ImpuestoNeto).Add(prestamos)
	netoPagado := totalIngresos.Sub(totalDeducciones)

	if !cuotaObrero.Equal(dec("162450.00")) {
		t.Errorf("cuotaObrero = %s, esperado 162450.00", cuotaObrero)
	}
	if !resultadoRenta.ImpuestoNeto.Equal(dec("64140.00")) {
		t.Errorf("ImpuestoRentaNeto = %s, esperado 64140.00", resultadoRenta.ImpuestoNeto)
	}
	if !prestamos.Equal(dec("50000.00")) {
		t.Errorf("prestamos = %s, esperado 50000.00", prestamos)
	}
	if !netoPagado.Equal(dec("1223410.00")) {
		t.Errorf("netoPagado = %s, esperado 1223410.00", netoPagado)
	}

	if !totalIngresos.Sub(totalDeducciones).Equal(netoPagado) {
		t.Errorf("no se cumple el balance: TotalIngresos(%s) - TotalDeducciones(%s) != NetoPagado(%s)",
			totalIngresos, totalDeducciones, netoPagado)
	}
}

// TestCalcularPrestamos_NoDescuentaMasQueElSaldoPendiente cubre el caso de
// la ultima cuota de un prestamo, donde el saldo pendiente es menor que la
// cuota periodica pactada.
func TestCalcularPrestamos_NoDescuentaMasQueElSaldoPendiente(t *testing.T) {
	total := CalcularPrestamos([]PrestamoDeduccionInput{
		{PrestamoID: 1, CuotaPeriodica: dec("50000.00"), SaldoPendiente: dec("200000.00")},
		{PrestamoID: 2, CuotaPeriodica: dec("30000.00"), SaldoPendiente: dec("12500.00")},
	})
	esperado := dec("62500.00")
	if !total.Equal(esperado) {
		t.Errorf("CalcularPrestamos = %s, esperado %s", total, esperado)
	}
}
