package planilla

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Banderas de base acumulada (columnas de planilla_detalle_empleado).
const (
	BanderaBaseAguinaldo  = "base_aguinaldo"
	BanderaBaseVacaciones = "base_vacaciones"
	BanderaBaseCesantia   = "base_cesantia"
	BanderaBaseCCSS       = "base_ccss"
)

type sumadorBase interface {
	SumarBaseAcumuladaEmpleado(ctx context.Context, empleadoID int64, desde, hasta time.Time, columna string) (decimal.Decimal, error)
}

func sumarBaseAcumulada(ctx context.Context, q sumadorBase, empleadoID int64, desde, hasta time.Time, bandera string) (decimal.Decimal, error) {
	switch bandera {
	case BanderaBaseAguinaldo, BanderaBaseVacaciones, BanderaBaseCesantia:
	default:
		return decimal.Zero, fmt.Errorf("planilla: bandera de base acumulada desconocida %q", bandera)
	}
	if hasta.Before(desde) {
		return decimal.Zero, fmt.Errorf("planilla: rango de base acumulada invertido (%s > %s)", desde.Format("2006-01-02"), hasta.Format("2006-01-02"))
	}
	total, err := q.SumarBaseAcumuladaEmpleado(ctx, empleadoID, desde, hasta, bandera)
	if err != nil {
		return decimal.Zero, fmt.Errorf("planilla: empleado_id=%d, sumando %s: %w", empleadoID, bandera, err)
	}
	return total, nil
}
