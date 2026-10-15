package planilla

import (
	"testing"

	"github.com/shopspring/decimal"
)

// dec es un atajo para construir decimal.Decimal desde un literal string en
// los tests, evitando por completo decimal.NewFromFloat (que arrastra
// imprecisión binaria) tal como exige ADR-000 §4.
func dec(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

// tramosRenta2026 reproduce la escala mensual vigente desde el 1° de enero
// de 2026 según el Decreto Ejecutivo N° 45333-H (Gaceta N° 229 del 5 de
// diciembre de 2025): exento hasta ¢918.000; 10% de ¢918.000 a ¢1.347.000;
// 15% de ¢1.347.000 a ¢2.364.000; 20% de ¢2.364.000 a ¢4.727.000; 25%
// sobre el exceso de ¢4.727.000. En producción estos tramos SIEMPRE se
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

// cargaSocial2026 reproduce el % obrero/patronal confirmado por el equipo
// para 2026 (10.83% obrero / 26.83% patronal).
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

// ============================================================================
// a. Salario en tramo exento (¢918.000,00)
// ============================================================================

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

// ============================================================================
// b. Tramos 10%, 15%, 20% y 25% con créditos de cónyuge e hijos
// ============================================================================

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
				{TipoCreditoFiscalID: 1, MontoTotal: dec("3420.00")}, // 2 hijos x 1710
				{TipoCreditoFiscalID: 2, MontoTotal: dec("2590.00")}, // 1 conyuge
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

// ============================================================================
// c. Retención de embargo por pensión alimentaria respetando la
//    inembargabilidad de ley (Art. 172 Código de Trabajo)
// ============================================================================

func TestCalcularEmbargos(t *testing.T) {
	smi := dec(salarioMinimoInembargable2026)

	t.Run("embargo general queda en cero si el salario liquido no supera el minimo inembargable", func(t *testing.T) {
		embargos := []EmbargoInput{
			{EmbargoID: 1, TipoEmbargoID: 2, Prioridad: 2, Porcentaje: dec("0.20")}, // judicial
		}
		resultado := CalcularEmbargos(embargos, dec("200000"), smi)
		if len(resultado) != 1 {
			t.Fatalf("se esperaba 1 resultado, se obtuvieron %d", len(resultado))
		}
		if !resultado[0].MontoAplicado.Equal(dec("0.00")) {
			t.Errorf("MontoAplicado = %s, esperado 0.00 (salario por debajo del minimo inembargable)", resultado[0].MontoAplicado)
		}
		if resultado[0].SaldoPendienteInsatisfecho.LessThanOrEqual(decimal.Zero) {
			t.Errorf("SaldoPendienteInsatisfecho = %s, se esperaba > 0 (embargo no satisfecho)", resultado[0].SaldoPendienteInsatisfecho)
		}
	})

	t.Run("pension alimentaria se practica siempre y respeta su propio tope del 50%", func(t *testing.T) {
		embargos := []EmbargoInput{
			{EmbargoID: 2, TipoEmbargoID: 1, Prioridad: PrioridadPensionAlimentaria, Porcentaje: dec("0.60")},
		}
		resultado := CalcularEmbargos(embargos, dec("800000"), smi)
		if len(resultado) != 1 {
			t.Fatalf("se esperaba 1 resultado, se obtuvieron %d", len(resultado))
		}
		esperado := dec("400000.00") // 50% de 800000, tope legal, aunque se solicito 60%
		if !resultado[0].MontoAplicado.Equal(esperado) {
			t.Errorf("MontoAplicado = %s, esperado %s (tope del 50%%)", resultado[0].MontoAplicado, esperado)
		}
	})

	t.Run("pension alimentaria tiene prioridad y el embargo general solo toma el fondo Art172 del remanente", func(t *testing.T) {
		embargos := []EmbargoInput{
			{EmbargoID: 3, TipoEmbargoID: 2, Prioridad: 2, Porcentaje: dec("0.50")}, // judicial, pide mucho mas de lo disponible
			{EmbargoID: 4, TipoEmbargoID: 1, Prioridad: PrioridadPensionAlimentaria, Porcentaje: dec("0.30")},
		}
		salarioLiquido := dec("1000000")
		resultado := CalcularEmbargos(embargos, salarioLiquido, smi)
		if len(resultado) != 2 {
			t.Fatalf("se esperaban 2 resultados, se obtuvieron %d", len(resultado))
		}

		var pension, judicial EmbargoAplicadoOutput
		for _, r := range resultado {
			if r.Prioridad == PrioridadPensionAlimentaria {
				pension = r
			} else {
				judicial = r
			}
		}

		if !pension.MontoAplicado.Equal(dec("300000.00")) {
			t.Errorf("pension MontoAplicado = %s, esperado 300000.00", pension.MontoAplicado)
		}

		// Remanente tras la pension: 1,000,000 - 300,000 = 700,000
		// Excedente sobre el minimo inembargable: 700,000 - 268,711.27 = 431,288.73
		// 431,288.73 < 3 x 268,711.27 (806,133.81) -> solo aplica la fraccion de 1/8
		// Fondo = 431,288.73 * 0.125 = 53,911.09 (redondeado)
		fondoEsperado := dec("53911.09")
		if !judicial.MontoAplicado.Equal(fondoEsperado) {
			t.Errorf("judicial MontoAplicado = %s, esperado %s", judicial.MontoAplicado, fondoEsperado)
		}
		if judicial.SaldoPendienteInsatisfecho.IsNegative() {
			t.Errorf("SaldoPendienteInsatisfecho no debe ser negativo, fue %s", judicial.SaldoPendienteInsatisfecho)
		}
		// El embargo judicial solicito 500,000 (50% de 1,000,000); el fondo
		// disponible es muchisimo menor, por lo que debe quedar insatisfecho.
		if !judicial.SaldoPendienteInsatisfecho.Equal(dec("446088.91")) {
			t.Errorf("SaldoPendienteInsatisfecho = %s, esperado 446088.91", judicial.SaldoPendienteInsatisfecho)
		}
	})
}

// ============================================================================
// d. Verificacion de balance: Total Ingresos - Total Deducciones == Neto Pagado
// ============================================================================

func TestBalanceIngresosDeduccionesNeto(t *testing.T) {
	// Escenario: salario bruto de 1,500,000, un credito fiscal por 1 hijo,
	// un prestamo activo con cuota periodica y saldo suficiente. Sin
	// embargos para aislar la identidad aritmetica del balance.
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
	_ = cuotaPatronal // informativo/contable, no participa del neto del empleado

	creditos := []CreditoFiscalResuelto{
		{TipoCreditoFiscalID: 1, MontoTotal: dec("1710.00")}, // 1 hijo
	}
	resultadoRenta, err := CalcularImpuestoRenta(baseRenta, tramosRenta2026(), creditos)
	if err != nil {
		t.Fatalf("error inesperado calculando renta: %v", err)
	}

	prestamos := CalcularPrestamos([]PrestamoDeduccionInput{
		{PrestamoID: 1, CuotaPeriodica: dec("50000.00"), SaldoPendiente: dec("200000.00")},
	})

	totalIngresos := baseCCSS // unico devengo del escenario
	totalDeducciones := cuotaObrero.Add(resultadoRenta.ImpuestoNeto).Add(prestamos)
	netoPagado := totalIngresos.Sub(totalDeducciones)

	// Valores esperados calculados a mano para el escenario:
	//   CCSS obrero   = 1,500,000 * 0.1083           = 162,450.00
	//   Renta bruta   = 42,900 (tramo 10%) + 22,950 (tramo 15%) = 65,850.00
	//   Renta neta    = 65,850.00 - 1,710.00 (credito) = 64,140.00
	//   Prestamo      = 50,000.00
	//   Deducciones   = 162,450.00 + 64,140.00 + 50,000.00 = 276,590.00
	//   Neto pagado   = 1,500,000.00 - 276,590.00 = 1,223,410.00
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

	// La identidad que exige el entregable d: Ingresos - Deducciones == Neto.
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
		{PrestamoID: 2, CuotaPeriodica: dec("30000.00"), SaldoPendiente: dec("12500.00")}, // ultima cuota, parcial
	})
	esperado := dec("62500.00") // 50000 + 12500 (no 30000)
	if !total.Equal(esperado) {
		t.Errorf("CalcularPrestamos = %s, esperado %s", total, esperado)
	}
}
