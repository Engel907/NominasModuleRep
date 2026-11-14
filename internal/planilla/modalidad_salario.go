package planilla

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Valores de contratos.modalidad_salario: unidad en que se pacta el salario.
const (
	ModalidadMensual = "mensual"  // monto del mes; se paga por días comerciales
	ModalidadDiario  = "diario"   // monto por día
	ModalidadPorHora = "por_hora" // monto por hora ordinaria
)

// ModalidadSalarioValida: mensual | diario | por_hora.
func ModalidadSalarioValida(m string) bool {
	return m == ModalidadMensual || m == ModalidadDiario || m == ModalidadPorHora
}

// ClaveSalarioDiarioBase: parametros_generales (texto). Qué días paga un
// salario diario:
//   - comerciales: los días comerciales con contrato del período (como el
//     salario mensual, incluidos descansos y feriados);
//   - asistencia: los días con asistencia registrada, más los feriados de
//     pago obligatorio y las vacaciones del período que no se trabajaron.
//
// Se exige solo si hay un contrato diario en el período; no se siembra.
const ClaveSalarioDiarioBase = "SALARIO_DIARIO_BASE"

const (
	SalarioDiarioComerciales = "comerciales"
	SalarioDiarioAsistencia  = "asistencia"
)

// BaseSalarioDiarioValida: comerciales | asistencia.
func BaseSalarioDiarioValida(b string) bool {
	return b == SalarioDiarioComerciales || b == SalarioDiarioAsistencia
}

// DiaAsistencia: día con asistencia registrada ('presente') y sus horas
// ordinarias.
type DiaAsistencia struct {
	Fecha time.Time
	Horas decimal.Decimal
}

// porAsistencia: el segmento se paga por lo registrado en asistencia (no por
// días comerciales).
func porAsistencia(s SegmentoSalario, p ParametrosCalculo) bool {
	switch s.Modalidad {
	case ModalidadPorHora:
		return true
	case ModalidadDiario:
		return p.SalarioDiarioBase == SalarioDiarioAsistencia
	}
	return false
}

// horasDiariasJornada: horas ordinarias de un día de la jornada
// (parametros_jornada.horas_diarias_ordinarias).
func horasDiariasJornada(jornadaID int64, p ParametrosCalculo) (decimal.Decimal, error) {
	h, ok := p.HorasDiariasPorJornada[jornadaID]
	if !ok || !h.IsPositive() {
		return decimal.Zero, fmt.Errorf("la jornada id=%d no tiene horas_diarias_ordinarias vigentes en parametros_jornada", jornadaID)
	}
	return h, nil
}

// ValorDia: lo que vale un día de salario del segmento. Mensual = salario ÷
// DIVISOR_DIAS_MES; diario = salario; por hora = salario × horas diarias de la
// jornada.
func (s SegmentoSalario) ValorDia(p ParametrosCalculo) (decimal.Decimal, error) {
	switch s.Modalidad {
	case ModalidadMensual, "":
		if !p.DivisorDiasMes.IsPositive() {
			return decimal.Zero, errors.New("DIVISOR_DIAS_MES no definido para valorar el día")
		}
		return s.Salario.Div(p.DivisorDiasMes), nil
	case ModalidadDiario:
		return s.Salario, nil
	case ModalidadPorHora:
		h, err := horasDiariasJornada(s.JornadaID, p)
		if err != nil {
			return decimal.Zero, err
		}
		return s.Salario.Mul(h), nil
	}
	return decimal.Zero, fmt.Errorf("modalidad de salario desconocida %q (contrato %d)", s.Modalidad, s.ContratoID)
}

// valorHoraOrdinaria: salario y divisor con los que se valora la hora
// ordinaria (monto = salario × horas ÷ divisor). Mensual: divisor de horas
// mensuales de la jornada; diario: horas diarias de la jornada; por hora: 1.
func valorHoraOrdinaria(c ContratoVigente, p ParametrosCalculo) (salario, divisor decimal.Decimal, err error) {
	if c.JornadaID == nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("el contrato %d no tiene jornada del catálogo", c.ContratoID)
	}
	switch c.ModalidadSalario {
	case ModalidadMensual, "":
		d, ok := p.DivisorPorJornada[*c.JornadaID]
		if !ok {
			return decimal.Zero, decimal.Zero, fmt.Errorf("la jornada %q (id %d) no tiene parametros_jornada vigente", c.Jornada, *c.JornadaID)
		}
		return c.SalarioPactado, d, nil
	case ModalidadDiario:
		h, err := horasDiariasJornada(*c.JornadaID, p)
		if err != nil {
			return decimal.Zero, decimal.Zero, err
		}
		return c.SalarioPactado, h, nil
	case ModalidadPorHora:
		return c.SalarioPactado, decimal.NewFromInt(1), nil
	}
	return decimal.Zero, decimal.Zero, fmt.Errorf("modalidad de salario desconocida %q (contrato %d)", c.ModalidadSalario, c.ContratoID)
}

// unidadesAsistencia: lo que se paga de un segmento por asistencia, en su
// unidad (días o horas), y los días que representa. Cuenta lo registrado
// dentro del segmento fuera de incapacidades; suma los feriados de pago
// obligatorio no trabajados y, si PAGO_VACACIONES no paga el disfrute, los
// días de vacaciones aprobadas (a horas diarias de la jornada si es por hora).
func unidadesAsistencia(s SegmentoSalario, in CalculoEmpleadoInput, p ParametrosCalculo) (unidades decimal.Decimal, dias int, err error) {
	dentro := func(f time.Time) bool { return !f.Before(s.Inicio) && !f.After(s.Fin) }
	excluido := func(f time.Time) bool {
		for _, inc := range in.Incapacidades {
			if !f.Before(soloFecha(inc.Desde)) && !f.After(soloFecha(inc.Hasta)) {
				return true
			}
		}
		return false
	}
	enVacaciones := func(f time.Time) bool {
		for _, v := range in.VacacionesDisfrute {
			if !f.Before(soloFecha(v.Desde)) && !f.After(soloFecha(v.Hasta)) {
				return true
			}
		}
		return false
	}
	var horasDia decimal.Decimal
	if s.Modalidad == ModalidadPorHora {
		if horasDia, err = horasDiariasJornada(s.JornadaID, p); err != nil {
			return decimal.Zero, 0, err
		}
	}
	unoDia := func() decimal.Decimal {
		if s.Modalidad == ModalidadPorHora {
			return horasDia
		}
		return decimal.NewFromInt(1)
	}
	trabajados := map[time.Time]bool{}
	for _, a := range in.Asistencia {
		f := soloFecha(a.Fecha)
		if !dentro(f) || excluido(f) || !a.Horas.IsPositive() {
			continue
		}
		trabajados[f] = true
		dias++
		if s.Modalidad == ModalidadPorHora {
			unidades = unidades.Add(a.Horas)
		} else {
			unidades = unidades.Add(decimal.NewFromInt(1))
		}
	}
	for _, f := range in.FeriadosPagoObligatorio {
		f = soloFecha(f)
		if !dentro(f) || excluido(f) || trabajados[f] || enVacaciones(f) {
			continue
		}
		trabajados[f] = true
		dias++
		unidades = unidades.Add(unoDia())
	}
	if !pagaDisfrute(p.VacacionesModoPago) {
		for _, v := range in.VacacionesDisfrute {
			ini, fin, ok := interseccion(s.Inicio, s.Fin, v.Desde, v.Hasta)
			if !ok {
				continue
			}
			for f := ini; !f.After(fin); f = f.AddDate(0, 0, 1) {
				if trabajados[f] || excluido(f) {
					continue
				}
				trabajados[f] = true
				dias++
				unidades = unidades.Add(unoDia())
			}
		}
	}
	return unidades, dias, nil
}
