package asistencia

import (
	"testing"
	"time"
)

func dia(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// la política de acumulación configurable da un período estable por día.
func TestPlanificarAcumulacion(t *testing.T) {
	casos := []struct {
		politica, ingreso, fecha string
		ok                       bool
		periodo, efectiva, frac  string
	}{
		{PoliticaAniversario, "2024-03-15", "2026-03-14", true, "ANIV-1", "2025-03-15", "1"},
		{PoliticaAniversario, "2024-03-15", "2026-03-15", true, "ANIV-2", "2026-03-15", "1"},
		{PoliticaAniversario, "2026-01-10", "2026-12-31", false, "", "", ""},
		{PoliticaAnioCalendario, "2025-01-01", "2026-01-02", true, "2025", "2025-12-31", "1"},
		{PoliticaAnioCalendario, "2025-02-01", "2026-01-02", false, "", "", ""},
		{PoliticaMensual, "2026-02-01", "2026-03-20", true, "2026-02", "2026-02-28", "0.0833333333333333"},
		{PoliticaMensual, "2026-02-02", "2026-03-20", false, "", "", ""},
		{PoliticaMensual, "2020-01-01", "2026-01-05", true, "2025-12", "2025-12-31", "0.0833333333333333"},
	}
	for _, c := range casos {
		p, err := PlanificarAcumulacion(c.politica, dia(c.ingreso), dia(c.fecha))
		if err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		if p.Ok != c.ok {
			t.Errorf("%s %s→%s: ok=%v, esperado %v", c.politica, c.ingreso, c.fecha, p.Ok, c.ok)
			continue
		}
		if !c.ok {
			continue
		}
		if p.PeriodoReferencia != c.periodo || p.FechaEfectiva.Format("2006-01-02") != c.efectiva || p.Fraccion.String() != c.frac {
			t.Errorf("%s %s→%s: %+v (esperado %s, %s, %s)", c.politica, c.ingreso, c.fecha, p, c.periodo, c.efectiva, c.frac)
		}
	}
	a, _ := PlanificarAcumulacion(PoliticaMensual, dia("2020-01-01"), dia("2026-03-01"))
	b, _ := PlanificarAcumulacion(PoliticaMensual, dia("2020-01-01"), dia("2026-03-31"))
	if a.PeriodoReferencia != b.PeriodoReferencia {
		t.Errorf("mismo mes, distinto período: %s vs %s", a.PeriodoReferencia, b.PeriodoReferencia)
	}
	if _, err := PlanificarAcumulacion("diaria", dia("2020-01-01"), dia("2026-03-01")); err == nil {
		t.Error("política desconocida debe dar error")
	}
}
