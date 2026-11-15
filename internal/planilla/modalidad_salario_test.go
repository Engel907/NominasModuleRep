package planilla

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func paramsModalidad() ParametrosCalculo {
	return ParametrosCalculo{
		DivisorDiasMes: d("30"), DiasPeriodoCompleto: d("30"), FraccionMensualPeriodo: d("1"),
		DivisorPorJornada:  map[int64]decimal.Decimal{1: d("240")},
		VacacionesModoPago: VacacionesPagoCompensacion,
	}
}

func segmentoUnico(t *testing.T, modalidad, salario, desde, hasta string) SegmentoSalario {
	t.Helper()
	segs, err := SegmentarPeriodo(fecha(desde), fecha(hasta), []VigenciaContrato{{ContratoID: 1,
		Desde: fecha(desde), Salario: d(salario), Modalidad: modalidad, JornadaID: 1}}, 30)
	if err != nil || len(segs) != 1 {
		t.Fatalf("segmentar: %v (%d segmentos)", err, len(segs))
	}
	return segs[0]
}

func TestSalarioBaseSegunModalidad(t *testing.T) {
	casos := []struct {
		nombre, modalidad, salario, desde, hasta string
		wantMonto                                string
		wantDias                                 int
	}{
		{"mensual: mes completo", ModalidadMensual, "900000", "2026-03-01", "2026-03-31", "900000.00", 30},
		{"mensual en una semana de fin de febrero: días comerciales", ModalidadMensual, "900000", "2026-02-22", "2026-02-28", "270000.00", 9},
		{"quincenal: primera quincena", ModalidadQuincenal, "450000", "2026-03-01", "2026-03-15", "450000.00", 15},
		{"quincenal: segunda quincena de febrero", ModalidadQuincenal, "450000", "2026-02-16", "2026-02-28", "450000.00", 15},
		{"quincenal: mes completo", ModalidadQuincenal, "450000", "2026-03-01", "2026-03-31", "900000.00", 30},
		{"semanal: una semana", ModalidadSemanal, "70000", "2026-03-02", "2026-03-08", "70000.00", 7},
		{"semanal: semana que cruza de mes", ModalidadSemanal, "70000", "2026-02-26", "2026-03-04", "70000.00", 7},
		{"semanal: marzo completo por días naturales", ModalidadSemanal, "70000", "2026-03-01", "2026-03-31", "310000.00", 31},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			seg := segmentoUnico(t, c.modalidad, c.salario, c.desde, c.hasta)
			monto, dias, err := salarioBaseSegmentos([]SegmentoSalario{seg}, paramsModalidad())
			if err != nil {
				t.Fatal(err)
			}
			if monto.StringFixed(2) != c.wantMonto || dias != c.wantDias {
				t.Errorf("monto %s días %d; esperado %s y %d", monto.StringFixed(2), dias, c.wantMonto, c.wantDias)
			}
		})
	}
}

func TestModalidadDesconocidaEsError(t *testing.T) {
	_, err := SegmentarPeriodo(fecha("2026-03-01"), fecha("2026-03-31"),
		[]VigenciaContrato{{ContratoID: 1, Desde: fecha("2026-03-01"), Salario: d("1000"), Modalidad: "anual"}}, 30)
	if err == nil {
		t.Fatal("una modalidad fuera de mensual | quincenal | semanal debe dar error")
	}
	if ModalidadSalarioValida("diario") || ModalidadSalarioValida("por_hora") {
		t.Error("diario y por_hora no son modalidades válidas")
	}
}

func TestEquivalenteMensual(t *testing.T) {
	for _, c := range []struct{ modalidad, salario, want string }{
		{ModalidadMensual, "900000", "900000"}, {ModalidadQuincenal, "450000", "900000"}, {ModalidadSemanal, "70000", "300000"},
	} {
		m, err := EquivalenteMensual(d(c.salario), c.modalidad, d("30"))
		if err != nil || !m.Equal(d(c.want)) {
			t.Errorf("%s: equivalente %s (%v), esperado %s", c.modalidad, m, err, c.want)
		}
	}
	if _, err := EquivalenteMensual(d("70000"), ModalidadSemanal, decimal.Zero); err == nil {
		t.Error("semanal sin DIVISOR_DIAS_MES debe dar error")
	}
}

func TestValorDiaYHora(t *testing.T) {
	p := paramsModalidad()
	for _, c := range []struct {
		modalidad, salario, wantDia string
	}{{ModalidadMensual, "900000", "30000"}, {ModalidadQuincenal, "450000", "30000"}, {ModalidadSemanal, "70000", "10000"}} {
		v, err := SegmentoSalario{ContratoID: 1, Salario: d(c.salario), Modalidad: c.modalidad}.ValorDia(p)
		if err != nil || !v.Equal(d(c.wantDia)) {
			t.Errorf("%s: valor día %s (%v), esperado %s", c.modalidad, v, err, c.wantDia)
		}
	}
	horas := []HoraExtraInput{{TipoHoraExtraID: 1, CantidadHoras: d("2")}}
	factores := map[int64]ParametroHoraExtra{1: {TipoHoraExtraID: 1, FactorRecargo: d("1.5")}}
	for _, c := range []struct {
		modalidad, salario, want string
	}{{ModalidadMensual, "960000", "12000.00"}, {ModalidadQuincenal, "480000", "12000.00"}, {ModalidadSemanal, "224000", "12000.00"}} {
		sal, div, err := valorHoraOrdinaria(ContratoVigente{ContratoID: 1, SalarioPactado: d(c.salario), ModalidadSalario: c.modalidad, JornadaID: ip(1)}, p)
		if err != nil {
			t.Fatal(err)
		}
		monto, _, err := CalcularHorasExtra(sal, div, horas, factores)
		if err != nil || monto.StringFixed(2) != c.want {
			t.Errorf("%s: horas extra %s (%v), esperado %s", c.modalidad, monto, err, c.want)
		}
	}
}

func TestBaseDias(t *testing.T) {
	com, nat := BaseDias{Divisor: 30}, BaseDias{Divisor: 30, Naturales: true}
	if n := com.Contar(fecha("2026-02-01"), fecha("2026-02-28")); n != 30 {
		t.Errorf("febrero comercial = %d, esperado 30", n)
	}
	if n := nat.Contar(fecha("2026-02-01"), fecha("2026-02-28")); n != 28 {
		t.Errorf("febrero natural = %d, esperado 28", n)
	}
	if n := nat.Contar(fecha("2026-03-05"), fecha("2026-03-04")); n != 0 {
		t.Errorf("rango invertido = %d, esperado 0", n)
	}
}

func TestValorDiaSegunBase(t *testing.T) {
	p := paramsModalidad()
	prom := &PromedioSalario{Total: d("2700000"), Meses: d("3"), Periodos: 3} // 900000/mes -> 30000/día
	vacio := &PromedioSalario{}
	casos := []struct {
		nombre, base, vigente string
		prom                  *PromedioSalario
		want                  string
		advierte              bool
	}{
		{"vigente", SalarioBaseVigente, "25000", prom, "25000", false},
		{"promedio", SalarioBasePromedio, "25000", prom, "30000", false},
		{"mayor: gana el promedio", SalarioBaseMayor, "25000", prom, "30000", false},
		{"mayor: gana el vigente", SalarioBaseMayor, "35000", prom, "35000", false},
		{"promedio sin planillas: vigente con advertencia", SalarioBasePromedio, "25000", vacio, "25000", true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			st := &estadoCalculo{res: &ResultadoEmpleadoOutput{}}
			v, err := valorDiaSegunBase(ClaveVacacionesSalarioBase, c.base, d(c.vigente), c.prom, p, st)
			if err != nil || !v.Equal(d(c.want)) {
				t.Fatalf("valor %s (%v), esperado %s", v, err, c.want)
			}
			if (len(st.res.Advertencias) > 0) != c.advierte {
				t.Errorf("advertencias %v", st.res.Advertencias)
			}
		})
	}
	st := &estadoCalculo{res: &ResultadoEmpleadoOutput{}}
	if _, err := valorDiaSegunBase(ClaveVacacionesSalarioBase, "", d("1"), nil, p, st); !errors.Is(err, ErrParametroFaltante) {
		t.Errorf("sin base configurada se esperaba ErrParametroFaltante, fue %v", err)
	}
}

func TestCompensacionVacacionesConPromedio(t *testing.T) {
	p := paramsModalidad()
	p.VacacionesSalarioBase = SalarioBasePromedio
	in := CalculoEmpleadoInput{
		Segmentos:              []SegmentoSalario{segmentoUnico(t, ModalidadMensual, "750000", "2026-03-01", "2026-03-31")},
		VacacionesCompensacion: []CompensacionVacacionesInput{{MovimientoID: 7, Dias: d("2")}},
		PromedioVacaciones:     &PromedioSalario{Total: d("10800000"), Meses: d("12"), Periodos: 12},
	}
	st := &estadoCalculo{res: &ResultadoEmpleadoOutput{}}
	monto, dias, movs, err := calcularPagoVacaciones(in, p, st)
	if err != nil {
		t.Fatal(err)
	}
	if monto.StringFixed(2) != "60000.00" || !dias.Equal(d("2")) || len(movs) != 1 {
		t.Errorf("monto %s días %s movs %v; esperado 60000.00 (2 días a 30000 de promedio)", monto.StringFixed(2), dias, movs)
	}
}

// En la liquidación el embargo se calcula sobre el mes: lo que retiene la
// liquidación más lo ya retenido en la planilla regular es lo mismo que daría
// un único cálculo sobre el líquido del mes.
func TestEmbargoLiquidacionAcumulaElMes(t *testing.T) {
	p := ParametrosCalculo{SalarioMinimoInembargable: d(salarioMinimoInembargable2026), EscalaEmbargo: escalaHistorica()}
	emb := []EmbargoInput{pension(2, "0.30"), judicial(1, "0.20")}
	regular, err := CalcularEmbargos(emb, d("700000"), p.SalarioMinimoInembargable, p.EscalaEmbargo)
	if err != nil {
		t.Fatal(err)
	}
	prev := EmbargoMesPrevio{Base: d("700000"), PorEmbargo: map[int64]EmbargoPrevio{}}
	for _, r := range regular {
		prev.PorEmbargo[r.EmbargoID] = EmbargoPrevio{Calculado: r.MontoCalculado, Aplicado: r.MontoAplicado}
	}
	liq, err := embargosDelMes(emb, d("900000"), prev, p)
	if err != nil {
		t.Fatal(err)
	}
	mes, err := CalcularEmbargos(emb, d("1600000"), p.SalarioMinimoInembargable, p.EscalaEmbargo)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range liq {
		total := r.MontoAplicado.Add(prev.PorEmbargo[r.EmbargoID].Aplicado)
		if !total.Equal(mes[i].MontoAplicado) {
			t.Errorf("embargo %d: regular + liquidación = %s, el mes completo da %s", r.EmbargoID, total, mes[i].MontoAplicado)
		}
		if !r.SaldoPendienteInsatisfecho.Equal(r.MontoCalculado.Sub(r.MontoAplicado)) || r.MontoAplicado.IsNegative() {
			t.Errorf("embargo %d con montos inconsistentes: %+v", r.EmbargoID, r)
		}
	}
	sinMes, err := CalcularEmbargos(emb, d("900000"), p.SalarioMinimoInembargable, p.EscalaEmbargo)
	if err != nil {
		t.Fatal(err)
	}
	if liq[1].MontoAplicado.Equal(sinMes[1].MontoAplicado) {
		t.Error("el judicial de la liquidación no debe volver a descontar el mínimo inembargable del mes")
	}
}

func TestEmbargoMontoFijoEnLiquidacion(t *testing.T) {
	fijo := EmbargoInput{EmbargoID: 3, TipoEmbargoID: 1, Prioridad: 1, PrioridadLegal: 1, EsPension: true, TopePctLiquido: d("0.5"), MontoFijo: d("100000")}
	prev := EmbargoMesPrevio{Base: d("700000"), PorEmbargo: map[int64]EmbargoPrevio{3: {Calculado: d("100000"), Aplicado: d("100000")}}}
	p := ParametrosCalculo{SalarioMinimoInembargable: d(salarioMinimoInembargable2026), EscalaEmbargo: escalaHistorica()}

	if _, err := embargosDelMes([]EmbargoInput{fijo}, d("500000"), prev, p); !errors.Is(err, ErrParametroFaltante) {
		t.Fatalf("sin LIQ_EMBARGO_MONTO_FIJO se esperaba ErrParametroFaltante, fue %v", err)
	}
	p.LiqEmbargoMontoFijo = LiqEmbargoMontoFijoMensual
	r, err := embargosDelMes([]EmbargoInput{fijo}, d("500000"), prev, p)
	if err != nil || !r[0].MontoAplicado.IsZero() {
		t.Errorf("mensual: el monto fijo ya retenido en el mes no se repite (%v, %+v)", err, r)
	}
	p.LiqEmbargoMontoFijo = LiqEmbargoMontoFijoPorPeriodo
	r, err = embargosDelMes([]EmbargoInput{fijo}, d("500000"), prev, p)
	if err != nil || !r[0].MontoAplicado.Equal(d("100000")) {
		t.Errorf("por_periodo: la liquidación vuelve a retener el monto fijo (%v, %+v)", err, r)
	}
}
