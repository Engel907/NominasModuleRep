// Package app arma la aplicación HTTP (servicios + router de Gin) a partir de
// la configuración, el pool de BD y el TokenMaker. Vive fuera de cmd/api para
// que las pruebas de integración levanten exactamente el mismo router que
// producción (httptest.NewServer(app.New(...))).
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"planillas-ccdr/internal/api/handlers"
	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/auth"
	"planillas-ccdr/internal/catalogos"
	"planillas-ccdr/internal/config"
	"planillas-ccdr/internal/contabilidad"
	"planillas-ccdr/internal/credenciales"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/liquidacion"
	"planillas-ccdr/internal/observabilidad"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/planilla/infra"
	"planillas-ccdr/internal/prestamos"
	"planillas-ccdr/internal/rrhh"

	"github.com/gin-gonic/gin"
)

// New construye los servicios y devuelve el engine de Gin listo para servir.
// Falla rápido si algún servicio no se puede armar (error de despliegue).
func New(cfg *config.Config, sqlDB *sql.DB, tokenMaker credenciales.TokenMaker) (*gin.Engine, error) {
	middleware.ConfigurarValidador()

	deps, err := buildDeps(cfg, sqlDB, tokenMaker)
	if err != nil {
		return nil, fmt.Errorf("deps: %w", err)
	}

	limAuth, err := middleware.NuevoLimitador(cfg.RateLimit.AuthPorMin, cfg.RateLimit.AuthBurst)
	if err != nil {
		return nil, fmt.Errorf("rate limit auth: %w", err)
	}
	limUsuario, err := middleware.NuevoLimitador(cfg.RateLimit.UsuarioPorMin, cfg.RateLimit.UsuarioBurst)
	if err != nil {
		return nil, fmt.Errorf("rate limit usuario: %w", err)
	}
	deps.LimiteAuth = middleware.LimitarPorIP(limAuth)
	deps.LimiteUsuario = middleware.LimitarPorUsuario(limUsuario)

	queries := dto.New(sqlDB)
	sesiones := middleware.NuevoSesionCache(func(ctx context.Context, id int64) (bool, uint32, error) {
		e, err := queries.GetEstadoSesion(ctx, id)
		return e.Activo, e.PermisosVersion, err
	}, cfg.Auth.CacheTTL)
	return newRouter(cfg, sqlDB, tokenMaker, sesiones, deps)
}

// NewTokenMaker arma el PasetoMaker real desde el llavero configurado
// (PASETO_LOCAL_KEYS_HEX / PASETO_ACTIVE_KID, o la clave legada), con issuer y
// audience que se firman y se exigen. config.validate ya lo comprobó,
// pero se revalida por si se llama fuera de ese flujo (tests).
func NewTokenMaker(authCfg config.AuthConfig) (credenciales.TokenMaker, error) {
	claves, activo, err := authCfg.ClavesPaseto()
	if err != nil {
		return nil, err
	}
	return credenciales.NuevoPasetoMaker(credenciales.ConfigPaseto{
		Claves: claves, KIDActivo: activo,
		Issuer: authCfg.Issuer, Audience: authCfg.Audience,
	})
}

// buildDeps construye los servicios que dependen de la capa de datos y los
// agrupa en handlers.Deps.
func buildDeps(cfg *config.Config, sqlDB *sql.DB, tokenMaker credenciales.TokenMaker) (handlers.Deps, error) {
	queries := dto.New(sqlDB)

	llavero, err := cfg.Datos.Llavero()
	if err != nil {
		return handlers.Deps{}, fmt.Errorf("datos personales: %w", err)
	}

	escritor := &planilla.EscritorDetalle{}
	repoParametros := parametros.NewRepositorioSQL(sqlDB)
	generales := parametros.NewGenerales(repoParametros, cfg.Parametros.CacheTTL).ConValoresPorEmpleado(repoParametros)
	ventanas := planilla.NewVentanas(generales)
	metricas := observabilidad.Nuevas()
	orquestador := planilla.NewOrquestador(
		planilla.NewHidratador(queries),
		planilla.NewMotorEmpleado(infra.NewMotorRepository(queries)),
		planilla.NewCargadorParametros(infra.NewFuenteParametros(queries), generales).
			ConAjustesPorEmpleado(queries, func(id int64) planilla.GeneralesLector { return generales.ParaEmpleado(id) }),
		escritor,
		queries,
		ventanas,
	).ConObservador(metricas)
	cfgConta := contabilidad.ConfigGeneracion{
		ConceptoAnclaNeto: "SAL_BASE",
		TipoAsientoPago:   "PAGO_PLANILLA",
	}
	ganchoPago, err := contabilidad.NewGanchoPago(cfgConta)
	if err != nil {
		return handlers.Deps{}, fmt.Errorf("contabilidad: %w", err)
	}
	ganchosPeriodo := planilla.PeriodoGanchos{
		Calculador: orquestador,
		Eliminador: escritor,
		AlAprobar:  planilla.GanchosAlAprobar{planilla.NewAplicadorPrestamos(), planilla.NewAplicadorAcumulados(), planilla.NewAplicadorVacaciones()},
		AlPagar:    ganchoPago,
	}
	periodoSvc := planilla.NewPeriodoService(sqlDB, ganchosPeriodo)

	asistenciaSvc, err := asistencia.NewService(sqlDB, queries, generales)
	if err != nil {
		return handlers.Deps{}, fmt.Errorf("asistencia: %w", err)
	}

	contaSvc, err := contabilidad.NewService(contabilidad.NewRepositorio(sqlDB), cfgConta)
	if err != nil {
		return handlers.Deps{}, fmt.Errorf("contabilidad: %w", err)
	}
	contaDetalles, err := contabilidad.NewDetallesSQL(sqlDB)
	if err != nil {
		return handlers.Deps{}, fmt.Errorf("contabilidad: %w", err)
	}

	notificador, err := auth.NuevoNotificador(cfg, slog.Default())
	if err != nil {
		return handlers.Deps{}, err
	}

	return handlers.Deps{
		Auth: auth.NuevoServicio(queries, sqlDB, tokenMaker, politicaAuth(cfg.Auth)).ConNotificador(notificador),

		Asistencia: asistenciaSvc,

		Prestamos: prestamos.NewService(sqlDB, queries),

		Dashboard: rrhh.NewDashboardService(rrhh.NewDashboardRepository(queries), sqlDB, 3*time.Second),

		Liquidacion: rrhh.NewLiquidacionService(sqlDB, queries, infra.NewMotorRepository(queries), generales),

		PlanillaExport:   planilla.NewExportService(infra.NewExportRepository(queries, llavero), infra.NewExportUnitOfWork(sqlDB, llavero)),
		PlanillaPeriodo:  periodoSvc,
		PlanillaDevengos: planilla.NewDevengosService(sqlDB),

		Vacaciones: asistencia.NewVacacionesService(sqlDB, queries),
		HorasExtra: asistencia.NewHorasExtraService(sqlDB),

		Acumulados: planilla.NewEjecutorAcumulados(periodoSvc, queries, ventanas),

		Llavero:   llavero,
		Generales: generales,
		Metricas:  metricas,

		Catalogos:  catalogos.NewService(sqlDB, catalogos.Registro()),
		Parametros: parametros.NewPropuestas(sqlDB, parametros.RegistroEspecs(parametros.Validadores{Renta: planilla.ValidarTramosRenta, Formula: liquidacion.ValidarExpresionFormula}), generales.Invalidar),

		Contabilidad: &handlers.ContabilidadDeps{
			Servicio: contaSvc,
			Detalles: contaDetalles,
			Lector:   contabilidad.NewLectorSQL(sqlDB),
			UsuarioID: func(c *gin.Context) (int64, bool) {
				id := middleware.UsuarioID(c)
				return id, id > 0
			},
		},
	}, nil
}

// politicaAuth aplica la configuración a auth.Politica.
func politicaAuth(a config.AuthConfig) auth.Politica {
	return auth.Politica{
		MaxIntentosFallidos:  a.MaxFailedLogins,
		MinutosBloqueo:       a.LockoutDuration,
		DuracionAccessToken:  a.AccessTokenDuration,
		DuracionRefreshToken: a.RefreshTokenDuration,
		DuracionTokenReset:   a.PasswordResetTokenDuration,
	}
}

// newRouter arma el engine de Gin con la cadena global de middleware y
// delega el montaje de las rutas de negocio en handlers.RegisterRoutes.
func newRouter(cfg *config.Config, sqlDB *sql.DB, tokenMaker credenciales.TokenMaker, sesiones middleware.SesionVerificador, deps handlers.Deps) (*gin.Engine, error) {
	if cfg.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	proxies := cfg.Server.TrustedProxies
	if proxies == nil {
		proxies = []string{}
	}
	if err := router.SetTrustedProxies(proxies); err != nil {
		return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}

	logger := slog.Default()

	if deps.Metricas != nil {
		router.Use(deps.Metricas.Middleware())
		router.GET("/metrics", deps.Metricas.Handler(cfg.Server.MetricsToken))
	}
	router.Use(
		middleware.RequestID(),
		middleware.SecurityHeaders(cfg.App.IsProduction()),
		middleware.RequestLogger(logger),
		middleware.Recovery(logger),
		middleware.ErrorHandler(),
		middleware.CORS(cfg.Server.CORSAllowedOrigins),
		middleware.LimitarCuerpo(cfg.Server.MaxBodyBytes),
	)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			middleware.Fail(c, http.StatusServiceUnavailable, "db_unavailable", "No se pudo conectar a la base de datos", nil)
			return
		}
		if deps.Generales != nil {
			faltan, err := deps.Generales.Faltantes(ctx, time.Now().UTC())
			if err != nil {
				middleware.Fail(c, http.StatusServiceUnavailable, "parametros_ilegibles", "No se pudieron leer los parámetros requeridos", nil)
				return
			}
			if len(faltan) > 0 {
				claves := make([]string, len(faltan))
				for i, f := range faltan {
					claves[i] = f.Clave
				}
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "parametros_faltantes", "faltantes": claves})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "listo"})
	})

	router.GET("/health", handlers.NewDashboardHandler(deps.Dashboard).GetHealth)

	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api, sqlDB, tokenMaker, sesiones, deps)

	return router, nil
}
