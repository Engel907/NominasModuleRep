//go:build integration

package e2e

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/contabilidad"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/planilla/infra"
	"planillas-ccdr/internal/rrhh"
	"planillas-ccdr/internal/testutil"
)

type cierre struct {
	t                                         *testing.T
	db                                        *sql.DB
	q                                         *dto.Queries
	prepara, aprueba, paga, contabiliza       int64
	programa, puesto, tipoContrato, regimenCT int64
	sem, tipoIdent, banco, tercero            int64
	concepto                                  map[string]int64
	periodos                                  *planilla.PeriodoService
	conta                                     *contabilidad.Service
	detalles                                  *contabilidad.DetallesSQL
	generales                                 *parametros.Generales
}

func (c *cierre) ex(q string, args ...any) int64 {
	c.t.Helper()
	res, err := c.db.Exec(q, args...)
	if err != nil {
		c.t.Fatalf("%v\n%s", err, q)
	}
	id, _ := res.LastInsertId()
	return id
}

func (c *cierre) id(q string, args ...any) int64 {
	c.t.Helper()
	var id int64
	if err := c.db.QueryRow(q, args...).Scan(&id); err != nil {
		c.t.Fatalf("%v\n%s", err, q)
	}
	return id
}

func nuevoCierre(t *testing.T) *cierre {
	t.Helper()
	db := testutil.NuevaBD(t)
	c := &cierre{t: t, db: db, q: dto.New(db), concepto: map[string]int64{}}
	for i, u := range []*int64{&c.prepara, &c.aprueba, &c.paga, &c.contabiliza} {
		*u = c.ex(`INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES (?, 'Usuario', ?, 'x')`,
			fmt.Sprintf("e2e-%d", i), fmt.Sprintf("e2e-%d@example.test", i))
	}
	rows, err := db.Query(`SELECT codigo, id FROM conceptos_salariales`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatal(err)
		}
		c.concepto[k] = v
	}
	rows.Close()
	c.programa = c.id(`SELECT id FROM programas_presupuestarios WHERE codigo = 'PROG1'`)
	c.regimenCT = c.id(`SELECT id FROM regimenes_laborales WHERE codigo = 'CODIGO_TRABAJO'`)
	c.sem = c.id(`SELECT id FROM regimenes_ccss WHERE codigo = 'SEM'`)
	c.tipoIdent = c.id(`SELECT id FROM tipos_identificacion WHERE codigo = 'CEDULA_NACIONAL'`)
	unidad := c.ex(`INSERT INTO unidades_organizativas (codigo, nombre) VALUES ('E2E', 'Unidad')`)
	c.puesto = c.ex(`INSERT INTO puestos (codigo, nombre, unidad_organizativa_id) VALUES ('E2E', 'Puesto', ?)`, unidad)
	c.tipoContrato = c.ex(`INSERT INTO tipos_contrato (codigo, nombre) VALUES ('E2E', 'Indefinido')`)
	c.banco = c.ex(`INSERT INTO bancos (codigo, nombre, es_depositario_nomina) VALUES ('E2E', 'Banco', 1)`)
	c.tercero = c.ex(`INSERT INTO terceros (nombre, tipo) VALUES ('Juzgado', 'juzgado')`)

	c.ex(`INSERT INTO parametros_generales (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES ('SALARIO_MINIMO_INEMBARGABLE', '300000', 'decimal', 'fixture', 'FIXTURE DE PRUEBA', '2025-01-01', 1),
       ('LIQ_PRESTAMOS_MODO', 'cuota', 'texto', 'fixture', 'FIXTURE DE PRUEBA', '2025-01-01', 1),
       ('LIQ_CESANTIA_METODO', 'acumulado_por_anio', 'texto', 'fixture', 'FIXTURE DE PRUEBA', '2025-01-01', 1),
       ('VACACIONES_SALARIO_BASE', 'vigente', 'texto', 'fixture', 'FIXTURE DE PRUEBA', '2025-01-01', 1),
       ('INCAPACIDAD_SALARIO_BASE', 'vigente', 'texto', 'fixture', 'FIXTURE DE PRUEBA', '2025-01-01', 1)`)
	c.ex(`INSERT INTO mapeo_concepto_cuenta (concepto_salarial_id, programa_presupuestario_id, tipo_asiento_id, cuenta_contable_id, tipo_movimiento, fecha_inicio_vigencia)
SELECT cs.id, ?, ta.id, cc.id, v.mov, '2025-01-01'
FROM (SELECT 'SAL_BASE' cod, 'PAGO_PLANILLA' ta, '1101' cta, 'haber' mov
      UNION ALL SELECT 'SUB_INCAP', 'DEVENGO_GASTO_PLANILLA', '5101', 'debe'
      UNION ALL SELECT 'COMISIONES', 'DEVENGO_GASTO_PLANILLA', '5101', 'debe'
      UNION ALL SELECT 'BONO_DISCRECIONAL', 'DEVENGO_GASTO_PLANILLA', '5101', 'debe'
      UNION ALL SELECT 'LIQ_AGUINALDO', 'DEVENGO_GASTO_PLANILLA', '5103', 'debe'
      UNION ALL SELECT 'LIQ_SAL_ESCOLAR', 'DEVENGO_GASTO_PLANILLA', '5104', 'debe') v
JOIN conceptos_salariales cs ON cs.codigo = v.cod
JOIN tipos_asiento ta ON ta.codigo = v.ta
JOIN cuentas_contables cc ON cc.codigo = v.cta`, c.programa)

	c.generales = parametros.NewGenerales(parametros.NewRepositorioSQL(db), 0)
	w := &planilla.EscritorDetalle{}
	ventanas := planilla.NewVentanas(c.generales)
	orq := planilla.NewOrquestador(planilla.NewHidratador(c.q), planilla.NewMotorEmpleado(infra.NewMotorRepository(c.q)),
		planilla.NewCargadorParametros(infra.NewFuenteParametros(c.q), c.generales), w, c.q, ventanas)
	cfg := contabilidad.ConfigGeneracion{ConceptoAnclaNeto: "SAL_BASE", TipoAsientoPago: "PAGO_PLANILLA"}
	pago, err := contabilidad.NewGanchoPago(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.periodos = planilla.NewPeriodoService(db, planilla.PeriodoGanchos{Calculador: orq, Eliminador: w, AlPagar: pago,
		AlAprobar: planilla.GanchosAlAprobar{planilla.NewAplicadorPrestamos(), planilla.NewAplicadorAcumulados(), planilla.NewAplicadorVacaciones()}})
	if c.conta, err = contabilidad.NewService(contabilidad.NewRepositorio(db), cfg); err != nil {
		t.Fatal(err)
	}
	if c.detalles, err = contabilidad.NewDetallesSQL(db); err != nil {
		t.Fatal(err)
	}
	return c
}

// empleado: contrato DIURNA en CODIGO_TRABAJO, datos laborales en PROG1 con
// régimen SEM y cuenta bancaria. finContrato "" = indefinido.
func (c *cierre) empleado(n int, salario, inicio, finContrato string) int64 {
	c.t.Helper()
	e := testutil.InsertarEmpleado(c.t, c.db, fmt.Sprintf("E%02d", n), c.tipoIdent, fmt.Sprintf("1-%04d-%04d", n, n), fmt.Sprintf("Empleado %02d", n), inicio)
	var fin any
	if finContrato != "" {
		fin = finContrato
	}
	c.ex(`INSERT INTO contratos (empleado_id, puesto_id, regimen_laboral_id, tipo_contrato_id, salario_pactado, modalidad_salario, jornada_id,
  fecha_inicio_contrato, fecha_fin_contrato, fecha_inicio_vigencia, creado_por)
VALUES (?, ?, ?, (SELECT MIN(id) FROM tipos_contrato), ?, 'mensual', (SELECT id FROM jornadas WHERE codigo = 'DIURNA'), ?, ?, ?, ?)`, e, c.puesto, c.regimenCT, salario, inicio, fin, inicio, c.prepara)
	c.ex(`INSERT INTO empleado_datos_laborales (empleado_id, puesto_id, salario_base_nominal, tipo_contrato_id, programa_presupuestario_id, regimen_ccss_id, fecha_inicio_vigencia)
VALUES (?, ?, ?, ?, ?, ?, ?)`, e, c.puesto, salario, c.tipoContrato, c.programa, c.sem, inicio)
	testutil.InsertarCuentaBancaria(c.t, c.db, e, c.banco, "ahorros", fmt.Sprintf("CR0501520200102628%04d", n), inicio)
	return e
}

func (c *cierre) linea(periodo, emp int64, codigo string) string {
	c.t.Helper()
	var m string
	if err := c.db.QueryRow(`
SELECT COALESCE(SUM(pdc.monto), 0) FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ? AND cs.codigo = ?`, periodo, emp, codigo).Scan(&m); err != nil {
		c.t.Fatal(err)
	}
	return m
}

func (c *cierre) neto(periodo, emp int64) string {
	c.t.Helper()
	var m string
	if err := c.db.QueryRow(`SELECT neto_pagado FROM planilla_detalle_empleado WHERE periodo_planilla_id = ? AND empleado_id = ?`, periodo, emp).Scan(&m); err != nil {
		c.t.Fatalf("neto del empleado %d en el período %d: %v", emp, periodo, err)
	}
	return m
}

func (c *cierre) calcular(p int64) {
	c.t.Helper()
	if _, err := c.periodos.Calcular(context.Background(), c.prepara, p); err != nil {
		var ce *planilla.ErrCalculoConErrores
		if errors.As(err, &ce) {
			c.t.Fatalf("calcular %d: %+v", p, ce.Errores)
		}
		c.t.Fatalf("calcular %d: %v", p, err)
	}
}

// cerrarCiclo: aprobar → pagar (asiento de pago) → contabilizar, cada paso
// con una persona distinta, y verificaciones de cuadre.
func (c *cierre) cerrarCiclo(p int64) {
	c.t.Helper()
	ctx := context.Background()
	if _, err := c.periodos.Aprobar(ctx, c.aprueba, p); err != nil {
		c.t.Fatalf("aprobar %d: %v", p, err)
	}
	if _, err := c.periodos.Pagar(ctx, c.paga, p); err != nil {
		c.t.Fatalf("pagar %d: %v", p, err)
	}
	det, err := c.detalles.DetallesParaContabilizar(ctx, p)
	if err != nil {
		c.t.Fatalf("detalles %d: %v", p, err)
	}
	if _, err := c.conta.ContabilizarPeriodo(ctx, p, c.contabiliza, det); err != nil {
		c.t.Fatalf("contabilizar %d: %v", p, err)
	}
	var cuadra int
	var estado string
	if err := c.db.QueryRow(`SELECT fn_detalle_cuadra(id), estado FROM periodos_planilla WHERE id = ?`, p).Scan(&cuadra, &estado); err != nil {
		c.t.Fatal(err)
	}
	var debe, haber, pagoDebe, neto string
	if err := c.db.QueryRow(`
SELECT SUM(d.monto_debe), SUM(d.monto_haber), SUM(CASE WHEN ta.codigo = 'PAGO_PLANILLA' THEN d.monto_debe ELSE 0 END)
FROM asientos_contables a JOIN asientos_contables_detalle d ON d.asiento_contable_id = a.id
JOIN tipos_asiento ta ON ta.id = a.tipo_asiento_id WHERE a.periodo_planilla_id = ?`, p).Scan(&debe, &haber, &pagoDebe); err != nil {
		c.t.Fatal(err)
	}
	if err := c.db.QueryRow(`SELECT SUM(neto_pagado) FROM planilla_detalle_empleado WHERE periodo_planilla_id = ?`, p).Scan(&neto); err != nil {
		c.t.Fatal(err)
	}
	if cuadra != 1 || estado != "contabilizada" || debe != haber || pagoDebe != neto {
		c.t.Fatalf("período %d: cuadra=%d estado=%s debe=%s haber=%s pago=%s Σneto=%s", p, cuadra, estado, debe, haber, pagoDebe, neto)
	}
}

func TestCierreDeMesCompleto(t *testing.T) {
	c := nuevoCierre(t)
	ctx := context.Background()

	e1 := c.empleado(1, "600000", "2020-01-01", "")
	e2 := c.empleado(2, "600000", "2026-03-16", "")
	e3 := c.empleado(3, "900000", "2020-01-01", "2026-03-20")
	e4 := c.empleado(4, "800000", "2020-01-01", "")
	e5 := c.empleado(5, "700000", "2020-01-01", "")
	e6 := c.empleado(6, "720000", "2020-01-01", "")
	e7 := c.empleado(7, "900000", "2020-01-01", "")
	e8 := c.empleado(8, "600000", "2020-01-01", "")
	e9 := c.empleado(9, "300000", "2020-01-01", "")
	e10 := c.empleado(10, "500000", "2020-01-01", "")
	e11 := c.empleado(11, "1500000", "2020-01-01", "")

	c.ex(`UPDATE contratos SET fecha_fin_vigencia = '2026-03-16' WHERE empleado_id = ?`, e4)
	c.ex(`INSERT INTO contratos (empleado_id, puesto_id, regimen_laboral_id, tipo_contrato_id, salario_pactado, modalidad_salario, jornada_id,
  fecha_inicio_contrato, fecha_inicio_vigencia, creado_por) VALUES (?, ?, ?, (SELECT MIN(id) FROM tipos_contrato), 1000000, 'mensual', (SELECT id FROM jornadas WHERE codigo = 'DIURNA'), '2020-01-01', '2026-03-16', ?)`,
		e4, c.puesto, c.regimenCT, c.prepara)
	c.ex(`INSERT INTO incapacidades (empleado_id, entidad_emisora, numero_boleta, fecha_inicio, fecha_fin, dias, creado_por)
VALUES (?, 'CCSS', 'E2E-1', '2026-02-27', '2026-03-04', 6, ?)`, e5, c.prepara)
	c.ex(`INSERT INTO asistencia_hora_extra_tipificada (empleado_id, fecha, tipo_hora_extra_id, horas)
SELECT ?, '2026-03-10', id, 10 FROM tipos_hora_extra WHERE clase = 'diurna'`, e6)
	c.ex(`INSERT INTO horas_extra_autorizadas (empleado_id, fecha, horas_autorizadas, motivo, estado, solicitado_por, resuelto_por, fecha_resolucion)
VALUES (?, '2026-03-10', 10, 'cierre', 'autorizada', ?, ?, NOW())`, e6, c.prepara, c.aprueba)
	pension := c.id(`SELECT id FROM parametros_tipos_embargo WHERE codigo = 'PENSION_ALIMENTARIA' AND fecha_fin_vigencia IS NULL`)
	judicial := c.id(`SELECT id FROM parametros_tipos_embargo WHERE codigo = 'EMBARGO_JUDICIAL' AND fecha_fin_vigencia IS NULL`)
	c.ex(`INSERT INTO embargos (empleado_id, tercero_id, tipo_embargo_id, porcentaje, prioridad_aplicacion, fecha_inicio_vigencia)
VALUES (?, ?, ?, 10.00, 1, '2025-01-01'), (?, ?, ?, 15.00, 2, '2025-01-01')`, e7, c.tercero, pension, e7, c.tercero, pension)
	embJudicial := c.ex(`INSERT INTO embargos (empleado_id, tercero_id, tipo_embargo_id, monto_fijo, prioridad_aplicacion, fecha_inicio_vigencia)
VALUES (?, ?, ?, 50000, 3, '2025-01-01')`, e7, c.tercero, judicial)
	prestamo8 := c.ex(`INSERT INTO prestamos (empleado_id, tercero_id, monto_total, cuota_periodica, saldo_pendiente, fecha_inicio)
VALUES (?, ?, 300000, 50000, 30000, '2025-01-01')`, e8, c.tercero)
	prestamo9 := c.ex(`INSERT INTO prestamos (empleado_id, tercero_id, monto_total, cuota_periodica, saldo_pendiente, fecha_inicio)
VALUES (?, ?, 1000000, 400000, 1000000, '2025-01-01')`, e9, c.tercero)

	tipoMensual := c.id(`SELECT id FROM tipos_planilla WHERE clase = 'regular' AND fraccion_mensual = 1`)
	marzo, err := c.periodos.Crear(ctx, c.prepara, tipoMensual, fecha("2026-03-01"), fecha("2026-03-31"))
	if err != nil {
		t.Fatal(err)
	}
	dev := planilla.NewDevengosService(c.db)
	for _, d := range []struct {
		cod, monto string
	}{{"COMISIONES", "100000"}, {"BONO_DISCRECIONAL", "20000"}} {
		if _, err := dev.Crear(ctx, c.prepara, marzo.ID, planilla.NuevoDevengo{EmpleadoID: e10, ConceptoSalarialID: c.concepto[d.cod], Monto: decimal.RequireFromString(d.monto)}); err != nil {
			t.Fatal(err)
		}
	}
	c.calcular(marzo.ID)

	casos := []struct {
		nombre string
		emp    int64
		lineas map[string]string
		neto   string
	}{
		{"regular", e1, map[string]string{"SAL_BASE": "600000.00", "CCSS_OBR": "64980.00"}, "535020.00"},
		{"ingreso a mitad de mes", e2, map[string]string{"SAL_BASE": "300000.00", "CCSS_OBR": "32490.00"}, "267510.00"},
		{"cese a mitad de mes", e3, map[string]string{"SAL_BASE": "600000.00", "CCSS_OBR": "64980.00"}, "535020.00"},
		{"aumento", e4, map[string]string{"SAL_BASE": "900000.00", "CCSS_OBR": "97470.00"}, "802530.00"},
		{"incapacidad que cruza períodos", e5, map[string]string{"SAL_BASE": "606666.67", "SUB_INCAP": "11666.67", "CCSS_OBR": "65702.00"}, "552631.34"},
		{"horas extra autorizadas", e6, map[string]string{"SAL_BASE": "720000.00", "HRS_EXTRA": "45000.00", "CCSS_OBR": "82849.50"}, "682150.50"},
		{"2 pensiones + embargo judicial", e7, map[string]string{"SAL_BASE": "900000.00", "CCSS_OBR": "97470.00", "EMBARGO": "238369.69"}, "564160.31"},
		{"préstamo que se salda", e8, map[string]string{"SAL_BASE": "600000.00", "CCSS_OBR": "64980.00", "PRESTAMO": "30000.00"}, "505020.00"},
		{"deducciones > remanente", e9, map[string]string{"SAL_BASE": "300000.00", "CCSS_OBR": "32490.00", "PRESTAMO": "267510.00"}, "0.00"},
		{"devengos manuales", e10, map[string]string{"SAL_BASE": "500000.00", "COMISIONES": "100000.00", "BONO_DISCRECIONAL": "20000.00", "CCSS_OBR": "67146.00"}, "552854.00"},
		{"renta en dos tramos", e11, map[string]string{"SAL_BASE": "1500000.00", "CCSS_OBR": "162450.00", "IMP_RENTA": "65850.00"}, "1271700.00"},
	}
	for _, k := range casos {
		for cod, quiere := range k.lineas {
			if got := c.linea(marzo.ID, k.emp, cod); got != quiere {
				t.Errorf("%s: %s = %s, esperado %s", k.nombre, cod, got, quiere)
			}
		}
		if got := c.neto(marzo.ID, k.emp); got != k.neto {
			t.Errorf("%s: neto %s, esperado %s", k.nombre, got, k.neto)
		}
	}
	var insatisfecho string
	if err := c.db.QueryRow(`SELECT monto_insatisfecho FROM planilla_detalle_embargo WHERE embargo_id = ?`, embJudicial).Scan(&insatisfecho); err != nil || insatisfecho != "12262.81" {
		t.Errorf("embargo judicial insatisfecho %s (%v)", insatisfecho, err)
	}

	if _, err := c.periodos.Aprobar(ctx, c.aprueba, marzo.ID); err != nil {
		t.Fatal(err)
	}
	llavero := testutil.Llavero(t)
	exp := planilla.NewExportService(infra.NewExportRepository(c.q, llavero), infra.NewExportUnitOfWork(c.db, llavero))
	archivo, _, err := exp.GenerarArchivoDispersionBancaria(ctx, marzo.ID, c.paga)
	if err != nil {
		t.Fatalf("dispersión: %v", err)
	}
	totalArchivo, lineas := decimal.Zero, strings.Split(strings.TrimRight(string(archivo), "\r\n"), "\r\n")
	for _, l := range lineas {
		campos := strings.Split(l, ",")
		totalArchivo = totalArchivo.Add(decimal.RequireFromString(campos[len(campos)-1]))
	}
	if len(lineas) != 10 || totalArchivo.String() != "6268596.15" {
		t.Fatalf("dispersión: %d filas, total %s", len(lineas), totalArchivo)
	}
	var est8, saldo8, saldo9 string
	if err := c.db.QueryRow(`SELECT estado, saldo_pendiente FROM prestamos WHERE id = ?`, prestamo8).Scan(&est8, &saldo8); err != nil || est8 != "pagado" || saldo8 != "0.00" {
		t.Errorf("préstamo saldado: %s %s (%v)", est8, saldo8, err)
	}
	if err := c.db.QueryRow(`SELECT saldo_pendiente FROM prestamos WHERE id = ?`, prestamo9).Scan(&saldo9); err != nil || saldo9 != "732490.00" {
		t.Errorf("préstamo parcial: saldo %s (%v)", saldo9, err)
	}
	if _, err := c.periodos.Pagar(ctx, c.paga, marzo.ID); err != nil {
		t.Fatal(err)
	}
	det, err := c.detalles.DetallesParaContabilizar(ctx, marzo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.conta.ContabilizarPeriodo(ctx, marzo.ID, c.paga, det); !errors.Is(err, planilla.ErrSegregacionFunciones) {
		t.Fatalf("pagador contabiliza: %v", err)
	}
	if _, err := c.conta.ContabilizarPeriodo(ctx, marzo.ID, c.contabiliza, det); err != nil {
		t.Fatal(err)
	}
	var debe, haber string
	if err := c.db.QueryRow(`SELECT SUM(d.monto_debe), SUM(d.monto_haber) FROM asientos_contables a
JOIN asientos_contables_detalle d ON d.asiento_contable_id = a.id WHERE a.periodo_planilla_id = ?`, marzo.ID).Scan(&debe, &haber); err != nil || debe != haber {
		t.Fatalf("asientos de marzo: debe %s haber %s (%v)", debe, haber, err)
	}

	liq := rrhh.NewLiquidacionService(c.db, c.q, infra.NewMotorRepository(c.q), c.generales)
	proc, err := liq.ProcesarLiquidacion(ctx, rrhh.ProyectarSolicitud{EmpleadoID: e1, FechaSalida: fecha("2026-03-31"), MotivoSalida: "renuncia"}, c.prepara)
	if err != nil {
		t.Fatal(err)
	}
	if proc.Rubros.TotalLiquidacion.String() != "99980" || proc.CierreProgramado {
		t.Fatalf("liquidación: total %s programada %t", proc.Rubros.TotalLiquidacion, proc.CierreProgramado)
	}
	c.calcular(proc.PeriodoPlanillaID)
	if c.linea(proc.PeriodoPlanillaID, e1, "LIQ_AGUINALDO") != "50000.00" || c.neto(proc.PeriodoPlanillaID, e1) != "99980.00" {
		t.Fatalf("planilla de liquidación: aguinaldo %s neto %s", c.linea(proc.PeriodoPlanillaID, e1, "LIQ_AGUINALDO"), c.neto(proc.PeriodoPlanillaID, e1))
	}
	c.cerrarCiclo(proc.PeriodoPlanillaID)

	tipoAgui := c.id(`SELECT id FROM tipos_planilla WHERE clase = 'aguinaldo'`)
	agui, err := c.periodos.Crear(ctx, c.prepara, tipoAgui, fecha("2025-12-01"), fecha("2026-11-30"))
	if err != nil {
		t.Fatal(err)
	}
	c.calcular(agui.ID)
	var nAgui int
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM planilla_detalle_empleado WHERE periodo_planilla_id = ?`, agui.ID).Scan(&nAgui); err != nil || nAgui != 10 {
		t.Fatalf("empleados en el aguinaldo: %d (%v)", nAgui, err)
	}
	for emp, quiere := range map[int64]string{e11: "125000.00", e6: "63750.00", e5: "50555.56"} {
		if got := c.linea(agui.ID, emp, "AGUINALDO"); got != quiere {
			t.Errorf("aguinaldo del empleado %d = %s, esperado %s", emp, got, quiere)
		}
	}
	c.cerrarCiclo(agui.ID)
	var procesados int
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM aguinaldos WHERE periodo_planilla_id = ? AND estado = 'procesado'`, agui.ID).Scan(&procesados); err != nil || procesados != 10 {
		t.Fatalf("aguinaldos procesados: %d (%v)", procesados, err)
	}

	tipoEsc := c.id(`SELECT id FROM tipos_planilla WHERE clase = 'salario_escolar'`)
	esc, err := c.periodos.Crear(ctx, c.prepara, tipoEsc, fecha("2026-01-01"), fecha("2026-12-31"))
	if err != nil {
		t.Fatal(err)
	}
	c.calcular(esc.ID)
	if got := c.linea(esc.ID, e11, "SALARIO_ESCOLAR"); got != "124950.00" {
		t.Errorf("salario escolar de e11 = %s", got)
	}
	c.cerrarCiclo(esc.ID)
}

func fecha(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
