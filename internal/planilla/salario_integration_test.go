//go:build integration

package planilla

import (
	"context"
	"testing"
)

// L10: aumento a mitad de período con la convención real de cierre (la versión
// vieja termina el mismo día en que empieza la nueva) y copia derivada de
// salario_base_nominal mantenida por los triggers de 000124.
func TestL10AumentoAMitadDePeriodoYSalarioDerivado(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	ctx := context.Background()

	// Aumento de emp[0] de 900 000 a 1 000 000 desde el 16 de marzo.
	mustExec(t, f.db, `UPDATE contratos SET fecha_fin_vigencia = '2026-03-16' WHERE id = ?`, f.contrato[0])
	mustExec(t, f.db, `
INSERT INTO contratos (empleado_id, puesto_id, regimen_laboral_id, tipo_contrato, salario_pactado, jornada,
                       fecha_inicio_contrato, fecha_inicio_vigencia, creado_por)
SELECT empleado_id, puesto_id, regimen_laboral_id, tipo_contrato, 1000000, jornada, fecha_inicio_contrato, '2026-03-16', creado_por
FROM contratos WHERE id = ?`, f.contrato[0])

	var derivado string
	if err := f.db.QueryRow(`SELECT salario_base_nominal FROM empleado_datos_laborales WHERE empleado_id = ? AND fecha_fin_vigencia IS NULL`, f.emp[0]).
		Scan(&derivado); err != nil || derivado != "1000000.00" {
		t.Fatalf("salario_base_nominal derivado = %s, %v (se esperaba 1000000.00)", derivado, err)
	}
	// Datos laborales nuevos con salario 0: el trigger copia el del contrato vigente.
	mustExec(t, f.db, `UPDATE empleado_datos_laborales SET fecha_fin_vigencia = '2026-03-31' WHERE empleado_id = ? AND fecha_fin_vigencia IS NULL`, f.emp[1])
	mustExec(t, f.db, `
INSERT INTO empleado_datos_laborales (empleado_id, puesto_id, salario_base_nominal, tipo_contrato_id, programa_presupuestario_id, fecha_inicio_vigencia)
SELECT empleado_id, puesto_id, 0, tipo_contrato_id, programa_presupuestario_id, '2026-04-01'
FROM empleado_datos_laborales WHERE empleado_id = ? ORDER BY id LIMIT 1`, f.emp[1])
	if err := f.db.QueryRow(`SELECT salario_base_nominal FROM empleado_datos_laborales WHERE empleado_id = ? AND fecha_fin_vigencia IS NULL`, f.emp[1]).
		Scan(&derivado); err != nil || derivado != "800000.00" {
		t.Fatalf("salario_base_nominal al insertar = %s, %v (se esperaba 800000.00)", derivado, err)
	}

	svc := f.servicio()
	p := marzo(t, svc, f)
	if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
		t.Fatalf("calcular: %v", err)
	}
	// 900 000 × 15/30 + 1 000 000 × 15/30 = 950 000; la CCSS sale de esa base.
	var salario, contrato string
	if err := f.db.QueryRow(`SELECT salario_base_periodo, contrato_id FROM planilla_detalle_empleado WHERE periodo_planilla_id = ? AND empleado_id = ?`,
		p.ID, f.emp[0]).Scan(&salario, &contrato); err != nil || salario != "950000.00" {
		t.Fatalf("salario_base_periodo = %s, %v", salario, err)
	}
	if contrato == "" || contrato == "0" {
		t.Errorf("contrato_id del detalle vacío")
	}
}
