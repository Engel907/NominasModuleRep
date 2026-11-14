//go:build integration

package rrhh

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/liquidacion"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla/infra"
	"planillas-ccdr/internal/testutil"
)

type fixtureLiq struct {
	db                    *sql.DB
	preparador, aprobador int64
	empleado              int64
	programa, puesto      int64
	tipoMensual           int64
	salBase               int64
	svc                   *LiquidacionService
}

func exec(t *testing.T, db *sql.DB, q string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(q, args...)
	if err != nil {
		t.Fatalf("%v\n%s", err, q)
	}
	id, _ := res.LastInsertId()
	return id
}

// nuevoFixtureLiq: empleado que ingresó el 2023-06-01 con DOS contratos
// consecutivos (el segundo empieza el 2025-01-01) y seis meses de planilla
// regular aprobada (ene–jun 2026) con 500 000 en todas las bases.
func nuevoFixtureLiq(t *testing.T) *fixtureLiq {
	t.Helper()
	db := testutil.NuevaBD(t)
	f := &fixtureLiq{db: db}
	f.preparador = exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('liq-p', 'Prepara', 'liq-p@example.test', 'x')`)
	exec(t, db, `INSERT INTO parametros_generales (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES ('LIQ_CESANTIA_METODO', 'acumulado_por_anio', 'texto', 'fixture', 'FIXTURE DE PRUEBA', '2020-01-01', ?)`, f.preparador)
	f.aprobador = exec(t, db, `INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('liq-a', 'Aprueba', 'liq-a@example.test', 'x')`)
	if err := db.QueryRow(`SELECT id FROM tipos_planilla WHERE clase = 'regular' AND fraccion_mensual = 1`).Scan(&f.tipoMensual); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM conceptos_salariales WHERE codigo = 'SAL_BASE'`).Scan(&f.salBase); err != nil {
		t.Fatal(err)
	}
	var tipoIdent, regimen int64
	if err := db.QueryRow(`SELECT id FROM tipos_identificacion WHERE codigo='CEDULA_NACIONAL'`).Scan(&tipoIdent); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM regimenes_laborales WHERE codigo='CODIGO_TRABAJO'`).Scan(&regimen); err != nil {
		t.Fatal(err)
	}
	unidad := exec(t, db, `INSERT INTO unidades_organizativas (codigo, nombre) VALUES ('LIQ', 'Unidad liq')`)
	f.puesto = exec(t, db, `INSERT INTO puestos (codigo, nombre, unidad_organizativa_id) VALUES ('LIQ', 'Puesto liq', ?)`, unidad)
	f.programa = exec(t, db, `INSERT INTO programas_presupuestarios (codigo, nombre) VALUES ('LIQ', 'Programa liq')`)
	tipoContrato := exec(t, db, `INSERT INTO tipos_contrato (codigo, nombre) VALUES ('LIQ_IND', 'Indefinido')`)

	f.empleado = testutil.InsertarEmpleado(t, db, "LIQ1", tipoIdent, "LIQ-0001", "Empleado liquidación", "2023-06-01")
	exec(t, db, `
INSERT INTO contratos (empleado_id, puesto_id, regimen_laboral_id, tipo_contrato_id, salario_pactado, modalidad_salario, jornada_id,
                       fecha_inicio_contrato, fecha_fin_contrato, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
VALUES (?, ?, ?, (SELECT MIN(id) FROM tipos_contrato), 400000, 'mensual', (SELECT id FROM jornadas WHERE codigo = 'DIURNA'), '2023-06-01', '2024-12-31', '2023-06-01', '2024-12-31', ?)`,
		f.empleado, f.puesto, regimen, f.preparador)
	exec(t, db, `
INSERT INTO contratos (empleado_id, puesto_id, regimen_laboral_id, tipo_contrato_id, salario_pactado, modalidad_salario, jornada_id,
                       fecha_inicio_contrato, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, ?, (SELECT MIN(id) FROM tipos_contrato), 500000, 'mensual', (SELECT id FROM jornadas WHERE codigo = 'DIURNA'), '2025-01-01', '2025-01-01', ?)`, f.empleado, f.puesto, regimen, f.preparador)
	exec(t, db, `
INSERT INTO empleado_datos_laborales (empleado_id, puesto_id, salario_base_nominal, tipo_contrato_id, programa_presupuestario_id, fecha_inicio_vigencia)
VALUES (?, ?, 500000, ?, ?, '2025-01-01')`, f.empleado, f.puesto, tipoContrato, f.programa)
	exec(t, db, `
INSERT INTO vacaciones_acumulado (empleado_id, fecha_corte, dias_acumulados, dias_disfrutados, saldo, dias_disponibles)
VALUES (?, '2026-06-01', 6, 0, 6, 6)`, f.empleado)

	for mes := 1; mes <= 6; mes++ {
		ini := time.Date(2026, time.Month(mes), 1, 0, 0, 0, 0, time.UTC)
		f.periodoAprobado(t, ini, ini.AddDate(0, 1, -1), "500000.00")
	}
	q := dto.New(db)
	generales := parametros.NewGenerales(parametros.NewRepositorioSQL(db), 0)
	f.svc = NewLiquidacionService(db, q, infra.NewMotorRepository(q), generales)
	return f
}

// periodoAprobado inserta un período regular con una línea SAL_BASE y lo
// lleva borrador → calculada → aprobada (triggers de ciclo y cuadre activos).
func (f *fixtureLiq) periodoAprobado(t *testing.T, ini, fin time.Time, monto string) {
	t.Helper()
	p := exec(t, f.db, `
INSERT INTO periodos_planilla (tipo_planilla_id, fecha_inicio, fecha_fin, estado, preparado_por_usuario_id, fecha_preparacion)
VALUES (?, ?, ?, 'borrador', ?, NOW())`, f.tipoMensual, ini, fin, f.preparador)
	pde := exec(t, f.db, `
INSERT INTO planilla_detalle_empleado (periodo_planilla_id, empleado_id, programa_presupuestario_id, salario_base_periodo,
  total_ingresos, total_deducciones, neto_pagado, base_ccss, base_renta, base_aguinaldo, base_cesantia, base_vacaciones)
VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?)`, p, f.empleado, f.programa, monto, monto, monto, monto, monto, monto, monto, monto)
	exec(t, f.db, `INSERT INTO planilla_detalle_concepto (planilla_detalle_empleado_id, concepto_salarial_id, monto) VALUES (?, ?, ?)`, pde, f.salBase, monto)
	exec(t, f.db, `UPDATE periodos_planilla SET estado = 'calculada', calculado_por_usuario_id = ?, fecha_calculo = NOW() WHERE id = ?`, f.preparador, p)
	exec(t, f.db, `UPDATE periodos_planilla SET estado = 'aprobada', aprobado_por_usuario_id = ?, fecha_aprobacion = NOW() WHERE id = ?`, f.aprobador, p)
}

func TestDespidoConDosContratosConsecutivos(t *testing.T) {
	f := nuevoFixtureLiq(t)
	ctx := context.Background()
	sol := ProyectarSolicitud{
		EmpleadoID: f.empleado, FechaSalida: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		MotivoSalida: "despido_con_responsabilidad", AplicaResponsabilidadPatronal: true,
	}
	r, err := f.svc.ProyectarLiquidacion(ctx, sol)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{
		"antiguedad": fmt.Sprint(r.AntiguedadMeses), "preaviso": r.MontoPreaviso.String(),
		"cesantia": r.MontoCesantia.String(), "vacaciones": r.MontoVacacionesProporcionales.String(),
		"aguinaldo": r.MontoAguinaldoProporcional.String(), "escolar": r.MontoSalarioEscolarProporcional.String(),
		"total": r.TotalLiquidacion.String(), "aguinaldo_desde": r.PeriodoAguinaldoDesde.Format("2006-01-02"),
	}
	want := map[string]string{
		"antiguedad": "36", "preaviso": "500000", "cesantia": "1000000", "vacaciones": "100000",
		"aguinaldo": "250000", "escolar": "249900", "total": "2099900", "aguinaldo_desde": "2025-12-01",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %s, esperado %s", k, got[k], v)
		}
	}
	// El sistema solo tiene seis meses de planilla y la ventana de vacaciones
	// es de 50 semanas (Art. 157 CT): el cálculo lo advierte.
	if len(r.Advertencias) != 1 || !strings.Contains(r.Advertencias[0], "promedio de vacaciones") {
		t.Errorf("advertencias: %v", r.Advertencias)
	}

	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM liquidaciones`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("proyectar escribió %d liquidaciones (%v)", n, err)
	}

	f.svc.ahora = func() time.Time { return time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC) }
	proc, err := f.svc.ProcesarLiquidacion(ctx, sol, f.aprobador)
	if err != nil {
		t.Fatal(err)
	}
	id := proc.LiquidacionID
	if !proc.Rubros.TotalLiquidacion.Equal(r.TotalLiquidacion) || proc.CierreProgramado || proc.PeriodoPlanillaID == 0 {
		t.Errorf("procesar %s ≠ proyectar %s, programado=%t, período=%d", proc.Rubros.TotalLiquidacion, r.TotalLiquidacion, proc.CierreProgramado, proc.PeriodoPlanillaID)
	}
	var total, escolar string
	var activo int
	if err := f.db.QueryRow(`SELECT l.total_liquidacion, l.monto_salario_escolar_proporcional, e.activo
FROM liquidaciones l JOIN empleados e ON e.id = l.empleado_id WHERE l.id = ?`, id).Scan(&total, &escolar, &activo); err != nil {
		t.Fatal(err)
	}
	if total != "2099900.00" || escolar != "249900.00" || activo != 0 {
		t.Errorf("persistido total=%s escolar=%s activo=%d", total, escolar, activo)
	}
}

func TestRenunciaNoAdmiteResponsabilidad(t *testing.T) {
	f := nuevoFixtureLiq(t)
	sol := ProyectarSolicitud{
		EmpleadoID: f.empleado, FechaSalida: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		MotivoSalida: "renuncia", AplicaResponsabilidadPatronal: true,
	}
	if _, err := f.svc.ProyectarLiquidacion(context.Background(), sol); !errors.Is(err, liquidacion.ErrResponsabilidadNoAdmitida) {
		t.Fatalf("esperaba ErrResponsabilidadNoAdmitida, fue %v", err)
	}
	sol.AplicaResponsabilidadPatronal = false
	r, err := f.svc.ProyectarLiquidacion(context.Background(), sol)
	if err != nil {
		t.Fatal(err)
	}
	if !r.MontoPreaviso.IsZero() || !r.MontoCesantia.IsZero() || r.TotalLiquidacion.String() != "599900" {
		t.Errorf("renuncia: preaviso %s cesantía %s total %s", r.MontoPreaviso, r.MontoCesantia, r.TotalLiquidacion)
	}
}

func TestMigracionTramosYMotivos(t *testing.T) {
	db := testutil.NuevaBD(t)
	var tramos, motivos int
	if err := db.QueryRow(`SELECT COUNT(*) FROM parametros_cesantia_escala WHERE anios_servicio_hasta <= 1 AND es_vigente = 1`).Scan(&tramos); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM parametros_motivos_salida WHERE es_vigente = 1`).Scan(&motivos); err != nil {
		t.Fatal(err)
	}
	if tramos != 3 || motivos != 7 {
		t.Fatalf("tramos < 1 año = %d (esperado 3), motivos = %d (esperado 7)", tramos, motivos)
	}
	if _, err := db.Exec(`UPDATE parametros_cesantia_escala SET dias_por_anio = 99 WHERE anios_servicio_desde = 0.25`); err == nil {
		t.Fatal("la escala de cesantía debe ser inmutable")
	}
	if _, err := db.Exec(`DELETE FROM parametros_motivos_salida`); err == nil {
		t.Fatal("parametros_motivos_salida no se debe poder borrar")
	}
}

func TestSalidaFuturaQuedaProgramada(t *testing.T) {
	f := nuevoFixtureLiq(t)
	ctx := context.Background()
	f.svc.ahora = func() time.Time { return time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC) }
	sol := ProyectarSolicitud{
		EmpleadoID: f.empleado, FechaSalida: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		MotivoSalida: "despido_con_responsabilidad", AplicaResponsabilidadPatronal: true,
	}
	proc, err := f.svc.ProcesarLiquidacion(ctx, sol, f.aprobador)
	if err != nil {
		t.Fatal(err)
	}
	if !proc.CierreProgramado {
		t.Fatal("salida futura debe quedar programada")
	}
	estado := func() (activo int, cierre bool, contratoAbierto int) {
		t.Helper()
		var c sql.NullTime
		if err := f.db.QueryRow(`SELECT e.activo, l.cierre_aplicado_en, (SELECT COUNT(*) FROM contratos WHERE empleado_id = e.id AND fecha_fin_vigencia IS NULL)
FROM empleados e JOIN liquidaciones l ON l.empleado_id = e.id WHERE l.id = ?`, proc.LiquidacionID).Scan(&activo, &c, &contratoAbierto); err != nil {
			t.Fatal(err)
		}
		return activo, c.Valid, contratoAbierto
	}
	if a, c, ab := estado(); a != 1 || c || ab != 1 {
		t.Fatalf("antes de la salida: activo=%d cierre=%t contrato abierto=%d", a, c, ab)
	}
	var ini, fin, est string
	if err := f.db.QueryRow(`SELECT DATE_FORMAT(pp.fecha_inicio, '%Y-%m-%d'), DATE_FORMAT(pp.fecha_fin, '%Y-%m-%d'), pp.estado FROM periodos_planilla pp JOIN tipos_planilla tp ON tp.id = pp.tipo_planilla_id
WHERE pp.id = ? AND tp.clase = 'liquidacion'`, proc.PeriodoPlanillaID).Scan(&ini, &fin, &est); err != nil {
		t.Fatal(err)
	}
	if ini != "2026-06-01" || fin != "2026-06-30" || est != "borrador" {
		t.Fatalf("período de liquidación %s..%s %s", ini, fin, est)
	}
	if _, err := f.svc.ProcesarLiquidacion(ctx, sol, f.aprobador); !errors.Is(err, ErrLiquidacionYaProcesada) {
		t.Fatalf("segunda liquidación: %v", err)
	}

	r, err := f.svc.CerrarSalidasPendientes(ctx, time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC), f.aprobador)
	if err != nil || r.Cerradas != 0 {
		t.Fatalf("antes del día de salida no se cierra: %+v %v", r, err)
	}
	for i := 0; i < 2; i++ {
		r, err = f.svc.CerrarSalidasPendientes(ctx, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), f.aprobador)
		if err != nil || len(r.Errores) != 0 || r.Cerradas != 1-i {
			t.Fatalf("corrida %d: %+v %v", i, r, err)
		}
	}
	if a, c, ab := estado(); a != 0 || !c || ab != 0 {
		t.Fatalf("tras la salida: activo=%d cierre=%t contrato abierto=%d", a, c, ab)
	}
	var finContrato string
	if err := f.db.QueryRow(`SELECT DATE_FORMAT(MAX(fecha_fin_vigencia), '%Y-%m-%d') FROM contratos WHERE empleado_id = ?`, f.empleado).Scan(&finContrato); err != nil || finContrato != "2026-06-30" {
		t.Fatalf("contrato cerrado al %s (%v)", finContrato, err)
	}
}

func TestPeriodoDelMesAprobadoUsaElMesEnCurso(t *testing.T) {
	f := nuevoFixtureLiq(t)
	ctx := context.Background()
	var tipoLiq int64
	if err := f.db.QueryRow(`SELECT id FROM tipos_planilla WHERE clase = 'liquidacion'`).Scan(&tipoLiq); err != nil {
		t.Fatal(err)
	}
	p := exec(t, f.db, `INSERT INTO periodos_planilla (tipo_planilla_id, fecha_inicio, fecha_fin, estado, preparado_por_usuario_id, fecha_preparacion)
VALUES (?, '2026-06-01', '2026-06-30', 'borrador', ?, NOW())`, tipoLiq, f.preparador)
	pde := exec(t, f.db, `INSERT INTO planilla_detalle_empleado (periodo_planilla_id, empleado_id, programa_presupuestario_id, salario_base_periodo, total_ingresos, neto_pagado)
VALUES (?, ?, ?, 0, 1, 1)`, p, f.empleado, f.programa)
	exec(t, f.db, `INSERT INTO planilla_detalle_concepto (planilla_detalle_empleado_id, concepto_salarial_id, monto) VALUES (?, ?, 1)`, pde, f.salBase)
	exec(t, f.db, `UPDATE periodos_planilla SET estado = 'calculada', calculado_por_usuario_id = ?, fecha_calculo = NOW() WHERE id = ?`, f.preparador, p)
	exec(t, f.db, `UPDATE periodos_planilla SET estado = 'aprobada', aprobado_por_usuario_id = ?, fecha_aprobacion = NOW() WHERE id = ?`, f.aprobador, p)

	f.svc.ahora = func() time.Time { return time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC) }
	proc, err := f.svc.ProcesarLiquidacion(ctx, ProyectarSolicitud{
		EmpleadoID: f.empleado, FechaSalida: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), MotivoSalida: "renuncia",
	}, f.aprobador)
	if err != nil {
		t.Fatal(err)
	}
	var ini string
	if err := f.db.QueryRow(`SELECT DATE_FORMAT(fecha_inicio, '%Y-%m-%d') FROM periodos_planilla WHERE id = ?`, proc.PeriodoPlanillaID).Scan(&ini); err != nil || ini != "2026-07-01" {
		t.Fatalf("debió usar el período de julio: %s (%v)", ini, err)
	}
	if proc.CierreProgramado {
		t.Fatal("salida pasada se cierra al procesar")
	}
}
