//go:build integration

package planilla

import (
	"context"
	"testing"
)

// L11 de punta a punta: boleta CCSS 27-feb..4-mar para emp[2] (salario 700 000).
// En marzo la serie va del día 3 al 6: el día 3 (1-mar) lo paga el patrono al
// 50 %; los días 4–6 (2..4-mar) son de la CCSS (informativos).
func TestL11IncapacidadEnElCalculo(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	f.crearIncapacidad(t)
	svc := f.servicio()
	p := marzo(t, svc, f)
	if _, err := svc.Calcular(context.Background(), f.preparador, p.ID); err != nil {
		t.Fatalf("calcular: %v", err)
	}
	var salario string
	if err := f.db.QueryRow(`SELECT salario_base_periodo FROM planilla_detalle_empleado WHERE periodo_planilla_id = ? AND empleado_id = ?`,
		p.ID, f.emp[2]).Scan(&salario); err != nil || salario != "606666.67" {
		t.Fatalf("salario base = %s, %v (700 000 × 26/30 = 606 666.67)", salario, err)
	}
	rows, err := f.db.Query(`
SELECT dia_serie_desde, dia_serie_hasta, dias, monto_patronal, monto_entidad_informativo
FROM planilla_detalle_incapacidad pdi
JOIN planilla_detalle_empleado pde ON pde.id = pdi.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ? ORDER BY dia_serie_desde`, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type linea struct {
		desde, hasta   int
		dias, pat, ent string
	}
	var got []linea
	for rows.Next() {
		var l linea
		if err := rows.Scan(&l.desde, &l.hasta, &l.dias, &l.pat, &l.ent); err != nil {
			t.Fatal(err)
		}
		got = append(got, l)
	}
	// 700 000/30 = 23 333.33…: patrono 1 día × 50 % = 11 666.67; CCSS 3 días × 60 % = 42 000
	want := []linea{{3, 3, "1.00", "11666.67", "0.00"}, {4, 6, "3.00", "0.00", "42000.00"}}
	if len(got) != len(want) {
		t.Fatalf("líneas de incapacidad: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("línea %d = %+v, esperado %+v", i, got[i], want[i])
		}
	}
	var sub string
	if err := f.db.QueryRow(`
SELECT pdc.monto FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ? AND pdc.concepto_salarial_id = ?`, p.ID, f.emp[2], f.concepto["SUB_INCAP"]).Scan(&sub); err != nil || sub != "11666.67" {
		t.Errorf("SUB_INCAP = %s, %v", sub, err)
	}
	var cuadra int
	if err := f.db.QueryRow(`SELECT fn_detalle_cuadra(?)`, p.ID).Scan(&cuadra); err != nil || cuadra != 1 {
		t.Errorf("fn_detalle_cuadra = %d, %v", cuadra, err)
	}
}
