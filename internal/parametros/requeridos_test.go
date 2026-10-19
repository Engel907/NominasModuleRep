package parametros

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFaltantes_TodosVigentes(t *testing.T) {
	g, _ := nuevo(map[string]Registro{
		ClaveDivisorDiasMes:            reg(ClaveDivisorDiasMes, "30", TipoEntero),
		ClaveSalarioMinimoInembargable: reg(ClaveSalarioMinimoInembargable, "1234.50", TipoDecimal),
	}, 0)
	f, err := g.Faltantes(context.Background(), fecha)
	if err != nil || len(f) != 0 {
		t.Fatalf("faltantes = %v, err = %v", f, err)
	}
}

func TestFaltantes_AceptaEnteroParaDecimal(t *testing.T) {
	g, _ := nuevo(map[string]Registro{
		ClaveDivisorDiasMes:            reg(ClaveDivisorDiasMes, "30", TipoEntero),
		ClaveSalarioMinimoInembargable: reg(ClaveSalarioMinimoInembargable, "1000", TipoEntero),
	}, 0)
	if f, err := g.Faltantes(context.Background(), fecha); err != nil || len(f) != 0 {
		t.Fatalf("faltantes = %v, err = %v", f, err)
	}
}

func TestFaltantes_SinNingunoEnOrden(t *testing.T) {
	g, _ := nuevo(map[string]Registro{}, time.Minute)
	f, err := g.Faltantes(context.Background(), fecha)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 2 || f[0].Clave != ClaveDivisorDiasMes || f[1].Clave != ClaveSalarioMinimoInembargable {
		t.Fatalf("faltantes inesperados: %+v", f)
	}
}

func TestFaltantes_SoloUno(t *testing.T) {
	g, _ := nuevo(map[string]Registro{
		ClaveDivisorDiasMes: reg(ClaveDivisorDiasMes, "30", TipoEntero),
	}, 0)
	f, err := g.Faltantes(context.Background(), fecha)
	if err != nil || len(f) != 1 || f[0].Clave != ClaveSalarioMinimoInembargable {
		t.Fatalf("faltantes = %+v, err = %v", f, err)
	}
}

func TestFaltantes_TipoIncorrectoEsErrorNoFaltante(t *testing.T) {
	g, _ := nuevo(map[string]Registro{
		ClaveDivisorDiasMes:            reg(ClaveDivisorDiasMes, "30", TipoTexto),
		ClaveSalarioMinimoInembargable: reg(ClaveSalarioMinimoInembargable, "1", TipoDecimal),
	}, 0)
	f, err := g.Faltantes(context.Background(), fecha)
	if !errors.Is(err, ErrTipoIncorrecto) || f != nil {
		t.Fatalf("se esperaba ErrTipoIncorrecto sin faltantes, llegó %v, %v", f, err)
	}
}

func TestFaltantes_ValorIlegibleEsError(t *testing.T) {
	g, _ := nuevo(map[string]Registro{
		ClaveDivisorDiasMes:            reg(ClaveDivisorDiasMes, "treinta", TipoEntero),
		ClaveSalarioMinimoInembargable: reg(ClaveSalarioMinimoInembargable, "1", TipoDecimal),
	}, 0)
	if _, err := g.Faltantes(context.Background(), fecha); !errors.Is(err, ErrValorInvalido) {
		t.Fatalf("se esperaba ErrValorInvalido, llegó %v", err)
	}
}

func TestFaltantes_ValoresNoPositivosSonError(t *testing.T) {
	casos := []struct {
		nombre string
		filas  map[string]Registro
	}{
		{
			nombre: "divisor cero",
			filas: map[string]Registro{
				ClaveDivisorDiasMes:            reg(ClaveDivisorDiasMes, "0", TipoEntero),
				ClaveSalarioMinimoInembargable: reg(ClaveSalarioMinimoInembargable, "1", TipoDecimal),
			},
		},
		{
			nombre: "salario mínimo negativo",
			filas: map[string]Registro{
				ClaveDivisorDiasMes:            reg(ClaveDivisorDiasMes, "30", TipoEntero),
				ClaveSalarioMinimoInembargable: reg(ClaveSalarioMinimoInembargable, "-1", TipoDecimal),
			},
		},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			g, _ := nuevo(tc.filas, 0)
			if _, err := g.Faltantes(context.Background(), fecha); !errors.Is(err, ErrValorInvalido) {
				t.Fatalf("se esperaba ErrValorInvalido, llegó %v", err)
			}
		})
	}
}
