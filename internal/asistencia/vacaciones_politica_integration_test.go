//go:build integration

package asistencia_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/testutil"
)

// el job de acumulación se puede correr a diario sin duplicar; la
// compensación resta saldo y queda pendiente de pago.
func TestAcumulacionRepetidaYCompensacion(t *testing.T) {
	db := testutil.NuevaBD(t)
	ctx := context.Background()
	u := exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('vac-job', 'Job vacaciones', 'vac@example.test', 'x')`)
	var tipoIdent int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	var emp [2]int64
	for i, ingreso := range []string{"2020-02-01", "2026-02-15"} {
		emp[i] = testutil.InsertarEmpleado(t, db, fmt.Sprintf("VAC%d", i), tipoIdent, fmt.Sprintf("VAC-%04d", i), "Empleado vacaciones", ingreso)
	}
	q := dto.New(db)
	svc := asistencia.NewVacacionesService(db, q)

	for _, d := range []string{"2026-03-05", "2026-03-20"} {
		f, _ := time.Parse("2006-01-02", d)
		res, err := svc.AcumularSegunPolitica(ctx, q, asistencia.PoliticaMensual, f, u)
		if err != nil || len(res.Errores) != 0 {
			t.Fatalf("acumular %s: %+v, %v", d, res, err)
		}
		if d == "2026-03-05" && (res.Acreditados != 1 || res.SinDerecho != 1) {
			t.Errorf("primera corrida: %+v", res)
		}
		if d == "2026-03-20" && (res.Acreditados != 0 || res.YaRegistrados != 1) {
			t.Errorf("segunda corrida no debe duplicar: %+v", res)
		}
	}
	saldo, err := svc.ConsultarSaldo(ctx, emp[0], time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
	if err != nil || !saldo.DiasDisponibles.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("saldo tras acumular febrero = %s, %v (esperado 1)", saldo.DiasDisponibles, err)
	}

	if _, err := svc.RegistrarCompensacion(ctx, asistencia.RegistrarCompensacionInput{EmpleadoID: emp[0], Dias: decimal.NewFromInt(2),
		FechaEfectiva: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC), RegistradoPor: u}); !errors.Is(err, asistencia.ErrVacSaldoInsuficiente) {
		t.Fatalf("compensar más que el saldo: %v", err)
	}
	s, err := svc.RegistrarCompensacion(ctx, asistencia.RegistrarCompensacionInput{EmpleadoID: emp[0], Dias: decimal.RequireFromString("0.5"),
		FechaEfectiva: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC), RegistradoPor: u})
	if err != nil || !s.Saldo.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("compensar 0.5: %+v, %v", s, err)
	}
	pend, err := q.ListarCompensacionesVacacionesPendientes(ctx, emp[0], 0, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if err != nil || len(pend) != 1 || !pend[0].Dias.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("compensaciones pendientes: %+v, %v", pend, err)
	}
	if _, err := svc.AcumularSegunPolitica(ctx, q, "diaria", time.Now(), u); !errors.Is(err, asistencia.ErrVacPoliticaInvalida) {
		t.Errorf("política inválida: %v", err)
	}
}
