// Package contabilidad genera los asientos contables de una corrida de
// planilla (Fase 10).
//
// Diseño: igual que el motor de cálculo, separa el ARMADO PURO de los
// asientos (GenerarAsientos, sin I/O) de la ORQUESTACIÓN (service.go: lectura
// de mapeos, transacción, persistencia).
//
// Reglas de fondo:
//   - Ninguna cuenta contable, programa ni proyecto se hardcodea: todo sale de
//     mapeo_concepto_cuenta (vigente a la fecha efectiva) y del programa que
//     planilla_detalle_empleado copió del expediente al calcular.
//   - Partida doble: si el juego de asientos no cuadra, se retorna error
//     explícito y NO se inserta nada. Nunca se "ajusta" un descuadre.
package contabilidad

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/planilla"
)

// Movimientos, tal como los persiste mapeo_concepto_cuenta.tipo_movimiento.
const (
	MovimientoDebe  = "debe"
	MovimientoHaber = "haber"
)

// Estados de asientos_contables.estado.
const EstadoAsientoGenerado = "generado"

var dosDecimales = int32(2)

// ============================================================================
// Entidades persistidas
// ============================================================================

// AsientoContable corresponde a asientos_contables (+ sus líneas).
type AsientoContable struct {
	ID                int64
	PeriodoPlanillaID int64
	TipoAsientoID     int64
	TipoAsientoCodigo string // solo informativo (no se persiste)
	NumeroAsiento     string
	FechaGeneracion   time.Time
	Estado            string
	Lineas            []AsientoContableDetalle
}

// AsientoContableDetalle corresponde a asientos_contables_detalle. Cada línea
// es débito XOR crédito con monto > 0 (CHECK chk_acd_debe_o_haber); aquí se
// valida ANTES de llegar a la base de datos.
type AsientoContableDetalle struct {
	ID                       int64
	AsientoContableID        int64
	CuentaContableID         int64
	ProgramaPresupuestarioID int64
	Proyecto                 string
	EmpleadoID               *int64 // nil = línea consolidada
	ConceptoSalarialID       *int64
	MontoDebe                decimal.Decimal
	MontoHaber               decimal.Decimal
}

func (a AsientoContable) TotalDebe() decimal.Decimal {
	t := decimal.Zero
	for _, l := range a.Lineas {
		t = t.Add(l.MontoDebe)
	}
	return t
}

func (a AsientoContable) TotalHaber() decimal.Decimal {
	t := decimal.Zero
	for _, l := range a.Lineas {
		t = t.Add(l.MontoHaber)
	}
	return t
}

// ============================================================================
// Entradas
// ============================================================================

// DetallePlanilla es el resultado calculado de UN empleado más el programa
// presupuestario que planilla_detalle_empleado copió del expediente al
// momento del cálculo. Calculo es el resultado de
// planilla.MotorEmpleado.CalcularEmpleado.
//
// IMPORTANTE: la cuota patronal de CCSS (Calculo.CuotaPatronalCCSS) NO se
// persiste en ninguna tabla ni aparece en DesglosePorConcepto, por lo que los
// asientos deben generarse con el resultado en memoria de la corrida, no
// reconstruyéndolo desde la base de datos.
type DetallePlanilla struct {
	ProgramaPresupuestarioID int64
	Calculo                  *planilla.CalculoEmpleadoDetalle
}

// MapeoConceptoCuenta es una fila vigente de mapeo_concepto_cuenta.
type MapeoConceptoCuenta struct {
	ID                       int64
	ConceptoSalarialID       int64
	ConceptoCodigo           string
	ProgramaPresupuestarioID int64
	TipoAsientoID            int64
	TipoAsientoCodigo        string
	CuentaContableID         int64
	TipoMovimiento           string
	Proyecto                 string
}

// ConfigGeneracion agrupa los códigos de concepto que el motor contable
// necesita conocer por nombre. No tiene valores por defecto: el wiring los
// entrega explícitamente.
type ConfigGeneracion struct {
	// ConceptoAnclaNeto: concepto cuyos mapeos con tipo_movimiento='haber'
	// representan la línea de "salarios por pagar (neto)". No existe un
	// concepto "neto" en el catálogo, y mapeo_concepto_cuenta.concepto_salarial_id
	// es NOT NULL; el seed usa SAL_BASE como ancla. Sus mapeos 'debe' siguen
	// registrando el monto del propio concepto.
	ConceptoAnclaNeto string
	// ConceptoCuotaPatronalCCSS: código del concepto al que se le imputa
	// Calculo.CuotaPatronalCCSS (que no viene en el desglose). Seed: CCSS_PAT.
	ConceptoCuotaPatronalCCSS string
}

// ============================================================================
// Errores
// ============================================================================

var (
	ErrMapeoFaltante          = errors.New("contabilidad: no hay mapeo_concepto_cuenta vigente")
	ErrDescuadre              = errors.New("contabilidad: el asiento no cuadra (debe != haber)")
	ErrConfigInvalida         = errors.New("contabilidad: configuración de generación inválida")
	ErrPeriodoEstadoInvalido  = errors.New("contabilidad: el período no está en un estado contabilizable")
	ErrAsientosYaGenerados    = errors.New("contabilidad: el período ya tiene asientos generados")
	ErrSinDetalles            = errors.New("contabilidad: no hay detalles de planilla para contabilizar")
	ErrMontoNegativoOInvalido = errors.New("contabilidad: monto inválido para línea de asiento")
)

// DescuadreError detalla dónde falló la partida doble.
type DescuadreError struct {
	Alcance    string // "empleado_id=7", "programa_id=1", "corrida"
	TotalDebe  decimal.Decimal
	TotalHaber decimal.Decimal
}

func (e *DescuadreError) Error() string {
	return fmt.Sprintf("%v: %s — debe=%s, haber=%s, diferencia=%s", ErrDescuadre, e.Alcance,
		e.TotalDebe.StringFixed(dosDecimales), e.TotalHaber.StringFixed(dosDecimales),
		e.TotalDebe.Sub(e.TotalHaber).StringFixed(dosDecimales))
}

func (e *DescuadreError) Unwrap() error { return ErrDescuadre }