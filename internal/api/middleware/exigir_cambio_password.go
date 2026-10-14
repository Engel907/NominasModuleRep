package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// rutasConCambioPendiente son las únicas rutas protegidas que puede usar una
// cuenta cuyo token lleva rcp=true. Se compara por método + SUFIJO de
// c.FullPath(): el prefijo del grupo raíz (p. ej. /api/v1) no es asunto de
// este middleware. /auth/logout y el reset son públicos (no pasan por acá);
// /auth/logout se lista igual por si en algún momento pasa a ser protegido.
var rutasConCambioPendiente = []struct{ metodo, sufijo string }{
	{http.MethodPost, "/auth/password/cambiar"},
	{http.MethodPost, "/auth/logout"},
	{http.MethodGet, "/auth/me"},
}

// ExigirCambioPassword (F14) debe montarse en el grupo protegido DESPUÉS de
// AuthPASETO (que deja el payload en el contexto). Mientras el access token
// lleve el claim rcp=true, toda ruta fuera de rutasConCambioPendiente responde
// 403 "cambio_password_requerido".
//
// Stateless a propósito: el claim se fija al emitir el token. Al cambiar la
// contraseña, el endpoint devuelve un par de tokens nuevo con rcp=false; el
// token viejo (rcp=true) sigue siendo "restringido" hasta que expire, lo cual
// es el lado seguro del desfase.
func ExigirCambioPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		payload := PayloadDesdeContexto(c)
		if payload == nil || !payload.RequiereCambioPassword {
			c.Next()
			return
		}

		ruta := c.FullPath()
		for _, p := range rutasConCambioPendiente {
			if c.Request.Method == p.metodo && strings.HasSuffix(ruta, p.sufijo) {
				c.Next()
				return
			}
		}

		Fail(c, http.StatusForbidden, "cambio_password_requerido",
			"Debe cambiar su contraseña antes de continuar", nil)
	}
}
