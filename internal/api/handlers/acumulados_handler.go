package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/planilla"
)

// AcumuladosHandler: crea el período de aguinaldo o salario escolar de
// un año. El cálculo, la aprobación y el pago siguen por /planillas/:id/*.
type AcumuladosHandler struct {
	svc *planilla.EjecutorAcumulados
}

func NewAcumuladosHandler(svc *planilla.EjecutorAcumulados) *AcumuladosHandler {
	return &AcumuladosHandler{svc: svc}
}

func (h *AcumuladosHandler) RegisterRoutes(g *gin.RouterGroup) {
	g.POST("/aguinaldo/:anio/ejecutar",
		middleware.RequirePermission(authz.PermAguinaldoEjecutar), h.ejecutar(planilla.ClaseAguinaldo))
	g.POST("/salario-escolar/:anio/ejecutar",
		middleware.RequirePermission(authz.PermSalarioEscolarEjecutar), h.ejecutar(planilla.ClaseSalarioEscolar))
}

// ejecutar: 201 con el período creado; fuera de ventana o ya ejecutado → 409.
func (h *AcumuladosHandler) ejecutar(clase planilla.ClaseTipoPlanilla) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuarioID, ok := usuarioActual(c)
		if !ok {
			return
		}
		anio, err := strconv.Atoi(c.Param("anio"))
		if err != nil || anio < 1900 || anio > 9999 {
			solicitudInvalida(c, "anio inválido")
			return
		}
		p, err := h.svc.Ejecutar(c.Request.Context(), usuarioID, clase, anio)
		if err != nil {
			fallar(c, err)
			return
		}
		c.JSON(http.StatusCreated, aRespuesta(p))
	}
}
