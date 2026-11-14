package dto

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

// ErrSinEstadosCerrados evita que una lista vacía degrade el filtro a
// "IN" (SQL inválido) o a "IN (NULL)" (métricas en cero sin aviso).
var ErrSinEstadosCerrados = errors.New("dto: la lista de estados cerrados no puede estar vacía")

// consultaMetricasDashboard arma el SQL con `n` placeholders por cada uso de
// la lista de estados. La lista aparece DOS veces (id del último período y
// su costo), por eso el llamador repite los argumentos.
func consultaMetricasDashboard(n int) string {
	in := "IN (" + strings.TrimSuffix(strings.Repeat("?,", n), ",") + ")"
	return `
SELECT
    (SELECT COUNT(*)
       FROM empleados
      WHERE activo = 1),

    (SELECT pp.id
       FROM periodos_planilla pp
      WHERE pp.estado ` + in + `
      ORDER BY pp.fecha_fin DESC, pp.id DESC
      LIMIT 1),

    (SELECT COALESCE(SUM(pde.total_ingresos), 0)
       FROM planilla_detalle_empleado pde
      WHERE pde.periodo_planilla_id = (
                SELECT pp.id
                  FROM periodos_planilla pp
                 WHERE pp.estado ` + in + `
                 ORDER BY pp.fecha_fin DESC, pp.id DESC
                 LIMIT 1
            )),

    (SELECT COUNT(*)
       FROM vacaciones_solicitudes vs
      WHERE vs.estado = 'pendiente'),

    (SELECT COUNT(*)
       FROM incapacidades i
      WHERE i.fecha_inicio <= ?
        AND i.fecha_fin    >= ?)`
}

// ObtenerMetricasDashboard calcula las métricas del dashboard gerencial.
func (q *Queries) ObtenerMetricasDashboard(ctx context.Context, fechaReferencia time.Time, estadosCerrados []string) (ObtenerMetricasDashboardRow, error) {
	var r ObtenerMetricasDashboardRow
	if len(estadosCerrados) == 0 {
		return r, ErrSinEstadosCerrados
	}

	fecha := fechaReferencia.Format("2006-01-02")
	args := make([]any, 0, 2*len(estadosCerrados)+2)
	for i := 0; i < 2; i++ {
		for _, e := range estadosCerrados {
			args = append(args, e)
		}
	}
	args = append(args, fecha, fecha)

	err := q.db.QueryRowContext(ctx, consultaMetricasDashboard(len(estadosCerrados)), args...).
		Scan(&r.TotalEmpleadosActivos, &r.UltimoPeriodoCerradoID, &r.CostoUltimaPlanillaCerrada,
			&r.SolicitudesVacacionesPendientes, &r.IncapacidadesVigentes)
	return r, err
}
