package dto

import (
	"context"
	"time"
)

// ParametroGeneralEmpleadoRow: valor propio de un empleado para una clave de
// parametros_generales.
type ParametroGeneralEmpleadoRow struct {
	Clave string
	Valor string
	Tipo  string
}

// ObtenerParametroGeneralEmpleadoEnFecha: valor propio del empleado vigente a
// `fecha`; sql.ErrNoRows si el empleado usa el valor general.
func (q *Queries) ObtenerParametroGeneralEmpleadoEnFecha(ctx context.Context, empleadoID int64, clave string, fecha time.Time) (ParametroGeneralEmpleadoRow, error) {
	var r ParametroGeneralEmpleadoRow
	err := q.db.QueryRowContext(ctx, `
SELECT clave, valor, tipo
FROM empleado_parametros_generales
WHERE empleado_id = ? AND clave = ?
  AND fecha_inicio_vigencia <= ? AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY fecha_inicio_vigencia DESC
LIMIT 1`, empleadoID, clave, fecha, fecha).Scan(&r.Clave, &r.Valor, &r.Tipo)
	return r, err
}

// ListarParametrosGeneralesEmpleadoEnFecha: todos los valores propios del
// empleado vigentes a `fecha`, por clave.
func (q *Queries) ListarParametrosGeneralesEmpleadoEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]ParametroGeneralEmpleadoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT clave, valor, tipo
FROM empleado_parametros_generales
WHERE empleado_id = ?
  AND fecha_inicio_vigencia <= ? AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY clave`, empleadoID, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ParametroGeneralEmpleadoRow
	for rows.Next() {
		var r ParametroGeneralEmpleadoRow
		if err := rows.Scan(&r.Clave, &r.Valor, &r.Tipo); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ValorParametroGeneralEmpleado: el valor que rige para el empleado a `fecha`
// (el propio si lo tiene, si no el general); sql.ErrNoRows si ninguno está
// vigente.
func (q *Queries) ValorParametroGeneralEmpleado(ctx context.Context, empleadoID int64, clave string, fecha time.Time) (string, error) {
	var v string
	err := q.db.QueryRowContext(ctx, `
SELECT valor FROM (
    SELECT valor, 0 AS prioridad, fecha_inicio_vigencia
    FROM empleado_parametros_generales
    WHERE empleado_id = ? AND clave = ?
      AND fecha_inicio_vigencia <= ? AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
    UNION ALL
    SELECT valor, 1 AS prioridad, fecha_inicio_vigencia
    FROM parametros_generales
    WHERE clave = ?
      AND fecha_inicio_vigencia <= ? AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
) v
ORDER BY prioridad, fecha_inicio_vigencia DESC
LIMIT 1`, empleadoID, clave, fecha, fecha, clave, fecha, fecha).Scan(&v)
	return v, err
}
