package porcentaje

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestDesdePorcentaje(t *testing.T) {
	casos := []struct {
		nombre   string
		entrada  string
		fraccion string
		err      bool
	}{
		{"CCSS obrera", "10.83", "0.1083", false},
		{"CCSS patronal", "26.83", "0.2683", false},
		{"salario escolar 1/12 truncado", "8.33", "0.0833", false},
		{"embargo 1/8", "12.5", "0.125", false},
		{"embargo 1/4", "25.00", "0.25", false},
		{"cero", "0", "0", false},
		{"cien", "100", "1", false},
		{"negativo", "-0.01", "", true},
		{"mayor a cien", "100.01", "", true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p, err := DesdePorcentaje(dec(c.entrada))
			if c.err {
				if !errors.Is(err, ErrFueraDeRango) {
					t.Fatalf("se esperaba ErrFueraDeRango, llegó %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !p.Fraccion().Equal(dec(c.fraccion)) {
				t.Errorf("Fraccion() = %s, se esperaba %s", p.Fraccion(), c.fraccion)
			}
			if !p.Porcentaje().Equal(dec(c.entrada)) {
				t.Errorf("Porcentaje() = %s, se esperaba %s", p.Porcentaje(), c.entrada)
			}
		})
	}
}

func TestDesdeFraccion(t *testing.T) {
	casos := []struct {
		nombre     string
		entrada    string
		porcentaje string
		err        bool
	}{
		{"CCSS obrera", "0.1083", "10.83", false},
		{"salario escolar", "0.0833", "8.33", false},
		{"cero", "0", "0", false},
		{"uno", "1", "100", false},
		{"negativa", "-0.1", "", true},
		{"mayor a uno", "1.0001", "", true},
		{"confusión típica: pasar porcentaje como fracción", "10.83", "", true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p, err := DesdeFraccion(dec(c.entrada))
			if c.err {
				if !errors.Is(err, ErrFueraDeRango) {
					t.Fatalf("se esperaba ErrFueraDeRango, llegó %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !p.Porcentaje().Equal(dec(c.porcentaje)) {
				t.Errorf("Porcentaje() = %s, se esperaba %s", p.Porcentaje(), c.porcentaje)
			}
			if !p.Fraccion().Equal(dec(c.entrada)) {
				t.Errorf("Fraccion() = %s, se esperaba %s", p.Fraccion(), c.entrada)
			}
		})
	}
}

// La conversión no redondea: ida y vuelta devuelve exactamente el valor original.
func TestIdaYVueltaExacta(t *testing.T) {
	for _, s := range []string{"0", "0.0001", "8.3333", "10.83", "12.5", "26.83", "99.9999", "100"} {
		p, err := DesdePorcentaje(dec(s))
		if err != nil {
			t.Fatal(err)
		}
		q, err := DesdeFraccion(p.Fraccion())
		if err != nil {
			t.Fatal(err)
		}
		if !q.Porcentaje().Equal(dec(s)) {
			t.Errorf("%s -> %s", s, q.Porcentaje())
		}
	}
}

// Efecto en el cálculo: el redondeo a 2 decimales lo decide el llamador sobre
// el monto, nunca sobre el porcentaje.
func TestAplicarSobreMonto(t *testing.T) {
	casos := []struct {
		base, pct, esperado string
	}{
		{"500000", "10.83", "54150.00"},
		{"431288.73", "12.5", "53911.09"}, // caso de calculator_test.go: fondo embargo 1/8
		{"123456.78", "8.33", "10283.95"},
	}
	for _, c := range casos {
		p, _ := DesdePorcentaje(dec(c.pct))
		got := dec(c.base).Mul(p.Fraccion()).Round(2)
		if !got.Equal(dec(c.esperado)) {
			t.Errorf("%s * %s%% = %s, se esperaba %s", c.base, c.pct, got, c.esperado)
		}
	}
}

func TestValorCeroEsValido(t *testing.T) {
	var p Porcentaje
	if !p.Fraccion().IsZero() || !p.Porcentaje().IsZero() {
		t.Error("el valor cero debe ser 0 %")
	}
	if got := (Porcentaje{}).String(); got != "0%" {
		t.Errorf("String() = %q", got)
	}
}