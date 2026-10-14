package middleware

// AUDITORÍA DE PRODUCCIÓN — este archivo reemplaza a un `auth.go` anterior
// que implementaba un SEGUNDO sistema de autenticación PASETO completo
// (paquete aidanwoods.dev/go-paseto, context keys "auth.usuario_id" /
// "auth.permisos") que coexistía, sin integrarse, con este de acá.
//
// Señales de que ESTE es el contrato real y el otro era el descartado:
//   1. routes.go (Fase 3) ya llama a `middleware.AuthPASETO(...)` — ese
//      nombre de función no existía en ningún lado; el otro archivo
//      exponía `(*AuthMiddleware).RequireAuth()`, que nadie llamaba.
//   2. context.go (que sí es importado por auth_handler.go y
//      planilla_export_handler.go) ya define CtxKeyPayload y
//      PayloadDesdeContexto(*security.Payload) — ese es el shape que
//      espera el resto del código, no TokenClaims del otro archivo.
//   3. require_permission_test.go inyecta un *security.Payload bajo
//      CtxKeyPayload y así prueba RequirePermission — confirma el mismo
//      contrato.
//   4. internal/security ya tiene la implementación completa (access +
//      refresh tokens, verificación de tipo de token) — el otro archivo
//      no tenía refresh tokens ("Todavía NO incluye refresh tokens",
//      decía su propio comentario).
//
// Si en algún momento se prefiere volver a aidanwoods.dev/go-paseto, es una
// decisión de diseño a tomar explícitamente (y a propagar a TODOS los
// lugares que hoy dependen de security.Payload/TokenMaker), no algo para
// resolver solo cambiando este archivo.

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/security"
)

// AuthPASETO valida el header "Authorization: Bearer <token>" como access
// token PASETO (ver internal/security.TokenMaker) y, si es válido, deja el
// *security.Payload verificado en el contexto bajo CtxKeyPayload (ver
// context.go) para que PayloadDesdeContexto y RequirePermission lo lean.
//
// Debe montarse en todo grupo de rutas que requiera sesión — todo excepto
// /healthz, /health y los endpoints públicos de internal/auth (login,
// refresh, logout, reset de contraseña).
//
// F15 (S7): tras verificar el token consulta al SesionVerificador (caché de TTL
// corto): si el usuario está inactivo o su permisos_version difiere del "pv" del
// token responde 401 "sesion_invalida" y el cliente debe refrescar el token (el
// refresh re-resuelve los permisos desde la BD). Si no se puede verificar (BD
// caída) falla CERRADO con 503: nunca se acepta un token sin comprobar su vigencia.
func AuthPASETO(tokenMaker security.TokenMaker, sesiones SesionVerificador) gin.HandlerFunc {
	if sesiones == nil {
		panic("middleware: AuthPASETO requiere un SesionVerificador")
	}
	return func(c *gin.Context) {
		rawToken, err := extractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			Fail(c, http.StatusUnauthorized, "missing_token", "Falta el token de autenticación", nil)
			return
		}

		payload, err := tokenMaker.VerificarToken(rawToken, security.TipoTokenAcceso)
		if err != nil {
			FailFromError(c, http.StatusUnauthorized, "invalid_token", "El token es inválido o expiró", err)
			return
		}

		activo, pv, err := sesiones.EstadoSesion(c.Request.Context(), payload.UsuarioID)
		if err != nil {
			FailFromError(c, http.StatusServiceUnavailable, "sesion_no_verificable", "No se pudo verificar la sesión", err)
			return
		}
		if !activo || pv != payload.PermisosVersion {
			Fail(c, http.StatusUnauthorized, "sesion_invalida", "La sesión ya no es válida; vuelva a autenticarse o refresque el token", nil)
			return
		}

		c.Set(CtxKeyPayload, payload)
		c.Next()
	}
}

func extractBearerToken(header string) (string, error) {
	const prefix = "Bearer "
	if header == "" || !strings.HasPrefix(header, prefix) {
		return "", errMissingBearer
	}
	rawToken := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if rawToken == "" {
		return "", errMissingBearer
	}
	return rawToken, nil
}

var errMissingBearer = &missingBearerError{}

type missingBearerError struct{}

func (e *missingBearerError) Error() string { return "falta el header Authorization: Bearer <token>" }