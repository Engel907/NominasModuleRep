package rrhh

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/paginacion"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/planilla"

	"github.com/shopspring/decimal"
)

// ErrSalarioInferiorAlMinimo se emite (envuelto en *SalarioInferiorError)
// cuando el salario pactado/actualizado en un contrato es menor al salario
// mínimo vigente para la clasificación del puesto. Es un error de negocio.
var ErrSalarioInferiorAlMinimo = errors.New("el salario pactado es inferior al salario mínimo vigente para la clasificación del puesto")

// ErrContratoNoEncontrado se devuelve cuando no existe contrato vigente
// para el empleado consultado.
var ErrContratoNoEncontrado = errors.New("el empleado no tiene un contrato vigente")

type CrearContratoRequest struct {
	EmpleadoID       int64 `json:"empleado_id" binding:"required,gt=0"`
	PuestoID         int64 `json:"puesto_id" binding:"required,gt=0"`
	RegimenLaboralID int64 `json:"regimen_laboral_id" binding:"required,gt=0"`
	TipoContratoID   int64 `json:"tipo_contrato_id" binding:"required,gt=0"` // catálogo tipos_contrato
	// SalarioPactado va en la unidad de ModalidadSalario (mes, día u hora
	// ordinaria); validado en el servicio (> 0).
	SalarioPactado      decimal.Decimal `json:"salario_pactado"`
	ModalidadSalario    string          `json:"modalidad_salario" binding:"required"`     // mensual | diario | por_hora
	JornadaID           int64           `json:"jornada_id" binding:"required,gt=0"`       // catálogo jornadas
	FechaInicioContrato string          `json:"fecha_inicio_contrato" binding:"required"` // YYYY-MM-DD
	FechaFinContrato    *string         `json:"fecha_fin_contrato"`                       // opcional (plazo fijo)
	FechaEfectiva       string          `json:"fecha_efectiva" binding:"required"`        // fecha_inicio_vigencia
}

type ActualizarContratoRequest struct {
	PuestoID            int64           `json:"puesto_id" binding:"required,gt=0"`
	RegimenLaboralID    int64           `json:"regimen_laboral_id" binding:"required,gt=0"`
	TipoContratoID      int64           `json:"tipo_contrato_id" binding:"required,gt=0"`
	SalarioPactado      decimal.Decimal `json:"salario_pactado"`
	ModalidadSalario    string          `json:"modalidad_salario" binding:"required"`
	JornadaID           int64           `json:"jornada_id" binding:"required,gt=0"`
	FechaInicioContrato string          `json:"fecha_inicio_contrato" binding:"required"`
	FechaFinContrato    *string         `json:"fecha_fin_contrato"`
	FechaEfectiva       string          `json:"fecha_efectiva" binding:"required"`
}

// ContratoResponse: tipo_contrato y jornada son los códigos de sus catálogos
// (solo lectura); se escriben por id.
type ContratoResponse struct {
	ID                  int64           `json:"id"`
	EmpleadoID          int64           `json:"empleado_id"`
	PuestoID            int64           `json:"puesto_id"`
	RegimenLaboralID    int64           `json:"regimen_laboral_id"`
	TipoContratoID      int64           `json:"tipo_contrato_id"`
	TipoContrato        string          `json:"tipo_contrato"`
	SalarioPactado      decimal.Decimal `json:"salario_pactado"`
	ModalidadSalario    string          `json:"modalidad_salario"`
	JornadaID           int64           `json:"jornada_id"`
	Jornada             string          `json:"jornada"`
	FechaInicioContrato string          `json:"fecha_inicio_contrato"`
	FechaFinContrato    *string         `json:"fecha_fin_contrato,omitempty"`
	FechaInicioVigencia string          `json:"fecha_inicio_vigencia"`
	EsVigente           bool            `json:"es_vigente"`
}

type ListarContratosRequest struct {
	// Paginacion no se bindea con gin: el handler la llena con paginacion.Desde(c).
	Paginacion     paginacion.Parametros `form:"-"`
	EsVigente      *bool                 `form:"es_vigente"`
	PuestoID       *int64                `form:"puesto_id"`
	TipoContratoID *int64                `form:"tipo_contrato_id"`
}

type ListarContratosResponse struct {
	Contratos  []ContratoResponse `json:"contratos"`
	Paginacion paginacion.Meta    `json:"paginacion"`
}

type ContratoService interface {
	CrearContrato(ctx context.Context, usuarioID int64, req CrearContratoRequest) (*ContratoResponse, error)
	ActualizarContrato(ctx context.Context, usuarioID, empleadoID int64, req ActualizarContratoRequest) (*ContratoResponse, error)
	GetContratoVigente(ctx context.Context, empleadoID int64) (*ContratoResponse, error)
	ListarHistoricoPorEmpleado(ctx context.Context, empleadoID int64, params paginacion.Parametros) ([]ContratoResponse, paginacion.Meta, error)
	ListarContratos(ctx context.Context, req ListarContratosRequest) (*ListarContratosResponse, error)
}

type contratoService struct {
	sqlDB *sql.DB
}

func NewContratoService(sqlDB *sql.DB) ContratoService {
	return &contratoService{sqlDB: sqlDB}
}

// ValidarSalarioMinimo valida `salario` (en la unidad de `modalidad`) contra
// el salario mínimo VIGENTE (parametros_salario_minimo.es_vigente = 1) de la
// clasificación del puesto. El mínimo es mensual: un salario diario se lleva
// al mes con DIVISOR_DIAS_MES y uno por hora con el divisor de horas
// mensuales de la jornada (parametros_jornada), los mismos con los que la
// planilla valora el día y la hora. Debe llamarse con el *dto.Queries de la
// Tx en curso.
func ValidarSalarioMinimo(ctx context.Context, q *dto.Queries, empleadoID, puestoID int64, salario decimal.Decimal,
	modalidad string, jornadaID int64, fecha time.Time) error {
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

	mensual, err := equivalenteMensual(ctx, q, empleadoID, salario, modalidad, jornadaID, fecha)
	if err != nil {
		return err
	}
	if mensual.LessThan(minimo.MontoMensual) {
		return &SalarioInferiorError{PuestoID: puestoID, Pactado: salario, Modalidad: modalidad,
			EquivalenteMensual: mensual, Minimo: minimo.MontoMensual}
	}
	return nil
}

// equivalenteMensual: el salario pactado llevado al mes.
func equivalenteMensual(ctx context.Context, q *dto.Queries, empleadoID int64, salario decimal.Decimal,
	modalidad string, jornadaID int64, fecha time.Time) (decimal.Decimal, error) {
	switch modalidad {
	case planilla.ModalidadMensual:
		return salario, nil
	case planilla.ModalidadDiario:
		v, err := q.ValorParametroGeneralEmpleado(ctx, empleadoID, parametros.ClaveDivisorDiasMes, fecha)
		if errors.Is(err, sql.ErrNoRows) {
			return decimal.Zero, validacion("modalidad_salario", parametros.ClaveDivisorDiasMes+" no está vigente; no se puede llevar el salario diario al mes")
		}
		if err != nil {
			return decimal.Zero, fmt.Errorf("leer %s: %w", parametros.ClaveDivisorDiasMes, err)
		}
		div, err := decimal.NewFromString(v)
		if err != nil || !div.IsPositive() {
			return decimal.Zero, fmt.Errorf("%s inválido (%q)", parametros.ClaveDivisorDiasMes, v)
		}
		return salario.Mul(div), nil
	case planilla.ModalidadPorHora:
		pj, err := q.ObtenerParametroJornadaEnFecha(ctx, jornadaID, fecha)
		if errors.Is(err, sql.ErrNoRows) {
			return decimal.Zero, validacion("jornada_id", "la jornada no tiene parametros_jornada vigente; no se puede llevar el salario por hora al mes")
		}
		if err != nil {
			return decimal.Zero, fmt.Errorf("leer parametros_jornada: %w", err)
		}
		return salario.Mul(pj.DivisorHorasMensuales), nil
	}
	return decimal.Zero, validacion("modalidad_salario", "debe ser mensual, diario o por_hora")
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
	if !planilla.ModalidadSalarioValida(req.ModalidadSalario) {
		return nil, validacion("modalidad_salario", "debe ser mensual, diario o por_hora")
	}
	fechaInicioContrato, fechaEfectiva, fechaFinContrato, err := parseFechasContrato(req.FechaInicioContrato, req.FechaFinContrato, req.FechaEfectiva)
	if err != nil {
		return nil, err
	}
	if err := validarMontoSalario("salario_pactado", req.SalarioPactado); err != nil {
		return nil, err
	}

	var respuesta ContratoResponse

	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := bloquearEmpleado(ctx, q, req.EmpleadoID); err != nil {
			return err
		}

		if err := validarCatalogosContrato(ctx, q, req.TipoContratoID, req.JornadaID); err != nil {
			return err
		}
		if err := ValidarSalarioMinimo(ctx, q, req.EmpleadoID, req.PuestoID, req.SalarioPactado, req.ModalidadSalario, req.JornadaID, fechaEfectiva); err != nil {
			return err
		}

		if _, err := q.GetContratoVigente(ctx, req.EmpleadoID); err == nil {
			return ErrContratoVigenteExiste
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("verificar contrato vigente existente: %w", err)
		}

		contratoID, err := q.CrearVersionContrato(ctx, req.EmpleadoID, req.PuestoID, req.RegimenLaboralID,
			req.TipoContratoID, req.SalarioPactado, req.ModalidadSalario, req.JornadaID, fechaInicioContrato, fechaFinContrato,
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
	if !planilla.ModalidadSalarioValida(req.ModalidadSalario) {
		return nil, validacion("modalidad_salario", "debe ser mensual, diario o por_hora")
	}
	fechaInicioContrato, fechaEfectiva, fechaFinContrato, err := parseFechasContrato(req.FechaInicioContrato, req.FechaFinContrato, req.FechaEfectiva)
	if err != nil {
		return nil, err
	}
	if err := validarMontoSalario("salario_pactado", req.SalarioPactado); err != nil {
		return nil, err
	}

	var respuesta ContratoResponse

	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := bloquearEmpleado(ctx, q, empleadoID); err != nil {
			return err
		}

		if err := validarCatalogosContrato(ctx, q, req.TipoContratoID, req.JornadaID); err != nil {
			return err
		}
		if err := ValidarSalarioMinimo(ctx, q, empleadoID, req.PuestoID, req.SalarioPactado, req.ModalidadSalario, req.JornadaID, fechaEfectiva); err != nil {
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

		if err := q.CerrarContratoVigente(ctx, empleadoID, fechaEfectiva); err != nil {
			return fmt.Errorf("cerrar contrato vigente: %w", err)
		}

		contratoID, err := q.CrearVersionContrato(ctx, empleadoID, req.PuestoID, req.RegimenLaboralID,
			req.TipoContratoID, req.SalarioPactado, req.ModalidadSalario, req.JornadaID, fechaInicioContrato, fechaFinContrato,
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

func (s *contratoService) ListarHistoricoPorEmpleado(ctx context.Context, empleadoID int64, params paginacion.Parametros) ([]ContratoResponse, paginacion.Meta, error) {
	pag := params.Normalizado()
	q := dto.New(s.sqlDB)
	rows, err := q.ListContratosHistoricoPorEmpleado(ctx, empleadoID, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, paginacion.Meta{}, fmt.Errorf("listar histórico de contratos: %w", err)
	}
	total, err := q.CountContratosHistoricoPorEmpleado(ctx, empleadoID)
	if err != nil {
		return nil, paginacion.Meta{}, fmt.Errorf("contar histórico de contratos: %w", err)
	}
	out := make([]ContratoResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, contratoRespuestaDesdeRow(r))
	}
	return out, paginacion.NuevaMeta(pag, total), nil
}

func (s *contratoService) ListarContratos(ctx context.Context, req ListarContratosRequest) (*ListarContratosResponse, error) {
	pag := req.Paginacion.Normalizado()
	q := dto.New(s.sqlDB)
	rows, err := q.ListContratos(ctx, req.EsVigente, req.PuestoID, req.TipoContratoID, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, fmt.Errorf("listar contratos: %w", err)
	}
	total, err := q.CountContratos(ctx, req.EsVigente, req.PuestoID, req.TipoContratoID)
	if err != nil {
		return nil, fmt.Errorf("contar contratos: %w", err)
	}
	out := make([]ContratoResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, contratoRespuestaDesdeRow(r))
	}
	return &ListarContratosResponse{Contratos: out, Paginacion: paginacion.NuevaMeta(pag, total)}, nil
}

func contratoRespuestaDesdeRow(c dto.Contrato) ContratoResponse {
	r := ContratoResponse{
		ID:                  c.ID,
		EmpleadoID:          c.EmpleadoID,
		PuestoID:            c.PuestoID,
		RegimenLaboralID:    c.RegimenLaboralID,
		TipoContratoID:      c.TipoContratoID,
		TipoContrato:        c.TipoContrato,
		SalarioPactado:      c.SalarioPactado,
		ModalidadSalario:    c.ModalidadSalario,
		JornadaID:           c.JornadaID,
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

// validarCatalogosContrato: el tipo de contrato debe existir y la jornada
// estar activa (el divisor de la hora depende de ella).
func validarCatalogosContrato(ctx context.Context, q *dto.Queries, tipoContratoID, jornadaID int64) error {
	ok, err := q.ExisteTipoContrato(ctx, tipoContratoID)
	if err != nil {
		return fmt.Errorf("validar tipo de contrato: %w", err)
	}
	if !ok {
		return validacion("tipo_contrato_id", "no existe en el catálogo de tipos de contrato")
	}
	ok, err = q.ExisteJornadaActiva(ctx, jornadaID)
	if err != nil {
		return fmt.Errorf("validar jornada: %w", err)
	}
	if !ok {
		return validacion("jornada_id", "no existe o no está activa en el catálogo de jornadas")
	}
	return nil
}
