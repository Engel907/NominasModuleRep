// services/contrato_service.go
//
// Fase 3 — Contratos.
//
// `contratos` no está en la lista de 19 tablas versionadas del ADR-000, pero
// se trata con el mismo patrón de vigencia 2-pasos por consistencia.
//
// F03 (esta revisión):
//   - Firma real de db.WithAuditContext (callback con ctx, usuarioID int64);
//     IDs int64 en toda la capa; paginación en internal/pagination.
//   - ValidarSalarioMinimo (exportada) lee SIEMPRE el parámetro vigente de
//     parametros_salario_minimo y devuelve errores tipados (D8):
//     *SalarioInferiorError (incumplimiento) y *SalarioMinimoNoParametrizadoError
//     (no hay parámetro: el alta/cambio se rechaza, no se asume un default).
//     Se eliminó la "advertencia" por prefijo de string (frágil) y el parche
//     parche de import no usado de helpers de fecha.
//   - CrearContrato/ActualizarContrato bloquean la fila del empleado
//     (FOR UPDATE) antes de leer el contrato vigente, para eliminar la
//     condición de carrera leer-vigente → cerrar → insertar.
//   - Validaciones: salario > 0 (escala DECIMAL(14,2)), fin de contrato >=
//     inicio, fecha efectiva posterior al inicio de la vigencia que se cierra,
//     puesto existente y activo.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/pagination"

	"github.com/shopspring/decimal"
)

// ErrSalarioInferiorAlMinimo se emite (envuelto en *SalarioInferiorError)
// cuando el salario pactado/actualizado en un contrato es menor al salario
// mínimo vigente para la clasificación del puesto. Es un error de negocio.
var ErrSalarioInferiorAlMinimo = errors.New("el salario pactado es inferior al salario mínimo vigente para la clasificación del puesto")

// ErrContratoNoEncontrado se devuelve cuando no existe contrato vigente
// para el empleado consultado.
var ErrContratoNoEncontrado = errors.New("el empleado no tiene un contrato vigente")

// ----------------------------- DTOs -----------------------------------------

type CrearContratoRequest struct {
	EmpleadoID          int64           `json:"empleado_id" binding:"required,gt=0"`
	PuestoID            int64           `json:"puesto_id" binding:"required,gt=0"`
	RegimenLaboralID    int64           `json:"regimen_laboral_id" binding:"required,gt=0"`
	TipoContrato        string          `json:"tipo_contrato" binding:"required,max=30"` // ej. indefinido, plazo_fijo, interino
	SalarioPactado      decimal.Decimal `json:"salario_pactado"`                         // validado en el servicio (> 0)
	Jornada             string          `json:"jornada" binding:"required,max=30"`
	FechaInicioContrato string          `json:"fecha_inicio_contrato" binding:"required"` // YYYY-MM-DD
	FechaFinContrato    *string         `json:"fecha_fin_contrato"`                       // opcional (plazo fijo)
	FechaEfectiva       string          `json:"fecha_efectiva" binding:"required"`        // fecha_inicio_vigencia
}

type ActualizarContratoRequest struct {
	PuestoID            int64           `json:"puesto_id" binding:"required,gt=0"`
	RegimenLaboralID    int64           `json:"regimen_laboral_id" binding:"required,gt=0"`
	TipoContrato        string          `json:"tipo_contrato" binding:"required,max=30"`
	SalarioPactado      decimal.Decimal `json:"salario_pactado"`
	Jornada             string          `json:"jornada" binding:"required,max=30"`
	FechaInicioContrato string          `json:"fecha_inicio_contrato" binding:"required"`
	FechaFinContrato    *string         `json:"fecha_fin_contrato"`
	FechaEfectiva       string          `json:"fecha_efectiva" binding:"required"`
}

type ContratoResponse struct {
	ID                  int64           `json:"id"`
	EmpleadoID          int64           `json:"empleado_id"`
	PuestoID            int64           `json:"puesto_id"`
	RegimenLaboralID    int64           `json:"regimen_laboral_id"`
	TipoContrato        string          `json:"tipo_contrato"`
	SalarioPactado      decimal.Decimal `json:"salario_pactado"`
	Jornada             string          `json:"jornada"`
	FechaInicioContrato string          `json:"fecha_inicio_contrato"`
	FechaFinContrato    *string         `json:"fecha_fin_contrato,omitempty"`
	FechaInicioVigencia string          `json:"fecha_inicio_vigencia"`
	EsVigente           bool            `json:"es_vigente"`
}

type ListarContratosRequest struct {
	// Paginacion NO se bindea con gin: el handler la llena con pagination.Desde(c).
	Paginacion   pagination.Parametros `form:"-"`
	EsVigente    *bool                 `form:"es_vigente"`
	PuestoID     *int64                `form:"puesto_id"`
	TipoContrato *string               `form:"tipo_contrato"`
}

type ListarContratosResponse struct {
	Contratos  []ContratoResponse `json:"contratos"`
	Paginacion pagination.Meta    `json:"paginacion"`
}

// ----------------------------- Servicio -------------------------------------

type ContratoService interface {
	CrearContrato(ctx context.Context, usuarioID int64, req CrearContratoRequest) (*ContratoResponse, error)
	ActualizarContrato(ctx context.Context, usuarioID, empleadoID int64, req ActualizarContratoRequest) (*ContratoResponse, error)
	GetContratoVigente(ctx context.Context, empleadoID int64) (*ContratoResponse, error)
	ListarHistoricoPorEmpleado(ctx context.Context, empleadoID int64, paginacion pagination.Parametros) ([]ContratoResponse, pagination.Meta, error)
	ListarContratos(ctx context.Context, req ListarContratosRequest) (*ListarContratosResponse, error)
}

type contratoService struct {
	sqlDB *sql.DB
}

func NewContratoService(sqlDB *sql.DB) ContratoService {
	return &contratoService{sqlDB: sqlDB}
}

// ValidarSalarioMinimo valida `salario` contra el salario mínimo VIGENTE
// (parametros_salario_minimo.es_vigente = 1) de la clasificación del puesto.
// NUNCA hardcodea el monto (ADR-000 sección 4/5). Debe llamarse con el
// *dto.Queries de la Tx en curso.
//
// Errores (todos tipados, D8):
//   - ErrPuestoNoEncontrado / ErrPuestoInactivo
//   - *SalarioMinimoNoParametrizadoError (errors.Is ErrSalarioMinimoNoParametrizado):
//     el puesto no tiene clasificación o no hay parámetro vigente. No se
//     devuelve ningún valor por defecto ni se deja pasar el salario.
//   - *SalarioInferiorError (errors.Is ErrSalarioInferiorAlMinimo)
//
// El salario mínimo se compara contra el salario mensual completo; no se
// prorratea por jornada (ver informe F03, punto dudoso).
func ValidarSalarioMinimo(ctx context.Context, q *dto.Queries, puestoID int64, salario decimal.Decimal) error {
	puesto, err := q.GetPuestoByID(ctx, puestoID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPuestoNoEncontrado
	}
	if err != nil {
		return fmt.Errorf("consultar puesto %d: %w", puestoID, err)
	}
	if !puesto.Activo {
		return ErrPuestoInactivo
	}
	if !puesto.ClasificacionPuestoID.Valid {
		return &SalarioMinimoNoParametrizadoError{PuestoID: puestoID}
	}

	clasificacionID := puesto.ClasificacionPuestoID.Int64
	minimo, err := q.GetSalarioMinimoVigentePorClasificacion(ctx, clasificacionID)
	if errors.Is(err, sql.ErrNoRows) {
		return &SalarioMinimoNoParametrizadoError{PuestoID: puestoID, ClasificacionPuestoID: &clasificacionID}
	}
	if err != nil {
		return fmt.Errorf("consultar salario mínimo vigente: %w", err)
	}

	if salario.LessThan(minimo.MontoMensual) {
		return &SalarioInferiorError{PuestoID: puestoID, Pactado: salario, Minimo: minimo.MontoMensual}
	}
	return nil
}

// parseFechasContrato parsea y valida las fechas comunes de crear/actualizar.
func parseFechasContrato(inicioContrato string, finContrato *string, efectiva string) (
	fechaInicio, fechaEfectiva time.Time, fechaFin sql.NullTime, err error,
) {
	fechaInicio, err = parseFecha("fecha_inicio_contrato", inicioContrato)
	if err != nil {
		return
	}
	fechaEfectiva, err = parseFecha("fecha_efectiva", efectiva)
	if err != nil {
		return
	}
	if finContrato != nil && strings.TrimSpace(*finContrato) != "" {
		var t time.Time
		t, err = parseFecha("fecha_fin_contrato", strings.TrimSpace(*finContrato))
		if err != nil {
			return
		}
		if err = validarRangoFechas("fecha_fin_contrato", fechaInicio, t); err != nil {
			return
		}
		fechaFin = sql.NullTime{Time: t, Valid: true}
	}
	return
}

func (s *contratoService) CrearContrato(ctx context.Context, usuarioID int64, req CrearContratoRequest) (*ContratoResponse, error) {
	tipoContrato, err := noVacio("tipo_contrato", req.TipoContrato)
	if err != nil {
		return nil, err
	}
	jornada, err := noVacio("jornada", req.Jornada)
	if err != nil {
		return nil, err
	}
	fechaInicioContrato, fechaEfectiva, fechaFinContrato, err := parseFechasContrato(req.FechaInicioContrato, req.FechaFinContrato, req.FechaEfectiva)
	if err != nil {
		return nil, err
	}
	// TODO F22: salario_pactado vs salario_base_nominal — fuente única (D5).
	if err := validarMontoSalario("salario_pactado", req.SalarioPactado); err != nil {
		return nil, err
	}

	var respuesta ContratoResponse

	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		// Serializa contra otras escrituras del mismo empleado y valida que exista.
		if err := bloquearEmpleado(ctx, q, req.EmpleadoID); err != nil {
			return err
		}

		if err := ValidarSalarioMinimo(ctx, q, req.PuestoID, req.SalarioPactado); err != nil {
			return err
		}

		// Un "crear" no debe pisar silenciosamente un contrato vigente: eso es
		// ActualizarContrato (2 pasos explícitos). Con el lock del empleado esta
		// lectura ya no compite con otra Tx.
		if _, err := q.GetContratoVigente(ctx, req.EmpleadoID); err == nil {
			return ErrContratoVigenteExiste
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("verificar contrato vigente existente: %w", err)
		}

		contratoID, err := q.CrearVersionContrato(ctx, req.EmpleadoID, req.PuestoID, req.RegimenLaboralID,
			tipoContrato, req.SalarioPactado, jornada, fechaInicioContrato, fechaFinContrato,
			fechaEfectiva, usuarioID)
		if err != nil {
			return traducirErrorBD(err, "crear contrato")
		}

		creado, err := q.GetContratoVigente(ctx, req.EmpleadoID)
		if err != nil {
			return fmt.Errorf("releer contrato creado %d: %w", contratoID, err)
		}
		respuesta = contratoRespuestaDesdeRow(creado)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &respuesta, nil
}

func (s *contratoService) ActualizarContrato(ctx context.Context, usuarioID, empleadoID int64, req ActualizarContratoRequest) (*ContratoResponse, error) {
	tipoContrato, err := noVacio("tipo_contrato", req.TipoContrato)
	if err != nil {
		return nil, err
	}
	jornada, err := noVacio("jornada", req.Jornada)
	if err != nil {
		return nil, err
	}
	fechaInicioContrato, fechaEfectiva, fechaFinContrato, err := parseFechasContrato(req.FechaInicioContrato, req.FechaFinContrato, req.FechaEfectiva)
	if err != nil {
		return nil, err
	}
	// TODO F22: salario_pactado vs salario_base_nominal — fuente única (D5).
	if err := validarMontoSalario("salario_pactado", req.SalarioPactado); err != nil {
		return nil, err
	}

	var respuesta ContratoResponse

	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := bloquearEmpleado(ctx, q, empleadoID); err != nil {
			return err
		}

		if err := ValidarSalarioMinimo(ctx, q, req.PuestoID, req.SalarioPactado); err != nil {
			return err
		}

		vigente, err := q.GetContratoVigente(ctx, empleadoID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrContratoNoEncontrado
		}
		if err != nil {
			return fmt.Errorf("obtener contrato vigente: %w", err)
		}
		if err := validarNuevaVigencia("fecha_efectiva", fechaEfectiva, vigente.FechaInicioVigencia); err != nil {
			return err
		}

		// Paso 1: cerrar el contrato vigente actual.
		if err := q.CerrarContratoVigente(ctx, empleadoID, fechaEfectiva); err != nil {
			return fmt.Errorf("cerrar contrato vigente: %w", err)
		}

		// Paso 2: abrir la nueva versión vigente.
		contratoID, err := q.CrearVersionContrato(ctx, empleadoID, req.PuestoID, req.RegimenLaboralID,
			tipoContrato, req.SalarioPactado, jornada, fechaInicioContrato, fechaFinContrato,
			fechaEfectiva, usuarioID)
		if err != nil {
			return traducirErrorBD(err, "crear nueva version de contrato")
		}

		creado, err := q.GetContratoVigente(ctx, empleadoID)
		if err != nil {
			return fmt.Errorf("releer contrato creado %d: %w", contratoID, err)
		}
		respuesta = contratoRespuestaDesdeRow(creado)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &respuesta, nil
}

func (s *contratoService) GetContratoVigente(ctx context.Context, empleadoID int64) (*ContratoResponse, error) {
	q := dto.New(s.sqlDB)
	c, err := q.GetContratoVigente(ctx, empleadoID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrContratoNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("obtener contrato vigente: %w", err)
	}
	r := contratoRespuestaDesdeRow(c)
	return &r, nil
}

func (s *contratoService) ListarHistoricoPorEmpleado(ctx context.Context, empleadoID int64, paginacion pagination.Parametros) ([]ContratoResponse, pagination.Meta, error) {
	pag := paginacion.Normalizado()
	q := dto.New(s.sqlDB)
	rows, err := q.ListContratosHistoricoPorEmpleado(ctx, empleadoID, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("listar histórico de contratos: %w", err)
	}
	total, err := q.CountContratosHistoricoPorEmpleado(ctx, empleadoID)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("contar histórico de contratos: %w", err)
	}
	out := make([]ContratoResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, contratoRespuestaDesdeRow(r))
	}
	return out, pagination.NuevaMeta(pag, total), nil
}

func (s *contratoService) ListarContratos(ctx context.Context, req ListarContratosRequest) (*ListarContratosResponse, error) {
	pag := req.Paginacion.Normalizado()
	q := dto.New(s.sqlDB)
	rows, err := q.ListContratos(ctx, req.EsVigente, req.PuestoID, req.TipoContrato, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, fmt.Errorf("listar contratos: %w", err)
	}
	total, err := q.CountContratos(ctx, req.EsVigente, req.PuestoID, req.TipoContrato)
	if err != nil {
		return nil, fmt.Errorf("contar contratos: %w", err)
	}
	out := make([]ContratoResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, contratoRespuestaDesdeRow(r))
	}
	return &ListarContratosResponse{Contratos: out, Paginacion: pagination.NuevaMeta(pag, total)}, nil
}

// ----------------------------- helpers de mapeo ------------------------------

func contratoRespuestaDesdeRow(c dto.Contrato) ContratoResponse {
	r := ContratoResponse{
		ID:                  c.ID,
		EmpleadoID:          c.EmpleadoID,
		PuestoID:            c.PuestoID,
		RegimenLaboralID:    c.RegimenLaboralID,
		TipoContrato:        c.TipoContrato,
		SalarioPactado:      c.SalarioPactado,
		Jornada:             c.Jornada,
		FechaInicioContrato: c.FechaInicioContrato.Format(layoutFecha),
		FechaInicioVigencia: c.FechaInicioVigencia.Format(layoutFecha),
		EsVigente:           c.EsVigente,
	}
	if c.FechaFinContrato.Valid {
		f := c.FechaFinContrato.Time.Format(layoutFecha)
		r.FechaFinContrato = &f
	}
	return r
}
