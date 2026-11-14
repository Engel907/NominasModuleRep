//go:build integration

package rrhh

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/testutil"
)

func TestAsignacionesDeConceptos(t *testing.T) {
	db := testutil.NuevaBD(t)
	ctx := context.Background()
	usuario := exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('asig', 'Asigna', 'asig@example.test', 'x')`)
	var tipoIdent, asoc, salBase int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM conceptos_salariales WHERE codigo = 'ASOC_SOLIDARISTA'`).Scan(&asoc); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM conceptos_salariales WHERE codigo = 'SAL_BASE'`).Scan(&salBase); err != nil {
		t.Fatal(err)
	}
	emp := testutil.InsertarEmpleado(t, db, "ASIG1", tipoIdent, "ASIG-0001", "Empleado asignaciones", "2025-01-01")
	asociacion := exec(t, db, `INSERT INTO terceros (nombre, tipo) VALUES ('Asociación Solidarista de Prueba', 'asociacion_solidarista')`)
	svc := NewAsignacionesService(db)

	in := AsignarConceptoInput{ConceptoSalarialID: asoc, Modo: "porcentaje", Valor: decimal.RequireFromString("5"),
		TerceroID: &asociacion, FechaInicioVigencia: "2026-01-01"}
	if _, err := svc.Asignar(ctx, usuario, emp, in); err != nil {
		t.Fatal(err)
	}

	// Reemplazo: cierra la anterior el día previo.
	in.Valor, in.FechaInicioVigencia = decimal.RequireFromString("4.5"), "2026-07-01"
	nueva, err := svc.Asignar(ctx, usuario, emp, in)
	if err != nil {
		t.Fatal(err)
	}
	lista, err := svc.Listar(ctx, emp)
	if err != nil || len(lista) != 2 || lista[1].FechaFinVigencia == nil || *lista[1].FechaFinVigencia != "2026-06-30" {
		t.Fatalf("historial inesperado: %+v, %v", lista, err)
	}

	// Una fecha que no es posterior a la vigente se rechaza.
	in.FechaInicioVigencia = "2026-07-01"
	var vig *VigenciaTraslapadaError
	if _, err := svc.Asignar(ctx, usuario, emp, in); !errors.As(err, &vig) {
		t.Fatalf("esperaba VigenciaTraslapadaError, obtuve %v", err)
	}

	// Un concepto sin fórmula asignacion_empleado no se puede asignar.
	in.ConceptoSalarialID, in.FechaInicioVigencia = salBase, "2026-08-01"
	var val *ErrValidacion
	if _, err := svc.Asignar(ctx, usuario, emp, in); !errors.As(err, &val) || val.Campo != "concepto_salarial_id" {
		t.Fatalf("esperaba error de validación del concepto, obtuve %v", err)
	}

	if err := svc.Finalizar(ctx, usuario, nueva, FinalizarAsignacionInput{FechaFinVigencia: "2026-12-31"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Finalizar(ctx, usuario, nueva, FinalizarAsignacionInput{FechaFinVigencia: "2026-12-31"}); !errors.Is(err, ErrAsignacionCerrada) {
		t.Fatalf("esperaba ErrAsignacionCerrada, obtuve %v", err)
	}
	// El historial no se puede alterar por fuera del servicio.
	if _, err := db.Exec(`UPDATE empleado_conceptos_asignados SET valor = 1 WHERE id = ?`, nueva); err == nil {
		t.Fatal("el trigger debía impedir cambiar el valor de una asignación")
	}
}
