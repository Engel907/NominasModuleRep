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
	empleados  []int64
	contratos  map[int64]dto.ContratoVigenteEnFechaRow
	laborales  map[int64]dto.ContextoLaboralRow
	horas      map[int64][]dto.HorasExtraPeriodoRow
	incaps     map[int64][]dto.IncapacidadSolapadaRow
	prestamos  map[int64][]dto.PrestamoActivoRow
	embargos   map[int64][]dto.EmbargoActivoRow
	familiares map[int64]dto.DatosFamiliaresEnFechaRow
	creditos   []dto.CreditoFiscalVigenteRow
	devengos   map[int64][]dto.DevengoManualEmpleadoRow
}

func (f *fuenteFalsa) ListarEmpleadosActivosEnRango(context.Context, time.Time, time.Time) ([]int64, error) {
	return f.empleados, nil
}
func (f *fuenteFalsa) ObtenerContratoVigenteEnFecha(_ context.Context, id int64, _ time.Time) (dto.ContratoVigenteEnFechaRow, error) {
	c, ok := f.contratos[id]
	if !ok {
		return c, sql.ErrNoRows
	}
	return c, nil
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
		FraccionMensual: d("1"), DiasPeriodoCompleto: d("30"),
	}
}

// Tres empleados: 1 con préstamo y devengo manual, 2 con embargo y familia,
// 3 con horas extra e incapacidad que empezó el mes anterior.
func fuentePrueba() *fuenteFalsa {
	obs := "comisión marzo"
	clasif := int64(4)
	return &fuenteFalsa{
		empleados: []int64{1, 2, 3},
		contratos: map[int64]dto.ContratoVigenteEnFechaRow{
			1: {ContratoID: 101, EmpleadoID: 1, RegimenLaboralID: 1, SalarioPactado: d("800000")},
			2: {ContratoID: 102, EmpleadoID: 2, RegimenLaboralID: 1, SalarioPactado: d("900000")},
			3: {ContratoID: 103, EmpleadoID: 3, RegimenLaboralID: 1, SalarioPactado: d("700000")},
		},
		laborales: map[int64]dto.ContextoLaboralRow{
			1: {ProgramaPresupuestarioID: 11, ClasificacionPuestoID: &clasif},
			2: {ProgramaPresupuestarioID: 12},
			3: {ProgramaPresupuestarioID: 13},
		},
		horas: map[int64][]dto.HorasExtraPeriodoRow{3: {{TipoHoraExtraID: 1, Horas: d("6")}}},
		incaps: map[int64][]dto.IncapacidadSolapadaRow{
			3: {{IncapacidadID: 9, EntidadEmisoraID: 2, FechaInicio: fecha("2026-02-27"), FechaFin: fecha("2026-03-04")}},
		},
		prestamos: map[int64][]dto.PrestamoActivoRow{1: {{PrestamoID: 7, CuotaPeriodica: d("25000"), SaldoPendiente: d("300000")}}},
		embargos: map[int64][]dto.EmbargoActivoRow{
			2: {{EmbargoID: 5, TipoEmbargoID: 3, PrioridadLegal: 1, Porcentaje: decimal.NewNullDecimal(d("12.5"))}},
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
	if e1.ContratoID != 101 || e1.ProgramaPresupuestarioID != 11 || e1.Input.ClasificacionPuestoID != 4 {
		t.Errorf("empleado 1: contrato/programa/clasificación = %d/%d/%d", e1.ContratoID, e1.ProgramaPresupuestarioID, e1.Input.ClasificacionPuestoID)
	}
	if !e1.Input.SalarioBaseNominal.IsZero() {
		t.Error("SalarioBaseNominal no debe llenarse (D5)")
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
	if len(e2.Input.Embargos) != 1 || !e2.Input.Embargos[0].Porcentaje.Equal(d("0.125")) || e2.Input.Embargos[0].Prioridad != 1 {
		t.Errorf("embargo debe llegar en fracción (12.5 %% -> 0.125): %+v", e2.Input.Embargos)
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
	// Boleta 27-feb a 4-mar: dentro de marzo son 4 días (1..4) y el 1-mar es
	// el día 3 de la incapacidad (27 y 28 de febrero fueron los días 1 y 2).
	if len(e3.Input.Incapacidades) != 1 {
		t.Fatalf("incapacidades: %+v", e3.Input.Incapacidades)
	}
	inc := e3.Input.Incapacidades[0]
	if inc.DiaDesde != 3 || !inc.DiasIncapacidad.Equal(d("4")) || inc.EntidadEmisoraID != 2 {
		t.Errorf("incapacidad = %+v, esperado DiaDesde 3, 4 días, entidad 2", inc)
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
	f.creditos = f.creditos[:1] // falta HIJO
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
