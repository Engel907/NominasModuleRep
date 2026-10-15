//go:build integration

package planilla_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/planilla/infra"
	"planillas-ccdr/internal/testutil"
)

func TestCargadorSeedsReales(t *testing.T) {
	db := testutil.NuevaBD(t)
	fuente := infra.NewFuenteParametros(dto.New(db))
	gen := parametros.NewGenerales(parametros.NewRepositorioSQL(db), 0)
	c := planilla.NewCargadorParametros(fuente, gen)

	ini := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	fin := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	p, err := c.Cargar(context.Background(), ini, fin)
	if err != nil {
		t.Fatalf("Cargar: %v", err)
	}
	eq := func(nombre string, got decimal.Decimal, want string) {
		t.Helper()
		if !got.Equal(decimal.RequireFromString(want)) {
			t.Errorf("%s = %s, se esperaba %s", nombre, got, want)
		}
	}
	eq("CCSS obrero", p.CargaSocial.PorcentajeObrero, "0.1083")
	eq("CCSS patronal", p.CargaSocial.PorcentajePatronal, "0.2683")
	if len(p.TramosRenta) != 5 {
		t.Fatalf("tramos renta = %d, se esperaban 5", len(p.TramosRenta))
	}
	for i, w := range []string{"0", "0.1", "0.15", "0.2", "0.25"} {
		eq("renta tramo", p.TramosRenta[i].Porcentaje, w)
	}
	if !p.TramosRenta[4].Abierto {
		t.Fatal("el último tramo de renta debe estar marcado como abierto")
	}
	eq("horas mensuales ordinarias", p.HorasMensualesOrdinarias, "240")
	if len(p.FactoresHoraExtra) == 0 || len(p.SubsidiosIncapacidad) == 0 ||
		len(p.BaseMinimaContributiva) == 0 || len(p.EscalaEmbargo) == 0 {
		t.Fatal("el cargador no resolvió todos los grupos de parámetros")
	}

	var esc decimal.Decimal
	if err := db.QueryRow(`SELECT porcentaje FROM parametros_salario_escolar
		WHERE fecha_fin_vigencia IS NULL`).Scan(&esc); err != nil {
		t.Fatal(err)
	}
	eq("salario escolar", esc, "0.0833")
}

func TestCargadorRechazaHuecoRenta(t *testing.T) {
	db := testutil.NuevaBD(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(context.Background(), `
UPDATE parametros_tramos_renta
SET monto_desde = monto_desde + 1000
WHERE orden_tramo = 3 AND fecha_fin_vigencia IS NULL`)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("actualización de fixture afectó %d filas (err=%v); se esperaba 1", n, err)
	}

	gen := parametros.NewGenerales(parametros.NewRepositorioSQL(db), 0)
	c := planilla.NewCargadorParametros(infra.NewFuenteParametros(dto.New(tx)), gen)
	fecha := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	if _, err := c.Cargar(context.Background(), fecha.AddDate(0, 0, -30), fecha); !errors.Is(err, planilla.ErrTramosRentaInvalidos) {
		t.Fatalf("se esperaba ErrTramosRentaInvalidos por hueco, llegó %v", err)
	}
}
