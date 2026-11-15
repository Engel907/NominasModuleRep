// Package planilla implementa el motor de cálculo de planilla (PayrollEngine).
package planilla

import (
	"time"

	"github.com/shopspring/decimal"
)

// CalculoEmpleadoInput agrupa todo lo que el motor necesita para calcular
// un empleado en un período específico. Debe llegar completamente hidratado
// desde el repositorio antes de invocar CalcularEmpleado — la función de
// cálculo en sí no hace queries a la BD.
type CalculoEmpleadoInput struct {
	EmpleadoID int64
	PeriodoID  int64

	// TipoPlanillaID filtra los conceptos que calcula el tipo de planilla
	// (tipo_planilla_concepto).
	TipoPlanillaID int64

	// FechaEfectiva es la fecha contra la cual se resuelven todos los
	// parámetros vigentes (cargas sociales, tramos de renta, salario
	// mínimo, etc.). Normalmente es la fecha de fin del período, pero en
	// recálculos retroactivos puede diferir.
	FechaEfectiva      time.Time
	FechaInicioPeriodo time.Time
	FechaFinPeriodo    time.Time

	// Datos laborales vigentes del empleado al momento del cálculo. El salario
	// no viene de aquí: sale de los contratos (Segmentos).
	ClasificacionPuestoID int64 // FK para resolver parametros_salario_minimo
	RegimenCCSSID         int64 // FK para resolver parametros_base_minima_contributiva

	// Segmentos del período con contrato y salario: ingreso, cese y
	// aumentos a mitad de período parten el salario por días comerciales.
	Segmentos []SegmentoSalario

	// Asistencia del período. DiasTrabajados = días con contrato (Σ
	// Segmentos.Dias, cada uno en su base); le descuenta la incapacidad.
	DiasTrabajados decimal.Decimal
	HorasExtra     []HoraExtraInput
	// Horas ordinarias trabajadas en feriados del período.
	FeriadosLaborados []FeriadoLaboradoInput
	Incapacidades     []IncapacidadInput // ver incapacidad.go

	// Devengos adicionales que no se derivan de una fórmula automática
	// (ej. bonos discrecionales, comisiones capturadas manualmente).
	DevengosManuales []ConceptoDevengoManualInput

	// Retroactivos: diferencias de períodos aprobados que paga este período
	// (retroactivo.go); cada una se agrega como línea de su concepto.
	Retroactivos []RetroactivoInput

	// Vacaciones: compensaciones pendientes de pago y días de
	// vacaciones aprobadas dentro del período (recortados al período).
	VacacionesCompensacion []CompensacionVacacionesInput
	VacacionesDisfrute     []RangoVacaciones
	// PromedioVacaciones: base de vacaciones de las semanas anteriores al
	// período; solo si VACACIONES_SALARIO_BASE es promedio | mayor.
	PromedioVacaciones *PromedioSalario

	// Bases acumuladas: solo en períodos de clase aguinaldo /
	// salario_escolar; Σ base_aguinaldo de los períodos regulares aprobados
	// de la ventana de cálculo.
	BaseAguinaldoAcumulada      decimal.Decimal
	BaseSalarioEscolarAcumulada decimal.Decimal

	// Asignaciones: conceptos recurrentes del empleado vigentes a la fecha
	// efectiva (solo en períodos regulares).
	Asignaciones []AsignacionInput

	// RentaMesPrevia: lo ya gravado y retenido al empleado en otros períodos
	// aprobados del mismo mes calendario (el impuesto al salario es mensual).
	RentaMesPrevia RentaAcumuladaMes

	// Liquidación: solo en períodos de clase liquidacion; montos por
	// rubro ya calculados y persistidos en `liquidaciones` (internal/liquidacion).
	Liquidacion *LiquidacionEmpleadoInput

	// Deducciones de terceros.
	Prestamos        []PrestamoDeduccionInput
	Embargos         []EmbargoInput
	CreditosFiscales []CreditoFiscalInput

	// EmbargoMesPrevio: líquido embargable y embargos ya aplicados en los
	// otros períodos aprobados del mes. Solo en la planilla de liquidación:
	// con él los embargos se calculan sobre el mes completo (regla_embargo.go).
	EmbargoMesPrevio *EmbargoMesPrevio
}

// EmbargoMesPrevio: acumulado del mes por embargo (lo pedido y lo retenido).
type EmbargoMesPrevio struct {
	Base       decimal.Decimal
	PorEmbargo map[int64]EmbargoPrevio
}

type EmbargoPrevio struct {
	Calculado decimal.Decimal
	Aplicado  decimal.Decimal
}

// LiquidacionEmpleadoInput: liquidación 'procesada' que paga el período.
// Rubros: clave = expresion_formula.rubro de los conceptos LIQ_*.
type LiquidacionEmpleadoInput struct {
	LiquidacionID int64
	FechaSalida   time.Time
	Rubros        map[string]decimal.Decimal
}

type HoraExtraInput struct {
	TipoHoraExtraID int64
	CantidadHoras   decimal.Decimal
}

// ConceptoDevengoManualInput inyecta un monto ya decidido para un
// concepto_salarial puntual (ej. bono discrecional) sin pasar por
// concepto_salarial_formula. El motor lo trata como un concepto más dentro
// del flujo de orden_calculo_concepto,
// pero sin recalcular su fórmula.
type ConceptoDevengoManualInput struct {
	ConceptoSalarialID int64
	Monto              decimal.Decimal
	Descripcion        string
}

type PrestamoDeduccionInput struct {
	PrestamoID     int64
	CuotaPeriodica decimal.Decimal
	SaldoPendiente decimal.Decimal
}

// EmbargoInput: embargo activo del empleado con los datos de su tipo
// (parametros_tipos_embargo) ya resueltos (todo desde BD).
type EmbargoInput struct {
	EmbargoID, TipoEmbargoID int64
	Prioridad                int32 // embargos.prioridad_aplicacion (1 = mayor)
	PrioridadLegal           int32 // parametros_tipos_embargo.prioridad_legal
	EsPension                bool  // parametros_tipos_embargo.es_pension_alimentaria
	// TopePctLiquido: tope del tipo sobre el líquido embargable, en FRACCIÓN
	// (0 = sin tope). En pensiones el tope es ACUMULADO sobre todas ellas.
	TopePctLiquido decimal.Decimal
	Porcentaje     decimal.Decimal // fracción; cero si aplica MontoFijo
	MontoFijo      decimal.Decimal // cero si aplica Porcentaje
	TopeMensual    decimal.Decimal // embargos.tope_mensual (cero = sin tope propio)
}

type CreditoFiscalInput struct {
	TipoCreditoFiscalID int64
	Cantidad            int32 // ej. número de hijos aplicables
}

// ResultadoEmpleadoOutput es la salida completa y auditable del cálculo.
// Se persiste 1:1 (a nivel de agregados) en planilla_detalle_empleado y
// 1:N (DesglosePorConcepto) en planilla_detalle_concepto.
type ResultadoEmpleadoOutput struct {
	EmpleadoID int64
	PeriodoID  int64

	// Bases gravables acumuladas a lo largo del flujo de conceptos.
	BaseCCSS       decimal.Decimal
	BaseRenta      decimal.Decimal
	BaseAguinaldo  decimal.Decimal
	BaseCesantia   decimal.Decimal
	BaseVacaciones decimal.Decimal
	// BaseEmbargo: líquido embargable del período (ingresos con
	// afecta_base_embargo − CCSS obrera − renta neta).
	BaseEmbargo decimal.Decimal

	CuotaObreroCCSS   decimal.Decimal
	CuotaPatronalCCSS decimal.Decimal

	ImpuestoRentaBruto        decimal.Decimal
	CreditosFiscalesAplicados decimal.Decimal
	ImpuestoRentaNeto         decimal.Decimal // ImpuestoRentaBruto - CreditosFiscalesAplicados, nunca negativo

	DeduccionesPrestamos decimal.Decimal
	EmbargosAplicados    []EmbargoAplicadoOutput

	TotalIngresos    decimal.Decimal
	TotalDeducciones decimal.Decimal
	NetoPagado       decimal.Decimal

	DesglosePorConcepto []ConceptoCalculadoOutput

	// Advertencias no fatales (ej. embargo topado por límite legal,
	// crédito fiscal parcialmente aplicado por insuficiencia de renta).
	// No detienen el cálculo pero deben quedar visibles para revisión.
	Advertencias []string
}

type EmbargoAplicadoOutput struct {
	EmbargoID                  int64
	Prioridad                  int32
	MontoCalculado             decimal.Decimal // lo que correspondía según % o monto fijo
	MontoAplicado              decimal.Decimal // lo realmente descontado (puede ser < calculado si no alcanzó el remanente)
	SaldoPendienteInsatisfecho decimal.Decimal // MontoCalculado - MontoAplicado
}

type ConceptoCalculadoOutput struct {
	ConceptoSalarialID int64
	Codigo             string // conceptos_salariales.codigo
	Naturaleza         string // ingreso | deduccion (conceptos_salariales.naturaleza)
	OrdenCalculo       int32
	Monto              decimal.Decimal
	Cantidad           decimal.Decimal // si aplica (ej. horas extra), cero si no aplica

	AfectaBaseCCSS       bool
	AfectaBaseRenta      bool
	AfectaBaseAguinaldo  bool
	AfectaBaseCesantia   bool
	AfectaBaseVacaciones bool
	AfectaBaseEmbargo    bool // integra el líquido embargable

	EsDeduccionObligatoria bool // no se topa por el neto

	// Retroactivo: la línea paga la diferencia de un período ya aprobado
	// (nil en las líneas propias del período).
	Retroactivo *RetroactivoInput
}

// ResultadoPeriodoOutput agrega los resultados de todos los empleados de un
// período, más metadatos de la corrida.
type ResultadoPeriodoOutput struct {
	PeriodoID             int64
	EjecutadoPorUsuarioID int64
	IniciadoEn            time.Time
	FinalizadoEn          time.Time

	Resultados []ResultadoEmpleadoOutput

	// EmpleadosConError lista los empleados que no se pudieron calcular.
	EmpleadosConError []EmpleadoErrorOutput
}

// EmpleadoErrorOutput: error de un empleado en una corrida. Codigo es estable
// (para el cliente); Mensaje es un texto de dominio, nunca un error interno.
type EmpleadoErrorOutput struct {
	EmpleadoID int64  `json:"empleado_id"`
	Codigo     string `json:"codigo"`
	Mensaje    string `json:"mensaje"`
}

// ConceptoOrdenCalculo es la fila resultante de unir orden_calculo_concepto +
// concepto_salarial_formula + conceptos_salariales (flags afecta_base_*),
// vigente a la fecha efectiva del cálculo.
type ConceptoOrdenCalculo struct {
	ConceptoSalarialID int64
	Codigo             string // conceptos_salariales.codigo
	OrdenCalculo       int32

	MontoFijo *decimal.Decimal // nil si el concepto no usa monto fijo directo

	AfectaBaseCCSS       bool
	AfectaBaseRenta      bool
	AfectaBaseAguinaldo  bool
	AfectaBaseCesantia   bool
	AfectaBaseVacaciones bool
	AfectaBaseEmbargo    bool // integra el líquido embargable

	EsDeduccionObligatoria bool // no se topa por el neto
}

// ParametroCargaSocial: tasa vigente de un concepto de carga social, en
// fracción. RegimenCCSSID = 0 es la tasa general del concepto; una fila con
// régimen la sustituye para los empleados de ese régimen.
type ParametroCargaSocial struct {
	ConceptoSalarialID int64
	RegimenCCSSID      int64
	PorcentajePatronal decimal.Decimal
	PorcentajeObrero   decimal.Decimal
}

type TramoRenta struct {
	OrdenTramo int32
	MontoDesde decimal.Decimal
	MontoHasta decimal.Decimal
	Porcentaje decimal.Decimal
	Abierto    bool
}

type CreditoFiscalParametro struct {
	TipoCreditoFiscalID int64
	MontoCredito        decimal.Decimal
}

type ParametroHoraExtra struct {
	TipoHoraExtraID int64
	FactorRecargo   decimal.Decimal
}

// PeriodoPlanillaEstado modela el subconjunto de columnas de
// periodos_planilla relevante para la estrategia de bloqueo (ver
// PayrollEngine.CalcularPeriodo).
type PeriodoPlanillaEstado struct {
	PeriodoID int64
	Estado    string // valor de EstadoPeriodo; coincide con el ENUM de periodos_planilla.estado
}

// RentaAcumuladaMes: Σ de base de renta, impuesto bruto, créditos aplicados e
// impuesto neto de los períodos aprobados de un mes.
type RentaAcumuladaMes struct {
	Base     decimal.Decimal
	Bruta    decimal.Decimal
	Creditos decimal.Decimal
	Neta     decimal.Decimal
}

// Modos de empleado_conceptos_asignados.modo.
const (
	AsignacionMonto      = "monto"
	AsignacionPorcentaje = "porcentaje"
)

// AsignacionInput: concepto recurrente de un empleado. Valor es el monto
// mensual (modo monto) o la FRACCIÓN (modo porcentaje: 0.05 = 5 %).
type AsignacionInput struct {
	AsignacionID       int64
	ConceptoSalarialID int64
	Modo               string
	Valor              decimal.Decimal
}
