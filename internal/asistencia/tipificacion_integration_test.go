//go:build integration

package asistencia_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/testutil"
)

// de punta a punta: RegistrarAsistencia tipifica el exceso y lo guarda en
// asistencia_hora_extra_tipificada (feriado/descanso → doble; franja nocturna →
// nocturna; resto → diurna). Sin franja vigente, un día hábil con exceso falla
// y no se guarda nada.
func TestRegistrarAsistenciaTipificaHorasExtra(t *testing.T) {
	db := testutil.NuevaBD(t)
	ctx := context.Background()
	usuario := exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('tip', 'Tip', 'tip@example.test', 'x')`)
	var tipoIdent, regimen int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM regimenes_laborales WHERE codigo='CODIGO_TRABAJO'`).Scan(&regimen); err != nil {
		t.Fatal(err)
	}
	emp := testutil.InsertarEmpleado(t, db, "TIP1", tipoIdent, "TIP-0001", "Empleado tipificación", "2020-01-01")
	unidad := exec(t, db, `INSERT INTO unidades_organizativas (codigo, nombre) VALUES ('TIP', 'Unidad')`)
	puesto := exec(t, db, `INSERT INTO puestos (codigo, nombre, unidad_organizativa_id) VALUES ('TIP', 'Puesto', ?)`, unidad)
	exec(t, db, `INSERT INTO tipos_contrato (codigo, nombre) VALUES ('TIP_IND', 'Indefinido')`)
	exec(t, db, `INSERT INTO contratos (empleado_id, puesto_id, regimen_laboral_id, tipo_contrato_id, salario_pactado, modalidad_salario, jornada_id,
                       fecha_inicio_contrato, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, ?, (SELECT MIN(id) FROM tipos_contrato), 700000, 'mensual', (SELECT id FROM jornadas WHERE codigo = 'DIURNA'), '2020-01-01', '2020-01-01', ?)`, emp, puesto, regimen, usuario)
	turno := exec(t, db, `INSERT INTO turnos (codigo, nombre, hora_inicio, hora_fin) VALUES ('T0715', 'Diurno', '07:00:00', '15:00:00')`)
	exec(t, db, `INSERT INTO empleado_turno (empleado_id, turno_id, fecha_inicio_vigencia) VALUES (?, ?, '2020-01-01')`, emp, turno)
	for _, d := range []struct {
		fecha          string
		habil, feriado int
		dia            int
	}{{"2026-03-10", 1, 0, 2}, {"2026-03-11", 0, 1, 3}, {"2026-03-14", 0, 0, 6}} {
		exec(t, db, `INSERT INTO calendario_laboral_dia (fecha, anio, es_habil, es_feriado, dia_semana) VALUES (?, 2026, ?, ?, ?)`, d.fecha, d.habil, d.feriado, d.dia)
	}

	svc, err := asistencia.NewService(db, dto.New(db), parametros.NewGenerales(parametros.NewRepositorioSQL(db), 0))
	if err != nil {
		t.Fatal(err)
	}
	registrar := func(fecha, entrada, salida string) error {
		f, _ := time.Parse("2006-01-02", fecha)
		e, _ := time.Parse("15:04", entrada)
		s, _ := time.Parse("15:04", salida)
		_, err := svc.RegistrarAsistencia(ctx, asistencia.RegistrarAsistenciaInput{
			EmpleadoID: emp, Fecha: f, HoraEntradaReal: &e, HoraSalidaReal: &s, Estado: "presente", CreadoPor: usuario})
		return err
	}
	tipificadas := func(fecha string) map[string]string {
		rows, err := db.Query(`SELECT t.clase, h.horas FROM asistencia_hora_extra_tipificada h JOIN tipos_hora_extra t ON t.id = h.tipo_hora_extra_id
WHERE h.empleado_id = ? AND h.fecha = ?`, emp, fecha)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var c, h string
			if err := rows.Scan(&c, &h); err != nil {
				t.Fatal(err)
			}
			out[c] = h
		}
		return out
	}

	if err := registrar("2026-03-10", "07:00", "17:00"); !errors.Is(err, asistencia.ErrFranjaNocturnaNoVigente) {
		t.Fatalf("sin franja: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM registro_asistencia_detalle WHERE empleado_id = ?`, emp).Scan(&n); err != nil || n != 0 {
		t.Fatalf("el registro fallido no debe guardarse: %d, %v", n, err)
	}
	if err := registrar("2026-03-11", "07:00", "17:00"); err != nil {
		t.Fatalf("feriado: %v", err)
	}
	if got := tipificadas("2026-03-11"); len(got) != 1 || got["doble"] != "2.00" {
		t.Errorf("feriado: %v", got)
	}
	if err := registrar("2026-03-14", "07:00", "16:30"); err != nil {
		t.Fatalf("descanso: %v", err)
	}
	if got := tipificadas("2026-03-14"); len(got) != 1 || got["doble"] != "1.50" {
		t.Errorf("descanso: %v", got)
	}

	exec(t, db, `INSERT INTO parametros_franja_nocturna (hora_inicio, hora_fin, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES ('19:00:00', '05:00:00', 'FIXTURE DE PRUEBA', '2026-01-01', 1)`)
	if err := registrar("2026-03-10", "07:00", "20:00"); err != nil {
		t.Fatalf("día hábil: %v", err)
	}
	if got := tipificadas("2026-03-10"); len(got) != 2 || got["diurna"] != "4.00" || got["nocturna"] != "1.00" {
		t.Errorf("día hábil: %v", got)
	}
	if err := registrar("2026-03-10", "07:00", "15:00"); err != nil {
		t.Fatal(err)
	}
	if got := tipificadas("2026-03-10"); len(got) != 0 {
		t.Errorf("re-registro sin exceso: %v", got)
	}
}
