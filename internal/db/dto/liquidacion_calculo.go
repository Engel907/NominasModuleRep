package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

// ReglaMotivoSalidaRow: fila vigente de parametros_motivos_salida.
type ReglaMotivoSalidaRow struct {
	PagaPreaviso       bool
	PagaCesantia       bool
	PagaVacaciones     bool
	PagaAguinaldo      bool
	PagaSalarioEscolar bool
	FuenteLegal        string
}

// ObtenerReglaMotivoSalidaEnFecha: regla del motivo vigente a `fecha`.
// sql.ErrNoRows si no hay ninguna.
func (q *Queries) ObtenerReglaMotivoSalidaEnFecha(ctx context.Context, motivo string, fecha time.Time) (ReglaMotivoSalidaRow, error) {
	var r ReglaMotivoSalidaRow
	err := q.db.QueryRowContext(ctx, `
SELECT paga_preaviso, paga_cesantia, paga_vacaciones, paga_aguinaldo, paga_salario_escolar, fuente_legal
FROM parametros_motivos_salida
WHERE motivo_salida = ?
  AND fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY fecha_inicio_vigencia DESC
LIMIT 1`, motivo, fecha, fecha).Scan(&r.PagaPreaviso, &r.PagaCesantia, &r.PagaVacaciones,
		&r.PagaAguinaldo, &r.PagaSalarioEscolar, &r.FuenteLegal)
	return r, err
}

// ObtenerFechaIngresoEmpleado: empleados.fecha_ingreso (antigüedad continua
// entre contratos). sql.ErrNoRows si el empleado no existe.
func (q *Queries) ObtenerFechaIngresoEmpleado(ctx context.Context, empleadoID int64) (time.Time, error) {
	var f time.Time
	err := q.db.QueryRowContext(ctx, `SELECT fecha_ingreso FROM empleados WHERE id = ?`, empleadoID).Scan(&f)
	return f, err
}

// BasePromedioRow: Σ de una base y Σ fraccion_mensual de los períodos.
type BasePromedioRow struct {
	Total    decimal.Decimal
	Meses    decimal.Decimal
	Periodos int64
}

// SumarBasePromedioEmpleado: como SumarBaseAcumuladaEmpleado (períodos
// REGULARES aprobada|pagada|contabilizada con fecha_fin en [desde, hasta]),
// pero además devuelve Σ tipos_planilla.fraccion_mensual y el número de
// períodos (cada mes de historico_salarios_pre_migracion cuenta como un
// período de un mes): el promedio mensual es Total / Meses (mensual y
// quincenal por igual). columna ∈ base_cesantia, base_vacaciones, base_aguinaldo.
func (q *Queries) SumarBasePromedioEmpleado(ctx context.Context, empleadoID int64, desde, hasta time.Time, columna string) (BasePromedioRow, error) {
	var r BasePromedioRow
	err := q.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(t.base), 0), COALESCE(SUM(t.meses), 0), COUNT(*) FROM (
  SELECT CASE ?
           WHEN 'base_aguinaldo'  THEN pde.base_aguinaldo
           WHEN 'base_vacaciones' THEN pde.base_vacaciones
           WHEN 'base_cesantia'   THEN pde.base_cesantia
           ELSE 0 END AS base,
         tp.fraccion_mensual AS meses
  FROM planilla_detalle_empleado pde
  JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
  JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
  WHERE pde.empleado_id = ?
    AND tp.clase = 'regular'
    AND pp.estado IN ('aprobada', 'pagada', 'contabilizada')
    AND pp.fecha_fin >= ?
    AND pp.fecha_fin <= ?
  UNION ALL
  SELECT CASE ?
           WHEN 'base_aguinaldo'  THEN h.base_aguinaldo
           WHEN 'base_vacaciones' THEN h.base_vacaciones
           WHEN 'base_cesantia'   THEN h.base_cesantia
           ELSE 0 END,
         1
  FROM historico_salarios_pre_migracion h
  WHERE h.empleado_id = ?
    AND LAST_DAY(MAKEDATE(h.anio, 1) + INTERVAL (h.mes - 1) MONTH) >= ?
    AND LAST_DAY(MAKEDATE(h.anio, 1) + INTERVAL (h.mes - 1) MONTH) <= ?
) t`, columna, empleadoID, desde, hasta, columna, empleadoID, desde, hasta).Scan(&r.Total, &r.Meses, &r.Periodos)
	return r, err
}

// UltimoFinPeriodoRegularEmpleado: mayor fecha_fin de los períodos regulares
// aprobada|pagada|contabilizada del empleado (NULL si no hay).
func (q *Queries) UltimoFinPeriodoRegularEmpleado(ctx context.Context, empleadoID int64) (sql.NullTime, error) {
	var f sql.NullTime
	err := q.db.QueryRowContext(ctx, `
SELECT MAX(pp.fecha_fin)
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
WHERE pde.empleado_id = ?
  AND tp.clase = 'regular'
  AND pp.estado IN ('aprobada', 'pagada', 'contabilizada')`, empleadoID).Scan(&f)
	return f, err
}
