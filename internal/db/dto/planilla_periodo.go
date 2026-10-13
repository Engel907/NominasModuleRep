// internal/db/dto/planilla_periodo.go
//
// Fase 9B — Lecturas para CalcularPeriodo, subsidio de incapacidad y las
// corridas periódicas de aguinaldo / salario escolar.
//
// Todas las columnas de este archivo se verificaron contra db/schema.sql
// (periodos_planilla.fecha_inicio/fecha_fin y su ENUM de estado,
// planilla_detalle_empleado.periodo_planilla_id, registro_asistencia, etc.).
// OJO: algunas queries previas del paquete (acumulados_liquidaciones.go,
// dashboard.go, planilla_export.go) usan nombres distintos
// (pde.periodo_id, pp.fecha_fin_periodo, estado 'calculado'/'cerrado') que
// NO existen en schema.sql; aquí no se reutilizan.
//
// Convención de unidades: estas funciones devuelven los valores TAL COMO
// están en la BD (porcentajes 0-100). La conversión a fracción la hace el
// repositorio de internal/planilla, en un solo lugar.
package dto

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// PERIODO
// ============================================================================

type PeriodoPlanillaRow struct {
	ID                 int64
	TipoPlanillaCodigo string
	FechaInicio        time.Time
	FechaFin           time.Time
	Estado             string // borrador | calculada | aprobada | pagada | contabilizada
}

func (q *Queries) ObtenerPeriodoPlanilla(ctx context.Context, periodoID int64) (PeriodoPlanillaRow, error) {
	var r PeriodoPlanillaRow
	err := q.db.QueryRowContext(ctx, `
SELECT pp.id, tp.codigo, pp.fecha_inicio, pp.fecha_fin, pp.estado
FROM periodos_planilla pp
JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
WHERE pp.id = ?`, periodoID).Scan(&r.ID, &r.TipoPlanillaCodigo, &r.FechaInicio, &r.FechaFin, &r.Estado)
	return r, err
}

// ListarEmpleadosActivosEnRango: empleados con activo = 1, ya ingresados al
// fin del rango y con al menos una versión de contrato que se solape con él.
func (q *Queries) ListarEmpleadosActivosEnRango(ctx context.Context, desde, hasta time.Time) ([]int64, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT e.id
FROM empleados e
WHERE e.activo = 1
  AND e.fecha_ingreso <= ?
  AND EXISTS (
        SELECT 1 FROM contratos c
        WHERE c.empleado_id = e.id
          AND c.fecha_inicio_vigencia <= ?
          AND (c.fecha_fin_vigencia IS NULL OR c.fecha_fin_vigencia >= ?)
  )
ORDER BY e.id ASC`, hasta, hasta, desde)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ============================================================================
// PARAMETROS VIGENTES A UNA FECHA
// ============================================================================

type CargaSocialVigenteRow struct {
	ConceptoCodigo     string
	PorcentajePatronal decimal.Decimal // 0-100
	PorcentajeObrero   decimal.Decimal // 0-100
}

// ListarCargasSocialesEnFecha: filas de parametros_cargas_sociales vigentes,
// resueltas por concepto_salarial_id (nunca por el texto de `rubro`).
func (q *Queries) ListarCargasSocialesEnFecha(ctx context.Context, fecha time.Time) ([]CargaSocialVigenteRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT cs.codigo, pcs.porcentaje_patronal, pcs.porcentaje_obrero
FROM parametros_cargas_sociales pcs
JOIN conceptos_salariales cs ON cs.id = pcs.concepto_salarial_id
WHERE pcs.fecha_inicio_vigencia <= ?
  AND (pcs.fecha_fin_vigencia IS NULL OR pcs.fecha_fin_vigencia >= ?)
ORDER BY cs.codigo ASC, pcs.id ASC`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CargaSocialVigenteRow
	for rows.Next() {
		var r CargaSocialVigenteRow
		if err := rows.Scan(&r.ConceptoCodigo, &r.PorcentajePatronal, &r.PorcentajeObrero); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type TramoRentaVigenteRow struct {
	OrdenTramo int32
	MontoDesde decimal.Decimal
	MontoHasta decimal.NullDecimal // NULL = tramo abierto
	Porcentaje decimal.Decimal     // 0-100
}

func (q *Queries) ListarTramosRentaEnFecha(ctx context.Context, fecha time.Time) ([]TramoRentaVigenteRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT orden_tramo, monto_desde, monto_hasta, porcentaje
FROM parametros_tramos_renta
WHERE fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY orden_tramo ASC`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TramoRentaVigenteRow
	for rows.Next() {
		var r TramoRentaVigenteRow
		if err := rows.Scan(&r.OrdenTramo, &r.MontoDesde, &r.MontoHasta, &r.Porcentaje); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type CreditoFiscalVigenteRow struct {
	TipoCreditoFiscalID int64
	TipoCodigo          string // CONYUGE | HIJO | ...
	MontoCredito        decimal.Decimal
}

func (q *Queries) ListarCreditosFiscalesEnFecha(ctx context.Context, fecha time.Time) ([]CreditoFiscalVigenteRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT t.id, t.codigo, p.monto_credito
FROM parametros_creditos_fiscales_renta p
JOIN tipos_credito_fiscal_renta t ON t.id = p.tipo_credito_fiscal_id
WHERE p.fecha_inicio_vigencia <= ?
  AND (p.fecha_fin_vigencia IS NULL OR p.fecha_fin_vigencia >= ?)
ORDER BY t.id ASC`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CreditoFiscalVigenteRow
	for rows.Next() {
		var r CreditoFiscalVigenteRow
		if err := rows.Scan(&r.TipoCreditoFiscalID, &r.TipoCodigo, &r.MontoCredito); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ============================================================================
// SUBSIDIO DE INCAPACIDAD (parametros_subsidios_incapacidad)
// ============================================================================

type TramoSubsidioIncapacidadRow struct {
	EntidadEmisoraID   int64
	EntidadCodigo      string // CCSS | INS
	DiaDesde           int32
	DiaHasta           *int32          // NULL = sin tope
	PorcentajePatronal decimal.Decimal // 0-100
	PorcentajeEntidad  decimal.NullDecimal
}

func (q *Queries) ListarTramosSubsidioIncapacidadEnFecha(ctx context.Context, fecha time.Time) ([]TramoSubsidioIncapacidadRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT p.entidad_emisora_id, e.codigo, p.dia_desde, p.dia_hasta, p.porcentaje_patronal, p.porcentaje_entidad
FROM parametros_subsidios_incapacidad p
JOIN entidades_emisoras_incapacidad e ON e.id = p.entidad_emisora_id
WHERE p.fecha_inicio_vigencia <= ?
  AND (p.fecha_fin_vigencia IS NULL OR p.fecha_fin_vigencia >= ?)
ORDER BY p.entidad_emisora_id ASC, p.dia_desde ASC`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TramoSubsidioIncapacidadRow
	for rows.Next() {
		var r TramoSubsidioIncapacidadRow
		var hasta *int64
		var desde int64
		if err := rows.Scan(&r.EntidadEmisoraID, &r.EntidadCodigo, &desde, &hasta, &r.PorcentajePatronal, &r.PorcentajeEntidad); err != nil {
			return nil, err
		}
		r.DiaDesde = int32(desde)
		if hasta != nil {
			h := int32(*hasta)
			r.DiaHasta = &h
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type IncapacidadSolapadaRow struct {
	IncapacidadID    int64
	EntidadEmisoraID int64
	FechaInicio      time.Time
	FechaFin         time.Time
}

// ListarIncapacidadesSolapadas: boletas de `incapacidades` que se solapan
// con [desde, hasta]. entidad_emisora (ENUM CCSS/INS) se convierte al id de
// entidades_emisoras_incapacidad por código.
func (q *Queries) ListarIncapacidadesSolapadas(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]IncapacidadSolapadaRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT i.id, eei.id, i.fecha_inicio, i.fecha_fin
FROM incapacidades i
JOIN entidades_emisoras_incapacidad eei ON eei.codigo = i.entidad_emisora
WHERE i.empleado_id = ?
  AND i.fecha_inicio <= ?
  AND i.fecha_fin >= ?
ORDER BY i.fecha_inicio ASC, i.id ASC`, empleadoID, hasta, desde)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IncapacidadSolapadaRow
	for rows.Next() {
		var r IncapacidadSolapadaRow
		if err := rows.Scan(&r.IncapacidadID, &r.EntidadEmisoraID, &r.FechaInicio, &r.FechaFin); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ============================================================================
// INSUMOS POR EMPLEADO
// ============================================================================

type EmbargoActivoRow struct {
	EmbargoID      int64
	TipoEmbargoID  int64
	PrioridadLegal int32
	Porcentaje     decimal.NullDecimal // 0-100
	MontoFijo      decimal.NullDecimal
}

// ListarEmbargosActivosEnFecha: embargos (tabla `embargos`, catálogo del
// empleado) activos y vigentes a `fecha`, por prioridad legal. La tabla
// planilla_detalle_embargo es el RESULTADO persistido de una corrida, no la
// fuente de embargos activos.
func (q *Queries) ListarEmbargosActivosEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]EmbargoActivoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT e.id, e.tipo_embargo_id, te.prioridad_legal, e.porcentaje, e.monto_fijo
FROM embargos e
JOIN parametros_tipos_embargo te ON te.id = e.tipo_embargo_id
WHERE e.empleado_id = ?
  AND e.estado = 'activo'
  AND e.fecha_inicio_vigencia <= ?
  AND (e.fecha_fin_vigencia IS NULL OR e.fecha_fin_vigencia >= ?)
ORDER BY te.prioridad_legal ASC, e.id ASC`, empleadoID, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EmbargoActivoRow
	for rows.Next() {
		var r EmbargoActivoRow
		if err := rows.Scan(&r.EmbargoID, &r.TipoEmbargoID, &r.PrioridadLegal, &r.Porcentaje, &r.MontoFijo); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type PrestamoActivoRow struct {
	PrestamoID     int64
	CuotaPeriodica decimal.Decimal
	SaldoPendiente decimal.Decimal
}

func (q *Queries) ListarPrestamosActivosEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]PrestamoActivoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, cuota_periodica, saldo_pendiente
FROM prestamos
WHERE empleado_id = ?
  AND estado = 'activo'
  AND saldo_pendiente > 0
  AND fecha_inicio <= ?
ORDER BY fecha_inicio ASC, id ASC`, empleadoID, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrestamoActivoRow
	for rows.Next() {
		var r PrestamoActivoRow
		if err := rows.Scan(&r.PrestamoID, &r.CuotaPeriodica, &r.SaldoPendiente); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type DatosFamiliaresEnFechaRow struct {
	AplicaCreditoConyuge    bool
	NumeroHijosDependientes int32
}

// ObtenerDatosFamiliaresEnFecha devuelve sql.ErrNoRows si el empleado no
// tiene declaración familiar vigente (= sin créditos fiscales, no es error
// del cálculo; lo decide el llamador).
func (q *Queries) ObtenerDatosFamiliaresEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (DatosFamiliaresEnFechaRow, error) {
	var r DatosFamiliaresEnFechaRow
	var hijos int64
	err := q.db.QueryRowContext(ctx, `
SELECT aplica_credito_conyuge, numero_hijos_dependientes
FROM empleado_datos_familiares
WHERE empleado_id = ?
  AND fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY fecha_inicio_vigencia DESC
LIMIT 1`, empleadoID, fecha, fecha).Scan(&r.AplicaCreditoConyuge, &hijos)
	r.NumeroHijosDependientes = int32(hijos)
	return r, err
}

type HorasExtraPeriodoRow struct {
	TipoHoraExtraID int64
	Horas           decimal.Decimal
}

// SumarHorasExtraPeriodo suma, por tipo de hora extra, las horas de
// registro_asistencia en [desde, hasta]. El mapeo columna -> tipo usa el
// código de tipos_hora_extra (DIURNA, NOCTURNA, MIXTA, DOBLE); un tipo
// activo sin columna en registro_asistencia simplemente suma 0.
func (q *Queries) SumarHorasExtraPeriodo(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]HorasExtraPeriodoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT t.id,
       COALESCE(SUM(CASE t.codigo
            WHEN 'DIURNA'   THEN ra.horas_extra_diurnas
            WHEN 'NOCTURNA' THEN ra.horas_extra_nocturnas
            WHEN 'MIXTA'    THEN ra.horas_extra_mixtas
            WHEN 'DOBLE'    THEN ra.horas_dobles
            ELSE 0 END), 0) AS total
FROM tipos_hora_extra t
LEFT JOIN registro_asistencia ra
  ON ra.empleado_id = ? AND ra.fecha >= ? AND ra.fecha <= ?
WHERE t.activo = 1
GROUP BY t.id, t.codigo
HAVING total > 0
ORDER BY t.id ASC`, empleadoID, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HorasExtraPeriodoRow
	for rows.Next() {
		var r HorasExtraPeriodoRow
		if err := rows.Scan(&r.TipoHoraExtraID, &r.Horas); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ObtenerSalarioMinimoEnFecha: salario mínimo legal (parametros_salario_minimo)
// de la clasificación del puesto del contrato vigente a `fecha`. Es el
// insumo de "salario mínimo inembargable" del Art. 172 CT.
func (q *Queries) ObtenerSalarioMinimoEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (decimal.Decimal, error) {
	var m decimal.Decimal
	err := q.db.QueryRowContext(ctx, `
SELECT psm.monto_mensual
FROM contratos c
JOIN puestos p ON p.id = c.puesto_id
JOIN parametros_salario_minimo psm
  ON psm.clasificacion_puesto_id = p.clasificacion_puesto_id
 AND psm.fecha_inicio_vigencia <= ?
 AND (psm.fecha_fin_vigencia IS NULL OR psm.fecha_fin_vigencia >= ?)
WHERE c.empleado_id = ?
  AND c.fecha_inicio_vigencia <= ?
  AND (c.fecha_fin_vigencia IS NULL OR c.fecha_fin_vigencia >= ?)
ORDER BY c.fecha_inicio_vigencia DESC
LIMIT 1`, fecha, fecha, empleadoID, fecha, fecha).Scan(&m)
	return m, err
}

// ============================================================================
// AGUINALDO / SALARIO ESCOLAR — base acumulada y existencia previa
// ============================================================================

type BaseAcumuladaEmpleadoRow struct {
	EmpleadoID     int64
	TotalBase      decimal.Decimal
	PlanillasCount int64
}

// SumarBaseAguinaldoPorEmpleado suma, por empleado, los conceptos de
// INGRESO con conceptos_salariales.afecta_base_aguinaldo = 1 de las
// planillas ya calculadas (estado <> 'borrador') cuyo fecha_fin cae en
// [desde, hasta]. Excluye planillas de tipo 'aguinaldo' y 'liquidacion'
// para no re-contar pagos que ya son resultado de otra corrida.
func (q *Queries) SumarBaseAguinaldoPorEmpleado(ctx context.Context, desde, hasta time.Time) ([]BaseAcumuladaEmpleadoRow, error) {
	return q.sumarBasePorEmpleado(ctx, "cs.afecta_base_aguinaldo", desde, hasta)
}

// SumarBaseSalarioEscolarPorEmpleado: igual, pero por total de ingresos que
// componen el salario bruto (afecta_base_ccss = 1): schema.sql no define
// una bandera afecta_base_salario_escolar (ver nota en la respuesta).
func (q *Queries) SumarBaseSalarioEscolarPorEmpleado(ctx context.Context, desde, hasta time.Time) ([]BaseAcumuladaEmpleadoRow, error) {
	return q.sumarBasePorEmpleado(ctx, "cs.afecta_base_ccss", desde, hasta)
}

func (q *Queries) sumarBasePorEmpleado(ctx context.Context, flagCol string, desde, hasta time.Time) ([]BaseAcumuladaEmpleadoRow, error) {
	// flagCol es una constante interna (nunca input externo).
	rows, err := q.db.QueryContext(ctx, `
SELECT pde.empleado_id, COALESCE(SUM(pdc.monto), 0), COUNT(DISTINCT pp.id)
FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
WHERE pp.estado <> 'borrador'
  AND tp.codigo NOT IN ('aguinaldo', 'liquidacion')
  AND pp.fecha_fin >= ?
  AND pp.fecha_fin <= ?
  AND cs.naturaleza = 'ingreso'
  AND `+flagCol+` = 1
GROUP BY pde.empleado_id
ORDER BY pde.empleado_id ASC`, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BaseAcumuladaEmpleadoRow
	for rows.Next() {
		var r BaseAcumuladaEmpleadoRow
		if err := rows.Scan(&r.EmpleadoID, &r.TotalBase, &r.PlanillasCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListarEmpleadosConAguinaldoEnAnio / ...SalarioEscolar...: empleados que ya
// tienen registro (empleado_id, anio) — UNIQUE en ambas tablas — para que la
// corrida sea idempotente.
func (q *Queries) ListarEmpleadosConAguinaldoEnAnio(ctx context.Context, anio int32) (map[int64]bool, error) {
	return q.listarIDs(ctx, `SELECT empleado_id FROM aguinaldos WHERE anio = ?`, anio)
}

func (q *Queries) ListarEmpleadosConSalarioEscolarEnAnio(ctx context.Context, anio int32) (map[int64]bool, error) {
	return q.listarIDs(ctx, `SELECT empleado_id FROM salarios_escolares WHERE anio = ?`, anio)
}

func (q *Queries) listarIDs(ctx context.Context, query string, arg interface{}) (map[int64]bool, error) {
	rows, err := q.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ObtenerPorcentajeSalarioEscolarEnFecha: porcentaje (fracción, ej. 0.0833)
// vigente a `fecha`. A diferencia de GetPorcentajeSalarioEscolarVigente
// (es_vigente = 1, o sea "hoy"), sirve para corridas retroactivas.
func (q *Queries) ObtenerPorcentajeSalarioEscolarEnFecha(ctx context.Context, fecha time.Time) (decimal.Decimal, error) {
	var p decimal.Decimal
	err := q.db.QueryRowContext(ctx, `
SELECT porcentaje
FROM parametros_salario_escolar
WHERE fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY fecha_inicio_vigencia DESC
LIMIT 1`, fecha, fecha).Scan(&p)
	return p, err
}
