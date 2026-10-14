package handlers_test

// Tests de coherencia de autorización (F12, tests a y c).
//
// Usa el PasetoMaker real de internal/security: el token de prueba es un access
// token válido de un usuario SIN ningún permiso, y pasa por AuthPASETO de verdad.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/handlers"
	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/security"
)

// claveDePrueba cumple el largo exacto de 32 bytes que exige NuevoPasetoMaker.
const claveDePrueba = "0123456789abcdef0123456789abcdef"

// tokenSinPermisos emite un access token válido de un usuario autenticado con
// la lista de permisos vacía.
func tokenSinPermisos(t *testing.T) (security.TokenMaker, string) {
	t.Helper()
	maker, err := security.NuevoPasetoMaker(security.ConfigPaseto{
		Claves:    map[string][]byte{"k1": []byte(claveDePrueba)},
		KIDActivo: "k1",
		Issuer:    "planillas-test",
		Audience:  "planillas-test-api",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := maker.CrearAccessToken(1, "sin.permisos", []string{}, false, 1, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return maker, token
}

// rutasSinPermiso es la lista blanca EXPLÍCITA de rutas de /api/v1 que no
// exigen RequirePermission: los endpoints públicos de auth. Cuando existan
// "cambio de contraseña" y "/me" (solo exigen sesión), se agregan aquí a mano:
// que una ruta nueva quede sin permiso debe ser una decisión visible en el diff.
var rutasSinPermiso = map[string]bool{
	"POST /api/v1/auth/login":                    true,
	"POST /api/v1/auth/refresh":                  true,
	"POST /api/v1/auth/logout":                   true,
	"POST /api/v1/auth/password/reset/solicitar": true,
	"POST /api/v1/auth/password/reset/confirmar": true,
	"POST /api/v1/auth/password/cambiar":         true,
	"GET /api/v1/auth/me":                        true,
}

// routerReal monta el router de producción (handlers.RegisterRoutes) con Deps
// vacías. Contabilidad se pasa como struct vacío porque RegisterRoutes la
// desreferencia (*deps.Contabilidad); el resto de dependencias nil no se usan
// antes de que RequirePermission corte la petición.
func routerReal(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	maker, token := tokenSinPermisos(t)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.ErrorHandler())
	sesiones := middleware.NuevoSesionCache(func(context.Context, int64) (bool, uint32, error) {
		return true, 1, nil
	}, time.Minute)
	handlers.RegisterRoutes(r.Group("/api/v1"), nil, maker, sesiones,
		handlers.Deps{Contabilidad: &handlers.ContabilidadDeps{}})
	return r, token
}

// (a) Todo permiso que una ruta exige existe en el catálogo de authz.
func TestPermisosExigidosPorLasRutasEstanEnAuthz(t *testing.T) {
	_, _ = routerReal(t)

	exigidos := middleware.PermisosExigidos()
	if len(exigidos) == 0 {
		t.Fatal("PermisosExigidos() está vacío: RequirePermission dejó de registrar los códigos")
	}
	catalogo := map[string]bool{}
	for _, c := range authz.Codigos() {
		catalogo[c] = true
	}
	var fuera []string
	for _, c := range exigidos {
		if !catalogo[c] {
			fuera = append(fuera, c)
		}
	}
	if len(fuera) > 0 {
		t.Errorf("rutas que exigen permisos que no están en authz.Todos():\n  %s", strings.Join(fuera, "\n  "))
	}
}

// (c) Toda ruta de /api/v1 fuera de la lista blanca responde 403 a un usuario
// autenticado sin permisos. Una ruta protegida SIN RequirePermission llegaría
// al handler (con dependencias nil): responderá otra cosa o entrará en pánico
// (gin.Recovery => 500) y el test falla señalando la ruta.
func TestRutasProtegidasExigenPermiso(t *testing.T) {
	r, token := routerReal(t)

	existentes := map[string]bool{}
	var probar []gin.RouteInfo
	for _, ri := range r.Routes() {
		clave := ri.Method + " " + ri.Path
		existentes[clave] = true
		if !strings.HasPrefix(ri.Path, "/api/v1/") || rutasSinPermiso[clave] {
			continue
		}
		probar = append(probar, ri)
	}
	if len(probar) == 0 {
		t.Fatal("no hay rutas protegidas que probar: el router no se montó como se esperaba")
	}

	// La lista blanca no debe pudrirse: cada entrada tiene que existir.
	var obsoletas []string
	for clave := range rutasSinPermiso {
		if !existentes[clave] {
			obsoletas = append(obsoletas, clave)
		}
	}
	sort.Strings(obsoletas)
	if len(obsoletas) > 0 {
		t.Errorf("entradas de la lista blanca que ya no existen como ruta:\n  %s", strings.Join(obsoletas, "\n  "))
	}

	for _, ri := range probar {
		ri := ri
		t.Run(ri.Method+" "+ri.Path, func(t *testing.T) {
			req := httptest.NewRequest(ri.Method, rellenarParametros(ri.Path), nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Errorf("respondió %d, se esperaba 403: ¿falta RequirePermission en esta ruta?", w.Code)
			}
		})
	}
}

// rellenarParametros sustituye :param y *comodín por valores válidos.
func rellenarParametros(path string) string {
	partes := strings.Split(path, "/")
	for i, p := range partes {
		switch {
		case strings.HasPrefix(p, ":"):
			partes[i] = "1"
		case strings.HasPrefix(p, "*"):
			partes[i] = "x"
		}
	}
	return strings.Join(partes, "/")
}
