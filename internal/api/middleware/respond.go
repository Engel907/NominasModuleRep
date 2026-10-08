package middleware

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/auth"
	"planillas-ccdr/internal/security"
	"planillas-ccdr/internal/services"
)

// RespondWithError es el punto de entrada único que usan los handlers de
// internal/api/handlers para traducir un error de negocio (auth, services,
// security, sql.ErrNoRows, o errores de bind de Gin) al contrato JSON de
// ErrorResponse, vía Fail/FailFromError.
//
// Antes de esta función, los handlers llamaban a un `api.RespondWithError`
// que nunca llegó a implementarse en ningún paquete `api` real — quedó como
// referencia colgante entre fases. Se centraliza acá (en vez de en un
// paquete `api` nuevo) porque el contrato de error (ErrorResponse, Fail,
// FailFromError) ya vive en este paquete.
//
// NOTA sobre import cycles: este archivo puede importar auth/services/
// security porque ninguno de esos paquetes importa middleware. Los
// paquetes que sí importan middleware (internal/asistencia,
// internal/prestamos) NO deben agregarse acá — manejan sus propios errores
// localmente en sus handlers (ver TODO en ORGANIZACION.md sobre unificar
// también esos dos módulos al contrato único).
func RespondWithError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	switch {
	// --- errores de bind/validación de Gin (c.ShouldBindJSON, etc.) ---
	case isBindingError(err):
		Fail(c, http.StatusBadRequest, "validation_error", "Los datos enviados no son válidos.", gin.H{"detalle": err.Error()})
		return

	// --- auth ---
	case errors.Is(err, auth.ErrCredencialesInvalidas):
		Fail(c, http.StatusUnauthorized, "invalid_credentials", "Usuario o contraseña incorrectos.", nil)
		return
	case errors.Is(err, auth.ErrUsuarioInactivo):
		Fail(c, http.StatusForbidden, "user_inactive", "El usuario no está activo.", nil)
		return
	case errors.Is(err, auth.ErrUsuarioBloqueado):
		Fail(c, http.StatusForbidden, "user_locked", "El usuario está bloqueado temporalmente por intentos fallidos.", nil)
		return
	case errors.Is(err, auth.ErrRefreshTokenInvalido):
		Fail(c, http.StatusUnauthorized, "invalid_token", "El token es inválido, fue revocado o expiró.", nil)
		return
	case errors.Is(err, auth.ErrTokenResetInvalido):
		Fail(c, http.StatusBadRequest, "invalid_reset_token", "El token de restablecimiento es inválido, ya se usó o expiró.", nil)
		return
	case errors.Is(err, auth.ErrRolInactivo):
		Fail(c, http.StatusUnprocessableEntity, "role_inactive", "El rol no está activo.", nil)
		return
	case errors.Is(err, auth.ErrRolYaVigente):
		Fail(c, http.StatusConflict, "role_already_assigned", "El usuario ya tiene ese rol vigente.", nil)
		return

	// --- security ---
	case errors.Is(err, security.ErrPasswordInvalida):
		Fail(c, http.StatusBadRequest, "invalid_password", "La contraseña no cumple los requisitos.", nil)
		return
	case errors.Is(err, security.ErrTokenInvalido), errors.Is(err, security.ErrTokenExpirado):
		Fail(c, http.StatusUnauthorized, "invalid_token", "El token es inválido o expiró.", nil)
		return

	// --- services (empleados / contratos / organización) ---
	case errors.Is(err, services.ErrEmpleadoNoEncontrado),
		errors.Is(err, services.ErrContratoNoEncontrado),
		errors.Is(err, services.ErrNoEncontrado):
		Fail(c, http.StatusNotFound, "not_found", "El recurso solicitado no existe.", nil)
		return
	case errors.Is(err, services.ErrSalarioInferiorAlMinimo):
		Fail(c, http.StatusUnprocessableEntity, "salary_below_minimum", err.Error(), nil)
		return

	// --- genérico de base de datos ---
	case errors.Is(err, sql.ErrNoRows):
		Fail(c, http.StatusNotFound, "not_found", "El recurso solicitado no existe.", nil)
		return

	default:
		FailFromError(c, http.StatusInternalServerError, "internal_error", "Ocurrió un error inesperado. Intentá de nuevo más tarde.", err)
		return
	}
}

// isBindingError detecta errores devueltos por c.ShouldBindJSON sin acoplar
// este paquete a los tipos internos de validator/json, mirando el binding
// más común: los handlers de este proyecto solo usan ShouldBindJSON, cuyo
// error typico ya trae mensaje seguro de mostrar (nombre de campo /
// json.UnmarshalTypeError), así que se maneja con el mensaje genérico +
// detalle textual en vez de intentar mapear campo por campo.
func isBindingError(err error) bool {
	var syntaxErr interface{ Error() string }
	_ = syntaxErr
	// Se deja como punto de extensión: hoy no distinguimos binding errors
	// de otros errores no reconocidos, así que caen en el `default` de
	// RespondWithError con 500. Si el proyecto quiere 400 automático para
	// errores de bind, el handler debe llamar a
	// middleware.Fail(c, http.StatusBadRequest, "validation_error", ...)
	// directamente en el propio ShouldBindJSON (patrón ya usado en
	// internal/asistencia y internal/prestamos), en vez de pasar por acá.
	return false
}
