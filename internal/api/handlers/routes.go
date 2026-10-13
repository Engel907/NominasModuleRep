// api/handlers/routes.go
//
// Wiring de rutas de la API bajo el grupo raíz (normalmente
// router.Group("/api/v1")):
//
//   - Fase 3: Empleados / Contratos / Organización.
//   - Fase 5: Autenticación (internal/auth) — login, refresh, logout, reset
//     de contraseña (públicas) y gestión de sesiones/roles (protegidas).
//   - Fase 4: Asistencia (internal/asistencia).
//   - Fase 12B: Vacaciones (internal/asistencia, VacacionesHandler).
//   - Préstamos / embargos / terceros (internal/prestamos).
//   - Fase 6c: Export bancario y boleta de pago (planilla).
//   - Fase 7: Liquidaciones laborales.
//   - Fase 8: Dashboard gerencial.
//   - Fase 11B: Contabilización (internal/contabilidad) — generar y consultar asientos.
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
	Auth       *auth.Servicio
	Asistencia *asistencia.Service
	// Vacaciones (Fase 12B): nil = no se montan las rutas (mismo criterio que
	// PlanillaExport). Lo arma main.go con asistencia.NewVacacionesService.
	Vacaciones     *asistencia.VacacionesService
	Prestamos      *prestamos.Service
	Liquidacion    *services.LiquidacionService
	Dashboard      *services.DashboardService
	PlanillaExport *planilla.ExportService
	// Contabilidad (Fase 11B): nil = no se montan las rutas (mismo criterio que
	// PlanillaExport). Lo arma main.go con contabilidad.Service, el proveedor de
	// detalles de la corrida y el lector de asientos.
	Contabilidad *ContabilidadDeps
}

// RegisterRoutes monta todos los endpoints de la API.
func RegisterRoutes(rg *gin.RouterGroup, sqlDB *sql.DB, tokenMaker security.TokenMaker, deps Deps) {
	// Grupo sin autenticación (solo endpoints públicos de auth).
	publico := rg.Group("")

	// Grupo con sesión obligatoria.
	protegido := rg.Group("")
	protegido.Use(middleware.AuthPASETO(tokenMaker))

	// --- Auth (Fase 5) ---------------------------------------------------
	NuevoAuthHandler(deps.Auth).RegistrarRutas(publico, protegido)

	// --- Empleados / Contratos / Organización (Fase 3) -------------------
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

	// --- Asistencia (Fase 4) ---------------------------------------------
	// Rutas: /turnos, /calendario, /asistencia, /incapacidades.
	// TODO(fase 9): PUT /turnos/:id responde 501 (DTO de actualización sin
	// definir en handler.actualizarTurno).
	asistencia.NewHandler(deps.Asistencia).RegisterRoutes(protegido)

	// --- Vacaciones (Fase 12B) -------------------------------------------
	// Rutas: /vacaciones/solicitudes (POST), /vacaciones/solicitudes/:id/
	// {aprobar,rechazar,disfrute} (POST) y /vacaciones/empleados/:empleado_id/
	// saldo (GET). Cada ruta aplica su RequirePermission dentro de
	// VacacionesHandler.RegisterRoutes (vacaciones.solicitar / .aprobar /
	// .registrar_disfrute / .ver). Solo se montan si main.go inyectó el
	// servicio (buildDeps).
	if deps.Vacaciones != nil {
		asistencia.NewVacacionesHandler(deps.Vacaciones).RegisterRoutes(protegido)
	}

	// --- Préstamos / embargos / terceros ---------------------------------
	prestamos.RegisterRoutes(protegido, prestamos.NewHandler(deps.Prestamos))

	// --- Liquidaciones (Fase 7) ------------------------------------------
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

	// --- Export bancario y boleta (Fase 6c) ------------------------------
	// Permisos según el comentario de PlanillaExportHandler. No se usa su
	// RegisterRoutes por la misma razón: no aplica RequirePermission.
	// Solo se montan si main.go inyectó el servicio: hoy no existe una
	// implementación concreta de planilla.ExportRepository (ver buildDeps).
	if deps.PlanillaExport != nil {
		exportHandler := NewPlanillaExportHandler(deps.PlanillaExport)
		planillas := protegido.Group("/planillas")
		{
			planillas.GET("/:id/exportar/bancario",
				middleware.RequirePermission("planilla.exportar_bancario"), exportHandler.ExportarDispersionBancaria)
			planillas.GET("/:id/boleta/:empleado_id",
				middleware.RequirePermission("planilla.ver_boleta"), exportHandler.ObtenerBoletaPago)
		}
	}

	// --- Contabilidad (Fase 11B) -----------------------------------------
	// Generar (escribe: asientos + período a 'contabilizada') y ver (solo
	// lectura) llevan permisos distintos por segregación de funciones.
	if deps.Contabilidad != nil {
		contabilidadHandler := NewContabilidadHandler(*deps.Contabilidad)
		protegido.POST("/planillas/:id/contabilizar",
			middleware.RequirePermission("asientos_contables.generar"), contabilidadHandler.Generar)
		protegido.GET("/asientos-contables",
			middleware.RequirePermission("asientos_contables.ver"), contabilidadHandler.Listar)
	}

	// --- Dashboard (Fase 8) ----------------------------------------------
	// GET /health (público) se monta en main.go a nivel raíz, fuera de
	// /api/v1, para que los probes no pasen por AuthPASETO.
	dashboardHandler := NewDashboardHandler(deps.Dashboard)
	protegido.GET("/dashboard/metricas",
		middleware.RequirePermission("dashboard.ver"), dashboardHandler.GetMetricas)
}
