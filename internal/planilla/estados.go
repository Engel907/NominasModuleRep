// Package planilla contiene el motor de cálculo de planillas, el ciclo de
// vida del período, los reportes y la exportación bancaria.
package planilla

import "strings"

// EstadoPeriodo es el estado de un período de planilla. Los valores deben
// coincidir exactamente con el ENUM de la columna de estado en la BD.
type EstadoPeriodo string

const (
	EstadoBorrador      EstadoPeriodo = "borrador"
	EstadoCalculada     EstadoPeriodo = "calculada"
	EstadoAprobada      EstadoPeriodo = "aprobada"
	EstadoPagada        EstadoPeriodo = "pagada"
	EstadoContabilizada EstadoPeriodo = "contabilizada"
)

// Valido indica si e es uno de los estados definidos.
func (e EstadoPeriodo) Valido() bool {
	switch e {
	case EstadoBorrador, EstadoCalculada, EstadoAprobada, EstadoPagada, EstadoContabilizada:
		return true
	}
	return false
}

// ClaseTipoPlanilla es la clase de un tipo de planilla. Los valores deben
// coincidir exactamente con el ENUM tipos_planilla.clase. Go decide el
// comportamiento por clase, nunca por el codigo de texto del tipo.
type ClaseTipoPlanilla string

const (
	ClaseRegular        ClaseTipoPlanilla = "regular"
	ClaseAguinaldo      ClaseTipoPlanilla = "aguinaldo"
	ClaseSalarioEscolar ClaseTipoPlanilla = "salario_escolar"
	ClaseLiquidacion    ClaseTipoPlanilla = "liquidacion"
)

// Valida indica si c es una de las clases definidas.
func (c ClaseTipoPlanilla) Valida() bool {
	switch c {
	case ClaseRegular, ClaseAguinaldo, ClaseSalarioEscolar, ClaseLiquidacion:
		return true
	}
	return false
}

// EstadosParaDispersion son los estados desde los que un período puede
// dispersarse / ya fue dispersado: aprobada, pagada.
func EstadosParaDispersion() []EstadoPeriodo {
	return []EstadoPeriodo{EstadoAprobada, EstadoPagada}
}

// EstadosParaAcumulados son los estados cuyos montos cuentan para los
// acumulados: aprobada, pagada, contabilizada.
func EstadosParaAcumulados() []EstadoPeriodo {
	return []EstadoPeriodo{EstadoAprobada, EstadoPagada, EstadoContabilizada}
}

// EstadosCerrados son los estados en que el período ya no admite cambios
// de cálculo: pagada, contabilizada.
func EstadosCerrados() []EstadoPeriodo {
	return []EstadoPeriodo{EstadoPagada, EstadoContabilizada}
}

// ClausulaIN arma el fragmento "IN (?,?,...)" y sus argumentos para usarlo
// en SQL con parámetros, p. ej.:
func ClausulaIN(estados []EstadoPeriodo) (string, []interface{}) {
	if len(estados) == 0 {
		return "IN (NULL)", nil
	}
	args := make([]interface{}, len(estados))
	for i, e := range estados {
		args[i] = string(e)
	}
	return "IN (" + strings.TrimSuffix(strings.Repeat("?,", len(estados)), ",") + ")", args
}
