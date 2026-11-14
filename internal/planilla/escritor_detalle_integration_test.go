//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

var tablasDetalle = []string{
	"planilla_detalle_incapacidad", "planilla_detalle_prestamo", "planilla_detalle_embargo",
	"planilla_detalle_patronal", "planilla_detalle_concepto", "planilla_detalle_empleado",
}

func (f *fixtureCalculo) linea(codigo, naturaleza, monto string) ConceptoCalculadoOutput {
	return ConceptoCalculadoOutput{ConceptoSalarialID: f.concepto[codigo], Codigo: codigo, Naturaleza: naturaleza, Monto: d(monto)}
}

// resultadosManuales: 3 empleados con embargo, préstamo e incapacidad; los
// montos se fijan a mano y cuadran entre sí.
func (f *fixtureCalculo) resultadosManuales() []ResultadoPersistible {
	pat := func(base, monto string) []LineaPatronal {
		return []LineaPatronal{{ConceptoSalarialID: f.concepto["CCSS_PAT"], Base: d(base), Porcentaje: d("0.2683"), Monto: d(monto)}}
	}
	det := func(emp int64, ing, ded string, lineas ...ConceptoCalculadoOutput) *CalculoEmpleadoDetalle {
		r := ResultadoEmpleadoOutput{EmpleadoID: emp, TotalIngresos: d(ing), TotalDeducciones: d(ded), NetoPagado: d(ing).Sub(d(ded)),
			BaseCCSS: d(ing), BaseRenta: d(ing), DesglosePorConcepto: lineas}
		return &CalculoEmpleadoDetalle{ResultadoEmpleadoOutput: r, SalarioBasePeriodo: lineas[0].Monto, FormulaPorConcepto: map[int64]int64{}}
	}

	d0 := det(f.emp[0], "900000", "197470",
		f.linea("SAL_BASE", "ingreso", "900000"), f.linea("CCSS_OBR", "deduccion", "97470"), f.linea("EMBARGO", "deduccion", "100000"))
	d0.CuotaObreroCCSS, d0.CuotaPatronalCCSS = d("97470"), d("241470")
	d0.EmbargosAplicados = []EmbargoAplicadoOutput{{EmbargoID: f.embargoID, Prioridad: 1, MontoCalculado: d("100000"), MontoAplicado: d("100000")}}
	d0.Advertencias = []string{"advertencia de prueba"}

	d1 := det(f.emp[1], "800000", "111640",
		f.linea("SAL_BASE", "ingreso", "800000"), f.linea("CCSS_OBR", "deduccion", "86640"), f.linea("PRESTAMO", "deduccion", "25000"))
	d1.CuotaPatronalCCSS = d("214640")

	d2 := det(f.emp[2], "700000", "75810",
		f.linea("SAL_BASE", "ingreso", "700000"), f.linea("CCSS_OBR", "deduccion", "75810"))
	d2.CuotaPatronalCCSS = d("187810")
	d2.DesglosePorConcepto[0].Cantidad = d("30")

	return []ResultadoPersistible{
		{Detalle: d0, ContratoID: f.contrato[0], ProgramaPresupuestarioID: f.programaID, Patronal: pat("900000", "241470")},
		{Detalle: d1, ContratoID: f.contrato[1], ProgramaPresupuestarioID: f.programaID, Patronal: pat("800000", "214640"),
			Prestamos: []LineaPrestamo{{PrestamoID: f.prestamoID, CuotaCalculada: d("25000"), MontoRetenido: d("25000")}}},
		{Detalle: d2, ContratoID: f.contrato[2], ProgramaPresupuestarioID: f.programaID, Patronal: pat("700000", "187810"),
			Incapacidad: []LineaIncapacidad{{IncapacidadID: f.incapID, DiaSerieDesde: 3, DiaSerieHasta: 6, Dias: d("4"),
				MontoPatronal: decimal.Zero, MontoEntidadInformativo: d("50000")}}},
	}
}

func (f *fixtureCalculo) contar(t *testing.T, periodoID int64) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, tabla := range tablasDetalle {
		q := `SELECT COUNT(*) FROM ` + tabla + ` h JOIN planilla_detalle_empleado pde ON pde.id = h.planilla_detalle_empleado_id WHERE pde.periodo_planilla_id = ?`
		if tabla == "planilla_detalle_empleado" {
			q = `SELECT COUNT(*) FROM planilla_detalle_empleado WHERE periodo_planilla_id = ?`
		}
		var n int
		if err := f.db.QueryRow(q, periodoID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[tabla] = n
	}
	return out
}

func TestEscribirYEliminarDetalle(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.crearIncapacidad(t)
	ctx := context.Background()
	svc := NewPeriodoService(f.db, PeriodoGanchos{})
	p, err := svc.Crear(ctx, f.preparador, f.tipoMensual, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	w := &EscritorDetalle{}

	err = db.WithAuditContext(ctx, f.db, f.preparador, func(ctx context.Context, q *dto.Queries) error {
		if err := w.Escribir(ctx, q, p.ID, f.resultadosManuales()); err != nil {
			return err
		}
		return w.GuardarSnapshot(ctx, q, p.ID, []byte(`{"prueba":true}`), "planilla-motor-test")
	})
	if err != nil {
		t.Fatalf("Escribir: %v", err)
	}

	var cuadra int
	if err := f.db.QueryRow(`SELECT fn_detalle_cuadra(?)`, p.ID).Scan(&cuadra); err != nil || cuadra != 1 {
		t.Fatalf("fn_detalle_cuadra = %d, %v; se esperaba 1", cuadra, err)
	}
	n := f.contar(t, p.ID)
	want := map[string]int{"planilla_detalle_empleado": 3, "planilla_detalle_concepto": 8, "planilla_detalle_patronal": 3,
		"planilla_detalle_embargo": 1, "planilla_detalle_prestamo": 1, "planilla_detalle_incapacidad": 1}
	for k, v := range want {
		if n[k] != v {
			t.Errorf("%s: %d filas, se esperaban %d", k, n[k], v)
		}
	}
	var adv, version string
	if err := f.db.QueryRow(`SELECT JSON_UNQUOTE(JSON_EXTRACT(advertencias, '$[0]')) FROM planilla_detalle_empleado WHERE periodo_planilla_id = ? AND empleado_id = ?`,
		p.ID, f.emp[0]).Scan(&adv); err != nil || adv != "advertencia de prueba" {
		t.Errorf("advertencias = %q, %v", adv, err)
	}
	if err := f.db.QueryRow(`SELECT version_motor FROM periodos_planilla WHERE id = ?`, p.ID).Scan(&version); err != nil || version != "planilla-motor-test" {
		t.Errorf("version_motor = %q, %v", version, err)
	}

	err = db.WithAuditContext(ctx, f.db, f.preparador, func(ctx context.Context, q *dto.Queries) error {
		return w.EliminarDetalle(ctx, q, p.ID)
	})
	if err != nil {
		t.Fatalf("EliminarDetalle: %v", err)
	}
	for tabla, c := range f.contar(t, p.ID) {
		if c != 0 {
			t.Errorf("%s conserva %d filas tras EliminarDetalle", tabla, c)
		}
	}
}

func TestDescuadreEnMemoriaNoInserta(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.crearIncapacidad(t)
	ctx := context.Background()
	p, err := NewPeriodoService(f.db, PeriodoGanchos{}).Crear(ctx, f.preparador, f.tipoMensual,
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	rs := f.resultadosManuales()
	rs[1].Patronal = nil
	err = db.WithAuditContext(ctx, f.db, f.preparador, func(ctx context.Context, q *dto.Queries) error {
		return (&EscritorDetalle{}).Escribir(ctx, q, p.ID, rs)
	})
	var e ErrDetalleNoCuadra
	if !errors.As(err, &e) || e.EmpleadoID != f.emp[1] {
		t.Fatalf("se esperaba ErrDetalleNoCuadra del empleado %d, se obtuvo %v", f.emp[1], err)
	}
	if n := f.contar(t, p.ID)["planilla_detalle_empleado"]; n != 0 {
		t.Errorf("con descuadre no debe insertarse nada; hay %d filas", n)
	}
}
