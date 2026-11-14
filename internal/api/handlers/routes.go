package handlers

import (
	"database/sql"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/auth"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/catalogos"
	"planillas-ccdr/internal/credenciales"
	"planillas-ccdr/internal/cripto"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/observabilidad"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/prestamos"
	"planillas-ccdr/internal/rrhh"

	"github.com/gin-gonic/gin"
)

// Deps agrupa los servicios ya construidos que las rutas necesitan.
type Deps struct {
	Auth             *auth.Servicio
	Asistencia       *asistencia.Service
	Vacaciones       *asistencia.VacacionesService
	HorasExtra       *asistencia.HorasExtraService
	Prestamos        *prestamos.Service
	Liquidacion      *rrhh.LiquidacionService
	Dashboard        *rrhh.DashboardService
	PlanillaExport   *planilla.ExportService
	PlanillaPeriodo  *planilla.PeriodoService
	PlanillaDevengos *planilla.DevengosService
	Contabilidad     *ContabilidadDeps

	// Acumulados: POST /aguinaldo/:anio/ejecutar y /salario-escolar/:anio/ejecutar.
	Acumulados *planilla.EjecutorAcumulados
	// /parametros (propuesta y aprobación de parámetros legales).
	Parametros *parametros.Propuestas
	Catalogos  *catalogos.Service
	// Ley 8968: llavero de cifrado de datos personales.
	Llavero *cripto.Llavero
	// /readyz (parámetros requeridos) y métricas Prometheus.
	Generales *parametros.Generales
	Metricas  *observabilidad.Metricas
	// rate limiting. app.New los arma desde la config. Si un test arma
	// Deps a mano y los deja nil, simplemente no se limita (en producción
	// siempre los provee app.New).
	//   LimiteAuth    -> grupo `publico` (login/refresh/logout/reset), por IP+ruta.
	//   LimiteUsuario -> grupo `protegido`, por usuario autenticado.
	LimiteAuth    gin.HandlerFunc
	LimiteUsuario gin.HandlerFunc
}

// RegisterRoutes monta todos los endpoints de la API.
func RegisterRoutes(rg *gin.RouterGroup, sqlDB *sql.DB, tokenMaker credenciales.TokenMaker, sesiones middleware.SesionVerificador, deps Deps) {
	queries := dto.New(sqlDB)
	publico := rg.Group("")
	if deps.LimiteAuth != nil {
		publico.Use(deps.LimiteAuth)
	}

	protegido := rg.Group("")
	protegido.Use(middleware.AuthPASETO(tokenMaker, sesiones))
	protegido.Use(middleware.ExigirCambioPassword())

	if deps.LimiteUsuario != nil {
		protegido.Use(deps.LimiteUsuario)
	}

	NuevoAuthHandler(deps.Auth).RegistrarRutas(publico, protegido)
	NewUsuariosHandler(deps.Auth).RegisterRoutes(protegido)
	NewAuditoriaHandler(queries).RegisterRoutes(protegido)

	empleadoService := rrhh.NewEmpleadoService(sqlDB, deps.Llavero)
	contratoService := rrhh.NewContratoService(sqlDB)
	clasificacionService := rrhh.NewClasificacionPuestoService(sqlDB)
	programaService := rrhh.NewProgramaPresupuestarioService(sqlDB)
	unidadService := rrhh.NewUnidadOrganizativaService(sqlDB)
	puestoService := rrhh.NewPuestoService(sqlDB)

	NewEmpleadoHandler(empleadoService).RegisterRoutes(protegido)
	NewAsignacionesHandler(rrhh.NewAsignacionesService(sqlDB)).RegisterRoutes(protegido)
	NewHistoricoSalariosHandler(rrhh.NewHistoricoSalariosService(sqlDB)).RegisterRoutes(protegido)
	NewContratoHandler(contratoService).RegisterRoutes(protegido)
	NewOrganizacionHandler(clasificacionService, programaService, unidadService, puestoService).RegisterRoutes(protegido)

	if deps.Catalogos != nil {
		NewCatalogosHandler(deps.Catalogos).RegisterRoutes(protegido)
	}
	if deps.Parametros != nil {
		NewParametrosHandler(deps.Parametros).RegisterRoutes(protegido)
	}

	asistencia.NewHandler(deps.Asistencia).RegisterRoutes(protegido)
	if deps.HorasExtra != nil {
		asistencia.NewHorasExtraHandler(deps.HorasExtra).RegisterRoutes(protegido)
	}

	asistencia.NewVacacionesHandler(deps.Vacaciones, queries).RegisterRoutes(protegido)

	prestamos.RegisterRoutes(protegido, prestamos.NewHandler(deps.Prestamos))

	liquidacionHandler := NewLiquidacionHandler(deps.Liquidacion)
	liquidaciones := protegido.Group("/empleados")
	{
		liquidaciones.POST("/:id/liquidaciones/proyectar",
			middleware.RequirePermission(authz.PermLiquidacionesProyectar), liquidacionHandler.Proyectar)
		liquidaciones.POST("/:id/liquidaciones/procesar",
			middleware.RequirePermission(authz.PermLiquidacionesProcesar), liquidacionHandler.Procesar)
	}

	exportHandler := NewPlanillaExportHandler(deps.PlanillaExport, queries)
	planillas := protegido.Group("/planillas")
	{
		planillas.GET("/:id/exportar/bancario",
			middleware.RequirePermission(authz.PermPlanillaExportarBancario), exportHandler.ExportarDispersionBancaria)
		planillas.GET("/:id/boleta/:empleado_id",
			middleware.RequireAnyPermission(authz.PermPlanillaVerBoleta, authz.PermPlanillaVerBoletaPropia), exportHandler.ObtenerBoletaPago)
		planillas.GET("/:id/boleta/:empleado_id/pdf",
			middleware.RequireAnyPermission(authz.PermPlanillaVerBoleta, authz.PermPlanillaVerBoletaPropia), exportHandler.ObtenerBoletaPagoPDF)
		reportesOficiales := planilla.NewReportesService(sqlDB, descifrador(deps.Llavero))
		reportes := NewReportesHandler(reportesOficiales)
		NewReportesGestionHandler(planilla.NewReportesGestionService(sqlDB), reportesOficiales).RegisterRoutes(protegido)
		planillas.GET("/:id/reportes/ccss", middleware.RequirePermission(authz.PermReportesCcss), reportes.generar(planilla.ReporteCCSS))
		planillas.GET("/:id/reportes/ins", middleware.RequirePermission(authz.PermReportesIns), reportes.generar(planilla.ReporteINS))
		planillas.GET("/:id/reportes/hacienda", middleware.RequirePermission(authz.PermReportesHacienda), reportes.generar(planilla.ReporteHacienda))
		NewPlanillaPeriodoHandler(deps.PlanillaPeriodo).RegisterRoutes(planillas)
		if deps.PlanillaDevengos != nil {
			NewPlanillaDevengosHandler(deps.PlanillaDevengos).RegisterRoutes(planillas)
		}
	}

	if deps.Acumulados != nil {
		NewAcumuladosHandler(deps.Acumulados).RegisterRoutes(protegido)
	}

	contabilidadHandler := NewContabilidadHandler(*deps.Contabilidad)
	protegido.POST("/planillas/:id/contabilizar",
		middleware.RequirePermission(authz.PermAsientosContablesGenerar), contabilidadHandler.Generar)
	protegido.GET("/asientos-contables",
		middleware.RequirePermission(authz.PermAsientosContablesVer), contabilidadHandler.Listar)

	dashboardHandler := NewDashboardHandler(deps.Dashboard)
	protegido.GET("/dashboard/metricas",
		middleware.RequirePermission(authz.PermDashboardVer), dashboardHandler.GetMetricas)
}

// descifrador evita pasar un *cripto.Llavero nil envuelto en una interfaz
// no nil.
func descifrador(l *cripto.Llavero) planilla.Descifrador {
	if l == nil {
		return nil
	}
	return l
}
