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
	s := strings.TrimLeft(b.String(), ".")
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
		return
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
	sol, ok := h.solicitudBoleta(c)
	if !ok {
		return
	}
	boleta, err := h.exportSvc.ObtenerBoletaPagoAutorizada(c.Request.Context(), sol.periodoID, sol.empleadoID, sol.empleadoUsuarioID, sol.permisos)
	if err != nil {
		fallarBoleta(c, err)
		return
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, boleta)
}

// ObtenerBoletaPagoPDF maneja GET /planillas/:id/boleta/:empleado_id/pdf:
// misma autorización por fila que la boleta JSON.
func (h *PlanillaExportHandler) ObtenerBoletaPagoPDF(c *gin.Context) {
	sol, ok := h.solicitudBoleta(c)
	if !ok {
		return
	}
	pdf, err := h.exportSvc.ObtenerBoletaPagoPDFAutorizada(c.Request.Context(), sol.periodoID, sol.empleadoID, sol.empleadoUsuarioID, sol.permisos)
	if err != nil {
		fallarBoleta(c, err)
		return
	}
	nombre := fmt.Sprintf("boleta_periodo_%d_empleado_%d.pdf", sol.periodoID, sol.empleadoID)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": nombreArchivoSeguro(nombre)}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/pdf", pdf)
}

type solicitudBoleta struct {
	periodoID, empleadoID, empleadoUsuarioID int64
	permisos                                 []string
}

// solicitudBoleta resuelve período, empleado, permisos y (si solo tiene
// ver_boleta_propia) el empleado vinculado al usuario.
func (h *PlanillaExportHandler) solicitudBoleta(c *gin.Context) (solicitudBoleta, bool) {
	var sol solicitudBoleta
	var ok bool
	if sol.periodoID, ok = parseIDParam(c, "id"); !ok {
		return sol, false
	}
	if sol.empleadoID, ok = parseIDParam(c, "empleado_id"); !ok {
		return sol, false
	}
	if _, ok := usuarioIDDesdeContexto(c); !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "Token de autenticación requerido", nil)
		return sol, false
	}
	payload := middleware.PayloadDesdeContexto(c)
	if payload != nil && !tienePermiso(payload.Permisos, authz.PermPlanillaVerBoleta) &&
		tienePermiso(payload.Permisos, authz.PermPlanillaVerBoletaPropia) {
		id, err := middleware.EmpleadoIDDelUsuario(c, h.empleados)
		if err != nil {
			if errors.Is(err, middleware.ErrEmpleadoIDNoVinculado) {
				middleware.Fail(c, http.StatusUnprocessableEntity, "employee_not_linked", "El usuario no tiene un empleado vinculado.", nil)
			} else {
				middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "No se pudo resolver el empleado del usuario.", err)
			}
			return sol, false
		}
		sol.empleadoUsuarioID = id
	}
	if payload != nil {
		sol.permisos = payload.Permisos
	}
	return sol, true
}

func fallarBoleta(c *gin.Context, err error) {
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

func tienePermiso(permisos []string, buscado string) bool {
	for _, permiso := range permisos {
		if permiso == buscado {
			return true
		}
	}
	return false
}
