package middleware

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/credenciales"
)

// CtxKeyPayload es la clave bajo la cual middleware.AuthPASETO
// debe guardar el *credenciales.Payload verificado en el gin.Context, y la
// misma que middleware.RequirePermission y PayloadDesdeContexto leen.
const CtxKeyPayload = "auth_payload"

var ErrEmpleadoIDNoVinculado = errors.New("middleware: el usuario autenticado no tiene empleado vinculado")

type EmpleadoIDResolver interface {
	EmpleadoIDDeUsuario(context.Context, int64) (int64, error)
}

// EmpleadoIDDelUsuario resuelve el vínculo actual del usuario autenticado
// desde la BD, en lugar de confiar en una copia posiblemente obsoleta del token.
func EmpleadoIDDelUsuario(c *gin.Context, resolver EmpleadoIDResolver) (int64, error) {
	usuarioID, ok := UsuarioIDDesdeContexto(c)
	if !ok {
		return 0, errors.New("middleware: falta el payload de autenticación")
	}
	if resolver == nil {
		return 0, errors.New("middleware: no se configuró el resolver de empleados")
	}
	empleadoID, err := resolver.EmpleadoIDDeUsuario(c.Request.Context(), usuarioID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrEmpleadoIDNoVinculado
	}
	if err != nil {
		return 0, err
	}
	if empleadoID <= 0 {
		return 0, ErrEmpleadoIDNoVinculado
	}
	return empleadoID, nil
}

// PayloadDesdeContexto extrae el payload PASETO seteado por AuthPASETO.
// Devuelve nil si no hay payload (ruta sin AuthPASETO, o token ausente).
func PayloadDesdeContexto(c *gin.Context) *credenciales.Payload {
	valor, existe := c.Get(CtxKeyPayload)
	if !existe {
		return nil
	}
	payload, ok := valor.(*credenciales.Payload)
	if !ok {
		return nil
	}
	return payload
}

// UsuarioIDDesdeContexto extrae el usuario_id autenticado que AuthPASETO
// dejó en el contexto. Devuelve (0, false) si no
// hay payload, típicamente porque la ruta no tiene AuthPASETO montado.
func UsuarioIDDesdeContexto(c *gin.Context) (int64, bool) {
	payload := PayloadDesdeContexto(c)
	if payload == nil {
		return 0, false
	}
	return payload.UsuarioID, true
}

// UsuarioID es un atajo de una sola variable de retorno sobre
// UsuarioIDDesdeContexto, para call sites que no necesitan distinguir
// "no autenticado" de "usuario_id == 0" (asumen que AuthPASETO ya corrió
// y ya devolvió 401 si no había payload). Devuelve 0 si no hay payload.
func UsuarioID(c *gin.Context) int64 {
	id, _ := UsuarioIDDesdeContexto(c)
	return id
}

// ErrParametroInvalido indica que un parámetro de ruta no es válido. Lo
// devuelve ParseIDParam sin escribir la respuesta HTTP; ErrorHandler (vía
// traducirError) lo convierte en 400 "parametro_invalido" detectándolo con
// errors.As:
type ErrParametroInvalido struct {
	// Nombre del parámetro de ruta (p. ej. "id").
	Nombre string
	// Valor tal como llegó en la URL. Solo para diagnóstico interno: no se
	// devuelve al cliente.
	Valor string
}

func (e *ErrParametroInvalido) Error() string {
	return fmt.Sprintf("parámetro de ruta '%s' inválido", e.Nombre)
}

// ParseIDParam parsea un parámetro de ruta (:id) como ID int64 estrictamente
// positivo. No responde ni aborta: devuelve *ErrParametroInvalido para que el
// handler lo propague (RespondWithError(c, err) o c.Error(err), y return) y
// ErrorHandler lo convierta en 400.
func ParseIDParam(c *gin.Context, nombre string) (int64, error) {
	valor := c.Param(nombre)
	id, err := strconv.ParseInt(valor, 10, 64)
	if err != nil || id <= 0 {
		return 0, &ErrParametroInvalido{Nombre: nombre, Valor: valor}
	}
	return id, nil
}
