//go:build integration

package contabilidad

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	auditdb "planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/testutil"
)

type fixtureConta struct {
	db                             *sql.DB
	preparador, aprobador, pagador int64
	contador                       int64
	programa, periodo, empleado    int64
	concepto                       map[string]int64
	svc                            *Service
	detalles                       *DetallesSQL
}

func ex(t *testing.T, db *sql.DB, q string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(q, args...)
	if err != nil {
		t.Fatalf("%v\n%s", err, q)
	}
	id, _ := res.LastInsertId()
	return id
}

func cfgReal() ConfigGeneracion {
	return ConfigGeneracion{ConceptoAnclaNeto: "SAL_BASE", TipoAsientoPago: "PAGO_PLANILLA"}
}

// nuevoFixtureConta: período mensual pagado (borrador → calculada → aprobada →
// pagada por SQL, con los triggers del ciclo activos) en el programa PROG1
// (con mapeos sembrados en 000002), un empleado con SAL_BASE 1 000 000,
// CCSS_OBR 108 300 y una cuota patronal PERSISTIDA de 300 000 que a propósito
// no es base × % vigente: contabilidad debe usarla tal cual.
func nuevoFixtureConta(t *testing.T) *fixtureConta {
	t.Helper()
	db := testutil.NuevaBD(t)
	f := &fixtureConta{db: db, concepto: map[string]int64{}}
	for i, u := range []*int64{&f.preparador, &f.aprobador, &f.pagador, &f.contador} {
		*u = ex(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES (?, ?, ?, 'x')`,
			"conta-"+string(rune('a'+i)), "Usuario conta", "conta-"+string(rune('a'+i))+"@example.test")
	}
	rows, err := db.Query(`SELECT codigo, id FROM conceptos_salariales`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var c string
		var id int64
		if err := rows.Scan(&c, &id); err != nil {
			t.Fatal(err)
		}
		f.concepto[c] = id
	}
	rows.Close()
	if err := db.QueryRow(`SELECT id FROM programas_presupuestarios WHERE codigo = 'PROG1'`).Scan(&f.programa); err != nil {
		t.Fatal(err)
	}
	var tipoIdent, tipoMensual int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM tipos_planilla WHERE clase = 'regular' AND fraccion_mensual = 1`).Scan(&tipoMensual); err != nil {
		t.Fatal(err)
	}
	f.empleado = testutil.InsertarEmpleado(t, db, "CONTA1", tipoIdent, "CONTA-1", "Empleado conta", "2020-01-01")

	f.periodo = ex(t, db, `INSERT INTO periodos_planilla (tipo_planilla_id, fecha_inicio, fecha_fin, estado, preparado_por_usuario_id, fecha_preparacion)
VALUES (?, '2026-03-01', '2026-03-31', 'borrador', ?, NOW())`, tipoMensual, f.preparador)
	pde := ex(t, db, `INSERT INTO planilla_detalle_empleado (periodo_planilla_id, empleado_id, programa_presupuestario_id, salario_base_periodo,
  total_ingresos, total_deducciones, neto_pagado, base_ccss, cuota_obrera_ccss, cuota_patronal_ccss)
VALUES (?, ?, ?, 1000000, 1000000, 108300, 891700, 1000000, 108300, 300000)`, f.periodo, f.empleado, f.programa)
	ex(t, db, `INSERT INTO planilla_detalle_concepto (planilla_detalle_empleado_id, concepto_salarial_id, monto) VALUES (?, ?, 1000000), (?, ?, 108300)`,
		pde, f.concepto["SAL_BASE"], pde, f.concepto["CCSS_OBR"])
	ex(t, db, `INSERT INTO planilla_detalle_patronal (planilla_detalle_empleado_id, concepto_salarial_id, base, porcentaje, monto) VALUES (?, ?, 1000000, 0.3, 300000)`,
		pde, f.concepto["CCSS_PAT"])
	ex(t, db, `UPDATE periodos_planilla SET estado='calculada', calculado_por_usuario_id=?, fecha_calculo=NOW() WHERE id=?`, f.preparador, f.periodo)
	ex(t, db, `UPDATE periodos_planilla SET estado='aprobada', aprobado_por_usuario_id=?, fecha_aprobacion=NOW() WHERE id=?`, f.aprobador, f.periodo)
	ex(t, db, `UPDATE periodos_planilla SET estado='pagada', pagado_por_usuario_id=?, fecha_pago=NOW() WHERE id=?`, f.pagador, f.periodo)

	f.svc, err = NewService(NewRepositorio(db), cfgReal())
	if err != nil {
		t.Fatal(err)
	}
	if f.detalles, err = NewDetallesSQL(db); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixtureConta) contabilizar(t *testing.T, usuario int64) ([]AsientoContable, error) {
	t.Helper()
	det, err := f.detalles.DetallesParaContabilizar(context.Background(), f.periodo)
	if err != nil {
		return nil, err
	}
	return f.svc.ContabilizarPeriodo(context.Background(), f.periodo, usuario, det)
}

func (f *fixtureConta) contarAsientos(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM asientos_contables WHERE periodo_planilla_id = ?`, f.periodo).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestContabilizaConCuotaPersistidaYSegregacion(t *testing.T) {
	f := nuevoFixtureConta(t)

	if _, err := f.contabilizar(t, f.pagador); !errors.Is(err, planilla.ErrSegregacionFunciones) {
		t.Fatalf("esperaba ErrSegregacionFunciones, fue %v", err)
	}
	if n := f.contarAsientos(t); n != 0 {
		t.Fatalf("la segregación no debe dejar asientos (%d)", n)
	}

	as, err := f.contabilizar(t, f.contador)
	if err != nil {
		t.Fatal(err)
	}
	var debe, haber, patronalDebe string
	if err := f.db.QueryRow(`
SELECT SUM(d.monto_debe), SUM(d.monto_haber),
       SUM(CASE WHEN d.concepto_salarial_id = ? THEN d.monto_debe ELSE 0 END)
FROM asientos_contables_detalle d JOIN asientos_contables a ON a.id = d.asiento_contable_id
WHERE a.periodo_planilla_id = ?`, f.concepto["CCSS_PAT"], f.periodo).Scan(&debe, &haber, &patronalDebe); err != nil {
		t.Fatal(err)
	}
	if len(as) != 2 || debe != "1300000.00" || haber != "1300000.00" || patronalDebe != "300000.00" {
		t.Fatalf("asientos=%d debe=%s haber=%s patronal=%s", len(as), debe, haber, patronalDebe)
	}
	var estado string
	var por sql.NullInt64
	var fecha sql.NullTime
	if err := f.db.QueryRow(`SELECT estado, contabilizado_por_usuario_id, fecha_contabilizacion FROM periodos_planilla WHERE id = ?`, f.periodo).
		Scan(&estado, &por, &fecha); err != nil {
		t.Fatal(err)
	}
	if estado != "contabilizada" || !por.Valid || por.Int64 != f.contador || !fecha.Valid {
		t.Fatalf("estado=%s por=%v fecha=%v", estado, por, fecha)
	}
	if _, err := f.contabilizar(t, f.contador); !errors.Is(err, ErrPeriodoEstadoInvalido) {
		t.Fatalf("segunda contabilización: %v", err)
	}
}

func TestBDExigeUsuarioAlContabilizar(t *testing.T) {
	f := nuevoFixtureConta(t)
	if _, err := f.db.Exec(`UPDATE periodos_planilla SET estado = 'contabilizada' WHERE id = ?`, f.periodo); err == nil {
		t.Fatal("la BD debe rechazar contabilizar sin usuario ni fecha")
	}
	if _, err := f.db.Exec(`UPDATE periodos_planilla SET estado = 'contabilizada', contabilizado_por_usuario_id = ?, fecha_contabilizacion = NOW() WHERE id = ?`,
		f.pagador, f.periodo); err == nil {
		t.Fatal("la BD debe rechazar que quien pagó contabilice")
	}
}

func TestAsientoDePagoYConceptoSinMapeo(t *testing.T) {
	f := nuevoFixtureConta(t)
	g, err := NewGanchoPago(cfgReal())
	if err != nil {
		t.Fatal(err)
	}
	pagar := func() error {
		return auditdb.WithAuditContext(context.Background(), f.db, f.pagador, func(ctx context.Context, q *dto.Queries) error {
			p, err := q.ObtenerPeriodoCiclo(ctx, f.periodo)
			if err != nil {
				return err
			}
			return g.AlPagar(ctx, q, p)
		})
	}
	if err := pagar(); !errors.Is(err, ErrMapeoFaltante) {
		t.Fatalf("esperaba ErrMapeoFaltante sin cuenta de bancos: %v", err)
	}
	ex(t, f.db, `INSERT INTO mapeo_concepto_cuenta (concepto_salarial_id, programa_presupuestario_id, tipo_asiento_id, cuenta_contable_id, tipo_movimiento, fecha_inicio_vigencia)
SELECT ?, ?, ta.id, cc.id, 'haber', '2026-01-01' FROM tipos_asiento ta, cuentas_contables cc WHERE ta.codigo = 'PAGO_PLANILLA' AND cc.codigo = '1101'`,
		f.concepto["SAL_BASE"], f.programa)
	if err := pagar(); err != nil {
		t.Fatal(err)
	}
	var debeCta, haberCta, monto string
	if err := f.db.QueryRow(`
SELECT MAX(CASE WHEN d.monto_debe > 0 THEN cc.codigo END), MAX(CASE WHEN d.monto_haber > 0 THEN cc.codigo END), SUM(d.monto_debe)
FROM asientos_contables a JOIN tipos_asiento ta ON ta.id = a.tipo_asiento_id
JOIN asientos_contables_detalle d ON d.asiento_contable_id = a.id JOIN cuentas_contables cc ON cc.id = d.cuenta_contable_id
WHERE a.periodo_planilla_id = ? AND ta.codigo = 'PAGO_PLANILLA'`, f.periodo).Scan(&debeCta, &haberCta, &monto); err != nil {
		t.Fatal(err)
	}
	if debeCta != "2106" || haberCta != "1101" || monto != "891700.00" {
		t.Fatalf("asiento de pago: debe %s haber %s monto %s", debeCta, haberCta, monto)
	}
	if _, err := f.contabilizar(t, f.contador); err != nil {
		t.Fatalf("contabilizar con el asiento de pago ya generado: %v", err)
	}
	if n := f.contarAsientos(t); n != 3 {
		t.Fatalf("asientos del período: %d (esperado devengo gasto + pasivo + pago)", n)
	}
}

func TestConceptoSinMapeoFalla(t *testing.T) {
	f := nuevoFixtureConta(t)
	det, err := f.detalles.DetallesParaContabilizar(context.Background(), f.periodo)
	if err != nil {
		t.Fatal(err)
	}
	res := &det[0].Calculo.ResultadoEmpleadoOutput
	res.DesglosePorConcepto = append(res.DesglosePorConcepto, planilla.ConceptoCalculadoOutput{
		ConceptoSalarialID: f.concepto["LIQ_CESANTIA"], Codigo: "LIQ_CESANTIA", Naturaleza: "ingreso", Monto: res.CuotaObreroCCSS,
	})
	_, err = f.svc.ContabilizarPeriodo(context.Background(), f.periodo, f.contador, det)
	if !errors.Is(err, ErrMapeoFaltante) {
		t.Fatalf("esperaba ErrMapeoFaltante, fue %v", err)
	}
	if n := f.contarAsientos(t); n != 0 {
		t.Fatalf("no debe persistir nada (%d)", n)
	}
}
