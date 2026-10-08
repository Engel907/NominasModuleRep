// Package middleware contiene middleware transversal de Gin: manejo de
// errores, autenticación (auth.go) y autorización por permiso
// (require_permission.go).
package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse es el contrato único de error de toda la API. Cualquier
// endpoint que falle, sin importar la capa (auth, autorización, validación,
// dominio, base de datos), responde con este mismo shape.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	// Code es un identificador estable en snake_case para que el cliente
	// (frontend, otro servicio) pueda tomar decisiones por código sin
	// parsear el mensaje humano. p.ej. "unauthorized", "invalid_token",
	// "permission_denied", "validation_error", "db_unavailable".
	Code string `json:"code"`
	// Message es un mensaje legible para humanos, en español, seguro de
	// mostrar tal cual (nunca debe filtrar detalles internos como queries
	// SQL, stack traces, o rutas de archivo).
	Message string `json:"message"`
	// Details es opcional: contexto adicional estructurado (p.ej. lista de
	// campos inválidos en un validation_error). nil si no aplica.
	Details interface{} `json:"details,omitempty"`
}

// apiError es el error interno que se guarda en el contexto de Gin vía
// c.Error(...) para que ErrorHandler lo traduzca a ErrorResponse. No se
// expone fuera de este paquete: los handlers usan Fail(...) / Abort(...).
type apiError struct {
	status  int
	code    string
	message string
	details interface{}
	cause   error
}

func (e *apiError) Error() string {
	if e.cause != nil {
		return e.message + ": " + e.cause.Error()
	}
	return e.message
}

func (e *apiError) Unwrap() error {
	return e.cause
}

// Fail registra el error en el contexto de Gin y corta la cadena de
// middleware/handlers (equivalente a c.AbortWithStatusJSON, pero pasando
// por el contrato único vía ErrorHandler). Es la forma esperada de
// reportar errores desde cualquier handler o middleware de este proyecto.
//
//	middleware.Fail(c, http.StatusForbidden, "permission_denied",
//	    "No tenés permiso para esta acción", gin.H{"permiso_requerido": "planilla.aprobar"})
func Fail(c *gin.Context, status int, code, message string, details interface{}) {
	_ = c.Error(&apiError{status: status, code: code, message: message, details: details})
	c.Abort()
}

// FailFromError es igual a Fail pero además envuelve un error de causa
// (p.ej. el error real de la base de datos) para que quede en los logs del
// servidor sin exponerlo al cliente.
func FailFromError(c *gin.Context, status int, code, message string, cause error) {
	_ = c.Error(&apiError{status: status, code: code, message: message, cause: cause})
	c.Abort()
}

// ErrorHandler es middleware global (montado una sola vez en main.go) que
// centraliza la traducción de cualquier error acumulado en el contexto de
// Gin al contrato único ErrorResponse. Debe montarse antes que las rutas,
// después de gin.Recovery().
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		// Se usa el último error reportado: en la práctica un handler
		// aborta en el primer Fail/FailFromError, así que suele haber uno
		// solo, pero por robustez tomamos el más reciente.
		last := c.Errors.Last()

		if apiErr, ok := last.Err.(*apiError); ok {
			if apiErr.cause != nil {
				log.Printf("[%s] %s: %v", apiErr.code, apiErr.message, apiErr.cause)
			}
			c.JSON(apiErr.status, ErrorResponse{Error: ErrorBody{
				Code:    apiErr.code,
				Message: apiErr.message,
				Details: apiErr.details,
			}})
			return
		}

		// Error no reconocido (no vino de Fail/FailFromError): no confiamos
		// en su mensaje para el cliente, pero sí lo logueamos completo.
		log.Printf("error no manejado: %v", last.Err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: ErrorBody{
			Code:    "internal_error",
			Message: "Ocurrió un error inesperado. Intentá de nuevo más tarde.",
		}})
	}
}
