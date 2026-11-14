package contabilidad

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

// DetallesSQL reconstruye la corrida de un período leyendo
// planilla_detalle_empleado, planilla_detalle_concepto y
// planilla_detalle_patronal.
type DetallesSQL struct {
	db *sql.DB
}

func NewDetallesSQL(db *sql.DB) (*DetallesSQL, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: conexión a la base de datos obligatoria", ErrConfigInvalida)
	}
	return &DetallesSQL{db: db}, nil
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

	rows, err := r.db.QueryContext(ctx, `
		SELECT pde.empleado_id, pde.programa_presupuestario_id, pde.neto_pagado,
		       pde.base_ccss, pde.cuota_obrera_ccss, pde.cuota_patronal_ccss,
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

	var orden []*DetallePlanilla
	porID := map[int64]*DetallePlanilla{}
	for rows.Next() {
		var (
			empID, prog                      int64
			neto, baseCCSS, obrero, patronal decimal.Decimal
			conceptoID                       sql.NullInt64
			codigo, naturaleza               sql.NullString
			afectaCCSS                       sql.NullInt64
			monto, cantidad                  decimal.NullDecimal
		)
		if err := rows.Scan(&empID, &prog, &neto, &baseCCSS, &obrero, &patronal,
			&conceptoID, &codigo, &naturaleza, &afectaCCSS, &monto, &cantidad); err != nil {
			return nil, fmt.Errorf("contabilidad: leyendo fila de detalle: %w", err)
		}
		e := porID[empID]
		if e == nil {
			e = &DetallePlanilla{
				ProgramaPresupuestarioID: prog,
				Calculo: &planilla.CalculoEmpleadoDetalle{ResultadoEmpleadoOutput: planilla.ResultadoEmpleadoOutput{
					EmpleadoID: empID, PeriodoID: periodoID, NetoPagado: neto,
					BaseCCSS: baseCCSS, CuotaObreroCCSS: obrero, CuotaPatronalCCSS: patronal,
				}},
			}
			porID[empID] = e
			orden = append(orden, e)
		}
		if !conceptoID.Valid || !monto.Valid {
			continue
		}
		linea := planilla.ConceptoCalculadoOutput{
			ConceptoSalarialID: conceptoID.Int64,
			Codigo:             codigo.String,
			Naturaleza:         naturaleza.String,
			Monto:              monto.Decimal,
			AfectaBaseCCSS:     afectaCCSS.Int64 == 1,
		}
		if cantidad.Valid {
			linea.Cantidad = cantidad.Decimal
		}
		res := &e.Calculo.ResultadoEmpleadoOutput
		res.DesglosePorConcepto = append(res.DesglosePorConcepto, linea)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("contabilidad: recorriendo detalle de planilla: %w", err)
	}
	if len(orden) == 0 {
		return nil, ErrSinDetalles
	}
	if err := r.cargarPatronal(ctx, periodoID, porID); err != nil {
		return nil, err
	}
	out := make([]DetallePlanilla, 0, len(orden))
	for _, e := range orden {
		out = append(out, *e)
	}
	return out, nil
}

// cargarPatronal agrega a cada empleado sus líneas patronales persistidas (una
// por concepto de carga social). Un concepto patronal que además figure en el
// desglose del neto es un detalle inconsistente.
func (r *DetallesSQL) cargarPatronal(ctx context.Context, periodoID int64, porID map[int64]*DetallePlanilla) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pde.empleado_id, pat.concepto_salarial_id, pat.base, pat.porcentaje, pat.monto
		FROM planilla_detalle_patronal pat
		JOIN planilla_detalle_empleado pde ON pde.id = pat.planilla_detalle_empleado_id
		WHERE pde.periodo_planilla_id = ?
		ORDER BY pde.empleado_id, pat.concepto_salarial_id`, periodoID)
	if err != nil {
		return fmt.Errorf("contabilidad: leyendo cargas patronales del periodo_id=%d: %w", periodoID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var empID int64
		var l planilla.LineaPatronal
		if err := rows.Scan(&empID, &l.ConceptoSalarialID, &l.Base, &l.Porcentaje, &l.Monto); err != nil {
			return fmt.Errorf("contabilidad: leyendo línea patronal: %w", err)
		}
		e := porID[empID]
		if e == nil {
			return fmt.Errorf("%w: línea patronal de empleado_id=%d sin detalle de empleado", ErrDetalleInconsistente, empID)
		}
		for _, d := range e.Calculo.DesglosePorConcepto {
			if d.ConceptoSalarialID == l.ConceptoSalarialID {
				return fmt.Errorf("%w: empleado_id=%d trae el concepto patronal %q también en el desglose del neto", ErrDetalleInconsistente, empID, d.Codigo)
			}
		}
		e.Calculo.Patronal = append(e.Calculo.Patronal, l)
	}
	return rows.Err()
}

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
