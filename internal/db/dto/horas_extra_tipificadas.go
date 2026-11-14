package dto

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type FranjaNocturnaRow struct {
	HoraInicio time.Time
	HoraFin    time.Time
}

// ObtenerFranjaNocturnaEnFecha: sql.ErrNoRows si no hay franja vigente.
func (q *Queries) ObtenerFranjaNocturnaEnFecha(ctx context.Context, fecha time.Time) (FranjaNocturnaRow, error) {
	var ini, fin nullTimeOfDay
	err := q.db.QueryRowContext(ctx, `
SELECT hora_inicio, hora_fin FROM parametros_franja_nocturna
WHERE fecha_inicio_vigencia <= ? AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY fecha_inicio_vigencia DESC LIMIT 1`, fecha, fecha).Scan(&ini, &fin)
	if err != nil {
		return FranjaNocturnaRow{}, err
	}
	return FranjaNocturnaRow{HoraInicio: timeOrZero(ini.V), HoraFin: timeOrZero(fin.V)}, nil
}

// ObtenerClaseJornadaContratoEnFecha: clase legal de la jornada del contrato
// vigente a `fecha`. sql.ErrNoRows si no hay contrato o su jornada no calzó.
func (q *Queries) ObtenerClaseJornadaContratoEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (string, error) {
	var clase string
	err := q.db.QueryRowContext(ctx, `
SELECT j.clase FROM contratos c JOIN jornadas j ON j.id = c.jornada_id
WHERE c.empleado_id = ? AND c.fecha_inicio_vigencia <= ? AND (c.fecha_fin_vigencia IS NULL OR c.fecha_fin_vigencia >= ?)
ORDER BY c.fecha_inicio_vigencia DESC LIMIT 1`, empleadoID, fecha, fecha).Scan(&clase)
	return clase, err
}

type HoraExtraTipificadaInsert struct {
	Clase string // tipos_hora_extra.clase
	Horas decimal.Decimal
}

// ReemplazarHorasExtraTipificadas deja las horas tipificadas del día iguales a
// `filas` (re-registrar la asistencia recalcula). Una clase sin tipo activo
// en el catálogo es error.
func (q *Queries) ReemplazarHorasExtraTipificadas(ctx context.Context, empleadoID int64, fecha time.Time, filas []HoraExtraTipificadaInsert) error {
	if _, err := q.db.ExecContext(ctx,
		`DELETE FROM asistencia_hora_extra_tipificada WHERE empleado_id = ? AND fecha = ?`, empleadoID, fecha); err != nil {
		return err
	}
	for _, f := range filas {
		res, err := q.db.ExecContext(ctx, `
INSERT INTO asistencia_hora_extra_tipificada (empleado_id, fecha, tipo_hora_extra_id, horas)
SELECT ?, ?, t.id, ? FROM tipos_hora_extra t WHERE t.clase = ? AND t.activo = 1`, empleadoID, fecha, f.Horas, f.Clase)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("no hay tipo de hora extra activo con clase %q", f.Clase)
		}
	}
	return nil
}
