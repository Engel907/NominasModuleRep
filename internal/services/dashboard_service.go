// Package services agrupa lógica de negocio que orquesta acceso a datos
// sin ejecutar SQL directamente.
//
// Fase 8 — Dashboard gerencial y monitoreo operativo.
//
// Principio de diseño (mismo que internal/planilla/service.go, Fase 6):
// el servicio depende de una INTERFAZ de repositorio, no de *dto.Queries ni
// de *sql.DB para las queries de negocio. Esto
// permite testear ObtenerMetricasGenerales con un mock (table-driven
// tests) sin una BD real. La excepción es VerificarEstadoSistema, que sí
// necesita el pool *sql.DB porque su trabajo ES verificar la conexión.
package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	// Alias dbdto: en este archivo varias funciones usan `dto` como nombre de
	// variable local (el DTO de salida), lo que sombrearía el paquete.
	dbdto "planillas-ccdr/internal/db/dto"
)

// ============================================================================
// DTOs de salida
// ============================================================================

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

// SystemHealthDTO es el payload de GET /health. Sirve tanto de liveness
// (¿el proceso responde?) como de readiness (¿tiene BD disponible?) en un
// solo DTO — hay una sola dependencia externa relevante (MySQL), así que
// separar dos structs sería complejidad sin beneficio.
type SystemHealthDTO struct {
	Status        string        `json:"status"` // "ok" | "degradado"
	BaseDeDatosOK bool          `json:"base_de_datos_ok"`
	Latencia      time.Duration `json:"latencia_db"`
	VerificadoEn  time.Time     `json:"verificado_en"`
}

// ============================================================================
// Repositorio
// ============================================================================

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

// NewDashboardRepository construye la implementación real de
// DashboardRepository. Wiring en main.go:
//
//	dashboardService := services.NewDashboardService(
//	    services.NewDashboardRepository(dbdto.New(sqlDB)), sqlDB, 3*time.Second)
func NewDashboardRepository(q *dbdto.Queries) DashboardRepository {
	return &dashboardRepo{q: q}
}

func (r *dashboardRepo) ObtenerMetricasDashboard(ctx context.Context, fechaReferencia time.Time) (MetricasDashboardRow, error) {
	fila, err := r.q.ObtenerMetricasDashboard(ctx, fechaReferencia)
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

// ============================================================================
// Servicio
// ============================================================================

// DashboardService agrupa las dos responsabilidades de Fase 8. El ping de
// salud se hace directo contra *sql.DB (no vía DashboardRepository) a
// propósito: es una verificación de infraestructura, no de dominio, y así
// /health sigue funcionando aunque hubiera un bug de mapeo en el
// repositorio de métricas.
type DashboardService struct {
	repo        DashboardRepository
	db          *sql.DB
	pingTimeout time.Duration
}

// NewDashboardService construye el servicio. pingTimeout acota cuánto
// puede tardar /health antes de reportar "degradado" — evita que un probe
// de Kubernetes se cuelgue si la BD está lenta pero no caída del todo.
// Default: 3s (igual al timeout que main.go ya usa en /healthz).
func NewDashboardService(repo DashboardRepository, db *sql.DB, pingTimeout time.Duration) *DashboardService {
	if pingTimeout <= 0 {
		pingTimeout = 3 * time.Second
	}
	return &DashboardService{repo: repo, db: db, pingTimeout: pingTimeout}
}

// ObtenerMetricasGenerales consolida las métricas del dashboard gerencial
// en un único DTO. fechaReferencia se recibe explícita (no time.Now()
// dentro del repositorio) por el mismo criterio de testabilidad que
// CalculoEmpleadoInput.FechaEfectiva en el motor de planilla — el llamador
// (el handler HTTP) decide la fecha, normalmente time.Now().
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

	// UltimoPeriodoCerradoID puede ser NULL (todavía no se ha cerrado
	// ningún período) — se traduce explícitamente a *int64 nil en vez de
	// dejar pasar un 0 que un consumidor podría confundir con un ID real.
	if fila.UltimoPeriodoCerradoID.Valid {
		id := fila.UltimoPeriodoCerradoID.Int64
		dto.UltimoPeriodoCerradoID = &id
	}

	return dto, nil
}

// VerificarEstadoSistema hace un PingContext acotado por pingTimeout y
// retorna el estado operativo de la API. NUNCA retorna error: un fallo de
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