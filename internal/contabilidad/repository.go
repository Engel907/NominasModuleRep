package contabilidad

import (
	"context"
	"database/sql"
	"time"

	"planillas-ccdr/internal/db/dto"
)

// Wiring: contabilidad.NewRepositorio(dto.NewContabilidadStore(sqlDB)).

type store interface {
	EnTx(ctx context.Context, fn func(q *dto.ContabilidadQueries) error) error
}

type repositorio struct{ s store }

func NewRepositorio(s store) Repositorio { return &repositorio{s: s} }

func (r *repositorio) EjecutarEnTx(ctx context.Context, fn func(tx RepositorioTx) error) error {
	return r.s.EnTx(ctx, func(q *dto.ContabilidadQueries) error { return fn(&repoTx{q: q}) })
}

type repoTx struct{ q *dto.ContabilidadQueries }

func (t *repoTx) FijarUsuarioAuditoria(ctx context.Context, usuarioID int64) error {
	return t.q.FijarUsuarioAuditoria(ctx, usuarioID)
}

func (t *repoTx) ObtenerPeriodoParaContabilizar(ctx context.Context, periodoID int64) (PeriodoContable, error) {
	x, err := t.q.ObtenerPeriodoParaContabilizar(ctx, periodoID)
	if err != nil {
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