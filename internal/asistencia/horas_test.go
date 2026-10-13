package asistencia

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func hm(h, m int) time.Time { return time.Date(2000, 1, 1, h, m, 0, 0, time.UTC) }

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestCalcularHoras(t *testing.T) {
	casos := []struct {
		nombre                             string
		entProg, salProg, entReal, salReal time.Time
		tolerancia                         int
		ordinarias, exceso, tardanza       string
		minTardanza                        int
	}{
		{"diurno exacto", hm(8, 0), hm(17, 0), hm(8, 0), hm(17, 0), 0, "9", "0", "0", 0},
		{"nocturno 22-06 exacto", hm(22, 0), hm(6, 0), hm(22, 0), hm(6, 0), 0, "8", "0", "0", 0},
		{"entrada tardia 20 min", hm(8, 0), hm(17, 0), hm(8, 20), hm(17, 0), 0, "8.67", "0", "0.33", 20},
		{"entrada tardia dentro de tolerancia", hm(8, 0), hm(17, 0), hm(8, 5), hm(17, 0), 5, "8.92", "0", "0", 0},
		{"entrada tardia supera tolerancia: cuenta el total", hm(8, 0), hm(17, 0), hm(8, 6), hm(17, 0), 5, "8.90", "0", "0.10", 6},
		{"salida anticipada", hm(8, 0), hm(17, 0), hm(8, 0), hm(16, 30), 0, "8.50", "0", "0", 0},
		{"exceso sobre jornada", hm(8, 0), hm(17, 0), hm(8, 0), hm(19, 30), 0, "9", "2.50", "0", 0},
		{"entrada anticipada no es tardanza", hm(8, 0), hm(17, 0), hm(7, 45), hm(17, 0), 0, "9", "0.25", "0", 0},
		{"nocturno con exceso", hm(22, 0), hm(6, 0), hm(22, 0), hm(7, 0), 0, "8", "1", "0", 0},
		{"nocturno entrada tardia 15 min", hm(22, 0), hm(6, 0), hm(22, 15), hm(6, 0), 0, "7.75", "0", "0.25", 15},
		{"nocturno entrada 10 min antes (no 23h50 tarde)", hm(22, 0), hm(6, 0), hm(21, 50), hm(6, 0), 0, "8", "0.17", "0", 0},
		{"nocturno salida anticipada", hm(22, 0), hm(6, 0), hm(22, 0), hm(5, 0), 0, "7", "0", "0", 0},
		{"turno que termina a medianoche", hm(16, 0), hm(0, 0), hm(16, 0), hm(0, 0), 0, "8", "0", "0", 0},
		{"entrada y salida iguales", hm(8, 0), hm(17, 0), hm(8, 0), hm(8, 0), 0, "0", "0", "0", 0},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := calcularHoras(c.entProg, c.salProg, c.entReal, c.salReal, c.tolerancia)
			if !got.Ordinarias.Equal(dec(c.ordinarias)) {
				t.Errorf("ordinarias = %s, want %s", got.Ordinarias, c.ordinarias)
			}
			if !got.ExcesoJornada.Equal(dec(c.exceso)) {
				t.Errorf("exceso = %s, want %s", got.ExcesoJornada, c.exceso)
			}
			if !got.Tardanza.Equal(dec(c.tardanza)) {
				t.Errorf("tardanza = %s, want %s", got.Tardanza, c.tardanza)
			}
			if got.MinutosTardanza != c.minTardanza {
				t.Errorf("minutosTardanza = %d, want %d", got.MinutosTardanza, c.minTardanza)
			}
		})
	}
}
