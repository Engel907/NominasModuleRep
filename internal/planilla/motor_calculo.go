// Cálculo de UN empleado: evaluarConcepto, despacho de cada concepto a su
// Regla (reglas.go y regla_*.go) y totales.
//
// Las reglas componen las funciones puras de calculator.go (CalcularBaseCCSS,
// CalcularCCSS, CalcularBaseRenta, CalcularImpuestoRenta,
// ResolverCreditosFiscales, CalcularEmbargos, CalcularPrestamos).
//
// Regla de diseño (igual que el resto del motor): nada de defaults
// implícitos. Si falta un parámetro, un régimen, una fórmula o un factor,
// se retorna error explícito.
package planilla

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// Acceso a datos
// ============================================================================

// ContratoVigente: subconjunto de contratos que necesita el motor.
type ContratoVigente struct {
	ContratoID       int64
	EmpleadoID       int64
	RegimenLaboralID int64
	SalarioPactado   decimal.Decimal
	Jornada          string
}

// ConceptoAplicableFila es la fila cruda (aún sin interpretar la fórmula).
type ConceptoAplicableFila struct {
	ConceptoOrdenCalculo // ConceptoSalarialID, Codigo, OrdenCalculo, MontoFijo, AfectaBase*
	Naturaleza           string
	TipoCalculo          string
	FormulaID            *int64
	ExpresionFormula     *string
}

// MotorRepository: lecturas de referencia del orquestador. Sin escrituras.
type MotorRepository interface {
	ObtenerContratoVigenteEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (ContratoVigente, error)
	ListarConceptosAplicablesEnFecha(ctx context.Context, regimenLaboralID, tipoPlanillaID int64, fecha time.Time) ([]ConceptoAplicableFila, error)
	ListarParametrosHorasExtraEnFecha(ctx context.Context, fecha time.Time) ([]ParametroHoraExtra, error)
}

// ============================================================================
// evaluarConcepto
// ============================================================================

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
	FormulaID   int64  // 0 si no tiene versión de fórmula (solo captura_manual)
	TipoFormula string // expresion_formula.tipo ("" si no aplica)
	Lado        string // expresion_formula.lado ("" si no aplica)
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
//
//	(a) vigencia a `fecha` de regla de aplicabilidad, orden y fórmula;
//	(b) el concepto está en el catálogo activo y tiene orden de cálculo;
//	(c) aplica al régimen laboral del contrato vigente a `fecha`;
//	(d) lo calcula el tipo de planilla (tipo_planilla_concepto, D18).
//
// Devuelve también el contrato (el salario base SAL_BASE sale de
// contratos.salario_pactado, ver seed de concepto_salarial_formula).
// Resultado ordenado por OrdenCalculo (luego ConceptoSalarialID).
func (m *MotorEmpleado) evaluarConcepto(ctx context.Context, empleadoID, tipoPlanillaID int64, fecha time.Time) (ContratoVigente, []ConceptoAplicable, error) {
	contrato, err := m.repo.ObtenerContratoVigenteEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return ContratoVigente{}, nil, fmt.Errorf("planilla: empleado_id=%d sin contrato vigente al %s (no se asume régimen): %w",
			empleadoID, fecha.Format("2006-01-02"), err)
	}

	filas, err := m.repo.ListarConceptosAplicablesEnFecha(ctx, contrato.RegimenLaboralID, tipoPlanillaID, fecha)
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

		switch c.TipoCalculo {
		case calculoCapturaManual:
			// No requiere fórmula.
		case calculoFijo:
			if c.FormulaID == 0 || f.MontoFijo == nil {
				return ContratoVigente{}, nil, fmt.Errorf("planilla: concepto fijo %q sin monto_fijo vigente al %s", c.Codigo, fecha.Format("2006-01-02"))
			}
		case calculoFormula:
			if c.FormulaID == 0 || f.ExpresionFormula == nil {
				return ContratoVigente{}, nil, fmt.Errorf("planilla: concepto %q (formula) sin expresion_formula vigente al %s", c.Codigo, fecha.Format("2006-01-02"))
			}
			var e struct {
				Tipo string `json:"tipo"`
				Lado string `json:"lado"`
			}
			if err := json.Unmarshal([]byte(*f.ExpresionFormula), &e); err != nil {
				return ContratoVigente{}, nil, fmt.Errorf("planilla: expresion_formula inválida en concepto %q: %w", c.Codigo, err)
			}
			if e.Tipo == "" {
				return ContratoVigente{}, nil, fmt.Errorf("planilla: expresion_formula de %q no define 'tipo'", c.Codigo)
			}
			c.TipoFormula, c.Lado = e.Tipo, e.Lado
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

// ============================================================================
// CalcularEmpleado
// ============================================================================

// ParametrosCalculo: parámetros vigentes ya resueltos por el llamador para
// la FechaEfectiva. Los campos se validan solo cuando el flujo los usa.
type ParametrosCalculo struct {
	// Carga social ya combinada (obrero de CCSS_OBR + patronal de CCSS_PAT),
	// EXPRESADA COMO FRACCIÓN (0.1083), no como 10.83 — ver nota en la respuesta.
	CargaSocial ParametroCargaSocial

	TramosRenta      []TramoRenta // Porcentaje como fracción (0.10), no 10.00
	CreditosFiscales map[int64]CreditoFiscalParametro

	SalarioMinimoInembargable decimal.Decimal
	FactoresHoraExtra         map[int64]ParametroHoraExtra
	SubsidiosIncapacidad      []TramoSubsidioIncapacidad
	BaseMinimaContributiva    map[int64]decimal.Decimal
	EscalaEmbargo             []TramoEscalaEmbargo

	// Divisor de días del mes para el salario diario
	// (parametros_generales.DIVISOR_DIAS_MES). L4 solo lo carga; lo consume L10.
	DivisorDiasMes decimal.Decimal

	// Divisor de la hora ordinaria (horas ordinarias del mes según jornada).
	HorasMensualesOrdinarias decimal.Decimal

	// Devengo de SAL_BASE en el período:
	//   salario * FraccionMensualPeriodo * DiasTrabajados / DiasPeriodoCompleto
	// (ej. quincena: 0.5 y 15; mes: 1 y 30).
	FraccionMensualPeriodo decimal.Decimal
	DiasPeriodoCompleto    decimal.Decimal
}

// CalculoEmpleadoDetalle: resultado + trazabilidad de fórmula por concepto
// (para planilla_detalle_concepto.concepto_salarial_formula_id).
type CalculoEmpleadoDetalle struct {
	ResultadoEmpleadoOutput
	FormulaPorConcepto map[int64]int64    // concepto_salarial_id -> concepto_salarial_formula_id
	SalarioBasePeriodo decimal.Decimal    // planilla_detalle_empleado.salario_base_periodo
	Patronal           []LineaPatronal    // planilla_detalle_patronal
	Prestamos          []LineaPrestamo    // planilla_detalle_prestamo
	Incapacidades      []LineaIncapacidad // planilla_detalle_incapacidad (L11)
}

type estadoCalculo struct {
	lineas   []ConceptoCalculadoOutput
	ingresos []ConceptoCalculadoOutput
	formulas map[int64]int64
	res      *ResultadoEmpleadoOutput
	usados   map[string]bool

	salarioBase              decimal.Decimal
	totalIng, totalDed       decimal.Decimal
	baseCCSS                 decimal.Decimal
	ccssObrero, ccssPatronal decimal.Decimal
	ccssHecho                bool
	rentaNeta                decimal.Decimal

	patronal      []LineaPatronal
	prestamos     []LineaPrestamo
	incapacidades []LineaIncapacidad
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
	}
	s.lineas = append(s.lineas, l)
	if c.FormulaID != 0 {
		s.formulas[c.ConceptoSalarialID] = c.FormulaID
	}
	if c.Naturaleza == naturalezaIngreso {
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
	st := &estadoCalculo{formulas: map[int64]int64{}, res: res, usados: map[string]bool{}}

	if !in.SalarioBaseNominal.IsZero() && !in.SalarioBaseNominal.Equal(contrato.SalarioPactado) {
		res.Advertencias = append(res.Advertencias, fmt.Sprintf(
			"salario_base_nominal del input (%s) difiere de contratos.salario_pactado (%s); se usó el del contrato",
			in.SalarioBaseNominal.StringFixed(dosDecimales), contrato.SalarioPactado.StringFixed(dosDecimales)))
	}

	// Devengos manuales por concepto; todos deben consumirse.
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

	// Factores de horas extra: solo si hay horas.
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
	}

	for _, c := range conceptos {
		if err := m.calcularConcepto(c, contrato, in, p, manuales, factores, st); err != nil {
			return nil, fmt.Errorf("planilla: empleado_id=%d, concepto %q: %w", in.EmpleadoID, c.Codigo, err)
		}
	}

	// Insumos que existen pero cuyo concepto no aplicó: nunca se ignoran en silencio.
	if len(in.HorasExtra) > 0 && !st.usados[formulaHorasExtra] {
		return nil, fmt.Errorf("planilla: empleado_id=%d tiene horas extra pero HRS_EXTRA no aplica a su régimen", in.EmpleadoID)
	}
	if len(in.Embargos) > 0 && !st.usados[formulaEmbargo] {
		return nil, fmt.Errorf("planilla: empleado_id=%d tiene embargos activos pero EMBARGO no aplica a su régimen", in.EmpleadoID)
	}
	if len(in.Prestamos) > 0 && !st.usados[formulaPrestamo] {
		return nil, fmt.Errorf("planilla: empleado_id=%d tiene préstamos activos pero PRESTAMO no aplica a su régimen", in.EmpleadoID)
	}

	// Totales y bases.
	res.TotalIngresos = st.totalIng.Round(dosDecimales)
	res.TotalDeducciones = st.totalDed.Round(dosDecimales)
	res.NetoPagado = res.TotalIngresos.Sub(res.TotalDeducciones)
	if res.NetoPagado.IsNegative() {
		return nil, fmt.Errorf("planilla: empleado_id=%d: deducciones (%s) exceden ingresos (%s)",
			in.EmpleadoID, res.TotalDeducciones.StringFixed(dosDecimales), res.TotalIngresos.StringFixed(dosDecimales))
	}

	res.BaseCCSS = CalcularBaseCCSS(st.ingresos)
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
	res.CuotaObreroCCSS, res.CuotaPatronalCCSS = st.ccssObrero, st.ccssPatronal
	res.ImpuestoRentaNeto = st.rentaNeta
	res.DesglosePorConcepto = st.lineas

	return &CalculoEmpleadoDetalle{ResultadoEmpleadoOutput: *res, FormulaPorConcepto: st.formulas, SalarioBasePeriodo: st.salarioBase,
		Patronal: st.patronal, Prestamos: st.prestamos, Incapacidades: st.incapacidades,
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
			return nil // nada capturado en este período
		}
		if c.Naturaleza != naturalezaIngreso {
			return errors.New("captura manual solo soportada para conceptos de ingreso (DevengosManuales)")
		}
		st.agregar(c, monto.Round(dosDecimales), decimal.Zero)
		return nil

	case calculoFijo:
		// evaluarConcepto ya garantizó MontoFijo != nil.
		st.agregar(c, c.montoFijoRedondeado(), decimal.NewFromInt(1))
		return nil
	}

	// tipo_calculo = formula: cada tipo de fórmula tiene su Regla.
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
