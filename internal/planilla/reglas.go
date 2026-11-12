// Contrato de las reglas de cálculo del motor.
//
// Cada valor de expresion_formula.tipo (sembrado en concepto_salarial_formula)
// tiene su propia Regla en un archivo regla_*.go. MotorEmpleado solo resuelve
// los conceptos aplicables, despacha cada uno a su regla y totaliza; las
// reglas legales nuevas se agregan o modifican tocando únicamente su archivo.
package planilla

import "github.com/shopspring/decimal"

// Valores de expresion_formula.tipo sembrados en concepto_salarial_formula.
const (
	formulaReferenciaDirecta = "referencia_directa"
	formulaHorasExtra        = "horas_extra"
	formulaIncapacidad       = "incapacidad"
	formulaAguinaldo         = "aguinaldo"
	formulaSalarioEscolar    = "salario_escolar"
	formulaVacaciones        = "vacaciones"
	formulaCargaSocial       = "carga_social"
	formulaRenta             = "renta"
	formulaEmbargo           = "embargo"
	formulaPrestamo          = "prestamo"
)

// ContextoRegla agrupa todo lo que una regla puede leer para calcular un
// concepto de un empleado. Lo único mutable es St (estado acumulado del
// cálculo del empleado: líneas, totales, cuotas ya resueltas).
type ContextoRegla struct {
	Concepto ConceptoAplicable
	Contrato ContratoVigente
	In       CalculoEmpleadoInput
	P        ParametrosCalculo
	Manuales map[int64]decimal.Decimal
	Factores map[int64]ParametroHoraExtra
	St       *estadoCalculo
}

// Regla calcula un concepto con tipo_calculo = formula.
type Regla interface {
	Aplicar(rc *ContextoRegla) error
}

// reglasPorDefecto: clave = valor de expresion_formula.tipo. Un tipo que no
// esté aquí produce error en el motor (nunca se ignora en silencio).
func reglasPorDefecto() map[string]Regla {
	return map[string]Regla{
		formulaReferenciaDirecta: reglaSalarioBase{},
		formulaHorasExtra:        reglaHorasExtra{},
		formulaIncapacidad:       reglaIncapacidad{},
		formulaAguinaldo:         reglaDiferida{},
		formulaSalarioEscolar:    reglaDiferida{},
		formulaVacaciones:        reglaDiferida{},
		formulaCargaSocial:       reglaCCSS{},
		formulaRenta:             reglaRenta{},
		formulaEmbargo:           reglaEmbargo{},
		formulaPrestamo:          reglaPrestamo{},
	}
}
