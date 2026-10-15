package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/app"
	"planillas-ccdr/internal/config"
	"planillas-ccdr/internal/parametros"
)

func main() {
	// Logger de arranque (antes de conocer LOG_LEVEL): JSON a stdout.
	slog.SetDefault(middleware.NuevoLogger(os.Stdout, slog.LevelInfo))

	cfg, err := config.LoadConfig()
	if err != nil {
		fatal("config", err)
	}

	// F16 (S16): logger definitivo. SetDefault también redirige el paquete
	// estándar "log": cualquier log.Printf que quede en el código sale por
	// slog (JSON) en vez de texto plano; conviene migrarlos igualmente
	// (grep -rn 'log\.\(Printf\|Println\|Fatalf\)' internal) para que lleven
	// atributos y request_id.
	slog.SetDefault(middleware.NuevoLogger(os.Stdout, cfg.App.SlogLevel()))

	if cfg.App.IsProduction() && cfg.Database.TLSMode == "required" {
		slog.Warn("DB_TLS_MODE=required cifra la conexión pero NO verifica el certificado del servidor; se recomienda verify-full")
	}

	sqlDB, err := connectDB(cfg.Database)
	if err != nil {
		fatal("db", err)
	}
	defer func() {
		if cerr := sqlDB.Close(); cerr != nil {
			slog.Error("db: error cerrando el pool", slog.String("error", cerr.Error()))
		}
	}()

	// F18A: servicio de parámetros generales (caché con TTL de PARAMETROS_CACHE_TTL).
	// F18B lo inyectará en app.New para que el cargador lo use; hasta entonces
	// nadie lo consume, de ahí el "_ =".
	generales := parametros.NewGenerales(parametros.NewRepositorioSQL(sqlDB), cfg.Parametros.CacheTTL)
	_ = generales

	// Falla rápido en el arranque: una clave PASETO mal configurada o un
	// servicio mal armado son errores de despliegue, no del primer login.
	tokenMaker, err := app.NewTokenMaker(cfg.Auth)
	if err != nil {
		fatal("security", err)
	}

	router, err := app.New(cfg, sqlDB, tokenMaker)
	if err != nil {
		fatal("app", err)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadTimeout, // F16: Slowloris sobre las cabeceras
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	go func() {
		slog.Info("planillas-ccdr escuchando",
			slog.String("addr", srv.Addr),
			slog.String("env", cfg.App.Env),
			slog.String("db_tls", cfg.Database.TLSMode),
		)
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			fatal("server", serveErr)
		}
	}()

	waitForShutdown(srv, cfg.Server.ShutdownTimeout)
}

// fatal registra el error y termina el proceso (equivale al antiguo log.Fatalf).
func fatal(contexto string, err error) {
	slog.Error("arranque fallido", slog.String("etapa", contexto), slog.String("error", err.Error()))
	os.Exit(1)
}

// connectDB abre el pool de conexiones a MySQL con timeouts explícitos y un
// PingContext acotado para no colgar el arranque si la BD no responde. Si
// DB_TLS_MODE no es disabled, registra primero la config TLS en el driver
// (el DSN trae tls=<nombre>).
func connectDB(dbCfg config.DatabaseConfig) (*sql.DB, error) {
	if err := dbCfg.RegistrarTLS(); err != nil {
		return nil, err
	}

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

// waitForShutdown bloquea hasta recibir SIGINT/SIGTERM y entonces intenta un
// apagado ordenado del servidor HTTP dentro del timeout configurado.
func waitForShutdown(srv *http.Server, timeout time.Duration) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("apagando servidor...")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server: apagado forzado", slog.String("error", err.Error()))
	}
}