package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestValidarPassword(t *testing.T) {
	const usuario = "mgarcia"
	const correo = "maria.lopez@ccdr.go.cr"

	casos := []struct {
		nombre  string
		pass    string
		usuario string
		correo  string
		motivo  MotivoPassword // "" = debe ser válida
	}{
		{"válida: frase larga", "caballo-azul-nube-47", usuario, correo, ""},
		{"válida: 12 caracteres justos", "Tr0ub4dor&3x", usuario, correo, ""},
		{"válida: 128 caracteres justos", strings.Repeat("ab", 64), usuario, correo, ""},
		{"válida: 12 runas con acentos", "ñandúñandúñ1", usuario, correo, ""},

		{"corta: 11 caracteres", "Tr0ub4dor&3", usuario, correo, MotivoMuyCorta},
		{"corta: vacía", "", usuario, correo, MotivoMuyCorta},
		{"larga: 129 caracteres", strings.Repeat("ab", 64) + "c", usuario, correo, MotivoMuyLarga},

		{"contiene usuario (otra capitalización)", "xx-MGarcia-xx-99", usuario, correo, MotivoContieneUsuario},
		{"contiene parte local del correo", "clave-maria.lopez-77", usuario, correo, MotivoContieneCorreo},
		{"usuario corto no se busca", "jo-caballo-azul-77", "jo", "jo@ccdr.go.cr", ""},

		{"común: exacta en la lista", "123456789012", usuario, correo, MotivoComun},
		{"común: con mayúsculas y ñ", "CONTRASEÑA1234", usuario, correo, MotivoComun},
		{"común: palabra + año + símbolos", "Password2026!!", usuario, correo, MotivoComun},
		{"común: palabra en español + dígitos", "Bienvenido2026", usuario, correo, MotivoComun},
		{"común: un solo carácter repetido", "aaaaaaaaaaaa", usuario, correo, MotivoComun},
		{"común: teclado", "qwertyuiop123", usuario, correo, MotivoComun},
	}

	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			err := ValidarPassword(tc.pass, tc.usuario, tc.correo)

			if tc.motivo == "" {
				if err != nil {
					t.Fatalf("se esperaba contraseña válida, error: %v", err)
				}
				return
			}

			var pe *ErrPoliticaPassword
			if !errors.As(err, &pe) {
				t.Fatalf("se esperaba *ErrPoliticaPassword con motivo %q, se obtuvo %v", tc.motivo, err)
			}
			if pe.Motivo != tc.motivo {
				t.Fatalf("motivo = %q, se esperaba %q", pe.Motivo, tc.motivo)
			}
			if pe.Mensaje == "" {
				t.Fatal("el mensaje al usuario no puede estar vacío")
			}
		})
	}
}

func TestListaComunesTieneAlMenos300Entradas(t *testing.T) {
	if n := len(cargarComunes()); n < 300 {
		t.Fatalf("la lista embebida tiene %d entradas, se exigen al menos 300", n)
	}
}

func TestListaComunesEnMinusculas(t *testing.T) {
	for entrada := range cargarComunes() {
		if entrada != strings.ToLower(entrada) {
			t.Errorf("entrada no normalizada: %q", entrada)
		}
	}
}
