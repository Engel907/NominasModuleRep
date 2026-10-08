package contabilidad

// FiltroAsientos son los criterios de consulta de asientos ya generados.
// Vive aquí (y no en el paquete de handlers) para que los adaptadores de
// lectura del propio paquete puedan implementar la interfaz del handler sin
// crear un ciclo de imports.
type FiltroAsientos struct {
	PeriodoID  *int64
	TipoCodigo string // tipos_asiento.codigo; vacío = todos los tipos
}