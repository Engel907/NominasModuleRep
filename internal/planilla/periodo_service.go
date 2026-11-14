package planilla

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/paginacion"
)

// PeriodoService orquesta el ciclo de vida del período.
type PeriodoService struct {
	db      *sql.DB
	ganchos PeriodoGanchos
}

func NewPeriodoService(sqlDB *sql.DB, ganchos PeriodoGanchos) *PeriodoService {
	return &PeriodoService{db: sqlDB, ganchos: ganchos}
}

// traducirErrorBD convierte los SIGNAL 45000 de los triggers (MySQL 1644) en
// errores de dominio, por el prefijo del mensaje.
func traducirErrorBD(err error) error {
	var me *mysql.MySQLError
	if err == nil || !errors.As(err, &me) || me.Number != 1644 {
		return err
	}
	switch {
	case strings.HasPrefix(me.Message, "segregacion_funciones"):
		return ErrSegregacionFunciones
	case strings.HasPrefix(me.Message, "transicion_invalida"):
		return ErrTransicionInvalida
	case strings.HasPrefix(me.Message, "periodo_inmutable"):
		return ErrPeriodoInmutable
	case strings.HasPrefix(me.Message, "detalle_descuadrado"):
		return ErrDetalleDescuadrado
	}
	return err
}

// Crear crea un período en borrador. Serializa por tipo de planilla para que
// el chequeo de traslape y el INSERT sean atómicos.
func (s *PeriodoService) Crear(ctx context.Context, usuarioID, tipoID int64, inicio, fin time.Time) (dto.PeriodoCicloRow, error) {
	if tipoID <= 0 || inicio.IsZero() || fin.IsZero() || fin.Before(inicio) {
		return dto.PeriodoCicloRow{}, ErrFechasInvalidas
	}
	var out dto.PeriodoCicloRow
	err := db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := q.BloquearTipoPlanilla(ctx, tipoID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTipoPlanillaNoEncontrado
			}
			return err
		}
		traslape, err := q.ExisteTraslapePeriodo(ctx, tipoID, inicio, fin)
		if err != nil {
			return err
		}
		if traslape {
			return ErrPeriodoTraslapado
		}
		id, err := q.InsertarPeriodoPlanilla(ctx, tipoID, inicio, fin)
		if err != nil {
			return err
		}
		out, err = q.ObtenerPeriodoCiclo(ctx, id)
		return err
	})
	return out, traducirErrorBD(err)
}

func (s *PeriodoService) Obtener(ctx context.Context, id int64) (dto.PeriodoCicloRow, error) {
	p, err := dto.New(s.db).ObtenerPeriodoCiclo(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrPeriodoNoEncontrado
	}
	return p, err
}

func (s *PeriodoService) Listar(ctx context.Context, f dto.FiltroPeriodosCiclo, pag paginacion.Parametros) ([]dto.PeriodoCicloRow, int64, error) {
	q := dto.New(s.db)
	total, err := q.ContarPeriodosCiclo(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	filas, err := q.ListarPeriodosCiclo(ctx, f, pag.Limit32(), pag.Offset32())
	return filas, total, err
}

// transicionar abre la transacción auditada, bloquea el período (FOR UPDATE),
// ejecuta fn y devuelve el período actualizado. Dos llamadas simultáneas se
// serializan en el lock: la segunda ve el estado nuevo y recibe 409.
func (s *PeriodoService) transicionar(ctx context.Context, usuarioID, periodoID int64,
	fn func(ctx context.Context, q *dto.Queries, p dto.PeriodoCicloRow) error) (dto.PeriodoCicloRow, error) {
	var out dto.PeriodoCicloRow
	err := db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		p, err := q.ObtenerPeriodoCicloParaActualizar(ctx, periodoID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPeriodoNoEncontrado
		}
		if err != nil {
			return err
		}
		if err := fn(ctx, q, p); err != nil {
			return err
		}
		out, err = q.ObtenerPeriodoCiclo(ctx, periodoID)
		return err
	})
	return out, traducirErrorBD(err)
}

// Calcular: borrador -> calculada, delegando el cálculo al gancho.
func (s *PeriodoService) Calcular(ctx context.Context, usuarioID, periodoID int64) (dto.PeriodoCicloRow, error) {
	if s.ganchos.Calculador == nil {
		return dto.PeriodoCicloRow{}, ErrCalculoNoDisponible
	}
	return s.transicionar(ctx, usuarioID, periodoID, func(ctx context.Context, q *dto.Queries, p dto.PeriodoCicloRow) error {
		if EstadoPeriodo(p.Estado) != EstadoBorrador {
			return estadoInvalido(EstadoBorrador, p.Estado)
		}
		return s.calcularEnTx(ctx, q, p, usuarioID)
	})
}

func (s *PeriodoService) calcularEnTx(ctx context.Context, q *dto.Queries, p dto.PeriodoCicloRow, usuarioID int64) error {
	if err := s.ganchos.Calculador.Calcular(ctx, q, p, usuarioID); err != nil {
		return err
	}
	n, err := q.MarcarPeriodoCalculado(ctx, p.ID, usuarioID)
	if err != nil {
		return err
	}
	if n == 0 {
		return estadoInvalido(EstadoBorrador, p.Estado)
	}
	return nil
}

// descartarEnTx: calculada -> borrador eliminando el detalle.
func (s *PeriodoService) descartarEnTx(ctx context.Context, q *dto.Queries, p dto.PeriodoCicloRow) error {
	if EstadoPeriodo(p.Estado) != EstadoCalculada {
		return estadoInvalido(EstadoCalculada, p.Estado)
	}
	if err := s.ganchos.Eliminador.EliminarDetalle(ctx, q, p.ID); err != nil {
		return err
	}
	n, err := q.MarcarPeriodoBorrador(ctx, p.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return estadoInvalido(EstadoCalculada, p.Estado)
	}
	return nil
}

// Descartar: calculada -> borrador (elimina el detalle calculado).
func (s *PeriodoService) Descartar(ctx context.Context, usuarioID, periodoID int64) (dto.PeriodoCicloRow, error) {
	if s.ganchos.Eliminador == nil {
		return dto.PeriodoCicloRow{}, ErrCalculoNoDisponible
	}
	return s.transicionar(ctx, usuarioID, periodoID, func(ctx context.Context, q *dto.Queries, p dto.PeriodoCicloRow) error {
		return s.descartarEnTx(ctx, q, p)
	})
}

// Recalcular: calculada -> borrador -> calculada, en una sola transacción.
func (s *PeriodoService) Recalcular(ctx context.Context, usuarioID, periodoID int64) (dto.PeriodoCicloRow, error) {
	if s.ganchos.Calculador == nil || s.ganchos.Eliminador == nil {
		return dto.PeriodoCicloRow{}, ErrCalculoNoDisponible
	}
	return s.transicionar(ctx, usuarioID, periodoID, func(ctx context.Context, q *dto.Queries, p dto.PeriodoCicloRow) error {
		if err := s.descartarEnTx(ctx, q, p); err != nil {
			return err
		}
		borrador, err := q.ObtenerPeriodoCiclo(ctx, p.ID)
		if err != nil {
			return err
		}
		return s.calcularEnTx(ctx, q, borrador, usuarioID)
	})
}

// Aprobar: calculada -> aprobada (líneas calculado -> aprobado).
func (s *PeriodoService) Aprobar(ctx context.Context, usuarioID, periodoID int64) (dto.PeriodoCicloRow, error) {
	return s.transicionar(ctx, usuarioID, periodoID, func(ctx context.Context, q *dto.Queries, p dto.PeriodoCicloRow) error {
		if EstadoPeriodo(p.Estado) != EstadoCalculada {
			return estadoInvalido(EstadoCalculada, p.Estado)
		}
		if err := VerificarSegregacion(EstadoAprobada, usuarioID, ParticipantesDe(p)); err != nil {
			return err
		}
		tieneDetalle, err := q.TieneDetallePeriodo(ctx, p.ID)
		if err != nil {
			return err
		}
		if !tieneDetalle {
			return ErrPeriodoSinDetalle
		}
		n, err := q.MarcarPeriodoAprobado(ctx, p.ID, usuarioID)
		if err != nil {
			return err
		}
		if n == 0 {
			return estadoInvalido(EstadoCalculada, p.Estado)
		}
		filas, err := q.ActualizarEstadoDetallePeriodo(ctx, p.ID, "calculado", "aprobado")
		if err != nil {
			return err
		}
		if filas == 0 {
			return ErrPeriodoSinDetalle
		}
		if s.ganchos.AlAprobar != nil {
			act, err := q.ObtenerPeriodoCiclo(ctx, p.ID)
			if err != nil {
				return err
			}
			return s.ganchos.AlAprobar.AlAprobar(ctx, q, act)
		}
		return nil
	})
}

// Pagar: aprobada -> pagada (líneas aprobado -> pagado).
func (s *PeriodoService) Pagar(ctx context.Context, usuarioID, periodoID int64) (dto.PeriodoCicloRow, error) {
	return s.transicionar(ctx, usuarioID, periodoID, func(ctx context.Context, q *dto.Queries, p dto.PeriodoCicloRow) error {
		if EstadoPeriodo(p.Estado) != EstadoAprobada {
			return estadoInvalido(EstadoAprobada, p.Estado)
		}
		if err := VerificarSegregacion(EstadoPagada, usuarioID, ParticipantesDe(p)); err != nil {
			return err
		}
		n, err := q.MarcarPeriodoPagado(ctx, p.ID, usuarioID)
		if err != nil {
			return err
		}
		if n == 0 {
			return estadoInvalido(EstadoAprobada, p.Estado)
		}
		if _, err := q.ActualizarEstadoDetallePeriodo(ctx, p.ID, "aprobado", "pagado"); err != nil {
			return err
		}
		if s.ganchos.AlPagar != nil {
			act, err := q.ObtenerPeriodoCiclo(ctx, p.ID)
			if err != nil {
				return err
			}
			return s.ganchos.AlPagar.AlPagar(ctx, q, act)
		}
		return nil
	})
}
