package planilla

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaIncapacidad (incapacidad): SUB_INCAP = parte PATRONAL del subsidio de
// cada boleta, por tramos de parametros_subsidios_incapacidad y con el
// salario diario que fija INCAPACIDAD_SALARIO_BASE: el del segmento de
// contrato de cada día (ValorDia), el promedio de los meses anteriores a la
// boleta o el mayor de ambos. La parte de la CCSS/INS no
// suma al neto: queda en planilla_detalle_incapacidad.monto_entidad_informativo.
// Los días ya se descontaron del salario base (regla_salario).
type reglaIncapacidad struct{}

func (reglaIncapacidad) Aplicar(rc *ContextoRegla) error {
	st := rc.St
	st.usados[formulaIncapacidad] = true
	if len(rc.In.Incapacidades) == 0 {
		return nil
	}
	divisor, err := divisorEntero(rc.P.DivisorDiasMes)
	if err != nil {
		return err
	}
	total, dias := decimal.Zero, 0
	for _, inc := range rc.In.Incapacidades {
		for _, seg := range rc.In.Segmentos {
			ini, fin, ok := interseccion(seg.Inicio, seg.Fin, inc.Desde, inc.Hasta)
			if !ok {
				continue
			}
			parte := inc
			parte.Desde, parte.Hasta = ini, fin
			parte.DiaSerieDesde = inc.DiaSerieDesde + int32(diasEntre(inc.Desde, ini))
			vigente, err := seg.ValorDia(rc.P)
			if err != nil {
				return err
			}
			diario, err := valorDiaSegunBase(ClaveIncapacidadSalarioBase, rc.P.IncapacidadSalarioBase, vigente, inc.Promedio, rc.P, st)
			if err != nil {
				return err
			}
			sub, err := CalcularSubsidioIncapacidad(diario, parte, rc.P.SubsidiosIncapacidad, seg.Base(divisor))
			if err != nil {
				return err
			}
			total = total.Add(sub.MontoPatronal)
			dias += sub.Dias
			st.incapacidades = append(st.incapacidades, sub.Desglose...)
		}
	}
	if total.IsNegative() {
		return fmt.Errorf("subsidio de incapacidad negativo (%s)", total)
	}
	if total.IsPositive() {
		st.agregar(rc.Concepto, total, decimal.NewFromInt(int64(dias)))
	}
	return nil
}
