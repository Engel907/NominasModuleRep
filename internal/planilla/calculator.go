package planilla

import (
	"errors"
	"fmt"
	"sort"

	"github.com/shopspring/decimal"
)

// Fase 6b — Implementación de las funciones ARITMÉTICAS PURAS del motor de
// cálculo (ver ADR-000 y el diseño de Fase 6a en domain.go / service.go).
//
// Ninguna función de este archivo hace I/O. Todos los parámetros vigentes
// (porcentajes de CCSS, tramos de renta, créditos fiscales, salario mínimo
// inembargable) se reciben como argumentos — nunca hardcodeados — para
// cumplir la directiva "sin valores por defecto hardcodeados en Go: si
// falta un parámetro vigente, debe retornar un error explícito".
//
// Excepción documentada: las FRACCIONES fijadas directamente por el
// Artículo 172 del Código de Trabajo (1/8, 1/4, 50% máximo en pensión
// alimentaria) sí se modelan como constantes de este paquete, porque son
// literales de ley que no cambian por decreto anual — a diferencia del
// salario mínimo inembargable en sí, que SÍ es un monto vigente por fecha
// y siempre llega como parámetro.

const (
	// PrioridadPensionAlimentaria referencia el mismo esquema de
	// prioridad legal usado en Fases 1-5: 1: Pensión Alimentaria,
	// 2: Judicial, 3: Hacienda, 4: CCSS.
	PrioridadPensionAlimentaria int32 = 1

	// art172MultiplicadorTramoUno: el Art. 172 del Código de Trabajo fija
	// el corte entre la fracción de 1/8 y la de 1/4 en tres veces el
	// salario mínimo inembargable, aplicado sobre el EXCEDENTE que queda
	// después de restar ese salario mínimo inembargable del remanente
	// (criterio confirmado por dictamen DAJ del MTSS, no por el salario
	// bruto total).
	art172MultiplicadorTramoUno int64 = 3
)

var (
	fraccionTramoUno                 = decimal.RequireFromString("0.125") // 1/8, Art. 172 CT
	fraccionTramoDos                 = decimal.RequireFromString("0.25")  // 1/4, Art. 172 CT
	fraccionMaximaPensionAlimentaria = decimal.RequireFromString("0.5")   // tope 50%, criterio de familia

	dosDecimales = int32(2)
)

// ============================================================================
// CalcularBaseCCSS / CalcularCCSS
// ============================================================================

// CalcularBaseCCSS suma los devengados marcados afecta_base_ccss = 1
// (ADR-000 §5). No incluye deducciones: solo conceptos de tipo devengo ya
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
// parametros_cargas_sociales DECIMAL(6,4) (ADR-000 §4). La cuota patronal
// es informativa/contable — nunca reduce el neto pagado al empleado.
func CalcularCCSS(baseCCSS decimal.Decimal, carga ParametroCargaSocial) (obrero decimal.Decimal, patronal decimal.Decimal) {
	obrero = baseCCSS.Mul(carga.PorcentajeObrero).Round(dosDecimales)
	patronal = baseCCSS.Mul(carga.PorcentajePatronal).Round(dosDecimales)
	return obrero, patronal
}

// ============================================================================
// CalcularBaseRenta / CalcularImpuestoRenta
// ============================================================================

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
// neto a retener. El impuesto neto NUNCA queda negativo: si los créditos
// exceden el impuesto bruto, el excedente simplemente no se aplica (no es
// un crédito reembolsable en planilla).
type ResultadoImpuestoRenta struct {
	ImpuestoBruto       decimal.Decimal
	CreditosSolicitados decimal.Decimal
	CreditosAplicados   decimal.Decimal
	ImpuestoNeto        decimal.Decimal
}

// CalcularImpuestoRenta aplica la escala progresiva de renta (Decreto
// 45333-H y sus actualizaciones anuales — tramos SIEMPRE recibidos como
// parámetro vigente, nunca hardcodeados) sobre baseRenta, y resta los
// créditos fiscales ya resueltos.
//
// tramos no necesita venir pre-ordenado — la función ordena por
// OrdenTramo. El techo solo se ignora cuando Abierto es true.
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
			continue // la base no alcanza este tramo
		}

		var excedenteHasta decimal.Decimal
		switch {
		case tramo.Abierto:
			// último tramo, abierto
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
		impuestoNeto = decimal.Zero // guarda de sanity, no debería ocurrir dado el cap anterior
	}

	return ResultadoImpuestoRenta{
		ImpuestoBruto:       impuestoBruto,
		CreditosSolicitados: creditosSolicitados,
		CreditosAplicados:   creditosAplicados,
		ImpuestoNeto:        impuestoNeto,
	}, nil
}

// ============================================================================
// CalcularEmbargos
// ============================================================================

// CalcularEmbargos aplica los embargos activos de un empleado sobre su
// salario líquido (bruto - CCSS obrero - impuesto de renta neto - lo que
// ya se dedujo antes en el flujo, calculado por el llamador), respetando:
//
//   - Pensión alimentaria (PrioridadPensionAlimentaria): siempre se
//     practica primero — "las retenciones por pensión alimenticia siempre
//     deben practicarse" — y puede alcanzar hasta el 50% del salario
//     líquido (criterio de derecho de familia, Ley de Pensiones
//     Alimentarias). NO está sujeta al tramo general del Art. 172 del
//     Código de Trabajo.
//   - Los demás embargos (Judicial, Hacienda, CCSS — prioridades 2 a 4):
//     se aplican sobre el remanente después de la pensión alimentaria,
//     respetando el Art. 172 del Código de Trabajo:
//     -- Hasta salarioMinimoInembargable: absolutamente inembargable.
//     -- Del excedente sobre ese mínimo, hasta 3 veces el mínimo: hasta
//     1/8 (12.5%) de esa porción es embargable.
//     -- Sobre el excedente más allá de esas 3 veces: hasta 1/4 (25%)
//     de esa porción es embargable.
//     Ese "monto embargable disponible" es un fondo COMPARTIDO que se
//     reparte entre los embargos de prioridad 2-4 en orden estricto; si no
//     alcanza para todos, los de menor prioridad quedan parcial o
//     totalmente insatisfechos (ver
//     EmbargoAplicadoOutput.SaldoPendienteInsatisfecho). La función NUNCA
//     aplica más de lo realmente disponible.
func CalcularEmbargos(embargos []EmbargoInput, salarioLiquido decimal.Decimal, salarioMinimoInembargable decimal.Decimal) []EmbargoAplicadoOutput {
	if len(embargos) == 0 {
		return nil
	}

	ordenados := make([]EmbargoInput, len(embargos))
	copy(ordenados, embargos)
	sort.Slice(ordenados, func(i, j int) bool { return ordenados[i].Prioridad < ordenados[j].Prioridad })

	resultados := make([]EmbargoAplicadoOutput, 0, len(ordenados))
	remanente := salarioLiquido

	// --- Pensión alimentaria: siempre primero, tope propio del 50% ---
	var pendientesGenerales []EmbargoInput
	for _, emb := range ordenados {
		if emb.Prioridad != PrioridadPensionAlimentaria {
			pendientesGenerales = append(pendientesGenerales, emb)
			continue
		}

		solicitado := montoSolicitadoEmbargo(emb, salarioLiquido)
		topePension := salarioLiquido.Mul(fraccionMaximaPensionAlimentaria).Round(dosDecimales)
		aplicado := decimal.Min(solicitado, topePension)
		aplicado = decimal.Min(aplicado, remanente)

		resultados = append(resultados, EmbargoAplicadoOutput{
			EmbargoID:                  emb.EmbargoID,
			Prioridad:                  emb.Prioridad,
			MontoCalculado:             solicitado,
			MontoAplicado:              aplicado,
			SaldoPendienteInsatisfecho: solicitado.Sub(aplicado),
		})
		remanente = remanente.Sub(aplicado)
	}

	if len(pendientesGenerales) == 0 {
		return resultados
	}

	// --- Fondo compartido para embargos generales, Art. 172 CT ---
	fondoDisponible := decimal.Zero
	if remanente.GreaterThan(salarioMinimoInembargable) {
		excedente := remanente.Sub(salarioMinimoInembargable)
		techoTramoUno := salarioMinimoInembargable.Mul(decimal.NewFromInt(art172MultiplicadorTramoUno))

		if excedente.LessThanOrEqual(techoTramoUno) {
			fondoDisponible = excedente.Mul(fraccionTramoUno)
		} else {
			fondoDisponible = techoTramoUno.Mul(fraccionTramoUno)
			restante := excedente.Sub(techoTramoUno)
			fondoDisponible = fondoDisponible.Add(restante.Mul(fraccionTramoDos))
		}
		fondoDisponible = fondoDisponible.Round(dosDecimales)
	}
	if fondoDisponible.GreaterThan(remanente) {
		fondoDisponible = remanente // guarda de sanity: nunca exceder lo realmente disponible
	}

	for _, emb := range pendientesGenerales {
		solicitado := montoSolicitadoEmbargo(emb, salarioLiquido)
		aplicado := decimal.Min(solicitado, fondoDisponible)

		resultados = append(resultados, EmbargoAplicadoOutput{
			EmbargoID:                  emb.EmbargoID,
			Prioridad:                  emb.Prioridad,
			MontoCalculado:             solicitado,
			MontoAplicado:              aplicado,
			SaldoPendienteInsatisfecho: solicitado.Sub(aplicado),
		})
		fondoDisponible = fondoDisponible.Sub(aplicado)
	}

	return resultados
}

// montoSolicitadoEmbargo resuelve cuánto pide un embargo ANTES de aplicar
// los límites legales compartidos: por porcentaje del salario líquido, o
// por monto fijo; y lo topa contra su propio TopeMensual si el embargo
// trae uno definido (ej. límite fijado en la resolución judicial).
func montoSolicitadoEmbargo(emb EmbargoInput, salarioLiquido decimal.Decimal) decimal.Decimal {
	var solicitado decimal.Decimal
	if emb.MontoFijo.IsPositive() {
		solicitado = emb.MontoFijo
	} else {
		solicitado = salarioLiquido.Mul(emb.Porcentaje).Round(dosDecimales)
	}
	if emb.TopeMensual.IsPositive() && solicitado.GreaterThan(emb.TopeMensual) {
		solicitado = emb.TopeMensual
	}
	return solicitado
}

// ============================================================================
// CalcularPrestamos
// ============================================================================

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
