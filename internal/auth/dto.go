package auth

import "time"

type LoginRequest struct {
	NombreUsuario string `json:"nombre_usuario" binding:"required"`
	Password      string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken            string    `json:"access_token"`
	AccessTokenExpiraEn    time.Time `json:"access_token_expira_en"`
	RefreshToken           string    `json:"refresh_token"`
	RefreshTokenExpiraEn   time.Time `json:"refresh_token_expira_en"`
	RequiereCambioPassword bool      `json:"requiere_cambio_password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
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

type SolicitarResetPasswordRequest struct {
	NombreUsuario string `json:"nombre_usuario" binding:"required"`
}

type ConfirmarResetPasswordRequest struct {
	Token         string `json:"token" binding:"required"`
	NuevaPassword string `json:"nueva_password" binding:"required,min=8"`
}
