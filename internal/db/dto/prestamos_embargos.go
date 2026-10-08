// internal/db/dto/prestamos_embargos.go
//
// Fase 5 — Préstamos, Embargos y Terceros. Estilo D2: SQL plano con `?`,
// Scan manual, decimal.Decimal para todo DECIMAL y sql.Null* /
// decimal.NullDecimal para columnas nullable. Helpers (rowScanner) en
// asistencia.go.
//
// F05: este archivo ahora está verificado columna por columna contra el
// DDL real de db/schema.sql (tablas `terceros`, `prestamos`, `embargos`,
// `parametros_tipos_embargo`, `empleados`, `contratos`). La versión anterior
// se había escrito contra "supuestos de esquema" que NO existen en el DDL
// (p. ej. prestamos.descripcion/cuotas_totales/cuotas_pagadas/creado_por,
// terceros.activo/identificacion, embargos.creado_por, estado 'cancelado'
// en embargos) y omitía columnas NOT NULL (embargos.prioridad_aplicacion).
//
// RECORDATORIO ADR-000: `embargos` y `prestamos` NO usan el patrón de
// vigencia de 2 pasos; su ciclo de vida es por `estado`.
//
// Métodos *ParaActualizar: SELECT ... FOR UPDATE. Solo tienen sentido dentro
// de una transacción (db.WithAuditContext); fuera de ella el lock se libera
// al terminar la sentencia y no protege nada.
package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// TERCEROS  (DDL: id, nombre, tipo ENUM('juzgado','entidad_financiera','otro'))
// ============================================================================

type Tercero struct {
	ID     int64
	Nombre string
	Tipo   string // juzgado | entidad_financiera | otro
}

const columnasTercero = `id, nombre, tipo`

func scanTercero(s rowScanner) (Tercero, error) {
	var t Tercero
	err := s.Scan(&t.ID, &t.Nombre, &t.Tipo)
	return t, err
}

type CreateTerceroParams struct {
	Nombre string
	Tipo   string
}

func (q *Queries) CreateTercero(ctx context.Context, arg CreateTerceroParams) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`INSERT INTO terceros (nombre, tipo) VALUES (?, ?)`, arg.Nombre, arg.Tipo)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) GetTercero(ctx context.Context, id int64) (Tercero, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasTercero+` FROM terceros WHERE id = ?`, id)
	return scanTercero(row)
}

// ListTercerosParams: Tipo y Nombre vacíos ("") significan "sin filtro".
type ListTercerosParams struct {
	Tipo   string
	Nombre string
	Limit  int32
	Offset int32
}

func (q *Queries) ListTerceros(ctx context.Context, arg ListTercerosParams) ([]Tercero, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasTercero+`
FROM terceros
WHERE (? = '' OR tipo = ?)
  AND (? = '' OR nombre LIKE CONCAT('%', ?, '%'))
ORDER BY nombre ASC, id ASC
LIMIT ? OFFSET ?`, arg.Tipo, arg.Tipo, arg.Nombre, arg.Nombre, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Tercero
	for rows.Next() {
		t, err := scanTercero(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type CountTercerosParams struct {
	Tipo   string
	Nombre string
}

func (q *Queries) CountTerceros(ctx context.Context, arg CountTercerosParams) (int64, error) {
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM terceros
WHERE (? = '' OR tipo = ?)
  AND (? = '' OR nombre LIKE CONCAT('%', ?, '%'))`,
		arg.Tipo, arg.Tipo, arg.Nombre, arg.Nombre).Scan(&total)
	return total, err
}

type UpdateTerceroParams struct {
	Nombre string
	Tipo   string
	ID     int64
}

func (q *Queries) UpdateTercero(ctx context.Context, arg UpdateTerceroParams) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE terceros SET nombre = ?, tipo = ? WHERE id = ?`, arg.Nombre, arg.Tipo, arg.ID)
	return err
}

// ============================================================================
// EMPLEADOS / CONTRATOS — solo las lecturas que necesita este módulo
// ============================================================================

// EmpleadoEstado es la proyección mínima de `empleados` para validar altas.
type EmpleadoEstado struct {
	ID     int64
	Activo bool
}

// GetEmpleadoParaActualizar bloquea la fila del empleado (FOR UPDATE). Sirve
// para serializar altas concurrentes para el mismo empleado (p. ej. la
// verificación de préstamo duplicado). Devuelve sql.ErrNoRows si no existe.
func (q *Queries) GetEmpleadoParaActualizar(ctx context.Context, id int64) (EmpleadoEstado, error) {
	var e EmpleadoEstado
	err := q.db.QueryRowContext(ctx,
		`SELECT id, activo FROM empleados WHERE id = ? FOR UPDATE`, id).Scan(&e.ID, &e.Activo)
	return e, err
}

// ExisteContratoVigente: hay una versión de contrato vigente (es_vigente = 1,
// o sea fecha_fin_vigencia IS NULL) cuya fecha_fin_contrato, si existe, no ha
// pasado. Se evalúa contra CURDATE() del servidor de BD.
func (q *Queries) ExisteContratoVigente(ctx context.Context, empleadoID int64) (bool, error) {
	var existe bool
	err := q.db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM contratos
    WHERE empleado_id = ?
      AND es_vigente = 1
      AND fecha_inicio_contrato <= CURDATE()
      AND (fecha_fin_contrato IS NULL OR fecha_fin_contrato >= CURDATE())
)`, empleadoID).Scan(&existe)
	return existe, err
}

// ============================================================================
// PRESTAMOS
// DDL: id, empleado_id, tercero_id (NOT NULL), monto_total, cuota_periodica,
//      saldo_pendiente, fecha_inicio, fecha_fin_estimada, estado
//      ENUM('activo','cancelado','suspendido'), origen_carga
//      ENUM('operacion','migracion_inicial'), registrado_por_usuario_id.
//      CHECK: 0 <= saldo_pendiente <= monto_total.
// ============================================================================

type Prestamo struct {
	ID                     int64
	EmpleadoID             int64
	TerceroID              int64
	MontoTotal             decimal.Decimal
	CuotaPeriodica         decimal.Decimal
	SaldoPendiente         decimal.Decimal
	FechaInicio            time.Time
	FechaFinEstimada       sql.NullTime
	Estado                 string // activo | cancelado | suspendido
	OrigenCarga            string // operacion | migracion_inicial
	RegistradoPorUsuarioID sql.NullInt64
}

const columnasPrestamo = `id, empleado_id, tercero_id, monto_total, cuota_periodica,
       saldo_pendiente, fecha_inicio, fecha_fin_estimada, estado,
       origen_carga, registrado_por_usuario_id`

func scanPrestamo(s rowScanner) (Prestamo, error) {
	var p Prestamo
	err := s.Scan(&p.ID, &p.EmpleadoID, &p.TerceroID, &p.MontoTotal, &p.CuotaPeriodica,
		&p.SaldoPendiente, &p.FechaInicio, &p.FechaFinEstimada, &p.Estado,
		&p.OrigenCarga, &p.RegistradoPorUsuarioID)
	return p, err
}

func scanPrestamos(rows *sql.Rows) ([]Prestamo, error) {
	var out []Prestamo
	for rows.Next() {
		p, err := scanPrestamo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type CreatePrestamoParams struct {
	EmpleadoID             int64
	TerceroID              int64
	MontoTotal             decimal.Decimal
	CuotaPeriodica         decimal.Decimal
	SaldoPendiente         decimal.Decimal
	FechaInicio            time.Time
	FechaFinEstimada       sql.NullTime
	RegistradoPorUsuarioID sql.NullInt64
}

// CreatePrestamo inserta con estado 'activo' y origen_carga 'operacion'.
func (q *Queries) CreatePrestamo(ctx context.Context, arg CreatePrestamoParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO prestamos (
    empleado_id, tercero_id, monto_total, cuota_periodica, saldo_pendiente,
    fecha_inicio, fecha_fin_estimada, estado, origen_carga, registrado_por_usuario_id
) VALUES (?, ?, ?, ?, ?, ?, ?, 'activo', 'operacion', ?)`,
		arg.EmpleadoID, arg.TerceroID, arg.MontoTotal, arg.CuotaPeriodica, arg.SaldoPendiente,
		arg.FechaInicio, arg.FechaFinEstimada, arg.RegistradoPorUsuarioID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) GetPrestamo(ctx context.Context, id int64) (Prestamo, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasPrestamo+` FROM prestamos WHERE id = ?`, id)
	return scanPrestamo(row)
}

// GetPrestamoParaActualizar: lectura con FOR UPDATE. Solo dentro de una Tx.
func (q *Queries) GetPrestamoParaActualizar(ctx context.Context, id int64) (Prestamo, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasPrestamo+` FROM prestamos WHERE id = ? FOR UPDATE`, id)
	return scanPrestamo(row)
}

// ExistePrestamoDuplicadoParams define "idéntico": mismo empleado, tercero,
// monto, cuota y fecha de inicio, en un préstamo todavía no cancelado.
type ExistePrestamoDuplicadoParams struct {
	EmpleadoID     int64
	TerceroID      int64
	MontoTotal     decimal.Decimal
	CuotaPeriodica decimal.Decimal
	FechaInicio    time.Time
}

func (q *Queries) ExistePrestamoDuplicado(ctx context.Context, arg ExistePrestamoDuplicadoParams) (bool, error) {
	var existe bool
	err := q.db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM prestamos
    WHERE empleado_id = ? AND tercero_id = ?
      AND monto_total = ? AND cuota_periodica = ?
      AND fecha_inicio = DATE(?)
      AND estado IN ('activo', 'suspendido')
)`, arg.EmpleadoID, arg.TerceroID, arg.MontoTotal, arg.CuotaPeriodica, arg.FechaInicio).Scan(&existe)
	return existe, err
}

// ListPrestamosPorEmpleadoParams: Estado "" = sin filtro.
type ListPrestamosPorEmpleadoParams struct {
	EmpleadoID int64
	Estado     string
	Limit      int32
	Offset     int32
}

func (q *Queries) ListPrestamosPorEmpleado(ctx context.Context, arg ListPrestamosPorEmpleadoParams) ([]Prestamo, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasPrestamo+`
FROM prestamos
WHERE empleado_id = ?
  AND (? = '' OR estado = ?)
ORDER BY fecha_inicio DESC, id DESC
LIMIT ? OFFSET ?`, arg.EmpleadoID, arg.Estado, arg.Estado, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrestamos(rows)
}

type CountPrestamosPorEmpleadoParams struct {
	EmpleadoID int64
	Estado     string
}

func (q *Queries) CountPrestamosPorEmpleado(ctx context.Context, arg CountPrestamosPorEmpleadoParams) (int64, error) {
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM prestamos
WHERE empleado_id = ?
  AND (? = '' OR estado = ?)`, arg.EmpleadoID, arg.Estado, arg.Estado).Scan(&total)
	return total, err
}

// ListPrestamosActivosPorEmpleado: usado por el motor de planilla (F23) para
// aplicar la cuota del período.
func (q *Queries) ListPrestamosActivosPorEmpleado(ctx context.Context, empleadoID int64) ([]Prestamo, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasPrestamo+`
FROM prestamos
WHERE empleado_id = ? AND estado = 'activo' AND saldo_pendiente > 0
ORDER BY fecha_inicio ASC, id ASC`, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrestamos(rows)
}

// AplicarAbonoPrestamoParams: SaldoPendiente es el MONTO A DESCONTAR del saldo
// (nombre heredado de sqlc), no el saldo nuevo.
type AplicarAbonoPrestamoParams struct {
	SaldoPendiente decimal.Decimal
	ID             int64
}

// AplicarAbonoPrestamo descuenta el monto del saldo. El schema no tiene
// cuotas_pagadas. El CHECK chk_pr_saldo (saldo >= 0) hace fallar el UPDATE si
// el monto excede el saldo; el servicio lo valida antes bajo FOR UPDATE.
func (q *Queries) AplicarAbonoPrestamo(ctx context.Context, arg AplicarAbonoPrestamoParams) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE prestamos
SET saldo_pendiente = saldo_pendiente - ?
WHERE id = ? AND estado = 'activo'`, arg.SaldoPendiente, arg.ID)
	return err
}

// CerrarPrestamoSiSaldoEnCero: idempotente; solo cierra si el saldo llegó a 0.
// El ENUM no tiene 'pagado'/'liquidado': el cierre por pago total usa
// 'cancelado' (igual que la cancelación manual).
func (q *Queries) CerrarPrestamoSiSaldoEnCero(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE prestamos SET estado = 'cancelado'
WHERE id = ? AND saldo_pendiente <= 0 AND estado = 'activo'`, id)
	return err
}

func (q *Queries) CancelarPrestamoManual(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE prestamos SET estado = 'cancelado'
WHERE id = ? AND estado IN ('activo', 'suspendido')`, id)
	return err
}

// ============================================================================
// TIPOS DE EMBARGO (parametros_tipos_embargo, versionado por vigencia)
// ============================================================================

type TipoEmbargo struct {
	ID               int64
	Codigo           string
	PrioridadLegal   int32 // menor número = mayor prioridad
	FechaFinVigencia sql.NullTime
}

func (q *Queries) GetTipoEmbargo(ctx context.Context, id int64) (TipoEmbargo, error) {
	var t TipoEmbargo
	err := q.db.QueryRowContext(ctx, `
SELECT id, codigo, prioridad_legal, fecha_fin_vigencia
FROM parametros_tipos_embargo WHERE id = ?`, id).
		Scan(&t.ID, &t.Codigo, &t.PrioridadLegal, &t.FechaFinVigencia)
	return t, err
}

// ============================================================================
// EMBARGOS
// DDL: id, empleado_id, tercero_id, tipo_embargo_id, numero_expediente,
//      porcentaje DECIMAL(5,2), monto_fijo, prioridad_aplicacion (NOT NULL),
//      fecha_inicio_vigencia, fecha_fin_vigencia, estado
//      ENUM('activo','finalizado','suspendido').
//      CHECK: porcentaje XOR monto_fijo; 0 < porcentaje <= 100.
// ============================================================================

type Embargo struct {
	ID                  int64
	EmpleadoID          int64
	TerceroID           int64
	TipoEmbargoID       int64
	NumeroExpediente    sql.NullString
	Porcentaje          decimal.NullDecimal
	MontoFijo           decimal.NullDecimal
	PrioridadAplicacion int32
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
	Estado              string // activo | suspendido | finalizado
}

const columnasEmbargo = `id, empleado_id, tercero_id, tipo_embargo_id, numero_expediente,
       porcentaje, monto_fijo, prioridad_aplicacion, fecha_inicio_vigencia,
       fecha_fin_vigencia, estado`

// Misma lista con alias `e.` para la query con JOIN.
const columnasEmbargoAlias = `e.id, e.empleado_id, e.tercero_id, e.tipo_embargo_id, e.numero_expediente,
       e.porcentaje, e.monto_fijo, e.prioridad_aplicacion, e.fecha_inicio_vigencia,
       e.fecha_fin_vigencia, e.estado`

func scanEmbargoExtra(s rowScanner, e *Embargo, extra ...interface{}) error {
	dest := []interface{}{
		&e.ID, &e.EmpleadoID, &e.TerceroID, &e.TipoEmbargoID, &e.NumeroExpediente,
		&e.Porcentaje, &e.MontoFijo, &e.PrioridadAplicacion, &e.FechaInicioVigencia,
		&e.FechaFinVigencia, &e.Estado,
	}
	return s.Scan(append(dest, extra...)...)
}

func scanEmbargo(s rowScanner) (Embargo, error) {
	var e Embargo
	err := scanEmbargoExtra(s, &e)
	return e, err
}

type CreateEmbargoParams struct {
	EmpleadoID          int64
	TerceroID           int64
	TipoEmbargoID       int64
	NumeroExpediente    sql.NullString
	Porcentaje          decimal.NullDecimal
	MontoFijo           decimal.NullDecimal
	PrioridadAplicacion int32
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
}

func (q *Queries) CreateEmbargo(ctx context.Context, arg CreateEmbargoParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO embargos (
    empleado_id, tercero_id, tipo_embargo_id, numero_expediente,
    porcentaje, monto_fijo, prioridad_aplicacion,
    fecha_inicio_vigencia, fecha_fin_vigencia, estado
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'activo')`,
		arg.EmpleadoID, arg.TerceroID, arg.TipoEmbargoID, arg.NumeroExpediente,
		arg.Porcentaje, arg.MontoFijo, arg.PrioridadAplicacion,
		arg.FechaInicioVigencia, arg.FechaFinVigencia)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) GetEmbargo(ctx context.Context, id int64) (Embargo, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasEmbargo+` FROM embargos WHERE id = ?`, id)
	return scanEmbargo(row)
}

// GetEmbargoParaActualizar: lectura con FOR UPDATE. Solo dentro de una Tx.
func (q *Queries) GetEmbargoParaActualizar(ctx context.Context, id int64) (Embargo, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasEmbargo+` FROM embargos WHERE id = ? FOR UPDATE`, id)
	return scanEmbargo(row)
}

// ListEmbargosPorEmpleadoParams: Estado "" = sin filtro.
type ListEmbargosPorEmpleadoParams struct {
	EmpleadoID int64
	Estado     string
	Limit      int32
	Offset     int32
}

func (q *Queries) ListEmbargosPorEmpleado(ctx context.Context, arg ListEmbargosPorEmpleadoParams) ([]Embargo, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasEmbargo+`
FROM embargos
WHERE empleado_id = ?
  AND (? = '' OR estado = ?)
ORDER BY fecha_inicio_vigencia DESC, id DESC
LIMIT ? OFFSET ?`, arg.EmpleadoID, arg.Estado, arg.Estado, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Embargo
	for rows.Next() {
		e, err := scanEmbargo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type CountEmbargosPorEmpleadoParams struct {
	EmpleadoID int64
	Estado     string
}

func (q *Queries) CountEmbargosPorEmpleado(ctx context.Context, arg CountEmbargosPorEmpleadoParams) (int64, error) {
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM embargos
WHERE empleado_id = ?
  AND (? = '' OR estado = ?)`, arg.EmpleadoID, arg.Estado, arg.Estado).Scan(&total)
	return total, err
}

type ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadParams struct {
	EmpleadoID int64
	FechaCorte time.Time // solo se incluyen embargos vigentes en esta fecha
}

// ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadRow agrega al embargo el
// código del tipo (catálogo parametros_tipos_embargo). La prioridad efectiva
// es Embargo.PrioridadAplicacion (la copia por empleado), no la del catálogo.
type ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadRow struct {
	Embargo
	TipoEmbargoCodigo string
}

// ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridad: embargos activos y
// vigentes en FechaCorte (inicio <= corte <= fin, o fin NULL), por
// prioridad_aplicacion ASC. El desempate por e.id hace el orden determinístico.
func (q *Queries) ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridad(ctx context.Context, arg ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadParams) ([]ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasEmbargoAlias+`, t.codigo
FROM embargos e
JOIN parametros_tipos_embargo t ON t.id = e.tipo_embargo_id
WHERE e.empleado_id = ?
  AND e.estado = 'activo'
  AND e.fecha_inicio_vigencia <= DATE(?)
  AND (e.fecha_fin_vigencia IS NULL OR e.fecha_fin_vigencia >= DATE(?))
ORDER BY e.prioridad_aplicacion ASC, e.id ASC`, arg.EmpleadoID, arg.FechaCorte, arg.FechaCorte)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadRow
	for rows.Next() {
		var r ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadRow
		if err := scanEmbargoExtra(rows, &r.Embargo, &r.TipoEmbargoCodigo); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type ActualizarEstadoEmbargoParams struct {
	Estado string
	// FechaFinVigencia: si es NULL se CONSERVA la fecha existente
	// (COALESCE). Antes se sobrescribía con NULL en cada transición, lo que
	// borraba la fecha de fin planificada al reactivar un embargo.
	FechaFinVigencia sql.NullTime
	ID               int64
}

// ActualizarEstadoEmbargo escribe estado y, si viene, fecha_fin_vigencia. La
// validez de la transición la valida el servicio bajo FOR UPDATE.
func (q *Queries) ActualizarEstadoEmbargo(ctx context.Context, arg ActualizarEstadoEmbargoParams) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE embargos
SET estado = ?, fecha_fin_vigencia = COALESCE(?, fecha_fin_vigencia)
WHERE id = ?`, arg.Estado, arg.FechaFinVigencia, arg.ID)
	return err
}