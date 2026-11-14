package dto

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

// EscalaCesantiaRow es una fila de parametros_cesantia_escala.
type EscalaCesantiaRow struct {
	AniosServicioDesde decimal.Decimal
	AniosServicioHasta decimal.NullDecimal // NULL = sin tope superior
	DiasPorAnio        decimal.Decimal
	TopeAnios          decimal.NullDecimal // NULL = la escala no define tope
}

// ListarEscalaCesantiaEnFecha devuelve los escalones de cesantía del régimen
// laboral indicado vigentes a `fecha`, ordenados por anios_servicio_desde.
func (q *Queries) ListarEscalaCesantiaEnFecha(ctx context.Context, regimenLaboralID int64, fecha time.Time) ([]EscalaCesantiaRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT anios_servicio_desde, anios_servicio_hasta, dias_por_anio, tope_anios
FROM parametros_cesantia_escala
WHERE regimen_laboral_id = ?
  AND fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY anios_servicio_desde`, regimenLaboralID, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EscalaCesantiaRow
	for rows.Next() {
		var r EscalaCesantiaRow
		if err := rows.Scan(&r.AniosServicioDesde, &r.AniosServicioHasta, &r.DiasPorAnio, &r.TopeAnios); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// EscalaPreavisoRow es una fila de parametros_preaviso_escala.
type EscalaPreavisoRow struct {
	AntiguedadMesesDesde int32
	AntiguedadMesesHasta sql.NullInt32 // NULL = último tramo, sin tope
	DiasPreaviso         int32
}

// ListarEscalaPreavisoEnFecha devuelve los escalones de preaviso del régimen
// laboral indicado vigentes a `fecha`, ordenados por antiguedad_meses_desde.
func (q *Queries) ListarEscalaPreavisoEnFecha(ctx context.Context, regimenLaboralID int64, fecha time.Time) ([]EscalaPreavisoRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT antiguedad_meses_desde, antiguedad_meses_hasta, dias_preaviso
FROM parametros_preaviso_escala
WHERE regimen_laboral_id = ?
  AND fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY antiguedad_meses_desde`, regimenLaboralID, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EscalaPreavisoRow
	for rows.Next() {
		var r EscalaPreavisoRow
		if err := rows.Scan(&r.AntiguedadMesesDesde, &r.AntiguedadMesesHasta, &r.DiasPreaviso); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
