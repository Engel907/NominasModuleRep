package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"time"

	"planillas-ccdr/internal/asistencia"
	"planillas-ccdr/internal/config"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
)

// timeoutAcumulacion: recorre todos los empleados activos (una transacción por empleado).
const timeoutAcumulacion = 10 * time.Minute

// cmdAcumularVacaciones: acumular-vacaciones --usuario-id N [--fecha YYYY-MM-DD].
func cmdAcumularVacaciones(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("acumular-vacaciones", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fechaFlag := fs.String("fecha", "", "fecha de corrida YYYY-MM-DD (por defecto, hoy)")
	usuarioFlag := fs.Int64("usuario-id", 0, "usuario que queda como creado_por de los movimientos (obligatorio)")
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
		fmt.Fprintln(stderr, "error: --usuario-id es obligatorio (auditoría de los movimientos)")
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

	ctx, cancel := context.WithTimeout(ctx, timeoutAcumulacion)
	defer cancel()

	g := parametros.NewGenerales(parametros.NewRepositorioSQL(sqlDB), 0)
	politica, err := g.Texto(ctx, asistencia.ClaveVacacionesPoliticaAcumulacion, fecha)
	if err != nil {
		fmt.Fprintf(stderr, "error leyendo la política de acumulación: %v\n", err)
		return codigoError
	}
	q := dto.New(sqlDB)
	svc := asistencia.NewVacacionesService(sqlDB, q)
	res, err := svc.AcumularSegunPolitica(ctx, q, politica, fecha, *usuarioFlag)
	return informarAcumulacion(res, err, politica, fecha, stdout, stderr)
}

func informarAcumulacion(res asistencia.ResumenAcumulacion, err error, politica string, fecha time.Time, stdout, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "error acumulando vacaciones: %v\n", err)
		return codigoError
	}
	fmt.Fprintf(stdout, "Acumulación de vacaciones al %s (política %s): %d acreditados, %d ya registrados, %d sin derecho todavía, %d con error.\n",
		fecha.Format(formatoFecha), politica, res.Acreditados, res.YaRegistrados, res.SinDerecho, len(res.Errores))
	if len(res.Errores) == 0 {
		return codigoOK
	}
	ids := make([]int64, 0, len(res.Errores))
	for id := range res.Errores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		fmt.Fprintf(stdout, "  - empleado %d: %s\n", id, res.Errores[id])
	}
	return codigoFaltantes
}
