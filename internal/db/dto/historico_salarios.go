package dto

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// HistoricoSalarioRow: un mes de salario anterior a la puesta en marcha.
type HistoricoSalarioRow struct {
	ID                  int64
	EmpleadoID          int64
	Anio, Mes           int32
	MontoBruto          decimal.Decimal
	BaseAguinaldo       decimal.Decimal
	BaseCesantia        decimal.Decimal
	BaseVacaciones      decimal.Decimal
	Fuente              string
	CargadoPorUsuarioID int64
	FechaCarga          time.Time
}

func (q *Queries) ListarHistoricoSalarios(ctx context.Context, empleadoID int64) ([]HistoricoSalarioRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, empleado_id, anio, mes, monto_bruto, base_aguinaldo, base_cesantia, base_vacaciones, fuente, cargado_por_usuario_id, fecha_carga
FROM historico_salarios_pre_migracion
WHERE empleado_id = ?
ORDER BY anio, mes`, empleadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HistoricoSalarioRow
	for rows.Next() {
		var r HistoricoSalarioRow
		if err := rows.Scan(&r.ID, &r.EmpleadoID, &r.Anio, &r.Mes, &r.MontoBruto, &r.BaseAguinaldo, &r.BaseCesantia,
			&r.BaseVacaciones, &r.Fuente, &r.CargadoPorUsuarioID, &r.FechaCarga); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type InsertarHistoricoSalarioParams struct {
	EmpleadoID     int64
	Anio, Mes      int32
	MontoBruto     decimal.Decimal
	BaseAguinaldo  decimal.Decimal
	BaseCesantia   decimal.Decimal
	BaseVacaciones decimal.Decimal
	Fuente         string
	CargadoPor     int64
}

func (q *Queries) InsertarHistoricoSalario(ctx context.Context, p InsertarHistoricoSalarioParams) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
INSERT INTO historico_salarios_pre_migracion
  (empleado_id, anio, mes, monto_bruto, base_aguinaldo, base_cesantia, base_vacaciones, fuente, cargado_por_usuario_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, p.EmpleadoID, p.Anio, p.Mes, p.MontoBruto, p.BaseAguinaldo, p.BaseCesantia,
		p.BaseVacaciones, p.Fuente, p.CargadoPor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// EliminarHistoricoSalario borra un mes del histórico del empleado.
func (q *Queries) EliminarHistoricoSalario(ctx context.Context, empleadoID, id int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `DELETE FROM historico_salarios_pre_migracion WHERE id = ? AND empleado_id = ?`, id, empleadoID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ContarPeriodosRegularesEnMes: períodos regulares aprobados, pagados o
// contabilizados del empleado con fecha_fin en el mes (el sistema ya tiene
// ese mes y el histórico lo duplicaría).
func (q *Queries) ContarPeriodosRegularesEnMes(ctx context.Context, empleadoID int64, anio, mes int32) (int64, error) {
	var n int64
	err := q.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
WHERE pde.empleado_id = ? AND tp.clase = 'regular'
  AND pp.estado IN ('aprobada', 'pagada', 'contabilizada')
  AND YEAR(pp.fecha_fin) = ? AND MONTH(pp.fecha_fin) = ?`, empleadoID, anio, mes).Scan(&n)
	return n, err
}
