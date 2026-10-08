// internal/db/dto/vacaciones_extra.go
//
// Fase 4 — Vacaciones: queries puntuales que la capa de servicio
// (internal/asistencia/vacaciones_service.go) necesita y que no existían en
// vacaciones.go. Mismo patrón sin sqlc: database/sql a mano, placeholders `?`.
//
//   - BloquearEmpleado: serializa TODAS las operaciones de vacaciones de un
//     mismo empleado (acumular, solicitar, aprobar, disfrutar). Es necesario
//     porque el lock FOR UPDATE sobre el corte de vacaciones_acumulado no
//     protege cuando el empleado todavía no tiene ningún corte.
//   - ContarSolicitudesVacacionesTraslapadas: verificación de traslape de
//     fechas contra solicitudes pendientes/aprobadas del mismo empleado.
//   - ContarSolicitudesVacacionesAprobadasTraslapadas: igual que la anterior pero
//     solo contra solicitudes ya 'aprobada' (validación previa a aprobar).
//   - SumarDiasVacacionesAprobadasSinDisfrutar: días ya comprometidos por
//     solicitudes aprobadas cuyo disfrute aún no se registró (no están
//     descontados de vacaciones_acumulado todavía).
//   - ExisteMovimientoVacacionesPorSolicitud / ExisteMovimientoAcumulacionVacaciones:
//     idempotencia del ledger (vacaciones_movimientos no tiene UNIQUE que
//     impida duplicar un disfrute o una acumulación de período).
package dto

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// BloquearEmpleado toma un lock de fila sobre empleados(id). Solo tiene
// sentido dentro de una Tx (WithTx / WithAuditContext). Devuelve
// sql.ErrNoRows si el empleado no existe.
func (q *Queries) BloquearEmpleado(ctx context.Context, empleadoID int64) error {
	var id int64
	return q.db.QueryRowContext(ctx, `SELECT id FROM empleados WHERE id = ? FOR UPDATE`, empleadoID).Scan(&id)
}

// ContarSolicitudesVacacionesTraslapadas cuenta las solicitudes 'pendiente' o
// 'aprobada' del empleado cuyo rango [fecha_inicio, fecha_fin] se cruza con
// [fechaInicio, fechaFin] (extremos inclusivos). excluirSolicitudID permite
// ignorar una solicitud concreta; pasar 0 para no excluir ninguna.
func (q *Queries) ContarSolicitudesVacacionesTraslapadas(ctx context.Context, empleadoID int64, fechaInicio, fechaFin time.Time, excluirSolicitudID int64) (int64, error) {
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM vacaciones_solicitudes
WHERE empleado_id = ?
  AND estado IN ('pendiente', 'aprobada')
  AND fecha_inicio <= ?
  AND fecha_fin >= ?
  AND id <> ?`, empleadoID, fechaFin, fechaInicio, excluirSolicitudID).Scan(&total)
	return total, err
}

// ContarSolicitudesVacacionesAprobadasTraslapadas cuenta las solicitudes ya
// 'aprobada' del empleado cuyo rango se cruza con [fechaInicio, fechaFin]
// (extremos inclusivos), ignorando excluirSolicitudID (0 = ninguna).
func (q *Queries) ContarSolicitudesVacacionesAprobadasTraslapadas(ctx context.Context, empleadoID int64, fechaInicio, fechaFin time.Time, excluirSolicitudID int64) (int64, error) {
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM vacaciones_solicitudes
WHERE empleado_id = ?
  AND estado = 'aprobada'
  AND fecha_inicio <= ?
  AND fecha_fin >= ?
  AND id <> ?`, empleadoID, fechaFin, fechaInicio, excluirSolicitudID).Scan(&total)
	return total, err
}

// SumarDiasVacacionesAprobadasSinDisfrutar suma dias_solicitados de las
// solicitudes 'aprobada' del empleado que todavía no tienen movimiento de
// disfrute en el ledger (días comprometidos, aún no descontados del saldo).
// excluirSolicitudID permite ignorar una solicitud concreta (0 = ninguna).
func (q *Queries) SumarDiasVacacionesAprobadasSinDisfrutar(ctx context.Context, empleadoID int64, excluirSolicitudID int64) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := q.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(vs.dias_solicitados), 0)
FROM vacaciones_solicitudes vs
WHERE vs.empleado_id = ?
  AND vs.estado = 'aprobada'
  AND vs.id <> ?
  AND NOT EXISTS (
        SELECT 1 FROM vacaciones_movimientos vm
        WHERE vm.solicitud_id = vs.id AND vm.tipo_movimiento = 'disfrute'
  )`, empleadoID, excluirSolicitudID).Scan(&total)
	return total, err
}

// ExisteMovimientoVacacionesPorSolicitud indica si ya hay un movimiento del
// tipo dado (acumulacion | disfrute | ajuste) ligado a la solicitud.
func (q *Queries) ExisteMovimientoVacacionesPorSolicitud(ctx context.Context, solicitudID int64, tipoMovimiento string) (bool, error) {
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM vacaciones_movimientos
WHERE solicitud_id = ? AND tipo_movimiento = ?`, solicitudID, tipoMovimiento).Scan(&total)
	return total > 0, err
}

// ExisteMovimientoAcumulacionVacaciones indica si el empleado ya tiene una
// acumulación registrada para periodoReferencia (p. ej. "2026").
func (q *Queries) ExisteMovimientoAcumulacionVacaciones(ctx context.Context, empleadoID int64, periodoReferencia string) (bool, error) {
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM vacaciones_movimientos
WHERE empleado_id = ? AND tipo_movimiento = 'acumulacion' AND periodo_referencia = ?`,
		empleadoID, periodoReferencia).Scan(&total)
	return total > 0, err
}