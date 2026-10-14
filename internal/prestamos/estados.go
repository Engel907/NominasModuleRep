package prestamos

// Estados de las tablas prestamos y embargos. Los valores deben coincidir con
// el ENUM de la columna estado en la BD: lo verifica
// internal/planilla/estados_enum_test.go (integración).

const (
	EstadoPrestamoActivo     = "activo"
	EstadoPrestamoSuspendido = "suspendido"
	EstadoPrestamoCancelado  = "cancelado"
	// ⚠️ Confirmar contra el ENUM real de prestamos.estado: es el estado al que
	// CerrarPrestamoSiSaldoEnCero deja el préstamo. Si el test de ENUM reporta
	// diferencias, ajusta o elimina esta constante.
	EstadoPrestamoPagado = "pagado"
)

const (
	EstadoEmbargoActivo     = "activo"
	EstadoEmbargoSuspendido = "suspendido"
	EstadoEmbargoFinalizado = "finalizado"
)