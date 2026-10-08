// api/handlers/rrhh_errores.go
//
// F03 — Traducción única de errores de los servicios de RRHH (empleados,
// contratos, organización) a respuestas HTTP.
//
// Reglas:
//   - Los errores de negocio tipados (services/rrhh_errores.go) salen con un
//     status y un mensaje REDACTADO aquí, y solo con detalle estructurado
//     construido a partir de campos tipados (nunca err.Error()).
//   - Todo lo demás (errores técnicos / de BD) sale por
//     middleware.FailFromError con el status/código de respaldo que pase el
//     handler; el error interno nunca se vuelca con c.JSON.
//   - Cada función responde UNA sola vez; el handler solo hace `return`.
package handlers

import (
	"errors"
	"net/http"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// responderErrorBind responde 400 ante un fallo de ShouldBindJSON/ShouldBindQuery.
// Para errores de validación expone solo (campo, regla) — nombres de campo y
// de regla, no el texto crudo del validador ni de encoding/json. Para
// cualquier otro error de bind (JSON mal formado, tipo incorrecto) responde
// un mensaje fijo.
func responderErrorBind(c *gin.Context, mensaje string, err error) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		campos := make([]gin.H, 0, len(ve))
		for _, fe := range ve {
			campos = append(campos, gin.H{"campo": fe.Field(), "regla": fe.Tag()})
		}
		middleware.Fail(c, http.StatusBadRequest, "validation_error", mensaje, gin.H{"campos": campos})
		return
	}
	middleware.Fail(c, http.StatusBadRequest, "validation_error", mensaje, nil)
}

// responderErrorRRHH traduce un error del servicio. (status, code, msg) son el
// respaldo para errores no tipados, que salen por middleware.FailFromError.
func responderErrorRRHH(c *gin.Context, err error, status int, code, msg string) {
	var (
		errVal *services.ErrValidacion
		errSal *services.SalarioInferiorError
		errSNP *services.SalarioMinimoNoParametrizadoError
		errVig *services.VigenciaTraslapadaError
	)

	switch {
	// --- 400: entrada inválida
	case errors.As(err, &errVal):
		middleware.Fail(c, http.StatusBadRequest, "validation_error", "datos inválidos",
			gin.H{"campo": errVal.Campo, "detalle": errVal.Mensaje})
	case errors.As(err, &errSal):
		middleware.Fail(c, http.StatusBadRequest, "validation_error", "salario pactado inferior al salario mínimo vigente",
			gin.H{
				"puesto_id":       errSal.PuestoID,
				"salario_pactado": errSal.Pactado.String(),
				"salario_minimo":  errSal.Minimo.String(),
			})

	// --- 404
	case errors.Is(err, services.ErrEmpleadoNoEncontrado):
		middleware.Fail(c, http.StatusNotFound, "not_found", "empleado no encontrado", nil)
	case errors.Is(err, services.ErrContratoNoEncontrado):
		middleware.Fail(c, http.StatusNotFound, "not_found", "el empleado no tiene contrato vigente", nil)
	case errors.Is(err, services.ErrFamiliarNoEncontrado):
		middleware.Fail(c, http.StatusNotFound, "not_found", "declaración familiar vigente no encontrada", nil)
	case errors.Is(err, services.ErrNoEncontrado):
		middleware.Fail(c, http.StatusNotFound, "not_found", "recurso no encontrado", nil)

	// --- 409: conflicto con el estado actual
	case errors.As(err, &errVig):
		middleware.Fail(c, http.StatusConflict, "conflict", "la fecha efectiva traslapa o precede la vigencia actual",
			gin.H{
				"campo":                  errVig.Campo,
				"fecha_efectiva":         errVig.FechaEfectiva.Format("2006-01-02"),
				"inicio_vigencia_actual": errVig.InicioVigencia.Format("2006-01-02"),
			})
	case errors.Is(err, services.ErrContratoVigenteExiste):
		middleware.Fail(c, http.StatusConflict, "conflict", "el empleado ya tiene un contrato vigente; use actualizar contrato", nil)
	case errors.Is(err, services.ErrIdentificacionDuplicada):
		middleware.Fail(c, http.StatusConflict, "conflict", "ya existe un empleado con ese tipo y número de identificación", nil)
	case errors.Is(err, services.ErrCodigoInternoDuplicado):
		middleware.Fail(c, http.StatusConflict, "conflict", "ya existe un empleado con ese código interno", nil)
	case errors.Is(err, services.ErrRegistroDuplicado):
		middleware.Fail(c, http.StatusConflict, "conflict", "ya existe un registro con ese código", nil)

	// --- 422: la solicitud es válida pero no procesable con los datos actuales
	case errors.As(err, &errSNP):
		// D8: sin salario mínimo parametrizado NO se acepta el salario ni se
		// asume un valor por defecto; el cliente puede distinguir este caso.
		det := gin.H{"puesto_id": errSNP.PuestoID}
		if errSNP.ClasificacionPuestoID != nil {
			det["clasificacion_puesto_id"] = *errSNP.ClasificacionPuestoID
		}
		middleware.Fail(c, http.StatusUnprocessableEntity, "salario_minimo_no_parametrizado",
			"no hay salario mínimo vigente parametrizado para la clasificación del puesto", det)
	case errors.Is(err, services.ErrPuestoNoEncontrado):
		middleware.Fail(c, http.StatusUnprocessableEntity, "unprocessable_entity", "el puesto indicado no existe", nil)
	case errors.Is(err, services.ErrPuestoInactivo):
		middleware.Fail(c, http.StatusUnprocessableEntity, "unprocessable_entity", "el puesto indicado está inactivo", nil)
	case errors.Is(err, services.ErrReferenciaInvalida):
		middleware.Fail(c, http.StatusUnprocessableEntity, "unprocessable_entity", "una referencia indicada no existe (puesto, banco, tipo de contrato, régimen, etc.)", nil)
	case errors.Is(err, services.ErrJerarquiaCiclica):
		middleware.Fail(c, http.StatusUnprocessableEntity, "unprocessable_entity", "la unidad padre indicada generaría un ciclo en la jerarquía", nil)

	default:
		middleware.FailFromError(c, status, code, msg, err)
	}
}