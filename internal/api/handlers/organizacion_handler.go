// api/handlers/organizacion_handler.go
//
// Fase 3 — Handlers delgados de los catálogos organizacionales: Puestos,
// Unidades Organizativas, Programas Presupuestarios y Clasificaciones de
// Puesto.
package handlers

import (
	"errors"
	"net/http"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/pagination"
	"planillas-ccdr/internal/services"

	"github.com/gin-gonic/gin"
)

// -----------------------------------------------------------------------------
// OrganizacionHandler agrupa los 4 catálogos porque comparten forma (CRUD
// simple + paginación) y así los handlers de negocio (empleados, contratos)
// no quedan mezclados con los de catálogo.
// -----------------------------------------------------------------------------

type OrganizacionHandler struct {
	clasificaciones services.ClasificacionPuestoService
	programas       services.ProgramaPresupuestarioService
	unidades        services.UnidadOrganizativaService
	puestos         services.PuestoService
}

func NewOrganizacionHandler(
	clasificaciones services.ClasificacionPuestoService,
	programas services.ProgramaPresupuestarioService,
	unidades services.UnidadOrganizativaService,
	puestos services.PuestoService,
) *OrganizacionHandler {
	return &OrganizacionHandler{
		clasificaciones: clasificaciones,
		programas:       programas,
		unidades:        unidades,
		puestos:         puestos,
	}
}

func (h *OrganizacionHandler) RegisterRoutes(rg *gin.RouterGroup) {
	clasificaciones := rg.Group("/clasificaciones-puesto")
	{
		clasificaciones.POST("", middleware.RequirePermission("organizacion.crear"), h.CrearClasificacionPuesto)
		clasificaciones.GET("", middleware.RequirePermission("organizacion.leer"), h.ListarClasificacionesPuesto)
		clasificaciones.GET("/:id", middleware.RequirePermission("organizacion.leer"), h.GetClasificacionPuesto)
		clasificaciones.PUT("/:id", middleware.RequirePermission("organizacion.actualizar"), h.ActualizarClasificacionPuesto)
	}

	programas := rg.Group("/programas-presupuestarios")
	{
		programas.POST("", middleware.RequirePermission("organizacion.crear"), h.CrearProgramaPresupuestario)
		programas.GET("", middleware.RequirePermission("organizacion.leer"), h.ListarProgramasPresupuestarios)
		programas.GET("/:id", middleware.RequirePermission("organizacion.leer"), h.GetProgramaPresupuestario)
		programas.PUT("/:id", middleware.RequirePermission("organizacion.actualizar"), h.ActualizarProgramaPresupuestario)
	}

	unidades := rg.Group("/unidades-organizativas")
	{
		unidades.POST("", middleware.RequirePermission("organizacion.crear"), h.CrearUnidadOrganizativa)
		unidades.GET("", middleware.RequirePermission("organizacion.leer"), h.ListarUnidadesOrganizativas)
		unidades.GET("/:id", middleware.RequirePermission("organizacion.leer"), h.GetUnidadOrganizativa)
		unidades.PUT("/:id", middleware.RequirePermission("organizacion.actualizar"), h.ActualizarUnidadOrganizativa)
	}

	puestos := rg.Group("/puestos")
	{
		puestos.POST("", middleware.RequirePermission("organizacion.crear"), h.CrearPuesto)
		puestos.GET("", middleware.RequirePermission("organizacion.leer"), h.ListarPuestos)
		puestos.GET("/:id", middleware.RequirePermission("organizacion.leer"), h.GetPuesto)
		puestos.PUT("/:id", middleware.RequirePermission("organizacion.actualizar"), h.ActualizarPuesto)
	}
}

// --------------------------- Clasificaciones de Puesto ----------------------

func (h *OrganizacionHandler) CrearClasificacionPuesto(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	var in services.ClasificacionPuestoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	creado, err := h.clasificaciones.Crear(c.Request.Context(), usuarioID, in)
	if err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo crear la clasificación de puesto")
		return
	}
	c.JSON(http.StatusCreated, creado)
}

func (h *OrganizacionHandler) ActualizarClasificacionPuesto(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var in services.ClasificacionPuestoUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	if err := h.clasificaciones.Actualizar(c.Request.Context(), usuarioID, id, in); err != nil {
		if errors.Is(err, services.ErrNoEncontrado) {
			middleware.Fail(c, http.StatusNotFound, "not_found", "clasificación de puesto no encontrada", nil)
			return
		}
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo actualizar la clasificación de puesto")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "clasificación de puesto actualizada"})
}

func (h *OrganizacionHandler) GetClasificacionPuesto(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.clasificaciones.Get(c.Request.Context(), id)
	if errors.Is(err, services.ErrNoEncontrado) {
		middleware.Fail(c, http.StatusNotFound, "not_found", "clasificación de puesto no encontrada", nil)
		return
	}
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al obtener clasificación de puesto", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *OrganizacionHandler) ListarClasificacionesPuesto(c *gin.Context) {
	var req services.CatalogoListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		responderErrorBind(c, "parámetros de consulta inválidos", err)
		return
	}
	req.Paginacion = pagination.Desde(c)
	items, paginacion, err := h.clasificaciones.List(c.Request.Context(), req)
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al listar clasificaciones de puesto", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"clasificaciones_puesto": items, "paginacion": paginacion})
}

// --------------------------- Programas Presupuestarios ----------------------

func (h *OrganizacionHandler) CrearProgramaPresupuestario(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	var in services.ProgramaPresupuestarioInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	creado, err := h.programas.Crear(c.Request.Context(), usuarioID, in)
	if err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo crear el programa presupuestario")
		return
	}
	c.JSON(http.StatusCreated, creado)
}

func (h *OrganizacionHandler) ActualizarProgramaPresupuestario(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var in services.ProgramaPresupuestarioUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	if err := h.programas.Actualizar(c.Request.Context(), usuarioID, id, in); err != nil {
		if errors.Is(err, services.ErrNoEncontrado) {
			middleware.Fail(c, http.StatusNotFound, "not_found", "programa presupuestario no encontrado", nil)
			return
		}
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo actualizar el programa presupuestario")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "programa presupuestario actualizado"})
}

func (h *OrganizacionHandler) GetProgramaPresupuestario(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.programas.Get(c.Request.Context(), id)
	if errors.Is(err, services.ErrNoEncontrado) {
		middleware.Fail(c, http.StatusNotFound, "not_found", "programa presupuestario no encontrado", nil)
		return
	}
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al obtener programa presupuestario", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *OrganizacionHandler) ListarProgramasPresupuestarios(c *gin.Context) {
	var req services.CatalogoListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		responderErrorBind(c, "parámetros de consulta inválidos", err)
		return
	}
	req.Paginacion = pagination.Desde(c)
	items, paginacion, err := h.programas.List(c.Request.Context(), req)
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al listar programas presupuestarios", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"programas_presupuestarios": items, "paginacion": paginacion})
}

// --------------------------- Unidades Organizativas --------------------------

func (h *OrganizacionHandler) CrearUnidadOrganizativa(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	var in services.UnidadOrganizativaInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	creado, err := h.unidades.Crear(c.Request.Context(), usuarioID, in)
	if err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo crear la unidad organizativa")
		return
	}
	c.JSON(http.StatusCreated, creado)
}

func (h *OrganizacionHandler) ActualizarUnidadOrganizativa(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var in services.UnidadOrganizativaUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	if err := h.unidades.Actualizar(c.Request.Context(), usuarioID, id, in); err != nil {
		if errors.Is(err, services.ErrNoEncontrado) {
			middleware.Fail(c, http.StatusNotFound, "not_found", "unidad organizativa no encontrada", nil)
			return
		}
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo actualizar la unidad organizativa")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "unidad organizativa actualizada"})
}

func (h *OrganizacionHandler) GetUnidadOrganizativa(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.unidades.Get(c.Request.Context(), id)
	if errors.Is(err, services.ErrNoEncontrado) {
		middleware.Fail(c, http.StatusNotFound, "not_found", "unidad organizativa no encontrada", nil)
		return
	}
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al obtener unidad organizativa", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *OrganizacionHandler) ListarUnidadesOrganizativas(c *gin.Context) {
	var req services.UnidadOrganizativaListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		responderErrorBind(c, "parámetros de consulta inválidos", err)
		return
	}
	req.Paginacion = pagination.Desde(c)
	items, paginacion, err := h.unidades.List(c.Request.Context(), req)
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al listar unidades organizativas", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"unidades_organizativas": items, "paginacion": paginacion})
}

// --------------------------------- Puestos -----------------------------------

func (h *OrganizacionHandler) CrearPuesto(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	var in services.PuestoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	creado, err := h.puestos.Crear(c.Request.Context(), usuarioID, in)
	if err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo crear el puesto")
		return
	}
	c.JSON(http.StatusCreated, creado)
}

func (h *OrganizacionHandler) ActualizarPuesto(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var in services.PuestoUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}
	if err := h.puestos.Actualizar(c.Request.Context(), usuarioID, id, in); err != nil {
		if errors.Is(err, services.ErrNoEncontrado) {
			middleware.Fail(c, http.StatusNotFound, "not_found", "puesto no encontrado", nil)
			return
		}
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo actualizar el puesto")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "puesto actualizado"})
}

func (h *OrganizacionHandler) GetPuesto(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.puestos.Get(c.Request.Context(), id)
	if errors.Is(err, services.ErrNoEncontrado) {
		middleware.Fail(c, http.StatusNotFound, "not_found", "puesto no encontrado", nil)
		return
	}
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al obtener puesto", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *OrganizacionHandler) ListarPuestos(c *gin.Context) {
	var req services.PuestoListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		responderErrorBind(c, "parámetros de consulta inválidos", err)
		return
	}
	req.Paginacion = pagination.Desde(c)
	items, paginacion, err := h.puestos.List(c.Request.Context(), req)
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al listar puestos", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"puestos": items, "paginacion": paginacion})
}
