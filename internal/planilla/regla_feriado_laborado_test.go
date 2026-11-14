package planilla

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func ptrDec(s string) *decimal.Decimal { d := dec(s); return &d }

func TestCalcularFeriadoLaborado(t *testing.T) {
	salario, divisor := dec("480000"), dec("240")
	casos := []struct {
		nombre            string
		feriados          []FeriadoLaboradoInput
		obligatorio, noOb *decimal.Decimal
		monto, horas      string
		faltaParametro    bool
	}{
		{"8 h en feriado obligatorio: doble", []FeriadoLaboradoInput{{PagoObligatorio: true, Horas: dec("8")}}, ptrDec("1"), nil, "16000", "8", false},
		{"no obligatorio con factor 0", []FeriadoLaboradoInput{{PagoObligatorio: false, Horas: dec("8")}}, ptrDec("1"), ptrDec("0"), "0", "8", false},
		{"ambos tipos", []FeriadoLaboradoInput{{PagoObligatorio: true, Horas: dec("8")}, {PagoObligatorio: false, Horas: dec("4")}}, ptrDec("1"), ptrDec("1"), "24000", "12", false},
		{"media jornada", []FeriadoLaboradoInput{{PagoObligatorio: true, Horas: dec("4.5")}}, ptrDec("1"), nil, "9000", "4.5", false},
		{"no obligatorio sin parámetro", []FeriadoLaboradoInput{{PagoObligatorio: false, Horas: dec("8")}}, ptrDec("1"), nil, "", "", true},
		{"obligatorio sin parámetro", []FeriadoLaboradoInput{{PagoObligatorio: true, Horas: dec("8")}}, nil, ptrDec("0"), "", "", true},
		{"sin horas no exige parámetros", []FeriadoLaboradoInput{{PagoObligatorio: false, Horas: dec("0")}}, nil, nil, "0", "0", false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			monto, horas, err := CalcularFeriadoLaborado(salario, divisor, c.feriados, c.obligatorio, c.noOb)
			if c.faltaParametro {
				if !errors.Is(err, ErrParametroFaltante) {
					t.Fatalf("se esperaba ErrParametroFaltante, se obtuvo %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if monto.String() != c.monto || horas.String() != c.horas {
				t.Fatalf("monto %s horas %s, se esperaba %s y %s", monto, horas, c.monto, c.horas)
			}
		})
	}
}

func TestCalcularFeriadoLaboradoSinDivisor(t *testing.T) {
	if _, _, err := CalcularFeriadoLaborado(dec("1"), dec("0"), nil, nil, nil); err == nil {
		t.Fatal("divisor cero debe fallar")
	}
}
