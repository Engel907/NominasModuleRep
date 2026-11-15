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
	"github.com/shopspring/decimal"

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

// ObservadorCalculo: recibe la duración y el resultado de cada cálculo
// de período (métricas). nil = sin observación.
type ObservadorCalculo interface {
	ObservarCalculo(clase string, duracion time.Duration, empleadosConError int, ok bool)
}

type Orquestador struct {
	obs      ObservadorCalculo
	h        *Hidratador
	m        *MotorEmpleado
	c        *CargadorParametros
	w        *EscritorDetalle
	tipos    TipoPlanillaLector
	ventanas *Ventanas // valida que un período de acumulado sea su ventana de cálculo
}

var _ Calculador = (*Orquestador)(nil)

func NewOrquestador(h *Hidratador, m *MotorEmpleado, c *CargadorParametros, w *EscritorDetalle, tipos TipoPlanillaLector, ventanas *Ventanas) *Orquestador {
	return &Orquestador{h: h, m: m, c: c, w: w, tipos: tipos, ventanas: ventanas}
}

// ErrPeriodoNoEsVentana: un período de aguinaldo/salario escolar debe tener
// exactamente las fechas de su ventana de cálculo (créelo con
// POST /aguinaldo|salario-escolar/:anio/ejecutar).
var ErrPeriodoNoEsVentana = &ErrorCiclo{"periodo_no_es_ventana", "las fechas del período no coinciden con la ventana de cálculo del acumulado", 422}

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
	// AjustesPorEmpleado: parámetros generales propios con que se calculó cada
	// empleado que los tiene (empleado_id -> clave -> valor).
	AjustesPorEmpleado map[int64]map[string]string `json:"ajustes_por_empleado,omitempty"`
}

// ConObservador agrega el observador de métricas.
func (o *Orquestador) ConObservador(obs ObservadorCalculo) *Orquestador {
	o.obs = obs
	return o
}

func (o *Orquestador) Calcular(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow, usuarioID int64) (err error) {
	if o.obs != nil {
		inicio, clase := time.Now(), "desconocida"
		defer func() {
			if t, e := o.tipos.ObtenerTipoPlanilla(ctx, periodo.TipoPlanillaID); e == nil {
				clase = t.Clase
			}
			n := 0
			var ce *ErrCalculoConErrores
			if errors.As(err, &ce) {
				n = len(ce.Errores)
			}
			o.obs.ObservarCalculo(clase, time.Since(inicio), n, err == nil)
		}()
	}
	return o.calcular(ctx, q, periodo)
}

func (o *Orquestador) calcular(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow) error {
	tipo, err := o.tipos.ObtenerTipoPlanilla(ctx, periodo.TipoPlanillaID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTipoPlanillaNoEncontrado
	}
	if err != nil {
		return fmt.Errorf("planilla: leyendo tipo de planilla %d: %w", periodo.TipoPlanillaID, err)
	}
	clase := ClaseTipoPlanilla(tipo.Clase)
	acumulado := clase == ClaseAguinaldo || clase == ClaseSalarioEscolar
	liquidacion := clase == ClaseLiquidacion
	if clase != ClaseRegular && !acumulado && !liquidacion {
		return ErrClaseNoSoportada
	}
	if clase == ClaseRegular && (!tipo.FraccionMensual.Valid || !tipo.FraccionMensual.Decimal.IsPositive()) {
		return fmt.Errorf("planilla: el tipo de planilla %d (regular) no tiene fraccion_mensual válida", tipo.ID)
	}

	params, err := o.c.Cargar(ctx, periodo.FechaInicio, periodo.FechaFin)
	if err == nil && acumulado {
		err = o.c.CargarAcumulado(ctx, clase, periodo.FechaFin, params)
	}
	if err == nil && liquidacion {
		err = o.c.CargarLiquidacion(ctx, periodo.FechaFin, params)
	}
	if err != nil {
		if errors.Is(err, ErrParametroFaltante) || errors.Is(err, ErrTramosRentaInvalidos) {
			return &ErrorCiclo{Codigo: ErrParametrosIncompletos.Codigo, Mensaje: err.Error(), HTTP: ErrParametrosIncompletos.HTTP}
		}
		return fmt.Errorf("planilla: cargando parámetros: %w", err)
	}
	divisor, err := divisorEntero(params.DivisorDiasMes)
	if err != nil {
		return &ErrorCiclo{Codigo: ErrParametrosIncompletos.Codigo, Mensaje: err.Error(), HTTP: ErrParametrosIncompletos.HTTP}
	}
	var fraccion decimal.Decimal
	anio := int32(0)
	if acumulado {
		v, err := o.ventanaDelPeriodo(ctx, clase, periodo)
		if err != nil {
			return err
		}
		anio = int32(v.Anio)
	} else if !liquidacion {
		fraccion = tipo.FraccionMensual.Decimal
		params.FraccionMensualPeriodo = fraccion
		params.DiasPeriodoCompleto = params.DivisorDiasMes.Mul(fraccion)
		if fraccion.LessThan(decimal.NewFromInt(1)) {
			if err := o.c.CargarMetodoRenta(ctx, periodo.FechaFin, params); err != nil {
				if errors.Is(err, ErrParametroFaltante) {
					return &ErrorCiclo{Codigo: ErrParametrosIncompletos.Codigo, Mensaje: err.Error(), HTTP: ErrParametrosIncompletos.HTTP}
				}
				return fmt.Errorf("planilla: cargando parámetros: %w", err)
			}
		}
	}

	pc := PeriodoParaCalculo{
		ID: periodo.ID, Inicio: periodo.FechaInicio, Fin: periodo.FechaFin,
		Clase: tipo.Clase, TipoPlanillaID: tipo.ID, FraccionMensual: fraccion,
		DiasPeriodoCompleto: params.DiasPeriodoCompleto, DivisorDiasMes: divisor,
		LiqPrestamosModo: params.LiqPrestamosModo,
	}
	pc.conBasesSalario(*params)
	ids, err := o.h.EmpleadosDelPeriodo(ctx, pc)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return ErrPeriodoSinEmpleados
	}

	var retros map[int64]dto.RetroactivoRow
	if clase == ClaseRegular {
		if retros, err = retroactivosDelPeriodo(ctx, q, periodo.ID); err != nil {
			return err
		}
	}

	var (
		resultados []ResultadoPersistible
		errores    []EmpleadoErrorOutput
		ajustes    = map[int64]map[string]string{}
		calculados = make(map[int64]bool, len(ids))
	)
	for _, id := range ids {
		calculados[id] = true
		pcE, pE, aj, err := o.parametrosDelEmpleado(ctx, clase, periodo, pc, *params, id)
		if err != nil {
			if esErrorInfraestructura(err) {
				return err
			}
			errores = append(errores, EmpleadoErrorOutput{EmpleadoID: id, Codigo: "parametros_empleado_invalidos", Mensaje: err.Error()})
			continue
		}
		if aj != nil {
			ajustes[id] = aj
		}
		var difs []RetroactivoInput
		var advRetro []string
		if sol, ok := retros[id]; ok {
			if difs, advRetro, err = o.retroactivosEmpleado(ctx, q, periodo, sol); err != nil {
				if esErrorInfraestructura(err) {
					return err
				}
				errores = append(errores, EmpleadoErrorOutput{EmpleadoID: id, Codigo: "retroactivo_invalido", Mensaje: err.Error()})
				continue
			}
		}
		r, codigo, err := o.calcularEmpleado(ctx, pcE, pE, id, difs)
		if err != nil {
			if esErrorInfraestructura(err) {
				return err
			}
			errores = append(errores, EmpleadoErrorOutput{EmpleadoID: id, Codigo: codigo, Mensaje: err.Error()})
			continue
		}
		r.Detalle.Advertencias = append(r.Detalle.Advertencias, advRetro...)
		if acumulado {
			if !r.Detalle.TotalIngresos.IsPositive() {
				continue
			}
			r.Acumulado = acumuladoDe(clase, anio, periodo, r, pE)
		}
		resultados = append(resultados, r)
	}
	for id := range retros {
		if !calculados[id] {
			errores = append(errores, EmpleadoErrorOutput{EmpleadoID: id, Codigo: "retroactivo_sin_contrato",
				Mensaje: fmt.Sprintf("el empleado %d tiene un retroactivo pero no tiene contrato en el período", id)})
		}
	}
	if len(errores) > 0 {
		return &ErrCalculoConErrores{Errores: errores}
	}
	if len(resultados) == 0 {
		return ErrPeriodoSinEmpleados
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

		AjustesPorEmpleado: ajustes,
	})
	if err != nil {
		return fmt.Errorf("planilla: serializando snapshot: %w", err)
	}
	return o.w.GuardarSnapshot(ctx, q, periodo.ID, snap, VersionMotor)
}

// parametrosDelEmpleado: período y parámetros con que se calcula el empleado.
// Si tiene parámetros generales propios vigentes a la fecha fin, se vuelven a
// resolver con ellos (y con ellos el divisor y los días del período); los
// factores de hora extra y divisores de jornada propios reemplazan a los
// generales. aj = los valores propios aplicados (nil si no tiene).
func (o *Orquestador) parametrosDelEmpleado(ctx context.Context, clase ClaseTipoPlanilla, periodo dto.PeriodoCicloRow,
	pc PeriodoParaCalculo, p ParametrosCalculo, empleadoID int64) (PeriodoParaCalculo, ParametrosCalculo, map[string]string, error) {
	aj, err := o.c.AjustesEmpleado(ctx, empleadoID, periodo.FechaFin)
	if err != nil {
		return pc, p, nil, err
	}
	pe := p
	if aj != nil {
		if pe, err = o.c.ParametrosEmpleado(ctx, empleadoID, clase, periodo.FechaFin, p); err != nil {
			return pc, p, nil, fmt.Errorf("planilla: empleado_id=%d, parámetros propios: %w", empleadoID, err)
		}
		div, err := divisorEntero(pe.DivisorDiasMes)
		if err != nil {
			return pc, p, nil, fmt.Errorf("planilla: empleado_id=%d, parámetros propios: %w", empleadoID, err)
		}
		pc.DivisorDiasMes, pc.DiasPeriodoCompleto, pc.LiqPrestamosModo = div, pe.DiasPeriodoCompleto, pe.LiqPrestamosModo
		pc.conBasesSalario(pe)
	}
	tablas, err := o.c.AjustesTablasEmpleado(ctx, empleadoID, periodo.FechaFin, &pe)
	if err != nil {
		return pc, p, nil, err
	}
	if len(tablas) > 0 {
		if aj == nil {
			aj = make(map[string]string, len(tablas))
		}
		for k, v := range tablas {
			aj[k] = v
		}
	}
	return pc, pe, aj, nil
}

// conBasesSalario copia al período las bases y ventanas del salario de
// vacaciones e incapacidad que rigen para el cálculo.
func (pc *PeriodoParaCalculo) conBasesSalario(p ParametrosCalculo) {
	pc.VacacionesSalarioBase, pc.VacacionesPromedioSemanas = p.VacacionesSalarioBase, p.VacacionesPromedioSemanas
	pc.IncapacidadSalarioBase, pc.IncapacidadPromedioMeses = p.IncapacidadSalarioBase, p.IncapacidadPromedioMeses
}

// ventanaDelPeriodo: el año del acumulado es el de fecha_fin y el período debe
// coincidir exactamente con la ventana de cálculo de ese año (los parámetros
// de la ventana se leen a fecha_fin).
func (o *Orquestador) ventanaDelPeriodo(ctx context.Context, clase ClaseTipoPlanilla, periodo dto.PeriodoCicloRow) (VentanaAcumulado, error) {
	if o.ventanas == nil {
		return VentanaAcumulado{}, ErrClaseNoSoportada
	}
	v, err := o.ventanas.Ventana(ctx, clase, periodo.FechaFin.Year(), periodo.FechaFin)
	if err != nil {
		return VentanaAcumulado{}, err
	}
	if !v.CalculoDesde.Equal(soloFecha(periodo.FechaInicio)) || !v.CalculoHasta.Equal(soloFecha(periodo.FechaFin)) {
		return VentanaAcumulado{}, &ErrorCiclo{Codigo: ErrPeriodoNoEsVentana.Codigo, HTTP: ErrPeriodoNoEsVentana.HTTP,
			Mensaje: fmt.Sprintf("el %s %d se calcula del %s al %s", clase, v.Anio,
				v.CalculoDesde.Format("2006-01-02"), v.CalculoHasta.Format("2006-01-02"))}
	}
	return v, nil
}

// acumuladoDe arma la fila de aguinaldos / salarios_escolares del empleado.
func acumuladoDe(clase ClaseTipoPlanilla, anio int32, periodo dto.PeriodoCicloRow, r ResultadoPersistible, p ParametrosCalculo) *AcumuladoPersistible {
	a := &AcumuladoPersistible{Clase: clase, Anio: anio, Desde: periodo.FechaInicio, Hasta: periodo.FechaFin,
		TotalBase: r.Detalle.baseAcumulada, Monto: r.Detalle.TotalIngresos}
	if clase == ClaseSalarioEscolar {
		a.Porcentaje = p.PorcentajeSalarioEscolar
	}
	return a
}

// calcularEmpleado hidrata, calcula y valida el cuadre de un empleado.
// Devuelve el código estable del error para EmpleadoErrorOutput.
func (o *Orquestador) calcularEmpleado(ctx context.Context, pc PeriodoParaCalculo, p ParametrosCalculo, empleadoID int64, retros []RetroactivoInput) (ResultadoPersistible, string, error) {
	ent, err := o.h.Hidratar(ctx, pc, empleadoID)
	if err != nil {
		var sinContrato ErrEmpleadoSinContrato
		var sinLaborales ErrEmpleadoSinDatosLaborales
		var sinRegimen ErrEmpleadoSinRegimenCCSS
		switch {
		case errors.As(err, &sinContrato):
			return ResultadoPersistible{}, "empleado_sin_contrato", err
		case errors.As(err, &sinLaborales):
			return ResultadoPersistible{}, "empleado_sin_datos_laborales", err
		case errors.As(err, &sinRegimen):
			return ResultadoPersistible{}, "empleado_sin_regimen_ccss", err
		}
		return ResultadoPersistible{}, "datos_incompletos", err
	}
	ent.Input.Retroactivos = retros
	det, err := o.m.CalcularEmpleado(ctx, ent.Input, p)
	if err != nil {
		return ResultadoPersistible{}, "calculo_invalido", err
	}
	det.baseAcumulada = ent.Input.BaseAguinaldoAcumulada.Add(ent.Input.BaseSalarioEscolarAcumulada)
	r := ResultadoPersistible{
		Detalle: det, ContratoID: ent.ContratoID, ProgramaPresupuestarioID: ent.ProgramaPresupuestarioID,
		Patronal: det.Patronal, Incapacidad: det.Incapacidades, Prestamos: det.Prestamos,
		VacacionesCompensadas: det.VacacionesCompensadas,
	}
	if ent.Input.Liquidacion != nil {
		r.LiquidacionID = ent.Input.Liquidacion.LiquidacionID
	}
	if err := verificarCuadre(r); err != nil {
		return ResultadoPersistible{}, "detalle_no_cuadra", err
	}
	return r, "", nil
}

// esErrorInfraestructura: errores de BD, conexión o contexto. Esos abortan el
// cálculo completo (500) en vez de reportarse como error de un empleado, y su
// texto nunca llega al cliente.
func esErrorInfraestructura(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) ||
		errors.Is(err, driver.ErrBadConn) ||
		errors.Is(err, sql.ErrConnDone) ||
		errors.Is(err, sql.ErrTxDone) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
