package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/credenciales"
)

// AuthPASETO valida el header "Authorization: Bearer <token>" como access
// token PASETO y, si es válido, deja el *credenciales.Payload verificado en el
// contexto bajo CtxKeyPayload.
func AuthPASETO(tokenMaker credenciales.TokenMaker, sesiones SesionVerificador) gin.HandlerFunc {
	if sesiones == nil {
		panic("middleware: AuthPASETO requiere un SesionVerificador")
	}
	return func(c *gin.Context) {
		rawToken, err := extractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			Fail(c, http.StatusUnauthorized, "missing_token", "Falta el token de autenticación", nil)
			return
		}

		payload, err := tokenMaker.VerificarToken(rawToken, credenciales.TipoTokenAcceso)
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
