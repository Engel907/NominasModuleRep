package planilla

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
)

type fuenteFalsa struct {
	rentaMes     map[int64]dto.RentaMesRow
	asignaciones map[int64][]dto.AsignacionVigenteRow
	empleados    []int64
	contratos    map[int64]dto.ContratoVigenteEnFechaRow
	rangos       map[int64][]dto.ContratoEnRangoRow // si está, reemplaza a contratos
	laborales    map[int64]dto.ContextoLaboralRow
	horas        map[int64][]dto.HorasExtraPeriodoRow
	feriados     map[int64][]dto.HorasFeriadoLaboradoRow
	incaps       map[int64][]dto.IncapacidadSolapadaRow
	prestamos    map[int64][]dto.PrestamoActivoRow
	embargos     map[int64][]dto.EmbargoActivoRow
	familiares   map[int64]dto.DatosFamiliaresEnFechaRow
	creditos     []dto.CreditoFiscalVigenteRow
	devengos     map[int64][]dto.DevengoManualEmpleadoRow
	bases        map[string]decimal.Decimal // columna -> total
	comps        map[int64][]dto.CompensacionVacacionesRow
	vacs         map[int64][]dto.VacacionesAprobadasRow
	liqs         map[int64][]dto.LiquidacionParaPeriodoRow
	embargosMes  map[int64]dto.EmbargosMesRow
	promedios    map[string]dto.BasePromedioRow // columna -> Σ de la ventana
	compsPagadas map[int64][]dto.CompensacionVacacionesRow
}

func (f *fuenteFalsa) SumarBasePromedioEmpleado(_ context.Context, _ int64, _, _ time.Time, columna string) (dto.BasePromedioRow, error) {
	return f.promedios[columna], nil
}
func (f *fuenteFalsa) ListarCompensacionesVacacionesPagadasPorPeriodo(_ context.Context, id, _ int64) ([]dto.CompensacionVacacionesRow, error) {
	return f.compsPagadas[id], nil
}
func (f *fuenteFalsa) SumarEmbargosMesEmpleado(_ context.Context, id, _ int64, _, _ time.Time) (dto.EmbargosMesRow, error) {
	return f.embargosMes[id], nil
}

func (f *fuenteFalsa) ListarEmpleadosActivosEnRango(context.Context, time.Time, time.Time) ([]int64, error) {
	return f.empleados, nil
}

// ListarContratosEnRango: cada contrato falso es una versión abierta desde 2020.
func (f *fuenteFalsa) ListarContratosEnRango(_ context.Context, id int64, _, _ time.Time) ([]dto.ContratoEnRangoRow, error) {
	if r, ok := f.rangos[id]; ok {
		return r, nil
	}
	c, ok := f.contratos[id]
	if !ok {
		return nil, nil
	}
	return []dto.ContratoEnRangoRow{{ContratoID: c.ContratoID, SalarioPactado: c.SalarioPactado, ModalidadSalario: c.ModalidadSalario,
		FechaInicioVigencia: fecha("2020-01-01"), FechaInicioContrato: fecha("2020-01-01")}}, nil
}
func (f *fuenteFalsa) ObtenerContextoLaboral(_ context.Context, id int64, _ time.Time) (dto.ContextoLaboralRow, error) {
	l, ok := f.laborales[id]
	if !ok {
		return l, sql.ErrNoRows
	}
	return l, nil
}
func (f *fuenteFalsa) SumarHorasExtraPeriodo(_ context.Context, id int64, _, _ time.Time) ([]dto.HorasExtraPeriodoRow, error) {
	return f.horas[id], nil
}
func (f *fuenteFalsa) SumarHorasFeriadoLaboradoPeriodo(_ context.Context, id int64, _, _ time.Time) ([]dto.HorasFeriadoLaboradoRow, error) {
	return f.feriados[id], nil
}
func (f *fuenteFalsa) ListarIncapacidadesSolapadas(_ context.Context, id int64, _, _ time.Time) ([]dto.IncapacidadSolapadaRow, error) {
	return f.incaps[id], nil
}
func (f *fuenteFalsa) ListarPrestamosActivosEnFecha(_ context.Context, id int64, _ time.Time) ([]dto.PrestamoActivoRow, error) {
	return f.prestamos[id], nil
}
func (f *fuenteFalsa) ListarEmbargosActivosEnFecha(_ context.Context, id int64, _ time.Time) ([]dto.EmbargoActivoRow, error) {
	return f.embargos[id], nil
}
func (f *fuenteFalsa) ObtenerDatosFamiliaresEnFecha(_ context.Context, id int64, _ time.Time) (dto.DatosFamiliaresEnFechaRow, error) {
	r, ok := f.familiares[id]
	if !ok {
		return r, sql.ErrNoRows
	}
	return r, nil
}
func (f *fuenteFalsa) ListarCreditosFiscalesEnFecha(context.Context, time.Time) ([]dto.CreditoFiscalVigenteRow, error) {
	return f.creditos, nil
}
func (f *fuenteFalsa) ListarDevengosManualesEmpleado(_ context.Context, _, id int64) ([]dto.DevengoManualEmpleadoRow, error) {
	return f.devengos[id], nil
}

func (f *fuenteFalsa) ListarAsignacionesVigentesEnFecha(_ context.Context, id int64, _ time.Time) ([]dto.AsignacionVigenteRow, error) {
	return f.asignaciones[id], nil
}

func (f *fuenteFalsa) SumarRentaMesEmpleado(_ context.Context, id, _ int64, _, _ time.Time) (dto.RentaMesRow, error) {
	return f.rentaMes[id], nil
}

func (f *fuenteFalsa) SumarBaseAcumuladaEmpleado(_ context.Context, _ int64, _, _ time.Time, columna string) (decimal.Decimal, error) {
	return f.bases[columna], nil
}

func (f *fuenteFalsa) ListarCompensacionesVacacionesPendientes(_ context.Context, id, _ int64, _ time.Time) ([]dto.CompensacionVacacionesRow, error) {
	return f.comps[id], nil
}

func (f *fuenteFalsa) ListarVacacionesAprobadasEnRango(_ context.Context, id int64, _, _ time.Time) ([]dto.VacacionesAprobadasRow, error) {
	return f.vacs[id], nil
}

func (f *fuenteFalsa) ListarEmpleadosConLiquidacionParaPeriodo(context.Context, int64, time.Time) ([]int64, error) {
	var out []int64
	for id := range f.liqs {
		out = append(out, id)
	}
	return out, nil
}

func (f *fuenteFalsa) ListarLiquidacionesParaPeriodo(_ context.Context, _, id int64, _ time.Time) ([]dto.LiquidacionParaPeriodoRow, error) {
	return f.liqs[id], nil
}

func fecha(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func periodoPrueba() PeriodoParaCalculo {
	return PeriodoParaCalculo{
		ID: 50, Inicio: fecha("2026-03-01"), Fin: fecha("2026-03-31"),
		Clase: "regular", TipoPlanillaID: 1,
		FraccionMensual: d("1"), DiasPeriodoCompleto: d("30"), DivisorDiasMes: 30,
	}
}

// Tres empleados: 1 con préstamo y devengo manual, 2 con embargo y familia,
// 3 con horas extra e incapacidad que empezó el mes anterior.
func fuentePrueba() *fuenteFalsa {
	obs := "comisión marzo"
	clasif, sem := int64(4), int64(1)
	return &fuenteFalsa{
		empleados: []int64{1, 2, 3},
		contratos: map[int64]dto.ContratoVigenteEnFechaRow{
			1: {ContratoID: 101, EmpleadoID: 1, RegimenLaboralID: 1, SalarioPactado: d("800000")},
			2: {ContratoID: 102, EmpleadoID: 2, RegimenLaboralID: 1, SalarioPactado: d("900000")},
			3: {ContratoID: 103, EmpleadoID: 3, RegimenLaboralID: 1, SalarioPactado: d("700000")},
		},
		laborales: map[int64]dto.ContextoLaboralRow{
			1: {ProgramaPresupuestarioID: 11, ClasificacionPuestoID: &clasif, RegimenCCSSID: &sem},
			2: {ProgramaPresupuestarioID: 12, RegimenCCSSID: &sem},
			3: {ProgramaPresupuestarioID: 13, RegimenCCSSID: &sem},
		},
		horas: map[int64][]dto.HorasExtraPeriodoRow{3: {{TipoHoraExtraID: 1, Horas: d("6")}}},
		incaps: map[int64][]dto.IncapacidadSolapadaRow{
			3: {{IncapacidadID: 9, EntidadEmisoraID: 2, FechaInicio: fecha("2026-02-27"), FechaFin: fecha("2026-03-04")}},
		},
		prestamos: map[int64][]dto.PrestamoActivoRow{1: {{PrestamoID: 7, CuotaPeriodica: d("25000"), SaldoPendiente: d("300000")}}},
		embargos: map[int64][]dto.EmbargoActivoRow{
			2: {{EmbargoID: 5, TipoEmbargoID: 3, PrioridadLegal: 1, PrioridadAplicacion: 2, EsPension: true,
				TopePorcentajeLiquido: decimal.NewNullDecimal(d("50")), Porcentaje: decimal.NewNullDecimal(d("12.5")),
				TopeMensual: decimal.NewNullDecimal(d("90000"))}},
		},
		familiares: map[int64]dto.DatosFamiliaresEnFechaRow{2: {AplicaCreditoConyuge: true, NumeroHijosDependientes: 2}},
		creditos: []dto.CreditoFiscalVigenteRow{
			{TipoCreditoFiscalID: 21, TipoCodigo: "CONYUGE", MontoCredito: d("2000")},
			{TipoCreditoFiscalID: 22, TipoCodigo: "HIJO", MontoCredito: d("1500")},
		},
		devengos: map[int64][]dto.DevengoManualEmpleadoRow{
			1: {{ID: 1, ConceptoSalarialID: 30, Monto: d("45000"), Observacion: &obs}},
		},
	}
}

func TestHidratadorTresEmpleados(t *testing.T) {
	h := NewHidratador(fuentePrueba())
	p := periodoPrueba()
	ctx := context.Background()

	ids, err := h.EmpleadosDelPeriodo(ctx, p)
	if err != nil || len(ids) != 3 {
		t.Fatalf("EmpleadosDelPeriodo = %v, %v", ids, err)
	}

	e1, err := h.Hidratar(ctx, p, 1)
	if err != nil {
		t.Fatal(err)
	}
	if e1.ContratoID != 101 || e1.ProgramaPresupuestarioID != 11 || e1.Input.ClasificacionPuestoID != 4 || e1.Input.RegimenCCSSID != 1 {
		t.Errorf("empleado 1: contrato/programa/clasificación = %d/%d/%d", e1.ContratoID, e1.ProgramaPresupuestarioID, e1.Input.ClasificacionPuestoID)
	}
	if len(e1.Input.Segmentos) != 1 || !e1.Input.Segmentos[0].Salario.Equal(d("800000")) || e1.Input.Segmentos[0].Dias != 30 {
		t.Errorf("segmentos (salario del contrato): %+v", e1.Input.Segmentos)
	}
	if !e1.Input.FechaEfectiva.Equal(p.Fin) || !e1.Input.DiasTrabajados.Equal(d("30")) {
		t.Errorf("fecha efectiva/días = %s/%s", e1.Input.FechaEfectiva, e1.Input.DiasTrabajados)
	}
	if len(e1.Input.DevengosManuales) != 1 || e1.Input.DevengosManuales[0].ConceptoSalarialID != 30 ||
		!e1.Input.DevengosManuales[0].Monto.Equal(d("45000")) || e1.Input.DevengosManuales[0].Descripcion != "comisión marzo" {
		t.Errorf("devengo manual no llegó al input: %+v", e1.Input.DevengosManuales)
	}
	if len(e1.Input.Prestamos) != 1 || !e1.Input.Prestamos[0].CuotaPeriodica.Equal(d("25000")) {
		t.Errorf("préstamos: %+v", e1.Input.Prestamos)
	}
	if len(e1.Input.CreditosFiscales) != 0 {
		t.Errorf("sin declaración familiar no hay créditos: %+v", e1.Input.CreditosFiscales)
	}

	e2, err := h.Hidratar(ctx, p, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(e2.Input.Embargos) != 1 {
		t.Fatalf("embargos: %+v", e2.Input.Embargos)
	}
	if e := e2.Input.Embargos[0]; !e.Porcentaje.Equal(d("0.125")) || !e.TopePctLiquido.Equal(d("0.5")) || !e.TopeMensual.Equal(d("90000")) ||
		e.Prioridad != 2 || e.PrioridadLegal != 1 || !e.EsPension {
		t.Errorf("embargo: porcentajes en fracción (12.5 %% -> 0.125, 50 %% -> 0.5), tope, prioridades y pensión desde BD: %+v", e)
	}
	want := map[int64]int32{21: 1, 22: 2}
	if len(e2.Input.CreditosFiscales) != 2 {
		t.Fatalf("créditos: %+v", e2.Input.CreditosFiscales)
	}
	for _, c := range e2.Input.CreditosFiscales {
		if want[c.TipoCreditoFiscalID] != c.Cantidad {
			t.Errorf("crédito %d cantidad %d, esperado %d", c.TipoCreditoFiscalID, c.Cantidad, want[c.TipoCreditoFiscalID])
		}
	}

	e3, err := h.Hidratar(ctx, p, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(e3.Input.HorasExtra) != 1 || !e3.Input.HorasExtra[0].CantidadHoras.Equal(d("6")) {
		t.Errorf("horas extra: %+v", e3.Input.HorasExtra)
	}
	if len(e3.Input.Incapacidades) != 1 {
		t.Fatalf("incapacidades: %+v", e3.Input.Incapacidades)
	}
	inc := e3.Input.Incapacidades[0]
	if inc.DiaSerieDesde != 3 || !inc.Desde.Equal(fecha("2026-03-01")) || !inc.Hasta.Equal(fecha("2026-03-04")) || inc.EntidadEmisoraID != 2 || inc.IncapacidadID != 9 {
		t.Errorf("incapacidad = %+v, esperado día de serie 3, 1–4 mar, entidad 2", inc)
	}
	if !e3.Input.DiasTrabajados.Equal(d("26")) {
		t.Errorf("DiasTrabajados = %s, esperado 26", e3.Input.DiasTrabajados)
	}
}

func TestHidratadorEmpleadoSinContrato(t *testing.T) {
	f := fuentePrueba()
	delete(f.contratos, 2)
	_, err := NewHidratador(f).Hidratar(context.Background(), periodoPrueba(), 2)
	var e ErrEmpleadoSinContrato
	if !errors.As(err, &e) || e.EmpleadoID != 2 {
		t.Fatalf("se esperaba ErrEmpleadoSinContrato{2}, se obtuvo %v", err)
	}
}

func TestHidratadorErroresExplicitos(t *testing.T) {
	ctx := context.Background()

	f := fuentePrueba()
	delete(f.laborales, 1)
	var sinLab ErrEmpleadoSinDatosLaborales
	if _, err := NewHidratador(f).Hidratar(ctx, periodoPrueba(), 1); !errors.As(err, &sinLab) {
		t.Errorf("sin datos laborales: se esperaba ErrEmpleadoSinDatosLaborales, se obtuvo %v", err)
	}

	f = fuentePrueba()
	f.laborales[3] = dto.ContextoLaboralRow{ProgramaPresupuestarioID: 13}
	var sinReg ErrEmpleadoSinRegimenCCSS
	if _, err := NewHidratador(f).Hidratar(ctx, periodoPrueba(), 3); !errors.As(err, &sinReg) || sinReg.EmpleadoID != 3 {
		t.Errorf("sin régimen CCSS: se esperaba ErrEmpleadoSinRegimenCCSS{3}, se obtuvo %v", err)
	}

	f = fuentePrueba()
	f.creditos = f.creditos[:1]
	if _, err := NewHidratador(f).Hidratar(ctx, periodoPrueba(), 2); err == nil {
		t.Error("crédito declarado sin parámetro vigente debe dar error")
	}

	f = fuentePrueba()
	f.embargos[2] = []dto.EmbargoActivoRow{{EmbargoID: 6, PrioridadLegal: 2}}
	if _, err := NewHidratador(f).Hidratar(ctx, periodoPrueba(), 2); err == nil {
		t.Error("embargo sin porcentaje ni monto fijo debe dar error")
	}

	p := periodoPrueba()
	p.DiasPeriodoCompleto = decimal.Zero
	if _, err := NewHidratador(fuentePrueba()).Hidratar(ctx, p, 1); err == nil {
		t.Error("DiasPeriodoCompleto en cero debe dar error")
	}
}
