package planilla

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Claves de parametros_generales del salario con que se pagan las vacaciones
// y el subsidio de incapacidad.
//
//   - VACACIONES_SALARIO_BASE (texto): vigente | promedio | mayor. Se exige
//     solo si el empleado tiene vacaciones que pagar en el período; no se
//     siembra.
//   - VENTANA_PROMEDIO_VACACIONES_SEMANAS (entero): semanas del promedio de
//     vacaciones (Código de Trabajo, art. 157); la misma ventana usa la
//     liquidación.
//   - INCAPACIDAD_SALARIO_BASE (texto): vigente | promedio | mayor. Se exige
//     solo si el empleado tiene incapacidades en el período; no se siembra.
//   - INCAPACIDAD_PROMEDIO_MESES (entero): meses anteriores al inicio de la
//     boleta que entran en el promedio; se exige con promedio | mayor.
const (
	ClaveVacacionesSalarioBase            = "VACACIONES_SALARIO_BASE"
	ClaveVentanaPromedioVacacionesSemanas = "VENTANA_PROMEDIO_VACACIONES_SEMANAS"
	ClaveIncapacidadSalarioBase           = "INCAPACIDAD_SALARIO_BASE"
	ClaveIncapacidadPromedioMeses         = "INCAPACIDAD_PROMEDIO_MESES"
)

// Valores de VACACIONES_SALARIO_BASE e INCAPACIDAD_SALARIO_BASE.
const (
	SalarioBaseVigente  = "vigente"  // salario del contrato de cada día
	SalarioBasePromedio = "promedio" // promedio de las planillas aprobadas de la ventana
	SalarioBaseMayor    = "mayor"    // el mayor de los dos, día por día
)

// SalarioBaseValida: vigente | promedio | mayor.
func SalarioBaseValida(b string) bool {
	return b == SalarioBaseVigente || b == SalarioBasePromedio || b == SalarioBaseMayor
}

// UsaPromedio: la base necesita el promedio de la ventana.
func UsaPromedio(b string) bool { return b == SalarioBasePromedio || b == SalarioBaseMayor }

// PromedioSalario: Σ de una base persistida de los períodos regulares
// aprobados de la ventana [Desde, Hasta] (más los meses del histórico previo a
// la puesta en marcha) y Σ de su fracción del mes.
type PromedioSalario struct {
	Desde, Hasta time.Time
	Total        decimal.Decimal
	Meses        decimal.Decimal
	Periodos     int
}

// Diario: promedio mensual (Total ÷ Meses, a 2 decimales) ÷ DIVISOR_DIAS_MES,
// a 4 decimales, como en la liquidación. ok = false si la ventana no tiene
// planillas.
func (ps *PromedioSalario) Diario(divisorDiasMes decimal.Decimal) (decimal.Decimal, bool) {
	if ps == nil || ps.Periodos == 0 || !ps.Meses.IsPositive() || !divisorDiasMes.IsPositive() || ps.Total.IsNegative() {
		return decimal.Zero, false
	}
	mensual := ps.Total.DivRound(ps.Meses, 2)
	return mensual.DivRound(divisorDiasMes, 4), true
}

// valorDiaSegunBase: valor del día con que se paga un rubro según su base
// (VACACIONES_SALARIO_BASE o INCAPACIDAD_SALARIO_BASE). Sin planillas en la
// ventana se usa el vigente y queda una advertencia.
func valorDiaSegunBase(clave, base string, vigente decimal.Decimal, prom *PromedioSalario, p ParametrosCalculo, st *estadoCalculo) (decimal.Decimal, error) {
	switch base {
	case SalarioBaseVigente:
		return vigente, nil
	case SalarioBasePromedio, SalarioBaseMayor:
		d, ok := prom.Diario(p.DivisorDiasMes)
		if !ok {
			st.advertir(fmt.Sprintf("%s = %s sin planillas aprobadas en la ventana del promedio: se usó el salario vigente", clave, base))
			return vigente, nil
		}
		if base == SalarioBaseMayor && vigente.GreaterThan(d) {
			return vigente, nil
		}
		return d, nil
	case "":
		return decimal.Zero, fmt.Errorf("%w: %s (vigente | promedio | mayor)", ErrParametroFaltante, clave)
	}
	return decimal.Zero, fmt.Errorf("%s = %q inválido (vigente | promedio | mayor)", clave, base)
}

// ventanaSemanas: [hasta − 7·semanas + 1, hasta].
func ventanaSemanas(hasta time.Time, semanas int) (time.Time, time.Time) {
	hasta = soloFecha(hasta)
	return hasta.AddDate(0, 0, -7*semanas+1), hasta
}

// ventanaMeses: [hasta − meses + 1 día, hasta].
func ventanaMeses(hasta time.Time, meses int) (time.Time, time.Time) {
	hasta = soloFecha(hasta)
	return hasta.AddDate(0, -meses, 1), hasta
}
