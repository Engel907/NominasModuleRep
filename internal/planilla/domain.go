// Package planilla implementa el motor de cálculo de planilla (PayrollEngine).
//
// Fase 6a — Diseño de interfaces, DTOs y estrategia de concurrencia.
// Ver ADR-000 (Documento de Decisiones de Arquitectura) para el contexto
// completo de esquema, patrón de vigencia y modelo de autorización.
//
// Principio de diseño: separar la ORQUESTACIÓN (acceso a BD, locking,
// transacciones — ver service.go) del CÁLCULO PURO (domain.go + la función
// CalcularEmpleado en service.go), que opera solo sobre datos ya
// hidratados en memoria. Esto permite testear el motor con tablas de casos
// (table-driven tests) sin necesidad de una BD real.
package planilla

import (
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// INPUT: datos de entrada para calcular la planilla de UN empleado en UN
// periodo
// ============================================================================

// CalculoEmpleadoInput agrupa todo lo que el motor necesita para calcular
// un empleado en un período específico. Debe llegar completamente hidratado
// desde el repositorio ANTES de invocar CalcularEmpleado — la función de
// cálculo en sí no hace queries a la BD (ver nota de diseño arriba).
type CalculoEmpleadoInput struct {
	EmpleadoID int64
	PeriodoID  int64

	// FechaEfectiva es la fecha contra la cual se resuelven TODOS los
	// parámetros vigentes (cargas sociales, tramos de renta, salario
	// mínimo, etc.) vía el patrón fecha_inicio_vigencia/fecha_fin_vigencia
	// (ADR-000 §2). Normalmente es la fecha de fin del período, pero se
	// modela explícita porque en recálculos retroactivos puede diferir.
	FechaEfectiva      time.Time
	FechaInicioPeriodo time.Time
	FechaFinPeriodo    time.Time

	// Datos laborales vigentes del empleado al momento del cálculo.
	SalarioBaseNominal    decimal.Decimal // empleado_datos_laborales.salario_base_nominal
	ClasificacionPuestoID int64           // FK para resolver parametros_salario_minimo
	RegimenCCSSID         int64           // FK para resolver parametros_base_minima_contributiva

	// Asistencia del período.
	DiasTrabajados decimal.Decimal
	HorasExtra     []HoraExtraInput
	Incapacidades  []IncapacidadInput

	// Devengos adicionales que no se derivan de una fórmula automática
	// (ej. bonos discrecionales, comisiones capturadas manualmente).
	DevengosManuales []ConceptoDevengoManualInput

	// Deducciones de terceros.
	Prestamos        []PrestamoDeduccionInput
	Embargos         []EmbargoInput
	CreditosFiscales []CreditoFiscalInput
}

type HoraExtraInput struct {
	TipoHoraExtraID int64
	CantidadHoras   decimal.Decimal
}

type IncapacidadInput struct {
	EntidadEmisoraID int64
	DiaDesde         int32 // ancla la escala en parametros_subsidios_incapacidad.dia_desde
	DiasIncapacidad  decimal.Decimal
}

// ConceptoDevengoManualInput inyecta un monto ya decidido para un
// concepto_salarial puntual (ej. bono discrecional) sin pasar por
// concepto_salarial_formula. El motor lo trata como un concepto más dentro
// del flujo de orden_calculo_concepto (ver service.go, evaluarConcepto),
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

// EmbargoInput trae ya resuelta la prioridad legal (1: Pensión Alimentaria,
// 2: Judicial, 3: Hacienda, 4: CCSS — ver Fases 1-5) para que el motor
// pueda aplicar los embargos en orden estricto de prioridad cuando el neto
// disponible no alcanza para todos.
type EmbargoInput struct {
	EmbargoID     int64
	TipoEmbargoID int64
	Prioridad     int32           // 1 = mayor prioridad
	Porcentaje    decimal.Decimal // cero si aplica MontoFijo en su lugar
	MontoFijo     decimal.Decimal // cero si aplica Porcentaje en su lugar
	TopeMensual   decimal.Decimal // límite legal, si aplica (cero = sin tope explícito en este embargo)
}

type CreditoFiscalInput struct {
	TipoCreditoFiscalID int64
	Cantidad            int32 // ej. número de hijos aplicables
}

// ============================================================================
// OUTPUT: resultado del cálculo de UN empleado
// ============================================================================

// ResultadoEmpleadoOutput es la salida completa y auditable del cálculo.
// Se persiste 1:1 (a nivel de agregados) en planilla_detalle_empleado y
// 1:N (DesglosePorConcepto) en planilla_detalle_concepto.
type ResultadoEmpleadoOutput struct {
	EmpleadoID int64
	PeriodoID  int64

	// Bases gravables acumuladas a lo largo del flujo de conceptos
	// (ver evaluarConcepto / afecta_base_* en service.go).
	BaseCCSS       decimal.Decimal
	BaseRenta      decimal.Decimal
	BaseAguinaldo  decimal.Decimal
	BaseCesantia   decimal.Decimal
	BaseVacaciones decimal.Decimal

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
	Codigo             string // conceptos_salariales.codigo (ver seed Fase 3.5)
	OrdenCalculo       int32
	Monto              decimal.Decimal
	Cantidad           decimal.Decimal // si aplica (ej. horas extra), cero si no aplica

	AfectaBaseCCSS       bool
	AfectaBaseRenta      bool
	AfectaBaseAguinaldo  bool
	AfectaBaseCesantia   bool
	AfectaBaseVacaciones bool
}

// ResultadoPeriodoOutput agrega los resultados de todos los empleados de un
// período, más metadatos de la corrida (auditoría y estado de
// periodos_planilla — ver service.go, estrategia de bloqueo).
type ResultadoPeriodoOutput struct {
	PeriodoID             int64
	EjecutadoPorUsuarioID int64
	IniciadoEn            time.Time
	FinalizadoEn          time.Time

	Resultados []ResultadoEmpleadoOutput

	// EmpleadosConError permite reportar fallos parciales (ej. falta un
	// parámetro vigente para un empleado puntual) sin descartar el resto
	// del período — decisión de negocio a confirmar en Fase 6b: ¿el
	// período completo falla si UN empleado falla, o se reporta parcial?
	EmpleadosConError []EmpleadoErrorOutput
}

type EmpleadoErrorOutput struct {
	EmpleadoID int64
	Error      string
}

// ============================================================================
// Parámetros vigentes resueltos por el repositorio (fecha-dependientes)
// ============================================================================

// ConceptoOrdenCalculo es la fila resultante de unir orden_calculo_concepto +
// concepto_salarial_formula + conceptos_salariales (flags afecta_base_*),
// vigente a la fecha efectiva del cálculo.
type ConceptoOrdenCalculo struct {
	ConceptoSalarialID int64
	Codigo             string // conceptos_salariales.codigo (seed Fase 3.5: SAL_BASE, CCSS_OBR, CCSS_PAT, IMP_RENTA, HRS_EXTRA, SUB_INCAP, AGUINALDO, VACACIONES, CESANTIA)
	OrdenCalculo       int32

	// NOTA (asunción a confirmar en Fase 6b): ADR-000 §4 solo documenta
	// concepto_salarial_formula.monto_fijo DECIMAL(14,4); no detalla cómo
	// se distingue fijo / porcentaje-de-base / fórmula dinámica en el
	// esquema real. Se modela aquí de forma provisional.
	MontoFijo *decimal.Decimal // nil si el concepto no usa monto fijo directo

	AfectaBaseCCSS       bool
	AfectaBaseRenta      bool
	AfectaBaseAguinaldo  bool
	AfectaBaseCesantia   bool
	AfectaBaseVacaciones bool
}

type ParametroCargaSocial struct {
	ConceptoSalarialID int64
	PorcentajePatronal decimal.Decimal
	PorcentajeObrero   decimal.Decimal
}

type TramoRenta struct {
	OrdenTramo int32
	MontoDesde decimal.Decimal
	MontoHasta decimal.Decimal // tramo abierto (último) a confirmar cómo se representa (nil vs. valor centinela)
	Porcentaje decimal.Decimal
}

type CreditoFiscalParametro struct {
	TipoCreditoFiscalID int64
	MontoCredito        decimal.Decimal
}

type TramoSubsidioIncapacidad struct {
	EntidadEmisoraID int64
	DiaDesde         int32
	Porcentaje       decimal.Decimal // asunción: escala expresada como % del salario, a confirmar
}

type ParametroHoraExtra struct {
	TipoHoraExtraID int64
	FactorRecargo   decimal.Decimal
}

// PeriodoPlanillaEstado modela el subconjunto de columnas de
// periodos_planilla relevante para la estrategia de bloqueo (ver
// PayrollEngine.CalcularPeriodo en service.go).
type PeriodoPlanillaEstado struct {
	PeriodoID int64
	Estado    string // 'abierto' | 'calculando' | 'calculado' | 'error' | 'cerrado' — enum real a confirmar contra el .sql fuente
}
