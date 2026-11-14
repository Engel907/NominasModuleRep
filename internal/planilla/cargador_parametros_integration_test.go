//go:build integration

package planilla_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/planilla/infra"
	"planillas-ccdr/internal/testutil"
)

// montoFixtureSalarioMinimo es un valor de PRUEBA, no un valor legal.
const montoFixtureSalarioMinimo = "1000000"

// sembrarSalarioMinimoFixture inserta SALARIO_MINIMO_INEMBARGABLE, que ninguna
// migración siembra (no se inventan valores legales).
func sembrarSalarioMinimoFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
INSERT INTO parametros_generales
	(clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
VALUES
	(?, ?, 'decimal', 'Fixture de prueba', 'FIXTURE DE PRUEBA - NO ES UN VALOR LEGAL', '2026-01-01', NULL, 1)`,
		parametros.ClaveSalarioMinimoInembargable, montoFixtureSalarioMinimo)
	if err != nil {
		t.Fatalf("sembrando fixture de %s: %v", parametros.ClaveSalarioMinimoInembargable, err)
	}
}

func TestCargadorSeedsReales(t *testing.T) {
	db := testutil.NuevaBD(t)
	sembrarSalarioMinimoFixture(t, db)
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
	idConcepto := func(codigo string) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRow(`SELECT id FROM conceptos_salariales WHERE codigo = ?`, codigo).Scan(&id); err != nil {
			t.Fatalf("concepto %s: %v", codigo, err)
		}
		return id
	}
	eq("CCSS obrero", p.CargasSociales[idConcepto("CCSS_OBR")][0].PorcentajeObrero, "0.1083")
	eq("CCSS patronal", p.CargasSociales[idConcepto("CCSS_PAT")][0].PorcentajePatronal, "0.2683")
	if len(p.TramosRenta) != 5 {
		t.Fatalf("tramos renta = %d, se esperaban 5", len(p.TramosRenta))
	}
	for i, w := range []string{"0", "0.1", "0.15", "0.2", "0.25"} {
		eq("renta tramo", p.TramosRenta[i].Porcentaje, w)
	}
	if !p.TramosRenta[4].Abierto {
		t.Fatal("el último tramo de renta debe estar marcado como abierto")
	}
	eq("divisor de la jornada DIURNA", p.DivisorPorJornada[1], "240")
	eq("divisor de días del mes", p.DivisorDiasMes, "30")
	eq("salario mínimo inembargable", p.SalarioMinimoInembargable, montoFixtureSalarioMinimo)
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

func TestCargadorFallaSinSalarioMinimoInembargable(t *testing.T) {
	db := testutil.NuevaBD(t)
	fuente := infra.NewFuenteParametros(dto.New(db))
	gen := parametros.NewGenerales(parametros.NewRepositorioSQL(db), 0)
	c := planilla.NewCargadorParametros(fuente, gen)

	fin := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	_, err := c.Cargar(context.Background(), fin.AddDate(0, 0, -30), fin)
	if !errors.Is(err, planilla.ErrParametroFaltante) {
		t.Fatalf("se esperaba ErrParametroFaltante, llegó %v", err)
	}
	if !strings.Contains(err.Error(), parametros.ClaveSalarioMinimoInembargable) {
		t.Fatalf("el mensaje debe nombrar la clave faltante: %v", err)
	}
	var nc parametros.ErrParametroNoConfigurado
	if !errors.As(err, &nc) || nc.Clave != parametros.ClaveSalarioMinimoInembargable {
		t.Fatalf("el error tipado de parametros debe seguir accesible: %v", err)
	}
}

func TestCargadorRechazaHuecoRenta(t *testing.T) {
	db := testutil.NuevaBD(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(context.Background(), `
UPDATE parametros_tramos_renta
SET fecha_fin_vigencia = '2026-02-28'
WHERE fecha_fin_vigencia IS NULL`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.ExecContext(context.Background(), `
INSERT INTO parametros_tramos_renta
	(orden_tramo, monto_desde, monto_hasta, porcentaje, fecha_inicio_vigencia)
SELECT orden_tramo,
       monto_desde + IF(orden_tramo = 3, 1000, 0),
       monto_hasta,
       porcentaje,
       '2026-03-01'
FROM parametros_tramos_renta
WHERE fecha_fin_vigencia = '2026-02-28'`)
	if err != nil {
		t.Fatalf("insertar versión de prueba con hueco: %v", err)
	}

	gen := parametros.NewGenerales(parametros.NewRepositorioSQL(db), 0)
	c := planilla.NewCargadorParametros(infra.NewFuenteParametros(dto.New(tx)), gen)
	fecha := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	if _, err := c.Cargar(context.Background(), fecha.AddDate(0, 0, -30), fecha); !errors.Is(err, planilla.ErrTramosRentaInvalidos) {
		t.Fatalf("se esperaba ErrTramosRentaInvalidos por hueco, llegó %v", err)
	}
}
