//go:build integration

package asistencia_test

import (
	"context"
	"errors"
	"testing"

	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/testutil"
)

func TestCalendarioDesdeCatalogoYDiasLaborales(t *testing.T) {
	db := testutil.NuevaBD(t)
	ctx := context.Background()
	usuario := exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('cal', 'Calendario', 'cal@example.test', 'x')`)
	svc, err := asistencia.NewService(db, dto.New(db), parametros.NewGenerales(parametros.NewRepositorioSQL(db), 0))
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.GenerarCalendarioAnual(ctx, 2026, usuario); !errors.Is(err, asistencia.ErrDiasLaboralesNoConfigurados) {
		t.Fatalf("sin DIAS_LABORALES_SEMANA: se esperaba ErrDiasLaboralesNoConfigurados, se obtuvo %v", err)
	}

	exec(t, db, `INSERT INTO parametros_generales (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES ('DIAS_LABORALES_SEMANA', '1,2,3,4,5,6', 'texto', 'Lunes a sábado', 'Reglamento interno', '2026-01-01', ?)`, usuario)
	if err := svc.GenerarCalendarioAnual(ctx, 2026, usuario); err != nil {
		t.Fatal(err)
	}

	esFeriado := func(fecha string) (feriado, habil bool) {
		t.Helper()
		if err := db.QueryRow(`SELECT es_feriado, es_habil FROM calendario_laboral_dia WHERE fecha = ?`, fecha).Scan(&feriado, &habil); err != nil {
			t.Fatalf("%s: %v", fecha, err)
		}
		return
	}
	for fecha, esperado := range map[string][2]bool{
		"2026-04-02": {true, false},
		"2026-08-31": {true, false},
		"2026-10-12": {false, true},
		"2026-12-01": {true, false},
		"2026-03-07": {false, true},
		"2026-03-08": {false, false},
	} {
		if f, h := esFeriado(fecha); f != esperado[0] || h != esperado[1] {
			t.Errorf("%s: feriado=%t hábil=%t, se esperaba feriado=%t hábil=%t", fecha, f, h, esperado[0], esperado[1])
		}
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM feriados WHERE anio = 2026`).Scan(&total); err != nil || total != 12 {
		t.Fatalf("feriados 2026 = %d (%v), se esperaban 12", total, err)
	}

	var idNavidad int64
	if err := db.QueryRow(`SELECT id FROM feriados_catalogo WHERE nombre = 'Navidad'`).Scan(&idNavidad); err != nil {
		t.Fatal(err)
	}
	navidad, err := dto.New(db).ObtenerFeriadoCatalogo(ctx, idNavidad)
	if err != nil {
		t.Fatal(err)
	}
	navidad.Activo = false
	if _, err := svc.ActualizarFeriadoCatalogo(ctx, navidad, usuario); err != nil {
		t.Fatal(err)
	}
	if err := svc.GenerarCalendarioAnual(ctx, 2026, usuario); err != nil {
		t.Fatal(err)
	}
	if f, h := esFeriado("2026-12-25"); f || !h {
		t.Errorf("Navidad desactivada sigue como feriado=%t hábil=%t", f, h)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM feriados WHERE anio = 2026`).Scan(&total); err != nil || total != 11 {
		t.Fatalf("feriados 2026 tras desactivar = %d (%v), se esperaban 11", total, err)
	}
	var auditado int
	if err := db.QueryRow(`SELECT COUNT(*) FROM auditoria_cambios WHERE tabla_afectada = 'feriados_catalogo' AND accion = 'update' AND usuario_id = ?`, usuario).Scan(&auditado); err != nil || auditado != 1 {
		t.Fatalf("auditoría del catálogo = %d (%v), se esperaba 1", auditado, err)
	}
}
