package planilla

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
)

func paramDecimal(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func rentaHasta(s string) decimal.NullDecimal {
	return decimal.NullDecimal{Decimal: paramDecimal(s), Valid: true}
}

func escalaOK() []dto.TramoRentaVigenteRow {
	return []dto.TramoRentaVigenteRow{
		{OrdenTramo: 1, MontoDesde: paramDecimal("0"), MontoHasta: rentaHasta("918000"), Porcentaje: paramDecimal("0")},
		{OrdenTramo: 2, MontoDesde: paramDecimal("918000"), MontoHasta: rentaHasta("1347000"), Porcentaje: paramDecimal("10")},
		{OrdenTramo: 3, MontoDesde: paramDecimal("1347000"), MontoHasta: rentaHasta("2364000"), Porcentaje: paramDecimal("15")},
		{OrdenTramo: 4, MontoDesde: paramDecimal("2364000"), MontoHasta: rentaHasta("4727000"), Porcentaje: paramDecimal("20")},
		{OrdenTramo: 5, MontoDesde: paramDecimal("4727000"), Porcentaje: paramDecimal("25")},
	}
}

func TestValidarTramosRenta(t *testing.T) {
	if err := ValidarTramosRenta(escalaOK()); err != nil {
		t.Fatalf("escala válida rechazada: %v", err)
	}
	casos := map[string]func([]dto.TramoRentaVigenteRow){
		"hueco":            func(r []dto.TramoRentaVigenteRow) { r[2].MontoDesde = paramDecimal("1400000") },
		"solape":           func(r []dto.TramoRentaVigenteRow) { r[2].MontoDesde = paramDecimal("1300000") },
		"no inicia en 0":   func(r []dto.TramoRentaVigenteRow) { r[0].MontoDesde = paramDecimal("1") },
		"ultimo cerrado":   func(r []dto.TramoRentaVigenteRow) { r[4].MontoHasta = rentaHasta("9999999") },
		"abierto no final": func(r []dto.TramoRentaVigenteRow) { r[1].MontoHasta = decimal.NullDecimal{} },
		"orden repetido":   func(r []dto.TramoRentaVigenteRow) { r[1].OrdenTramo = 1 },
	}
	for nombre, mut := range casos {
		t.Run(nombre, func(t *testing.T) {
			r := escalaOK()
			mut(r)
			if err := ValidarTramosRenta(r); !errors.Is(err, ErrTramosRentaInvalidos) {
				t.Fatalf("se esperaba ErrTramosRentaInvalidos, llegó %v", err)
			}
		})
	}
}
