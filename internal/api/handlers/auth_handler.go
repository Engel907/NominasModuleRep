// Package handlers contiene los handlers Gin del módulo de seguridad.
// Deliberadamente delgados: parsean el request, llaman a internal/auth.Servicio,
// y traducen el resultado a HTTP — sin lógica de negocio acá.
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/auth"
)

type AuthHandler struct {
	servicio *auth.Servicio
}

func NuevoAuthHandler(servicio *auth.Servicio) *AuthHandler {
	return &AuthHandler{servicio: servicio}
}

// RegistrarRutas monta las rutas del módulo. `protegido` es el grupo de
// rutas que ya pasa por middleware.AuthPASETO.
func (h *AuthHandler) RegistrarRutas(publico *gin.RouterGroup, protegido *gin.RouterGroup) {
	publico.POST("/auth/login", h.Login)
	publico.POST("/auth/refresh", h.Refresh)
	publico.POST("/auth/logout", h.Logout)
	publico.POST("/auth/password/reset/solicitar", h.SolicitarResetPassword)
	publico.POST("/auth/password/reset/confirmar", h.ConfirmarResetPassword)

	// Requiere estar autenticado; RequirePermission acota por permiso
	// específico. Los códigos de permiso usados acá son ILUSTRATIVOS —
	// ADR-000 §5 señala que el esquema no siembra `permisos.codigo` todavía;
	// ajustar a los códigos reales una vez exista esa migración de datos
	// (F12: no se tocan en esta fase).
	protegido.POST("/usuarios/:id/roles",
		middleware.RequirePermission("usuarios.asignar_rol"), h.AsignarRol)
	protegido.DELETE("/usuarios/:id/roles",
		middleware.RequirePermission("usuarios.asignar_rol"), h.RevocarRol)
	protegido.POST("/auth/logout-todas-las-sesiones",
		middleware.RequirePermission("usuarios.gestionar_sesiones"), h.LogoutTodasLasSesiones)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req auth.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := h.servicio.Login(c.Request.Context(), req)
	if err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req auth.RefreshRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := h.servicio.Refresh(c.Request.Context(), req)
	if err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req auth.LogoutRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.servicio.Logout(c.Request.Context(), req); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) LogoutTodasLasSesiones(c *gin.Context) {
	actorID, ok := actorAutenticado(c)
	if !ok {
		return
	}
	if err := h.servicio.LogoutTodasLasSesiones(c.Request.Context(), actorID); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) AsignarRol(c *gin.Context) {
	usuarioID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		// ParseIDParam no escribe la respuesta: ErrorHandler traduce
		// *ErrParametroInvalido a 400.
		_ = c.Error(err)
		return
	}
	var req auth.AsignarRolRequest
	if !bindJSON(c, &req) {
		return
	}
	actorID, ok := actorAutenticado(c)
	if !ok {
		return
	}
	if err := h.servicio.AsignarRol(c.Request.Context(), usuarioID, req, actorID); err != nil {
		if errors.Is(err, auth.ErrRolYaVigente) {
			middleware.FailFromError(c, http.StatusConflict, "rol_ya_vigente", "El usuario ya tiene ese rol vigente", err)
			return
		}
		middleware.RespondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) RevocarRol(c *gin.Context) {
	usuarioID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req auth.RevocarRolRequest
	if !bindJSON(c, &req) {
		return
	}
	actorID, ok := actorAutenticado(c)
	if !ok {
		return
	}
	if err := h.servicio.RevocarRol(c.Request.Context(), usuarioID, req, actorID); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SolicitarResetPassword SIEMPRE responde 202 con el mismo mensaje genérico,
// exista o no el usuario — evita user enumeration por este endpoint.
//
// F15: el token en claro que devuelve el servicio se descarta acá; enviarlo
// por el canal de notificación correspondiente (inyectar un
// notifier.EnviarTokenReset(...)) queda pendiente. Hoy el flujo de reset no
// entrega el token a nadie.
func (h *AuthHandler) SolicitarResetPassword(c *gin.Context) {
	var req auth.SolicitarResetPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	if _, _, err := h.servicio.SolicitarResetPassword(c.Request.Context(), req); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"mensaje": "Si el usuario existe, se enviaron instrucciones para restablecer la contraseña.",
	})
}

func (h *AuthHandler) ConfirmarResetPassword(c *gin.Context) {
	var req auth.ConfirmarResetPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.servicio.ConfirmarResetPassword(c.Request.Context(), req); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// bindJSON deserializa y valida el cuerpo. Si falla responde 400 con un
// mensaje fijo: el detalle del validador/decoder (que puede nombrar campos
// y tipos internos) solo va al log vía FailFromError, nunca al cliente.
func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		middleware.FailFromError(c, http.StatusBadRequest, "invalid_request", "La solicitud es inválida", err)
		return false
	}
	return true
}

// actorAutenticado devuelve el usuario_id del token ya verificado por
// AuthPASETO. Si no hay payload responde 401 y devuelve false.
func actorAutenticado(c *gin.Context) (int64, bool) {
	id, ok := middleware.UsuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "missing_token", "Falta el token de autenticación", nil)
		return 0, false
	}
	return id, true
}