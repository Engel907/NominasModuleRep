package planilla

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// ContratoVigente: subconjunto de contratos que necesita el motor.
type ContratoVigente struct {
	ContratoID       int64
	EmpleadoID       int64
	RegimenLaboralID int64
	SalarioPactado   decimal.Decimal
	ModalidadSalario string // mensual | quincenal | semanal: período que remunera SalarioPactado
	Jornada          string
	JornadaID        *int64 // el divisor de la hora extra depende de la jornada
}

// ConceptoAplicableFila es la fila cruda (aún sin interpretar la fórmula).
type ConceptoAplicableFila struct {
	ConceptoOrdenCalculo // ConceptoSalarialID, Codigo, OrdenCalculo, MontoFijo, AfectaBase*
	Naturaleza           string
	TipoCalculo          string
	FormulaID            *int64
	ExpresionFormula     *string
	FormulaEmpleadoID    *int64 // fórmula propia del empleado (reemplaza a la general)
}

// MotorRepository: lecturas de referencia del orquestador. Sin escrituras.
type MotorRepository interface {
	ObtenerContratoVigenteEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (ContratoVigente, error)
	// Con la fórmula propia del empleado donde la tenga.
	ListarConceptosAplicablesEnFecha(ctx context.Context, empleadoID, regimenLaboralID, tipoPlanillaID int64, fecha time.Time) ([]ConceptoAplicableFila, error)
	ListarParametrosHorasExtraEnFecha(ctx context.Context, fecha time.Time) ([]ParametroHoraExtra, error)
}

const (
	naturalezaIngreso   = "ingreso"
	naturalezaDeduccion = "deduccion"

	calculoFijo          = "fijo"
	calculoFormula       = "formula"
	calculoCapturaManual = "captura_manual"
)

// ConceptoAplicable: concepto ya resuelto y validado para (régimen, fecha).
type ConceptoAplicable struct {
	ConceptoOrdenCalculo
	Naturaleza  string
	TipoCalculo string
	FormulaID   int64 // 0 si no tiene versión de fórmula (solo captura_manual)
	// FormulaEmpleadoID: empleado_concepto_formula usada en lugar de la
	// general (FormulaID queda en 0).
	FormulaEmpleadoID int64
	TipoFormula       string // expresion_formula.tipo ("" si no aplica)
	Lado              string // expresion_formula.lado ("" si no aplica)
	Rubro             string // expresion_formula.rubro (solo tipo liquidacion)
	Base              string // expresion_formula.base (solo tipo asignacion_empleado)
}

// MotorEmpleado orquesta el cálculo de un empleado.
type MotorEmpleado struct {
	repo   MotorRepository
	reglas map[string]Regla // clave = expresion_formula.tipo
}

func NewMotorEmpleado(repo MotorRepository) *MotorEmpleado {
	return &MotorEmpleado{repo: repo, reglas: reglasPorDefecto()}
}

// evaluarConcepto resuelve qué conceptos aplican a un empleado en una fecha:
func (m *MotorEmpleado) evaluarConcepto(ctx context.Context, empleadoID, tipoPlanillaID int64, fecha time.Time) (ContratoVigente, []ConceptoAplicable, error) {
	contrato, err := m.repo.ObtenerContratoVigenteEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return ContratoVigente{}, nil, fmt.Errorf("planilla: empleado_id=%d sin contrato vigente al %s (no se asume régimen): %w",
			empleadoID, fecha.Format("2006-01-02"), err)
	}

	filas, err := m.repo.ListarConceptosAplicablesEnFecha(ctx, empleadoID, contrato.RegimenLaboralID, tipoPlanillaID, fecha)
	if err != nil {
		return ContratoVigente{}, nil, fmt.Errorf("planilla: error resolviendo conceptos aplicables (regimen_laboral_id=%d): %w", contrato.RegimenLaboralID, err)
	}
	if len(filas) == 0 {
		return ContratoVigente{}, nil, fmt.Errorf("planilla: el régimen laboral id=%d no tiene conceptos aplicables vigentes al %s para el tipo de planilla id=%d (¿falta regimen_concepto_aplicable, orden_calculo_concepto o tipo_planilla_concepto?)",
			contrato.RegimenLaboralID, fecha.Format("2006-01-02"), tipoPlanillaID)
	}

	vistos := make(map[int64]bool, len(filas))
	out := make([]ConceptoAplicable, 0, len(filas))
	for _, f := range filas {
		if vistos[f.ConceptoSalarialID] {
			return ContratoVigente{}, nil, fmt.Errorf("planilla: concepto %q aparece duplicado (vigencias solapadas en aplicabilidad/orden/fórmula)", f.Codigo)
		}
		vistos[f.ConceptoSalarialID] = true

		c := ConceptoAplicable{
			ConceptoOrdenCalculo: f.ConceptoOrdenCalculo,
			Naturaleza:           f.Naturaleza,
			TipoCalculo:          f.TipoCalculo,
		}
		if c.Naturaleza != naturalezaIngreso && c.Naturaleza != naturalezaDeduccion {
			return ContratoVigente{}, nil, fmt.Errorf("planilla: concepto %q tiene naturaleza desconocida %q", c.Codigo, c.Naturaleza)
		}
		if f.FormulaID != nil {
			c.FormulaID = *f.FormulaID
		}
		if f.FormulaEmpleadoID != nil {
			c.FormulaID, c.FormulaEmpleadoID = 0, *f.FormulaEmpleadoID
		}
		tieneFormula := c.FormulaID != 0 || c.FormulaEmpleadoID != 0

		switch c.TipoCalculo {
		case calculoCapturaManual:
		case calculoFijo:
			if !tieneFormula || f.MontoFijo == nil {
				return ContratoVigente{}, nil, fmt.Errorf("planilla: concepto fijo %q sin monto_fijo vigente al %s", c.Codigo, fecha.Format("2006-01-02"))
			}
		case calculoFormula:
			if !tieneFormula || f.ExpresionFormula == nil {
				return ContratoVigente{}, nil, fmt.Errorf("planilla: concepto %q (formula) sin expresion_formula vigente al %s", c.Codigo, fecha.Format("2006-01-02"))
			}
			e, err := ParsearExpresionFormula(*f.ExpresionFormula)
			if err != nil {
				return ContratoVigente{}, nil, fmt.Errorf("planilla: concepto %q: %w", c.Codigo, err)
			}
			c.TipoFormula, c.Lado, c.Rubro, c.Base = e.Tipo, e.Lado, e.Rubro, e.Base
		default:
			return ContratoVigente{}, nil, fmt.Errorf("planilla: concepto %q tiene tipo_calculo desconocido %q", c.Codigo, c.TipoCalculo)
		}
		out = append(out, c)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].OrdenCalculo != out[j].OrdenCalculo {
			return out[i].OrdenCalculo < out[j].OrdenCalculo
		}
		return out[i].ConceptoSalarialID < out[j].ConceptoSalarialID
	})
	return contrato, out, nil
}

// ParametrosCalculo: parámetros vigentes ya resueltos por el llamador para
// la FechaEfectiva. Los campos se validan solo cuando el flujo los usa.
type ParametrosCalculo struct {
	// CargasSociales: concepto_salarial_id -> regimen_ccss_id (0 = general)
	// -> tasa en fracción (0.1083, no 10.83).
	CargasSociales map[int64]map[int64]ParametroCargaSocial

	TramosRenta      []TramoRenta // Porcentaje como fracción (0.10), no 10.00
	CreditosFiscales map[int64]CreditoFiscalParametro

	SalarioMinimoInembargable decimal.Decimal
	FactoresHoraExtra         map[int64]ParametroHoraExtra
	SubsidiosIncapacidad      []TramoSubsidio
	BaseMinimaContributiva    map[int64]decimal.Decimal
	EscalaEmbargo             []TramoEscalaEmbargo

	// Divisor de días del mes (parametros_generales.DIVISOR_DIAS_MES): base
	// de los días comerciales y del salario diario.
	DivisorDiasMes decimal.Decimal

	// Divisor de la hora ordinaria por jornada (parametros_jornada, con los
	// divisores propios del empleado ya aplicados): jornada_id -> horas
	// mensuales ordinarias.
	DivisorPorJornada map[int64]decimal.Decimal
	// FactoresHoraExtraEmpleado: factores propios del empleado
	// (empleado_parametros_horas_extra); prevalecen sobre los generales.
	FactoresHoraExtraEmpleado map[int64]ParametroHoraExtra `json:"-"`

	// Fracción del mes que cubre el tipo de planilla (mes 1, quincena 0.5) y
	// días comerciales del período completo (= DivisorDiasMes × fracción).
	// SAL_BASE se calcula por segmentos; DiasPeriodoCompleto acota
	// DiasTrabajados.
	FraccionMensualPeriodo decimal.Decimal
	DiasPeriodoCompleto    decimal.Decimal

	// Acumulados (CargarAcumulado): solo en períodos de su clase.
	AguinaldoMeses           decimal.Decimal // parametros_generales.AGUINALDO_MESES
	PorcentajeSalarioEscolar decimal.Decimal // parametros_salario_escolar, fracción

	// VACACIONES_MODO_PAGO vigente; "" = no configurado.
	VacacionesModoPago string

	// Salario con que se pagan vacaciones e incapacidad (salario_promedio.go):
	// vigente | promedio | mayor; "" = no configurado. Las ventanas en
	// semanas / meses; 0 = no configurada.
	VacacionesSalarioBase     string
	VacacionesPromedioSemanas int
	IncapacidadSalarioBase    string
	IncapacidadPromedioMeses  int

	// Factores del feriado laborado; nil = no configurado.
	FactorFeriadoObligatorio   *decimal.Decimal
	FactorFeriadoNoObligatorio *decimal.Decimal

	// RENTA_METODO_PERIODO vigente: cómo se retiene la renta en un período
	// que cubre menos de un mes; "" = período mensual o de liquidación.
	RentaMetodoPeriodo string

	// SinBaseMinimaCCSS: la planilla de liquidación cotiza sobre la base
	// real de sus rubros, sin base mínima contributiva (la mínima del mes la
	// cubre la planilla regular).
	SinBaseMinimaCCSS bool
	// LIQ_PRESTAMOS_MODO vigente; "" = no configurado.
	LiqPrestamosModo string
	// LIQ_EMBARGO_MONTO_FIJO vigente; "" = no configurado.
	LiqEmbargoMontoFijo string
}

// TasaCarga: la tasa del concepto para el régimen CCSS del empleado; si el
// régimen no tiene fila propia, la general del concepto.
func (p ParametrosCalculo) TasaCarga(conceptoID, regimenCCSSID int64) (ParametroCargaSocial, bool) {
	porRegimen := p.CargasSociales[conceptoID]
	if t, ok := porRegimen[regimenCCSSID]; ok && regimenCCSSID != 0 {
		return t, true
	}
	t, ok := porRegimen[0]
	return t, ok
}

// CalculoEmpleadoDetalle: resultado + trazabilidad de fórmula por concepto
// (para planilla_detalle_concepto.concepto_salarial_formula_id).
type CalculoEmpleadoDetalle struct {
	ResultadoEmpleadoOutput
	FormulaPorConcepto map[int64]int64 // concepto_salarial_id -> concepto_salarial_formula_id
	// concepto_salarial_id -> empleado_concepto_formula_id (fórmula propia).
	FormulaEmpleadoPorConcepto map[int64]int64
	SalarioBasePeriodo         decimal.Decimal    // planilla_detalle_empleado.salario_base_periodo
	Patronal                   []LineaPatronal    // planilla_detalle_patronal
	Prestamos                  []LineaPrestamo    // planilla_detalle_prestamo
	Incapacidades              []LineaIncapacidad // planilla_detalle_incapacidad

	baseAcumulada decimal.Decimal // base del aguinaldo / salario escolar (la fija el orquestador)

	VacacionesCompensadas []int64 // movimientos 'compensacion' que paga el período
}

type estadoCalculo struct {
	lineas   []ConceptoCalculadoOutput
	ingresos []ConceptoCalculadoOutput
	formulas map[int64]int64
	// formulasEmpleado: concepto -> empleado_concepto_formula_id.
	formulasEmpleado map[int64]int64
	res              *ResultadoEmpleadoOutput
	usados           map[string]bool

	salarioBase              decimal.Decimal
	totalIng, totalDed       decimal.Decimal
	baseCCSS                 decimal.Decimal
	ccssObrero, ccssPatronal decimal.Decimal
	ccssHecho                bool
	rentaNeta                decimal.Decimal

	patronal      []LineaPatronal
	prestamos     []LineaPrestamo
	incapacidades []LineaIncapacidad

	conceptoPrestamoID int64 // concepto de la regla prestamo (reparte el tope de neto)

	vacacionesCompensadas []int64
}

// advertir agrega una advertencia no fatal al resultado del empleado.
func (s *estadoCalculo) advertir(m string) {
	s.res.Advertencias = append(s.res.Advertencias, m)
}

func (s *estadoCalculo) agregar(c ConceptoAplicable, monto, cantidad decimal.Decimal) {
	l := ConceptoCalculadoOutput{
		ConceptoSalarialID:   c.ConceptoSalarialID,
		Codigo:               c.Codigo,
		Naturaleza:           c.Naturaleza,
		OrdenCalculo:         c.OrdenCalculo,
		Monto:                monto,
		Cantidad:             cantidad,
		AfectaBaseCCSS:       c.AfectaBaseCCSS,
		AfectaBaseRenta:      c.AfectaBaseRenta,
		AfectaBaseAguinaldo:  c.AfectaBaseAguinaldo,
		AfectaBaseCesantia:   c.AfectaBaseCesantia,
		AfectaBaseVacaciones: c.AfectaBaseVacaciones,
		AfectaBaseEmbargo:    c.AfectaBaseEmbargo,

		EsDeduccionObligatoria: c.EsDeduccionObligatoria,
	}
	if c.FormulaID != 0 {
		s.formulas[c.ConceptoSalarialID] = c.FormulaID
	}
	if c.FormulaEmpleadoID != 0 {
		s.formulasEmpleado[c.ConceptoSalarialID] = c.FormulaEmpleadoID
	}
	s.sumar(l)
}

// agregarRetroactivo: línea del concepto que paga la diferencia de un período
// aprobado; lleva las banderas de base del concepto, sin fórmula.
func (s *estadoCalculo) agregarRetroactivo(c ConceptoAplicable, r RetroactivoInput) {
	l := ConceptoCalculadoOutput{
		ConceptoSalarialID:   c.ConceptoSalarialID,
		Codigo:               c.Codigo,
		Naturaleza:           c.Naturaleza,
		OrdenCalculo:         c.OrdenCalculo,
		Monto:                r.Monto,
		AfectaBaseCCSS:       c.AfectaBaseCCSS,
		AfectaBaseRenta:      c.AfectaBaseRenta,
		AfectaBaseAguinaldo:  c.AfectaBaseAguinaldo,
		AfectaBaseCesantia:   c.AfectaBaseCesantia,
		AfectaBaseVacaciones: c.AfectaBaseVacaciones,
		AfectaBaseEmbargo:    c.AfectaBaseEmbargo,
		Retroactivo:          &r,
	}
	s.sumar(l)
}

func (s *estadoCalculo) sumar(l ConceptoCalculadoOutput) {
	s.lineas = append(s.lineas, l)
	monto := l.Monto
	if l.Naturaleza == naturalezaIngreso {
		s.ingresos = append(s.ingresos, l)
		s.totalIng = s.totalIng.Add(monto)
	} else {
		s.totalDed = s.totalDed.Add(monto)
	}
}

// CalcularEmpleado calcula la planilla individual: evaluarConcepto -> cada
// concepto en orden_calculo -> totales. Las bases (CCSS, renta) se toman de
// los ingresos ya calculados en el flujo, así que el orden sembrado
// (ingresos < CCSS_OBR < IMP_RENTA < EMBARGO < PRESTAMO) es contractual.
func (m *MotorEmpleado) CalcularEmpleado(ctx context.Context, in CalculoEmpleadoInput, p ParametrosCalculo) (*CalculoEmpleadoDetalle, error) {
	contrato, conceptos, err := m.evaluarConcepto(ctx, in.EmpleadoID, in.TipoPlanillaID, in.FechaEfectiva)
	if err != nil {
		return nil, err
	}

	res := &ResultadoEmpleadoOutput{EmpleadoID: in.EmpleadoID, PeriodoID: in.PeriodoID}
	st := &estadoCalculo{formulas: map[int64]int64{}, formulasEmpleado: map[int64]int64{}, res: res, usados: map[string]bool{}}

	manuales := map[int64]decimal.Decimal{}
	for _, d := range in.DevengosManuales {
		manuales[d.ConceptoSalarialID] = manuales[d.ConceptoSalarialID].Add(d.Monto)
	}
	aplicables := map[int64]bool{}
	for _, c := range conceptos {
		aplicables[c.ConceptoSalarialID] = true
	}
	for id := range manuales {
		if !aplicables[id] {
			return nil, fmt.Errorf("planilla: devengo manual para concepto_salarial_id=%d que no aplica al régimen/fecha del empleado_id=%d", id, in.EmpleadoID)
		}
	}

	var factores map[int64]ParametroHoraExtra
	if len(in.HorasExtra) > 0 {
		ps, err := m.repo.ListarParametrosHorasExtraEnFecha(ctx, in.FechaEfectiva)
		if err != nil {
			return nil, fmt.Errorf("planilla: error leyendo parametros_horas_extra: %w", err)
		}
		factores = make(map[int64]ParametroHoraExtra, len(ps))
		for _, x := range ps {
			factores[x.TipoHoraExtraID] = x
		}
		for id, x := range p.FactoresHoraExtraEmpleado {
			factores[id] = x
		}
	}

	retros := map[int64][]RetroactivoInput{}
	for _, r := range in.Retroactivos {
		if !aplicables[r.ConceptoSalarialID] {
			return nil, fmt.Errorf("planilla: empleado_id=%d tiene un retroactivo del concepto_salarial_id=%d que no aplica a su régimen/tipo de planilla",
				in.EmpleadoID, r.ConceptoSalarialID)
		}
		retros[r.ConceptoSalarialID] = append(retros[r.ConceptoSalarialID], r)
	}

	for _, c := range conceptos {
		if err := m.calcularConcepto(c, contrato, in, p, manuales, factores, st); err != nil {
			return nil, fmt.Errorf("planilla: empleado_id=%d, concepto %q: %w", in.EmpleadoID, c.Codigo, err)
		}
		for _, r := range retros[c.ConceptoSalarialID] {
			if c.Naturaleza != naturalezaIngreso {
				return nil, fmt.Errorf("planilla: empleado_id=%d, retroactivo del concepto %q que no es ingreso", in.EmpleadoID, c.Codigo)
			}
			st.agregarRetroactivo(c, r)
		}
	}

	if len(in.HorasExtra) > 0 && !st.usados[formulaHorasExtra] {
		return nil, fmt.Errorf("planilla: empleado_id=%d tiene horas extra pero HRS_EXTRA no aplica a su régimen", in.EmpleadoID)
	}
	if len(in.FeriadosLaborados) > 0 && !st.usados[formulaFeriadoLaborado] {
		return nil, fmt.Errorf("planilla: empleado_id=%d trabajó feriados pero FERIADO_LAB no aplica a su régimen/tipo de planilla", in.EmpleadoID)
	}
	if len(in.Embargos) > 0 && !st.usados[formulaEmbargo] {
		return nil, fmt.Errorf("planilla: empleado_id=%d tiene embargos activos pero EMBARGO no aplica a su régimen", in.EmpleadoID)
	}
	if len(in.Incapacidades) > 0 && !st.usados[formulaIncapacidad] {
		return nil, fmt.Errorf("planilla: empleado_id=%d tiene incapacidades pero SUB_INCAP no aplica a su régimen/tipo de planilla", in.EmpleadoID)
	}
	if len(in.Prestamos) > 0 && !st.usados[formulaPrestamo] {
		return nil, fmt.Errorf("planilla: empleado_id=%d tiene préstamos activos pero PRESTAMO no aplica a su régimen", in.EmpleadoID)
	}
	for _, a := range in.Asignaciones {
		if !aplicables[a.ConceptoSalarialID] {
			return nil, fmt.Errorf("planilla: empleado_id=%d tiene la asignación %d de un concepto que no aplica a su régimen/tipo de planilla (concepto_salarial_id=%d)",
				in.EmpleadoID, a.AsignacionID, a.ConceptoSalarialID)
		}
	}
	if in.Liquidacion != nil && !st.usados[formulaLiquidacion] {
		return nil, fmt.Errorf("planilla: empleado_id=%d tiene una liquidación que pagar pero ningún concepto LIQ_* aplica a su régimen/tipo de planilla", in.EmpleadoID)
	}
	if (len(in.VacacionesCompensacion) > 0 || (len(in.VacacionesDisfrute) > 0 && pagaDisfrute(p.VacacionesModoPago))) && !st.usados[formulaVacaciones] {
		return nil, fmt.Errorf("planilla: empleado_id=%d tiene vacaciones que pagar pero PAGO_VACACIONES no aplica a su régimen/tipo de planilla", in.EmpleadoID)
	}

	if err := AplicarTopeNeto(st); err != nil {
		return nil, fmt.Errorf("planilla: empleado_id=%d: %w", in.EmpleadoID, err)
	}

	res.TotalIngresos = st.totalIng.Round(dosDecimales)
	res.TotalDeducciones = st.totalDed.Round(dosDecimales)
	res.NetoPagado = res.TotalIngresos.Sub(res.TotalDeducciones)

	res.BaseCCSS = CalcularBaseCCSS(st.ingresos)
	if st.ccssHecho {
		res.BaseCCSS = st.baseCCSS
	}
	res.BaseRenta = CalcularBaseRenta(st.ingresos)
	for _, l := range st.ingresos {
		if l.AfectaBaseAguinaldo {
			res.BaseAguinaldo = res.BaseAguinaldo.Add(l.Monto)
		}
		if l.AfectaBaseCesantia {
			res.BaseCesantia = res.BaseCesantia.Add(l.Monto)
		}
		if l.AfectaBaseVacaciones {
			res.BaseVacaciones = res.BaseVacaciones.Add(l.Monto)
		}
	}
	res.BaseEmbargo = liquidoEmbargable(st)
	res.CuotaObreroCCSS, res.CuotaPatronalCCSS = st.ccssObrero, st.ccssPatronal
	res.ImpuestoRentaNeto = st.rentaNeta
	res.DesglosePorConcepto = st.lineas

	return &CalculoEmpleadoDetalle{ResultadoEmpleadoOutput: *res, FormulaPorConcepto: st.formulas,
		FormulaEmpleadoPorConcepto: st.formulasEmpleado, SalarioBasePeriodo: st.salarioBase,
		Patronal: st.patronal, Prestamos: st.prestamos, Incapacidades: st.incapacidades,
		VacacionesCompensadas: st.vacacionesCompensadas,
	}, nil
}

func (m *MotorEmpleado) calcularConcepto(
	c ConceptoAplicable, contrato ContratoVigente, in CalculoEmpleadoInput, p ParametrosCalculo,
	manuales map[int64]decimal.Decimal, factores map[int64]ParametroHoraExtra, st *estadoCalculo,
) error {
	switch c.TipoCalculo {
	case calculoCapturaManual:
		monto, ok := manuales[c.ConceptoSalarialID]
		if !ok {
			return nil
		}
		st.agregar(c, monto.Round(dosDecimales), decimal.Zero)
		return nil

	case calculoFijo:
		st.agregar(c, c.montoFijoRedondeado(), decimal.NewFromInt(1))
		return nil
	}

	regla, ok := m.reglas[c.TipoFormula]
	if !ok {
		return fmt.Errorf("tipo de fórmula desconocido %q", c.TipoFormula)
	}
	return regla.Aplicar(&ContextoRegla{
		Concepto: c, Contrato: contrato, In: in, P: p,
		Manuales: manuales, Factores: factores, St: st,
	})
}

func (c ConceptoAplicable) montoFijoRedondeado() decimal.Decimal {
	if c.MontoFijo == nil {
		return decimal.Zero
	}
	return c.MontoFijo.Round(dosDecimales)
}
