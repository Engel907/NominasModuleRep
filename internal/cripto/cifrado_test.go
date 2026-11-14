package cripto

import (
	"errors"
	"strings"
	"testing"
)

var (
	k1 = strings.Repeat("11", 32)
	k2 = strings.Repeat("22", 32)
	hk = strings.Repeat("33", 32)
)

func llavero(t *testing.T, llaves, activo string) *Llavero {
	t.Helper()
	l, err := NuevoLlavero(llaves, activo, hk)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestIdaYVueltaYRotacion(t *testing.T) {
	viejo := llavero(t, "k1:"+k1, "")
	ct, kid, err := viejo.Cifrar("empleados.numero_identificacion", "1-0234-0567")
	if err != nil || kid != "k1" {
		t.Fatalf("cifrar: %v kid=%s", err, kid)
	}
	ct2, _, _ := viejo.Cifrar("empleados.numero_identificacion", "1-0234-0567")
	if string(ct) == string(ct2) {
		t.Fatal("el nonce debe ser aleatorio por valor")
	}
	rotado := llavero(t, "k1:"+k1+",k2:"+k2, "k2")
	v, err := rotado.Descifrar("empleados.numero_identificacion", ct, kid)
	if err != nil || v != "1-0234-0567" {
		t.Fatalf("descifrar tras rotación: %q %v", v, err)
	}
	if _, k, _ := rotado.Cifrar("x.y", "z"); k != "k2" {
		t.Fatalf("debe cifrar con la activa, usó %s", k)
	}
}

func TestLlaveDesconocidaYAlteracion(t *testing.T) {
	l := llavero(t, "k1:"+k1, "k1")
	ct, _, _ := l.Cifrar("empleado_datos_bancarios.numero_cuenta", "CR05015202001026284066")
	if _, err := l.Descifrar("empleado_datos_bancarios.numero_cuenta", ct, "k9"); !errors.Is(err, ErrLlaveDesconocida) {
		t.Fatalf("kid desconocido: %v", err)
	}
	if _, err := l.Descifrar("empleados.numero_identificacion", ct, "k1"); !errors.Is(err, ErrCifradoInvalido) {
		t.Fatalf("cifrado movido a otra columna: %v", err)
	}
	ct[len(ct)-1] ^= 1
	if _, err := l.Descifrar("empleado_datos_bancarios.numero_cuenta", ct, "k1"); !errors.Is(err, ErrCifradoInvalido) {
		t.Fatalf("cifrado alterado: %v", err)
	}
}

func TestLlaveroInvalido(t *testing.T) {
	casos := []struct{ llaves, activo, hmac string }{
		{"", "", hk},
		{"k1:abc", "", hk},
		{"k1:" + k1 + ",k1:" + k2, "", hk},
		{"k1:" + k1 + ",k2:" + k2, "", hk},
		{"k1:" + k1, "k3", hk},
		{"k1:" + k1, "", "corta"},
	}
	for _, c := range casos {
		if _, err := NuevoLlavero(c.llaves, c.activo, c.hmac); !errors.Is(err, ErrLlaveroInvalido) {
			t.Errorf("%+v: esperaba ErrLlaveroInvalido, fue %v", c, err)
		}
	}
}

func TestHashBusquedaNormaliza(t *testing.T) {
	l := llavero(t, "k1:"+k1, "")
	a := l.HashBusqueda("empleados.numero_identificacion", "1-0234-0567")
	b := l.HashBusqueda("empleados.numero_identificacion", " 102340567 ")
	c := l.HashBusqueda("empleado_datos_personales.telefono", "102340567")
	if a != b || len(a) != 64 {
		t.Fatalf("misma cédula, distinto hash: %s %s", a, b)
	}
	if a == c {
		t.Fatal("el hash debe depender del campo")
	}
}

func TestEnmascarar(t *testing.T) {
	casos := []struct {
		tipo      TipoDato
		v, quiere string
	}{
		{DatoIdentificacion, "102340567", "*****0567"},
		{DatoCuenta, "CR05015202001026284066", "******************4066"},
		{DatoTelefono, "8888-1234", "*****1234"},
		{DatoCorreo, "ana.perez@ccdr.go.cr", "a***@ccdr.go.cr"},
		{DatoDireccion, "San José, 100 m norte", "***"},
		{DatoTelefono, "123", "***"},
		{DatoCorreo, "", ""},
	}
	for _, c := range casos {
		if got := Enmascarar(c.tipo, c.v); got != c.quiere {
			t.Errorf("Enmascarar(%d, %q) = %q, esperado %q", c.tipo, c.v, got, c.quiere)
		}
	}
}
