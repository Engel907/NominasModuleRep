//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"
)

// Una jornada sin parametros_jornada vigente (NOCTURNA no se siembra) no
// calcula horas extra.
func TestJornadaYDivisor(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	var jornada string
	if err := f.db.QueryRow(`SELECT j.codigo FROM contratos c JOIN jornadas j ON j.id = c.jornada_id WHERE c.id = ?`, f.contrato[2]).Scan(&jornada); err != nil || jornada != "DIURNA" {
		t.Fatalf("jornada del contrato = %q, %v", jornada, err)
	}
	mustExec(t, f.db, `UPDATE contratos SET jornada_id = (SELECT id FROM jornadas WHERE codigo = 'NOCTURNA') WHERE id = ?`, f.contrato[2])

	svc := f.servicio()
	p := marzo(t, svc, f)
	_, err := svc.Calcular(context.Background(), f.preparador, p.ID)
	var ce *ErrCalculoConErrores
	if !errors.As(err, &ce) || len(ce.Errores) != 1 || ce.Errores[0].EmpleadoID != f.emp[2] || ce.Errores[0].Codigo != "calculo_invalido" {
		t.Fatalf("se esperaba error de jornada sin divisor solo para emp[2]: %v", err)
	}
}
