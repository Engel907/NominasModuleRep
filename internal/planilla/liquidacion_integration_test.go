//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"
	"time"
)

// insertarLiquidacion: fila 'procesada' como la deja LiquidacionService (los
// montos ya calculados por internal/liquidacion).
func (f *fixtureCalculo) insertarLiquidacion(t *testing.T, emp int64, salida, preaviso, cesantia, vacaciones string) int64 {
	t.Helper()
	return mustExec(t, f.db, `
INSERT INTO liquidaciones (empleado_id, fecha_salida, motivo_salida, aplica_responsabilidad_patronal, fecha_ingreso,
  antiguedad_meses, salario_promedio_6meses, salario_diario_promedio, monto_preaviso, monto_cesantia,
  monto_vacaciones_proporcionales, periodo_aguinaldo_desde, total_liquidacion, estado, procesado_por, procesado_en)
VALUES (?, ?, 'despido_con_responsabilidad', 1, '2020-01-01', 74, 0, 0, ?, ?, ?, '2025-12-01', ?, 'procesada', ?, NOW())`,
		emp, salida, preaviso, cesantia, vacaciones, "0", f.preparador)
}

func TestLiquidacionPorElMotor(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	mustExec(t, f.db, `
INSERT INTO parametros_generales (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES (?, 'cuota', 'texto', 'fixture', 'FIXTURE DE PRUEBA', '2026-01-01', 1)`, ClaveLiqPrestamosModo)
	liq0 := f.insertarLiquidacion(t, f.emp[0], "2026-03-31", "0", "1000000", "100000")
	liq1 := f.insertarLiquidacion(t, f.emp[1], "2026-03-31", "200000", "0", "50000")
	f.insertarLiquidacion(t, f.emp[2], "2026-04-15", "0", "0", "10000")
	var tipoLiq int64
	if err := f.db.QueryRow(`SELECT id FROM tipos_planilla WHERE clase = 'liquidacion'`).Scan(&tipoLiq); err != nil {
		t.Fatal(err)
	}
	svc := f.servicio()
	ctx := context.Background()
	p, err := svc.Crear(ctx, f.preparador, tipoLiq, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	calcular := func() {
		t.Helper()
		if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
			var ce *ErrCalculoConErrores
			if errors.As(err, &ce) {
				t.Fatalf("calcular: %+v", ce.Errores)
			}
			t.Fatalf("calcular: %v", err)
		}
	}
	calcular()

	monto := func(emp int64, codigo string) string {
		t.Helper()
		var m string
		err := f.db.QueryRow(`
SELECT COALESCE(SUM(pdc.monto), 0) FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ? AND cs.codigo = ?`, p.ID, emp, codigo).Scan(&m)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	verificar := func() {
		t.Helper()
		n := f.contar(t, p.ID)
		if n["planilla_detalle_empleado"] != 2 || n["planilla_detalle_embargo"] != 1 || n["planilla_detalle_prestamo"] != 1 {
			t.Fatalf("filas de detalle: %v", n)
		}
		casos := []struct {
			emp           int64
			codigo, monto string
		}{
			{f.emp[0], "LIQ_CESANTIA", "1000000.00"}, {f.emp[0], "LIQ_VACACIONES", "100000.00"},
			{f.emp[0], "CCSS_OBR", "10830.00"}, {f.emp[0], "SAL_BASE", "0.00"},
			{f.emp[1], "LIQ_PREAVISO", "200000.00"}, {f.emp[1], "LIQ_VACACIONES", "50000.00"},
			{f.emp[1], "CCSS_OBR", "5415.00"}, {f.emp[1], "PRESTAMO", "25000.00"},
		}
		for _, c := range casos {
			if got := monto(c.emp, c.codigo); got != c.monto {
				t.Errorf("empleado %d %s = %s, esperado %s", c.emp, c.codigo, got, c.monto)
			}
		}
		if got := monto(f.emp[0], "EMBARGO"); got != "11146.25" {
			t.Errorf("embargo aplicado %s, esperado 11146.25", got)
		}
		var ligadas, cuadra int
		if err := f.db.QueryRow(`SELECT COUNT(*) FROM liquidaciones WHERE periodo_planilla_id = ? AND id IN (?, ?)`, p.ID, liq0, liq1).Scan(&ligadas); err != nil {
			t.Fatal(err)
		}
		if err := f.db.QueryRow(`SELECT fn_detalle_cuadra(?)`, p.ID).Scan(&cuadra); err != nil {
			t.Fatal(err)
		}
		if ligadas != 2 || cuadra != 1 {
			t.Fatalf("liquidaciones ligadas %d (esperado 2), cuadra %d", ligadas, cuadra)
		}
	}
	verificar()

	if _, err := svc.Recalcular(ctx, f.preparador, p.ID); err != nil {
		t.Fatal(err)
	}
	verificar()

	if _, err := svc.Descartar(ctx, f.preparador, p.ID); err != nil {
		t.Fatal(err)
	}
	var libres int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM liquidaciones WHERE periodo_planilla_id IS NULL`).Scan(&libres); err != nil || libres != 3 {
		t.Fatalf("liquidaciones libres tras descartar: %d (%v)", libres, err)
	}
	calcular()
	verificar()
	if _, err := svc.Aprobar(ctx, f.aprobador, p.ID); err != nil {
		t.Fatalf("aprobar: %v", err)
	}
	var saldo string
	if err := f.db.QueryRow(`SELECT saldo_pendiente FROM prestamos WHERE id = ?`, f.prestamoID).Scan(&saldo); err != nil || saldo != "275000.00" {
		t.Fatalf("saldo del préstamo tras aprobar la liquidación: %s (%v)", saldo, err)
	}
}

func TestSinModoPrestamosEsErrorDelEmpleado(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	f.insertarLiquidacion(t, f.emp[1], "2026-03-31", "200000", "0", "0")
	var tipoLiq int64
	if err := f.db.QueryRow(`SELECT id FROM tipos_planilla WHERE clase = 'liquidacion'`).Scan(&tipoLiq); err != nil {
		t.Fatal(err)
	}
	svc := f.servicio()
	ctx := context.Background()
	p, err := svc.Crear(ctx, f.preparador, tipoLiq, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Calcular(ctx, f.preparador, p.ID)
	var ce *ErrCalculoConErrores
	if !errors.As(err, &ce) || len(ce.Errores) != 1 || ce.Errores[0].EmpleadoID != f.emp[1] {
		t.Fatalf("esperaba el error del empleado con préstamo sin LIQ_PRESTAMOS_MODO: %v", err)
	}
}
