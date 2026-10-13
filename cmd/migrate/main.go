// Comando migrate: aplica y revierte las migraciones de db/migrations con
// golang-migrate. Es una herramienta manual (CI, despliegues, desarrollo):
// la API NUNCA corre migraciones al iniciar.
//
// Uso:
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down N
//	go run ./cmd/migrate version
//	go run ./cmd/migrate force V
//
// Conexión: DATABASE_URL (p.ej. mysql://user:pass@tcp(host:3306)/db) o, si no
// existe, las variables DB_* de internal/config (DB_HOST, DB_PORT, DB_USER,
// DB_PASSWORD, DB_NAME, DB_PARAMS, DB_CONNECT_TIMEOUT_SECONDS). También se lee
// un .env en el directorio actual. Carpeta de migraciones: MIGRATIONS_PATH
// (por defecto db/migrations).
package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/viper"

	"planillas-ccdr/internal/config"
)

const rutaMigracionesPorDefecto = "db/migrations"

const textoUso = `uso: migrate <comando>

comandos:
  up          aplica todas las migraciones pendientes
  down N      revierte las últimas N migraciones (N obligatorio, > 0)
  version     muestra la versión actual y si está en estado "dirty"
  force V     fuerza la versión V sin ejecutar SQL (V = -1 borra el registro);
              úsese solo para recuperarse de un estado "dirty" ya corregido a mano`

// errUso distingue los errores de invocación (código 2) de los de ejecución
// (código 1).
type errUso string

func (e errUso) Error() string { return string(e) }

func main() {
	if err := ejecutar(os.Args[1:]); err != nil {
		var uso errUso
		if errors.As(err, &uso) {
			fmt.Fprintln(os.Stderr, uso.Error())
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "migrate: error:", err)
		os.Exit(1)
	}
}

func ejecutar(args []string) error {
	if len(args) == 0 {
		return errUso(textoUso)
	}
	comando := args[0]

	// Se validan los argumentos antes de abrir la conexión.
	var n int
	switch comando {
	case "up", "version":
		if len(args) != 1 {
			return errUso(textoUso)
		}
	case "down", "force":
		if len(args) != 2 {
			return errUso(textoUso)
		}
		v, err := strconv.Atoi(args[1])
		if err != nil {
			return errUso(fmt.Sprintf("migrate: %q no es un entero válido\n\n%s", args[1], textoUso))
		}
		if comando == "down" && v <= 0 {
			return errUso("migrate: down requiere N > 0 (no se permite revertir todo sin indicar cuántas)")
		}
		if comando == "force" && v < -1 {
			return errUso("migrate: force requiere V >= -1")
		}
		n = v
	default:
		return errUso(fmt.Sprintf("migrate: comando desconocido %q\n\n%s", comando, textoUso))
	}

	urlBD, err := urlBaseDatos()
	if err != nil {
		return err
	}
	ruta := os.Getenv("MIGRATIONS_PATH")
	if ruta == "" {
		ruta = rutaMigracionesPorDefecto
	}
	m, err := migrate.New("file://"+filepath.ToSlash(ruta), urlBD)
	if err != nil {
		return fmt.Errorf("no se pudo inicializar golang-migrate: %w", err)
	}
	defer m.Close()

	if err := imprimirVersion(m, "versión antes"); err != nil {
		return err
	}

	switch comando {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-n)
	case "force":
		err = m.Force(n)
	case "version":
		return nil
	}
	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("sin cambios: no había migraciones por aplicar")
		err = nil
	}
	if err != nil {
		// Se intenta mostrar la versión final aunque haya fallado (puede quedar dirty).
		_ = imprimirVersion(m, "versión tras el error")
		return fmt.Errorf("%s falló: %w", comando, err)
	}
	return imprimirVersion(m, "versión después")
}

func imprimirVersion(m *migrate.Migrate, etiqueta string) error {
	v, sucia, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		fmt.Printf("%s: ninguna migración aplicada\n", etiqueta)
		return nil
	}
	if err != nil {
		return fmt.Errorf("no se pudo leer la versión: %w", err)
	}
	if sucia {
		fmt.Printf("%s: %d (DIRTY: una migración falló a medias; corrija a mano y use force)\n", etiqueta, v)
		return nil
	}
	fmt.Printf("%s: %d\n", etiqueta, v)
	return nil
}

// urlBaseDatos arma la URL para el driver mysql de golang-migrate y fuerza
// multiStatements=true (los CREATE TRIGGER ... BEGIN ... END; y los scripts con
// varias sentencias lo necesitan). No usa config.LoadConfig a propósito: su
// validación exige claves de PASETO que este comando no necesita.
func urlBaseDatos() (string, error) {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AutomaticEnv()
	v.SetDefault("DB_HOST", "127.0.0.1")
	v.SetDefault("DB_PORT", "3306")
	v.SetDefault("DB_CONNECT_TIMEOUT_SECONDS", 5)

	if err := v.ReadInConfig(); err != nil {
		var noHay viper.ConfigFileNotFoundError
		if !errors.As(err, &noHay) {
			return "", fmt.Errorf("error leyendo .env: %w", err)
		}
	}

	if u := v.GetString("DATABASE_URL"); u != "" {
		return conMultiStatements(u)
	}

	bd := config.DatabaseConfig{
		Host:           v.GetString("DB_HOST"),
		Port:           v.GetString("DB_PORT"),
		User:           v.GetString("DB_USER"),
		Password:       v.GetString("DB_PASSWORD"),
		Name:           v.GetString("DB_NAME"),
		ExtraParams:    v.GetString("DB_PARAMS"),
		ConnectTimeout: v.GetDuration("DB_CONNECT_TIMEOUT_SECONDS") * time.Second,
	}
	var faltan []string
	if bd.User == "" {
		faltan = append(faltan, "DB_USER")
	}
	if bd.Name == "" {
		faltan = append(faltan, "DB_NAME")
	}
	if len(faltan) > 0 {
		return "", fmt.Errorf("faltan variables de entorno requeridas (o defina DATABASE_URL): %v", faltan)
	}
	return conMultiStatements(bd.DSN())
}

// conMultiStatements agrega el prefijo mysql:// y multiStatements=true a un DSN
// de go-sql-driver/mysql. Conserva el resto de los parámetros.
func conMultiStatements(dsn string) (string, error) {
	dsn = strings.TrimPrefix(dsn, "mysql://")
	base, consulta := dsn, ""
	if i := strings.LastIndex(dsn, "?"); i >= 0 {
		base, consulta = dsn[:i], dsn[i+1:]
	}
	params, err := url.ParseQuery(consulta)
	if err != nil {
		return "", fmt.Errorf("parámetros de conexión inválidos: %w", err)
	}
	params.Set("multiStatements", "true")
	return "mysql://" + base + "?" + params.Encode(), nil
}
