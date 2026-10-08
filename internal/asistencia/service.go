// Package asistencia implementa la lógica de negocio de Fase 4 para turnos,
// calendario laboral, marcaje/asistencia e incapacidades, siguiendo las reglas
// del ADR-000: toda escritura pasa por db.WithAuditContext (trazabilidad del
// usuario en los triggers), transacción de 2 pasos para tablas con patrón
// `es_vigente` (aquí: empleado_turno) y decimal.Decimal para toda columna
// DECIMAL — sin float64 en los cálculos.
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

// Queries es el subconjunto de LECTURAS de *dto.Queries que el servicio usa
// fuera de transacción — permite mockear en pruebas. Deliberadamente NO
// incluye ninguna escritura: toda escritura se hace con el *dto.Queries que
// db.WithAuditContext entrega ligado a la transacción, de modo que sea
// imposible escribir sin auditoría a través de s.queries.
type Queries interface {
	ObtenerTurno(ctx context.Context, id int64) (dto.Turno, error)
	ListarTurnos(ctx context.Context, arg dto.ListarTurnosParams) ([]dto.Turno, error)
	ContarTurnos(ctx context.Context, activo *bool) (int64, error)

	ObtenerTurnoVigenteEmpleado(ctx context.Context, empleadoID int64) (dto.ObtenerTurnoVigenteEmpleadoRow, error)

	ObtenerDiaCalendario(ctx context.Context, fecha time.Time) (dto.CalendarioLaboralDia, error)
	ListarCalendarioPorRango(ctx context.Context, desde, hasta time.Time) ([]dto.ListarCalendarioPorRangoRow, error)

	ObtenerTipoAusenciaPorCodigo(ctx context.Context, codigo string) (dto.TiposAusencium, error)

	ListarIncapacidadesEmpleado(ctx context.Context, arg dto.ListarIncapacidadesEmpleadoParams) ([]dto.Incapacidade, error)
	ContarIncapacidadesEmpleado(ctx context.Context, arg dto.ContarIncapacidadesEmpleadoParams) (int64, error)

	ObtenerRegistroAsistencia(ctx context.Context, arg dto.ObtenerRegistroAsistenciaParams) (dto.RegistroAsistenciaDetalle, error)
	ListarAsistenciaEmpleado(ctx context.Context, arg dto.ListarAsistenciaEmpleadoParams) ([]dto.ListarAsistenciaEmpleadoRow, error)
	ContarAsistenciaEmpleado(ctx context.Context, arg dto.ContarAsistenciaEmpleadoParams) (int64, error)
}

// Opciones son los parámetros de negocio inyectados al construir el servicio.
type Opciones struct {
	// ToleranciaTardanzaMin: minutos de gracia en la entrada antes de contar
	// tardanza. SIN valor por defecto: el llamador debe decidirlo (hoy desde
	// config; desde parametros_generales cuando exista, F18). 0 es válido
	// ("sin gracia"); un valor negativo es error de configuración.
	ToleranciaTardanzaMin int
}

// Service agrupa la conexión (para las escrituras auditadas), las lecturas y
// las opciones de negocio.
type Service struct {
	sqlDB   *sql.DB
	queries Queries
	opts    Opciones
}

func NewService(sqlDB *sql.DB, queries Queries, opts Opciones) (*Service, error) {
	if sqlDB == nil {
		return nil, errors.New("asistencia: sqlDB es requerido")
	}
	if queries == nil {
		return nil, errors.New("asistencia: queries es requerido")
	}
	if opts.ToleranciaTardanzaMin < 0 {
		return nil, fmt.Errorf("asistencia: ToleranciaTardanzaMin no puede ser negativa (%d)", opts.ToleranciaTardanzaMin)
	}
	return &Service{sqlDB: sqlDB, queries: queries, opts: opts}, nil
}

// vacioANil devuelve nil (→ NULL en la BD) si s está vacío o solo tiene espacios.
func vacioANil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// --- Lecturas (sin transacción) ----------------------------------------------

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

// --- Turnos --------------------------------------------------------------

type CrearTurnoInput struct {
	Codigo                    string
	Nombre                    string
	HoraEntrada, HoraSalida   time.Time
	HoraEntradaAlmuerzo       *time.Time
	HoraSalidaAlmuerzo        *time.Time
	DiasLaborables            string // CSV "1,2,3,4,5"
	ToleranciaTardanzaMinutos int16
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

	// `turnos` (v11 base) solo tiene codigo, nombre, hora_inicio y hora_fin:
	// HoraEntradaAlmuerzo, HoraSalidaAlmuerzo, DiasLaborables y
	// ToleranciaTardanzaMinutos de CrearTurnoInput NO se persisten hasta que la
	// tabla tenga esas columnas (ver dto/asistencia.go).
	var creado dto.Turno
	err := db.WithAuditContext(ctx, s.sqlDB, creadoPor, func(ctx context.Context, q *dto.Queries) error {
		t, err := q.CrearTurno(ctx, dto.CrearTurnoParams{
			Codigo:      in.Codigo,
			Nombre:      in.Nombre,
			HoraEntrada: in.HoraEntrada,
			HoraSalida:  in.HoraSalida,
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
	ID                      int64
	Nombre                  string
	HoraEntrada, HoraSalida time.Time
}

// ActualizarTurno cambia nombre y/u horario de un turno. Regla de histórico:
// si el horario cambia y el turno tiene asignaciones vigentes (empleado_turno
// con fecha_fin_vigencia NULL), se rechaza con ErrTurnoConAsignacionesVigentes
// — el camino correcto es crear un turno nuevo y reasignar (AsignarTurnoEmpleado
// cierra y abre vigencia). Renombrar siempre se permite. El conteo y el UPDATE
// van en la misma transacción auditada.
func (s *Service) ActualizarTurno(ctx context.Context, in ActualizarTurnoInput, usuarioID int64) (dto.Turno, error) {
	in.Nombre = strings.TrimSpace(in.Nombre)
	if in.Nombre == "" {
		return dto.Turno{}, fmt.Errorf("%w: nombre es requerido", ErrTurnoInvalido)
	}
	if err := validarHorarioTurno(in.HoraEntrada, in.HoraSalida); err != nil {
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

		if !cambiaHorario && actual.Nombre == in.Nombre {
			resultado = actual // nada que escribir
			return nil
		}

		t, err := q.ActualizarTurno(ctx, dto.ActualizarTurnoParams{
			ID:          in.ID,
			Nombre:      in.Nombre,
			HoraEntrada: in.HoraEntrada,
			HoraSalida:  in.HoraSalida,
		})
		if err != nil {
			return fmt.Errorf("actualizando turno: %w", err)
		}
		resultado = t
		return nil
	})
	return resultado, err
}

// AsignarTurnoEmpleado aplica la transacción de 2 pasos obligatoria del ADR-000
// §2 para tablas con patrón de vigencia: cierra la fila vigente de
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
		// Paso 1: cerrar la fila vigente anterior (si la hay). No es error que
		// no exista ninguna — es la primera asignación del empleado.
		if err := q.CerrarTurnoVigenteEmpleado(ctx, dto.CerrarTurnoVigenteEmpleadoParams{
			EmpleadoID:       empleadoID,
			FechaFinVigencia: &fechaEfectiva,
		}); err != nil {
			return fmt.Errorf("cerrando turno vigente: %w", err)
		}

		// Paso 2: abrir la nueva fila vigente.
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

// --- Asistencia ------------------------------------------------------------

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
// del día dentro de WithAuditContext. No usa la transacción de 2 pasos:
// registro_asistencia_detalle es único por (empleado_id, fecha), no tiene
// patrón es_vigente — ver excepción documentada en ADR-000 §2.
//
// El "exceso sobre jornada" se guarda en la columna horas_extra pero es solo
// una hora extra CANDIDATA: queda pendiente de autorización y de tipificación
// (diurna/nocturna/mixta), que son de F22. Aquí no se conecta a tipos de hora
// extra ni se decide su pago.
func (s *Service) RegistrarAsistencia(ctx context.Context, in RegistrarAsistenciaInput) (dto.RegistroAsistenciaDetalle, error) {
	if _, ok := estadosAsistencia[in.Estado]; !ok {
		return dto.RegistroAsistenciaDetalle{}, fmt.Errorf("%w: estado %q no válido", ErrRegistroAsistenciaInvalido, in.Estado)
	}

	turno, err := s.queries.ObtenerTurnoVigenteEmpleado(ctx, in.EmpleadoID)
	var turnoID *int64
	horas := calcularHorasCero()
	switch {
	case err == nil:
		// turno.ID es el id de la fila de empleado_turno; el turno es TurnoID.
		turnoID = &turno.TurnoID
		if in.Estado == "presente" && in.HoraEntradaReal != nil && in.HoraSalidaReal != nil {
			horas = calcularHoras(turno.HoraEntrada, turno.HoraSalida,
				*in.HoraEntradaReal, *in.HoraSalidaReal, s.opts.ToleranciaTardanzaMin)
		}
	case errors.Is(err, sql.ErrNoRows):
		// Sin turno vigente: se guarda igual con horas en cero — el handler debe
		// advertir al usuario (ErrSinTurnoVigente queda disponible por si el
		// negocio decide bloquearlo).
	default:
		return dto.RegistroAsistenciaDetalle{}, fmt.Errorf("obteniendo turno vigente: %w", err)
	}

	var registro dto.RegistroAsistenciaDetalle
	err = db.WithAuditContext(ctx, s.sqlDB, in.CreadoPor, func(ctx context.Context, q *dto.Queries) error {
		r, err := q.UpsertRegistroAsistencia(ctx, dto.UpsertRegistroAsistenciaParams{
			EmpleadoID:      in.EmpleadoID,
			TurnoID:         turnoID,
			Fecha:           in.Fecha,
			HoraEntradaReal: in.HoraEntradaReal,
			HoraSalidaReal:  in.HoraSalidaReal,
			HorasOrdinarias: horas.Ordinarias,
			HorasExtra:      horas.ExcesoJornada, // candidata, pendiente F22
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

// --- Incapacidades ----------------------------------------------------------

type RegistrarIncapacidadInput struct {
	EmpleadoID        int64
	EntidadEmisora    string // CCSS | INS
	NumeroBoleta      string
	FechaInicio       time.Time
	FechaFin          time.Time
	DiagnosticoCodigo string
	Observaciones     string
	CreadoPor         int64
}

func (s *Service) RegistrarIncapacidad(ctx context.Context, in RegistrarIncapacidadInput) (dto.Incapacidade, error) {
	if in.EntidadEmisora != "CCSS" && in.EntidadEmisora != "INS" {
		return dto.Incapacidade{}, fmt.Errorf("%w: entidad_emisora debe ser CCSS o INS", ErrRegistroAsistenciaInvalido)
	}
	// Fechas (sin hora) → días corridos inclusivos, con aritmética entera.
	dias := int(in.FechaFin.Sub(in.FechaInicio)/(24*time.Hour)) + 1
	if dias < 1 {
		return dto.Incapacidade{}, fmt.Errorf("%w: fecha_fin anterior a fecha_inicio", ErrRegistroAsistenciaInvalido)
	}
	if dias > math.MaxInt16 {
		return dto.Incapacidade{}, fmt.Errorf("%w: el rango de la incapacidad es demasiado largo", ErrRegistroAsistenciaInvalido)
	}

	var creada dto.Incapacidade
	err := db.WithAuditContext(ctx, s.sqlDB, in.CreadoPor, func(ctx context.Context, q *dto.Queries) error {
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