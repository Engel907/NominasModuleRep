// Package observabilidad: métricas Prometheus de la API.
package observabilidad

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metricas: registro propio (no el global) con las métricas de la API.
type Metricas struct {
	reg          *prometheus.Registry
	duracionHTTP *prometheus.HistogramVec
	errores5xx   *prometheus.CounterVec
	calculo      *prometheus.HistogramVec
	empleadosErr *prometheus.CounterVec
}

// Nuevas registra las métricas y los colectores de proceso y del runtime de Go.
func Nuevas() *Metricas {
	m := &Metricas{
		reg: prometheus.NewRegistry(),
		duracionHTTP: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "planillas_http_duracion_segundos", Help: "Latencia de las solicitudes HTTP por ruta.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		}, []string{"metodo", "ruta", "codigo"}),
		errores5xx: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "planillas_http_errores_5xx_total", Help: "Respuestas HTTP 5xx por ruta.",
		}, []string{"metodo", "ruta"}),
		calculo: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "planillas_calculo_periodo_duracion_segundos", Help: "Duración del cálculo de un período de planilla.",
			Buckets: []float64{.1, .5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		}, []string{"clase", "resultado"}),
		empleadosErr: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "planillas_calculo_empleados_con_error_total", Help: "Empleados con error en cálculos de período.",
		}, []string{"clase"}),
	}
	m.reg.MustRegister(m.duracionHTTP, m.errores5xx, m.calculo, m.empleadosErr,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

// Middleware mide cada solicitud (ruta = plantilla de Gin).
func (m *Metricas) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		inicio := time.Now()
		c.Next()
		ruta := c.FullPath()
		if ruta == "" {
			ruta = "no_encontrada"
		}
		codigo := c.Writer.Status()
		metodo := c.Request.Method
		m.duracionHTTP.WithLabelValues(metodo, ruta, strconv.Itoa(codigo)).Observe(time.Since(inicio).Seconds())
		if codigo >= 500 {
			m.errores5xx.WithLabelValues(metodo, ruta).Inc()
		}
	}
}

// ObservarCalculo implementa planilla.ObservadorCalculo.
func (m *Metricas) ObservarCalculo(clase string, duracion time.Duration, empleadosConError int, ok bool) {
	resultado := "ok"
	if !ok {
		resultado = "error"
	}
	m.calculo.WithLabelValues(clase, resultado).Observe(duracion.Seconds())
	if empleadosConError > 0 {
		m.empleadosErr.WithLabelValues(clase).Add(float64(empleadosConError))
	}
}

// Handler: /metrics protegido con Bearer token (comparación en tiempo
// constante). token vacío → 404 (métricas deshabilitadas).
func (m *Metricas) Handler(token string) gin.HandlerFunc {
	h := promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
	return func(c *gin.Context) {
		if token == "" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		dado, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(dado), []byte(token)) != 1 {
			c.Header("WWW-Authenticate", `Bearer realm="metrics"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(c.Writer, c.Request)
	}
}
