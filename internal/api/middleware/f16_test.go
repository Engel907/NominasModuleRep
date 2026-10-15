package middleware_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/security"
	"planillas-ccdr/internal/services"
)

// ---------------------------------------------------------------- helpers

func nuevoRouter(extra ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	middleware.ConfigurarValidador()
	r := gin.New()
	_ = r.SetTrustedProxies(nil) // como en producción sin TRUSTED_PROXIES
	r.Use(middleware.RequestID(), middleware.ErrorHandler())
	r.Use(extra...)
	return r
}

func peticion(r http.Handler, metodo, ruta, cuerpo string, cab map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	// request_id fijo por defecto: los UUID aleatorios podrían contener por azar
	// una subcadena que un NotContains busca (p. ej. "abc").
	req.Header.Set("X-Request-ID", "test-req-0001")
	for k, v := range cab {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// unicoJSON verifica que el cuerpo sea EXACTAMENTE un objeto JSON (una doble
// escritura dejaría dos objetos concatenados) y lo decodifica.
func unicoJSON(t *testing.T, w *httptest.ResponseRecorder) middleware.ErrorResponse {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(w.Body.Bytes()))
	var resp middleware.ErrorResponse
	require.NoError(t, dec.Decode(&resp), "cuerpo: %s", w.Body.String())
	assert.False(t, dec.More(), "el cuerpo tiene más de un JSON (doble respuesta): %s", w.Body.String())
	return resp
}

type entrada struct {
	Nombre string `json:"nombre" binding:"required"`
	Edad   int    `json:"edad" binding:"gte=0,lte=150"`
}

func rutaBind(r *gin.Engine) {
	r.POST("/x", func(c *gin.Context) {
		var in entrada
		if err := c.ShouldBindJSON(&in); err != nil {
			middleware.RespondWithError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
}

// ---------------------------------------------------------------- C3: binding

func TestBindingInvalido_400DatosInvalidos(t *testing.T) {
	r := nuevoRouter()
	rutaBind(r)

	casos := []struct {
		nombre, cuerpo, campo string
	}{
		{"json mal formado", `{nombre: x}`, ""},
		{"json truncado", `{"nombre":"a"`, ""},
		{"cuerpo vacio", ``, ""},
		{"tipo incorrecto", `{"nombre":"a","edad":"abc"}`, "edad"},
		{"falta requerido", `{"edad":5}`, "nombre"},
		{"fuera de rango", `{"nombre":"a","edad":999}`, "edad"},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			w := peticion(r, http.MethodPost, "/x", tc.cuerpo, nil)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

			resp := unicoJSON(t, w)
			assert.Equal(t, "datos_invalidos", resp.Error.Code)
			assert.NotEmpty(t, resp.Error.RequestID)
			assert.Equal(t, w.Header().Get("X-Request-ID"), resp.Error.RequestID)

			det, ok := resp.Error.Details.(map[string]any)
			require.True(t, ok)
			campos, ok := det["campos"].([]any)
			require.True(t, ok)
			require.NotEmpty(t, campos)
			if tc.campo != "" {
				assert.Equal(t, tc.campo, campos[0].(map[string]any)["campo"])
			}
			// El mensaje nunca debe traer el texto crudo del decoder ni el valor recibido.
			assert.NotContains(t, w.Body.String(), "invalid character")
			assert.NotContains(t, w.Body.String(), "abc")
			assert.NotContains(t, w.Body.String(), "json:")
		})
	}
}

func TestBindingValido_204(t *testing.T) {
	r := nuevoRouter()
	rutaBind(r)
	w := peticion(r, http.MethodPost, "/x", `{"nombre":"Ana","edad":30}`, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestErrorInterno_500GenericoSinFiltrar(t *testing.T) {
	r := nuevoRouter()
	r.GET("/boom", func(c *gin.Context) {
		middleware.RespondWithError(c, errors.New("dial tcp 10.9.8.7:3306: password=hunter2"))
	})
	w := peticion(r, http.MethodGet, "/boom", "", nil)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	resp := unicoJSON(t, w)
	assert.Equal(t, "internal_error", resp.Error.Code)
	assert.NotEmpty(t, resp.Error.RequestID)
	assert.NotContains(t, w.Body.String(), "10.9.8.7")
	assert.NotContains(t, w.Body.String(), "hunter2")
}

func TestErrSalarioInferiorAlMinimo_MensajeFijo(t *testing.T) {
	r := nuevoRouter()
	r.GET("/s", func(c *gin.Context) {
		middleware.RespondWithError(c, fmt.Errorf("%w: ₡123456 < ₡999999 (detalle interno)", services.ErrSalarioInferiorAlMinimo))
	})
	w := peticion(r, http.MethodGet, "/s", "", nil)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	resp := unicoJSON(t, w)
	assert.Equal(t, "salary_below_minimum", resp.Error.Code)
	assert.NotContains(t, w.Body.String(), "123456")
	assert.NotContains(t, w.Body.String(), "interno")
}

func TestSqlErrNoRows_404(t *testing.T) {
	r := nuevoRouter()
	r.GET("/n", func(c *gin.Context) { middleware.RespondWithError(c, fmt.Errorf("buscando: %w", sql.ErrNoRows)) })
	w := peticion(r, http.MethodGet, "/n", "", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestParseIDParam_Invalido_400SinDobleRespuesta(t *testing.T) {
	r := nuevoRouter()
	r.GET("/e/:id", func(c *gin.Context) {
		id, err := middleware.ParseIDParam(c, "id")
		if err != nil {
			_ = c.Error(err) // patrón documentado: propagar y retornar
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id})
	})

	w := peticion(r, http.MethodGet, "/e/abc", "", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	resp := unicoJSON(t, w)
	assert.Equal(t, "parametro_invalido", resp.Error.Code)
	assert.NotContains(t, w.Body.String(), "abc") // el valor crudo no se devuelve

	assert.Equal(t, http.StatusBadRequest, peticion(r, http.MethodGet, "/e/-3", "", nil).Code)
	assert.Equal(t, http.StatusOK, peticion(r, http.MethodGet, "/e/7", "", nil).Code)
}

func TestHandlerQueYaEscribio_NoSeEscribeSegundaRespuesta(t *testing.T) {
	r := nuevoRouter()
	r.GET("/dup", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		_ = c.Error(errors.New("algo falló después de responder"))
	})
	w := peticion(r, http.MethodGet, "/dup", "", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"ok":true}`, w.Body.String())
}

// ---------------------------------------------------------------- cuerpo (413)

func TestLimiteDeCuerpo_413(t *testing.T) {
	r := nuevoRouter(middleware.LimitarCuerpo(1024))
	rutaBind(r)
	grande := `{"nombre":"` + strings.Repeat("a", 4096) + `"}`

	t.Run("Content-Length declarado", func(t *testing.T) {
		w := peticion(r, http.MethodPost, "/x", grande, nil)
		require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
		assert.Equal(t, "cuerpo_demasiado_grande", unicoJSON(t, w).Error.Code)
	})

	t.Run("sin Content-Length (chunked)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(grande))
		req.ContentLength = -1
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
		assert.Equal(t, "cuerpo_demasiado_grande", unicoJSON(t, w).Error.Code)
	})

	t.Run("dentro del límite", func(t *testing.T) {
		w := peticion(r, http.MethodPost, "/x", `{"nombre":"Ana","edad":1}`, nil)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

// ---------------------------------------------------------------- rate limit (429)

func TestRateLimitPorIP_429ConRetryAfter(t *testing.T) {
	lim, err := middleware.NuevoLimitador(60, 2)
	require.NoError(t, err)

	r := nuevoRouter()
	r.POST("/auth/login", middleware.LimitarPorIP(lim), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.POST("/auth/refresh", middleware.LimitarPorIP(lim), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	desde := func(ruta, ip string, xff string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, ruta, nil)
		req.RemoteAddr = ip + ":4000"
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	assert.Equal(t, http.StatusNoContent, desde("/auth/login", "203.0.113.5", "").Code)
	assert.Equal(t, http.StatusNoContent, desde("/auth/login", "203.0.113.5", "").Code)

	// Tercera: rechazada. Falsificar X-Forwarded-For NO ayuda (sin proxies de confianza).
	w := desde("/auth/login", "203.0.113.5", "1.2.3.4")
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "demasiadas_solicitudes", unicoJSON(t, w).Error.Code)
	seg := w.Header().Get("Retry-After")
	require.NotEmpty(t, seg)
	assert.NotEqual(t, "0", seg)

	// Otra ruta de la misma IP y otra IP en la misma ruta tienen su propio cupo.
	assert.Equal(t, http.StatusNoContent, desde("/auth/refresh", "203.0.113.5", "").Code)
	assert.Equal(t, http.StatusNoContent, desde("/auth/login", "198.51.100.9", "").Code)
}

func TestRateLimitPorUsuario(t *testing.T) {
	lim, err := middleware.NuevoLimitador(60, 1)
	require.NoError(t, err)

	r := nuevoRouter(func(c *gin.Context) { // simula AuthPASETO
		var id int64
		fmt.Sscan(c.GetHeader("X-Test-Usuario"), &id)
		c.Set(middleware.CtxKeyPayload, &security.Payload{UsuarioID: id})
		c.Next()
	}, middleware.LimitarPorUsuario(lim))
	r.GET("/p", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	con := func(u string) int {
		return peticion(r, http.MethodGet, "/p", "", map[string]string{"X-Test-Usuario": u}).Code
	}
	assert.Equal(t, http.StatusNoContent, con("1"))
	assert.Equal(t, http.StatusTooManyRequests, con("1"))
	assert.Equal(t, http.StatusNoContent, con("2")) // otro usuario, otro cupo
}

func TestLimitador_RellenoYPurga(t *testing.T) {
	ahora := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lim, err := middleware.NuevoLimitadorConReloj(60, 1, func() time.Time { return ahora })
	require.NoError(t, err)

	ok, _ := lim.Permitir("a")
	assert.True(t, ok)
	ok, espera := lim.Permitir("a")
	assert.False(t, ok)
	assert.InDelta(t, 1.0, espera.Seconds(), 0.01) // 60/min = 1 ficha por segundo

	ahora = ahora.Add(1100 * time.Millisecond)
	ok, _ = lim.Permitir("a")
	assert.True(t, ok, "tras 1 s se repone una ficha")

	lim.Permitir("b")
	require.Equal(t, 2, lim.Tamano())
	ahora = ahora.Add(10 * time.Minute)
	lim.Permitir("c") // dispara la purga de las inactivas
	assert.Equal(t, 1, lim.Tamano(), "a y b se purgan; queda c")
}

func TestNuevoLimitador_RechazaCero(t *testing.T) {
	_, err := middleware.NuevoLimitador(0, 5)
	assert.Error(t, err)
	_, err = middleware.NuevoLimitador(5, 0)
	assert.Error(t, err)
}

// ---------------------------------------------------------------- CORS

func TestCORS(t *testing.T) {
	const ok = "https://planillas.ccdrliberia.go.cr"
	r := nuevoRouter(middleware.CORS([]string{ok}))
	r.GET("/d", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	t.Run("preflight de origen permitido", func(t *testing.T) {
		w := peticion(r, http.MethodOptions, "/d", "", map[string]string{
			"Origin": ok, "Access-Control-Request-Method": "GET",
		})
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, ok, w.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, w.Header().Values("Vary"), "Origin")
	})

	t.Run("preflight de origen NO permitido", func(t *testing.T) {
		w := peticion(r, http.MethodOptions, "/d", "", map[string]string{
			"Origin": "https://evil.example", "Access-Control-Request-Method": "GET",
		})
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "origen_no_permitido", unicoJSON(t, w).Error.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("petición normal de origen NO permitido: sin cabeceras CORS", func(t *testing.T) {
		w := peticion(r, http.MethodGet, "/d", "", map[string]string{"Origin": "https://evil.example"})
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("petición de origen permitido", func(t *testing.T) {
		w := peticion(r, http.MethodGet, "/d", "", map[string]string{"Origin": ok})
		assert.Equal(t, ok, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("sin Origin no agrega nada", func(t *testing.T) {
		w := peticion(r, http.MethodGet, "/d", "", nil)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("lista vacía o con * no permite a nadie y jamás emite *", func(t *testing.T) {
		for _, lista := range [][]string{nil, {"*"}} {
			rr := nuevoRouter(middleware.CORS(lista))
			rr.GET("/d", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			w := peticion(rr, http.MethodGet, "/d", "", map[string]string{"Origin": "https://x.example"})
			assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
		}
	})
}

// ---------------------------------------------------------------- cabeceras

func TestCabecerasDeSeguridad(t *testing.T) {
	for _, prod := range []bool{false, true} {
		r := gin.New()
		r.Use(middleware.SecurityHeaders(prod))
		r.GET("/h", func(c *gin.Context) { c.Status(http.StatusNoContent) })

		for _, ruta := range []string{"/h", "/no-existe"} { // también en el 404
			w := peticion(r, http.MethodGet, ruta, "", nil)
			h := w.Header()
			assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
			assert.Equal(t, "DENY", h.Get("X-Frame-Options"))
			assert.Equal(t, "no-referrer", h.Get("Referrer-Policy"))
			assert.Equal(t, "no-store", h.Get("Cache-Control"))
			assert.NotEmpty(t, h.Get("Content-Security-Policy"))
			if prod {
				assert.Contains(t, h.Get("Strict-Transport-Security"), "max-age=")
			} else {
				assert.Empty(t, h.Get("Strict-Transport-Security"))
			}
		}
	}
}

// ---------------------------------------------------------------- logging

func routerConLog(buf *bytes.Buffer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := middleware.NuevoLogger(buf, slog.LevelDebug)
	slog.SetDefault(logger)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(middleware.RequestID(), middleware.RequestLogger(logger), middleware.Recovery(logger), middleware.ErrorHandler())
	return r
}

func TestLog_NoFiltraQueryTokensNiContrasenas(t *testing.T) {
	var buf bytes.Buffer
	r := routerConLog(&buf)
	r.GET("/api/v1/items/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	peticion(r, http.MethodGet, "/api/v1/items/42?token=SECRETO&password=abc123", "", map[string]string{
		"Authorization": "Bearer v4.local.LLAVE_SECRETA",
	})
	peticion(r, http.MethodPost, "/api/v1/auth/login", `{"usuario":"ana","password":"PASS_SUPER_SECRETA"}`, nil)
	peticion(r, http.MethodGet, "/ruta/que/no/existe?x=VALOR_QUERY", "", nil)

	salida := buf.String()
	for _, prohibido := range []string{"SECRETO", "abc123", "LLAVE_SECRETA", "PASS_SUPER_SECRETA", "VALOR_QUERY", "token=", "password=", "/items/42", "no/existe"} {
		assert.NotContains(t, salida, prohibido)
	}
	assert.Contains(t, salida, `"route":"/api/v1/items/:id"`)
	assert.Contains(t, salida, `"status":200`)
	assert.Contains(t, salida, `"method":"GET"`)
	assert.Contains(t, salida, `"request_id"`)
	assert.Contains(t, salida, `"latency_ms"`)
	assert.Contains(t, salida, `"route":"(sin_ruta)"`)
}

func TestLog_RedactaAtributosSensibles(t *testing.T) {
	var buf bytes.Buffer
	l := middleware.NuevoLogger(&buf, slog.LevelInfo)
	l.Info("x", "password", "hunter2", "access_token", "v4.local.abc", "usuario", "ana")
	assert.NotContains(t, buf.String(), "hunter2")
	assert.NotContains(t, buf.String(), "v4.local.abc")
	assert.Contains(t, buf.String(), "[REDACTADO]")
	assert.Contains(t, buf.String(), `"usuario":"ana"`)
}

func TestRequestID_EcoYValidacion(t *testing.T) {
	var buf bytes.Buffer
	r := routerConLog(&buf)
	r.GET("/r", func(c *gin.Context) {
		c.String(http.StatusOK, middleware.RequestIDDesdeContext(c.Request.Context()))
	})

	w := peticion(r, http.MethodGet, "/r", "", map[string]string{"X-Request-ID": "abc-12345"})
	assert.Equal(t, "abc-12345", w.Header().Get("X-Request-ID"))
	assert.Equal(t, "abc-12345", w.Body.String(), "el request_id se propaga en el context")

	w = peticion(r, http.MethodGet, "/r", "", map[string]string{"X-Request-ID": `mal "id"` + "\n{x}"})
	generado := w.Header().Get("X-Request-ID")
	assert.NotContains(t, generado, `"`)
	assert.Len(t, generado, 36) // UUID

	assert.Contains(t, buf.String(), `"request_id":"abc-12345"`)
}

func TestRecovery_PanicEs500GenericoYSeLoguea(t *testing.T) {
	var buf bytes.Buffer
	r := routerConLog(&buf)
	r.GET("/panic", func(c *gin.Context) { panic("detalle interno con password=hunter2") })

	w := peticion(r, http.MethodGet, "/panic", "", nil)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	resp := unicoJSON(t, w)
	assert.Equal(t, "internal_error", resp.Error.Code)
	assert.NotEmpty(t, resp.Error.RequestID)
	assert.NotContains(t, w.Body.String(), "hunter2")

	assert.Contains(t, buf.String(), "panic_recuperado")
	assert.Contains(t, buf.String(), `"status":500`)
}

func TestErrorHandler_LogueaCausaConRequestIDPeroNoLaDevuelve(t *testing.T) {
	var buf bytes.Buffer
	r := routerConLog(&buf)
	r.GET("/e", func(c *gin.Context) { middleware.RespondWithError(c, errors.New("fallo SQL interno XYZ")) })

	w := peticion(r, http.MethodGet, "/e", "", nil)
	assert.NotContains(t, w.Body.String(), "XYZ")
	assert.Contains(t, buf.String(), "fallo SQL interno XYZ")
	assert.Contains(t, buf.String(), `"msg":"error_api"`)
}
