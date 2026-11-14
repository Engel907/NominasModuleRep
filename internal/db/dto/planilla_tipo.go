package dto

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shopspring/decimal"
)

// ErrClaseTipoAmbigua: hay más de un tipo con esa clase. dto no puede importar
// planilla (planilla importa dto), así que la capa de servicio traduce este
// error y sql.ErrNoRows a planilla.ErrTipoPlanillaNoEncontrado.
var ErrClaseTipoAmbigua = errors.New("más de un tipo de planilla con esa clase")

// TipoPlanillaRow es un tipo de planilla con su clase y fracción mensual.
// FraccionMensual no es válida (NULL) cuando la clase no es regular.
type TipoPlanillaRow struct {
	ID              int64
	Codigo          string
	Nombre          string
	Clase           string
	FraccionMensual decimal.NullDecimal
}

const columnasTipoPlanilla = `tp.id, tp.codigo, tp.nombre, tp.clase, tp.fraccion_mensual`

type scannerTipo interface{ Scan(dest ...any) error }

func scanTipoPlanilla(s scannerTipo) (TipoPlanillaRow, error) {
	var r TipoPlanillaRow
	err := s.Scan(&r.ID, &r.Codigo, &r.Nombre, &r.Clase, &r.FraccionMensual)
	return r, err
}

// ObtenerTipoPlanilla: lectura sin bloqueo. sql.ErrNoRows si no existe.
func (q *Queries) ObtenerTipoPlanilla(ctx context.Context, id int64) (TipoPlanillaRow, error) {
	return scanTipoPlanilla(q.db.QueryRowContext(ctx,
		`SELECT `+columnasTipoPlanilla+` FROM tipos_planilla tp WHERE tp.id = ?`, id))
}

// ObtenerTipoPlanillaPorClase devuelve el único tipo de una clase no regular
// (aguinaldo, salario_escolar, liquidacion). sql.ErrNoRows si no hay ninguno;
// ErrClaseTipoAmbigua si hay más de uno. Para 'regular' siempre es ambiguo
// (mensual y quincenal): ahí se debe usar el id del período.
func (q *Queries) ObtenerTipoPlanillaPorClase(ctx context.Context, clase string) (TipoPlanillaRow, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT `+columnasTipoPlanilla+` FROM tipos_planilla tp WHERE tp.clase = ? ORDER BY tp.id LIMIT 2`, clase)
	if err != nil {
		return TipoPlanillaRow{}, err
	}
	defer rows.Close()
	var out []TipoPlanillaRow
	for rows.Next() {
		r, err := scanTipoPlanilla(rows)
		if err != nil {
			return TipoPlanillaRow{}, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return TipoPlanillaRow{}, err
	}
	switch len(out) {
	case 0:
		return TipoPlanillaRow{}, sql.ErrNoRows
	case 1:
		return out[0], nil
	}
	return TipoPlanillaRow{}, ErrClaseTipoAmbigua
}
