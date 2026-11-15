package rrhh

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/liquidacion"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla"
)

// Claves de parametros_generales de la liquidación (000118 / 000134).
const (
	ClaveLiqFraccionMesesUmbral          = "LIQ_FRACCION_MESES_UMBRAL"
	ClaveLiqPromedioMeses                = "LIQ_PROMEDIO_MESES"
	ClaveVentanaPromedioVacacionesSemana = planilla.ClaveVentanaPromedioVacacionesSemanas
)

// Errores de la liquidación (el handler los traduce a 422).
var (
	// ErrSinDatosLaborales: no se puede resolver el contrato/régimen.
	ErrSinDatosLaborales = errors.New("liquidacion: el empleado no tiene contrato/datos_laborales vigente")
	// ErrMotivoSinRegla: no hay parametros_motivos_salida vigente para el motivo.
	ErrMotivoSinRegla = errors.New("liquidacion: no hay regla vigente en parametros_motivos_salida para el motivo de salida")
	// ErrParametroLiquidacion: falta o es inválido un parámetro legal.
	ErrParametroLiquidacion = errors.New("liquidacion: parámetro legal faltante o inválido")
)

// LiquidacionRepository: lecturas que hidratan el cálculo (*dto.Queries).
type LiquidacionRepository interface {
	ObtenerFechaIngresoEmpleado(ctx context.Context, empleadoID int64) (time.Time, error)
	ObtenerReglaMotivoSalidaEnFecha(ctx context.Context, motivo string, fecha time.Time) (dto.ReglaMotivoSalidaRow, error)
	SumarBasePromedioEmpleado(ctx context.Context, empleadoID int64, desde, hasta time.Time, columna string) (dto.BasePromedioRow, error)
	SumarBaseAcumuladaEmpleado(ctx context.Context, empleadoID int64, desde, hasta time.Time, columna string) (decimal.Decimal, error)
	UltimoFinPeriodoRegularEmpleado(ctx context.Context, empleadoID int64) (sql.NullTime, error)
	GetSaldoVacacionesDisponibles(ctx context.Context, empleadoID int64) (decimal.Decimal, error)
	ObtenerPorcentajeSalarioEscolarEnFecha(ctx context.Context, fecha time.Time) (decimal.Decimal, error)
	ListarEscalaCesantiaEnFecha(ctx context.Context, regimenLaboralID int64, fecha time.Time) ([]dto.EscalaCesantiaRow, error)
	ListarEscalaPreavisoEnFecha(ctx context.Context, regimenLaboralID int64, fecha time.Time) ([]dto.EscalaPreavisoRow, error)
}

var _ LiquidacionRepository = (*dto.Queries)(nil)

// RegimenResolver: contrato (y régimen laboral) vigente en una fecha; lo
// satisface planilla.MotorRepository.
type RegimenResolver interface {
	ObtenerContratoVigenteEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (planilla.ContratoVigente, error)
}

// EnterosGenerales: lectura de parámetros generales enteros (parametros.Generales).
type EnterosGenerales interface {
	Entero(ctx context.Context, clave string, fecha time.Time) (int64, error)
	Texto(ctx context.Context, clave string, fecha time.Time) (string, error)
}

// LiquidacionService orquesta hidratación, cálculo puro y persistencia.
type LiquidacionService struct {
	sqlDB     *sql.DB
	repo      LiquidacionRepository
	regimen   RegimenResolver
	generales EnterosGenerales
	ventanas  *planilla.Ventanas
	ahora     func() time.Time // nil = time.Now (las pruebas lo fijan)
}

// NewLiquidacionService: regimen típicamente infra.NewMotorRepository(queries);
// generales, el mismo *parametros.Generales del cargador del motor.
func NewLiquidacionService(sqlDB *sql.DB, repo LiquidacionRepository, regimen RegimenResolver, generales EnterosGenerales) *LiquidacionService {
	return &LiquidacionService{sqlDB: sqlDB, repo: repo, regimen: regimen, generales: generales, ventanas: planilla.NewVentanas(generales)}
}

// ProyectarSolicitud: datos que da RR. HH. AplicaResponsabilidadPatronal es
// explícito; el preaviso y la cesantía se pagan solo si además el motivo los
// admite (parametros_motivos_salida).
type ProyectarSolicitud struct {
	EmpleadoID                    int64
	FechaSalida                   time.Time
	MotivoSalida                  string
	AplicaResponsabilidadPatronal bool
}

// calculoPreparado: entrada, parámetros y resultado de una liquidación.
type calculoPreparado struct {
	entrada liquidacion.EntradaLiquidacion
	rubros  liquidacion.RubrosLiquidacion
}

// prepararCalculo hidrata y calcula. Compartido por proyectar y procesar.
func (s *LiquidacionService) prepararCalculo(ctx context.Context, sol ProyectarSolicitud) (*calculoPreparado, error) {
	if s.generales == nil {
		return nil, fmt.Errorf("%w: parámetros generales no configurados", ErrParametroLiquidacion)
	}
	salida := soloFechaUTC(sol.FechaSalida)
	generales := s.generalesDe(sol.EmpleadoID)

	ingreso, err := s.repo.ObtenerFechaIngresoEmpleado(ctx, sol.EmpleadoID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: el empleado %d no existe", ErrSinDatosLaborales, sol.EmpleadoID)
	}
	if err != nil {
		return nil, fmt.Errorf("liquidacion: leyendo fecha de ingreso: %w", err)
	}

	regla, err := s.repo.ObtenerReglaMotivoSalidaEnFecha(ctx, sol.MotivoSalida, salida)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w (%s al %s)", ErrMotivoSinRegla, sol.MotivoSalida, salida.Format("2006-01-02"))
	}
	if err != nil {
		return nil, fmt.Errorf("liquidacion: leyendo parametros_motivos_salida: %w", err)
	}
	p := liquidacion.ParametrosLiquidacion{Motivo: liquidacion.ReglaMotivo{
		PagaPreaviso: regla.PagaPreaviso, PagaCesantia: regla.PagaCesantia,
		PagaVacaciones: regla.PagaVacaciones, PagaAguinaldo: regla.PagaAguinaldo,
		PagaSalarioEscolar: regla.PagaSalarioEscolar,
	}}

	entero := func(clave string) (int64, error) {
		v, err := generales.Entero(ctx, clave, salida)
		if err != nil {
			return 0, fmt.Errorf("%w: %s: %v", ErrParametroLiquidacion, clave, err)
		}
		if v <= 0 && clave != ClaveLiqFraccionMesesUmbral {
			return 0, fmt.Errorf("%w: %s debe ser > 0 (es %d)", ErrParametroLiquidacion, clave, v)
		}
		return v, nil
	}
	divisor, err := entero(parametros.ClaveDivisorDiasMes)
	if err != nil {
		return nil, err
	}
	p.DivisorDiasMes = int(divisor)

	in := liquidacion.EntradaLiquidacion{
		FechaIngreso:                  ingreso,
		FechaSalida:                   salida,
		AplicaResponsabilidadPatronal: sol.AplicaResponsabilidadPatronal,
		DiasVacacionesPendientes:      decimal.Zero,
		BaseAguinaldo:                 decimal.Zero,
		BaseSalarioEscolar:            decimal.Zero,
	}

	responsabilidad := sol.AplicaResponsabilidadPatronal && (regla.PagaPreaviso || regla.PagaCesantia)
	if responsabilidad {
		if err := s.cargarResponsabilidad(ctx, sol.EmpleadoID, salida, &in, &p, entero); err != nil {
			return nil, err
		}
	}

	if regla.PagaVacaciones {
		saldo, err := s.repo.GetSaldoVacacionesDisponibles(ctx, sol.EmpleadoID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			saldo = decimal.Zero
		case err != nil:
			return nil, fmt.Errorf("liquidacion: consultando saldo de vacaciones: %w", err)
		}
		in.DiasVacacionesPendientes = saldo
		if saldo.IsPositive() {
			semanas, err := entero(ClaveVentanaPromedioVacacionesSemana)
			if err != nil {
				return nil, err
			}
			desde := salida.AddDate(0, 0, -7*int(semanas)+1)
			in.BaseVacaciones, err = s.basePromedio(ctx, sol.EmpleadoID, desde, salida, planilla.BanderaBaseVacaciones,
				decimal.NewFromInt(semanas*12/52))
			if err != nil {
				return nil, err
			}
		}
	}

	if regla.PagaAguinaldo {
		v, err := s.ventanaQueContiene(ctx, planilla.ClaseAguinaldo, salida)
		if err != nil {
			return nil, err
		}
		meses, err := entero(planilla.ClaveAguinaldoMeses)
		if err != nil {
			return nil, err
		}
		p.AguinaldoMeses = decimal.NewFromInt(meses)
		in.AguinaldoDesde = v.CalculoDesde
		in.BaseAguinaldo, err = s.repo.SumarBaseAcumuladaEmpleado(ctx, sol.EmpleadoID, v.CalculoDesde, salida, planilla.BanderaBaseAguinaldo)
		if err != nil {
			return nil, fmt.Errorf("liquidacion: base de aguinaldo: %w", err)
		}
	}

	if regla.PagaSalarioEscolar {
		v, err := s.ventanaQueContiene(ctx, planilla.ClaseSalarioEscolar, salida)
		if err != nil {
			return nil, err
		}
		pct, err := s.repo.ObtenerPorcentajeSalarioEscolarEnFecha(ctx, salida)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: no hay parametros_salario_escolar vigente al %s", ErrParametroLiquidacion, salida.Format("2006-01-02"))
		}
		if err != nil {
			return nil, fmt.Errorf("liquidacion: leyendo parametros_salario_escolar: %w", err)
		}
		p.PorcentajeSalarioEscolar = pct
		in.SalarioEscolarDesde = v.CalculoDesde
		in.BaseSalarioEscolar, err = s.repo.SumarBaseAcumuladaEmpleado(ctx, sol.EmpleadoID, v.CalculoDesde, salida, planilla.BanderaBaseAguinaldo)
		if err != nil {
			return nil, fmt.Errorf("liquidacion: base de salario escolar: %w", err)
		}
	}

	rubros, err := liquidacion.CalcularRubros(in, p)
	if err != nil {
		return nil, err
	}

	ultimo, err := s.repo.UltimoFinPeriodoRegularEmpleado(ctx, sol.EmpleadoID)
	if err != nil {
		return nil, fmt.Errorf("liquidacion: último período aprobado: %w", err)
	}
	if !ultimo.Valid || ultimo.Time.Before(salida) {
		rubros.Advertencias = append(rubros.Advertencias,
			"no hay un período regular aprobado que llegue a la fecha de salida: las bases no incluyen el último salario")
	}
	return &calculoPreparado{entrada: in, rubros: rubros}, nil
}

// generalesDe: los parámetros generales como rigen para el empleado (con sus
// valores propios, si el lector los admite).
func (s *LiquidacionService) generalesDe(empleadoID int64) EnterosGenerales {
	if g, ok := s.generales.(*parametros.Generales); ok {
		return g.ParaEmpleado(empleadoID)
	}
	return s.generales
}

// cargarResponsabilidad: escalas del régimen, umbral y promedio de cesantía.
func (s *LiquidacionService) cargarResponsabilidad(ctx context.Context, empleadoID int64, salida time.Time,
	in *liquidacion.EntradaLiquidacion, p *liquidacion.ParametrosLiquidacion, entero func(string) (int64, error)) error {
	contrato, err := s.regimen.ObtenerContratoVigenteEnFecha(ctx, empleadoID, salida)
	if err != nil {
		return fmt.Errorf("%w: empleado_id=%d sin contrato vigente al %s: %v", ErrSinDatosLaborales, empleadoID, salida.Format("2006-01-02"), err)
	}
	filasCes, err := s.repo.ListarEscalaCesantiaEnFecha(ctx, contrato.RegimenLaboralID, salida)
	if err != nil {
		return fmt.Errorf("liquidacion: escala de cesantía (regimen_laboral_id=%d): %w", contrato.RegimenLaboralID, err)
	}
	filasPre, err := s.repo.ListarEscalaPreavisoEnFecha(ctx, contrato.RegimenLaboralID, salida)
	if err != nil {
		return fmt.Errorf("liquidacion: escala de preaviso (regimen_laboral_id=%d): %w", contrato.RegimenLaboralID, err)
	}
	p.EscalaCesantia = escalaCesantiaDesdeDTO(filasCes)
	p.EscalaPreaviso = escalaPreavisoDesdeDTO(filasPre)

	umbral, err := entero(ClaveLiqFraccionMesesUmbral)
	if err != nil {
		return err
	}
	p.FraccionMesesUmbral = int(umbral)

	if p.Motivo.PagaCesantia {
		metodo, err := s.generalesDe(empleadoID).Texto(ctx, liquidacion.ClaveMetodoCesantia, salida)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrParametroLiquidacion, liquidacion.ClaveMetodoCesantia, err)
		}
		if !liquidacion.MetodoCesantiaValido(metodo) {
			return fmt.Errorf("%w: %s = %q (%s | %s)", ErrParametroLiquidacion, liquidacion.ClaveMetodoCesantia, metodo,
				liquidacion.MetodoCesantiaAcumulado, liquidacion.MetodoCesantiaAntiguedadTotal)
		}
		p.MetodoCesantia = liquidacion.MetodoCesantia(metodo)
	}

	meses, err := entero(ClaveLiqPromedioMeses)
	if err != nil {
		return err
	}
	desde := salida.AddDate(0, -int(meses), 1)
	in.BaseCesantia, err = s.basePromedio(ctx, empleadoID, desde, salida, planilla.BanderaBaseCesantia, decimal.NewFromInt(meses))
	return err
}

func (s *LiquidacionService) basePromedio(ctx context.Context, empleadoID int64, desde, hasta time.Time, columna string, mesesVentana decimal.Decimal) (liquidacion.BasePromedio, error) {
	r, err := s.repo.SumarBasePromedioEmpleado(ctx, empleadoID, desde, hasta, columna)
	if err != nil {
		return liquidacion.BasePromedio{}, fmt.Errorf("liquidacion: sumando %s: %w", columna, err)
	}
	return liquidacion.BasePromedio{Total: r.Total, Meses: r.Meses, Periodos: int(r.Periodos), MesesVentana: mesesVentana}, nil
}

// ventanaQueContiene: ventana de cálculo de la clase (aguinaldo o salario
// escolar) que contiene la fecha, según parametros_generales.
func (s *LiquidacionService) ventanaQueContiene(ctx context.Context, clase planilla.ClaseTipoPlanilla, f time.Time) (planilla.VentanaAcumulado, error) {
	for _, anio := range []int{f.Year() - 1, f.Year(), f.Year() + 1} {
		v, err := s.ventanas.Ventana(ctx, clase, anio, f)
		if err != nil {
			return planilla.VentanaAcumulado{}, fmt.Errorf("%w: ventana de %s: %v", ErrParametroLiquidacion, clase, err)
		}
		if !f.Before(v.CalculoDesde) && !f.After(v.CalculoHasta) {
			return v, nil
		}
	}
	return planilla.VentanaAcumulado{}, fmt.Errorf("%w: ninguna ventana de %s contiene el %s", ErrParametroLiquidacion, clase, f.Format("2006-01-02"))
}

func escalaCesantiaDesdeDTO(filas []dto.EscalaCesantiaRow) []liquidacion.EscalonCesantia {
	out := make([]liquidacion.EscalonCesantia, 0, len(filas))
	for _, f := range filas {
		e := liquidacion.EscalonCesantia{AniosDesde: f.AniosServicioDesde, DiasPorAnio: f.DiasPorAnio}
		if f.AniosServicioHasta.Valid {
			v := f.AniosServicioHasta.Decimal
			e.AniosHasta = &v
		}
		if f.TopeAnios.Valid {
			v := f.TopeAnios.Decimal
			e.TopeAnios = &v
		}
		out = append(out, e)
	}
	return out
}

func escalaPreavisoDesdeDTO(filas []dto.EscalaPreavisoRow) []liquidacion.EscalonPreaviso {
	out := make([]liquidacion.EscalonPreaviso, 0, len(filas))
	for _, f := range filas {
		e := liquidacion.EscalonPreaviso{MesesDesde: int(f.AntiguedadMesesDesde), DiasPreaviso: int(f.DiasPreaviso)}
		if f.AntiguedadMesesHasta.Valid {
			v := int(f.AntiguedadMesesHasta.Int32)
			e.MesesHasta = &v
		}
		out = append(out, e)
	}
	return out
}

// ProyectarLiquidacion calcula sin escribir nada.
func (s *LiquidacionService) ProyectarLiquidacion(ctx context.Context, sol ProyectarSolicitud) (*liquidacion.RubrosLiquidacion, error) {
	c, err := s.prepararCalculo(ctx, sol)
	if err != nil {
		return nil, err
	}
	return &c.rubros, nil
}

// ProcesoLiquidacion: resultado de procesar.
type ProcesoLiquidacion struct {
	Rubros            liquidacion.RubrosLiquidacion `json:"resultado"`
	LiquidacionID     int64                         `json:"liquidacion_id"`
	PeriodoPlanillaID int64                         `json:"periodo_planilla_id"`
	// CierreProgramado: la salida es futura; contrato, datos laborales y
	// estado del empleado los cierra `cmd/jobs cerrar-salidas` ese día.
	CierreProgramado bool `json:"cierre_programado"`
}

// Errores de procesar (409).
var (
	ErrEmpleadoInactivo          = errors.New("liquidacion: el empleado no está activo")
	ErrLiquidacionYaProcesada    = errors.New("liquidacion: el empleado ya tiene una liquidación procesada desde su ingreso")
	ErrPeriodoLiquidacionCerrado = errors.New("liquidacion: el período de liquidaciones del mes ya está aprobado y no hay uno abierto para pagarla")
)

// ProcesarLiquidacion recalcula (no confía en una proyección previa) y, en
// una transacción auditada:
//  1. Persiste la liquidación ('procesada').
//  2. Si la salida es hoy o pasada: cierra contrato y datos laborales,
//     desactiva al empleado y registra cierre_aplicado_en. Si es futura, el
//     cierre queda programado para `cmd/jobs cerrar-salidas`.
//  3. Asegura el período mensual de clase 'liquidacion' que la pagará:
//     el del mes de la salida, o el del mes en curso si aquel ya se aprobó.
//     El motor la toma al calcular ese período (el hidratador la liga).
func (s *LiquidacionService) ProcesarLiquidacion(ctx context.Context, sol ProyectarSolicitud, usuarioID int64) (*ProcesoLiquidacion, error) {
	c, err := s.prepararCalculo(ctx, sol)
	if err != nil {
		return nil, err
	}
	hoy := soloFechaUTC(s.reloj())
	out := &ProcesoLiquidacion{Rubros: c.rubros}
	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		activo, err := q.EmpleadoActivo(ctx, sol.EmpleadoID)
		if err != nil {
			return err
		}
		if !activo {
			return ErrEmpleadoInactivo
		}
		ya, err := q.ExisteLiquidacionProcesadaDesde(ctx, sol.EmpleadoID, c.entrada.FechaIngreso)
		if err != nil {
			return err
		}
		if ya {
			return ErrLiquidacionYaProcesada
		}
		id, err := q.CrearLiquidacion(ctx, crearLiquidacionParams(sol, c, usuarioID))
		if err != nil {
			return fmt.Errorf("crear liquidacion: %w", err)
		}
		out.LiquidacionID = id
		if c.entrada.FechaSalida.After(hoy) {
			out.CierreProgramado = true
		} else if err := cerrarSalida(ctx, q, id, sol.EmpleadoID, c.entrada.FechaSalida); err != nil {
			return err
		}
		out.PeriodoPlanillaID, err = asegurarPeriodoLiquidacion(ctx, q, c.entrada.FechaSalida, hoy)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// cerrarSalida: cierra contrato y datos laborales a la fecha de salida,
// desactiva al empleado y registra el cierre (idempotente por liquidación).
func cerrarSalida(ctx context.Context, q *dto.Queries, liquidacionID, empleadoID int64, salida time.Time) error {
	if err := q.CerrarContratoVigente(ctx, empleadoID, salida); err != nil {
		return fmt.Errorf("cerrar contrato vigente: %w", err)
	}
	if err := q.CerrarDatosLaboralesVigente(ctx, empleadoID, salida); err != nil {
		return fmt.Errorf("cerrar datos laborales vigentes: %w", err)
	}
	if err := q.SetEmpleadoActivo(ctx, empleadoID, false); err != nil {
		return fmt.Errorf("desactivar expediente: %w", err)
	}
	if _, err := q.MarcarCierreLiquidacion(ctx, liquidacionID); err != nil {
		return fmt.Errorf("registrar cierre de la liquidación %d: %w", liquidacionID, err)
	}
	return nil
}

// asegurarPeriodoLiquidacion devuelve el período mensual de liquidaciones que
// pagará la salida, creándolo en borrador si no existe.
func asegurarPeriodoLiquidacion(ctx context.Context, q *dto.Queries, salida, hoy time.Time) (int64, error) {
	tipo, err := q.ObtenerTipoPlanillaPorClase(ctx, string(planilla.ClaseLiquidacion))
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%w: no hay tipo de planilla de clase liquidacion", ErrParametroLiquidacion)
	}
	if err != nil {
		return 0, err
	}
	if err := q.BloquearTipoPlanilla(ctx, tipo.ID); err != nil {
		return 0, err
	}
	meses := []time.Time{inicioMes(salida)}
	if inicioMes(hoy).After(meses[0]) {
		meses = append(meses, inicioMes(hoy))
	}
	for _, ini := range meses {
		fin := ini.AddDate(0, 1, -1)
		p, err := q.ObtenerPeriodoPorTipoFechas(ctx, tipo.ID, ini, fin)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			traslape, err := q.ExisteTraslapePeriodo(ctx, tipo.ID, ini, fin)
			if err != nil {
				return 0, err
			}
			if traslape {
				continue
			}
			return q.InsertarPeriodoPlanilla(ctx, tipo.ID, ini, fin)
		case err != nil:
			return 0, err
		case p.Estado == string(planilla.EstadoBorrador) || p.Estado == string(planilla.EstadoCalculada):
			return p.ID, nil
		}
	}
	return 0, ErrPeriodoLiquidacionCerrado
}

// ResumenCierres: resultado de CerrarSalidasPendientes.
type ResumenCierres struct {
	Cerradas int
	Errores  map[int64]string // liquidacion_id -> mensaje
}

// CerrarSalidasPendientes (cmd/jobs cerrar-salidas): cierra las salidas
// programadas con fecha <= hoy, una transacción auditada por liquidación.
// Idempotente: una liquidación ya cerrada no se vuelve a tocar.
func (s *LiquidacionService) CerrarSalidasPendientes(ctx context.Context, hoy time.Time, usuarioID int64) (ResumenCierres, error) {
	res := ResumenCierres{Errores: map[int64]string{}}
	pend, err := s.repoCierre().ListarLiquidacionesPorCerrar(ctx, soloFechaUTC(hoy))
	if err != nil {
		return res, fmt.Errorf("liquidacion: listando salidas por cerrar: %w", err)
	}
	for _, l := range pend {
		err := db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
			pendiente, err := q.ObtenerLiquidacionPorCerrarParaActualizar(ctx, l.ID)
			if err != nil || !pendiente {
				return err
			}
			if err := cerrarSalida(ctx, q, l.ID, l.EmpleadoID, soloFechaUTC(l.FechaSalida)); err != nil {
				return err
			}
			res.Cerradas++
			return nil
		})
		if err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			res.Errores[l.ID] = err.Error()
		}
	}
	return res, nil
}

func (s *LiquidacionService) repoCierre() *dto.Queries { return dto.New(s.sqlDB) }

func (s *LiquidacionService) reloj() time.Time {
	if s.ahora != nil {
		return s.ahora()
	}
	return time.Now()
}

func soloFechaUTC(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func inicioMes(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func crearLiquidacionParams(sol ProyectarSolicitud, c *calculoPreparado, usuarioID int64) dto.CrearLiquidacionParams {
	r := c.rubros
	escolarDesde := sql.NullTime{}
	if !c.entrada.SalarioEscolarDesde.IsZero() {
		escolarDesde = sql.NullTime{Time: c.entrada.SalarioEscolarDesde, Valid: true}
	}
	aguinaldoDesde := c.entrada.AguinaldoDesde
	if aguinaldoDesde.IsZero() {
		aguinaldoDesde = c.entrada.FechaSalida
	}
	return dto.CrearLiquidacionParams{
		EmpleadoID:                             sol.EmpleadoID,
		FechaSalida:                            c.entrada.FechaSalida,
		MotivoSalida:                           sol.MotivoSalida,
		AplicaResponsabilidadPatronal:          sol.AplicaResponsabilidadPatronal,
		FechaIngreso:                           c.entrada.FechaIngreso,
		AntiguedadMeses:                        int32(r.AntiguedadMeses),
		SalarioPromedio6Meses:                  r.SalarioPromedioMensual,
		SalarioDiarioPromedio:                  r.SalarioDiarioPromedio,
		DiasPreaviso:                           int32(r.DiasPreaviso),
		MontoPreaviso:                          r.MontoPreaviso,
		DiasCesantia:                           r.DiasCesantia,
		MontoCesantia:                          r.MontoCesantia,
		DiasVacacionesPendientes:               r.DiasVacacionesPendientes,
		MontoVacacionesProporcionales:          r.MontoVacacionesProporcionales,
		PeriodoAguinaldoDesde:                  aguinaldoDesde,
		TotalSalariosBrutosAguinaldoProporcion: r.BaseAguinaldoProporcional,
		MontoAguinaldoProporcional:             r.MontoAguinaldoProporcional,
		SalarioDiarioVacaciones:                r.SalarioDiarioVacaciones,
		PeriodoSalarioEscolarDesde:             escolarDesde,
		TotalBaseSalarioEscolarProporcional:    r.BaseSalarioEscolarProporcional,
		MontoSalarioEscolarProporcional:        r.MontoSalarioEscolarProporcional,
		TotalLiquidacion:                       r.TotalLiquidacion,
		ProcesadoPor:                           usuarioID,
	}
}
