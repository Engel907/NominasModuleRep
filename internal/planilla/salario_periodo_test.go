package planilla

import (
	"context"
	"testing"
	"time"

	"planillas-ccdr/internal/db/dto"
)

func TestDiasComerciales(t *testing.T) {
	casos := []struct {
		desde, hasta string
		want         int
	}{
		{"2026-03-01", "2026-03-31", 30}, // mes de 31: el 31 vale 30
		{"2026-04-01", "2026-04-30", 30},
		{"2026-02-01", "2026-02-28", 30}, // febrero completo
		{"2028-02-01", "2028-02-29", 30}, // bisiesto
		{"2026-03-01", "2026-03-15", 15}, // primera quincena
		{"2026-03-16", "2026-03-31", 15}, // segunda quincena de mes de 31
		{"2026-02-16", "2026-02-28", 15}, // segunda quincena de febrero
		{"2026-04-10", "2026-04-30", 21}, // ingreso el 10
		{"2026-04-01", "2026-04-20", 20}, // cese el 20
		{"2026-03-31", "2026-03-31", 0},  // el 31 no suma (el 30 ya es el día 30)
		{"2026-02-28", "2026-02-28", 3},  // último de febrero = días 28, 29 y 30
		{"2026-01-16", "2026-02-15", 30}, // cruza meses
		{"2026-03-10", "2026-03-09", 0},
	}
	for _, c := range casos {
		if got := DiasComerciales(fecha(c.desde), fecha(c.hasta), 30); got != c.want {
			t.Errorf("DiasComerciales(%s, %s) = %d, esperado %d", c.desde, c.hasta, got, c.want)
		}
	}
	// Cualquier partición de un mes suma el divisor.
	for _, corte := range []string{"2026-03-10", "2026-03-15", "2026-03-30", "2026-03-31"} {
		c := fecha(corte)
		a := DiasComerciales(fecha("2026-03-01"), c.AddDate(0, 0, -1), 30)
		b := DiasComerciales(c, fecha("2026-03-31"), 30)
		if a+b != 30 {
			t.Errorf("partición en %s: %d + %d ≠ 30", corte, a, b)
		}
	}
}

func TestSalarioBasePeriodoGolden(t *testing.T) {
	marzoIni, marzoFin := fecha("2026-03-01"), fecha("2026-03-31")
	abierto := time.Time{}
	casos := []struct {
		nombre      string
		desde, hast time.Time
		vig         []VigenciaContrato
		wantDias    int
		wantSalario string
	}{
		{"mes completo", marzoIni, marzoFin,
			[]VigenciaContrato{{1, fecha("2020-01-01"), abierto, d("900000")}}, 30, "900000.00"},
		// 900 000 × 21 / 30 = 630 000
		{"ingreso día 10", marzoIni, marzoFin,
			[]VigenciaContrato{{1, fecha("2026-03-10"), abierto, d("900000")}}, 21, "630000.00"},
		// 900 000 × 15/30 + 1 000 000 × 15/30 = 450 000 + 500 000
		{"aumento día 16", marzoIni, marzoFin, []VigenciaContrato{
			{1, fecha("2020-01-01"), fecha("2026-03-15"), d("900000")},
			{2, fecha("2026-03-16"), abierto, d("1000000")}}, 30, "950000.00"},
		// Convención de CerrarContratoVigente: la versión vieja cierra el mismo
		// día 16 en que abre la nueva; el 16 es de la nueva.
		{"aumento día 16 con frontera compartida", marzoIni, marzoFin, []VigenciaContrato{
			{1, fecha("2020-01-01"), fecha("2026-03-16"), d("900000")},
			{2, fecha("2026-03-16"), abierto, d("1000000")}}, 30, "950000.00"},
		// 900 000 × 20 / 30 = 600 000
		{"cese día 20", marzoIni, marzoFin,
			[]VigenciaContrato{{1, fecha("2020-01-01"), fecha("2026-03-20"), d("900000")}}, 20, "600000.00"},
		{"febrero completo = salario mensual exacto", fecha("2026-02-01"), fecha("2026-02-28"),
			[]VigenciaContrato{{1, fecha("2020-01-01"), abierto, d("777777.77")}}, 30, "777777.77"},
		{"quincena 16–31 = medio salario", fecha("2026-03-16"), marzoFin,
			[]VigenciaContrato{{1, fecha("2020-01-01"), abierto, d("900000")}}, 15, "450000.00"},
		// 1 000 000 × 7 / 30 = 233 333.333… → un solo redondeo al final
		{"redondeo único", fecha("2026-04-01"), fecha("2026-04-07"),
			[]VigenciaContrato{{1, fecha("2020-01-01"), abierto, d("1000000")}}, 7, "233333.33"},
	}
	for _, c := range casos {
		segs, err := SegmentarPeriodo(c.desde, c.hast, c.vig, 30)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if got := DiasSegmentos(segs); got != c.wantDias {
			t.Errorf("%s: días = %d, esperado %d", c.nombre, got, c.wantDias)
		}
		if got := SalarioBasePeriodo(segs, d("30")); got.StringFixed(2) != c.wantSalario {
			t.Errorf("%s: salario = %s, esperado %s", c.nombre, got.StringFixed(2), c.wantSalario)
		}
	}
}

func TestSegmentarPeriodoErrores(t *testing.T) {
	solapados := []VigenciaContrato{
		{1, fecha("2020-01-01"), fecha("2026-03-20"), d("1")},
		{2, fecha("2026-03-15"), time.Time{}, d("1")},
	}
	if _, err := SegmentarPeriodo(fecha("2026-03-01"), fecha("2026-03-31"), solapados, 30); err == nil {
		t.Error("vigencias solapadas deben dar error")
	}
	if _, err := SegmentarPeriodo(fecha("2026-03-01"), fecha("2026-03-31"), nil, 0); err == nil {
		t.Error("divisor 0 debe dar error")
	}
	segs, err := SegmentarPeriodo(fecha("2026-03-01"), fecha("2026-03-31"),
		[]VigenciaContrato{{1, fecha("2020-01-01"), fecha("2026-02-28"), d("1")}}, 30)
	if err != nil || len(segs) != 0 {
		t.Errorf("contrato terminado antes del período: %v, %v", segs, err)
	}
	if _, err := divisorEntero(d("30.5")); err == nil {
		t.Error("DIVISOR_DIAS_MES no entero debe dar error")
	}
}

// Cese el 20 con la versión de contrato aún abierta: el segmento termina en
// fecha_fin_contrato y la fecha efectiva es el día del cese.
func TestHidratadorCeseAMitadDePeriodo(t *testing.T) {
	f := fuentePrueba()
	cese := fecha("2026-03-20")
	f.rangos = map[int64][]dto.ContratoEnRangoRow{1: {{ContratoID: 101, SalarioPactado: d("900000"),
		FechaInicioVigencia: fecha("2020-01-01"), FechaInicioContrato: fecha("2020-01-01"), FechaFinContrato: &cese}}}
	e, err := NewHidratador(f).Hidratar(context.Background(), periodoPrueba(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Input.FechaEfectiva.Equal(cese) || !e.Input.DiasTrabajados.Equal(d("20")) || e.ContratoID != 101 {
		t.Errorf("fecha efectiva %s, días %s, contrato %d", e.Input.FechaEfectiva.Format("2006-01-02"), e.Input.DiasTrabajados, e.ContratoID)
	}
}
