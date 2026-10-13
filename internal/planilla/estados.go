// Package planilla — estados.go define los estados del período de planilla
// como tipo propio, para no usar strings sueltos en SQL ni en servicios.
//
// Este archivo NO contiene lógica de transiciones entre estados (eso
// corresponde a la F19); solo el vocabulario y los conjuntos de estados que
// otras fases consultan.
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

// EstadosParaDispersion son los estados desde los que un período puede
// dispersarse / ya fue dispersado: aprobada, pagada.
//
// Devuelve un slice nuevo en cada llamada, para que ningún llamador pueda
// alterar el conjunto compartido.
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
//
//	in, args := planilla.ClausulaIN(planilla.EstadosParaAcumulados())
//	query := "SELECT ... FROM periodos WHERE estado " + in
//	rows, err := q.QueryContext(ctx, query, args...)
//
// Con una lista vacía devuelve "IN (NULL)", que no coincide con ninguna fila
// (y mantiene la sintaxis SQL válida). Solo es seguro para IN, no para
// NOT IN.
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
