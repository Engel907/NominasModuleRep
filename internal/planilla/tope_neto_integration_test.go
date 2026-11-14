//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"
)

// el préstamo se reduce al remanente tras las obligatorias y el neto queda en 0.
func TestTopeNetoPrestamoParcial(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	var oblig int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM conceptos_salariales WHERE es_deduccion_obligatoria = 1 AND codigo IN ('CCSS_OBR','IMP_RENTA','EMBARGO')`).Scan(&oblig); err != nil || oblig != 3 {
		t.Fatalf("seed de es_deduccion_obligatoria: %d, %v", oblig, err)
	}
	mustExec(t, f.db, `UPDATE contratos SET salario_pactado = 150000 WHERE id = ?`, f.contrato[1])
	mustExec(t, f.db, `UPDATE prestamos SET cuota_periodica = 200000 WHERE id = ?`, f.prestamoID)

	svc := f.servicio()
	p := marzo(t, svc, f)
	if _, err := svc.Calcular(context.Background(), f.preparador, p.ID); err != nil {
		var ce *ErrCalculoConErrores
		if errors.As(err, &ce) {
			t.Fatalf("calcular: %+v", ce.Errores)
		}
		t.Fatalf("calcular: %v", err)
	}
	var neto, cuota, retenido string
	var advertencias *string
	if err := f.db.QueryRow(`
SELECT pde.neto_pagado, pdp.cuota_calculada, pdp.monto_retenido, pde.advertencias
FROM planilla_detalle_empleado pde
JOIN planilla_detalle_prestamo pdp ON pdp.planilla_detalle_empleado_id = pde.id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ?`, p.ID, f.emp[1]).Scan(&neto, &cuota, &retenido, &advertencias); err != nil {
		t.Fatal(err)
	}
	if neto != "0.00" || cuota != "200000.00" || retenido != "124826.64" || advertencias == nil {
		t.Errorf("neto %s, cuota %s, retenido %s, advertencias %v", neto, cuota, retenido, advertencias)
	}
}
