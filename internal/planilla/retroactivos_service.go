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

var (
	ErrRetroactivoNoEncontrado  = &ErrorCiclo{"retroactivo_no_encontrado", "el retroactivo no existe en este período", 404}
	ErrRetroactivoDuplicado     = &ErrorCiclo{"retroactivo_duplicado", "el empleado ya tiene un retroactivo en este período; elimínelo y regístrelo con el rango completo", 409}
	ErrRetroactivoRangoInvalido = &ErrorCiclo{"rango_invalido", "desde debe ser <= hasta y hasta anterior al inicio del período", 400}
	ErrRetroactivoMotivo        = &ErrorCiclo{"motivo_invalido", "el motivo es obligatorio y admite como máximo 255 caracteres", 400}
	ErrRetroactivoNoRegular     = &ErrorCiclo{"periodo_no_regular", "los retroactivos solo se pagan en un período regular", 422}
)

// NuevoRetroactivo: datos de registro. Se recalculan los períodos regulares
// aprobados del empleado con fecha_fin en [Desde, Hasta].
type NuevoRetroactivo struct {
	EmpleadoID   int64
	Desde, Hasta time.Time
	Motivo       string
}

// RetroactivosService registra qué empleados reciben en un período en
// borrador la diferencia de sus períodos ya aprobados. El cálculo del período
// hace el recálculo y agrega las líneas.
type RetroactivosService struct{ db *sql.DB }

func NewRetroactivosService(sqlDB *sql.DB) *RetroactivosService {
	return &RetroactivosService{db: sqlDB}
}

func (s *RetroactivosService) Crear(ctx context.Context, usuarioID, periodoID int64, in NuevoRetroactivo) (dto.RetroactivoRow, error) {
	in.Motivo = strings.TrimSpace(in.Motivo)
	if in.Motivo == "" || len([]rune(in.Motivo)) > 255 {
		return dto.RetroactivoRow{}, ErrRetroactivoMotivo
	}
	in.Desde, in.Hasta = soloFecha(in.Desde), soloFecha(in.Hasta)
	if in.Desde.IsZero() || in.Hasta.Before(in.Desde) {
		return dto.RetroactivoRow{}, ErrRetroactivoRangoInvalido
	}
	var out dto.RetroactivoRow
	err := db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		p, err := enBorrador(ctx, q, periodoID)
		if err != nil {
			return err
		}
		if !in.Hasta.Before(soloFecha(p.FechaInicio)) {
			return ErrRetroactivoRangoInvalido
		}
		if ok, err := q.PeriodoEsRegular(ctx, periodoID); err != nil {
			return err
		} else if !ok {
			return ErrRetroactivoNoRegular
		}
		if ok, err := q.EmpleadoActivo(ctx, in.EmpleadoID); err != nil {
			return err
		} else if !ok {
			return ErrEmpleadoNoActivo
		}
		id, err := q.InsertarRetroactivo(ctx, dto.InsertarRetroactivoParams{PeriodoPlanillaID: periodoID, EmpleadoID: in.EmpleadoID,
			Desde: in.Desde, Hasta: in.Hasta, Motivo: in.Motivo, CreadoPor: usuarioID})
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return ErrRetroactivoDuplicado
		}
		if err != nil {
			return err
		}
		out, err = q.ObtenerRetroactivo(ctx, periodoID, id)
		return err
	})
	return out, traducirErrorBD(err)
}

func (s *RetroactivosService) Listar(ctx context.Context, periodoID int64, empleadoID *int64, pag paginacion.Parametros) ([]dto.RetroactivoRow, int64, error) {
	q := dto.New(s.db)
	if _, err := q.ObtenerPeriodoCiclo(ctx, periodoID); errors.Is(err, sql.ErrNoRows) {
		return nil, 0, ErrPeriodoNoEncontrado
	} else if err != nil {
		return nil, 0, err
	}
	total, err := q.ContarRetroactivos(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, 0, err
	}
	filas, err := q.ListarRetroactivos(ctx, periodoID, empleadoID, pag.Limit32(), pag.Offset32())
	return filas, total, err
}

func (s *RetroactivosService) Eliminar(ctx context.Context, usuarioID, periodoID, retroactivoID int64) error {
	err := db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if _, err := enBorrador(ctx, q, periodoID); err != nil {
			return err
		}
		n, err := q.EliminarRetroactivo(ctx, periodoID, retroactivoID)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrRetroactivoNoEncontrado
		}
		return nil
	})
	return traducirErrorBD(err)
}
