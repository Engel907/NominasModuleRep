package asistencia

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

const vacMovCompensacion = "compensacion"

// ClaveVacacionesPoliticaAcumulacion: aniversario | anio_calendario | mensual.
const ClaveVacacionesPoliticaAcumulacion = "VACACIONES_POLITICA_ACUMULACION"

const (
	PoliticaAniversario    = "aniversario"
	PoliticaAnioCalendario = "anio_calendario"
	PoliticaMensual        = "mensual"
)

var (
	ErrVacDiasCompensacionInvalidos = errors.New("los días a compensar deben ser mayores que 0 y tener como máximo 2 decimales")
	ErrVacPoliticaInvalida          = fmt.Errorf("%s debe ser aniversario, anio_calendario o mensual", ClaveVacacionesPoliticaAcumulacion)
	ErrVacObservacionMuyLarga       = errors.New("las observaciones exceden 255 caracteres")
)

type RegistrarCompensacionInput struct {
	EmpleadoID    int64
	Dias          decimal.Decimal
	FechaEfectiva time.Time
	Observaciones string
	RegistradoPor int64
}

// RegistrarCompensacion registra días de vacaciones pagados en dinero: resta
// del saldo (movimiento 'compensacion' con días negativos) y queda pendiente
// de pago hasta que lo apruebe una planilla regular. Solo se paga si
// VACACIONES_MODO_PAGO es compensacion o ambos (si no, el cálculo del
// empleado falla con error explícito).
func (s *VacacionesService) RegistrarCompensacion(ctx context.Context, in RegistrarCompensacionInput) (dto.VacacionesAcumulado, error) {
	if !in.Dias.IsPositive() || !in.Dias.Equal(in.Dias.Round(2)) {
		return dto.VacacionesAcumulado{}, ErrVacDiasCompensacionInvalidos
	}
	obs := strings.TrimSpace(in.Observaciones)
	if utf8.RuneCountInString(obs) > 255 {
		return dto.VacacionesAcumulado{}, ErrVacObservacionMuyLarga
	}
	emp, err := s.obtenerEmpleado(ctx, in.EmpleadoID)
	if err != nil {
		return dto.VacacionesAcumulado{}, err
	}
	if !emp.Activo {
		return dto.VacacionesAcumulado{}, ErrVacEmpleadoInactivo
	}
	fecha := vacSoloFecha(in.FechaEfectiva)
	var resultado dto.VacacionesAcumulado
	err = s.runTx(ctx, in.RegistradoPor, func(ctx context.Context, q vacTx) error {
		if err := vacBloquearEmpleado(ctx, q, in.EmpleadoID); err != nil {
			return err
		}
		previo, err := vacLeerSaldoParaActualizar(ctx, q, in.EmpleadoID)
		if err != nil {
			return err
		}
		if in.Dias.GreaterThan(previo.Saldo) {
			return fmt.Errorf("%w: compensa %s, disponible %s", ErrVacSaldoInsuficiente, in.Dias, previo.Saldo)
		}
		if _, err := q.InsertarMovimientoVacaciones(ctx, in.EmpleadoID, vacMovCompensacion, in.Dias.Neg(), fecha,
			sql.NullString{}, sql.NullInt64{}, sql.NullString{String: obs, Valid: obs != ""}, in.RegistradoPor); err != nil {
			return fmt.Errorf("registrando compensación: %w", err)
		}
		if err := vacGuardarSaldo(ctx, q, in.EmpleadoID, previo, fecha, decimal.Zero, in.Dias, in.RegistradoPor); err != nil {
			return err
		}
		resultado, err = q.ObtenerSaldoVacacionesEmpleado(ctx, in.EmpleadoID)
		return err
	})
	return resultado, err
}

// PlanAcumulacion: qué acreditar a un empleado en `fecha` según la política.
// Ok = false si todavía no corresponde nada. Fraccion es la parte de los días
// anuales de la escala que se acredita (1 o 1/12).
type PlanAcumulacion struct {
	PeriodoReferencia string
	FechaEfectiva     time.Time
	Fraccion          decimal.Decimal
	Ok                bool
}

// PlanificarAcumulacion (pura). Re-ejecutarla el mismo día o días después da
// el mismo PeriodoReferencia: AcumularVacaciones es idempotente por período.
func PlanificarAcumulacion(politica string, ingreso, fecha time.Time) (PlanAcumulacion, error) {
	ingreso, fecha = vacSoloFecha(ingreso), vacSoloFecha(fecha)
	uno := decimal.NewFromInt(1)
	switch politica {
	case PoliticaAniversario:
		n := fecha.Year() - ingreso.Year()
		if ingreso.AddDate(n, 0, 0).After(fecha) {
			n--
		}
		if n < 1 {
			return PlanAcumulacion{}, nil
		}
		return PlanAcumulacion{PeriodoReferencia: fmt.Sprintf("ANIV-%d", n), FechaEfectiva: ingreso.AddDate(n, 0, 0), Fraccion: uno, Ok: true}, nil
	case PoliticaAnioCalendario:
		anio := fecha.Year() - 1
		inicio := time.Date(anio, time.January, 1, 0, 0, 0, 0, time.UTC)
		if ingreso.After(inicio) {
			return PlanAcumulacion{}, nil
		}
		return PlanAcumulacion{PeriodoReferencia: fmt.Sprintf("%04d", anio),
			FechaEfectiva: time.Date(anio, time.December, 31, 0, 0, 0, 0, time.UTC), Fraccion: uno, Ok: true}, nil
	case PoliticaMensual:
		inicio := time.Date(fecha.Year(), fecha.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		if ingreso.After(inicio) {
			return PlanAcumulacion{}, nil
		}
		fin := inicio.AddDate(0, 1, -1)
		return PlanAcumulacion{PeriodoReferencia: inicio.Format("2006-01"), FechaEfectiva: fin,
			Fraccion: uno.Div(decimal.NewFromInt(12)), Ok: true}, nil
	}
	return PlanAcumulacion{}, ErrVacPoliticaInvalida
}

// ResumenAcumulacion: resultado del job acumular-vacaciones.
type ResumenAcumulacion struct {
	Acreditados, YaRegistrados, SinDerecho int
	Errores                                map[int64]string
}

// EmpleadosIngresoLector: empleados activos con su fecha de ingreso.
type EmpleadosIngresoLector interface {
	ListarEmpleadosActivosIngreso(ctx context.Context) ([]dto.EmpleadoIngresoRow, error)
}

// AcumularSegunPolitica recorre los empleados activos y acredita lo que
// corresponda en `fecha`. Es seguro re-ejecutarlo: una acumulación ya
// registrada cuenta como YaRegistrados. Un error de un empleado no detiene
// a los demás (queda en Errores); un error de lectura de la lista, sí.
func (s *VacacionesService) AcumularSegunPolitica(ctx context.Context, empleados EmpleadosIngresoLector, politica string, fecha time.Time, usuarioID int64) (ResumenAcumulacion, error) {
	res := ResumenAcumulacion{Errores: map[int64]string{}}
	if _, err := PlanificarAcumulacion(politica, fecha, fecha); err != nil {
		return res, err
	}
	lista, err := empleados.ListarEmpleadosActivosIngreso(ctx)
	if err != nil {
		return res, fmt.Errorf("listando empleados activos: %w", err)
	}
	for _, e := range lista {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		plan, err := PlanificarAcumulacion(politica, e.FechaIngreso, fecha)
		if err != nil {
			return res, err
		}
		if !plan.Ok {
			res.SinDerecho++
			continue
		}
		_, err = s.AcumularVacaciones(ctx, AcumularVacacionesInput{EmpleadoID: e.EmpleadoID, PeriodoReferencia: plan.PeriodoReferencia,
			FechaEfectiva: plan.FechaEfectiva, Fraccion: plan.Fraccion, CreadoPor: usuarioID})
		switch {
		case err == nil:
			res.Acreditados++
		case errors.Is(err, ErrAcumulacionYaRegistrada):
			res.YaRegistrados++
		default:
			res.Errores[e.EmpleadoID] = err.Error()
		}
	}
	return res, nil
}
