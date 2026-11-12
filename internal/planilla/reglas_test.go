package planilla

import "testing"

// Cada expresion_formula.tipo sembrado debe tener su Regla registrada; un
// tipo desconocido debe producir error en el motor.
func TestReglasPorDefectoCubreTodosLosTipos(t *testing.T) {
	r := reglasPorDefecto()
	tipos := []string{
		formulaReferenciaDirecta, formulaHorasExtra, formulaIncapacidad,
		formulaAguinaldo, formulaSalarioEscolar, formulaVacaciones,
		formulaCargaSocial, formulaRenta, formulaEmbargo, formulaPrestamo,
	}
	for _, tipo := range tipos {
		if r[tipo] == nil {
			t.Errorf("falta la regla para expresion_formula.tipo=%q", tipo)
		}
	}
	if len(r) != len(tipos) {
		t.Errorf("reglasPorDefecto tiene %d reglas, se esperaban %d", len(r), len(tipos))
	}
}

func TestCalcularConceptoTipoDesconocido(t *testing.T) {
	m := NewMotorEmpleado(nil)
	st := &estadoCalculo{formulas: map[int64]int64{}, res: &ResultadoEmpleadoOutput{}, usados: map[string]bool{}}
	c := ConceptoAplicable{TipoCalculo: calculoFormula, TipoFormula: "inexistente"}
	if err := m.calcularConcepto(c, ContratoVigente{}, CalculoEmpleadoInput{}, ParametrosCalculo{}, nil, nil, st); err == nil {
		t.Fatal("se esperaba error por tipo de fórmula desconocido")
	}
}
