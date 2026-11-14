package rrhh

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"planillas-ccdr/internal/cripto"
	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

// ResumenCifrado: filas cifradas o recifradas por campo ("tabla.columna").
type ResumenCifrado struct {
	PorCampo map[string]int
	Total    int
}

func (r *ResumenCifrado) sumar(c dto.CampoSensible, n int) {
	r.PorCampo[c.Campo()] += n
	r.Total += n
}

// CifrarExistentes: lote = filas por transacción (> 0).
func CifrarExistentes(ctx context.Context, sqlDB *sql.DB, llavero *cripto.Llavero, usuarioID int64, lote int, recifrar bool) (ResumenCifrado, error) {
	res := ResumenCifrado{PorCampo: map[string]int{}}
	if llavero == nil {
		return res, cripto.ErrLlaveroNoProvisto
	}
	if lote <= 0 || usuarioID <= 0 {
		return res, errors.New("cifrar-existentes: lote y usuario deben ser > 0")
	}
	q := dto.New(sqlDB)
	for _, c := range dto.CamposSensibles {
		enClaro, err := q.ExisteColumnaEnClaro(ctx, c)
		if err != nil {
			return res, fmt.Errorf("cifrar-existentes: %s: %w", c.Campo(), err)
		}
		if enClaro {
			if err := cifrarEnClaro(ctx, sqlDB, q, llavero, usuarioID, lote, c, &res); err != nil {
				return res, err
			}
		}
		if recifrar {
			if err := rotarLlave(ctx, sqlDB, q, llavero, usuarioID, lote, c, &res); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

func guardar(ctx context.Context, tx *dto.Queries, llavero *cripto.Llavero, c dto.CampoSensible, id int64, valor string) error {
	ct, kid, err := llavero.Cifrar(c.Campo(), valor)
	if err != nil {
		return err
	}
	return tx.GuardarCifrado(ctx, c, id, dto.ValorCifrado{Cifrado: ct, Hash: llavero.HashBusqueda(c.Campo(), valor), Kid: kid})
}

func cifrarEnClaro(ctx context.Context, sqlDB *sql.DB, q *dto.Queries, llavero *cripto.Llavero, usuarioID int64, lote int, c dto.CampoSensible, res *ResumenCifrado) error {
	var desde int64
	for {
		filas, err := q.ListarPendientesCifrado(ctx, c, desde, lote)
		if err != nil {
			return fmt.Errorf("cifrar-existentes: listando %s: %w", c.Campo(), err)
		}
		if len(filas) == 0 {
			return nil
		}
		err = db.WithAuditContext(ctx, sqlDB, usuarioID, func(ctx context.Context, tx *dto.Queries) error {
			for _, f := range filas {
				if err := guardar(ctx, tx, llavero, c, f.ID, f.Valor); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("cifrar-existentes: %s desde id %d: %w", c.Campo(), desde, err)
		}
		res.sumar(c, len(filas))
		desde = filas[len(filas)-1].ID
	}
}

func rotarLlave(ctx context.Context, sqlDB *sql.DB, q *dto.Queries, llavero *cripto.Llavero, usuarioID int64, lote int, c dto.CampoSensible, res *ResumenCifrado) error {
	var desde int64
	for {
		filas, err := q.ListarCifradosConOtraLlave(ctx, c, llavero.KidActivo(), desde, lote)
		if err != nil {
			return fmt.Errorf("recifrar: listando %s: %w", c.Campo(), err)
		}
		if len(filas) == 0 {
			return nil
		}
		err = db.WithAuditContext(ctx, sqlDB, usuarioID, func(ctx context.Context, tx *dto.Queries) error {
			for _, f := range filas {
				valor, err := llavero.Descifrar(c.Campo(), f.Cifrado, f.Kid)
				if err != nil {
					return fmt.Errorf("descifrando id %d: %w", f.ID, err)
				}
				if err := guardar(ctx, tx, llavero, c, f.ID, valor); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("recifrar: %s desde id %d: %w", c.Campo(), desde, err)
		}
		res.sumar(c, len(filas))
		desde = filas[len(filas)-1].ID
	}
}
