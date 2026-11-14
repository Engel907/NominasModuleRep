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
	"planillas-ccdr/internal/rrhh"
)

const timeoutCifrado = 30 * time.Minute

// cmdCifrarExistentes: cifrar-existentes --usuario-id N [--lote 500] [--recifrar].
func cmdCifrarExistentes(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cifrar-existentes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	usuarioFlag := fs.Int64("usuario-id", 0, "usuario que queda en la auditoría (obligatorio)")
	loteFlag := fs.Int("lote", 500, "filas por transacción")
	recifrarFlag := fs.Bool("recifrar", false, "volver a cifrar con la llave activa lo cifrado con otra")
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
	if *usuarioFlag <= 0 || *loteFlag <= 0 {
		fmt.Fprintln(stderr, "error: --usuario-id es obligatorio y --lote debe ser > 0")
		return codigoError
	}
	cfg, err := config.LoadJobsConfig()
	if err != nil {
		fmt.Fprintf(stderr, "error de configuración: %v\n", err)
		return codigoError
	}
	llavero, err := cfg.Datos.Llavero()
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
	ctx, cancel := context.WithTimeout(ctx, timeoutCifrado)
	defer cancel()
	res, err := rrhh.CifrarExistentes(ctx, sqlDB, llavero, *usuarioFlag, *loteFlag, *recifrarFlag)
	if err != nil {
		fmt.Fprintf(stderr, "error cifrando datos existentes: %v\n", err)
		return codigoError
	}
	campos := make([]string, 0, len(res.PorCampo))
	for c := range res.PorCampo {
		campos = append(campos, c)
	}
	sort.Strings(campos)
	fmt.Fprintf(stdout, "Cifrado de datos personales (llave %s): %d valores.\n", llavero.KidActivo(), res.Total)
	for _, c := range campos {
		fmt.Fprintf(stdout, "  - %s: %d\n", c, res.PorCampo[c])
	}
	return codigoOK
}
