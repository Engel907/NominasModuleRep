package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// LimitarCuerpo acota el tamaño del cuerpo de la petición a max bytes
// (MAX_BODY_BYTES):
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
