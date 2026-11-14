package planilla

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// IncapacidadInput: parte de una boleta dentro del período.
type IncapacidadInput struct {
	IncapacidadID    int64
	EntidadEmisoraID int64
	Desde, Hasta     time.Time // ya recortadas al período
	DiaSerieDesde    int32     // número de día de la boleta que corresponde a Desde
}

// TramoSubsidio: tramo vigente de la escala. Porcentajes en FRACCIÓN.
type TramoSubsidio struct {
	EntidadEmisoraID int64
	DiaDesde         int32
	DiaHasta         *int32 // nil = sin tope
	PctPatronal      decimal.Decimal
	PctEntidad       decimal.Decimal
}

// SubsidioIncapacidad: resultado de una boleta (o un tramo de ella).
type SubsidioIncapacidad struct {
	Dias                    int // días comerciales
	MontoPatronal           decimal.Decimal
	MontoEntidadInformativo decimal.Decimal
	Desglose                []LineaIncapacidad
}

func (t TramoSubsidio) cubre(dia int32) bool {
	return dia >= t.DiaDesde && (t.DiaHasta == nil || dia <= *t.DiaHasta)
}

// CalcularSubsidioIncapacidad reparte los días de la boleta entre los tramos
// de su entidad emisora. Un día sin tramo vigente es error.
func CalcularSubsidioIncapacidad(salarioDiario decimal.Decimal, inc IncapacidadInput, tramos []TramoSubsidio, divisor int) (SubsidioIncapacidad, error) {
	if divisor <= 0 {
		return SubsidioIncapacidad{}, errors.New("planilla: divisor de días del mes no definido para la incapacidad")
	}
	if salarioDiario.IsNegative() {
		return SubsidioIncapacidad{}, errors.New("planilla: salario diario negativo en incapacidad")
	}
	if inc.DiaSerieDesde < 1 {
		return SubsidioIncapacidad{}, fmt.Errorf("planilla: incapacidad_id=%d con día de serie inválido %d", inc.IncapacidadID, inc.DiaSerieDesde)
	}
	desde, hasta := soloFecha(inc.Desde), soloFecha(inc.Hasta)
	if hasta.Before(desde) {
		return SubsidioIncapacidad{}, nil
	}
	propios := make([]TramoSubsidio, 0, len(tramos))
	for _, t := range tramos {
		if t.EntidadEmisoraID == inc.EntidadEmisoraID {
			propios = append(propios, t)
		}
	}
	sort.Slice(propios, func(i, j int) bool { return propios[i].DiaDesde < propios[j].DiaDesde })

	var out SubsidioIncapacidad
	var actual *LineaIncapacidad
	var tramoActual int = -1
	cerrar := func() {
		if actual == nil {
			return
		}
		t := propios[tramoActual]
		base := salarioDiario.Mul(actual.Dias)
		actual.MontoPatronal = base.Mul(t.PctPatronal).Round(dosDecimales)
		actual.MontoEntidadInformativo = base.Mul(t.PctEntidad).Round(dosDecimales)
		out.MontoPatronal = out.MontoPatronal.Add(actual.MontoPatronal)
		out.MontoEntidadInformativo = out.MontoEntidadInformativo.Add(actual.MontoEntidadInformativo)
		out.Desglose = append(out.Desglose, *actual)
		actual = nil
	}
	serie := inc.DiaSerieDesde
	for dia := desde; !dia.After(hasta); dia, serie = dia.AddDate(0, 0, 1), serie+1 {
		idx := -1
		for i, t := range propios {
			if t.cubre(serie) {
				if idx >= 0 {
					return SubsidioIncapacidad{}, fmt.Errorf("planilla: tramos de subsidio solapados para el día %d (entidad %d)", serie, inc.EntidadEmisoraID)
				}
				idx = i
			}
		}
		if idx < 0 {
			return SubsidioIncapacidad{}, fmt.Errorf("planilla: no hay tramo de subsidio vigente para el día %d de la incapacidad_id=%d (entidad %d)",
				serie, inc.IncapacidadID, inc.EntidadEmisoraID)
		}
		peso := DiasComerciales(dia, dia, divisor)
		out.Dias += peso
		if idx != tramoActual {
			cerrar()
			tramoActual = idx
			actual = &LineaIncapacidad{IncapacidadID: inc.IncapacidadID, DiaSerieDesde: serie, Dias: decimal.Zero}
		}
		actual.DiaSerieHasta = serie
		actual.Dias = actual.Dias.Add(decimal.NewFromInt(int64(peso)))
	}
	cerrar()
	return out, nil
}

// interseccion de [a1,a2] y [b1,b2]; ok=false si no se tocan.
func interseccion(a1, a2, b1, b2 time.Time) (time.Time, time.Time, bool) {
	ini, fin := maxFecha(soloFecha(a1), soloFecha(b1)), minFecha(soloFecha(a2), soloFecha(b2))
	return ini, fin, !fin.Before(ini)
}

// validarIncapacidadesSinSolape: dos boletas el mismo día descontarían el
// salario dos veces.
func validarIncapacidadesSinSolape(incs []IncapacidadInput) error {
	for i := range incs {
		for j := i + 1; j < len(incs); j++ {
			if _, _, ok := interseccion(incs[i].Desde, incs[i].Hasta, incs[j].Desde, incs[j].Hasta); ok {
				return fmt.Errorf("planilla: las incapacidades %d y %d se solapan", incs[i].IncapacidadID, incs[j].IncapacidadID)
			}
		}
	}
	return nil
}

// diasIncapacidadEnSegmento: días comerciales del segmento cubiertos por
// incapacidad.
func diasIncapacidadEnSegmento(s SegmentoSalario, incs []IncapacidadInput, divisor int) int {
	n := 0
	for _, inc := range incs {
		if ini, fin, ok := interseccion(s.Inicio, s.Fin, inc.Desde, inc.Hasta); ok {
			n += DiasComerciales(ini, fin, divisor)
		}
	}
	return n
}

// SegmentosNetos descuenta de cada segmento los días de incapacidad (no se
// paga salario por los días subsidiados).
func SegmentosNetos(segs []SegmentoSalario, incs []IncapacidadInput, divisor int) ([]SegmentoSalario, error) {
	if err := validarIncapacidadesSinSolape(incs); err != nil {
		return nil, err
	}
	out := make([]SegmentoSalario, len(segs))
	for i, s := range segs {
		s.Dias -= diasIncapacidadEnSegmento(s, incs, divisor)
		if s.Dias < 0 {
			return nil, fmt.Errorf("planilla: incapacidad mayor que los días del segmento del contrato %d", s.ContratoID)
		}
		out[i] = s
	}
	return out, nil
}
