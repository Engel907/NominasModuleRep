// Fase 6c — Endpoints de exportación bancaria y boleta de pago.
//
// AUDITORÍA DE PRODUCCIÓN: este handler asumía un paquete
// "planillas-ccdr/internal/api" con una función RespondWithError(c, status,
// err) de 3 argumentos que nunca existió en ningún paquete `api` real del
// proyecto (era una referencia colgante entre fases, como el propio
// comentario original ya admitía). El punto de entrada real y único para
// esto ya existe en middleware.RespondWithError(c, err) — ver
// internal/api/middleware/respond.go, que además ya lo documenta.
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/planilla"
)

// PlanillaExportHandler agrupa los endpoints de comprobantes/exportación
// de la Fase 6c. No conoce sqlc ni el detalle de acceso a datos: delega
// todo en planilla.ExportService (ver internal/planilla/export_service.go).
type PlanillaExportHandler struct {
	exportSvc *planilla.ExportService
}

func NewPlanillaExportHandler(exportSvc *planilla.ExportService) *PlanillaExportHandler {
	return &PlanillaExportHandler{exportSvc: exportSvc}
}

// RegisterRoutes registra los endpoints de esta fase bajo el grupo de
// rutas ya autenticado (AuthPASETO) del router principal.
//
// Permisos esperados (a resolver por middleware.RequirePermission, según
// el modelo de autorización de ADR-000 §3 — código de permiso, no rol):
//   - GET /planillas/:id/exportar/bancario -> "planilla.exportar_bancario"
//   - GET /planillas/:id/boleta/:empleado_id -> "planilla.ver_boleta"
//
// No se enforcan aquí adentro del handler: el middleware de la ruta debe
// aplicarlos antes de llegar a estas funciones, igual que el resto del
// sistema. Ver cmd/api/main.go para el wiring real de permisos por ruta.
func (h *PlanillaExportHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/planillas/:id/exportar/bancario", h.ExportarDispersionBancaria)
	router.GET("/planillas/:id/boleta/:empleado_id", h.ObtenerBoletaPago)
}

// ExportarDispersionBancaria maneja
// GET /planillas/:id/exportar/bancario
//
// Genera y entrega como descarga el archivo plano de dispersión bancaria
// del período (planilla) indicado: una línea por empleado con
// "IBAN,Cédula,Nombre,Monto". Solo incluye empleados de períodos
// 'calculado'/'cerrado' (ver ExportService.GenerarArchivoDispersionBancaria).
func (h *PlanillaExportHandler) ExportarDispersionBancaria(c *gin.Context) {
	periodoID, err := middleware.ParseUintParam(c, "id")
	if err != nil {
		// ParseUintParam ya respondió 400 y abortó el contexto.
		return
	}

	payload := middleware.PayloadDesdeContexto(c)
	if payload == nil {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "Token de autenticación requerido", nil)
		return
	}

	contenido, nombreArchivo, err := h.exportSvc.GenerarArchivoDispersionBancaria(c.Request.Context(), int64(periodoID))
	if err != nil {
		if errors.Is(err, planilla.ErrPlanillaSinFilasParaDispersion) {
			middleware.Fail(c, http.StatusUnprocessableEntity, "no_dispersion_rows", err.Error(), nil)
			return
		}
		middleware.RespondWithError(c, err)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\""+nombreArchivo+"\"")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", contenido)
}

// ObtenerBoletaPago maneja
// GET /planillas/:id/boleta/:empleado_id
//
// Devuelve la boleta individual de pago (encabezado + ingresos +
// deducciones + embargos aplicados) como JSON, para que el frontend la
// renderice o el propio cliente la convierta a PDF si lo necesita (fuera
// de alcance de esta fase).
func (h *PlanillaExportHandler) ObtenerBoletaPago(c *gin.Context) {
	periodoID, err := middleware.ParseUintParam(c, "id")
	if err != nil {
		return
	}
	empleadoID, err := middleware.ParseUintParam(c, "empleado_id")
	if err != nil {
		return
	}

	payload := middleware.PayloadDesdeContexto(c)
	if payload == nil {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "Token de autenticación requerido", nil)
		return
	}

	boleta, err := h.exportSvc.ObtenerBoletaPago(c.Request.Context(), int64(periodoID), int64(empleadoID))
	if err != nil {
		middleware.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, boleta)
}
