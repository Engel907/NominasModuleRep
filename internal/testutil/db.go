// Package testutil agrupa helpers para pruebas de integración.
//
// NuevaBD crea una base MySQL temporal (nombre aleatorio), le aplica las
// migraciones de db/migrations con golang-migrate y la elimina al terminar
// el test. Requiere TEST_DATABASE_URL; si no está definida, el test se salta.
package testutil

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// EnvURL es la variable con el DSN de un servidor MySQL 8 donde el usuario
// puede CREATE/DROP DATABASE y crear triggers (p. ej. root del docker-compose).
// Formato go-sql-driver (el prefijo mysql:// es opcional):
//
//	root:root_dev@tcp(127.0.0.1:3307)/
const EnvURL = "TEST_DATABASE_URL"

// RaizRepo sube desde el directorio actual hasta encontrar go.mod.
func RaizRepo(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("testutil: getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			t.Fatal("testutil: no se encontró go.mod subiendo desde el directorio de trabajo")
		}
		dir = padre
	}
}

// NuevaBD devuelve un *sql.DB conectado a una base temporal ya migrada.
func NuevaBD(t *testing.T) *sql.DB {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv(EnvURL))
	if raw == "" {
		t.Skipf("%s no definida: se salta la prueba de integración", EnvURL)
	}
	cfg, err := mysql.ParseDSN(strings.TrimPrefix(raw, "mysql://"))
	if err != nil {
		t.Fatalf("testutil: %s inválida: %v", EnvURL, err)
	}
	cfg.DBName = ""
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	if _, ok := cfg.Params["charset"]; !ok {
		cfg.Params["charset"] = "utf8mb4"
	}

	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatalf("testutil: abrir conexión admin: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Fatalf("testutil: no se pudo conectar a MySQL (%s): %v", EnvURL, err)
	}

	suf := make([]byte, 6)
	if _, err := rand.Read(suf); err != nil {
		t.Fatalf("testutil: rand: %v", err)
	}
	nombre := "ccdr_test_" + hex.EncodeToString(suf)

	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+nombre+"` CHARACTER SET utf8mb4"); err != nil {
		_ = admin.Close()
		t.Fatalf("testutil: CREATE DATABASE %s: %v", nombre, err)
	}
	t.Cleanup(func() {
		c, cn := context.WithTimeout(context.Background(), 30*time.Second)
		defer cn()
		if _, err := admin.ExecContext(c, "DROP DATABASE IF EXISTS `"+nombre+"`"); err != nil {
			t.Logf("testutil: no se pudo eliminar %s: %v", nombre, err)
		}
		_ = admin.Close()
	})

	// Migraciones con una conexión propia (multiStatements=true).
	mig := *cfg
	mig.DBName = nombre
	mig.MultiStatements = true
	if err := migrar(t, &mig); err != nil {
		t.Fatalf("testutil: migraciones: %v", err)
	}

	cfg.DBName = nombre
	sqlDB, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatalf("testutil: abrir BD de prueba: %v", err)
	}
	sqlDB.SetMaxOpenConns(10)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("testutil: ping BD de prueba: %v", err)
	}
	return sqlDB
}

func migrar(t testing.TB, cfg *mysql.Config) error {
	ruta := filepath.ToSlash(filepath.Join(RaizRepo(t), "db", "migrations"))
	if !strings.HasPrefix(ruta, "/") { // Windows: C:/... -> /C:/...
		ruta = "/" + ruta
	}
	m, err := migrate.New("file://"+ruta, "mysql://"+cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("inicializar golang-migrate: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("up: %w", err)
	}
	return nil
}