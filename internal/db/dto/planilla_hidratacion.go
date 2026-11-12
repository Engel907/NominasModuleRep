// internal/db/dto/planilla_hidratacion.go
//
// Lecturas que usa el hidratador del cálculo (internal/planilla/hidratador.go)
// y que no existían en planilla_periodo.go / planilla_motor.go.
package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type DevengoManualEmpleadoRow struct {
	ID                 int64
	ConceptoSalarialID int64
	Monto              decimal.Decimal
	Observacion        *string
}

// ListarDevengosManualesEmpleado: devengos capturados a mano
// (planilla_devengos_manuales) de un empleado en un período.
func (q *Queries) ListarDevengosManualesEmpleado(ctx context.Context, periodoID, empleadoID int64) ([]DevengoManualEmpleadoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, concepto_salarial_id, monto, observacion
FROM planilla_devengos_manuales
WHERE periodo_planilla_id = ?
  AND empleado_id = ?
ORDER BY id ASC`, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DevengoManualEmpleadoRow
	for rows.Next() {
		var r DevengoManualEmpleadoRow
		var obs sql.NullString
		if err := rows.Scan(&r.ID, &r.ConceptoSalarialID, &r.Monto, &obs); err != nil {
			return nil, err
		}
		if obs.Valid {
			r.Observacion = &obs.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type ContextoLaboralRow struct {
	ProgramaPresupuestarioID int64
	ClasificacionPuestoID    *int64 // puestos.clasificacion_puesto_id es NULL-able
	RegimenCCSSID            *int64 // la columna aún no existe en BD: siempre nil hasta L15
}

// ObtenerContextoLaboral: datos laborales vigentes a `fecha` (versión de
// empleado_datos_laborales que cubre la fecha) y la clasificación de su
// puesto. Devuelve sql.ErrNoRows si no hay versión vigente.
func (q *Queries) ObtenerContextoLaboral(ctx context.Context, empleadoID int64, fecha time.Time) (ContextoLaboralRow, error) {
	var r ContextoLaboralRow
	var clasif sql.NullInt64
	err := q.db.QueryRowContext(ctx, `
SELECT edl.programa_presupuestario_id, p.clasificacion_puesto_id
FROM empleado_datos_laborales edl
JOIN puestos p ON p.id = edl.puesto_id
WHERE edl.empleado_id = ?
  AND edl.fecha_inicio_vigencia <= ?
  AND (edl.fecha_fin_vigencia IS NULL OR edl.fecha_fin_vigencia >= ?)
ORDER BY edl.fecha_inicio_vigencia DESC
LIMIT 1`, empleadoID, fecha, fecha).Scan(&r.ProgramaPresupuestarioID, &clasif)
	if err != nil {
		return ContextoLaboralRow{}, err
	}
	if clasif.Valid {
		v := clasif.Int64
		r.ClasificacionPuestoID = &v
	}
	return r, nil
}

type ContratoEnRangoRow struct {
	ContratoID          int64
	SalarioPactado      decimal.Decimal
	FechaInicioVigencia time.Time
	FechaFinVigencia    *time.Time
	FechaInicioContrato time.Time
	FechaFinContrato    *time.Time
}

// ListarContratosEnRango: versiones de contrato del empleado cuya vigencia se
// solapa con [desde, hasta], por inicio de vigencia (L10: segmentación del
// salario por ingreso, cese y aumentos).
func (q *Queries) ListarContratosEnRango(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]ContratoEnRangoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, salario_pactado, fecha_inicio_vigencia, fecha_fin_vigencia, fecha_inicio_contrato, fecha_fin_contrato
FROM contratos
WHERE empleado_id = ?
  AND fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY fecha_inicio_vigencia ASC, id ASC`, empleadoID, hasta, desde)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContratoEnRangoRow
	for rows.Next() {
		var r ContratoEnRangoRow
		var finVig, finCon sql.NullTime
		if err := rows.Scan(&r.ContratoID, &r.SalarioPactado, &r.FechaInicioVigencia, &finVig, &r.FechaInicioContrato, &finCon); err != nil {
			return nil, err
		}
		if finVig.Valid {
			r.FechaFinVigencia = &finVig.Time
		}
		if finCon.Valid {
			r.FechaFinContrato = &finCon.Time
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
