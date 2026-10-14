// Package handlers — Fase 8: endpoints de dashboard gerencial y monitoreo
// operativo (healthcheck/readiness).
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/services"
)

// DashboardHandler expone los endpoints HTTP de Fase 8. Depende de
// *services.DashboardService (no de sqlc ni de BD directamente), mismo
// patrón handler -> service -> repository que las fases anteriores.
type DashboardHandler struct {
	service *services.DashboardService
}

func NewDashboardHandler(service *services.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// GetMetricas atiende GET /api/v1/dashboard/metricas.
//
// Protegido por middleware.RequirePermission(authz.PermDashboardVer) — el
// middleware se monta en el router al registrar la ruta (ver bloque de
// wiring al final de este archivo), no dentro del handler, para mantener
// el handler agnóstico de cómo se arma el grupo de rutas.
func (h *DashboardHandler) GetMetricas(c *gin.Context) {
	dto, err := h.service.ObtenerMetricasGenerales(c.Request.Context(), time.Now())
	if err != nil {
		// El detalle va al log, no al cliente: antes el error se descartaba
		// y una falla de SQL era imposible de diagnosticar.
		slog.Error("dashboard: error obteniendo métricas", slog.String("error", err.Error()))
		middleware.Fail(c, http.StatusInternalServerError, "dashboard_metrics_error",
			"No se pudieron obtener las métricas del dashboard", nil)
		return
	}

	c.JSON(http.StatusOK, dto)
}

// GetHealth atiende GET /health. Es público (sin AuthRequired) a
// propósito: los probes de liveness/readiness de Docker/Kubernetes no
// autentican, y exigirles un token PASETO los rompería.
//
// NOTA DE INTEGRACIÓN (a resolver antes de mergear, no asumida
// silenciosamente): main.go (Fase 1) ya registra un GET /healthz que hace
// un PingContext directo contra sqlDB con timeout fijo de 3s. Este /health
// es más rico (latencia del ping + timestamp además de ok/error) y está
// pensado para REEMPLAZAR a /healthz, no para coexistir con dos rutas casi
// idénticas. Decidir con el equipo: (a) eliminar /healthz de main.go y
// dejar solo /health, o (b) mantener /healthz como alias liviano para
// compatibilidad con probes ya configurados en el orquestador.
//
// Semántica HTTP: 200 si la BD respondió dentro del timeout, 503 si no —
// esto es lo que un probe de Kubernetes usa para sacar el pod del balanceo,
// no basta con responder 200 siempre.
func (h *DashboardHandler) GetHealth(c *gin.Context) {
	estado := h.service.VerificarEstadoSistema(c.Request.Context())

	status := http.StatusOK
	if !estado.BaseDeDatosOK {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, gin.H{
		"status":           estado.Status,
		"base_de_datos_ok": estado.BaseDeDatosOK,
		"latencia_db_ms":   estado.Latencia.Milliseconds(),
		"verificado_en":    estado.VerificadoEn,
	})
}

// ============================================================================
// Wiring de referencia — pegar en newRouter() (main.go) o donde el equipo
// centralice el registro de rutas. No se modifica main.go directamente
// aquí porque el archivo no fue provisto como editable en este entregable.
//
//   dashboardHandler := handlers.NewDashboardHandler(dashboardService)
//
//   router.GET("/health", dashboardHandler.GetHealth) // público, sin auth
//
//   v1 := router.Group("/api/v1")
//   v1.Use(middleware.AuthRequired(cfg.Auth))
//   {
//       v1.GET("/dashboard/metricas",
//           middleware.RequirePermission(authz.PermDashboardVer),
//           dashboardHandler.GetMetricas)
//   }
// ============================================================================