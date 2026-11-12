package planilla

import (
	"context"
	"testing"
	"time"
)

// Escala sembrada en 000005 (CCSS = entidad 1): días 1–3 patrono 50 %;
// día 4 en adelante (sin dia_hasta) CCSS 60 %.
func tramosCCSS() []TramoSubsidio {
	tres := int32(3)
	return []TramoSubsidio{
		{EntidadEmisoraID: 1, DiaDesde: 1, DiaHasta: &tres, PctPatronal: d("0.5"), PctEntidad: d("0")},
		{EntidadEmisoraID: 1, DiaDesde: 4, PctPatronal: d("0"), PctEntidad: d("0.6")},
	}
}

func TestCalcularSubsidioIncapacidadGolden(t *testing.T) {
	diario := d("30000") // 900 000 / 30
	casos := []struct {
		nombre               string
		desde, hasta         string
		serie                int32
		dias                 int
		patronal, entidad    string
		lineas               int
		primeraSerieHastaEsp int32
	}{
		// 2 días (serie 1–2): 30 000 × 2 × 50 % = 30 000
		{"2 días dentro del período", "2026-03-10", "2026-03-11", 1, 2, "30000", "0", 1, 2},
		// Marzo de una boleta 25-mar..3-abr: serie 1–7. Días 1–3 → patrono
		// 30 000 × 3 × 50 % = 45 000; días 4–7 (28, 29, 30, 31; el 31 vale 0) →
		// CCSS 30 000 × 3 × 60 % = 54 000.
		{"boleta que cruza: parte de marzo", "2026-03-25", "2026-03-31", 1, 6, "45000", "54000", 2, 3},
		// Abril de la misma boleta: serie 8–10, todo tramo abierto (sin dia_hasta).
		{"boleta que cruza: parte de abril (serie continua)", "2026-04-01", "2026-04-03", 8, 3, "0", "54000", 1, 10},
		// Todo marzo: 3 días patrono (45 000) + 27 días CCSS × 30 000 × 60 % = 486 000.
		{"cubre todo el período", "2026-03-01", "2026-03-31", 1, 30, "45000", "486000", 2, 3},
	}
	for _, c := range casos {
		inc := IncapacidadInput{IncapacidadID: 9, EntidadEmisoraID: 1, Desde: fecha(c.desde), Hasta: fecha(c.hasta), DiaSerieDesde: c.serie}
		s, err := CalcularSubsidioIncapacidad(diario, inc, tramosCCSS(), 30)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if s.Dias != c.dias || !s.MontoPatronal.Equal(d(c.patronal)) || !s.MontoEntidadInformativo.Equal(d(c.entidad)) {
			t.Errorf("%s: días %d patronal %s entidad %s; esperado %d / %s / %s",
				c.nombre, s.Dias, s.MontoPatronal, s.MontoEntidadInformativo, c.dias, c.patronal, c.entidad)
		}
		if len(s.Desglose) != c.lineas || s.Desglose[0].DiaSerieDesde != c.serie || s.Desglose[0].DiaSerieHasta != c.primeraSerieHastaEsp {
			t.Errorf("%s: desglose %+v", c.nombre, s.Desglose)
		}
	}
}

func TestCalcularSubsidioIncapacidadSinTramo(t *testing.T) {
	tres := int32(3)
	soloTres := []TramoSubsidio{{EntidadEmisoraID: 1, DiaDesde: 1, DiaHasta: &tres, PctPatronal: d("0.5")}}
	inc := IncapacidadInput{IncapacidadID: 9, EntidadEmisoraID: 1, Desde: fecha("2026-03-10"), Hasta: fecha("2026-03-14"), DiaSerieDesde: 1}
	if _, err := CalcularSubsidioIncapacidad(d("30000"), inc, soloTres, 30); err == nil {
		t.Fatal("el día 4 sin tramo vigente debe dar error")
	}
	otraEntidad := inc
	otraEntidad.EntidadEmisoraID = 2
	if _, err := CalcularSubsidioIncapacidad(d("30000"), otraEntidad, tramosCCSS(), 30); err == nil {
		t.Fatal("entidad sin tramos debe dar error")
	}
}

func TestSegmentosNetosRechazaBoletasSolapadas(t *testing.T) {
	segs := []SegmentoSalario{{Inicio: fecha("2026-03-01"), Fin: fecha("2026-03-31"), Salario: d("1"), Dias: 30}}
	incs := []IncapacidadInput{
		{IncapacidadID: 1, Desde: fecha("2026-03-01"), Hasta: fecha("2026-03-05")},
		{IncapacidadID: 2, Desde: fecha("2026-03-05"), Hasta: fecha("2026-03-06")},
	}
	if _, err := SegmentosNetos(segs, incs, 30); err == nil {
		t.Fatal("dos boletas el mismo día deben dar error")
	}
}

// Motor completo: el salario base descuenta los días incapacitados y
// SUB_INCAP paga solo la parte patronal.
func TestCalcularEmpleadoConIncapacidad(t *testing.T) {
	repo := repoBase()
	repo.contrato.SalarioPactado = d("900000")
	repo.conceptos = append(repo.conceptos, fila(6, "SUB_INCAP", "ingreso", 60, `{"tipo":"incapacidad"}`, false, false))
	p := params()
	p.SubsidiosIncapacidad = tramosCCSS()
	seg := []SegmentoSalario{{ContratoID: 1, Inicio: fecha("2026-03-01"), Fin: fecha("2026-03-31"), Salario: d("900000"), Dias: 30}}
	casos := []struct {
		nombre             string
		desde, hasta       string
		diasTrab           string
		salBase, subIncap  string
		lineasIncapacidad  int
		totalIngresosEsper string
	}{
		// 900 000 × 28/30 = 840 000; SUB_INCAP 30 000
		{"2 días", "2026-03-10", "2026-03-11", "28", "840000", "30000", 1, "870000"},
		// salario 0; SUB_INCAP solo días 1–3 = 45 000 (los 27 de CCSS no suman)
		{"todo el período", "2026-03-01", "2026-03-31", "0", "0", "45000", 2, "45000"},
	}
	for _, c := range casos {
		in := CalculoEmpleadoInput{EmpleadoID: 7, FechaEfectiva: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			Segmentos: seg, DiasTrabajados: d(c.diasTrab),
			Incapacidades: []IncapacidadInput{{IncapacidadID: 9, EntidadEmisoraID: 1, Desde: fecha(c.desde), Hasta: fecha(c.hasta), DiaSerieDesde: 1}}}
		r, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if !r.SalarioBasePeriodo.Equal(d(c.salBase)) || !r.TotalIngresos.Equal(d(c.totalIngresosEsper)) || len(r.Incapacidades) != c.lineasIncapacidad {
			t.Errorf("%s: salario base %s, ingresos %s, líneas %d", c.nombre, r.SalarioBasePeriodo, r.TotalIngresos, len(r.Incapacidades))
		}
		var sub string
		for _, l := range r.DesglosePorConcepto {
			if l.Codigo == "SUB_INCAP" {
				sub = l.Monto.String()
			}
		}
		if sub != c.subIncap {
			t.Errorf("%s: SUB_INCAP = %q, esperado %s", c.nombre, sub, c.subIncap)
		}
	}

	// DiasTrabajados que no descuenta la incapacidad: error (no se paga dos veces).
	in := CalculoEmpleadoInput{EmpleadoID: 7, FechaEfectiva: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), Segmentos: seg, DiasTrabajados: d("30"),
		Incapacidades: []IncapacidadInput{{IncapacidadID: 9, EntidadEmisoraID: 1, Desde: fecha("2026-03-10"), Hasta: fecha("2026-03-11"), DiaSerieDesde: 1}}}
	if _, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p); err == nil {
		t.Fatal("DiasTrabajados sin descontar la incapacidad debe dar error")
	}
}
