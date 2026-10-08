// internal/db/dto/acumulados_liquidaciones.go
//
// Fase 7 — Acumulados (Aguinaldo, Salario Escolar) y Liquidaciones Laborales.
// Reemplaza db/queries/acumulados_liquidaciones.sql (referencia legible; ya no
// se compila). Helpers (rowScanner) en asistencia.go. Estilo de contratos.go /
// vacaciones.go: argumentos posicionales para queries chicas y un struct
// *Params para los INSERT anchos.
//
// NOTA DE MIGRACIÓN:
//
//   - Las columnas de aguinaldos, salarios_escolares, liquidaciones y
//     parametros_salario_escolar salen de los INSERT del .sql (migración
//     000004), NO de db/schema.sql (no disponible al escribir esto). Los SELECT
//     de este archivo listan columnas explícitas en vez de `SELECT *`; si el
//     schema real tiene columnas extra (created_at, updated_at, ...) o tipos
//     distintos, ajustar structs + columnas* aquí.
//   - CORRECCIÓN respecto al .sql: GetSaldoVacacionesDisponibles hacía
//     `WHERE empleado_id = ? LIMIT 1` asumiendo una fila por empleado.
//     vacaciones_acumulado es una tabla de CORTES históricos (UNIQUE
//     empleado_id+fecha_corte, ver vacaciones.go), así que sin ORDER BY
//     devolvería un corte arbitrario. Ahora toma el más reciente, igual que
//     ObtenerSaldoVacacionesEmpleado.
//   - Tipos de liquidaciones: dias_preaviso y antiguedad_meses como int32;
//     dias_cesantia y dias_vacaciones_pendientes como decimal (pueden ser
//     fraccionarios). Ajustar si la migración los definió distinto.
//   - CerrarPorcentajeSalarioEscolarVigente + CrearVersionPorcentajeSalarioEscolar
//     son un patrón de vigencia de 2 pasos (ADR-000 §2): el llamador DEBE
//     ejecutarlos en la misma Tx (db.WithAuditContext / WithTx).
package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// SALARIOS BRUTOS DEVENGADOS (fuente compartida por aguinaldo, salario
// escolar y liquidación). Solo periodos con estado = 'calculado'.
// ============================================================================

type SumarSalariosBrutosPeriodoRow struct {
	TotalSalariosBrutos decimal.Decimal
	PeriodosIncluidos   int64
}

// SumarSalariosBrutosPeriodo suma total_ingresos (bruto, no neto) de
// planilla_detalle_empleado para un empleado, filtrando por la fecha de fin de
// cada periodo entre fechaDesde y fechaHasta (ambas inclusivas).
func (q *Queries) SumarSalariosBrutosPeriodo(ctx context.Context, empleadoID int64, fechaDesde, fechaHasta time.Time) (SumarSalariosBrutosPeriodoRow, error) {
	var r SumarSalariosBrutosPeriodoRow
	err := q.db.QueryRowContext(ctx, `
SELECT
    COALESCE(SUM(pde.total_ingresos), 0),
    COUNT(*)
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_id
WHERE pde.empleado_id = ?
  AND pp.estado = 'calculado'
  AND pp.fecha_fin_periodo >= ?
  AND pp.fecha_fin_periodo <= ?`, empleadoID, fechaDesde, fechaHasta).
		Scan(&r.TotalSalariosBrutos, &r.PeriodosIncluidos)
	return r, err
}

type SalarioBrutoPeriodo struct {
	PeriodoID          int64
	FechaInicioPeriodo time.Time
	FechaFinPeriodo    time.Time
	TotalIngresos      decimal.Decimal
}

// ListSalariosBrutosUltimosPeriodos devuelve el detalle por periodo de los
// últimos `limit` periodos calculados hasta fechaCorte, más reciente primero.
// Se trae el detalle (no solo el agregado) para que el llamador decida si
// excluye periodos con salario cero/incompleto; esta query no toma esa
// decisión de negocio.
func (q *Queries) ListSalariosBrutosUltimosPeriodos(ctx context.Context, empleadoID int64, fechaCorte time.Time, limit int32) ([]SalarioBrutoPeriodo, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT pp.id, pp.fecha_inicio_periodo, pp.fecha_fin_periodo, pde.total_ingresos
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_id
WHERE pde.empleado_id = ?
  AND pp.estado = 'calculado'
  AND pp.fecha_fin_periodo <= ?
ORDER BY pp.fecha_fin_periodo DESC
LIMIT ?`, empleadoID, fechaCorte, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SalarioBrutoPeriodo
	for rows.Next() {
		var s SalarioBrutoPeriodo
		if err := rows.Scan(&s.PeriodoID, &s.FechaInicioPeriodo, &s.FechaFinPeriodo, &s.TotalIngresos); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ============================================================================
// PARAMETROS_SALARIO_ESCOLAR (vigencia — único registro vigente global)
// ============================================================================

type ParametroSalarioEscolar struct {
	ID                  int64
	Porcentaje          decimal.Decimal
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
	EsVigente           bool
	CreadoPor           int64
	CreatedAt           time.Time
}

func (q *Queries) GetPorcentajeSalarioEscolarVigente(ctx context.Context) (ParametroSalarioEscolar, error) {
	var p ParametroSalarioEscolar
	err := q.db.QueryRowContext(ctx, `
SELECT id, porcentaje, fecha_inicio_vigencia, fecha_fin_vigencia, es_vigente, creado_por, created_at
FROM parametros_salario_escolar
WHERE es_vigente = 1
LIMIT 1`).Scan(&p.ID, &p.Porcentaje, &p.FechaInicioVigencia, &p.FechaFinVigencia,
		&p.EsVigente, &p.CreadoPor, &p.CreatedAt)
	return p, err
}

// CerrarPorcentajeSalarioEscolarVigente paso 1 de 2 (misma Tx que CrearVersion...).
func (q *Queries) CerrarPorcentajeSalarioEscolarVigente(ctx context.Context, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE parametros_salario_escolar
SET fecha_fin_vigencia = ?
WHERE es_vigente = 1`, fechaFinVigencia)
	return err
}

// CrearVersionPorcentajeSalarioEscolar paso 2 de 2.
func (q *Queries) CrearVersionPorcentajeSalarioEscolar(ctx context.Context, porcentaje decimal.Decimal, fechaInicioVigencia time.Time, creadoPor int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO parametros_salario_escolar (
    porcentaje, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por, created_at
) VALUES (?, ?, NULL, ?, NOW())`, porcentaje, fechaInicioVigencia, creadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ============================================================================
// VACACIONES (saldo materializado — ver vacaciones.go)
// ============================================================================

// GetSaldoVacacionesDisponibles devuelve dias_disponibles del corte más
// reciente del empleado (ver nota de migración arriba). sql.ErrNoRows si el
// empleado no tiene ningún corte.
func (q *Queries) GetSaldoVacacionesDisponibles(ctx context.Context, empleadoID int64) (decimal.Decimal, error) {
	var dias decimal.Decimal
	err := q.db.QueryRowContext(ctx, `
SELECT dias_disponibles
FROM vacaciones_acumulado
WHERE empleado_id = ?
ORDER BY fecha_corte DESC, id DESC
LIMIT 1`, empleadoID).Scan(&dias)
	return dias, err
}

// ============================================================================
// AGUINALDOS (un registro por empleado+anio)
// ============================================================================

type Aguinaldo struct {
	ID                  int64
	EmpleadoID          int64
	Anio                int32
	PeriodoDesde        time.Time
	PeriodoHasta        time.Time
	TotalSalariosBrutos decimal.Decimal
	MontoAguinaldo      decimal.Decimal
	Estado              string
	CalculadoEn         time.Time
	ProcesadoPor        sql.NullInt64
	ProcesadoEn         sql.NullTime
}

const columnasAguinaldo = `id, empleado_id, anio, periodo_desde, periodo_hasta,
       total_salarios_brutos, monto_aguinaldo, estado, calculado_en, procesado_por, procesado_en`

func scanAguinaldo(s rowScanner) (Aguinaldo, error) {
	var a Aguinaldo
	err := s.Scan(&a.ID, &a.EmpleadoID, &a.Anio, &a.PeriodoDesde, &a.PeriodoHasta,
		&a.TotalSalariosBrutos, &a.MontoAguinaldo, &a.Estado, &a.CalculadoEn, &a.ProcesadoPor, &a.ProcesadoEn)
	return a, err
}

func (q *Queries) GetAguinaldoPorAnio(ctx context.Context, empleadoID int64, anio int32) (Aguinaldo, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT `+columnasAguinaldo+`
FROM aguinaldos
WHERE empleado_id = ? AND anio = ?
LIMIT 1`, empleadoID, anio)
	return scanAguinaldo(row)
}

func (q *Queries) ListAguinaldosPorEmpleado(ctx context.Context, empleadoID int64, limit, offset int32) ([]Aguinaldo, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasAguinaldo+`
FROM aguinaldos
WHERE empleado_id = ?
ORDER BY anio DESC
LIMIT ? OFFSET ?`, empleadoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Aguinaldo
	for rows.Next() {
		a, err := scanAguinaldo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type CrearAguinaldoParams struct {
	EmpleadoID          int64
	Anio                int32
	PeriodoDesde        time.Time
	PeriodoHasta        time.Time
	TotalSalariosBrutos decimal.Decimal
	MontoAguinaldo      decimal.Decimal
	ProcesadoPor        int64
}

// CrearAguinaldo inserta ya en estado 'procesado'. Si se quiere "proyectar"
// sin persistir, el servicio simplemente no llama esta función.
func (q *Queries) CrearAguinaldo(ctx context.Context, arg CrearAguinaldoParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO aguinaldos (
    empleado_id, anio, periodo_desde, periodo_hasta,
    total_salarios_brutos, monto_aguinaldo,
    estado, calculado_en, procesado_por, procesado_en
) VALUES (?, ?, ?, ?, ?, ?, 'procesado', NOW(), ?, NOW())`,
		arg.EmpleadoID, arg.Anio, arg.PeriodoDesde, arg.PeriodoHasta,
		arg.TotalSalariosBrutos, arg.MontoAguinaldo, arg.ProcesadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ============================================================================
// SALARIOS_ESCOLARES (un registro por empleado+anio)
// ============================================================================

type SalarioEscolar struct {
	ID                  int64
	EmpleadoID          int64
	Anio                int32
	PeriodoDesde        time.Time
	PeriodoHasta        time.Time
	PorcentajeAplicado  decimal.Decimal
	TotalSalariosBrutos decimal.Decimal
	MontoSalarioEscolar decimal.Decimal
	Estado              string
	CalculadoEn         time.Time
	ProcesadoPor        sql.NullInt64
	ProcesadoEn         sql.NullTime
}

const columnasSalarioEscolar = `id, empleado_id, anio, periodo_desde, periodo_hasta,
       porcentaje_aplicado, total_salarios_brutos, monto_salario_escolar,
       estado, calculado_en, procesado_por, procesado_en`

func scanSalarioEscolar(s rowScanner) (SalarioEscolar, error) {
	var e SalarioEscolar
	err := s.Scan(&e.ID, &e.EmpleadoID, &e.Anio, &e.PeriodoDesde, &e.PeriodoHasta,
		&e.PorcentajeAplicado, &e.TotalSalariosBrutos, &e.MontoSalarioEscolar,
		&e.Estado, &e.CalculadoEn, &e.ProcesadoPor, &e.ProcesadoEn)
	return e, err
}

func (q *Queries) GetSalarioEscolarPorAnio(ctx context.Context, empleadoID int64, anio int32) (SalarioEscolar, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT `+columnasSalarioEscolar+`
FROM salarios_escolares
WHERE empleado_id = ? AND anio = ?
LIMIT 1`, empleadoID, anio)
	return scanSalarioEscolar(row)
}

func (q *Queries) ListSalariosEscolaresPorEmpleado(ctx context.Context, empleadoID int64, limit, offset int32) ([]SalarioEscolar, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasSalarioEscolar+`
FROM salarios_escolares
WHERE empleado_id = ?
ORDER BY anio DESC
LIMIT ? OFFSET ?`, empleadoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SalarioEscolar
	for rows.Next() {
		e, err := scanSalarioEscolar(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type CrearSalarioEscolarParams struct {
	EmpleadoID          int64
	Anio                int32
	PeriodoDesde        time.Time
	PeriodoHasta        time.Time
	PorcentajeAplicado  decimal.Decimal
	TotalSalariosBrutos decimal.Decimal
	MontoSalarioEscolar decimal.Decimal
	ProcesadoPor        int64
}

func (q *Queries) CrearSalarioEscolar(ctx context.Context, arg CrearSalarioEscolarParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO salarios_escolares (
    empleado_id, anio, periodo_desde, periodo_hasta,
    porcentaje_aplicado, total_salarios_brutos, monto_salario_escolar,
    estado, calculado_en, procesado_por, procesado_en
) VALUES (?, ?, ?, ?, ?, ?, ?, 'procesado', NOW(), ?, NOW())`,
		arg.EmpleadoID, arg.Anio, arg.PeriodoDesde, arg.PeriodoHasta,
		arg.PorcentajeAplicado, arg.TotalSalariosBrutos, arg.MontoSalarioEscolar, arg.ProcesadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ============================================================================
// LIQUIDACIONES — SOLO se inserta en /procesar; /proyectar nunca llama
// CrearLiquidacion. Debe ir en la MISMA Tx que el cierre de expediente
// (CerrarContratoVigente, CerrarDatosLaboralesVigente, SetEmpleadoActivo),
// que ya existen en contratos.go / empleados.go y no se duplican aquí.
// ============================================================================

type Liquidacion struct {
	ID                                     int64
	EmpleadoID                             int64
	FechaSalida                            time.Time
	MotivoSalida                           string
	AplicaResponsabilidadPatronal          bool
	FechaIngreso                           time.Time
	AntiguedadMeses                        int32
	SalarioPromedio6Meses                  decimal.Decimal
	SalarioDiarioPromedio                  decimal.Decimal
	DiasPreaviso                           int32
	MontoPreaviso                          decimal.Decimal
	DiasCesantia                           decimal.Decimal
	MontoCesantia                          decimal.Decimal
	DiasVacacionesPendientes               decimal.Decimal
	MontoVacacionesProporcionales          decimal.Decimal
	PeriodoAguinaldoDesde                  time.Time
	TotalSalariosBrutosAguinaldoProporcion decimal.Decimal
	MontoAguinaldoProporcional             decimal.Decimal
	TotalLiquidacion                       decimal.Decimal
	Estado                                 string
	CalculadoEn                            time.Time
	ProcesadoPor                           sql.NullInt64
	ProcesadoEn                            sql.NullTime
}

const columnasLiquidacion = `id, empleado_id, fecha_salida, motivo_salida, aplica_responsabilidad_patronal,
       fecha_ingreso, antiguedad_meses, salario_promedio_6meses, salario_diario_promedio,
       dias_preaviso, monto_preaviso, dias_cesantia, monto_cesantia,
       dias_vacaciones_pendientes, monto_vacaciones_proporcionales,
       periodo_aguinaldo_desde, total_salarios_brutos_aguinaldo_proporcional, monto_aguinaldo_proporcional,
       total_liquidacion, estado, calculado_en, procesado_por, procesado_en`

func scanLiquidacion(s rowScanner) (Liquidacion, error) {
	var l Liquidacion
	err := s.Scan(&l.ID, &l.EmpleadoID, &l.FechaSalida, &l.MotivoSalida, &l.AplicaResponsabilidadPatronal,
		&l.FechaIngreso, &l.AntiguedadMeses, &l.SalarioPromedio6Meses, &l.SalarioDiarioPromedio,
		&l.DiasPreaviso, &l.MontoPreaviso, &l.DiasCesantia, &l.MontoCesantia,
		&l.DiasVacacionesPendientes, &l.MontoVacacionesProporcionales,
		&l.PeriodoAguinaldoDesde, &l.TotalSalariosBrutosAguinaldoProporcion, &l.MontoAguinaldoProporcional,
		&l.TotalLiquidacion, &l.Estado, &l.CalculadoEn, &l.ProcesadoPor, &l.ProcesadoEn)
	return l, err
}

func (q *Queries) GetLiquidacionByID(ctx context.Context, id int64) (Liquidacion, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT `+columnasLiquidacion+`
FROM liquidaciones
WHERE id = ?
LIMIT 1`, id)
	return scanLiquidacion(row)
}

func (q *Queries) ListLiquidacionesPorEmpleado(ctx context.Context, empleadoID int64, limit, offset int32) ([]Liquidacion, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasLiquidacion+`
FROM liquidaciones
WHERE empleado_id = ?
ORDER BY fecha_salida DESC
LIMIT ? OFFSET ?`, empleadoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Liquidacion
	for rows.Next() {
		l, err := scanLiquidacion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

type CrearLiquidacionParams struct {
	EmpleadoID                             int64
	FechaSalida                            time.Time
	MotivoSalida                           string
	AplicaResponsabilidadPatronal          bool
	FechaIngreso                           time.Time
	AntiguedadMeses                        int32
	SalarioPromedio6Meses                  decimal.Decimal
	SalarioDiarioPromedio                  decimal.Decimal
	DiasPreaviso                           int32
	MontoPreaviso                          decimal.Decimal
	DiasCesantia                           decimal.Decimal
	MontoCesantia                          decimal.Decimal
	DiasVacacionesPendientes               decimal.Decimal
	MontoVacacionesProporcionales          decimal.Decimal
	PeriodoAguinaldoDesde                  time.Time
	TotalSalariosBrutosAguinaldoProporcion decimal.Decimal
	MontoAguinaldoProporcional             decimal.Decimal
	TotalLiquidacion                       decimal.Decimal
	ProcesadoPor                           int64
}

// CrearLiquidacion inserta en estado 'procesada'.
func (q *Queries) CrearLiquidacion(ctx context.Context, arg CrearLiquidacionParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO liquidaciones (
    empleado_id, fecha_salida, motivo_salida, aplica_responsabilidad_patronal,
    fecha_ingreso, antiguedad_meses,
    salario_promedio_6meses, salario_diario_promedio,
    dias_preaviso, monto_preaviso,
    dias_cesantia, monto_cesantia,
    dias_vacaciones_pendientes, monto_vacaciones_proporcionales,
    periodo_aguinaldo_desde, total_salarios_brutos_aguinaldo_proporcional, monto_aguinaldo_proporcional,
    total_liquidacion, estado, calculado_en, procesado_por, procesado_en
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'procesada', NOW(), ?, NOW())`,
		arg.EmpleadoID, arg.FechaSalida, arg.MotivoSalida, arg.AplicaResponsabilidadPatronal,
		arg.FechaIngreso, arg.AntiguedadMeses,
		arg.SalarioPromedio6Meses, arg.SalarioDiarioPromedio,
		arg.DiasPreaviso, arg.MontoPreaviso,
		arg.DiasCesantia, arg.MontoCesantia,
		arg.DiasVacacionesPendientes, arg.MontoVacacionesProporcionales,
		arg.PeriodoAguinaldoDesde, arg.TotalSalariosBrutosAguinaldoProporcion, arg.MontoAguinaldoProporcional,
		arg.TotalLiquidacion, arg.ProcesadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}