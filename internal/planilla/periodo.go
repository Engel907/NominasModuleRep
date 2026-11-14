// Package planilla implementa el motor de cálculo de planilla y sus exportaciones.
package planilla

import (
	"context"
	"fmt"

	"planillas-ccdr/internal/db/dto"
)

// ErrorCiclo es un error de dominio con código estable y mensaje seguro
// (apto para devolver al cliente) y el estado HTTP que le corresponde.
type ErrorCiclo struct {
	Codigo  string
	Mensaje string
	HTTP    int
}

func (e *ErrorCiclo) Error() string { return e.Mensaje }

// Is permite errors.Is(err, ErrEstadoInvalido) aunque el mensaje varíe.
func (e *ErrorCiclo) Is(target error) bool {
	o, ok := target.(*ErrorCiclo)
	return ok && o.Codigo == e.Codigo
}

var (
	ErrPeriodoNoEncontrado      = &ErrorCiclo{"periodo_no_encontrado", "el período de planilla no existe", 404}
	ErrFechasInvalidas          = &ErrorCiclo{"fechas_invalidas", "las fechas del período no son válidas (fecha_fin debe ser >= fecha_inicio)", 400}
	ErrTipoPlanillaNoEncontrado = &ErrorCiclo{"tipo_planilla_no_encontrado", "el tipo de planilla no existe", 422}
	ErrPeriodoTraslapado        = &ErrorCiclo{"periodo_traslapado", "las fechas se traslapan con otro período del mismo tipo de planilla", 409}
	ErrEstadoInvalido           = &ErrorCiclo{"estado_periodo_invalido", "el período no está en el estado requerido para esta operación", 409}
	ErrTransicionInvalida       = &ErrorCiclo{"transicion_invalida", "la transición de estado no está permitida", 409}
	ErrPeriodoInmutable         = &ErrorCiclo{"periodo_inmutable", "el período ya no admite cambios", 409}
	ErrSegregacionFunciones     = &ErrorCiclo{"segregacion_funciones", "segregación de funciones: esta acción no puede hacerla la misma persona que participó en un paso incompatible del período", 403}
	ErrCalculoNoDisponible      = &ErrorCiclo{"calculo_no_disponible", "el cálculo de planilla aún no está disponible", 501}
	ErrPeriodoSinDetalle        = &ErrorCiclo{"periodo_sin_detalle", "el período no tiene líneas de planilla calculadas", 422}
	ErrDetalleDescuadrado       = &ErrorCiclo{"detalle_descuadrado", "el detalle de la planilla no cuadra con los totales de cada empleado o con la cuota patronal; recalcule el período", 409}
)

func estadoInvalido(esperado EstadoPeriodo, actual string) error {
	return &ErrorCiclo{
		Codigo:  ErrEstadoInvalido.Codigo,
		Mensaje: fmt.Sprintf("el período está en estado %q y la operación requiere %q", actual, esperado),
		HTTP:    ErrEstadoInvalido.HTTP,
	}
}

var transicionesPermitidas = map[EstadoPeriodo][]EstadoPeriodo{
	EstadoBorrador:  {EstadoCalculada},
	EstadoCalculada: {EstadoBorrador, EstadoAprobada},
	EstadoAprobada:  {EstadoPagada},
	EstadoPagada:    {EstadoContabilizada},
}

// TransicionPermitida indica si el período puede pasar de `desde` a `hasta`.
func TransicionPermitida(desde, hasta EstadoPeriodo) bool {
	for _, h := range transicionesPermitidas[desde] {
		if h == hasta {
			return true
		}
	}
	return false
}

// Participantes son los usuarios que ya intervinieron en el período (nil =
// aún no hubo ese paso).
type Participantes struct {
	Preparo *int64
	Calculo *int64
	Aprobo  *int64
	Pago    *int64
}

// ParticipantesDe extrae los participantes de una fila de período.
func ParticipantesDe(p dto.PeriodoCicloRow) Participantes {
	return Participantes{
		Preparo: p.PreparadoPorUsuarioID,
		Calculo: p.CalculadoPorUsuarioID,
		Aprobo:  p.AprobadoPorUsuarioID,
		Pago:    p.PagadoPorUsuarioID,
	}
}

// VerificarSegregacion aplica la regla: quien prepara/calcula != quien
// aprueba != quien paga; quien contabiliza != quien pagó. `destino` es el
// estado al que se quiere llegar y usuarioID quien lo intenta.
// Debe usarla para EstadoContabilizada.
func VerificarSegregacion(destino EstadoPeriodo, usuarioID int64, p Participantes) error {
	var previos []*int64
	switch destino {
	case EstadoAprobada:
		previos = []*int64{p.Preparo, p.Calculo}
	case EstadoPagada:
		previos = []*int64{p.Preparo, p.Calculo, p.Aprobo}
	case EstadoContabilizada:
		previos = []*int64{p.Pago}
	}
	for _, id := range previos {
		if id != nil && *id == usuarioID {
			return ErrSegregacionFunciones
		}
	}
	return nil
}

// Calculador lo implementa. Debe insertar las líneas de detalle del
// período (que está en borrador); el servicio hace después la transición a
// calculada.
type Calculador interface {
	Calcular(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow, usuarioID int64) error
}

// EliminadorDetalle lo implementa borra planilla_detalle_concepto y
// planilla_detalle_empleado del período (que está en calculada).
type EliminadorDetalle interface {
	EliminarDetalle(ctx context.Context, q *dto.Queries, periodoID int64) error
}

// GanchoAlAprobar recibe el período ya aprobado (préstamos,
// aguinaldo/salario escolar, vacaciones).
type GanchoAlAprobar interface {
	AlAprobar(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow) error
}

// GanchosAlAprobar ejecuta varios ganchos en orden y corta en el primer
// error (la transacción de la aprobación se revierte completa).
type GanchosAlAprobar []GanchoAlAprobar

func (gs GanchosAlAprobar) AlAprobar(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow) error {
	for _, g := range gs {
		if g == nil {
			continue
		}
		if err := g.AlAprobar(ctx, q, periodo); err != nil {
			return err
		}
	}
	return nil
}

// GanchoAlPagar lo implementa. Recibe el período ya pagado.
type GanchoAlPagar interface {
	AlPagar(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow) error
}

// PeriodoGanchos agrupa los ganchos; cualquiera puede ser nil.
type PeriodoGanchos struct {
	Calculador Calculador
	Eliminador EliminadorDetalle
	AlAprobar  GanchoAlAprobar
	AlPagar    GanchoAlPagar
}
