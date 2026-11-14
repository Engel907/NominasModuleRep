package liquidacion

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func dp(s string) *decimal.Decimal {
	v := d(s)
	return &v
}
func ip(n int) *int { return &n }
func fecha(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// Escala sembrada en 000005 + tramos < 1 año de 000134 (Código de Trabajo).
func escalaCesantia() []EscalonCesantia {
	filas := [][3]string{
		{"0.00", "0.25", "0"}, {"0.25", "0.50", "7"}, {"0.50", "1.00", "14"},
		{"1", "2", "19.5"}, {"2", "3", "20"}, {"3", "4", "20.5"}, {"4", "5", "21"},
		{"5", "6", "21.24"}, {"6", "7", "21.5"}, {"7", "8", "22"}, {"8", "9", "22"},
		{"9", "10", "22"}, {"10", "11", "21.5"}, {"11", "12", "21"},
	}
	out := make([]EscalonCesantia, 0, len(filas)+1)
	for _, f := range filas {
		out = append(out, EscalonCesantia{AniosDesde: d(f[0]), AniosHasta: dp(f[1]), DiasPorAnio: d(f[2]), TopeAnios: dp("8")})
	}
	out = append(out, EscalonCesantia{AniosDesde: d("12"), AniosHasta: dp("13"), DiasPorAnio: d("20.5"), TopeAnios: dp("8")})
	return append(out, EscalonCesantia{AniosDesde: d("13"), DiasPorAnio: d("20"), TopeAnios: dp("8")})
}

func escalaPreaviso() []EscalonPreaviso {
	return []EscalonPreaviso{
		{MesesDesde: 0, MesesHasta: ip(3), DiasPreaviso: 0},
		{MesesDesde: 3, MesesHasta: ip(6), DiasPreaviso: 7},
		{MesesDesde: 6, MesesHasta: ip(12), DiasPreaviso: 15},
		{MesesDesde: 12, DiasPreaviso: 30},
	}
}

func parametros(m ReglaMotivo) ParametrosLiquidacion {
	return ParametrosLiquidacion{
		Motivo: m, EscalaCesantia: escalaCesantia(), EscalaPreaviso: escalaPreaviso(),
		DivisorDiasMes: 30, FraccionMesesUmbral: 6, MetodoCesantia: MetodoCesantiaAcumulado,
		AguinaldoMeses: d("12"), PorcentajeSalarioEscolar: d("0.0833"),
	}
}

var (
	motivoDespido  = ReglaMotivo{PagaPreaviso: true, PagaCesantia: true, PagaVacaciones: true, PagaAguinaldo: true, PagaSalarioEscolar: true}
	motivoRenuncia = ReglaMotivo{PagaVacaciones: true, PagaAguinaldo: true, PagaSalarioEscolar: true}
)

// 6 meses de 500 000 en base_cesantia/base_vacaciones.
func seisMeses() BasePromedio {
	return BasePromedio{Total: d("3000000"), Meses: d("6"), Periodos: 6, MesesVentana: d("6")}
}

func TestDespidoConTresAnios(t *testing.T) {
	in := EntradaLiquidacion{
		FechaIngreso: fecha("2023-06-01"), FechaSalida: fecha("2026-06-01"),
		AplicaResponsabilidadPatronal: true,
		BaseCesantia:                  seisMeses(), BaseVacaciones: seisMeses(),
		DiasVacacionesPendientes: d("6"),
		BaseAguinaldo:            d("3000000"), AguinaldoDesde: fecha("2025-12-01"),
		BaseSalarioEscolar: d("2500000"), SalarioEscolarDesde: fecha("2026-01-01"),
	}
	r, err := CalcularRubros(in, parametros(motivoDespido))
	if err != nil {
		t.Fatal(err)
	}
	casos := map[string][2]string{
		"diario":     {r.SalarioDiarioPromedio.String(), "16666.6667"},
		"preaviso":   {r.MontoPreaviso.String(), "500000"},
		"diasCes":    {r.DiasCesantia.String(), "60"},
		"cesantia":   {r.MontoCesantia.String(), "1000000"},
		"vacaciones": {r.MontoVacacionesProporcionales.String(), "100000"},
		"aguinaldo":  {r.MontoAguinaldoProporcional.String(), "250000"},
		"escolar":    {r.MontoSalarioEscolarProporcional.String(), "208250"},
		"total":      {r.TotalLiquidacion.String(), "2058250"},
	}
	for n, c := range casos {
		if c[0] != c[1] {
			t.Errorf("%s = %s, esperado %s", n, c[0], c[1])
		}
	}
	if r.AntiguedadMeses != 36 || r.DiasPreaviso != 30 {
		t.Errorf("antigüedad %d, preaviso %d días", r.AntiguedadMeses, r.DiasPreaviso)
	}
	if len(r.Advertencias) != 0 {
		t.Errorf("advertencias inesperadas: %v", r.Advertencias)
	}
}

func TestRenunciaConCincoMeses(t *testing.T) {
	in := EntradaLiquidacion{
		FechaIngreso: fecha("2026-01-10"), FechaSalida: fecha("2026-06-15"),
		BaseCesantia:             BasePromedio{},
		BaseVacaciones:           BasePromedio{Total: d("2000000"), Meses: d("5"), Periodos: 5, MesesVentana: d("6")},
		DiasVacacionesPendientes: d("4"),
		BaseAguinaldo:            d("2000000"), AguinaldoDesde: fecha("2025-12-01"),
		BaseSalarioEscolar: d("2000000"), SalarioEscolarDesde: fecha("2026-01-01"),
	}
	r, err := CalcularRubros(in, parametros(motivoRenuncia))
	if err != nil {
		t.Fatal(err)
	}
	if r.AntiguedadMeses != 5 {
		t.Errorf("antigüedad %d, esperado 5", r.AntiguedadMeses)
	}
	if !r.MontoPreaviso.IsZero() || !r.MontoCesantia.IsZero() {
		t.Errorf("renuncia no paga preaviso/cesantía: %s / %s", r.MontoPreaviso, r.MontoCesantia)
	}
	if r.MontoVacacionesProporcionales.String() != "53333.33" {
		t.Errorf("vacaciones %s", r.MontoVacacionesProporcionales)
	}
	if r.MontoAguinaldoProporcional.String() != "166666.67" || r.MontoSalarioEscolarProporcional.String() != "166600" {
		t.Errorf("aguinaldo %s, escolar %s", r.MontoAguinaldoProporcional, r.MontoSalarioEscolarProporcional)
	}
	if r.TotalLiquidacion.String() != "386600" {
		t.Errorf("total %s", r.TotalLiquidacion)
	}
	if len(r.Advertencias) != 1 {
		t.Errorf("advertencias %v", r.Advertencias)
	}
}

func TestResponsabilidadEnMotivoQueNoLaAdmite(t *testing.T) {
	in := EntradaLiquidacion{FechaIngreso: fecha("2025-01-01"), FechaSalida: fecha("2026-01-01"), AplicaResponsabilidadPatronal: true}
	if _, err := CalcularRubros(in, parametros(motivoRenuncia)); !errors.Is(err, ErrResponsabilidadNoAdmitida) {
		t.Fatalf("esperaba ErrResponsabilidadNoAdmitida, fue %v", err)
	}
}

func TestDespidoSinResponsabilidadAdvierte(t *testing.T) {
	in := EntradaLiquidacion{FechaIngreso: fecha("2025-01-01"), FechaSalida: fecha("2026-01-01")}
	r, err := CalcularRubros(in, parametros(ReglaMotivo{PagaPreaviso: true, PagaCesantia: true}))
	if err != nil {
		t.Fatal(err)
	}
	if !r.TotalLiquidacion.IsZero() || len(r.Advertencias) != 1 {
		t.Errorf("total %s, advertencias %v", r.TotalLiquidacion, r.Advertencias)
	}
}

func TestCesantiaFraccionEnElUmbral(t *testing.T) {
	casos := []struct {
		meses int
		dias  string
	}{
		{2, "0"}, {3, "7"}, {5, "7"}, {6, "14"}, {11, "14"},
		{12, "19.5"},
		{18, "19.5"},
		{19, "39.5"},
		{96, "167.74"},
		{110, "167.74"},
	}
	for _, c := range casos {
		got, err := CalcularDiasCesantia(c.meses, escalaCesantia(), 6, MetodoCesantiaAcumulado)
		if err != nil {
			t.Fatalf("%d meses: %v", c.meses, err)
		}
		if got.String() != c.dias {
			t.Errorf("%d meses: %s días, esperado %s", c.meses, got, c.dias)
		}
	}
}

func TestCesantiaTasaDeAntiguedadTotal(t *testing.T) {
	casos := []struct {
		meses int
		dias  string
	}{
		{6, "14"},
		{12, "19.5"},
		{24, "40"},
		{60, "106.2"},
		{66, "106.2"},
		{67, "129"},
		{96, "176"},
		{156, "160"},
		{240, "160"},
	}
	for _, c := range casos {
		got, err := CalcularDiasCesantia(c.meses, escalaCesantia(), 6, MetodoCesantiaAntiguedadTotal)
		if err != nil {
			t.Fatalf("%d meses: %v", c.meses, err)
		}
		if got.String() != c.dias {
			t.Errorf("%d meses: %s días, esperado %s", c.meses, got, c.dias)
		}
	}
}

func TestCesantiaSinMetodoEsError(t *testing.T) {
	if _, err := CalcularDiasCesantia(24, escalaCesantia(), 6, ""); err == nil {
		t.Fatal("sin método de cesantía debe fallar")
	}
}

func TestEscalaSinTramoEsError(t *testing.T) {
	sinMenores := escalaCesantia()[3:]
	if _, err := CalcularDiasCesantia(5, sinMenores, 6, MetodoCesantiaAcumulado); err == nil {
		t.Fatal("5 meses sin tramo < 1 año debe fallar")
	}
	if _, err := CalcularDiasCesantia(30, escalaCesantia()[:4], 6, MetodoCesantiaAcumulado); err == nil {
		t.Fatal("año 2 sin escalón debe fallar")
	}
	if _, err := CalcularDiasPreaviso(13, escalaPreaviso()[:3]); err == nil {
		t.Fatal("preaviso sin tramo debe fallar")
	}
}

func TestSinPeriodosParaPromedioEsError(t *testing.T) {
	in := EntradaLiquidacion{FechaIngreso: fecha("2025-01-01"), FechaSalida: fecha("2026-01-01"), AplicaResponsabilidadPatronal: true}
	if _, err := CalcularRubros(in, parametros(motivoDespido)); !errors.Is(err, ErrSinPeriodosParaPromedio) {
		t.Fatalf("esperaba ErrSinPeriodosParaPromedio, fue %v", err)
	}
}

func TestMesesCompletosEntre(t *testing.T) {
	casos := []struct {
		a, b string
		n    int
	}{
		{"2024-01-31", "2024-02-29", 0},
		{"2024-01-15", "2024-02-15", 1},
		{"2023-06-01", "2026-06-01", 36},
		{"2023-06-02", "2026-06-01", 35},
		{"2026-06-01", "2026-05-01", 0},
	}
	for _, c := range casos {
		if got := MesesCompletosEntre(fecha(c.a), fecha(c.b)); got != c.n {
			t.Errorf("%s → %s: %d, esperado %d", c.a, c.b, got, c.n)
		}
	}
}
