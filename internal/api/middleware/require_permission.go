package middleware

// Autorización por PERMISO, nunca por rol (ADR-000 sección 3): este
// middleware nunca debe comparar contra roles.codigo. La resolución
// usuario -> roles vigentes -> permisos ya ocurrió al emitir el access
// token (ver internal/security.PasetoMaker.CrearAccessToken) o, si en el
// futuro se decide resolver contra la BD en cada request en vez de confiar
// en el token (trade-off documentado en ADR-000 sección 3), acá es donde
// se haría esa consulta antes de comparar.
//
// AUDITORÍA DE PRODUCCIÓN: esta versión lee el payload desde
// PayloadDesdeContexto (context.go / CtxKeyPayload), que es lo que deja
// AuthPASETO (auth.go). Antes leía de un PermisosFromContext que
// pertenecía a una segunda implementación de auth (eliminada) que nunca
// llegó a montarse en ninguna ruta real — ver auth.go para el detalle.

import (
	"net/http"
	"sort"
	"sync"

	"github.com/gin-gonic/gin"
)

// Registro de permisos exigidos. Cada vez que se CONSTRUYE un middleware con
// RequirePermission / RequireAnyPermission (al montar las rutas, no por
// request) se anota el código. Existe para poder testear que todo permiso que
// una ruta exige pertenece al catálogo de internal/authz (ver el test de
// coherencia en internal/api/handlers).
var (
	permisosExigidosMu sync.Mutex
	permisosExigidos   = map[string]struct{}{}
)

func registrarPermisosExigidos(codigos ...string) {
	permisosExigidosMu.Lock()
	defer permisosExigidosMu.Unlock()
	for _, c := range codigos {
		permisosExigidos[c] = struct{}{}
	}
}

// PermisosExigidos devuelve, ordenados, los códigos de permiso con los que se
// han construido middlewares de autorización en este proceso.
func PermisosExigidos() []string {
	permisosExigidosMu.Lock()
	defer permisosExigidosMu.Unlock()
	out := make([]string, 0, len(permisosExigidos))
	for c := range permisosExigidos {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// RequirePermission exige que el usuario autenticado (AuthPASETO debe
// correr antes en la cadena de middleware) tenga exactamente el código de
// permiso indicado entre sus permisos vigentes.
//
//	rutas := router.Group("/planilla", middleware.AuthPASETO(tokenMaker))
//	rutas.POST("/aprobar", middleware.RequirePermission(authz.PermPlanillaAprobar), handlerAprobar)
//
// Los códigos siempre vienen de internal/authz; nunca se escriben como literal.
func RequirePermission(codigoPermiso string) gin.HandlerFunc {
	registrarPermisosExigidos(codigoPermiso)
	return func(c *gin.Context) {
		payload := PayloadDesdeContexto(c)
		if payload == nil {
			// AuthPASETO no corrió antes de este middleware en la cadena
			// de rutas: es un error de wiring, no una decisión de negocio,
			// pero se responde igual como no autorizado para no filtrar
			// detalles internos.
			Fail(c, http.StatusUnauthorized, "unauthorized", "No se pudo determinar el usuario autenticado", nil)
			return
		}

		if !hasPermission(payload.Permisos, codigoPermiso) {
			Fail(c, http.StatusForbidden, "permission_denied", "No tenés permiso para realizar esta acción", gin.H{
				"permiso_requerido": codigoPermiso,
			})
			return
		}

		c.Next()
	}
}

// RequireAnyPermission autoriza si el usuario tiene AL MENOS UNO de los
// códigos de permiso indicados. Útil para acciones que varios roles
// distintos pueden ejecutar por caminos de permiso distintos.
func RequireAnyPermission(codigosPermiso ...string) gin.HandlerFunc {
	registrarPermisosExigidos(codigosPermiso...)
	return func(c *gin.Context) {
		payload := PayloadDesdeContexto(c)
		if payload == nil {
			Fail(c, http.StatusUnauthorized, "unauthorized", "No se pudo determinar el usuario autenticado", nil)
			return
		}

		for _, codigo := range codigosPermiso {
			if hasPermission(payload.Permisos, codigo) {
				c.Next()
				return
			}
		}

		Fail(c, http.StatusForbidden, "permission_denied", "No tenés permiso para realizar esta acción", gin.H{
			"permisos_requeridos_alguno_de": codigosPermiso,
		})
	}
}

func hasPermission(permisos []string, codigo string) bool {
	for _, p := range permisos {
		if p == codigo {
			return true
		}
	}
	return false
}