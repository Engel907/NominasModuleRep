package planilla

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type fakeMotorRepo struct {
	contrato  ContratoVigente
	conceptos []ConceptoAplicableFila
	factores  []ParametroHoraExtra
}

func (f fakeMotorRepo) ObtenerContratoVigenteEnFecha(context.Context, int64, time.Time) (ContratoVigente, error) {
	return f.contrato, nil
}
func (f fakeMotorRepo) ListarConceptosAplicablesEnFecha(context.Context, int64, time.Time) ([]ConceptoAplicableFila, error) {
	return f.conceptos, nil
}
func (f fakeMotorRepo) ListarParametrosHorasExtraEnFecha(context.Context, time.Time) ([]ParametroHoraExtra, error) {
	return f.factores, nil
}

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func sp(s string) *string        { return &s }
func ip(i int64) *int64          { return &i }

func fila(id int64, cod, nat string, orden int32, expr string, ccss, renta bool) ConceptoAplicableFila {
	return ConceptoAplicableFila{
		ConceptoOrdenCalculo: ConceptoOrdenCalculo{ConceptoSalarialID: id, Codigo: cod, OrdenCalculo: orden, AfectaBaseCCSS: ccss, AfectaBaseRenta: renta},
		Naturaleza:           nat, TipoCalculo: "formula", FormulaID: ip(id * 100), ExpresionFormula: sp(expr),
	}
}

func repoBase() fakeMotorRepo {
	return fakeMotorRepo{
		contrato: ContratoVigente{ContratoID: 1, EmpleadoID: 7, RegimenLaboralID: 1, SalarioPactado: d("1000000")},
		conceptos: []ConceptoAplicableFila{ // desordenados a propósito
			fila(3, "IMP_RENTA", "deduccion", 110, `{"tipo":"renta"}`, false, false),
			fila(1, "SAL_BASE", "ingreso", 10, `{"tipo":"referencia_directa"}`, true, true),
			fila(4, "CCSS_OBR", "deduccion", 100, `{"tipo":"carga_social","lado":"obrero"}`, false, false),
			fila(2, "HRS_EXTRA", "ingreso", 20, `{"tipo":"horas_extra"}`, true, true),
			fila(5, "CCSS_PAT", "deduccion", 160, `{"tipo":"carga_social","lado":"patronal"}`, false, false),
		},
		factores: []ParametroHoraExtra{{TipoHoraExtraID: 1, FactorRecargo: d("1.5")}},
	}
}

func params() ParametrosCalculo {
	return ParametrosCalculo{
		CargaSocial: ParametroCargaSocial{PorcentajeObrero: d("0.1083"), PorcentajePatronal: d("0.2683")},
		TramosRenta: []TramoRenta{
			{OrdenTramo: 1, MontoDesde: d("0"), MontoHasta: d("918000"), Porcentaje: d("0")},
			{OrdenTramo: 2, MontoDesde: d("918000"), MontoHasta: d("1347000"), Porcentaje: d("0.10")},
			{OrdenTramo: 3, MontoDesde: d("1347000"), Porcentaje: d("0.15"), Abierto: true},
		},
		HorasMensualesOrdinarias: d("240"), FraccionMensualPeriodo: d("1"), DiasPeriodoCompleto: d("30"),
	}
}

func TestCalcularHorasExtra(t *testing.T) {
	f := map[int64]ParametroHoraExtra{1: {1, d("1.5")}, 4: {4, d("2")}}
	m, h, err := CalcularHorasExtra(d("1000000"), d("240"),
		[]HoraExtraInput{{1, d("10")}, {4, d("6")}}, f)
	// 1.000.000/240 * (10*1.5 + 6*2) = 4166.666.. * 27 = 112500
	if err != nil || !m.Equal(d("112500")) || !h.Equal(d("16")) {
		t.Fatalf("monto=%s horas=%s err=%v", m, h, err)
	}
	if _, _, err := CalcularHorasExtra(d("1000000"), d("240"), []HoraExtraInput{{9, d("1")}}, f); err == nil {
		t.Fatal("esperaba error por factor faltante")
	}
	if _, _, err := CalcularHorasExtra(d("1000000"), decimal.Zero, []HoraExtraInput{{1, d("1")}}, f); err == nil {
		t.Fatal("esperaba error por divisor cero")
	}
}

func TestCalcularEmpleado_FlujoCompleto(t *testing.T) {
	m := NewMotorEmpleado(repoBase())
	in := CalculoEmpleadoInput{
		EmpleadoID: 7, PeriodoID: 1, FechaEfectiva: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("30"), HorasExtra: []HoraExtraInput{{1, d("10")}},
	}
	r, err := m.CalcularEmpleado(context.Background(), in, params())
	if err != nil {
		t.Fatal(err)
	}
	// SAL_BASE 1.000.000 + HRS_EXTRA 62.500 = 1.062.500
	// CCSS obrera 10.83% = 115.068,75 ; renta (1.062.500-918.000)*10% = 14.450
	want := map[string]string{
		"TotalIngresos": "1062500", "CuotaObreroCCSS": "115068.75", "CuotaPatronalCCSS": "285068.75",
		"ImpuestoRentaNeto": "14450", "TotalDeducciones": "129518.75", "NetoPagado": "932981.25",
	}
	got := map[string]decimal.Decimal{
		"TotalIngresos": r.TotalIngresos, "CuotaObreroCCSS": r.CuotaObreroCCSS, "CuotaPatronalCCSS": r.CuotaPatronalCCSS,
		"ImpuestoRentaNeto": r.ImpuestoRentaNeto, "TotalDeducciones": r.TotalDeducciones, "NetoPagado": r.NetoPagado,
	}
	for k, v := range want {
		if !got[k].Equal(d(v)) {
			t.Errorf("%s = %s, esperado %s", k, got[k], v)
		}
	}
	if len(r.DesglosePorConcepto) != 4 || r.DesglosePorConcepto[0].Codigo != "SAL_BASE" {
		t.Errorf("desglose/orden inesperado: %+v", r.DesglosePorConcepto)
	}
	if r.FormulaPorConcepto[1] != 100 {
		t.Errorf("trazabilidad de fórmula faltante")
	}
}

func TestCalcularEmpleado_HorasExtraSinConceptoAplicable(t *testing.T) {
	repo := repoBase()
	repo.conceptos = append([]ConceptoAplicableFila{}, repo.conceptos[:0]...)
	repo.conceptos = append(repo.conceptos, fila(1, "SAL_BASE", "ingreso", 10, `{"tipo":"referencia_directa"}`, true, true))
	in := CalculoEmpleadoInput{EmpleadoID: 7, FechaEfectiva: time.Now(), DiasTrabajados: d("30"), HorasExtra: []HoraExtraInput{{1, d("2")}}}
	if _, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, params()); err == nil {
		t.Fatal("esperaba error: horas extra sin HRS_EXTRA aplicable al régimen")
	}
}

func TestEvaluarConcepto_SinConceptos(t *testing.T) {
	repo := repoBase()
	repo.conceptos = nil
	if _, _, err := NewMotorEmpleado(repo).evaluarConcepto(context.Background(), 7, time.Now()); err == nil {
		t.Fatal("esperaba error por régimen sin conceptos")
	}
}
