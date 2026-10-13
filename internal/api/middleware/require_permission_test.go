// Pruebas unitarias de middleware.RequirePermission.
//
// SUPUESTO IMPORTANTE: RequirePermission ya fue implementado en Fase 1
// (firma dada: middleware.RequirePermission(perm string) gin.HandlerFunc)
// y no está en este contexto, por lo que estas pruebas no pueden ver su
// código fuente. Se asume el contrato más estándar para este tipo de
// middleware en el proyecto:
//
//  1. Lee *security.Payload desde el gin.Context con la clave CtxKeyPayload
//     (ver context.go) — el mismo payload que AuthPASETO deja seteado.
//  2. Si no hay payload -> 401 Unauthorized y Abort().
//  3. Si el payload existe pero payload.Permisos no contiene `perm` -> 403
//     Forbidden y Abort().
//  4. Si `perm` está presente -> deja pasar (next handler se ejecuta).
//
// Si la implementación real de Fase 1 usa otra clave de contexto, otro
// código de estado, o compara contra roles en vez de permisos, ajustar
// estas pruebas para que reflejen el contrato real — el valor de este
// archivo es la cobertura de los 3 casos (sin sesión / sin permiso / con
// permiso), no los números exactos de status si difieren.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"planillas-ccdr/internal/security"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// construirRouter arma un router mínimo con un endpoint protegido por
// RequirePermission(permisoRequerido). `payload` (puede ser nil) se
// inyecta antes del middleware bajo prueba, simulando lo que AuthPASETO
// dejaría en el contexto tras verificar el token.
func construirRouter(permisoRequerido string, payload *security.Payload) *gin.Engine {
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
	payload := &security.Payload{
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
	payload := &security.Payload{
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
	// Caso borde: usuario con token válido pero sin ningún permiso
	// resuelto (p.ej. todos sus roles fueron desactivados entre la emisión
	// del token y su expiración) — no debe autorizar por default.
	payload := &security.Payload{
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
