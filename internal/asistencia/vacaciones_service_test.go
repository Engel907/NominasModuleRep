package asistencia

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
)

// --- fakes -------------------------------------------------------------------

type fakeVacQueries struct {
	emp      dto.Empleado
	escala   dto.ParametrosVacacionesEscala
	escalaEr error
	saldo    dto.VacacionesAcumulado
	saldoEr  error
	cal      []dto.ListarCalendarioPorRangoRow
}

func (f *fakeVacQueries) GetEmpleadoByID(context.Context, int64) (dto.Empleado, error) {
	return f.emp, nil
}
func (f *fakeVacQueries) ObtenerEscalaVacacionesVigentePorAntiguedad(context.Context, decimal.Decimal) (dto.ParametrosVacacionesEscala, error) {
	return f.escala, f.escalaEr
}
func (f *fakeVacQueries) ObtenerSaldoVacacionesEmpleado(context.Context, int64) (dto.VacacionesAcumulado, error) {
	return f.saldo, f.saldoEr
}
func (f *fakeVacQueries) ListarCalendarioPorRango(context.Context, time.Time, time.Time) ([]dto.ListarCalendarioPorRangoRow, error) {
	return f.cal, nil
}

// fakeVacTx implementa vacTx en memoria; solo lo que usan los tests.
type fakeVacTx struct {
	yaAcumulado  bool
	saldo        dto.VacacionesAcumulado
	comprometido decimal.Decimal
	traslapes    int64
	solicitud    dto.VacacionesSolicitud

	movimientos int
	upserts     int
	creadas     int
}

func (t *fakeVacTx) BloquearEmpleado(context.Context, int64) error { return nil }
func (t *fakeVacTx) ExisteMovimientoAcumulacionVacaciones(context.Context, int64, string) (bool, error) {
	return t.yaAcumulado, nil
}
func (t *fakeVacTx) ExisteMovimientoVacacionesPorSolicitud(context.Context, int64, string) (bool, error) {
	return false, nil
}
func (t *fakeVacTx) ObtenerSaldoVacacionesEmpleado(context.Context, int64) (dto.VacacionesAcumulado, error) {
	return t.saldo, nil
}
func (t *fakeVacTx) ObtenerSaldoVacacionesEmpleadoForUpdate(context.Context, int64) (dto.VacacionesAcumulado, error) {
	return t.saldo, nil
}
func (t *fakeVacTx) UpsertSaldoVacacionesEmpleado(context.Context, int64, time.Time, decimal.Decimal, decimal.Decimal, decimal.Decimal, sql.NullInt64) error {
	t.upserts++
	return nil
}
func (t *fakeVacTx) InsertarMovimientoVacaciones(context.Context, int64, string, decimal.Decimal, time.Time, sql.NullString, sql.NullInt64, sql.NullString, int64) (dto.VacacionesMovimiento, error) {
	t.movimientos++
	return dto.VacacionesMovimiento{}, nil
}
func (t *fakeVacTx) ContarSolicitudesVacacionesTraslapadas(context.Context, int64, time.Time, time.Time, int64) (int64, error) {
	return t.traslapes, nil
}
func (t *fakeVacTx) ContarSolicitudesVacacionesAprobadasTraslapadas(context.Context, int64, time.Time, time.Time, int64) (int64, error) {
	return t.traslapes, nil
}
func (t *fakeVacTx) SumarDiasVacacionesAprobadasSinDisfrutar(context.Context, int64, int64) (decimal.Decimal, error) {
	return t.comprometido, nil
}
func (t *fakeVacTx) CrearSolicitudVacaciones(_ context.Context, _ int64, _, _ time.Time, dias decimal.Decimal, _ int64) (dto.VacacionesSolicitud, error) {
	t.creadas++
	return dto.VacacionesSolicitud{DiasSolicitados: dias}, nil
}
func (t *fakeVacTx) ObtenerSolicitudVacacionesForUpdate(context.Context, int64) (dto.VacacionesSolicitud, error) {
	return t.solicitud, nil
}
func (t *fakeVacTx) ResolverSolicitudVacaciones(_ context.Context, _ int64, estado string, _ int64, _ sql.NullString) (dto.VacacionesSolicitud, error) {
	s := t.solicitud
	s.Estado = estado
	return s, nil
}

func nuevoSvc(q *fakeVacQueries, tx *fakeVacTx) *VacacionesService {
	return &VacacionesService{
		queries: q,
		runTx: func(ctx context.Context, _ int64, fn func(context.Context, vacTx) error) error {
			return fn(ctx, tx)
		},
	}
}

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func fecha(y int, m time.Month, dd int) time.Time {
	return time.Date(y, m, dd, 0, 0, 0, 0, time.UTC)
}

// --- tests -------------------------------------------------------------------

func TestAcumularVacaciones(t *testing.T) {
	casos := []struct {
		nombre      string
		yaExiste    bool
		periodo     string
		wantErr     error
		wantMovs    int
		wantUpserts int
	}{
		{"primera acumulación del período", false, "2026", nil, 1, 1},
		{"acumulación repetida no escribe", true, "2026", ErrAcumulacionYaRegistrada, 0, 0},
		{"período vacío", false, "  ", ErrVacPeriodoReferenciaInvalid, 0, 0},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			q := &fakeVacQueries{
				emp:    dto.Empleado{FechaIngreso: fecha(2025, 1, 15), Activo: true},
				escala: dto.ParametrosVacacionesEscala{DiasVacaciones: d("12")},
			}
			tx := &fakeVacTx{yaAcumulado: c.yaExiste}
			_, err := nuevoSvc(q, tx).AcumularVacaciones(context.Background(), AcumularVacacionesInput{
				EmpleadoID: 1, PeriodoReferencia: c.periodo, FechaEfectiva: fecha(2026, 1, 1), CreadoPor: 9,
			})
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, quiero %v", err, c.wantErr)
			}
			if tx.movimientos != c.wantMovs || tx.upserts != c.wantUpserts {
				t.Fatalf("movimientos=%d upserts=%d, quiero %d/%d", tx.movimientos, tx.upserts, c.wantMovs, c.wantUpserts)
			}
		})
	}
}

func TestSolicitarVacaciones_CalendarioIncompleto(t *testing.T) {
	// Pide lun 5 a mié 7 oct 2026 pero el calendario solo trae lun y mié.
	q := &fakeVacQueries{
		emp: dto.Empleado{Activo: true},
		cal: []dto.ListarCalendarioPorRangoRow{
			{CalendarioLaboralDia: dto.CalendarioLaboralDia{Fecha: fecha(2026, 10, 5), EsHabil: true}},
			{CalendarioLaboralDia: dto.CalendarioLaboralDia{Fecha: fecha(2026, 10, 7), EsHabil: true}},
		},
	}
	tx := &fakeVacTx{}
	_, err := nuevoSvc(q, tx).SolicitarVacaciones(context.Background(), SolicitarVacacionesInput{
		EmpleadoID: 1, FechaInicio: fecha(2026, 10, 5), FechaFin: fecha(2026, 10, 7), CreadoPor: 9,
		PuedeSolicitarTerceros: true,
	})
	var inc *ErrCalendarioIncompleto
	if !errors.As(err, &inc) {
		t.Fatalf("quiero *ErrCalendarioIncompleto, obtuve %v", err)
	}
	if len(inc.Fechas) != 1 || !inc.Fechas[0].Equal(fecha(2026, 10, 6)) {
		t.Fatalf("fechas faltantes = %v, quiero [2026-10-06]", inc.Fechas)
	}
	if tx.creadas != 0 {
		t.Fatal("no debía crear solicitud")
	}
}

func TestSolicitarVacaciones_Saldo(t *testing.T) {
	calendario := []dto.ListarCalendarioPorRangoRow{
		{CalendarioLaboralDia: dto.CalendarioLaboralDia{Fecha: fecha(2026, 10, 5), EsHabil: true}},
		{CalendarioLaboralDia: dto.CalendarioLaboralDia{Fecha: fecha(2026, 10, 6), EsHabil: true}},
		{CalendarioLaboralDia: dto.CalendarioLaboralDia{Fecha: fecha(2026, 10, 7), EsHabil: true}},
	}
	casos := []struct {
		nombre       string
		saldo        string
		comprometido string
		traslapes    int64
		wantErr      error
	}{
		{"saldo justo alcanza", "3", "0", 0, nil},
		{"saldo insuficiente", "2", "0", 0, ErrVacSaldoInsuficiente},
		{"saldo se reduce por aprobadas sin disfrutar", "4", "2", 0, ErrVacSaldoInsuficiente},
		{"traslape", "10", "0", 1, ErrVacSolicitudTraslapada},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			q := &fakeVacQueries{emp: dto.Empleado{Activo: true}, cal: calendario}
			tx := &fakeVacTx{
				saldo:        dto.VacacionesAcumulado{Saldo: d(c.saldo)},
				comprometido: d(c.comprometido),
				traslapes:    c.traslapes,
			}
			_, err := nuevoSvc(q, tx).SolicitarVacaciones(context.Background(), SolicitarVacacionesInput{
				EmpleadoID: 1, FechaInicio: fecha(2026, 10, 5), FechaFin: fecha(2026, 10, 7), CreadoPor: 9,
				PuedeSolicitarTerceros: true,
			})
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, quiero %v", err, c.wantErr)
			}
		})
	}
}

func TestResolverSolicitud_Estados(t *testing.T) {
	casos := []struct {
		nombre  string
		estado  string
		aprobar bool
		motivo  string
		wantErr error
	}{
		{"aprobar pendiente", "pendiente", true, "", nil},
		{"aprobar dos veces", "aprobada", true, "", ErrVacSolicitudNoPendiente},
		{"rechazar sin motivo", "pendiente", false, "", ErrVacMotivoRechazoRequerido},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			tx := &fakeVacTx{
				saldo:     dto.VacacionesAcumulado{Saldo: d("10")},
				solicitud: dto.VacacionesSolicitud{ID: 1, EmpleadoID: 1, Estado: c.estado, DiasSolicitados: d("3")},
			}
			_, err := nuevoSvc(&fakeVacQueries{}, tx).ResolverSolicitud(context.Background(), ResolverSolicitudInput{
				SolicitudID: 1, Aprobar: c.aprobar, MotivoRechazo: c.motivo, AprobadoPor: 9,
			})
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, quiero %v", err, c.wantErr)
			}
		})
	}
}

func TestVacGuardarSaldo_NoPermiteNegativo(t *testing.T) {
	tx := &fakeVacTx{}
	previo := dto.VacacionesAcumulado{Saldo: d("2")}
	err := vacGuardarSaldo(context.Background(), tx, 1, previo, fecha(2026, 1, 1), decimal.Zero, d("3"), 9)
	if !errors.Is(err, ErrVacSaldoInsuficiente) {
		t.Fatalf("err = %v, quiero ErrVacSaldoInsuficiente", err)
	}
	if tx.upserts != 0 {
		t.Fatal("no debía escribir con saldo negativo")
	}
}
