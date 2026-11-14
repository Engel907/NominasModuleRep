package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type Contrato struct {
	ID                  int64
	EmpleadoID          int64
	PuestoID            int64
	RegimenLaboralID    int64
	TipoContratoID      int64
	TipoContrato        string // tipos_contrato.codigo (solo lectura)
	SalarioPactado      decimal.Decimal
	ModalidadSalario    string
	JornadaID           int64
	Jornada             string // jornadas.codigo (solo lectura)
	FechaInicioContrato time.Time
	FechaFinContrato    sql.NullTime
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
	EsVigente           bool
	CreadoPor           int64
	CreatedAt           time.Time
}

// es_vigente es una columna GENERATED que vale 1 si la fila está vigente y
// NULL (no 0) si está cerrada. Scan de NULL a bool falla, así que se
// COALESCE-a a 0 en todas las lecturas. Por la misma razón los filtros
// por "no vigentes" usan COALESCE(c.es_vigente, 0) = ? (es_vigente = 0 nunca
// matchea, porque el valor real es NULL).
const columnasContrato = `c.id, c.empleado_id, c.puesto_id, c.regimen_laboral_id, c.tipo_contrato_id, tc.codigo,
       c.salario_pactado, c.modalidad_salario, c.jornada_id, j.codigo, c.fecha_inicio_contrato, c.fecha_fin_contrato,
       c.fecha_inicio_vigencia, c.fecha_fin_vigencia, COALESCE(c.es_vigente, 0), c.creado_por, c.created_at`

// desdeContratos: contratos con los catálogos de los que se leen los códigos.
const desdeContratos = `contratos c
JOIN tipos_contrato tc ON tc.id = c.tipo_contrato_id
JOIN jornadas j ON j.id = c.jornada_id`

func scanContratoRow(row *sql.Row) (Contrato, error) {
	var c Contrato
	err := row.Scan(&c.ID, &c.EmpleadoID, &c.PuestoID, &c.RegimenLaboralID, &c.TipoContratoID, &c.TipoContrato,
		&c.SalarioPactado, &c.ModalidadSalario, &c.JornadaID, &c.Jornada, &c.FechaInicioContrato, &c.FechaFinContrato,
		&c.FechaInicioVigencia, &c.FechaFinVigencia, &c.EsVigente, &c.CreadoPor, &c.CreatedAt)
	return c, err
}

func scanContratoRows(rows *sql.Rows) (Contrato, error) {
	var c Contrato
	err := rows.Scan(&c.ID, &c.EmpleadoID, &c.PuestoID, &c.RegimenLaboralID, &c.TipoContratoID, &c.TipoContrato,
		&c.SalarioPactado, &c.ModalidadSalario, &c.JornadaID, &c.Jornada, &c.FechaInicioContrato, &c.FechaFinContrato,
		&c.FechaInicioVigencia, &c.FechaFinVigencia, &c.EsVigente, &c.CreadoPor, &c.CreatedAt)
	return c, err
}

func (q *Queries) GetContratoVigente(ctx context.Context, empleadoID int64) (Contrato, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasContrato+` FROM `+desdeContratos+` WHERE c.empleado_id = ? AND c.es_vigente = 1 LIMIT 1`, empleadoID)
	return scanContratoRow(row)
}

func (q *Queries) ListContratosHistoricoPorEmpleado(ctx context.Context, empleadoID int64, limit, offset int32) ([]Contrato, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasContrato+`
FROM `+desdeContratos+`
WHERE c.empleado_id = ?
ORDER BY c.fecha_inicio_vigencia DESC
LIMIT ? OFFSET ?`, empleadoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Contrato
	for rows.Next() {
		c, err := scanContratoRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (q *Queries) CountContratosHistoricoPorEmpleado(ctx context.Context, empleadoID int64) (int64, error) {
	row := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contratos WHERE empleado_id = ?`, empleadoID)
	var total int64
	err := row.Scan(&total)
	return total, err
}

func (q *Queries) ListContratos(ctx context.Context, esVigente *bool, puestoID, tipoContratoID *int64, limit, offset int32) ([]Contrato, error) {
	var vigArg, puestoArg, tipoArg interface{}
	if esVigente != nil {
		vigArg = *esVigente
	}
	if puestoID != nil {
		puestoArg = *puestoID
	}
	if tipoContratoID != nil {
		tipoArg = *tipoContratoID
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasContrato+`
FROM `+desdeContratos+`
WHERE (? IS NULL OR COALESCE(c.es_vigente, 0) = ?)
  AND (? IS NULL OR c.puesto_id = ?)
  AND (? IS NULL OR c.tipo_contrato_id = ?)
ORDER BY c.id DESC
LIMIT ? OFFSET ?`, vigArg, vigArg, puestoArg, puestoArg, tipoArg, tipoArg, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Contrato
	for rows.Next() {
		c, err := scanContratoRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (q *Queries) CountContratos(ctx context.Context, esVigente *bool, puestoID, tipoContratoID *int64) (int64, error) {
	var vigArg, puestoArg, tipoArg interface{}
	if esVigente != nil {
		vigArg = *esVigente
	}
	if puestoID != nil {
		puestoArg = *puestoID
	}
	if tipoContratoID != nil {
		tipoArg = *tipoContratoID
	}
	row := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM contratos c
WHERE (? IS NULL OR COALESCE(c.es_vigente, 0) = ?)
  AND (? IS NULL OR c.puesto_id = ?)
  AND (? IS NULL OR c.tipo_contrato_id = ?)`, vigArg, vigArg, puestoArg, puestoArg, tipoArg, tipoArg)
	var total int64
	err := row.Scan(&total)
	return total, err
}

func (q *Queries) CerrarContratoVigente(ctx context.Context, empleadoID int64, fechaFinVigencia time.Time) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE contratos
SET fecha_fin_vigencia = ?
WHERE empleado_id = ? AND es_vigente = 1`, fechaFinVigencia, empleadoID)
	return err
}

func (q *Queries) CrearVersionContrato(
	ctx context.Context,
	empleadoID, puestoID int64,
	regimenLaboralID, tipoContratoID int64,
	salarioPactado decimal.Decimal,
	modalidadSalario string,
	jornadaID int64,
	fechaInicioContrato time.Time,
	fechaFinContrato sql.NullTime,
	fechaInicioVigencia time.Time,
	creadoPor int64,
) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO contratos (
    empleado_id, puesto_id, regimen_laboral_id, tipo_contrato_id, salario_pactado, modalidad_salario, jornada_id,
    fecha_inicio_contrato, fecha_fin_contrato, fecha_inicio_vigencia, fecha_fin_vigencia,
    creado_por, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NOW())`,
		empleadoID, puestoID, regimenLaboralID, tipoContratoID, salarioPactado, modalidadSalario, jornadaID,
		fechaInicioContrato, fechaFinContrato, fechaInicioVigencia, creadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

type ParametroSalarioMinimo struct {
	ID                    int64
	ClasificacionPuestoID int64
	MontoMensual          decimal.Decimal
	FechaInicioVigencia   time.Time
	FechaFinVigencia      sql.NullTime
	FuenteLegal           sql.NullString
}

func (q *Queries) GetSalarioMinimoVigentePorClasificacion(ctx context.Context, clasificacionPuestoID int64) (ParametroSalarioMinimo, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, clasificacion_puesto_id, monto_mensual, fecha_inicio_vigencia, fecha_fin_vigencia, fuente_legal
FROM parametros_salario_minimo
WHERE clasificacion_puesto_id = ? AND es_vigente = 1
LIMIT 1`, clasificacionPuestoID)
	var p ParametroSalarioMinimo
	err := row.Scan(&p.ID, &p.ClasificacionPuestoID, &p.MontoMensual, &p.FechaInicioVigencia, &p.FechaFinVigencia, &p.FuenteLegal)
	return p, err
}

// GetSalarioMinimoVigentePorPuesto resuelve puestos -> clasificaciones_puesto
// -> parametros_salario_minimo en una sola consulta.
func (q *Queries) GetSalarioMinimoVigentePorPuesto(ctx context.Context, puestoID int64) (ParametroSalarioMinimo, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT psm.id, psm.clasificacion_puesto_id, psm.monto_mensual, psm.fecha_inicio_vigencia, psm.fecha_fin_vigencia, psm.fuente_legal
FROM puestos p
JOIN parametros_salario_minimo psm
  ON psm.clasificacion_puesto_id = p.clasificacion_puesto_id
 AND psm.es_vigente = 1
WHERE p.id = ?
LIMIT 1`, puestoID)
	var p ParametroSalarioMinimo
	err := row.Scan(&p.ID, &p.ClasificacionPuestoID, &p.MontoMensual, &p.FechaInicioVigencia, &p.FechaFinVigencia, &p.FuenteLegal)
	return p, err
}

// ExisteJornadaActiva indica si `id` es una jornada activa del catálogo.
func (q *Queries) ExisteJornadaActiva(ctx context.Context, id int64) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jornadas WHERE id = ? AND activo = 1`, id).Scan(&n)
	return n > 0, err
}

// ExisteTipoContrato indica si `id` existe en el catálogo tipos_contrato.
func (q *Queries) ExisteTipoContrato(ctx context.Context, id int64) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tipos_contrato WHERE id = ?`, id).Scan(&n)
	return n > 0, err
}

// ParametroJornadaVigente: divisor y horas diarias de la jornada a `fecha`.
type ParametroJornadaVigente struct {
	DivisorHorasMensuales  decimal.Decimal
	HorasDiariasOrdinarias decimal.NullDecimal
}

// ObtenerParametroJornadaEnFecha: parametros_jornada vigente de la jornada;
// sql.ErrNoRows si no hay.
func (q *Queries) ObtenerParametroJornadaEnFecha(ctx context.Context, jornadaID int64, fecha time.Time) (ParametroJornadaVigente, error) {
	var p ParametroJornadaVigente
	err := q.db.QueryRowContext(ctx, `
SELECT divisor_horas_mensuales, horas_diarias_ordinarias
FROM parametros_jornada
WHERE jornada_id = ? AND fecha_inicio_vigencia <= ? AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY fecha_inicio_vigencia DESC LIMIT 1`, jornadaID, fecha, fecha).Scan(&p.DivisorHorasMensuales, &p.HorasDiariasOrdinarias)
	return p, err
}
