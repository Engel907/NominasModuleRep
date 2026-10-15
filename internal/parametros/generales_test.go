package parametros

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type repoFake struct {
	filas    map[string]Registro
	llamadas int
}

func (r *repoFake) BuscarVigente(_ context.Context, clave string, _ time.Time) (Registro, error) {
	r.llamadas++
	if reg, ok := r.filas[clave]; ok {
		return reg, nil
	}
	return Registro{}, ErrNoEncontrado
}

var fecha = time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)

func nuevo(filas map[string]Registro, ttl time.Duration) (*Generales, *repoFake) {
	r := &repoFake{filas: filas}
	return NewGenerales(r, ttl), r
}

func reg(clave, valor, tipo string) Registro { return Registro{Clave: clave, Valor: valor, Tipo: tipo} }

func TestAccesoresPorTipo(t *testing.T) {
	g, _ := nuevo(map[string]Registro{
		"E": reg("E", "30", TipoEntero),
		"N": reg("N", "-1", TipoEntero),
		"D": reg("D", "1.2500", TipoDecimal),
		"P": reg("P", "10.83", TipoPorcentaje),
		"T": reg("T", "hola", TipoTexto),
		"F": reg("F", "2026-12-20", TipoFecha),
		"B": reg("B", "true", TipoBooleano),
	}, 0)
	ctx := context.Background()

	if n, err := g.Entero(ctx, "E", fecha); err != nil || n != 30 {
		t.Errorf("Entero = %d, %v", n, err)
	}
	if n, err := g.Entero(ctx, "N", fecha); err != nil || n != -1 {
		t.Errorf("Entero negativo = %d, %v", n, err)
	}
	if d, err := g.Decimal(ctx, "D", fecha); err != nil || !d.Equal(decimal.RequireFromString("1.25")) {
		t.Errorf("Decimal = %s, %v", d, err)
	}
	if d, err := g.Decimal(ctx, "E", fecha); err != nil || !d.Equal(decimal.NewFromInt(30)) {
		t.Errorf("Decimal desde entero = %s, %v", d, err)
	}
	if p, err := g.Porcentaje(ctx, "P", fecha); err != nil || !p.Fraccion().Equal(decimal.RequireFromString("0.1083")) {
		t.Errorf("Porcentaje fracción = %s, %v", p.Fraccion(), err)
	}
	if s, err := g.Texto(ctx, "T", fecha); err != nil || s != "hola" {
		t.Errorf("Texto = %q, %v", s, err)
	}
	if f, err := g.Fecha(ctx, "F", fecha); err != nil || !f.Equal(time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Fecha = %v, %v", f, err)
	}
	if b, err := g.Booleano(ctx, "B", fecha); err != nil || !b {
		t.Errorf("Booleano = %v, %v", b, err)
	}
}

func TestNoConfigurado(t *testing.T) {
	g, _ := nuevo(map[string]Registro{}, time.Minute)
	ctx := context.Background()
	_, err := g.Entero(ctx, "NO_EXISTE", fecha)
	var nc ErrParametroNoConfigurado
	if !errors.As(err, &nc) {
		t.Fatalf("se esperaba ErrParametroNoConfigurado, llegó %v", err)
	}
	if nc.Clave != "NO_EXISTE" || !nc.Fecha.Equal(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("campos del error: %+v", nc)
	}
	// Todos los accesores respetan D8.
	if _, err := g.Decimal(ctx, "X", fecha); !errors.As(err, &nc) {
		t.Errorf("Decimal: %v", err)
	}
	if _, err := g.Porcentaje(ctx, "X", fecha); !errors.As(err, &nc) {
		t.Errorf("Porcentaje: %v", err)
	}
	if _, err := g.Texto(ctx, "X", fecha); !errors.As(err, &nc) {
		t.Errorf("Texto: %v", err)
	}
	if _, err := g.Fecha(ctx, "X", fecha); !errors.As(err, &nc) {
		t.Errorf("Fecha: %v", err)
	}
	if _, err := g.Booleano(ctx, "X", fecha); !errors.As(err, &nc) {
		t.Errorf("Booleano: %v", err)
	}
}

func TestErroresDeTipoYValor(t *testing.T) {
	g, _ := nuevo(map[string]Registro{
		"D":    reg("D", "1.5", TipoDecimal),
		"EBAD": reg("EBAD", "abc", TipoEntero),
		"PBAD": reg("PBAD", "150", TipoPorcentaje),
		"FBAD": reg("FBAD", "20/12/2026", TipoFecha),
		"BBAD": reg("BBAD", "quizas", TipoBooleano),
	}, 0)
	ctx := context.Background()
	if _, err := g.Entero(ctx, "D", fecha); !errors.Is(err, ErrTipoIncorrecto) {
		t.Errorf("Entero sobre decimal: %v", err)
	}
	if _, err := g.Entero(ctx, "EBAD", fecha); !errors.Is(err, ErrValorInvalido) {
		t.Errorf("entero inválido: %v", err)
	}
	if _, err := g.Porcentaje(ctx, "PBAD", fecha); !errors.Is(err, ErrValorInvalido) {
		t.Errorf("porcentaje >100: %v", err)
	}
	if _, err := g.Fecha(ctx, "FBAD", fecha); !errors.Is(err, ErrValorInvalido) {
		t.Errorf("fecha inválida: %v", err)
	}
	if _, err := g.Booleano(ctx, "BBAD", fecha); !errors.Is(err, ErrValorInvalido) {
		t.Errorf("booleano inválido: %v", err)
	}
}

func TestCacheTTLEInvalidar(t *testing.T) {
	g, r := nuevo(map[string]Registro{"E": reg("E", "30", TipoEntero)}, time.Minute)
	reloj := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g.ahora = func() time.Time { return reloj }
	ctx := context.Background()

	g.Entero(ctx, "E", fecha)
	g.Entero(ctx, "E", fecha.Add(3*time.Hour)) // misma fecha calendario -> misma llave
	if r.llamadas != 1 {
		t.Fatalf("dentro del TTL debe leerse 1 vez, fueron %d", r.llamadas)
	}

	g.Entero(ctx, "E", fecha.AddDate(0, 0, 1)) // otra fecha -> otra llave
	if r.llamadas != 2 {
		t.Fatalf("otra fecha debe consultar, llamadas=%d", r.llamadas)
	}

	reloj = reloj.Add(2 * time.Minute) // expira
	g.Entero(ctx, "E", fecha)
	if r.llamadas != 3 {
		t.Fatalf("tras el TTL debe consultar, llamadas=%d", r.llamadas)
	}

	r.filas["E"] = reg("E", "31", TipoEntero)
	if n, _ := g.Entero(ctx, "E", fecha); n != 30 {
		t.Fatalf("antes de Invalidar se espera el valor cacheado, llegó %d", n)
	}
	g.Invalidar()
	if n, _ := g.Entero(ctx, "E", fecha); n != 31 {
		t.Fatalf("tras Invalidar se espera el valor nuevo, llegó %d", n)
	}
}

func TestSinCacheConTTLCero(t *testing.T) {
	g, r := nuevo(map[string]Registro{"E": reg("E", "1", TipoEntero)}, 0)
	g.Entero(context.Background(), "E", fecha)
	g.Entero(context.Background(), "E", fecha)
	if r.llamadas != 2 {
		t.Errorf("ttl=0 no debe cachear, llamadas=%d", r.llamadas)
	}
}

func TestNoSeCacheaElFallo(t *testing.T) {
	g, r := nuevo(map[string]Registro{}, time.Minute)
	ctx := context.Background()
	g.Entero(ctx, "E", fecha)
	r.filas["E"] = reg("E", "7", TipoEntero)
	if n, err := g.Entero(ctx, "E", fecha); err != nil || n != 7 {
		t.Errorf("un parámetro recién configurado debe verse de inmediato: %d, %v", n, err)
	}
}