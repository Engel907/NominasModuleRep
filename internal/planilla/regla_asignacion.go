package planilla

import (
	"fmt"
	"slices"

	"github.com/shopspring/decimal"
)

// Bases de una asignacion_empleado en porcentaje (expresion_formula.base),
// tomadas de lo ya calculado en el flujo (orden_calculo).
const (
	baseAsignacionBruto       = "bruto"        // Σ ingresos
	baseAsignacionSalarioBase = "salario_base" // concepto de referencia_directa
	baseAsignacionCCSS        = "ccss"         // Σ ingresos con afecta_base_ccss
	baseAsignacionRenta       = "renta"        // Σ ingresos con afecta_base_renta
	baseAsignacionLiquido     = "liquido"      // Σ ingresos − Σ deducciones
)

// BasesAsignacion: valores admitidos de expresion_formula.base.
func BasesAsignacion() []string {
	return []string{baseAsignacionBruto, baseAsignacionSalarioBase, baseAsignacionCCSS, baseAsignacionRenta, baseAsignacionLiquido}
}

// reglaAsignacion (asignacion_empleado): suma las asignaciones vigentes del
// empleado para el concepto. Monto mensual × fracción del mes del período (una
// planilla sin fracción, como la de liquidación, no cobra montos mensuales);
// porcentaje × la base de la fórmula. Sirve para ingresos (pluses) y
// deducciones (asociación, sindicato, pólizas); estas se topan por neto.
type reglaAsignacion struct{}

func (reglaAsignacion) Aplicar(rc *ContextoRegla) error {
	if !slices.Contains(BasesAsignacion(), rc.Concepto.Base) {
		return fmt.Errorf("asignacion_empleado con base %q inválida (%v)", rc.Concepto.Base, BasesAsignacion())
	}
	total := decimal.Zero
	for _, a := range rc.In.Asignaciones {
		if a.ConceptoSalarialID != rc.Concepto.ConceptoSalarialID {
			continue
		}
		if !a.Valor.IsPositive() {
			return fmt.Errorf("asignación %d con valor %s (debe ser > 0)", a.AsignacionID, a.Valor)
		}
		var m decimal.Decimal
		switch a.Modo {
		case AsignacionMonto:
			m = a.Valor.Mul(rc.P.FraccionMensualPeriodo)
		case AsignacionPorcentaje:
			m = baseAsignacion(rc.Concepto.Base, rc.St).Mul(a.Valor)
		default:
			return fmt.Errorf("asignación %d con modo desconocido %q", a.AsignacionID, a.Modo)
		}
		total = total.Add(m.Round(dosDecimales))
	}
	if total.IsPositive() {
		rc.St.agregar(rc.Concepto, total, decimal.Zero)
	}
	return nil
}

func baseAsignacion(base string, st *estadoCalculo) decimal.Decimal {
	switch base {
	case baseAsignacionSalarioBase:
		return st.salarioBase
	case baseAsignacionCCSS:
		return CalcularBaseCCSS(st.ingresos)
	case baseAsignacionRenta:
		return CalcularBaseRenta(st.ingresos)
	case baseAsignacionLiquido:
		return st.totalIng.Sub(st.totalDed)
	default:
		return st.totalIng
	}
}
