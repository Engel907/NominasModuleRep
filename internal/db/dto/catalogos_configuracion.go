package dto

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func seleccionTexto(cols []string) string {
	sel := make([]string, len(cols))
	for i, c := range cols {
		sel[i] = "CAST(" + c + " AS CHAR)"
	}
	return strings.Join(sel, ", ")
}

func escanearFilaTexto(s rowScanner, cols []string) (map[string]*string, error) {
	vals := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := s.Scan(ptrs...); err != nil {
		return nil, err
	}
	fila := make(map[string]*string, len(cols))
	for i, c := range cols {
		if vals[i].Valid {
			v := vals[i].String
			fila[c] = &v
		} else {
			fila[c] = nil
		}
	}
	return fila, nil
}

func (q *Queries) ContarFilasCatalogo(ctx context.Context, tabla string) (int64, error) {
	if err := identificadores(tabla); err != nil {
		return 0, err
	}
	var n int64
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tabla).Scan(&n)
	return n, err
}

func (q *Queries) ListarFilasCatalogo(ctx context.Context, tabla string, cols []string, limit, offset int32) ([]map[string]*string, error) {
	if err := identificadores(append([]string{tabla}, cols...)...); err != nil {
		return nil, err
	}
	rows, err := q.db.QueryContext(ctx, `SELECT `+seleccionTexto(cols)+` FROM `+tabla+` ORDER BY id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]*string
	for rows.Next() {
		f, err := escanearFilaTexto(rows, cols)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (q *Queries) ObtenerFilaCatalogo(ctx context.Context, tabla string, cols []string, id int64, bloquear bool) (map[string]*string, error) {
	if err := identificadores(append([]string{tabla}, cols...)...); err != nil {
		return nil, err
	}
	consulta := `SELECT ` + seleccionTexto(cols) + ` FROM ` + tabla + ` WHERE id = ?`
	if bloquear {
		consulta += ` FOR UPDATE`
	}
	return escanearFilaTexto(q.db.QueryRowContext(ctx, consulta, id), cols)
}

func (q *Queries) InsertarFilaCatalogo(ctx context.Context, tabla string, cols []string, vals []any) (int64, error) {
	if err := identificadores(append([]string{tabla}, cols...)...); err != nil {
		return 0, err
	}
	if len(cols) == 0 || len(cols) != len(vals) {
		return 0, fmt.Errorf("dto: %d columnas y %d valores", len(cols), len(vals))
	}
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	res, err := q.db.ExecContext(ctx, `INSERT INTO `+tabla+` (`+strings.Join(cols, ", ")+`) VALUES (`+ph+`)`, vals...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) ActualizarFilaCatalogo(ctx context.Context, tabla string, cols []string, vals []any, id int64) error {
	if err := identificadores(append([]string{tabla}, cols...)...); err != nil {
		return err
	}
	if len(cols) == 0 || len(cols) != len(vals) {
		return fmt.Errorf("dto: %d columnas y %d valores", len(cols), len(vals))
	}
	sets := make([]string, len(cols))
	for i, c := range cols {
		sets[i] = c + " = ?"
	}
	_, err := q.db.ExecContext(ctx, `UPDATE `+tabla+` SET `+strings.Join(sets, ", ")+` WHERE id = ?`, append(vals, id)...)
	return err
}

func (q *Queries) BorrarFilaCatalogo(ctx context.Context, tabla string, id int64) (bool, error) {
	if err := identificadores(tabla); err != nil {
		return false, err
	}
	res, err := q.db.ExecContext(ctx, `DELETE FROM `+tabla+` WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
