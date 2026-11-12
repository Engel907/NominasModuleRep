// internal/db/dto/horas_extra_autorizadas.go
//
// L13 — Autorización previa de horas extra (horas_extra_autorizadas).
package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type HoraExtraAutorizacionRow struct {
	ID               int64
	EmpleadoID       int64
	Fecha            time.Time
	HorasAutorizadas decimal.Decimal
	TipoHoraExtraID  *int64
	Motivo           string
	Estado           string // pendiente | autorizada | rechazada
	SolicitadoPor    int64
	FechaSolicitud   time.Time
	ResueltoPor      *int64
	FechaResolucion  *time.Time
	MotivoResolucion *string
}

const selectHoraExtraAutorizacion = `
SELECT id, empleado_id, fecha, horas_autorizadas, tipo_hora_extra_id, motivo, estado,
       solicitado_por, fecha_solicitud, resuelto_por, fecha_resolucion, motivo_resolucion
FROM horas_extra_autorizadas`

func scanHoraExtraAutorizacion(s interface{ Scan(...any) error }) (HoraExtraAutorizacionRow, error) {
	var r HoraExtraAutorizacionRow
	var motivoRes sql.NullString
	var fechaRes sql.NullTime
	err := s.Scan(&r.ID, &r.EmpleadoID, &r.Fecha, &r.HorasAutorizadas, &r.TipoHoraExtraID, &r.Motivo, &r.Estado,
		&r.SolicitadoPor, &r.FechaSolicitud, &r.ResueltoPor, &fechaRes, &motivoRes)
	if motivoRes.Valid {
		r.MotivoResolucion = &motivoRes.String
	}
	if fechaRes.Valid {
		r.FechaResolucion = &fechaRes.Time
	}
	return r, err
}

type InsertarHoraExtraAutorizacionParams struct {
	EmpleadoID       int64
	Fecha            time.Time
	HorasAutorizadas decimal.Decimal
	TipoHoraExtraID  *int64
	Motivo           string
	SolicitadoPor    int64
}

func (q *Queries) InsertarHoraExtraAutorizacion(ctx context.Context, p InsertarHoraExtraAutorizacionParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO horas_extra_autorizadas (empleado_id, fecha, horas_autorizadas, tipo_hora_extra_id, motivo, solicitado_por)
VALUES (?, ?, ?, ?, ?, ?)`, p.EmpleadoID, p.Fecha, p.HorasAutorizadas, p.TipoHoraExtraID, p.Motivo, p.SolicitadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) ObtenerHoraExtraAutorizacion(ctx context.Context, id int64) (HoraExtraAutorizacionRow, error) {
	return scanHoraExtraAutorizacion(q.db.QueryRowContext(ctx, selectHoraExtraAutorizacion+` WHERE id = ?`, id))
}

// ObtenerHoraExtraAutorizacionParaActualizar bloquea la fila (FOR UPDATE):
// dos resoluciones simultáneas se serializan.
func (q *Queries) ObtenerHoraExtraAutorizacionParaActualizar(ctx context.Context, id int64) (HoraExtraAutorizacionRow, error) {
	return scanHoraExtraAutorizacion(q.db.QueryRowContext(ctx, selectHoraExtraAutorizacion+` WHERE id = ? FOR UPDATE`, id))
}

// ResolverHoraExtraAutorizacion: pendiente -> autorizada|rechazada. Devuelve
// filas afectadas (0 = ya no estaba pendiente).
func (q *Queries) ResolverHoraExtraAutorizacion(ctx context.Context, id int64, estado string, resueltoPor int64, motivo *string) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
UPDATE horas_extra_autorizadas
SET estado = ?, resuelto_por = ?, fecha_resolucion = NOW(), motivo_resolucion = ?
WHERE id = ? AND estado = 'pendiente'`, estado, resueltoPor, motivo, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// FiltroHorasExtraAutorizaciones: todos opcionales (nil = sin filtro).
type FiltroHorasExtraAutorizaciones struct {
	EmpleadoID *int64
	Estado     *string
	Desde      *time.Time
	Hasta      *time.Time
}

func (q *Queries) ListarHorasExtraAutorizaciones(ctx context.Context, f FiltroHorasExtraAutorizaciones, limit, offset int32) ([]HoraExtraAutorizacionRow, error) {
	rows, err := q.db.QueryContext(ctx, selectHoraExtraAutorizacion+`
WHERE (? IS NULL OR empleado_id = ?) AND (? IS NULL OR estado = ?)
  AND (? IS NULL OR fecha >= ?) AND (? IS NULL OR fecha <= ?)
ORDER BY fecha DESC, id DESC
LIMIT ? OFFSET ?`, f.EmpleadoID, f.EmpleadoID, f.Estado, f.Estado, f.Desde, f.Desde, f.Hasta, f.Hasta, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HoraExtraAutorizacionRow
	for rows.Next() {
		r, err := scanHoraExtraAutorizacion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ContarHorasExtraAutorizaciones(ctx context.Context, f FiltroHorasExtraAutorizaciones) (int64, error) {
	var n int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM horas_extra_autorizadas
WHERE (? IS NULL OR empleado_id = ?) AND (? IS NULL OR estado = ?)
  AND (? IS NULL OR fecha >= ?) AND (? IS NULL OR fecha <= ?)`,
		f.EmpleadoID, f.EmpleadoID, f.Estado, f.Estado, f.Desde, f.Desde, f.Hasta, f.Hasta).Scan(&n)
	return n, err
}

type HoraExtraCandidataRow struct {
	EmpleadoID       int64
	Fecha            time.Time
	HorasRegistradas decimal.Decimal
}

// ListarHorasExtraCandidatas: días con horas extra registradas en
// registro_asistencia y SIN autorización (ni pendiente, ni resuelta). Son las
// que no llegarán a planilla si nadie las solicita.
func (q *Queries) ListarHorasExtraCandidatas(ctx context.Context, empleadoID *int64, desde, hasta time.Time) ([]HoraExtraCandidataRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT ra.empleado_id, ra.fecha,
       ra.horas_extra_diurnas + ra.horas_extra_nocturnas + ra.horas_extra_mixtas + ra.horas_dobles AS horas
FROM registro_asistencia ra
LEFT JOIN horas_extra_autorizadas a ON a.empleado_id = ra.empleado_id AND a.fecha = ra.fecha
WHERE ra.fecha >= ? AND ra.fecha <= ? AND (? IS NULL OR ra.empleado_id = ?)
  AND a.id IS NULL
  AND ra.horas_extra_diurnas + ra.horas_extra_nocturnas + ra.horas_extra_mixtas + ra.horas_dobles > 0
ORDER BY ra.fecha ASC, ra.empleado_id ASC`, desde, hasta, empleadoID, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HoraExtraCandidataRow
	for rows.Next() {
		var r HoraExtraCandidataRow
		if err := rows.Scan(&r.EmpleadoID, &r.Fecha, &r.HorasRegistradas); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ExisteTipoHoraExtraActivo indica si el tipo existe y está activo.
func (q *Queries) ExisteTipoHoraExtraActivo(ctx context.Context, id int64) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tipos_hora_extra WHERE id = ? AND activo = 1`, id).Scan(&n)
	return n > 0, err
}
