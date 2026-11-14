package asistencia

import (
	"errors"
	"testing"

	"planillas-ccdr/internal/db/dto"
)

func ent(n int) *int { return &n }

func fijo(nombre string, mes, d int, obligatorio bool) dto.FeriadoCatalogo {
	return dto.FeriadoCatalogo{Nombre: nombre, Tipo: feriadoFijo, Mes: ent(mes), Dia: ent(d),
		PagoObligatorio: obligatorio, FuenteLegal: "CT art. 148", AnioDesde: 2000, Activo: true}
}

func movil(nombre string, desplazamiento int, obligatorio bool) dto.FeriadoCatalogo {
	return dto.FeriadoCatalogo{Nombre: nombre, Tipo: feriadoMovil, DesplazamientoPascua: ent(desplazamiento),
		PagoObligatorio: obligatorio, FuenteLegal: "CT art. 148", AnioDesde: 2000, Activo: true}
}

func TestPascuaOccidental(t *testing.T) {
	for anio, esperado := range map[int]string{2024: "2024-03-31", 2025: "2025-04-20", 2026: "2026-04-05", 2027: "2027-03-28"} {
		if got := pascuaOccidental(anio).Format("2006-01-02"); got != esperado {
			t.Errorf("Pascua %d = %s, se esperaba %s", anio, got, esperado)
		}
	}
}

func TestResolverFeriados2026(t *testing.T) {
	catalogo := []dto.FeriadoCatalogo{
		movil("Jueves Santo", -3, true),
		movil("Viernes Santo", -2, true),
		fijo("Persona Negra", 8, 31, false),
		fijo("Navidad", 12, 25, true),
	}
	resueltos, err := ResolverFeriadosAnio(2026, catalogo)
	if err != nil {
		t.Fatal(err)
	}
	esperado := []struct {
		fecha       string
		obligatorio bool
	}{{"2026-04-02", true}, {"2026-04-03", true}, {"2026-08-31", false}, {"2026-12-25", true}}
	if len(resueltos) != len(esperado) {
		t.Fatalf("%d feriados, se esperaban %d", len(resueltos), len(esperado))
	}
	for i, e := range esperado {
		if got := resueltos[i].Fecha.Format("2006-01-02"); got != e.fecha || resueltos[i].PagoObligatorio != e.obligatorio {
			t.Errorf("feriado %d = %s (obligatorio %t), se esperaba %s (%t)", i, got, resueltos[i].PagoObligatorio, e.fecha, e.obligatorio)
		}
	}
}

func TestResolverFeriadosFusionaLaMismaFecha(t *testing.T) {
	resueltos, err := ResolverFeriadosAnio(2026, []dto.FeriadoCatalogo{
		fijo("Local", 4, 2, false),
		movil("Jueves Santo", -3, true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resueltos) != 1 || !resueltos[0].PagoObligatorio || resueltos[0].Nombre != "Local / Jueves Santo" || resueltos[0].Tipo != feriadoFijo {
		t.Fatalf("fusión inesperada: %+v", resueltos)
	}
}

func TestResolverFeriadosOmite29DeFebreroEnAnioNoBisiesto(t *testing.T) {
	resueltos, err := ResolverFeriadosAnio(2026, []dto.FeriadoCatalogo{fijo("Bisiesto", 2, 29, false)})
	if err != nil || len(resueltos) != 0 {
		t.Fatalf("resueltos %+v, err %v", resueltos, err)
	}
	resueltos, err = ResolverFeriadosAnio(2028, []dto.FeriadoCatalogo{fijo("Bisiesto", 2, 29, false)})
	if err != nil || len(resueltos) != 1 {
		t.Fatalf("2028: resueltos %+v, err %v", resueltos, err)
	}
}

func TestParsearDiasLaborales(t *testing.T) {
	dias, err := ParsearDiasLaborales("1, 2,3,4,5,6")
	if err != nil || len(dias) != 6 || !dias[6] || dias[7] {
		t.Fatalf("dias %v, err %v", dias, err)
	}
	for _, invalido := range []string{"", "0", "8", "1,1", "lunes", "1,,2"} {
		if _, err := ParsearDiasLaborales(invalido); !errors.Is(err, ErrDiasLaboralesInvalidos) {
			t.Errorf("%q: se esperaba ErrDiasLaboralesInvalidos, se obtuvo %v", invalido, err)
		}
	}
}

func TestValidarFeriadoCatalogo(t *testing.T) {
	validoFijo, validoMovil := fijo("A", 9, 15, true), movil("B", -2, true)
	if err := ValidarFeriadoCatalogo(validoFijo); err != nil {
		t.Fatal(err)
	}
	if err := ValidarFeriadoCatalogo(validoMovil); err != nil {
		t.Fatal(err)
	}
	casos := map[string]func(*dto.FeriadoCatalogo){
		"sin nombre":         func(f *dto.FeriadoCatalogo) { f.Nombre = " " },
		"sin fuente":         func(f *dto.FeriadoCatalogo) { f.FuenteLegal = "" },
		"fecha imposible":    func(f *dto.FeriadoCatalogo) { f.Mes, f.Dia = ent(2), ent(30) },
		"mes fuera de rango": func(f *dto.FeriadoCatalogo) { f.Mes = ent(13) },
		"fijo con pascua":    func(f *dto.FeriadoCatalogo) { f.DesplazamientoPascua = ent(1) },
		"rango de años":      func(f *dto.FeriadoCatalogo) { f.AnioHasta = ent(1999) },
		"tipo desconocido":   func(f *dto.FeriadoCatalogo) { f.Tipo = "otro" },
		"fijo sin día":       func(f *dto.FeriadoCatalogo) { f.Dia = nil },
		"nombre demasiado largo": func(f *dto.FeriadoCatalogo) {
			f.Nombre = string(make([]rune, 101))
		},
	}
	for nombre, mutar := range casos {
		f := validoFijo
		mutar(&f)
		if err := ValidarFeriadoCatalogo(f); !errors.Is(err, ErrFeriadoCatalogoInvalido) {
			t.Errorf("%s: se esperaba ErrFeriadoCatalogoInvalido, se obtuvo %v", nombre, err)
		}
	}
	m := validoMovil
	m.Mes = ent(1)
	if err := ValidarFeriadoCatalogo(m); !errors.Is(err, ErrFeriadoCatalogoInvalido) {
		t.Errorf("móvil con mes: %v", err)
	}
}
