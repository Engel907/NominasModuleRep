package handlers

import (
	"errors"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/planilla"
)

type ReportesHandler struct{ svc *planilla.ReportesService }

func NewReportesHandler(svc *planilla.ReportesService) *ReportesHandler {
	return &ReportesHandler{svc: svc}
}

func (h *ReportesHandler) generar(reporte planilla.ReporteOficial) gin.HandlerFunc {
	return func(c *gin.Context) {
		periodoID, ok := parseIDParam(c, "id")
		if !ok {
			return
		}
		usuarioID, ok := usuarioIDDesdeContexto(c)
		if !ok {
			middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "Token de autenticación requerido", nil)
			return
		}
		a, err := h.svc.Generar(c.Request.Context(), periodoID, reporte, c.Query("formato"), usuarioID)
		if err != nil {
			var ce *planilla.ErrorCiclo
			if errors.As(err, &ce) {
				middleware.Fail(c, ce.HTTP, ce.Codigo, ce.Mensaje, nil)
				return
			}
			middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "Error interno del servidor", err)
			return
		}
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": nombreArchivoSeguro(a.NombreArchivo)}))
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-SHA256", a.HashSHA256)
		c.Data(http.StatusOK, "text/csv; charset=utf-8", a.Contenido)
	}
}
