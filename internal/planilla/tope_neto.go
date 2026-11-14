package planilla

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// AplicarTopeNeto se ejecuta tras evaluar todos los conceptos:
func AplicarTopeNeto(st *estadoCalculo) error {
	obligatorias := decimal.Zero
	for _, l := range st.lineas {
		if l.Naturaleza == naturalezaDeduccion && l.EsDeduccionObligatoria {
			obligatorias = obligatorias.Add(l.Monto)
		}
	}
	if obligatorias.GreaterThan(st.totalIng) {
		return fmt.Errorf("las deducciones obligatorias (%s) superan los ingresos (%s): no se puede pagar un neto negativo",
			obligatorias.StringFixed(dosDecimales), st.totalIng.StringFixed(dosDecimales))
	}

	remanente := st.totalIng.Sub(obligatorias)
	totalDed := obligatorias
	lineas := make([]ConceptoCalculadoOutput, 0, len(st.lineas))
	for _, l := range st.lineas {
		if l.Naturaleza != naturalezaDeduccion || l.EsDeduccionObligatoria {
			lineas = append(lineas, l)
			continue
		}
		aplicado := decimal.Min(l.Monto, remanente)
		if aplicado.LessThan(l.Monto) {
			st.res.Advertencias = append(st.res.Advertencias, fmt.Sprintf(
				"deducción voluntaria %s reducida de %s a %s por tope de neto",
				l.Codigo, l.Monto.StringFixed(dosDecimales), aplicado.StringFixed(dosDecimales)))
		}
		if st.conceptoPrestamoID != 0 && l.ConceptoSalarialID == st.conceptoPrestamoID {
			st.repartirPrestamos(aplicado)
		}
		remanente = remanente.Sub(aplicado)
		totalDed = totalDed.Add(aplicado)
		if aplicado.IsPositive() {
			l.Monto = aplicado
			lineas = append(lineas, l)
		}
	}
	st.lineas = lineas
	st.totalDed = totalDed
	return nil
}

// repartirPrestamos retiene `total` entre los préstamos en su orden: cada uno
// hasta su cuota; los últimos absorben la reducción.
func (s *estadoCalculo) repartirPrestamos(total decimal.Decimal) {
	disponible := total
	for i := range s.prestamos {
		ret := decimal.Min(s.prestamos[i].CuotaCalculada, disponible)
		s.prestamos[i].MontoRetenido = ret
		disponible = disponible.Sub(ret)
	}
	s.res.DeduccionesPrestamos = total
}
