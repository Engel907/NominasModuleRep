package contabilidad

// Adaptadores database/sql de Fase 11B:
//
//   - DetallesSQL reconstruye, desde lo persistido, los DetallePlanilla que
//     exige Service.ContabilizarPeriodo.
//   - LectorSQL lee asientos ya generados (cabecera + líneas).
//
// Asume que el DSN usa parseTime=true (igual que el código sqlc del resto del
// proyecto, que ya recibe/entrega time.Time).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/planilla"
)

// ============================================================================
// DetallesSQL
// ============================================================================

// DetallesSQL reconstruye la corrida de un período leyendo
// planilla_detalle_empleado y planilla_detalle_concepto.
//
// Lo único que NO está persistido es la cuota patronal de CCSS. Se deriva con
// la misma fórmula del motor (motor_calculo.go / CalcularCCSS):
//
//	patronal = round2( Σ ingresos con afecta_base_ccss × %patronal )
//
// usando el % vigente de parametros_cargas_sociales a periodos_planilla.fecha_fin.
//
// Supuestos verificables (si fallan, se retorna error; nunca se contabiliza a ciegas):
//  1. La FechaEfectiva del cálculo fue periodos_planilla.fecha_fin.
//  2. El % patronal es el de la fila vigente del concepto CCSS_PAT y el obrero
//     el de CCSS_OBR, guardados como puntos porcentuales (26.8300 = 26.83 %),
//     por lo que se dividen entre 100 (el motor trabaja con fracciones).
//
// D3 — escala de porcentajes: el DOMINIO trabaja con fracciones (0.2683); la
// base de datos guarda puntos porcentuales (parametros_cargas_sociales.
// porcentaje_* DECIMAL(6,4), seed 26.8300 / 10.8300). La división entre 100
// vive SOLO en porcentajeVigente (único punto de conversión de este archivo).
// TODO(F18): mover la conversión a un helper compartido de parámetros.
//
// Solo se reconstruyen períodos en estado `pagada` (D4); la verificación
// definitiva (con bloqueo FOR UPDATE) la hace Service.ContabilizarPeriodo
// dentro de la transacción. Aquí se comprueba antes para devolver el error
// correcto (409) en vez de uno derivado ("sin detalles").
//
// Salvaguarda: con el mismo método se recalcula la cuota OBRERA y se compara con
// la línea CCSS_OBR persistida de cada empleado. Si no coincide, algún supuesto
// no se cumple y se aborta. Aun así, la solución de fondo es persistir la cuota
// patronal al calcular.
type DetallesSQL struct {
	db             *sql.DB
	codigoPatronal string
	codigoObrero   string
}

// NewDetallesSQL: cfg aporta el código del concepto patronal (Seed: CCSS_PAT);
// codigoObrero es el del concepto obrero (Seed: CCSS_OBR).
func NewDetallesSQL(db *sql.DB, cfg ConfigGeneracion, codigoObrero string) (*DetallesSQL, error) {
	if strings.TrimSpace(cfg.ConceptoCuotaPatronalCCSS) == "" || strings.TrimSpace(codigoObrero) == "" {
		return nil, fmt.Errorf("%w: códigos de concepto CCSS obligatorios", ErrConfigInvalida)
	}
	return &DetallesSQL{db: db, codigoPatronal: cfg.ConceptoCuotaPatronalCCSS, codigoObrero: codigoObrero}, nil
}

var cien = decimal.NewFromInt(100)

// porcentajeVigente devuelve (patronal, obrero) como FRACCIÓN, exigiendo
// exactamente una fila vigente a fecha_fin del período.
func (r *DetallesSQL) porcentajeVigente(ctx context.Context, periodoID int64, codigo string) (decimal.Decimal, decimal.Decimal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pcs.porcentaje_patronal, pcs.porcentaje_obrero
		FROM parametros_cargas_sociales pcs
		JOIN conceptos_salariales cs ON cs.id = pcs.concepto_salarial_id
		JOIN periodos_planilla pp    ON pp.id = ?
		WHERE cs.codigo = ?
		  AND pcs.fecha_inicio_vigencia <= pp.fecha_fin
		  AND (pcs.fecha_fin_vigencia IS NULL OR pcs.fecha_fin_vigencia >= pp.fecha_fin)`, periodoID, codigo)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	defer rows.Close()

	var pat, obr decimal.Decimal
	n := 0
	for rows.Next() {
		if err := rows.Scan(&pat, &obr); err != nil {
			return decimal.Zero, decimal.Zero, err
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	if n != 1 {
		return decimal.Zero, decimal.Zero, fmt.Errorf("%w: se esperaba 1 parametros_cargas_sociales vigente para %q a la fecha fin del periodo_id=%d y hay %d", ErrConfigInvalida, codigo, periodoID, n)
	}
	return pat.Div(cien), obr.Div(cien), nil
}

func (r *DetallesSQL) DetallesParaContabilizar(ctx context.Context, periodoID int64) ([]DetallePlanilla, error) {
	var estado string
	if err := r.db.QueryRowContext(ctx, `SELECT estado FROM periodos_planilla WHERE id = ?`, periodoID).Scan(&estado); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: periodo_id=%d", ErrPeriodoNoExiste, periodoID)
		}
		return nil, fmt.Errorf("contabilidad: leyendo periodo_id=%d: %w", periodoID, err)
	}
	if planilla.EstadoPeriodo(estado) != estadoContabilizable {
		return nil, fmt.Errorf("%w: periodo_id=%d está %q (se requiere %q)", ErrPeriodoEstadoInvalido, periodoID, estado, estadoContabilizable)
	}

	pctPat, _, err := r.porcentajeVigente(ctx, periodoID, r.codigoPatronal)
	if err != nil {
		return nil, err
	}
	_, pctObr, err := r.porcentajeVigente(ctx, periodoID, r.codigoObrero)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT pde.empleado_id, pde.programa_presupuestario_id, pde.neto_pagado,
		       pdc.concepto_salarial_id, cs.codigo, cs.naturaleza, cs.afecta_base_ccss,
		       pdc.monto, pdc.cantidad
		FROM planilla_detalle_empleado pde
		LEFT JOIN planilla_detalle_concepto pdc ON pdc.planilla_detalle_empleado_id = pde.id
		LEFT JOIN conceptos_salariales cs       ON cs.id = pdc.concepto_salarial_id
		WHERE pde.periodo_planilla_id = ?
		ORDER BY pde.empleado_id, pdc.id`, periodoID)
	if err != nil {
		return nil, fmt.Errorf("contabilidad: leyendo detalle de planilla del periodo_id=%d: %w", periodoID, err)
	}
	defer rows.Close()

	type emp struct {
		det         DetallePlanilla
		baseCCSS    decimal.Decimal
		obreroSaved decimal.Decimal
	}
	var orden []*emp
	porID := map[int64]*emp{}

	for rows.Next() {
		var (
			empID, prog int64
			neto        decimal.Decimal
			conceptoID  sql.NullInt64
			codigo      sql.NullString
			naturaleza  sql.NullString
			afectaCCSS  sql.NullInt64
			monto       decimal.NullDecimal
			cantidad    decimal.NullDecimal
		)
		if err := rows.Scan(&empID, &prog, &neto, &conceptoID, &codigo, &naturaleza, &afectaCCSS, &monto, &cantidad); err != nil {
			return nil, fmt.Errorf("contabilidad: leyendo fila de detalle: %w", err)
		}
		e := porID[empID]
		if e == nil {
			e = &emp{det: DetallePlanilla{
				ProgramaPresupuestarioID: prog,
				Calculo: &planilla.CalculoEmpleadoDetalle{ResultadoEmpleadoOutput: planilla.ResultadoEmpleadoOutput{
					EmpleadoID: empID, PeriodoID: periodoID, NetoPagado: neto,
				}},
			}}
			porID[empID] = e
			orden = append(orden, e)
		}
		if !conceptoID.Valid || !monto.Valid {
			continue // empleado sin líneas de concepto
		}
		linea := planilla.ConceptoCalculadoOutput{
			ConceptoSalarialID: conceptoID.Int64,
			Codigo:             codigo.String,
			Monto:              monto.Decimal,
			AfectaBaseCCSS:     afectaCCSS.Int64 == 1,
		}
		if cantidad.Valid {
			linea.Cantidad = cantidad.Decimal
		}
		if linea.Codigo == r.codigoPatronal {
			// La cuota patronal se DERIVA más abajo; si además viniera
			// persistida como línea se contaría dos veces.
			return nil, fmt.Errorf("%w: empleado_id=%d trae persistida una línea %q; la cuota patronal se deriva y no debe existir como concepto", ErrDetalleInconsistente, empID, r.codigoPatronal)
		}
		res := &e.det.Calculo.ResultadoEmpleadoOutput
		res.DesglosePorConcepto = append(res.DesglosePorConcepto, linea)
		if naturaleza.String == "ingreso" && linea.AfectaBaseCCSS {
			e.baseCCSS = e.baseCCSS.Add(monto.Decimal)
		}
		if linea.Codigo == r.codigoObrero {
			e.obreroSaved = e.obreroSaved.Add(monto.Decimal)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("contabilidad: recorriendo detalle de planilla: %w", err)
	}
	if len(orden) == 0 {
		return nil, ErrSinDetalles
	}

	out := make([]DetallePlanilla, 0, len(orden))
	for _, e := range orden {
		base := e.baseCCSS.Round(dosDecimales)
		obreroEsperado := base.Mul(pctObr).Round(dosDecimales)
		if !obreroEsperado.Equal(e.obreroSaved) {
			return nil, fmt.Errorf("%w: empleado_id=%d: la cuota obrera recalculada (%s) no coincide con la persistida (%s); no se puede derivar la cuota patronal con certeza",
				ErrDetalleInconsistente, e.det.Calculo.EmpleadoID, obreroEsperado.StringFixed(dosDecimales), e.obreroSaved.StringFixed(dosDecimales))
		}
		res := &e.det.Calculo.ResultadoEmpleadoOutput
		res.BaseCCSS = base
		res.CuotaObreroCCSS = e.obreroSaved
		res.CuotaPatronalCCSS = base.Mul(pctPat).Round(dosDecimales)
		out = append(out, e.det)
	}
	return out, nil
}

// ============================================================================
// LectorSQL
// ============================================================================

type LectorSQL struct{ db *sql.DB }

func NewLectorSQL(db *sql.DB) *LectorSQL { return &LectorSQL{db: db} }

// ListarAsientos devuelve asientos con sus líneas, ordenados por período y tipo.
func (l *LectorSQL) ListarAsientos(ctx context.Context, f FiltroAsientos) ([]AsientoContable, error) {
	q := `SELECT ac.id, ac.periodo_planilla_id, ac.tipo_asiento_id, ta.codigo,
	             ac.numero_asiento, ac.fecha_generacion, ac.estado
	      FROM asientos_contables ac
	      JOIN tipos_asiento ta ON ta.id = ac.tipo_asiento_id
	      WHERE 1=1`
	var args []any
	if f.PeriodoID != nil {
		q += ` AND ac.periodo_planilla_id = ?`
		args = append(args, *f.PeriodoID)
	}
	if f.TipoCodigo != "" {
		q += ` AND ta.codigo = ?`
		args = append(args, f.TipoCodigo)
	}
	q += ` ORDER BY ac.periodo_planilla_id, ac.tipo_asiento_id`

	rows, err := l.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("contabilidad: consultando asientos: %w", err)
	}
	defer rows.Close()

	asientos := []AsientoContable{}
	idx := map[int64]int{}
	for rows.Next() {
		var a AsientoContable
		var fecha time.Time
		if err := rows.Scan(&a.ID, &a.PeriodoPlanillaID, &a.TipoAsientoID, &a.TipoAsientoCodigo, &a.NumeroAsiento, &fecha, &a.Estado); err != nil {
			return nil, err
		}
		a.FechaGeneracion = fecha
		idx[a.ID] = len(asientos)
		asientos = append(asientos, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(asientos) == 0 {
		return asientos, nil
	}

	ph := make([]string, 0, len(asientos))
	largs := make([]any, 0, len(asientos))
	for _, a := range asientos {
		ph = append(ph, "?")
		largs = append(largs, a.ID)
	}
	lrows, err := l.db.QueryContext(ctx, `
		SELECT id, asiento_contable_id, cuenta_contable_id, programa_presupuestario_id, proyecto,
		       empleado_id, concepto_salarial_id, monto_debe, monto_haber
		FROM asientos_contables_detalle
		WHERE asiento_contable_id IN (`+strings.Join(ph, ",")+`)
		ORDER BY asiento_contable_id, programa_presupuestario_id, id`, largs...)
	if err != nil {
		return nil, fmt.Errorf("contabilidad: consultando líneas de asientos: %w", err)
	}
	defer lrows.Close()
	for lrows.Next() {
		var d AsientoContableDetalle
		var emp, con sql.NullInt64
		if err := lrows.Scan(&d.ID, &d.AsientoContableID, &d.CuentaContableID, &d.ProgramaPresupuestarioID, &d.Proyecto, &emp, &con, &d.MontoDebe, &d.MontoHaber); err != nil {
			return nil, err
		}
		if emp.Valid {
			v := emp.Int64
			d.EmpleadoID = &v
		}
		if con.Valid {
			v := con.Int64
			d.ConceptoSalarialID = &v
		}
		i := idx[d.AsientoContableID]
		asientos[i].Lineas = append(asientos[i].Lineas, d)
	}
	if err := lrows.Err(); err != nil {
		return nil, err
	}
	return asientos, nil
}
