//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

// embargos con escala, prioridad, tope de pensión y tope mensual desde BD;
// el líquido embargable solo suma ingresos con afecta_base_embargo.
func TestEmbargosDesdeBD(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)

	var afectaSal, afectaViaticos int
	if err := f.db.QueryRow(`SELECT
	  (SELECT afecta_base_embargo FROM conceptos_salariales WHERE codigo = 'SAL_BASE'),
	  (SELECT afecta_base_embargo FROM conceptos_salariales WHERE codigo = 'VIATICOS')`).Scan(&afectaSal, &afectaViaticos); err != nil {
		t.Fatal(err)
	}
	if afectaSal != 1 || afectaViaticos != 0 {
		t.Fatalf("afecta_base_embargo: SAL_BASE=%d VIATICOS=%d (esperado 1/0)", afectaSal, afectaViaticos)
	}

	var tipoJudicial, tercero int64
	if err := f.db.QueryRow(`SELECT id FROM parametros_tipos_embargo WHERE codigo = 'EMBARGO_JUDICIAL' AND fecha_fin_vigencia IS NULL`).Scan(&tipoJudicial); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`SELECT tercero_id FROM embargos WHERE id = ?`, f.embargoID).Scan(&tercero); err != nil {
		t.Fatal(err)
	}
	judicial := mustExec(t, f.db, `
INSERT INTO embargos (empleado_id, tercero_id, tipo_embargo_id, porcentaje, tope_mensual, prioridad_aplicacion, fecha_inicio_vigencia)
VALUES (?, ?, ?, 10.00, 10000, 2, '2025-01-01')`, f.emp[0], tercero, tipoJudicial)

	svc := f.servicio()
	p := marzo(t, svc, f)
	if _, err := svc.Calcular(context.Background(), f.preparador, p.ID); err != nil {
		var ce *ErrCalculoConErrores
		if errors.As(err, &ce) {
			t.Fatalf("calcular: %+v", ce.Errores)
		}
		t.Fatalf("calcular: %v", err)
	}

	var obrera, renta decimal.Decimal
	if err := f.db.QueryRow(`SELECT cuota_obrera_ccss, renta_neta FROM planilla_detalle_empleado WHERE periodo_planilla_id = ? AND empleado_id = ?`,
		p.ID, f.emp[0]).Scan(&obrera, &renta); err != nil {
		t.Fatal(err)
	}
	liquido := decimal.RequireFromString("900000").Sub(obrera).Sub(renta)
	pension := liquido.Mul(decimal.RequireFromString("0.125")).Round(2)

	filas := map[int64][3]decimal.Decimal{}
	rows, err := f.db.Query(`
SELECT pdemb.embargo_id, pdemb.monto_calculado, pdemb.monto, pdemb.monto_insatisfecho
FROM planilla_detalle_embargo pdemb
JOIN planilla_detalle_empleado pde ON pde.id = pdemb.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ?`, p.ID, f.emp[0])
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		var c, m, i decimal.Decimal
		if err := rows.Scan(&id, &c, &m, &i); err != nil {
			t.Fatal(err)
		}
		filas[id] = [3]decimal.Decimal{c, m, i}
	}
	rows.Close()
	if len(filas) != 2 {
		t.Fatalf("filas de planilla_detalle_embargo: %v", filas)
	}
	if x := filas[f.embargoID]; !x[0].Equal(pension) || !x[1].Equal(pension) || !x[2].IsZero() {
		t.Errorf("pensión calculado/aplicado/insatisfecho = %v, esperado %s completo", x, pension)
	}
	if x := filas[judicial]; !x[0].Equal(decimal.NewFromInt(10000)) || !x[1].Equal(decimal.NewFromInt(10000)) || !x[2].IsZero() {
		t.Errorf("judicial calculado/aplicado/insatisfecho = %v, esperado 10000 topado", x)
	}
}
