//go:build integration

package planilla

import (
	"context"
	"testing"

	"planillas-ccdr/internal/db/dto"
)

// Un aumento con fecha pasada sobre un mes ya aprobado: el período siguiente
// paga la diferencia como línea del mismo concepto ligada al mes de origen, y
// un segundo retroactivo sobre el mismo mes ya no la vuelve a pagar.
func TestRetroactivoPagaLaDiferenciaUnaVez(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	svc := f.servicio()
	ctx := context.Background()
	f.mesAprobado(t, svc, "2026-03-01", "2026-03-31")
	var marzoID int64
	if err := f.db.QueryRow(`SELECT id FROM periodos_planilla WHERE fecha_inicio = '2026-03-01' AND tipo_planilla_id = ?`, f.tipoMensual).Scan(&marzoID); err != nil {
		t.Fatal(err)
	}
	pagadoMarzo, _ := f.montoConcepto(t, marzoID, f.emp[0], "SAL_BASE")

	mustExec(t, f.db, `UPDATE contratos SET salario_pactado = salario_pactado + 100000 WHERE id = ?`, f.contrato[0])

	retro := NewRetroactivosService(f.db)
	diferencia := func(periodo dto.PeriodoCicloRow) (string, int) {
		t.Helper()
		var monto string
		var n int
		if err := f.db.QueryRow(`
SELECT COALESCE(SUM(pdc.monto), 0), COUNT(*)
FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ? AND pdc.concepto_salarial_id = ?
  AND pdc.retroactivo_periodo_origen_id = ?`, periodo.ID, f.emp[0], f.concepto["SAL_BASE"], marzoID).Scan(&monto, &n); err != nil {
			t.Fatal(err)
		}
		return monto, n
	}
	periodoCon := func(inicio, fin string) dto.PeriodoCicloRow {
		t.Helper()
		p, err := svc.Crear(ctx, f.preparador, f.tipoMensual, fecha(inicio), fecha(fin))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := retro.Crear(ctx, f.preparador, p.ID, NuevoRetroactivo{EmpleadoID: f.emp[0],
			Desde: fecha("2026-03-01"), Hasta: fecha("2026-03-31"), Motivo: "Aumento con rige 1-mar"}); err != nil {
			t.Fatal(err)
		}
		f.calcularOFallar(t, svc, p)
		return p
	}

	abril := periodoCon("2026-04-01", "2026-04-30")
	if m, n := diferencia(abril); m != "100000.00" || n != 1 {
		t.Fatalf("abril paga %s en %d línea(s) de SAL_BASE por marzo; esperado 100000.00 en 1 (marzo pagó %s)", m, n, pagadoMarzo)
	}
	var ajusteDe int64
	if err := f.db.QueryRow(`
SELECT COALESCE(pdc.concepto_ajuste_de_id, 0) FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ? AND pdc.retroactivo_periodo_origen_id = ? AND pdc.concepto_salarial_id = ?`,
		abril.ID, marzoID, f.concepto["SAL_BASE"]).Scan(&ajusteDe); err != nil || ajusteDe == 0 {
		t.Errorf("la línea del retroactivo debe apuntar a la de marzo (concepto_ajuste_de_id=%d, %v)", ajusteDe, err)
	}
	if _, err := svc.Aprobar(ctx, f.aprobador, abril.ID); err != nil {
		t.Fatal(err)
	}

	mayo := periodoCon("2026-05-01", "2026-05-31")
	if m, n := diferencia(mayo); n != 0 {
		t.Errorf("mayo vuelve a pagar %s por marzo en %d línea(s); ya lo pagó abril", m, n)
	}
}
