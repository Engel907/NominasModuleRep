package planilla

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaEmbargo aplica los embargos activos sobre el líquido embargable:
// Σ ingresos con afecta_base_embargo − CCSS obrera − renta neta (lo
// deducido antes en el flujo). Escala, tope de pensión y prioridad legal
// vienen de BD. El mínimo inembargable y el tope de cada embargo son
// mensuales: en un período que cubre una fracción del mes se prorratean.
//
// En la planilla de liquidación (In.EmbargoMesPrevio) el mes es la unidad:
// los embargos se calculan sobre el líquido del mes (el de los períodos ya
// aprobados más el de la liquidación), con el mínimo y los topes mensuales
// completos, y se retiene la diferencia con lo que cada embargo ya retuvo en
// el mes (embargosDelMes).
type reglaEmbargo struct{}

// ClaveLiqEmbargoMontoFijo: parametros_generales (texto). Qué pide en la
// planilla de liquidación un embargo de monto fijo:
//   - mensual: el monto fijo es el del mes; la liquidación solo cubre lo que
//     falte por retener en el mes;
//   - por_periodo: la liquidación es un período más y vuelve a pedir el monto
//     fijo, dentro de los límites del mes.
//
// Se exige solo si hay un embargo de monto fijo en la liquidación; no se
// siembra.
const ClaveLiqEmbargoMontoFijo = "LIQ_EMBARGO_MONTO_FIJO"

const (
	LiqEmbargoMontoFijoMensual    = "mensual"
	LiqEmbargoMontoFijoPorPeriodo = "por_periodo"
)

// ModoEmbargoMontoFijoValido: mensual | por_periodo.
func ModoEmbargoMontoFijoValido(m string) bool {
	return m == LiqEmbargoMontoFijoMensual || m == LiqEmbargoMontoFijoPorPeriodo
}

// liquidoEmbargable: Σ ingresos con afecta_base_embargo − CCSS obrera − renta
// neta, con lo calculado hasta el momento.
func liquidoEmbargable(st *estadoCalculo) decimal.Decimal {
	embargable := decimal.Zero
	for _, l := range st.ingresos {
		if l.AfectaBaseEmbargo {
			embargable = embargable.Add(l.Monto)
		}
	}
	return embargable.Sub(st.ccssObrero).Sub(st.rentaNeta)
}

func (reglaEmbargo) Aplicar(rc *ContextoRegla) error {
	st := rc.St
	st.usados[formulaEmbargo] = true
	if len(rc.In.Embargos) == 0 {
		return nil
	}
	liquido := liquidoEmbargable(st)
	var (
		aplicados []EmbargoAplicadoOutput
		err       error
	)
	if prev := rc.In.EmbargoMesPrevio; prev != nil {
		aplicados, err = embargosDelMes(rc.In.Embargos, liquido, *prev, rc.P)
	} else {
		embargos, minimo := rc.In.Embargos, rc.P.SalarioMinimoInembargable
		if f := rc.P.FraccionMensualPeriodo; f.IsPositive() && f.LessThan(decimal.NewFromInt(1)) {
			minimo = minimo.Mul(f).Round(dosDecimales)
			embargos = make([]EmbargoInput, len(rc.In.Embargos))
			for i, e := range rc.In.Embargos {
				if e.TopeMensual.IsPositive() {
					e.TopeMensual = e.TopeMensual.Mul(f).Round(dosDecimales)
				}
				embargos[i] = e
			}
		}
		aplicados, err = CalcularEmbargos(embargos, liquido, minimo, rc.P.EscalaEmbargo)
	}
	if err != nil {
		return err
	}
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

// embargosDelMes: embargos de la liquidación calculados sobre el mes. Por
// embargo: pedido = lo que pide el mes − lo ya retenido; retenido = lo que el
// mes permite retener − lo ya retenido, sin pasar del pedido ni del líquido de
// la liquidación (se reparte en orden de prioridad).
func embargosDelMes(emb []EmbargoInput, liquido decimal.Decimal, prev EmbargoMesPrevio, p ParametrosCalculo) ([]EmbargoAplicadoOutput, error) {
	mes := make([]EmbargoInput, len(emb))
	for i, e := range emb {
		if e.MontoFijo.IsPositive() {
			switch p.LiqEmbargoMontoFijo {
			case LiqEmbargoMontoFijoMensual:
			case LiqEmbargoMontoFijoPorPeriodo:
				e.MontoFijo = e.MontoFijo.Add(prev.PorEmbargo[e.EmbargoID].Calculado)
			case "":
				return nil, fmt.Errorf("%w: %s (el embargo_id=%d es de monto fijo; mensual | por_periodo)",
					ErrParametroFaltante, ClaveLiqEmbargoMontoFijo, e.EmbargoID)
			default:
				return nil, fmt.Errorf("planilla: %s = %q inválido (mensual | por_periodo)", ClaveLiqEmbargoMontoFijo, p.LiqEmbargoMontoFijo)
			}
		}
		mes[i] = e
	}
	delMes, err := CalcularEmbargos(mes, prev.Base.Add(liquido), p.SalarioMinimoInembargable, p.EscalaEmbargo)
	if err != nil {
		return nil, err
	}
	disponible := decimal.Max(liquido, decimal.Zero)
	out := make([]EmbargoAplicadoOutput, 0, len(delMes))
	for _, r := range delMes {
		ya := prev.PorEmbargo[r.EmbargoID].Aplicado
		pedido := decimal.Max(r.MontoCalculado.Sub(ya), decimal.Zero)
		retenido := decimal.Min(decimal.Max(r.MontoAplicado.Sub(ya), decimal.Zero), pedido)
		retenido = decimal.Min(retenido, disponible)
		disponible = disponible.Sub(retenido)
		out = append(out, EmbargoAplicadoOutput{
			EmbargoID: r.EmbargoID, Prioridad: r.Prioridad,
			MontoCalculado: pedido, MontoAplicado: retenido,
			SaldoPendienteInsatisfecho: pedido.Sub(retenido),
		})
	}
	return out, nil
}
