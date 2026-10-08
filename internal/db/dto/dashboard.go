// internal/db/dto/dashboard.go
//
// Fase 8 — Dashboard gerencial. Reemplaza db/queries/dashboard.sql
// (referencia legible; ya no se compila).
//
// NOTA DE MIGRACIÓN (verificado contra vacaciones.go y asistencia.go, que sí
// reflejan db/schema.sql):
//
//   - El .sql asumía una tabla `solicitudes_vacaciones` (supuesto #2 del
//     encabezado original). La tabla real se llama `vacaciones_solicitudes`
//     (estado: pendiente | aprobada | rechazada | cancelada). Corregido.
//   - `incapacidades` (fecha_inicio / fecha_fin) SÍ coincide con el supuesto #3.
//   - Sin verificar contra schema.sql: periodos_planilla.calculo_finalizado_en,
//     planilla_detalle_empleado.total_ingresos y el valor 'cerrado' de
//     periodos_planilla.estado. Ojo: acumulados_liquidaciones.go filtra
//     periodos por estado = 'calculado' y esta query por 'cerrado'. Si ambos
//     valores existen en el ENUM real es correcto; si "cerrado" y "calculado"
//     son el mismo estado lógico, alguna de las dos queries está mal.
//   - Se agregó `, pp.id DESC` al ORDER BY del "último período cerrado" para
//     desempatar de forma determinística si dos períodos comparten (o tienen
//     NULL en) calculo_finalizado_en.
//   - "en una sola ejecución": un solo SELECT de subqueries escalares, una
//     sola fila, un solo roundtrip.
package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type ObtenerMetricasDashboardRow struct {
	TotalEmpleadosActivos           int64
	UltimoPeriodoCerradoID          sql.NullInt64 // NULL si nunca se cerró un período
	CostoUltimaPlanillaCerrada      decimal.Decimal
	SolicitudesVacacionesPendientes int64
	IncapacidadesVigentes           int64
}

// ObtenerMetricasDashboard calcula las métricas del dashboard gerencial.
// fechaReferencia se recibe explícita (no NOW()) para que sea testeable; el
// llamador decide la fecha.
//
// Costo de la última planilla cerrada = SUM(total_ingresos) devengado (sin
// cuota patronal CCSS; ver supuesto #1 del .sql original).
func (q *Queries) ObtenerMetricasDashboard(ctx context.Context, fechaReferencia time.Time) (ObtenerMetricasDashboardRow, error) {
	var r ObtenerMetricasDashboardRow
	err := q.db.QueryRowContext(ctx, `
SELECT
    (SELECT COUNT(*)
       FROM empleados
      WHERE activo = 1),

    (SELECT pp.id
       FROM periodos_planilla pp
      WHERE pp.estado = 'cerrado'
      ORDER BY pp.calculo_finalizado_en DESC, pp.id DESC
      LIMIT 1),

    (SELECT COALESCE(SUM(pde.total_ingresos), 0)
       FROM planilla_detalle_empleado pde
      WHERE pde.periodo_id = (
                SELECT pp.id
                  FROM periodos_planilla pp
                 WHERE pp.estado = 'cerrado'
                 ORDER BY pp.calculo_finalizado_en DESC, pp.id DESC
                 LIMIT 1
            )),

    (SELECT COUNT(*)
       FROM vacaciones_solicitudes vs
      WHERE vs.estado = 'pendiente'),

    (SELECT COUNT(*)
       FROM incapacidades i
      WHERE i.fecha_inicio <= ?
        AND i.fecha_fin    >= ?)`, fechaReferencia, fechaReferencia).
		Scan(&r.TotalEmpleadosActivos, &r.UltimoPeriodoCerradoID, &r.CostoUltimaPlanillaCerrada,
			&r.SolicitudesVacacionesPendientes, &r.IncapacidadesVigentes)
	return r, err
}