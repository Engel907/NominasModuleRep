// internal/db/dto/empleados.go
//
// Fase 3B — RRHH: empleados y sus 4 infotypes con vigencia (datos
// personales, laborales, bancarios, familiares).
//
// NOTA DE MIGRACIÓN IMPORTANTE — leer antes de tocar este archivo:
// db/queries/empleados_puestos.sql y los DTOs originales de
// internal/services/empleado_service.go se escribieron contra un esquema
// SUPUESTO para `empleados` y sus infotypes que no coincide con las tablas
// reales de db/schema.sql. Se corrigió contra el schema real; diferencias
// puntuales:
//
//  1. `empleados` (tabla base, NO versionada) tiene codigo_interno,
//     tipo_identificacion_id (FK), numero_identificacion, nombre_completo,
//     fecha_ingreso, activo, created_at, updated_at. NO tiene
//     `numero_empleado` (se usa codigo_interno). nombre_completo y
//     fecha_ingreso viven AQUÍ, no en los infotypes versionados — un
//     empleado no cambia de nombre ni de fecha de ingreso con el patrón de
//     vigencia, así que CrearEmpleadoRequest los sube a nivel raíz en
//     internal/services/empleado_service.go (antes estaban, sin lugar
//     donde persistirlos, dentro de DatosPersonalesInput/DatosLaboralesInput).
//
//  2. `empleado_datos_personales` (vigencia) SOLO tiene direccion, telefono,
//     correo_electronico — NO tiene cedula, nombre_completo,
//     fecha_nacimiento, genero, estado_civil ni nacionalidad (no existen en
//     ninguna tabla del esquema actual). Esos campos se quitaron de
//     DatosPersonalesInput. Tampoco tiene columna `creado_por` (la
//     auditoría de esta tabla es solo vía trigger genérico +
//     db.WithAuditContext, no una columna de negocio explícita).
//
//  3. `empleado_datos_laborales` (vigencia) tiene puesto_id,
//     salario_base_nominal, tipo_contrato_id (FK a tipos_contrato, NO un
//     string libre), programa_presupuestario_id, sindicato_id (FK
//     opcional). NO tiene unidad_organizativa_id (ya se deriva de
//     puesto_id → puestos.unidad_organizativa_id, no se duplica), NO tiene
//     jornada, NO tiene fecha_ingreso (vive en `empleados`, ver punto 1), y
//     tampoco tiene `creado_por`.
//
//  4. `empleado_datos_bancarios` (vigencia) tiene banco_id (FK a bancos, no
//     un nombre de banco libre) y tipo_cuenta ENUM('corriente','ahorros').
//     NO tiene numero_iban ni `creado_por`.
//
//  5. `empleado_datos_familiares` (vigencia) — la más distinta: en el
//     schema real es UNA fila agregada por empleado (aplica_credito_conyuge
//     bool + numero_hijos_dependientes int), con UNIQUE KEY
//     uk_edf_una_vigente(empleado_id, es_vigente) que permite como máximo
//     UNA fila vigente por empleado. NO es una lista de familiares con
//     nombre propio (nombre_familiar/parentesco/fecha_nacimiento no
//     existen en esta tabla). DatosFamiliarInput se ajustó en
//     internal/services/empleado_service.go a
//     (AplicaCreditoConyuge, NumeroHijosDependientes). Como consecuencia,
//     AgregarFamiliar YA NO puede solo insertar (el comentario original
//     asumía "puede haber N vigentes simultáneos", que es falso contra
//     este schema y violaría uk_edf_una_vigente en el segundo alta) —
//     ahora cierra la vigente anterior antes de abrir la nueva, igual que
//     el resto de infotypes.
//
// IDs: todos los IDs y FKs de este archivo son int64 (decisión D1, Fase 02),
// igual que security.go, contratos.go y organizacion.go. Las columnas
// siguen siendo BIGINT UNSIGNED en la BD; database/sql convierte al hacer
// Scan. Los contadores (p. ej. numero_hijos_dependientes) no son IDs y
// conservan su tipo.
package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// EMPLEADOS (tabla base, NO versionada)
// ============================================================================

type Empleado struct {
	ID                   int64
	CodigoInterno        string
	TipoIdentificacionID int64
	NumeroIdentificacion string
	NombreCompleto       string
	FechaIngreso         time.Time
	Activo               bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

const columnasEmpleado = `id, codigo_interno, tipo_identificacion_id, numero_identificacion,
       nombre_completo, fecha_ingreso, activo, created_at, updated_at`

func scanEmpleado(row *sql.Row) (Empleado, error) {
	var e Empleado
	err := row.Scan(
		&e.ID, &e.CodigoInterno, &e.TipoIdentificacionID, &e.NumeroIdentificacion,
		&e.NombreCompleto, &e.FechaIngreso, &e.Activo, &e.CreatedAt, &e.UpdatedAt,
	)
	return e, err
}

func (q *Queries) CreateEmpleado(ctx context.Context, codigoInterno string, tipoIdentificacionID int64, numeroIdentificacion, nombreCompleto string, fechaIngreso time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleados (codigo_interno, tipo_identificacion_id, numero_identificacion, nombre_completo, fecha_ingreso, activo, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, NOW(), NOW())`,
		codigoInterno, tipoIdentificacionID, numeroIdentificacion, nombreCompleto, fechaIngreso)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) GetEmpleadoByID(ctx context.Context, id int64) (Empleado, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasEmpleado+` FROM empleados WHERE id = ? LIMIT 1`, id)
	return scanEmpleado(row)
}

// GetEmpleadoByIDForUpdate toma un lock de fila (SELECT ... FOR UPDATE) sobre el
// empleado. F03: se usa al inicio de toda escritura versionada (cerrar+abrir) de
// un empleado para serializar escrituras concurrentes sobre el mismo empleado y
// para validar que exista. Solo tiene sentido dentro de una transacción
// (db.WithAuditContext).
func (q *Queries) GetEmpleadoByIDForUpdate(ctx context.Context, id int64) (Empleado, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasEmpleado+` FROM empleados WHERE id = ? LIMIT 1 FOR UPDATE`, id)
	return scanEmpleado(row)
}

// GetEmpleadoByCodigoInterno — equivalente a GetEmpleadoByNumero de la query
// original, renombrado porque la columna real es codigo_interno.
func (q *Queries) GetEmpleadoByCodigoInterno(ctx context.Context, codigoInterno string) (Empleado, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasEmpleado+` FROM empleados WHERE codigo_interno = ? LIMIT 1`, codigoInterno)
	return scanEmpleado(row)
}

func (q *Queries) SetEmpleadoActivo(ctx context.Context, id int64, activo bool) error {
	_, err := q.db.ExecContext(ctx, `UPDATE empleados SET activo = ?, updated_at = NOW() WHERE id = ?`, activo, id)
	return err
}

func (q *Queries) ListEmpleados(ctx context.Context, activo *bool, codigoInterno *string, limit, offset int32) ([]Empleado, error) {
	var activoArg, codigoArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if codigoInterno != nil {
		codigoArg = *codigoInterno
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasEmpleado+`
FROM empleados
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR codigo_interno LIKE CONCAT('%', ?, '%'))
ORDER BY id DESC
LIMIT ? OFFSET ?`, activoArg, activoArg, codigoArg, codigoArg, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Empleado
	for rows.Next() {
		var e Empleado
		if err := rows.Scan(&e.ID, &e.CodigoInterno, &e.TipoIdentificacionID, &e.NumeroIdentificacion,
			&e.NombreCompleto, &e.FechaIngreso, &e.Activo, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (q *Queries) CountEmpleados(ctx context.Context, activo *bool, codigoInterno *string) (int64, error) {
	var activoArg, codigoArg interface{}
	if activo != nil {
		activoArg = *activo
	}
	if codigoInterno != nil {
		codigoArg = *codigoInterno
	}
	row := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM empleados
WHERE (? IS NULL OR activo = ?)
  AND (? IS NULL OR codigo_interno LIKE CONCAT('%', ?, '%'))`, activoArg, activoArg, codigoArg, codigoArg)
	var total int64
	err := row.Scan(&total)
	return total, err
}

// ============================================================================
// EMPLEADO_DATOS_PERSONALES (vigencia — único vigente por empleado_id)
// Solo direccion/telefono/correo_electronico — ver NOTA DE MIGRACIÓN punto 2.
// ============================================================================

type EmpleadoDatosPersonales struct {
	ID                  int64
	EmpleadoID          int64
	Direccion           sql.NullString
	Telefono            sql.NullString
	CorreoElectronico   sql.NullString
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func (q *Queries) GetDatosPersonalesVigente(ctx context.Context, empleadoID int64) (EmpleadoDatosPersonales, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, empleado_id, direccion, telefono, correo_electronico, fecha_inicio_vigencia, fecha_fin_vigencia
FROM empleado_datos_personales
WHERE empleado_id = ? AND es_vigente = 1
LIMIT 1`, empleadoID)
	var d EmpleadoDatosPersonales
	err := row.Scan(&d.ID, &d.EmpleadoID, &d.Direccion, &d.Telefono, &d.CorreoElectronico, &d.FechaInicioVigencia, &d.FechaFinVigencia)
	return d, err
}

func (q *Queries) CerrarDatosPersonalesVigente(ctx context.Context, empleadoID int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE empleado_datos_personales
SET fecha_fin_vigencia = ?
WHERE empleado_id = ? AND es_vigente = 1`, fechaFinVigencia, empleadoID)
	return err
}

func (q *Queries) CrearVersionDatosPersonales(ctx context.Context, empleadoID int64, direccion, telefono, correoElectronico string, fechaInicioVigencia time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleado_datos_personales (empleado_id, direccion, telefono, correo_electronico, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES (?, ?, ?, ?, ?, NULL)`,
		empleadoID, nullStr(direccion), nullStr(telefono), nullStr(correoElectronico), fechaInicioVigencia)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ============================================================================
// EMPLEADO_DATOS_LABORALES (vigencia — único vigente por empleado_id)
// ============================================================================

type EmpleadoDatosLaborales struct {
	ID                       int64
	EmpleadoID               int64
	PuestoID                 int64
	SalarioBaseNominal       decimal.Decimal
	TipoContratoID           int64
	ProgramaPresupuestarioID int64
	SindicatoID              sql.NullInt64
	FechaInicioVigencia      time.Time
	FechaFinVigencia         sql.NullTime
}

func (q *Queries) GetDatosLaboralesVigente(ctx context.Context, empleadoID int64) (EmpleadoDatosLaborales, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, empleado_id, puesto_id, salario_base_nominal, tipo_contrato_id, programa_presupuestario_id, sindicato_id, fecha_inicio_vigencia, fecha_fin_vigencia
FROM empleado_datos_laborales
WHERE empleado_id = ? AND es_vigente = 1
LIMIT 1`, empleadoID)
	var d EmpleadoDatosLaborales
	err := row.Scan(&d.ID, &d.EmpleadoID, &d.PuestoID, &d.SalarioBaseNominal, &d.TipoContratoID,
		&d.ProgramaPresupuestarioID, &d.SindicatoID, &d.FechaInicioVigencia, &d.FechaFinVigencia)
	return d, err
}

func (q *Queries) CerrarDatosLaboralesVigente(ctx context.Context, empleadoID int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE empleado_datos_laborales
SET fecha_fin_vigencia = ?
WHERE empleado_id = ? AND es_vigente = 1`, fechaFinVigencia, empleadoID)
	return err
}

func (q *Queries) CrearVersionDatosLaborales(ctx context.Context, empleadoID, puestoID int64, salarioBaseNominal decimal.Decimal, tipoContratoID, programaPresupuestarioID int64, sindicatoID *int64, fechaInicioVigencia time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleado_datos_laborales (empleado_id, puesto_id, salario_base_nominal, tipo_contrato_id, programa_presupuestario_id, sindicato_id, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
		empleadoID, puestoID, salarioBaseNominal, tipoContratoID, programaPresupuestarioID, nullInt64FromPtr(sindicatoID), fechaInicioVigencia)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ============================================================================
// EMPLEADO_DATOS_BANCARIOS (vigencia — único vigente por empleado_id)
// ============================================================================

type EmpleadoDatosBancarios struct {
	ID                  int64
	EmpleadoID          int64
	BancoID             int64
	TipoCuenta          string
	NumeroCuenta        string
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
}

func (q *Queries) GetDatosBancariosVigente(ctx context.Context, empleadoID int64) (EmpleadoDatosBancarios, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, empleado_id, banco_id, tipo_cuenta, numero_cuenta, fecha_inicio_vigencia, fecha_fin_vigencia
FROM empleado_datos_bancarios
WHERE empleado_id = ? AND es_vigente = 1
LIMIT 1`, empleadoID)
	var d EmpleadoDatosBancarios
	err := row.Scan(&d.ID, &d.EmpleadoID, &d.BancoID, &d.TipoCuenta, &d.NumeroCuenta, &d.FechaInicioVigencia, &d.FechaFinVigencia)
	return d, err
}

func (q *Queries) CerrarDatosBancariosVigente(ctx context.Context, empleadoID int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE empleado_datos_bancarios
SET fecha_fin_vigencia = ?
WHERE empleado_id = ? AND es_vigente = 1`, fechaFinVigencia, empleadoID)
	return err
}

func (q *Queries) CrearVersionDatosBancarios(ctx context.Context, empleadoID, bancoID int64, tipoCuenta, numeroCuenta string, fechaInicioVigencia time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleado_datos_bancarios (empleado_id, banco_id, tipo_cuenta, numero_cuenta, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES (?, ?, ?, ?, ?, NULL)`,
		empleadoID, bancoID, tipoCuenta, numeroCuenta, fechaInicioVigencia)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ============================================================================
// EMPLEADO_DATOS_FAMILIARES (vigencia — UNA fila agregada por empleado, ver
// NOTA DE MIGRACIÓN punto 5. Se mantiene el nombre en plural
// "DatosFamiliares" por continuidad con el código existente, aunque
// representa una sola declaración, no una lista de personas.)
// ============================================================================

type EmpleadoDatosFamiliare struct {
	ID                      int64
	EmpleadoID              int64
	AplicaCreditoConyuge    bool
	NumeroHijosDependientes uint32
	FechaInicioVigencia     time.Time
	FechaFinVigencia        sql.NullTime
}

// ListDatosFamiliaresVigentes devuelve 0 o 1 fila (uk_edf_una_vigente
// garantiza como máximo una vigente por empleado_id). Se conserva la forma
// de lista para no romper la firma de
// EmpleadoService.ListarFamiliaresVigentes ni el handler, que ya itera
// sobre un slice.
func (q *Queries) ListDatosFamiliaresVigentes(ctx context.Context, empleadoID int64) ([]EmpleadoDatosFamiliare, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, empleado_id, aplica_credito_conyuge, numero_hijos_dependientes, fecha_inicio_vigencia, fecha_fin_vigencia
FROM empleado_datos_familiares
WHERE empleado_id = ? AND es_vigente = 1
ORDER BY id ASC`, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EmpleadoDatosFamiliare
	for rows.Next() {
		var f EmpleadoDatosFamiliare
		if err := rows.Scan(&f.ID, &f.EmpleadoID, &f.AplicaCreditoConyuge, &f.NumeroHijosDependientes, &f.FechaInicioVigencia, &f.FechaFinVigencia); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (q *Queries) GetDatosFamiliarVigentePorID(ctx context.Context, id int64) (EmpleadoDatosFamiliare, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, empleado_id, aplica_credito_conyuge, numero_hijos_dependientes, fecha_inicio_vigencia, fecha_fin_vigencia
FROM empleado_datos_familiares
WHERE id = ? AND es_vigente = 1
LIMIT 1`, id)
	var f EmpleadoDatosFamiliare
	err := row.Scan(&f.ID, &f.EmpleadoID, &f.AplicaCreditoConyuge, &f.NumeroHijosDependientes, &f.FechaInicioVigencia, &f.FechaFinVigencia)
	return f, err
}

func (q *Queries) CerrarDatosFamiliarVigentePorID(ctx context.Context, id int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE empleado_datos_familiares
SET fecha_fin_vigencia = ?
WHERE id = ? AND es_vigente = 1`, fechaFinVigencia, id)
	return err
}

// CerrarDatosFamiliarVigentePorEmpleado cierra (si existe) la declaración
// familiar vigente del empleado. Necesaria para AgregarFamiliar: a
// diferencia del código original (que solo insertaba, asumiendo que podían
// convivir N familiares vigentes), el schema real permite como máximo UNA
// fila vigente por empleado_id (uk_edf_una_vigente) — sin este cierre, un
// segundo alta para el mismo empleado violaría esa unicidad.
func (q *Queries) CerrarDatosFamiliarVigentePorEmpleado(ctx context.Context, empleadoID int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE empleado_datos_familiares
SET fecha_fin_vigencia = ?
WHERE empleado_id = ? AND es_vigente = 1`, fechaFinVigencia, empleadoID)
	return err
}

func (q *Queries) CrearVersionDatosFamiliares(ctx context.Context, empleadoID int64, aplicaCreditoConyuge bool, numeroHijosDependientes uint32, fechaInicioVigencia time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleado_datos_familiares (empleado_id, aplica_credito_conyuge, numero_hijos_dependientes, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES (?, ?, ?, ?, NULL)`,
		empleadoID, aplicaCreditoConyuge, numeroHijosDependientes, fechaInicioVigencia)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
