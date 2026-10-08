package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/security"
)

// CtxKeyPayload es la clave bajo la cual middleware.AuthPASETO (Fase 1)
// debe guardar el *security.Payload verificado en el gin.Context, y la
// misma que middleware.RequirePermission y PayloadDesdeContexto leen.
//
// SUPUESTO A CONFIRMAR: si la implementación real de AuthPASETO (Fase 1,
// no incluida en este contexto) ya usa otra clave, actualizarla acá — es
// el único lugar que debería requerir cambio.
const CtxKeyPayload = "auth_payload"

// PayloadDesdeContexto extrae el payload PASETO seteado por AuthPASETO.
// Devuelve nil si no hay payload (ruta sin AuthPASETO, o token ausente).
func PayloadDesdeContexto(c *gin.Context) *security.Payload {
	valor, existe := c.Get(CtxKeyPayload)
	if !existe {
		return nil
	}
	payload, ok := valor.(*security.Payload)
	if !ok {
		return nil
	}
	return payload
}

// UsuarioIDDesdeContexto extrae el usuario_id autenticado que AuthPASETO
// dejó en el contexto (ver PayloadDesdeContexto). Devuelve (0, false) si no
// hay payload, típicamente porque la ruta no tiene AuthPASETO montado.
//
// El ID es int64 de punta a punta (Payload.UsuarioID, internal/db/dto y el
// parámetro usuarioID de db.WithAuditContext — decisión D1): sin conversiones.
//
// AUDITORÍA DE PRODUCCIÓN: existían DOS copias de esta misma función,
// declaradas por separado en distintos handlers del mismo paquete
// `handlers` — lo cual ni siquiera compila (identificador duplicado en el
// paquete) — y ninguna de las dos leía la clave/tipo real que usa
// AuthPASETO (una leía "authorization_payload", la otra "usuario_id" como
// int64; la real es CtxKeyPayload="auth_payload" con *security.Payload).
// Se unifica acá, junto a PayloadDesdeContexto.
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
//
// AUDITORÍA DE PRODUCCIÓN: internal/asistencia/handler.go ya llamaba a
// `middleware.UsuarioID(c)` (4 veces) sin que esta función existiera en
// ningún lado — se agrega acá con el nombre y forma que ese archivo ya
// esperaba, apuntando al mismo contrato real (CtxKeyPayload).
func UsuarioID(c *gin.Context) int64 {
	id, _ := UsuarioIDDesdeContexto(c)
	return id
}

// ErrParametroInvalido indica que un parámetro de ruta no es válido. Lo
// devuelve ParseIDParam SIN escribir la respuesta HTTP; ErrorHandler lo
// traduce a 400 detectándolo con errors.As:
//
//	var pe *ErrParametroInvalido
//	if errors.As(err, &pe) { /* 400, pe.Nombre */ }
type ErrParametroInvalido struct {
	// Nombre del parámetro de ruta (p. ej. "id").
	Nombre string
	// Valor tal como llegó en la URL.
	Valor string
}

func (e *ErrParametroInvalido) Error() string {
	return fmt.Sprintf("parámetro de ruta '%s' inválido", e.Nombre)
}

// ParseIDParam parsea un parámetro de ruta (:id) como ID int64 estrictamente
// positivo. A diferencia de ParseUintParam NO responde ni aborta: devuelve
// *ErrParametroInvalido para que el handler lo propague (c.Error(err) +
// return) y ErrorHandler lo convierta en 400.
//
// Uso: id, err := ParseIDParam(c, "id").
func ParseIDParam(c *gin.Context, nombre string) (int64, error) {
	valor := c.Param(nombre)
	id, err := strconv.ParseInt(valor, 10, 64)
	if err != nil || id <= 0 {
		return 0, &ErrParametroInvalido{Nombre: nombre, Valor: valor}
	}
	return id, nil
}

// ParseUintParam parsea un parámetro de ruta (:id) como uint64, respondiendo
// 400 automáticamente si no es válido. Uso: id, err := ParseUintParam(c, "id").
//
// Deprecated: usar ParseIDParam. Se conserva solo mientras los handlers de
// otras fases migran uno por uno; la F16 la elimina.
func ParseUintParam(c *gin.Context, nombre string) (uint64, error) {
	valor, err := strconv.ParseUint(c.Param(nombre), 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "parámetro de ruta '" + nombre + "' inválido",
		})
		return 0, err
	}
	return valor, nil
}
