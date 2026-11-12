//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"

	"planillas-ccdr/internal/pagination"
)

func TestL9DevengosManuales(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	ctx := context.Background()
	svc := f.servicio()
	dev := NewDevengosService(f.db)
	p := marzo(t, svc, f)

	obs := "comisión de marzo"
	r, err := dev.Crear(ctx, f.preparador, p.ID, NuevoDevengo{EmpleadoID: f.emp[2], ConceptoSalarialID: f.concepto["COMISIONES"], Monto: d("45000.50"), Observacion: &obs})
	if err != nil {
		t.Fatalf("crear devengo: %v", err)
	}
	if r.ConceptoCodigo != "COMISIONES" || !r.Monto.Equal(d("45000.50")) || r.CreadoPor != f.preparador {
		t.Fatalf("devengo creado: %+v", r)
	}

	// Concepto que no es de captura manual (SAL_BASE) -> 422; monto inválido -> 400.
	if _, err := dev.Crear(ctx, f.preparador, p.ID, NuevoDevengo{EmpleadoID: f.emp[2], ConceptoSalarialID: f.concepto["SAL_BASE"], Monto: d("10")}); !errors.Is(err, ErrConceptoNoCapturable) {
		t.Errorf("SAL_BASE no es capturable: %v", err)
	}
	if _, err := dev.Crear(ctx, f.preparador, p.ID, NuevoDevengo{EmpleadoID: f.emp[2], ConceptoSalarialID: f.concepto["BONO_DISCRECIONAL"], Monto: d("10.555")}); !errors.Is(err, ErrMontoDevengoInvalido) {
		t.Errorf("3 decimales debe ser inválido: %v", err)
	}
	borrar, err := dev.Crear(ctx, f.preparador, p.ID, NuevoDevengo{EmpleadoID: f.emp[0], ConceptoSalarialID: f.concepto["VIATICOS"], Monto: d("1000")})
	if err != nil {
		t.Fatal(err)
	}
	if err := dev.Eliminar(ctx, f.preparador, p.ID, borrar.ID); err != nil {
		t.Fatalf("eliminar: %v", err)
	}
	if err := dev.Eliminar(ctx, f.preparador, p.ID, borrar.ID); !errors.Is(err, ErrDevengoNoEncontrado) {
		t.Errorf("eliminar dos veces: %v", err)
	}
	filas, total, err := dev.Listar(ctx, p.ID, nil, pagination.Parametros{})
	if err != nil || total != 1 || len(filas) != 1 {
		t.Fatalf("listar: %d filas, total %d, %v", len(filas), total, err)
	}

	// El motor consume el devengo.
	if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
		t.Fatalf("calcular: %v", err)
	}
	var monto string
	if err := f.db.QueryRow(`
SELECT pdc.monto FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ? AND pdc.concepto_salarial_id = ?`, p.ID, f.emp[2], f.concepto["COMISIONES"]).Scan(&monto); err != nil || monto != "45000.50" {
		t.Fatalf("línea COMISIONES = %q, %v", monto, err)
	}

	// Período calculado: no admite devengos (409).
	if _, err := dev.Crear(ctx, f.preparador, p.ID, NuevoDevengo{EmpleadoID: f.emp[1], ConceptoSalarialID: f.concepto["COMISIONES"], Monto: d("5")}); !errors.Is(err, ErrEstadoInvalido) {
		t.Errorf("devengo en período calculado: %v, se esperaba 409", err)
	}
	if err := dev.Eliminar(ctx, f.preparador, p.ID, r.ID); !errors.Is(err, ErrEstadoInvalido) {
		t.Errorf("eliminar en período calculado: %v, se esperaba 409", err)
	}
}
