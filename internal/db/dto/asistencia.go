// internal/db/dto/asistencia.go
//
// Fase 4 — Turnos, calendario laboral, tipos de ausencia, incapacidades y
// registro diario de asistencia. Reemplaza las queries `-- name:` de
// db/queries/asistencia_vacaciones.sql (sección turnos → registro de
// asistencia); las de vacaciones están en vacaciones.go.
//
// NOTA DE MIGRACIÓN (mismo criterio que contratos.go / organizacion.go): se
// verificó contra db/schema.sql, no contra el .sql de queries. Diferencias:
//
//   - `turnos` (v11 base, línea ~852) SOLO tiene id, codigo, nombre,
//     hora_inicio, hora_fin. El CREATE TABLE IF NOT EXISTS de la migración
//     000003 es un no-op porque la tabla ya existe, así que NO existen
//     hora_entrada_almuerzo, hora_salida_almuerzo, dias_laborables,
//     tolerancia_tardanza_minutos, activo, created_at ni updated_at. Las
//     columnas hora_inicio/hora_fin se exponen como HoraEntrada/HoraSalida en
//     los structs para no cambiar el contrato con internal/asistencia.
//   - `tipos_ausencia` (v11 base) solo tiene id, codigo, nombre,
//     requiere_documento_soporte (sin `activo`, sin afecta_salario, sin
//     entidad_emisora).
//   - `empleado_turno.es_vigente` es columna GENERADA (NULL cuando
//     fecha_fin_vigencia no es NULL): no se puede escribir. Cerrar = poner
//     fecha_fin_vigencia; abrir = insertar sin es_vigente. Tampoco tiene
//     `creado_por` (la auditoría es vía trigger + WithAuditContext), por eso
//     AbrirTurnoEmpleadoParams.CreadoPor se recibe pero no se persiste.
//   - MySQL no soporta RETURNING: los Crear*/Upsert* insertan y releen la fila.
//   - Las columnas TIME llegan del driver como []byte aunque parseTime=true;
//     se leen con nullTimeOfDay y se escriben como "15:04:05".
//   - Todas las FK de empleado_id de estas tablas apuntan a empleados(id)
//     (fase 1A). creado_por / aprobado_por siguen siendo IDs de usuarios.
package dto

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// Helpers internos
// ============================================================================

// rowScanner lo implementan *sql.Row y *sql.Rows, así cada tabla tiene un solo
// scanX en vez de uno para Row y otro para Rows.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

// nullTimeOfDay lee una columna TIME (nullable o no) hacia *time.Time.
// Solo hora/minuto/segundo son significativos (año 0000, igual que
// time.Parse("15:04")); calcularHoras en service.go normaliza la fecha.
type nullTimeOfDay struct {
	V *time.Time
}

func (n *nullTimeOfDay) Scan(src interface{}) error {
	switch v := src.(type) {
	case nil:
		n.V = nil
		return nil
	case []byte:
		return n.parse(string(v))
	case string:
		return n.parse(v)
	case time.Time:
		t := v
		n.V = &t
		return nil
	default:
		return fmt.Errorf("dto: no se puede leer TIME desde %T", src)
	}
}

func (n *nullTimeOfDay) parse(s string) error {
	t, err := time.Parse("15:04:05", s) // acepta fracciones de segundo
	if err != nil {
		return fmt.Errorf("dto: TIME inválido %q: %w", s, err)
	}
	n.V = &t
	return nil
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func timeOfDayArg(t time.Time) interface{} {
	return t.Format("15:04:05")
}

func timeOfDayPtrArg(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format("15:04:05")
}

func datePtrArg(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

func stringPtrArg(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func int64PtrArg(p *int64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// ============================================================================
// TURNOS
// ============================================================================

// Turno expone hora_inicio/hora_fin como HoraEntrada/HoraSalida (ver NOTA).
type Turno struct {
	ID          int64
	Codigo      string
	Nombre      string
	HoraEntrada time.Time
	HoraSalida  time.Time
}

const columnasTurno = `id, codigo, nombre, hora_inicio, hora_fin`

func scanTurno(s rowScanner) (Turno, error) {
	var t Turno
	var he, hs nullTimeOfDay
	if err := s.Scan(&t.ID, &t.Codigo, &t.Nombre, &he, &hs); err != nil {
		return Turno{}, err
	}
	t.HoraEntrada, t.HoraSalida = timeOrZero(he.V), timeOrZero(hs.V)
	return t, nil
}

// CrearTurnoParams solo lleva lo que `turnos` puede persistir hoy. Los campos
// de almuerzo, días laborables y tolerancia de CrearTurnoInput no tienen
// columna: ver NOTA y el resumen de la fase.
type CrearTurnoParams struct {
	Codigo      string
	Nombre      string
	HoraEntrada time.Time
	HoraSalida  time.Time
}

func (q *Queries) CrearTurno(ctx context.Context, arg CrearTurnoParams) (Turno, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO turnos (codigo, nombre, hora_inicio, hora_fin)
VALUES (?, ?, ?, ?)`,
		arg.Codigo, arg.Nombre, timeOfDayArg(arg.HoraEntrada), timeOfDayArg(arg.HoraSalida))
	if err != nil {
		return Turno{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Turno{}, err
	}
	return q.ObtenerTurno(ctx, id)
}

func (q *Queries) ObtenerTurno(ctx context.Context, id int64) (Turno, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasTurno+` FROM turnos WHERE id = ? LIMIT 1`, id)
	return scanTurno(row)
}

// ListarTurnosParams conserva Activo porque filtros.go lo construye, pero
// `turnos` no tiene columna `activo`: el filtro se ignora hasta que exista.
type ListarTurnosParams struct {
	Activo *bool
	Limit  int32
	Offset int32
}

func (q *Queries) ListarTurnos(ctx context.Context, arg ListarTurnosParams) ([]Turno, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasTurno+`
FROM turnos
ORDER BY nombre ASC
LIMIT ? OFFSET ?`, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Turno
	for rows.Next() {
		t, err := scanTurno(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ContarTurnos: `activo` se ignora por la misma razón que en ListarTurnos.
func (q *Queries) ContarTurnos(ctx context.Context, activo *bool) (int64, error) {
	_ = activo
	var total int64
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM turnos`).Scan(&total)
	return total, err
}

type ActualizarTurnoParams struct {
	ID          int64
	Nombre      string
	HoraEntrada time.Time
	HoraSalida  time.Time
}

func (q *Queries) ActualizarTurno(ctx context.Context, arg ActualizarTurnoParams) (Turno, error) {
	_, err := q.db.ExecContext(ctx, `
UPDATE turnos
SET nombre = ?, hora_inicio = ?, hora_fin = ?
WHERE id = ?`, arg.Nombre, timeOfDayArg(arg.HoraEntrada), timeOfDayArg(arg.HoraSalida), arg.ID)
	if err != nil {
		return Turno{}, err
	}
	return q.ObtenerTurno(ctx, arg.ID)
}

// ============================================================================
// EMPLEADO_TURNO (patrón de vigencia, transacción de 2 pasos — ADR-000 §2)
// ============================================================================

type EmpleadoTurno struct {
	ID                  int64
	EmpleadoID          int64
	TurnoID             int64
	FechaInicioVigencia time.Time
	FechaFinVigencia    *time.Time
}

// ObtenerTurnoVigenteEmpleadoRow: ID es el de empleado_turno (et.id); el turno
// asignado está en TurnoID.
type ObtenerTurnoVigenteEmpleadoRow struct {
	ID                  int64
	EmpleadoID          int64
	TurnoID             int64
	FechaInicioVigencia time.Time
	FechaFinVigencia    *time.Time
	TurnoNombre         string
	HoraEntrada         time.Time
	HoraSalida          time.Time
}

func (q *Queries) ObtenerTurnoVigenteEmpleado(ctx context.Context, empleadoID int64) (ObtenerTurnoVigenteEmpleadoRow, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT et.id, et.empleado_id, et.turno_id, et.fecha_inicio_vigencia, et.fecha_fin_vigencia,
       t.nombre, t.hora_inicio, t.hora_fin
FROM empleado_turno et
JOIN turnos t ON t.id = et.turno_id
WHERE et.empleado_id = ? AND et.es_vigente = 1
LIMIT 1`, empleadoID)
	var r ObtenerTurnoVigenteEmpleadoRow
	var he, hs nullTimeOfDay
	if err := row.Scan(&r.ID, &r.EmpleadoID, &r.TurnoID, &r.FechaInicioVigencia, &r.FechaFinVigencia,
		&r.TurnoNombre, &he, &hs); err != nil {
		return ObtenerTurnoVigenteEmpleadoRow{}, err
	}
	r.HoraEntrada, r.HoraSalida = timeOrZero(he.V), timeOrZero(hs.V)
	return r, nil
}

type CerrarTurnoVigenteEmpleadoParams struct {
	EmpleadoID       int64
	FechaFinVigencia *time.Time
}

// CerrarTurnoVigenteEmpleado es el paso 1 del patrón de 2 pasos. es_vigente es
// generada: al fijar fecha_fin_vigencia pasa sola a NULL.
func (q *Queries) CerrarTurnoVigenteEmpleado(ctx context.Context, arg CerrarTurnoVigenteEmpleadoParams) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE empleado_turno
SET fecha_fin_vigencia = ?
WHERE empleado_id = ? AND es_vigente = 1`, datePtrArg(arg.FechaFinVigencia), arg.EmpleadoID)
	return err
}

// AbrirTurnoEmpleadoParams.CreadoPor no se persiste: empleado_turno no tiene
// esa columna (auditoría vía trigger + WithAuditContext).
type AbrirTurnoEmpleadoParams struct {
	EmpleadoID          int64
	TurnoID             int64
	FechaInicioVigencia time.Time
	CreadoPor           int64
}

// AbrirTurnoEmpleado es el paso 2; debe ir en la misma Tx que el cierre.
func (q *Queries) AbrirTurnoEmpleado(ctx context.Context, arg AbrirTurnoEmpleadoParams) (EmpleadoTurno, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleado_turno (empleado_id, turno_id, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES (?, ?, ?, NULL)`, arg.EmpleadoID, arg.TurnoID, arg.FechaInicioVigencia)
	if err != nil {
		return EmpleadoTurno{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return EmpleadoTurno{}, err
	}
	var et EmpleadoTurno
	err = q.db.QueryRowContext(ctx, `
SELECT id, empleado_id, turno_id, fecha_inicio_vigencia, fecha_fin_vigencia
FROM empleado_turno WHERE id = ?`, id).
		Scan(&et.ID, &et.EmpleadoID, &et.TurnoID, &et.FechaInicioVigencia, &et.FechaFinVigencia)
	return et, err
}

// ============================================================================
// FERIADOS Y CALENDARIO LABORAL
// ============================================================================

type Feriado struct {
	ID              int64
	Fecha           time.Time
	Nombre          string
	Tipo            string // fijo | movil
	PagoObligatorio bool
	Anio            int32
	CreatedAt       time.Time
}

const columnasFeriado = `id, fecha, nombre, tipo, pago_obligatorio, anio, created_at`

func scanFeriado(s rowScanner) (Feriado, error) {
	var f Feriado
	err := s.Scan(&f.ID, &f.Fecha, &f.Nombre, &f.Tipo, &f.PagoObligatorio, &f.Anio, &f.CreatedAt)
	return f, err
}

type CrearFeriadoParams struct {
	Fecha           time.Time
	Nombre          string
	Tipo            string
	PagoObligatorio bool
	Anio            int32
}

// CrearFeriado es idempotente por fecha (UNIQUE feriados_fecha_unique). Se
// relee por fecha y no por LastInsertId, que en un upsert sin cambios no es
// confiable.
func (q *Queries) CrearFeriado(ctx context.Context, arg CrearFeriadoParams) (Feriado, error) {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO feriados (fecha, nombre, tipo, pago_obligatorio, anio)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE nombre = VALUES(nombre), tipo = VALUES(tipo),
    pago_obligatorio = VALUES(pago_obligatorio)`,
		arg.Fecha, arg.Nombre, arg.Tipo, arg.PagoObligatorio, arg.Anio)
	if err != nil {
		return Feriado{}, err
	}
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasFeriado+` FROM feriados WHERE fecha = ? LIMIT 1`, arg.Fecha)
	return scanFeriado(row)
}

func (q *Queries) ListarFeriadosPorAnio(ctx context.Context, anio int32) ([]Feriado, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT `+columnasFeriado+` FROM feriados WHERE anio = ? ORDER BY fecha ASC`, anio)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Feriado
	for rows.Next() {
		f, err := scanFeriado(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

type CalendarioLaboralDia struct {
	ID         int64
	Fecha      time.Time
	Anio       int32
	EsHabil    bool
	EsFeriado  bool
	FeriadoID  *int64
	DiaSemana  int16
	GeneradoEn time.Time
}

const columnasCalendarioDia = `id, fecha, anio, es_habil, es_feriado, feriado_id, dia_semana, generado_en`

func scanCalendarioDia(s rowScanner) (CalendarioLaboralDia, error) {
	var d CalendarioLaboralDia
	err := s.Scan(&d.ID, &d.Fecha, &d.Anio, &d.EsHabil, &d.EsFeriado, &d.FeriadoID, &d.DiaSemana, &d.GeneradoEn)
	return d, err
}

type InsertarDiaCalendarioParams struct {
	Fecha     time.Time
	Anio      int32
	EsHabil   bool
	EsFeriado bool
	FeriadoID *int64
	DiaSemana int16
}

func (q *Queries) InsertarDiaCalendario(ctx context.Context, arg InsertarDiaCalendarioParams) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO calendario_laboral_dia (fecha, anio, es_habil, es_feriado, feriado_id, dia_semana)
VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE es_habil = VALUES(es_habil), es_feriado = VALUES(es_feriado),
    feriado_id = VALUES(feriado_id)`,
		arg.Fecha, arg.Anio, arg.EsHabil, arg.EsFeriado, int64PtrArg(arg.FeriadoID), arg.DiaSemana)
	return err
}

func (q *Queries) ObtenerDiaCalendario(ctx context.Context, fecha time.Time) (CalendarioLaboralDia, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasCalendarioDia+` FROM calendario_laboral_dia WHERE fecha = ? LIMIT 1`, fecha)
	return scanCalendarioDia(row)
}

type ListarCalendarioPorRangoRow struct {
	CalendarioLaboralDia
	FeriadoNombre   *string
	PagoObligatorio *bool
}

func (q *Queries) ListarCalendarioPorRango(ctx context.Context, desde, hasta time.Time) ([]ListarCalendarioPorRangoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT cld.id, cld.fecha, cld.anio, cld.es_habil, cld.es_feriado, cld.feriado_id, cld.dia_semana, cld.generado_en,
       f.nombre, f.pago_obligatorio
FROM calendario_laboral_dia cld
LEFT JOIN feriados f ON f.id = cld.feriado_id
WHERE cld.fecha BETWEEN ? AND ?
ORDER BY cld.fecha ASC`, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ListarCalendarioPorRangoRow
	for rows.Next() {
		var r ListarCalendarioPorRangoRow
		d := &r.CalendarioLaboralDia
		if err := rows.Scan(&d.ID, &d.Fecha, &d.Anio, &d.EsHabil, &d.EsFeriado, &d.FeriadoID, &d.DiaSemana, &d.GeneradoEn,
			&r.FeriadoNombre, &r.PagoObligatorio); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ExisteCalendarioGenerado devuelve la cantidad de días generados para el año
// (0 = no generado), igual que el COUNT(*) original.
func (q *Queries) ExisteCalendarioGenerado(ctx context.Context, anio int32) (int64, error) {
	var total int64
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendario_laboral_dia WHERE anio = ?`, anio).Scan(&total)
	return total, err
}

// ============================================================================
// TIPOS_AUSENCIA
// ============================================================================

// TiposAusencium conserva el nombre que generaba sqlc (lo usa service.go).
type TiposAusencium struct {
	ID                       int64
	Codigo                   string
	Nombre                   string
	RequiereDocumentoSoporte bool
}

func (q *Queries) ListarTiposAusencia(ctx context.Context) ([]TiposAusencium, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, codigo, nombre, requiere_documento_soporte
FROM tipos_ausencia
ORDER BY nombre ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TiposAusencium
	for rows.Next() {
		var t TiposAusencium
		if err := rows.Scan(&t.ID, &t.Codigo, &t.Nombre, &t.RequiereDocumentoSoporte); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (q *Queries) ObtenerTipoAusenciaPorCodigo(ctx context.Context, codigo string) (TiposAusencium, error) {
	var t TiposAusencium
	err := q.db.QueryRowContext(ctx, `
SELECT id, codigo, nombre, requiere_documento_soporte
FROM tipos_ausencia
WHERE codigo = ?
LIMIT 1`, codigo).Scan(&t.ID, &t.Codigo, &t.Nombre, &t.RequiereDocumentoSoporte)
	return t, err
}

// ============================================================================
// INCAPACIDADES (CCSS / INS) — empleado_id → empleados(id)
// ============================================================================

// Incapacidade conserva el nombre que generaba sqlc (lo usa service.go).
type Incapacidade struct {
	ID                int64
	EmpleadoID        int64
	EntidadEmisora    string // CCSS | INS
	NumeroBoleta      string
	FechaInicio       time.Time
	FechaFin          time.Time
	Dias              int16
	DiagnosticoCodigo *string
	Observaciones     *string
	CreadoPor         int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

const columnasIncapacidad = `id, empleado_id, entidad_emisora, numero_boleta, fecha_inicio, fecha_fin,
       dias, diagnostico_codigo, observaciones, creado_por, created_at, updated_at`

func scanIncapacidad(s rowScanner) (Incapacidade, error) {
	var i Incapacidade
	err := s.Scan(&i.ID, &i.EmpleadoID, &i.EntidadEmisora, &i.NumeroBoleta, &i.FechaInicio, &i.FechaFin,
		&i.Dias, &i.DiagnosticoCodigo, &i.Observaciones, &i.CreadoPor, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

type CrearIncapacidadParams struct {
	EmpleadoID        int64
	EntidadEmisora    string
	NumeroBoleta      string
	FechaInicio       time.Time
	FechaFin          time.Time
	Dias              int16
	DiagnosticoCodigo *string
	Observaciones     *string
	CreadoPor         int64
}

func (q *Queries) CrearIncapacidad(ctx context.Context, arg CrearIncapacidadParams) (Incapacidade, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO incapacidades (
    empleado_id, entidad_emisora, numero_boleta, fecha_inicio, fecha_fin,
    dias, diagnostico_codigo, observaciones, creado_por
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.EmpleadoID, arg.EntidadEmisora, arg.NumeroBoleta, arg.FechaInicio, arg.FechaFin,
		arg.Dias, stringPtrArg(arg.DiagnosticoCodigo), stringPtrArg(arg.Observaciones), arg.CreadoPor)
	if err != nil {
		return Incapacidade{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Incapacidade{}, err
	}
	return q.ObtenerIncapacidad(ctx, id)
}

func (q *Queries) ObtenerIncapacidad(ctx context.Context, id int64) (Incapacidade, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasIncapacidad+` FROM incapacidades WHERE id = ? LIMIT 1`, id)
	return scanIncapacidad(row)
}

type ListarIncapacidadesEmpleadoParams struct {
	EmpleadoID     int64
	EntidadEmisora *string
	FechaDesde     *time.Time
	FechaHasta     *time.Time
	Limit          int32
	Offset         int32
}

func (q *Queries) ListarIncapacidadesEmpleado(ctx context.Context, arg ListarIncapacidadesEmpleadoParams) ([]Incapacidade, error) {
	entidad := stringPtrArg(arg.EntidadEmisora)
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasIncapacidad+`
FROM incapacidades
WHERE empleado_id = ?
  AND (? IS NULL OR entidad_emisora = ?)
  AND fecha_inicio >= COALESCE(?, fecha_inicio)
  AND fecha_fin <= COALESCE(?, fecha_fin)
ORDER BY fecha_inicio DESC
LIMIT ? OFFSET ?`,
		arg.EmpleadoID, entidad, entidad, datePtrArg(arg.FechaDesde), datePtrArg(arg.FechaHasta), arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Incapacidade
	for rows.Next() {
		i, err := scanIncapacidad(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

type ContarIncapacidadesEmpleadoParams struct {
	EmpleadoID     int64
	EntidadEmisora *string
	FechaDesde     *time.Time
	FechaHasta     *time.Time
}

func (q *Queries) ContarIncapacidadesEmpleado(ctx context.Context, arg ContarIncapacidadesEmpleadoParams) (int64, error) {
	entidad := stringPtrArg(arg.EntidadEmisora)
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM incapacidades
WHERE empleado_id = ?
  AND (? IS NULL OR entidad_emisora = ?)
  AND fecha_inicio >= COALESCE(?, fecha_inicio)
  AND fecha_fin <= COALESCE(?, fecha_fin)`,
		arg.EmpleadoID, entidad, entidad, datePtrArg(arg.FechaDesde), datePtrArg(arg.FechaHasta)).Scan(&total)
	return total, err
}

// ============================================================================
// REGISTRO_ASISTENCIA_DETALLE — empleado_id → empleados(id), único (empleado_id, fecha)
// ============================================================================

type RegistroAsistenciaDetalle struct {
	ID              int64
	EmpleadoID      int64
	TurnoID         *int64
	Fecha           time.Time
	HoraEntradaReal *time.Time
	HoraSalidaReal  *time.Time
	HorasOrdinarias decimal.Decimal
	HorasExtra      decimal.Decimal
	HorasTardanza   decimal.Decimal
	MinutosTardanza int16
	Estado          string
	TipoAusenciaID  *int64
	IncapacidadID   *int64
	Observaciones   *string
	CreadoPor       int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// columnasRegistroAsistencia con prefijo de tabla `rad.` para poder reusarse en
// el listado con JOIN y en las consultas simples (alias rad en ambas).
const columnasRegistroAsistencia = `rad.id, rad.empleado_id, rad.turno_id, rad.fecha, rad.hora_entrada_real, rad.hora_salida_real,
       rad.horas_ordinarias, rad.horas_extra, rad.horas_tardanza, rad.minutos_tardanza,
       rad.estado, rad.tipo_ausencia_id, rad.incapacidad_id, rad.observaciones,
       rad.creado_por, rad.created_at, rad.updated_at`

// scanRegistroAsistenciaExtra escanea las columnas de columnasRegistroAsistencia
// más `extra` (columnas adicionales del JOIN). Las dos columnas TIME pasan por
// nullTimeOfDay y se copian a la struct tras el Scan.
func scanRegistroAsistenciaExtra(s rowScanner, r *RegistroAsistenciaDetalle, extra ...interface{}) error {
	var he, hs nullTimeOfDay
	dest := []interface{}{
		&r.ID, &r.EmpleadoID, &r.TurnoID, &r.Fecha, &he, &hs,
		&r.HorasOrdinarias, &r.HorasExtra, &r.HorasTardanza, &r.MinutosTardanza,
		&r.Estado, &r.TipoAusenciaID, &r.IncapacidadID, &r.Observaciones,
		&r.CreadoPor, &r.CreatedAt, &r.UpdatedAt,
	}
	dest = append(dest, extra...)
	if err := s.Scan(dest...); err != nil {
		return err
	}
	r.HoraEntradaReal, r.HoraSalidaReal = he.V, hs.V
	return nil
}

func scanRegistroAsistencia(s rowScanner) (RegistroAsistenciaDetalle, error) {
	var r RegistroAsistenciaDetalle
	err := scanRegistroAsistenciaExtra(s, &r)
	return r, err
}

type UpsertRegistroAsistenciaParams struct {
	EmpleadoID      int64
	TurnoID         *int64
	Fecha           time.Time
	HoraEntradaReal *time.Time
	HoraSalidaReal  *time.Time
	HorasOrdinarias decimal.Decimal
	HorasExtra      decimal.Decimal
	HorasTardanza   decimal.Decimal
	MinutosTardanza int16
	Estado          string
	TipoAusenciaID  *int64
	IncapacidadID   *int64
	Observaciones   *string
	CreadoPor       int64
}

// UpsertRegistroAsistencia inserta o actualiza el día del empleado (único por
// empleado_id+fecha, sin patrón es_vigente → no aplica la Tx de 2 pasos).
// creado_por solo se escribe en el alta, igual que el SQL original.
func (q *Queries) UpsertRegistroAsistencia(ctx context.Context, arg UpsertRegistroAsistenciaParams) (RegistroAsistenciaDetalle, error) {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO registro_asistencia_detalle (
    empleado_id, turno_id, fecha, hora_entrada_real, hora_salida_real,
    horas_ordinarias, horas_extra, horas_tardanza, minutos_tardanza,
    estado, tipo_ausencia_id, incapacidad_id, observaciones, creado_por
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    turno_id = VALUES(turno_id),
    hora_entrada_real = VALUES(hora_entrada_real),
    hora_salida_real = VALUES(hora_salida_real),
    horas_ordinarias = VALUES(horas_ordinarias),
    horas_extra = VALUES(horas_extra),
    horas_tardanza = VALUES(horas_tardanza),
    minutos_tardanza = VALUES(minutos_tardanza),
    estado = VALUES(estado),
    tipo_ausencia_id = VALUES(tipo_ausencia_id),
    incapacidad_id = VALUES(incapacidad_id),
    observaciones = VALUES(observaciones)`,
		arg.EmpleadoID, int64PtrArg(arg.TurnoID), arg.Fecha,
		timeOfDayPtrArg(arg.HoraEntradaReal), timeOfDayPtrArg(arg.HoraSalidaReal),
		arg.HorasOrdinarias, arg.HorasExtra, arg.HorasTardanza, arg.MinutosTardanza,
		arg.Estado, int64PtrArg(arg.TipoAusenciaID), int64PtrArg(arg.IncapacidadID),
		stringPtrArg(arg.Observaciones), arg.CreadoPor)
	if err != nil {
		return RegistroAsistenciaDetalle{}, err
	}
	return q.ObtenerRegistroAsistencia(ctx, ObtenerRegistroAsistenciaParams{EmpleadoID: arg.EmpleadoID, Fecha: arg.Fecha})
}

type ObtenerRegistroAsistenciaParams struct {
	EmpleadoID int64
	Fecha      time.Time
}

func (q *Queries) ObtenerRegistroAsistencia(ctx context.Context, arg ObtenerRegistroAsistenciaParams) (RegistroAsistenciaDetalle, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT `+columnasRegistroAsistencia+`
FROM registro_asistencia_detalle rad
WHERE rad.empleado_id = ? AND rad.fecha = ?
LIMIT 1`, arg.EmpleadoID, arg.Fecha)
	return scanRegistroAsistencia(row)
}

type ListarAsistenciaEmpleadoParams struct {
	EmpleadoID int64
	Estado     *string
	FechaDesde *time.Time
	FechaHasta *time.Time
	Limit      int32
	Offset     int32
}

type ListarAsistenciaEmpleadoRow struct {
	RegistroAsistenciaDetalle
	TipoAusenciaCodigo *string
	TipoAusenciaNombre *string
}

func (q *Queries) ListarAsistenciaEmpleado(ctx context.Context, arg ListarAsistenciaEmpleadoParams) ([]ListarAsistenciaEmpleadoRow, error) {
	estado := stringPtrArg(arg.Estado)
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasRegistroAsistencia+`, ta.codigo, ta.nombre
FROM registro_asistencia_detalle rad
LEFT JOIN tipos_ausencia ta ON ta.id = rad.tipo_ausencia_id
WHERE rad.empleado_id = ?
  AND (? IS NULL OR rad.estado = ?)
  AND rad.fecha >= COALESCE(?, rad.fecha)
  AND rad.fecha <= COALESCE(?, rad.fecha)
ORDER BY rad.fecha DESC
LIMIT ? OFFSET ?`,
		arg.EmpleadoID, estado, estado, datePtrArg(arg.FechaDesde), datePtrArg(arg.FechaHasta), arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ListarAsistenciaEmpleadoRow
	for rows.Next() {
		var r ListarAsistenciaEmpleadoRow
		if err := scanRegistroAsistenciaExtra(rows, &r.RegistroAsistenciaDetalle, &r.TipoAusenciaCodigo, &r.TipoAusenciaNombre); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type ContarAsistenciaEmpleadoParams struct {
	EmpleadoID int64
	Estado     *string
	FechaDesde *time.Time
	FechaHasta *time.Time
}

func (q *Queries) ContarAsistenciaEmpleado(ctx context.Context, arg ContarAsistenciaEmpleadoParams) (int64, error) {
	estado := stringPtrArg(arg.Estado)
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM registro_asistencia_detalle
WHERE empleado_id = ?
  AND (? IS NULL OR estado = ?)
  AND fecha >= COALESCE(?, fecha)
  AND fecha <= COALESCE(?, fecha)`,
		arg.EmpleadoID, estado, estado, datePtrArg(arg.FechaDesde), datePtrArg(arg.FechaHasta)).Scan(&total)
	return total, err
}

// ListarAusenciasInjustificadasRango lo usa el motor de planilla (Fase 5+)
// para deducciones por ausencia.
func (q *Queries) ListarAusenciasInjustificadasRango(ctx context.Context, desde, hasta time.Time) ([]RegistroAsistenciaDetalle, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasRegistroAsistencia+`
FROM registro_asistencia_detalle rad
WHERE rad.estado = 'ausente_injustificada' AND rad.fecha BETWEEN ? AND ?
ORDER BY rad.empleado_id, rad.fecha`, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RegistroAsistenciaDetalle
	for rows.Next() {
		r, err := scanRegistroAsistencia(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ContarAsignacionesVigentesTurno cuenta las filas vigentes de empleado_turno
// (fecha_fin_vigencia IS NULL → es_vigente = 1) que apuntan a un turno. Lo usa
// Service.ActualizarTurno para impedir que se cambie el horario de un turno
// en uso sin pasar por una nueva asignación (que cierra y abre vigencia).
func (q *Queries) ContarAsignacionesVigentesTurno(ctx context.Context, turnoID int64) (int64, error) {
	var total int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM empleado_turno WHERE turno_id = ? AND es_vigente = 1`, turnoID).Scan(&total)
	return total, err
}
