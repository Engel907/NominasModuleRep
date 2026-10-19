package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/pagination"
	"planillas-ccdr/internal/planilla"
)

// PlanillaPeriodoHandler expone el ciclo de vida del período (F19).
type PlanillaPeriodoHandler struct {
	svc *planilla.PeriodoService
}

func NewPlanillaPeriodoHandler(svc *planilla.PeriodoService) *PlanillaPeriodoHandler {
	return &PlanillaPeriodoHandler{svc: svc}
}

// RegisterRoutes monta las rutas sobre el grupo /planillas (ya protegido).
// No registra /:id/contabilizar (F28) ni /:id/exportar/* ni /:id/boleta/* (F09).
func (h *PlanillaPeriodoHandler) RegisterRoutes(g *gin.RouterGroup) {
	g.POST("", middleware.RequirePermission(authz.PermPlanillaCrear), h.Crear)
	g.GET("", middleware.RequirePermission(authz.PermPlanillaVer), h.Listar)
	g.GET("/:id", middleware.RequirePermission(authz.PermPlanillaVer), h.Obtener)
	g.POST("/:id/calcular", middleware.RequirePermission(authz.PermPlanillaCalcular), h.accion(h.svc.Calcular))
	g.POST("/:id/recalcular", middleware.RequirePermission(authz.PermPlanillaCalcular), h.accion(h.svc.Recalcular))
	g.POST("/:id/aprobar", middleware.RequirePermission(authz.PermPlanillaAprobar), h.accion(h.svc.Aprobar))
	g.POST("/:id/pagar", middleware.RequirePermission(authz.PermPlanillaPagar), h.accion(h.svc.Pagar))
}

type paso struct {
	UsuarioID *int64     `json:"usuario_id"`
	Fecha     *time.Time `json:"fecha"`
}

type periodoResponse struct {
	ID                 int64  `json:"id"`
	TipoPlanillaID     int64  `json:"tipo_planilla_id"`
	TipoPlanillaCodigo string `json:"tipo_planilla_codigo"`
	FechaInicio        string `json:"fecha_inicio"`
	FechaFin           string `json:"fecha_fin"`
	Estado             string `json:"estado"`
	VersionCalculo     int32  `json:"version_calculo"`
	Preparado          paso   `json:"preparado"`
	Calculado          paso   `json:"calculado"`
	Aprobado           paso   `json:"aprobado"`
	Pagado             paso   `json:"pagado"`
	Contabilizado      paso   `json:"contabilizado"`
}

func aRespuesta(p dto.PeriodoCicloRow) periodoResponse {
	return periodoResponse{
		ID: p.ID, TipoPlanillaID: p.TipoPlanillaID, TipoPlanillaCodigo: p.TipoPlanillaCodigo,
		FechaInicio: p.FechaInicio.Format("2006-01-02"), FechaFin: p.FechaFin.Format("2006-01-02"),
		Estado: p.Estado, VersionCalculo: p.VersionCalculo,
		Preparado:     paso{p.PreparadoPorUsuarioID, p.FechaPreparacion},
		Calculado:     paso{p.CalculadoPorUsuarioID, p.FechaCalculo},
		Aprobado:      paso{p.AprobadoPorUsuarioID, p.FechaAprobacion},
		Pagado:        paso{p.PagadoPorUsuarioID, p.FechaPago},
		Contabilizado: paso{p.ContabilizadoPorUsuarioID, p.FechaContabilizacion},
	}
}

// fallar escribe los errores de dominio del ciclo; el resto se delega al
// ErrorHandler global (c.Error).
func fallar(c *gin.Context, err error) {
	var ec *planilla.ErrorCiclo
	if errors.As(err, &ec) {
		c.AbortWithStatusJSON(ec.HTTP, gin.H{"error": gin.H{"codigo": ec.Codigo, "mensaje": ec.Mensaje}})
		return
	}
	_ = c.Error(err)
}

func solicitudInvalida(c *gin.Context, mensaje string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"codigo": "solicitud_invalida", "mensaje": mensaje}})
}

func usuarioActual(c *gin.Context) (int64, bool) {
	id, ok := middleware.UsuarioIDDesdeContexto(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"codigo": "no_autenticado", "mensaje": "se requiere autenticación"}})
	}
	return id, ok
}

type crearPeriodoRequest struct {
	TipoPlanillaID int64  `json:"tipo_planilla_id" binding:"required"`
	FechaInicio    string `json:"fecha_inicio" binding:"required"`
	FechaFin       string `json:"fecha_fin" binding:"required"`
}

func (h *PlanillaPeriodoHandler) Crear(c *gin.Context) {
	usuarioID, ok := usuarioActual(c)
	if !ok {
		return
	}
	var req crearPeriodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		solicitudInvalida(c, "cuerpo inválido: se requieren tipo_planilla_id, fecha_inicio y fecha_fin")
		return
	}
	inicio, err1 := time.Parse("2006-01-02", req.FechaInicio)
	fin, err2 := time.Parse("2006-01-02", req.FechaFin)
	if err1 != nil || err2 != nil {
		solicitudInvalida(c, "las fechas deben tener formato YYYY-MM-DD")
		return
	}
	p, err := h.svc.Crear(c.Request.Context(), usuarioID, req.TipoPlanillaID, inicio, fin)
	if err != nil {
		fallar(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": aRespuesta(p)})
}

func (h *PlanillaPeriodoHandler) Obtener(c *gin.Context) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		fallar(c, err)
		return
	}
	p, err := h.svc.Obtener(c.Request.Context(), id)
	if err != nil {
		fallar(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": aRespuesta(p)})
}

func (h *PlanillaPeriodoHandler) Listar(c *gin.Context) {
	var f dto.FiltroPeriodosCiclo
	if v := c.Query("tipo_planilla_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			solicitudInvalida(c, "tipo_planilla_id inválido")
			return
		}
		f.TipoPlanillaID = &id
	}
	if v := c.Query("estado"); v != "" {
		if !planilla.EstadoPeriodo(v).Valido() {
			solicitudInvalida(c, "estado inválido")
			return
		}
		f.Estado = v
	}
	for nombre, destino := range map[string]**time.Time{"desde": &f.Desde, "hasta": &f.Hasta} {
		if v := c.Query(nombre); v != "" {
			t, err := time.Parse("2006-01-02", v)
			if err != nil {
				solicitudInvalida(c, nombre+" debe tener formato YYYY-MM-DD")
				return
			}
			*destino = &t
		}
	}
	pag := pagination.Desde(c)
	filas, total, err := h.svc.Listar(c.Request.Context(), f, pag)
	if err != nil {
		fallar(c, err)
		return
	}
	datos := make([]periodoResponse, len(filas))
	for i, p := range filas {
		datos[i] = aRespuesta(p)
	}
	c.JSON(http.StatusOK, gin.H{"data": datos, "meta": pagination.NuevaMeta(pag, total)})
}

// accion adapta una operación del servicio (usuario, período) a un handler.
func (h *PlanillaPeriodoHandler) accion(
	op func(ctx context.Context, usuarioID, periodoID int64) (dto.PeriodoCicloRow, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuarioID, ok := usuarioActual(c)
		if !ok {
			return
		}
		id, err := middleware.ParseIDParam(c, "id")
		if err != nil {
			fallar(c, err)
			return
		}
		p, err := op(c.Request.Context(), usuarioID, id)
		if err != nil {
			fallar(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": aRespuesta(p)})
	}
}