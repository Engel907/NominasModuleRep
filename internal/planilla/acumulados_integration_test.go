//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"
	"time"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
)

// mesAprobado crea, calcula y aprueba el período mensual regular [inicio, fin].
func (f *fixtureCalculo) mesAprobado(t *testing.T, svc *PeriodoService, inicio, fin string) {
	t.Helper()
	ctx := context.Background()
	p, err := svc.Crear(ctx, f.preparador, f.tipoMensual, fecha(inicio), fecha(fin))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
		var ce *ErrCalculoConErrores
		if errors.As(err, &ce) {
			t.Fatalf("calcular %s: %+v", inicio, ce.Errores)
		}
		t.Fatalf("calcular %s: %v", inicio, err)
	}
	if _, err := svc.Aprobar(ctx, f.aprobador, p.ID); err != nil {
		t.Fatalf("aprobar %s: %v", inicio, err)
	}
}

func (f *fixtureCalculo) ejecutor(svc *PeriodoService, hoy string) *EjecutorAcumulados {
	e := NewEjecutorAcumulados(svc, dto.New(f.db), NewVentanas(parametros.NewGenerales(parametros.NewRepositorioSQL(f.db), 0)))
	e.ahora = func() time.Time { return fecha(hoy) }
	return e
}

// aguinaldo y salario escolar por el ciclo normal del período.
func TestAguinaldoYSalarioEscolarEnElCiclo(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	svc := f.servicio()
	ctx := context.Background()

	mustExec(t, f.db, `UPDATE contratos SET fecha_inicio_contrato = '2026-11-01', fecha_inicio_vigencia = '2026-11-01' WHERE id = ?`, f.contrato[2])
	mustExec(t, f.db, `UPDATE empleado_datos_laborales SET fecha_inicio_vigencia = '2026-11-01' WHERE empleado_id = ?`, f.emp[2])
	mustExec(t, f.db, `
INSERT INTO incapacidades (empleado_id, entidad_emisora, numero_boleta, fecha_inicio, fecha_fin, dias, creado_por)
VALUES (?, 'CCSS', 'B-AG-1', '2026-11-10', '2026-11-12', 3, ?)`, f.emp[1], f.preparador)

	f.mesAprobado(t, svc, "2026-10-01", "2026-10-31")
	f.mesAprobado(t, svc, "2026-11-01", "2026-11-30")

	if _, err := f.ejecutor(svc, "2026-11-12").Ejecutar(ctx, f.preparador, ClaseAguinaldo, 2026); !errors.Is(err, ErrFueraDeVentana) {
		t.Fatalf("fuera de ventana: %v", err)
	}
	ej := f.ejecutor(svc, "2026-12-05")
	p, err := ej.Ejecutar(ctx, f.preparador, ClaseAguinaldo, 2026)
	if err != nil {
		t.Fatalf("ejecutar aguinaldo: %v", err)
	}
	if !p.FechaInicio.Equal(fecha("2025-12-01")) || !p.FechaFin.Equal(fecha("2026-11-30")) {
		t.Fatalf("fechas del período de aguinaldo: %s – %s", p.FechaInicio, p.FechaFin)
	}
	if _, err := ej.Ejecutar(ctx, f.preparador, ClaseAguinaldo, 2026); !errors.Is(err, ErrAcumuladoYaEjecutado) {
		t.Fatalf("doble ejecución: %v", err)
	}

	if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
		var ce *ErrCalculoConErrores
		if errors.As(err, &ce) {
			t.Fatalf("calcular aguinaldo: %+v", ce.Errores)
		}
		t.Fatalf("calcular aguinaldo: %v", err)
	}
	monto := func(emp int64) (string, string, string) {
		var m, base, estado string
		if err := f.db.QueryRow(`SELECT monto_aguinaldo, total_salarios_brutos, estado FROM aguinaldos WHERE empleado_id = ? AND anio = 2026 AND periodo_planilla_id = ?`,
			emp, p.ID).Scan(&m, &base, &estado); err != nil {
			t.Fatalf("aguinaldo del empleado %d: %v", emp, err)
		}
		return m, base, estado
	}
	if m, b, e := monto(f.emp[0]); m != "150000.00" || b != "1800000.00" || e != "proyectado" {
		t.Errorf("emp[0] aguinaldo %s base %s estado %s", m, b, e)
	}
	if m, b, _ := monto(f.emp[2]); m != "58333.33" || b != "700000.00" {
		t.Errorf("emp[2] (ingreso en noviembre) aguinaldo %s base %s", m, b)
	}
	var baseEsperada string
	if err := f.db.QueryRow(`
SELECT SUM(pde.base_aguinaldo) FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
WHERE pde.empleado_id = ? AND pp.tipo_planilla_id = ?`, f.emp[1], f.tipoMensual).Scan(&baseEsperada); err != nil {
		t.Fatal(err)
	}
	if _, b, _ := monto(f.emp[1]); b != baseEsperada || b == "1600000.00" {
		t.Errorf("emp[1] base %s, esperado %s (< 1 600 000 por la incapacidad)", b, baseEsperada)
	}
	var lineas int
	var cargas string
	if err := f.db.QueryRow(`
SELECT COUNT(*), CAST(COALESCE(SUM(pde.cuota_obrera_ccss + pde.renta_neta), 0) AS CHAR) FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
WHERE pde.periodo_planilla_id = ? AND cs.codigo <> 'AGUINALDO'`, p.ID).Scan(&lineas, &cargas); err != nil || lineas != 0 || (cargas != "0" && cargas != "0.00") {
		t.Errorf("el período de aguinaldo solo debe tener líneas AGUINALDO (otras=%d, cargas=%s, %v)", lineas, cargas, err)
	}

	if _, err := svc.Recalcular(ctx, f.preparador, p.ID); err != nil {
		t.Fatalf("recalcular aguinaldo: %v", err)
	}
	var filas int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM aguinaldos WHERE anio = 2026`).Scan(&filas); err != nil || filas != 3 {
		t.Fatalf("filas de aguinaldo tras recalcular = %d, %v", filas, err)
	}
	if _, err := svc.Aprobar(ctx, f.aprobador, p.ID); err != nil {
		t.Fatalf("aprobar aguinaldo: %v", err)
	}
	if _, _, e := monto(f.emp[0]); e != "procesado" {
		t.Errorf("tras aprobar el aguinaldo debe quedar procesado: %s", e)
	}

	pe, err := f.ejecutor(svc, "2027-01-15").Ejecutar(ctx, f.preparador, ClaseSalarioEscolar, 2026)
	if err != nil {
		t.Fatalf("ejecutar salario escolar: %v", err)
	}
	if _, err := svc.Calcular(ctx, f.preparador, pe.ID); err != nil {
		t.Fatalf("calcular salario escolar: %v", err)
	}
	var se, pct string
	if err := f.db.QueryRow(`SELECT monto_salario_escolar, porcentaje_aplicado FROM salarios_escolares WHERE empleado_id = ? AND anio = 2026`, f.emp[0]).
		Scan(&se, &pct); err != nil || se != "149940.00" || pct != "0.0833" {
		t.Errorf("salario escolar emp[0] = %s (%s), %v; esperado 149940.00 (1 800 000 × 0.0833)", se, pct, err)
	}
	if _, err := svc.Descartar(ctx, f.preparador, pe.ID); err != nil {
		t.Fatalf("descartar: %v", err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM salarios_escolares`).Scan(&filas); err != nil || filas != 0 {
		t.Errorf("descartar debe borrar los salarios escolares proyectados: %d, %v", filas, err)
	}
}

// Un período de clase aguinaldo con fechas distintas a su ventana no se calcula.
func TestPeriodoAguinaldoFueraDeVentanaNoSeCalcula(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	svc := f.servicio()
	var tipo int64
	if err := f.db.QueryRow(`SELECT id FROM tipos_planilla WHERE clase = 'aguinaldo'`).Scan(&tipo); err != nil {
		t.Fatal(err)
	}
	p, err := svc.Crear(context.Background(), f.preparador, tipo, fecha("2026-01-01"), fecha("2026-11-30"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Calcular(context.Background(), f.preparador, p.ID); !errors.Is(err, ErrPeriodoNoEsVentana) {
		t.Fatalf("se esperaba periodo_no_es_ventana, se obtuvo %v", err)
	}
}
