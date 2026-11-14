package handlers

import (
	"errors"
	"net/http"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/paginacion"
	"planillas-ccdr/internal/rrhh"

	"github.com/gin-gonic/gin"
)

type ContratoHandler struct {
	service rrhh.ContratoService
}

type contratoResponseSegura struct {
	rrhh.ContratoResponse
	SalarioPactado any `json:"salario_pactado"`
}

func NewContratoHandler(service rrhh.ContratoService) *ContratoHandler {
	return &ContratoHandler{service: service}
}

func (h *ContratoHandler) RegisterRoutes(rg *gin.RouterGroup) {
	contratos := rg.Group("/contratos")
	{
		contratos.POST("", middleware.RequirePermission(authz.PermContratosCrear), h.CrearContrato)
		contratos.GET("", middleware.RequirePermission(authz.PermContratosLeer), h.ListarContratos)
	}

	empleados := rg.Group("/empleados")
	{
		empleados.PUT("/:id/contrato", middleware.RequirePermission(authz.PermContratosActualizar), h.ActualizarContrato)
		empleados.GET("/:id/contrato", middleware.RequirePermission(authz.PermContratosLeer), h.GetContratoVigente)
		empleados.GET("/:id/contratos/historico", middleware.RequirePermission(authz.PermContratosLeer), h.ListarHistoricoPorEmpleado)
	}
}

func (h *ContratoHandler) CrearContrato(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}

	var req rrhh.CrearContratoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}

	resp, err := h.service.CrearContrato(c.Request.Context(), usuarioID, req)
	if err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo procesar el contrato")
		return
	}
	c.JSON(http.StatusCreated, contratoSeguro(c, *resp))
}

func (h *ContratoHandler) ActualizarContrato(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req rrhh.ActualizarContratoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}

	resp, err := h.service.ActualizarContrato(c.Request.Context(), usuarioID, empleadoID, req)
	if err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo procesar el contrato")
		return
	}
	c.JSON(http.StatusOK, contratoSeguro(c, *resp))
}

func (h *ContratoHandler) GetContratoVigente(c *gin.Context) {
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	resp, err := h.service.GetContratoVigente(c.Request.Context(), empleadoID)
	if errors.Is(err, rrhh.ErrContratoNoEncontrado) {
		middleware.Fail(c, http.StatusNotFound, "not_found", "el empleado no tiene contrato vigente", nil)
		return
	}
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al obtener contrato vigente", err)
		return
	}
	c.JSON(http.StatusOK, contratoSeguro(c, *resp))
}

func (h *ContratoHandler) ListarHistoricoPorEmpleado(c *gin.Context) {
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	pag := paginacion.Desde(c)

	items, resumenPaginacion, err := h.service.ListarHistoricoPorEmpleado(c.Request.Context(), empleadoID, pag)
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al listar histórico de contratos", err)
		return
	}
	contratos := make([]contratoResponseSegura, 0, len(items))
	for _, item := range items {
		contratos = append(contratos, contratoSeguro(c, item))
	}
	c.JSON(http.StatusOK, gin.H{"contratos": contratos, "paginacion": resumenPaginacion})
}

func (h *ContratoHandler) ListarContratos(c *gin.Context) {
	var req rrhh.ListarContratosRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		responderErrorBind(c, "parámetros de consulta inválidos", err)
		return
	}
	req.Paginacion = paginacion.Desde(c)

	resp, err := h.service.ListarContratos(c.Request.Context(), req)
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al listar contratos", err)
		return
	}
	contratos := make([]contratoResponseSegura, 0, len(resp.Contratos))
	for _, item := range resp.Contratos {
		contratos = append(contratos, contratoSeguro(c, item))
	}
	c.JSON(http.StatusOK, gin.H{"contratos": contratos, "paginacion": resp.Paginacion})
}

func contratoSeguro(c *gin.Context, contrato rrhh.ContratoResponse) contratoResponseSegura {
	salario := any(contrato.SalarioPactado)
	payload := middleware.PayloadDesdeContexto(c)
	if payload == nil || !tienePermiso(payload.Permisos, authz.PermDatosPersonalesVerSensible) {
		salario = "***REDACTED***"
	}
	return contratoResponseSegura{ContratoResponse: contrato, SalarioPactado: salario}
}
