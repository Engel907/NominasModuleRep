//go:build integration

package parametros_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"planillas-ccdr/internal/liquidacion"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/testutil"
)

type fixtureL24 struct {
	db               *sql.DB
	propone, aprueba int64
	svc              *parametros.Propuestas
	gen              *parametros.Generales
}

func nuevoFixtureL24(t *testing.T) *fixtureL24 {
	t.Helper()
	db := testutil.NuevaBD(t)
	f := &fixtureL24{db: db}
	for i, u := range []*int64{&f.propone, &f.aprueba} {
		res, err := db.Exec(`INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES (?, 'Usuario L24', ?, 'x')`,
			fmt.Sprintf("l24-%d", i), fmt.Sprintf("l24-%d@example.test", i))
		if err != nil {
			t.Fatal(err)
		}
		*u, _ = res.LastInsertId()
	}
	f.gen = parametros.NewGenerales(parametros.NewRepositorioSQL(db), time.Hour)
	f.svc = parametros.NewPropuestas(db, parametros.RegistroEspecs(parametros.Validadores{Renta: planilla.ValidarTramosRenta, Formula: liquidacion.ValidarExpresionFormula}), f.gen.Invalidar)
	return f
}

func (f *fixtureL24) id(t *testing.T, q string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := f.db.QueryRow(q, args...).Scan(&id); err != nil {
		t.Fatalf("%v\n%s", err, q)
	}
	return id
}

func (f *fixtureL24) proponer(t *testing.T, tabla, desde string, filas ...map[string]any) (parametros.Propuesta, error) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"filas": filas})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := time.Parse("2006-01-02", desde)
	return f.svc.Proponer(context.Background(), f.propone, parametros.NuevaPropuesta{
		Tabla: tabla, Payload: raw, FechaInicioVigencia: d, FuenteLegal: "Decreto de prueba",
	})
}

func fecha(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }

// Ejecuta las variantes de SQL dinámico de las propuestas con cada tabla de la
// lista blanca.
func TestAplicaCadaTabla(t *testing.T) {
	f := nuevoFixtureL24(t)
	ctx := context.Background()
	if v, err := f.gen.Entero(ctx, "AGUINALDO_MESES", fecha("2027-01-01")); err != nil || v != 12 {
		t.Fatalf("AGUINALDO_MESES previo: %d %v", v, err)
	}
	ccssPat := f.id(t, `SELECT id FROM conceptos_salariales WHERE codigo = 'CCSS_PAT'`)
	conyuge := f.id(t, `SELECT id FROM tipos_credito_fiscal_renta WHERE codigo = 'CONYUGE'`)
	diurna := f.id(t, `SELECT id FROM tipos_hora_extra WHERE codigo = 'DIURNA'`)
	sem := f.id(t, `SELECT id FROM regimenes_ccss WHERE codigo = 'SEM'`)
	entidad := f.id(t, `SELECT entidad_emisora_id FROM parametros_subsidios_incapacidad WHERE es_vigente = 1 ORDER BY id LIMIT 1`)
	jornada := f.id(t, `SELECT id FROM jornadas WHERE codigo = 'MIXTA'`)

	casos := []struct {
		tabla string
		filas []map[string]any
	}{
		{"parametros_generales", []map[string]any{{"clave": "AGUINALDO_MESES", "valor": "13", "tipo": "entero", "descripcion": "prueba"}}},
		{"parametros_cargas_sociales", []map[string]any{{"concepto_salarial_id": ccssPat, "rubro": "SEM_IVM_PRUEBA", "porcentaje_patronal": "27.0000", "porcentaje_obrero": "0"}}},
		{"parametros_tramos_renta", []map[string]any{
			{"orden_tramo": 1, "monto_desde": "0", "monto_hasta": "1000000", "porcentaje": "0"},
			{"orden_tramo": 2, "monto_desde": "1000000", "monto_hasta": nil, "porcentaje": "15"}}},
		{"parametros_creditos_fiscales_renta", []map[string]any{{"tipo_credito_fiscal_id": conyuge, "monto_credito": "2500"}}},
		{"parametros_horas_extra", []map[string]any{{"tipo_hora_extra_id": diurna, "factor_recargo": "1.5000"}}},
		{"parametros_base_minima_contributiva", []map[string]any{{"regimen_ccss_id": sem, "monto": "250000"}}},
		{"parametros_embargo_escala", []map[string]any{
			{"desde_multiplo": "0", "hasta_multiplo": "3", "porcentaje_maximo": "12.5"},
			{"desde_multiplo": "3", "hasta_multiplo": nil, "porcentaje_maximo": "25"}}},
		{"parametros_subsidios_incapacidad", []map[string]any{
			{"entidad_emisora_id": entidad, "dia_desde": 1, "dia_hasta": 3, "porcentaje_patronal": "50", "porcentaje_entidad": "50"},
			{"entidad_emisora_id": entidad, "dia_desde": 4, "dia_hasta": nil, "porcentaje_patronal": "40", "porcentaje_entidad": "60"}}},
		{"parametros_jornada", []map[string]any{{"jornada_id": jornada, "horas_diarias_ordinarias": "7", "divisor_horas_mensuales": "210"}}},
		{"parametros_franja_nocturna", []map[string]any{{"hora_inicio": "19:00", "hora_fin": "05:00"}}},
		{"parametros_motivos_salida", []map[string]any{{"motivo_salida": "fallecimiento", "paga_preaviso": false, "paga_cesantia": true,
			"paga_vacaciones": true, "paga_aguinaldo": true, "paga_salario_escolar": true, "observacion": "Art. 85 CT"}}},
		{"parametros_salario_escolar", []map[string]any{{"porcentaje": "0.0819"}}},
	}
	for _, c := range casos {
		t.Run(c.tabla, func(t *testing.T) {
			antes, err := f.svc.Vigentes(ctx, c.tabla, fecha("2026-12-31"))
			if err != nil {
				t.Fatal(err)
			}
			p, err := f.proponer(t, c.tabla, "2027-01-01", c.filas...)
			if err != nil {
				t.Fatalf("proponer: %v", err)
			}
			if p.Estado != parametros.EstadoPropuestaPendiente {
				t.Fatalf("estado %s", p.Estado)
			}
			if _, err := f.svc.Aprobar(ctx, f.aprueba, p.ID); err != nil {
				t.Fatalf("aprobar: %v", err)
			}
			nuevas, err := f.svc.Vigentes(ctx, c.tabla, fecha("2027-01-01"))
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
				t.Fatalf("filas nuevas vigentes = %d, esperadas %d", n, len(c.filas))
			}
			despues, err := f.svc.Vigentes(ctx, c.tabla, fecha("2026-12-31"))
			if err != nil || len(despues) != len(antes) {
				t.Fatalf("vigentes al 2026-12-31: antes %d, después %d (%v)", len(antes), len(despues), err)
			}
		})
	}
	if v, err := f.gen.Entero(ctx, "AGUINALDO_MESES", fecha("2027-01-01")); err != nil || v != 13 {
		t.Fatalf("la aprobación debe invalidar la caché: AGUINALDO_MESES = %d (%v)", v, err)
	}
	if v, err := f.gen.Entero(ctx, "AGUINALDO_MESES", fecha("2026-06-01")); err != nil || v != 12 {
		t.Fatalf("la vigencia anterior debe conservarse: %d (%v)", v, err)
	}
}

func TestCuatroOjosYReglas(t *testing.T) {
	f := nuevoFixtureL24(t)
	ctx := context.Background()
	p, err := f.proponer(t, "parametros_generales", "2027-01-01",
		map[string]any{"clave": "LIQ_PRESTAMOS_MODO", "valor": "cuota", "tipo": "texto", "descripcion": "prueba"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Aprobar(ctx, f.propone, p.ID); !errors.Is(err, parametros.ErrCuatroOjos) {
		t.Fatalf("misma persona aprueba: %v", err)
	}
	if _, err := f.svc.Rechazar(ctx, f.propone, p.ID, "no"); !errors.Is(err, parametros.ErrCuatroOjos) {
		t.Fatalf("misma persona rechaza: %v", err)
	}
	if _, err := f.db.Exec(`UPDATE parametros_propuestas SET estado = 'aplicada', resuelto_por = propuesto_por, fecha_resolucion = NOW() WHERE id = ?`, p.ID); err == nil {
		t.Fatal("el CHECK de cuatro ojos debe rechazar resuelto_por = propuesto_por")
	}
	if _, err := f.svc.Rechazar(ctx, f.aprueba, p.ID, ""); !errors.Is(err, parametros.ErrPropuestaInvalida) {
		t.Fatalf("rechazo sin motivo: %v", err)
	}
	r, err := f.svc.Rechazar(ctx, f.aprueba, p.ID, "falta acuerdo de RR. HH.")
	if err != nil || r.Estado != parametros.EstadoPropuestaRechazada {
		t.Fatalf("rechazar: %+v %v", r, err)
	}
	if _, err := f.svc.Aprobar(ctx, f.aprueba, p.ID); !errors.Is(err, parametros.ErrPropuestaResuelta) {
		t.Fatalf("aprobar una rechazada: %v", err)
	}

	_, err = f.proponer(t, "parametros_tramos_renta", "2027-01-01",
		map[string]any{"orden_tramo": 1, "monto_desde": "0", "monto_hasta": "100", "porcentaje": "0"},
		map[string]any{"orden_tramo": 2, "monto_desde": "200", "monto_hasta": nil, "porcentaje": "10"})
	if !errors.Is(err, parametros.ErrPropuestaInvalida) {
		t.Fatalf("renta con hueco: %v", err)
	}

	p2, err := f.proponer(t, "parametros_base_minima_contributiva", "2026-01-01",
		map[string]any{"regimen_ccss_id": f.id(t, `SELECT id FROM regimenes_ccss WHERE codigo = 'SEM'`), "monto": "1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Aprobar(ctx, f.aprueba, p2.ID); !errors.Is(err, parametros.ErrVigenciaNoPosterior) {
		t.Fatalf("vigencia no posterior: %v", err)
	}
	var estado string
	if err := f.db.QueryRow(`SELECT estado FROM parametros_propuestas WHERE id = ?`, p2.ID).Scan(&estado); err != nil || estado != "pendiente" {
		t.Fatalf("la propuesta fallida debe seguir pendiente: %s (%v)", estado, err)
	}
	lista, total, err := f.svc.Listar(ctx, "pendiente", "", 10, 0)
	if err != nil || total != 1 || len(lista) != 1 || lista[0].ID != p2.ID {
		t.Fatalf("listar pendientes: %d %+v %v", total, lista, err)
	}
}
