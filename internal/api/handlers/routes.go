// api/handlers/routes.go
//
// Wiring de rutas de la API bajo el grupo raíz (normalmente
// router.Group("/api/v1")):
//
//   - Autenticación (internal/auth): login, refresh, logout, reset de
//     contraseña (públicas) y gestión de sesiones/roles (protegidas).
//   - Empleados / Contratos / Organización.
//   - Asistencia y Vacaciones (internal/asistencia).
//   - Préstamos / embargos / terceros (internal/prestamos).
//   - Liquidaciones laborales.
//   - Export bancario y boleta de pago (planilla).
//   - Contabilización (internal/contabilidad): generar y consultar asientos.
//   - Dashboard gerencial.
//
// Regla de middleware (ADR-000 sección 3):
//
//   - `publico`   → SIN middleware. Solo endpoints que por definición no
//     tienen sesión: /auth/login, /auth/refresh, /auth/logout y el reset de
//     contraseña. Todo lo demás va en `protegido`.
//   - `protegido` → middleware.AuthPASETO (valida el access token y deja el
//     *security.Payload en el contexto).
//   - Cada ruta agrega además middleware.RequirePermission("<codigo>").
//     Nunca se autoriza por nombre de rol.
//
// IMPORTANTE (main.go): middleware.ErrorHandler() debe estar montado
// globalmente (después de gin.Recovery()), porque Fail/FailFromError solo
// registran el error en el contexto; ErrorHandler es quien escribe el JSON.
package handlers

import (
	"database/sql"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/auth"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/prestamos"
	"planillas-ccdr/internal/security"
	"planillas-ccdr/internal/services"

	"github.com/gin-gonic/gin"
)

// Deps agrupa los servicios ya construidos de los módulos que no se arman
// dentro de este archivo (dependen de sqlc, config de TTLs, hashing, etc.).
// Se construyen en cmd/api/main.go (buildDeps) y se pasan acá; así routes.go
// no tiene que adivinar los constructores de cada módulo.
type Deps struct {
	Auth           *auth.Servicio
	Asistencia     *asistencia.Service
	Vacaciones     *asistencia.VacacionesService
	Prestamos      *prestamos.Service
	Liquidacion    *services.LiquidacionService
	Dashboard      *services.DashboardService
	PlanillaExport *planilla.ExportService
	Contabilidad   *ContabilidadDeps

	// MotorEmpleado y Acumulados están construidos pero aún sin rutas:
	// se usan desde F20/F24.
	MotorEmpleado *planilla.MotorEmpleado
	Acumulados    *planilla.AcumuladosService
}

// RegisterRoutes monta todos los endpoints de la API.
func RegisterRoutes(rg *gin.RouterGroup, sqlDB *sql.DB, tokenMaker security.TokenMaker, deps Deps) {
	// Grupo sin autenticación (solo endpoints públicos de auth).
	publico := rg.Group("")

	// Grupo con sesión obligatoria.
	protegido := rg.Group("")
	protegido.Use(middleware.AuthPASETO(tokenMaker))

	// --- Auth ------------------------------------------------------------
	NuevoAuthHandler(deps.Auth).RegistrarRutas(publico, protegido)

	// --- Empleados / Contratos / Organización ----------------------------
	empleadoService := services.NewEmpleadoService(sqlDB)
	contratoService := services.NewContratoService(sqlDB)
	clasificacionService := services.NewClasificacionPuestoService(sqlDB)
	programaService := services.NewProgramaPresupuestarioService(sqlDB)
	unidadService := services.NewUnidadOrganizativaService(sqlDB)
	puestoService := services.NewPuestoService(sqlDB)

	NewEmpleadoHandler(empleadoService).RegisterRoutes(protegido)
	// Contratos: cada ruta ya lleva RequirePermission("contratos.*") dentro
	// de ContratoHandler.RegisterRoutes (crear/leer/actualizar).
	NewContratoHandler(contratoService).RegisterRoutes(protegido)
	NewOrganizacionHandler(clasificacionService, programaService, unidadService, puestoService).RegisterRoutes(protegido)

	// --- Asistencia ------------------------------------------------------
	// Rutas: /turnos, /calendario, /asistencia, /incapacidades.
	// Pendiente: PUT /turnos/:id responde 501 (DTO de actualización sin
	// definir en handler.actualizarTurno).
	asistencia.NewHandler(deps.Asistencia).RegisterRoutes(protegido)

	// --- Vacaciones ------------------------------------------------------
	// Rutas: /vacaciones/solicitudes (POST), /vacaciones/solicitudes/:id/
	// {aprobar,rechazar,disfrute} (POST) y /vacaciones/empleados/:empleado_id/
	// saldo (GET). Cada ruta aplica su RequirePermission dentro de
	// VacacionesHandler.RegisterRoutes (vacaciones.solicitar / .aprobar /
	// .registrar_disfrute / .ver).
	asistencia.NewVacacionesHandler(deps.Vacaciones).RegisterRoutes(protegido)

	// --- Préstamos / embargos / terceros ---------------------------------
	prestamos.RegisterRoutes(protegido, prestamos.NewHandler(deps.Prestamos))

	// --- Liquidaciones ---------------------------------------------------
	// No se usa LiquidacionHandler.RegisterRoutes: registra las dos rutas
	// sin permisos. Acá se montan los mismos métodos con permiso propio por
	// ruta, porque proyectar (simulación, no altera estado) y procesar
	// (persiste, cierra contrato y desactiva el expediente) no deben
	// compartir permiso.
	liquidacionHandler := NewLiquidacionHandler(deps.Liquidacion)
	liquidaciones := protegido.Group("/empleados")
	{
		liquidaciones.POST("/:id/liquidaciones/proyectar",
			middleware.RequirePermission("liquidaciones.proyectar"), liquidacionHandler.Proyectar)
		liquidaciones.POST("/:id/liquidaciones/procesar",
			middleware.RequirePermission("liquidaciones.procesar"), liquidacionHandler.Procesar)
	}

	// --- Export bancario y boleta ----------------------------------------
	// Permisos según el comentario de PlanillaExportHandler. No se usa su
	// RegisterRoutes porque no aplica RequirePermission.
	exportHandler := NewPlanillaExportHandler(deps.PlanillaExport)
	planillas := protegido.Group("/planillas")
	{
		planillas.GET("/:id/exportar/bancario",
			middleware.RequirePermission("planilla.exportar_bancario"), exportHandler.ExportarDispersionBancaria)
		planillas.GET("/:id/boleta/:empleado_id",
			middleware.RequirePermission("planilla.ver_boleta"), exportHandler.ObtenerBoletaPago)
	}

	// --- Contabilidad ----------------------------------------------------
	// Generar (escribe: asientos + período a 'contabilizada') y ver (solo
	// lectura) llevan permisos distintos por segregación de funciones.
	contabilidadHandler := NewContabilidadHandler(*deps.Contabilidad)
	protegido.POST("/planillas/:id/contabilizar",
		middleware.RequirePermission("asientos_contables.generar"), contabilidadHandler.Generar)
	protegido.GET("/asientos-contables",
		middleware.RequirePermission("asientos_contables.ver"), contabilidadHandler.Listar)

	// --- Dashboard -------------------------------------------------------
	// GET /health (público) se monta en main.go a nivel raíz, fuera de
	// /api/v1, para que los probes no pasen por AuthPASETO.
	dashboardHandler := NewDashboardHandler(deps.Dashboard)
	protegido.GET("/dashboard/metricas",
		middleware.RequirePermission("dashboard.ver"), dashboardHandler.GetMetricas)
}
