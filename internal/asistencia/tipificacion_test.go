package asistencia

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func reloj(s string) time.Time {
	t, err := time.Parse("15:04", s)
	if err != nil {
		panic(err)
	}
	return t
}

// Franja nocturna de prueba (el valor real lo carga Planillas: no se siembra).
var franjaPrueba = FranjaNocturna{Vigente: true, Inicio: reloj("19:00"), Fin: reloj("05:00")}

func TestTipificarHorasExtraGolden(t *testing.T) {
	casos := []struct {
		nombre                      string
		entrada, salida, tIni, tFin string
		feriado, descanso, mixta    bool
		want                        map[string]string
	}{
		// Turno 07:00–15:00, sale 17:00: 2 h diurnas.
		{"diurna", "07:00", "17:00", "07:00", "15:00", false, false, false, map[string]string{"diurna": "2"}},
		// Turno 11:00–19:00, sale 21:30: 2.5 h de exceso, todo desde las 19:00 → nocturna.
		{"cruza a nocturna", "11:00", "21:30", "11:00", "19:00", false, false, false, map[string]string{"nocturna": "2.5"}},
		// Turno 10:00–18:00, sale 20:00: 18–19 diurna, 19–20 nocturna.
		{"mitad y mitad", "10:00", "20:00", "10:00", "18:00", false, false, false, map[string]string{"diurna": "1", "nocturna": "1"}},
		// Feriado: todo doble.
		{"feriado", "07:00", "17:00", "07:00", "15:00", true, false, false, map[string]string{"doble": "2"}},
		// Día de descanso: todo doble.
		{"descanso", "07:00", "16:00", "07:00", "15:00", false, true, false, map[string]string{"doble": "1"}},
		// Turno nocturno 22:00–06:00 (cruza medianoche), sale 08:00: 06–08 diurna.
		{"turno que cruza medianoche", "22:00", "08:00", "22:00", "06:00", false, false, false, map[string]string{"diurna": "2"}},
		// Turno 21:00–03:00 (6 h), sale 04:30: 1.5 h dentro de la franja.
		{"exceso nocturno tras medianoche", "21:00", "04:30", "21:00", "03:00", false, false, false, map[string]string{"nocturna": "1.5"}},
		// Jornada mixta: el tramo nocturno se tipifica como mixta.
		{"jornada mixta", "10:00", "20:00", "10:00", "18:00", false, false, true, map[string]string{"diurna": "1", "mixta": "1"}},
		// Llega 1 h tarde y sale 1 h tarde: sin exceso.
		{"tarde y tarde", "08:00", "16:00", "07:00", "15:00", false, false, false, map[string]string{}},
	}
	for _, c := range casos {
		got, err := TipificarHorasExtra(reloj(c.entrada), reloj(c.salida), reloj(c.tIni), reloj(c.tFin), franjaPrueba, c.feriado, c.descanso, c.mixta)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: %+v, esperado %v", c.nombre, got, c.want)
			continue
		}
		for _, h := range got {
			if w, ok := c.want[h.Clase]; !ok || !h.Horas.Equal(decimal.RequireFromString(w)) {
				t.Errorf("%s: %s = %s, esperado %v", c.nombre, h.Clase, h.Horas, c.want)
			}
		}
	}
}

func TestTipificarSinFranjaVigente(t *testing.T) {
	_, err := TipificarHorasExtra(reloj("07:00"), reloj("17:00"), reloj("07:00"), reloj("15:00"), FranjaNocturna{}, false, false, false)
	if !errors.Is(err, ErrFranjaNocturnaNoVigente) {
		t.Fatalf("sin franja vigente debe dar error: %v", err)
	}
	// Feriado y sin exceso no necesitan franja.
	if _, err := TipificarHorasExtra(reloj("07:00"), reloj("17:00"), reloj("07:00"), reloj("15:00"), FranjaNocturna{}, true, false, false); err != nil {
		t.Errorf("feriado sin franja: %v", err)
	}
	if got, err := TipificarHorasExtra(reloj("07:00"), reloj("15:00"), reloj("07:00"), reloj("15:00"), FranjaNocturna{}, false, false, false); err != nil || got != nil {
		t.Errorf("sin exceso: %v, %v", got, err)
	}
}
