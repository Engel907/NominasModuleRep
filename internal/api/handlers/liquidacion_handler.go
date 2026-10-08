// Package handlers — Fase 7: endpoints HTTP de Liquidaciones Laborales.
//
// Convención heredada: los handlers son delgados — parsean/validan input,
// resuelven el usuario autenticado (para auditoría), llaman al servicio, y
// traducen errores de dominio a códigos HTTP. Toda la lógica de negocio
// vive en internal/services/liquidacion_service.go.
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/services"
)

// LiquidacionHandler expone los endpoints de proyección y procesamiento de
// liquidaciones.
type LiquidacionHandler struct {
	service *services.LiquidacionService
}

func NewLiquidacionHandler(service *services.LiquidacionService) *LiquidacionHandler {
	return &LiquidacionHandler{service: service}
}

// liquidacionRequestBody es el body compartido por /proyectar y /procesar.
//
// AplicaResponsabilidadPatronal es OBLIGATORIO (no tiene default: un
// *bool para poder distinguir "no lo mandaron" de "lo mandaron en false",
// consistente con la directiva de "sin valores por defecto implícitos" del
// resto del proyecto — decidir si aplica preaviso/cesantía es una decisión
// de RRHH/Legal que este endpoint no debe adivinar).
type liquidacionRequestBody struct {
	FechaSalida                   string `json:"fecha_salida" binding:"required"` // formato YYYY-MM-DD
	MotivoSalida                  string `json:"motivo_salida" binding:"required,oneof=despido_sin_responsabilidad despido_con_responsabilidad despido_indirecto renuncia vencimiento_contrato mutuo_acuerdo fallecimiento"`
	AplicaResponsabilidadPatronal *bool  `json:"aplica_responsabilidad_patronal" binding:"required"`
}

func (h *LiquidacionHandler) parseSolicitud(c *gin.Context) (services.ProyectarSolicitud, bool) {
	empleadoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || empleadoID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de empleado inválido"})
		return services.ProyectarSolicitud{}, false
	}

	var body liquidacionRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cuerpo de la solicitud inválido: " + err.Error()})
		return services.ProyectarSolicitud{}, false
	}

	fechaSalida, err := time.Parse("2006-01-02", body.FechaSalida)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fecha_salida debe tener formato YYYY-MM-DD"})
		return services.ProyectarSolicitud{}, false
	}

	return services.ProyectarSolicitud{
		EmpleadoID:                     empleadoID,
		FechaSalida:                    fechaSalida,
		MotivoSalida:                   body.MotivoSalida,
		AplicaResponsabilidadPatronal:  *body.AplicaResponsabilidadPatronal,
	}, true
}

// Proyectar — POST /empleados/:id/liquidaciones/proyectar
// Calcula la liquidación completa SIN alterar ningún estado (no cierra
// contrato, no desactiva expediente, no persiste fila en `liquidaciones`).
// Pensado para que RRHH simule distintas fechas de salida antes de
// confirmar.
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

// Procesar — POST /empleados/:id/liquidaciones/procesar
// Recalcula (no confía en un cálculo previo del cliente), persiste la
// liquidación con estado='procesada', cierra el contrato/datos_laborales
// vigentes y desactiva el expediente del empleado — todo dentro de una
// única transacción auditada (WithAuditContext). Requiere el usuario
// autenticado en el contexto de Gin (ver middleware.AuthPASETO /
// middleware.UsuarioIDDesdeContexto) para la trazabilidad de
// auditoria_cambios.
func (h *LiquidacionHandler) Procesar(c *gin.Context) {
	sol, ok := h.parseSolicitud(c)
	if !ok {
		return
	}

	usuarioID, ok := middleware.UsuarioIDDesdeContexto(c)
	if !ok {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "No se pudo determinar el usuario autenticado para auditoría", nil)
		return
	}

	resultado, liquidacionID, err := h.service.ProcesarLiquidacion(c.Request.Context(), sol, int64(usuarioID))
	if err != nil {
		h.responderError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"liquidacion_id": liquidacionID,
		"empleado_id":    sol.EmpleadoID,
		"estado":         "procesada",
		"resultado":      resultado,
	})
}

func (h *LiquidacionHandler) responderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrSinDatosLaborales):
		middleware.Fail(c, http.StatusUnprocessableEntity, "sin_datos_laborales", err.Error(), nil)
	default:
		// No se distingue más finamente sin conocer el resto de la capa de
		// errores del proyecto (ej. si hay un paquete apperrors con
		// códigos); se deja como 422 genérico de negocio en vez de 500,
		// porque las fuentes de error de CalcularLiquidacion son todas de
		// validación/datos faltantes, no de infraestructura — TODO:
		// revisar contra la convención real de manejo de errores del
		// proyecto (fuera del material fuente entregado).
		middleware.Fail(c, http.StatusUnprocessableEntity, "unprocessable_entity", err.Error(), nil)
	}
}

// RegisterRoutes registra los endpoints de este handler en el grupo de
// rutas de empleados. Ejemplo de uso en el wiring de cmd/api:
//
//	empleados := router.Group("/empleados")
//	liqHandler.RegisterRoutes(empleados)
func (h *LiquidacionHandler) RegisterRoutes(empleados *gin.RouterGroup) {
	empleados.POST("/:id/liquidaciones/proyectar", h.Proyectar)
	empleados.POST("/:id/liquidaciones/procesar", h.Procesar)
}
