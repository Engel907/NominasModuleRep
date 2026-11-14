package planilla

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

func ultimoDiaDelMes(t time.Time) bool {
	return t.AddDate(0, 0, 1).Month() != t.Month()
}

// diaComercial: valor del día dentro de su mes (1..divisor).
func diaComercial(t time.Time, divisor int) int {
	if ultimoDiaDelMes(t) {
		return divisor
	}
	return min(t.Day(), divisor)
}

// DiasComerciales cuenta los días comerciales de [desde, hasta] (ambos
// inclusive). 0 si hasta < desde. Dentro de cada mes es
// posición(fin) − posición(día anterior a inicio), así que cualquier partición
// del mes en tramos suma exactamente `divisor` (un aumento el día 31 aporta 0
// días; un ingreso el último día de febrero aporta divisor − 27).
func DiasComerciales(desde, hasta time.Time, divisor int) int {
	desde, hasta = soloFecha(desde), soloFecha(hasta)
	if hasta.Before(desde) || divisor <= 0 {
		return 0
	}
	total := 0
	for ini := desde; !ini.After(hasta); {
		finMes := time.Date(ini.Year(), ini.Month()+1, 0, 0, 0, 0, 0, time.UTC)
		fin := minFecha(finMes, hasta)
		previo := 0
		if ini.Day() > 1 {
			previo = diaComercial(ini.AddDate(0, 0, -1), divisor)
		}
		total += diaComercial(fin, divisor) - previo
		ini = finMes.AddDate(0, 0, 1)
	}
	return total
}

// VigenciaContrato: tramo en que un contrato (versión) estuvo vigente con un
// salario. Hasta cero = sin fin.
type VigenciaContrato struct {
	ContratoID   int64
	Desde, Hasta time.Time
	Salario      decimal.Decimal
	Modalidad    string // contratos.modalidad_salario
	JornadaID    int64
}

// SegmentoSalario: parte del período con un mismo contrato y salario.
// Salario va en la unidad de Modalidad (mes, día u hora).
type SegmentoSalario struct {
	Inicio, Fin time.Time
	Salario     decimal.Decimal
	Modalidad   string
	JornadaID   int64
	ContratoID  int64
	Dias        int // días comerciales
}

// SegmentarPeriodo recorta las vigencias a [desde, hasta] y las ordena. Los
// días sin contrato quedan fuera. Convención del proyecto (CerrarContratoVigente):
// la versión que se cierra termina el mismo día en que empieza la nueva; ese
// día de frontera es de la versión nueva. Cualquier otro solape es error (no
// se sabe qué salario pagar).
func SegmentarPeriodo(desde, hasta time.Time, v []VigenciaContrato, divisor int) ([]SegmentoSalario, error) {
	if divisor <= 0 {
		return nil, errors.New("planilla: divisor de días del mes no definido para segmentar el período")
	}
	desde, hasta = soloFecha(desde), soloFecha(hasta)
	var segs []SegmentoSalario
	for _, x := range v {
		if x.Salario.IsNegative() {
			return nil, fmt.Errorf("planilla: contrato_id=%d con salario negativo", x.ContratoID)
		}
		ini := maxFecha(soloFecha(x.Desde), desde)
		fin := hasta
		if !x.Hasta.IsZero() {
			fin = minFecha(soloFecha(x.Hasta), hasta)
		}
		if fin.Before(ini) {
			continue
		}
		segs = append(segs, SegmentoSalario{Inicio: ini, Fin: fin, Salario: x.Salario, Modalidad: x.Modalidad,
			JornadaID: x.JornadaID, ContratoID: x.ContratoID})
	}
	sort.SliceStable(segs, func(i, j int) bool { return segs[i].Inicio.Before(segs[j].Inicio) })
	for i := 1; i < len(segs); i++ {
		prev, act := &segs[i-1], segs[i]
		switch {
		case prev.Fin.Before(act.Inicio):
		case prev.Fin.Equal(act.Inicio) && prev.Inicio.Before(act.Inicio):
			prev.Fin = act.Inicio.AddDate(0, 0, -1)
		default:
			return nil, fmt.Errorf("planilla: los contratos %d y %d se solapan entre %s y %s",
				prev.ContratoID, act.ContratoID, act.Inicio.Format("2006-01-02"), prev.Fin.Format("2006-01-02"))
		}
	}
	out := segs[:0]
	for _, x := range segs {
		x.Dias = DiasComerciales(x.Inicio, x.Fin, divisor)
		out = append(out, x)
	}
	return out, nil
}

// SalarioBasePeriodo = Σ(salario × días ÷ divisor), con un solo redondeo a 2
// decimales al final.
func SalarioBasePeriodo(s []SegmentoSalario, divisor decimal.Decimal) decimal.Decimal {
	if !divisor.IsPositive() {
		return decimal.Zero
	}
	total := decimal.Zero
	for _, x := range s {
		total = total.Add(x.Salario.Mul(decimal.NewFromInt(int64(x.Dias))))
	}
	return total.DivRound(divisor, 6).Round(dosDecimales)
}

// DiasSegmentos: total de días comerciales con contrato.
func DiasSegmentos(s []SegmentoSalario) int {
	n := 0
	for _, x := range s {
		n += x.Dias
	}
	return n
}

// divisorEntero convierte DIVISOR_DIAS_MES a entero (debe ser entero > 0).
func divisorEntero(d decimal.Decimal) (int, error) {
	if !d.IsPositive() || !d.Equal(d.Truncate(0)) {
		return 0, fmt.Errorf("planilla: DIVISOR_DIAS_MES debe ser un entero positivo (es %s)", d)
	}
	return int(d.IntPart()), nil
}
