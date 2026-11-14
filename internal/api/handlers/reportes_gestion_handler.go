package handlers

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/planilla"
)

// ReportesGestionHandler: reportes internos (JSON por defecto, CSV con
// ?formato=csv) y los reportes oficiales consolidados por mes.
type ReportesGestionHandler struct {
	gestion   *planilla.ReportesGestionService
	oficiales *planilla.ReportesService
}

func NewReportesGestionHandler(g *planilla.ReportesGestionService, o *planilla.ReportesService) *ReportesGestionHandler {
	return &ReportesGestionHandler{gestion: g, oficiales: o}
}

func (h *ReportesGestionHandler) RegisterRoutes(rg *gin.RouterGroup) {
	r := rg.Group("/reportes")
	gestion := middleware.RequirePermission(authz.PermReportesGestion)
	r.GET("/periodos/:id/conceptos", gestion, h.porPeriodo(h.gestion.ResumenConceptos))
	r.GET("/periodos/:id/costo-programas", gestion, h.porPeriodo(h.gestion.CostoPorPrograma))
	r.GET("/acumulado-empleados", gestion, h.porRango(h.gestion.AcumuladoEmpleados))
	r.GET("/retenciones-terceros", gestion, h.porRango(h.gestion.RetencionesTerceros))
	r.GET("/vacaciones/saldos", gestion, h.saldosVacaciones)

	r.GET("/mensual/ccss", middleware.RequirePermission(authz.PermReportesCcss), h.oficialMensual(planilla.ReporteCCSS))
	r.GET("/mensual/ins", middleware.RequirePermission(authz.PermReportesIns), h.oficialMensual(planilla.ReporteINS))
	r.GET("/mensual/hacienda", middleware.RequirePermission(authz.PermReportesHacienda), h.oficialMensual(planilla.ReporteHacienda))
}

func (h *ReportesGestionHandler) porPeriodo(f func(context.Context, int64) (planilla.TablaReporte, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseIDParam(c, "id")
		if !ok {
			return
		}
		t, err := f(c.Request.Context(), id)
		h.responder(c, t, err)
	}
}

// porRango: ?anio=AAAA y, opcional, ?mes=1..12 (sin mes = el año completo).
func (h *ReportesGestionHandler) porRango(f func(context.Context, int, int) (planilla.TablaReporte, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		anio, mes, ok := anioMes(c, false)
		if !ok {
			return
		}
		t, err := f(c.Request.Context(), anio, mes)
		h.responder(c, t, err)
	}
}

func (h *ReportesGestionHandler) saldosVacaciones(c *gin.Context) {
	fecha := time.Now().UTC()
	if v := c.Query("fecha"); v != "" {
		f, err := time.Parse("2006-01-02", v)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, "validation_error", "fecha inválida, se espera AAAA-MM-DD", gin.H{"parametro": "fecha"})
			return
		}
		fecha = f
	}
	t, err := h.gestion.SaldosVacaciones(c.Request.Context(), fecha)
	h.responder(c, t, err)
}

func (h *ReportesGestionHandler) oficialMensual(reporte planilla.ReporteOficial) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuarioID, ok := usuarioIDDesdeContexto(c)
		if !ok {
			middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "Token de autenticación requerido", nil)
			return
		}
		anio, mes, ok := anioMes(c, true)
		if !ok {
			return
		}
		a, err := h.oficiales.GenerarMensual(c.Request.Context(), anio, mes, reporte, c.Query("formato"), usuarioID)
		if err != nil {
			responderErrorReporte(c, err)
			return
		}
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": nombreArchivoSeguro(a.NombreArchivo)}))
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-SHA256", a.HashSHA256)
		c.Data(http.StatusOK, "text/csv; charset=utf-8", a.Contenido)
	}
}

func (h *ReportesGestionHandler) responder(c *gin.Context, t planilla.TablaReporte, err error) {
	if err != nil {
		responderErrorReporte(c, err)
		return
	}
	if c.Query("formato") == "csv" {
		contenido, err := t.CSV()
		if err != nil {
			middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "Error interno del servidor", err)
			return
		}
		nombre := fmt.Sprintf("%s_%s.csv", t.Reporte, time.Now().UTC().Format("20060102_150405"))
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": nombreArchivoSeguro(nombre)}))
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/csv; charset=utf-8", contenido)
		return
	}
	c.JSON(http.StatusOK, gin.H{"reporte": t.Reporte, "columnas": t.Columnas, "filas": t.Objetos()})
}

func responderErrorReporte(c *gin.Context, err error) {
	var ce *planilla.ErrorCiclo
	if errors.As(err, &ce) {
		middleware.Fail(c, ce.HTTP, ce.Codigo, ce.Mensaje, nil)
		return
	}
	middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "Error interno del servidor", err)
}

// anioMes lee ?anio y ?mes; si mesObligatorio es falso, mes ausente = 0.
func anioMes(c *gin.Context, mesObligatorio bool) (int, int, bool) {
	anio, err := strconv.Atoi(c.Query("anio"))
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "validation_error", "anio es obligatorio y numérico", gin.H{"parametro": "anio"})
		return 0, 0, false
	}
	mes := 0
	if v := c.Query("mes"); v != "" || mesObligatorio {
		if mes, err = strconv.Atoi(v); err != nil {
			middleware.Fail(c, http.StatusBadRequest, "validation_error", "mes debe ser numérico (1-12)", gin.H{"parametro": "mes"})
			return 0, 0, false
		}
	}
	return anio, mes, true
}
