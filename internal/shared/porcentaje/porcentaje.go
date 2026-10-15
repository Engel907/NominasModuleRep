// Package porcentaje es el ÚNICO lugar del sistema donde se convierte entre
// las dos representaciones de un porcentaje (regla D3):
//
//   - "porcentaje" tal como se persiste y se muestra en BD/UI: 10.83
//   - "fracción" tal como se multiplica en el motor de cálculo: 0.1083
//
// Ningún otro paquete debe dividir ni multiplicar por 100 para convertir un
// porcentaje. Los valores se envuelven en Porcentaje y se piden con
// Fraccion() (para calcular) o Porcentaje() (para mostrar/persistir).
package porcentaje

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// ErrFueraDeRango se retorna cuando el valor no está entre 0 y 100 %.
var ErrFueraDeRango = errors.New("porcentaje: valor fuera de rango [0, 100]")

var (
	cien = decimal.NewFromInt(100)
	uno  = decimal.NewFromInt(1)
)

// Porcentaje guarda internamente la fracción (0 <= f <= 1). El valor cero de
// la estructura es el 0 %, que es válido.
type Porcentaje struct {
	fraccion decimal.Decimal
}

// DesdePorcentaje construye un Porcentaje a partir de un valor expresado en
// porcentaje (10.83 -> fracción 0.1083). Exige 0 <= d <= 100. La conversión
// es exacta: se desplaza el punto decimal, no se redondea.
func DesdePorcentaje(d decimal.Decimal) (Porcentaje, error) {
	if d.IsNegative() || d.GreaterThan(cien) {
		return Porcentaje{}, fmt.Errorf("%w: %s %%", ErrFueraDeRango, d.String())
	}
	return Porcentaje{fraccion: d.Shift(-2)}, nil
}

// DesdeFraccion construye un Porcentaje a partir de una fracción
// (0.1083 -> 10.83 %). Exige 0 <= d <= 1.
func DesdeFraccion(d decimal.Decimal) (Porcentaje, error) {
	if d.IsNegative() || d.GreaterThan(uno) {
		return Porcentaje{}, fmt.Errorf("%w: fracción %s", ErrFueraDeRango, d.String())
	}
	return Porcentaje{fraccion: d}, nil
}

// Fraccion devuelve el valor como fracción, listo para multiplicar (0.1083).
func (p Porcentaje) Fraccion() decimal.Decimal { return p.fraccion }

// Porcentaje devuelve el valor como porcentaje, para mostrar o persistir (10.83).
func (p Porcentaje) Porcentaje() decimal.Decimal { return p.fraccion.Shift(2) }

// String muestra el valor como porcentaje, ej. "10.83%".
func (p Porcentaje) String() string { return p.Porcentaje().String() + "%" }