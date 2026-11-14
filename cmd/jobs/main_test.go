package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/parametros"
)

type proveedorFaltantesFake struct {
	faltan []parametros.Requerido
	err    error
}

func (p proveedorFaltantesFake) Faltantes(context.Context, time.Time) ([]parametros.Requerido, error) {
	return p.faltan, p.err
}

func TestResolverFecha(t *testing.T) {
	ahora := time.Date(2026, time.October, 19, 23, 45, 0, 0, time.FixedZone("local", -6*60*60))
	t.Run("por defecto usa hoy en UTC", func(t *testing.T) {
		got, err := resolverFecha("", ahora)
		want := time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)
		if err != nil || !got.Equal(want) || got.Location() != time.UTC {
			t.Fatalf("fecha=%v, err=%v; se esperaba %v en UTC", got, err, want)
		}
	})
	t.Run("fecha explícita", func(t *testing.T) {
		got, err := resolverFecha("2026-10-01", ahora)
		want := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
		if err != nil || !got.Equal(want) {
			t.Fatalf("fecha=%v, err=%v; se esperaba %v", got, err, want)
		}
	})
	t.Run("fecha inválida", func(t *testing.T) {
		if _, err := resolverFecha("19-10-2026", ahora); err == nil {
			t.Fatal("se esperaba error por formato de fecha inválido")
		}
	})
}

func TestVerificar(t *testing.T) {
	fecha := time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)
	t.Run("sin faltantes", func(t *testing.T) {
		var stdout, stderr strings.Builder
		codigo := verificar(context.Background(), proveedorFaltantesFake{}, fecha, &stdout, &stderr)
		if codigo != codigoOK || !strings.Contains(stdout.String(), "Todos los parámetros") || stderr.Len() != 0 {
			t.Fatalf("codigo=%d stdout=%q stderr=%q", codigo, stdout.String(), stderr.String())
		}
	})
	t.Run("faltante retorna uno y muestra clave", func(t *testing.T) {
		var stdout, stderr strings.Builder
		faltan := []parametros.Requerido{{
			Clave: parametros.ClaveSalarioMinimoInembargable,
			Tipo:  parametros.TipoDecimal,
		}}
		codigo := verificar(context.Background(), proveedorFaltantesFake{faltan: faltan}, fecha, &stdout, &stderr)
		if codigo != codigoFaltantes ||
			!strings.Contains(stdout.String(), parametros.ClaveSalarioMinimoInembargable) ||
			stderr.Len() != 0 {
			t.Fatalf("codigo=%d stdout=%q stderr=%q", codigo, stdout.String(), stderr.String())
		}
	})
	t.Run("error de lectura retorna dos", func(t *testing.T) {
		var stdout, stderr strings.Builder
		codigo := verificar(context.Background(), proveedorFaltantesFake{err: errors.New("fallo de lectura")}, fecha, &stdout, &stderr)
		if codigo != codigoError || !strings.Contains(stderr.String(), "fallo de lectura") {
			t.Fatalf("codigo=%d stdout=%q stderr=%q", codigo, stdout.String(), stderr.String())
		}
	})
}

func TestAcumularVacacionesUsoEInforme(t *testing.T) {
	var stdout, stderr strings.Builder
	if c := ejecutar(context.Background(), []string{"acumular-vacaciones"}, &stdout, &stderr); c != codigoError || !strings.Contains(stderr.String(), "--usuario-id") {
		t.Fatalf("sin --usuario-id: código %d, stderr %q", c, stderr.String())
	}
	fecha := time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC)
	stdout.Reset()
	ok := asistencia.ResumenAcumulacion{Acreditados: 2, YaRegistrados: 1, Errores: map[int64]string{}}
	if c := informarAcumulacion(ok, nil, "aniversario", fecha, &stdout, &stderr); c != codigoOK || !strings.Contains(stdout.String(), "2 acreditados, 1 ya registrados") {
		t.Fatalf("informe ok: %d %q", c, stdout.String())
	}
	stdout.Reset()
	malo := asistencia.ResumenAcumulacion{Errores: map[int64]string{7: "sin escala"}}
	if c := informarAcumulacion(malo, nil, "mensual", fecha, &stdout, &stderr); c != codigoFaltantes || !strings.Contains(stdout.String(), "empleado 7: sin escala") {
		t.Fatalf("informe con errores: %d %q", c, stdout.String())
	}
}
