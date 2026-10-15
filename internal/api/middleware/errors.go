// Package middleware contiene middleware transversal de Gin: manejo de
// errores, autenticación (auth.go), autorización por permiso
// (require_permission.go) y endurecimiento HTTP (F16): request_id y logging
// (logging.go), cabeceras (headers.go), CORS (cors.go), límite de cuerpo
// (bodylimit.go) y rate limiting (ratelimit.go).
package middleware

import (
	"log/slog"
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
	// "permission_denied", "datos_invalidos", "db_unavailable".
	Code string `json:"code"`
	// Message es un mensaje legible para humanos, en español, seguro de
	// mostrar tal cual (nunca debe filtrar detalles internos como queries
	// SQL, stack traces, o rutas de archivo).
	Message string `json:"message"`
	// Details es opcional: contexto adicional estructurado (p.ej. la lista
	// "campos" de un datos_invalidos). nil si no aplica.
	Details interface{} `json:"details,omitempty"`
	// RequestID (F16/S15) correlaciona la respuesta con la línea de log del
	// servidor (el mismo valor va en la cabecera X-Request-ID).
	RequestID string `json:"request_id,omitempty"`
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
// IMPORTANTE: solo registra el error; la respuesta la escribe ErrorHandler,
// por lo que todo middleware que use Fail debe montarse DESPUÉS de
// ErrorHandler en la cadena global.
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

// ErrorHandler es middleware global (montado una sola vez en app.newRouter)
// que centraliza la traducción de cualquier error acumulado en el contexto
// de Gin al contrato único ErrorResponse. Debe montarse después de
// RequestID/RequestLogger/Recovery y ANTES de CORS, límites y rutas (esos
// usan Fail).
//
// Acepta tanto *apiError (Fail/FailFromError/RespondWithError) como errores
// crudos registrados con c.Error(err): estos pasan por traducirError, así
// que *ErrParametroInvalido -> 400, errores de dominio -> su código, y lo
// desconocido -> 500 genérico.
//
// Si el handler ya escribió el cuerpo (doble respuesta), NO escribe otra:
// lo deja en el log como advertencia.
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
		apiErr := traducirError(last.Err)

		ctx := c.Request.Context()
		if apiErr.cause != nil {
			// El detalle interno SOLO va al log, nunca al cliente.
			attrs := []any{
				slog.String("code", apiErr.code),
				slog.Int("status", apiErr.status),
				slog.String("error", apiErr.cause.Error()),
			}
			if uid := UsuarioID(c); uid > 0 {
				attrs = append(attrs, slog.Int64("usuario_id", uid))
			}
			slog.ErrorContext(ctx, "error_api", attrs...)
		}

		if c.Writer.Written() {
			slog.WarnContext(ctx, "respuesta_ya_escrita",
				slog.String("code", apiErr.code),
				slog.Int("status_enviado", c.Writer.Status()),
			)
			return
		}

		c.JSON(apiErr.status, ErrorResponse{Error: ErrorBody{
			Code:      apiErr.code,
			Message:   apiErr.message,
			Details:   apiErr.details,
			RequestID: RequestIDDesdeGin(c),
		}})
	}
}

// respuestaErrorInterno es el 500 genérico que escribe Recovery (que está
// fuera de ErrorHandler y por eso no puede usar Fail).
func respuestaErrorInterno(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{Error: ErrorBody{
		Code:      "internal_error",
		Message:   mensajeErrorInterno,
		RequestID: RequestIDDesdeGin(c),
	}})
}