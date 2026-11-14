package rrhh

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	auditdb "planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

// AsignacionesService administra los conceptos recurrentes de cada empleado
// (asociación solidarista, sindicato, pólizas, ahorro, pluses): qué concepto,
// cuánto (monto mensual o porcentaje) y desde cuándo. El motor los aplica en
// cada planilla regular mientras estén vigentes.
type AsignacionesService struct{ db *sql.DB }

func NewAsignacionesService(db *sql.DB) *AsignacionesService { return &AsignacionesService{db: db} }

// AsignarConceptoInput: cuerpo de POST /empleados/:id/conceptos-asignados.
// Si el empleado ya tiene una asignación vigente del concepto, se cierra el día
// anterior a fecha_inicio_vigencia y rige la nueva.
type AsignarConceptoInput struct {
	ConceptoSalarialID  int64           `json:"concepto_salarial_id" binding:"required,gt=0"`
	Modo                string          `json:"modo" binding:"required,oneof=monto porcentaje"`
	Valor               decimal.Decimal `json:"valor"`
	TerceroID           *int64          `json:"tercero_id" binding:"omitempty,gt=0"`
	Referencia          *string         `json:"referencia" binding:"omitempty,max=60"`
	Observacion         *string         `json:"observacion" binding:"omitempty,max=255"`
	FechaInicioVigencia string          `json:"fecha_inicio_vigencia" binding:"required"`
}

// FinalizarAsignacionInput: cuerpo de POST /empleados/conceptos-asignados/:asignacion_id/finalizar.
type FinalizarAsignacionInput struct {
	FechaFinVigencia string `json:"fecha_fin_vigencia" binding:"required"`
}

// Asignacion: vista de una asignación para la API.
type Asignacion struct {
	ID                  int64           `json:"id"`
	EmpleadoID          int64           `json:"empleado_id"`
	ConceptoSalarialID  int64           `json:"concepto_salarial_id"`
	ConceptoCodigo      string          `json:"concepto_codigo"`
	ConceptoNombre      string          `json:"concepto_nombre"`
	Naturaleza          string          `json:"naturaleza"`
	Modo                string          `json:"modo"`
	Valor               decimal.Decimal `json:"valor"`
	TerceroID           *int64          `json:"tercero_id"`
	TerceroNombre       *string         `json:"tercero_nombre"`
	Referencia          *string         `json:"referencia"`
	Observacion         *string         `json:"observacion"`
	FechaInicioVigencia string          `json:"fecha_inicio_vigencia"`
	FechaFinVigencia    *string         `json:"fecha_fin_vigencia"`
	Vigente             bool            `json:"vigente"`
}

var (
	ErrAsignacionNoEncontrada = errors.New("la asignación no existe")
	ErrAsignacionCerrada      = errors.New("la asignación ya está finalizada")
)

const maxDecimalesAsignacion = 4

// Asignar registra la asignación y devuelve su id.
func (s *AsignacionesService) Asignar(ctx context.Context, usuarioID, empleadoID int64, in AsignarConceptoInput) (int64, error) {
	if !in.Valor.IsPositive() {
		return 0, validacion("valor", "debe ser mayor que cero")
	}
	if !in.Valor.Equal(in.Valor.Round(maxDecimalesAsignacion)) {
		return 0, validacion("valor", fmt.Sprintf("admite como máximo %d decimales", maxDecimalesAsignacion))
	}
	if in.Modo == "porcentaje" && in.Valor.GreaterThan(decimal.NewFromInt(100)) {
		return 0, validacion("valor", "un porcentaje no puede superar 100")
	}
	inicio, err := parseFecha("fecha_inicio_vigencia", in.FechaInicioVigencia)
	if err != nil {
		return 0, err
	}
	referencia, observacion := textoOpcional(in.Referencia), textoOpcional(in.Observacion)

	var id int64
	err = auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := bloquearEmpleado(ctx, q, empleadoID); err != nil {
			return err
		}
		ok, err := q.ConceptoAsignable(ctx, in.ConceptoSalarialID, inicio)
		if err != nil {
			return fmt.Errorf("verificando el concepto %d: %w", in.ConceptoSalarialID, err)
		}
		if !ok {
			return validacion("concepto_salarial_id", "el concepto no está activo o su fórmula vigente no es de tipo asignacion_empleado")
		}
		if in.TerceroID != nil {
			if _, err := q.GetTercero(ctx, *in.TerceroID); errors.Is(err, sql.ErrNoRows) {
				return validacion("tercero_id", "el tercero no existe")
			} else if err != nil {
				return fmt.Errorf("verificando el tercero %d: %w", *in.TerceroID, err)
			}
		}
		vigente, err := q.ObtenerAsignacionVigenteDeConcepto(ctx, empleadoID, in.ConceptoSalarialID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return fmt.Errorf("leyendo la asignación vigente: %w", err)
		default:
			if !inicio.After(vigente.FechaInicioVigencia) {
				return &VigenciaTraslapadaError{Campo: "fecha_inicio_vigencia", FechaEfectiva: inicio, InicioVigencia: vigente.FechaInicioVigencia}
			}
			if _, err := q.CerrarAsignacion(ctx, vigente.ID, inicio.AddDate(0, 0, -1)); err != nil {
				return fmt.Errorf("cerrando la asignación %d: %w", vigente.ID, err)
			}
		}
		id, err = q.InsertarAsignacion(ctx, dto.InsertarAsignacionParams{
			EmpleadoID: empleadoID, ConceptoSalarialID: in.ConceptoSalarialID, Modo: in.Modo, Valor: in.Valor,
			TerceroID: in.TerceroID, Referencia: referencia, Observacion: observacion,
			FechaInicioVigencia: inicio, CreadoPor: usuarioID,
		})
		return err
	})
	return id, err
}

// Finalizar cierra una asignación vigente en la fecha indicada (último día en
// que se aplica).
func (s *AsignacionesService) Finalizar(ctx context.Context, usuarioID, asignacionID int64, in FinalizarAsignacionInput) error {
	fin, err := parseFecha("fecha_fin_vigencia", in.FechaFinVigencia)
	if err != nil {
		return err
	}
	return auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		a, err := q.ObtenerAsignacionParaActualizar(ctx, asignacionID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAsignacionNoEncontrada
		}
		if err != nil {
			return fmt.Errorf("leyendo la asignación %d: %w", asignacionID, err)
		}
		if a.FechaFinVigencia != nil {
			return ErrAsignacionCerrada
		}
		if fin.Before(a.FechaInicioVigencia) {
			return validacion("fecha_fin_vigencia", "no puede ser anterior al inicio de la asignación")
		}
		_, err = q.CerrarAsignacion(ctx, asignacionID, fin)
		return err
	})
}

// Listar devuelve el historial de asignaciones del empleado.
func (s *AsignacionesService) Listar(ctx context.Context, empleadoID int64) ([]Asignacion, error) {
	q := dto.New(s.db)
	if _, err := q.GetEmpleadoByID(ctx, empleadoID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEmpleadoNoEncontrado
	} else if err != nil {
		return nil, err
	}
	filas, err := q.ListarAsignacionesEmpleado(ctx, empleadoID)
	if err != nil {
		return nil, err
	}
	hoy := time.Now().UTC()
	out := make([]Asignacion, 0, len(filas))
	for _, f := range filas {
		a := Asignacion{
			ID: f.ID, EmpleadoID: f.EmpleadoID, ConceptoSalarialID: f.ConceptoSalarialID,
			ConceptoCodigo: f.ConceptoCodigo, ConceptoNombre: f.ConceptoNombre, Naturaleza: f.Naturaleza,
			Modo: f.Modo, Valor: f.Valor, TerceroID: f.TerceroID, TerceroNombre: f.TerceroNombre,
			Referencia: f.Referencia, Observacion: f.Observacion,
			FechaInicioVigencia: f.FechaInicioVigencia.Format(layoutFecha),
			Vigente:             !f.FechaInicioVigencia.After(hoy) && (f.FechaFinVigencia == nil || !f.FechaFinVigencia.Before(hoy.Truncate(24*time.Hour))),
		}
		if f.FechaFinVigencia != nil {
			s := f.FechaFinVigencia.Format(layoutFecha)
			a.FechaFinVigencia = &s
		}
		out = append(out, a)
	}
	return out, nil
}

func textoOpcional(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
