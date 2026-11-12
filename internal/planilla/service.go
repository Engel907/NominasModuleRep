// Orquestador del cálculo de un período (implementa Calculador de F19).
//
// Corre DENTRO de la transacción que abre PeriodoService: si cualquier
// empleado falla, devuelve ErrCalculoConErrores con TODOS los errores, la
// transacción se revierte y la BD queda idéntica (todo o nada).
package planilla

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/db/dto"
)

// VersionMotor queda en periodos_planilla.version_motor de cada cálculo.
const VersionMotor = "planilla-motor-1.0.0"

var (
	ErrClaseNoSoportada      = &ErrorCiclo{"clase_no_soportada", "este tipo de planilla todavía no se calcula con el motor", 422}
	ErrPeriodoSinEmpleados   = &ErrorCiclo{"periodo_sin_empleados", "no hay empleados activos con contrato en las fechas del período", 422}
	ErrParametrosIncompletos = &ErrorCiclo{"parametros_incompletos", "faltan parámetros legales vigentes para calcular el período", 422}
)

// TipoPlanillaLector: lectura del tipo de planilla del período.
type TipoPlanillaLector interface {
	ObtenerTipoPlanilla(ctx context.Context, id int64) (dto.TipoPlanillaRow, error)
}

// ErrCalculoConErrores: uno o más empleados no se pudieron calcular. No se
// persistió nada.
type ErrCalculoConErrores struct {
	Errores []EmpleadoErrorOutput
}

func (e *ErrCalculoConErrores) Error() string {
	return fmt.Sprintf("planilla: %d empleado(s) con errores; no se persistió el cálculo", len(e.Errores))
}

type Orquestador struct {
	h     *Hidratador
	m     *MotorEmpleado
	c     *CargadorParametros
	w     *EscritorDetalle
	tipos TipoPlanillaLector
}

var _ Calculador = (*Orquestador)(nil)

func NewOrquestador(h *Hidratador, m *MotorEmpleado, c *CargadorParametros, w *EscritorDetalle, tipos TipoPlanillaLector) *Orquestador {
	return &Orquestador{h: h, m: m, c: c, w: w, tipos: tipos}
}

// snapshotCalculo: lo que se guarda en periodos_planilla.parametros_snapshot
// para reconstruir con qué valores se calculó (porcentajes en FRACCIÓN).
type snapshotCalculo struct {
	VersionMotor   string             `json:"version_motor"`
	FechaEfectiva  string             `json:"fecha_efectiva"`
	PeriodoInicio  string             `json:"periodo_inicio"`
	PeriodoFin     string             `json:"periodo_fin"`
	TipoPlanillaID int64              `json:"tipo_planilla_id"`
	Clase          string             `json:"clase"`
	CalculadoEn    time.Time          `json:"calculado_en"`
	Parametros     *ParametrosCalculo `json:"parametros"`
}

func (o *Orquestador) Calcular(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow, usuarioID int64) error {
	tipo, err := o.tipos.ObtenerTipoPlanilla(ctx, periodo.TipoPlanillaID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTipoPlanillaNoEncontrado
	}
	if err != nil {
		return fmt.Errorf("planilla: leyendo tipo de planilla %d: %w", periodo.TipoPlanillaID, err)
	}
	// Aguinaldo, salario escolar y liquidación entran al motor en L19/L22.
	if ClaseTipoPlanilla(tipo.Clase) != ClaseRegular {
		return ErrClaseNoSoportada
	}
	if !tipo.FraccionMensual.Valid || !tipo.FraccionMensual.Decimal.IsPositive() {
		return fmt.Errorf("planilla: el tipo de planilla %d (regular) no tiene fraccion_mensual válida", tipo.ID)
	}

	params, err := o.c.Cargar(ctx, periodo.FechaInicio, periodo.FechaFin)
	if err != nil {
		if errors.Is(err, ErrParametroFaltante) || errors.Is(err, ErrTramosRentaInvalidos) {
			// El mensaje del cargador es de dominio (clave y fecha), apto para el cliente.
			return &ErrorCiclo{Codigo: ErrParametrosIncompletos.Codigo, Mensaje: err.Error(), HTTP: ErrParametrosIncompletos.HTTP}
		}
		return fmt.Errorf("planilla: cargando parámetros: %w", err)
	}
	divisor, err := divisorEntero(params.DivisorDiasMes)
	if err != nil {
		return &ErrorCiclo{Codigo: ErrParametrosIncompletos.Codigo, Mensaje: err.Error(), HTTP: ErrParametrosIncompletos.HTTP}
	}
	fraccion := tipo.FraccionMensual.Decimal
	params.FraccionMensualPeriodo = fraccion
	params.DiasPeriodoCompleto = params.DivisorDiasMes.Mul(fraccion)

	pc := PeriodoParaCalculo{
		ID: periodo.ID, Inicio: periodo.FechaInicio, Fin: periodo.FechaFin,
		Clase: tipo.Clase, TipoPlanillaID: tipo.ID, FraccionMensual: fraccion,
		DiasPeriodoCompleto: params.DiasPeriodoCompleto, DivisorDiasMes: divisor,
	}
	ids, err := o.h.EmpleadosDelPeriodo(ctx, pc)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return ErrPeriodoSinEmpleados
	}

	var (
		resultados []ResultadoPersistible
		errores    []EmpleadoErrorOutput
	)
	for _, id := range ids {
		r, codigo, err := o.calcularEmpleado(ctx, pc, *params, id)
		if err != nil {
			if esErrorInfraestructura(err) {
				return err // falla de BD/red: no es culpa del empleado, se aborta con 500
			}
			errores = append(errores, EmpleadoErrorOutput{EmpleadoID: id, Codigo: codigo, Mensaje: err.Error()})
			continue
		}
		resultados = append(resultados, r)
	}
	if len(errores) > 0 {
		return &ErrCalculoConErrores{Errores: errores}
	}

	if err := o.w.Escribir(ctx, q, periodo.ID, resultados); err != nil {
		return err
	}
	snap, err := json.Marshal(snapshotCalculo{
		VersionMotor:   VersionMotor,
		FechaEfectiva:  periodo.FechaFin.Format("2006-01-02"),
		PeriodoInicio:  periodo.FechaInicio.Format("2006-01-02"),
		PeriodoFin:     periodo.FechaFin.Format("2006-01-02"),
		TipoPlanillaID: tipo.ID,
		Clase:          tipo.Clase,
		CalculadoEn:    time.Now().UTC(),
		Parametros:     params,
	})
	if err != nil {
		return fmt.Errorf("planilla: serializando snapshot: %w", err)
	}
	return o.w.GuardarSnapshot(ctx, q, periodo.ID, snap, VersionMotor)
}

// calcularEmpleado hidrata, calcula y valida el cuadre de un empleado.
// Devuelve el código estable del error para EmpleadoErrorOutput.
func (o *Orquestador) calcularEmpleado(ctx context.Context, pc PeriodoParaCalculo, p ParametrosCalculo, empleadoID int64) (ResultadoPersistible, string, error) {
	ent, err := o.h.Hidratar(ctx, pc, empleadoID)
	if err != nil {
		var sinContrato ErrEmpleadoSinContrato
		var sinLaborales ErrEmpleadoSinDatosLaborales
		switch {
		case errors.As(err, &sinContrato):
			return ResultadoPersistible{}, "empleado_sin_contrato", err
		case errors.As(err, &sinLaborales):
			return ResultadoPersistible{}, "empleado_sin_datos_laborales", err
		}
		return ResultadoPersistible{}, "datos_incompletos", err
	}
	det, err := o.m.CalcularEmpleado(ctx, ent.Input, p)
	if err != nil {
		return ResultadoPersistible{}, "calculo_invalido", err
	}
	r := ResultadoPersistible{
		Detalle: det, ContratoID: ent.ContratoID, ProgramaPresupuestarioID: ent.ProgramaPresupuestarioID,
		Patronal: det.Patronal, Incapacidad: det.Incapacidades, Prestamos: det.Prestamos,
	}
	if err := verificarCuadre(r); err != nil {
		return ResultadoPersistible{}, "detalle_no_cuadra", err
	}
	return r, "", nil
}

// esErrorInfraestructura: errores de BD, conexión o contexto. Esos abortan el
// cálculo completo (500) en vez de reportarse como error de un empleado, y su
// texto nunca llega al cliente (D9).
func esErrorInfraestructura(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) ||
		errors.Is(err, driver.ErrBadConn) ||
		errors.Is(err, sql.ErrConnDone) ||
		errors.Is(err, sql.ErrTxDone) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
