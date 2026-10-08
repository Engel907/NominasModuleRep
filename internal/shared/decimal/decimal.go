// Package decimal contiene helpers puntuales de conversión sobre
// shopspring/decimal que no encajan en ningún dominio específico.
//
// No confundir con github.com/shopspring/decimal (el paquete de terceros):
// este paquete es un wrapper delgado propio del proyecto.
package decimal

import "github.com/shopspring/decimal"

// ToNumeric convierte un decimal.Decimal al tipo numérico que sqlc genera
// por defecto para columnas DECIMAL que NO están en la lista de overrides
// de sqlc.yaml (ver ADR-000 sección 4: solo las columnas monetarias listadas
// ahí se mapean a decimal.Decimal en la capa de DB; columnas de horas/días
// como registro_asistencia_detalle.horas_* quedan en el tipo default que
// sqlc usa para MySQL DECIMAL, que es float64).
//
// Se centraliza acá para que, si más adelante se decide agregar overrides
// también para esas columnas de horas/días (el ADR lo sugiere como posible
// mejora, sección 4, último párrafo), baste con cambiar esta función (o
// eliminarla y pasar decimal.Decimal end-to-end) en vez de tocar cada
// lugar donde se arma un Params de sqlc.
func ToNumeric(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}
