// Comando de jobs/batch de planillas. Subcomandos:
//
//	verificar-parametros [--fecha YYYY-MM-DD]
//
// Códigos de salida: 0 = todo en orden; 1 = hay requeridos sin valor vigente;
// 2 = uso incorrecto o error operativo (config, BD, tipo/valor ilegible).
// L25 y L27 agregan más subcomandos.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/config"
	"planillas-ccdr/internal/parametros"
)

const (
	formatoFecha    = "2006-01-02"
	codigoOK        = 0
	codigoFaltantes = 1
	codigoError     = 2
	timeoutComando  = 30 * time.Second
)

func main() {
	os.Exit(ejecutar(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func ejecutar(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		uso(stderr)
		return codigoError
	}
	switch args[0] {
	case "verificar-parametros":
		return cmdVerificarParametros(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		uso(stdout)
		return codigoOK
	default:
		fmt.Fprintf(stderr, "subcomando desconocido: %q\n", args[0])
		uso(stderr)
		return codigoError
	}
}

func uso(w io.Writer) {
	fmt.Fprintln(w, "uso: jobs <subcomando> [opciones]")
	fmt.Fprintln(w, "  verificar-parametros [--fecha YYYY-MM-DD]   lista los parámetros requeridos sin valor vigente (sale con 1 si hay alguno)")
}

func cmdVerificarParametros(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verificar-parametros", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fechaFlag := fs.String("fecha", "", "fecha de vigencia YYYY-MM-DD (por defecto, hoy)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return codigoOK
		}
		return codigoError
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "argumentos no esperados: %v\n", fs.Args())
		return codigoError
	}

	fecha, err := resolverFecha(*fechaFlag, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return codigoError
	}

	cfg, err := config.LoadJobsConfig()
	if err != nil {
		fmt.Fprintf(stderr, "error de configuración: %v\n", err)
		return codigoError
	}
	sqlDB, err := connectDB(cfg.Database)
	if err != nil {
		fmt.Fprintf(stderr, "error conectando a la BD: %v\n", err)
		return codigoError
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(ctx, timeoutComando)
	defer cancel()

	// TTL 0: un job siempre lee el valor fresco de la BD.
	g := parametros.NewGenerales(parametros.NewRepositorioSQL(sqlDB), 0)
	return verificar(ctx, g, fecha, stdout, stderr)
}

type proveedorFaltantes interface {
	Faltantes(ctx context.Context, fecha time.Time) ([]parametros.Requerido, error)
}

func verificar(ctx context.Context, p proveedorFaltantes, fecha time.Time, stdout, stderr io.Writer) int {
	faltan, err := p.Faltantes(ctx, fecha)
	if err != nil {
		fmt.Fprintf(stderr, "error verificando parámetros: %v\n", err)
		return codigoError
	}
	f := fecha.Format(formatoFecha)
	if len(faltan) == 0 {
		fmt.Fprintf(stdout, "Todos los parámetros requeridos tienen valor vigente al %s.\n", f)
		return codigoOK
	}
	fmt.Fprintf(stdout, "Parámetros requeridos sin valor vigente al %s:\n", f)
	for _, r := range faltan {
		fmt.Fprintf(stdout, "  - %s (%s): %s\n", r.Clave, r.Tipo, r.Descripcion)
	}
	return codigoFaltantes
}

func resolverFecha(valor string, ahora time.Time) (time.Time, error) {
	if valor == "" {
		ahora = ahora.UTC()
		return time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.ParseInLocation(formatoFecha, valor, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("--fecha %q inválida, formato esperado YYYY-MM-DD", valor)
	}
	return t, nil
}

// connectDB abre el pool a MySQL con los mismos timeouts y TLS que cmd/api.
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
