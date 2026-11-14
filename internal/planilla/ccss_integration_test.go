//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"
)

// base mínima contributiva por régimen CCSS y persistencia de la base usada.
func TestBaseMinimaCCSS(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	mustExec(t, f.db, `UPDATE contratos SET salario_pactado = 150000 WHERE id = ?`, f.contrato[1])

	svc := f.servicio()
	p := marzo(t, svc, f)
	if _, err := svc.Calcular(context.Background(), f.preparador, p.ID); err != nil {
		var ce *ErrCalculoConErrores
		if errors.As(err, &ce) {
			t.Fatalf("calcular: %+v", ce.Errores)
		}
		t.Fatalf("calcular: %v", err)
	}
	var base, obrera, patronal, basePat string
	if err := f.db.QueryRow(`
SELECT pde.base_ccss, pde.cuota_obrera_ccss, pde.cuota_patronal_ccss, pdp.base
FROM planilla_detalle_empleado pde
JOIN planilla_detalle_patronal pdp ON pdp.planilla_detalle_empleado_id = pde.id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ?`, p.ID, f.emp[1]).Scan(&base, &obrera, &patronal, &basePat); err != nil {
		t.Fatal(err)
	}
	if base != "232441.00" || basePat != "232441.00" || obrera != "25173.36" || patronal != "62363.92" {
		t.Errorf("base %s / base patronal %s / obrera %s / patronal %s", base, basePat, obrera, patronal)
	}
	if err := f.db.QueryRow(`SELECT base_ccss FROM planilla_detalle_empleado WHERE periodo_planilla_id = ? AND empleado_id = ?`, p.ID, f.emp[0]).Scan(&base); err != nil || base != "900000.00" {
		t.Errorf("emp[0] base_ccss = %s, %v (esperado 900000.00)", base, err)
	}
}

func TestEmpleadoSinRegimenCCSS(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	mustExec(t, f.db, `UPDATE empleado_datos_laborales SET regimen_ccss_id = NULL WHERE empleado_id = ?`, f.emp[1])

	svc := f.servicio()
	p := marzo(t, svc, f)
	_, err := svc.Calcular(context.Background(), f.preparador, p.ID)
	var ce *ErrCalculoConErrores
	if !errors.As(err, &ce) || len(ce.Errores) != 1 || ce.Errores[0].EmpleadoID != f.emp[1] || ce.Errores[0].Codigo != "empleado_sin_regimen_ccss" {
		t.Fatalf("se esperaba empleado_sin_regimen_ccss solo para emp[1]: %v", err)
	}
}
