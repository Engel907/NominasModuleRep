package dto

import (
	"context"
	"strings"
	"time"
)

// PeriodoCicloRow es el período con quién y cuándo de cada paso del ciclo.
type PeriodoCicloRow struct {
	ID                        int64
	TipoPlanillaID            int64
	TipoPlanillaCodigo        string
	FechaInicio               time.Time
	FechaFin                  time.Time
	Estado                    string
	VersionCalculo            int32
	FechaGeneracion           *time.Time
	PreparadoPorUsuarioID     *int64
	FechaPreparacion          *time.Time
	CalculadoPorUsuarioID     *int64
	FechaCalculo              *time.Time
	AprobadoPorUsuarioID      *int64
	FechaAprobacion           *time.Time
	PagadoPorUsuarioID        *int64
	FechaPago                 *time.Time
	ContabilizadoPorUsuarioID *int64
	FechaContabilizacion      *time.Time
}

// FiltroPeriodosCiclo: todos los campos son opcionales. Desde/Hasta filtran
// por solape con [Desde, Hasta].
type FiltroPeriodosCiclo struct {
	TipoPlanillaID *int64
	Estado         string
	Desde          *time.Time
	Hasta          *time.Time
}

// La columna del tipo va como subconsulta escalar para que la misma consulta
// sirva con FOR UPDATE sin bloquear filas de tipos_planilla.
const columnasPeriodoCiclo = `
pp.id, pp.tipo_planilla_id,
COALESCE((SELECT tp.codigo FROM tipos_planilla tp WHERE tp.id = pp.tipo_planilla_id), ''),
pp.fecha_inicio, pp.fecha_fin, pp.estado, pp.version_calculo,
pp.fecha_generacion, pp.preparado_por_usuario_id, pp.fecha_preparacion,
pp.calculado_por_usuario_id, pp.fecha_calculo,
pp.aprobado_por_usuario_id, pp.fecha_aprobacion,
pp.pagado_por_usuario_id, pp.fecha_pago,
pp.contabilizado_por_usuario_id, pp.fecha_contabilizacion`

type scannerCiclo interface{ Scan(dest ...any) error }

func scanPeriodoCiclo(s scannerCiclo) (PeriodoCicloRow, error) {
	var r PeriodoCicloRow
	err := s.Scan(&r.ID, &r.TipoPlanillaID, &r.TipoPlanillaCodigo, &r.FechaInicio, &r.FechaFin,
		&r.Estado, &r.VersionCalculo, &r.FechaGeneracion, &r.PreparadoPorUsuarioID, &r.FechaPreparacion,
		&r.CalculadoPorUsuarioID, &r.FechaCalculo, &r.AprobadoPorUsuarioID, &r.FechaAprobacion,
		&r.PagadoPorUsuarioID, &r.FechaPago, &r.ContabilizadoPorUsuarioID, &r.FechaContabilizacion)
	return r, err
}

// ObtenerPeriodoCiclo: lectura sin bloqueo. sql.ErrNoRows si no existe.
func (q *Queries) ObtenerPeriodoCiclo(ctx context.Context, id int64) (PeriodoCicloRow, error) {
	return scanPeriodoCiclo(q.db.QueryRowContext(ctx,
		`SELECT `+columnasPeriodoCiclo+` FROM periodos_planilla pp WHERE pp.id = ?`, id))
}

// ObtenerPeriodoCicloParaActualizar: SELECT ... FOR UPDATE. Solo dentro de
// una transacción. Un lock FOR UPDATE lee lo último confirmado, por eso la
// segunda de dos aprobaciones simultáneas ve el estado ya cambiado.
func (q *Queries) ObtenerPeriodoCicloParaActualizar(ctx context.Context, id int64) (PeriodoCicloRow, error) {
	return scanPeriodoCiclo(q.db.QueryRowContext(ctx,
		`SELECT `+columnasPeriodoCiclo+` FROM periodos_planilla pp WHERE pp.id = ? FOR UPDATE`, id))
}

// TieneDetallePeriodo indica si el período tiene al menos una línea de empleado.
func (q *Queries) TieneDetallePeriodo(ctx context.Context, periodoID int64) (bool, error) {
	var tiene bool
	err := q.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM planilla_detalle_empleado WHERE periodo_planilla_id = ?
)`, periodoID).Scan(&tiene)
	return tiene, err
}

// BloquearTipoPlanilla serializa la creación de períodos del mismo tipo (el
// chequeo de traslape + insert no es atómico sin esto). sql.ErrNoRows si el
// tipo no existe.
func (q *Queries) BloquearTipoPlanilla(ctx context.Context, tipoID int64) error {
	var id int64
	return q.db.QueryRowContext(ctx, `SELECT id FROM tipos_planilla WHERE id = ? FOR UPDATE`, tipoID).Scan(&id)
}

// ExisteTraslapePeriodo: otro período del mismo tipo cuyo rango se solapa.
func (q *Queries) ExisteTraslapePeriodo(ctx context.Context, tipoID int64, inicio, fin time.Time) (bool, error) {
	var n int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM periodos_planilla
WHERE tipo_planilla_id = ? AND fecha_inicio <= ? AND fecha_fin >= ?`, tipoID, fin, inicio).Scan(&n)
	return n > 0, err
}

func (q *Queries) InsertarPeriodoPlanilla(ctx context.Context, tipoID int64, inicio, fin time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO periodos_planilla (tipo_planilla_id, fecha_inicio, fecha_fin) VALUES (?, ?, ?)`, tipoID, inicio, fin)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// MarcarPeriodoCalculado: borrador -> calculada. preparado_por conserva a
// quien calculó primero (así un recálculo por otra persona no libera a la
// primera de la segregación); calculado_por es el último.
func (q *Queries) MarcarPeriodoCalculado(ctx context.Context, id, usuarioID int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE periodos_planilla
SET estado = 'calculada',
    fecha_generacion = NOW(),
    preparado_por_usuario_id = COALESCE(preparado_por_usuario_id, ?),
    fecha_preparacion = COALESCE(fecha_preparacion, NOW()),
    calculado_por_usuario_id = ?,
    fecha_calculo = NOW(),
    version_calculo = version_calculo + 1
WHERE id = ? AND estado = 'borrador'`, usuarioID, usuarioID, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (q *Queries) MarcarPeriodoBorrador(ctx context.Context, id int64) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`UPDATE periodos_planilla SET estado = 'borrador' WHERE id = ? AND estado = 'calculada'`, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (q *Queries) MarcarPeriodoAprobado(ctx context.Context, id, usuarioID int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE periodos_planilla
SET estado = 'aprobada', aprobado_por_usuario_id = ?, fecha_aprobacion = NOW()
WHERE id = ? AND estado = 'calculada'`, usuarioID, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (q *Queries) MarcarPeriodoPagado(ctx context.Context, id, usuarioID int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE periodos_planilla
SET estado = 'pagada', pagado_por_usuario_id = ?, fecha_pago = NOW()
WHERE id = ? AND estado = 'aprobada'`, usuarioID, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ActualizarEstadoDetallePeriodo mantiene planilla_detalle_empleado.estado en
// sincronía con el período (calculado -> aprobado -> pagado).
func (q *Queries) ActualizarEstadoDetallePeriodo(ctx context.Context, periodoID int64, desde, hasta string) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE planilla_detalle_empleado SET estado = ?
WHERE periodo_planilla_id = ? AND estado = ?`, hasta, periodoID, desde)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (f FiltroPeriodosCiclo) donde() (string, []any) {
	var cond []string
	var args []any
	if f.TipoPlanillaID != nil {
		cond = append(cond, "pp.tipo_planilla_id = ?")
		args = append(args, *f.TipoPlanillaID)
	}
	if f.Estado != "" {
		cond = append(cond, "pp.estado = ?")
		args = append(args, f.Estado)
	}
	if f.Desde != nil {
		cond = append(cond, "pp.fecha_fin >= ?")
		args = append(args, *f.Desde)
	}
	if f.Hasta != nil {
		cond = append(cond, "pp.fecha_inicio <= ?")
		args = append(args, *f.Hasta)
	}
	if len(cond) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(cond, " AND "), args
}

func (q *Queries) ListarPeriodosCiclo(ctx context.Context, f FiltroPeriodosCiclo, limit, offset int32) ([]PeriodoCicloRow, error) {
	donde, args := f.donde()
	args = append(args, limit, offset)
	rows, err := q.db.QueryContext(ctx,
		`SELECT `+columnasPeriodoCiclo+` FROM periodos_planilla pp`+donde+
			` ORDER BY pp.fecha_inicio DESC, pp.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PeriodoCicloRow
	for rows.Next() {
		r, err := scanPeriodoCiclo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ContarPeriodosCiclo(ctx context.Context, f FiltroPeriodosCiclo) (int64, error) {
	donde, args := f.donde()
	var n int64
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM periodos_planilla pp`+donde, args...).Scan(&n)
	return n, err
}
