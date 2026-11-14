package liquidacion

import "planillas-ccdr/internal/planilla"

// ValidarExpresionFormula valida una fórmula de concepto contra las reglas del
// motor y los rubros de liquidación existentes.
func ValidarExpresionFormula(expresion string) error {
	rubros := Rubros()
	nombres := make([]string, len(rubros))
	for i, r := range rubros {
		nombres[i] = string(r)
	}
	return planilla.ValidarExpresionFormula(expresion, nombres)
}
