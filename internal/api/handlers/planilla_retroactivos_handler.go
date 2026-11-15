package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/paginacion"
	"planillas-ccdr/internal/planilla"
)

// PlanillaRetroactivosHandler expone los retroactivos que paga un período.
type PlanillaRetroactivosHandler struct {
	svc *planilla.RetroactivosService
}

func NewPlanillaRetroactivosHandler(svc *planilla.RetroactivosService) *PlanillaRetroactivosHandler {
	return &PlanillaRetroactivosHandler{svc: svc}
}

// RegisterRoutes monta /planillas/:id/retroactivos sobre el grupo /planillas.
func (h *PlanillaRetroactivosHandler) RegisterRoutes(g *gin.RouterGroup) {
	g.POST("/:id/retroactivos", middleware.RequirePermission(authz.PermPlanillaCrear), h.Crear)
	g.GET("/:id/retroactivos", middleware.RequirePermission(authz.PermPlanillaVer), h.Listar)
	g.DELETE("/:id/retroactivos/:retroactivo_id", middleware.RequirePermission(authz.PermPlanillaCrear), h.Eliminar)
}

type retroactivoResponse struct {
	ID                int64     `json:"id"`
	PeriodoPlanillaID int64     `json:"periodo_planilla_id"`
	EmpleadoID        int64     `json:"empleado_id"`
	Desde             string    `json:"desde"`
	Hasta             string    `json:"hasta"`
	Motivo            string    `json:"motivo"`
	CreadoPor         int64     `json:"creado_por"`
	CreatedAt         time.Time `json:"created_at"`
}

func aRetroactivoResponse(r dto.RetroactivoRow) retroactivoResponse {
	return retroactivoResponse{ID: r.ID, PeriodoPlanillaID: r.PeriodoPlanillaID, EmpleadoID: r.EmpleadoID,
		Desde: r.Desde.Format("2006-01-02"), Hasta: r.Hasta.Format("2006-01-02"), Motivo: r.Motivo,
		CreadoPor: r.CreadoPor, CreatedAt: r.CreatedAt}
}

type crearRetroactivoRequest struct {
	EmpleadoID int64  `json:"empleado_id" binding:"required"`
	Desde      string `json:"desde" binding:"required"` // YYYY-MM-DD
	Hasta      string `json:"hasta" binding:"required"` // YYYY-MM-DD
	Motivo     string `json:"motivo" binding:"required"`
}

func (h *PlanillaRetroactivosHandler) Crear(c *gin.Context) {
	usuarioID, ok := usuarioActual(c)
	if !ok {
		return
	}
	periodoID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		fallar(c, err)
		return
	}
	var req crearRetroactivoRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.EmpleadoID <= 0 {
		solicitudInvalida(c, "cuerpo inválido: se requieren empleado_id, desde, hasta (YYYY-MM-DD) y motivo")
		return
	}
	desde, err1 := time.Parse("2006-01-02", req.Desde)
	hasta, err2 := time.Parse("2006-01-02", req.Hasta)
	if err1 != nil || err2 != nil {
		solicitudInvalida(c, "desde y hasta deben ser fechas YYYY-MM-DD")
		return
	}
	r, err := h.svc.Crear(c.Request.Context(), usuarioID, periodoID, planilla.NuevoRetroactivo{
		EmpleadoID: req.EmpleadoID, Desde: desde, Hasta: hasta, Motivo: req.Motivo,
	})
	if err != nil {
		fallar(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": aRetroactivoResponse(r)})
}

func (h *PlanillaRetroactivosHandler) Listar(c *gin.Context) {
	periodoID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		fallar(c, err)
		return
	}
	var empleadoID *int64
	if v := c.Query("empleado_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			solicitudInvalida(c, "empleado_id inválido")
			return
		}
		empleadoID = &id
	}
	pag := paginacion.Desde(c)
	filas, total, err := h.svc.Listar(c.Request.Context(), periodoID, empleadoID, pag)
	if err != nil {
		fallar(c, err)
		return
	}
	datos := make([]retroactivoResponse, len(filas))
	for i, r := range filas {
		datos[i] = aRetroactivoResponse(r)
	}
	c.JSON(http.StatusOK, gin.H{"data": datos, "meta": paginacion.NuevaMeta(pag, total)})
}

func (h *PlanillaRetroactivosHandler) Eliminar(c *gin.Context) {
	usuarioID, ok := usuarioActual(c)
	if !ok {
		return
	}
	periodoID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		fallar(c, err)
		return
	}
	retroID, err := middleware.ParseIDParam(c, "retroactivo_id")
	if err != nil {
		fallar(c, err)
		return
	}
	if err := h.svc.Eliminar(c.Request.Context(), usuarioID, periodoID, retroID); err != nil {
		fallar(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
