package asistencia

// Flujo de negocio de vacaciones (Fase 4). Solo orquesta las funciones de
// internal/db/dto (vacaciones.go y vacaciones_extra.go); no contiene SQL.
//
// Modelo de datos (ver dto/vacaciones.go):
//   - vacaciones_acumulado: cortes por (empleado_id, fecha_corte); el saldo
//     actual es el corte más reciente. Aquí se mantiene saldo = dias_disponibles.
//   - vacaciones_movimientos: ledger append-only. dias > 0 acumulación,
//     dias < 0 disfrute.
//   - vacaciones_solicitudes: pendiente -> aprobada | rechazada (| cancelada).
//
// Flujo: AcumularVacaciones (suma días según la escala por antigüedad) ->
// SolicitarVacaciones (pendiente) -> ResolverSolicitud (aprobada/rechazada) ->
// RegistrarDisfrute (mueve días de disponibles a disfrutados).
//
// Toda escritura corre en una Tx vía withAuditCtx y arranca bloqueando la fila
// del empleado (BloquearEmpleado), así dos operaciones concurrentes sobre el
// mismo empleado se serializan. Orden de locks constante: solicitud -> empleado.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
)

var (
	ErrVacEmpleadoNoEncontrado     = errors.New("empleado no encontrado")
	ErrVacEmpleadoInactivo         = errors.New("el empleado está inactivo")
	ErrVacSolicitudNoEncontrada    = errors.New("solicitud de vacaciones no encontrada")
	ErrVacSolicitudNoPendiente     = errors.New("la solicitud de vacaciones ya no está pendiente")
	ErrVacSolicitudNoAprobada      = errors.New("la solicitud de vacaciones no está aprobada")
	ErrVacRangoFechasInvalido      = errors.New("rango de fechas de vacaciones inválido")
	ErrVacSolicitudTraslapada      = errors.New("las fechas se traslapan con otra solicitud pendiente o aprobada")
	ErrVacSaldoInsuficiente        = errors.New("saldo de vacaciones insuficiente")
	ErrVacCalendarioIncompleto     = errors.New("el calendario laboral no está generado para todo el rango solicitado")
	ErrVacSinDiasHabiles           = errors.New("el rango solicitado no contiene días hábiles")
	ErrVacEscalaNoConfigurada      = errors.New("no hay escala de vacaciones vigente para la antigüedad del empleado")
	ErrVacDisfruteYaRegistrado     = errors.New("el disfrute de esta solicitud ya fue registrado")
	ErrVacAcumulacionYaRegistrada  = errors.New("la acumulación de este período ya fue registrada")
	ErrVacMotivoRechazoRequerido   = errors.New("el motivo de rechazo es obligatorio")
	ErrVacMotivoRechazoMuyLargo    = errors.New("el motivo de rechazo excede 255 caracteres")
	ErrVacPeriodoReferenciaInvalid = errors.New("periodo_referencia es obligatorio y no puede exceder 20 caracteres")
)

const (
	vacEstadoAprobada  = "aprobada"
	vacEstadoRechazada = "rechazada"

	vacMovAcumulacion = "acumulacion"
	vacMovDisfrute    = "disfrute"
)

// VacacionesQueries es el subconjunto de *dto.Queries que el servicio usa
// FUERA de una transacción (lecturas). Las escrituras y las lecturas con lock
// usan el *dto.Queries que entrega withAuditCtx.
type VacacionesQueries interface {
	GetEmpleadoByID(ctx context.Context, id uint64) (dto.Empleado, error)
	ObtenerEscalaVacacionesVigentePorAntiguedad(ctx context.Context, aniosServicio decimal.Decimal) (dto.ParametrosVacacionesEscala, error)
	ObtenerSaldoVacacionesEmpleado(ctx context.Context, empleadoID int64) (dto.VacacionesAcumulado, error)
	ListarCalendarioPorRango(ctx context.Context, desde, hasta time.Time) ([]dto.ListarCalendarioPorRangoRow, error)
}

type VacacionesService struct {
	queries      VacacionesQueries
	withAuditCtx func(ctx context.Context, userID int64, fn func(*dto.Queries) error) error
}

func NewVacacionesService(queries VacacionesQueries, withAuditCtx func(context.Context, int64, func(*dto.Queries) error) error) *VacacionesService {
	return &VacacionesService{queries: queries, withAuditCtx: withAuditCtx}
}

// --- Consulta de saldo -------------------------------------------------------

// SaldoVacaciones es la vista de saldo de un empleado a una fecha.
type SaldoVacaciones struct {
	EmpleadoID      int64
	FechaCorte      *time.Time // nil si el empleado aún no tiene ningún corte
	DiasAcumulados  decimal.Decimal
	DiasDisfrutados decimal.Decimal
	DiasDisponibles decimal.Decimal

	AniosServicio decimal.Decimal
	// DiasPorPeriodoEscala es lo que la escala vigente otorga por período para
	// la antigüedad actual; cero si no hay escala aplicable.
	DiasPorPeriodoEscala decimal.Decimal
}

// ConsultarSaldo devuelve el saldo actual (corte más reciente) y la
// antigüedad/escala aplicable a fechaReferencia. Un empleado sin cortes tiene
// saldo cero, no es error.
func (s *VacacionesService) ConsultarSaldo(ctx context.Context, empleadoID int64, fechaReferencia time.Time) (SaldoVacaciones, error) {
	emp, err := s.obtenerEmpleado(ctx, empleadoID)
	if err != nil {
		return SaldoVacaciones{}, err
	}

	out := SaldoVacaciones{EmpleadoID: empleadoID}
	saldo, err := s.queries.ObtenerSaldoVacacionesEmpleado(ctx, empleadoID)
	switch {
	case err == nil:
		corte := saldo.FechaCorte
		out.FechaCorte = &corte
		out.DiasAcumulados = saldo.DiasAcumulados
		out.DiasDisfrutados = saldo.DiasDisfrutados
		out.DiasDisponibles = saldo.Saldo
	case errors.Is(err, sql.ErrNoRows):
		// sin cortes: todo en cero
	default:
		return SaldoVacaciones{}, fmt.Errorf("consultando saldo de vacaciones: %w", err)
	}

	out.AniosServicio = vacAniosServicio(emp.FechaIngreso, fechaReferencia)
	if escala, err := s.queries.ObtenerEscalaVacacionesVigentePorAntiguedad(ctx, out.AniosServicio); err == nil {
		out.DiasPorPeriodoEscala = escala.DiasVacaciones
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SaldoVacaciones{}, fmt.Errorf("consultando escala de vacaciones: %w", err)
	}
	return out, nil
}

// --- Acumulación -------------------------------------------------------------

type AcumularVacacionesInput struct {
	EmpleadoID        int64
	PeriodoReferencia string    // p. ej. "2026"; identifica el período para idempotencia
	FechaEfectiva     time.Time // fecha a la que se mide la antigüedad
	CreadoPor         int64
}

// AcumularVacaciones suma al saldo los días que la escala vigente otorga por
// período según la antigüedad del empleado a FechaEfectiva. El número de días
// sale SIEMPRE de parametros_vacaciones_escala. Registrar dos veces el mismo
// (empleado, período) devuelve ErrVacAcumulacionYaRegistrada.
func (s *VacacionesService) AcumularVacaciones(ctx context.Context, in AcumularVacacionesInput) (dto.VacacionesAcumulado, error) {
	periodo := strings.TrimSpace(in.PeriodoReferencia)
	if periodo == "" || utf8.RuneCountInString(periodo) > 20 {
		return dto.VacacionesAcumulado{}, ErrVacPeriodoReferenciaInvalid
	}
	emp, err := s.obtenerEmpleado(ctx, in.EmpleadoID)
	if err != nil {
		return dto.VacacionesAcumulado{}, err
	}

	fecha := vacSoloFecha(in.FechaEfectiva)
	anios := vacAniosServicio(emp.FechaIngreso, fecha)
	escala, err := s.queries.ObtenerEscalaVacacionesVigentePorAntiguedad(ctx, anios)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.VacacionesAcumulado{}, ErrVacEscalaNoConfigurada
	}
	if err != nil {
		return dto.VacacionesAcumulado{}, fmt.Errorf("consultando escala de vacaciones: %w", err)
	}
	dias := escala.DiasVacaciones

	var resultado dto.VacacionesAcumulado
	err = s.withAuditCtx(ctx, in.CreadoPor, func(q *dto.Queries) error {
		if err := vacBloquearEmpleado(ctx, q, in.EmpleadoID); err != nil {
			return err
		}
		existe, err := q.ExisteMovimientoAcumulacionVacaciones(ctx, in.EmpleadoID, periodo)
		if err != nil {
			return fmt.Errorf("verificando acumulación del período: %w", err)
		}
		if existe {
			return ErrVacAcumulacionYaRegistrada
		}

		previo, err := vacLeerSaldoParaActualizar(ctx, q, in.EmpleadoID)
		if err != nil {
			return err
		}
		if _, err := q.InsertarMovimientoVacaciones(ctx, in.EmpleadoID, vacMovAcumulacion, dias, fecha,
			sql.NullString{String: periodo, Valid: true}, sql.NullInt64{}, sql.NullString{}, in.CreadoPor); err != nil {
			return fmt.Errorf("registrando movimiento de acumulación: %w", err)
		}
		if err := vacGuardarSaldo(ctx, q, in.EmpleadoID, previo, fecha, dias, decimal.Zero, in.CreadoPor); err != nil {
			return err
		}
		resultado, err = q.ObtenerSaldoVacacionesEmpleado(ctx, in.EmpleadoID)
		return err
	})
	return resultado, err
}

// --- Solicitud ---------------------------------------------------------------

type SolicitarVacacionesInput struct {
	EmpleadoID  int64
	FechaInicio time.Time
	FechaFin    time.Time
	CreadoPor   int64
}

// SolicitarVacaciones crea una solicitud 'pendiente'. Los días solicitados se
// calculan contra el calendario laboral (solo días hábiles). Valida traslape y
// que el saldo disponible alcance.
func (s *VacacionesService) SolicitarVacaciones(ctx context.Context, in SolicitarVacacionesInput) (dto.VacacionesSolicitud, error) {
	inicio, fin := vacSoloFecha(in.FechaInicio), vacSoloFecha(in.FechaFin)
	if fin.Before(inicio) {
		return dto.VacacionesSolicitud{}, fmt.Errorf("%w: fecha_fin anterior a fecha_inicio", ErrVacRangoFechasInvalido)
	}
	emp, err := s.obtenerEmpleado(ctx, in.EmpleadoID)
	if err != nil {
		return dto.VacacionesSolicitud{}, err
	}
	if !emp.Activo {
		return dto.VacacionesSolicitud{}, ErrVacEmpleadoInactivo
	}
	dias, err := s.contarDiasHabiles(ctx, inicio, fin)
	if err != nil {
		return dto.VacacionesSolicitud{}, err
	}

	var creada dto.VacacionesSolicitud
	err = s.withAuditCtx(ctx, in.CreadoPor, func(q *dto.Queries) error {
		if err := vacBloquearEmpleado(ctx, q, in.EmpleadoID); err != nil {
			return err
		}
		traslapes, err := q.ContarSolicitudesVacacionesTraslapadas(ctx, in.EmpleadoID, inicio, fin, 0)
		if err != nil {
			return fmt.Errorf("verificando traslape de fechas: %w", err)
		}
		if traslapes > 0 {
			return ErrVacSolicitudTraslapada
		}
		// Solo compara contra el saldo actual; no descuenta otras solicitudes
		// pendientes/aprobadas aún sin disfrutar. La verificación definitiva
		// es la de RegistrarDisfrute, bajo lock.
		saldo, err := vacLeerSaldoParaActualizar(ctx, q, in.EmpleadoID)
		if err != nil {
			return err
		}
		if dias.GreaterThan(saldo.Saldo) {
			return fmt.Errorf("%w: solicita %s, disponible %s", ErrVacSaldoInsuficiente, dias, saldo.Saldo)
		}
		creada, err = q.CrearSolicitudVacaciones(ctx, in.EmpleadoID, inicio, fin, dias, in.CreadoPor)
		if err != nil {
			return fmt.Errorf("creando solicitud de vacaciones: %w", err)
		}
		return nil
	})
	return creada, err
}

type ResolverSolicitudInput struct {
	SolicitudID   int64
	Aprobar       bool
	MotivoRechazo string // obligatorio si Aprobar == false
	AprobadoPor   int64  // id de usuarios
}

// ResolverSolicitud aprueba o rechaza una solicitud pendiente. Aprobar NO
// descuenta días: el descuento ocurre en RegistrarDisfrute. Al aprobar, bajo
// lock del empleado, se valida (1) que las fechas no se traslapen con otra
// solicitud ya aprobada y (2) que el saldo disponible, menos los días ya
// comprometidos por solicitudes aprobadas sin disfrutar, cubra los días
// solicitados.
func (s *VacacionesService) ResolverSolicitud(ctx context.Context, in ResolverSolicitudInput) (dto.VacacionesSolicitud, error) {
	motivo := strings.TrimSpace(in.MotivoRechazo)
	if !in.Aprobar {
		if motivo == "" {
			return dto.VacacionesSolicitud{}, ErrVacMotivoRechazoRequerido
		}
		if utf8.RuneCountInString(motivo) > 255 {
			return dto.VacacionesSolicitud{}, ErrVacMotivoRechazoMuyLargo
		}
	}

	var resuelta dto.VacacionesSolicitud
	err := s.withAuditCtx(ctx, in.AprobadoPor, func(q *dto.Queries) error {
		sol, err := q.ObtenerSolicitudVacacionesForUpdate(ctx, in.SolicitudID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrVacSolicitudNoEncontrada
		}
		if err != nil {
			return fmt.Errorf("leyendo solicitud: %w", err)
		}
		if sol.Estado != "pendiente" {
			return ErrVacSolicitudNoPendiente
		}

		estado, motivoSQL := vacEstadoRechazada, sql.NullString{String: motivo, Valid: true}
		if in.Aprobar {
			estado, motivoSQL = vacEstadoAprobada, sql.NullString{}
			if err := vacBloquearEmpleado(ctx, q, sol.EmpleadoID); err != nil {
				return err
			}
			// Regla 1: no traslapar con otra solicitud ya aprobada del empleado.
			traslapes, err := q.ContarSolicitudesVacacionesAprobadasTraslapadas(ctx, sol.EmpleadoID,
				vacSoloFecha(sol.FechaInicio), vacSoloFecha(sol.FechaFin), sol.ID)
			if err != nil {
				return fmt.Errorf("verificando traslape de fechas: %w", err)
			}
			if traslapes > 0 {
				return ErrVacSolicitudTraslapada
			}
			// Regla 2: saldo disponible suficiente, descontando los días ya
			// comprometidos por otras solicitudes aprobadas sin disfrutar.
			saldo, err := vacLeerSaldoParaActualizar(ctx, q, sol.EmpleadoID)
			if err != nil {
				return err
			}
			comprometidos, err := q.SumarDiasVacacionesAprobadasSinDisfrutar(ctx, sol.EmpleadoID, sol.ID)
			if err != nil {
				return fmt.Errorf("sumando días comprometidos: %w", err)
			}
			libres := saldo.Saldo.Sub(comprometidos)
			if sol.DiasSolicitados.GreaterThan(libres) {
				return fmt.Errorf("%w: solicita %s, disponible %s (saldo %s, ya comprometidos %s)",
					ErrVacSaldoInsuficiente, sol.DiasSolicitados, libres, saldo.Saldo, comprometidos)
			}
		}

		resuelta, err = q.ResolverSolicitudVacaciones(ctx, in.SolicitudID, estado, in.AprobadoPor, motivoSQL)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrVacSolicitudNoPendiente
		}
		if err != nil {
			return fmt.Errorf("resolviendo solicitud: %w", err)
		}
		return nil
	})
	return resuelta, err
}

// --- Disfrute ----------------------------------------------------------------

type RegistrarDisfruteInput struct {
	SolicitudID   int64
	FechaEfectiva time.Time // si es cero se usa fecha_inicio de la solicitud
	RegistradoPor int64
}

// RegistrarDisfrute convierte una solicitud aprobada en disfrute: inserta un
// movimiento negativo en el ledger y mueve los días de disponibles a
// disfrutados en vacaciones_acumulado. Es idempotente por solicitud.
func (s *VacacionesService) RegistrarDisfrute(ctx context.Context, in RegistrarDisfruteInput) (dto.VacacionesAcumulado, error) {
	var resultado dto.VacacionesAcumulado
	err := s.withAuditCtx(ctx, in.RegistradoPor, func(q *dto.Queries) error {
		sol, err := q.ObtenerSolicitudVacacionesForUpdate(ctx, in.SolicitudID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrVacSolicitudNoEncontrada
		}
		if err != nil {
			return fmt.Errorf("leyendo solicitud: %w", err)
		}
		if sol.Estado != vacEstadoAprobada {
			return ErrVacSolicitudNoAprobada
		}
		if err := vacBloquearEmpleado(ctx, q, sol.EmpleadoID); err != nil {
			return err
		}
		ya, err := q.ExisteMovimientoVacacionesPorSolicitud(ctx, sol.ID, vacMovDisfrute)
		if err != nil {
			return fmt.Errorf("verificando disfrute previo: %w", err)
		}
		if ya {
			return ErrVacDisfruteYaRegistrado
		}

		fecha := vacSoloFecha(in.FechaEfectiva)
		if in.FechaEfectiva.IsZero() {
			fecha = vacSoloFecha(sol.FechaInicio)
		}
		previo, err := vacLeerSaldoParaActualizar(ctx, q, sol.EmpleadoID)
		if err != nil {
			return err
		}
		if sol.DiasSolicitados.GreaterThan(previo.Saldo) {
			return fmt.Errorf("%w: solicita %s, disponible %s", ErrVacSaldoInsuficiente, sol.DiasSolicitados, previo.Saldo)
		}

		if _, err := q.InsertarMovimientoVacaciones(ctx, sol.EmpleadoID, vacMovDisfrute, sol.DiasSolicitados.Neg(), fecha,
			sql.NullString{}, sql.NullInt64{Int64: sol.ID, Valid: true}, sql.NullString{}, in.RegistradoPor); err != nil {
			return fmt.Errorf("registrando movimiento de disfrute: %w", err)
		}
		if err := vacGuardarSaldo(ctx, q, sol.EmpleadoID, previo, fecha, decimal.Zero, sol.DiasSolicitados, in.RegistradoPor); err != nil {
			return err
		}
		resultado, err = q.ObtenerSaldoVacacionesEmpleado(ctx, sol.EmpleadoID)
		return err
	})
	return resultado, err
}

// --- Helpers internos --------------------------------------------------------

func (s *VacacionesService) obtenerEmpleado(ctx context.Context, empleadoID int64) (dto.Empleado, error) {
	if empleadoID <= 0 {
		return dto.Empleado{}, ErrVacEmpleadoNoEncontrado
	}
	emp, err := s.queries.GetEmpleadoByID(ctx, uint64(empleadoID))
	if errors.Is(err, sql.ErrNoRows) {
		return dto.Empleado{}, ErrVacEmpleadoNoEncontrado
	}
	if err != nil {
		return dto.Empleado{}, fmt.Errorf("leyendo empleado: %w", err)
	}
	return emp, nil
}

// contarDiasHabiles cuenta los días con es_habil = 1 en calendario_laboral_dia
// para [desde, hasta]. Si el calendario no cubre todo el rango falla en vez de
// adivinar. Único punto a cambiar si la política pasa a días naturales.
func (s *VacacionesService) contarDiasHabiles(ctx context.Context, desde, hasta time.Time) (decimal.Decimal, error) {
	dias, err := s.queries.ListarCalendarioPorRango(ctx, desde, hasta)
	if err != nil {
		return decimal.Zero, fmt.Errorf("consultando calendario laboral: %w", err)
	}
	esperados := int(hasta.Sub(desde).Hours()/24) + 1
	if len(dias) != esperados {
		return decimal.Zero, fmt.Errorf("%w: hay %d de %d días", ErrVacCalendarioIncompleto, len(dias), esperados)
	}
	habiles := 0
	for _, d := range dias {
		if d.EsHabil {
			habiles++
		}
	}
	if habiles == 0 {
		return decimal.Zero, ErrVacSinDiasHabiles
	}
	return decimal.NewFromInt(int64(habiles)), nil
}

func vacBloquearEmpleado(ctx context.Context, q *dto.Queries, empleadoID int64) error {
	err := q.BloquearEmpleado(ctx, empleadoID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrVacEmpleadoNoEncontrado
	}
	if err != nil {
		return fmt.Errorf("bloqueando empleado: %w", err)
	}
	return nil
}

// vacLeerSaldoParaActualizar lee (con lock) el corte más reciente; sin cortes
// devuelve el valor cero, no error.
func vacLeerSaldoParaActualizar(ctx context.Context, q *dto.Queries, empleadoID int64) (dto.VacacionesAcumulado, error) {
	saldo, err := q.ObtenerSaldoVacacionesEmpleadoForUpdate(ctx, empleadoID)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.VacacionesAcumulado{}, nil
	}
	if err != nil {
		return dto.VacacionesAcumulado{}, fmt.Errorf("leyendo saldo de vacaciones: %w", err)
	}
	return saldo, nil
}

// vacGuardarSaldo aplica deltas sobre el corte previo y hace upsert. Se parte
// de `Saldo` (no de acumulados - disfrutados) porque los cortes de
// migracion_inicial pueden traer saldo digitado a mano sin historial que lo
// respalde. El corte nunca queda con fecha anterior al último existente.
func vacGuardarSaldo(ctx context.Context, q *dto.Queries, empleadoID int64, previo dto.VacacionesAcumulado, fecha time.Time, deltaAcumulado, deltaDisfrutado decimal.Decimal, usuarioID int64) error {
	corte := fecha
	if corte.Before(previo.FechaCorte) {
		corte = vacSoloFecha(previo.FechaCorte)
	}
	disponibles := previo.Saldo.Add(deltaAcumulado).Sub(deltaDisfrutado)
	if disponibles.IsNegative() {
		return ErrVacSaldoInsuficiente
	}
	if err := q.UpsertSaldoVacacionesEmpleado(ctx, empleadoID, corte,
		previo.DiasAcumulados.Add(deltaAcumulado),
		previo.DiasDisfrutados.Add(deltaDisfrutado),
		disponibles,
		sql.NullInt64{Int64: usuarioID, Valid: true}); err != nil {
		return fmt.Errorf("guardando saldo de vacaciones: %w", err)
	}
	return nil
}

func vacSoloFecha(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// vacAniosServicio devuelve la antigüedad en años (años cumplidos + fracción
// del año en curso, truncada a 2 decimales para no cruzar un escalón antes de
// tiempo). Cero si `hasta` es anterior al ingreso.
func vacAniosServicio(ingreso, hasta time.Time) decimal.Decimal {
	ingreso, hasta = vacSoloFecha(ingreso), vacSoloFecha(hasta)
	if hasta.Before(ingreso) {
		return decimal.Zero
	}
	anios := hasta.Year() - ingreso.Year()
	aniversario := ingreso.AddDate(anios, 0, 0)
	if aniversario.After(hasta) {
		anios--
		aniversario = ingreso.AddDate(anios, 0, 0)
	}
	siguiente := ingreso.AddDate(anios+1, 0, 0)
	transcurridos := int64(hasta.Sub(aniversario).Hours() / 24)
	enElAnio := int64(siguiente.Sub(aniversario).Hours() / 24)
	fraccion := decimal.NewFromInt(transcurridos).Div(decimal.NewFromInt(enElAnio)).Truncate(2)
	return decimal.NewFromInt(int64(anios)).Add(fraccion)
}