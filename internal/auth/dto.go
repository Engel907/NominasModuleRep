package auth

import "time"

// OrigenSolicitud lo completa el handler desde la conexión HTTP (nunca del
// JSON del cliente: por eso `json:"-"` en los requests que lo llevan). Se usa
// para registrar desde dónde falló un login y desde dónde se emitió cada sesión.
type OrigenSolicitud struct {
	IP        string
	UserAgent string
}

type LoginRequest struct {
	NombreUsuario string          `json:"nombre_usuario" binding:"required"`
	Password      string          `json:"password" binding:"required"`
	Origen        OrigenSolicitud `json:"-"`
}

type LoginResponse struct {
	AccessToken            string    `json:"access_token"`
	AccessTokenExpiraEn    time.Time `json:"access_token_expira_en"`
	RefreshToken           string    `json:"refresh_token"`
	RefreshTokenExpiraEn   time.Time `json:"refresh_token_expira_en"`
	RequiereCambioPassword bool      `json:"requiere_cambio_password"`
}

type RefreshRequest struct {
	RefreshToken string          `json:"refresh_token" binding:"required"`
	Origen       OrigenSolicitud `json:"-"`
}

type RefreshResponse struct {
	AccessToken          string    `json:"access_token"`
	AccessTokenExpiraEn  time.Time `json:"access_token_expira_en"`
	RefreshToken         string    `json:"refresh_token"`
	RefreshTokenExpiraEn time.Time `json:"refresh_token_expira_en"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// CambiarPasswordRequest (F14): la política de contraseñas la aplica el
// servicio (ValidarPassword), por eso acá no hay min/max: así el usuario recibe
// el mensaje específico de la regla incumplida y no un 400 genérico.
type CambiarPasswordRequest struct {
	PasswordActual string          `json:"password_actual" binding:"required"`
	NuevaPassword  string          `json:"nueva_password" binding:"required"`
	Origen         OrigenSolicitud `json:"-"`
}

// MeResponse es lo que GET /auth/me devuelve; sale del token, sin tocar la BD.
type MeResponse struct {
	UsuarioID              int64    `json:"usuario_id"`
	NombreUsuario          string   `json:"nombre_usuario"`
	Permisos               []string `json:"permisos"`
	RequiereCambioPassword bool     `json:"requiere_cambio_password"`
}

type AsignarRolRequest struct {
	// gt=0: un rol_id negativo ya se rechazaba con el tipo sin signo anterior;
	// con int64 hay que rechazarlo explícitamente para conservar ese comportamiento.
	RolID int64 `json:"rol_id" binding:"required,gt=0"`
	// FechaEfectiva es opcional; si viene vacía el servicio usa NOW().
	// Permite programar asignaciones a futuro sin reabrir la fila luego.
	FechaEfectiva *time.Time `json:"fecha_efectiva"`
}

type RevocarRolRequest struct {
	RolID int64 `json:"rol_id" binding:"required,gt=0"`
}

type CambiarEstadoRequest struct {
	Activo *bool `json:"activo"`
}

type ReemplazarPermisosRolRequest struct {
	Codigos []string `json:"codigos"`
}

type SolicitarResetPasswordRequest struct {
	NombreUsuario string `json:"nombre_usuario" binding:"required"`
}

// ConfirmarResetPasswordRequest: sin min=8 a propósito (ver CambiarPasswordRequest).
type ConfirmarResetPasswordRequest struct {
	Token         string `json:"token" binding:"required"`
	NuevaPassword string `json:"nueva_password" binding:"required"`
}
