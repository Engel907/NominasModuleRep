package planilla

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func paramsModalidad(base string) ParametrosCalculo {
	return ParametrosCalculo{
		DivisorDiasMes: d("30"), DiasPeriodoCompleto: d("30"), FraccionMensualPeriodo: d("1"),
		DivisorPorJornada:      map[int64]decimal.Decimal{1: d("240")},
		HorasDiariasPorJornada: map[int64]decimal.Decimal{1: d("8")},
		SalarioDiarioBase:      base,
		VacacionesModoPago:     VacacionesPagoCompensacion,
	}
}

func segmentoMarzo(modalidad, salario string) SegmentoSalario {
	return SegmentoSalario{ContratoID: 1, Inicio: fecha("2026-03-01"), Fin: fecha("2026-03-31"),
		Salario: d(salario), Modalidad: modalidad, JornadaID: 1, Dias: 30}
}

func asistencia(horas string, fechas ...string) []DiaAsistencia {
	out := make([]DiaAsistencia, 0, len(fechas))
	for _, f := range fechas {
		out = append(out, DiaAsistencia{Fecha: fecha(f), Horas: d(horas)})
	}
	return out
}

func TestSalarioBaseSegunModalidad(t *testing.T) {
	trabajados := asistencia("8", "2026-03-02", "2026-03-03", "2026-03-04", "2026-03-05", "2026-03-06")
	casos := []struct {
		nombre    string
		seg       SegmentoSalario
		base      string
		in        CalculoEmpleadoInput
		wantMonto string
		wantDias  int
	}{
		{"mensual: días comerciales", segmentoMarzo(ModalidadMensual, "900000"), "", CalculoEmpleadoInput{}, "900000.00", 30},
		{"diario por días comerciales", segmentoMarzo(ModalidadDiario, "20000"), SalarioDiarioComerciales, CalculoEmpleadoInput{}, "600000.00", 30},
		{"diario por asistencia", segmentoMarzo(ModalidadDiario, "20000"), SalarioDiarioAsistencia,
			CalculoEmpleadoInput{Asistencia: trabajados}, "100000.00", 5},
		{"diario por asistencia + feriado obligatorio no trabajado", segmentoMarzo(ModalidadDiario, "20000"), SalarioDiarioAsistencia,
			CalculoEmpleadoInput{Asistencia: trabajados, FeriadosPagoObligatorio: []time.Time{fecha("2026-03-19"), fecha("2026-03-02")}}, "120000.00", 6},
		{"por hora: horas registradas", segmentoMarzo(ModalidadPorHora, "2500"), "",
			CalculoEmpleadoInput{Asistencia: append(trabajados, DiaAsistencia{Fecha: fecha("2026-03-07"), Horas: d("4.5")})}, "111250.00", 6},
		{"por hora: feriado obligatorio a horas diarias de la jornada", segmentoMarzo(ModalidadPorHora, "2500"), "",
			CalculoEmpleadoInput{FeriadosPagoObligatorio: []time.Time{fecha("2026-03-19")}}, "20000.00", 1},
		{"por hora: el día incapacitado no se paga aunque tenga marca", segmentoMarzo(ModalidadPorHora, "2500"), "",
			CalculoEmpleadoInput{Asistencia: trabajados, Incapacidades: []IncapacidadInput{{IncapacidadID: 9, Desde: fecha("2026-03-06"), Hasta: fecha("2026-03-06"), DiaSerieDesde: 1}}},
			"80000.00", 4},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p := paramsModalidad(c.base)
			monto, dias, err := salarioBaseSegmentos([]SegmentoSalario{c.seg}, c.in, p)
			if err != nil {
				t.Fatal(err)
			}
			if monto.StringFixed(2) != c.wantMonto || dias != c.wantDias {
				t.Errorf("monto %s días %d; esperado %s y %d", monto.StringFixed(2), dias, c.wantMonto, c.wantDias)
			}
		})
	}
}

func TestSalarioDiarioExigeBase(t *testing.T) {
	_, _, err := salarioBaseSegmentos([]SegmentoSalario{segmentoMarzo(ModalidadDiario, "20000")}, CalculoEmpleadoInput{}, paramsModalidad(""))
	if !errors.Is(err, ErrParametroFaltante) {
		t.Fatalf("sin SALARIO_DIARIO_BASE se esperaba ErrParametroFaltante, fue %v", err)
	}
}

func TestVacacionesPorAsistenciaSePaganEnSalario(t *testing.T) {
	in := CalculoEmpleadoInput{Asistencia: asistencia("8", "2026-03-02"),
		VacacionesDisfrute: []RangoVacaciones{{SolicitudID: 1, Desde: fecha("2026-03-09"), Hasta: fecha("2026-03-11")}}}
	monto, dias, err := salarioBaseSegmentos([]SegmentoSalario{segmentoMarzo(ModalidadDiario, "20000")}, in, paramsModalidad(SalarioDiarioAsistencia))
	if err != nil {
		t.Fatal(err)
	}
	if monto.StringFixed(2) != "80000.00" || dias != 4 {
		t.Errorf("monto %s días %d; esperado 80000.00 y 4 (1 trabajado + 3 de vacaciones)", monto.StringFixed(2), dias)
	}
}

func TestValorDiaYHora(t *testing.T) {
	p := paramsModalidad(SalarioDiarioComerciales)
	for _, c := range []struct {
		modalidad, salario, wantDia string
	}{{ModalidadMensual, "900000", "30000"}, {ModalidadDiario, "20000", "20000"}, {ModalidadPorHora, "2500", "20000"}} {
		v, err := segmentoMarzo(c.modalidad, c.salario).ValorDia(p)
		if err != nil || !v.Equal(d(c.wantDia)) {
			t.Errorf("%s: valor día %s (%v), esperado %s", c.modalidad, v, err, c.wantDia)
		}
	}
	horas := []HoraExtraInput{{TipoHoraExtraID: 1, CantidadHoras: d("2")}}
	factores := map[int64]ParametroHoraExtra{1: {TipoHoraExtraID: 1, FactorRecargo: d("1.5")}}
	for _, c := range []struct {
		modalidad, salario, want string
	}{{ModalidadMensual, "960000", "12000.00"}, {ModalidadDiario, "32000", "12000.00"}, {ModalidadPorHora, "4000", "12000.00"}} {
		sal, div, err := valorHoraOrdinaria(ContratoVigente{ContratoID: 1, SalarioPactado: d(c.salario), ModalidadSalario: c.modalidad, JornadaID: ip(1)}, p)
		if err != nil {
			t.Fatal(err)
		}
		monto, _, err := CalcularHorasExtra(sal, div, horas, factores)
		if err != nil || monto.StringFixed(2) != c.want {
			t.Errorf("%s: horas extra %s (%v), esperado %s", c.modalidad, monto, err, c.want)
		}
	}
	sinHoras := p
	sinHoras.HorasDiariasPorJornada = nil
	if _, err := segmentoMarzo(ModalidadPorHora, "2500").ValorDia(sinHoras); err == nil {
		t.Error("salario por hora sin horas diarias de la jornada debe dar error")
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
