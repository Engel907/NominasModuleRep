package rrhh

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	dbdto "planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
)

// DashboardMetricsDTO es el payload que consume el handler de
// GET /api/v1/dashboard/metricas. GeneradoEn permite al frontend saber
// qué tan fresca es la información si en el futuro se agrega caché.
type DashboardMetricsDTO struct {
	TotalEmpleadosActivos           int64           `json:"total_empleados_activos"`
	UltimoPeriodoCerradoID          *int64          `json:"ultimo_periodo_cerrado_id"` // nil si nunca se ha cerrado un período
	CostoUltimaPlanillaCerrada      decimal.Decimal `json:"costo_ultima_planilla_cerrada"`
	SolicitudesVacacionesPendientes int64           `json:"solicitudes_vacaciones_pendientes"`
	IncapacidadesVigentes           int64           `json:"incapacidades_vigentes"`
	GeneradoEn                      time.Time       `json:"generado_en"`
}

// SystemHealthDTO es el payload de GET /health (liveness y readiness).
type SystemHealthDTO struct {
	Status        string        `json:"status"` // "ok" | "degradado"
	BaseDeDatosOK bool          `json:"base_de_datos_ok"`
	Latencia      time.Duration `json:"latencia_db"`
	VerificadoEn  time.Time     `json:"verificado_en"`
}

// MetricasDashboardRow es la fila cruda de las métricas del dashboard,
// modelada aquí para que la interfaz DashboardRepository (y los mocks de los
// tests) no dependan del paquete dto. El adaptador de más abajo
// (dashboardRepo) implementa DashboardRepository llamando a
// (*dbdto.Queries).ObtenerMetricasDashboard y convirtiendo su Row a este tipo.
type MetricasDashboardRow struct {
	TotalEmpleadosActivos           int64
	UltimoPeriodoCerradoID          sql.NullInt64
	CostoUltimaPlanillaCerrada      decimal.Decimal
	SolicitudesVacacionesPendientes int64
	IncapacidadesVigentes           int64
}

// DashboardRepository es el contrato de acceso a datos de este servicio.
type DashboardRepository interface {
	ObtenerMetricasDashboard(ctx context.Context, fechaReferencia time.Time) (MetricasDashboardRow, error)
}

// dashboardRepo adapta *dbdto.Queries a DashboardRepository. Es una
// conversión campo a campo, sin lógica.
type dashboardRepo struct {
	q *dbdto.Queries
}

// NewDashboardRepository construye el DashboardRepository sobre dto.Queries.
func NewDashboardRepository(q *dbdto.Queries) DashboardRepository {
	return &dashboardRepo{q: q}
}

func (r *dashboardRepo) ObtenerMetricasDashboard(ctx context.Context, fechaReferencia time.Time) (MetricasDashboardRow, error) {
	cerrados := planilla.EstadosCerrados()
	estados := make([]string, len(cerrados))
	for i, e := range cerrados {
		estados[i] = string(e)
	}
	fila, err := r.q.ObtenerMetricasDashboard(ctx, fechaReferencia, estados)
	if err != nil {
		return MetricasDashboardRow{}, err
	}
	return MetricasDashboardRow{
		TotalEmpleadosActivos:           fila.TotalEmpleadosActivos,
		UltimoPeriodoCerradoID:          fila.UltimoPeriodoCerradoID,
		CostoUltimaPlanillaCerrada:      fila.CostoUltimaPlanillaCerrada,
		SolicitudesVacacionesPendientes: fila.SolicitudesVacacionesPendientes,
		IncapacidadesVigentes:           fila.IncapacidadesVigentes,
	}, nil
}

// DashboardService entrega las métricas gerenciales y el estado de salud. El
// ping de salud va directo contra *sql.DB, independiente del repositorio.
type DashboardService struct {
	repo        DashboardRepository
	db          *sql.DB
	pingTimeout time.Duration
}

// NewDashboardService construye el servicio. pingTimeout acota cuánto
// puede tardar /health antes de reportar "degradado" — evita que un probe
// de Kubernetes se cuelgue si la BD está lenta. Si es <= 0 se usan 3 s.
func NewDashboardService(repo DashboardRepository, db *sql.DB, pingTimeout time.Duration) *DashboardService {
	if pingTimeout <= 0 {
		pingTimeout = 3 * time.Second
	}
	return &DashboardService{repo: repo, db: db, pingTimeout: pingTimeout}
}

// ObtenerMetricasGenerales consolida las métricas del dashboard gerencial
// en un único DTO. fechaReferencia se recibe explícita (no time.Now
// dentro del repositorio): el llamador decide la fecha.
func (s *DashboardService) ObtenerMetricasGenerales(ctx context.Context, fechaReferencia time.Time) (*DashboardMetricsDTO, error) {
	fila, err := s.repo.ObtenerMetricasDashboard(ctx, fechaReferencia)
	if err != nil {
		return nil, fmt.Errorf("dashboard: error obteniendo métricas: %w", err)
	}

	dto := &DashboardMetricsDTO{
		TotalEmpleadosActivos:           fila.TotalEmpleadosActivos,
		CostoUltimaPlanillaCerrada:      fila.CostoUltimaPlanillaCerrada,
		SolicitudesVacacionesPendientes: fila.SolicitudesVacacionesPendientes,
		IncapacidadesVigentes:           fila.IncapacidadesVigentes,
		GeneradoEn:                      time.Now().UTC(),
	}

	if fila.UltimoPeriodoCerradoID.Valid {
		id := fila.UltimoPeriodoCerradoID.Int64
		dto.UltimoPeriodoCerradoID = &id
	}

	return dto, nil
}

// VerificarEstadoSistema hace un PingContext acotado por pingTimeout y
// retorna el estado operativo de la API. Nunca retorna error: un fallo de
// BD es un resultado válido de healthcheck ("degradado"), no una
// excepción del servicio — el handler HTTP decide el código HTTP (200 vs
// 503) a partir de dto.Status / dto.BaseDeDatosOK.
func (s *DashboardService) VerificarEstadoSistema(ctx context.Context) *SystemHealthDTO {
	ctx, cancel := context.WithTimeout(ctx, s.pingTimeout)
	defer cancel()

	inicio := time.Now()
	err := s.db.PingContext(ctx)
	latencia := time.Since(inicio)

	dto := &SystemHealthDTO{
		BaseDeDatosOK: err == nil,
		Latencia:      latencia,
		VerificadoEn:  time.Now().UTC(),
	}

	if err != nil {
		dto.Status = "degradado"
	} else {
		dto.Status = "ok"
	}

	return dto
}
