package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"time"

	"planillas-ccdr/internal/config"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla/infra"
	"planillas-ccdr/internal/rrhh"
)

const timeoutCierreSalidas = 10 * time.Minute

// cmdCerrarSalidas: cerrar-salidas --usuario-id N [--fecha YYYY-MM-DD].
func cmdCerrarSalidas(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cerrar-salidas", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fechaFlag := fs.String("fecha", "", "fecha de corrida YYYY-MM-DD (por defecto, hoy)")
	usuarioFlag := fs.Int64("usuario-id", 0, "usuario que queda en la auditoría de los cierres (obligatorio)")
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
	if *usuarioFlag <= 0 {
		fmt.Fprintln(stderr, "error: --usuario-id es obligatorio (auditoría de los cierres)")
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

	ctx, cancel := context.WithTimeout(ctx, timeoutCierreSalidas)
	defer cancel()
	q := dto.New(sqlDB)
	g := parametros.NewGenerales(parametros.NewRepositorioSQL(sqlDB), 0)
	svc := rrhh.NewLiquidacionService(sqlDB, q, infra.NewMotorRepository(q), g)
	res, err := svc.CerrarSalidasPendientes(ctx, fecha, *usuarioFlag)
	return informarCierres(res, err, fecha, stdout, stderr)
}

func informarCierres(res rrhh.ResumenCierres, err error, fecha time.Time, stdout, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "error cerrando salidas: %v\n", err)
		return codigoError
	}
	fmt.Fprintf(stdout, "Cierre de salidas al %s: %d cerradas, %d con error.\n", fecha.Format(formatoFecha), res.Cerradas, len(res.Errores))
	if len(res.Errores) == 0 {
		return codigoOK
	}
	ids := make([]int64, 0, len(res.Errores))
	for id := range res.Errores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		fmt.Fprintf(stdout, "  - liquidación %d: %s\n", id, res.Errores[id])
	}
	return codigoFaltantes
}
