package planilla

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func repoConPrestamo() fakeMotorRepo {
	r := repoBase()
	r.conceptos = append(r.conceptos, fila(6, "PRESTAMO", "deduccion", 130, `{"tipo":"prestamo"}`, false, false))
	return r
}

func inputTope(salario string, prestamos ...PrestamoDeduccionInput) CalculoEmpleadoInput {
	return CalculoEmpleadoInput{EmpleadoID: 7, RegimenCCSSID: 1, FechaEfectiva: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("30"), Segmentos: []SegmentoSalario{{ContratoID: 1, Salario: d(salario), Dias: 30}}, Prestamos: prestamos}
}

func TestTopeNetoGolden(t *testing.T) {
	ctx := context.Background()

	t.Run("deducciones sobre el remanente: neto 0 y préstamo parcial", func(t *testing.T) {
		in := inputTope("100000",
			PrestamoDeduccionInput{PrestamoID: 1, CuotaPeriodica: d("50000"), SaldoPendiente: d("300000")},
			PrestamoDeduccionInput{PrestamoID: 2, CuotaPeriodica: d("40000"), SaldoPendiente: d("300000")})
		r, err := NewMotorEmpleado(repoConPrestamo()).CalcularEmpleado(ctx, in, params())
		if err != nil {
			t.Fatal(err)
		}
		if !r.NetoPagado.IsZero() || !r.TotalDeducciones.Equal(d("100000")) || !r.DeduccionesPrestamos.Equal(d("74826.64")) {
			t.Errorf("neto %s, deducciones %s, préstamos %s", r.NetoPagado, r.TotalDeducciones, r.DeduccionesPrestamos)
		}
		if len(r.Prestamos) != 2 || !r.Prestamos[0].MontoRetenido.Equal(d("50000")) ||
			!r.Prestamos[1].MontoRetenido.Equal(d("24826.64")) || !r.Prestamos[1].CuotaCalculada.Equal(d("40000")) {
			t.Errorf("líneas de préstamo: %+v", r.Prestamos)
		}
		if len(r.Advertencias) != 1 || !strings.Contains(r.Advertencias[0], "PRESTAMO") {
			t.Errorf("advertencias: %v", r.Advertencias)
		}
		if err := verificarCuadre(ResultadoPersistible{Detalle: r, ContratoID: 1, ProgramaPresupuestarioID: 1, Patronal: r.Patronal, Prestamos: r.Prestamos}); err != nil {
			t.Errorf("cuadre: %v", err)
		}
	})

	t.Run("todas caben", func(t *testing.T) {
		in := inputTope("1000000", PrestamoDeduccionInput{PrestamoID: 1, CuotaPeriodica: d("25000"), SaldoPendiente: d("300000")})
		r, err := NewMotorEmpleado(repoConPrestamo()).CalcularEmpleado(ctx, in, params())
		if err != nil {
			t.Fatal(err)
		}
		if !r.NetoPagado.Equal(d("858500")) || !r.Prestamos[0].MontoRetenido.Equal(d("25000")) || len(r.Advertencias) != 0 {
			t.Errorf("neto %s, retenido %s, advertencias %v", r.NetoPagado, r.Prestamos[0].MontoRetenido, r.Advertencias)
		}
	})

	t.Run("préstamo que no cabe en nada queda en 0 y sale del desglose", func(t *testing.T) {
		st := &estadoCalculo{res: &ResultadoEmpleadoOutput{}, totalIng: d("1000"), conceptoPrestamoID: 6,
			prestamos: []LineaPrestamo{{PrestamoID: 1, CuotaCalculada: d("500"), MontoRetenido: d("500")}},
			lineas: []ConceptoCalculadoOutput{
				{ConceptoSalarialID: 1, Codigo: "SAL_BASE", Naturaleza: naturalezaIngreso, Monto: d("1000")},
				{ConceptoSalarialID: 4, Codigo: "CCSS_OBR", Naturaleza: naturalezaDeduccion, Monto: d("1000"), EsDeduccionObligatoria: true},
				{ConceptoSalarialID: 6, Codigo: "PRESTAMO", Naturaleza: naturalezaDeduccion, Monto: d("500")},
			}}
		if err := AplicarTopeNeto(st); err != nil {
			t.Fatal(err)
		}
		if len(st.lineas) != 2 || !st.totalDed.Equal(d("1000")) || !st.prestamos[0].MontoRetenido.IsZero() {
			t.Errorf("líneas %+v, deducciones %s, préstamo %+v", st.lineas, st.totalDed, st.prestamos)
		}
	})

	t.Run("solo obligatorias que superan los ingresos: error", func(t *testing.T) {
		if _, err := NewMotorEmpleado(repoConPrestamo()).CalcularEmpleado(ctx, inputTope("10000"), params()); err == nil {
			t.Fatal("obligatorias > ingresos debe dar error")
		}
		st := &estadoCalculo{res: &ResultadoEmpleadoOutput{}, totalIng: decimal.Zero}
		if err := AplicarTopeNeto(st); err != nil {
			t.Errorf("sin líneas no hay error: %v", err)
		}
	})
}
