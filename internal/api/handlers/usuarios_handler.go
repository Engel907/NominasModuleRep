package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/auth"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/pagination"
)

type UsuariosHandler struct {
	service *auth.Servicio
}

func NewUsuariosHandler(service *auth.Servicio) *UsuariosHandler {
	return &UsuariosHandler{service: service}
}

func (h *UsuariosHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/usuarios", middleware.RequirePermission(authz.PermUsuariosCrear), h.crear)
	rg.GET("/usuarios", middleware.RequirePermission(authz.PermUsuariosVer), h.listar)
	rg.GET("/usuarios/:id", middleware.RequirePermission(authz.PermUsuariosVer), h.obtener)
	rg.PATCH("/usuarios/:id", middleware.RequirePermission(authz.PermUsuariosActualizar), h.actualizar)
	rg.POST("/usuarios/:id/activar", middleware.RequirePermission(authz.PermUsuariosActualizar), h.activar)
	rg.POST("/usuarios/:id/desactivar", middleware.RequirePermission(authz.PermUsuariosActualizar), h.desactivar)
	rg.POST("/usuarios/:id/password/reset", middleware.RequirePermission(authz.PermUsuariosResetPassword), h.resetPassword)
	rg.POST("/usuarios/:id/sesiones/revocar", middleware.RequirePermission(authz.PermUsuariosGestionarSesiones), h.revocarSesiones)
	rg.GET("/roles", middleware.RequirePermission(authz.PermUsuariosVer), h.roles)
	rg.GET("/roles/:id/permisos", middleware.RequirePermission(authz.PermUsuariosVer), h.permisosRol)
}

func (h *UsuariosHandler) crear(c *gin.Context) {
	var req auth.CrearUsuarioRequest
	if !bindJSON(c, &req) {
		return
	}
	actor, ok := actorAutenticado(c)
	if !ok {
		return
	}
	resp, err := h.service.CrearUsuario(c.Request.Context(), req, actor)
	if err != nil {
		responderErrorAdministracion(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, resp)
}

func (h *UsuariosHandler) listar(c *gin.Context) {
	p := pagination.Desde(c)
	var activo *bool
	if raw := c.Query("activo"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, "invalid_filter", "El filtro activo debe ser true o false.", nil)
			return
		}
		activo = &value
	}
	users, total, err := h.service.ListarUsuarios(c.Request.Context(), dto.FiltroUsuarios{
		Texto: strings.TrimSpace(c.Query("texto")), Activo: activo,
		Rol: strings.TrimSpace(c.Query("rol")), Limit: p.Limit32(), Offset: p.Offset32(),
	})
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "No se pudieron consultar los usuarios.", err)
		return
	}
	data := make([]auth.UsuarioPublico, 0, len(users))
	for _, u := range users {
		data = append(data, publicUser(u))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "paginacion": pagination.NuevaMeta(p, total)})
}

func (h *UsuariosHandler) obtener(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	u, err := h.service.ObtenerUsuarioAdministracion(c.Request.Context(), id)
	if err != nil {
		responderErrorAdministracion(c, err)
		return
	}
	c.JSON(http.StatusOK, publicUser(u))
}

func (h *UsuariosHandler) actualizar(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req auth.ActualizarUsuarioRequest
	if !bindJSON(c, &req) {
		return
	}
	actor, ok := actorAutenticado(c)
	if !ok {
		return
	}
	if err := h.service.ActualizarUsuario(c.Request.Context(), id, req, actor); err != nil {
		responderErrorAdministracion(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UsuariosHandler) activar(c *gin.Context) {
	h.cambiarEstado(c, true)
}

func (h *UsuariosHandler) desactivar(c *gin.Context) {
	h.cambiarEstado(c, false)
}

func (h *UsuariosHandler) cambiarEstado(c *gin.Context, activo bool) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	actor, ok := actorAutenticado(c)
	if !ok {
		return
	}
	if err := h.service.CambiarEstadoUsuario(c.Request.Context(), id, activo, actor); err != nil {
		responderErrorAdministracion(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UsuariosHandler) resetPassword(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	actor, ok := actorAutenticado(c)
	if !ok {
		return
	}
	password, err := h.service.RestablecerPasswordAdmin(c.Request.Context(), id, actor)
	if err != nil {
		responderErrorAdministracion(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"password_temporal": password, "requiere_cambio_password": true})
}

func (h *UsuariosHandler) revocarSesiones(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	actor, ok := actorAutenticado(c)
	if !ok {
		return
	}
	if err := h.service.RevocarSesionesUsuario(c.Request.Context(), id, actor); err != nil {
		responderErrorAdministracion(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UsuariosHandler) roles(c *gin.Context) {
	roles, err := h.service.Roles(c.Request.Context())
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "No se pudieron consultar los roles.", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": roles})
}

func (h *UsuariosHandler) permisosRol(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	permisos, err := h.service.PermisosDeRol(c.Request.Context(), id)
	if err != nil {
		responderErrorAdministracion(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": permisos})
}

func publicUser(u dto.UsuarioAdministracion) auth.UsuarioPublico {
	return auth.UsuarioPublico{
		ID: u.ID, NombreUsuario: u.NombreUsuario, NombreCompleto: u.NombreCompleto,
		Correo: u.Correo, EmpleadoID: u.EmpleadoIDJSON, Activo: u.Activo,
		RequiereCambioPassword: u.RequiereCambioPassword, Roles: u.Roles,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func idParam(c *gin.Context) (int64, bool) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		middleware.RespondWithError(c, err)
		return 0, false
	}
	return id, true
}

func responderErrorAdministracion(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrDatosAdministracionInvalidos):
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "Los datos del usuario no son válidos.", nil)
	case errors.Is(err, auth.ErrUsuarioAdministracionNoEncontrado), errors.Is(err, auth.ErrRolAdministracionNoEncontrado):
		middleware.Fail(c, http.StatusNotFound, "not_found", "El recurso solicitado no existe.", nil)
	case errors.Is(err, auth.ErrUltimoAdministrador):
		middleware.Fail(c, http.StatusConflict, "last_administrator", "No se puede quitar el último rol de administración.", nil)
	case errors.Is(err, auth.ErrNoEmpleadoVinculado):
		middleware.Fail(c, http.StatusUnprocessableEntity, "employee_not_linked", "El usuario no tiene un empleado vinculado.", nil)
	default:
		middleware.RespondWithError(c, err)
	}
}
