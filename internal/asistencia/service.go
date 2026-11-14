// Package asistencia gestiona turnos, calendario laboral, marcas de asistencia,
// incapacidades, horas extra y vacaciones. Toda escritura pasa por
// db.WithAuditContext.
package asistencia

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

var (
	ErrTurnoNoEncontrado            = errors.New("turno no encontrado")
	ErrSinTurnoVigente              = errors.New("el empleado no tiene turno vigente")
	ErrRegistroAsistenciaInvalido   = errors.New("registro de asistencia inválido")
	ErrTurnoInvalido                = errors.New("turno inválido")
	ErrTurnoConAsignacionesVigentes = errors.New("el turno tiene asignaciones vigentes: no se puede cambiar su horario; cree un turno nuevo y reasigne a los empleados")
	ErrAnioFueraDeRango             = errors.New("año fuera de rango razonable")
)

// estadosAsistencia replica el ENUM de registro_asistencia_detalle.estado.
var estadosAsistencia = map[string]struct{}{
	"presente": {}, "ausente_justificada": {}, "ausente_injustificada": {},
	"incapacidad": {}, "vacaciones": {}, "feriado": {}, "dia_libre": {},
}

// Queries es el subconjunto de lecturas de *dto.Queries que el servicio usa
// fuera de transacción. No incluye escrituras: toda escritura usa el
// *dto.Queries ligado a la transacción auditada.
type Queries interface {
	ObtenerTurno(ctx context.Context, id int64) (dto.Turno, error)
	ListarTurnos(ctx context.Context, arg dto.ListarTurnosParams) ([]dto.Turno, error)
	ContarTurnos(ctx context.Context, activo *bool) (int64, error)

	ObtenerTurnoVigenteEmpleado(ctx context.Context, empleadoID int64) (dto.ObtenerTurnoVigenteEmpleadoRow, error)

	ListarFeriadosCatalogo(ctx context.Context) ([]dto.FeriadoCatalogo, error)
	ExisteEntidadEmisoraActiva(ctx context.Context, codigo string) (bool, error)
	ObtenerDiaCalendario(ctx context.Context, fecha time.Time) (dto.CalendarioLaboralDia, error)
	ListarCalendarioPorRango(ctx context.Context, desde, hasta time.Time) ([]dto.ListarCalendarioPorRangoRow, error)

	ObtenerTipoAusenciaPorCodigo(ctx context.Context, codigo string) (dto.TiposAusencium, error)

	ListarIncapacidadesEmpleado(ctx context.Context, arg dto.ListarIncapacidadesEmpleadoParams) ([]dto.Incapacidade, error)
	ContarIncapacidadesEmpleado(ctx context.Context, arg dto.ContarIncapacidadesEmpleadoParams) (int64, error)

	ObtenerRegistroAsistencia(ctx context.Context, arg dto.ObtenerRegistroAsistenciaParams) (dto.RegistroAsistenciaDetalle, error)
	ListarAsistenciaEmpleado(ctx context.Context, arg dto.ListarAsistenciaEmpleadoParams) ([]dto.ListarAsistenciaEmpleadoRow, error)
	ContarAsistenciaEmpleado(ctx context.Context, arg dto.ContarAsistenciaEmpleadoParams) (int64, error)
}

// LectorParametros lee parametros_generales vigentes a una fecha.
type LectorParametros interface {
	Texto(ctx context.Context, clave string, fecha time.Time) (string, error)
}

type Service struct {
	sqlDB      *sql.DB
	queries    Queries
	parametros LectorParametros
}

func NewService(sqlDB *sql.DB, queries Queries, parametros LectorParametros) (*Service, error) {
	if sqlDB == nil {
		return nil, errors.New("asistencia: sqlDB es requerido")
	}
	if queries == nil {
		return nil, errors.New("asistencia: queries es requerido")
	}
	if parametros == nil {
		return nil, errors.New("asistencia: lector de parámetros es requerido")
	}
	return &Service{sqlDB: sqlDB, queries: queries, parametros: parametros}, nil
}

// vacioANil devuelve nil (→ NULL en la BD) si s está vacío o solo tiene espacios.
func vacioANil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) ListarTurnos(ctx context.Context, arg dto.ListarTurnosParams) ([]dto.Turno, error) {
	return s.queries.ListarTurnos(ctx, arg)
}

func (s *Service) ContarTurnos(ctx context.Context, activo *bool) (int64, error) {
	return s.queries.ContarTurnos(ctx, activo)
}

func (s *Service) ListarCalendario(ctx context.Context, desde, hasta time.Time) ([]dto.ListarCalendarioPorRangoRow, error) {
	return s.queries.ListarCalendarioPorRango(ctx, desde, hasta)
}

func (s *Service) ListarAsistenciaEmpleado(ctx context.Context, arg dto.ListarAsistenciaEmpleadoParams) ([]dto.ListarAsistenciaEmpleadoRow, error) {
	return s.queries.ListarAsistenciaEmpleado(ctx, arg)
}

func (s *Service) ContarAsistenciaEmpleado(ctx context.Context, arg dto.ContarAsistenciaEmpleadoParams) (int64, error) {
	return s.queries.ContarAsistenciaEmpleado(ctx, arg)
}

func (s *Service) ListarIncapacidadesEmpleado(ctx context.Context, arg dto.ListarIncapacidadesEmpleadoParams) ([]dto.Incapacidade, error) {
	return s.queries.ListarIncapacidadesEmpleado(ctx, arg)
}

type CrearTurnoInput struct {
	Codigo                    string
	Nombre                    string
	HoraEntrada, HoraSalida   time.Time
	ToleranciaTardanzaMinutos int
}

func validarTolerancia(minutos int) error {
	if minutos < 0 || minutos > 240 {
		return fmt.Errorf("%w: tolerancia_tardanza_minutos debe estar entre 0 y 240", ErrTurnoInvalido)
	}
	return nil
}

// validarHorarioTurno exige entrada != salida (una jornada de 0 h, o de 24 h
// exactas, no es representable). salida < entrada es válido: cruza medianoche.
func validarHorarioTurno(entrada, salida time.Time) error {
	if minutosDelDia(entrada) == minutosDelDia(salida) {
		return fmt.Errorf("%w: hora_entrada y hora_salida no pueden ser iguales", ErrTurnoInvalido)
	}
	return nil
}

func (s *Service) CrearTurno(ctx context.Context, in CrearTurnoInput, creadoPor int64) (dto.Turno, error) {
	in.Codigo, in.Nombre = strings.TrimSpace(in.Codigo), strings.TrimSpace(in.Nombre)
	if in.Codigo == "" || in.Nombre == "" {
		return dto.Turno{}, fmt.Errorf("%w: codigo y nombre son requeridos", ErrTurnoInvalido)
	}
	if err := validarHorarioTurno(in.HoraEntrada, in.HoraSalida); err != nil {
		return dto.Turno{}, err
	}

	if err := validarTolerancia(in.ToleranciaTardanzaMinutos); err != nil {
		return dto.Turno{}, err
	}
	var creado dto.Turno
	err := db.WithAuditContext(ctx, s.sqlDB, creadoPor, func(ctx context.Context, q *dto.Queries) error {
		t, err := q.CrearTurno(ctx, dto.CrearTurnoParams{
			Codigo:                    in.Codigo,
			Nombre:                    in.Nombre,
			HoraEntrada:               in.HoraEntrada,
			HoraSalida:                in.HoraSalida,
			ToleranciaTardanzaMinutos: in.ToleranciaTardanzaMinutos,
		})
		if err != nil {
			return err
		}
		creado = t
		return nil
	})
	return creado, err
}

type ActualizarTurnoInput struct {
	ID                        int64
	Nombre                    string
	HoraEntrada, HoraSalida   time.Time
	ToleranciaTardanzaMinutos int
}

// ActualizarTurno cambia nombre y/u horario de un turno. Regla de histórico:
// si el horario cambia y el turno tiene asignaciones vigentes (empleado_turno
// con fecha_fin_vigencia NULL), se rechaza con ErrTurnoConAsignacionesVigentes;
// se debe crear un turno nuevo y reasignar. Renombrar siempre se permite. El conteo y el UPDATE
// van en la misma transacción auditada.
func (s *Service) ActualizarTurno(ctx context.Context, in ActualizarTurnoInput, usuarioID int64) (dto.Turno, error) {
	in.Nombre = strings.TrimSpace(in.Nombre)
	if in.Nombre == "" {
		return dto.Turno{}, fmt.Errorf("%w: nombre es requerido", ErrTurnoInvalido)
	}
	if err := validarHorarioTurno(in.HoraEntrada, in.HoraSalida); err != nil {
		return dto.Turno{}, err
	}
	if err := validarTolerancia(in.ToleranciaTardanzaMinutos); err != nil {
		return dto.Turno{}, err
	}

	var resultado dto.Turno
	err := db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		actual, err := q.ObtenerTurno(ctx, in.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTurnoNoEncontrado
		}
		if err != nil {
			return fmt.Errorf("obteniendo turno: %w", err)
		}

		cambiaHorario := minutosDelDia(actual.HoraEntrada) != minutosDelDia(in.HoraEntrada) ||
			minutosDelDia(actual.HoraSalida) != minutosDelDia(in.HoraSalida)

		if cambiaHorario {
			vigentes, err := q.ContarAsignacionesVigentesTurno(ctx, in.ID)
			if err != nil {
				return fmt.Errorf("contando asignaciones vigentes: %w", err)
			}
			if vigentes > 0 {
				return ErrTurnoConAsignacionesVigentes
			}
		}

		if !cambiaHorario && actual.Nombre == in.Nombre && actual.ToleranciaTardanzaMinutos == in.ToleranciaTardanzaMinutos {
			resultado = actual
			return nil
		}

		t, err := q.ActualizarTurno(ctx, dto.ActualizarTurnoParams{
			ID:                        in.ID,
			Nombre:                    in.Nombre,
			HoraEntrada:               in.HoraEntrada,
			HoraSalida:                in.HoraSalida,
			ToleranciaTardanzaMinutos: in.ToleranciaTardanzaMinutos,
		})
		if err != nil {
			return fmt.Errorf("actualizando turno: %w", err)
		}
		resultado = t
		return nil
	})
	return resultado, err
}

// AsignarTurnoEmpleado aplica la transacción de 2 pasos de las tablas con
// vigencia: cierra la fila vigente de
// empleado_turno (si existe) y abre la nueva, todo dentro de una sola Tx.
func (s *Service) AsignarTurnoEmpleado(ctx context.Context, empleadoID, turnoID int64, fechaEfectiva time.Time, creadoPor int64) (dto.EmpleadoTurno, error) {
	if _, err := s.queries.ObtenerTurno(ctx, turnoID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.EmpleadoTurno{}, ErrTurnoNoEncontrado
		}
		return dto.EmpleadoTurno{}, fmt.Errorf("obteniendo turno: %w", err)
	}

	var nuevo dto.EmpleadoTurno
	err := db.WithAuditContext(ctx, s.sqlDB, creadoPor, func(ctx context.Context, q *dto.Queries) error {
		if err := q.CerrarTurnoVigenteEmpleado(ctx, dto.CerrarTurnoVigenteEmpleadoParams{
			EmpleadoID:       empleadoID,
			FechaFinVigencia: &fechaEfectiva,
		}); err != nil {
			return fmt.Errorf("cerrando turno vigente: %w", err)
		}

		creado, err := q.AbrirTurnoEmpleado(ctx, dto.AbrirTurnoEmpleadoParams{
			EmpleadoID:          empleadoID,
			TurnoID:             turnoID,
			FechaInicioVigencia: fechaEfectiva,
			CreadoPor:           creadoPor,
		})
		if err != nil {
			return fmt.Errorf("abriendo nuevo turno vigente: %w", err)
		}
		nuevo = creado
		return nil
	})
	return nuevo, err
}

type RegistrarAsistenciaInput struct {
	EmpleadoID      int64
	Fecha           time.Time
	HoraEntradaReal *time.Time
	HoraSalidaReal  *time.Time
	Estado          string // presente | ausente_justificada | ausente_injustificada | incapacidad | vacaciones | feriado | dia_libre
	TipoAusenciaID  *int64
	IncapacidadID   *int64
	Observaciones   string
	CreadoPor       int64
}

// RegistrarAsistencia calcula horas ordinarias, exceso sobre jornada y
// tardanza contra el turno vigente del empleado y hace upsert del registro
// del día dentro de WithAuditContext. registro_asistencia_detalle es único
// por (empleado_id, fecha) y no tiene vigencias.
func (s *Service) RegistrarAsistencia(ctx context.Context, in RegistrarAsistenciaInput) (dto.RegistroAsistenciaDetalle, error) {
	if _, ok := estadosAsistencia[in.Estado]; !ok {
		return dto.RegistroAsistenciaDetalle{}, fmt.Errorf("%w: estado %q no válido", ErrRegistroAsistenciaInvalido, in.Estado)
	}

	turno, err := s.queries.ObtenerTurnoVigenteEmpleado(ctx, in.EmpleadoID)
	var turnoID *int64
	horas := calcularHorasCero()
	switch {
	case err == nil:
		turnoID = &turno.TurnoID
		if in.Estado == "presente" && in.HoraEntradaReal != nil && in.HoraSalidaReal != nil {
			horas = calcularHoras(turno.HoraEntrada, turno.HoraSalida,
				*in.HoraEntradaReal, *in.HoraSalidaReal, turno.ToleranciaTardanzaMinutos)
		}
	case errors.Is(err, sql.ErrNoRows):
	default:
		return dto.RegistroAsistenciaDetalle{}, fmt.Errorf("obteniendo turno vigente: %w", err)
	}

	var registro dto.RegistroAsistenciaDetalle
	err = db.WithAuditContext(ctx, s.sqlDB, in.CreadoPor, func(ctx context.Context, q *dto.Queries) error {
		tipificadas, err := s.tipificar(ctx, q, in, turno, horas)
		if err != nil {
			return err
		}
		if err := q.ReemplazarHorasExtraTipificadas(ctx, in.EmpleadoID, in.Fecha, tipificadas); err != nil {
			return err
		}
		r, err := q.UpsertRegistroAsistencia(ctx, dto.UpsertRegistroAsistenciaParams{
			EmpleadoID:      in.EmpleadoID,
			TurnoID:         turnoID,
			Fecha:           in.Fecha,
			HoraEntradaReal: in.HoraEntradaReal,
			HoraSalidaReal:  in.HoraSalidaReal,
			HorasOrdinarias: horas.Ordinarias,
			HorasExtra:      horas.ExcesoJornada,
			HorasTardanza:   horas.Tardanza,
			MinutosTardanza: int16(horas.MinutosTardanza),
			Estado:          in.Estado,
			TipoAusenciaID:  in.TipoAusenciaID,
			IncapacidadID:   in.IncapacidadID,
			Observaciones:   vacioANil(in.Observaciones),
			CreadoPor:       in.CreadoPor,
		})
		if err != nil {
			return err
		}
		registro = r
		return nil
	})
	return registro, err
}

type RegistrarIncapacidadInput struct {
	EmpleadoID        int64
	EntidadEmisora    string
	NumeroBoleta      string
	FechaInicio       time.Time
	FechaFin          time.Time
	DiagnosticoCodigo string
	Observaciones     string
	CreadoPor         int64
}

func (s *Service) RegistrarIncapacidad(ctx context.Context, in RegistrarIncapacidadInput) (dto.Incapacidade, error) {
	in.EntidadEmisora = strings.TrimSpace(in.EntidadEmisora)
	activa, err := s.queries.ExisteEntidadEmisoraActiva(ctx, in.EntidadEmisora)
	if err != nil {
		return dto.Incapacidade{}, fmt.Errorf("verificando entidad emisora: %w", err)
	}
	if !activa {
		return dto.Incapacidade{}, fmt.Errorf("%w: entidad_emisora %q no existe o está inactiva en el catálogo", ErrRegistroAsistenciaInvalido, in.EntidadEmisora)
	}
	dias := int(in.FechaFin.Sub(in.FechaInicio)/(24*time.Hour)) + 1
	if dias < 1 {
		return dto.Incapacidade{}, fmt.Errorf("%w: fecha_fin anterior a fecha_inicio", ErrRegistroAsistenciaInvalido)
	}
	if dias > math.MaxInt16 {
		return dto.Incapacidade{}, fmt.Errorf("%w: el rango de la incapacidad es demasiado largo", ErrRegistroAsistenciaInvalido)
	}

	var creada dto.Incapacidade
	err = db.WithAuditContext(ctx, s.sqlDB, in.CreadoPor, func(ctx context.Context, q *dto.Queries) error {
		i, err := q.CrearIncapacidad(ctx, dto.CrearIncapacidadParams{
			EmpleadoID:        in.EmpleadoID,
			EntidadEmisora:    in.EntidadEmisora,
			NumeroBoleta:      in.NumeroBoleta,
			FechaInicio:       in.FechaInicio,
			FechaFin:          in.FechaFin,
			Dias:              int16(dias),
			DiagnosticoCodigo: vacioANil(in.DiagnosticoCodigo),
			Observaciones:     vacioANil(in.Observaciones),
			CreadoPor:         in.CreadoPor,
		})
		if err != nil {
			return err
		}
		creada = i
		return nil
	})
	return creada, err
}

// claseJornadaMixta: jornadas.clase cuyo tramo nocturno se tipifica como "mixta".
const claseJornadaMixta = "mixta"

var (
	ErrCalendarioNoGenerado = errors.New("el calendario laboral no está generado para esa fecha (no se sabe si es feriado o descanso)")
	ErrSinJornadaContrato   = errors.New("el empleado no tiene contrato vigente con una jornada del catálogo")
)

// tipificar reparte el exceso del día entre clases de hora extra. Sin exceso
// no consulta nada y devuelve nil.
func (s *Service) tipificar(ctx context.Context, q *dto.Queries, in RegistrarAsistenciaInput,
	turno dto.ObtenerTurnoVigenteEmpleadoRow, horas ResultadoHoras) ([]dto.HoraExtraTipificadaInsert, error) {
	if !horas.ExcesoJornada.IsPositive() || in.HoraEntradaReal == nil || in.HoraSalidaReal == nil {
		return nil, nil
	}
	dia, err := q.ObtenerDiaCalendario(ctx, in.Fecha)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCalendarioNoGenerado
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo calendario laboral: %w", err)
	}
	franja := FranjaNocturna{}
	if f, err := q.ObtenerFranjaNocturnaEnFecha(ctx, in.Fecha); err == nil {
		franja = FranjaNocturna{Vigente: true, Inicio: f.HoraInicio, Fin: f.HoraFin}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("leyendo franja nocturna: %w", err)
	}
	jornada, err := q.ObtenerClaseJornadaContratoEnFecha(ctx, in.EmpleadoID, in.Fecha)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSinJornadaContrato
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo jornada del contrato: %w", err)
	}
	tip, err := TipificarHorasExtra(*in.HoraEntradaReal, *in.HoraSalidaReal, turno.HoraEntrada, turno.HoraSalida,
		franja, dia.EsFeriado, !dia.EsHabil && !dia.EsFeriado, jornada == claseJornadaMixta)
	if err != nil {
		return nil, err
	}
	out := make([]dto.HoraExtraTipificadaInsert, 0, len(tip))
	for _, t := range tip {
		out = append(out, dto.HoraExtraTipificadaInsert{Clase: t.Clase, Horas: t.Horas})
	}
	return out, nil
}
