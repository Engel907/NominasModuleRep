package prestamos

import (
	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
)

// RegisterRoutes monta los endpoints del módulo bajo el grupo recibido
// (normalmente /api/v1). Cada ruta está protegida por AuthMiddleware
// (ya aplicado al grupo raíz por el caller) y por RequirePermission con
// el código de permiso atómico correspondiente — nunca por rol directo,
// según la regla de autorización de ADR-000 sección 3.
//
// Códigos de permiso esperados en el catálogo `permisos` (modulo =
// 'planilla'), a sembrar si aún no existen:
//
//	prestamos.crear, prestamos.ver, prestamos.abonar, prestamos.cancelar
//	embargos.crear, embargos.ver, embargos.cambiar_estado
//	terceros.crear, terceros.ver
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	terceros := rg.Group("/terceros")
	{
		terceros.POST("", middleware.RequirePermission("terceros.crear"), h.CrearTercero)
		terceros.GET("", middleware.RequirePermission("terceros.ver"), h.ListarTerceros)
	}

	prestamosGrp := rg.Group("/prestamos")
	{
		prestamosGrp.POST("", middleware.RequirePermission("prestamos.crear"), h.CrearPrestamo)
		prestamosGrp.GET("/empleado/:empleado_id", middleware.RequirePermission("prestamos.ver"), h.ListarPrestamosPorEmpleado)
		prestamosGrp.POST("/:id/abonos", middleware.RequirePermission("prestamos.abonar"), h.AplicarAbonoPrestamo)
		prestamosGrp.POST("/:id/cancelar", middleware.RequirePermission("prestamos.cancelar"), h.CancelarPrestamo)
	}

	embargosGrp := rg.Group("/embargos")
	{
		embargosGrp.POST("", middleware.RequirePermission("embargos.crear"), h.CrearEmbargo)
		embargosGrp.GET("/empleado/:empleado_id", middleware.RequirePermission("embargos.ver"), h.ListarEmbargosPorEmpleado)
		embargosGrp.GET("/empleado/:empleado_id/activos", middleware.RequirePermission("embargos.ver"), h.ListarEmbargosActivosOrdenados)
		embargosGrp.PATCH("/:id/estado", middleware.RequirePermission("embargos.cambiar_estado"), h.CambiarEstadoEmbargo)
	}
}
