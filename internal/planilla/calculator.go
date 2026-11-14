package planilla

import (
	"errors"
	"fmt"
	"sort"

	"github.com/shopspring/decimal"
)

var dosDecimales = int32(2)

// CalcularBaseCCSS suma los devengados marcados afecta_base_ccss = 1. No
// incluye deducciones: solo conceptos de tipo devengo ya
// resueltos en el desglose del empleado.
func CalcularBaseCCSS(devengos []ConceptoCalculadoOutput) decimal.Decimal {
	total := decimal.Zero
	for _, d := range devengos {
		if d.AfectaBaseCCSS {
			total = total.Add(d.Monto)
		}
	}
	return total.Round(dosDecimales)
}

// CalcularCCSS aplica la retención obrera y la cuota patronal sobre la base
// CCSS ya calculada. carga.PorcentajeObrero / PorcentajePatronal se esperan
// como fracción (ej. 0.1083 para 10.83%), tal como se persisten en
// parametros_cargas_sociales DECIMAL(6,4). La cuota patronal
// es informativa/contable — nunca reduce el neto pagado al empleado.
func CalcularCCSS(baseCCSS decimal.Decimal, carga ParametroCargaSocial) (obrero decimal.Decimal, patronal decimal.Decimal) {
	obrero = baseCCSS.Mul(carga.PorcentajeObrero).Round(dosDecimales)
	patronal = baseCCSS.Mul(carga.PorcentajePatronal).Round(dosDecimales)
	return obrero, patronal
}

// CalcularBaseRenta suma los devengados marcados afecta_base_renta = 1. La
// base del impuesto al salario es el salario BRUTO — las cargas sociales
// obreras se deducen del líquido que recibe el trabajador pero no reducen
// esta base gravable (así lo indica la guía de retención vigente para el
// Impuesto Único sobre las Rentas del Trabajo Personal Dependiente).
func CalcularBaseRenta(devengos []ConceptoCalculadoOutput) decimal.Decimal {
	total := decimal.Zero
	for _, d := range devengos {
		if d.AfectaBaseRenta {
			total = total.Add(d.Monto)
		}
	}
	return total.Round(dosDecimales)
}

// CreditoFiscalResuelto es un crédito fiscal ya resuelto a un monto total
// (Cantidad del empleado * parametros_creditos_fiscales_renta.monto_credito
// vigente).
type CreditoFiscalResuelto struct {
	TipoCreditoFiscalID int64
	MontoTotal          decimal.Decimal
}

// ResolverCreditosFiscales multiplica cada crédito solicitado por el
// empleado (ej. número de hijos, cónyuge) por el monto vigente de ese tipo
// de crédito. Retorna error explícito si falta el parámetro vigente para
// algún tipo solicitado — nunca asume monto cero.
func ResolverCreditosFiscales(creditos []CreditoFiscalInput, parametros map[int64]CreditoFiscalParametro) ([]CreditoFiscalResuelto, error) {
	resueltos := make([]CreditoFiscalResuelto, 0, len(creditos))
	for _, c := range creditos {
		param, ok := parametros[c.TipoCreditoFiscalID]
		if !ok {
			return nil, fmt.Errorf("planilla: no hay parametro vigente de credito fiscal para tipo_credito_fiscal_id=%d", c.TipoCreditoFiscalID)
		}
		monto := param.MontoCredito.Mul(decimal.NewFromInt(int64(c.Cantidad))).Round(dosDecimales)
		resueltos = append(resueltos, CreditoFiscalResuelto{
			TipoCreditoFiscalID: c.TipoCreditoFiscalID,
			MontoTotal:          monto,
		})
	}
	return resueltos, nil
}

// ResultadoImpuestoRenta separa el impuesto bruto (antes de créditos), los
// créditos fiscales solicitados vs. efectivamente aplicados y el impuesto
// neto a retener. El impuesto neto nunca queda negativo: si los créditos
// exceden el impuesto bruto, el excedente simplemente no se aplica (no es
// un crédito reembolsable en planilla).
type ResultadoImpuestoRenta struct {
	ImpuestoBruto       decimal.Decimal
	CreditosSolicitados decimal.Decimal
	CreditosAplicados   decimal.Decimal
	ImpuestoNeto        decimal.Decimal
}

// CalcularImpuestoRenta aplica la escala progresiva de renta (Decreto
// 45333-H y sus actualizaciones anuales — tramos siempre recibidos como
// parámetro vigente, nunca hardcodeados) sobre baseRenta, y resta los
// créditos fiscales ya resueltos.
func CalcularImpuestoRenta(baseRenta decimal.Decimal, tramos []TramoRenta, creditos []CreditoFiscalResuelto) (ResultadoImpuestoRenta, error) {
	if len(tramos) == 0 {
		return ResultadoImpuestoRenta{}, errors.New("planilla: no hay tramos de renta vigentes para la fecha efectiva")
	}

	ordenados := make([]TramoRenta, len(tramos))
	copy(ordenados, tramos)
	sort.Slice(ordenados, func(i, j int) bool { return ordenados[i].OrdenTramo < ordenados[j].OrdenTramo })

	impuestoBruto := decimal.Zero
	for i, tramo := range ordenados {
		if tramo.Abierto && i != len(ordenados)-1 {
			return ResultadoImpuestoRenta{}, fmt.Errorf("planilla: tramo de renta abierto (%d) no es el último", tramo.OrdenTramo)
		}
		if !tramo.Abierto && !tramo.MontoHasta.GreaterThan(tramo.MontoDesde) {
			return ResultadoImpuestoRenta{}, fmt.Errorf("planilla: tramo de renta %d tiene un límite superior inválido", tramo.OrdenTramo)
		}
		if baseRenta.LessThanOrEqual(tramo.MontoDesde) {
			continue
		}

		var excedenteHasta decimal.Decimal
		switch {
		case tramo.Abierto:
			excedenteHasta = baseRenta
		case baseRenta.LessThan(tramo.MontoHasta):
			excedenteHasta = baseRenta
		default:
			excedenteHasta = tramo.MontoHasta
		}

		excedente := excedenteHasta.Sub(tramo.MontoDesde)
		if excedente.IsNegative() {
			continue
		}
		impuestoBruto = impuestoBruto.Add(excedente.Mul(tramo.Porcentaje))
	}
	impuestoBruto = impuestoBruto.Round(dosDecimales)

	creditosSolicitados := decimal.Zero
	for _, c := range creditos {
		creditosSolicitados = creditosSolicitados.Add(c.MontoTotal)
	}
	creditosSolicitados = creditosSolicitados.Round(dosDecimales)

	creditosAplicados := creditosSolicitados
	if creditosAplicados.GreaterThan(impuestoBruto) {
		creditosAplicados = impuestoBruto
	}

	impuestoNeto := impuestoBruto.Sub(creditosAplicados)
	if impuestoNeto.IsNegative() {
		impuestoNeto = decimal.Zero
	}

	return ResultadoImpuestoRenta{
		ImpuestoBruto:       impuestoBruto,
		CreditosSolicitados: creditosSolicitados,
		CreditosAplicados:   creditosAplicados,
		ImpuestoNeto:        impuestoNeto,
	}, nil
}

// CalcularEmbargos aplica los embargos activos de un empleado sobre su
// líquido embargable (Σ ingresos con afecta_base_embargo − CCSS obrera −
// renta neta, lo calcula el llamador). Todo valor legal llega por parámetro:
func CalcularEmbargos(emb []EmbargoInput, liquidoEmbargable, salarioMinInembargable decimal.Decimal, escala []TramoEscalaEmbargo) ([]EmbargoAplicadoOutput, error) {
	if len(emb) == 0 {
		return nil, nil
	}
	if !salarioMinInembargable.IsPositive() {
		return nil, errors.New("planilla: falta SALARIO_MINIMO_INEMBARGABLE vigente para calcular embargos")
	}
	liquido := decimal.Max(liquidoEmbargable, decimal.Zero)

	ordenados := make([]EmbargoInput, len(emb))
	copy(ordenados, emb)
	sort.SliceStable(ordenados, func(i, j int) bool {
		a, b := ordenados[i], ordenados[j]
		if a.PrioridadLegal != b.PrioridadLegal {
			return a.PrioridadLegal < b.PrioridadLegal
		}
		if a.Prioridad != b.Prioridad {
			return a.Prioridad < b.Prioridad
		}
		return a.EmbargoID < b.EmbargoID
	})

	resultados := make([]EmbargoAplicadoOutput, 0, len(ordenados))
	agregar := func(e EmbargoInput, solicitado, aplicado decimal.Decimal) {
		resultados = append(resultados, EmbargoAplicadoOutput{
			EmbargoID: e.EmbargoID, Prioridad: e.Prioridad,
			MontoCalculado: solicitado, MontoAplicado: aplicado,
			SaldoPendienteInsatisfecho: solicitado.Sub(aplicado),
		})
	}

	remanente := liquido
	pensiones := decimal.Zero
	var generales []EmbargoInput
	for _, e := range ordenados {
		if !e.EsPension {
			generales = append(generales, e)
			continue
		}
		solicitado := montoSolicitadoEmbargo(e, liquido)
		aplicado := decimal.Min(solicitado, remanente)
		if e.TopePctLiquido.IsPositive() {
			disponible := liquido.Mul(e.TopePctLiquido).Round(dosDecimales).Sub(pensiones)
			aplicado = decimal.Min(aplicado, decimal.Max(disponible, decimal.Zero))
		}
		agregar(e, solicitado, aplicado)
		pensiones = pensiones.Add(aplicado)
		remanente = remanente.Sub(aplicado)
	}
	if len(generales) == 0 {
		return resultados, nil
	}

	fondo, err := fondoEmbargable(remanente, salarioMinInembargable, escala)
	if err != nil {
		return nil, err
	}
	for _, e := range generales {
		solicitado := montoSolicitadoEmbargo(e, liquido)
		aplicado := decimal.Min(solicitado, fondo)
		if e.TopePctLiquido.IsPositive() {
			aplicado = decimal.Min(aplicado, liquido.Mul(e.TopePctLiquido).Round(dosDecimales))
		}
		agregar(e, solicitado, aplicado)
		fondo = fondo.Sub(aplicado)
	}
	return resultados, nil
}

// fondoEmbargable: monto embargable del remanente según la escala. Los
// límites de cada tramo son múltiplos del mínimo inembargable y se aplican
// sobre el EXCEDENTE (remanente − mínimo). Un solo redondeo al final.
func fondoEmbargable(remanente, minimo decimal.Decimal, escala []TramoEscalaEmbargo) (decimal.Decimal, error) {
	if len(escala) == 0 {
		return decimal.Zero, errors.New("planilla: no hay parametros_embargo_escala vigentes")
	}
	tramos := make([]TramoEscalaEmbargo, len(escala))
	copy(tramos, escala)
	sort.SliceStable(tramos, func(i, j int) bool { return tramos[i].SalarioDesde.LessThan(tramos[j].SalarioDesde) })
	if !tramos[0].SalarioDesde.IsZero() {
		return decimal.Zero, fmt.Errorf("planilla: la escala de embargo debe empezar en 0 (empieza en %s)", tramos[0].SalarioDesde)
	}
	for i, t := range tramos {
		if !t.Porcentaje.IsPositive() || t.Porcentaje.GreaterThan(decimal.NewFromInt(1)) {
			return decimal.Zero, fmt.Errorf("planilla: tramo de escala de embargo desde %s con porcentaje inválido %s (fracción)", t.SalarioDesde, t.Porcentaje)
		}
		ultimo := i == len(tramos)-1
		if !t.SalarioHasta.Valid {
			if !ultimo {
				return decimal.Zero, fmt.Errorf("planilla: tramo abierto de la escala de embargo (desde %s) no es el último", t.SalarioDesde)
			}
			continue
		}
		if !ultimo && !t.SalarioHasta.Decimal.Equal(tramos[i+1].SalarioDesde) {
			return decimal.Zero, fmt.Errorf("planilla: la escala de embargo tiene un hueco o traslape entre %s y %s", t.SalarioHasta.Decimal, tramos[i+1].SalarioDesde)
		}
	}

	if !remanente.GreaterThan(minimo) {
		return decimal.Zero, nil
	}
	excedente := remanente.Sub(minimo)
	fondo := decimal.Zero
	for _, t := range tramos {
		desde := t.SalarioDesde.Mul(minimo)
		if !excedente.GreaterThan(desde) {
			break
		}
		hasta := excedente
		if t.SalarioHasta.Valid {
			hasta = decimal.Min(excedente, t.SalarioHasta.Decimal.Mul(minimo))
		}
		fondo = fondo.Add(hasta.Sub(desde).Mul(t.Porcentaje))
	}
	return decimal.Min(fondo.Round(dosDecimales), remanente), nil
}

// montoSolicitadoEmbargo: lo que pide un embargo antes de los límites
// compartidos (monto fijo, o porcentaje del líquido), topado por su
// TopeMensual si lo tiene.
func montoSolicitadoEmbargo(emb EmbargoInput, liquido decimal.Decimal) decimal.Decimal {
	var solicitado decimal.Decimal
	if emb.MontoFijo.IsPositive() {
		solicitado = emb.MontoFijo
	} else {
		solicitado = liquido.Mul(emb.Porcentaje).Round(dosDecimales)
	}
	if emb.TopeMensual.IsPositive() && solicitado.GreaterThan(emb.TopeMensual) {
		solicitado = emb.TopeMensual
	}
	return solicitado
}

// CalcularPrestamos suma las cuotas periódicas de los préstamos activos de
// un empleado, sin descontar nunca más que el saldo pendiente restante de
// cada préstamo (protege la última cuota, que suele ser parcial).
func CalcularPrestamos(prestamos []PrestamoDeduccionInput) decimal.Decimal {
	total := decimal.Zero
	for _, p := range prestamos {
		cuota := decimal.Min(p.CuotaPeriodica, p.SaldoPendiente)
		if cuota.IsNegative() {
			cuota = decimal.Zero
		}
		total = total.Add(cuota)
	}
	return total.Round(dosDecimales)
}
