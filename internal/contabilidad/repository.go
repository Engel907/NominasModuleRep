package contabilidad

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	auditdb "planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

// Wiring (main.go / buildDeps):
//
//	contabilidad.NewRepositorio(sqlDB)
//
// Toda escritura pasa por db.WithAuditContext: fija @auditoria_usuario_id para
// los triggers trg_aud_*, abre la transacción, hace commit/rollback y limpia la
// variable de sesión antes de devolver la conexión al pool. Este paquete no
// abre transacciones por su cuenta.

type repositorio struct{ sqlDB *sql.DB }

func NewRepositorio(sqlDB *sql.DB) Repositorio { return &repositorio{sqlDB: sqlDB} }

func (r *repositorio) EjecutarEnTx(ctx context.Context, usuarioID int64, fn func(tx RepositorioTx) error) error {
	return auditdb.WithAuditContext(ctx, r.sqlDB, usuarioID, func(_ context.Context, q *dto.Queries) error {
		return fn(&repoTx{q: q.Contabilidad()})
	})
}

type repoTx struct{ q *dto.ContabilidadQueries }

func (t *repoTx) ObtenerPeriodoParaContabilizar(ctx context.Context, periodoID int64) (PeriodoContable, error) {
	x, err := t.q.ObtenerPeriodoParaContabilizar(ctx, periodoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PeriodoContable{}, fmt.Errorf("%w: periodo_id=%d", ErrPeriodoNoExiste, periodoID)
		}
		return PeriodoContable{}, err
	}
	return PeriodoContable{ID: x.ID, FechaFin: x.FechaFin, Estado: x.Estado}, nil
}

func (t *repoTx) ContarAsientosDelPeriodo(ctx context.Context, periodoID int64) (int64, error) {
	return t.q.ContarAsientosDelPeriodo(ctx, periodoID)
}

func (t *repoTx) ListarMapeosVigentesEnFecha(ctx context.Context, fecha time.Time) ([]MapeoConceptoCuenta, error) {
	rows, err := t.q.ListarMapeosVigentesEnFecha(ctx, fecha)
	if err != nil {
		return nil, err
	}
	out := make([]MapeoConceptoCuenta, 0, len(rows))
	for _, x := range rows {
		out = append(out, MapeoConceptoCuenta(x))
	}
	return out, nil
}

func (t *repoTx) GuardarAsiento(ctx context.Context, a AsientoContable) (int64, error) {
	id, err := t.q.CrearAsientoContable(ctx, dto.CrearAsientoContableParams{
		PeriodoPlanillaID: a.PeriodoPlanillaID, TipoAsientoID: a.TipoAsientoID,
		NumeroAsiento: a.NumeroAsiento, FechaGeneracion: a.FechaGeneracion, Estado: a.Estado,
	})
	if err != nil {
		return 0, err
	}
	for _, l := range a.Lineas {
		p := dto.CrearAsientoContableDetalleParams{
			AsientoContableID: id, CuentaContableID: l.CuentaContableID,
			ProgramaPresupuestarioID: l.ProgramaPresupuestarioID, Proyecto: l.Proyecto,
			MontoDebe: l.MontoDebe, MontoHaber: l.MontoHaber,
		}
		if l.EmpleadoID != nil {
			p.EmpleadoID = sql.NullInt64{Int64: *l.EmpleadoID, Valid: true}
		}
		if l.ConceptoSalarialID != nil {
			p.ConceptoSalarialID = sql.NullInt64{Int64: *l.ConceptoSalarialID, Valid: true}
		}
		if err := t.q.CrearAsientoContableDetalle(ctx, p); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (t *repoTx) MarcarPeriodoContabilizado(ctx context.Context, periodoID int64, estadoActual string) error {
	return t.q.MarcarPeriodoContabilizado(ctx, periodoID, estadoActual)
}
