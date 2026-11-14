package dto

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var errExportSinEstados = errors.New("dto: lista de estados de período vacía para la exportación")

// placeholdersExport devuelve "?,?,..." con n marcadores (n >= 1).
func placeholdersExport(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func argsEstadosExport(estados []string, prefix ...interface{}) []interface{} {
	args := make([]interface{}, 0, len(prefix)+len(estados))
	args = append(args, prefix...)
	for _, e := range estados {
		args = append(args, e)
	}
	return args
}

type ObtenerDetallesPlanillaParaDispersionRow struct {
	EmpleadoID     int64
	Cedula         ValorSensible
	NombreCompleto string
	IBAN           ValorSensible
	NetoPagado     decimal.Decimal
}

// ObtenerDetallesPlanillaParaDispersion devuelve una fila por empleado con lo
// estrictamente necesario para el archivo plano de dispersión. Solo períodos
// cuyo estado esté en `estados` (el servicio pasa aprobada|pagada).
func (q *Queries) ObtenerDetallesPlanillaParaDispersion(ctx context.Context, periodoID int64, estados []string) ([]ObtenerDetallesPlanillaParaDispersionRow, error) {
	if len(estados) == 0 {
		return nil, errExportSinEstados
	}
	query := `
SELECT
    e.id,
    e.numero_identificacion_cifrado,
    e.numero_identificacion_kid,
    e.nombre_completo,
    edb.numero_cuenta_cifrado,
    edb.numero_cuenta_kid,
    pde.neto_pagado
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp
  ON pp.id = pde.periodo_planilla_id
JOIN empleados e
  ON e.id = pde.empleado_id
JOIN empleado_datos_bancarios edb
  ON edb.empleado_id = e.id AND edb.es_vigente = 1
WHERE pde.periodo_planilla_id = ?
  AND pp.estado IN (` + placeholdersExport(len(estados)) + `)
ORDER BY e.nombre_completo ASC, e.id ASC`

	rows, err := q.db.QueryContext(ctx, query, argsEstadosExport(estados, periodoID)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ObtenerDetallesPlanillaParaDispersionRow
	for rows.Next() {
		var r ObtenerDetallesPlanillaParaDispersionRow
		if err := rows.Scan(&r.EmpleadoID, &r.Cedula.Cifrado, &r.Cedula.Kid, &r.NombreCompleto, &r.IBAN.Cifrado, &r.IBAN.Kid, &r.NetoPagado); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// InsertarExportacionGeneradaParams: PeriodoID > 0 para el archivo de un
// período, o PeriodoID = 0 con Anio y Mes para el consolidado de un mes.
type InsertarExportacionGeneradaParams struct {
	PeriodoID     int64
	Anio, Mes     int32
	FormatoCodigo string
	NombreArchivo string
	HashSHA256    string
	CantidadFilas int64
	TotalMonto    decimal.Decimal
	UsuarioID     int64
}

// InsertarExportacionGenerada registra una fila en exportaciones_generadas y
// devuelve su id. Resuelve el formato por formatos_exportacion.codigo en el
// mismo INSERT; si el código no existe no se inserta nada y devuelve error.
// Debe llamarse dentro de WithAuditContext. ruta_archivo queda NULL: el
// archivo no se persiste en disco, solo su hash (migración 000109).
func (q *Queries) InsertarExportacionGenerada(ctx context.Context, p InsertarExportacionGeneradaParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO exportaciones_generadas
    (periodo_planilla_id, anio, mes, formato_exportacion_id, fecha_generacion, ruta_archivo,
     hash_verificacion, generado_por_usuario_id, nombre_archivo, cantidad_filas, total_monto)
SELECT NULLIF(?, 0), NULLIF(?, 0), NULLIF(?, 0), f.id, NOW(), NULL, ?, ?, ?, ?, ?
FROM formatos_exportacion f
WHERE f.codigo = ?`,
		p.PeriodoID, p.Anio, p.Mes, p.HashSHA256, p.UsuarioID, p.NombreArchivo, p.CantidadFilas, p.TotalMonto, p.FormatoCodigo)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n != 1 {
		return 0, errors.New("dto: formato de exportación inexistente en formatos_exportacion: " + p.FormatoCodigo)
	}
	return res.LastInsertId()
}

type ObtenerDatosBoletaPagoRow struct {
	PlanillaDetalleEmpleadoID int64
	EmpleadoID                int64
	PeriodoID                 int64
	NombreCompleto            string
	Cedula                    ValorSensible
	SalarioBasePeriodo        decimal.Decimal
	TotalIngresos             decimal.Decimal
	TotalDeducciones          decimal.Decimal
	NetoPagado                decimal.Decimal
}

// ObtenerDatosBoletaPago devuelve el encabezado de la boleta. Si no existe
// planilla_detalle_empleado para periodoID+empleadoID, o el período no está
// en `estados` (el servicio pasa aprobada|pagada|contabilizada), devuelve
// sql.ErrNoRows (nunca una fila zero-value).
func (q *Queries) ObtenerDatosBoletaPago(ctx context.Context, periodoID, empleadoID int64, estados []string) (ObtenerDatosBoletaPagoRow, error) {
	var r ObtenerDatosBoletaPagoRow
	if len(estados) == 0 {
		return r, errExportSinEstados
	}
	query := `
SELECT
    pde.id,
    pde.empleado_id,
    pde.periodo_planilla_id,
    e.nombre_completo,
    e.numero_identificacion_cifrado,
    e.numero_identificacion_kid,
    pde.salario_base_periodo,
    pde.total_ingresos,
    pde.total_deducciones,
    pde.neto_pagado
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp
  ON pp.id = pde.periodo_planilla_id
JOIN empleados e
  ON e.id = pde.empleado_id
WHERE pde.periodo_planilla_id = ?
  AND pde.empleado_id = ?
  AND pp.estado IN (` + placeholdersExport(len(estados)) + `)`

	err := q.db.QueryRowContext(ctx, query, argsEstadosExport(estados, periodoID, empleadoID)...).
		Scan(&r.PlanillaDetalleEmpleadoID, &r.EmpleadoID, &r.PeriodoID, &r.NombreCompleto, &r.Cedula.Cifrado, &r.Cedula.Kid,
			&r.SalarioBasePeriodo, &r.TotalIngresos, &r.TotalDeducciones, &r.NetoPagado)
	return r, err
}

type ObtenerDesgloseConceptosBoletaPagoRow struct {
	CodigoConcepto string
	NombreConcepto string
	Naturaleza     string // 'ingreso' | 'deduccion' (conceptos_salariales.naturaleza)
	Monto          decimal.Decimal
	Cantidad       decimal.Decimal
	OrdenCalculo   int32 // orden_calculo_concepto.orden; 0 si no hay orden vigente
}

// ObtenerDesgloseConceptosBoletaPago devuelve el desglose por concepto en
// orden de cálculo. El orden sale de orden_calculo_concepto vigente a la
// fecha fin del período. LEFT JOIN: un concepto sin orden vigente no se
// pierde de la boleta (queda al final, OrdenCalculo = 0). cantidad es NULL
// en la tabla cuando no aplica: se normaliza a 0.
func (q *Queries) ObtenerDesgloseConceptosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]ObtenerDesgloseConceptosBoletaPagoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT
    cs.codigo,
    cs.nombre,
    cs.naturaleza,
    pdc.monto,
    COALESCE(pdc.cantidad, 0),
    COALESCE(occ.orden, 0)
FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde
  ON pde.id = pdc.planilla_detalle_empleado_id
JOIN periodos_planilla pp
  ON pp.id = pde.periodo_planilla_id
JOIN conceptos_salariales cs
  ON cs.id = pdc.concepto_salarial_id
LEFT JOIN orden_calculo_concepto occ
  ON occ.concepto_salarial_id = pdc.concepto_salarial_id
 AND occ.fecha_inicio_vigencia <= pp.fecha_fin
 AND (occ.fecha_fin_vigencia IS NULL OR occ.fecha_fin_vigencia >= pp.fecha_fin)
WHERE pde.periodo_planilla_id = ?
  AND pde.empleado_id = ?
ORDER BY (occ.orden IS NULL) ASC, occ.orden ASC, pdc.id ASC`, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ObtenerDesgloseConceptosBoletaPagoRow
	for rows.Next() {
		var r ObtenerDesgloseConceptosBoletaPagoRow
		if err := rows.Scan(&r.CodigoConcepto, &r.NombreConcepto, &r.Naturaleza, &r.Monto, &r.Cantidad, &r.OrdenCalculo); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type ObtenerEmbargosBoletaPagoRow struct {
	EmbargoID         int64
	TipoEmbargoCodigo string
	TipoEmbargoNombre string
	OrdenPrioridad    int32 // planilla_detalle_embargo.orden_prioridad
	Monto             decimal.Decimal
}

// ObtenerEmbargosBoletaPago devuelve los embargos efectivamente retenidos en
// el período, en el orden de prioridad con que se calcularon
// (planilla_detalle_embargo.orden_prioridad, copia histórica).
func (q *Queries) ObtenerEmbargosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]ObtenerEmbargosBoletaPagoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT
    pdemb.embargo_id,
    te.codigo,
    te.nombre,
    pdemb.orden_prioridad,
    pdemb.monto
FROM planilla_detalle_embargo pdemb
JOIN planilla_detalle_empleado pde
  ON pde.id = pdemb.planilla_detalle_empleado_id
JOIN embargos e
  ON e.id = pdemb.embargo_id
JOIN parametros_tipos_embargo te
  ON te.id = e.tipo_embargo_id
WHERE pde.periodo_planilla_id = ?
  AND pde.empleado_id = ?
ORDER BY pdemb.orden_prioridad ASC, pdemb.id ASC`, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ObtenerEmbargosBoletaPagoRow
	for rows.Next() {
		var r ObtenerEmbargosBoletaPagoRow
		if err := rows.Scan(&r.EmbargoID, &r.TipoEmbargoCodigo, &r.TipoEmbargoNombre, &r.OrdenPrioridad, &r.Monto); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ObtenerIncapacidadesBoletaPagoRow: tramo de subsidio de incapacidad (boleta).
type ObtenerIncapacidadesBoletaPagoRow struct {
	IncapacidadID           int64
	NumeroBoleta            string
	FechaInicio, FechaFin   time.Time
	DiaSerieDesde           int32
	DiaSerieHasta           int32
	Dias                    decimal.Decimal
	MontoPatronal           decimal.Decimal
	MontoEntidadInformativo decimal.Decimal
}

func (q *Queries) ObtenerIncapacidadesBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]ObtenerIncapacidadesBoletaPagoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT pdi.incapacidad_id, i.numero_boleta, i.fecha_inicio, i.fecha_fin, pdi.dia_serie_desde, pdi.dia_serie_hasta,
       pdi.dias, pdi.monto_patronal, pdi.monto_entidad_informativo
FROM planilla_detalle_incapacidad pdi
JOIN planilla_detalle_empleado pde ON pde.id = pdi.planilla_detalle_empleado_id
JOIN incapacidades i ON i.id = pdi.incapacidad_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ?
ORDER BY i.fecha_inicio, pdi.dia_serie_desde`, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObtenerIncapacidadesBoletaPagoRow
	for rows.Next() {
		var r ObtenerIncapacidadesBoletaPagoRow
		if err := rows.Scan(&r.IncapacidadID, &r.NumeroBoleta, &r.FechaInicio, &r.FechaFin, &r.DiaSerieDesde, &r.DiaSerieHasta,
			&r.Dias, &r.MontoPatronal, &r.MontoEntidadInformativo); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
