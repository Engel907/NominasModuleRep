package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/api/handlers"
	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/auth"
	"planillas-ccdr/internal/config"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/planilla/infra"
	"planillas-ccdr/internal/prestamos"
	"planillas-ccdr/internal/security"
	"planillas-ccdr/internal/services"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	sqlDB, err := connectDB(cfg.Database)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer func() {
		if cerr := sqlDB.Close(); cerr != nil {
			log.Printf("db: error cerrando el pool: %v", cerr)
		}
	}()

	// Falla rápido en el arranque, no en el primer request: una clave
	// PASETO mal configurada o un servicio mal armado son errores de
	// despliegue, no algo que deba descubrirse en producción con el primer
	// login.
	tokenMaker, err := newTokenMaker(cfg.Auth)
	if err != nil {
		log.Fatalf("security: %v", err)
	}

	deps, err := buildDeps(cfg, sqlDB, tokenMaker)
	if err != nil {
		log.Fatalf("deps: %v", err)
	}

	router := newRouter(cfg, sqlDB, tokenMaker, deps)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		log.Printf("planillas-ccdr escuchando en :%s (env=%s)", cfg.Server.Port, cfg.App.Env)
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("server: %v", serveErr)
		}
	}()

	waitForShutdown(srv, cfg.Server.ShutdownTimeout)
}

// connectDB abre el pool de conexiones a MySQL con timeouts explícitos:
// tanto el timeout de conexión (en el propio DSN, ver DatabaseConfig.DSN)
// como un PingContext acotado para no colgar el arranque si la BD no
// responde, y los límites de pool recomendados para no agotar
// max_connections del lado del servidor.
func connectDB(dbCfg config.DatabaseConfig) (*sql.DB, error) {
	sqlDB, err := sql.Open("mysql", dbCfg.DSN())
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(dbCfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(dbCfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(dbCfg.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), dbCfg.PingTimeout)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return sqlDB, nil
}

// buildDeps construye los servicios de los módulos que dependen de la capa
// de datos (internal/db/dto) y los agrupa en handlers.Deps. Es el único
// lugar donde se conecta la capa de datos con los servicios: routes.go solo
// monta rutas. Los servicios de Fase 3 se arman dentro de
// handlers.RegisterRoutes porque reciben *sql.DB directo.
//
// Verificado contra el código fuente: auth, dashboard y liquidación.
// Pendiente de verificar (VERIFICAR:): asistencia y préstamos, cuyo código
// no he visto. Export de planilla NO tiene implementación concreta de
// ExportRepository en el repo; ver el comentario en PlanillaExport.
func buildDeps(cfg *config.Config, sqlDB *sql.DB, tokenMaker security.TokenMaker) (handlers.Deps, error) {
	// Verificado contra internal/db/dto/db.go: New(DBTX) *Queries.
	queries := dto.New(sqlDB)

	asistenciaSvc, err := asistencia.NewService(sqlDB, queries, asistencia.Opciones{
		ToleranciaTardanzaMin: cfg.Asistencia.ToleranciaTardanzaMin,
	})
	if err != nil {
		return handlers.Deps{}, fmt.Errorf("asistencia: %w", err)
	}

	return handlers.Deps{
		// auth.Store son métodos de *dto.Queries (ver auth/service.go).
		Auth: auth.NuevoServicio(queries, sqlDB, tokenMaker, politicaAuth(cfg.Auth)),

		// VERIFICAR: firma de asistencia.NewService y que appdb.WithAuditContext
		// tenga la forma que asistencia espera (ver nota sobre las dos
		// firmas distintas de WithAuditContext en el resumen).
		Asistencia: asistenciaSvc,

		// VERIFICAR: que *dto.Queries satisfaga prestamos.Querier.
		Prestamos: prestamos.NewService(sqlDB, queries),

		// Firma tomada de dashboard_service.go (NewDashboardService).
		Dashboard: services.NewDashboardService(services.NewDashboardRepository(queries), sqlDB, 3*time.Second),

		// Firma tomada de liquidacion_service.go. Requiere que *dto.Queries
		// implemente services.LiquidacionRepository.
		Liquidacion: services.NewLiquidacionService(sqlDB, queries, infra.NewMotorRepository(queries)),

		PlanillaExport: planilla.NewExportService(infra.NewExportRepository(queries), infra.NewExportUnitOfWork(sqlDB)),

		Vacaciones: asistencia.NewVacacionesService(sqlDB, queries),

		// PlanillaExport: sin asignar a propósito. planilla.ExportRepository
		// devuelve tipos propios de planilla (FilaDispersionBancaria,
		// EncabezadoBoletaPago, ...), no filas de dto, así que *dto.Queries
		// no lo satisface directo y hace falta un adaptador (como
		// services.NewDashboardRepository) que no existe todavía. RegisterRoutes
		// omite estas dos rutas mientras el campo sea nil.
	}, nil
}

// politicaAuth aplica la configuración a auth.Politica. Los campos salen de
// auth/service.go. Ojo: PASETO_ISSUER y PASETO_AUDIENCE de la config siguen
// sin consumirse (NuevoPasetoMaker solo recibe la clave).
func politicaAuth(a config.AuthConfig) auth.Politica {
	return auth.Politica{
		MaxIntentosFallidos:  a.MaxFailedLogins,
		MinutosBloqueo:       a.LockoutDuration,
		DuracionAccessToken:  a.AccessTokenDuration,
		DuracionRefreshToken: a.RefreshTokenDuration,
		DuracionTokenReset:   a.PasswordResetTokenDuration,
	}
}

// newRouter arma el engine de Gin con el middleware de manejo centralizado
// de errores ya montado globalmente (ver /api/middleware) y delega el
// montaje de todas las rutas de negocio en handlers.RegisterRoutes.
func newRouter(cfg *config.Config, sqlDB *sql.DB, tokenMaker security.TokenMaker, deps handlers.Deps) *gin.Engine {
	if cfg.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(middleware.ErrorHandler())

	// /healthz se mantiene tal cual (compatibilidad con probes ya
	// configurados en el orquestador).
	router.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		if err := sqlDB.PingContext(ctx); err != nil {
			middleware.Fail(c, http.StatusServiceUnavailable, "db_unavailable", "No se pudo conectar a la base de datos", nil)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// /health (Fase 8): público, sin auth, con latencia y timestamp. Coexiste
	// con /healthz; ver la nota de integración en dashboard_handler.go.
	router.GET("/health", handlers.NewDashboardHandler(deps.Dashboard).GetHealth)

	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api, sqlDB, tokenMaker, deps)

	return router
}

// newTokenMaker decodifica PASETO_LOCAL_KEY_HEX (64 caracteres hex = 32
// bytes) y construye el PasetoMaker real. config.Validate() ya garantiza
// el largo del string hex, pero el hex-decode y el chequeo de 32 bytes
// resultantes se hacen igual acá por si esta función se llama fuera de
// ese flujo (tests, por ejemplo).
func newTokenMaker(authCfg config.AuthConfig) (security.TokenMaker, error) {
	claveBytes, err := hex.DecodeString(authCfg.PasetoLocalKeyHex)
	if err != nil {
		return nil, fmt.Errorf("security: PASETO_LOCAL_KEY_HEX no es hex válido: %w", err)
	}
	maker, err := security.NuevoPasetoMaker(string(claveBytes))
	if err != nil {
		return nil, err
	}
	return maker, nil
}

// waitForShutdown bloquea hasta recibir SIGINT/SIGTERM y entonces intenta
// un apagado ordenado del servidor HTTP dentro del timeout configurado.
func waitForShutdown(srv *http.Server, timeout time.Duration) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("apagando servidor...")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server: apagado forzado, error: %v", err)
	}
}
