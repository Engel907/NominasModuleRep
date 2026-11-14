//go:build integration

package parametros_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"planillas-ccdr/internal/parametros"
)

func (f *fixtureL24) proponerBaja(t *testing.T, tabla, desde string, clave map[string]any) (parametros.Propuesta, error) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"baja": true, "filas": []map[string]any{clave}})
	if err != nil {
		t.Fatal(err)
	}
	return f.svc.Proponer(context.Background(), f.propone, parametros.NuevaPropuesta{
		Tabla: tabla, Payload: raw, FechaInicioVigencia: fecha(desde), FuenteLegal: "Acuerdo de prueba",
	})
}

func (f *fixtureL24) aprobar(t *testing.T, p parametros.Propuesta, err error) error {
	t.Helper()
	if err != nil {
		t.Fatalf("proponer: %v", err)
	}
	_, err = f.svc.Aprobar(context.Background(), f.aprueba, p.ID)
	return err
}

func TestConfiguracionVersionadaPorPropuesta(t *testing.T) {
	f := nuevoFixtureL24(t)
	ctx := context.Background()
	salBase := f.id(t, `SELECT id FROM conceptos_salariales WHERE codigo = 'SAL_BASE'`)
	comisiones := f.id(t, `SELECT id FROM conceptos_salariales WHERE codigo = 'COMISIONES'`)
	regimen := f.id(t, `SELECT regimen_laboral_id FROM regimen_concepto_aplicable WHERE concepto_salarial_id = ? AND es_vigente = 1`, comisiones)

	t.Run("fórmula reemplazada", func(t *testing.T) {
		p, err := f.proponer(t, "concepto_salarial_formula", "2027-01-01",
			map[string]any{"concepto_salarial_id": salBase, "monto_fijo": nil, "expresion_formula": `{"tipo":"referencia_directa"}`})
		if err := f.aprobar(t, p, err); err != nil {
			t.Fatal(err)
		}
		var expresion string
		if err := f.db.QueryRow(`SELECT expresion_formula FROM concepto_salarial_formula WHERE concepto_salarial_id = ? AND es_vigente = 1`, salBase).Scan(&expresion); err != nil || expresion != `{"tipo":"referencia_directa"}` {
			t.Fatalf("fórmula vigente %q (%v)", expresion, err)
		}
	})

	t.Run("fórmula que el motor no ejecuta", func(t *testing.T) {
		_, err := f.proponer(t, "concepto_salarial_formula", "2027-02-01",
			map[string]any{"concepto_salarial_id": salBase, "monto_fijo": nil, "expresion_formula": `{"tipo":"carga_social","lado":"ambos"}`})
		if !errors.Is(err, parametros.ErrPropuestaInvalida) {
			t.Fatalf("se esperaba ErrPropuestaInvalida, se obtuvo %v", err)
		}
	})

	t.Run("baja de un concepto del régimen", func(t *testing.T) {
		clave := map[string]any{"regimen_laboral_id": regimen, "concepto_salarial_id": comisiones}
		p, err := f.proponerBaja(t, "regimen_concepto_aplicable", "2027-01-01", clave)
		if err := f.aprobar(t, p, err); err != nil {
			t.Fatal(err)
		}
		var abiertas int
		if err := f.db.QueryRow(`SELECT COUNT(*) FROM regimen_concepto_aplicable WHERE concepto_salarial_id = ? AND fecha_fin_vigencia IS NULL`, comisiones).Scan(&abiertas); err != nil || abiertas != 0 {
			t.Fatalf("filas abiertas = %d (%v)", abiertas, err)
		}
		vigentes2026, err := f.svc.Vigentes(ctx, "regimen_concepto_aplicable", fecha("2026-12-31"))
		if err != nil {
			t.Fatal(err)
		}
		vigentes2027, err := f.svc.Vigentes(ctx, "regimen_concepto_aplicable", fecha("2027-01-01"))
		if err != nil || len(vigentes2027) != len(vigentes2026)-1 {
			t.Fatalf("vigentes 2026 = %d, 2027 = %d (%v)", len(vigentes2026), len(vigentes2027), err)
		}
		p, err = f.proponerBaja(t, "regimen_concepto_aplicable", "2027-06-01", clave)
		if err := f.aprobar(t, p, err); !errors.Is(err, parametros.ErrSerieSinVigente) {
			t.Fatalf("segunda baja: se esperaba ErrSerieSinVigente, se obtuvo %v", err)
		}
	})

	t.Run("referencia inexistente", func(t *testing.T) {
		p, err := f.proponer(t, "orden_calculo_concepto", "2027-01-01", map[string]any{"concepto_salarial_id": 999999, "orden": 10})
		if err := f.aprobar(t, p, err); !errors.Is(err, parametros.ErrPropuestaInvalida) {
			t.Fatalf("se esperaba ErrPropuestaInvalida, se obtuvo %v", err)
		}
	})

	t.Run("mapeo contable reemplazado", func(t *testing.T) {
		var concepto, programa, asiento, cuenta int64
		var movimiento, proyecto string
		if err := f.db.QueryRow(`SELECT concepto_salarial_id, programa_presupuestario_id, tipo_asiento_id, cuenta_contable_id, tipo_movimiento, proyecto
FROM mapeo_concepto_cuenta WHERE es_vigente = 1 ORDER BY id LIMIT 1`).Scan(&concepto, &programa, &asiento, &cuenta, &movimiento, &proyecto); err != nil {
			t.Fatal(err)
		}
		p, err := f.proponer(t, "mapeo_concepto_cuenta", "2027-01-01", map[string]any{
			"concepto_salarial_id": concepto, "programa_presupuestario_id": programa, "tipo_asiento_id": asiento,
			"cuenta_contable_id": cuenta, "tipo_movimiento": movimiento, "proyecto": "NUEVO"})
		if err := f.aprobar(t, p, err); err != nil {
			t.Fatal(err)
		}
		var vigente string
		if err := f.db.QueryRow(`SELECT proyecto FROM mapeo_concepto_cuenta WHERE concepto_salarial_id = ? AND programa_presupuestario_id = ? AND tipo_asiento_id = ? AND es_vigente = 1`,
			concepto, programa, asiento).Scan(&vigente); err != nil || vigente != "NUEVO" {
			t.Fatalf("proyecto vigente %q (%v)", vigente, err)
		}
	})
}

func TestParametrosLegalesDeLiquidacionPorPropuesta(t *testing.T) {
	f := nuevoFixtureL24(t)
	regimen := f.id(t, `SELECT regimen_laboral_id FROM parametros_cesantia_escala WHERE es_vigente = 1 LIMIT 1`)
	clasificacion := f.id(t, `SELECT clasificacion_puesto_id FROM parametros_salario_minimo WHERE es_vigente = 1 LIMIT 1`)
	casos := []struct {
		tabla string
		filas []map[string]any
	}{
		{"parametros_cesantia_escala", []map[string]any{
			{"regimen_laboral_id": regimen, "anios_servicio_desde": "0", "anios_servicio_hasta": "1", "dias_por_anio": "14", "tope_anios": "8"},
			{"regimen_laboral_id": regimen, "anios_servicio_desde": "1", "anios_servicio_hasta": nil, "dias_por_anio": "20", "tope_anios": "8"}}},
		{"parametros_preaviso_escala", []map[string]any{
			{"regimen_laboral_id": regimen, "antiguedad_meses_desde": 0, "antiguedad_meses_hasta": 3, "dias_preaviso": 0},
			{"regimen_laboral_id": regimen, "antiguedad_meses_desde": 3, "antiguedad_meses_hasta": nil, "dias_preaviso": 30}}},
		{"parametros_vacaciones_escala", []map[string]any{
			{"anios_servicio_desde": "0", "anios_servicio_hasta": "5", "dias_vacaciones": "15"},
			{"anios_servicio_desde": "5", "anios_servicio_hasta": nil, "dias_vacaciones": "26"}}},
		{"parametros_salario_minimo", []map[string]any{{"clasificacion_puesto_id": clasificacion, "monto_mensual": "390000.00"}}},
		{"parametros_tipos_embargo", []map[string]any{{"codigo": "EMBARGO_CCSS", "nombre": "Embargo CCSS", "prioridad_legal": 4,
			"es_pension_alimentaria": false, "tope_porcentaje_liquido": nil}}},
	}
	for _, c := range casos {
		t.Run(c.tabla, func(t *testing.T) {
			p, err := f.proponer(t, c.tabla, "2027-01-01", c.filas...)
			if err := f.aprobar(t, p, err); err != nil {
				t.Fatal(err)
			}
			nuevas, err := f.svc.Vigentes(context.Background(), c.tabla, fecha("2027-01-01"))
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for _, fila := range nuevas {
				if *fila["fecha_inicio_vigencia"] == "2027-01-01" {
					n++
				}
			}
			if n != len(c.filas) {
				t.Fatalf("filas nuevas = %d, esperadas %d", n, len(c.filas))
			}
		})
	}
}
