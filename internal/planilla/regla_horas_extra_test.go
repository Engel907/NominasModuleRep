package planilla

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func inputHorasExtra() CalculoEmpleadoInput {
	return CalculoEmpleadoInput{EmpleadoID: 7, FechaEfectiva: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		DiasTrabajados: d("30"), Segmentos: mesCompleto(), HorasExtra: []HoraExtraInput{{1, d("8")}}}
}

// L12: el divisor de la hora sale de la jornada del contrato.
func TestHorasExtraPorJornada(t *testing.T) {
	// Diurna (divisor 240) con factor 1.5: 1 000 000 / 240 × 8 × 1.5 = 50 000.
	r, err := NewMotorEmpleado(repoBase()).CalcularEmpleado(context.Background(), inputHorasExtra(), params())
	if err != nil {
		t.Fatal(err)
	}
	var he decimal.Decimal
	for _, l := range r.DesglosePorConcepto {
		if l.Codigo == "HRS_EXTRA" {
			he = l.Monto
		}
	}
	if !he.Equal(d("50000")) {
		t.Errorf("HRS_EXTRA diurna = %s, esperado 50000", he)
	}

	// Jornada nocturna (id 3) sin parametros_jornada vigente: error explícito.
	repo := repoBase()
	repo.contrato.Jornada, repo.contrato.JornadaID = "NOCTURNA", ip(3)
	if _, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), inputHorasExtra(), params()); err == nil {
		t.Error("jornada sin divisor vigente debe dar error")
	}

	// Contrato cuyo texto de jornada no calzó con el catálogo: error.
	repo = repoBase()
	repo.contrato.Jornada, repo.contrato.JornadaID = "TIEMPO COMPLETO", nil
	if _, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), inputHorasExtra(), params()); err == nil {
		t.Error("contrato sin jornada_id debe dar error si tiene horas extra")
	}

	// Sin horas extra la jornada no importa.
	sinHoras := inputHorasExtra()
	sinHoras.HorasExtra = nil
	if _, err := NewMotorEmpleado(repo).CalcularEmpleado(context.Background(), sinHoras, params()); err != nil {
		t.Errorf("sin horas extra no debe exigir jornada: %v", err)
	}
}
