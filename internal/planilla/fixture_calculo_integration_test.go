//go:build integration

package planilla

import (
	"database/sql"
	"fmt"
	"testing"

	"planillas-ccdr/internal/testutil"
)

// fixtureCalculo: BD efímera con todo lo necesario para calcular un período
// mensual de marzo 2026 con 3 empleados:
//   - emp[0]: pensión alimentaria del 12.5 %
//   - emp[1]: préstamo con cuota 25 000
//   - emp[2]: 6 horas extra diurnas autorizadas (incapacidad opcional con crearIncapacidad)
type fixtureCalculo struct {
	db             *sql.DB
	preparador     int64
	aprobador      int64
	pagador        int64
	tipoMensual    int64
	programaID     int64
	emp            [3]int64
	contrato       [3]int64
	embargoID      int64
	prestamoID     int64
	incapID        int64
	autorizacionHE int64
	concepto       map[string]int64 // codigo -> conceptos_salariales.id
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(q, args...)
	if err != nil {
		t.Fatalf("%v\n%s", err, q)
	}
	id, _ := res.LastInsertId()
	return id
}

func nuevoFixtureCalculo(t *testing.T) *fixtureCalculo {
	t.Helper()
	db := testutil.NuevaBD(t)
	f := &fixtureCalculo{db: db, concepto: map[string]int64{}}

	usuarios := []*int64{&f.preparador, &f.aprobador, &f.pagador}
	for i, u := range usuarios {
		*u = mustExec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES (?, ?, ?, 'test-hash')`,
			fmt.Sprintf("calc-%d", i), fmt.Sprintf("Usuario cálculo %d", i), fmt.Sprintf("calc-%d@example.test", i))
	}
	if err := db.QueryRow(`SELECT id FROM tipos_planilla WHERE clase = 'regular' AND fraccion_mensual = 1`).Scan(&f.tipoMensual); err != nil {
		t.Fatalf("tipo mensual sembrado (L3): %v", err)
	}
	rows, err := db.Query(`SELECT codigo, id FROM conceptos_salariales`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var c string
		var id int64
		if err := rows.Scan(&c, &id); err != nil {
			t.Fatal(err)
		}
		f.concepto[c] = id
	}
	rows.Close()

	var tipoIdent, regimen int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM regimenes_laborales WHERE codigo='CODIGO_TRABAJO'`).Scan(&regimen); err != nil {
		t.Fatal(err)
	}
	unidad := mustExec(t, db, `INSERT INTO unidades_organizativas (codigo, nombre) VALUES ('CALC', 'Unidad cálculo')`)
	puesto := mustExec(t, db, `INSERT INTO puestos (codigo, nombre, unidad_organizativa_id) VALUES ('CALC', 'Puesto cálculo', ?)`, unidad)
	f.programaID = mustExec(t, db, `INSERT INTO programas_presupuestarios (codigo, nombre) VALUES ('CALC', 'Programa cálculo')`)
	tipoContrato := mustExec(t, db, `INSERT INTO tipos_contrato (codigo, nombre) VALUES ('CALC_IND', 'Indefinido (prueba)')`)
	tercero := mustExec(t, db, `INSERT INTO terceros (nombre, tipo) VALUES ('Juzgado de prueba', 'juzgado')`)

	salarios := []string{"900000.00", "800000.00", "700000.00"}
	for i := range f.emp {
		f.emp[i] = mustExec(t, db, `
INSERT INTO empleados (codigo_interno, tipo_identificacion_id, numero_identificacion, nombre_completo, fecha_ingreso)
VALUES (?, ?, ?, ?, '2020-01-01')`, fmt.Sprintf("CALC%d", i), tipoIdent, fmt.Sprintf("CALC-%04d", i), fmt.Sprintf("Empleado cálculo %d", i))
		f.contrato[i] = mustExec(t, db, `
INSERT INTO contratos (empleado_id, puesto_id, regimen_laboral_id, tipo_contrato, salario_pactado, jornada,
                       fecha_inicio_contrato, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, ?, 'INDEFINIDO', ?, 'DIURNA', '2020-01-01', '2020-01-01', ?)`, f.emp[i], puesto, regimen, salarios[i], f.preparador)
		mustExec(t, db, `
INSERT INTO empleado_datos_laborales (empleado_id, puesto_id, salario_base_nominal, tipo_contrato_id, programa_presupuestario_id, fecha_inicio_vigencia)
VALUES (?, ?, ?, ?, ?, '2020-01-01')`, f.emp[i], puesto, salarios[i], tipoContrato, f.programaID)
	}

	var tipoPension int64
	if err := db.QueryRow(`SELECT id FROM parametros_tipos_embargo WHERE es_pension_alimentaria = 1 AND fecha_fin_vigencia IS NULL`).Scan(&tipoPension); err != nil {
		t.Fatal(err)
	}
	f.embargoID = mustExec(t, db, `
INSERT INTO embargos (empleado_id, tercero_id, tipo_embargo_id, porcentaje, prioridad_aplicacion, fecha_inicio_vigencia)
VALUES (?, ?, ?, 12.50, 1, '2025-01-01')`, f.emp[0], tercero, tipoPension)
	f.prestamoID = mustExec(t, db, `
INSERT INTO prestamos (empleado_id, tercero_id, monto_total, cuota_periodica, saldo_pendiente, fecha_inicio)
VALUES (?, ?, 300000, 25000, 300000, '2025-01-01')`, f.emp[1], tercero)
	mustExec(t, db, `
INSERT INTO registro_asistencia (empleado_id, fecha, horas_ordinarias, horas_extra_diurnas) VALUES (?, '2026-03-10', 8, 6)`, f.emp[2])
	// L13: sin autorización la hora extra no llega a planilla.
	f.autorizacionHE = mustExec(t, db, `
INSERT INTO horas_extra_autorizadas (empleado_id, fecha, horas_autorizadas, motivo, estado, solicitado_por, resuelto_por, fecha_resolucion)
VALUES (?, '2026-03-10', 6, 'fixture', 'autorizada', ?, ?, NOW())`, f.emp[2], f.preparador, f.aprobador)
	return f
}

// crearIncapacidad registra una boleta CCSS del 27-feb al 4-mar-2026 para emp[2].
func (f *fixtureCalculo) crearIncapacidad(t *testing.T) {
	t.Helper()
	f.incapID = mustExec(t, f.db, `
INSERT INTO incapacidades (empleado_id, entidad_emisora, numero_boleta, fecha_inicio, fecha_fin, dias, creado_por)
VALUES (?, 'CCSS', 'B-CALC-1', '2026-02-27', '2026-03-04', 6, ?)`, f.emp[2], f.preparador)
}
