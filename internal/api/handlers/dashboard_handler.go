package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/rrhh"
)

// DashboardHandler expone las métricas gerenciales y el estado de salud.
type DashboardHandler struct {
	service *rrhh.DashboardService
}

func NewDashboardHandler(service *rrhh.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// GetMetricas atiende GET /api/v1/dashboard/metricas.
func (h *DashboardHandler) GetMetricas(c *gin.Context) {
	dto, err := h.service.ObtenerMetricasGenerales(c.Request.Context(), time.Now())
	if err != nil {
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
