package planilla

import (
	"context"
	"errors"
	"testing"
	"time"

	"planillas-ccdr/internal/parametros"
)

// enterosFalsos: parámetros generales enteros como los sembrados en 000118.
type enterosFalsos map[string]int64

func (e enterosFalsos) Entero(_ context.Context, clave string, f time.Time) (int64, error) {
	v, ok := e[clave]
	if !ok {
		return 0, parametros.ErrParametroNoConfigurado{Clave: clave, Fecha: f}
	}
	return v, nil
}

func ventanasSembradas() enterosFalsos {
	return enterosFalsos{
		"AGUINALDO_PERIODO_INICIO_MES": 12, "AGUINALDO_PERIODO_INICIO_DIA": 1, "AGUINALDO_PERIODO_INICIO_ANIO_OFFSET": -1,
		"AGUINALDO_PERIODO_FIN_MES": 11, "AGUINALDO_PERIODO_FIN_DIA": 30, "AGUINALDO_PERIODO_FIN_ANIO_OFFSET": 0,
		"AGUINALDO_VENTANA_INICIO_MES": 12, "AGUINALDO_VENTANA_INICIO_DIA": 1, "AGUINALDO_VENTANA_INICIO_ANIO_OFFSET": 0,
		"AGUINALDO_VENTANA_FIN_MES": 12, "AGUINALDO_VENTANA_FIN_DIA": 31, "AGUINALDO_VENTANA_FIN_ANIO_OFFSET": 0,
		"AGUINALDO_PAGO_LIMITE_MES": 12, "AGUINALDO_PAGO_LIMITE_DIA": 20, "AGUINALDO_PAGO_LIMITE_ANIO_OFFSET": 0,
		"SALESC_PERIODO_INICIO_MES": 1, "SALESC_PERIODO_INICIO_DIA": 1, "SALESC_PERIODO_INICIO_ANIO_OFFSET": 0,
		"SALESC_PERIODO_FIN_MES": 12, "SALESC_PERIODO_FIN_DIA": 31, "SALESC_PERIODO_FIN_ANIO_OFFSET": 0,
		"SALESC_VENTANA_INICIO_MES": 1, "SALESC_VENTANA_INICIO_DIA": 1, "SALESC_VENTANA_INICIO_ANIO_OFFSET": 1,
		"SALESC_VENTANA_FIN_MES": 1, "SALESC_VENTANA_FIN_DIA": 31, "SALESC_VENTANA_FIN_ANIO_OFFSET": 1,
	}
}

func TestVentanasDesdeParametros(t *testing.T) {
	ctx := context.Background()
	v := NewVentanas(ventanasSembradas())
	a, err := v.Aguinaldo(ctx, 2026, fecha("2026-12-05"))
	if err != nil {
		t.Fatal(err)
	}
	if !a.CalculoDesde.Equal(fecha("2025-12-01")) || !a.CalculoHasta.Equal(fecha("2026-11-30")) ||
		!a.EjecucionDesde.Equal(fecha("2026-12-01")) || !a.EjecucionHasta.Equal(fecha("2026-12-31")) ||
		a.PagoLimite == nil || !a.PagoLimite.Equal(fecha("2026-12-20")) {
		t.Errorf("ventana de aguinaldo: %+v", a)
	}
	if !a.EnEjecucion(fecha("2026-12-31")) || a.EnEjecucion(fecha("2026-11-30")) || a.EnEjecucion(fecha("2027-01-01")) {
		t.Error("EnEjecucion debe incluir solo diciembre de 2026")
	}
	s, err := v.SalarioEscolar(ctx, 2026, fecha("2027-01-10"))
	if err != nil {
		t.Fatal(err)
	}
	if !s.CalculoDesde.Equal(fecha("2026-01-01")) || !s.CalculoHasta.Equal(fecha("2026-12-31")) ||
		!s.EjecucionDesde.Equal(fecha("2027-01-01")) || !s.EjecucionHasta.Equal(fecha("2027-01-31")) || s.PagoLimite != nil {
		t.Errorf("ventana de salario escolar: %+v", s)
	}

	falta := ventanasSembradas()
	delete(falta, "SALESC_VENTANA_FIN_DIA")
	var ec *ErrorCiclo
	if _, err := NewVentanas(falta).SalarioEscolar(ctx, 2026, fecha("2027-01-10")); !errors.As(err, &ec) || ec.Codigo != ErrParametrosIncompletos.Codigo {
		t.Errorf("parámetro faltante: se esperaba parametros_incompletos, se obtuvo %v", err)
	}
	mala := ventanasSembradas()
	mala["AGUINALDO_PERIODO_FIN_MES"], mala["AGUINALDO_PERIODO_FIN_DIA"] = 2, 30
	if _, err := NewVentanas(mala).Aguinaldo(ctx, 2026, fecha("2026-12-05")); err == nil {
		t.Error("30 de febrero debe dar error")
	}
}

func TestCalcularAguinaldoYSalarioEscolarGolden(t *testing.T) {
	casos := []struct {
		nombre, base, meses, esperado string
	}{
		{"12 meses completos de 1 000 000", "12000000", "12", "1000000"},
		{"ingreso a mitad de año (6.5 meses de 1 000 000)", "6500000", "12", "541666.67"},
		{"base cero", "0", "12", "0"},
	}
	for _, c := range casos {
		got, err := CalcularAguinaldo(d(c.base), d(c.meses))
		if err != nil || !got.Equal(d(c.esperado)) {
			t.Errorf("%s: %s, %v (esperado %s)", c.nombre, got, err, c.esperado)
		}
	}
	if _, err := CalcularAguinaldo(d("100"), d("0")); err == nil {
		t.Error("AGUINALDO_MESES en 0 debe dar error")
	}
	got, err := CalcularSalarioEscolar(d("12000000"), d("0.0833"))
	if err != nil || !got.Equal(d("999600")) {
		t.Errorf("salario escolar 8.33 %% de 12 000 000 = %s, %v", got, err)
	}
	if _, err := CalcularSalarioEscolar(d("100"), d("8.33")); err == nil {
		t.Error("porcentaje en escala 0-100 debe dar error (se espera fracción)")
	}
}

// El motor paga AGUINALDO / SALARIO_ESCOLAR desde la base acumulada del input.
func TestReglaDiferidaEnElMotor(t *testing.T) {
	repo := repoBase()
	repo.conceptos = []ConceptoAplicableFila{
		fila(7, "AGUINALDO", "ingreso", 70, `{"tipo":"aguinaldo"}`, false, false),
		fila(8, "SALARIO_ESCOLAR", "ingreso", 80, `{"tipo":"salario_escolar"}`, false, false),
	}
	p := ParametrosCalculo{AguinaldoMeses: d("12"), PorcentajeSalarioEscolar: d("0.0833")}
	in := CalculoEmpleadoInput{EmpleadoID: 7, FechaEfectiva: fecha("2026-11-30"),
		BaseAguinaldoAcumulada: d("6500000"), BaseSalarioEscolarAcumulada: d("1000000")}
	r, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.TotalIngresos.Equal(d("624966.67")) || !r.TotalDeducciones.IsZero() || len(r.DesglosePorConcepto) != 2 {
		t.Errorf("ingresos %s, deducciones %s, líneas %+v", r.TotalIngresos, r.TotalDeducciones, r.DesglosePorConcepto)
	}
	p.AguinaldoMeses = d("0")
	if _, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p); err == nil {
		t.Error("sin AGUINALDO_MESES debe dar error")
	}
}
