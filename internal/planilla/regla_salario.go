package planilla

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// reglaSalarioBase (referencia_directa): SAL_BASE a partir de
// contratos.salario_pactado, por segmentos de vigencia y según la modalidad de
// cada segmento:
//   - mensual: salario × días comerciales ÷ DIVISOR_DIAS_MES;
//   - diario con SALARIO_DIARIO_BASE = comerciales: salario × días comerciales;
//   - diario con SALARIO_DIARIO_BASE = asistencia: salario × días pagados por
//     asistencia (unidadesAsistencia);
//   - por hora: salario × horas pagadas por asistencia.
//
// Los días de incapacidad y, en modo disfrute, los de vacaciones no se pagan
// aquí (los pagan SUB_INCAP y PAGO_VACACIONES). Un solo redondeo al final.
type reglaSalarioBase struct{}

func (reglaSalarioBase) Aplicar(rc *ContextoRegla) error {
	p, in := rc.P, rc.In
	if !p.DivisorDiasMes.IsPositive() || !p.DiasPeriodoCompleto.IsPositive() {
		return errors.New("faltan DivisorDiasMes/DiasPeriodoCompleto para calcular el salario base")
	}
	if len(in.Segmentos) == 0 {
		return errors.New("el empleado no tiene segmentos de contrato en el período")
	}
	divisor, err := divisorEntero(p.DivisorDiasMes)
	if err != nil {
		return err
	}
	if total := decimal.NewFromInt(int64(DiasSegmentos(in.Segmentos))); total.GreaterThan(p.DiasPeriodoCompleto) {
		return fmt.Errorf("los segmentos suman %s días comerciales, más que el período (%s)", total, p.DiasPeriodoCompleto)
	}
	netos, err := SegmentosNetos(in.Segmentos, in.Incapacidades, divisor)
	if err != nil {
		return err
	}
	dias := decimal.NewFromInt(int64(DiasSegmentos(netos)))
	if !in.DiasTrabajados.Equal(dias) {
		return fmt.Errorf("DiasTrabajados=%s no coincide con los días con contrato sin incapacidad (%s)", in.DiasTrabajados, dias)
	}
	pagados, err := segmentosSinVacaciones(netos, in, p, divisor)
	if err != nil {
		return err
	}
	monto, diasPagados, err := salarioBaseSegmentos(pagados, in, p)
	if err != nil {
		return err
	}
	rc.St.salarioBase = monto
	rc.St.agregar(rc.Concepto, monto, decimal.NewFromInt(int64(diasPagados)))
	return nil
}

// salarioBaseSegmentos: monto (redondeado a 2 decimales) y días pagados de
// los segmentos ya netos de incapacidad y vacaciones.
func salarioBaseSegmentos(segs []SegmentoSalario, in CalculoEmpleadoInput, p ParametrosCalculo) (decimal.Decimal, int, error) {
	mensual, otros, dias := decimal.Zero, decimal.Zero, 0
	for _, s := range segs {
		switch s.Modalidad {
		case ModalidadMensual, "":
			mensual = mensual.Add(s.Salario.Mul(decimal.NewFromInt(int64(s.Dias))))
			dias += s.Dias
		case ModalidadDiario, ModalidadPorHora:
			if s.Modalidad == ModalidadDiario && p.SalarioDiarioBase == "" {
				return decimal.Zero, 0, fmt.Errorf("%w: %s (el contrato %d tiene salario diario; comerciales | asistencia)",
					ErrParametroFaltante, ClaveSalarioDiarioBase, s.ContratoID)
			}
			if !porAsistencia(s, p) {
				otros = otros.Add(s.Salario.Mul(decimal.NewFromInt(int64(s.Dias))))
				dias += s.Dias
				continue
			}
			u, d, err := unidadesAsistencia(s, in, p)
			if err != nil {
				return decimal.Zero, 0, fmt.Errorf("contrato %d: %w", s.ContratoID, err)
			}
			otros = otros.Add(s.Salario.Mul(u))
			dias += d
		default:
			return decimal.Zero, 0, fmt.Errorf("el contrato %d tiene una modalidad de salario desconocida %q", s.ContratoID, s.Modalidad)
		}
	}
	monto := otros
	if mensual.IsPositive() {
		monto = monto.Add(mensual.DivRound(p.DivisorDiasMes, 6))
	}
	return monto.Round(dosDecimales), dias, nil
}
