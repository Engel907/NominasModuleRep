//go:build integration

package db

import (
	"database/sql"
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

// TestMigracionesDownUp baja y reaplica las últimas seis migraciones.
func TestMigracionesDownUp(t *testing.T) {
	const pasos = 6
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
	if err := m.Steps(-pasos); err != nil {
		t.Fatalf("bajar %d migraciones: %v", pasos, err)
	}
	if err := m.Steps(pasos); err != nil {
		t.Fatalf("reaplicar %d migraciones: %v", pasos, err)
	}
	version, dirty, err := m.Version()
	if err != nil {
		t.Fatalf("consultar versión final: %v", err)
	}
	if dirty || version != versionInicial {
		t.Fatalf("versión final = (%d, dirty=%t), se esperaba %d limpio", version, dirty, versionInicial)
	}
}

// nuevoMigrador: golang-migrate sobre una BD efímera ya migrada.
func nuevoMigrador(t *testing.T) (*sql.DB, *migrate.Migrate) {
	t.Helper()
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
	t.Cleanup(func() { m.Close() })
	return db, m
}

// TestEnmascaraAuditoriaHistorica: 000139 enmascara los valores sensibles
// que la auditoría ya tenía en claro (irreversible).
func TestEnmascaraAuditoriaHistorica(t *testing.T) {
	db, m := nuevoMigrador(t)
	if err := m.Migrate(138); err != nil {
		t.Fatalf("bajar 000139: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
VALUES ('empleados', 1, NULL, 'insert', NULL, JSON_OBJECT('id', 1, 'numero_identificacion', '1-0234-0567', 'nombre_completo', 'Ana')),
       ('empleado_datos_bancarios', 1, NULL, 'update', JSON_OBJECT('numero_cuenta', 'CR050152'), JSON_OBJECT('numero_cuenta', 'CR990152'))`); err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(139); err != nil {
		t.Fatalf("reaplicar 000139: %v", err)
	}
	var ced, ant, nvo, nombre string
	if err := db.QueryRow(`SELECT JSON_UNQUOTE(JSON_EXTRACT(valor_nuevo, '$.numero_identificacion')), JSON_UNQUOTE(JSON_EXTRACT(valor_nuevo, '$.nombre_completo'))
FROM auditoria_cambios WHERE tabla_afectada = 'empleados' AND registro_id = 1`).Scan(&ced, &nombre); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT JSON_UNQUOTE(JSON_EXTRACT(valor_anterior, '$.numero_cuenta')), JSON_UNQUOTE(JSON_EXTRACT(valor_nuevo, '$.numero_cuenta'))
FROM auditoria_cambios WHERE tabla_afectada = 'empleado_datos_bancarios'`).Scan(&ant, &nvo); err != nil {
		t.Fatal(err)
	}
	if ced != "***" || ant != "***" || nvo != "***" || nombre != "Ana" {
		t.Fatalf("cédula %q, cuenta %q→%q, nombre %q", ced, ant, nvo, nombre)
	}
}

func TestMigracionSeedsTiposYConceptos(t *testing.T) {
	db := testutil.NuevaBD(t)

	var cantidadTipos, cantidadConceptos int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tipos_planilla`).Scan(&cantidadTipos); err != nil {
		t.Fatalf("contar tipos de planilla: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM tipo_planilla_concepto x
JOIN tipos_planilla t ON t.id = x.tipo_planilla_id
JOIN conceptos_salariales c ON c.id = x.concepto_salarial_id
WHERE t.clase = 'regular' AND c.codigo <> 'FERIADO_LAB'`).Scan(&cantidadConceptos); err != nil {
		t.Fatalf("contar conceptos por tipo: %v", err)
	}
	if cantidadTipos != 5 {
		t.Errorf("tipos de planilla sembrados = %d, se esperaban 5", cantidadTipos)
	}
	if cantidadConceptos != 28 {
		t.Errorf("relaciones de conceptos = %d, se esperaban 28 (14 conceptos para cada tipo regular)", cantidadConceptos)
	}
}
