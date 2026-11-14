//go:build integration

package rrhh

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"planillas-ccdr/internal/cripto"
	"planillas-ccdr/internal/testutil"
)

const (
	cedulaPrueba = "1-0234-0567"
	ibanPrueba   = "CR05015202001026284066"
)

func llaveroPrueba(t *testing.T, llaves, activo string) *cripto.Llavero {
	t.Helper()
	l, err := cripto.NuevoLlavero(llaves, activo, strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

var llaveD1 = "d1:" + strings.Repeat("cd", 32)

type fixtureL26 struct {
	db       *sql.DB
	usuario  int64
	empleado int64
	svc      EmpleadoService
	llavero  *cripto.Llavero
}

func nuevoFixtureL26(t *testing.T) *fixtureL26 {
	t.Helper()
	db := testutil.NuevaBD(t)
	f := &fixtureL26{db: db, llavero: llaveroPrueba(t, llaveD1, "")}
	f.usuario = exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('l26', 'RR. HH.', 'l26@example.test', 'x')`)
	var tipoIdent int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	unidad := exec(t, db, `INSERT INTO unidades_organizativas (codigo, nombre) VALUES ('L26', 'Unidad')`)
	puesto := exec(t, db, `INSERT INTO puestos (codigo, nombre, unidad_organizativa_id) VALUES ('L26', 'Puesto', ?)`, unidad)
	programa := exec(t, db, `INSERT INTO programas_presupuestarios (codigo, nombre) VALUES ('L26', 'Programa')`)
	tipoContrato := exec(t, db, `INSERT INTO tipos_contrato (codigo, nombre) VALUES ('L26', 'Indefinido')`)
	banco := exec(t, db, `INSERT INTO bancos (codigo, nombre, es_depositario_nomina) VALUES ('L26', 'Banco prueba', 1)`)

	f.svc = NewEmpleadoService(db, f.llavero)
	resp, err := f.svc.CrearExpediente(context.Background(), f.usuario, CrearEmpleadoRequest{
		CodigoInterno: "L26-1", TipoIdentificacionID: tipoIdent, NumeroIdentificacion: cedulaPrueba,
		NombreCompleto: "Ana Pérez", FechaIngreso: "2024-01-15",
		DatosPersonales: DatosPersonalesInput{Direccion: "San José, 100 m norte", Telefono: "8888-1234",
			Email: "ana.perez@ccdr.go.cr", FechaEfectiva: "2024-01-15"},
		DatosLaborales: DatosLaboralesInput{PuestoID: puesto, TipoContratoID: tipoContrato, ProgramaPresupuestarioID: programa,
			FechaEfectiva: "2024-01-15"},
		DatosBancarios: &DatosBancariosInput{BancoID: banco, TipoCuenta: "ahorros", NumeroCuenta: ibanPrueba, FechaEfectiva: "2024-01-15"},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.empleado = resp.ID
	return f
}

func (f *fixtureL26) contarAccesos(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM registro_accesos_datos WHERE empleado_id = ?`, f.empleado).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Ejecuta las variantes dinámicas de GuardarCifrado y ListarPendientesCifrado
// en las 5 columnas sensibles.
func TestCifradoYAccesos(t *testing.T) {
	f := nuevoFixtureL26(t)
	ctx := context.Background()

	var ct []byte
	var hash, kid string
	if err := f.db.QueryRow(`SELECT numero_identificacion_cifrado, numero_identificacion_hash, numero_identificacion_kid FROM empleados WHERE id = ?`, f.empleado).
		Scan(&ct, &hash, &kid); err != nil {
		t.Fatal(err)
	}
	if len(ct) == 0 || kid != "d1" || hash != f.llavero.HashBusqueda("empleados.numero_identificacion", "102340567") {
		t.Fatalf("cifrado=%d bytes kid=%s hash=%s", len(ct), kid, hash)
	}
	var cuentas int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM empleado_datos_bancarios WHERE numero_cuenta_cifrado IS NOT NULL`).Scan(&cuentas); err != nil || cuentas != 1 {
		t.Fatalf("cuenta cifrada: %d %v", cuentas, err)
	}

	var fugas int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM auditoria_cambios
WHERE CAST(valor_nuevo AS CHAR) LIKE '%0234-0567%' OR CAST(valor_nuevo AS CHAR) LIKE '%8888-1234%'
   OR CAST(valor_nuevo AS CHAR) LIKE '%015202001026284066%' OR CAST(valor_anterior AS CHAR) LIKE '%0234-0567%'`).Scan(&fugas); err != nil || fugas != 0 {
		t.Fatalf("auditoria_cambios con datos en claro: %d (%v)", fugas, err)
	}

	r, err := f.svc.DatosSensibles(ctx, SolicitudDatosSensibles{UsuarioID: f.usuario, Recurso: "prueba"}, f.empleado)
	if err != nil {
		t.Fatal(err)
	}
	if r.Revelado || r.NumeroIdentificacion != "*******0567" || r.NumeroCuenta != "******************4066" ||
		r.CorreoElectronico != "a***@ccdr.go.cr" || r.Direccion != "***" {
		t.Fatalf("enmascarado: %+v", r)
	}
	if n := f.contarAccesos(t); n != 0 {
		t.Fatalf("enmascarar no registra acceso de revelado (%d)", n)
	}

	for i := 1; i <= 2; i++ {
		r, err = f.svc.DatosSensibles(ctx, SolicitudDatosSensibles{UsuarioID: f.usuario, VerSensible: true, Recurso: "prueba", IP: "10.0.0.1", RequestID: "req-1"}, f.empleado)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Revelado || r.NumeroIdentificacion != cedulaPrueba || r.NumeroCuenta != ibanPrueba || r.Telefono != "8888-1234" {
			t.Fatalf("revelado: %+v", r)
		}
		if n := f.contarAccesos(t); n != i {
			t.Fatalf("accesos registrados %d, esperado %d", n, i)
		}
	}
	var campos string
	if err := f.db.QueryRow(`SELECT campos FROM registro_accesos_datos WHERE empleado_id = ? ORDER BY id LIMIT 1`, f.empleado).Scan(&campos); err != nil ||
		campos != "numero_identificacion,direccion,telefono,correo_electronico,numero_cuenta" {
		t.Fatalf("campos registrados %q (%v)", campos, err)
	}
	if _, err := f.db.Exec(`UPDATE registro_accesos_datos SET campos = 'x'`); err == nil {
		t.Fatal("la bitácora de accesos debe ser solo inserción (UPDATE)")
	}
	if _, err := f.db.Exec(`DELETE FROM registro_accesos_datos`); err == nil {
		t.Fatal("la bitácora de accesos debe ser solo inserción (DELETE)")
	}

	otro := NewEmpleadoService(f.db, llaveroPrueba(t, "d9:"+strings.Repeat("99", 32), ""))
	if _, err := otro.DatosSensibles(ctx, SolicitudDatosSensibles{UsuarioID: f.usuario, VerSensible: true, Recurso: "prueba"}, f.empleado); err == nil ||
		!strings.Contains(err.Error(), "kid de cifrado desconocido") {
		t.Fatalf("llave desconocida: %v", err)
	}
}

func TestRotacionDeLlaveDeDatos(t *testing.T) {
	f := nuevoFixtureL26(t)
	ctx := context.Background()
	if res, err := CifrarExistentes(ctx, f.db, f.llavero, f.usuario, 2, false); err != nil || res.Total != 0 {
		t.Fatalf("sin columnas en claro no hay nada que cifrar: %+v %v", res, err)
	}
	rotado := llaveroPrueba(t, llaveD1+",d2:"+strings.Repeat("aa", 32), "d2")
	res, err := CifrarExistentes(ctx, f.db, rotado, f.usuario, 2, true)
	if err != nil || res.Total != 5 {
		t.Fatalf("recifrar: %+v %v", res, err)
	}
	if res, err = CifrarExistentes(ctx, f.db, rotado, f.usuario, 2, true); err != nil || res.Total != 0 {
		t.Fatalf("rotación idempotente: %+v %v", res, err)
	}
	var viejos int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM empleados WHERE numero_identificacion_kid <> 'd2'`).Scan(&viejos); err != nil || viejos != 0 {
		t.Fatalf("quedan %d cédulas con la llave vieja (%v)", viejos, err)
	}
	r, err := NewEmpleadoService(f.db, rotado).DatosSensibles(ctx, SolicitudDatosSensibles{UsuarioID: f.usuario, VerSensible: true, Recurso: "prueba"}, f.empleado)
	if err != nil || r.NumeroCuenta != ibanPrueba || r.NumeroIdentificacion != cedulaPrueba {
		t.Fatalf("tras rotar: %+v %v", r, err)
	}
}

func TestActualizacionExigeCifrarAntesDeBorrarDatosEnClaro(t *testing.T) {
	db, m := testutil.NuevaBDConMigrador(t)
	ctx := context.Background()
	if err := m.Migrate(145); err != nil {
		t.Fatalf("bajar a 145: %v", err)
	}
	usuario := exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('upg', 'Actualización', 'upg@example.test', 'x')`)
	var tipoIdent int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	empleado := exec(t, db, `INSERT INTO empleados (codigo_interno, tipo_identificacion_id, numero_identificacion, nombre_completo, fecha_ingreso)
VALUES ('UPG1', ?, ?, 'Empleado legado', '2020-01-01')`, tipoIdent, cedulaPrueba)

	if err := m.Up(); err == nil || !strings.Contains(err.Error(), "datos_sin_cifrar") {
		t.Fatalf("la 146 debía negarse con datos en claro, se obtuvo %v", err)
	}
	if err := m.Force(145); err != nil {
		t.Fatal(err)
	}
	llavero := llaveroPrueba(t, llaveD1, "")
	res, err := CifrarExistentes(ctx, db, llavero, usuario, 10, false)
	if err != nil || res.PorCampo["empleados.numero_identificacion"] != 1 {
		t.Fatalf("cifrar existentes: %+v %v", res, err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("la 146 debía aplicar tras cifrar: %v", err)
	}
	r, err := NewEmpleadoService(db, llavero).DatosSensibles(ctx, SolicitudDatosSensibles{UsuarioID: usuario, VerSensible: true, Recurso: "prueba"}, empleado)
	if err != nil || r.NumeroIdentificacion != cedulaPrueba {
		t.Fatalf("dato legado tras la actualización: %+v %v", r, err)
	}
	var columnas int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE()
AND ((table_name = 'empleados' AND column_name = 'numero_identificacion')
  OR (table_name = 'empleado_datos_bancarios' AND column_name = 'numero_cuenta')
  OR (table_name = 'empleado_datos_personales' AND column_name IN ('direccion', 'telefono', 'correo_electronico')))`).Scan(&columnas); err != nil || columnas != 0 {
		t.Fatalf("quedan %d columnas en claro (%v)", columnas, err)
	}
}
