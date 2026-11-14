//go:build integration

package planilla

import (
	"context"
	"testing"
)

// aumento a mitad de período con la convención real de cierre (la versión
// vieja termina el mismo día en que empieza la nueva) y copia derivada de
// salario_base_nominal mantenida por los triggers de 000124.
func TestAumentoAMitadDePeriodoYSalarioDerivado(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	ctx := context.Background()

	mustExec(t, f.db, `UPDATE contratos SET fecha_fin_vigencia = '2026-03-16' WHERE id = ?`, f.contrato[0])
	mustExec(t, f.db, `
INSERT INTO contratos (empleado_id, puesto_id, regimen_laboral_id, tipo_contrato_id, salario_pactado, modalidad_salario, jornada_id,
                       fecha_inicio_contrato, fecha_inicio_vigencia, creado_por)
SELECT empleado_id, puesto_id, regimen_laboral_id, tipo_contrato_id, 1000000, modalidad_salario, jornada_id, fecha_inicio_contrato, '2026-03-16', creado_por
FROM contratos WHERE id = ?`, f.contrato[0])

	var derivado string
	if err := f.db.QueryRow(`SELECT salario_base_nominal FROM empleado_datos_laborales WHERE empleado_id = ? AND fecha_fin_vigencia IS NULL`, f.emp[0]).
		Scan(&derivado); err != nil || derivado != "1000000.00" {
		t.Fatalf("salario_base_nominal derivado = %s, %v (se esperaba 1000000.00)", derivado, err)
	}
	mustExec(t, f.db, `UPDATE empleado_datos_laborales SET fecha_fin_vigencia = '2026-03-31' WHERE empleado_id = ? AND fecha_fin_vigencia IS NULL`, f.emp[1])
	mustExec(t, f.db, `
INSERT INTO empleado_datos_laborales (empleado_id, puesto_id, salario_base_nominal, tipo_contrato_id, programa_presupuestario_id, regimen_ccss_id, fecha_inicio_vigencia)
SELECT empleado_id, puesto_id, 0, tipo_contrato_id, programa_presupuestario_id, regimen_ccss_id, '2026-04-01'
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
	var salario, contrato string
	if err := f.db.QueryRow(`SELECT salario_base_periodo, contrato_id FROM planilla_detalle_empleado WHERE periodo_planilla_id = ? AND empleado_id = ?`,
		p.ID, f.emp[0]).Scan(&salario, &contrato); err != nil || salario != "950000.00" {
		t.Fatalf("salario_base_periodo = %s, %v", salario, err)
	}
	if contrato == "" || contrato == "0" {
		t.Errorf("contrato_id del detalle vacío")
	}
}
