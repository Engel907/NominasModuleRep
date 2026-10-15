package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders agrega las cabeceras de seguridad a TODA respuesta (incluidos
// errores, 404 y preflights), porque se monta antes que el resto de la
// cadena:
//
//   - X-Content-Type-Options: nosniff
//   - X-Frame-Options: DENY
//   - Referrer-Policy: no-referrer
//   - Cache-Control: no-store (respuestas de API con datos de planilla: no
//     deben quedar en cachés del navegador ni de proxies). Un handler puede
//     sobrescribirlo explícitamente si algún día sirve algo cacheable.
//   - Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
//     (la API solo sirve JSON/archivos; nada debe ejecutarse ni embeberse).
//   - Strict-Transport-Security: SOLO en producción (en desarrollo con HTTP
//     plano no aplica y un HSTS mal puesto "pega" el dominio a HTTPS).
func SecurityHeaders(produccion bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if produccion {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}