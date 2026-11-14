package dto

import (
	"context"
	"database/sql"
	"strings"
)

type FeriadoCatalogo struct {
	ID                   int64
	Nombre               string
	Tipo                 string
	Mes                  *int
	Dia                  *int
	DesplazamientoPascua *int
	PagoObligatorio      bool
	FuenteLegal          string
	AnioDesde            int
	AnioHasta            *int
	Activo               bool
}

const columnasFeriadoCatalogo = `id, nombre, tipo, mes, dia, desplazamiento_pascua, pago_obligatorio, fuente_legal, anio_desde, anio_hasta, activo`

func scanFeriadoCatalogo(s rowScanner) (FeriadoCatalogo, error) {
	var f FeriadoCatalogo
	var mes, dia, desp, hasta sql.NullInt64
	if err := s.Scan(&f.ID, &f.Nombre, &f.Tipo, &mes, &dia, &desp, &f.PagoObligatorio, &f.FuenteLegal, &f.AnioDesde, &hasta, &f.Activo); err != nil {
		return FeriadoCatalogo{}, err
	}
	f.Mes, f.Dia, f.DesplazamientoPascua, f.AnioHasta = intNulo(mes), intNulo(dia), intNulo(desp), intNulo(hasta)
	return f, nil
}

func intNulo(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func intPtrArg(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func (q *Queries) listarFeriadosCatalogo(ctx context.Context, consulta string, args ...interface{}) ([]FeriadoCatalogo, error) {
	rows, err := q.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FeriadoCatalogo
	for rows.Next() {
		f, err := scanFeriadoCatalogo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (q *Queries) ListarFeriadosCatalogo(ctx context.Context) ([]FeriadoCatalogo, error) {
	return q.listarFeriadosCatalogo(ctx, `SELECT `+columnasFeriadoCatalogo+` FROM feriados_catalogo ORDER BY anio_desde, nombre`)
}

func (q *Queries) ListarFeriadosCatalogoAnio(ctx context.Context, anio int) ([]FeriadoCatalogo, error) {
	return q.listarFeriadosCatalogo(ctx, `SELECT `+columnasFeriadoCatalogo+` FROM feriados_catalogo
WHERE activo = 1 AND anio_desde <= ? AND (anio_hasta IS NULL OR anio_hasta >= ?)
ORDER BY id`, anio, anio)
}

func (q *Queries) ObtenerFeriadoCatalogo(ctx context.Context, id int64) (FeriadoCatalogo, error) {
	return scanFeriadoCatalogo(q.db.QueryRowContext(ctx, `SELECT `+columnasFeriadoCatalogo+` FROM feriados_catalogo WHERE id = ?`, id))
}

func (q *Queries) CrearFeriadoCatalogo(ctx context.Context, f FeriadoCatalogo, creadoPor int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO feriados_catalogo (nombre, tipo, mes, dia, desplazamiento_pascua, pago_obligatorio, fuente_legal, anio_desde, anio_hasta, activo, creado_por)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.Nombre, f.Tipo, intPtrArg(f.Mes), intPtrArg(f.Dia), intPtrArg(f.DesplazamientoPascua), f.PagoObligatorio,
		f.FuenteLegal, f.AnioDesde, intPtrArg(f.AnioHasta), f.Activo, creadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) ActualizarFeriadoCatalogo(ctx context.Context, f FeriadoCatalogo) (bool, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE feriados_catalogo
SET nombre = ?, tipo = ?, mes = ?, dia = ?, desplazamiento_pascua = ?, pago_obligatorio = ?,
    fuente_legal = ?, anio_desde = ?, anio_hasta = ?, activo = ?
WHERE id = ?`,
		f.Nombre, f.Tipo, intPtrArg(f.Mes), intPtrArg(f.Dia), intPtrArg(f.DesplazamientoPascua), f.PagoObligatorio,
		f.FuenteLegal, f.AnioDesde, intPtrArg(f.AnioHasta), f.Activo, f.ID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (q *Queries) ExisteFeriadoCatalogo(ctx context.Context, id int64) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM feriados_catalogo WHERE id = ?`, id).Scan(&n)
	return n > 0, err
}

// EliminarFeriadosAnioExcepto borra los feriados del año que no están en
// conservar; sus días ya fueron desligados al regenerar el calendario.
func (q *Queries) EliminarFeriadosAnioExcepto(ctx context.Context, anio int32, conservar []int64) error {
	consulta := `DELETE FROM feriados WHERE anio = ?`
	args := []interface{}{anio}
	if len(conservar) > 0 {
		consulta += ` AND id NOT IN (` + strings.TrimSuffix(strings.Repeat("?,", len(conservar)), ",") + `)`
		for _, id := range conservar {
			args = append(args, id)
		}
	}
	_, err := q.db.ExecContext(ctx, consulta, args...)
	return err
}

func (q *Queries) ExisteEntidadEmisoraActiva(ctx context.Context, codigo string) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entidades_emisoras_incapacidad WHERE codigo = ? AND activo = 1`, codigo).Scan(&n)
	return n > 0, err
}
