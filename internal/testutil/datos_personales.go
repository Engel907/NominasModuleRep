package testutil

import (
	"database/sql"
	"strings"
	"testing"

	"planillas-ccdr/internal/cripto"
	"planillas-ccdr/internal/db/dto"
)

// Llaves fijas solo para pruebas (los datos personales se guardan cifrados).
var (
	llaveDatosPrueba = "tp:" + strings.Repeat("5a", 32)
	hmacDatosPrueba  = strings.Repeat("6b", 32)
)

// Llavero devuelve el llavero de pruebas (kid "tp").
func Llavero(t testing.TB) *cripto.Llavero {
	t.Helper()
	l, err := cripto.NuevoLlavero(llaveDatosPrueba, "tp", hmacDatosPrueba)
	if err != nil {
		t.Fatalf("testutil: llavero de pruebas: %v", err)
	}
	return l
}

// Cifrar devuelve el cifrado, el hash de búsqueda y la llave de un valor.
func Cifrar(t testing.TB, c dto.CampoSensible, valor string) ([]byte, string, string) {
	t.Helper()
	l := Llavero(t)
	ct, kid, err := l.Cifrar(c.Campo(), valor)
	if err != nil {
		t.Fatalf("testutil: cifrar %s: %v", c.Campo(), err)
	}
	return ct, l.HashBusqueda(c.Campo(), valor), kid
}

// InsertarEmpleado inserta un empleado con su identificación cifrada y
// devuelve su id.
func InsertarEmpleado(t testing.TB, db *sql.DB, codigo string, tipoIdentificacionID int64, identificacion, nombre, fechaIngreso string) int64 {
	t.Helper()
	ct, hash, kid := Cifrar(t, dto.CampoIdentificacion, identificacion)
	res, err := db.Exec(`INSERT INTO empleados (codigo_interno, tipo_identificacion_id, numero_identificacion_cifrado,
    numero_identificacion_hash, numero_identificacion_kid, nombre_completo, fecha_ingreso)
VALUES (?, ?, ?, ?, ?, ?, ?)`, codigo, tipoIdentificacionID, ct, hash, kid, nombre, fechaIngreso)
	if err != nil {
		t.Fatalf("testutil: insertar empleado %s: %v", codigo, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// InsertarCuentaBancaria inserta la cuenta vigente (cifrada) de un empleado.
func InsertarCuentaBancaria(t testing.TB, db *sql.DB, empleadoID, bancoID int64, tipoCuenta, cuenta, fechaInicio string) {
	t.Helper()
	ct, hash, kid := Cifrar(t, dto.CampoCuenta, cuenta)
	if _, err := db.Exec(`INSERT INTO empleado_datos_bancarios (empleado_id, banco_id, tipo_cuenta, numero_cuenta_cifrado,
    numero_cuenta_hash, numero_cuenta_kid, fecha_inicio_vigencia)
VALUES (?, ?, ?, ?, ?, ?, ?)`, empleadoID, bancoID, tipoCuenta, ct, hash, kid, fechaInicio); err != nil {
		t.Fatalf("testutil: insertar cuenta del empleado %d: %v", empleadoID, err)
	}
}
