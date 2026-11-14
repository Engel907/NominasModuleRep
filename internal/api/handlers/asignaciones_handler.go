package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/rrhh"
)

// AsignacionesHandler: conceptos recurrentes del empleado.
type AsignacionesHandler struct{ service *rrhh.AsignacionesService }

func NewAsignacionesHandler(s *rrhh.AsignacionesService) *AsignacionesHandler {
	return &AsignacionesHandler{service: s}
}

func (h *AsignacionesHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/empleados")
	g.GET("/:id/conceptos-asignados", middleware.RequirePermission(authz.PermEmpleadosLeer), h.Listar)
	g.POST("/:id/conceptos-asignados", middleware.RequirePermission(authz.PermEmpleadosActualizar), h.Asignar)
	g.POST("/conceptos-asignados/:asignacion_id/finalizar", middleware.RequirePermission(authz.PermEmpleadosActualizar), h.Finalizar)
}

func (h *AsignacionesHandler) Listar(c *gin.Context) {
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	out, err := h.service.Listar(c.Request.Context(), empleadoID)
	if err != nil {
		responderErrorAsignacion(c, err, "error al listar los conceptos asignados")
		return
	}
	c.JSON(http.StatusOK, gin.H{"asignaciones": out})
}

func (h *AsignacionesHandler) Asignar(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var in rrhh.AsignarConceptoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	id, err := h.service.Asignar(c.Request.Context(), usuarioID, empleadoID, in)
	if err != nil {
		responderErrorAsignacion(c, err, "no se pudo asignar el concepto")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *AsignacionesHandler) Finalizar(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	id, ok := parseIDParam(c, "asignacion_id")
	if !ok {
		return
	}
	var in rrhh.FinalizarAsignacionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	if err := h.service.Finalizar(c.Request.Context(), usuarioID, id, in); err != nil {
		responderErrorAsignacion(c, err, "no se pudo finalizar la asignación")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "asignación finalizada"})
}

func responderErrorAsignacion(c *gin.Context, err error, msg string) {
	switch {
	case errors.Is(err, rrhh.ErrAsignacionNoEncontrada):
		middleware.Fail(c, http.StatusNotFound, "not_found", "la asignación no existe", nil)
	case errors.Is(err, rrhh.ErrAsignacionCerrada):
		middleware.Fail(c, http.StatusConflict, "conflict", "la asignación ya está finalizada", nil)
	default:
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", msg)
	}
}
