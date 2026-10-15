// Fase 6c / F09 — Endpoints de exportación bancaria y boleta de pago.
//
// Las rutas NO se registran aquí: routes.go las monta con el middleware de
// permisos (planilla.exportar_bancario y planilla.ver_boleta):
//
//	GET /planillas/:id/exportar/bancario   -> ExportarDispersionBancaria
//	GET /planillas/:id/boleta/:empleado_id -> ObtenerBoletaPago
package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/planilla"
)

// PlanillaExportHandler agrupa los endpoints de comprobantes/exportación.
// Delega todo en planilla.ExportService.
type PlanillaExportHandler struct {
	exportSvc *planilla.ExportService
	empleados middleware.EmpleadoIDResolver
}

func NewPlanillaExportHandler(exportSvc *planilla.ExportService, resolver ...middleware.EmpleadoIDResolver) *PlanillaExportHandler {
	h := &PlanillaExportHandler{exportSvc: exportSvc}
	if len(resolver) > 0 {
		h.empleados = resolver[0]
	}
	return h
}

// nombreArchivoSeguro deja solo [A-Za-z0-9._-]; cualquier otro carácter
// (comillas, CR/LF, separadores de ruta, espacios, no ASCII) pasa a '_'.
func nombreArchivoSeguro(nombre string) string {
	var b strings.Builder
	for _, r := range nombre {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := strings.TrimLeft(b.String(), ".") // sin archivos ocultos ni ".."
	if s == "" {
		return "exportacion.txt"
	}
	return s
}

// ExportarDispersionBancaria maneja GET /planillas/:id/exportar/bancario.
// Genera y entrega como descarga el TXT de dispersión (aprobada|pagada) y
// deja registro en exportaciones_generadas.
func (h *PlanillaExportHandler) ExportarDispersionBancaria(c *gin.Context) {
	periodoID, ok := parseIDParam(c, "id")
	if !ok {
		return // ParseIDParam ya respondió 400
	}
	usuarioID, ok := usuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "Token de autenticación requerido", nil)
		return
	}

	contenido, nombreArchivo, err := h.exportSvc.GenerarArchivoDispersionBancaria(c.Request.Context(), periodoID, usuarioID)
	if err != nil {
		var filaErr *planilla.FilaDispersionError
		switch {
		case errors.Is(err, planilla.ErrPlanillaSinFilasParaDispersion):
			middleware.Fail(c, http.StatusUnprocessableEntity, "no_dispersion_rows",
				"El período no tiene filas para dispersar (debe estar aprobado o pagado y con cuenta bancaria vigente)", nil)
		case errors.As(err, &filaErr):
			// EmpleadoID y Motivo son valores controlados, no texto de error interno.
			middleware.Fail(c, http.StatusUnprocessableEntity, "invalid_dispersion_row",
				fmt.Sprintf("No se puede generar el archivo: empleado %d: %s", filaErr.EmpleadoID, filaErr.Motivo), nil)
		default:
			middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "Error interno del servidor", err)
		}
		return
	}

	disposicion := mime.FormatMediaType("attachment", map[string]string{"filename": nombreArchivoSeguro(nombreArchivo)})
	c.Header("Content-Disposition", disposicion)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", contenido)
}

// ObtenerBoletaPago maneja GET /planillas/:id/boleta/:empleado_id.
// Devuelve la boleta individual como JSON (aprobada|pagada|contabilizada).
func (h *PlanillaExportHandler) ObtenerBoletaPago(c *gin.Context) {
	periodoID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	empleadoID, ok := parseIDParam(c, "empleado_id")
	if !ok {
		return
	}
	if _, ok := usuarioIDDesdeContexto(c); !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "Token de autenticación requerido", nil)
		return
	}
	payload := middleware.PayloadDesdeContexto(c)
	var empleadoUsuarioID int64
	var err error
	if payload != nil && !tienePermiso(payload.Permisos, authz.PermPlanillaVerBoleta) &&
		tienePermiso(payload.Permisos, authz.PermPlanillaVerBoletaPropia) {
		empleadoUsuarioID, err = middleware.EmpleadoIDDelUsuario(c, h.empleados)
		if err != nil {
			if errors.Is(err, middleware.ErrEmpleadoIDNoVinculado) {
				middleware.Fail(c, http.StatusUnprocessableEntity, "employee_not_linked", "El usuario no tiene un empleado vinculado.", nil)
			} else {
				middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "No se pudo resolver el empleado del usuario.", err)
			}
			return
		}
	}
	permisos := []string(nil)
	if payload != nil {
		permisos = payload.Permisos
	}
	boleta, err := h.exportSvc.ObtenerBoletaPagoAutorizada(c.Request.Context(), periodoID, empleadoID, empleadoUsuarioID, permisos)
	if err != nil {
		switch {
		case errors.Is(err, planilla.ErrBoletaSinPermiso), errors.Is(err, planilla.ErrBoletaNoPropia):
			middleware.Fail(c, http.StatusForbidden, "row_access_denied", "No tiene permiso para consultar esta boleta.", nil)
			return
		case errors.Is(err, planilla.ErrBoletaSinEmpleadoVinculado):
			middleware.Fail(c, http.StatusUnprocessableEntity, "employee_not_linked", "El usuario no tiene un empleado vinculado.", nil)
			return
		case errors.Is(err, sql.ErrNoRows):
			middleware.Fail(c, http.StatusNotFound, "not_found",
				"No existe boleta para ese empleado en el período, o el período aún no está aprobado", nil)
			return
		default:
			middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "Error interno del servidor", err)
			return
		}
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, boleta)
}

func tienePermiso(permisos []string, buscado string) bool {
	for _, permiso := range permisos {
		if permiso == buscado {
			return true
		}
	}
	return false
}
