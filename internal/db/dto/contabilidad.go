package dto

// Fase 10 — Consultas de contabilización (sin sqlc, SQL escrito a mano con
// placeholders `?` siempre; nunca se concatenan valores en el SQL).
//
// Se usa un tipo propio (ContabilidadQueries) en vez de extender Queries para
// no asumir los campos internos de éste.

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ContabilidadDB es lo mínimo que comparten *sql.DB y *sql.Tx.
type ContabilidadDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type ContabilidadQueries struct{ db ContabilidadDB }

func NewContabilidadQueries(db ContabilidadDB) *ContabilidadQueries {
	return &ContabilidadQueries{db: db}
}

// ContabilidadStore abre transacciones. Uso típico:
//
//	dto.NewContabilidadStore(sqlDB)
type ContabilidadStore struct{ sqlDB *sql.DB }

func NewContabilidadStore(sqlDB *sql.DB) *ContabilidadStore { return &ContabilidadStore{sqlDB: sqlDB} }

// EnTx ejecuta fn dentro de una transacción; commit si fn devuelve nil,
// rollback en cualquier otro caso.
func (s *ContabilidadStore) EnTx(ctx context.Context, fn func(q *ContabilidadQueries) error) error {
	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(NewContabilidadQueries(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------

// FijarUsuarioAuditoria: los triggers trg_aud_* leen @auditoria_usuario_id.
// Es una variable de sesión, por eso debe ejecutarse en la misma conexión
// (la transacción) que las escrituras.
func (q *ContabilidadQueries) FijarUsuarioAuditoria(ctx context.Context, usuarioID int64) error {
	_, err := q.db.ExecContext(ctx, `SET @auditoria_usuario_id = ?`, usuarioID)
	return err
}

type PeriodoParaContabilizarRow struct {
	ID       int64
	FechaFin time.Time
	Estado   string
}

// ObtenerPeriodoParaContabilizar bloquea la fila del período (FOR UPDATE) para
// serializar corridas concurrentes de contabilización. sql.ErrNoRows si no existe.
func (q *ContabilidadQueries) ObtenerPeriodoParaContabilizar(ctx context.Context, periodoID int64) (PeriodoParaContabilizarRow, error) {
	var r PeriodoParaContabilizarRow
	err := q.db.QueryRowContext(ctx,
		`SELECT id, fecha_fin, estado FROM periodos_planilla WHERE id = ? FOR UPDATE`, periodoID,
	).Scan(&r.ID, &r.FechaFin, &r.Estado)
	return r, err
}

func (q *ContabilidadQueries) ContarAsientosDelPeriodo(ctx context.Context, periodoID int64) (int64, error) {
	var n int64
	err := q.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM asientos_contables WHERE periodo_planilla_id = ?`, periodoID).Scan(&n)
	return n, err
}

type MapeoVigenteRow struct {
	ID                       int64
	ConceptoSalarialID       int64
	ConceptoCodigo           string
	ProgramaPresupuestarioID int64
	TipoAsientoID            int64
	TipoAsientoCodigo        string
	CuentaContableID         int64
	TipoMovimiento           string
	Proyecto                 string
}

// ListarMapeosVigentesEnFecha devuelve los mapeos cuya vigencia cubre `fecha`.
func (q *ContabilidadQueries) ListarMapeosVigentesEnFecha(ctx context.Context, fecha time.Time) ([]MapeoVigenteRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT m.id, m.concepto_salarial_id, cs.codigo, m.programa_presupuestario_id,
       m.tipo_asiento_id, ta.codigo, m.cuenta_contable_id, m.tipo_movimiento, m.proyecto
FROM mapeo_concepto_cuenta m
JOIN conceptos_salariales cs ON cs.id = m.concepto_salarial_id
JOIN tipos_asiento ta        ON ta.id = m.tipo_asiento_id
WHERE m.fecha_inicio_vigencia <= ?
  AND (m.fecha_fin_vigencia IS NULL OR m.fecha_fin_vigencia >= ?)
ORDER BY m.concepto_salarial_id, m.programa_presupuestario_id, m.tipo_asiento_id`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MapeoVigenteRow
	for rows.Next() {
		var r MapeoVigenteRow
		if err := rows.Scan(&r.ID, &r.ConceptoSalarialID, &r.ConceptoCodigo, &r.ProgramaPresupuestarioID,
			&r.TipoAsientoID, &r.TipoAsientoCodigo, &r.CuentaContableID, &r.TipoMovimiento, &r.Proyecto); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type CrearAsientoContableParams struct {
	PeriodoPlanillaID int64
	TipoAsientoID     int64
	NumeroAsiento     string
	FechaGeneracion   time.Time
	Estado            string
}

func (q *ContabilidadQueries) CrearAsientoContable(ctx context.Context, p CrearAsientoContableParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO asientos_contables (periodo_planilla_id, tipo_asiento_id, numero_asiento, fecha_generacion, estado)
VALUES (?, ?, ?, ?, ?)`, p.PeriodoPlanillaID, p.TipoAsientoID, p.NumeroAsiento, p.FechaGeneracion, p.Estado)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

type CrearAsientoContableDetalleParams struct {
	AsientoContableID        int64
	CuentaContableID         int64
	ProgramaPresupuestarioID int64
	Proyecto                 string
	EmpleadoID               sql.NullInt64
	ConceptoSalarialID       sql.NullInt64
	MontoDebe                decimal.Decimal
	MontoHaber               decimal.Decimal
}

func (q *ContabilidadQueries) CrearAsientoContableDetalle(ctx context.Context, p CrearAsientoContableDetalleParams) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO asientos_contables_detalle
  (asiento_contable_id, cuenta_contable_id, programa_presupuestario_id, proyecto,
   empleado_id, concepto_salarial_id, monto_debe, monto_haber)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.AsientoContableID, p.CuentaContableID, p.ProgramaPresupuestarioID, p.Proyecto,
		p.EmpleadoID, p.ConceptoSalarialID, p.MontoDebe, p.MontoHaber)
	return err
}

// MarcarPeriodoContabilizado exige que el estado siga siendo estadoActual
// (optimista); si no se afectó exactamente 1 fila retorna error.
func (q *ContabilidadQueries) MarcarPeriodoContabilizado(ctx context.Context, periodoID int64, estadoActual string) error {
	res, err := q.db.ExecContext(ctx,
		`UPDATE periodos_planilla SET estado = 'contabilizada' WHERE id = ? AND estado = ?`, periodoID, estadoActual)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("dto: se esperaba actualizar 1 período y se actualizaron %d (¿cambió de estado?)", n)
	}
	return nil
}