package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/pagination"
	"planillas-ccdr/internal/planilla"
)

// PlanillaDevengosHandler expone los devengos manuales del período (L9).
type PlanillaDevengosHandler struct {
	svc *planilla.DevengosService
}

func NewPlanillaDevengosHandler(svc *planilla.DevengosService) *PlanillaDevengosHandler {
	return &PlanillaDevengosHandler{svc: svc}
}

// RegisterRoutes monta /planillas/:id/devengos sobre el grupo /planillas.
func (h *PlanillaDevengosHandler) RegisterRoutes(g *gin.RouterGroup) {
	g.POST("/:id/devengos", middleware.RequirePermission(authz.PermPlanillaCrear), h.Crear)
	g.GET("/:id/devengos", middleware.RequirePermission(authz.PermPlanillaVer), h.Listar)
	g.DELETE("/:id/devengos/:devengo_id", middleware.RequirePermission(authz.PermPlanillaCrear), h.Eliminar)
}

type devengoResponse struct {
	ID                 int64     `json:"id"`
	PeriodoPlanillaID  int64     `json:"periodo_planilla_id"`
	EmpleadoID         int64     `json:"empleado_id"`
	ConceptoSalarialID int64     `json:"concepto_salarial_id"`
	ConceptoCodigo     string    `json:"concepto_codigo"`
	Monto              string    `json:"monto"`
	Observacion        *string   `json:"observacion"`
	CreadoPor          int64     `json:"creado_por"`
	CreatedAt          time.Time `json:"created_at"`
}

func aDevengoResponse(r dto.DevengoManualRow) devengoResponse {
	return devengoResponse{
		ID: r.ID, PeriodoPlanillaID: r.PeriodoPlanillaID, EmpleadoID: r.EmpleadoID,
		ConceptoSalarialID: r.ConceptoSalarialID, ConceptoCodigo: r.ConceptoCodigo,
		Monto: r.Monto.StringFixed(2), Observacion: r.Observacion, CreadoPor: r.CreadoPor, CreatedAt: r.CreatedAt,
	}
}

type crearDevengoRequest struct {
	EmpleadoID         int64   `json:"empleado_id" binding:"required"`
	ConceptoSalarialID int64   `json:"concepto_salarial_id" binding:"required"`
	Monto              string  `json:"monto" binding:"required"` // texto decimal, ej. "45000.00" (nunca float)
	Observacion        *string `json:"observacion"`
}

func (h *PlanillaDevengosHandler) Crear(c *gin.Context) {
	usuarioID, ok := usuarioActual(c)
	if !ok {
		return
	}
	periodoID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		fallar(c, err)
		return
	}
	var req crearDevengoRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.EmpleadoID <= 0 || req.ConceptoSalarialID <= 0 {
		solicitudInvalida(c, "cuerpo inválido: se requieren empleado_id, concepto_salarial_id y monto (texto decimal)")
		return
	}
	monto, err := decimal.NewFromString(req.Monto)
	if err != nil {
		fallar(c, planilla.ErrMontoDevengoInvalido)
		return
	}
	r, err := h.svc.Crear(c.Request.Context(), usuarioID, periodoID, planilla.NuevoDevengo{
		EmpleadoID: req.EmpleadoID, ConceptoSalarialID: req.ConceptoSalarialID, Monto: monto, Observacion: req.Observacion,
	})
	if err != nil {
		fallar(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": aDevengoResponse(r)})
}

func (h *PlanillaDevengosHandler) Listar(c *gin.Context) {
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
	pag := pagination.Desde(c)
	filas, total, err := h.svc.Listar(c.Request.Context(), periodoID, empleadoID, pag)
	if err != nil {
		fallar(c, err)
		return
	}
	datos := make([]devengoResponse, len(filas))
	for i, r := range filas {
		datos[i] = aDevengoResponse(r)
	}
	c.JSON(http.StatusOK, gin.H{"data": datos, "meta": pagination.NuevaMeta(pag, total)})
}

func (h *PlanillaDevengosHandler) Eliminar(c *gin.Context) {
	usuarioID, ok := usuarioActual(c)
	if !ok {
		return
	}
	periodoID, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		fallar(c, err)
		return
	}
	devengoID, err := middleware.ParseIDParam(c, "devengo_id")
	if err != nil {
		fallar(c, err)
		return
	}
	if err := h.svc.Eliminar(c.Request.Context(), usuarioID, periodoID, devengoID); err != nil {
		fallar(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
