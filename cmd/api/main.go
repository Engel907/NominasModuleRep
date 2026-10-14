package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/app"
	"planillas-ccdr/internal/config"
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

	// Falla rápido en el arranque: una clave PASETO mal configurada o un
	// servicio mal armado son errores de despliegue, no del primer login.
	tokenMaker, err := app.NewTokenMaker(cfg.Auth)
	if err != nil {
		log.Fatalf("security: %v", err)
	}

	router, err := app.New(cfg, sqlDB, tokenMaker)
	if err != nil {
		log.Fatalf("app: %v", err)
	}

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

// connectDB abre el pool de conexiones a MySQL con timeouts explícitos y un
// PingContext acotado para no colgar el arranque si la BD no responde.
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

// waitForShutdown bloquea hasta recibir SIGINT/SIGTERM y entonces intenta un
// apagado ordenado del servidor HTTP dentro del timeout configurado.
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