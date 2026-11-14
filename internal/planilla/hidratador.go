package planilla

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/shared/porcentaje"
)

// Códigos del catálogo tipos_credito_fiscal_renta que se derivan de
// empleado_datos_familiares.
const (
	codigoCreditoConyuge = "CONYUGE"
	codigoCreditoHijo    = "HIJO"
)

// PeriodoParaCalculo: datos del período que necesita la hidratación. El
// orquestador llena DivisorDiasMes (DIVISOR_DIAS_MES) y DiasPeriodoCompleto
// (= DivisorDiasMes × FraccionMensual) con los parámetros ya cargados.
type PeriodoParaCalculo struct {
	ID                  int64
	Inicio, Fin         time.Time
	Clase               string
	TipoPlanillaID      int64
	FraccionMensual     decimal.Decimal
	DiasPeriodoCompleto decimal.Decimal
	DivisorDiasMes      int
	// LiqPrestamosModo: LIQ_PRESTAMOS_MODO vigente; "" = no configurado.
	LiqPrestamosModo string
}

// EntradaEmpleado: input del motor más los datos que el escritor de detalle
// necesita y que el motor no usa.
type EntradaEmpleado struct {
	Input                    CalculoEmpleadoInput
	ContratoID               int64
	ProgramaPresupuestarioID int64
}

// FuenteHidratacion: subconjunto de dto.Queries que usa el hidratador.
type FuenteHidratacion interface {
	ListarEmpleadosActivosEnRango(ctx context.Context, desde, hasta time.Time) ([]int64, error)
	ListarContratosEnRango(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]dto.ContratoEnRangoRow, error)
	ObtenerContextoLaboral(ctx context.Context, empleadoID int64, fecha time.Time) (dto.ContextoLaboralRow, error)
	SumarHorasExtraPeriodo(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]dto.HorasExtraPeriodoRow, error)
	SumarHorasFeriadoLaboradoPeriodo(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]dto.HorasFeriadoLaboradoRow, error)
	ListarIncapacidadesSolapadas(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]dto.IncapacidadSolapadaRow, error)
	ListarPrestamosActivosEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]dto.PrestamoActivoRow, error)
	ListarEmbargosActivosEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]dto.EmbargoActivoRow, error)
	ObtenerDatosFamiliaresEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (dto.DatosFamiliaresEnFechaRow, error)
	ListarCreditosFiscalesEnFecha(ctx context.Context, fecha time.Time) ([]dto.CreditoFiscalVigenteRow, error)
	ListarDevengosManualesEmpleado(ctx context.Context, periodoID, empleadoID int64) ([]dto.DevengoManualEmpleadoRow, error)
	SumarBaseAcumuladaEmpleado(ctx context.Context, empleadoID int64, desde, hasta time.Time, columna string) (decimal.Decimal, error)
	ListarCompensacionesVacacionesPendientes(ctx context.Context, empleadoID, periodoID int64, hasta time.Time) ([]dto.CompensacionVacacionesRow, error)
	ListarVacacionesAprobadasEnRango(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]dto.VacacionesAprobadasRow, error)
	ListarEmpleadosConLiquidacionParaPeriodo(ctx context.Context, periodoID int64, hasta time.Time) ([]int64, error)
	ListarLiquidacionesParaPeriodo(ctx context.Context, periodoID, empleadoID int64, hasta time.Time) ([]dto.LiquidacionParaPeriodoRow, error)
	SumarRentaMesEmpleado(ctx context.Context, empleadoID, excluirPeriodoID int64, desde, hasta time.Time) (dto.RentaMesRow, error)
	ListarAsignacionesVigentesEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]dto.AsignacionVigenteRow, error)
	ListarAsistenciaPresenteEnRango(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]dto.DiaAsistenciaRow, error)
	ListarFeriadosPagoObligatorioEnRango(ctx context.Context, desde, hasta time.Time) ([]time.Time, error)
	SumarEmbargosMesEmpleado(ctx context.Context, empleadoID, excluirPeriodoID int64, desde, hasta time.Time) (dto.EmbargosMesRow, error)
}

var _ FuenteHidratacion = (*dto.Queries)(nil)

// ErrEmpleadoSinContrato: el empleado entra en el período pero no tiene
// contrato vigente a la fecha efectiva (no se asume régimen ni salario).
type ErrEmpleadoSinContrato struct{ EmpleadoID int64 }

func (e ErrEmpleadoSinContrato) Error() string {
	return fmt.Sprintf("planilla: empleado_id=%d sin contrato vigente en el período", e.EmpleadoID)
}

// ErrEmpleadoSinDatosLaborales: no hay versión de empleado_datos_laborales
// vigente a la fecha efectiva (sin programa presupuestario no se puede
// imputar el gasto).
type ErrEmpleadoSinDatosLaborales struct{ EmpleadoID int64 }

func (e ErrEmpleadoSinDatosLaborales) Error() string {
	return fmt.Sprintf("planilla: empleado_id=%d sin datos laborales vigentes en el período", e.EmpleadoID)
}

// ErrEmpleadoSinRegimenCCSS: la versión de datos laborales vigente no tiene
// regimen_ccss_id: sin régimen no se resuelve la base mínima CCSS.
type ErrEmpleadoSinRegimenCCSS struct{ EmpleadoID int64 }

func (e ErrEmpleadoSinRegimenCCSS) Error() string {
	return fmt.Sprintf("planilla: empleado_id=%d sin regimen_ccss_id en sus datos laborales vigentes", e.EmpleadoID)
}

type Hidratador struct{ f FuenteHidratacion }

func NewHidratador(f FuenteHidratacion) *Hidratador { return &Hidratador{f: f} }

// EmpleadosDelPeriodo: empleados activos con algún contrato que se solape con
// el período, ordenados por id.
func (h *Hidratador) EmpleadosDelPeriodo(ctx context.Context, p PeriodoParaCalculo) ([]int64, error) {
	if ClaseTipoPlanilla(p.Clase) == ClaseLiquidacion {
		ids, err := h.f.ListarEmpleadosConLiquidacionParaPeriodo(ctx, p.ID, p.Fin)
		if err != nil {
			return nil, fmt.Errorf("planilla: error listando liquidaciones del período %d: %w", p.ID, err)
		}
		return ids, nil
	}
	ids, err := h.f.ListarEmpleadosActivosEnRango(ctx, p.Inicio, p.Fin)
	if err != nil {
		return nil, fmt.Errorf("planilla: error listando empleados del período %d: %w", p.ID, err)
	}
	return ids, nil
}

// Hidratar arma la entrada de un empleado. La fecha efectiva es el último día
// con contrato dentro del período (fin del período, o la fecha de cese): a esa
// fecha se resuelven contrato, datos laborales, préstamos, embargos y familia.
func (h *Hidratador) Hidratar(ctx context.Context, p PeriodoParaCalculo, empleadoID int64) (EntradaEmpleado, error) {
	switch ClaseTipoPlanilla(p.Clase) {
	case ClaseAguinaldo, ClaseSalarioEscolar:
		return h.hidratarAcumulado(ctx, p, empleadoID)
	case ClaseLiquidacion:
		return h.hidratarLiquidacion(ctx, p, empleadoID)
	}
	if !p.DiasPeriodoCompleto.IsPositive() || p.DivisorDiasMes <= 0 {
		return EntradaEmpleado{}, errors.New("planilla: DiasPeriodoCompleto/DivisorDiasMes del período no definidos para hidratar")
	}
	segs, err := h.segmentos(ctx, p, empleadoID)
	if err != nil {
		return EntradaEmpleado{}, err
	}
	if len(segs) == 0 {
		return EntradaEmpleado{}, ErrEmpleadoSinContrato{EmpleadoID: empleadoID}
	}
	ultimo := segs[len(segs)-1]
	fecha := ultimo.Fin

	lab, err := h.f.ObtenerContextoLaboral(ctx, empleadoID, fecha)
	if errors.Is(err, sql.ErrNoRows) {
		return EntradaEmpleado{}, ErrEmpleadoSinDatosLaborales{EmpleadoID: empleadoID}
	}
	if err != nil {
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d, datos laborales: %w", empleadoID, err)
	}

	in := CalculoEmpleadoInput{
		EmpleadoID:         empleadoID,
		PeriodoID:          p.ID,
		TipoPlanillaID:     p.TipoPlanillaID,
		FechaEfectiva:      fecha,
		FechaInicioPeriodo: p.Inicio,
		FechaFinPeriodo:    p.Fin,
		Segmentos:          segs,
		DiasTrabajados:     decimal.NewFromInt(int64(DiasSegmentos(segs))),
	}
	if lab.ClasificacionPuestoID != nil {
		in.ClasificacionPuestoID = *lab.ClasificacionPuestoID
	}
	if lab.RegimenCCSSID == nil {
		return EntradaEmpleado{}, ErrEmpleadoSinRegimenCCSS{EmpleadoID: empleadoID}
	}
	in.RegimenCCSSID = *lab.RegimenCCSSID

	if err := h.hidratarAsistencia(ctx, p, empleadoID, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarDeducciones(ctx, empleadoID, fecha, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarVacaciones(ctx, p, empleadoID, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarCreditos(ctx, empleadoID, fecha, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarRentaMes(ctx, p, empleadoID, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarAsignaciones(ctx, empleadoID, fecha, &in); err != nil {
		return EntradaEmpleado{}, err
	}

	devs, err := h.f.ListarDevengosManualesEmpleado(ctx, p.ID, empleadoID)
	if err != nil {
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d, devengos manuales: %w", empleadoID, err)
	}
	for _, d := range devs {
		dev := ConceptoDevengoManualInput{ConceptoSalarialID: d.ConceptoSalarialID, Monto: d.Monto}
		if d.Observacion != nil {
			dev.Descripcion = *d.Observacion
		}
		in.DevengosManuales = append(in.DevengosManuales, dev)
	}

	return EntradaEmpleado{
		Input:                    in,
		ContratoID:               ultimo.ContratoID,
		ProgramaPresupuestarioID: lab.ProgramaPresupuestarioID,
	}, nil
}

// hidratarAcumulado: en un período de aguinaldo o salario escolar el
// período es la ventana de cálculo. Solo se resuelven el contrato (último día
// con contrato en la ventana), el programa presupuestario y la base acumulada
// (Σ base_aguinaldo de los períodos regulares aprobados de la ventana). El
// salario escolar usa la misma base que el aguinaldo.
func (h *Hidratador) hidratarAcumulado(ctx context.Context, p PeriodoParaCalculo, empleadoID int64) (EntradaEmpleado, error) {
	if p.DivisorDiasMes <= 0 {
		return EntradaEmpleado{}, errors.New("planilla: DivisorDiasMes del período no definido para hidratar")
	}
	segs, err := h.segmentos(ctx, p, empleadoID)
	if err != nil {
		return EntradaEmpleado{}, err
	}
	if len(segs) == 0 {
		return EntradaEmpleado{}, ErrEmpleadoSinContrato{EmpleadoID: empleadoID}
	}
	ultimo := segs[len(segs)-1]
	lab, err := h.f.ObtenerContextoLaboral(ctx, empleadoID, ultimo.Fin)
	if errors.Is(err, sql.ErrNoRows) {
		return EntradaEmpleado{}, ErrEmpleadoSinDatosLaborales{EmpleadoID: empleadoID}
	}
	if err != nil {
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d, datos laborales: %w", empleadoID, err)
	}
	base, err := sumarBaseAcumulada(ctx, h.f, empleadoID, p.Inicio, p.Fin, BanderaBaseAguinaldo)
	if err != nil {
		return EntradaEmpleado{}, err
	}
	in := CalculoEmpleadoInput{
		EmpleadoID: empleadoID, PeriodoID: p.ID, TipoPlanillaID: p.TipoPlanillaID,
		FechaEfectiva: ultimo.Fin, FechaInicioPeriodo: p.Inicio, FechaFinPeriodo: p.Fin,
	}
	if ClaseTipoPlanilla(p.Clase) == ClaseAguinaldo {
		in.BaseAguinaldoAcumulada = base
	} else {
		in.BaseSalarioEscolarAcumulada = base
	}
	return EntradaEmpleado{Input: in, ContratoID: ultimo.ContratoID, ProgramaPresupuestarioID: lab.ProgramaPresupuestarioID}, nil
}

// Modos de LIQ_PRESTAMOS_MODO (configurable; no sembrado): qué se
// retiene de los préstamos activos en la planilla de liquidación.
const (
	LiqPrestamosCuota   = "cuota"   // la cuota periódica, como en cualquier período
	LiqPrestamosSaldo   = "saldo"   // todo el saldo pendiente
	LiqPrestamosNinguno = "ninguno" // no se retiene nada en la liquidación
)

// ClaveLiqPrestamosModo: parametros_generales (texto).
const ClaveLiqPrestamosModo = "LIQ_PRESTAMOS_MODO"

// ModoPrestamosLiquidacionValido: uno de cuota | saldo | ninguno.
func ModoPrestamosLiquidacionValido(m string) bool {
	return m == LiqPrestamosCuota || m == LiqPrestamosSaldo || m == LiqPrestamosNinguno
}

// hidratarLiquidacion: la liquidación 'procesada' del empleado que paga
// el período. La fecha efectiva es la fecha de salida: a ella se resuelven
// contrato (régimen y conceptos), datos laborales, embargos y préstamos.
// No se aplican créditos fiscales de renta (ya los aplicó la planilla
// regular del mes) y el régimen CCSS no se exige porque la
// liquidación cotiza sin base mínima (SinBaseMinimaCCSS).
func (h *Hidratador) hidratarLiquidacion(ctx context.Context, p PeriodoParaCalculo, empleadoID int64) (EntradaEmpleado, error) {
	liqs, err := h.f.ListarLiquidacionesParaPeriodo(ctx, p.ID, empleadoID, p.Fin)
	if err != nil {
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d, liquidaciones: %w", empleadoID, err)
	}
	switch len(liqs) {
	case 0:
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d sin liquidación procesada que pagar en el período", empleadoID)
	case 1:
	default:
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d tiene %d liquidaciones pendientes de pago; revise antes de calcular", empleadoID, len(liqs))
	}
	l := liqs[0]
	fecha := soloFecha(l.FechaSalida)

	filas, err := h.f.ListarContratosEnRango(ctx, empleadoID, fecha, fecha)
	if err != nil {
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d, contrato a la salida: %w", empleadoID, err)
	}
	var contratoID int64
	var desde time.Time
	for _, c := range filas {
		if contratoID == 0 || c.FechaInicioVigencia.After(desde) {
			contratoID, desde = c.ContratoID, c.FechaInicioVigencia
		}
	}
	if contratoID == 0 {
		return EntradaEmpleado{}, ErrEmpleadoSinContrato{EmpleadoID: empleadoID}
	}
	lab, err := h.f.ObtenerContextoLaboral(ctx, empleadoID, fecha)
	if errors.Is(err, sql.ErrNoRows) {
		return EntradaEmpleado{}, ErrEmpleadoSinDatosLaborales{EmpleadoID: empleadoID}
	}
	if err != nil {
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d, datos laborales: %w", empleadoID, err)
	}

	in := CalculoEmpleadoInput{
		EmpleadoID: empleadoID, PeriodoID: p.ID, TipoPlanillaID: p.TipoPlanillaID,
		FechaEfectiva: fecha, FechaInicioPeriodo: p.Inicio, FechaFinPeriodo: p.Fin,
		Liquidacion: &LiquidacionEmpleadoInput{LiquidacionID: l.ID, FechaSalida: fecha, Rubros: map[string]decimal.Decimal{
			"preaviso": l.MontoPreaviso, "cesantia": l.MontoCesantia, "vacaciones": l.MontoVacaciones,
			"aguinaldo": l.MontoAguinaldo, "salario_escolar": l.MontoSalarioEscolar,
		}},
	}
	if lab.ClasificacionPuestoID != nil {
		in.ClasificacionPuestoID = *lab.ClasificacionPuestoID
	}
	if lab.RegimenCCSSID != nil {
		in.RegimenCCSSID = *lab.RegimenCCSSID
	}
	if err := h.hidratarDeducciones(ctx, empleadoID, fecha, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarCreditos(ctx, empleadoID, fecha, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarRentaMes(ctx, p, empleadoID, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarEmbargosMes(ctx, p, empleadoID, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if len(in.Prestamos) > 0 {
		switch p.LiqPrestamosModo {
		case LiqPrestamosCuota:
		case LiqPrestamosSaldo:
			for i := range in.Prestamos {
				in.Prestamos[i].CuotaPeriodica = in.Prestamos[i].SaldoPendiente
			}
		case LiqPrestamosNinguno:
			in.Prestamos = nil
		case "":
			return EntradaEmpleado{}, fmt.Errorf("%w: %s no está configurado y el empleado_id=%d tiene préstamos activos",
				ErrParametroFaltante, ClaveLiqPrestamosModo, empleadoID)
		default:
			return EntradaEmpleado{}, fmt.Errorf("planilla: %s = %q inválido (cuota | saldo | ninguno)", ClaveLiqPrestamosModo, p.LiqPrestamosModo)
		}
	}
	return EntradaEmpleado{Input: in, ContratoID: contratoID, ProgramaPresupuestarioID: lab.ProgramaPresupuestarioID}, nil
}

// hidratarAsignaciones: conceptos recurrentes del empleado vigentes a la
// fecha; el porcentaje pasa a fracción aquí.
func (h *Hidratador) hidratarAsignaciones(ctx context.Context, empleadoID int64, fecha time.Time, in *CalculoEmpleadoInput) error {
	filas, err := h.f.ListarAsignacionesVigentesEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, conceptos asignados: %w", empleadoID, err)
	}
	for _, a := range filas {
		valor := a.Valor
		if a.Modo == AsignacionPorcentaje {
			pct, err := porcentaje.DesdePorcentaje(a.Valor)
			if err != nil {
				return fmt.Errorf("planilla: empleado_id=%d, asignación %d: %w", empleadoID, a.ID, err)
			}
			valor = pct.Fraccion()
		}
		in.Asignaciones = append(in.Asignaciones, AsignacionInput{AsignacionID: a.ID, ConceptoSalarialID: a.ConceptoSalarialID, Modo: a.Modo, Valor: valor})
	}
	return nil
}

// mesDelPeriodo: primer y último día del mes calendario de fecha_fin.
func mesDelPeriodo(p PeriodoParaCalculo) (time.Time, time.Time) {
	fin := soloFecha(p.Fin)
	desde := time.Date(fin.Year(), fin.Month(), 1, 0, 0, 0, 0, time.UTC)
	return desde, desde.AddDate(0, 1, -1)
}

// hidratarEmbargosMes: líquido embargable y embargos ya retenidos en los
// otros períodos aprobados del mes calendario de fecha_fin (solo si el
// empleado tiene embargos activos).
func (h *Hidratador) hidratarEmbargosMes(ctx context.Context, p PeriodoParaCalculo, empleadoID int64, in *CalculoEmpleadoInput) error {
	if len(in.Embargos) == 0 {
		return nil
	}
	desde, hasta := mesDelPeriodo(p)
	r, err := h.f.SumarEmbargosMesEmpleado(ctx, empleadoID, p.ID, desde, hasta)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, embargos del mes: %w", empleadoID, err)
	}
	prev := &EmbargoMesPrevio{Base: r.BaseEmbargo, PorEmbargo: make(map[int64]EmbargoPrevio, len(r.PorEmbargo))}
	for _, e := range r.PorEmbargo {
		prev.PorEmbargo[e.EmbargoID] = EmbargoPrevio{Calculado: e.Calculado, Aplicado: e.Aplicado}
	}
	in.EmbargoMesPrevio = prev
	return nil
}

// hidratarRentaMes: renta ya gravada y retenida en otros períodos aprobados
// del mes calendario de fecha_fin del período.
func (h *Hidratador) hidratarRentaMes(ctx context.Context, p PeriodoParaCalculo, empleadoID int64, in *CalculoEmpleadoInput) error {
	desde, hasta := mesDelPeriodo(p)
	r, err := h.f.SumarRentaMesEmpleado(ctx, empleadoID, p.ID, desde, hasta)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, renta del mes: %w", empleadoID, err)
	}
	in.RentaMesPrevia = RentaAcumuladaMes{Base: r.BaseRenta, Bruta: r.RentaBruta, Creditos: r.Creditos, Neta: r.RentaNeta}
	return nil
}

// segmentos: tramos del período con contrato. Cada versión de contrato vale
// en la intersección de su vigencia con [fecha_inicio_contrato,
// fecha_fin_contrato] (así un cese o un ingreso a mitad de período parten el
// salario).
func (h *Hidratador) segmentos(ctx context.Context, p PeriodoParaCalculo, empleadoID int64) ([]SegmentoSalario, error) {
	filas, err := h.f.ListarContratosEnRango(ctx, empleadoID, p.Inicio, p.Fin)
	if err != nil {
		return nil, fmt.Errorf("planilla: empleado_id=%d, contratos del período: %w", empleadoID, err)
	}
	vig := make([]VigenciaContrato, 0, len(filas))
	for _, c := range filas {
		v := VigenciaContrato{ContratoID: c.ContratoID, Salario: c.SalarioPactado,
			Modalidad: c.ModalidadSalario, JornadaID: c.JornadaID,
			Desde: maxFecha(c.FechaInicioVigencia, c.FechaInicioContrato)}
		switch {
		case c.FechaFinVigencia != nil && c.FechaFinContrato != nil:
			v.Hasta = minFecha(*c.FechaFinVigencia, *c.FechaFinContrato)
		case c.FechaFinVigencia != nil:
			v.Hasta = *c.FechaFinVigencia
		case c.FechaFinContrato != nil:
			v.Hasta = *c.FechaFinContrato
		}
		vig = append(vig, v)
	}
	segs, err := SegmentarPeriodo(p.Inicio, p.Fin, vig, p.DivisorDiasMes)
	if err != nil {
		return nil, fmt.Errorf("planilla: empleado_id=%d: %w", empleadoID, err)
	}
	return segs, nil
}

func (h *Hidratador) hidratarAsistencia(ctx context.Context, p PeriodoParaCalculo, empleadoID int64, in *CalculoEmpleadoInput) error {
	horas, err := h.f.SumarHorasExtraPeriodo(ctx, empleadoID, p.Inicio, p.Fin)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, horas extra: %w", empleadoID, err)
	}
	for _, x := range horas {
		in.HorasExtra = append(in.HorasExtra, HoraExtraInput{TipoHoraExtraID: x.TipoHoraExtraID, CantidadHoras: x.Horas})
	}

	feriados, err := h.f.SumarHorasFeriadoLaboradoPeriodo(ctx, empleadoID, p.Inicio, p.Fin)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, feriados laborados: %w", empleadoID, err)
	}
	for _, x := range feriados {
		in.FeriadosLaborados = append(in.FeriadosLaborados, FeriadoLaboradoInput{PagoObligatorio: x.PagoObligatorio, Horas: x.Horas})
	}

	incs, err := h.f.ListarIncapacidadesSolapadas(ctx, empleadoID, p.Inicio, p.Fin)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, incapacidades: %w", empleadoID, err)
	}
	for _, x := range incs {
		desde, hasta := maxFecha(soloFecha(x.FechaInicio), p.Inicio), minFecha(soloFecha(x.FechaFin), p.Fin)
		in.Incapacidades = append(in.Incapacidades, IncapacidadInput{
			IncapacidadID:    x.IncapacidadID,
			EntidadEmisoraID: x.EntidadEmisoraID,
			Desde:            desde,
			Hasta:            hasta,
			DiaSerieDesde:    int32(diasEntre(x.FechaInicio, desde)) + 1,
		})
	}
	netos, err := SegmentosNetos(in.Segmentos, in.Incapacidades, p.DivisorDiasMes)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d: %w", empleadoID, err)
	}
	in.DiasTrabajados = decimal.NewFromInt(int64(DiasSegmentos(netos)))
	return h.hidratarDiasPagables(ctx, p, empleadoID, in)
}

// hidratarDiasPagables: asistencia registrada y feriados de pago obligatorio
// del período, solo si algún segmento se paga por día u hora.
func (h *Hidratador) hidratarDiasPagables(ctx context.Context, p PeriodoParaCalculo, empleadoID int64, in *CalculoEmpleadoInput) error {
	necesita := false
	for _, s := range in.Segmentos {
		if s.Modalidad == ModalidadDiario || s.Modalidad == ModalidadPorHora {
			necesita = true
			break
		}
	}
	if !necesita {
		return nil
	}
	dias, err := h.f.ListarAsistenciaPresenteEnRango(ctx, empleadoID, p.Inicio, p.Fin)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, asistencia: %w", empleadoID, err)
	}
	for _, d := range dias {
		in.Asistencia = append(in.Asistencia, DiaAsistencia{Fecha: soloFecha(d.Fecha), Horas: d.Horas})
	}
	feriados, err := h.f.ListarFeriadosPagoObligatorioEnRango(ctx, p.Inicio, p.Fin)
	if err != nil {
		return fmt.Errorf("planilla: feriados de pago obligatorio: %w", err)
	}
	for _, f := range feriados {
		in.FeriadosPagoObligatorio = append(in.FeriadosPagoObligatorio, soloFecha(f))
	}
	return nil
}

func (h *Hidratador) hidratarDeducciones(ctx context.Context, empleadoID int64, fecha time.Time, in *CalculoEmpleadoInput) error {
	prestamos, err := h.f.ListarPrestamosActivosEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, préstamos: %w", empleadoID, err)
	}
	for _, x := range prestamos {
		in.Prestamos = append(in.Prestamos, PrestamoDeduccionInput{
			PrestamoID: x.PrestamoID, CuotaPeriodica: x.CuotaPeriodica, SaldoPendiente: x.SaldoPendiente,
		})
	}

	embargos, err := h.f.ListarEmbargosActivosEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, embargos: %w", empleadoID, err)
	}
	for _, x := range embargos {
		e := EmbargoInput{EmbargoID: x.EmbargoID, TipoEmbargoID: x.TipoEmbargoID,
			Prioridad: x.PrioridadAplicacion, PrioridadLegal: x.PrioridadLegal, EsPension: x.EsPension}
		if x.TopePorcentajeLiquido.Valid {
			tope, err := porcentaje.DesdePorcentaje(x.TopePorcentajeLiquido.Decimal)
			if err != nil {
				return fmt.Errorf("planilla: tope del tipo de embargo %d: %w", x.TipoEmbargoID, err)
			}
			e.TopePctLiquido = tope.Fraccion()
		}
		if x.TopeMensual.Valid {
			e.TopeMensual = x.TopeMensual.Decimal
		}
		if x.Porcentaje.Valid {
			pct, err := porcentaje.DesdePorcentaje(x.Porcentaje.Decimal)
			if err != nil {
				return fmt.Errorf("planilla: embargo_id=%d: %w", x.EmbargoID, err)
			}
			e.Porcentaje = pct.Fraccion()
		}
		if x.MontoFijo.Valid {
			e.MontoFijo = x.MontoFijo.Decimal
		}
		if e.Porcentaje.IsZero() && e.MontoFijo.IsZero() {
			return fmt.Errorf("planilla: embargo_id=%d sin porcentaje ni monto fijo", x.EmbargoID)
		}
		in.Embargos = append(in.Embargos, e)
	}
	return nil
}

// hidratarVacaciones: compensaciones pendientes hasta el fin del período
// y vacaciones aprobadas que tocan el período (recortadas a él).
func (h *Hidratador) hidratarVacaciones(ctx context.Context, p PeriodoParaCalculo, empleadoID int64, in *CalculoEmpleadoInput) error {
	comps, err := h.f.ListarCompensacionesVacacionesPendientes(ctx, empleadoID, p.ID, p.Fin)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, compensaciones de vacaciones: %w", empleadoID, err)
	}
	for _, c := range comps {
		in.VacacionesCompensacion = append(in.VacacionesCompensacion, CompensacionVacacionesInput{MovimientoID: c.MovimientoID, Dias: c.Dias})
	}
	sols, err := h.f.ListarVacacionesAprobadasEnRango(ctx, empleadoID, p.Inicio, p.Fin)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, vacaciones aprobadas: %w", empleadoID, err)
	}
	for _, s := range sols {
		in.VacacionesDisfrute = append(in.VacacionesDisfrute, RangoVacaciones{SolicitudID: s.SolicitudID,
			Desde: maxFecha(soloFecha(s.FechaInicio), p.Inicio), Hasta: minFecha(soloFecha(s.FechaFin), p.Fin)})
	}
	return nil
}

// hidratarCreditos traduce la declaración familiar vigente a créditos
// fiscales por tipo. Sin declaración vigente = sin créditos (no es error).
func (h *Hidratador) hidratarCreditos(ctx context.Context, empleadoID int64, fecha time.Time, in *CalculoEmpleadoInput) error {
	fam, err := h.f.ObtenerDatosFamiliaresEnFecha(ctx, empleadoID, fecha)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, datos familiares: %w", empleadoID, err)
	}
	if !fam.AplicaCreditoConyuge && fam.NumeroHijosDependientes <= 0 {
		return nil
	}

	tipos, err := h.f.ListarCreditosFiscalesEnFecha(ctx, fecha)
	if err != nil {
		return fmt.Errorf("planilla: error leyendo créditos fiscales vigentes: %w", err)
	}
	idPorCodigo := make(map[string]int64, len(tipos))
	for _, t := range tipos {
		idPorCodigo[t.TipoCodigo] = t.TipoCreditoFiscalID
	}
	agregar := func(codigo string, cantidad int32) error {
		id, ok := idPorCodigo[codigo]
		if !ok {
			return fmt.Errorf("planilla: empleado_id=%d declara crédito %s pero no hay parámetro de crédito fiscal vigente al %s",
				empleadoID, codigo, fecha.Format("2006-01-02"))
		}
		in.CreditosFiscales = append(in.CreditosFiscales, CreditoFiscalInput{TipoCreditoFiscalID: id, Cantidad: cantidad})
		return nil
	}
	if fam.AplicaCreditoConyuge {
		if err := agregar(codigoCreditoConyuge, 1); err != nil {
			return err
		}
	}
	if fam.NumeroHijosDependientes > 0 {
		if err := agregar(codigoCreditoHijo, fam.NumeroHijosDependientes); err != nil {
			return err
		}
	}
	return nil
}

// diasEntre: días naturales de a a b (b - a), comparando solo la fecha.
func diasEntre(a, b time.Time) int64 {
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int64(db.Sub(da).Hours() / 24)
}

func maxFecha(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minFecha(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
