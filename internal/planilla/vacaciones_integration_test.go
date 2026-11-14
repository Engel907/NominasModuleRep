//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"

	"planillas-ccdr/internal/db/dto"
)

func (f *fixtureCalculo) modoPagoVacaciones(t *testing.T, modo string) {
	t.Helper()
	mustExec(t, f.db, `
INSERT INTO parametros_generales (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, 'texto', 'fixture de prueba', 'FIXTURE DE PRUEBA', '2026-01-01', 1)`, ClaveVacacionesModoPago, modo)
}

// la compensación se paga en la planilla regular y queda pagada al
// aprobar, una sola vez; en modo compensación el disfrute no descuenta.
func TestCompensacionPagadaUnaVez(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	ctx := context.Background()
	mov := mustExec(t, f.db, `
INSERT INTO vacaciones_movimientos (empleado_id, tipo_movimiento, dias, fecha_efectiva, creado_por)
VALUES (?, 'compensacion', -2, '2026-03-05', ?)`, f.emp[0], f.preparador)
	mustExec(t, f.db, `
INSERT INTO vacaciones_solicitudes (empleado_id, fecha_inicio, fecha_fin, dias_solicitados, estado, creado_por)
VALUES (?, '2026-03-10', '2026-03-14', 5, 'aprobada', ?)`, f.emp[1], f.preparador)

	svc := f.servicio()
	p := marzo(t, svc, f)
	_, err := svc.Calcular(ctx, f.preparador, p.ID)
	var ce *ErrCalculoConErrores
	if !errors.As(err, &ce) || len(ce.Errores) != 2 {
		t.Fatalf("sin modo de pago configurado: %v", err)
	}

	f.modoPagoVacaciones(t, VacacionesPagoCompensacion)
	p = f.calcularMarzoExistente(t, svc, p.ID)
	linea := func(periodo, emp int64, codigo string) string {
		var m string
		_ = f.db.QueryRow(`
SELECT pdc.monto FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ? AND cs.codigo = ?`, periodo, emp, codigo).Scan(&m)
		return m
	}
	if got := linea(p.ID, f.emp[0], "PAGO_VACACIONES"); got != "60000.00" {
		t.Errorf("PAGO_VACACIONES emp[0] = %q, esperado 60000.00", got)
	}
	if got, vac := linea(p.ID, f.emp[1], "SAL_BASE"), linea(p.ID, f.emp[1], "PAGO_VACACIONES"); got != "800000.00" || vac != "" {
		t.Errorf("emp[1] SAL_BASE %q PAGO_VACACIONES %q (el disfrute no descuenta en modo compensación)", got, vac)
	}
	var ligado *int64
	if err := f.db.QueryRow(`SELECT periodo_planilla_id FROM vacaciones_movimientos WHERE id = ?`, mov).Scan(&ligado); err != nil || ligado == nil || *ligado != p.ID {
		t.Fatalf("la compensación debe quedar ligada al período: %v, %v", ligado, err)
	}
	if _, err := svc.Recalcular(ctx, f.preparador, p.ID); err != nil {
		t.Fatalf("recalcular: %v", err)
	}
	if _, err := svc.Aprobar(ctx, f.aprobador, p.ID); err != nil {
		t.Fatalf("aprobar: %v", err)
	}
	var pagado *string
	if err := f.db.QueryRow(`SELECT pagado_en FROM vacaciones_movimientos WHERE id = ?`, mov).Scan(&pagado); err != nil || pagado == nil {
		t.Fatalf("pagado_en tras aprobar: %v, %v", pagado, err)
	}

	abril, err := svc.Crear(ctx, f.preparador, f.tipoMensual, fecha("2026-04-01"), fecha("2026-04-30"))
	if err != nil {
		t.Fatal(err)
	}
	f.calcularMarzoExistente(t, svc, abril.ID)
	if got := linea(abril.ID, f.emp[0], "PAGO_VACACIONES"); got != "" {
		t.Errorf("abril volvió a pagar la compensación: %q", got)
	}
}

// en modo disfrute los días de vacaciones salen de SAL_BASE y los paga PAGO_VACACIONES.
func TestModoDisfrute(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	f.modoPagoVacaciones(t, VacacionesPagoDisfrute)
	mustExec(t, f.db, `
INSERT INTO vacaciones_solicitudes (empleado_id, fecha_inicio, fecha_fin, dias_solicitados, estado, creado_por)
VALUES (?, '2026-03-10', '2026-03-14', 5, 'aprobada', ?)`, f.emp[1], f.preparador)
	svc := f.servicio()
	p := f.calcularMarzoExistente(t, svc, marzo(t, svc, f).ID)
	var sal, vac, neto string
	if err := f.db.QueryRow(`
SELECT
  (SELECT pdc.monto FROM planilla_detalle_concepto pdc JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id WHERE pdc.planilla_detalle_empleado_id = pde.id AND cs.codigo = 'SAL_BASE'),
  (SELECT pdc.monto FROM planilla_detalle_concepto pdc JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id WHERE pdc.planilla_detalle_empleado_id = pde.id AND cs.codigo = 'PAGO_VACACIONES'),
  pde.total_ingresos
FROM planilla_detalle_empleado pde WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ?`, p.ID, f.emp[1]).Scan(&sal, &vac, &neto); err != nil {
		t.Fatal(err)
	}
	if sal != "666666.67" || vac != "133333.33" || neto != "800000.00" {
		t.Errorf("SAL_BASE %s, PAGO_VACACIONES %s, ingresos %s", sal, vac, neto)
	}
}

// calcularMarzoExistente calcula un período ya creado y falla con el detalle de errores.
func (f *fixtureCalculo) calcularMarzoExistente(t *testing.T, svc *PeriodoService, id int64) dto.PeriodoCicloRow {
	t.Helper()
	p, err := svc.Calcular(context.Background(), f.preparador, id)
	if err != nil {
		var ce *ErrCalculoConErrores
		if errors.As(err, &ce) {
			t.Fatalf("calcular: %+v", ce.Errores)
		}
		t.Fatalf("calcular: %v", err)
	}
	return p
}
