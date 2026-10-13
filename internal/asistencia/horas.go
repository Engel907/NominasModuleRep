package asistencia

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	minutosPorHora = 60
	minutosPorDia  = 24 * minutosPorHora
)

// ResultadoHoras es el resultado del cálculo de una jornada contra su turno.
type ResultadoHoras struct {
	// Ordinarias: horas trabajadas hasta completar la jornada programada.
	Ordinarias decimal.Decimal
	// ExcesoJornada: tiempo trabajado por encima de la jornada programada.
	// Es una CANDIDATA a hora extra: queda pendiente de autorización y de
	// tipificación (diurna/nocturna/mixta) en F22. Esta fase no decide si se
	// paga ni a qué tarifa. Se persiste en registro_asistencia_detalle.horas_extra.
	ExcesoJornada decimal.Decimal
	// Tardanza en horas (2 decimales) y en minutos enteros.
	Tardanza        decimal.Decimal
	MinutosTardanza int
}

// minutosDelDia devuelve los minutos transcurridos desde las 00:00 en la hora
// de reloj de t. Las columnas TIME no llevan fecha, así que solo cuenta
// hh:mm; los segundos se truncan.
func minutosDelDia(t time.Time) int {
	return t.Hour()*minutosPorHora + t.Minute()
}

// duracionMinutos es el tiempo entre dos horas de reloj. Si fin < inicio se
// asume que la jornada cruza la medianoche y se suman 24 h. fin == inicio
// da 0 (no se puede representar una jornada de 24 h exactas).
func duracionMinutos(inicio, fin int) int {
	d := fin - inicio
	if d < 0 {
		d += minutosPorDia
	}
	return d
}

// desfaseMinutos devuelve real-esperado normalizado a [-12h, +12h), para
// que un turno 22:00–06:00 trate una entrada 21:50 como 10 min antes y no
// como 23 h 50 min tarde.
func desfaseMinutos(esperado, real int) int {
	d := (real - esperado) % minutosPorDia
	if d < 0 {
		d += minutosPorDia
	}
	if d >= minutosPorDia/2 {
		d -= minutosPorDia
	}
	return d
}

// minutosAHoras convierte minutos enteros a horas con 2 decimales, sin float.
func minutosAHoras(m int) decimal.Decimal {
	return decimal.NewFromInt(int64(m)).DivRound(decimal.NewFromInt(minutosPorHora), 2)
}

// calcularHoras compara la jornada real contra la programada del turno.
// toleranciaMin: si la entrada tardía es <= tolerancia no se registra
// tardanza; si la supera se registra el total de minutos de atraso (no solo
// el exceso sobre la tolerancia).
func calcularHoras(entradaTurno, salidaTurno, entradaReal, salidaReal time.Time, toleranciaMin int) ResultadoHoras {
	entradaProg, salidaProg := minutosDelDia(entradaTurno), minutosDelDia(salidaTurno)
	entradaR, salidaR := minutosDelDia(entradaReal), minutosDelDia(salidaReal)

	res := ResultadoHoras{Tardanza: decimal.Zero}
	if atraso := desfaseMinutos(entradaProg, entradaR); atraso > toleranciaMin && atraso > 0 {
		res.MinutosTardanza = atraso
		res.Tardanza = minutosAHoras(atraso)
	}

	programada := duracionMinutos(entradaProg, salidaProg)
	real := duracionMinutos(entradaR, salidaR)

	ordinarias, exceso := real, 0
	if real > programada {
		ordinarias, exceso = programada, real-programada
	}
	res.Ordinarias = minutosAHoras(ordinarias)
	res.ExcesoJornada = minutosAHoras(exceso)
	return res
}

// calcularHorasCero es el resultado para días sin jornada calculable
// (ausencias, sin turno vigente, o sin hora de entrada/salida).
func calcularHorasCero() ResultadoHoras {
	return ResultadoHoras{Ordinarias: decimal.Zero, ExcesoJornada: decimal.Zero, Tardanza: decimal.Zero}
}
