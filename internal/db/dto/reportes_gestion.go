package dto

import (
	"context"
	"time"
)

// Consultas de los reportes de gestión. Devuelven texto (nil = NULL) en el
// orden de columnas documentado en cada función; los montos ya vienen
// sumados por la base de datos. Solo leen períodos con detalle calculado.

// ReporteResumenConceptos: codigo, nombre, naturaleza, empleados, cantidad,
// monto. Las cargas patronales salen con naturaleza 'patronal'.
func (q *Queries) ReporteResumenConceptos(ctx context.Context, periodoID int64) ([][]*string, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT cs.codigo, cs.nombre, cs.naturaleza, CAST(COUNT(DISTINCT pde.empleado_id) AS CHAR),
       CAST(SUM(COALESCE(pdc.cantidad, 0)) AS CHAR), CAST(SUM(pdc.monto) AS CHAR)
FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
WHERE pde.periodo_planilla_id = ?
GROUP BY cs.id, cs.codigo, cs.nombre, cs.naturaleza
UNION ALL
SELECT cs.codigo, cs.nombre, 'patronal', CAST(COUNT(DISTINCT pde.empleado_id) AS CHAR), '0', CAST(SUM(pat.monto) AS CHAR)
FROM planilla_detalle_patronal pat
JOIN planilla_detalle_empleado pde ON pde.id = pat.planilla_detalle_empleado_id
JOIN conceptos_salariales cs ON cs.id = pat.concepto_salarial_id
WHERE pde.periodo_planilla_id = ?
GROUP BY cs.id, cs.codigo, cs.nombre
ORDER BY 3, 1`, periodoID, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return escanearFilasTexto(rows, 6)
}

// ReporteCostoPorPrograma: programa_codigo, programa_nombre, empleados,
// total_ingresos, total_deducciones, neto_pagado, cuota_patronal, costo_total
// (ingresos + cargas patronales).
func (q *Queries) ReporteCostoPorPrograma(ctx context.Context, periodoID int64) ([][]*string, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT pp.codigo, pp.nombre, CAST(COUNT(*) AS CHAR),
       CAST(SUM(pde.total_ingresos) AS CHAR), CAST(SUM(pde.total_deducciones) AS CHAR), CAST(SUM(pde.neto_pagado) AS CHAR),
       CAST(SUM(pde.cuota_patronal_ccss) AS CHAR), CAST(SUM(pde.total_ingresos + pde.cuota_patronal_ccss) AS CHAR)
FROM planilla_detalle_empleado pde
JOIN programas_presupuestarios pp ON pp.id = pde.programa_presupuestario_id
WHERE pde.periodo_planilla_id = ?
GROUP BY pp.id, pp.codigo, pp.nombre
ORDER BY pp.codigo`, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return escanearFilasTexto(rows, 8)
}

// ReporteAcumuladoEmpleados: codigo_interno, nombre_completo, periodos,
// salario_bruto, base_ccss, cuota_obrera_ccss, cuota_patronal_ccss,
// base_renta, renta_retenida, creditos_fiscales, neto_pagado de los períodos
// cerrados con fecha_fin en [desde, hasta].
func (q *Queries) ReporteAcumuladoEmpleados(ctx context.Context, desde, hasta time.Time) ([][]*string, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT e.codigo_interno, e.nombre_completo, CAST(COUNT(*) AS CHAR),
       CAST(SUM(pde.total_ingresos) AS CHAR), CAST(SUM(pde.base_ccss) AS CHAR), CAST(SUM(pde.cuota_obrera_ccss) AS CHAR),
       CAST(SUM(pde.cuota_patronal_ccss) AS CHAR), CAST(SUM(pde.base_renta) AS CHAR), CAST(SUM(pde.renta_neta) AS CHAR),
       CAST(SUM(pde.creditos_fiscales_aplicados) AS CHAR), CAST(SUM(pde.neto_pagado) AS CHAR)
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
JOIN empleados e ON e.id = pde.empleado_id
WHERE pp.estado IN ('aprobada', 'pagada', 'contabilizada') AND pp.fecha_fin >= ? AND pp.fecha_fin <= ?
GROUP BY e.id, e.codigo_interno, e.nombre_completo
ORDER BY e.codigo_interno`, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return escanearFilasTexto(rows, 11)
}

// ReporteSaldosVacaciones: codigo_interno, nombre_completo, fecha_corte,
// dias_acumulados, dias_disfrutados, saldo del último corte a la fecha de cada
// empleado activo.
func (q *Queries) ReporteSaldosVacaciones(ctx context.Context, fecha time.Time) ([][]*string, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT e.codigo_interno, e.nombre_completo, CAST(va.fecha_corte AS CHAR),
       CAST(va.dias_acumulados AS CHAR), CAST(va.dias_disfrutados AS CHAR), CAST(va.saldo AS CHAR)
FROM empleados e
JOIN vacaciones_acumulado va ON va.empleado_id = e.id
WHERE e.activo = 1
  AND va.fecha_corte = (SELECT MAX(v2.fecha_corte) FROM vacaciones_acumulado v2
                        WHERE v2.empleado_id = e.id AND v2.fecha_corte <= ?)
ORDER BY e.codigo_interno`, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return escanearFilasTexto(rows, 6)
}

// ReporteRetencionesTerceros: tercero, tipo_tercero, origen, referencia,
// codigo_interno, nombre_completo, monto de embargos y préstamos retenidos en
// los períodos cerrados con fecha_fin en [desde, hasta] (lo que se gira a
// cada tercero).
func (q *Queries) ReporteRetencionesTerceros(ctx context.Context, desde, hasta time.Time) ([][]*string, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT t.nombre, t.tipo, 'embargo', em.numero_expediente, e.codigo_interno, e.nombre_completo, CAST(SUM(pdm.monto) AS CHAR)
FROM planilla_detalle_embargo pdm
JOIN planilla_detalle_empleado pde ON pde.id = pdm.planilla_detalle_empleado_id
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
JOIN embargos em ON em.id = pdm.embargo_id
JOIN terceros t ON t.id = em.tercero_id
JOIN empleados e ON e.id = pde.empleado_id
WHERE pp.estado IN ('aprobada', 'pagada', 'contabilizada') AND pp.fecha_fin >= ? AND pp.fecha_fin <= ? AND pdm.monto > 0
GROUP BY t.id, t.nombre, t.tipo, em.id, em.numero_expediente, e.id, e.codigo_interno, e.nombre_completo
UNION ALL
SELECT t.nombre, t.tipo, 'prestamo', CAST(pr.id AS CHAR), e.codigo_interno, e.nombre_completo, CAST(SUM(pdp.monto_retenido) AS CHAR)
FROM planilla_detalle_prestamo pdp
JOIN planilla_detalle_empleado pde ON pde.id = pdp.planilla_detalle_empleado_id
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
JOIN prestamos pr ON pr.id = pdp.prestamo_id
JOIN terceros t ON t.id = pr.tercero_id
JOIN empleados e ON e.id = pde.empleado_id
WHERE pp.estado IN ('aprobada', 'pagada', 'contabilizada') AND pp.fecha_fin >= ? AND pp.fecha_fin <= ? AND pdp.monto_retenido > 0
GROUP BY t.id, t.nombre, t.tipo, pr.id, e.id, e.codigo_interno, e.nombre_completo
ORDER BY 1, 3, 5`, desde, hasta, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return escanearFilasTexto(rows, 7)
}

// PeriodoTieneDetalle: el período existe y está calculado o en un estado
// posterior (tiene detalle que reportar).
func (q *Queries) PeriodoTieneDetalle(ctx context.Context, periodoID int64) (existe, conDetalle bool, err error) {
	var estado string
	err = q.db.QueryRowContext(ctx, `SELECT estado FROM periodos_planilla WHERE id = ?`, periodoID).Scan(&estado)
	if err != nil {
		return false, false, err
	}
	var n int
	err = q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM periodos_planilla WHERE id = ? AND estado IN ('calculada', 'aprobada', 'pagada', 'contabilizada')`, periodoID).Scan(&n)
	return true, n > 0, err
}
