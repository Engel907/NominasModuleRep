package rrhh

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"

	auditdb "planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

// HistoricoSalariosService carga los salarios de los meses anteriores a la
// puesta en marcha del sistema. Esos meses completan las bases del aguinaldo,
// del salario escolar y de los promedios de liquidación de quien ya trabajaba
// antes del primer período calculado aquí.
type HistoricoSalariosService struct{ db *sql.DB }

func NewHistoricoSalariosService(db *sql.DB) *HistoricoSalariosService {
	return &HistoricoSalariosService{db: db}
}

// MesHistoricoInput: un mes del histórico.
type MesHistoricoInput struct {
	Anio           int32           `json:"anio" binding:"required,gte=1990,lte=2100"`
	Mes            int32           `json:"mes" binding:"required,gte=1,lte=12"`
	MontoBruto     decimal.Decimal `json:"monto_bruto"`
	BaseAguinaldo  decimal.Decimal `json:"base_aguinaldo"`
	BaseCesantia   decimal.Decimal `json:"base_cesantia"`
	BaseVacaciones decimal.Decimal `json:"base_vacaciones"`
}

// CargarHistoricoInput: cuerpo de POST /empleados/:id/historico-salarios.
type CargarHistoricoInput struct {
	Fuente string              `json:"fuente" binding:"required,max=100"`
	Meses  []MesHistoricoInput `json:"meses" binding:"required,min=1,max=120,dive"`
}

// MesHistorico: vista de un mes cargado.
type MesHistorico struct {
	ID             int64           `json:"id"`
	Anio           int32           `json:"anio"`
	Mes            int32           `json:"mes"`
	MontoBruto     decimal.Decimal `json:"monto_bruto"`
	BaseAguinaldo  decimal.Decimal `json:"base_aguinaldo"`
	BaseCesantia   decimal.Decimal `json:"base_cesantia"`
	BaseVacaciones decimal.Decimal `json:"base_vacaciones"`
	Fuente         string          `json:"fuente"`
	FechaCarga     time.Time       `json:"fecha_carga"`
}

var ErrHistoricoNoEncontrado = errors.New("el mes del histórico no existe")

// Cargar inserta los meses (todo o nada). Un mes ya cargado o que el sistema
// ya calculó (período regular aprobado) se rechaza.
func (s *HistoricoSalariosService) Cargar(ctx context.Context, usuarioID, empleadoID int64, in CargarHistoricoInput) (int, error) {
	fuente := strings.TrimSpace(in.Fuente)
	if fuente == "" {
		return 0, validacion("fuente", "es obligatoria")
	}
	if err := validarMesesHistorico(in.Meses); err != nil {
		return 0, err
	}
	err := auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := bloquearEmpleado(ctx, q, empleadoID); err != nil {
			return err
		}
		for i, m := range in.Meses {
			campo := fmt.Sprintf("meses[%d]", i)
			n, err := q.ContarPeriodosRegularesEnMes(ctx, empleadoID, m.Anio, m.Mes)
			if err != nil {
				return err
			}
			if n > 0 {
				return validacion(campo, fmt.Sprintf("%04d-%02d ya tiene planilla calculada en el sistema", m.Anio, m.Mes))
			}
			_, err = q.InsertarHistoricoSalario(ctx, dto.InsertarHistoricoSalarioParams{
				EmpleadoID: empleadoID, Anio: m.Anio, Mes: m.Mes, MontoBruto: m.MontoBruto, BaseAguinaldo: m.BaseAguinaldo,
				BaseCesantia: m.BaseCesantia, BaseVacaciones: m.BaseVacaciones, Fuente: fuente, CargadoPor: usuarioID,
			})
			var me *mysql.MySQLError
			if errors.As(err, &me) && me.Number == 1062 {
				return validacion(campo, fmt.Sprintf("%04d-%02d ya está cargado", m.Anio, m.Mes))
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(in.Meses), nil
}

func validarMesesHistorico(meses []MesHistoricoInput) error {
	vistos := map[[2]int32]bool{}
	for i, m := range meses {
		campo := fmt.Sprintf("meses[%d]", i)
		clave := [2]int32{m.Anio, m.Mes}
		if vistos[clave] {
			return validacion(campo, "mes repetido en la carga")
		}
		vistos[clave] = true
		if !m.MontoBruto.IsPositive() || !m.MontoBruto.Equal(m.MontoBruto.Round(2)) {
			return validacion(campo+".monto_bruto", "debe ser mayor que cero, con a lo sumo 2 decimales")
		}
		bases := []struct {
			nombre string
			v      decimal.Decimal
		}{{"base_aguinaldo", m.BaseAguinaldo}, {"base_cesantia", m.BaseCesantia}, {"base_vacaciones", m.BaseVacaciones}}
		for _, b := range bases {
			if b.v.IsNegative() || !b.v.Equal(b.v.Round(2)) || b.v.GreaterThan(m.MontoBruto) {
				return validacion(campo+"."+b.nombre, "debe estar entre 0 y monto_bruto, con a lo sumo 2 decimales")
			}
		}
	}
	return nil
}

func (s *HistoricoSalariosService) Listar(ctx context.Context, empleadoID int64) ([]MesHistorico, error) {
	q := dto.New(s.db)
	if _, err := q.GetEmpleadoByID(ctx, empleadoID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEmpleadoNoEncontrado
	} else if err != nil {
		return nil, err
	}
	filas, err := q.ListarHistoricoSalarios(ctx, empleadoID)
	if err != nil {
		return nil, err
	}
	out := make([]MesHistorico, 0, len(filas))
	for _, f := range filas {
		out = append(out, MesHistorico{ID: f.ID, Anio: f.Anio, Mes: f.Mes, MontoBruto: f.MontoBruto, BaseAguinaldo: f.BaseAguinaldo,
			BaseCesantia: f.BaseCesantia, BaseVacaciones: f.BaseVacaciones, Fuente: f.Fuente, FechaCarga: f.FechaCarga})
	}
	return out, nil
}

// Eliminar borra un mes cargado por error (el borrado queda en la auditoría).
func (s *HistoricoSalariosService) Eliminar(ctx context.Context, usuarioID, empleadoID, id int64) error {
	return auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		n, err := q.EliminarHistoricoSalario(ctx, empleadoID, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrHistoricoNoEncontrado
		}
		return nil
	})
}
