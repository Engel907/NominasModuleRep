package planilla

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
)

func periodoLiquidacion(modo string) PeriodoParaCalculo {
	return PeriodoParaCalculo{ID: 60, Inicio: fecha("2026-03-01"), Fin: fecha("2026-03-31"),
		Clase: string(ClaseLiquidacion), TipoPlanillaID: 5, DivisorDiasMes: 30, LiqPrestamosModo: modo}
}

func fuenteLiquidacion() *fuenteFalsa {
	f := fuentePrueba()
	f.liqs = map[int64][]dto.LiquidacionParaPeriodoRow{1: {{ID: 77, FechaSalida: fecha("2026-03-15"),
		MontoPreaviso: d("100"), MontoCesantia: d("200"), MontoVacaciones: d("300"), MontoAguinaldo: d("400"), MontoSalarioEscolar: d("0")}}}
	return f
}

func TestHidratarLiquidacionRubrosYFecha(t *testing.T) {
	h := NewHidratador(fuenteLiquidacion())
	e, err := h.Hidratar(context.Background(), periodoLiquidacion(LiqPrestamosCuota), 1)
	if err != nil {
		t.Fatal(err)
	}
	in := e.Input
	if in.Liquidacion == nil || in.Liquidacion.LiquidacionID != 77 || !in.FechaEfectiva.Equal(fecha("2026-03-15")) {
		t.Fatalf("liquidación/fecha efectiva: %+v %s", in.Liquidacion, in.FechaEfectiva)
	}
	if in.Liquidacion.Rubros["cesantia"].String() != "200" || in.Liquidacion.Rubros["aguinaldo"].String() != "400" {
		t.Errorf("rubros %v", in.Liquidacion.Rubros)
	}
	if len(in.Segmentos) != 0 || len(in.CreditosFiscales) != 0 || len(in.DevengosManuales) != 0 {
		t.Errorf("la liquidación no lleva salario, créditos ni devengos: %+v", in)
	}
	if e.ContratoID != 101 || e.ProgramaPresupuestarioID != 11 {
		t.Errorf("contrato %d programa %d", e.ContratoID, e.ProgramaPresupuestarioID)
	}
	if len(in.Prestamos) != 1 || in.Prestamos[0].CuotaPeriodica.String() != "25000" {
		t.Errorf("modo cuota: %+v", in.Prestamos)
	}
}

func TestHidratarLiquidacionModosPrestamo(t *testing.T) {
	h := NewHidratador(fuenteLiquidacion())
	ctx := context.Background()
	e, err := h.Hidratar(ctx, periodoLiquidacion(LiqPrestamosSaldo), 1)
	if err != nil || e.Input.Prestamos[0].CuotaPeriodica.String() != "300000" {
		t.Fatalf("modo saldo: %+v %v", e.Input.Prestamos, err)
	}
	e, err = h.Hidratar(ctx, periodoLiquidacion(LiqPrestamosNinguno), 1)
	if err != nil || len(e.Input.Prestamos) != 0 {
		t.Fatalf("modo ninguno: %+v %v", e.Input.Prestamos, err)
	}
	if _, err = h.Hidratar(ctx, periodoLiquidacion(""), 1); !errors.Is(err, ErrParametroFaltante) {
		t.Fatalf("sin modo con préstamos debe pedir LIQ_PRESTAMOS_MODO: %v", err)
	}
}

func TestHidratarLiquidacionDobleEsError(t *testing.T) {
	f := fuenteLiquidacion()
	f.liqs[1] = append(f.liqs[1], dto.LiquidacionParaPeriodoRow{ID: 78, FechaSalida: fecha("2026-03-20")})
	_, err := NewHidratador(f).Hidratar(context.Background(), periodoLiquidacion(LiqPrestamosCuota), 1)
	if err == nil || !strings.Contains(err.Error(), "2 liquidaciones") {
		t.Fatalf("dos liquidaciones pendientes deben fallar: %v", err)
	}
}

func TestReglaLiquidacion(t *testing.T) {
	nuevo := func(rubro string, liq *LiquidacionEmpleadoInput) (*ContextoRegla, *estadoCalculo) {
		st := &estadoCalculo{formulas: map[int64]int64{}, usados: map[string]bool{}, res: &ResultadoEmpleadoOutput{}}
		c := ConceptoAplicable{TipoFormula: formulaLiquidacion, Rubro: rubro, Naturaleza: naturalezaIngreso}
		c.ConceptoSalarialID, c.Codigo = 9, "LIQ_CESANTIA"
		return &ContextoRegla{Concepto: c, In: CalculoEmpleadoInput{Liquidacion: liq}, St: st}, st
	}
	liq := &LiquidacionEmpleadoInput{LiquidacionID: 1, Rubros: map[string]decimal.Decimal{"cesantia": d("1000.005"), "preaviso": d("0")}}
	rc, st := nuevo("cesantia", liq)
	if err := (reglaLiquidacion{}).Aplicar(rc); err != nil {
		t.Fatal(err)
	}
	if len(st.lineas) != 1 || st.lineas[0].Monto.String() != "1000.01" || !st.usados[formulaLiquidacion] {
		t.Fatalf("líneas %+v", st.lineas)
	}
	rc, st = nuevo("preaviso", liq)
	if err := (reglaLiquidacion{}).Aplicar(rc); err != nil || len(st.lineas) != 0 {
		t.Fatalf("rubro en cero: %v %+v", err, st.lineas)
	}
	rc, _ = nuevo("bono", liq)
	if err := (reglaLiquidacion{}).Aplicar(rc); err == nil {
		t.Fatal("rubro desconocido debe fallar")
	}
	rc, _ = nuevo("cesantia", nil)
	if err := (reglaLiquidacion{}).Aplicar(rc); err == nil {
		t.Fatal("concepto LIQ_* sin liquidación debe fallar")
	}
}
