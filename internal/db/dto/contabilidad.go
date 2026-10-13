package dto

// Fase 10 — Consultas de contabilización (sin sqlc, SQL escrito a mano con
// placeholders `?` siempre; nunca se concatenan valores en el SQL).
//
// Se usa un tipo propio (ContabilidadQueries) en vez de extender Queries; se
// obtiene con (*Queries).Contabilidad() dentro de db.WithAuditContext.
//
// TODA escritura de este archivo (asientos_contables, asientos_contables_detalle,
// periodos_planilla) dispara triggers trg_aud_* que leen @auditoria_usuario_id:
// solo debe invocarse desde una transacción abierta por db.WithAuditContext.

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

// Contabilidad devuelve las consultas de contabilización ligadas a la MISMA
// conexión/transacción que q. Es la forma de usarlas dentro de
// db.WithAuditContext (internal/db/audit.go), que entrega un *Queries ya
// ligado a la transacción auditada:
//
//	db.WithAuditContext(ctx, sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
//	    cq := q.Contabilidad()
//	    ...
//	})
//
// Este paquete ya NO abre transacciones ni fija @auditoria_usuario_id por su
// cuenta: eso sería saltarse el wrapper obligatorio (la variable de sesión
// sobrevive al COMMIT y quedaría pegada a una conexión del pool).
//
// Requiere que el campo db de Queries cumpla ContabilidadDB (sqlc.DBTX lo
// cumple: ExecContext, QueryContext y QueryRowContext).
func (q *Queries) Contabilidad() *ContabilidadQueries { return NewContabilidadQueries(q.db) }

// ---------------------------------------------------------------------------

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

// MarcarPeriodoContabilizado pasa el período a 'contabilizada' (valor del ENUM
// periodos_planilla.estado = planilla.EstadoContabilizada; dto no puede
// importar planilla, ver dashboard.go). Exige que el estado siga siendo
// estadoActual (control optimista); si no se afectó exactamente 1 fila
// retorna error.
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
