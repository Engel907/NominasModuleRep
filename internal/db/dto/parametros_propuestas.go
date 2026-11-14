package dto

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var reIdentificador = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func identificadores(ns ...string) error {
	for _, n := range ns {
		if !reIdentificador.MatchString(n) {
			return fmt.Errorf("dto: identificador inválido %q", n)
		}
	}
	return nil
}

// PropuestaParametroRow: fila de parametros_propuestas.
type PropuestaParametroRow struct {
	ID                  int64
	Tabla               string
	ClaveNatural        string
	Payload             []byte
	FechaInicioVigencia time.Time
	FuenteLegal         string
	Estado              string
	PropuestoPor        int64
	FechaPropuesta      time.Time
	ResueltoPor         sql.NullInt64
	FechaResolucion     sql.NullTime
	MotivoRechazo       *string
}

const columnasPropuesta = `id, tabla, clave_natural, payload, fecha_inicio_vigencia, fuente_legal, estado,
       propuesto_por, fecha_propuesta, resuelto_por, fecha_resolucion, motivo_rechazo`

func scanPropuesta(s rowScanner) (PropuestaParametroRow, error) {
	var r PropuestaParametroRow
	var motivo sql.NullString
	err := s.Scan(&r.ID, &r.Tabla, &r.ClaveNatural, &r.Payload, &r.FechaInicioVigencia, &r.FuenteLegal, &r.Estado,
		&r.PropuestoPor, &r.FechaPropuesta, &r.ResueltoPor, &r.FechaResolucion, &motivo)
	if motivo.Valid {
		v := motivo.String
		r.MotivoRechazo = &v
	}
	return r, err
}

// InsertarPropuestaParametro registra una propuesta 'pendiente'.
func (q *Queries) InsertarPropuestaParametro(ctx context.Context, tabla, clave string, payload []byte, desde time.Time, fuente string, propuestoPor int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO parametros_propuestas (tabla, clave_natural, payload, fecha_inicio_vigencia, fuente_legal, propuesto_por)
VALUES (?, ?, ?, ?, ?, ?)`, tabla, clave, string(payload), desde, fuente, propuestoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ObtenerPropuestaParametro (paraActualizar = FOR UPDATE).
func (q *Queries) ObtenerPropuestaParametro(ctx context.Context, id int64, paraActualizar bool) (PropuestaParametroRow, error) {
	sufijo := ""
	if paraActualizar {
		sufijo = " FOR UPDATE"
	}
	return scanPropuesta(q.db.QueryRowContext(ctx, `SELECT `+columnasPropuesta+` FROM parametros_propuestas WHERE id = ?`+sufijo, id))
}

func filtroPropuestas(estado, tabla string) (string, []any) {
	w := "1 = 1"
	var args []any
	if estado != "" {
		w += " AND estado = ?"
		args = append(args, estado)
	}
	if tabla != "" {
		w += " AND tabla = ?"
		args = append(args, tabla)
	}
	return w, args
}

// ContarPropuestasParametro / ListarPropuestasParametro: filtros opcionales.
func (q *Queries) ContarPropuestasParametro(ctx context.Context, estado, tabla string) (int64, error) {
	w, args := filtroPropuestas(estado, tabla)
	var n int64
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM parametros_propuestas WHERE `+w, args...).Scan(&n)
	return n, err
}

func (q *Queries) ListarPropuestasParametro(ctx context.Context, estado, tabla string, limit, offset int32) ([]PropuestaParametroRow, error) {
	w, args := filtroPropuestas(estado, tabla)
	rows, err := q.db.QueryContext(ctx, `SELECT `+columnasPropuesta+` FROM parametros_propuestas WHERE `+w+
		` ORDER BY fecha_propuesta DESC, id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PropuestaParametroRow
	for rows.Next() {
		r, err := scanPropuesta(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ResolverPropuestaParametro: pendiente → aplicada | rechazada.
func (q *Queries) ResolverPropuestaParametro(ctx context.Context, id int64, estado string, resueltoPor int64, motivo *string) error {
	res, err := q.db.ExecContext(ctx, `
UPDATE parametros_propuestas
SET estado = ?, resuelto_por = ?, fecha_resolucion = NOW(), motivo_rechazo = ?
WHERE id = ? AND estado = 'pendiente'`, estado, resueltoPor, motivo, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n != 1 {
		err = fmt.Errorf("dto: la propuesta %d ya no está pendiente", id)
	}
	return err
}

// ContarFilasParametro: COUNT(*) de `tabla` con el filtro (FOR UPDATE: bloquea la serie).
func (q *Queries) ContarFilasParametro(ctx context.Context, tabla, where string, args ...any) (int64, error) {
	if err := identificadores(tabla); err != nil {
		return 0, err
	}
	var n int64
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tabla+` WHERE `+where+` FOR UPDATE`, args...).Scan(&n)
	return n, err
}

// CerrarVigenciaParametro: paso 1 de 2 (fecha_fin_vigencia de las vigentes de la serie).
func (q *Queries) CerrarVigenciaParametro(ctx context.Context, tabla, where string, fin time.Time, args ...any) error {
	if err := identificadores(tabla); err != nil {
		return err
	}
	_, err := q.db.ExecContext(ctx, `UPDATE `+tabla+` SET fecha_fin_vigencia = ? WHERE `+where+` AND fecha_fin_vigencia IS NULL`,
		append([]any{fin}, args...)...)
	return err
}

// InsertarFilaParametro: paso 2 de 2.
func (q *Queries) InsertarFilaParametro(ctx context.Context, tabla string, cols []string, vals []any) error {
	if err := identificadores(append([]string{tabla}, cols...)...); err != nil {
		return err
	}
	if len(cols) != len(vals) || len(cols) == 0 {
		return fmt.Errorf("dto: %d columnas y %d valores", len(cols), len(vals))
	}
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	_, err := q.db.ExecContext(ctx, `INSERT INTO `+tabla+` (`+strings.Join(cols, ", ")+`) VALUES (`+ph+`)`, vals...)
	return err
}

// ListarFilasParametroVigentes: filas vigentes a `fecha`, cada columna como texto (nil = NULL).
func (q *Queries) ListarFilasParametroVigentes(ctx context.Context, tabla string, cols []string, fecha time.Time) ([]map[string]*string, error) {
	if err := identificadores(append([]string{tabla}, cols...)...); err != nil {
		return nil, err
	}
	sel := make([]string, len(cols))
	for i, c := range cols {
		sel[i] = "CAST(" + c + " AS CHAR)"
	}
	rows, err := q.db.QueryContext(ctx, `SELECT `+strings.Join(sel, ", ")+` FROM `+tabla+`
WHERE fecha_inicio_vigencia <= ? AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY id`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]*string
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]*string, len(cols))
		for i, c := range cols {
			if vals[i].Valid {
				v := vals[i].String
				m[c] = &v
			} else {
				m[c] = nil
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
