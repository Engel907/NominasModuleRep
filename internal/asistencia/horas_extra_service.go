package asistencia

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/paginacion"
)

// ErrorHorasExtra: error de dominio con código estable, mensaje seguro y HTTP.
type ErrorHorasExtra struct {
	Codigo  string
	Mensaje string
	HTTP    int
}

func (e *ErrorHorasExtra) Error() string { return e.Mensaje }

func (e *ErrorHorasExtra) Is(target error) bool {
	o, ok := target.(*ErrorHorasExtra)
	return ok && o.Codigo == e.Codigo
}

var (
	ErrAutorizacionNoEncontrada = &ErrorHorasExtra{"autorizacion_no_encontrada", "la autorización de horas extra no existe", 404}
	ErrAutorizacionYaResuelta   = &ErrorHorasExtra{"autorizacion_ya_resuelta", "la autorización ya fue resuelta", 409}
	ErrAutorizacionDuplicada    = &ErrorHorasExtra{"autorizacion_duplicada", "ya existe una autorización de horas extra para ese empleado y día", 409}
	ErrSegregacionHorasExtra    = &ErrorHorasExtra{"segregacion_funciones", "quien solicitó las horas extra no puede autorizarlas ni rechazarlas", 403}
	ErrHorasAutorizadasInvalida = &ErrorHorasExtra{"horas_invalidas", "las horas autorizadas deben ser > 0, <= 24 y tener como máximo 2 decimales", 400}
	ErrMotivoRequerido          = &ErrorHorasExtra{"motivo_requerido", "el motivo es obligatorio (máximo 255 caracteres)", 400}
	ErrEmpleadoInactivoHE       = &ErrorHorasExtra{"empleado_no_activo", "el empleado no existe o no está activo", 422}
	ErrTipoHoraExtraInvalido    = &ErrorHorasExtra{"tipo_hora_extra_invalido", "el tipo de hora extra no existe o no está activo", 422}
	ErrRangoFechasInvalido      = &ErrorHorasExtra{"rango_invalido", "se requieren desde y hasta (YYYY-MM-DD) con hasta >= desde y un rango de como máximo 366 días", 400}
)

const (
	EstadoHEPendiente  = "pendiente"
	EstadoHEAutorizada = "autorizada"
	EstadoHERechazada  = "rechazada"
)

// SolicitudHorasExtra: datos para pedir la autorización de un día.
type SolicitudHorasExtra struct {
	EmpleadoID       int64
	Fecha            time.Time
	HorasAutorizadas decimal.Decimal
	TipoHoraExtraID  *int64
	Motivo           string
}

type HorasExtraService struct{ db *sql.DB }

func NewHorasExtraService(sqlDB *sql.DB) *HorasExtraService { return &HorasExtraService{db: sqlDB} }

func motivoValido(m string) (string, bool) {
	m = strings.TrimSpace(m)
	return m, m != "" && len([]rune(m)) <= 255
}

func (s *HorasExtraService) Solicitar(ctx context.Context, usuarioID int64, in SolicitudHorasExtra) (dto.HoraExtraAutorizacionRow, error) {
	h := in.HorasAutorizadas
	if !h.IsPositive() || h.GreaterThan(decimal.NewFromInt(24)) || !h.Equal(h.Round(2)) {
		return dto.HoraExtraAutorizacionRow{}, ErrHorasAutorizadasInvalida
	}
	motivo, ok := motivoValido(in.Motivo)
	if !ok {
		return dto.HoraExtraAutorizacionRow{}, ErrMotivoRequerido
	}
	var out dto.HoraExtraAutorizacionRow
	err := db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if activo, err := q.EmpleadoActivo(ctx, in.EmpleadoID); err != nil {
			return err
		} else if !activo {
			return ErrEmpleadoInactivoHE
		}
		if in.TipoHoraExtraID != nil {
			if ok, err := q.ExisteTipoHoraExtraActivo(ctx, *in.TipoHoraExtraID); err != nil {
				return err
			} else if !ok {
				return ErrTipoHoraExtraInvalido
			}
		}
		id, err := q.InsertarHoraExtraAutorizacion(ctx, dto.InsertarHoraExtraAutorizacionParams{
			EmpleadoID: in.EmpleadoID, Fecha: in.Fecha, HorasAutorizadas: h,
			TipoHoraExtraID: in.TipoHoraExtraID, Motivo: motivo, SolicitadoPor: usuarioID,
		})
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return ErrAutorizacionDuplicada
		}
		if err != nil {
			return err
		}
		out, err = q.ObtenerHoraExtraAutorizacion(ctx, id)
		return err
	})
	return out, err
}

// Autorizar y Rechazar: pendiente -> autorizada | rechazada.
func (s *HorasExtraService) Autorizar(ctx context.Context, usuarioID, id int64) (dto.HoraExtraAutorizacionRow, error) {
	return s.resolver(ctx, usuarioID, id, EstadoHEAutorizada, nil)
}

func (s *HorasExtraService) Rechazar(ctx context.Context, usuarioID, id int64, motivo string) (dto.HoraExtraAutorizacionRow, error) {
	m, ok := motivoValido(motivo)
	if !ok {
		return dto.HoraExtraAutorizacionRow{}, ErrMotivoRequerido
	}
	return s.resolver(ctx, usuarioID, id, EstadoHERechazada, &m)
}

func (s *HorasExtraService) resolver(ctx context.Context, usuarioID, id int64, estado string, motivo *string) (dto.HoraExtraAutorizacionRow, error) {
	var out dto.HoraExtraAutorizacionRow
	err := db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		a, err := q.ObtenerHoraExtraAutorizacionParaActualizar(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAutorizacionNoEncontrada
		}
		if err != nil {
			return err
		}
		if a.Estado != EstadoHEPendiente {
			return ErrAutorizacionYaResuelta
		}
		if a.SolicitadoPor == usuarioID {
			return ErrSegregacionHorasExtra
		}
		n, err := q.ResolverHoraExtraAutorizacion(ctx, id, estado, usuarioID, motivo)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrAutorizacionYaResuelta
		}
		out, err = q.ObtenerHoraExtraAutorizacion(ctx, id)
		return err
	})
	return out, err
}

func (s *HorasExtraService) Listar(ctx context.Context, f dto.FiltroHorasExtraAutorizaciones, pag paginacion.Parametros) ([]dto.HoraExtraAutorizacionRow, int64, error) {
	q := dto.New(s.db)
	total, err := q.ContarHorasExtraAutorizaciones(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	filas, err := q.ListarHorasExtraAutorizaciones(ctx, f, pag.Limit32(), pag.Offset32())
	return filas, total, err
}

// Candidatas: días con horas extra registradas y sin autorización en el rango.
func (s *HorasExtraService) Candidatas(ctx context.Context, empleadoID *int64, desde, hasta time.Time) ([]dto.HoraExtraCandidataRow, error) {
	if desde.IsZero() || hasta.IsZero() || hasta.Before(desde) || hasta.Sub(desde) > 366*24*time.Hour {
		return nil, ErrRangoFechasInvalido
	}
	return dto.New(s.db).ListarHorasExtraCandidatas(ctx, empleadoID, desde, hasta)
}
