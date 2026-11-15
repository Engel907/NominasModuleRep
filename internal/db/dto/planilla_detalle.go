package dto

import (
	"context"
	"strings"

	"github.com/shopspring/decimal"
)

// tamLote: filas por sentencia INSERT multi-fila.
const tamLote = 500

// insertarLote inserta filas en `tabla` en sentencias de hasta tamLote filas.
// `tabla` y `cols` son constantes de este archivo, nunca datos del usuario.
func (q *Queries) insertarLote(ctx context.Context, tabla string, cols []string, filas [][]any) error {
	fila := "(" + strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",") + ")"
	for ini := 0; ini < len(filas); ini += tamLote {
		fin := min(ini+tamLote, len(filas))
		lote := filas[ini:fin]
		var sb strings.Builder
		sb.WriteString("INSERT INTO " + tabla + " (" + strings.Join(cols, ", ") + ") VALUES ")
		args := make([]any, 0, len(lote)*len(cols))
		for i, f := range lote {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fila)
			args = append(args, f...)
		}
		if _, err := q.db.ExecContext(ctx, sb.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

type DetalleEmpleadoInsert struct {
	EmpleadoID                int64
	ProgramaPresupuestarioID  int64
	ContratoID                int64
	SalarioBasePeriodo        decimal.Decimal
	TotalIngresos             decimal.Decimal
	TotalDeducciones          decimal.Decimal
	NetoPagado                decimal.Decimal
	BaseCCSS                  decimal.Decimal
	BaseRenta                 decimal.Decimal
	BaseAguinaldo             decimal.Decimal
	BaseCesantia              decimal.Decimal
	BaseVacaciones            decimal.Decimal
	BaseEmbargo               decimal.Decimal
	CuotaObreraCCSS           decimal.Decimal
	CuotaPatronalCCSS         decimal.Decimal
	RentaBruta                decimal.Decimal
	CreditosFiscalesAplicados decimal.Decimal
	RentaNeta                 decimal.Decimal
	Advertencias              []byte // JSON; nil = NULL
}

var colsDetalleEmpleado = []string{
	"periodo_planilla_id", "empleado_id", "programa_presupuestario_id", "contrato_id",
	"salario_base_periodo", "total_ingresos", "total_deducciones", "neto_pagado",
	"base_ccss", "base_renta", "base_aguinaldo", "base_cesantia", "base_vacaciones", "base_embargo",
	"cuota_obrera_ccss", "cuota_patronal_ccss", "renta_bruta", "creditos_fiscales_aplicados",
	"renta_neta", "advertencias",
}

// InsertarDetallesEmpleado inserta las líneas de empleado del período y
// devuelve empleado_id -> planilla_detalle_empleado.id (leído de vuelta, sin
// suponer ids consecutivos).
func (q *Queries) InsertarDetallesEmpleado(ctx context.Context, periodoID int64, filas []DetalleEmpleadoInsert) (map[int64]int64, error) {
	vals := make([][]any, 0, len(filas))
	for _, f := range filas {
		var adv any
		if f.Advertencias != nil {
			adv = string(f.Advertencias)
		}
		vals = append(vals, []any{
			periodoID, f.EmpleadoID, f.ProgramaPresupuestarioID, f.ContratoID,
			f.SalarioBasePeriodo, f.TotalIngresos, f.TotalDeducciones, f.NetoPagado,
			f.BaseCCSS, f.BaseRenta, f.BaseAguinaldo, f.BaseCesantia, f.BaseVacaciones, f.BaseEmbargo,
			f.CuotaObreraCCSS, f.CuotaPatronalCCSS, f.RentaBruta, f.CreditosFiscalesAplicados,
			f.RentaNeta, adv,
		})
	}
	if err := q.insertarLote(ctx, "planilla_detalle_empleado", colsDetalleEmpleado, vals); err != nil {
		return nil, err
	}

	rows, err := q.db.QueryContext(ctx,
		`SELECT empleado_id, id FROM planilla_detalle_empleado WHERE periodo_planilla_id = ?`, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]int64, len(filas))
	for rows.Next() {
		var emp, id int64
		if err := rows.Scan(&emp, &id); err != nil {
			return nil, err
		}
		out[emp] = id
	}
	return out, rows.Err()
}

type DetalleConceptoInsert struct {
	DetalleEmpleadoID  int64
	ConceptoSalarialID int64
	FormulaID          *int64           // concepto_salarial_formula_id
	FormulaEmpleadoID  *int64           // empleado_concepto_formula_id (excluye FormulaID)
	Cantidad           *decimal.Decimal // NULL = no aplica
	Monto              decimal.Decimal
	// Línea de retroactivo: período cuya diferencia paga, línea de ese período
	// que corrige (si la tiene) y motivo. NULL en las líneas propias.
	RetroactivoPeriodoOrigenID *int64
	AjusteDeID                 *int64
	MotivoAjuste               *string
}

func (q *Queries) InsertarDetallesConcepto(ctx context.Context, filas []DetalleConceptoInsert) error {
	vals := make([][]any, 0, len(filas))
	for _, f := range filas {
		vals = append(vals, []any{f.DetalleEmpleadoID, f.ConceptoSalarialID, f.FormulaID, f.FormulaEmpleadoID, f.Cantidad, f.Monto,
			f.AjusteDeID, f.MotivoAjuste, f.RetroactivoPeriodoOrigenID})
	}
	return q.insertarLote(ctx, "planilla_detalle_concepto",
		[]string{"planilla_detalle_empleado_id", "concepto_salarial_id", "concepto_salarial_formula_id", "empleado_concepto_formula_id", "cantidad", "monto",
			"concepto_ajuste_de_id", "motivo_ajuste", "retroactivo_periodo_origen_id"}, vals)
}

type DetallePatronalInsert struct {
	DetalleEmpleadoID  int64
	ConceptoSalarialID int64
	Base               decimal.Decimal
	Porcentaje         decimal.Decimal // FRACCIÓN (0.2667)
	Monto              decimal.Decimal
}

func (q *Queries) InsertarDetallesPatronal(ctx context.Context, filas []DetallePatronalInsert) error {
	vals := make([][]any, 0, len(filas))
	for _, f := range filas {
		vals = append(vals, []any{f.DetalleEmpleadoID, f.ConceptoSalarialID, f.Base, f.Porcentaje, f.Monto})
	}
	return q.insertarLote(ctx, "planilla_detalle_patronal",
		[]string{"planilla_detalle_empleado_id", "concepto_salarial_id", "base", "porcentaje", "monto"}, vals)
}

type DetalleEmbargoInsert struct {
	DetalleEmpleadoID int64
	EmbargoID         int64
	OrdenPrioridad    int32
	MontoAplicado     decimal.Decimal // columna `monto`
	MontoCalculado    decimal.Decimal
	MontoInsatisfecho decimal.Decimal
}

func (q *Queries) InsertarDetallesEmbargo(ctx context.Context, filas []DetalleEmbargoInsert) error {
	vals := make([][]any, 0, len(filas))
	for _, f := range filas {
		vals = append(vals, []any{f.DetalleEmpleadoID, f.EmbargoID, f.OrdenPrioridad, f.MontoAplicado, f.MontoCalculado, f.MontoInsatisfecho})
	}
	return q.insertarLote(ctx, "planilla_detalle_embargo",
		[]string{"planilla_detalle_empleado_id", "embargo_id", "orden_prioridad", "monto", "monto_calculado", "monto_insatisfecho"}, vals)
}

type DetalleIncapacidadInsert struct {
	DetalleEmpleadoID       int64
	IncapacidadID           int64
	DiaSerieDesde           int32
	DiaSerieHasta           int32
	Dias                    decimal.Decimal
	MontoPatronal           decimal.Decimal
	MontoEntidadInformativo decimal.Decimal
}

func (q *Queries) InsertarDetallesIncapacidad(ctx context.Context, filas []DetalleIncapacidadInsert) error {
	vals := make([][]any, 0, len(filas))
	for _, f := range filas {
		vals = append(vals, []any{f.DetalleEmpleadoID, f.IncapacidadID, f.DiaSerieDesde, f.DiaSerieHasta, f.Dias, f.MontoPatronal, f.MontoEntidadInformativo})
	}
	return q.insertarLote(ctx, "planilla_detalle_incapacidad",
		[]string{"planilla_detalle_empleado_id", "incapacidad_id", "dia_serie_desde", "dia_serie_hasta", "dias", "monto_patronal", "monto_entidad_informativo"}, vals)
}

type DetallePrestamoInsert struct {
	DetalleEmpleadoID int64
	PrestamoID        int64
	CuotaCalculada    decimal.Decimal
	MontoRetenido     decimal.Decimal
}

func (q *Queries) InsertarDetallesPrestamo(ctx context.Context, filas []DetallePrestamoInsert) error {
	vals := make([][]any, 0, len(filas))
	for _, f := range filas {
		vals = append(vals, []any{f.DetalleEmpleadoID, f.PrestamoID, f.CuotaCalculada, f.MontoRetenido})
	}
	return q.insertarLote(ctx, "planilla_detalle_prestamo",
		[]string{"planilla_detalle_empleado_id", "prestamo_id", "cuota_calculada", "monto_retenido"}, vals)
}

// EliminarDetallePeriodo borra el detalle del período en orden de FK
// (hijos primero; en planilla_detalle_concepto, primero los ajustes que
// apuntan a otra línea). Los triggers de inmutabilidad rechazan el borrado
// si el período ya no está en borrador/calculada.
func (q *Queries) EliminarDetallePeriodo(ctx context.Context, periodoID int64) error {
	sentencias := []string{
		`DELETE h FROM planilla_detalle_incapacidad h JOIN planilla_detalle_empleado pde ON pde.id = h.planilla_detalle_empleado_id WHERE pde.periodo_planilla_id = ?`,
		`DELETE h FROM planilla_detalle_prestamo h JOIN planilla_detalle_empleado pde ON pde.id = h.planilla_detalle_empleado_id WHERE pde.periodo_planilla_id = ?`,
		`DELETE h FROM planilla_detalle_embargo h JOIN planilla_detalle_empleado pde ON pde.id = h.planilla_detalle_empleado_id WHERE pde.periodo_planilla_id = ?`,
		`DELETE h FROM planilla_detalle_patronal h JOIN planilla_detalle_empleado pde ON pde.id = h.planilla_detalle_empleado_id WHERE pde.periodo_planilla_id = ?`,
		`DELETE h FROM planilla_detalle_concepto h JOIN planilla_detalle_empleado pde ON pde.id = h.planilla_detalle_empleado_id WHERE pde.periodo_planilla_id = ? AND h.concepto_ajuste_de_id IS NOT NULL`,
		`DELETE h FROM planilla_detalle_concepto h JOIN planilla_detalle_empleado pde ON pde.id = h.planilla_detalle_empleado_id WHERE pde.periodo_planilla_id = ?`,
		`DELETE FROM planilla_detalle_empleado WHERE periodo_planilla_id = ?`,
	}
	for _, s := range sentencias {
		if _, err := q.db.ExecContext(ctx, s, periodoID); err != nil {
			return err
		}
	}
	return nil
}

// GuardarSnapshotPeriodo guarda con qué parámetros y qué versión del motor se
// calculó el período (trazabilidad legal).
func (q *Queries) GuardarSnapshotPeriodo(ctx context.Context, periodoID int64, snapshotJSON []byte, versionMotor string) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE periodos_planilla SET parametros_snapshot = ?, version_motor = ? WHERE id = ?`,
		string(snapshotJSON), versionMotor, periodoID)
	return err
}
