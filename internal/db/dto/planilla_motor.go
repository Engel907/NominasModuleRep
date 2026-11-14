package dto

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type ContratoVigenteEnFechaRow struct {
	ContratoID       int64
	EmpleadoID       int64
	RegimenLaboralID int64
	SalarioPactado   decimal.Decimal
	ModalidadSalario string // mensual | diario | por_hora
	Jornada          string // jornadas.codigo
	JornadaID        *int64
}

// ObtenerContratoVigenteEnFecha devuelve la versión de contrato cuya
// vigencia cubre `fecha`. Si no hay ninguna devuelve sql.ErrNoRows (el
// llamador no debe asumir un régimen por defecto).
func (q *Queries) ObtenerContratoVigenteEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (ContratoVigenteEnFechaRow, error) {
	var r ContratoVigenteEnFechaRow
	err := q.db.QueryRowContext(ctx, `
SELECT c.id, c.empleado_id, c.regimen_laboral_id, c.salario_pactado, c.modalidad_salario, j.codigo, c.jornada_id
FROM contratos c
JOIN jornadas j ON j.id = c.jornada_id
WHERE c.empleado_id = ?
  AND c.fecha_inicio_vigencia <= ?
  AND (c.fecha_fin_vigencia IS NULL OR c.fecha_fin_vigencia >= ?)
ORDER BY c.fecha_inicio_vigencia DESC
LIMIT 1`, empleadoID, fecha, fecha).
		Scan(&r.ContratoID, &r.EmpleadoID, &r.RegimenLaboralID, &r.SalarioPactado, &r.ModalidadSalario, &r.Jornada, &r.JornadaID)
	return r, err
}

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
	// FormulaEmpleadoID: fórmula propia del empleado (empleado_concepto_formula)
	// que reemplaza a la general; si no es nil, FormulaID es nil y MontoFijo /
	// ExpresionFormula son los del empleado.
	FormulaEmpleadoID *int64

	AfectaBaseCCSS       bool
	AfectaBaseRenta      bool
	AfectaBaseAguinaldo  bool
	AfectaBaseCesantia   bool
	AfectaBaseVacaciones bool
	AfectaBaseEmbargo    bool

	EsDeduccionObligatoria bool
}

// ListarConceptosAplicablesEnFecha resuelve, para un régimen laboral y una
// fecha, los conceptos que aplican: (a) regimen_concepto_aplicable vigente,
// (b) con orden de cálculo vigente (INNER: un concepto sin orden no se puede
// calcular), (c) con su fórmula/monto vigente (LEFT: captura_manual no tiene),
// (d) que el tipo de planilla calcula (tipo_planilla_concepto).
// Ordenado por orden, luego id, para que el cálculo sea determinístico.
//
// Si el empleado tiene fórmula propia vigente para el concepto
// (empleado_concepto_formula), esa reemplaza a la general.
func (q *Queries) ListarConceptosAplicablesEnFecha(ctx context.Context, empleadoID, regimenLaboralID, tipoPlanillaID int64, fecha time.Time) ([]ConceptoAplicableRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT
    cs.id, cs.codigo, cs.naturaleza, cs.tipo_calculo, occ.orden,
    CASE WHEN ecf.id IS NULL THEN csf.id END,
    CASE WHEN ecf.id IS NULL THEN csf.monto_fijo ELSE ecf.monto_fijo END,
    CASE WHEN ecf.id IS NULL THEN csf.expresion_formula ELSE ecf.expresion_formula END,
    ecf.id,
    cs.afecta_base_ccss, cs.afecta_base_renta, cs.afecta_base_aguinaldo,
    cs.afecta_base_cesantia, cs.afecta_base_vacaciones, cs.afecta_base_embargo,
    cs.es_deduccion_obligatoria
FROM regimen_concepto_aplicable rca
JOIN conceptos_salariales cs
  ON cs.id = rca.concepto_salarial_id
 AND cs.activo = 1
JOIN tipo_planilla_concepto tpc
  ON tpc.concepto_salarial_id = cs.id
 AND tpc.tipo_planilla_id = ?
JOIN orden_calculo_concepto occ
  ON occ.concepto_salarial_id = cs.id
 AND occ.fecha_inicio_vigencia <= ?
 AND (occ.fecha_fin_vigencia IS NULL OR occ.fecha_fin_vigencia >= ?)
LEFT JOIN concepto_salarial_formula csf
  ON csf.concepto_salarial_id = cs.id
 AND csf.fecha_inicio_vigencia <= ?
 AND (csf.fecha_fin_vigencia IS NULL OR csf.fecha_fin_vigencia >= ?)
LEFT JOIN empleado_concepto_formula ecf
  ON ecf.concepto_salarial_id = cs.id
 AND ecf.empleado_id = ?
 AND ecf.fecha_inicio_vigencia <= ?
 AND (ecf.fecha_fin_vigencia IS NULL OR ecf.fecha_fin_vigencia >= ?)
WHERE rca.regimen_laboral_id = ?
  AND rca.fecha_inicio_vigencia <= ?
  AND (rca.fecha_fin_vigencia IS NULL OR rca.fecha_fin_vigencia >= ?)
ORDER BY occ.orden ASC, cs.id ASC`,
		tipoPlanillaID, fecha, fecha, fecha, fecha, empleadoID, fecha, fecha, regimenLaboralID, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ConceptoAplicableRow
	for rows.Next() {
		var r ConceptoAplicableRow
		if err := rows.Scan(&r.ConceptoSalarialID, &r.Codigo, &r.Naturaleza, &r.TipoCalculo, &r.OrdenCalculo,
			&r.FormulaID, &r.MontoFijo, &r.ExpresionFormula, &r.FormulaEmpleadoID,
			&r.AfectaBaseCCSS, &r.AfectaBaseRenta, &r.AfectaBaseAguinaldo,
			&r.AfectaBaseCesantia, &r.AfectaBaseVacaciones, &r.AfectaBaseEmbargo,
			&r.EsDeduccionObligatoria); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

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

type ParametroJornadaRow struct {
	JornadaID              int64
	JornadaCodigo          string
	DivisorHorasMensuales  decimal.Decimal
	HorasDiariasOrdinarias decimal.NullDecimal
}

// ListarParametrosJornadaEnFecha: divisor de la hora ordinaria vigente por jornada.
func (q *Queries) ListarParametrosJornadaEnFecha(ctx context.Context, fecha time.Time) ([]ParametroJornadaRow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT p.jornada_id, j.codigo, p.divisor_horas_mensuales, p.horas_diarias_ordinarias
FROM parametros_jornada p
JOIN jornadas j ON j.id = p.jornada_id
WHERE p.fecha_inicio_vigencia <= ?
  AND (p.fecha_fin_vigencia IS NULL OR p.fecha_fin_vigencia >= ?)
ORDER BY p.jornada_id ASC`, fecha, fecha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ParametroJornadaRow
	for rows.Next() {
		var r ParametroJornadaRow
		if err := rows.Scan(&r.JornadaID, &r.JornadaCodigo, &r.DivisorHorasMensuales, &r.HorasDiariasOrdinarias); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
