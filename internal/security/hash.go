// Package security implementa las primitivas de bajo nivel del módulo de
// seguridad: hashing de contraseñas (argon2id, con verificación de hashes
// bcrypt legados) y construcción/verificación de tokens PASETO v2 local.
//
// No conoce la base de datos ni Gin — es la capa que internal/auth orquesta.
package security

import (
	"errors"
	"strings"

	"github.com/alexedwards/argon2id"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrPasswordInvalida se devuelve cuando la contraseña no coincide con el hash.
	ErrPasswordInvalida = errors.New("security: contraseña inválida")
	// ErrHashDesconocido se devuelve si el hash no tiene formato argon2id ni bcrypt.
	ErrHashDesconocido = errors.New("security: formato de hash desconocido")
)

// paramsArgon2id son los parámetros de costo para todo hash NUEVO.
// 64 MiB / 3 iteraciones / 2 hilos es un perfil razonable para un backend
// concurrente (ajustar según benchmark de hardware real antes de producción).
var paramsArgon2id = &argon2id.Params{
	Memory:      64 * 1024, // KiB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// HashPassword genera un hash argon2id de la contraseña en texto plano.
// Todo password NUEVO (alta de usuario, cambio de contraseña, reset,
// re-hash de un login exitoso con hash bcrypt legado) debe usar este método.
func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, paramsArgon2id)
}

// VerifyPassword compara `password` contra `hash`, soportando:
//   - hashes argon2id nuevos (prefijo "$argon2id$")
//   - hashes bcrypt legados del esquema v11 (prefijo "$2a$"/"$2b$"/"$2y$")
//
// necesitaRehash=true cuando el hash verificado fue bcrypt: la capa de
// servicio debe entonces llamar a HashPassword tras un login exitoso y
// persistir el nuevo hash argon2id, migrando al usuario de forma perezosa
// sin forzar una migración masiva de toda `usuarios.password_hash`.
func VerifyPassword(password, hash string) (necesitaRehash bool, err error) {
	switch {
	case strings.HasPrefix(hash, "$argon2id$"):
		match, verifyErr := argon2id.ComparePasswordAndHash(password, hash)
		if verifyErr != nil {
			return false, verifyErr
		}
		if !match {
			return false, ErrPasswordInvalida
		}
		return false, nil

	case strings.HasPrefix(hash, "$2a$"), strings.HasPrefix(hash, "$2b$"), strings.HasPrefix(hash, "$2y$"):
		compareErr := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
		if compareErr != nil {
			return false, ErrPasswordInvalida
		}
		return true, nil

	default:
		return false, ErrHashDesconocido
	}
}
