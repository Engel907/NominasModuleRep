// Fase 18B — Cargador único de parámetros de cálculo.
//
// Resuelve en UN solo lugar todo parámetro "vigente a la fecha X" y convierte
// los porcentajes de la BD (10.83) a fracción (0.1083) con porcentaje.Porcentaje
// (regla D3). El dominio solo recibe fracciones; nadie más divide/multiplica
// por 100.
//
// Convención de fecha: todo se resuelve a la fecha FIN del período
// (FechaEfectiva normal del motor; mismo supuesto que contabilidad/sql_adapters).
// fechaInicio solo se valida (inicio <= fin).
//
// Regla del motor (sin defaults implícitos): si falta un parámetro obligatorio
// o los tramos de renta son incoherentes, se retorna error explícito.
package planilla

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/shared/porcentaje"
)

// Códigos de concepto sembrados (000002) de los que salen los % de CCSS.
const (
	codigoCCSSObrero   = "CCSS_OBR"
	codigoCCSSPatronal = "CCSS_PAT"
)

// Claves de parametros_generales usadas por el cargador.
const (
	ClaveHorasMensualesOrdinarias = "HORAS_MENSUALES_ORDINARIAS"
)

// ErrParametroFaltante envuelve cualquier parámetro obligatorio no vigente.
var ErrParametroFaltante = errors.New("planilla: parámetro de cálculo faltante")

// ErrTramosRentaInvalidos: huecos, solapes o estructura incoherente en renta.
var ErrTramosRentaInvalidos = errors.New("planilla: tramos de renta inválidos")

// TramoEscalaEmbargo: escala Art. 172 CT (porcentaje embargable por tramo de
// salario). Porcentaje YA en fracción.
type TramoEscalaEmbargo struct {
	OrdenTramo   int32
	SalarioDesde decimal.Decimal
	SalarioHasta decimal.NullDecimal // NULL = tramo abierto
	Porcentaje   decimal.Decimal     // fracción
}

// BaseMinimaContributivaFila: monto vigente por régimen CCSS.
type BaseMinimaContributivaFila struct {
	RegimenCCSSID int64
	Monto         decimal.Decimal
}

// FuenteParametros: lecturas "vigente a fecha" que necesita el cargador.
// Los % llegan TAL COMO están en BD (0-100).
type FuenteParametros interface {
	ListarCargasSocialesEnFecha(ctx context.Context, fecha time.Time) ([]dto.CargaSocialVigenteRow, error)
	ListarTramosRentaEnFecha(ctx context.Context, fecha time.Time) ([]dto.TramoRentaVigenteRow, error)
	ListarCreditosFiscalesEnFecha(ctx context.Context, fecha time.Time) ([]dto.CreditoFiscalVigenteRow, error)
	ListarParametrosHorasExtraEnFecha(ctx context.Context, fecha time.Time) ([]dto.ParametroHoraExtraRow, error)
	ListarTramosSubsidioIncapacidadEnFecha(ctx context.Context, fecha time.Time) ([]dto.TramoSubsidioIncapacidadRow, error)
	// Escala de embargo en % de BD (0-100).
	ListarEscalaEmbargoEnFecha(ctx context.Context, fecha time.Time) ([]TramoEscalaEmbargo, error)
	ListarBaseMinimaContributivaEnFecha(ctx context.Context, fecha time.Time) ([]BaseMinimaContributivaFila, error)
}

// GeneralesLector: subconjunto de parametros.Generales usado aquí.
type GeneralesLector interface {
	Decimal(ctx context.Context, clave string, fecha time.Time) (decimal.Decimal, error)
}

type CargadorParametros struct {
	fuente    FuenteParametros
	generales GeneralesLector
}

func NewCargadorParametros(f FuenteParametros, g GeneralesLector) *CargadorParametros {
	return &CargadorParametros{fuente: f, generales: g}
}

func faltante(formato string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrParametroFaltante, fmt.Sprintf(formato, a...))
}

func frac(d decimal.Decimal, ctxMsg string) (decimal.Decimal, error) {
	p, err := porcentaje.DesdePorcentaje(d)
	if err != nil {
		return decimal.Zero, fmt.Errorf("planilla: %s: %w", ctxMsg, err)
	}
	return p.Fraccion(), nil
}

// Cargar resuelve todos los parámetros a la fecha fin del período.
//
// Quedan SIN cargar aquí (dependen de datos que este método no recibe):
//   - SalarioMinimoInembargable: es por empleado (clasificación del puesto);
//     usar Queries.ObtenerSalarioMinimoEnFecha por empleado.
//   - FraccionMensualPeriodo / DiasPeriodoCompleto: dependen del tipo de
//     planilla (quincena 0.5/15, mes 1/30); los fija el orquestador. El motor
//     ya falla si faltan cuando prorratea SAL_BASE.
func (c *CargadorParametros) Cargar(ctx context.Context, fechaInicio, fechaFin time.Time) (*ParametrosCalculo, error) {
	if fechaFin.Before(fechaInicio) {
		return nil, fmt.Errorf("planilla: fechaFin %s anterior a fechaInicio %s",
			fechaFin.Format("2006-01-02"), fechaInicio.Format("2006-01-02"))
	}
	f := fechaFin
	out := &ParametrosCalculo{}

	if err := c.cargarCCSS(ctx, f, out); err != nil {
		return nil, err
	}
	if err := c.cargarRenta(ctx, f, out); err != nil {
		return nil, err
	}
	if err := c.cargarCreditos(ctx, f, out); err != nil {
		return nil, err
	}
	if err := c.cargarHorasExtra(ctx, f, out); err != nil {
		return nil, err
	}
	if err := c.cargarSubsidios(ctx, f, out); err != nil {
		return nil, err
	}
	if err := c.cargarBaseMinima(ctx, f, out); err != nil {
		return nil, err
	}
	if err := c.cargarEscalaEmbargo(ctx, f, out); err != nil {
		return nil, err
	}

	h, err := c.generales.Decimal(ctx, ClaveHorasMensualesOrdinarias, f)
	if err != nil {
		return nil, err // ya es ErrParametroNoConfigurado explícito
	}
	if !h.IsPositive() {
		return nil, fmt.Errorf("planilla: %s debe ser > 0, es %s", ClaveHorasMensualesOrdinarias, h)
	}
	out.HorasMensualesOrdinarias = h
	return out, nil
}

func (c *CargadorParametros) cargarCCSS(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	rows, err := c.fuente.ListarCargasSocialesEnFecha(ctx, f)
	if err != nil {
		return fmt.Errorf("planilla: leyendo cargas sociales: %w", err)
	}
	var nObr, nPat int
	for _, r := range rows {
		switch r.ConceptoCodigo {
		case codigoCCSSObrero:
			nObr++
			v, err := frac(r.PorcentajeObrero, "CCSS obrero")
			if err != nil {
				return err
			}
			out.CargaSocial.PorcentajeObrero = v
		case codigoCCSSPatronal:
			nPat++
			v, err := frac(r.PorcentajePatronal, "CCSS patronal")
			if err != nil {
				return err
			}
			out.CargaSocial.PorcentajePatronal = v
		}
	}
	if nObr != 1 || nPat != 1 {
		return faltante("se esperaba 1 fila vigente de %s y 1 de %s al %s (hay %d y %d)",
			codigoCCSSObrero, codigoCCSSPatronal, f.Format("2006-01-02"), nObr, nPat)
	}
	return nil
}

func (c *CargadorParametros) cargarRenta(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	rows, err := c.fuente.ListarTramosRentaEnFecha(ctx, f)
	if err != nil {
		return fmt.Errorf("planilla: leyendo tramos de renta: %w", err)
	}
	if len(rows) == 0 {
		return faltante("no hay tramos de renta vigentes al %s", f.Format("2006-01-02"))
	}
	if err := ValidarTramosRenta(rows); err != nil {
		return err
	}
	sorted := append([]dto.TramoRentaVigenteRow(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].OrdenTramo < sorted[j].OrdenTramo })
	for _, r := range sorted {
		p, err := frac(r.Porcentaje, fmt.Sprintf("renta tramo %d", r.OrdenTramo))
		if err != nil {
			return err
		}
		t := TramoRenta{OrdenTramo: r.OrdenTramo, MontoDesde: r.MontoDesde, Porcentaje: p}
		if r.MontoHasta.Valid {
			t.MontoHasta = r.MontoHasta.Decimal
		}
		t.Abierto = !r.MontoHasta.Valid
		out.TramosRenta = append(out.TramosRenta, t)
	}
	return nil
}

// ValidarTramosRenta exige escala contigua: orden 1..n sin repetir, el primero
// desde 0, cada desde == hasta del anterior (sin huecos ni solapes), hasta >
// desde, y solo el último abierto. Exportada para testearla sin BD.
func ValidarTramosRenta(rows []dto.TramoRentaVigenteRow) error {
	s := append([]dto.TramoRentaVigenteRow(nil), rows...)
	if len(s) == 0 {
		return fmt.Errorf("%w: no hay tramos", ErrTramosRentaInvalidos)
	}
	sort.Slice(s, func(i, j int) bool { return s[i].OrdenTramo < s[j].OrdenTramo })
	if !s[0].MontoDesde.IsZero() {
		return fmt.Errorf("%w: el primer tramo debe iniciar en 0 y inicia en %s", ErrTramosRentaInvalidos, s[0].MontoDesde)
	}
	for i, t := range s {
		if i > 0 && t.OrdenTramo == s[i-1].OrdenTramo {
			return fmt.Errorf("%w: orden_tramo %d repetido", ErrTramosRentaInvalidos, t.OrdenTramo)
		}
		last := i == len(s)-1
		if !t.MontoHasta.Valid {
			if !last {
				return fmt.Errorf("%w: tramo %d abierto pero no es el último", ErrTramosRentaInvalidos, t.OrdenTramo)
			}
			continue
		}
		if last {
			return fmt.Errorf("%w: el último tramo (%d) debe ser abierto (monto_hasta NULL)", ErrTramosRentaInvalidos, t.OrdenTramo)
		}
		if !t.MontoHasta.Decimal.GreaterThan(t.MontoDesde) {
			return fmt.Errorf("%w: tramo %d con hasta <= desde", ErrTramosRentaInvalidos, t.OrdenTramo)
		}
		next := s[i+1]
		switch next.MontoDesde.Cmp(t.MontoHasta.Decimal) {
		case 1:
			return fmt.Errorf("%w: hueco entre tramo %d (hasta %s) y tramo %d (desde %s)",
				ErrTramosRentaInvalidos, t.OrdenTramo, t.MontoHasta.Decimal, next.OrdenTramo, next.MontoDesde)
		case -1:
			return fmt.Errorf("%w: solape entre tramo %d (hasta %s) y tramo %d (desde %s)",
				ErrTramosRentaInvalidos, t.OrdenTramo, t.MontoHasta.Decimal, next.OrdenTramo, next.MontoDesde)
		}
	}
	return nil
}

func (c *CargadorParametros) cargarCreditos(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	rows, err := c.fuente.ListarCreditosFiscalesEnFecha(ctx, f)
	if err != nil {
		return fmt.Errorf("planilla: leyendo créditos fiscales: %w", err)
	}
	if len(rows) == 0 {
		return faltante("no hay créditos fiscales de renta vigentes al %s", f.Format("2006-01-02"))
	}
	out.CreditosFiscales = make(map[int64]CreditoFiscalParametro, len(rows))
	for _, r := range rows {
		out.CreditosFiscales[r.TipoCreditoFiscalID] = CreditoFiscalParametro{
			TipoCreditoFiscalID: r.TipoCreditoFiscalID, MontoCredito: r.MontoCredito}
	}
	return nil
}

func (c *CargadorParametros) cargarHorasExtra(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	rows, err := c.fuente.ListarParametrosHorasExtraEnFecha(ctx, f)
	if err != nil {
		return fmt.Errorf("planilla: leyendo parametros_horas_extra: %w", err)
	}
	if len(rows) == 0 {
		return faltante("no hay parametros_horas_extra vigentes al %s", f.Format("2006-01-02"))
	}
	// factor_recargo es multiplicador (1.5), NO porcentaje: no se convierte.
	out.FactoresHoraExtra = make(map[int64]ParametroHoraExtra, len(rows))
	for _, r := range rows {
		out.FactoresHoraExtra[r.TipoHoraExtraID] = ParametroHoraExtra{
			TipoHoraExtraID: r.TipoHoraExtraID, FactorRecargo: r.FactorRecargo}
	}
	return nil
}

func (c *CargadorParametros) cargarSubsidios(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	rows, err := c.fuente.ListarTramosSubsidioIncapacidadEnFecha(ctx, f)
	if err != nil {
		return fmt.Errorf("planilla: leyendo subsidios de incapacidad: %w", err)
	}
	if len(rows) == 0 {
		return faltante("no hay parametros_subsidios_incapacidad vigentes al %s", f.Format("2006-01-02"))
	}
	for _, r := range rows {
		p, err := frac(r.PorcentajePatronal, fmt.Sprintf("subsidio %s día %d", r.EntidadCodigo, r.DiaDesde))
		if err != nil {
			return err
		}
		out.SubsidiosIncapacidad = append(out.SubsidiosIncapacidad, TramoSubsidioIncapacidad{
			EntidadEmisoraID: r.EntidadEmisoraID, DiaDesde: r.DiaDesde, Porcentaje: p})
	}
	return nil
}

func (c *CargadorParametros) cargarBaseMinima(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	rows, err := c.fuente.ListarBaseMinimaContributivaEnFecha(ctx, f)
	if err != nil {
		return fmt.Errorf("planilla: leyendo base mínima contributiva: %w", err)
	}
	if len(rows) == 0 {
		return faltante("no hay parametros_base_minima_contributiva vigentes al %s", f.Format("2006-01-02"))
	}
	out.BaseMinimaContributiva = make(map[int64]decimal.Decimal, len(rows))
	for _, r := range rows {
		out.BaseMinimaContributiva[r.RegimenCCSSID] = r.Monto
	}
	return nil
}

func (c *CargadorParametros) cargarEscalaEmbargo(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	rows, err := c.fuente.ListarEscalaEmbargoEnFecha(ctx, f)
	if err != nil {
		return fmt.Errorf("planilla: leyendo parametros_embargo_escala: %w", err)
	}
	if len(rows) == 0 {
		return faltante("no hay parametros_embargo_escala vigentes al %s", f.Format("2006-01-02"))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].OrdenTramo < rows[j].OrdenTramo })
	for _, r := range rows {
		p, err := frac(r.Porcentaje, fmt.Sprintf("escala embargo tramo %d", r.OrdenTramo))
		if err != nil {
			return err
		}
		r.Porcentaje = p
		out.EscalaEmbargo = append(out.EscalaEmbargo, r)
	}
	return nil
}
