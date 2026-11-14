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
func (f fakeMotorRepo) ListarConceptosAplicablesEnFecha(context.Context, int64, int64, int64, time.Time) ([]ConceptoAplicableFila, error) {
	return f.conceptos, nil
}
func (f fakeMotorRepo) ListarParametrosHorasExtraEnFecha(context.Context, time.Time) ([]ParametroHoraExtra, error) {
	return f.factores, nil
}

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func sp(s string) *string        { return &s }
func ip(i int64) *int64          { return &i }

// fila: CCSS_OBR, IMP_RENTA y EMBARGO son obligatorias.
func fila(id int64, cod, nat string, orden int32, expr string, ccss, renta bool) ConceptoAplicableFila {
	oblig := cod == "CCSS_OBR" || cod == "IMP_RENTA" || cod == "EMBARGO"
	return ConceptoAplicableFila{
		ConceptoOrdenCalculo: ConceptoOrdenCalculo{ConceptoSalarialID: id, Codigo: cod, OrdenCalculo: orden, AfectaBaseCCSS: ccss, AfectaBaseRenta: renta,
			AfectaBaseEmbargo: renta && nat == "ingreso", EsDeduccionObligatoria: oblig},
		Naturaleza: nat, TipoCalculo: "formula", FormulaID: ip(id * 100), ExpresionFormula: sp(expr),
	}
}

func repoBase() fakeMotorRepo {
	return fakeMotorRepo{
		contrato: ContratoVigente{ContratoID: 1, EmpleadoID: 7, RegimenLaboralID: 1, SalarioPactado: d("1000000"), Jornada: "DIURNA", JornadaID: ip(1)},
		conceptos: []ConceptoAplicableFila{
			fila(3, "IMP_RENTA", "deduccion", 110, `{"tipo":"renta"}`, false, false),
			fila(1, "SAL_BASE", "ingreso", 10, `{"tipo":"referencia_directa"}`, true, true),
			fila(4, "CCSS_OBR", "deduccion", 100, `{"tipo":"carga_social","lado":"obrero"}`, false, false),
			fila(2, "HRS_EXTRA", "ingreso", 20, `{"tipo":"horas_extra"}`, true, true),
			fila(5, "CCSS_PAT", "deduccion", 160, `{"tipo":"carga_social","lado":"patronal"}`, false, false),
		},
		factores: []ParametroHoraExtra{{TipoHoraExtraID: 1, FactorRecargo: d("1.5")}},
	}
}

// mesCompleto: un segmento de 30 días comerciales con el salario del contrato falso.
func mesCompleto() []SegmentoSalario {
	return []SegmentoSalario{{ContratoID: 1, Salario: d("1000000"), Dias: 30}}
}

func params() ParametrosCalculo {
	return ParametrosCalculo{
		CargasSociales: map[int64]map[int64]ParametroCargaSocial{
			4: {0: {ConceptoSalarialID: 4, PorcentajeObrero: d("0.1083")}},
			5: {0: {ConceptoSalarialID: 5, PorcentajePatronal: d("0.2683")}},
		},
		TramosRenta: []TramoRenta{
			{OrdenTramo: 1, MontoDesde: d("0"), MontoHasta: d("918000"), Porcentaje: d("0")},
			{OrdenTramo: 2, MontoDesde: d("918000"), MontoHasta: d("1347000"), Porcentaje: d("0.10")},
			{OrdenTramo: 3, MontoDesde: d("1347000"), Porcentaje: d("0.15"), Abierto: true},
		},
		BaseMinimaContributiva: map[int64]decimal.Decimal{1: d("232441")},
		DivisorPorJornada:      map[int64]decimal.Decimal{1: d("240")}, FraccionMensualPeriodo: d("1"), DiasPeriodoCompleto: d("30"), DivisorDiasMes: d("30"),
	}
}

func TestCalcularHorasExtra(t *testing.T) {
	f := map[int64]ParametroHoraExtra{1: {1, d("1.5")}, 4: {4, d("2")}}
	m, h, err := CalcularHorasExtra(d("1000000"), d("240"),
		[]HoraExtraInput{{1, d("10")}, {4, d("6")}}, f)
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
		EmpleadoID: 7, PeriodoID: 1, RegimenCCSSID: 1, FechaEfectiva: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("30"), Segmentos: mesCompleto(), HorasExtra: []HoraExtraInput{{1, d("10")}},
	}
	r, err := m.CalcularEmpleado(context.Background(), in, params())
	if err != nil {
		t.Fatal(err)
	}
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

// Un régimen CCSS con fila propia (p. ej. pensionado que no cotiza IVM)
// sustituye la tasa general del concepto; los demás regímenes siguen con la
// general.
func TestCalcularEmpleado_TasaCargaPorRegimen(t *testing.T) {
	p := params()
	p.BaseMinimaContributiva[2] = d("232441")
	p.CargasSociales[4][2] = ParametroCargaSocial{ConceptoSalarialID: 4, RegimenCCSSID: 2, PorcentajeObrero: d("0.0650")}
	in := CalculoEmpleadoInput{
		EmpleadoID: 7, PeriodoID: 1, RegimenCCSSID: 2, FechaEfectiva: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("30"), Segmentos: mesCompleto(),
	}
	r, err := NewMotorEmpleado(repoBase()).CalcularEmpleado(context.Background(), in, p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.CuotaObreroCCSS.Equal(d("65000")) || !r.CuotaPatronalCCSS.Equal(d("268300")) {
		t.Fatalf("obrero=%s patronal=%s; esperaba 65000 (tasa del régimen) y 268300 (general)", r.CuotaObreroCCSS, r.CuotaPatronalCCSS)
	}
}

// Varios conceptos patronales (componentes desglosados) suman la cuota
// patronal y dejan una línea contable cada uno.
func TestCalcularEmpleado_CargasDesglosadas(t *testing.T) {
	repo := repoBase()
	repo.conceptos = append(repo.conceptos, fila(6, "INA_PAT", "deduccion", 165, `{"tipo":"carga_social","lado":"patronal"}`, false, false))
	p := params()
	p.CargasSociales[6] = map[int64]ParametroCargaSocial{0: {ConceptoSalarialID: 6, PorcentajePatronal: d("0.015")}}
	in := CalculoEmpleadoInput{
		EmpleadoID: 7, PeriodoID: 1, RegimenCCSSID: 1, FechaEfectiva: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("30"), Segmentos: mesCompleto(),
	}
	r, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.CuotaPatronalCCSS.Equal(d("283300")) || len(r.Patronal) != 2 {
		t.Fatalf("patronal=%s líneas=%d; esperaba 283300 en 2 líneas", r.CuotaPatronalCCSS, len(r.Patronal))
	}
}

func TestCalcularEmpleado_CargaSinTasaVigente(t *testing.T) {
	p := params()
	delete(p.CargasSociales, 5)
	in := CalculoEmpleadoInput{
		EmpleadoID: 7, PeriodoID: 1, RegimenCCSSID: 1, FechaEfectiva: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("30"), Segmentos: mesCompleto(),
	}
	if _, err := NewMotorEmpleado(repoBase()).CalcularEmpleado(context.Background(), in, p); err == nil {
		t.Fatal("esperaba error: CCSS_PAT sin tasa vigente")
	}
}

func TestCalcularEmpleado_HorasExtraSinConceptoAplicable(t *testing.T) {
	repo := repoBase()
	repo.conceptos = append([]ConceptoAplicableFila{}, repo.conceptos[:0]...)
	repo.conceptos = append(repo.conceptos, fila(1, "SAL_BASE", "ingreso", 10, `{"tipo":"referencia_directa"}`, true, true))
	in := CalculoEmpleadoInput{EmpleadoID: 7, RegimenCCSSID: 1, FechaEfectiva: time.Now(), DiasTrabajados: d("30"), Segmentos: mesCompleto(), HorasExtra: []HoraExtraInput{{1, d("2")}}}
	if _, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, params()); err == nil {
		t.Fatal("esperaba error: horas extra sin HRS_EXTRA aplicable al régimen")
	}
}

func TestEvaluarConcepto_SinConceptos(t *testing.T) {
	repo := repoBase()
	repo.conceptos = nil
	if _, _, err := NewMotorEmpleado(repo).evaluarConcepto(context.Background(), 7, 1, time.Now()); err == nil {
		t.Fatal("esperaba error por régimen sin conceptos")
	}
}

// En una quincena el mínimo inembargable y el tope mensual del embargo se
// prorratean: 600 000 de líquido con un mínimo mensual de 400 000 (200 000 en
// la quincena) deja 400 000 de excedente, todo en el primer tramo de la
// escala (12,5 %): fondo de 50 000. El tope mensual de 60 000 baja a 30 000.
func TestEmbargoQuincenalProrrateado(t *testing.T) {
	repo := repoBase()
	repo.conceptos = []ConceptoAplicableFila{
		fila(1, "SAL_BASE", "ingreso", 10, `{"tipo":"referencia_directa"}`, true, true),
		fila(8, "EMBARGO", "deduccion", 120, `{"tipo":"embargo"}`, false, false),
	}
	repo.conceptos[0].AfectaBaseEmbargo = true
	p := params()
	p.FraccionMensualPeriodo, p.DiasPeriodoCompleto = d("0.5"), d("15")
	p.SalarioMinimoInembargable = d("400000")
	p.EscalaEmbargo = escalaHistorica()
	in := CalculoEmpleadoInput{
		EmpleadoID: 7, PeriodoID: 1, RegimenCCSSID: 1, FechaEfectiva: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("15"), Segmentos: []SegmentoSalario{{ContratoID: 1, Salario: d("1200000"), Dias: 15}},
		Embargos: []EmbargoInput{{EmbargoID: 1, Prioridad: 1, PrioridadLegal: 2, Porcentaje: d("1"), TopeMensual: d("60000")}},
	}
	r, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.EmbargosAplicados) != 1 || !r.EmbargosAplicados[0].MontoAplicado.Equal(d("30000")) {
		t.Fatalf("embargo aplicado %+v; esperaba 30000 (tope mensual prorrateado)", r.EmbargosAplicados)
	}
	in.Embargos[0].TopeMensual = decimal.Zero
	r, err = NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.EmbargosAplicados[0].MontoAplicado.Equal(d("50000")) {
		t.Fatalf("embargo aplicado %s; esperaba 50000 (mínimo prorrateado)", r.EmbargosAplicados[0].MontoAplicado)
	}
}

// Asignaciones: un porcentaje sobre el bruto (asociación) y un monto mensual
// prorrateado en la quincena (sindicato); las deducciones voluntarias se topan
// por el neto.
func TestCalcularEmpleado_Asignaciones(t *testing.T) {
	repo := repoBase()
	repo.conceptos = []ConceptoAplicableFila{
		fila(1, "SAL_BASE", "ingreso", 10, `{"tipo":"referencia_directa"}`, true, true),
		fila(20, "ASOC", "deduccion", 140, `{"tipo":"asignacion_empleado","base":"bruto"}`, false, false),
		fila(21, "SIND", "deduccion", 150, `{"tipo":"asignacion_empleado","base":"bruto"}`, false, false),
	}
	p := params()
	p.FraccionMensualPeriodo, p.DiasPeriodoCompleto = d("0.5"), d("15")
	in := CalculoEmpleadoInput{
		EmpleadoID: 7, PeriodoID: 1, RegimenCCSSID: 1, FechaEfectiva: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("15"), Segmentos: []SegmentoSalario{{ContratoID: 1, Salario: d("1000000"), Dias: 15}},
		Asignaciones: []AsignacionInput{
			{AsignacionID: 1, ConceptoSalarialID: 20, Modo: AsignacionPorcentaje, Valor: d("0.05")},
			{AsignacionID: 2, ConceptoSalarialID: 21, Modo: AsignacionMonto, Valor: d("8000")},
		},
	}
	r, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range r.DesglosePorConcepto {
		got[l.Codigo] = l.Monto.String()
	}
	if got["ASOC"] != "25000" || got["SIND"] != "4000" || !r.NetoPagado.Equal(d("471000")) {
		t.Fatalf("desglose %v neto %s; esperaba ASOC 25000, SIND 4000, neto 471000", got, r.NetoPagado)
	}

	in.Asignaciones = append(in.Asignaciones, AsignacionInput{AsignacionID: 3, ConceptoSalarialID: 99, Modo: AsignacionMonto, Valor: d("1")})
	if _, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), in, p); err == nil {
		t.Fatal("esperaba error: asignación de un concepto que no aplica")
	}
}

func TestValidarExpresionAsignacion(t *testing.T) {
	if err := ValidarExpresionFormula(`{"tipo":"asignacion_empleado","base":"ccss"}`, nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidarExpresionFormula(`{"tipo":"asignacion_empleado"}`, nil); err == nil {
		t.Fatal("esperaba error: asignacion_empleado sin base")
	}
}
