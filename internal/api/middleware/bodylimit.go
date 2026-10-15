package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// LimitarCuerpo acota el tamaño del cuerpo de la petición a max bytes
// (MAX_BODY_BYTES):
//
//  1. Si Content-Length ya declara más que max, responde 413 sin leer nada.
//  2. Si no (chunked o declarado de menos), envuelve el cuerpo con
//     http.MaxBytesReader: la lectura falla con *http.MaxBytesError al pasar
//     el límite, y RespondWithError/ErrorHandler lo traducen a 413
//     "cuerpo_demasiado_grande" (ver traducirError).
//
// Se aplica global (MAX_BODY_BYTES). Hoy ningún endpoint recibe cargas
// grandes; si alguno lo necesitara (p. ej. una importación masiva) habría que
// excluirlo del límite global o subir el valor, decisión que conviene tomar
// explícitamente para ese endpoint y no relajar el límite de toda la API.
//
// Debe montarse después de ErrorHandler (usa Fail).
func LimitarCuerpo(max int64) gin.HandlerFunc {
	if max <= 0 {
		panic("middleware: LimitarCuerpo requiere max > 0")
	}
	return func(c *gin.Context) {
		if c.Request.ContentLength > max {
			ae := errCuerpoGrande(max)
			Fail(c, ae.status, ae.code, ae.message, ae.details)
			return
		}
		if c.Request.Body != nil && c.Request.Body != http.NoBody {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		}
		c.Next()
	}
}