// Fase 9B — Corridas periódicas de aguinaldo y salario escolar para TODOS los
// empleados activos (distinto del cálculo proporcional por empleado de la
// liquidación).
//
// Ambas corridas son idempotentes: aguinaldos y salarios_escolares tienen
// UNIQUE (empleado_id, anio); un empleado que ya tiene registro se omite y
// se reporta, así que re-ejecutar tras una falla parcial es seguro.
package planilla

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
)

// mesesAguinaldo: el aguinaldo es 1/12 del total de salarios devengados en
// el período legal (Ley 2412). Es una fracción fijada por ley, no un monto
// vigente por fecha (misma excepción documentada en calculator.go).
const mesesAguinaldo = int64(12)

// CalcularAguinaldo (pura): total base / 12, a 2 decimales.
func CalcularAguinaldo(totalBase decimal.Decimal) (decimal.Decimal, error) {
	if totalBase.IsNegative() {
		return decimal.Zero, errors.New("planilla: base de aguinaldo negativa")
	}
	return totalBase.DivRound(decimal.NewFromInt(mesesAguinaldo), 6).Round(dosDecimales), nil
}

// CalcularSalarioEscolar (pura): total base * porcentaje vigente (fracción,
// ej. 0.0833). El porcentaje siempre llega como parámetro.
func CalcularSalarioEscolar(totalBase, porcentaje decimal.Decimal) (decimal.Decimal, error) {
	if totalBase.IsNegative() {
		return decimal.Zero, errors.New("planilla: base de salario escolar negativa")
	}
	if !porcentaje.IsPositive() || porcentaje.GreaterThan(decimal.NewFromInt(1)) {
		return decimal.Zero, fmt.Errorf("planilla: porcentaje de salario escolar inválido (%s); se espera una fracción en (0, 1]", porcentaje)
	}
	return totalBase.Mul(porcentaje).Round(dosDecimales), nil
}

// ---------------------------------------------------------------------------
// Repositorio
// ---------------------------------------------------------------------------

type BaseAcumulada struct {
	Total          decimal.Decimal
	PlanillasCount int64
}

type RegistroAcumulado struct {
	EmpleadoID   int64
	Anio         int32
	PeriodoDesde time.Time
	PeriodoHasta time.Time
	TotalBase    decimal.Decimal
	Monto        decimal.Decimal
	Porcentaje   decimal.Decimal // solo salario escolar
	ProcesadoPor int64
}

type AcumuladosRepository interface {
	ListarEmpleadosActivosEnRango(ctx context.Context, desde, hasta time.Time) ([]int64, error)
	SumarBaseAguinaldo(ctx context.Context, desde, hasta time.Time) (map[int64]BaseAcumulada, error)
	SumarBaseSalarioEscolar(ctx context.Context, desde, hasta time.Time) (map[int64]BaseAcumulada, error)
	EmpleadosConAguinaldo(ctx context.Context, anio int32) (map[int64]bool, error)
	EmpleadosConSalarioEscolar(ctx context.Context, anio int32) (map[int64]bool, error)
	PorcentajeSalarioEscolarEnFecha(ctx context.Context, fecha time.Time) (decimal.Decimal, error)
	GuardarAguinaldo(ctx context.Context, r RegistroAcumulado) error
	GuardarSalarioEscolar(ctx context.Context, r RegistroAcumulado) error
}

type acumuladosQueries interface {
	ListarEmpleadosActivosEnRango(ctx context.Context, desde, hasta time.Time) ([]int64, error)
	SumarBaseAguinaldoPorEmpleado(ctx context.Context, desde, hasta time.Time) ([]dto.BaseAcumuladaEmpleadoRow, error)
	SumarBaseSalarioEscolarPorEmpleado(ctx context.Context, desde, hasta time.Time) ([]dto.BaseAcumuladaEmpleadoRow, error)
	ListarEmpleadosConAguinaldoEnAnio(ctx context.Context, anio int32) (map[int64]bool, error)
	ListarEmpleadosConSalarioEscolarEnAnio(ctx context.Context, anio int32) (map[int64]bool, error)
	ObtenerPorcentajeSalarioEscolarEnFecha(ctx context.Context, fecha time.Time) (decimal.Decimal, error)
	CrearAguinaldo(ctx context.Context, arg dto.CrearAguinaldoParams) (int64, error)
	CrearSalarioEscolar(ctx context.Context, arg dto.CrearSalarioEscolarParams) (int64, error)
}

var _ acumuladosQueries = (*dto.Queries)(nil)

type acumuladosRepository struct{ q acumuladosQueries }

func NewAcumuladosRepository(q acumuladosQueries) AcumuladosRepository {
	return &acumuladosRepository{q: q}
}

func toMap(rows []dto.BaseAcumuladaEmpleadoRow) map[int64]BaseAcumulada {
	m := make(map[int64]BaseAcumulada, len(rows))
	for _, r := range rows {
		m[r.EmpleadoID] = BaseAcumulada{Total: r.TotalBase, PlanillasCount: r.PlanillasCount}
	}
	return m
}

func (r *acumuladosRepository) ListarEmpleadosActivosEnRango(ctx context.Context, d, h time.Time) ([]int64, error) {
	return r.q.ListarEmpleadosActivosEnRango(ctx, d, h)
}
func (r *acumuladosRepository) SumarBaseAguinaldo(ctx context.Context, d, h time.Time) (map[int64]BaseAcumulada, error) {
	rows, err := r.q.SumarBaseAguinaldoPorEmpleado(ctx, d, h)
	return toMap(rows), err
}
func (r *acumuladosRepository) SumarBaseSalarioEscolar(ctx context.Context, d, h time.Time) (map[int64]BaseAcumulada, error) {
	rows, err := r.q.SumarBaseSalarioEscolarPorEmpleado(ctx, d, h)
	return toMap(rows), err
}
func (r *acumuladosRepository) EmpleadosConAguinaldo(ctx context.Context, anio int32) (map[int64]bool, error) {
	return r.q.ListarEmpleadosConAguinaldoEnAnio(ctx, anio)
}
func (r *acumuladosRepository) EmpleadosConSalarioEscolar(ctx context.Context, anio int32) (map[int64]bool, error) {
	return r.q.ListarEmpleadosConSalarioEscolarEnAnio(ctx, anio)
}
func (r *acumuladosRepository) PorcentajeSalarioEscolarEnFecha(ctx context.Context, f time.Time) (decimal.Decimal, error) {
	return r.q.ObtenerPorcentajeSalarioEscolarEnFecha(ctx, f)
}
func (r *acumuladosRepository) GuardarAguinaldo(ctx context.Context, x RegistroAcumulado) error {
	_, err := r.q.CrearAguinaldo(ctx, dto.CrearAguinaldoParams{
		EmpleadoID: x.EmpleadoID, Anio: x.Anio, PeriodoDesde: x.PeriodoDesde, PeriodoHasta: x.PeriodoHasta,
		TotalSalariosBrutos: x.TotalBase, MontoAguinaldo: x.Monto, ProcesadoPor: x.ProcesadoPor,
	})
	return err
}
func (r *acumuladosRepository) GuardarSalarioEscolar(ctx context.Context, x RegistroAcumulado) error {
	_, err := r.q.CrearSalarioEscolar(ctx, dto.CrearSalarioEscolarParams{
		EmpleadoID: x.EmpleadoID, Anio: x.Anio, PeriodoDesde: x.PeriodoDesde, PeriodoHasta: x.PeriodoHasta,
		PorcentajeAplicado: x.Porcentaje, TotalSalariosBrutos: x.TotalBase, MontoSalarioEscolar: x.Monto,
		ProcesadoPor: x.ProcesadoPor,
	})
	return err
}

// ---------------------------------------------------------------------------
// Servicio
// ---------------------------------------------------------------------------

type EmpleadoOmitido struct {
	EmpleadoID int64
	Motivo     string
}

type ResultadoCorridaAcumulado struct {
	Tipo         string // "aguinaldo" | "salario_escolar"
	Anio         int32
	PeriodoDesde time.Time
	PeriodoHasta time.Time
	Procesados   []RegistroAcumulado
	Omitidos     []EmpleadoOmitido
	Errores      []EmpleadoErrorOutput
	TotalMonto   decimal.Decimal
	Advertencias []string
}

var ErrFueraDeVentanaLegal = errors.New("planilla: fecha de ejecución fuera de la ventana legal de la corrida")

type AcumuladosService struct{ repo AcumuladosRepository }

func NewAcumuladosService(repo AcumuladosRepository) *AcumuladosService {
	return &AcumuladosService{repo: repo}
}

func fechaUTC(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// EjecutarAguinaldo: período legal 1-dic (anio-1) a 30-nov (anio). Ventana
// de ejecución: diciembre de `anio` (el plazo legal de pago vence el 20 de
// diciembre; ejecutar después NO se bloquea pero deja advertencia).
// forzarFueraDeVentana permite corridas retroactivas/correctivas.
func (s *AcumuladosService) EjecutarAguinaldo(ctx context.Context, anio int32, fechaEjecucion time.Time, usuarioID int64, forzarFueraDeVentana bool) (*ResultadoCorridaAcumulado, error) {
	y := int(anio)
	desde, hasta := fechaUTC(y-1, time.December, 1), fechaUTC(y, time.November, 30)
	res := &ResultadoCorridaAcumulado{Tipo: "aguinaldo", Anio: anio, PeriodoDesde: desde, PeriodoHasta: hasta}

	f := soloFecha(fechaEjecucion)
	if f.Before(fechaUTC(y, time.December, 1)) || f.After(fechaUTC(y, time.December, 31)) {
		if !forzarFueraDeVentana {
			return nil, fmt.Errorf("%w: el aguinaldo %d se ejecuta en diciembre de %d (recibido %s)", ErrFueraDeVentanaLegal, anio, anio, f.Format("2006-01-02"))
		}
		res.Advertencias = append(res.Advertencias, "corrida forzada fuera de diciembre")
	} else if f.After(fechaUTC(y, time.December, 20)) {
		res.Advertencias = append(res.Advertencias, "ejecutado después del 20 de diciembre (plazo legal de pago)")
	}

	empleados, err := s.repo.ListarEmpleadosActivosEnRango(ctx, desde, hasta)
	if err != nil {
		return nil, fmt.Errorf("planilla: listando empleados activos: %w", err)
	}
	bases, err := s.repo.SumarBaseAguinaldo(ctx, desde, hasta)
	if err != nil {
		return nil, fmt.Errorf("planilla: sumando base de aguinaldo: %w", err)
	}
	ya, err := s.repo.EmpleadosConAguinaldo(ctx, anio)
	if err != nil {
		return nil, fmt.Errorf("planilla: leyendo aguinaldos existentes: %w", err)
	}

	for _, emp := range empleados {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if ya[emp] {
			res.Omitidos = append(res.Omitidos, EmpleadoOmitido{emp, "ya tiene aguinaldo del año"})
			continue
		}
		b, ok := bases[emp]
		if !ok || !b.Total.IsPositive() {
			res.Omitidos = append(res.Omitidos, EmpleadoOmitido{emp, "sin salarios calculados en el período legal"})
			continue
		}
		monto, err := CalcularAguinaldo(b.Total)
		if err != nil {
			res.Errores = append(res.Errores, EmpleadoErrorOutput{emp, err.Error()})
			continue
		}
		reg := RegistroAcumulado{EmpleadoID: emp, Anio: anio, PeriodoDesde: desde, PeriodoHasta: hasta, TotalBase: b.Total, Monto: monto, ProcesadoPor: usuarioID}
		if err := s.repo.GuardarAguinaldo(ctx, reg); err != nil {
			res.Errores = append(res.Errores, EmpleadoErrorOutput{emp, "al guardar: " + err.Error()})
			continue
		}
		res.Procesados = append(res.Procesados, reg)
		res.TotalMonto = res.TotalMonto.Add(monto)
	}
	return res, nil
}

// EjecutarSalarioEscolar: período 1-ene a 31-dic de `anio` (como documenta
// salarios_escolares.anio). Ventana de ejecución: enero de anio+1, cuando el
// período ya está completo. El porcentaje es el vigente a la fecha de
// ejecución y se copia a cada registro (porcentaje_aplicado).
func (s *AcumuladosService) EjecutarSalarioEscolar(ctx context.Context, anio int32, fechaEjecucion time.Time, usuarioID int64, forzarFueraDeVentana bool) (*ResultadoCorridaAcumulado, error) {
	y := int(anio)
	desde, hasta := fechaUTC(y, time.January, 1), fechaUTC(y, time.December, 31)
	res := &ResultadoCorridaAcumulado{Tipo: "salario_escolar", Anio: anio, PeriodoDesde: desde, PeriodoHasta: hasta}

	f := soloFecha(fechaEjecucion)
	if f.Before(fechaUTC(y+1, time.January, 1)) || f.After(fechaUTC(y+1, time.January, 31)) {
		if !forzarFueraDeVentana {
			return nil, fmt.Errorf("%w: el salario escolar %d se ejecuta en enero de %d (recibido %s)", ErrFueraDeVentanaLegal, anio, y+1, f.Format("2006-01-02"))
		}
		res.Advertencias = append(res.Advertencias, "corrida forzada fuera de enero")
	}

	pct, err := s.repo.PorcentajeSalarioEscolarEnFecha(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("planilla: no hay porcentaje de salario escolar vigente al %s: %w", f.Format("2006-01-02"), err)
	}
	if !pct.IsPositive() {
		return nil, fmt.Errorf("planilla: porcentaje de salario escolar inválido (%s)", pct)
	}

	empleados, err := s.repo.ListarEmpleadosActivosEnRango(ctx, desde, hasta)
	if err != nil {
		return nil, fmt.Errorf("planilla: listando empleados activos: %w", err)
	}
	bases, err := s.repo.SumarBaseSalarioEscolar(ctx, desde, hasta)
	if err != nil {
		return nil, fmt.Errorf("planilla: sumando base de salario escolar: %w", err)
	}
	ya, err := s.repo.EmpleadosConSalarioEscolar(ctx, anio)
	if err != nil {
		return nil, fmt.Errorf("planilla: leyendo salarios escolares existentes: %w", err)
	}

	for _, emp := range empleados {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if ya[emp] {
			res.Omitidos = append(res.Omitidos, EmpleadoOmitido{emp, "ya tiene salario escolar del año"})
			continue
		}
		b, ok := bases[emp]
		if !ok || !b.Total.IsPositive() {
			res.Omitidos = append(res.Omitidos, EmpleadoOmitido{emp, "sin salarios calculados en el período"})
			continue
		}
		monto, err := CalcularSalarioEscolar(b.Total, pct)
		if err != nil {
			res.Errores = append(res.Errores, EmpleadoErrorOutput{emp, err.Error()})
			continue
		}
		reg := RegistroAcumulado{EmpleadoID: emp, Anio: anio, PeriodoDesde: desde, PeriodoHasta: hasta, TotalBase: b.Total, Monto: monto, Porcentaje: pct, ProcesadoPor: usuarioID}
		if err := s.repo.GuardarSalarioEscolar(ctx, reg); err != nil {
			res.Errores = append(res.Errores, EmpleadoErrorOutput{emp, "al guardar: " + err.Error()})
			continue
		}
		res.Procesados = append(res.Procesados, reg)
		res.TotalMonto = res.TotalMonto.Add(monto)
	}
	return res, nil
}