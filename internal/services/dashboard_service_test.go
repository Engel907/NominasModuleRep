package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type repoFalso struct {
	fila MetricasDashboardRow
	err  error
	got  time.Time
}

func (r *repoFalso) ObtenerMetricasDashboard(_ context.Context, f time.Time) (MetricasDashboardRow, error) {
	r.got = f
	return r.fila, r.err
}

func TestObtenerMetricasGenerales(t *testing.T) {
	ref := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	casos := []struct {
		nombre    string
		repo      *repoFalso
		wantErr   bool
		wantIDNil bool
		wantID    int64
	}{
		{"sin período cerrado → nil, no 0", &repoFalso{fila: MetricasDashboardRow{TotalEmpleadosActivos: 5}}, false, true, 0},
		{"con período cerrado", &repoFalso{fila: MetricasDashboardRow{
			UltimoPeriodoCerradoID: sql.NullInt64{Int64: 7, Valid: true}, CostoUltimaPlanillaCerrada: decimal.RequireFromString("100.25")}}, false, false, 7},
		{"error del repo se envuelve", &repoFalso{err: errors.New("boom")}, true, false, 0},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dto, err := NewDashboardService(c.repo, nil, 0).ObtenerMetricasGenerales(context.Background(), ref)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v", err)
			}
			if c.wantErr {
				if !errors.Is(err, c.repo.err) {
					t.Errorf("no preserva la causa: %v", err)
				}
				return
			}
			if !c.repo.got.Equal(ref) {
				t.Errorf("fechaReferencia no se propagó: %v", c.repo.got)
			}
			if c.wantIDNil != (dto.UltimoPeriodoCerradoID == nil) || (!c.wantIDNil && *dto.UltimoPeriodoCerradoID != c.wantID) {
				t.Errorf("UltimoPeriodoCerradoID=%v", dto.UltimoPeriodoCerradoID)
			}
		})
	}
}
