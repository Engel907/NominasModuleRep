package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS implementa CORS con LISTA BLANCA exacta de orígenes
// (CORS_ALLOWED_ORIGINS). Decisiones:
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
