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
	return rc.St.resolverCCSS(rc.Concepto, rc.In, rc.P)
}

// BaseCCSSUsada aplica la base mínima contributiva:
func BaseCCSSUsada(baseReal decimal.Decimal, regimenCCSSID int64, p ParametrosCalculo) (decimal.Decimal, error) {
	if p.SinBaseMinimaCCSS {
		return baseReal, nil
	}
	if regimenCCSSID <= 0 {
		return decimal.Zero, errors.New("el empleado no tiene regimen_ccss_id: no se puede resolver la base mínima contributiva")
	}
	minima, ok := p.BaseMinimaContributiva[regimenCCSSID]
	if !ok || !minima.IsPositive() {
		return decimal.Zero, fmt.Errorf("no hay parametros_base_minima_contributiva vigente para regimen_ccss_id=%d", regimenCCSSID)
	}
	if !p.FraccionMensualPeriodo.IsPositive() {
		return decimal.Zero, errors.New("FraccionMensualPeriodo no definida: no se puede prorratear la base mínima contributiva")
	}
	minimaPeriodo := minima.Mul(p.FraccionMensualPeriodo).Round(dosDecimales)
	if baseReal.LessThan(minimaPeriodo) {
		return minimaPeriodo, nil
	}
	return baseReal, nil
}

// resolverCCSS calcula la base CCSS usada (real o mínima) una sola vez y, con
// ella, la cuota del concepto según su 'lado' y su tasa vigente para el
// régimen CCSS del empleado. Cada componente (SEM, IVM, Banco Popular, …)
// puede ser un concepto propio o uno solo agregado: el motor no distingue.
// La cuota obrera es una deducción del neto; la patronal es contable y no
// genera línea en el desglose (la boleta no la muestra ni resta del neto).
func (s *estadoCalculo) resolverCCSS(c ConceptoAplicable, in CalculoEmpleadoInput, p ParametrosCalculo) error {
	tasa, ok := p.TasaCarga(c.ConceptoSalarialID, in.RegimenCCSSID)
	if !ok {
		return fmt.Errorf("no hay parametros_cargas_sociales vigente para el concepto %q (ni general ni del regimen_ccss_id=%d)", c.Codigo, in.RegimenCCSSID)
	}
	if !s.ccssHecho {
		base, err := BaseCCSSUsada(CalcularBaseCCSS(s.ingresos), in.RegimenCCSSID, p)
		if err != nil {
			return err
		}
		s.baseCCSS = base
		s.ccssHecho = true
	}
	obrero, patronal := CalcularCCSS(s.baseCCSS, tasa)
	switch c.Lado {
	case ladoObrero:
		s.ccssObrero = s.ccssObrero.Add(obrero)
		if obrero.IsPositive() {
			s.agregar(c, obrero, decimal.Zero)
		}
	case ladoPatronal:
		s.ccssPatronal = s.ccssPatronal.Add(patronal)
		s.patronal = append(s.patronal, LineaPatronal{
			ConceptoSalarialID: c.ConceptoSalarialID,
			Base:               s.baseCCSS,
			Porcentaje:         tasa.PorcentajePatronal,
			Monto:              patronal,
		})
	default:
		return fmt.Errorf("carga_social con lado desconocido %q", c.Lado)
	}
	return nil
}
