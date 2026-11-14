package prestamos

const (
	EstadoPrestamoActivo     = "activo"
	EstadoPrestamoSuspendido = "suspendido"
	EstadoPrestamoCancelado  = "cancelado"
	// EstadoPrestamoPagado: saldo llevado a 0 (planilla o abono). Lo fija
	// trg_prestamos_saldado_bu (000131); el test de ENUM lo confirma.
	EstadoPrestamoPagado = "pagado"
)

const (
	EstadoEmbargoActivo     = "activo"
	EstadoEmbargoSuspendido = "suspendido"
	EstadoEmbargoFinalizado = "finalizado"
)
