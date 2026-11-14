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

type EmpleadoHandler struct {
	service rrhh.EmpleadoService
}

func NewEmpleadoHandler(service rrhh.EmpleadoService) *EmpleadoHandler {
	return &EmpleadoHandler{service: service}
}

// usuarioIDDesdeContexto y parseIDParam son helpers compartidos por todos los
// handlers del paquete `handlers` (contratos y organización también los usan).
func usuarioIDDesdeContexto(c *gin.Context) (int64, bool) {
	id := middleware.UsuarioID(c)
	return id, id > 0
}

// parseIDParam lee el parámetro de ruta como ID int64 estrictamente positivo
// con middleware.ParseIDParam. Si es inválido responde 400 una sola vez y
// devuelve (0, false): el handler solo debe hacer `return`. (ParseIDParam no
// escribe la respuesta; aquí no se llama además a c.Error, para no duplicarla
// con ErrorHandler.)
func parseIDParam(c *gin.Context, nombre string) (int64, bool) {
	id, err := middleware.ParseIDParam(c, nombre)
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "validation_error", "parámetro de ruta inválido", gin.H{"parametro": nombre})
		return 0, false
	}
	return id, true
}

// RegisterRoutes registra las rutas de empleados bajo el grupo dado.
func (h *EmpleadoHandler) RegisterRoutes(rg *gin.RouterGroup) {
	empleados := rg.Group("/empleados")
	{
		empleados.POST("", middleware.RequirePermission(authz.PermEmpleadosCrear), h.CrearExpediente)
		empleados.GET("", middleware.RequirePermission(authz.PermEmpleadosLeer), h.ListarEmpleados)
		empleados.GET("/:id", middleware.RequirePermission(authz.PermEmpleadosLeer), h.GetEmpleado)
		empleados.GET("/:id/datos-sensibles", middleware.RequirePermission(authz.PermEmpleadosLeer), h.DatosSensibles)

		empleados.PUT("/:id/datos-personales", middleware.RequirePermission(authz.PermEmpleadosActualizar), h.ActualizarDatosPersonales)
		empleados.PUT("/:id/datos-laborales", middleware.RequirePermission(authz.PermEmpleadosActualizar), h.ActualizarDatosLaborales)
		empleados.PUT("/:id/datos-bancarios", middleware.RequirePermission(authz.PermEmpleadosActualizar), h.ActualizarDatosBancarios)

		empleados.POST("/:id/familiares", middleware.RequirePermission(authz.PermEmpleadosActualizar), h.AgregarFamiliar)
		empleados.PUT("/familiares/:familiar_id", middleware.RequirePermission(authz.PermEmpleadosActualizar), h.ActualizarFamiliar)
		empleados.GET("/:id/familiares", middleware.RequirePermission(authz.PermEmpleadosLeer), h.ListarFamiliaresVigentes)
	}
}

func (h *EmpleadoHandler) CrearExpediente(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}

	var req rrhh.CrearEmpleadoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}

	resp, err := h.service.CrearExpediente(c.Request.Context(), usuarioID, req)
	if err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo crear el expediente de empleado")
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *EmpleadoHandler) ListarEmpleados(c *gin.Context) {
	var req rrhh.ListarEmpleadosRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		responderErrorBind(c, "parámetros de consulta inválidos", err)
		return
	}
	req.Paginacion = paginacion.Desde(c)

	resp, err := h.service.ListarEmpleados(c.Request.Context(), req)
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al listar empleados", err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *EmpleadoHandler) GetEmpleado(c *gin.Context) {
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	resp, err := h.service.GetEmpleado(c.Request.Context(), empleadoID)
	if errors.Is(err, rrhh.ErrEmpleadoNoEncontrado) {
		middleware.Fail(c, http.StatusNotFound, "not_found", "empleado no encontrado", nil)
		return
	}
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al obtener empleado", err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DatosSensibles maneja GET /empleados/:id/datos-sensibles.
func (h *EmpleadoHandler) DatosSensibles(c *gin.Context) {
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "Token de autenticación requerido", nil)
		return
	}
	verSensible := false
	if p := middleware.PayloadDesdeContexto(c); p != nil {
		verSensible = tienePermiso(p.Permisos, authz.PermDatosPersonalesVerSensible)
	}
	resp, err := h.service.DatosSensibles(c.Request.Context(), rrhh.SolicitudDatosSensibles{
		UsuarioID: usuarioID, VerSensible: verSensible, Recurso: "GET /empleados/:id/datos-sensibles",
		IP: c.ClientIP(), RequestID: middleware.RequestIDDesdeGin(c),
	}, empleadoID)
	if errors.Is(err, rrhh.ErrEmpleadoNoEncontrado) {
		middleware.Fail(c, http.StatusNotFound, "not_found", "empleado no encontrado", nil)
		return
	}
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al leer datos del empleado", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
}

func (h *EmpleadoHandler) ActualizarDatosPersonales(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var in rrhh.DatosPersonalesInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}

	if err := h.service.ActualizarDatosPersonales(c.Request.Context(), usuarioID, empleadoID, in); err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo actualizar datos personales")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "datos personales actualizados"})
}

func (h *EmpleadoHandler) ActualizarDatosLaborales(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var in rrhh.DatosLaboralesInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}

	if err := h.service.ActualizarDatosLaborales(c.Request.Context(), usuarioID, empleadoID, in); err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo actualizar datos laborales")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "datos laborales actualizados"})
}

func (h *EmpleadoHandler) ActualizarDatosBancarios(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var in rrhh.DatosBancariosInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}

	if err := h.service.ActualizarDatosBancarios(c.Request.Context(), usuarioID, empleadoID, in); err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo actualizar datos bancarios")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "datos bancarios actualizados"})
}

func (h *EmpleadoHandler) AgregarFamiliar(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var in rrhh.DatosFamiliarInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}

	if err := h.service.AgregarFamiliar(c.Request.Context(), usuarioID, empleadoID, in); err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo agregar familiar")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"mensaje": "familiar agregado"})
}

func (h *EmpleadoHandler) ActualizarFamiliar(c *gin.Context) {
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "no autenticado", nil)
		return
	}
	familiarID, ok := parseIDParam(c, "familiar_id")
	if !ok {
		return
	}

	var in rrhh.DatosFamiliarInput
	if err := c.ShouldBindJSON(&in); err != nil {
		responderErrorBind(c, "cuerpo de la solicitud inválido", err)
		return
	}

	if err := h.service.ActualizarFamiliar(c.Request.Context(), usuarioID, familiarID, in); err != nil {
		responderErrorRRHH(c, err, http.StatusUnprocessableEntity, "unprocessable_entity", "no se pudo actualizar familiar")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "familiar actualizado"})
}

func (h *EmpleadoHandler) ListarFamiliaresVigentes(c *gin.Context) {
	empleadoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	familiares, err := h.service.ListarFamiliaresVigentes(c.Request.Context(), empleadoID)
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "error al listar familiares", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"familiares": familiares})
}
