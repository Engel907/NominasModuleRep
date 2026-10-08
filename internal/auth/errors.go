package auth

import "errors"

// Errores sentinela que api/handlers mapea a códigos HTTP vía
// api.RespondWithError. Se mantienen específicos (en vez de reusar
// security.ErrPasswordInvalida directamente) para no filtrar detalles de
// la capa de hashing hacia el handler, y para no revelar si fue el usuario
// o el password lo que falló (mismo mensaje/código para ambos casos).
var (
	ErrCredencialesInvalidas = errors.New("auth: usuario o contraseña incorrectos")
	ErrUsuarioInactivo       = errors.New("auth: usuario inactivo")
	ErrUsuarioBloqueado      = errors.New("auth: usuario bloqueado temporalmente por intentos fallidos")
	ErrRefreshTokenInvalido  = errors.New("auth: refresh token inválido, revocado o expirado")
	ErrTokenResetInvalido    = errors.New("auth: token de reset de contraseña inválido, usado o expirado")
	ErrRolInactivo           = errors.New("auth: el rol no está activo")
	ErrRolYaVigente          = errors.New("auth: el usuario ya tiene ese rol vigente")
)
