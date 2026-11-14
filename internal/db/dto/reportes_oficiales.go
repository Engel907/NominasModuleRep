package dto

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// FormatoReporteRow: formato de un reporte oficial.
type FormatoReporteRow struct {
	ID           int64
	Codigo       string
	ColumnaTotal *string
}

// ListarFormatosReporte: formatos del reporte (filtra por código si se da).
func (q *Queries) ListarFormatosReporte(ctx context.Context, reporte, codigo string) ([]FormatoReporteRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, codigo, columna_total FROM formatos_exportacion
WHERE reporte_oficial = ? AND (? = '' OR codigo = ?)
ORDER BY id`, reporte, codigo, codigo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FormatoReporteRow
	for rows.Next() {
		var r FormatoReporteRow
		var total sql.NullString
		if err := rows.Scan(&r.ID, &r.Codigo, &total); err != nil {
			return nil, err
		}
		if total.Valid {
			v := total.String
			r.ColumnaTotal = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ColumnaFormatoRow: columna del layout.
type ColumnaFormatoRow struct {
	Orden         int32
	ColumnaOrigen string
	Encabezado    string
}

func (q *Queries) ListarColumnasFormato(ctx context.Context, formatoID int64) ([]ColumnaFormatoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT orden, columna_origen, encabezado FROM formatos_exportacion_columnas
WHERE formato_exportacion_id = ? ORDER BY orden`, formatoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ColumnaFormatoRow
	for rows.Next() {
		var r ColumnaFormatoRow
		if err := rows.Scan(&r.Orden, &r.ColumnaOrigen, &r.Encabezado); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListarFilasReporte: columnas pedidas de la vista para el período (texto; nil = NULL).
func (q *Queries) ListarFilasReporte(ctx context.Context, vista string, cols []string, periodoID int64) ([][]*string, error) {
	if err := identificadores(append([]string{vista}, cols...)...); err != nil {
		return nil, err
	}
	sel := make([]string, len(cols))
	for i, c := range cols {
		sel[i] = "CAST(" + c + " AS CHAR)"
	}
	rows, err := q.db.QueryContext(ctx, `SELECT `+strings.Join(sel, ", ")+` FROM `+vista+
		` WHERE periodo_planilla_id = ? ORDER BY empleado_id`, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]*string
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		fila := make([]*string, len(cols))
		for i, v := range vals {
			if v.Valid {
				s := v.String
				fila[i] = &s
			}
		}
		out = append(out, fila)
	}
	return out, rows.Err()
}

// ListarFilasReporteMensual: como ListarFilasReporte, pero consolida por
// empleado todos los períodos aprobados, pagados o contabilizados cuyo
// fecha_fin cae en [desde, hasta]. Las columnas numéricas se suman; las demás
// agrupan.
func (q *Queries) ListarFilasReporteMensual(ctx context.Context, vista string, cols []string, numericas map[string]bool, desde, hasta time.Time) ([][]*string, error) {
	if err := identificadores(append([]string{vista}, cols...)...); err != nil {
		return nil, err
	}
	sel := make([]string, len(cols))
	grupo := []string{"empleado_id"}
	for i, c := range cols {
		if numericas[c] {
			sel[i] = "CAST(SUM(" + c + ") AS CHAR)"
			continue
		}
		sel[i] = "CAST(" + c + " AS CHAR)"
		grupo = append(grupo, c)
	}
	rows, err := q.db.QueryContext(ctx, `SELECT `+strings.Join(sel, ", ")+` FROM `+vista+`
WHERE periodo_planilla_id IN (
  SELECT id FROM periodos_planilla
  WHERE estado IN ('aprobada', 'pagada', 'contabilizada') AND fecha_fin >= ? AND fecha_fin <= ?)
GROUP BY `+strings.Join(grupo, ", ")+`
ORDER BY empleado_id`, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return escanearFilasTexto(rows, len(cols))
}

func escanearFilasTexto(rows *sql.Rows, n int) ([][]*string, error) {
	var out [][]*string
	for rows.Next() {
		vals := make([]sql.NullString, n)
		ptrs := make([]any, n)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		fila := make([]*string, n)
		for i, v := range vals {
			if v.Valid {
				s := v.String
				fila[i] = &s
			}
		}
		out = append(out, fila)
	}
	return out, rows.Err()
}
