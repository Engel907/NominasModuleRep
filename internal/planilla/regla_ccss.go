package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// Valores de expresion_formula.lado para carga_social.
const (
	ladoObrero   = "obrero"
	ladoPatronal = "patronal"
)

// reglaCCSS (carga_social): cuotas obrera y patronal sobre la base CCSS de
// los ingresos ya calculados en el flujo.
type reglaCCSS struct{}

func (reglaCCSS) Aplicar(rc *ContextoRegla) error {
	return rc.St.resolverCCSS(rc.Concepto, rc.P)
}

// resolverCCSS calcula obrero y patronal una sola vez sobre la base CCSS de
// los ingresos ya calculados. La cuota obrera es una deducción del neto; la
// patronal es informativa/contable y NO genera línea en el desglose (para
// que la boleta no la muestre como deducción del empleado ni reste del neto).
func (s *estadoCalculo) resolverCCSS(c ConceptoAplicable, p ParametrosCalculo) error {
	if p.CargaSocial.PorcentajeObrero.IsZero() && p.CargaSocial.PorcentajePatronal.IsZero() {
		return errors.New("no hay parametros_cargas_sociales vigentes (ambos porcentajes en cero)")
	}
	if !s.ccssHecho {
		s.ccssObrero, s.ccssPatronal = CalcularCCSS(CalcularBaseCCSS(s.ingresos), p.CargaSocial)
		s.ccssHecho = true
	}
	switch c.Lado {
	case ladoObrero:
		if s.ccssObrero.IsPositive() {
			s.agregar(c, s.ccssObrero, decimal.Zero)
		}
	case ladoPatronal:
		// solo informativo; ya quedó en s.ccssPatronal
	default:
		return fmt.Errorf("carga_social con lado desconocido %q", c.Lado)
	}
	return nil
}
