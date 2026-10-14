package main

import (
	"strings"
	"testing"

	"planillas-ccdr/internal/seedadmin"
)

const alfabetoPassword = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!#%*-_=+@"

func TestGenerarPassword_LongitudComposicionYUnicidad(t *testing.T) {
	vistas := map[string]bool{}
	for i := 0; i < 200; i++ {
		p, err := seedadmin.GenerarPassword(seedadmin.LongitudPassword)
		if err != nil {
			t.Fatal(err)
		}
		if len(p) < 20 {
			t.Fatalf("contraseña demasiado corta (%d)", len(p))
		}
		if !strings.ContainsAny(p, "abcdefghijkmnopqrstuvwxyz") ||
			!strings.ContainsAny(p, "ABCDEFGHJKLMNPQRSTUVWXYZ") ||
			!strings.ContainsAny(p, "23456789") ||
			!strings.ContainsAny(p, "!#%*-_=+@") {
			t.Fatalf("falta una clase de caracteres: %q", p)
		}
		if strings.Trim(p, alfabetoPassword) != "" {
			t.Fatalf("caracteres fuera del alfabeto: %q", p)
		}
		if vistas[p] {
			t.Fatalf("contraseña repetida: %q", p)
		}
		vistas[p] = true
	}
}

func TestGenerarPassword_RechazaLongitudCorta(t *testing.T) {
	if _, err := seedadmin.GenerarPassword(19); err == nil {
		t.Fatal("debía rechazar menos de 20 caracteres")
	}
}

func TestValidarEntrada(t *testing.T) {
	casos := []struct {
		nombre, usuario, completo, correo string
		ok                                bool
	}{
		{"válido", "jperez", "Juan Pérez", "jperez@ejemplo.cr", true},
		{"usuario corto", "ab", "Juan", "a@b.cr", false},
		{"usuario con espacio", "j perez", "Juan", "a@b.cr", false},
		{"nombre vacío", "jperez", "  ", "a@b.cr", false},
		{"correo inválido", "jperez", "Juan", "no-es-correo", false},
		{"correo con nombre", "jperez", "Juan", "Juan <a@b.cr>", false},
	}
	for _, c := range casos {
		err := validarEntrada(c.usuario, c.completo, c.correo)
		if (err == nil) != c.ok {
			t.Errorf("%s: err=%v, se esperaba ok=%v", c.nombre, err, c.ok)
		}
	}
}