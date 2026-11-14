package prestamos

import (
	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
)

// RegisterRoutes monta los endpoints del módulo; cada ruta exige su permiso.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	terceros := rg.Group("/terceros")
	{
		terceros.POST("", middleware.RequirePermission(authz.PermTercerosCrear), h.CrearTercero)
		terceros.GET("", middleware.RequirePermission(authz.PermTercerosVer), h.ListarTerceros)
	}

	prestamosGrp := rg.Group("/prestamos")
	{
		prestamosGrp.POST("", middleware.RequirePermission(authz.PermPrestamosCrear), h.CrearPrestamo)
		prestamosGrp.GET("/empleado/:empleado_id", middleware.RequirePermission(authz.PermPrestamosVer), h.ListarPrestamosPorEmpleado)
		prestamosGrp.POST("/:id/abonos", middleware.RequirePermission(authz.PermPrestamosAbonar), h.AplicarAbonoPrestamo)
		prestamosGrp.POST("/:id/cancelar", middleware.RequirePermission(authz.PermPrestamosCancelar), h.CancelarPrestamo)
	}

	embargosGrp := rg.Group("/embargos")
	{
		embargosGrp.POST("", middleware.RequirePermission(authz.PermEmbargosCrear), h.CrearEmbargo)
		embargosGrp.GET("/empleado/:empleado_id", middleware.RequirePermission(authz.PermEmbargosVer), h.ListarEmbargosPorEmpleado)
		embargosGrp.GET("/empleado/:empleado_id/activos", middleware.RequirePermission(authz.PermEmbargosVer), h.ListarEmbargosActivosOrdenados)
		embargosGrp.PATCH("/:id/estado", middleware.RequirePermission(authz.PermEmbargosCambiarEstado), h.CambiarEstadoEmbargo)
	}
}
