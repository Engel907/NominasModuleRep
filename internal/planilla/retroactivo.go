package planilla

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
)

// RetroactivoInput: diferencia de un concepto de ingreso de un período ya
// aprobado que paga el período en cálculo. Monto puede ser negativo (un
// concepto que bajó), pero la diferencia neta del período de origen es
// siempre positiva.
type RetroactivoInput struct {
	ConceptoSalarialID int64
	Monto              decimal.Decimal
	PeriodoOrigenID    int64
	LineaOrigenID      int64 // línea del período de origen que corrige (0 = no tenía)
	Motivo             string
}

// largoMotivoAjuste: planilla_detalle_concepto.motivo_ajuste.
const largoMotivoAjuste = 255

// retroactivosEmpleado: por cada período regular aprobado del empleado con
// fecha_fin en [desde, hasta] del retroactivo, lo recalcula hoy (contratos,
// parámetros y registros vigentes para esas fechas) y compara sus ingresos por
// concepto con lo que pagó más las diferencias que ya calcularon otros
// períodos. Un período cuya diferencia neta no es positiva no se paga (queda
// una advertencia): descontar lo pagado de más no es un retroactivo.
func (o *Orquestador) retroactivosEmpleado(ctx context.Context, q *dto.Queries, destino dto.PeriodoCicloRow, sol dto.RetroactivoRow) ([]RetroactivoInput, []string, error) {
	origenes, err := q.ListarPeriodosOrigenRetroactivo(ctx, sol.EmpleadoID, sol.Desde, sol.Hasta, destino.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("planilla: empleado_id=%d, períodos del retroactivo: %w", sol.EmpleadoID, err)
	}
	var (
		out []RetroactivoInput
		adv []string
	)
	for _, org := range origenes {
		if !soloFecha(org.FechaFin).Before(soloFecha(destino.FechaInicio)) {
			continue
		}
		etiqueta := fmt.Sprintf("%s del %s al %s", org.Codigo, org.FechaInicio.Format("2006-01-02"), org.FechaFin.Format("2006-01-02"))
		recalculado, err := o.recalcularOrigen(ctx, org, sol.EmpleadoID)
		if err != nil {
			return nil, nil, fmt.Errorf("retroactivo, período %s: %w", etiqueta, err)
		}
		pagados, err := q.SumarIngresosPagadosPeriodo(ctx, sol.EmpleadoID, org.ID, destino.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("planilla: empleado_id=%d, pagado en el período %d: %w", sol.EmpleadoID, org.ID, err)
		}
		lineas := map[int64]int64{}
		diferencia := map[int64]decimal.Decimal{}
		for id, m := range recalculado {
			diferencia[id] = m
		}
		for _, pg := range pagados {
			diferencia[pg.ConceptoSalarialID] = diferencia[pg.ConceptoSalarialID].Sub(pg.Monto)
			lineas[pg.ConceptoSalarialID] = pg.LineaID
		}
		ids := make([]int64, 0, len(diferencia))
		neta := decimal.Zero
		for id, d := range diferencia {
			d = d.Round(dosDecimales)
			diferencia[id] = d
			if d.IsZero() {
				continue
			}
			ids = append(ids, id)
			neta = neta.Add(d)
		}
		if len(ids) == 0 {
			continue
		}
		if !neta.IsPositive() {
			adv = append(adv, fmt.Sprintf("retroactivo, período %s: la diferencia neta es %s; no se paga", etiqueta, neta.StringFixed(2)))
			continue
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		motivo := recortar(fmt.Sprintf("Retroactivo %s: %s", etiqueta, sol.Motivo), largoMotivoAjuste)
		for _, id := range ids {
			out = append(out, RetroactivoInput{ConceptoSalarialID: id, Monto: diferencia[id],
				PeriodoOrigenID: org.ID, LineaOrigenID: lineas[id], Motivo: motivo})
		}
	}
	return out, adv, nil
}

// recalcularOrigen: ingresos por concepto del empleado en el período de
// origen calculado con los datos de hoy (sin retroactivos, con las
// compensaciones de vacaciones que ese período pagó y sin deducciones de
// terceros, que no cambian lo devengado).
func (o *Orquestador) recalcularOrigen(ctx context.Context, org dto.PeriodoOrigenRetroactivoRow, empleadoID int64) (map[int64]decimal.Decimal, error) {
	tipo, err := o.tipos.ObtenerTipoPlanilla(ctx, org.TipoPlanillaID)
	if err != nil {
		return nil, fmt.Errorf("planilla: leyendo tipo de planilla %d: %w", org.TipoPlanillaID, err)
	}
	pr := dto.PeriodoCicloRow{ID: org.ID, TipoPlanillaID: org.TipoPlanillaID, TipoPlanillaCodigo: org.Codigo,
		FechaInicio: org.FechaInicio, FechaFin: org.FechaFin}
	params, pc, err := o.parametrosRegular(ctx, pr, tipo)
	if err != nil {
		return nil, err
	}
	pcE, pE, _, err := o.parametrosDelEmpleado(ctx, ClaseRegular, pr, pc, *params, empleadoID)
	if err != nil {
		return nil, err
	}
	ent, err := o.h.HidratarRecalculo(ctx, pcE, empleadoID)
	if err != nil {
		return nil, err
	}
	det, err := o.m.CalcularEmpleado(ctx, ent.Input, pE)
	if err != nil {
		return nil, err
	}
	out := map[int64]decimal.Decimal{}
	for _, l := range det.DesglosePorConcepto {
		if l.Naturaleza == naturalezaIngreso {
			out[l.ConceptoSalarialID] = out[l.ConceptoSalarialID].Add(l.Monto)
		}
	}
	return out, nil
}

// parametrosRegular: parámetros y datos de cálculo de un período regular
// (los mismos que usa Calcular para esa clase).
func (o *Orquestador) parametrosRegular(ctx context.Context, periodo dto.PeriodoCicloRow, tipo dto.TipoPlanillaRow) (*ParametrosCalculo, PeriodoParaCalculo, error) {
	if ClaseTipoPlanilla(tipo.Clase) != ClaseRegular || !tipo.FraccionMensual.Valid || !tipo.FraccionMensual.Decimal.IsPositive() {
		return nil, PeriodoParaCalculo{}, fmt.Errorf("planilla: el tipo de planilla %d no es regular con fraccion_mensual válida", tipo.ID)
	}
	params, err := o.c.Cargar(ctx, periodo.FechaInicio, periodo.FechaFin)
	if err != nil {
		return nil, PeriodoParaCalculo{}, err
	}
	divisor, err := divisorEntero(params.DivisorDiasMes)
	if err != nil {
		return nil, PeriodoParaCalculo{}, err
	}
	fraccion := tipo.FraccionMensual.Decimal
	params.FraccionMensualPeriodo = fraccion
	params.DiasPeriodoCompleto = params.DivisorDiasMes.Mul(fraccion)
	if fraccion.LessThan(decimal.NewFromInt(1)) {
		if err := o.c.CargarMetodoRenta(ctx, periodo.FechaFin, params); err != nil {
			return nil, PeriodoParaCalculo{}, err
		}
	}
	pc := PeriodoParaCalculo{
		ID: periodo.ID, Inicio: periodo.FechaInicio, Fin: periodo.FechaFin,
		Clase: tipo.Clase, TipoPlanillaID: tipo.ID, FraccionMensual: fraccion,
		DiasPeriodoCompleto: params.DiasPeriodoCompleto, DivisorDiasMes: divisor,
	}
	pc.conBasesSalario(*params)
	return params, pc, nil
}

// retroactivosDelPeriodo: retroactivos registrados para el período, por
// empleado.
func retroactivosDelPeriodo(ctx context.Context, q *dto.Queries, periodoID int64) (map[int64]dto.RetroactivoRow, error) {
	filas, err := q.ListarRetroactivos(ctx, periodoID, nil, 0, 0)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("planilla: leyendo retroactivos del período %d: %w", periodoID, err)
	}
	out := make(map[int64]dto.RetroactivoRow, len(filas))
	for _, f := range filas {
		out[f.EmpleadoID] = f
	}
	return out, nil
}

func recortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
