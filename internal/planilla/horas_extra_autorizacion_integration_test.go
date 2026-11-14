//go:build integration

package planilla

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func (f *fixtureCalculo) montoConcepto(t *testing.T, periodoID, emp int64, codigo string) (string, bool) {
	t.Helper()
	var m string
	err := f.db.QueryRow(`
SELECT pdc.monto FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ? AND pdc.concepto_salarial_id = ?`, periodoID, emp, f.concepto[codigo]).Scan(&m)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return m, true
}

// lo que llega a planilla es min(horas registradas, horas autorizadas) de
// los días autorizados; sin autorización no se paga.
func TestHorasExtraSoloAutorizadas(t *testing.T) {
	ctx := context.Background()
	casos := []struct {
		nombre string
		prep   func(t *testing.T, f *fixtureCalculo)
		monto  string
	}{
		{"autorizadas las 6", func(*testing.T, *fixtureCalculo) {}, "26250.00"},
		{"autorizado menos de lo trabajado", func(t *testing.T, f *fixtureCalculo) {
			mustExec(t, f.db, `DELETE FROM horas_extra_autorizadas WHERE id = ?`, f.autorizacionHE)
			mustExec(t, f.db, `
INSERT INTO horas_extra_autorizadas (empleado_id, fecha, horas_autorizadas, motivo, estado, solicitado_por, resuelto_por, fecha_resolucion)
VALUES (?, '2026-03-10', 4, 'tope', 'autorizada', ?, ?, NOW())`, f.emp[2], f.preparador, f.aprobador)
		}, "17500.00"},
		{"exceso sin autorización", func(t *testing.T, f *fixtureCalculo) {
			mustExec(t, f.db, `DELETE FROM horas_extra_autorizadas WHERE id = ?`, f.autorizacionHE)
		}, ""},
		{"solicitud pendiente no se paga", func(t *testing.T, f *fixtureCalculo) {
			mustExec(t, f.db, `UPDATE horas_extra_autorizadas SET estado = 'pendiente', resuelto_por = NULL, fecha_resolucion = NULL WHERE id = ?`, f.autorizacionHE)
		}, ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			f := nuevoFixtureCalculo(t)
			f.sembrarSalarioMinimoInembargable(t)
			c.prep(t, f)
			svc := f.servicio()
			p := marzo(t, svc, f)
			if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
				t.Fatalf("calcular: %v", err)
			}
			got, ok := f.montoConcepto(t, p.ID, f.emp[2], "HRS_EXTRA")
			if c.monto == "" && ok {
				t.Fatalf("no debía haber HRS_EXTRA y hay %s", got)
			}
			if c.monto != "" && got != c.monto {
				t.Fatalf("HRS_EXTRA = %q, esperado %s", got, c.monto)
			}
		})
	}
}
