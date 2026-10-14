package auth

import "errors"

// Errores sentinela que api/handlers mapea a códigos HTTP vía
// api.RespondWithError. Se mantienen específicos (en vez de reusar
// security.ErrPasswordInvalida directamente) para no filtrar detalles de
// la capa de hashing hacia el handler, y para no revelar si fue el usuario
// o el password lo que falló (mismo mensaje/código para ambos casos).
var (
	ErrCredencialesInvalidas = errors.New("auth: usuario o contraseña incorrectos")

	// F14 / S5: Login YA NO devuelve ErrUsuarioInactivo ni ErrUsuarioBloqueado
	// (revelaban el estado de la cuenta). Siguen declarados porque el motivo
	// real se registra en el log interno con estos mismos textos, y para no
	// romper código que los referencie.
	ErrUsuarioInactivo  = errors.New("auth: usuario inactivo")
	ErrUsuarioBloqueado = errors.New("auth: usuario bloqueado temporalmente por intentos fallidos")

	ErrRefreshTokenInvalido = errors.New("auth: refresh token inválido, revocado o expirado")
	ErrTokenResetInvalido   = errors.New("auth: token de reset de contraseña inválido, usado o expirado")
	ErrRolInactivo          = errors.New("auth: el rol no está activo")
	ErrRolYaVigente         = errors.New("auth: el usuario ya tiene ese rol vigente")
	ErrPermisoInvalido      = errors.New("auth: el código de permiso no pertenece al catálogo")
	ErrPermisoNoEncontrado  = errors.New("auth: el permiso solicitado no está sembrado en la base de datos")
	ErrPermisoDuplicado     = errors.New("auth: la lista contiene códigos de permiso duplicados")

	// ErrPasswordActualIncorrecta: la contraseña actual enviada a
	// CambiarPassword no coincide. El handler responde 400 (no 401: el token
	// de acceso es válido, y un 401 haría que el cliente intente refrescarlo).
	ErrPasswordActualIncorrecta = errors.New("auth: la contraseña actual es incorrecta")

	// errReutilizacionRefresh es interno: aborta la transacción de rotación
	// cuando el token ya había sido revocado por otro request.
	errReutilizacionRefresh = errors.New("auth: reutilización de refresh token")
)
