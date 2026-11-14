// Package infra implementa sobre internal/db/dto los repositorios que define
// el paquete planilla.
package infra

import (
	"context"
	"database/sql"
	"fmt"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
)

// exportQueries es el subconjunto de *dto.Queries que usa el repositorio.
// Los estados se pasan como []string porque dto no puede importar planilla
// (planilla importa dto: sería un ciclo).
type exportQueries interface {
	ObtenerDetallesPlanillaParaDispersion(ctx context.Context, periodoID int64, estados []string) ([]dto.ObtenerDetallesPlanillaParaDispersionRow, error)
	ObtenerDatosBoletaPago(ctx context.Context, periodoID, empleadoID int64, estados []string) (dto.ObtenerDatosBoletaPagoRow, error)
	ObtenerDesgloseConceptosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]dto.ObtenerDesgloseConceptosBoletaPagoRow, error)
	ObtenerEmbargosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]dto.ObtenerEmbargosBoletaPagoRow, error)
	ObtenerIncapacidadesBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]dto.ObtenerIncapacidadesBoletaPagoRow, error)
	InsertarExportacionGenerada(ctx context.Context, p dto.InsertarExportacionGeneradaParams) (int64, error)
}

// Verificación en compilación: *dto.Queries implementa exportQueries.
var _ exportQueries = (*dto.Queries)(nil)

func estadosComoStrings(es []planilla.EstadoPeriodo) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = string(e)
	}
	return out
}

// estadosParaBoleta: aprobada|pagada|contabilizada (mismo conjunto que
// EstadosParaAcumulados: montos ya aprobados).
func estadosParaBoleta() []string { return estadosComoStrings(planilla.EstadosParaAcumulados()) }

// estadosDispersion: aprobada|pagada.
func estadosDispersion() []string { return estadosComoStrings(planilla.EstadosParaDispersion()) }

type exportRepository struct {
	q exportQueries
	d planilla.Descifrador
}

// NewExportRepository conecta ExportService con la base de datos; descifra
// la cédula y la cuenta al leer. Solo lecturas.
func NewExportRepository(q exportQueries, d planilla.Descifrador) planilla.ExportRepository {
	return &exportRepository{q: q, d: d}
}

func (r *exportRepository) descifrar(c dto.CampoSensible, v dto.ValorSensible, empleadoID int64) (string, error) {
	if len(v.Cifrado) == 0 {
		return "", nil
	}
	if r.d == nil {
		return "", fmt.Errorf("infra: no hay llavero para descifrar %s del empleado %d", c.Campo(), empleadoID)
	}
	valor, err := r.d.Descifrar(c.Campo(), v.Cifrado, v.Kid.String)
	if err != nil {
		return "", fmt.Errorf("infra: descifrando %s del empleado %d: %w", c.Campo(), empleadoID, err)
	}
	return valor, nil
}

func (r *exportRepository) ObtenerDetallesPlanillaParaDispersion(ctx context.Context, periodoID int64) ([]planilla.FilaDispersionBancaria, error) {
	rows, err := r.q.ObtenerDetallesPlanillaParaDispersion(ctx, periodoID, estadosDispersion())
	if err != nil {
		return nil, err
	}
	out := make([]planilla.FilaDispersionBancaria, 0, len(rows))
	for _, x := range rows {
		cedula, err := r.descifrar(dto.CampoIdentificacion, x.Cedula, x.EmpleadoID)
		if err != nil {
			return nil, err
		}
		iban, err := r.descifrar(dto.CampoCuenta, x.IBAN, x.EmpleadoID)
		if err != nil {
			return nil, err
		}
		out = append(out, planilla.FilaDispersionBancaria{
			EmpleadoID:     x.EmpleadoID,
			Cedula:         cedula,
			NombreCompleto: x.NombreCompleto,
			IBAN:           iban,
			NetoPagado:     x.NetoPagado,
		})
	}
	return out, nil
}

// ObtenerDatosBoletaPago propaga sql.ErrNoRows si no existe el detalle o el
// período no está en un estado que permita boleta.
func (r *exportRepository) ObtenerDatosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) (planilla.EncabezadoBoletaPago, error) {
	x, err := r.q.ObtenerDatosBoletaPago(ctx, periodoID, empleadoID, estadosParaBoleta())
	if err != nil {
		return planilla.EncabezadoBoletaPago{}, err
	}
	cedula, err := r.descifrar(dto.CampoIdentificacion, x.Cedula, x.EmpleadoID)
	if err != nil {
		return planilla.EncabezadoBoletaPago{}, err
	}
	return planilla.EncabezadoBoletaPago{
		PlanillaDetalleEmpleadoID: x.PlanillaDetalleEmpleadoID,
		EmpleadoID:                x.EmpleadoID,
		PeriodoID:                 x.PeriodoID,
		NombreCompleto:            x.NombreCompleto,
		Cedula:                    cedula,
		SalarioBasePeriodo:        x.SalarioBasePeriodo,
		TotalIngresos:             x.TotalIngresos,
		TotalDeducciones:          x.TotalDeducciones,
		NetoPagado:                x.NetoPagado,
	}, nil
}

func (r *exportRepository) ObtenerDesgloseConceptosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) ([]planilla.ConceptoBoletaPago, error) {
	rows, err := r.q.ObtenerDesgloseConceptosBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	out := make([]planilla.ConceptoBoletaPago, 0, len(rows))
	for _, x := range rows {
		out = append(out, planilla.ConceptoBoletaPago{
			CodigoConcepto: x.CodigoConcepto,
			NombreConcepto: x.NombreConcepto,
			Naturaleza:     x.Naturaleza,
			Monto:          x.Monto,
			Cantidad:       x.Cantidad,
			OrdenCalculo:   x.OrdenCalculo,
		})
	}
	return out, nil
}

func (r *exportRepository) ObtenerEmbargosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) ([]planilla.EmbargoBoletaPago, error) {
	rows, err := r.q.ObtenerEmbargosBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	out := make([]planilla.EmbargoBoletaPago, 0, len(rows))
	for _, x := range rows {
		out = append(out, planilla.EmbargoBoletaPago{
			EmbargoID:         x.EmbargoID,
			TipoEmbargoCodigo: x.TipoEmbargoCodigo,
			TipoEmbargoNombre: x.TipoEmbargoNombre,
			OrdenPrioridad:    x.OrdenPrioridad,
			Monto:             x.Monto,
		})
	}
	return out, nil
}

func (r *exportRepository) ObtenerIncapacidadesBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) ([]planilla.IncapacidadBoletaPago, error) {
	rows, err := r.q.ObtenerIncapacidadesBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	out := make([]planilla.IncapacidadBoletaPago, 0, len(rows))
	for _, x := range rows {
		out = append(out, planilla.IncapacidadBoletaPago(x))
	}
	return out, nil
}

type exportTx struct {
	planilla.ExportRepository
	q exportQueries
}

func (t *exportTx) RegistrarExportacion(ctx context.Context, r planilla.ExportacionRegistro) (int64, error) {
	return t.q.InsertarExportacionGenerada(ctx, dto.InsertarExportacionGeneradaParams{
		PeriodoID:     r.PeriodoID,
		FormatoCodigo: r.FormatoCodigo,
		NombreArchivo: r.NombreArchivo,
		HashSHA256:    r.HashSHA256,
		CantidadFilas: r.CantidadFilas,
		TotalMonto:    r.TotalMonto,
		UsuarioID:     r.UsuarioID,
	})
}

type exportUnitOfWork struct {
	sqlDB *sql.DB
	d     planilla.Descifrador
}

// NewExportUnitOfWork: wiring típico
// planilla.NewExportService(infra.NewExportRepository(dto.New(sqlDB), llavero), infra.NewExportUnitOfWork(sqlDB, llavero)).
func NewExportUnitOfWork(sqlDB *sql.DB, d planilla.Descifrador) planilla.ExportUnitOfWork {
	return &exportUnitOfWork{sqlDB: sqlDB, d: d}
}

func (u *exportUnitOfWork) EjecutarAuditado(ctx context.Context, usuarioID int64, fn func(ctx context.Context, tx planilla.ExportTx) error) error {
	return db.WithAuditContext(ctx, u.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		return fn(ctx, &exportTx{ExportRepository: NewExportRepository(q, u.d), q: q})
	})
}
