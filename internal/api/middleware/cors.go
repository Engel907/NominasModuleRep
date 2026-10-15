package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS implementa CORS con LISTA BLANCA exacta de orígenes
// (CORS_ALLOWED_ORIGINS). Decisiones:
//
//   - Nunca se emite "Access-Control-Allow-Origin: *" (config.validate además
//     rechaza "*" en la lista) y nunca "Access-Control-Allow-Credentials":
//     la API autentica con Authorization: Bearer, no con cookies, así que no
//     hace falta, y así "* + credenciales" es imposible por construcción.
//   - Lista vacía = ningún origen cross-origin permitido (solo same-origin y
//     clientes no-navegador, que no mandan Origin).
//   - Preflight (OPTIONS + Access-Control-Request-Method) de un origen NO
//     permitido -> 403 sin cabeceras CORS. Una petición normal de un origen
//     no permitido sigue su curso SIN cabeceras CORS (el navegador bloquea la
//     lectura de la respuesta; CORS no es un control de acceso del servidor:
//     eso lo hacen AuthPASETO y RequirePermission).
//   - Se responde el preflight acá (204) porque las rutas no registran OPTIONS.
//
// Debe montarse DESPUÉS de ErrorHandler (usa Fail) y antes de AuthPASETO y los
// limitadores, para que un preflight —que no lleva Authorization— no sea
// rechazado por autenticación.
func CORS(origenesPermitidos []string) gin.HandlerFunc {
	permitidos := make(map[string]struct{}, len(origenesPermitidos))
	for _, o := range origenesPermitidos {
		if o != "" && o != "*" {
			permitidos[o] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origen := c.GetHeader("Origin")
		if origen == "" {
			c.Next()
			return
		}

		h := c.Writer.Header()
		h.Add("Vary", "Origin")

		esPreflight := c.Request.Method == http.MethodOptions &&
			c.GetHeader("Access-Control-Request-Method") != ""
		if esPreflight {
			h.Add("Vary", "Access-Control-Request-Method")
			h.Add("Vary", "Access-Control-Request-Headers")
		}

		if _, ok := permitidos[origen]; !ok {
			if esPreflight {
				Fail(c, http.StatusForbidden, "origen_no_permitido", "El origen no está permitido.", nil)
				return
			}
			c.Next()
			return
		}

		h.Set("Access-Control-Allow-Origin", origen)
		h.Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After, Content-Disposition")

		if esPreflight {
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			h.Set("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}