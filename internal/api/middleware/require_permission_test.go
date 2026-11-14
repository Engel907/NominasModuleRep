package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"planillas-ccdr/internal/credenciales"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// construirRouter arma un router mínimo con un endpoint protegido por
// RequirePermission(permisoRequerido). `payload` (puede ser nil) se
// inyecta antes del middleware bajo prueba, simulando lo que AuthPASETO
// dejaría en el contexto tras verificar el token.
func construirRouter(permisoRequerido string, payload *credenciales.Payload) *gin.Engine {
	router := gin.New()
	router.Use(ErrorHandler())
	router.GET("/protegido",
		func(c *gin.Context) {
			if payload != nil {
				c.Set(CtxKeyPayload, payload)
			}
			c.Next()
		},
		RequirePermission(permisoRequerido),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)
	return router
}

func TestRequirePermission_SinPayload_Retorna401(t *testing.T) {
	router := construirRouter("planilla.calcular", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"sin payload en el contexto, RequirePermission debe rechazar con 401 antes de evaluar permisos")
}

func TestRequirePermission_PermisoAusente_Retorna403(t *testing.T) {
	payload := &credenciales.Payload{
		UsuarioID:     1,
		NombreUsuario: "jgarcia",
		Permisos:      []string{"empleados.leer", "asistencia.leer"},
		ExpiraEn:      time.Now().Add(time.Hour),
	}
	router := construirRouter("planilla.calcular", payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code,
		"un usuario autenticado sin el permiso requerido debe recibir 403, no 401")
}

func TestRequirePermission_PermisoPresente_DejaPasar(t *testing.T) {
	payload := &credenciales.Payload{
		UsuarioID:     1,
		NombreUsuario: "jgarcia",
		Permisos:      []string{"empleados.leer", "planilla.calcular"},
		ExpiraEn:      time.Now().Add(time.Hour),
	}
	router := construirRouter("planilla.calcular", payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"ok":true}`, w.Body.String())
}

func TestRequirePermission_ListaDePermisosVacia_Retorna403(t *testing.T) {
	payload := &credenciales.Payload{
		UsuarioID:     2,
		NombreUsuario: "sin.permisos",
		Permisos:      []string{},
		ExpiraEn:      time.Now().Add(time.Hour),
	}
	router := construirRouter("planilla.calcular", payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
