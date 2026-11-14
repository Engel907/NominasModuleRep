//go:build integration

package db

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"planillas-ccdr/internal/testutil"
)

func TestMigracionUltimaDownUp(t *testing.T) {
	db := testutil.NuevaBD(t)
	var database string
	if err := db.QueryRow(`SELECT DATABASE()`).Scan(&database); err != nil {
		t.Fatalf("identificar base temporal: %v", err)
	}

	cfg, err := mysql.ParseDSN(strings.TrimPrefix(strings.TrimSpace(os.Getenv(testutil.EnvURL)), "mysql://"))
	if err != nil {
		t.Fatalf("parsear %s: %v", testutil.EnvURL, err)
	}
	cfg.DBName = database
	cfg.MultiStatements = true

	migrationPath := filepath.Join(testutil.RaizRepo(t), "db", "migrations")
	sourceURL := "file://" + filepath.ToSlash(migrationPath)
	if volume := filepath.VolumeName(migrationPath); volume != "" {
		migrationPath = filepath.ToSlash(migrationPath)
		sourceURL = (&url.URL{
			Scheme: "file",
			Host:   volume,
			Path:   strings.TrimPrefix(migrationPath, filepath.ToSlash(volume)),
		}).String()
	}

	m, err := migrate.New(sourceURL, "mysql://"+cfg.FormatDSN())
	if err != nil {
		t.Fatalf("inicializar migrador: %v", err)
	}
	defer m.Close()

	versionInicial, dirty, err := m.Version()
	if err != nil {
		t.Fatalf("consultar versión inicial: %v", err)
	}
	if dirty {
		t.Fatal("la base temporal inicia con una migración marcada como dirty")
	}
	if err := m.Steps(-1); err != nil {
		t.Fatalf("bajar la última migración: %v", err)
	}
	if err := m.Steps(1); err != nil {
		t.Fatalf("volver a aplicar la última migración: %v", err)
	}
	version, dirty, err := m.Version()
	if err != nil {
		t.Fatalf("consultar versión final: %v", err)
	}
	if dirty || version != versionInicial {
		t.Fatalf("versión final = (%d, dirty=%t), se esperaba %d limpio", version, dirty, versionInicial)
	}
}

func TestMigracionL3SeedsTiposYConceptos(t *testing.T) {
	db := testutil.NuevaBD(t)

	var cantidadTipos, cantidadConceptos int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tipos_planilla`).Scan(&cantidadTipos); err != nil {
		t.Fatalf("contar tipos de planilla: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM tipo_planilla_concepto`).Scan(&cantidadConceptos); err != nil {
		t.Fatalf("contar conceptos por tipo: %v", err)
	}
	if cantidadTipos != 5 {
		t.Errorf("tipos de planilla sembrados = %d, se esperaban 5", cantidadTipos)
	}
	if cantidadConceptos != 28 {
		t.Errorf("relaciones de conceptos = %d, se esperaban 28 (14 conceptos para cada tipo regular)", cantidadConceptos)
	}
}
