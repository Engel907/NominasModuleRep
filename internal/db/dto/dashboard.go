// internal/db/dto/dashboard.go
//
// Fase 8 (corregida en F10) — Dashboard gerencial. Reemplaza
// db/queries/dashboard.sql (referencia legible; ya no se compila).
//
// Verificado contra el DDL (000000_esquema_base + 000003):
//
//   - empleados.activo                                   → existe (TINYINT(1)).
//   - vacaciones_solicitudes.estado = 'pendiente'        → existe (000003).
//   - incapacidades.fecha_inicio / fecha_fin             → existen (000003).
//   - planilla_detalle_empleado.periodo_planilla_id      → es el nombre real.
//     (La versión anterior usaba pde.periodo_id, que NO existe.)
//   - planilla_detalle_empleado.total_ingresos           → existe.
//   - periodos_planilla.calculo_finalizado_en            → NO existe. Se ordena
//     por periodos_planilla.fecha_fin (NOT NULL) y se desempata por id.
//   - periodos_planilla.estado 'cerrado'                 → NO existe en el ENUM
//     ('borrador','calculada','aprobada','pagada','contabilizada'). "Cerrado"
//     es el conjunto planilla.EstadosCerrados() = pagada | contabilizada (D4);
//     el llamador lo entrega como []string (ver más abajo por qué).
//
// "Último período cerrado" = el de mayor fecha_fin entre los cerrados. Si dos
// tipos de planilla comparten fecha_fin, gana el de mayor id. No se agregan
// columnas al esquema.
//
// Este paquete NO importa internal/planilla: planilla ya depende de dto
// (adaptadores de repositorio), así que importarlo desde aquí crearía un
// ciclo. El servicio (internal/services) traduce planilla.EstadosCerrados()
// a []string y lo pasa por parámetro; los valores siguen saliendo de un solo
// lugar.
//
// "en una sola ejecución": un solo SELECT de subqueries escalares, una sola
// fila, un solo roundtrip. Todos los valores viajan como parámetros `?`.
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
// "IN ()" (SQL inválido) o a "IN (NULL)" (métricas en cero sin aviso).
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
//
// fechaReferencia se recibe explícita (no NOW()) para que sea testeable; el
// llamador decide la fecha. Se envía como 'YYYY-MM-DD' (solo fecha): las
// columnas fecha_inicio/fecha_fin son DATE, y enviar un time.Time con hora
// (y convertido a la zona del DSN) haría que una incapacidad que termina HOY
// quedara fuera, o que el día cambiara por la conversión de zona horaria.
//
// estadosCerrados: valores del ENUM periodos_planilla.estado que cuentan como
// "cerrado" (planilla.EstadosCerrados()). No puede estar vacío.
//
// Costo de la última planilla cerrada = SUM(total_ingresos) devengado (sin
// cuota patronal CCSS: no se persiste; ver F28).
func (q *Queries) ObtenerMetricasDashboard(ctx context.Context, fechaReferencia time.Time, estadosCerrados []string) (ObtenerMetricasDashboardRow, error) {
	var r ObtenerMetricasDashboardRow
	if len(estadosCerrados) == 0 {
		return r, ErrSinEstadosCerrados
	}

	fecha := fechaReferencia.Format("2006-01-02")
	args := make([]any, 0, 2*len(estadosCerrados)+2)
	for i := 0; i < 2; i++ { // la lista de estados se usa dos veces en el SQL
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
