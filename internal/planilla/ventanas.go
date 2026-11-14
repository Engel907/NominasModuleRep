package planilla

import (
	"context"
	"fmt"
	"time"
)

// EnterosLector: subconjunto de parametros.Generales que usan las ventanas.
type EnterosLector interface {
	Entero(ctx context.Context, clave string, fecha time.Time) (int64, error)
}

// VentanaAcumulado: fechas de un acumulado anual, todas desde
// parametros_generales (sembradas en 000118; nada escrito en Go).
//   - Cálculo: rango de períodos cuya base se suma (y fechas del período que
//     paga el acumulado).
//   - Ejecución: cuándo se puede crear ese período.
//   - PagoLimite: plazo legal de pago, si la clase lo define (aguinaldo).
type VentanaAcumulado struct {
	Clase                          ClaseTipoPlanilla
	Anio                           int
	CalculoDesde, CalculoHasta     time.Time
	EjecucionDesde, EjecucionHasta time.Time
	PagoLimite                     *time.Time
}

// EnEjecucion: hoy está dentro de la ventana de ejecución (inclusive).
func (v VentanaAcumulado) EnEjecucion(hoy time.Time) bool {
	h := soloFecha(hoy)
	return !h.Before(v.EjecucionDesde) && !h.After(v.EjecucionHasta)
}

type Ventanas struct{ g EnterosLector }

func NewVentanas(g EnterosLector) *Ventanas { return &Ventanas{g: g} }

// Aguinaldo: claves AGUINALDO_PERIODO_*, AGUINALDO_VENTANA_*, AGUINALDO_PAGO_LIMITE_*.
func (v *Ventanas) Aguinaldo(ctx context.Context, anio int, hoy time.Time) (VentanaAcumulado, error) {
	return v.leer(ctx, ClaseAguinaldo, "AGUINALDO", anio, hoy, true)
}

// SalarioEscolar: claves SALESC_PERIODO_*, SALESC_VENTANA_* (sin plazo de pago).
func (v *Ventanas) SalarioEscolar(ctx context.Context, anio int, hoy time.Time) (VentanaAcumulado, error) {
	return v.leer(ctx, ClaseSalarioEscolar, "SALESC", anio, hoy, false)
}

// Ventana despacha por clase.
func (v *Ventanas) Ventana(ctx context.Context, clase ClaseTipoPlanilla, anio int, hoy time.Time) (VentanaAcumulado, error) {
	switch clase {
	case ClaseAguinaldo:
		return v.Aguinaldo(ctx, anio, hoy)
	case ClaseSalarioEscolar:
		return v.SalarioEscolar(ctx, anio, hoy)
	}
	return VentanaAcumulado{}, ErrClaseNoAcumulable
}

// leer resuelve los parámetros vigentes a `hoy`. Un parámetro faltante → 422
// (parametros_incompletos); una fecha imposible (31 de febrero) → error.
func (v *Ventanas) leer(ctx context.Context, clase ClaseTipoPlanilla, prefijo string, anio int, hoy time.Time, conPago bool) (VentanaAcumulado, error) {
	if anio < 1900 || anio > 9999 {
		return VentanaAcumulado{}, fmt.Errorf("planilla: año %d inválido", anio)
	}
	f := soloFecha(hoy)
	fecha := func(grupo string) (time.Time, error) {
		var vals [3]int64
		for i, suf := range []string{"MES", "DIA", "ANIO_OFFSET"} {
			x, err := v.g.Entero(ctx, prefijo+"_"+grupo+"_"+suf, f)
			if err != nil {
				return time.Time{}, errorParametrosVentana(err)
			}
			vals[i] = x
		}
		mes, dia, off := int(vals[0]), int(vals[1]), int(vals[2])
		t := time.Date(anio+off, time.Month(mes), dia, 0, 0, 0, 0, time.UTC)
		if mes < 1 || mes > 12 || t.Day() != dia || int(t.Month()) != mes {
			return time.Time{}, fmt.Errorf("planilla: %s_%s define una fecha inválida (mes %d, día %d)", prefijo, grupo, mes, dia)
		}
		return t, nil
	}
	out := VentanaAcumulado{Clase: clase, Anio: anio}
	var err error
	if out.CalculoDesde, err = fecha("PERIODO_INICIO"); err != nil {
		return VentanaAcumulado{}, err
	}
	if out.CalculoHasta, err = fecha("PERIODO_FIN"); err != nil {
		return VentanaAcumulado{}, err
	}
	if out.EjecucionDesde, err = fecha("VENTANA_INICIO"); err != nil {
		return VentanaAcumulado{}, err
	}
	if out.EjecucionHasta, err = fecha("VENTANA_FIN"); err != nil {
		return VentanaAcumulado{}, err
	}
	if conPago {
		p, err := fecha("PAGO_LIMITE")
		if err != nil {
			return VentanaAcumulado{}, err
		}
		out.PagoLimite = &p
	}
	if out.CalculoHasta.Before(out.CalculoDesde) || out.EjecucionHasta.Before(out.EjecucionDesde) {
		return VentanaAcumulado{}, fmt.Errorf("planilla: las ventanas de %s %d están invertidas", clase, anio)
	}
	return out, nil
}
