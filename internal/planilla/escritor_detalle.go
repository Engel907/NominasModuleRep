package planilla

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
)

// LineaPatronal: cuota patronal persistida. Porcentaje en FRACCIÓN.
type LineaPatronal struct {
	ConceptoSalarialID int64
	Base               decimal.Decimal
	Porcentaje         decimal.Decimal
	Monto              decimal.Decimal
}

// LineaPrestamo: cuota de un préstamo en el período. MontoRetenido <=
// CuotaCalculada (puede reducirse por el tope de neto).
type LineaPrestamo struct {
	PrestamoID     int64
	CuotaCalculada decimal.Decimal
	MontoRetenido  decimal.Decimal
}

// LineaIncapacidad: tramo de subsidio de una incapacidad dentro del período.
// MontoEntidadInformativo no se paga por planilla.
type LineaIncapacidad struct {
	IncapacidadID           int64
	DiaSerieDesde           int32
	DiaSerieHasta           int32
	Dias                    decimal.Decimal
	MontoPatronal           decimal.Decimal
	MontoEntidadInformativo decimal.Decimal
}

// ResultadoPersistible: lo que se guarda de un empleado.
type ResultadoPersistible struct {
	Detalle                  *CalculoEmpleadoDetalle
	ContratoID               int64
	ProgramaPresupuestarioID int64
	Patronal                 []LineaPatronal
	Incapacidad              []LineaIncapacidad
	Prestamos                []LineaPrestamo
	Acumulado                *AcumuladoPersistible // aguinaldo / salario escolar
	VacacionesCompensadas    []int64               // movimientos 'compensacion' pagados por el período
	LiquidacionID            int64                 // liquidación que paga el período (0 = ninguna)
}

// AcumuladoPersistible: fila del año en aguinaldos o salarios_escolares que
// deja el cálculo de un período de esa clase (estado 'proyectado').
type AcumuladoPersistible struct {
	Clase        ClaseTipoPlanilla
	Anio         int32
	Desde, Hasta time.Time
	TotalBase    decimal.Decimal
	Monto        decimal.Decimal
	Porcentaje   decimal.Decimal // solo salario escolar
}

// ErrDetalleNoCuadra: el resultado en memoria no cuadra (Σ líneas ≠ totales);
// se detecta antes de insertar. Es un error del motor, no del usuario.
type ErrDetalleNoCuadra struct {
	EmpleadoID int64
	Motivo     string
}

func (e ErrDetalleNoCuadra) Error() string {
	return fmt.Sprintf("planilla: detalle del empleado_id=%d no cuadra: %s", e.EmpleadoID, e.Motivo)
}

// EscritorDetalle implementa EliminadorDetalle.
type EscritorDetalle struct{}

var _ EliminadorDetalle = (*EscritorDetalle)(nil)

// Escribir valida en memoria y luego inserta todo el detalle del período.
func (e *EscritorDetalle) Escribir(ctx context.Context, q *dto.Queries, periodoID int64, rs []ResultadoPersistible) error {
	if len(rs) == 0 {
		return errors.New("planilla: no hay empleados que persistir en el período")
	}
	empleados := make([]dto.DetalleEmpleadoInsert, 0, len(rs))
	vistos := make(map[int64]bool, len(rs))
	for _, r := range rs {
		if r.Detalle == nil {
			return errors.New("planilla: resultado sin detalle")
		}
		d := r.Detalle
		if vistos[d.EmpleadoID] {
			return fmt.Errorf("planilla: empleado_id=%d repetido en el período", d.EmpleadoID)
		}
		vistos[d.EmpleadoID] = true
		if err := verificarCuadre(r); err != nil {
			return err
		}
		var adv []byte
		if len(d.Advertencias) > 0 {
			var err error
			if adv, err = json.Marshal(d.Advertencias); err != nil {
				return fmt.Errorf("planilla: advertencias del empleado_id=%d: %w", d.EmpleadoID, err)
			}
		}
		empleados = append(empleados, dto.DetalleEmpleadoInsert{
			EmpleadoID:                d.EmpleadoID,
			ProgramaPresupuestarioID:  r.ProgramaPresupuestarioID,
			ContratoID:                r.ContratoID,
			SalarioBasePeriodo:        d.SalarioBasePeriodo,
			TotalIngresos:             d.TotalIngresos,
			TotalDeducciones:          d.TotalDeducciones,
			NetoPagado:                d.NetoPagado,
			BaseCCSS:                  d.BaseCCSS,
			BaseRenta:                 d.BaseRenta,
			BaseAguinaldo:             d.BaseAguinaldo,
			BaseCesantia:              d.BaseCesantia,
			BaseVacaciones:            d.BaseVacaciones,
			BaseEmbargo:               d.BaseEmbargo,
			CuotaObreraCCSS:           d.CuotaObreroCCSS,
			CuotaPatronalCCSS:         d.CuotaPatronalCCSS,
			RentaBruta:                d.ImpuestoRentaBruto,
			CreditosFiscalesAplicados: d.CreditosFiscalesAplicados,
			RentaNeta:                 d.ImpuestoRentaNeto,
			Advertencias:              adv,
		})
	}

	ids, err := q.InsertarDetallesEmpleado(ctx, periodoID, empleados)
	if err != nil {
		return fmt.Errorf("planilla: insertando planilla_detalle_empleado: %w", err)
	}

	var (
		conceptos []dto.DetalleConceptoInsert
		patronal  []dto.DetallePatronalInsert
		embargos  []dto.DetalleEmbargoInsert
		incaps    []dto.DetalleIncapacidadInsert
		prestamos []dto.DetallePrestamoInsert
	)
	for _, r := range rs {
		d := r.Detalle
		pdeID, ok := ids[d.EmpleadoID]
		if !ok {
			return fmt.Errorf("planilla: no se obtuvo el id de detalle del empleado_id=%d", d.EmpleadoID)
		}
		for _, l := range d.DesglosePorConcepto {
			c := dto.DetalleConceptoInsert{DetalleEmpleadoID: pdeID, ConceptoSalarialID: l.ConceptoSalarialID, Monto: l.Monto}
			if r := l.Retroactivo; r != nil {
				origen, motivo := r.PeriodoOrigenID, r.Motivo
				c.RetroactivoPeriodoOrigenID, c.MotivoAjuste = &origen, &motivo
				if r.LineaOrigenID != 0 {
					linea := r.LineaOrigenID
					c.AjusteDeID = &linea
				}
				conceptos = append(conceptos, c)
				continue
			}
			if f, ok := d.FormulaPorConcepto[l.ConceptoSalarialID]; ok && f != 0 {
				c.FormulaID = &f
			}
			if f, ok := d.FormulaEmpleadoPorConcepto[l.ConceptoSalarialID]; ok && f != 0 {
				c.FormulaEmpleadoID = &f
			}
			if !l.Cantidad.IsZero() {
				cant := l.Cantidad
				c.Cantidad = &cant
			}
			conceptos = append(conceptos, c)
		}
		for _, p := range r.Patronal {
			patronal = append(patronal, dto.DetallePatronalInsert{
				DetalleEmpleadoID: pdeID, ConceptoSalarialID: p.ConceptoSalarialID,
				Base: p.Base, Porcentaje: p.Porcentaje, Monto: p.Monto,
			})
		}
		for _, x := range d.EmbargosAplicados {
			embargos = append(embargos, dto.DetalleEmbargoInsert{
				DetalleEmpleadoID: pdeID, EmbargoID: x.EmbargoID, OrdenPrioridad: x.Prioridad,
				MontoAplicado: x.MontoAplicado, MontoCalculado: x.MontoCalculado, MontoInsatisfecho: x.SaldoPendienteInsatisfecho,
			})
		}
		for _, x := range r.Incapacidad {
			incaps = append(incaps, dto.DetalleIncapacidadInsert{
				DetalleEmpleadoID: pdeID, IncapacidadID: x.IncapacidadID,
				DiaSerieDesde: x.DiaSerieDesde, DiaSerieHasta: x.DiaSerieHasta, Dias: x.Dias,
				MontoPatronal: x.MontoPatronal, MontoEntidadInformativo: x.MontoEntidadInformativo,
			})
		}
		for _, x := range r.Prestamos {
			prestamos = append(prestamos, dto.DetallePrestamoInsert{
				DetalleEmpleadoID: pdeID, PrestamoID: x.PrestamoID,
				CuotaCalculada: x.CuotaCalculada, MontoRetenido: x.MontoRetenido,
			})
		}
	}

	pasos := []struct {
		tabla string
		fn    func() error
	}{
		{"planilla_detalle_concepto", func() error { return q.InsertarDetallesConcepto(ctx, conceptos) }},
		{"planilla_detalle_patronal", func() error { return q.InsertarDetallesPatronal(ctx, patronal) }},
		{"planilla_detalle_embargo", func() error { return q.InsertarDetallesEmbargo(ctx, embargos) }},
		{"planilla_detalle_incapacidad", func() error { return q.InsertarDetallesIncapacidad(ctx, incaps) }},
		{"planilla_detalle_prestamo", func() error { return q.InsertarDetallesPrestamo(ctx, prestamos) }},
	}
	for _, p := range pasos {
		if err := p.fn(); err != nil {
			return fmt.Errorf("planilla: insertando %s: %w", p.tabla, err)
		}
	}
	if err := e.escribirAcumulados(ctx, q, periodoID, rs); err != nil {
		return err
	}
	for _, r := range rs {
		if r.LiquidacionID == 0 {
			continue
		}
		err := q.LigarLiquidacionPeriodo(ctx, r.LiquidacionID, periodoID)
		if errors.Is(err, dto.ErrLiquidacionNoDisponible) {
			return &ErrorCiclo{Codigo: "liquidacion_no_disponible", HTTP: 409,
				Mensaje: fmt.Sprintf("la liquidación %d del empleado %d ya la paga otro período", r.LiquidacionID, r.Detalle.EmpleadoID)}
		}
		if err != nil {
			return fmt.Errorf("planilla: ligando liquidación %d: %w", r.LiquidacionID, err)
		}
	}
	for _, r := range rs {
		for _, mov := range r.VacacionesCompensadas {
			err := q.LigarCompensacionVacaciones(ctx, mov, periodoID)
			if errors.Is(err, dto.ErrCompensacionNoDisponible) {
				return &ErrorCiclo{Codigo: "compensacion_no_disponible", HTTP: 409,
					Mensaje: fmt.Sprintf("la compensación de vacaciones %d del empleado %d ya fue pagada o la tomó otro período", mov, r.Detalle.EmpleadoID)}
			}
			if err != nil {
				return fmt.Errorf("planilla: ligando compensación de vacaciones %d: %w", mov, err)
			}
		}
	}
	return nil
}

// escribirAcumulados: upsert 'proyectado' en aguinaldos / salarios_escolares
// (UNIQUE empleado+año). Una fila ya 'procesado' por otro período → 409.
func (e *EscritorDetalle) escribirAcumulados(ctx context.Context, q *dto.Queries, periodoID int64, rs []ResultadoPersistible) error {
	for _, r := range rs {
		a := r.Acumulado
		if a == nil {
			continue
		}
		fila := dto.AcumuladoProyectado{EmpleadoID: r.Detalle.EmpleadoID, Anio: a.Anio, PeriodoID: periodoID,
			Desde: a.Desde, Hasta: a.Hasta, TotalBase: a.TotalBase, Monto: a.Monto, Porcentaje: a.Porcentaje}
		var err error
		switch a.Clase {
		case ClaseAguinaldo:
			err = q.GuardarAguinaldoProyectado(ctx, fila)
		case ClaseSalarioEscolar:
			err = q.GuardarSalarioEscolarProyectado(ctx, fila)
		default:
			return fmt.Errorf("planilla: acumulado de clase %q no persistible", a.Clase)
		}
		if errors.Is(err, dto.ErrAcumuladoYaProcesado) {
			return &ErrorCiclo{Codigo: ErrAcumuladoYaProcesado.Codigo, HTTP: ErrAcumuladoYaProcesado.HTTP,
				Mensaje: fmt.Sprintf("el empleado %d ya tiene el %s %d procesado por otro período", r.Detalle.EmpleadoID, a.Clase, a.Anio)}
		}
		if err != nil {
			return fmt.Errorf("planilla: guardando %s del empleado_id=%d: %w", a.Clase, r.Detalle.EmpleadoID, err)
		}
	}
	return nil
}

// verificarCuadre replica en memoria fn_detalle_cuadra y los CHECK de
// las tablas hijas, para fallar con un mensaje claro antes de insertar.
func verificarCuadre(r ResultadoPersistible) error {
	d := r.Detalle
	mal := func(f string, a ...any) error {
		return ErrDetalleNoCuadra{EmpleadoID: d.EmpleadoID, Motivo: fmt.Sprintf(f, a...)}
	}
	if r.ContratoID <= 0 || r.ProgramaPresupuestarioID <= 0 {
		return mal("falta contrato o programa presupuestario")
	}
	ing, ded := decimal.Zero, decimal.Zero
	for _, l := range d.DesglosePorConcepto {
		switch l.Naturaleza {
		case naturalezaIngreso:
			ing = ing.Add(l.Monto)
		case naturalezaDeduccion:
			ded = ded.Add(l.Monto)
		default:
			return mal("línea %q con naturaleza desconocida %q", l.Codigo, l.Naturaleza)
		}
	}
	if !ing.Equal(d.TotalIngresos) {
		return mal("Σ ingresos %s ≠ total_ingresos %s", ing, d.TotalIngresos)
	}
	if !ded.Equal(d.TotalDeducciones) {
		return mal("Σ deducciones %s ≠ total_deducciones %s", ded, d.TotalDeducciones)
	}
	if d.NetoPagado.IsNegative() || !d.NetoPagado.Equal(d.TotalIngresos.Sub(d.TotalDeducciones)) {
		return mal("neto %s inválido para ingresos %s y deducciones %s", d.NetoPagado, d.TotalIngresos, d.TotalDeducciones)
	}
	pat := decimal.Zero
	for _, p := range r.Patronal {
		pat = pat.Add(p.Monto)
	}
	if !pat.Equal(d.CuotaPatronalCCSS) {
		return mal("Σ cuota patronal %s ≠ cuota_patronal_ccss %s (¿falta el concepto patronal en el régimen?)", pat, d.CuotaPatronalCCSS)
	}
	for _, x := range d.EmbargosAplicados {
		if x.MontoAplicado.IsNegative() || x.MontoCalculado.LessThan(x.MontoAplicado) ||
			!x.SaldoPendienteInsatisfecho.Equal(x.MontoCalculado.Sub(x.MontoAplicado)) {
			return mal("embargo_id=%d con montos inconsistentes", x.EmbargoID)
		}
	}
	for _, x := range r.Prestamos {
		if x.MontoRetenido.IsNegative() || x.MontoRetenido.GreaterThan(x.CuotaCalculada) {
			return mal("préstamo_id=%d retiene %s de una cuota de %s", x.PrestamoID, x.MontoRetenido, x.CuotaCalculada)
		}
	}
	return nil
}

// EliminarDetalle borra el detalle de las 6 tablas del período (
// descartar / recalcular).
func (e *EscritorDetalle) EliminarDetalle(ctx context.Context, q *dto.Queries, periodoID int64) error {
	if err := q.EliminarDetallePeriodo(ctx, periodoID); err != nil {
		return fmt.Errorf("planilla: eliminando detalle del período %d: %w", periodoID, err)
	}
	if err := q.EliminarAcumuladosProyectadosPeriodo(ctx, periodoID); err != nil {
		return fmt.Errorf("planilla: eliminando acumulados proyectados del período %d: %w", periodoID, err)
	}
	if err := q.DesligarCompensacionesVacacionesPeriodo(ctx, periodoID); err != nil {
		return fmt.Errorf("planilla: desligando compensaciones de vacaciones del período %d: %w", periodoID, err)
	}
	if err := q.DesligarLiquidacionesPeriodo(ctx, periodoID); err != nil {
		return fmt.Errorf("planilla: desligando liquidaciones del período %d: %w", periodoID, err)
	}
	return nil
}

// GuardarSnapshot registra los parámetros usados y la versión del motor.
func (e *EscritorDetalle) GuardarSnapshot(ctx context.Context, q *dto.Queries, periodoID int64, snapshotJSON []byte, versionMotor string) error {
	if !json.Valid(snapshotJSON) {
		return errors.New("planilla: snapshot de parámetros no es JSON válido")
	}
	if err := q.GuardarSnapshotPeriodo(ctx, periodoID, snapshotJSON, versionMotor); err != nil {
		return fmt.Errorf("planilla: guardando snapshot del período %d: %w", periodoID, err)
	}
	return nil
}
