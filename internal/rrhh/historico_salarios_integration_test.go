//go:build integration

package rrhh

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/testutil"
)

func TestHistoricoSalariosCompletaLasBases(t *testing.T) {
	db := testutil.NuevaBD(t)
	ctx := context.Background()
	usuario := exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('hist', 'Histórico', 'hist@example.test', 'x')`)
	var tipoIdent int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	emp := testutil.InsertarEmpleado(t, db, "HIST1", tipoIdent, "HIST-0001", "Empleado histórico", "2024-01-01")
	svc := NewHistoricoSalariosService(db)
	d := decimal.RequireFromString
	mes := func(anio, m int32, bruto, agui string) MesHistoricoInput {
		return MesHistoricoInput{Anio: anio, Mes: m, MontoBruto: d(bruto), BaseAguinaldo: d(agui), BaseCesantia: d(bruto), BaseVacaciones: d(bruto)}
	}
	in := CargarHistoricoInput{Fuente: "Sistema anterior", Meses: []MesHistoricoInput{mes(2025, 12, "500000", "500000"), mes(2026, 1, "520000", "500000")}}
	if n, err := svc.Cargar(ctx, usuario, emp, in); err != nil || n != 2 {
		t.Fatalf("cargar: %d, %v", n, err)
	}
	var val *ErrValidacion
	if _, err := svc.Cargar(ctx, usuario, emp, CargarHistoricoInput{Fuente: "x", Meses: []MesHistoricoInput{mes(2026, 1, "1", "1")}}); !errors.As(err, &val) {
		t.Fatalf("esperaba rechazo del mes ya cargado, obtuve %v", err)
	}
	if _, err := svc.Cargar(ctx, usuario, emp, CargarHistoricoInput{Fuente: "x", Meses: []MesHistoricoInput{mes(2026, 2, "100", "200")}}); !errors.As(err, &val) {
		t.Fatalf("esperaba rechazo de una base mayor que el bruto, obtuve %v", err)
	}

	q := dto.New(db)
	desde, hasta := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)
	base, err := q.SumarBaseAcumuladaEmpleado(ctx, emp, desde, hasta, "base_aguinaldo")
	if err != nil || !base.Equal(d("1000000")) {
		t.Fatalf("base de aguinaldo %s, %v; esperaba 1000000", base, err)
	}
	prom, err := q.SumarBasePromedioEmpleado(ctx, emp, desde, hasta, "base_cesantia")
	if err != nil || !prom.Total.Equal(d("1020000")) || !prom.Meses.Equal(d("2")) || prom.Periodos != 2 {
		t.Fatalf("promedio %+v, %v", prom, err)
	}

	lista, err := svc.Listar(ctx, emp)
	if err != nil || len(lista) != 2 {
		t.Fatalf("listar: %+v, %v", lista, err)
	}
	if err := svc.Eliminar(ctx, usuario, emp, lista[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Eliminar(ctx, usuario, emp, lista[0].ID); !errors.Is(err, ErrHistoricoNoEncontrado) {
		t.Fatalf("esperaba ErrHistoricoNoEncontrado, obtuve %v", err)
	}
}
