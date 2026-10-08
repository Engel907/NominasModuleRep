// internal/db/dto/planilla_motor.go
//
// Fase 6d — Lecturas que necesita el orquestador del motor de cálculo
// (evaluarConcepto / CalcularEmpleado, ver internal/planilla/motor_calculo.go).
// Mismo estilo que contratos.go / planilla_export.go: argumentos
// posicionales, structs *Row, decimal.Decimal para todo monto.
//
// Columnas verificadas contra db/schema.sql: contratos, conceptos_salariales,
// concepto_salarial_formula, orden_calculo_concepto,
// regimen_concepto_aplicable, parametros_horas_extra.
//
// Vigencia "a una fecha" (NO es_vigente): en un recálculo retroactivo la
// fecha efectiva puede ser anterior a la versión hoy vigente, por eso todas
// las consultas usan  inicio <= fecha AND (fin IS NULL OR fin >= fecha)
// en vez de la columna generada es_vigente.
package dto

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// CONTRATO VIGENTE A UNA FECHA (régimen laboral + salario pactado)
// ============================================================================

type ContratoVigenteEnFechaRow struct {
	ContratoID       int64
	EmpleadoID       int64
	RegimenLaboralID int64
	SalarioPactado   decimal.Decimal
	Jornada          string
}

// ObtenerContratoVigenteEnFecha devuelve la versión de contrato cuya
// vigencia cubre `fecha`. Si no hay ninguna devuelve sql.ErrNoRows (el
// llamador NO debe asumir un régimen por defecto).
func (q *Queries) ObtenerContratoVigenteEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (ContratoVigenteEnFechaRow, error) {
	var r ContratoVigenteEnFechaRow
	err := q.db.QueryRowContext(ctx, `
SELECT id, empleado_id, regimen_laboral_id, salario_pactado, jornada
FROM contratos
WHERE empleado_id = ?
  AND fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY fecha_inicio_vigencia DESC
LIMIT 1`, empleadoID, fecha, fecha).
		Scan(&r.ContratoID, &r.EmpleadoID, &r.RegimenLaboralID, &r.SalarioPactado, &r.Jornada)
	return r, err
}

// ============================================================================
// CONCEPTOS APLICABLES (régimen ∩ orden de cálculo ∩ fórmula) A UNA FECHA
// ============================================================================

type ConceptoAplicableRow struct {
	ConceptoSalarialID int64
	Codigo             string
	Naturaleza         string // 'ingreso' | 'deduccion'
	TipoCalculo        string // 'fijo' | 'formula' | 'captura_manual'
	OrdenCalculo       int32

	// NULL cuando el concepto no tiene versión de fórmula vigente a la fecha
	// (esperado solo para captura_manual).
	FormulaID        *int64
	MontoFijo        decimal.NullDecimal
	ExpresionFormula *string

	AfectaBaseCCSS       bool
	AfectaBaseRenta      bool
	AfectaBaseAguinaldo  bool
	AfectaBaseCesantia   bool
	AfectaBaseVacaciones bool
}

// ListarConceptosAplicablesEnFecha resuelve, para un régimen laboral y una
// fecha, los conceptos que aplican: (a) regimen_concepto_aplicable vigente,
// (b) con orden de cálculo vigente (INNER: un concepto sin orden no se puede
// calcular), (c) con su fórmula/monto vigente (LEFT: captura_manual no tiene).
// Ordenado por orden, luego id, para que el cálculo sea determinístico.
func (q *Queries) ListarConceptosAplicablesEnFecha(ctx context.Context, regimenLaboralID int64, fecha time.Time) ([]ConceptoAplicableRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT
    cs.id, cs.codigo, cs.naturaleza, cs.tipo_calculo, occ.orden,
    csf.id, csf.monto_fijo, csf.expresion_formula,
    cs.afecta_base_ccss, cs.afecta_base_renta, cs.afecta_base_aguinaldo,
    cs.afecta_base_cesantia, cs.afecta_base_vacaciones
FROM regimen_concepto_aplicable rca
JOIN conceptos_salariales cs
  ON cs.id = rca.concepto_salarial_id
 AND cs.activo = 1
JOIN orden_calculo_concepto occ
  ON occ.concepto_salarial_id = cs.id
 AND occ.fecha_inicio_vigencia <= ?
 AND (occ.fecha_fin_vigencia IS NULL OR occ.fecha_fin_vigencia >= ?)
LEFT JOIN concepto_salarial_formula csf
  ON csf.concepto_salarial_id = cs.id
 AND csf.fecha_inicio_vigencia <= ?
 AND (csf.fecha_fin_vigencia IS NULL OR csf.fecha_fin_vigencia >= ?)
WHERE rca.regimen_laboral_id = ?
  AND rca.fecha_inicio_vigencia <= ?
  AND (rca.fecha_fin_vigencia IS NULL OR rca.fecha_fin_vigencia >= ?)
ORDER BY occ.orden ASC, cs.id ASC`,
		fecha, fecha, fecha, fecha, regimenLaboralID, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ConceptoAplicableRow
	for rows.Next() {
		var r ConceptoAplicableRow
		if err := rows.Scan(&r.ConceptoSalarialID, &r.Codigo, &r.Naturaleza, &r.TipoCalculo, &r.OrdenCalculo,
			&r.FormulaID, &r.MontoFijo, &r.ExpresionFormula,
			&r.AfectaBaseCCSS, &r.AfectaBaseRenta, &r.AfectaBaseAguinaldo,
			&r.AfectaBaseCesantia, &r.AfectaBaseVacaciones); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ============================================================================
// PARAMETROS_HORAS_EXTRA A UNA FECHA
// ============================================================================

type ParametroHoraExtraRow struct {
	TipoHoraExtraID int64
	FactorRecargo   decimal.Decimal
}

// ListarParametrosHorasExtraEnFecha devuelve el factor vigente por tipo de
// hora extra. factor_recargo es el MULTIPLICADOR TOTAL sobre el valor de la
// hora ordinaria (1.5 = pago al 150%), tal como lo siembra el schema.
func (q *Queries) ListarParametrosHorasExtraEnFecha(ctx context.Context, fecha time.Time) ([]ParametroHoraExtraRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT tipo_hora_extra_id, factor_recargo
FROM parametros_horas_extra
WHERE fecha_inicio_vigencia <= ?
  AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
ORDER BY tipo_hora_extra_id ASC`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ParametroHoraExtraRow
	for rows.Next() {
		var r ParametroHoraExtraRow
		if err := rows.Scan(&r.TipoHoraExtraID, &r.FactorRecargo); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}