package dto

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// ErrAcumuladoYaProcesado: el empleado ya tiene la fila del año en
// 'procesado' (otro período aprobado la pagó).
var ErrAcumuladoYaProcesado = errors.New("dto: el acumulado del año ya está procesado")

// SumarBaseAcumuladaEmpleado suma planilla_detalle_empleado.<columna> de los
// períodos REGULARES del empleado en estado aprobada|pagada|contabilizada
// cuyo fecha_fin cae en [desde, hasta], más los meses de
// historico_salarios_pre_migracion cuyo último día cae en el rango. columna ∈ base_aguinaldo,
// base_vacaciones, base_cesantia (cualquier otro valor suma 0: lo valida el
// llamador). Los estados deben coincidir con planilla.EstadosParaAcumulados.
func (q *Queries) SumarBaseAcumuladaEmpleado(ctx context.Context, empleadoID int64, desde, hasta time.Time, columna string) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := q.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(t.base), 0) FROM (
  SELECT CASE ?
           WHEN 'base_aguinaldo'  THEN pde.base_aguinaldo
           WHEN 'base_vacaciones' THEN pde.base_vacaciones
           WHEN 'base_cesantia'   THEN pde.base_cesantia
           ELSE 0 END AS base
  FROM planilla_detalle_empleado pde
  JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
  JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
  WHERE pde.empleado_id = ?
    AND tp.clase = 'regular'
    AND pp.estado IN ('aprobada', 'pagada', 'contabilizada')
    AND pp.fecha_fin >= ?
    AND pp.fecha_fin <= ?
  UNION ALL
  SELECT CASE ?
           WHEN 'base_aguinaldo'  THEN h.base_aguinaldo
           WHEN 'base_vacaciones' THEN h.base_vacaciones
           WHEN 'base_cesantia'   THEN h.base_cesantia
           ELSE 0 END
  FROM historico_salarios_pre_migracion h
  WHERE h.empleado_id = ?
    AND LAST_DAY(MAKEDATE(h.anio, 1) + INTERVAL (h.mes - 1) MONTH) >= ?
    AND LAST_DAY(MAKEDATE(h.anio, 1) + INTERVAL (h.mes - 1) MONTH) <= ?
) t`, columna, empleadoID, desde, hasta, columna, empleadoID, desde, hasta).Scan(&total)
	return total, err
}

// AcumuladoProyectado: fila de aguinaldos / salarios_escolares que deja el cálculo.
type AcumuladoProyectado struct {
	EmpleadoID   int64
	Anio         int32
	PeriodoID    int64
	Desde, Hasta time.Time
	TotalBase    decimal.Decimal
	Monto        decimal.Decimal
	Porcentaje   decimal.Decimal // solo salario escolar (fracción)
}

// GuardarAguinaldoProyectado inserta o actualiza la fila (empleado, año) en
// 'proyectado' con el período. Si ya está 'procesado' devuelve
// ErrAcumuladoYaProcesado (nunca se pisa un aguinaldo pagado).
func (q *Queries) GuardarAguinaldoProyectado(ctx context.Context, a AcumuladoProyectado) error {
	var estado string
	err := q.db.QueryRowContext(ctx,
		`SELECT estado FROM aguinaldos WHERE empleado_id = ? AND anio = ? FOR UPDATE`, a.EmpleadoID, a.Anio).Scan(&estado)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = q.db.ExecContext(ctx, `
INSERT INTO aguinaldos (empleado_id, anio, periodo_desde, periodo_hasta, total_salarios_brutos, monto_aguinaldo, estado, periodo_planilla_id)
VALUES (?, ?, ?, ?, ?, ?, 'proyectado', ?)`, a.EmpleadoID, a.Anio, a.Desde, a.Hasta, a.TotalBase, a.Monto, a.PeriodoID)
		return err
	case err != nil:
		return err
	case estado != "proyectado":
		return ErrAcumuladoYaProcesado
	}
	_, err = q.db.ExecContext(ctx, `
UPDATE aguinaldos
SET periodo_desde = ?, periodo_hasta = ?, total_salarios_brutos = ?, monto_aguinaldo = ?,
    periodo_planilla_id = ?, calculado_en = NOW()
WHERE empleado_id = ? AND anio = ? AND estado = 'proyectado'`, a.Desde, a.Hasta, a.TotalBase, a.Monto, a.PeriodoID, a.EmpleadoID, a.Anio)
	return err
}

// GuardarSalarioEscolarProyectado: igual que GuardarAguinaldoProyectado.
func (q *Queries) GuardarSalarioEscolarProyectado(ctx context.Context, a AcumuladoProyectado) error {
	var estado string
	err := q.db.QueryRowContext(ctx,
		`SELECT estado FROM salarios_escolares WHERE empleado_id = ? AND anio = ? FOR UPDATE`, a.EmpleadoID, a.Anio).Scan(&estado)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = q.db.ExecContext(ctx, `
INSERT INTO salarios_escolares (empleado_id, anio, periodo_desde, periodo_hasta, porcentaje_aplicado, total_salarios_brutos, monto_salario_escolar, estado, periodo_planilla_id)
VALUES (?, ?, ?, ?, ?, ?, ?, 'proyectado', ?)`, a.EmpleadoID, a.Anio, a.Desde, a.Hasta, a.Porcentaje, a.TotalBase, a.Monto, a.PeriodoID)
		return err
	case err != nil:
		return err
	case estado != "proyectado":
		return ErrAcumuladoYaProcesado
	}
	_, err = q.db.ExecContext(ctx, `
UPDATE salarios_escolares
SET periodo_desde = ?, periodo_hasta = ?, porcentaje_aplicado = ?, total_salarios_brutos = ?, monto_salario_escolar = ?,
    periodo_planilla_id = ?, calculado_en = NOW()
WHERE empleado_id = ? AND anio = ? AND estado = 'proyectado'`, a.Desde, a.Hasta, a.Porcentaje, a.TotalBase, a.Monto, a.PeriodoID, a.EmpleadoID, a.Anio)
	return err
}

// EliminarAcumuladosProyectadosPeriodo: al descartar/recalcular, las filas
// 'proyectado' del período desaparecen junto con su detalle.
func (q *Queries) EliminarAcumuladosProyectadosPeriodo(ctx context.Context, periodoID int64) error {
	if _, err := q.db.ExecContext(ctx,
		`DELETE FROM aguinaldos WHERE periodo_planilla_id = ? AND estado = 'proyectado'`, periodoID); err != nil {
		return err
	}
	_, err := q.db.ExecContext(ctx,
		`DELETE FROM salarios_escolares WHERE periodo_planilla_id = ? AND estado = 'proyectado'`, periodoID)
	return err
}

// ProcesarAcumuladosPeriodo: al aprobar, las filas 'proyectado' del período
// pasan a 'procesado' (idempotente: solo toca las que siguen proyectadas).
func (q *Queries) ProcesarAcumuladosPeriodo(ctx context.Context, periodoID int64, aprobadoPor *int64) error {
	if _, err := q.db.ExecContext(ctx, `
UPDATE aguinaldos SET estado = 'procesado', procesado_por = ?, procesado_en = NOW()
WHERE periodo_planilla_id = ? AND estado = 'proyectado'`, aprobadoPor, periodoID); err != nil {
		return err
	}
	_, err := q.db.ExecContext(ctx, `
UPDATE salarios_escolares SET estado = 'procesado', procesado_por = ?, procesado_en = NOW()
WHERE periodo_planilla_id = ? AND estado = 'proyectado'`, aprobadoPor, periodoID)
	return err
}

// RentaMesRow: Σ de renta de un empleado en los períodos aprobados de un mes.
type RentaMesRow struct {
	BaseRenta, RentaBruta, Creditos, RentaNeta decimal.Decimal
}

// SumarRentaMesEmpleado: base de renta, impuesto bruto, créditos aplicados e
// impuesto neto del empleado en los períodos aprobados, pagados o
// contabilizados (de cualquier clase) con fecha_fin en [desde, hasta], salvo
// el período excluirPeriodoID.
func (q *Queries) SumarRentaMesEmpleado(ctx context.Context, empleadoID, excluirPeriodoID int64, desde, hasta time.Time) (RentaMesRow, error) {
	var r RentaMesRow
	err := q.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(pde.base_renta), 0), COALESCE(SUM(pde.renta_bruta), 0),
       COALESCE(SUM(pde.creditos_fiscales_aplicados), 0), COALESCE(SUM(pde.renta_neta), 0)
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
WHERE pde.empleado_id = ?
  AND pp.id <> ?
  AND pp.estado IN ('aprobada', 'pagada', 'contabilizada')
  AND pp.fecha_fin >= ?
  AND pp.fecha_fin <= ?`, empleadoID, excluirPeriodoID, desde, hasta).Scan(&r.BaseRenta, &r.RentaBruta, &r.Creditos, &r.RentaNeta)
	return r, err
}
