package auth

// Política de contraseñas (F14 / S11).
//
// ValidarPassword se usa en TODO punto donde se fija una contraseña nueva:
// cambio autenticado (CambiarPassword) y confirmación de reset
// (ConfirmarResetPassword). El login NO la aplica: debe poder verificar
// contraseñas anteriores a esta política.
//
// El nombre `Politica` ya lo usa service.go (parámetros de sesión/bloqueo),
// por eso esto vive bajo ErrPoliticaPassword / ValidarPassword.

import (
	_ "embed"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	PasswordLongitudMinima = 12
	PasswordLongitudMaxima = 128

	// Un nombre de usuario o parte local de correo más corto que esto no se
	// busca dentro de la contraseña: "a" o "jo" vetarían casi cualquier clave.
	longitudMinimaCoincidencia = 3
)

// MotivoPassword identifica qué regla incumplió la contraseña. Es estable y
// apto para que el frontend lo use como código.
type MotivoPassword string

const (
	MotivoMuyCorta        MotivoPassword = "muy_corta"
	MotivoMuyLarga        MotivoPassword = "muy_larga"
	MotivoContieneUsuario MotivoPassword = "contiene_usuario"
	MotivoContieneCorreo  MotivoPassword = "contiene_correo"
	MotivoComun           MotivoPassword = "comun"
	MotivoIgualActual     MotivoPassword = "igual_actual"
)

// ErrPoliticaPassword es el error tipado de la política. Mensaje es apto para
// mostrarse tal cual al usuario (el handler lo devuelve en el 400).
// Detectarlo: var pe *ErrPoliticaPassword; errors.As(err, &pe).
type ErrPoliticaPassword struct {
	Motivo  MotivoPassword
	Mensaje string
}

func (e *ErrPoliticaPassword) Error() string {
	return "auth: la contraseña no cumple la política (" + string(e.Motivo) + "): " + e.Mensaje
}

func errPolitica(m MotivoPassword, mensaje string) error {
	return &ErrPoliticaPassword{Motivo: m, Mensaje: mensaje}
}

//go:embed passwords_comunes.txt
var passwordsComunesRaw string

var (
	comunesOnce sync.Once
	comunes     map[string]struct{}
)

func cargarComunes() map[string]struct{} {
	comunesOnce.Do(func() {
		comunes = make(map[string]struct{}, 512)
		for _, linea := range strings.Split(passwordsComunesRaw, "\n") {
			linea = strings.ToLower(strings.TrimSpace(linea))
			if linea == "" || strings.HasPrefix(linea, "#") {
				continue
			}
			comunes[linea] = struct{}{}
		}
	})
	return comunes
}

// ValidarPassword aplica, en este orden: longitud mínima (12), longitud
// máxima (128), no contener el nombre de usuario ni la parte local del
// correo, y no estar en la lista de contraseñas comunes. Devuelve nil o un
// *ErrPoliticaPassword. La longitud se mide en caracteres (runas), no en bytes.
func ValidarPassword(password, nombreUsuario, correo string) error {
	n := utf8.RuneCountInString(password)
	if n < PasswordLongitudMinima {
		return errPolitica(MotivoMuyCorta, "La contraseña debe tener al menos 12 caracteres.")
	}
	if n > PasswordLongitudMaxima {
		return errPolitica(MotivoMuyLarga, "La contraseña no puede superar los 128 caracteres.")
	}

	minusculas := strings.ToLower(password)

	if usuario := strings.ToLower(strings.TrimSpace(nombreUsuario)); utf8.RuneCountInString(usuario) >= longitudMinimaCoincidencia &&
		strings.Contains(minusculas, usuario) {
		return errPolitica(MotivoContieneUsuario, "La contraseña no puede contener su nombre de usuario.")
	}

	if local := parteLocalCorreo(correo); utf8.RuneCountInString(local) >= longitudMinimaCoincidencia &&
		strings.Contains(minusculas, local) {
		return errPolitica(MotivoContieneCorreo, "La contraseña no puede contener su correo electrónico.")
	}

	if esPasswordComun(minusculas) {
		return errPolitica(MotivoComun, "Esa contraseña es demasiado común o fácil de adivinar. Use una frase más larga o menos predecible.")
	}

	return nil
}

// parteLocalCorreo devuelve lo que va antes de la '@', en minúsculas.
func parteLocalCorreo(correo string) string {
	correo = strings.ToLower(strings.TrimSpace(correo))
	if i := strings.IndexByte(correo, '@'); i >= 0 {
		return correo[:i]
	}
	return correo
}

// sufijoDescartable son los caracteres que la gente agrega al final de una
// palabra común para "reforzarla" (Password2026!!).
const sufijoDescartable = "0123456789!@#$%^&*()_-+=.?¡¿ "

// esPasswordComun recibe la contraseña YA en minúsculas.
func esPasswordComun(minusculas string) bool {
	lista := cargarComunes()

	if _, ok := lista[minusculas]; ok {
		return true
	}
	if base := strings.TrimRight(minusculas, sufijoDescartable); base != "" && base != minusculas {
		if _, ok := lista[base]; ok {
			return true
		}
	}
	return todosIguales(minusculas)
}

// todosIguales detecta "aaaaaaaaaaaa", "000000000000", etc.
func todosIguales(s string) bool {
	primero, tam := utf8.DecodeRuneInString(s)
	if tam == 0 {
		return false
	}
	for _, r := range s[tam:] {
		if r != primero {
			return false
		}
	}
	return true
}
