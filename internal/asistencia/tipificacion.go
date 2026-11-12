// L14 — Tipificación de horas extra por minuto (enteros, sin float).
//
// SUPUESTOS A VALIDAR (RR. HH./Legal):
//   - Son horas extra los ÚLTIMOS N minutos trabajados, con
//     N = minutos trabajados − minutos de la jornada del turno. Así la suma
//     tipificada coincide con ExcesoJornada (calcularHoras) y llegar tarde y
//     salir tarde no genera extra.
//   - Feriado o día de descanso: todo el exceso es "doble".
//   - Si no: minuto dentro de la franja nocturna vigente → "nocturna", o
//     "mixta" si la jornada del contrato es mixta; resto → "diurna".
package asistencia

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// Clases de tipos_hora_extra.clase (Go nunca decide por el código de texto).
const (
	ClaseHEDiurna   = "diurna"
	ClaseHENocturna = "nocturna"
	ClaseHEMixta    = "mixta"
	ClaseHEDoble    = "doble"
)

// ErrFranjaNocturnaNoVigente: hay horas extra que tipificar fuera de feriado y
// descanso pero no hay parametros_franja_nocturna vigente (D8: sin default).
var ErrFranjaNocturnaNoVigente = errors.New("no hay franja nocturna vigente (parametros_franja_nocturna) para tipificar las horas extra")

// FranjaNocturna: horas de reloj; puede cruzar la medianoche (22:00–05:00).
type FranjaNocturna struct {
	Vigente     bool
	Inicio, Fin time.Time
}

// contiene indica si el minuto del día m está en [Inicio, Fin).
func (f FranjaNocturna) contiene(m int) bool {
	ini, fin := minutosDelDia(f.Inicio), minutosDelDia(f.Fin)
	if ini < fin {
		return m >= ini && m < fin
	}
	return m >= ini || m < fin // cruza la medianoche
}

// HoraExtraTipificada: horas (2 decimales) de una clase en un día.
type HoraExtraTipificada struct {
	Clase string
	Horas decimal.Decimal
}

// TipificarHorasExtra reparte el exceso de la jornada real sobre la del turno
// entre las clases de hora extra. Sin exceso devuelve nil (no exige franja).
func TipificarHorasExtra(entrada, salida, turnoIni, turnoFin time.Time, franja FranjaNocturna, esFeriado, esDescanso bool, jornadaMixta bool) ([]HoraExtraTipificada, error) {
	inicio := minutosDelDia(entrada)
	trabajados := duracionMinutos(inicio, minutosDelDia(salida))
	programados := duracionMinutos(minutosDelDia(turnoIni), minutosDelDia(turnoFin))
	exceso := trabajados - programados
	if exceso <= 0 {
		return nil, nil
	}
	if esFeriado || esDescanso {
		return []HoraExtraTipificada{{Clase: ClaseHEDoble, Horas: minutosAHoras(exceso)}}, nil
	}
	if !franja.Vigente {
		return nil, ErrFranjaNocturnaNoVigente
	}
	nocturna := ClaseHENocturna
	if jornadaMixta {
		nocturna = ClaseHEMixta
	}
	minutos := map[string]int{}
	for i := trabajados - exceso; i < trabajados; i++ {
		m := (inicio + i) % minutosPorDia
		if franja.contiene(m) {
			minutos[nocturna]++
		} else {
			minutos[ClaseHEDiurna]++
		}
	}
	var out []HoraExtraTipificada
	for _, c := range []string{ClaseHEDiurna, ClaseHENocturna, ClaseHEMixta} {
		if n := minutos[c]; n > 0 {
			out = append(out, HoraExtraTipificada{Clase: c, Horas: minutosAHoras(n)})
		}
	}
	return out, nil
}
