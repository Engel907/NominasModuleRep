package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/rrhh"
)

// HistoricoSalariosHandler: meses de salario anteriores a la puesta en marcha.
type HistoricoSalariosHandler struct {
	service *rrhh.HistoricoSalariosService
}

func NewHistoricoSalariosHandler(s *rrhh.HistoricoSalariosService) *HistoricoSalariosHandler {
	return &HistoricoSalariosHandler{service: s}
}

func (h *HistoricoSalariosHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/empleados")
	g.GET("/:id/historico-salarios", middleware.RequirePermission(authz.PermEmpleadosLeer), h.Listar)
	g.POST("/:id/historico-salarios", middleware.RequirePermission(authz.PermEmpleadosActualizar), h.Cargar)
	g.DELETE("/:id/historico-salarios/:mes_id", middleware.RequirePermission(authz.PermEmpleadosActualizar), h.Eliminar)
}

func (h *HistoricoSalariosHandler) Listar(c *gin.Context) {
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	out, err := h.service.Listar(c.Request.Context(), empleadoID)
	if err != nil {
		responderErrorRRHH(c, err, http.StatusInternalServerError, "internal_error", "error al listar el histórico de salarios")
		return
	}
	c.JSON(http.StatusOK, gin.H{"meses": out})
}

func (h *HistoricoSalariosHandler) Cargar(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var in rrhh.CargarHistoricoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	n, err := h.service.Cargar(c.Request.Context(), usuarioID, empleadoID, in)
	if err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo cargar el histórico de salarios")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"meses_cargados": n})
}

func (h *HistoricoSalariosHandler) Eliminar(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	mesID, ok := parseIDParam(c, "mes_id")
	if !ok {
		return
	}
	if err := h.service.Eliminar(c.Request.Context(), usuarioID, empleadoID, mesID); err != nil {
		if errors.Is(err, rrhh.ErrHistoricoNoEncontrado) {
			middleware.Fail(c, http.StatusNotFound, "not_found", "el mes del histórico no existe", nil)
			return
		}
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo eliminar el mes")
		return
	}
	c.Status(http.StatusNoContent)
}
