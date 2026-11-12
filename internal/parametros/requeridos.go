package parametros

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Claves de parametros_generales que el motor exige vigentes (L4).
const (
	ClaveDivisorDiasMes            = "DIVISOR_DIAS_MES"
	ClaveSalarioMinimoInembargable = "SALARIO_MINIMO_INEMBARGABLE"
)

// Requerido describe un parámetro general sin el cual el motor no puede calcular.
type Requerido struct{ Clave, Tipo, Descripcion string }

// Requeridos lista los parámetros generales obligatorios.
// HORAS_MENSUALES_ORDINARIAS está deprecada desde L12: el divisor de la hora
// sale de parametros_jornada (por jornada) y nadie la lee.
func Requeridos() []Requerido {
	return []Requerido{
		{
			Clave:       ClaveDivisorDiasMes,
			Tipo:        TipoEntero,
			Descripcion: "Divisor para obtener el salario diario a partir del mensual",
		},
		{
			Clave:       ClaveSalarioMinimoInembargable,
			Tipo:        TipoDecimal,
			Descripcion: "Salario mínimo inembargable (monto global, D13 del ADR-001) para el cálculo de embargos",
		},
	}
}

// Faltantes devuelve los requeridos sin valor vigente a la fecha, en el orden de
// Requeridos(). Solo ErrParametroNoConfigurado cuenta como faltante: un tipo
// incorrecto, un valor ilegible o un fallo de lectura se devuelven como error
// y nunca se ocultan.
func (g *Generales) Faltantes(ctx context.Context, fecha time.Time) ([]Requerido, error) {
	var faltan []Requerido
	for _, r := range Requeridos() {
		err := g.leerRequerido(ctx, r, fecha)
		if err == nil {
			continue
		}
		var nc ErrParametroNoConfigurado
		if errors.As(err, &nc) {
			faltan = append(faltan, r)
			continue
		}
		return nil, err
	}
	return faltan, nil
}

// leerRequerido usa el accesor tipado: así también valida tipo y formato del valor.
func (g *Generales) leerRequerido(ctx context.Context, r Requerido, fecha time.Time) error {
	switch r.Tipo {
	case TipoEntero:
		valor, err := g.Entero(ctx, r.Clave, fecha)
		if err != nil {
			return err
		}
		if valor <= 0 {
			return fmt.Errorf("%w: %q debe ser > 0 (es %d)", ErrValorInvalido, r.Clave, valor)
		}
		return nil
	case TipoDecimal:
		valor, err := g.Decimal(ctx, r.Clave, fecha)
		if err != nil {
			return err
		}
		if !valor.IsPositive() {
			return fmt.Errorf("%w: %q debe ser > 0 (es %s)", ErrValorInvalido, r.Clave, valor)
		}
		return nil
	default:
		return fmt.Errorf("parametros: tipo %q no soportado para el requerido %q", r.Tipo, r.Clave)
	}
}
