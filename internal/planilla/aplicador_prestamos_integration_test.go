//go:build integration

package planilla

import (
	"context"
	"errors"
	"sync"
	"testing"

	"planillas-ccdr/internal/db/dto"
)

func (f *fixtureCalculo) saldoYEstado(t *testing.T) (string, string) {
	t.Helper()
	var saldo, estado string
	if err := f.db.QueryRow(`SELECT saldo_pendiente, estado FROM prestamos WHERE id = ?`, f.prestamoID).Scan(&saldo, &estado); err != nil {
		t.Fatal(err)
	}
	return saldo, estado
}

func (f *fixtureCalculo) calcularMarzo(t *testing.T, svc *PeriodoService) dto.PeriodoCicloRow {
	t.Helper()
	p := marzo(t, svc, f)
	p, err := svc.Calcular(context.Background(), f.preparador, p.ID)
	if err != nil {
		var ce *ErrCalculoConErrores
		if errors.As(err, &ce) {
			t.Fatalf("calcular: %+v", ce.Errores)
		}
		t.Fatalf("calcular: %v", err)
	}
	return p
}

// el saldo baja al aprobar, exactamente una vez; calcular/recalcular no lo toca.
func TestAprobarAplicaPrestamoUnaVez(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	svc := f.servicio()
	ctx := context.Background()

	p := f.calcularMarzo(t, svc)
	if _, err := svc.Recalcular(ctx, f.preparador, p.ID); err != nil {
		t.Fatalf("recalcular: %v", err)
	}
	if saldo, _ := f.saldoYEstado(t); saldo != "300000.00" {
		t.Fatalf("calcular/recalcular no deben tocar el saldo: %s", saldo)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Aprobar(ctx, f.aprobador, p.ID)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, e := range errs {
		switch {
		case e == nil:
			ok++
		case errors.Is(e, ErrEstadoInvalido):
		default:
			t.Errorf("error inesperado: %v", e)
		}
	}
	if ok != 1 {
		t.Fatalf("aprobaciones exitosas = %d, esperado 1 (%v)", ok, errs)
	}
	if saldo, estado := f.saldoYEstado(t); saldo != "275000.00" || estado != "activo" {
		t.Fatalf("saldo/estado tras aprobar = %s/%s, esperado 275000.00/activo", saldo, estado)
	}
	var aplicado int
	if err := f.db.QueryRow(`
SELECT COUNT(*) FROM planilla_detalle_prestamo pdp
JOIN planilla_detalle_empleado pde ON pde.id = pdp.planilla_detalle_empleado_id
WHERE pde.periodo_planilla_id = ? AND pdp.aplicado_en IS NOT NULL`, p.ID).Scan(&aplicado); err != nil || aplicado != 1 {
		t.Fatalf("detalles aplicados = %d, %v", aplicado, err)
	}

	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewAplicadorPrestamos().AlAprobar(ctx, dto.New(tx), p); err != nil {
		t.Fatalf("reintento: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if saldo, _ := f.saldoYEstado(t); saldo != "275000.00" {
		t.Fatalf("el reintento duplicó la aplicación: saldo %s", saldo)
	}
}

func TestPrestamoSaldadoQuedaPagado(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	mustExec(t, f.db, `UPDATE prestamos SET saldo_pendiente = 10000 WHERE id = ?`, f.prestamoID)
	svc := f.servicio()
	p := f.calcularMarzo(t, svc)
	if _, err := svc.Aprobar(context.Background(), f.aprobador, p.ID); err != nil {
		t.Fatalf("aprobar: %v", err)
	}
	if saldo, estado := f.saldoYEstado(t); saldo != "0.00" || estado != "pagado" {
		t.Fatalf("saldo/estado = %s/%s, esperado 0.00/pagado", saldo, estado)
	}
}

func TestPrestamoCambiadoTrasCalcularBloqueaAprobacion(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	svc := f.servicio()
	p := f.calcularMarzo(t, svc)
	mustExec(t, f.db, `UPDATE prestamos SET estado = 'suspendido' WHERE id = ?`, f.prestamoID)
	if _, err := svc.Aprobar(context.Background(), f.aprobador, p.ID); !errors.Is(err, ErrPrestamoNoAplicable) {
		t.Fatalf("se esperaba prestamo_no_aplicable, se obtuvo %v", err)
	}
	var estado string
	if err := f.db.QueryRow(`SELECT estado FROM periodos_planilla WHERE id = ?`, p.ID).Scan(&estado); err != nil || estado != string(EstadoCalculada) {
		t.Fatalf("la aprobación fallida debe revertirse: estado %s, %v", estado, err)
	}
}
