package planilla

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
)

const ClaveVacacionesModoPago = "VACACIONES_MODO_PAGO"

const (
	VacacionesPagoCompensacion = "compensacion"
	VacacionesPagoDisfrute     = "disfrute"
	VacacionesPagoAmbos        = "ambos"
)

// ModoPagoVacacionesValido: valores admitidos de VACACIONES_MODO_PAGO.
func ModoPagoVacacionesValido(m string) bool {
	switch m {
	case VacacionesPagoCompensacion, VacacionesPagoDisfrute, VacacionesPagoAmbos:
		return true
	}
	return false
}

func pagaCompensacion(m string) bool {
	return m == VacacionesPagoCompensacion || m == VacacionesPagoAmbos
}
func pagaDisfrute(m string) bool { return m == VacacionesPagoDisfrute || m == VacacionesPagoAmbos }

// CompensacionVacacionesInput: movimiento 'compensacion' pendiente de pago.
type CompensacionVacacionesInput struct {
	MovimientoID int64
	Dias         decimal.Decimal // positivo
}

// RangoVacaciones: días de una solicitud aprobada dentro del período.
type RangoVacaciones struct {
	SolicitudID int64
	Desde       time.Time
	Hasta       time.Time
}

var errModoVacacionesNoConfigurado = fmt.Errorf("el empleado tiene vacaciones que pagar pero %s no está vigente (configúrelo: compensacion | disfrute | ambos)", ClaveVacacionesModoPago)

// diasVacacionesEnSegmento: días comerciales del segmento dentro de las
// vacaciones aprobadas. Todo el rango de la solicitud
// (fecha_inicio..fecha_fin, incluidos los días no hábiles que contiene) se
// paga como vacaciones en modo disfrute.
func diasVacacionesEnSegmento(s SegmentoSalario, rangos []RangoVacaciones, divisor int) int {
	n := 0
	for _, r := range rangos {
		if ini, fin, ok := interseccion(s.Inicio, s.Fin, r.Desde, r.Hasta); ok {
			n += DiasComerciales(ini, fin, divisor)
		}
	}
	return n
}

// validarVacaciones: sin solapes entre solicitudes ni con incapacidades (se
// descontaría el mismo día dos veces).
func validarVacaciones(rangos []RangoVacaciones, incs []IncapacidadInput) error {
	for i := range rangos {
		for j := i + 1; j < len(rangos); j++ {
			if _, _, ok := interseccion(rangos[i].Desde, rangos[i].Hasta, rangos[j].Desde, rangos[j].Hasta); ok {
				return fmt.Errorf("las solicitudes de vacaciones %d y %d se solapan", rangos[i].SolicitudID, rangos[j].SolicitudID)
			}
		}
		for _, inc := range incs {
			if _, _, ok := interseccion(rangos[i].Desde, rangos[i].Hasta, inc.Desde, inc.Hasta); ok {
				return fmt.Errorf("la solicitud de vacaciones %d se solapa con la incapacidad %d", rangos[i].SolicitudID, inc.IncapacidadID)
			}
		}
	}
	return nil
}

// segmentosSinVacaciones (modo disfrute): descuenta de SAL_BASE los días de
// vacaciones aprobadas; los paga PAGO_VACACIONES.
func segmentosSinVacaciones(netos []SegmentoSalario, in CalculoEmpleadoInput, p ParametrosCalculo, divisor int) ([]SegmentoSalario, error) {
	if len(in.VacacionesDisfrute) == 0 {
		return netos, nil
	}
	if p.VacacionesModoPago == "" {
		return nil, errModoVacacionesNoConfigurado
	}
	if !pagaDisfrute(p.VacacionesModoPago) {
		return netos, nil
	}
	if err := validarVacaciones(in.VacacionesDisfrute, in.Incapacidades); err != nil {
		return nil, err
	}
	out := make([]SegmentoSalario, len(netos))
	for i, s := range netos {
		s.Dias -= diasVacacionesEnSegmento(s, in.VacacionesDisfrute, divisor)
		if s.Dias < 0 {
			return nil, fmt.Errorf("vacaciones mayores que los días del segmento del contrato %d", s.ContratoID)
		}
		out[i] = s
	}
	return out, nil
}

// calcularPagoVacaciones (regla vacaciones): monto y días de PAGO_VACACIONES y
// los movimientos de compensación que quedan ligados al período.
func calcularPagoVacaciones(in CalculoEmpleadoInput, p ParametrosCalculo) (monto, dias decimal.Decimal, movimientos []int64, err error) {
	if len(in.VacacionesCompensacion) == 0 && len(in.VacacionesDisfrute) == 0 {
		return decimal.Zero, decimal.Zero, nil, nil
	}
	modo := p.VacacionesModoPago
	if modo == "" {
		return decimal.Zero, decimal.Zero, nil, errModoVacacionesNoConfigurado
	}
	if len(in.Segmentos) == 0 {
		return decimal.Zero, decimal.Zero, nil, errors.New("sin segmentos de contrato para valorar las vacaciones")
	}
	divisor, err := divisorEntero(p.DivisorDiasMes)
	if err != nil {
		return decimal.Zero, decimal.Zero, nil, err
	}
	bruto := decimal.Zero
	if len(in.VacacionesCompensacion) > 0 {
		if !pagaCompensacion(modo) {
			return decimal.Zero, decimal.Zero, nil, fmt.Errorf("hay compensaciones de vacaciones pendientes pero %s = %s", ClaveVacacionesModoPago, modo)
		}
		diario, err := in.Segmentos[len(in.Segmentos)-1].ValorDia(p)
		if err != nil {
			return decimal.Zero, decimal.Zero, nil, err
		}
		for _, c := range in.VacacionesCompensacion {
			if !c.Dias.IsPositive() {
				return decimal.Zero, decimal.Zero, nil, fmt.Errorf("compensación de vacaciones %d con días no positivos", c.MovimientoID)
			}
			bruto = bruto.Add(diario.Mul(c.Dias))
			dias = dias.Add(c.Dias)
			movimientos = append(movimientos, c.MovimientoID)
		}
	}
	if pagaDisfrute(modo) {
		for _, s := range in.Segmentos {
			n := diasVacacionesEnSegmento(s, in.VacacionesDisfrute, divisor)
			if n == 0 {
				continue
			}
			diario, err := s.ValorDia(p)
			if err != nil {
				return decimal.Zero, decimal.Zero, nil, err
			}
			bruto = bruto.Add(diario.Mul(decimal.NewFromInt(int64(n))))
			dias = dias.Add(decimal.NewFromInt(int64(n)))
		}
	}
	return bruto.Round(dosDecimales), dias, movimientos, nil
}

type aplicadorVacaciones struct{}

// NewAplicadorVacaciones: gancho AlAprobar que marca pagadas
// (pagado_en) las compensaciones ligadas al período. Idempotente.
func NewAplicadorVacaciones() GanchoAlAprobar { return aplicadorVacaciones{} }

func (aplicadorVacaciones) AlAprobar(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow) error {
	if err := q.MarcarCompensacionesVacacionesPagadas(ctx, periodo.ID); err != nil {
		return fmt.Errorf("planilla: marcando compensaciones de vacaciones del período %d: %w", periodo.ID, err)
	}
	return nil
}
