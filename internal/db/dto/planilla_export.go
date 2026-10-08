// internal/db/dto/planilla_export.go
//
// Fase 6c — Comprobantes, boletas de pago y exportación bancaria/CCSS.
// Reemplaza db/queries/planilla_export.sql (referencia legible; ya no se
// compila). Helpers (rowScanner) en asistencia.go. Mismo estilo que
// vacaciones.go / dashboard.go: argumentos posicionales, structs *Row para
// los resultados y decimal.Decimal para todo monto.
//
// NOTA DE MIGRACIÓN — diferencias contra el .sql original, verificadas contra
// lo que documentan empleados.go y vacaciones.go (que sí reflejan
// db/schema.sql). La lógica de negocio (qué períodos se exportan, qué filas
// entran, orden) NO cambia; solo se corrigió DE DÓNDE sale cada dato:
//
//   - planilla_detalle_empleado.empleado_id apunta a empleados(id), no a
//     usuarios(id) (misma corrección de FK que vacaciones.go, fase 1A). El
//     .sql hacía JOIN a `usuarios`, lo que devolvía filas equivocadas o
//     ninguna. Ahora el JOIN es a `empleados`.
//   - nombre_completo vive en `empleados` (no en usuarios).
//   - cedula NO existe en empleado_datos_personales (solo direccion,
//     telefono, correo_electronico). La identificación es
//     empleados.numero_identificacion, así que se eliminó el JOIN a
//     empleado_datos_personales (solo se usaba para leer la cédula).
//   - empleado_datos_bancarios NO tiene columna `iban` (tiene banco_id,
//     tipo_cuenta, numero_cuenta). ⚠️ SUPUESTO SIN VERIFICAR: se lee
//     `numero_cuenta` como IBAN. Si esa columna guarda el número de cuenta
//     local (no el IBAN de 22 caracteres) el archivo de dispersión saldría
//     con un valor que el banco rechaza; confirmar con el dato real antes de
//     usarlo en producción.
//   - Se agregó `e.id ASC` como desempate del ORDER BY de dispersión, para
//     que el archivo sea determinístico aun con nombres repetidos (el propio
//     contrato de ExportRepository exige orden determinístico).
//
// Sin verificar contra schema.sql (se dejaron tal cual el .sql original):
// periodos_planilla.estado con valores 'calculado' y 'cerrado' (ver la
// advertencia sobre ambos estados en dashboard.go),
// planilla_detalle_concepto (planilla_detalle_empleado_id,
// concepto_salarial_id, monto, cantidad, orden_calculo),
// conceptos_salariales.naturaleza, la tabla planilla_detalle_embargo
// completa y parametros_tipos_embargo.nombre.
package dto

import (
	"context"

	"github.com/shopspring/decimal"
)

// ============================================================================
// DISPERSIÓN BANCARIA
// ============================================================================

type ObtenerDetallesPlanillaParaDispersionRow struct {
	EmpleadoID     int64
	Cedula         string
	NombreCompleto string
	IBAN           string
	NetoPagado     decimal.Decimal
}

// ObtenerDetallesPlanillaParaDispersion devuelve una fila por empleado con lo
// estrictamente necesario para el archivo plano de dispersión (IBAN, cédula,
// nombre, neto pagado). Solo períodos 'calculado'/'cerrado': nunca uno
// 'calculando' o 'error', para no dispersar un cálculo a medias o fallido.
func (q *Queries) ObtenerDetallesPlanillaParaDispersion(ctx context.Context, periodoID int64) ([]ObtenerDetallesPlanillaParaDispersionRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT
    e.id,
    e.numero_identificacion,
    e.nombre_completo,
    edb.numero_cuenta,
    pde.neto_pagado
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp
  ON pp.id = pde.periodo_id
JOIN empleados e
  ON e.id = pde.empleado_id
JOIN empleado_datos_bancarios edb
  ON edb.empleado_id = e.id AND edb.es_vigente = 1
WHERE pde.periodo_id = ?
  AND pp.estado IN ('calculado', 'cerrado')
ORDER BY e.nombre_completo ASC, e.id ASC`, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ObtenerDetallesPlanillaParaDispersionRow
	for rows.Next() {
		var r ObtenerDetallesPlanillaParaDispersionRow
		if err := rows.Scan(&r.EmpleadoID, &r.Cedula, &r.NombreCompleto, &r.IBAN, &r.NetoPagado); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ============================================================================
// BOLETA DE PAGO
// ============================================================================

type ObtenerDatosBoletaPagoRow struct {
	PlanillaDetalleEmpleadoID int64
	EmpleadoID                int64
	PeriodoID                 int64
	NombreCompleto            string
	Cedula                    string
	SalarioBasePeriodo        decimal.Decimal
	TotalIngresos             decimal.Decimal
	TotalDeducciones          decimal.Decimal
	NetoPagado                decimal.Decimal
}

// ObtenerDatosBoletaPago devuelve el encabezado agregado de la boleta. Si no
// existe planilla_detalle_empleado para periodoID+empleadoID devuelve
// sql.ErrNoRows (nunca una fila zero-value).
func (q *Queries) ObtenerDatosBoletaPago(ctx context.Context, periodoID, empleadoID int64) (ObtenerDatosBoletaPagoRow, error) {
	var r ObtenerDatosBoletaPagoRow
	err := q.db.QueryRowContext(ctx, `
SELECT
    pde.id,
    pde.empleado_id,
    pde.periodo_id,
    e.nombre_completo,
    e.numero_identificacion,
    pde.salario_base_periodo,
    pde.total_ingresos,
    pde.total_deducciones,
    pde.neto_pagado
FROM planilla_detalle_empleado pde
JOIN empleados e
  ON e.id = pde.empleado_id
WHERE pde.periodo_id = ?
  AND pde.empleado_id = ?`, periodoID, empleadoID).
		Scan(&r.PlanillaDetalleEmpleadoID, &r.EmpleadoID, &r.PeriodoID, &r.NombreCompleto, &r.Cedula,
			&r.SalarioBasePeriodo, &r.TotalIngresos, &r.TotalDeducciones, &r.NetoPagado)
	return r, err
}

type ObtenerDesgloseConceptosBoletaPagoRow struct {
	CodigoConcepto string
	NombreConcepto string
	Naturaleza     string // 'ingreso' | 'deduccion' (conceptos_salariales.naturaleza)
	Monto          decimal.Decimal
	Cantidad       decimal.Decimal
	OrdenCalculo   int32
}

// ObtenerDesgloseConceptosBoletaPago devuelve el desglose de ingresos y
// deducciones por concepto, en orden de cálculo. `naturaleza` permite al
// servicio decidir en qué columna de la boleta va cada monto.
func (q *Queries) ObtenerDesgloseConceptosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]ObtenerDesgloseConceptosBoletaPagoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT
    cs.codigo,
    cs.nombre,
    cs.naturaleza,
    pdc.monto,
    pdc.cantidad,
    pdc.orden_calculo
FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde
  ON pde.id = pdc.planilla_detalle_empleado_id
JOIN conceptos_salariales cs
  ON cs.id = pdc.concepto_salarial_id
WHERE pde.periodo_id = ?
  AND pde.empleado_id = ?
ORDER BY pdc.orden_calculo ASC`, periodoID, empleadoID)
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
	EmbargoID                  int64
	TipoEmbargoCodigo          string
	TipoEmbargoNombre          string
	PrioridadLegal             int32
	MontoCalculado             decimal.Decimal
	MontoAplicado              decimal.Decimal
	SaldoPendienteInsatisfecho decimal.Decimal
}

// ObtenerEmbargosBoletaPago devuelve los embargos efectivamente aplicados en
// el período, en orden de prioridad legal (Art. 172 Código de Trabajo / Ley
// 7654; ver internal/planilla/domain.go, CalcularEmbargos).
func (q *Queries) ObtenerEmbargosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]ObtenerEmbargosBoletaPagoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT
    pdemb.embargo_id,
    te.codigo,
    te.nombre,
    te.prioridad_legal,
    pdemb.monto_calculado,
    pdemb.monto_aplicado,
    pdemb.saldo_pendiente_insatisfecho
FROM planilla_detalle_embargo pdemb
JOIN embargos e
  ON e.id = pdemb.embargo_id
JOIN parametros_tipos_embargo te
  ON te.id = e.tipo_embargo_id
WHERE pdemb.planilla_detalle_empleado_id = (
    SELECT pde.id
    FROM planilla_detalle_empleado pde
    WHERE pde.periodo_id = ? AND pde.empleado_id = ?
)
ORDER BY te.prioridad_legal ASC`, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ObtenerEmbargosBoletaPagoRow
	for rows.Next() {
		var r ObtenerEmbargosBoletaPagoRow
		if err := rows.Scan(&r.EmbargoID, &r.TipoEmbargoCodigo, &r.TipoEmbargoNombre, &r.PrioridadLegal,
			&r.MontoCalculado, &r.MontoAplicado, &r.SaldoPendienteInsatisfecho); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}