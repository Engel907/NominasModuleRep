package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/security"
)

// Supuesto: ErrorHandler() (ver routes.go) escribe el JSON de lo que Fail
// registra, y Fail aborta la cadena. Ambos existen en este paquete.
func routerExigirCambio(rcp bool) (*gin.Engine, *int) {
	gin.SetMode(gin.TestMode)
	llamadas := 0
	r := gin.New()
	r.Use(ErrorHandler())
	api := r.Group("/api/v1")
	api.Use(func(c *gin.Context) {
		c.Set(CtxKeyPayload, &security.Payload{UsuarioID: 7, RequiereCambioPassword: rcp})
		c.Next()
	})
	api.Use(ExigirCambioPassword())

	ok := func(c *gin.Context) { llamadas++; c.Status(http.StatusNoContent) }
	api.POST("/auth/password/cambiar", ok)
	api.GET("/auth/me", ok)
	api.POST("/auth/logout", ok)
	api.GET("/empleados", ok)
	api.POST("/auth/logout-todas-las-sesiones", ok)
	return r, &llamadas
}

func TestExigirCambioPassword(t *testing.T) {
	casos := []struct {
		nombre, metodo, ruta string
		rcp                  bool
		status               int
	}{
		{"rcp: cambiar contraseña permitido", "POST", "/api/v1/auth/password/cambiar", true, http.StatusNoContent},
		{"rcp: /auth/me permitido", "GET", "/api/v1/auth/me", true, http.StatusNoContent},
		{"rcp: logout permitido", "POST", "/api/v1/auth/logout", true, http.StatusNoContent},
		{"rcp: ruta de negocio bloqueada", "GET", "/api/v1/empleados", true, http.StatusForbidden},
		{"rcp: logout-todas NO se cuela por prefijo", "POST", "/api/v1/auth/logout-todas-las-sesiones", true, http.StatusForbidden},
		{"rcp: método incorrecto en ruta permitida", "GET", "/api/v1/auth/password/cambiar", true, http.StatusNotFound},
		{"sin rcp: ruta de negocio pasa", "GET", "/api/v1/empleados", false, http.StatusNoContent},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			r, llamadas := routerExigirCambio(tc.rcp)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.metodo, tc.ruta, nil))

			if w.Code != tc.status {
				t.Fatalf("status = %d, se esperaba %d (cuerpo: %s)", w.Code, tc.status, w.Body.String())
			}
			if tc.status == http.StatusForbidden {
				if !strings.Contains(w.Body.String(), "cambio_password_requerido") {
					t.Fatalf("el cuerpo debe llevar el código cambio_password_requerido: %s", w.Body.String())
				}
				if *llamadas != 0 {
					t.Fatal("el handler de negocio no debe ejecutarse")
				}
			}
		})
	}
}
