package planilla

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// la CCSS cotiza sobre max(base real, base mínima del régimen × fracción).
func TestCCSSBaseMinimaGolden(t *testing.T) {
	repo := repoBase()
	repo.contrato.SalarioPactado = d("150000")
	casos := []struct {
		nombre         string
		salario        string
		dias           int
		fraccion       string
		base, obr, pat string
		netoEsperado   string
	}{
		{"bajo la mínima", "150000", 30, "1", "232441", "25173.36", "62363.92", "124826.64"},
		{"sobre la mínima", "1000000", 30, "1", "1000000", "108300", "268300", "883500"},
		{"quincena", "200000", 15, "0.5", "116220.5", "12586.68", "31181.96", "87413.32"},
	}
	for _, c := range casos {
		p := params()
		p.FraccionMensualPeriodo = d(c.fraccion)
		p.RentaMetodoPeriodo = RentaAcumuladoMes
		p.DiasPeriodoCompleto = d(c.fraccion).Mul(d("30"))
		in := CalculoEmpleadoInput{EmpleadoID: 7, RegimenCCSSID: 1, FechaEfectiva: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			DiasTrabajados: decimal.NewFromInt(int64(c.dias)),
			Segmentos:      []SegmentoSalario{{ContratoID: 1, Salario: d(c.salario), Dias: c.dias}}}
		r, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if !r.BaseCCSS.Equal(d(c.base)) || !r.CuotaObreroCCSS.Equal(d(c.obr)) || !r.CuotaPatronalCCSS.Equal(d(c.pat)) {
			t.Errorf("%s: base %s obrera %s patronal %s; esperado %s/%s/%s", c.nombre, r.BaseCCSS, r.CuotaObreroCCSS, r.CuotaPatronalCCSS, c.base, c.obr, c.pat)
		}
		if !r.NetoPagado.Equal(d(c.netoEsperado)) {
			t.Errorf("%s: neto %s, esperado %s", c.nombre, r.NetoPagado, c.netoEsperado)
		}
		if len(r.Patronal) != 1 || !r.Patronal[0].Base.Equal(d(c.base)) {
			t.Errorf("%s: línea patronal %+v, base esperada %s", c.nombre, r.Patronal, c.base)
		}
	}
}

func TestCCSSSinRegimenOSinBaseMinima(t *testing.T) {
	in := CalculoEmpleadoInput{EmpleadoID: 7, FechaEfectiva: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("30"), Segmentos: mesCompleto()}
	if _, err := NewMotorEmpleado(repoBase()).CalcularEmpleado(context.Background(), in, params()); err == nil {
		t.Fatal("empleado sin régimen CCSS debe dar error")
	}
	in.RegimenCCSSID = 2
	if _, err := NewMotorEmpleado(repoBase()).CalcularEmpleado(context.Background(), in, params()); err == nil {
		t.Fatal("régimen sin base mínima vigente debe dar error")
	}
}
