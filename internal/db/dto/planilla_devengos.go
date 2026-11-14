package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type DevengoManualRow struct {
	ID                 int64
	PeriodoPlanillaID  int64
	EmpleadoID         int64
	ConceptoSalarialID int64
	ConceptoCodigo     string
	Monto              decimal.Decimal
	Observacion        *string
	CreadoPor          int64
	CreatedAt          time.Time
}

const selectDevengoManual = `
SELECT d.id, d.periodo_planilla_id, d.empleado_id, d.concepto_salarial_id, cs.codigo,
       d.monto, d.observacion, d.creado_por, d.created_at
FROM planilla_devengos_manuales d
JOIN conceptos_salariales cs ON cs.id = d.concepto_salarial_id`

func scanDevengoManual(s interface{ Scan(...any) error }) (DevengoManualRow, error) {
	var r DevengoManualRow
	var obs sql.NullString
	err := s.Scan(&r.ID, &r.PeriodoPlanillaID, &r.EmpleadoID, &r.ConceptoSalarialID, &r.ConceptoCodigo,
		&r.Monto, &obs, &r.CreadoPor, &r.CreatedAt)
	if obs.Valid {
		r.Observacion = &obs.String
	}
	return r, err
}

type InsertarDevengoManualParams struct {
	PeriodoPlanillaID  int64
	EmpleadoID         int64
	ConceptoSalarialID int64
	Monto              decimal.Decimal
	Observacion        *string
	CreadoPor          int64
}

func (q *Queries) InsertarDevengoManual(ctx context.Context, p InsertarDevengoManualParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO planilla_devengos_manuales (periodo_planilla_id, empleado_id, concepto_salarial_id, monto, observacion, creado_por)
VALUES (?, ?, ?, ?, ?, ?)`, p.PeriodoPlanillaID, p.EmpleadoID, p.ConceptoSalarialID, p.Monto, p.Observacion, p.CreadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ObtenerDevengoManual: sql.ErrNoRows si no existe en ese período.
func (q *Queries) ObtenerDevengoManual(ctx context.Context, periodoID, id int64) (DevengoManualRow, error) {
	return scanDevengoManual(q.db.QueryRowContext(ctx, selectDevengoManual+`
WHERE d.periodo_planilla_id = ? AND d.id = ?`, periodoID, id))
}

// ListarDevengosManuales: empleadoID nil = todos los empleados del período.
func (q *Queries) ListarDevengosManuales(ctx context.Context, periodoID int64, empleadoID *int64, limit, offset int32) ([]DevengoManualRow, error) {
	rows, err := q.db.QueryContext(ctx, selectDevengoManual+`
WHERE d.periodo_planilla_id = ? AND (? IS NULL OR d.empleado_id = ?)
ORDER BY d.empleado_id ASC, d.id ASC
LIMIT ? OFFSET ?`, periodoID, empleadoID, empleadoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DevengoManualRow
	for rows.Next() {
		r, err := scanDevengoManual(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ContarDevengosManuales(ctx context.Context, periodoID int64, empleadoID *int64) (int64, error) {
	var n int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM planilla_devengos_manuales
WHERE periodo_planilla_id = ? AND (? IS NULL OR empleado_id = ?)`, periodoID, empleadoID, empleadoID).Scan(&n)
	return n, err
}

// EliminarDevengoManual devuelve filas afectadas (0 = no existía en ese período).
func (q *Queries) EliminarDevengoManual(ctx context.Context, periodoID, id int64) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`DELETE FROM planilla_devengos_manuales WHERE periodo_planilla_id = ? AND id = ?`, periodoID, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ConceptoCapturable: concepto activo, de captura manual (ingreso o
// deducción) y que el tipo de planilla calcula (tipo_planilla_concepto).
func (q *Queries) ConceptoCapturable(ctx context.Context, tipoPlanillaID, conceptoID int64) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM conceptos_salariales cs
JOIN tipo_planilla_concepto tpc ON tpc.concepto_salarial_id = cs.id AND tpc.tipo_planilla_id = ?
WHERE cs.id = ? AND cs.activo = 1 AND cs.tipo_calculo = 'captura_manual'`,
		tipoPlanillaID, conceptoID).Scan(&n)
	return n > 0, err
}

// EmpleadoActivo indica si el empleado existe y está activo.
func (q *Queries) EmpleadoActivo(ctx context.Context, empleadoID int64) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM empleados WHERE id = ? AND activo = 1`, empleadoID).Scan(&n)
	return n > 0, err
}
