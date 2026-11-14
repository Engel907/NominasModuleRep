//go:build integration

package planilla

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/testutil"
)

type fixtureIntegridadL2 struct {
	db         *sql.DB
	service    *PeriodoService
	tipoID     int64
	empleadoID int64
	programaID int64
	preparador int64
	aprobador  int64
	periodoSeq int
}

func nuevoFixtureIntegridadL2(t *testing.T) fixtureIntegridadL2 {
	t.Helper()
	db := testutil.NuevaBD(t)
	f := fixtureIntegridadL2{db: db}

	for i := 0; i < 2; i++ {
		result, err := db.Exec(`
INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash)
VALUES (?, ?, ?, 'test-hash')`,
			fmt.Sprintf("l2-user-%d", i), fmt.Sprintf("Usuario L2 %d", i),
			fmt.Sprintf("l2-user-%d@example.test", i))
		if err != nil {
			t.Fatalf("crear usuario de prueba: %v", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatalf("obtener ID del usuario de prueba: %v", err)
		}
		if i == 0 {
			f.preparador = id
		} else {
			f.aprobador = id
		}
	}

	if _, err := db.Exec(`
INSERT INTO tipos_planilla (codigo, nombre, clase, fraccion_mensual) VALUES ('L2_TEST', 'Integración L2', 'regular', 1)`); err != nil {
		t.Fatalf("crear tipo de planilla: %v", err)
	}
	if err := db.QueryRow(`SELECT id FROM tipos_planilla WHERE codigo = 'L2_TEST'`).Scan(&f.tipoID); err != nil {
		t.Fatalf("obtener tipo de planilla: %v", err)
	}

	var tipoIdentificacionID int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdentificacionID); err != nil {
		t.Fatalf("obtener tipo de identificación semilla: %v", err)
	}
	result, err := db.Exec(`
INSERT INTO empleados (codigo_interno, tipo_identificacion_id, numero_identificacion, nombre_completo, fecha_ingreso)
VALUES ('L2TEST', ?, 'L2-0001', 'Empleado de integración L2', '2020-01-01')`, tipoIdentificacionID)
	if err != nil {
		t.Fatalf("crear empleado de prueba: %v", err)
	}
	if f.empleadoID, err = result.LastInsertId(); err != nil {
		t.Fatal(err)
	}
	result, err = db.Exec(`
INSERT INTO programas_presupuestarios (codigo, nombre) VALUES ('L2TEST', 'Programa integración L2')`)
	if err != nil {
		t.Fatalf("crear programa presupuestario: %v", err)
	}
	if f.programaID, err = result.LastInsertId(); err != nil {
		t.Fatal(err)
	}
	f.service = NewPeriodoService(db, PeriodoGanchos{Calculador: calculadorNulo{}})
	return f
}

func (f *fixtureIntegridadL2) periodoCalculado(t *testing.T, conDetalle bool) dto.PeriodoCicloRow {
	t.Helper()
	f.periodoSeq++
	ctx := context.Background()
	inicio := time.Date(2300, time.January, f.periodoSeq*2-1, 0, 0, 0, 0, time.UTC)
	fin := inicio.AddDate(0, 0, 1)
	p, err := f.service.Crear(ctx, f.preparador, f.tipoID, inicio, fin)
	if err != nil {
		t.Fatalf("crear período: %v", err)
	}
	if conDetalle {
		if _, err := f.db.Exec(`
INSERT INTO planilla_detalle_empleado
  (periodo_planilla_id, empleado_id, programa_presupuestario_id, salario_base_periodo)
VALUES (?, ?, ?, 0)`, p.ID, f.empleadoID, f.programaID); err != nil {
			t.Fatalf("insertar detalle: %v", err)
		}
	}
	p, err = f.service.Calcular(ctx, f.preparador, p.ID)
	if err != nil {
		t.Fatalf("calcular período: %v", err)
	}
	return p
}

func errorTriggerIntegridadL2(err error, prefijo string) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1644 &&
		len(mysqlErr.Message) >= len(prefijo) && mysqlErr.Message[:len(prefijo)] == prefijo
}

func TestL2AprobarDetalleDescuadradoYPeriodoVacio(t *testing.T) {
	f := nuevoFixtureIntegridadL2(t)
	ctx := context.Background()

	p := f.periodoCalculado(t, true)
	if _, err := f.db.Exec(`
UPDATE planilla_detalle_empleado
SET total_ingresos = 100, neto_pagado = 100
WHERE periodo_planilla_id = ?`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.Aprobar(ctx, f.aprobador, p.ID); !errors.Is(err, ErrDetalleDescuadrado) {
		t.Fatalf("aprobar detalle descuadrado: err=%v, se esperaba ErrDetalleDescuadrado", err)
	}

	vacio := f.periodoCalculado(t, false)
	if _, err := f.service.Aprobar(ctx, f.aprobador, vacio.ID); !errors.Is(err, ErrPeriodoSinDetalle) {
		t.Fatalf("aprobar período vacío: err=%v, se esperaba ErrPeriodoSinDetalle", err)
	}
}

func TestL2EmbargoDePeriodoAprobadoEsInmutable(t *testing.T) {
	f := nuevoFixtureIntegridadL2(t)
	ctx := context.Background()
	p := f.periodoCalculado(t, true)

	var tipoEmbargoID int64
	if err := f.db.QueryRow(`SELECT id FROM parametros_tipos_embargo ORDER BY id LIMIT 1`).Scan(&tipoEmbargoID); err != nil {
		t.Fatalf("obtener tipo de embargo: %v", err)
	}
	result, err := f.db.Exec(`
INSERT INTO terceros (nombre, tipo) VALUES ('Tercero L2', 'juzgado')`)
	if err != nil {
		t.Fatalf("crear tercero: %v", err)
	}
	terceroID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	result, err = f.db.Exec(`
INSERT INTO embargos
  (empleado_id, tercero_id, tipo_embargo_id, monto_fijo, prioridad_aplicacion, fecha_inicio_vigencia)
VALUES (?, ?, ?, 10, 1, '2300-01-01')`, f.empleadoID, terceroID, tipoEmbargoID)
	if err != nil {
		t.Fatalf("crear embargo: %v", err)
	}
	embargoID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	result, err = f.db.Exec(`
INSERT INTO planilla_detalle_embargo
  (planilla_detalle_empleado_id, embargo_id, orden_prioridad, monto, monto_calculado, monto_insatisfecho)
SELECT id, ?, 1, 1, 1, 0
FROM planilla_detalle_empleado WHERE periodo_planilla_id = ?`, embargoID, p.ID)
	if err != nil {
		t.Fatalf("crear detalle de embargo: %v", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("insertar detalle de embargo afectó %d filas (err=%v)", n, err)
	}
	if _, err := f.service.Aprobar(ctx, f.aprobador, p.ID); err != nil {
		t.Fatalf("aprobar período: %v", err)
	}
	_, err = f.db.Exec(`
UPDATE planilla_detalle_embargo SET orden_prioridad = 2 WHERE embargo_id = ?`, embargoID)
	if !errorTriggerIntegridadL2(err, "periodo_inmutable:") {
		t.Fatalf("esperaba bloqueo de modificación de embargo aprobado; err=%v", err)
	}
}

func TestL2PorcentajeObreroInmutable(t *testing.T) {
	f := nuevoFixtureIntegridadL2(t)
	var id int64
	if err := f.db.QueryRow(`
SELECT id FROM parametros_cargas_sociales WHERE porcentaje_obrero <> 0 ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("obtener parámetro obrero semilla: %v", err)
	}
	_, err := f.db.Exec(`
UPDATE parametros_cargas_sociales SET porcentaje_obrero = porcentaje_obrero + 1 WHERE id = ?`, id)
	if !errorTriggerIntegridadL2(err, "parametro_inmutable:") {
		t.Fatalf("esperaba inmutabilidad de porcentaje_obrero; err=%v", err)
	}
}

func TestL2VigenciasGeneralesPermitenCierreYRechazanTraslape(t *testing.T) {
	f := nuevoFixtureIntegridadL2(t)

	insertar := func(clave, inicio string, fin any) int64 {
		t.Helper()
		result, err := f.db.Exec(`
INSERT INTO parametros_generales
  (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
VALUES (?, '1', 'entero', 'Prueba L2', 'TEST', ?, ?, ?)`,
			clave, inicio, fin, f.preparador)
		if err != nil {
			t.Fatalf("insertar parámetro %s: %v", clave, err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	primero := insertar("L2_TRASLAPE", "2301-01-01", "2301-06-30")
	insertar("L2_TRASLAPE", "2301-07-01", nil)
	_, err := f.db.Exec(`
UPDATE parametros_generales SET fecha_fin_vigencia = '2301-07-01' WHERE id = ?`, primero)
	if !errorTriggerIntegridadL2(err, "vigencia_traslapada:") {
		t.Fatalf("extender vigencia con traslape: err=%v", err)
	}

	cerrable := insertar("L2_CIERRE", "2302-01-01", nil)
	if _, err := f.db.Exec(`
UPDATE parametros_generales SET fecha_fin_vigencia = '2302-12-31' WHERE id = ?`, cerrable); err != nil {
		t.Fatalf("cerrar vigencia sin traslape: %v", err)
	}
	if _, err := f.db.Exec(`
UPDATE parametros_generales SET fecha_fin_vigencia = NULL WHERE id = ?`, cerrable); !errorTriggerIntegridadL2(err, "vigencia_cerrada:") {
		t.Fatalf("reabrir vigencia cerrada: err=%v", err)
	}
}
