//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"
)

func (f *fixtureCalculo) trabajarFeriado(t *testing.T, fecha string, pagoObligatorio int, horas string) {
	t.Helper()
	mustExec(t, f.db, `INSERT INTO feriados (fecha, nombre, tipo, pago_obligatorio, anio) VALUES (?, 'Feriado de prueba', 'fijo', ?, 2026)`, fecha, pagoObligatorio)
	mustExec(t, f.db, `
INSERT INTO calendario_laboral_dia (fecha, anio, es_habil, es_feriado, feriado_id, dia_semana)
SELECT ?, 2026, 0, 1, id, 4 FROM feriados WHERE fecha = ?
ON DUPLICATE KEY UPDATE es_habil = 0, es_feriado = 1, feriado_id = VALUES(feriado_id)`, fecha, fecha)
	mustExec(t, f.db, `
INSERT INTO registro_asistencia_detalle (empleado_id, fecha, horas_ordinarias, horas_extra, estado, creado_por)
VALUES (?, ?, ?, 0, 'presente', ?)`, f.emp[2], fecha, horas, f.preparador)
}

func TestFeriadoLaboradoEnLaPlanilla(t *testing.T) {
	ctx := context.Background()

	t.Run("pago obligatorio se paga doble", func(t *testing.T) {
		f := nuevoFixtureCalculo(t)
		f.sembrarSalarioMinimoInembargable(t)
		f.trabajarFeriado(t, "2026-03-12", 1, "8")
		svc := f.servicio()
		p := marzo(t, svc, f)
		if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
			t.Fatalf("calcular: %v", err)
		}
		if got, ok := f.montoConcepto(t, p.ID, f.emp[2], "FERIADO_LAB"); !ok || got != "23333.33" {
			t.Fatalf("FERIADO_LAB = %q (%t), esperado 23333.33 (700 000 / 240 × 8)", got, ok)
		}
		if _, ok := f.montoConcepto(t, p.ID, f.emp[0], "FERIADO_LAB"); ok {
			t.Fatal("un empleado que no trabajó el feriado no debe tener FERIADO_LAB")
		}
	})

	t.Run("pago no obligatorio sin factor configurado bloquea el cálculo", func(t *testing.T) {
		f := nuevoFixtureCalculo(t)
		f.sembrarSalarioMinimoInembargable(t)
		f.trabajarFeriado(t, "2026-03-12", 0, "8")
		svc := f.servicio()
		p := marzo(t, svc, f)
		_, err := svc.Calcular(ctx, f.preparador, p.ID)
		var ce *ErrCalculoConErrores
		if !errors.As(err, &ce) || len(ce.Errores) != 1 || ce.Errores[0].EmpleadoID != f.emp[2] {
			t.Fatalf("se esperaba un error del empleado que trabajó el feriado, se obtuvo %v", err)
		}
	})

	t.Run("pago no obligatorio con factor 0 no agrega pago", func(t *testing.T) {
		f := nuevoFixtureCalculo(t)
		f.sembrarSalarioMinimoInembargable(t)
		mustExec(t, f.db, `INSERT INTO parametros_generales (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES (?, '0', 'decimal', 'fixture', 'FIXTURE DE PRUEBA', '2026-01-01', 1)`, ClaveFeriadoFactorNoObligatorio)
		f.trabajarFeriado(t, "2026-03-12", 0, "8")
		svc := f.servicio()
		p := marzo(t, svc, f)
		if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
			t.Fatalf("calcular: %v", err)
		}
		if got, ok := f.montoConcepto(t, p.ID, f.emp[2], "FERIADO_LAB"); ok {
			t.Fatalf("con factor 0 no debe haber línea, hay %s", got)
		}
	})
}
