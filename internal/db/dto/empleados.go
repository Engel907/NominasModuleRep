package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type Empleado struct {
	ID                   int64
	CodigoInterno        string
	TipoIdentificacionID int64
	NombreCompleto       string
	FechaIngreso         time.Time
	Activo               bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

const columnasEmpleado = `id, codigo_interno, tipo_identificacion_id,
       nombre_completo, fecha_ingreso, activo, created_at, updated_at`

func scanEmpleado(row *sql.Row) (Empleado, error) {
	var e Empleado
	err := row.Scan(
		&e.ID, &e.CodigoInterno, &e.TipoIdentificacionID,
		&e.NombreCompleto, &e.FechaIngreso, &e.Activo, &e.CreatedAt, &e.UpdatedAt,
	)
	return e, err
}

func (q *Queries) CreateEmpleado(ctx context.Context, codigoInterno string, tipoIdentificacionID int64, identificacion ValorCifrado, nombreCompleto string, fechaIngreso time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleados (codigo_interno, tipo_identificacion_id, numero_identificacion_cifrado, numero_identificacion_hash,
    numero_identificacion_kid, nombre_completo, fecha_ingreso, activo, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 1, NOW(), NOW())`,
		codigoInterno, tipoIdentificacionID, identificacion.Cifrado, identificacion.Hash, identificacion.Kid, nombreCompleto, fechaIngreso)
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
// empleado. se usa al inicio de toda escritura versionada (cerrar+abrir) de
// un empleado para serializar escrituras concurrentes sobre el mismo empleado y
// para validar que exista. Solo tiene sentido dentro de una transacción
// (db.WithAuditContext).
func (q *Queries) GetEmpleadoByIDForUpdate(ctx context.Context, id int64) (Empleado, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasEmpleado+` FROM empleados WHERE id = ? LIMIT 1 FOR UPDATE`, id)
	return scanEmpleado(row)
}

// GetEmpleadoByCodigoInterno busca un empleado por su código interno.
func (q *Queries) GetEmpleadoByCodigoInterno(ctx context.Context, codigoInterno string) (Empleado, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasEmpleado+` FROM empleados WHERE codigo_interno = ? LIMIT 1`, codigoInterno)
	return scanEmpleado(row)
}

func (q *Queries) SetEmpleadoActivo(ctx context.Context, id int64, activo bool) error {
	_, err := q.db.ExecContext(ctx, `UPDATE empleados SET activo = ?, updated_at = NOW() WHERE id = ?`, activo, id)
	return err
}

// sinRegimenCCSS = true: solo empleados cuya versión vigente de datos
// laborales no tiene regimen_ccss_id (o que no tienen versión vigente).
func (q *Queries) ListEmpleados(ctx context.Context, activo *bool, codigoInterno *string, sinRegimenCCSS bool, limit, offset int32) ([]Empleado, error) {
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
  AND (? = 0 OR NOT EXISTS (
        SELECT 1 FROM empleado_datos_laborales edl
         WHERE edl.empleado_id = empleados.id AND edl.es_vigente = 1 AND edl.regimen_ccss_id IS NOT NULL))
ORDER BY id DESC
LIMIT ? OFFSET ?`, activoArg, activoArg, codigoArg, codigoArg, sinRegimenCCSS, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Empleado
	for rows.Next() {
		var e Empleado
		if err := rows.Scan(&e.ID, &e.CodigoInterno, &e.TipoIdentificacionID,
			&e.NombreCompleto, &e.FechaIngreso, &e.Activo, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (q *Queries) CountEmpleados(ctx context.Context, activo *bool, codigoInterno *string, sinRegimenCCSS bool) (int64, error) {
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
  AND (? IS NULL OR codigo_interno LIKE CONCAT('%', ?, '%'))
  AND (? = 0 OR NOT EXISTS (
        SELECT 1 FROM empleado_datos_laborales edl
         WHERE edl.empleado_id = empleados.id AND edl.es_vigente = 1 AND edl.regimen_ccss_id IS NOT NULL))`, activoArg, activoArg, codigoArg, codigoArg, sinRegimenCCSS)
	var total int64
	err := row.Scan(&total)
	return total, err
}

type EmpleadoDatosPersonales struct {
	ID                  int64
	EmpleadoID          int64
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
}

func (q *Queries) GetDatosPersonalesVigente(ctx context.Context, empleadoID int64) (EmpleadoDatosPersonales, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, empleado_id, fecha_inicio_vigencia, fecha_fin_vigencia
FROM empleado_datos_personales
WHERE empleado_id = ? AND es_vigente = 1
LIMIT 1`, empleadoID)
	var d EmpleadoDatosPersonales
	err := row.Scan(&d.ID, &d.EmpleadoID, &d.FechaInicioVigencia, &d.FechaFinVigencia)
	return d, err
}

func (q *Queries) CerrarDatosPersonalesVigente(ctx context.Context, empleadoID int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE empleado_datos_personales
SET fecha_fin_vigencia = ?
WHERE empleado_id = ? AND es_vigente = 1`, fechaFinVigencia, empleadoID)
	return err
}

// CrearVersionDatosPersonales abre la versión; los valores se guardan
// cifrados con GuardarCifrado en la misma transacción.
func (q *Queries) CrearVersionDatosPersonales(ctx context.Context, empleadoID int64, fechaInicioVigencia time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleado_datos_personales (empleado_id, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES (?, ?, NULL)`,
		empleadoID, fechaInicioVigencia)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

type EmpleadoDatosLaborales struct {
	ID                       int64
	EmpleadoID               int64
	PuestoID                 int64
	SalarioBaseNominal       decimal.Decimal
	TipoContratoID           int64
	ProgramaPresupuestarioID int64
	SindicatoID              sql.NullInt64
	RegimenCCSSID            sql.NullInt64
	FechaInicioVigencia      time.Time
	FechaFinVigencia         sql.NullTime
}

func (q *Queries) GetDatosLaboralesVigente(ctx context.Context, empleadoID int64) (EmpleadoDatosLaborales, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, empleado_id, puesto_id, salario_base_nominal, tipo_contrato_id, programa_presupuestario_id, sindicato_id, regimen_ccss_id, fecha_inicio_vigencia, fecha_fin_vigencia
FROM empleado_datos_laborales
WHERE empleado_id = ? AND es_vigente = 1
LIMIT 1`, empleadoID)
	var d EmpleadoDatosLaborales
	err := row.Scan(&d.ID, &d.EmpleadoID, &d.PuestoID, &d.SalarioBaseNominal, &d.TipoContratoID,
		&d.ProgramaPresupuestarioID, &d.SindicatoID, &d.RegimenCCSSID, &d.FechaInicioVigencia, &d.FechaFinVigencia)
	return d, err
}

func (q *Queries) CerrarDatosLaboralesVigente(ctx context.Context, empleadoID int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE empleado_datos_laborales
SET fecha_fin_vigencia = ?
WHERE empleado_id = ? AND es_vigente = 1`, fechaFinVigencia, empleadoID)
	return err
}

func (q *Queries) CrearVersionDatosLaborales(ctx context.Context, empleadoID, puestoID int64, salarioBaseNominal decimal.Decimal, tipoContratoID, programaPresupuestarioID int64, sindicatoID, regimenCCSSID *int64, fechaInicioVigencia time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleado_datos_laborales (empleado_id, puesto_id, salario_base_nominal, tipo_contrato_id, programa_presupuestario_id, sindicato_id, regimen_ccss_id, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		empleadoID, puestoID, salarioBaseNominal, tipoContratoID, programaPresupuestarioID, nullInt64FromPtr(sindicatoID), nullInt64FromPtr(regimenCCSSID), fechaInicioVigencia)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

type EmpleadoDatosBancarios struct {
	ID                  int64
	EmpleadoID          int64
	BancoID             int64
	TipoCuenta          string
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
}

func (q *Queries) GetDatosBancariosVigente(ctx context.Context, empleadoID int64) (EmpleadoDatosBancarios, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, empleado_id, banco_id, tipo_cuenta, fecha_inicio_vigencia, fecha_fin_vigencia
FROM empleado_datos_bancarios
WHERE empleado_id = ? AND es_vigente = 1
LIMIT 1`, empleadoID)
	var d EmpleadoDatosBancarios
	err := row.Scan(&d.ID, &d.EmpleadoID, &d.BancoID, &d.TipoCuenta, &d.FechaInicioVigencia, &d.FechaFinVigencia)
	return d, err
}

func (q *Queries) CerrarDatosBancariosVigente(ctx context.Context, empleadoID int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE empleado_datos_bancarios
SET fecha_fin_vigencia = ?
WHERE empleado_id = ? AND es_vigente = 1`, fechaFinVigencia, empleadoID)
	return err
}

func (q *Queries) CrearVersionDatosBancarios(ctx context.Context, empleadoID, bancoID int64, tipoCuenta string, cuenta ValorCifrado, fechaInicioVigencia time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO empleado_datos_bancarios (empleado_id, banco_id, tipo_cuenta, numero_cuenta_cifrado, numero_cuenta_hash, numero_cuenta_kid,
    fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
		empleadoID, bancoID, tipoCuenta, cuenta.Cifrado, cuenta.Hash, cuenta.Kid, fechaInicioVigencia)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

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
// familiar vigente del empleado; solo puede haber una vigente por empleado
// (uk_edf_una_vigente).
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

// ExisteRegimenCCSSActivo: el régimen existe en regimenes_ccss y está activo.
func (q *Queries) ExisteRegimenCCSSActivo(ctx context.Context, id int64) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM regimenes_ccss WHERE id = ? AND activo = 1`, id).Scan(&n)
	return n > 0, err
}
