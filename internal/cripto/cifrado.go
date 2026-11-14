// Package cripto: cifrado de datos personales en reposo (Ley 8968 de
// Protección de la Persona frente al Tratamiento de sus Datos Personales).
package cripto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrLlaveDesconocida  = errors.New("seguridad: kid de cifrado desconocido")
	ErrCifradoInvalido   = errors.New("seguridad: dato cifrado inválido o alterado")
	ErrLlaveroInvalido   = errors.New("seguridad: llavero de cifrado inválido")
	ErrLlaveroNoProvisto = errors.New("seguridad: el cifrado de datos personales no está configurado")
)

var reKid = regexp.MustCompile(`^[A-Za-z0-9_-]{1,20}$`)

// Llavero: llaves de cifrado por kid + llave HMAC de búsqueda.
type Llavero struct {
	activa  string
	aeads   map[string]cipher.AEAD
	hmacKey []byte
}

// NuevoLlavero valida y construye el llavero. llavesHex = "kid:hex,…".
func NuevoLlavero(llavesHex, kidActivo, hmacHex string) (*Llavero, error) {
	if strings.TrimSpace(llavesHex) == "" {
		return nil, fmt.Errorf("%w: DATA_KEYS_HEX vacío", ErrLlaveroInvalido)
	}
	l := &Llavero{aeads: map[string]cipher.AEAD{}}
	for _, par := range strings.Split(llavesHex, ",") {
		kid, h, ok := strings.Cut(strings.TrimSpace(par), ":")
		if !ok || !reKid.MatchString(kid) {
			return nil, fmt.Errorf("%w: DATA_KEYS_HEX debe tener formato kid:hex,kid:hex (kid: letras, dígitos, _ o -, máx. 20)", ErrLlaveroInvalido)
		}
		if _, dup := l.aeads[kid]; dup {
			return nil, fmt.Errorf("%w: kid %q repetido", ErrLlaveroInvalido, kid)
		}
		k, err := llave32(h)
		if err != nil {
			return nil, fmt.Errorf("%w: llave %q: %v", ErrLlaveroInvalido, kid, err)
		}
		bloque, err := aes.NewCipher(k)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrLlaveroInvalido, err)
		}
		aead, err := cipher.NewGCM(bloque)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrLlaveroInvalido, err)
		}
		l.aeads[kid] = aead
	}
	l.activa = strings.TrimSpace(kidActivo)
	if l.activa == "" && len(l.aeads) == 1 {
		for kid := range l.aeads {
			l.activa = kid
		}
	}
	if _, ok := l.aeads[l.activa]; !ok {
		return nil, fmt.Errorf("%w: DATA_ACTIVE_KID=%q no está en DATA_KEYS_HEX (kids: %v)", ErrLlaveroInvalido, l.activa, l.Kids())
	}
	hk, err := llave32(hmacHex)
	if err != nil {
		return nil, fmt.Errorf("%w: DATA_HMAC_KEY_HEX: %v", ErrLlaveroInvalido, err)
	}
	l.hmacKey = hk
	return l, nil
}

func llave32(h string) ([]byte, error) {
	h = strings.TrimSpace(h)
	if len(h) != 64 {
		return nil, fmt.Errorf("debe tener 64 caracteres hex (32 bytes), tiene %d", len(h))
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, errors.New("no es hex válido")
	}
	return b, nil
}

// Kids: kids del llavero (ordenados).
func (l *Llavero) Kids() []string {
	out := make([]string, 0, len(l.aeads))
	for k := range l.aeads {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// KidActivo: kid con el que se cifra.
func (l *Llavero) KidActivo() string { return l.activa }

// Cifrar devuelve nonce‖ciphertext y el kid usado. campo = "tabla.columna".
func (l *Llavero) Cifrar(campo, valor string) ([]byte, string, error) {
	if l == nil {
		return nil, "", ErrLlaveroNoProvisto
	}
	aead := l.aeads[l.activa]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, "", fmt.Errorf("seguridad: generando nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, []byte(valor), []byte(campo)), l.activa, nil
}

// Descifrar: kid desconocido → ErrLlaveDesconocida; alterado o de otro campo
// → ErrCifradoInvalido.
func (l *Llavero) Descifrar(campo string, ct []byte, kid string) (string, error) {
	if l == nil {
		return "", ErrLlaveroNoProvisto
	}
	aead, ok := l.aeads[kid]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrLlaveDesconocida, kid)
	}
	if len(ct) < aead.NonceSize()+aead.Overhead() {
		return "", ErrCifradoInvalido
	}
	n := aead.NonceSize()
	plano, err := aead.Open(nil, ct[:n], ct[n:], []byte(campo))
	if err != nil {
		return "", ErrCifradoInvalido
	}
	return string(plano), nil
}

// HashBusqueda: HMAC-SHA256 hex (64) de campo + valor normalizado.
func (l *Llavero) HashBusqueda(campo, valor string) string {
	m := hmac.New(sha256.New, l.hmacKey)
	m.Write([]byte(campo))
	m.Write([]byte{0})
	m.Write([]byte(Normalizar(valor)))
	return hex.EncodeToString(m.Sum(nil))
}

// Normalizar: sin espacios ni guiones, en mayúsculas (cédula, IBAN, teléfono,
// correo). "1-0234-0567" y "102340567" dan el mismo hash.
func Normalizar(v string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' {
			return -1
		}
		return unicode.ToUpper(r)
	}, strings.TrimSpace(v))
}

// Enmascarar deja visibles solo los últimos 4 caracteres (correo: primera
// letra y dominio; dirección: nada). Cadena vacía queda vacía.
func Enmascarar(tipo TipoDato, v string) string {
	if v == "" {
		return ""
	}
	switch tipo {
	case DatoDireccion:
		return "***"
	case DatoCorreo:
		local, dominio, ok := strings.Cut(v, "@")
		if !ok || local == "" {
			return "***"
		}
		r, _ := utf8.DecodeRuneInString(local)
		return string(r) + "***@" + dominio
	}
	runas := []rune(v)
	if len(runas) <= 4 {
		return strings.Repeat("*", len(runas))
	}
	return strings.Repeat("*", len(runas)-4) + string(runas[len(runas)-4:])
}

// TipoDato: cómo se enmascara un dato.
type TipoDato int

const (
	DatoIdentificacion TipoDato = iota
	DatoCuenta
	DatoTelefono
	DatoCorreo
	DatoDireccion
)
