//go:build integration

package asistencia_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/testutil"
)

func exec(t *testing.T, db *sql.DB, q string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(q, args...)
	if err != nil {
		t.Fatalf("%v\n%s", err, q)
	}
	id, _ := res.LastInsertId()
	return id
}

// quien solicita no puede resolver (403 en Go, CHECK en la BD); una
// autorización se resuelve una sola vez; candidatas = días sin autorización.
func TestSegregacionAutorizacion(t *testing.T) {
	db := testutil.NuevaBD(t)
	ctx := context.Background()
	var u [3]int64
	for i := range u {
		u[i] = exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES (?, ?, ?, 'x')`,
			fmt.Sprintf("he-%d", i), fmt.Sprintf("HE %d", i), fmt.Sprintf("he-%d@example.test", i))
	}
	var tipoIdent int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	emp := testutil.InsertarEmpleado(t, db, "HE1", tipoIdent, "HE-0001", "Empleado HE", "2020-01-01")

	he := asistencia.NewHorasExtraService(db)
	dia := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	sol, err := he.Solicitar(ctx, u[0], asistencia.SolicitudHorasExtra{EmpleadoID: emp, Fecha: dia, HorasAutorizadas: decimal.NewFromInt(2), Motivo: "cierre de mes"})
	if err != nil || sol.Estado != asistencia.EstadoHEPendiente {
		t.Fatalf("solicitar: %+v, %v", sol, err)
	}
	if _, err := he.Solicitar(ctx, u[0], asistencia.SolicitudHorasExtra{EmpleadoID: emp, Fecha: dia, HorasAutorizadas: decimal.NewFromInt(1), Motivo: "otra"}); !errors.Is(err, asistencia.ErrAutorizacionDuplicada) {
		t.Errorf("segunda solicitud del mismo día: %v", err)
	}
	if _, err := he.Solicitar(ctx, u[0], asistencia.SolicitudHorasExtra{EmpleadoID: emp, Fecha: dia.AddDate(0, 0, 1), HorasAutorizadas: decimal.RequireFromString("25"), Motivo: "x"}); !errors.Is(err, asistencia.ErrHorasAutorizadasInvalida) {
		t.Errorf("25 horas: %v", err)
	}
	if _, err := he.Autorizar(ctx, u[0], sol.ID); !errors.Is(err, asistencia.ErrSegregacionHorasExtra) {
		t.Fatalf("el solicitante no puede autorizar: %v", err)
	}
	_, err = db.Exec(`UPDATE horas_extra_autorizadas SET estado = 'autorizada', resuelto_por = solicitado_por WHERE id = ?`, sol.ID)
	var me *mysql.MySQLError
	if !errors.As(err, &me) || me.Number != 3819 {
		t.Fatalf("el CHECK chk_hea_segregacion debía rechazar: %v", err)
	}
	r, err := he.Autorizar(ctx, u[1], sol.ID)
	if err != nil || r.Estado != asistencia.EstadoHEAutorizada || r.ResueltoPor == nil || *r.ResueltoPor != u[1] {
		t.Fatalf("autorizar por otro usuario: %+v, %v", r, err)
	}
	if _, err := he.Rechazar(ctx, u[2], sol.ID, "tarde"); !errors.Is(err, asistencia.ErrAutorizacionYaResuelta) {
		t.Errorf("resolver dos veces: %v", err)
	}

	exec(t, db, `INSERT INTO asistencia_hora_extra_tipificada (empleado_id, fecha, tipo_hora_extra_id, horas) SELECT ?, '2026-03-12', id, 3 FROM tipos_hora_extra WHERE clase = 'diurna'`, emp)
	exec(t, db, `INSERT INTO asistencia_hora_extra_tipificada (empleado_id, fecha, tipo_hora_extra_id, horas) SELECT ?, '2026-03-20', id, 2 FROM tipos_hora_extra WHERE clase = 'nocturna'`, emp)
	cand, err := he.Candidatas(ctx, nil, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if err != nil || len(cand) != 1 || cand[0].Fecha.Day() != 20 || !cand[0].HorasRegistradas.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("candidatas: %+v, %v", cand, err)
	}
}
