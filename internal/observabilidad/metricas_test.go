package observabilidad

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func router(m *Metricas, token string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/metrics", m.Handler(token))
	r.GET("/api/v1/planillas/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/falla", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	return r
}

func pedir(r http.Handler, ruta, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, ruta, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMetricsExigeToken(t *testing.T) {
	m := Nuevas()
	if w := pedir(router(m, ""), "/metrics", "Bearer x"); w.Code != http.StatusNotFound {
		t.Fatalf("sin token configurado: %d", w.Code)
	}
	r := router(m, strings.Repeat("t", 32))
	if w := pedir(r, "/metrics", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("sin Authorization: %d", w.Code)
	}
	if w := pedir(r, "/metrics", "Bearer otro"); w.Code != http.StatusUnauthorized {
		t.Fatalf("token incorrecto: %d", w.Code)
	}
}

func TestMetricsPorPlantillaDeRuta(t *testing.T) {
	m := Nuevas()
	token := strings.Repeat("t", 32)
	r := router(m, token)
	pedir(r, "/api/v1/planillas/123", "")
	pedir(r, "/falla", "")
	m.ObservarCalculo("regular", 2*time.Second, 3, false)
	w := pedir(r, "/metrics", "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("metrics: %d", w.Code)
	}
	cuerpo := w.Body.String()
	for _, quiere := range []string{
		`planillas_http_duracion_segundos_count{codigo="200",metodo="GET",ruta="/api/v1/planillas/:id"} 1`,
		`planillas_http_errores_5xx_total{metodo="GET",ruta="/falla"} 1`,
		`planillas_calculo_periodo_duracion_segundos_count{clase="regular",resultado="error"} 1`,
		`planillas_calculo_empleados_con_error_total{clase="regular"} 3`,
	} {
		if !strings.Contains(cuerpo, quiere) {
			t.Errorf("falta %q", quiere)
		}
	}
	if strings.Contains(cuerpo, "/api/v1/planillas/123") {
		t.Error("la ruta real (con id) no debe aparecer en las etiquetas")
	}
}
