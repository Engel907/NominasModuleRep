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
	"planillas-ccdr/internal/config"
	"planillas-ccdr/internal/contabilidad"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/planilla/infra"
	"planillas-ccdr/internal/prestamos"
	"planillas-ccdr/internal/security"
	"planillas-ccdr/internal/services"

	"github.com/gin-gonic/gin"
)

// New construye los servicios y devuelve el engine de Gin listo para servir.
// Falla rápido si algún servicio no se puede armar (error de despliegue).
func New(cfg *config.Config, sqlDB *sql.DB, tokenMaker security.TokenMaker) (*gin.Engine, error) {
	// F16: errores de validación con el nombre del tag json (una sola vez,
	// antes de servir tráfico).
	middleware.ConfigurarValidador()

	deps, err := buildDeps(cfg, sqlDB, tokenMaker)
	if err != nil {
		return nil, fmt.Errorf("deps: %w", err)
	}

	// F16 (S8): limitadores en memoria (una réplica; ver middleware/ratelimit.go).
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

	// F15: verificador de sesión (activo + permisos_version) con caché de TTL corto.
	queries := dto.New(sqlDB)
	sesiones := middleware.NuevoSesionCache(func(ctx context.Context, id int64) (bool, uint32, error) {
		e, err := queries.GetEstadoSesion(ctx, id)
		return e.Activo, e.PermisosVersion, err
	}, cfg.Auth.CacheTTL)
	return newRouter(cfg, sqlDB, tokenMaker, sesiones, deps)
}

// NewTokenMaker arma el PasetoMaker real desde el llavero configurado
// (PASETO_LOCAL_KEYS_HEX / PASETO_ACTIVE_KID, o la clave legada), con issuer y
// audience que se firman y se exigen (F15). config.validate() ya lo comprobó,
// pero se revalida por si se llama fuera de ese flujo (tests).
func NewTokenMaker(authCfg config.AuthConfig) (security.TokenMaker, error) {
	claves, activo, err := authCfg.ClavesPaseto()
	if err != nil {
		return nil, err
	}
	return security.NuevoPasetoMaker(security.ConfigPaseto{
		Claves: claves, KIDActivo: activo,
		Issuer: authCfg.Issuer, Audience: authCfg.Audience,
	})
}

// buildDeps construye los servicios que dependen de la capa de datos
// (internal/db/dto) y los agrupa en handlers.Deps. Es el único lugar donde se
// conecta la capa de datos con los servicios: routes.go solo monta rutas. Los
// servicios de empleados, contratos y organización se arman dentro de
// handlers.RegisterRoutes porque reciben *sql.DB directo.
func buildDeps(cfg *config.Config, sqlDB *sql.DB, tokenMaker security.TokenMaker) (handlers.Deps, error) {
	queries := dto.New(sqlDB)

	asistenciaSvc, err := asistencia.NewService(sqlDB, queries, asistencia.Opciones{
		ToleranciaTardanzaMin: cfg.Asistencia.ToleranciaTardanzaMin,
	})
	if err != nil {
		return handlers.Deps{}, fmt.Errorf("asistencia: %w", err)
	}

	cfgConta := contabilidad.ConfigGeneracion{
		ConceptoAnclaNeto:         "SAL_BASE",
		ConceptoCuotaPatronalCCSS: "CCSS_PAT",
	}
	contaSvc, err := contabilidad.NewService(contabilidad.NewRepositorio(sqlDB), cfgConta)
	if err != nil {
		return handlers.Deps{}, fmt.Errorf("contabilidad: %w", err)
	}
	contaDetalles, err := contabilidad.NewDetallesSQL(sqlDB, cfgConta, "CCSS_OBR")
	if err != nil {
		return handlers.Deps{}, fmt.Errorf("contabilidad: %w", err)
	}

	return handlers.Deps{
		// auth.Store son métodos de *dto.Queries (ver auth/service.go).
		Auth: auth.NuevoServicio(queries, sqlDB, tokenMaker, politicaAuth(cfg.Auth)),

		Asistencia: asistenciaSvc,

		Prestamos: prestamos.NewService(sqlDB, queries),

		// Dashboard gerencial.
		Dashboard: services.NewDashboardService(services.NewDashboardRepository(queries), sqlDB, 3*time.Second),

		// Liquidaciones: *dto.Queries debe implementar services.LiquidacionRepository.
		Liquidacion: services.NewLiquidacionService(sqlDB, queries, infra.NewMotorRepository(queries)),

		PlanillaExport: planilla.NewExportService(infra.NewExportRepository(queries), infra.NewExportUnitOfWork(sqlDB)),

		Vacaciones: asistencia.NewVacacionesService(sqlDB, queries),

		// Construidos, sin rutas todavía: se usan desde F20/F24.
		MotorEmpleado: planilla.NewMotorEmpleado(infra.NewMotorRepository(queries)),
		Acumulados:    planilla.NewAcumuladosService(infra.NewAcumuladosRepository(queries)),

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

// politicaAuth aplica la configuración a auth.Politica. (F15: PASETO_ISSUER y
// PASETO_AUDIENCE ahora los consume NewTokenMaker.)
func politicaAuth(a config.AuthConfig) auth.Politica {
	return auth.Politica{
		MaxIntentosFallidos:  a.MaxFailedLogins,
		MinutosBloqueo:       a.LockoutDuration,
		DuracionAccessToken:  a.AccessTokenDuration,
		DuracionRefreshToken: a.RefreshTokenDuration,
		DuracionTokenReset:   a.PasswordResetTokenDuration,
	}
}

// newRouter arma el engine de Gin con la cadena global de middleware (F16) y
// delega el montaje de las rutas de negocio en handlers.RegisterRoutes.
func newRouter(cfg *config.Config, sqlDB *sql.DB, tokenMaker security.TokenMaker, sesiones middleware.SesionVerificador, deps handlers.Deps) (*gin.Engine, error) {
	if cfg.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// S8: sin esto gin confía en TODOS los proxies y ClientIP() sale de
	// X-Forwarded-For, que el cliente controla (evade el rate limiter y
	// falsea los logs). Lista vacía = ninguno: se usa la IP del socket.
	proxies := cfg.Server.TrustedProxies
	if proxies == nil {
		proxies = []string{}
	}
	if err := router.SetTrustedProxies(proxies); err != nil {
		return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}

	logger := slog.Default() // main.go lo reemplaza por el logger JSON configurado

	// ORDEN (importa):
	//  1. RequestID        — primero: todo lo demás ya tiene request_id.
	//  2. SecurityHeaders  — antes de nada que pueda responder: errores y 404 también las llevan.
	//  3. RequestLogger    — envuelve al resto para registrar el estado final.
	//  4. Recovery         — dentro del logger (éste ve el 500) y fuera de ErrorHandler.
	//  5. ErrorHandler     — escribe el JSON de Fail/RespondWithError; todo lo que use
	//                        Fail debe ir DESPUÉS de él.
	//  6. CORS             — antes de auth/limitadores: el preflight no lleva Authorization.
	//  7. LimitarCuerpo    — 413 antes de que un handler lea el cuerpo.
	router.Use(
		middleware.RequestID(),
		middleware.SecurityHeaders(cfg.App.IsProduction()),
		middleware.RequestLogger(logger),
		middleware.Recovery(logger),
		middleware.ErrorHandler(),
		middleware.CORS(cfg.Server.CORSAllowedOrigins),
		middleware.LimitarCuerpo(cfg.Server.MaxBodyBytes),
	)

	// /healthz se mantiene tal cual (compatibilidad con probes ya configurados).
	router.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		if err := sqlDB.PingContext(ctx); err != nil {
			middleware.Fail(c, http.StatusServiceUnavailable, "db_unavailable", "No se pudo conectar a la base de datos", nil)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// /health: público, sin auth, con latencia y timestamp.
	router.GET("/health", handlers.NewDashboardHandler(deps.Dashboard).GetHealth)

	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api, sqlDB, tokenMaker, sesiones, deps)

	return router, nil
}