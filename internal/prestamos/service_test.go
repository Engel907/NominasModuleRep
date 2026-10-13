package prestamos

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func i32(n int32) *int32         { return &n }

// Las validaciones de coherencia corren ANTES de abrir la transacción, así
// que se prueban sin base de datos (dbConn nil nunca se toca).
func TestCrearPrestamo_ValidacionesPrevias(t *testing.T) {
	s := &Service{}
	hoy := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	ayer := hoy.AddDate(0, 0, -1)

	base := CrearPrestamoInput{
		EmpleadoID: 1, TerceroID: 1,
		MontoTotal: d("100000.00"), CuotaPeriodica: d("10000.00"),
		CuotasTotales: i32(10), FechaInicio: hoy,
	}
	casos := []struct {
		nombre string
		mut    func(*CrearPrestamoInput)
		want   error
	}{
		{"monto cero", func(in *CrearPrestamoInput) { in.MontoTotal = d("0") }, ErrMontoInvalido},
		{"cuota negativa", func(in *CrearPrestamoInput) { in.CuotaPeriodica = d("-1") }, ErrMontoInvalido},
		{"tres decimales", func(in *CrearPrestamoInput) { in.MontoTotal = d("100.001") }, ErrMontoPrecision},
		{"excede DECIMAL(14,2)", func(in *CrearPrestamoInput) { in.MontoTotal = d("1000000000000") }, ErrMontoPrecision},
		{"cuota > monto", func(in *CrearPrestamoInput) { in.CuotaPeriodica = d("100001") }, ErrCuotaExcedeMonto},
		{"cuotas cero", func(in *CrearPrestamoInput) { in.CuotasTotales = i32(0) }, ErrCuotasInvalidas},
		{"cuotas incoherentes", func(in *CrearPrestamoInput) { in.CuotasTotales = i32(3) }, ErrCuotasIncoherentes},
		{"fecha fin < inicio", func(in *CrearPrestamoInput) { in.FechaFinEstimada = &ayer }, ErrFechasInvalidas},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			in := base
			c.mut(&in)
			_, err := s.CrearPrestamo(context.Background(), 1, in)
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestCuotasTolerancia(t *testing.T) {
	// 100000 / 30000 = 3.33 -> ceil 4; 3, 4 y 5 dentro de ±1; 2 y 6 fuera.
	s := &Service{}
	for n, ok := range map[int32]bool{2: false, 3: true, 4: true, 5: true, 6: false} {
		_, err := s.CrearPrestamo(context.Background(), 0, CrearPrestamoInput{
			MontoTotal: d("100000"), CuotaPeriodica: d("30000"), CuotasTotales: i32(n),
		})
		// Con userID 0 las que pasan la validación previa fallan después en
		// WithAuditContext (ErrNilUserID); lo que importa es NO recibir
		// ErrCuotasIncoherentes cuando el número está dentro de tolerancia.
		got := errors.Is(err, ErrCuotasIncoherentes)
		if got == ok {
			t.Fatalf("cuotas=%d: incoherente=%v, esperado %v", n, got, !ok)
		}
	}
}

func TestCrearEmbargo_Validaciones(t *testing.T) {
	s := &Service{}
	hoy := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	p := func(v string) *decimal.Decimal { x := d(v); return &x }
	casos := []struct {
		nombre string
		in     CrearEmbargoInput
		want   error
	}{
		{"sin base", CrearEmbargoInput{FechaInicio: hoy}, ErrEmbargoSinBase},
		{"ambas bases", CrearEmbargoInput{FechaInicio: hoy, Porcentaje: p("10"), MontoFijo: p("5000")}, ErrEmbargoBaseDuplicada},
		{"porcentaje 0", CrearEmbargoInput{FechaInicio: hoy, Porcentaje: p("0")}, ErrPorcentajeInvalido},
		{"porcentaje 100.01", CrearEmbargoInput{FechaInicio: hoy, Porcentaje: p("100.01")}, ErrPorcentajeInvalido},
		{"monto fijo negativo", CrearEmbargoInput{FechaInicio: hoy, MontoFijo: p("-5")}, ErrMontoInvalido},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := s.CrearEmbargo(context.Background(), 1, c.in)
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}
