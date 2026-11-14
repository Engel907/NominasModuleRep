package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/liquidacion"
	"planillas-ccdr/internal/rrhh"
)

// LiquidacionHandler expone los endpoints de proyección y procesamiento de
// liquidaciones.
type LiquidacionHandler struct {
	service *rrhh.LiquidacionService
}

func NewLiquidacionHandler(service *rrhh.LiquidacionService) *LiquidacionHandler {
	return &LiquidacionHandler{service: service}
}

// liquidacionRequestBody es el body compartido por /proyectar y /procesar.
type liquidacionRequestBody struct {
	FechaSalida                   string `json:"fecha_salida" binding:"required"` // formato YYYY-MM-DD
	MotivoSalida                  string `json:"motivo_salida" binding:"required,oneof=despido_sin_responsabilidad despido_con_responsabilidad despido_indirecto renuncia vencimiento_contrato mutuo_acuerdo fallecimiento"`
	AplicaResponsabilidadPatronal *bool  `json:"aplica_responsabilidad_patronal" binding:"required"`
}

func (h *LiquidacionHandler) parseSolicitud(c *gin.Context) (rrhh.ProyectarSolicitud, bool) {
	empleadoID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return rrhh.ProyectarSolicitud{}, false
	}

	var body liquidacionRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		middleware.RespondWithError(c, err)
		return rrhh.ProyectarSolicitud{}, false
	}

	fechaSalida, err := time.Parse("2006-01-02", body.FechaSalida)
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "fecha_invalida", "fecha_salida debe tener formato YYYY-MM-DD", nil)
		return rrhh.ProyectarSolicitud{}, false
	}

	return rrhh.ProyectarSolicitud{
		EmpleadoID:                    empleadoID,
		FechaSalida:                   fechaSalida,
		MotivoSalida:                  body.MotivoSalida,
		AplicaResponsabilidadPatronal: *body.AplicaResponsabilidadPatronal,
	}, true
}

// Proyectar calcula la liquidación completa sin alterar ningún estado: no
// cierra el contrato, no desactiva el expediente ni persiste la liquidación.
func (h *LiquidacionHandler) Proyectar(c *gin.Context) {
	sol, ok := h.parseSolicitud(c)
	if !ok {
		return
	}

	resultado, err := h.service.ProyectarLiquidacion(c.Request.Context(), sol)
	if err != nil {
		h.responderError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"empleado_id": sol.EmpleadoID,
		"estado":      "proyectada",
		"resultado":   resultado,
	})
}

// Procesar recalcula la liquidación, la persiste como procesada, cierra el
// contrato y los datos laborales vigentes y desactiva el expediente, todo en
// una única transacción auditada.
func (h *LiquidacionHandler) Procesar(c *gin.Context) {
	sol, ok := h.parseSolicitud(c)
	if !ok {
		return
	}

	usuarioID := middleware.UsuarioID(c)
	if usuarioID <= 0 {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "No se pudo determinar el usuario autenticado para auditoría", nil)
		return
	}

	proc, err := h.service.ProcesarLiquidacion(c.Request.Context(), sol, usuarioID)
	if err != nil {
		h.responderError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"liquidacion_id":      proc.LiquidacionID,
		"empleado_id":         sol.EmpleadoID,
		"estado":              "procesada",
		"periodo_planilla_id": proc.PeriodoPlanillaID,
		"cierre_programado":   proc.CierreProgramado,
		"resultado":           proc.Rubros,
	})
}

func (h *LiquidacionHandler) responderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, rrhh.ErrSinDatosLaborales):
		middleware.Fail(c, http.StatusUnprocessableEntity, "sin_datos_laborales", err.Error(), nil)
		return
	case errors.Is(err, rrhh.ErrMotivoSinRegla):
		middleware.Fail(c, http.StatusUnprocessableEntity, "motivo_sin_regla", err.Error(), nil)
		return
	case errors.Is(err, rrhh.ErrParametroLiquidacion):
		middleware.Fail(c, http.StatusUnprocessableEntity, "parametros_incompletos", err.Error(), nil)
		return
	case errors.Is(err, liquidacion.ErrResponsabilidadNoAdmitida):
		middleware.Fail(c, http.StatusUnprocessableEntity, "responsabilidad_no_admitida", err.Error(), nil)
		return
	case errors.Is(err, liquidacion.ErrSinPeriodosParaPromedio):
		middleware.Fail(c, http.StatusUnprocessableEntity, "sin_periodos_para_promedio", err.Error(), nil)
		return
	case errors.Is(err, rrhh.ErrEmpleadoInactivo):
		middleware.Fail(c, http.StatusConflict, "empleado_inactivo", err.Error(), nil)
		return
	case errors.Is(err, rrhh.ErrLiquidacionYaProcesada):
		middleware.Fail(c, http.StatusConflict, "liquidacion_ya_procesada", err.Error(), nil)
		return
	case errors.Is(err, rrhh.ErrPeriodoLiquidacionCerrado):
		middleware.Fail(c, http.StatusConflict, "periodo_liquidacion_cerrado", err.Error(), nil)
		return
	case errors.Is(err, liquidacion.ErrFechasInvalidas):
		middleware.Fail(c, http.StatusBadRequest, "fechas_invalidas", err.Error(), nil)
		return
	}
	middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "Error interno del servidor", err)
}
