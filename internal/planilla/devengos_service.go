package planilla

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/paginacion"
)

var (
	ErrDevengoNoEncontrado  = &ErrorCiclo{"devengo_no_encontrado", "el devengo manual no existe en este período", 404}
	ErrConceptoNoCapturable = &ErrorCiclo{"concepto_no_capturable", "el concepto no es de captura manual, no está activo o no lo calcula este tipo de planilla", 422}
	ErrEmpleadoNoActivo     = &ErrorCiclo{"empleado_no_activo", "el empleado no existe o no está activo", 422}
	ErrMontoDevengoInvalido = &ErrorCiclo{"monto_invalido", "el monto debe ser mayor que cero y tener como máximo 2 decimales", 400}
	ErrObservacionMuyLarga  = &ErrorCiclo{"observacion_muy_larga", "la observación admite como máximo 255 caracteres", 400}
)

// NuevoDevengo: datos de captura.
type NuevoDevengo struct {
	EmpleadoID         int64
	ConceptoSalarialID int64
	Monto              decimal.Decimal
	Observacion        *string
}

type DevengosService struct{ db *sql.DB }

func NewDevengosService(sqlDB *sql.DB) *DevengosService { return &DevengosService{db: sqlDB} }

// enBorrador bloquea el período (FOR UPDATE: serializa con Calcular) y exige
// estado borrador.
func enBorrador(ctx context.Context, q *dto.Queries, periodoID int64) (dto.PeriodoCicloRow, error) {
	p, err := q.ObtenerPeriodoCicloParaActualizar(ctx, periodoID)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrPeriodoNoEncontrado
	}
	if err != nil {
		return p, err
	}
	if EstadoPeriodo(p.Estado) != EstadoBorrador {
		return p, estadoInvalido(EstadoBorrador, p.Estado)
	}
	return p, nil
}

func (s *DevengosService) Crear(ctx context.Context, usuarioID, periodoID int64, in NuevoDevengo) (dto.DevengoManualRow, error) {
	if !in.Monto.IsPositive() || !in.Monto.Equal(in.Monto.Round(2)) || in.Monto.GreaterThanOrEqual(decimal.New(1, 12)) {
		return dto.DevengoManualRow{}, ErrMontoDevengoInvalido
	}
	if in.Observacion != nil && len([]rune(*in.Observacion)) > 255 {
		return dto.DevengoManualRow{}, ErrObservacionMuyLarga
	}
	var out dto.DevengoManualRow
	err := db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		p, err := enBorrador(ctx, q, periodoID)
		if err != nil {
			return err
		}
		ok, err := q.ConceptoCapturable(ctx, p.TipoPlanillaID, in.ConceptoSalarialID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrConceptoNoCapturable
		}
		if ok, err = q.EmpleadoActivo(ctx, in.EmpleadoID); err != nil {
			return err
		} else if !ok {
			return ErrEmpleadoNoActivo
		}
		id, err := q.InsertarDevengoManual(ctx, dto.InsertarDevengoManualParams{
			PeriodoPlanillaID: periodoID, EmpleadoID: in.EmpleadoID, ConceptoSalarialID: in.ConceptoSalarialID,
			Monto: in.Monto, Observacion: in.Observacion, CreadoPor: usuarioID,
		})
		if err != nil {
			return err
		}
		out, err = q.ObtenerDevengoManual(ctx, periodoID, id)
		return err
	})
	return out, traducirErrorBD(err)
}

func (s *DevengosService) Listar(ctx context.Context, periodoID int64, empleadoID *int64, pag paginacion.Parametros) ([]dto.DevengoManualRow, int64, error) {
	q := dto.New(s.db)
	if _, err := q.ObtenerPeriodoCiclo(ctx, periodoID); errors.Is(err, sql.ErrNoRows) {
		return nil, 0, ErrPeriodoNoEncontrado
	} else if err != nil {
		return nil, 0, err
	}
	total, err := q.ContarDevengosManuales(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, 0, err
	}
	filas, err := q.ListarDevengosManuales(ctx, periodoID, empleadoID, pag.Limit32(), pag.Offset32())
	return filas, total, err
}

func (s *DevengosService) Eliminar(ctx context.Context, usuarioID, periodoID, devengoID int64) error {
	err := db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if _, err := enBorrador(ctx, q, periodoID); err != nil {
			return err
		}
		n, err := q.EliminarDevengoManual(ctx, periodoID, devengoID)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrDevengoNoEncontrado
		}
		return nil
	})
	return traducirErrorBD(err)
}
