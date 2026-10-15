package middleware

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"planillas-ccdr/internal/auth"
	"planillas-ccdr/internal/security"
	"planillas-ccdr/internal/services"
)

// Mensaje genérico de los 500 (S15): nunca incluye detalles internos. El
// cliente correlaciona con el request_id que viaja en el cuerpo y en la
// cabecera X-Request-ID.
const mensajeErrorInterno = "Ocurrió un error inesperado. Intentá de nuevo más tarde."

// CampoInvalido describe un campo con problema en un 400 "datos_invalidos".
// Va en ErrorBody.Details bajo la clave "campos".
type CampoInvalido struct {
	Campo   string `json:"campo"`
	Mensaje string `json:"mensaje"`
}

// RespondWithError es el punto de entrada único que usan los handlers de
// internal/api/handlers para traducir un error (auth, services, security,
// sql.ErrNoRows, errores de bind/JSON de Gin, cuerpo demasiado grande,
// parámetros de ruta inválidos) al contrato JSON de ErrorResponse.
//
// No escribe la respuesta: registra un *apiError en el contexto y aborta;
// ErrorHandler es quien escribe el JSON (una sola vez). Los handlers deben
// hacer `RespondWithError(c, err); return`.
//
// NOTA sobre import cycles: este archivo puede importar auth/services/
// security porque ninguno de esos paquetes importa middleware. Los
// paquetes que sí importan middleware (internal/asistencia,
// internal/prestamos) NO deben agregarse acá.
func RespondWithError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	_ = c.Error(traducirError(err))
	c.Abort()
}

// traducirError convierte cualquier error en el *apiError que se le muestra
// al cliente. Es la única función que decide status/código/mensaje: la usan
// RespondWithError y ErrorHandler (que también recibe errores crudos vía
// c.Error(err)).
//
// Regla de oro (S15): el mensaje del cliente sale SIEMPRE de este código,
// jamás de err.Error(). El error original viaja en apiError.cause solo para
// el log del servidor.
func traducirError(err error) *apiError {
	// Ya traducido (p. ej. vino de Fail/FailFromError envuelto).
	var yaApi *apiError
	if errors.As(err, &yaApi) {
		return yaApi
	}

	// --- cuerpo demasiado grande (http.MaxBytesReader) ---
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return errCuerpoGrande(maxErr.Limit)
	}

	// --- parámetro de ruta inválido (ParseIDParam) ---
	var pe *ErrParametroInvalido
	if errors.As(err, &pe) {
		return &apiError{
			status:  http.StatusBadRequest,
			code:    "parametro_invalido",
			message: fmt.Sprintf("El parámetro de ruta '%s' no es válido.", pe.Nombre),
			details: gin.H{"parametro": pe.Nombre},
		}
	}

	// --- bind / validación / JSON malformado ---
	if campos, ok := camposInvalidos(err); ok {
		return &apiError{
			status:  http.StatusBadRequest,
			code:    "datos_invalidos",
			message: "Los datos enviados no son válidos.",
			details: gin.H{"campos": campos},
		}
	}

	// --- errores de dominio ---
	for _, r := range reglasDominio {
		if errors.Is(err, r.err) {
			return &apiError{status: r.status, code: r.code, message: r.mensaje}
		}
	}

	// --- genérico de base de datos ---
	if errors.Is(err, sql.ErrNoRows) {
		return &apiError{status: http.StatusNotFound, code: "not_found", message: "El recurso solicitado no existe."}
	}

	return &apiError{
		status:  http.StatusInternalServerError,
		code:    "internal_error",
		message: mensajeErrorInterno,
		cause:   err,
	}
}

func errCuerpoGrande(limite int64) *apiError {
	return &apiError{
		status:  http.StatusRequestEntityTooLarge,
		code:    "cuerpo_demasiado_grande",
		message: "El cuerpo de la petición supera el tamaño máximo permitido.",
		details: gin.H{"limite_bytes": limite},
	}
}

// reglasDominio mapea errores sentinela a respuestas con mensaje fijo y
// seguro. El orden no importa salvo que un error envuelva a otro.
var reglasDominio = []struct {
	err     error
	status  int
	code    string
	mensaje string
}{
	// --- auth ---
	{auth.ErrCredencialesInvalidas, http.StatusUnauthorized, "invalid_credentials", "Usuario o contraseña incorrectos."},
	{auth.ErrUsuarioInactivo, http.StatusForbidden, "user_inactive", "El usuario no está activo."},
	{auth.ErrUsuarioBloqueado, http.StatusForbidden, "user_locked", "El usuario está bloqueado temporalmente por intentos fallidos."},
	{auth.ErrRefreshTokenInvalido, http.StatusUnauthorized, "invalid_token", "El token es inválido, fue revocado o expiró."},
	{auth.ErrTokenResetInvalido, http.StatusBadRequest, "invalid_reset_token", "El token de restablecimiento es inválido, ya se usó o expiró."},
	{auth.ErrRolInactivo, http.StatusUnprocessableEntity, "role_inactive", "El rol no está activo."},
	{auth.ErrRolYaVigente, http.StatusConflict, "role_already_assigned", "El usuario ya tiene ese rol vigente."},
	{auth.ErrAutoDesactivacion, http.StatusForbidden, "self_deactivation_forbidden", "No puede desactivar su propia cuenta."},
	{auth.ErrUltimoAdministrador, http.StatusConflict, "last_administrator", "No se puede quitar el último rol de administración."},
	{auth.ErrUsuarioAdministracionNoEncontrado, http.StatusNotFound, "not_found", "El usuario solicitado no existe."},
	{auth.ErrRolAdministracionNoEncontrado, http.StatusNotFound, "not_found", "El rol solicitado no existe."},
	{auth.ErrDatosAdministracionInvalidos, http.StatusBadRequest, "invalid_request", "Los datos del usuario no son válidos."},
	{auth.ErrPermisoInvalido, http.StatusBadRequest, "invalid_permission", "La lista contiene un código de permiso no válido."},
	{auth.ErrPermisoDuplicado, http.StatusBadRequest, "duplicate_permission", "La lista no puede repetir códigos de permiso."},

	// --- security ---
	{security.ErrPasswordInvalida, http.StatusBadRequest, "invalid_password", "La contraseña no cumple los requisitos."},
	{security.ErrTokenInvalido, http.StatusUnauthorized, "invalid_token", "El token es inválido o expiró."},
	{security.ErrTokenExpirado, http.StatusUnauthorized, "invalid_token", "El token es inválido o expiró."},

	// --- services (empleados / contratos / organización) ---
	{services.ErrEmpleadoNoEncontrado, http.StatusNotFound, "not_found", "El recurso solicitado no existe."},
	{services.ErrContratoNoEncontrado, http.StatusNotFound, "not_found", "El recurso solicitado no existe."},
	{services.ErrNoEncontrado, http.StatusNotFound, "not_found", "El recurso solicitado no existe."},
	// Mensaje FIJO: antes devolvía err.Error(), que puede traer montos y
	// detalles del cálculo interno.
	{services.ErrSalarioInferiorAlMinimo, http.StatusUnprocessableEntity, "salary_below_minimum", "El salario es inferior al mínimo permitido."},
}

// camposInvalidos detecta (con errors.As / errors.Is, sin comparar texto
// salvo el caso de campos desconocidos) los errores que produce
// c.ShouldBindJSON y los traduce a una lista de campos en español. Devuelve
// ok=false si el error no es de binding.
//
// Los mensajes se construyen a partir del tag de validación y del nombre
// del campo; NUNCA incluyen el valor recibido (podría ser una contraseña).
func camposInvalidos(err error) ([]CampoInvalido, bool) {
	// Errores de validación (binding:"required", etc.).
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		campos := make([]CampoInvalido, 0, len(ve))
		for _, fe := range ve {
			campos = append(campos, CampoInvalido{Campo: nombreCampo(fe), Mensaje: mensajeValidacion(fe)})
		}
		return campos, true
	}

	// Tipo JSON incompatible con el campo destino (p. ej. "abc" en un int).
	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) {
		return []CampoInvalido{{
			Campo:   ute.Field,
			Mensaje: "Tipo de dato inválido: se esperaba " + tipoEsperado(ute.Type) + ".",
		}}, true
	}

	// JSON mal formado.
	var se *json.SyntaxError
	if errors.As(err, &se) {
		return []CampoInvalido{{Mensaje: "El cuerpo no es un JSON válido."}}, true
	}

	// Cuerpo vacío o cortado. OJO: io.EOF / io.ErrUnexpectedEOF también
	// podrían venir de una capa de datos; es poco probable (el driver MySQL
	// usa driver.ErrBadConn / ErrInvalidConn) y el costo de un falso positivo
	// es un 400 en vez de un 500.
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return []CampoInvalido{{Mensaje: "El cuerpo de la petición está vacío o incompleto."}}, true
	}

	// Fecha/hora con formato inválido dentro del JSON (time.Time).
	var pe *time.ParseError
	if errors.As(err, &pe) {
		return []CampoInvalido{{Mensaje: "Una fecha u hora tiene un formato inválido."}}, true
	}

	// Decoder con DisallowUnknownFields: el error es un string plano.
	if strings.HasPrefix(err.Error(), "json: unknown field") {
		return []CampoInvalido{{Mensaje: "El cuerpo contiene un campo que no está permitido."}}, true
	}

	return nil, false
}

// nombreCampo devuelve la ruta del campo sin el nombre del struct raíz
// ("Entrada.items[0].cantidad" -> "items[0].cantidad"). Con
// ConfigurarValidador los segmentos usan el nombre del tag json.
func nombreCampo(fe validator.FieldError) string {
	ns := fe.Namespace()
	if i := strings.Index(ns, "."); i >= 0 {
		return ns[i+1:]
	}
	return ns
}

func mensajeValidacion(fe validator.FieldError) string {
	cadenaOColeccion := func() string {
		switch fe.Kind() {
		case reflect.String:
			return "caracteres"
		case reflect.Slice, reflect.Array, reflect.Map:
			return "elementos"
		}
		return ""
	}
	switch fe.Tag() {
	case "required", "required_if", "required_with", "required_without":
		return "Este campo es obligatorio."
	case "email":
		return "Debe ser un correo electrónico válido."
	case "oneof":
		return "Debe ser uno de: " + strings.ReplaceAll(fe.Param(), " ", ", ") + "."
	case "eqfield":
		return "No coincide con el campo " + fe.Param() + "."
	case "len":
		if u := cadenaOColeccion(); u != "" {
			return "Debe tener exactamente " + fe.Param() + " " + u + "."
		}
		return "Debe ser igual a " + fe.Param() + "."
	case "min", "gte":
		if u := cadenaOColeccion(); u != "" {
			return "Debe tener al menos " + fe.Param() + " " + u + "."
		}
		return "Debe ser mayor o igual a " + fe.Param() + "."
	case "max", "lte":
		if u := cadenaOColeccion(); u != "" {
			return "Debe tener como máximo " + fe.Param() + " " + u + "."
		}
		return "Debe ser menor o igual a " + fe.Param() + "."
	case "gt":
		if u := cadenaOColeccion(); u != "" {
			return "Debe tener más de " + fe.Param() + " " + u + "."
		}
		return "Debe ser mayor que " + fe.Param() + "."
	case "lt":
		if u := cadenaOColeccion(); u != "" {
			return "Debe tener menos de " + fe.Param() + " " + u + "."
		}
		return "Debe ser menor que " + fe.Param() + "."
	case "uuid", "uuid4":
		return "Debe ser un UUID válido."
	case "datetime":
		return "Debe tener el formato de fecha " + fe.Param() + "."
	}
	return "El valor no es válido."
}

func tipoEsperado(t reflect.Type) string {
	if t == nil {
		return "un valor compatible"
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "un número"
	case reflect.String:
		return "un texto"
	case reflect.Bool:
		return "verdadero o falso"
	case reflect.Slice, reflect.Array:
		return "una lista"
	case reflect.Struct, reflect.Map:
		return "un objeto"
	}
	return "un valor compatible"
}

// ConfigurarValidador hace que los errores de validación usen el nombre del
// tag json (p. ej. "fecha_ingreso") en vez del nombre del campo Go. Llamar
// una vez al arrancar, antes de servir tráfico (app.New lo hace).
func ConfigurarValidador() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		nombre := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if nombre == "-" {
			return ""
		}
		return nombre // "" => el validador usa el nombre del campo Go
	})
}
