//go:build integration

package planilla

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
)

func (f *fixtureCalculo) general(t *testing.T, clave, valor, tipo string) {
	t.Helper()
	mustExec(t, f.db, `
INSERT INTO parametros_generales (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, ?, 'fixture', 'FIXTURE DE PRUEBA', '2026-01-01', 1)`, clave, valor, tipo)
}

func (f *fixtureCalculo) generalDelEmpleado(t *testing.T, emp int64, clave, valor, tipo string) {
	t.Helper()
	mustExec(t, f.db, `
INSERT INTO empleado_parametros_generales (empleado_id, clave, valor, tipo, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, ?, ?, 'Adenda de contrato', '2026-01-01', ?)`, emp, clave, valor, tipo, f.aprobador)
}

func (f *fixtureCalculo) asistir(t *testing.T, emp int64, horas string, fechas ...string) {
	t.Helper()
	for _, d := range fechas {
		mustExec(t, f.db, `
INSERT INTO registro_asistencia_detalle (empleado_id, fecha, horas_ordinarias, estado, creado_por)
VALUES (?, ?, ?, 'presente', ?)`, emp, d, horas, f.preparador)
	}
}

func (f *fixtureCalculo) calcularOFallar(t *testing.T, svc *PeriodoService, p dto.PeriodoCicloRow) {
	t.Helper()
	if _, err := svc.Calcular(context.Background(), f.preparador, p.ID); err != nil {
		var ce *ErrCalculoConErrores
		if errors.As(err, &ce) {
			t.Fatalf("calcular: %+v", ce.Errores)
		}
		t.Fatalf("calcular: %v", err)
	}
}

// Salario diario y por hora: la base del diario es un parámetro general
// (para todos) que un empleado puede tener distinto (solo para él).
func TestModalidadesDeSalarioYAjustePorEmpleado(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	f.general(t, ClaveSalarioDiarioBase, SalarioDiarioComerciales, "texto")

	mustExec(t, f.db, `UPDATE contratos SET modalidad_salario = 'diario', salario_pactado = 30000 WHERE id = ?`, f.contrato[1])
	mustExec(t, f.db, `UPDATE contratos SET modalidad_salario = 'por_hora', salario_pactado = 4000 WHERE id = ?`, f.contrato[2])
	mustExec(t, f.db, `
UPDATE parametros_jornada SET fecha_fin_vigencia = '2025-12-31'
WHERE jornada_id = (SELECT id FROM jornadas WHERE codigo = 'DIURNA') AND fecha_fin_vigencia IS NULL`)
	mustExec(t, f.db, `
INSERT INTO parametros_jornada (jornada_id, horas_diarias_ordinarias, divisor_horas_mensuales, fuente_legal, fecha_inicio_vigencia, creado_por)
SELECT id, 8, 240, 'Código de Trabajo, art. 136', '2026-01-01', 1 FROM jornadas WHERE codigo = 'DIURNA'`)
	f.asistir(t, f.emp[2], "8", "2026-03-02", "2026-03-03")
	f.asistir(t, f.emp[2], "4", "2026-03-04")

	svc := f.servicio()
	p := marzo(t, svc, f)
	f.calcularOFallar(t, svc, p)
	if m, _ := f.montoConcepto(t, p.ID, f.emp[1], "SAL_BASE"); m != "900000.00" {
		t.Errorf("diario por días comerciales: SAL_BASE %s, esperado 30 × 30000 = 900000.00", m)
	}
	if m, _ := f.montoConcepto(t, p.ID, f.emp[2], "SAL_BASE"); m != "80000.00" {
		t.Errorf("por hora: SAL_BASE %s, esperado 20 h × 4000 = 80000.00", m)
	}
	if m, _ := f.montoConcepto(t, p.ID, f.emp[2], "HRS_EXTRA"); m != "36000.00" {
		t.Errorf("por hora: HRS_EXTRA %s, esperado 6 h × 4000 × 1.5 = 36000.00", m)
	}

	if _, err := svc.Descartar(context.Background(), f.preparador, p.ID); err != nil {
		t.Fatal(err)
	}
	f.generalDelEmpleado(t, f.emp[1], ClaveSalarioDiarioBase, SalarioDiarioAsistencia, "texto")
	f.asistir(t, f.emp[1], "8", "2026-03-09", "2026-03-10", "2026-03-11")
	f.calcularOFallar(t, svc, p)
	if m, _ := f.montoConcepto(t, p.ID, f.emp[1], "SAL_BASE"); m != "90000.00" {
		t.Errorf("diario por asistencia (solo emp[1]): SAL_BASE %s, esperado 3 × 30000 = 90000.00", m)
	}
	var snap string
	if err := f.db.QueryRow(`SELECT parametros_snapshot FROM periodos_planilla WHERE id = ?`, p.ID).Scan(&snap); err != nil {
		t.Fatal(err)
	}
	var s struct {
		Ajustes map[string]map[string]string `json:"ajustes_por_empleado"`
	}
	if err := json.Unmarshal([]byte(snap), &s); err != nil || len(s.Ajustes) != 1 {
		t.Fatalf("snapshot sin el ajuste del empleado: %v %s", err, snap)
	}
}

// Una fórmula propia del empleado reemplaza a la general solo para él y queda
// trazada en planilla_detalle_concepto.
func TestFormulaPropiaDelEmpleado(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	asoc := f.concepto["ASOC_SOLIDARISTA"]
	for _, e := range f.emp[:2] {
		mustExec(t, f.db, `
INSERT INTO empleado_conceptos_asignados (empleado_id, concepto_salarial_id, modo, valor, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, 'porcentaje', 5, '2026-01-01', ?)`, e, asoc, f.preparador)
	}
	propia := mustExec(t, f.db, `
INSERT INTO empleado_concepto_formula (empleado_id, concepto_salarial_id, expresion_formula, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, '{"tipo":"asignacion_empleado","base":"liquido"}', 'Convenio de afiliación', '2026-01-01', ?)`, f.emp[0], asoc, f.aprobador)

	svc := f.servicio()
	p := marzo(t, svc, f)
	f.calcularOFallar(t, svc, p)

	linea := func(emp int64) (string, sql.NullInt64, sql.NullInt64) {
		t.Helper()
		var m string
		var general, delEmpleado sql.NullInt64
		if err := f.db.QueryRow(`
SELECT pdc.monto, pdc.concepto_salarial_formula_id, pdc.empleado_concepto_formula_id
FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ? AND pdc.concepto_salarial_id = ?`, p.ID, emp, asoc).Scan(&m, &general, &delEmpleado); err != nil {
			t.Fatal(err)
		}
		return m, general, delEmpleado
	}
	m0, g0, e0 := linea(f.emp[0])
	if g0.Valid || !e0.Valid || e0.Int64 != propia {
		t.Errorf("emp[0]: fórmula general %v, propia %v; esperada solo la propia %d", g0, e0, propia)
	}
	if m0 == "45000.00" || !decimal.RequireFromString(m0).IsPositive() {
		t.Errorf("emp[0]: ASOC_SOLIDARISTA %s debe salir del líquido, no del bruto (45000.00)", m0)
	}
	m1, g1, e1 := linea(f.emp[1])
	if !g1.Valid || e1.Valid || m1 != "40000.00" {
		t.Errorf("emp[1]: %s con fórmula general %v y propia %v; esperado 40000.00 con la general", m1, g1, e1)
	}
}

// El embargo de la planilla de liquidación se calcula sobre el mes: suma el
// líquido de la planilla regular aprobada y descuenta lo que ya retuvo.
func TestEmbargoLiquidacionAcumulaConLaRegular(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	f.general(t, ClaveLiqPrestamosModo, LiqPrestamosCuota, "texto")
	var tipoJudicial, tercero int64
	if err := f.db.QueryRow(`SELECT id FROM parametros_tipos_embargo WHERE codigo = 'EMBARGO_JUDICIAL' AND fecha_fin_vigencia IS NULL`).Scan(&tipoJudicial); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`SELECT tercero_id FROM embargos WHERE id = ?`, f.embargoID).Scan(&tercero); err != nil {
		t.Fatal(err)
	}
	judicial := mustExec(t, f.db, `
INSERT INTO embargos (empleado_id, tercero_id, tipo_embargo_id, porcentaje, prioridad_aplicacion, fecha_inicio_vigencia)
VALUES (?, ?, ?, 40.00, 2, '2025-01-01')`, f.emp[0], tercero, tipoJudicial)

	svc := f.servicio()
	ctx := context.Background()
	f.mesAprobado(t, svc, "2026-03-01", "2026-03-31")
	f.insertarLiquidacion(t, f.emp[0], "2026-03-31", "0", "0", "600000")
	var tipoLiq int64
	if err := f.db.QueryRow(`SELECT id FROM tipos_planilla WHERE clase = 'liquidacion'`).Scan(&tipoLiq); err != nil {
		t.Fatal(err)
	}
	p, err := svc.Crear(ctx, f.preparador, tipoLiq, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	f.calcularOFallar(t, svc, p)

	type fila struct{ base, pension, judicial decimal.Decimal }
	leer := func(clase string) fila {
		t.Helper()
		var r fila
		if err := f.db.QueryRow(`
SELECT pde.base_embargo,
       COALESCE(SUM(CASE WHEN pdemb.embargo_id = ? THEN pdemb.monto END), 0),
       COALESCE(SUM(CASE WHEN pdemb.embargo_id = ? THEN pdemb.monto END), 0)
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
LEFT JOIN planilla_detalle_embargo pdemb ON pdemb.planilla_detalle_empleado_id = pde.id
WHERE pde.empleado_id = ? AND tp.clase = ?
GROUP BY pde.id, pde.base_embargo`, f.embargoID, judicial, f.emp[0], clase).Scan(&r.base, &r.pension, &r.judicial); err != nil {
			t.Fatal(err)
		}
		return r
	}
	reg, liq := leer("regular"), leer("liquidacion")

	gen := parametros.NewGenerales(parametros.NewRepositorioSQL(f.db), 0)
	params, err := NewCargadorParametros(fuenteCargadorSQL{dto.New(f.db)}, gen).Cargar(ctx, fecha("2026-03-01"), fecha("2026-03-31"))
	if err != nil {
		t.Fatal(err)
	}
	emb := []EmbargoInput{
		{EmbargoID: f.embargoID, Prioridad: 1, PrioridadLegal: 1, EsPension: true, TopePctLiquido: decimal.RequireFromString("0.5"), Porcentaje: decimal.RequireFromString("0.125")},
		{EmbargoID: judicial, Prioridad: 2, PrioridadLegal: 2, Porcentaje: decimal.RequireFromString("0.4")},
	}
	var tope decimal.NullDecimal
	if err := f.db.QueryRow(`SELECT tope_porcentaje_liquido FROM parametros_tipos_embargo WHERE id = ?`, tipoJudicial).Scan(&tope); err != nil {
		t.Fatal(err)
	}
	if tope.Valid {
		emb[1].TopePctLiquido = tope.Decimal.Div(decimal.NewFromInt(100))
	}
	mes, err := CalcularEmbargos(emb, reg.base.Add(liq.base), params.SalarioMinimoInembargable, params.EscalaEmbargo)
	if err != nil {
		t.Fatal(err)
	}
	if got := reg.pension.Add(liq.pension); !got.Equal(mes[0].MontoAplicado) {
		t.Errorf("pensión regular %s + liquidación %s = %s; el mes da %s", reg.pension, liq.pension, got, mes[0].MontoAplicado)
	}
	if got := reg.judicial.Add(liq.judicial); !got.Equal(mes[1].MontoAplicado) {
		t.Errorf("judicial regular %s + liquidación %s = %s; el mes da %s", reg.judicial, liq.judicial, got, mes[1].MontoAplicado)
	}
	if !liq.judicial.IsPositive() {
		t.Errorf("la liquidación debe retener parte del judicial (%s)", liq.judicial)
	}
}
