package planilla

import (
	"context"
	"testing"
)

func repoConVacaciones() fakeMotorRepo {
	r := repoBase()
	r.conceptos = append(r.conceptos, fila(9, "PAGO_VACACIONES", "ingreso", 90, `{"tipo":"vacaciones"}`, true, true))
	return r
}

func inputVacaciones() CalculoEmpleadoInput {
	in := inputTope("900000")
	in.Segmentos[0].Inicio, in.Segmentos[0].Fin = fecha("2026-03-01"), fecha("2026-03-31")
	in.VacacionesDisfrute = []RangoVacaciones{{SolicitudID: 3, Desde: fecha("2026-03-10"), Hasta: fecha("2026-03-14")}}
	return in
}

func lineaDe(r *CalculoEmpleadoDetalle, codigo string) string {
	for _, l := range r.DesglosePorConcepto {
		if l.Codigo == codigo {
			return l.Monto.String()
		}
	}
	return ""
}

// PAGO_VACACIONES según VACACIONES_MODO_PAGO.
func TestPagoVacacionesSegunModo(t *testing.T) {
	ctx := context.Background()

	t.Run("compensación: paga los días compensados; el disfrute no descuenta", func(t *testing.T) {
		p := params()
		p.VacacionesModoPago = VacacionesPagoCompensacion
		in := inputVacaciones()
		in.VacacionesCompensacion = []CompensacionVacacionesInput{{MovimientoID: 41, Dias: d("2.5")}}
		r, err := NewMotorEmpleado(repoConVacaciones()).CalcularEmpleado(ctx, in, p)
		if err != nil {
			t.Fatal(err)
		}
		if lineaDe(r, "SAL_BASE") != "900000" || lineaDe(r, "PAGO_VACACIONES") != "75000" ||
			len(r.VacacionesCompensadas) != 1 || r.VacacionesCompensadas[0] != 41 {
			t.Errorf("SAL_BASE %s, PAGO_VACACIONES %s, movimientos %v", lineaDe(r, "SAL_BASE"), lineaDe(r, "PAGO_VACACIONES"), r.VacacionesCompensadas)
		}
	})

	t.Run("disfrute: los días aprobados salen de SAL_BASE y los paga PAGO_VACACIONES", func(t *testing.T) {
		p := params()
		p.VacacionesModoPago = VacacionesPagoDisfrute
		r, err := NewMotorEmpleado(repoConVacaciones()).CalcularEmpleado(ctx, inputVacaciones(), p)
		if err != nil {
			t.Fatal(err)
		}
		if lineaDe(r, "SAL_BASE") != "750000" || lineaDe(r, "PAGO_VACACIONES") != "150000" || !r.TotalIngresos.Equal(d("900000")) {
			t.Errorf("SAL_BASE %s, PAGO_VACACIONES %s, ingresos %s", lineaDe(r, "SAL_BASE"), lineaDe(r, "PAGO_VACACIONES"), r.TotalIngresos)
		}
	})

	t.Run("ambos", func(t *testing.T) {
		p := params()
		p.VacacionesModoPago = VacacionesPagoAmbos
		in := inputVacaciones()
		in.VacacionesCompensacion = []CompensacionVacacionesInput{{MovimientoID: 41, Dias: d("1")}}
		r, err := NewMotorEmpleado(repoConVacaciones()).CalcularEmpleado(ctx, in, p)
		if err != nil {
			t.Fatal(err)
		}
		if lineaDe(r, "PAGO_VACACIONES") != "180000" {
			t.Errorf("PAGO_VACACIONES %s, esperado 180000", lineaDe(r, "PAGO_VACACIONES"))
		}
	})

	t.Run("errores explícitos", func(t *testing.T) {
		m := NewMotorEmpleado(repoConVacaciones())
		if _, err := m.CalcularEmpleado(ctx, inputVacaciones(), params()); err == nil {
			t.Error("vacaciones en el período sin VACACIONES_MODO_PAGO debe dar error")
		}
		p := params()
		p.VacacionesModoPago = VacacionesPagoDisfrute
		in := inputTope("900000")
		in.VacacionesCompensacion = []CompensacionVacacionesInput{{MovimientoID: 41, Dias: d("1")}}
		if _, err := m.CalcularEmpleado(ctx, in, p); err == nil {
			t.Error("compensación pendiente con modo disfrute debe dar error (no se deja de pagar en silencio)")
		}
		in = inputVacaciones()
		in.Incapacidades = []IncapacidadInput{{IncapacidadID: 1, EntidadEmisoraID: 1, Desde: fecha("2026-03-14"), Hasta: fecha("2026-03-15"), DiaSerieDesde: 1}}
		in.DiasTrabajados = d("28")
		if _, err := m.CalcularEmpleado(ctx, in, p); err == nil {
			t.Error("vacaciones solapadas con una incapacidad deben dar error")
		}
		sinConcepto := NewMotorEmpleado(repoBase())
		in = inputTope("900000")
		in.VacacionesCompensacion = []CompensacionVacacionesInput{{MovimientoID: 41, Dias: d("1")}}
		p.VacacionesModoPago = VacacionesPagoCompensacion
		if _, err := sinConcepto.CalcularEmpleado(ctx, in, p); err == nil {
			t.Error("compensación sin PAGO_VACACIONES aplicable debe dar error")
		}
	})
}
