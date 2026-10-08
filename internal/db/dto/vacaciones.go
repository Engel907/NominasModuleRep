// internal/db/dto/vacaciones.go
//
// Fase 4 — Vacaciones: escala legal, saldo materializado, ledger de
// movimientos y solicitudes. Reemplaza las queries de vacaciones de
// db/queries/asistencia_vacaciones.sql. Helpers (rowScanner, *PtrArg) en
// asistencia.go. Estilo de contratos.go: argumentos posicionales y sql.Null*.
//
// NOTA DE MIGRACIÓN (verificado contra db/schema.sql):
//
//   - `vacaciones_acumulado` NO es "una fila por empleado": es una tabla de
//     cortes históricos, UNIQUE (empleado_id, fecha_corte), con fecha_corte
//     NOT NULL sin default, `saldo`, `origen_carga` y
//     `registrado_por_usuario_id` (FK a usuarios). La migración 000003 solo le
//     agrega dias_disponibles. Por eso "el saldo actual" es el corte más
//     reciente (ORDER BY fecha_corte DESC, id DESC) y UpsertSaldoVacaciones
//     Empleado recibe fechaCorte y registradoPor, y mantiene saldo =
//     dias_disponibles.
//   - `parametros_vacaciones_escala.es_vigente` es columna generada (solo
//     lectura aquí).
//   - Corrección de FK (fase 1A): vacaciones_solicitudes y
//     vacaciones_movimientos apuntan a empleados(id), así que el listado de
//     solicitudes hace JOIN a `empleados` (antes `usuarios`, por la FK rota).
//     creado_por y aprobado_por SIGUEN siendo IDs de usuarios.
//   - MySQL no soporta RETURNING: se inserta/actualiza y se relee la fila.
package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// PARAMETROS_VACACIONES_ESCALA (solo lectura)
// ============================================================================

type ParametrosVacacionesEscala struct {
	ID                  int64
	AniosServicioDesde  decimal.Decimal
	AniosServicioHasta  decimal.NullDecimal
	DiasVacaciones      decimal.Decimal
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
}

// ObtenerEscalaVacacionesVigentePorAntiguedad lee el escalón vigente cuyo
// anios_servicio_desde es el mayor <= la antigüedad (en años) del empleado.
func (q *Queries) ObtenerEscalaVacacionesVigentePorAntiguedad(ctx context.Context, aniosServicio decimal.Decimal) (ParametrosVacacionesEscala, error) {
	var p ParametrosVacacionesEscala
	err := q.db.QueryRowContext(ctx, `
SELECT id, anios_servicio_desde, anios_servicio_hasta, dias_vacaciones, fecha_inicio_vigencia, fecha_fin_vigencia
FROM parametros_vacaciones_escala
WHERE es_vigente = 1
  AND anios_servicio_desde <= ?
ORDER BY anios_servicio_desde DESC
LIMIT 1`, aniosServicio).Scan(&p.ID, &p.AniosServicioDesde, &p.AniosServicioHasta, &p.DiasVacaciones,
		&p.FechaInicioVigencia, &p.FechaFinVigencia)
	return p, err
}

// ============================================================================
// VACACIONES_ACUMULADO (cortes; el saldo actual es el más reciente)
// ============================================================================

type VacacionesAcumulado struct {
	ID                     int64
	EmpleadoID             int64
	FechaCorte             time.Time
	DiasAcumulados         decimal.Decimal
	DiasDisfrutados        decimal.Decimal
	Saldo                  decimal.Decimal
	OrigenCarga            string // operacion | migracion_inicial
	RegistradoPorUsuarioID sql.NullInt64
	DiasDisponibles        decimal.Decimal
}

const columnasVacacionesAcumulado = `id, empleado_id, fecha_corte, dias_acumulados, dias_disfrutados, saldo,
       origen_carga, registrado_por_usuario_id, dias_disponibles`

func scanVacacionesAcumulado(s rowScanner) (VacacionesAcumulado, error) {
	var v VacacionesAcumulado
	err := s.Scan(&v.ID, &v.EmpleadoID, &v.FechaCorte, &v.DiasAcumulados, &v.DiasDisfrutados, &v.Saldo,
		&v.OrigenCarga, &v.RegistradoPorUsuarioID, &v.DiasDisponibles)
	return v, err
}

func (q *Queries) ObtenerSaldoVacacionesEmpleado(ctx context.Context, empleadoID int64) (VacacionesAcumulado, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT `+columnasVacacionesAcumulado+`
FROM vacaciones_acumulado
WHERE empleado_id = ?
ORDER BY fecha_corte DESC, id DESC
LIMIT 1`, empleadoID)
	return scanVacacionesAcumulado(row)
}

// ObtenerSaldoVacacionesEmpleadoForUpdate bloquea el corte más reciente para
// serializar acumulaciones/disfrutes concurrentes; solo tiene sentido dentro
// de una Tx (WithTx).
func (q *Queries) ObtenerSaldoVacacionesEmpleadoForUpdate(ctx context.Context, empleadoID int64) (VacacionesAcumulado, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT `+columnasVacacionesAcumulado+`
FROM vacaciones_acumulado
WHERE empleado_id = ?
ORDER BY fecha_corte DESC, id DESC
LIMIT 1
FOR UPDATE`, empleadoID)
	return scanVacacionesAcumulado(row)
}

// UpsertSaldoVacacionesEmpleado crea o actualiza el corte (empleado_id,
// fecha_corte). saldo y dias_disponibles quedan iguales (acumulados -
// disfrutados, calculado por el llamador). origen_carga = 'operacion'.
func (q *Queries) UpsertSaldoVacacionesEmpleado(
	ctx context.Context,
	empleadoID int64,
	fechaCorte time.Time,
	diasAcumulados, diasDisfrutados, diasDisponibles decimal.Decimal,
	registradoPor sql.NullInt64,
) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO vacaciones_acumulado (
    empleado_id, fecha_corte, dias_acumulados, dias_disfrutados, saldo, dias_disponibles,
    origen_carga, registrado_por_usuario_id
) VALUES (?, ?, ?, ?, ?, ?, 'operacion', ?)
ON DUPLICATE KEY UPDATE
    dias_acumulados = VALUES(dias_acumulados),
    dias_disfrutados = VALUES(dias_disfrutados),
    saldo = VALUES(saldo),
    dias_disponibles = VALUES(dias_disponibles)`,
		empleadoID, fechaCorte, diasAcumulados, diasDisfrutados, diasDisponibles, diasDisponibles, registradoPor)
	return err
}

// ============================================================================
// VACACIONES_MOVIMIENTOS (ledger append-only) — empleado_id → empleados(id)
// ============================================================================

type VacacionesMovimiento struct {
	ID                int64
	EmpleadoID        int64
	TipoMovimiento    string // acumulacion | disfrute | ajuste
	Dias              decimal.Decimal
	FechaEfectiva     time.Time
	PeriodoReferencia sql.NullString
	SolicitudID       sql.NullInt64
	Observaciones     sql.NullString
	CreadoPor         int64
	CreatedAt         time.Time
}

const columnasVacacionesMovimiento = `id, empleado_id, tipo_movimiento, dias, fecha_efectiva,
       periodo_referencia, solicitud_id, observaciones, creado_por, created_at`

func scanVacacionesMovimiento(s rowScanner) (VacacionesMovimiento, error) {
	var m VacacionesMovimiento
	err := s.Scan(&m.ID, &m.EmpleadoID, &m.TipoMovimiento, &m.Dias, &m.FechaEfectiva,
		&m.PeriodoReferencia, &m.SolicitudID, &m.Observaciones, &m.CreadoPor, &m.CreatedAt)
	return m, err
}

func (q *Queries) InsertarMovimientoVacaciones(
	ctx context.Context,
	empleadoID int64,
	tipoMovimiento string,
	dias decimal.Decimal,
	fechaEfectiva time.Time,
	periodoReferencia sql.NullString,
	solicitudID sql.NullInt64,
	observaciones sql.NullString,
	creadoPor int64,
) (VacacionesMovimiento, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO vacaciones_movimientos (
    empleado_id, tipo_movimiento, dias, fecha_efectiva,
    periodo_referencia, solicitud_id, observaciones, creado_por
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		empleadoID, tipoMovimiento, dias, fechaEfectiva, periodoReferencia, solicitudID, observaciones, creadoPor)
	if err != nil {
		return VacacionesMovimiento{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return VacacionesMovimiento{}, err
	}
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasVacacionesMovimiento+` FROM vacaciones_movimientos WHERE id = ?`, id)
	return scanVacacionesMovimiento(row)
}

func (q *Queries) ListarMovimientosVacacionesEmpleado(ctx context.Context, empleadoID int64, fechaDesde, fechaHasta *time.Time, limit, offset int32) ([]VacacionesMovimiento, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasVacacionesMovimiento+`
FROM vacaciones_movimientos
WHERE empleado_id = ?
  AND fecha_efectiva >= COALESCE(?, fecha_efectiva)
  AND fecha_efectiva <= COALESCE(?, fecha_efectiva)
ORDER BY fecha_efectiva DESC, id DESC
LIMIT ? OFFSET ?`, empleadoID, datePtrArg(fechaDesde), datePtrArg(fechaHasta), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []VacacionesMovimiento
	for rows.Next() {
		m, err := scanVacacionesMovimiento(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ============================================================================
// VACACIONES_SOLICITUDES — empleado_id → empleados(id)
// ============================================================================

type VacacionesSolicitud struct {
	ID              int64
	EmpleadoID      int64
	FechaInicio     time.Time
	FechaFin        time.Time
	DiasSolicitados decimal.Decimal
	Estado          string // pendiente | aprobada | rechazada | cancelada
	AprobadoPor     sql.NullInt64
	FechaAprobacion sql.NullTime
	MotivoRechazo   sql.NullString
	CreadoPor       int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

const columnasVacacionesSolicitud = `vs.id, vs.empleado_id, vs.fecha_inicio, vs.fecha_fin, vs.dias_solicitados, vs.estado,
       vs.aprobado_por, vs.fecha_aprobacion, vs.motivo_rechazo, vs.creado_por, vs.created_at, vs.updated_at`

func scanVacacionesSolicitudExtra(s rowScanner, v *VacacionesSolicitud, extra ...interface{}) error {
	dest := []interface{}{
		&v.ID, &v.EmpleadoID, &v.FechaInicio, &v.FechaFin, &v.DiasSolicitados, &v.Estado,
		&v.AprobadoPor, &v.FechaAprobacion, &v.MotivoRechazo, &v.CreadoPor, &v.CreatedAt, &v.UpdatedAt,
	}
	return s.Scan(append(dest, extra...)...)
}

func scanVacacionesSolicitud(s rowScanner) (VacacionesSolicitud, error) {
	var v VacacionesSolicitud
	err := scanVacacionesSolicitudExtra(s, &v)
	return v, err
}

func (q *Queries) CrearSolicitudVacaciones(ctx context.Context, empleadoID int64, fechaInicio, fechaFin time.Time, diasSolicitados decimal.Decimal, creadoPor int64) (VacacionesSolicitud, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO vacaciones_solicitudes (
    empleado_id, fecha_inicio, fecha_fin, dias_solicitados, estado, creado_por
) VALUES (?, ?, ?, ?, 'pendiente', ?)`, empleadoID, fechaInicio, fechaFin, diasSolicitados, creadoPor)
	if err != nil {
		return VacacionesSolicitud{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return VacacionesSolicitud{}, err
	}
	return q.ObtenerSolicitudVacaciones(ctx, id)
}

func (q *Queries) ObtenerSolicitudVacaciones(ctx context.Context, id int64) (VacacionesSolicitud, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasVacacionesSolicitud+` FROM vacaciones_solicitudes vs WHERE vs.id = ? LIMIT 1`, id)
	return scanVacacionesSolicitud(row)
}

func (q *Queries) ObtenerSolicitudVacacionesForUpdate(ctx context.Context, id int64) (VacacionesSolicitud, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasVacacionesSolicitud+` FROM vacaciones_solicitudes vs WHERE vs.id = ? LIMIT 1 FOR UPDATE`, id)
	return scanVacacionesSolicitud(row)
}

// ResolverSolicitudVacaciones aprueba/rechaza una solicitud PENDIENTE. Si ya
// no estaba pendiente (o no existe) devuelve sql.ErrNoRows, igual que el
// UPDATE ... RETURNING original al no encontrar fila.
func (q *Queries) ResolverSolicitudVacaciones(ctx context.Context, id int64, estado string, aprobadoPor int64, motivoRechazo sql.NullString) (VacacionesSolicitud, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE vacaciones_solicitudes
SET estado = ?, aprobado_por = ?, fecha_aprobacion = NOW(), motivo_rechazo = ?
WHERE id = ? AND estado = 'pendiente'`, estado, aprobadoPor, motivoRechazo, id)
	if err != nil {
		return VacacionesSolicitud{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return VacacionesSolicitud{}, err
	}
	if n == 0 {
		return VacacionesSolicitud{}, sql.ErrNoRows
	}
	return q.ObtenerSolicitudVacaciones(ctx, id)
}

// ListarSolicitudesVacacionesRow agrega datos del empleado (tabla `empleados`,
// no `usuarios`) y, si el empleado tiene cuenta, su nombre de usuario.
type ListarSolicitudesVacacionesRow struct {
	VacacionesSolicitud
	EmpleadoCodigoInterno string
	EmpleadoNombre        string
	EmpleadoNombreUsuario sql.NullString
}

// nombre_usuario sale de un subselect y no de un JOIN: usuarios.empleado_id no
// es UNIQUE, y un JOIN duplicaría solicitudes (y rompería LIMIT/OFFSET y el
// COUNT) si un empleado tuviera más de una cuenta.
func (q *Queries) ListarSolicitudesVacaciones(ctx context.Context, empleadoID *int64, estado *string, limit, offset int32) ([]ListarSolicitudesVacacionesRow, error) {
	emp, est := int64PtrArg(empleadoID), stringPtrArg(estado)
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasVacacionesSolicitud+`,
       e.codigo_interno, e.nombre_completo,
       (SELECT u.nombre_usuario FROM usuarios u WHERE u.empleado_id = e.id ORDER BY u.id LIMIT 1)
FROM vacaciones_solicitudes vs
JOIN empleados e ON e.id = vs.empleado_id
WHERE (? IS NULL OR vs.empleado_id = ?)
  AND (? IS NULL OR vs.estado = ?)
ORDER BY vs.created_at DESC
LIMIT ? OFFSET ?`, emp, emp, est, est, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ListarSolicitudesVacacionesRow
	for rows.Next() {
		var r ListarSolicitudesVacacionesRow
		if err := scanVacacionesSolicitudExtra(rows, &r.VacacionesSolicitud,
			&r.EmpleadoCodigoInterno, &r.EmpleadoNombre, &r.EmpleadoNombreUsuario); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ContarSolicitudesVacaciones(ctx context.Context, empleadoID *int64, estado *string) (int64, error) {
	emp, est := int64PtrArg(empleadoID), stringPtrArg(estado)
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM vacaciones_solicitudes vs
WHERE (? IS NULL OR vs.empleado_id = ?)
  AND (? IS NULL OR vs.estado = ?)`, emp, emp, est, est).Scan(&total)
	return total, err
}