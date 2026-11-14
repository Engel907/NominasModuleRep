package planilla

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
)

// ExpresionFormula es la parte de concepto_salarial_formula.expresion_formula
// que el motor interpreta; las demás claves son descriptivas.
type ExpresionFormula struct {
	Tipo  string `json:"tipo"`
	Lado  string `json:"lado"`
	Rubro string `json:"rubro"`
	// Base: sobre qué se calcula una asignacion_empleado en porcentaje.
	Base string `json:"base"`
}

// ParsearExpresionFormula exige un objeto JSON con 'tipo'.
func ParsearExpresionFormula(raw string) (ExpresionFormula, error) {
	var e ExpresionFormula
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return ExpresionFormula{}, fmt.Errorf("expresion_formula no es un objeto JSON válido: %w", err)
	}
	if e.Tipo == "" {
		return ExpresionFormula{}, errors.New("expresion_formula no define 'tipo'")
	}
	return e, nil
}

// TiposFormula: valores de 'tipo' que el motor sabe calcular.
func TiposFormula() []string {
	tipos := make([]string, 0, len(reglasPorDefecto()))
	for t := range reglasPorDefecto() {
		tipos = append(tipos, t)
	}
	sort.Strings(tipos)
	return tipos
}

// ValidarExpresionFormula comprueba que el motor pueda ejecutar la fórmula:
// 'tipo' implementado, 'lado' de una carga social y 'rubro' de liquidación
// dentro de los admitidos.
func ValidarExpresionFormula(raw string, rubros []string) error {
	e, err := ParsearExpresionFormula(raw)
	if err != nil {
		return err
	}
	if _, ok := reglasPorDefecto()[e.Tipo]; !ok {
		return fmt.Errorf("tipo de fórmula %q no implementado (admitidos: %v)", e.Tipo, TiposFormula())
	}
	switch e.Tipo {
	case formulaCargaSocial:
		if e.Lado != ladoObrero && e.Lado != ladoPatronal {
			return fmt.Errorf("una carga_social requiere 'lado' %q o %q", ladoObrero, ladoPatronal)
		}
	case formulaLiquidacion:
		if !slices.Contains(rubros, e.Rubro) {
			return fmt.Errorf("una fórmula de liquidación requiere 'rubro' en %v", rubros)
		}
	case formulaAsignacion:
		if !slices.Contains(BasesAsignacion(), e.Base) {
			return fmt.Errorf("una asignacion_empleado requiere 'base' en %v", BasesAsignacion())
		}
	}
	return nil
}
