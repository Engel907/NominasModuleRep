// Package handlers contiene los handlers Gin del módulo de seguridad.
// Deliberadamente delgados: parsean el request, llaman a internal/auth.Servicio,
// y traducen el resultado a HTTP — sin lógica de negocio acá.
package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/auth"
	"planillas-ccdr/internal/authz"
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

	// Autoservicio (F14): solo exigen sesión, NO un permiso — cualquier usuario
	// autenticado puede cambiar SU contraseña y consultar quién es. Siguen
	// siendo las únicas rutas protegidas que permite middleware.ExigirCambioPassword
	// mientras el token lleva rcp=true. permisos_coherencia_test.go debe
	// eximirlas explícitamente de la regla "toda ruta protegida lleva
	// RequirePermission".
	protegido.POST("/auth/password/cambiar", h.CambiarPassword)
	protegido.GET("/auth/me", h.Me)

	// Requiere estar autenticado; RequirePermission acota por permiso
	// específico. Los códigos viven en internal/authz (única fuente de verdad)
	// y los siembra la migración 000112.
	protegido.POST("/usuarios/:id/roles",
		middleware.RequirePermission(authz.PermUsuariosAsignarRol), h.AsignarRol)
	protegido.DELETE("/usuarios/:id/roles",
		middleware.RequirePermission(authz.PermUsuariosAsignarRol), h.RevocarRol)
	protegido.PATCH("/usuarios/:id/estado",
		middleware.RequirePermission(authz.PermUsuariosActualizar), h.CambiarEstadoUsuario)
	protegido.PATCH("/roles/:id/estado",
		middleware.RequirePermission(authz.PermRolesAdministrar), h.CambiarEstadoRol)
	protegido.PUT("/roles/:id/permisos",
		middleware.RequirePermission(authz.PermRolesAdministrar), h.ReemplazarPermisosRol)
	protegido.POST("/auth/logout-todas-las-sesiones",
		middleware.RequirePermission(authz.PermUsuariosGestionarSesiones), h.LogoutTodasLasSesiones)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req auth.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	req.Origen = origenDe(c)
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
	req.Origen = origenDe(c)
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

// CambiarPassword (F14 / S1) cambia la contraseña del usuario autenticado.
// Responde 200 con un par de tokens NUEVO (rcp=false): el cambio revoca todas
// las sesiones, incluida la actual, y el cliente debe reemplazar sus tokens.
func (h *AuthHandler) CambiarPassword(c *gin.Context) {
	actorID, ok := actorAutenticado(c)
	if !ok {
		return
	}
	var req auth.CambiarPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	req.Origen = origenDe(c)
	resp, err := h.servicio.CambiarPassword(c.Request.Context(), actorID, req)
	if err != nil {
		if responderErrorPassword(c, err) {
			return
		}
		middleware.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Me devuelve la identidad que ya viene verificada en el token (sin BD).
func (h *AuthHandler) Me(c *gin.Context) {
	payload := middleware.PayloadDesdeContexto(c)
	if payload == nil {
		middleware.Fail(c, http.StatusUnauthorized, "missing_token", "Falta el token de autenticación", nil)
		return
	}
	permisos := payload.Permisos
	if permisos == nil {
		permisos = []string{}
	}
	c.JSON(http.StatusOK, auth.MeResponse{
		UsuarioID:              payload.UsuarioID,
		NombreUsuario:          payload.NombreUsuario,
		Permisos:               permisos,
		RequiereCambioPassword: payload.RequiereCambioPassword,
	})
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

func (h *AuthHandler) CambiarEstadoUsuario(c *gin.Context) {
	usuarioID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req auth.CambiarEstadoRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Activo == nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "Debe indicar el campo activo.", nil)
		return
	}
	actorID, ok := actorAutenticado(c)
	if !ok {
		return
	}
	if err := h.servicio.CambiarEstadoUsuario(c.Request.Context(), usuarioID, *req.Activo, actorID); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) CambiarEstadoRol(c *gin.Context) {
	rolID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req auth.CambiarEstadoRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Activo == nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "Debe indicar el campo activo.", nil)
		return
	}
	actorID, ok := actorAutenticado(c)
	if !ok {
		return
	}
	if err := h.servicio.CambiarEstadoRol(c.Request.Context(), rolID, *req.Activo, actorID); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) ReemplazarPermisosRol(c *gin.Context) {
	rolID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req auth.ReemplazarPermisosRolRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Codigos == nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "Debe enviar el campo codigos como una lista.", nil)
		return
	}
	actorID, ok := actorAutenticado(c)
	if !ok {
		return
	}
	if err := h.servicio.ReemplazarPermisosRol(c.Request.Context(), rolID, req.Codigos, actorID); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SolicitarResetPassword SIEMPRE responde 202 con el mismo mensaje genérico,
// exista o no el usuario — evita user enumeration por este endpoint. El
// servicio entrega el token por el Notificador (SMTP, o el log en
// development); acá no se ve ni se devuelve nunca. Un error interno se
// registra y se responde igual: si no, el error distinguiría las cuentas
// que existen.
func (h *AuthHandler) SolicitarResetPassword(c *gin.Context) {
	var req auth.SolicitarResetPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.servicio.SolicitarResetPassword(c.Request.Context(), req); err != nil {
		slog.ErrorContext(c.Request.Context(), "auth: error inesperado al solicitar el reset", "error", err)
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
		if responderErrorPassword(c, err) {
			return
		}
		middleware.RespondWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// origenDe arma el origen de la solicitud desde la conexión HTTP. Para que
// ClientIP() devuelva la IP real detrás de un proxy, main.go debe configurar
// router.SetTrustedProxies; si no, será la IP del proxy.
func origenDe(c *gin.Context) auth.OrigenSolicitud {
	return auth.OrigenSolicitud{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}

// responderErrorPassword traduce los errores de contraseña de CambiarPassword
// y ConfirmarResetPassword. Devuelve true si ya respondió.
//   - *ErrPoliticaPassword → 400 "password_invalida" con el mensaje de la regla
//     incumplida (es texto fijo del servicio, seguro de mostrar al usuario).
//   - ErrPasswordActualIncorrecta → 400 "password_actual_incorrecta".
func responderErrorPassword(c *gin.Context, err error) bool {
	var pe *auth.ErrPoliticaPassword
	if errors.As(err, &pe) {
		middleware.FailFromError(c, http.StatusBadRequest, "password_invalida", pe.Mensaje, err)
		return true
	}
	if errors.Is(err, auth.ErrPasswordActualIncorrecta) {
		middleware.FailFromError(c, http.StatusBadRequest, "password_actual_incorrecta", "La contraseña actual es incorrecta", err)
		return true
	}
	return false
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
