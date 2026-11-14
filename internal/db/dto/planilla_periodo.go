package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

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

type CargaSocialVigenteRow struct {
	ConceptoSalarialID int64
	ConceptoCodigo     string
	RegimenCCSSID      sql.NullInt64   // NULL = aplica a todo régimen CCSS sin fila propia
	PorcentajePatronal decimal.Decimal // 0-100
	PorcentajeObrero   decimal.Decimal // 0-100
}

// ListarCargasSocialesEnFecha: filas de parametros_cargas_sociales vigentes,
// resueltas por concepto_salarial_id (nunca por el texto de `rubro`).
func (q *Queries) ListarCargasSocialesEnFecha(ctx context.Context, fecha time.Time) ([]CargaSocialVigenteRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT pcs.concepto_salarial_id, cs.codigo, pcs.regimen_ccss_id, pcs.porcentaje_patronal, pcs.porcentaje_obrero
FROM parametros_cargas_sociales pcs
JOIN conceptos_salariales cs ON cs.id = pcs.concepto_salarial_id
WHERE pcs.fecha_inicio_vigencia <= ?
  AND (pcs.fecha_fin_vigencia IS NULL OR pcs.fecha_fin_vigencia >= ?)
ORDER BY cs.codigo ASC, pcs.regimen_ccss_id ASC, pcs.id ASC`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CargaSocialVigenteRow
	for rows.Next() {
		var r CargaSocialVigenteRow
		if err := rows.Scan(&r.ConceptoSalarialID, &r.ConceptoCodigo, &r.RegimenCCSSID, &r.PorcentajePatronal, &r.PorcentajeObrero); err != nil {
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

type TramoEscalaEmbargoVigenteRow struct {
	DesdeMultiplo    decimal.Decimal
	HastaMultiplo    decimal.NullDecimal
	PorcentajeMaximo decimal.Decimal // 0-100
}

func (q *Queries) ListarEscalaEmbargoEnFecha(ctx context.Context, fecha time.Time) ([]TramoEscalaEmbargoVigenteRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT desde_multiplo, hasta_multiplo, porcentaje_maximo
FROM parametros_embargo_escala
WHERE fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY desde_multiplo ASC`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TramoEscalaEmbargoVigenteRow
	for rows.Next() {
		var r TramoEscalaEmbargoVigenteRow
		if err := rows.Scan(&r.DesdeMultiplo, &r.HastaMultiplo, &r.PorcentajeMaximo); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type BaseMinimaContributivaVigenteRow struct {
	RegimenCCSSID int64
	Monto         decimal.Decimal
}

func (q *Queries) ListarBaseMinimaContributivaEnFecha(ctx context.Context, fecha time.Time) ([]BaseMinimaContributivaVigenteRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT regimen_ccss_id, monto
FROM parametros_base_minima_contributiva
WHERE fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY regimen_ccss_id ASC`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BaseMinimaContributivaVigenteRow
	for rows.Next() {
		var r BaseMinimaContributivaVigenteRow
		if err := rows.Scan(&r.RegimenCCSSID, &r.Monto); err != nil {
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

type EmbargoActivoRow struct {
	EmbargoID             int64
	TipoEmbargoID         int64
	PrioridadLegal        int32
	PrioridadAplicacion   int32
	EsPension             bool
	TopePorcentajeLiquido decimal.NullDecimal // 0-100, del tipo (NULL = sin tope)
	Porcentaje            decimal.NullDecimal // 0-100
	MontoFijo             decimal.NullDecimal
	TopeMensual           decimal.NullDecimal // embargos.tope_mensual
}

// ListarEmbargosActivosEnFecha: embargos (tabla `embargos`, catálogo del
// empleado) activos y vigentes a `fecha`, por prioridad legal. La tabla
// planilla_detalle_embargo es el RESULTADO persistido de una corrida, no la
// fuente de embargos activos.
func (q *Queries) ListarEmbargosActivosEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]EmbargoActivoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT e.id, e.tipo_embargo_id, te.prioridad_legal, e.prioridad_aplicacion, te.es_pension_alimentaria,
       te.tope_porcentaje_liquido, e.porcentaje, e.monto_fijo, e.tope_mensual
FROM embargos e
JOIN parametros_tipos_embargo te ON te.id = e.tipo_embargo_id
WHERE e.empleado_id = ?
  AND e.estado = 'activo'
  AND e.fecha_inicio_vigencia <= ?
  AND (e.fecha_fin_vigencia IS NULL OR e.fecha_fin_vigencia >= ?)
ORDER BY te.prioridad_legal ASC, e.prioridad_aplicacion ASC, e.id ASC`, empleadoID, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EmbargoActivoRow
	for rows.Next() {
		var r EmbargoActivoRow
		if err := rows.Scan(&r.EmbargoID, &r.TipoEmbargoID, &r.PrioridadLegal, &r.PrioridadAplicacion, &r.EsPension,
			&r.TopePorcentajeLiquido, &r.Porcentaje, &r.MontoFijo, &r.TopeMensual); err != nil {
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

type HorasFeriadoLaboradoRow struct {
	PagoObligatorio bool
	Horas           decimal.Decimal
}

// SumarHorasFeriadoLaboradoPeriodo: horas ordinarias de los días presentes
// que el calendario laboral marca como feriado, por tipo de pago del feriado.
func (q *Queries) SumarHorasFeriadoLaboradoPeriodo(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]HorasFeriadoLaboradoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT f.pago_obligatorio, SUM(r.horas_ordinarias)
FROM registro_asistencia_detalle r
JOIN calendario_laboral_dia c ON c.fecha = r.fecha AND c.es_feriado = 1
JOIN feriados f ON f.id = c.feriado_id
WHERE r.empleado_id = ? AND r.fecha >= ? AND r.fecha <= ?
  AND r.estado = 'presente' AND r.horas_ordinarias > 0
GROUP BY f.pago_obligatorio
ORDER BY f.pago_obligatorio DESC`, empleadoID, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HorasFeriadoLaboradoRow
	for rows.Next() {
		var r HorasFeriadoLaboradoRow
		if err := rows.Scan(&r.PagoObligatorio, &r.Horas); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type HorasExtraPeriodoRow struct {
	TipoHoraExtraID int64
	Horas           decimal.Decimal
}

// SumarHorasExtraPeriodo suma, por tipo, las horas extra tipificadas en
// [desde, hasta] que estén autorizadas: solo cuentan los días con
// horas_extra_autorizadas en estado 'autorizada', y de cada día se paga como
// máximo horas_autorizadas. Si la autorización fija un tipo, solo cuenta ese
// tipo; si no, el tope se reparte entre los tipos del día en orden de id.
func (q *Queries) SumarHorasExtraPeriodo(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]HorasExtraPeriodoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
WITH dia AS (
    SELECT h.fecha, h.tipo_hora_extra_id AS tipo, h.horas
    FROM asistencia_hora_extra_tipificada h
    JOIN tipos_hora_extra t ON t.id = h.tipo_hora_extra_id AND t.activo = 1
    WHERE h.empleado_id = ? AND h.fecha >= ? AND h.fecha <= ?
),
autorizado AS (
    SELECT d.fecha, d.tipo, d.horas, a.horas_autorizadas AS tope
    FROM dia d
    JOIN horas_extra_autorizadas a
      ON a.empleado_id = ? AND a.fecha = d.fecha AND a.estado = 'autorizada'
    WHERE d.horas > 0 AND (a.tipo_hora_extra_id IS NULL OR a.tipo_hora_extra_id = d.tipo)
),
acumulado AS (
    SELECT tipo, horas, tope,
           SUM(horas) OVER (PARTITION BY fecha ORDER BY tipo) - horas AS previas
    FROM autorizado
)
SELECT tipo, SUM(GREATEST(LEAST(horas, tope - previas), 0)) AS total
FROM acumulado
GROUP BY tipo
HAVING total > 0
ORDER BY tipo ASC`, empleadoID, desde, hasta, empleadoID)
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
