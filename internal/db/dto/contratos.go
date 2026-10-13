// internal/db/dto/contratos.go
//
// Fase 3B — RRHH: contratos (versionados, patrón 2-pasos) y lectura de
// parametros_salario_minimo para la validación de salario mínimo.
//
// NOTA DE MIGRACIÓN: la tabla `contratos` real (ver db/schema.sql, sección
// "CONTRATOS") SÍ coincide en su mayoría con lo que asumía
// db/queries/empleados_puestos.sql (empleado_id, puesto_id, tipo_contrato
// VARCHAR libre, salario_pactado, jornada, fechas, patrón de vigencia,
// creado_por, created_at) — es la única de las tablas de Fase 3 que además
// SÍ tiene columna `creado_por` a nivel de negocio, no solo trigger
// genérico. La única diferencia real es que `contratos.regimen_laboral_id`
// es NOT NULL (FK a regimenes_laborales) y ninguno de los DTOs originales
// de contrato_service.go lo contemplaba. Se agregó
// CrearContratoRequest.RegimenLaboralID / ActualizarContratoRequest.RegimenLaboralID
// en internal/services/contrato_service.go — no hay valor por defecto
// razonable para una FK obligatoria sin ese dato desde el cliente.
package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// CONTRATOS
// ============================================================================

type Contrato struct {
	ID                  int64
	EmpleadoID          int64
	PuestoID            int64
	RegimenLaboralID    int64
	TipoContrato        string
	SalarioPactado      decimal.Decimal
	Jornada             string
	FechaInicioContrato time.Time
	FechaFinContrato    sql.NullTime
	FechaInicioVigencia time.Time
	FechaFinVigencia    sql.NullTime
	EsVigente           bool
	CreadoPor           int64
	CreatedAt           time.Time
}

// F03: es_vigente es una columna GENERATED que vale 1 si la fila está vigente y
// NULL (no 0) si está cerrada. Scan de NULL a bool falla, así que se
// COALESCE-a a 0 en todas las lecturas. Por la misma razón los filtros
// por "no vigentes" usan COALESCE(c.es_vigente, 0) = ? (es_vigente = 0 nunca
// matchea, porque el valor real es NULL).
const columnasContrato = `id, empleado_id, puesto_id, regimen_laboral_id, tipo_contrato,
       salario_pactado, jornada, fecha_inicio_contrato, fecha_fin_contrato,
       fecha_inicio_vigencia, fecha_fin_vigencia, COALESCE(es_vigente, 0), creado_por, created_at`

func scanContratoRow(row *sql.Row) (Contrato, error) {
	var c Contrato
	err := row.Scan(&c.ID, &c.EmpleadoID, &c.PuestoID, &c.RegimenLaboralID, &c.TipoContrato,
		&c.SalarioPactado, &c.Jornada, &c.FechaInicioContrato, &c.FechaFinContrato,
		&c.FechaInicioVigencia, &c.FechaFinVigencia, &c.EsVigente, &c.CreadoPor, &c.CreatedAt)
	return c, err
}

func scanContratoRows(rows *sql.Rows) (Contrato, error) {
	var c Contrato
	err := rows.Scan(&c.ID, &c.EmpleadoID, &c.PuestoID, &c.RegimenLaboralID, &c.TipoContrato,
		&c.SalarioPactado, &c.Jornada, &c.FechaInicioContrato, &c.FechaFinContrato,
		&c.FechaInicioVigencia, &c.FechaFinVigencia, &c.EsVigente, &c.CreadoPor, &c.CreatedAt)
	return c, err
}

func (q *Queries) GetContratoVigente(ctx context.Context, empleadoID int64) (Contrato, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columnasContrato+` FROM contratos WHERE empleado_id = ? AND es_vigente = 1 LIMIT 1`, empleadoID)
	return scanContratoRow(row)
}

func (q *Queries) ListContratosHistoricoPorEmpleado(ctx context.Context, empleadoID int64, limit, offset int32) ([]Contrato, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasContrato+`
FROM contratos
WHERE empleado_id = ?
ORDER BY fecha_inicio_vigencia DESC
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

func (q *Queries) ListContratos(ctx context.Context, esVigente *bool, puestoID *int64, tipoContrato *string, limit, offset int32) ([]Contrato, error) {
	var vigArg, puestoArg, tipoArg interface{}
	if esVigente != nil {
		vigArg = *esVigente
	}
	if puestoID != nil {
		puestoArg = *puestoID
	}
	if tipoContrato != nil {
		tipoArg = *tipoContrato
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT `+columnasContrato+`
FROM contratos c
WHERE (? IS NULL OR COALESCE(c.es_vigente, 0) = ?)
  AND (? IS NULL OR c.puesto_id = ?)
  AND (? IS NULL OR c.tipo_contrato = ?)
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

func (q *Queries) CountContratos(ctx context.Context, esVigente *bool, puestoID *int64, tipoContrato *string) (int64, error) {
	var vigArg, puestoArg, tipoArg interface{}
	if esVigente != nil {
		vigArg = *esVigente
	}
	if puestoID != nil {
		puestoArg = *puestoID
	}
	if tipoContrato != nil {
		tipoArg = *tipoContrato
	}
	row := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM contratos c
WHERE (? IS NULL OR COALESCE(c.es_vigente, 0) = ?)
  AND (? IS NULL OR c.puesto_id = ?)
  AND (? IS NULL OR c.tipo_contrato = ?)`, vigArg, vigArg, puestoArg, puestoArg, tipoArg, tipoArg)
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
	regimenLaboralID int64,
	tipoContrato string,
	salarioPactado decimal.Decimal,
	jornada string,
	fechaInicioContrato time.Time,
	fechaFinContrato sql.NullTime,
	fechaInicioVigencia time.Time,
	creadoPor int64,
) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO contratos (
    empleado_id, puesto_id, regimen_laboral_id, tipo_contrato, salario_pactado, jornada,
    fecha_inicio_contrato, fecha_fin_contrato, fecha_inicio_vigencia, fecha_fin_vigencia,
    creado_por, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NOW())`,
		empleadoID, puestoID, regimenLaboralID, tipoContrato, salarioPactado, jornada,
		fechaInicioContrato, fechaFinContrato, fechaInicioVigencia, creadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ============================================================================
// PARAMETROS_SALARIO_MINIMO (solo lectura en esta fase)
// ============================================================================

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
// -> parametros_salario_minimo en una sola consulta, igual que la query
// original — esta parte del schema sí coincidía con lo asumido.
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
