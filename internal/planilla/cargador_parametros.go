package planilla

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
	"planillas-ccdr/internal/shared/porcentaje"
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
	ListarParametrosJornadaEnFecha(ctx context.Context, fecha time.Time) ([]dto.ParametroJornadaRow, error)
	// Fracción (0.0833); sql.ErrNoRows si no hay vigente.
	ObtenerPorcentajeSalarioEscolarEnFecha(ctx context.Context, fecha time.Time) (decimal.Decimal, error)
}

// GeneralesLector: subconjunto de parametros.Generales usado aquí.
type GeneralesLector interface {
	Decimal(ctx context.Context, clave string, fecha time.Time) (decimal.Decimal, error)
	Texto(ctx context.Context, clave string, fecha time.Time) (string, error)
}

type CargadorParametros struct {
	fuente    FuenteParametros
	generales GeneralesLector

	ajustes      AjustesEmpleadoLector
	paraEmpleado func(empleadoID int64) GeneralesLector
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
	if err := c.cargarGenerales(ctx, f, out); err != nil {
		return nil, err
	}
	if err := c.cargarJornadas(ctx, f, out); err != nil {
		return nil, err
	}
	return out, nil
}

// decimalGeneral lee un decimal de parametros_generales. Si la clave no está
// vigente, el error envuelve ErrParametroFaltante y también el error tipado de
// parametros (errors.Is/As funcionan con ambos; el mensaje trae la clave).
func (c *CargadorParametros) decimalGeneral(ctx context.Context, clave string, f time.Time) (decimal.Decimal, error) {
	d, err := c.generales.Decimal(ctx, clave, f)
	if err != nil {
		var nc parametros.ErrParametroNoConfigurado
		if errors.As(err, &nc) {
			return decimal.Zero, fmt.Errorf("%w: %w", ErrParametroFaltante, err)
		}
		return decimal.Zero, err
	}
	return d, nil
}

func (c *CargadorParametros) cargarGenerales(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	div, err := c.decimalGeneral(ctx, parametros.ClaveDivisorDiasMes, f)
	if err != nil {
		return err
	}
	if !div.IsPositive() {
		return fmt.Errorf("planilla: %s debe ser > 0, es %s", parametros.ClaveDivisorDiasMes, div)
	}
	out.DivisorDiasMes = div

	smi, err := c.decimalGeneral(ctx, parametros.ClaveSalarioMinimoInembargable, f)
	if err != nil {
		return err
	}
	if !smi.IsPositive() {
		return fmt.Errorf("planilla: %s debe ser > 0, es %s", parametros.ClaveSalarioMinimoInembargable, smi)
	}
	out.SalarioMinimoInembargable = smi

	modo, err := c.generales.Texto(ctx, ClaveVacacionesModoPago, f)
	var nc parametros.ErrParametroNoConfigurado
	switch {
	case errors.As(err, &nc):
		modo = ""
	case err != nil:
		return err
	case !ModoPagoVacacionesValido(modo):
		return fmt.Errorf("planilla: %s = %q inválido (compensacion | disfrute | ambos)", ClaveVacacionesModoPago, modo)
	}
	out.VacacionesModoPago = modo

	if out.VacacionesSalarioBase, err = c.textoOpcional(ctx, ClaveVacacionesSalarioBase, f, SalarioBaseValida, "vigente | promedio | mayor"); err != nil {
		return err
	}
	if out.IncapacidadSalarioBase, err = c.textoOpcional(ctx, ClaveIncapacidadSalarioBase, f, SalarioBaseValida, "vigente | promedio | mayor"); err != nil {
		return err
	}
	if out.VacacionesPromedioSemanas, err = c.enteroOpcional(ctx, ClaveVentanaPromedioVacacionesSemanas, f); err != nil {
		return err
	}
	if out.IncapacidadPromedioMeses, err = c.enteroOpcional(ctx, ClaveIncapacidadPromedioMeses, f); err != nil {
		return err
	}

	if out.FactorFeriadoObligatorio, err = c.factorOpcional(ctx, ClaveFeriadoFactorObligatorio, f); err != nil {
		return err
	}
	out.FactorFeriadoNoObligatorio, err = c.factorOpcional(ctx, ClaveFeriadoFactorNoObligatorio, f)
	return err
}

// textoOpcional: "" si la clave no está configurada; si lo está, debe ser
// uno de los valores admitidos.
func (c *CargadorParametros) textoOpcional(ctx context.Context, clave string, f time.Time, valido func(string) bool, admitidos string) (string, error) {
	v, err := c.generales.Texto(ctx, clave, f)
	var nc parametros.ErrParametroNoConfigurado
	switch {
	case errors.As(err, &nc):
		return "", nil
	case err != nil:
		return "", err
	case !valido(v):
		return "", fmt.Errorf("planilla: %s = %q inválido (%s)", clave, v, admitidos)
	}
	return v, nil
}

// enteroOpcional: 0 si la clave no está configurada; si lo está, debe ser un
// entero > 0.
func (c *CargadorParametros) enteroOpcional(ctx context.Context, clave string, f time.Time) (int, error) {
	d, err := c.generales.Decimal(ctx, clave, f)
	var nc parametros.ErrParametroNoConfigurado
	switch {
	case errors.As(err, &nc):
		return 0, nil
	case err != nil:
		return 0, err
	case !d.IsPositive() || !d.Equal(d.Truncate(0)):
		return 0, fmt.Errorf("planilla: %s debe ser un entero > 0, es %s", clave, d)
	}
	return int(d.IntPart()), nil
}

// factorOpcional: nil si la clave no está configurada; si lo está, debe ser
// >= 0.
func (c *CargadorParametros) factorOpcional(ctx context.Context, clave string, f time.Time) (*decimal.Decimal, error) {
	d, err := c.generales.Decimal(ctx, clave, f)
	var nc parametros.ErrParametroNoConfigurado
	switch {
	case errors.As(err, &nc):
		return nil, nil
	case err != nil:
		return nil, err
	case d.IsNegative():
		return nil, fmt.Errorf("planilla: %s debe ser >= 0, es %s", clave, d)
	}
	return &d, nil
}

// cargarCCSS: tasas de todos los conceptos de carga social vigentes. Qué
// concepto es obrero o patronal lo dice su fórmula ('lado'); aquí solo se
// exige una fila por concepto y régimen.
func (c *CargadorParametros) cargarCCSS(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	rows, err := c.fuente.ListarCargasSocialesEnFecha(ctx, f)
	if err != nil {
		return fmt.Errorf("planilla: leyendo cargas sociales: %w", err)
	}
	if len(rows) == 0 {
		return faltante("no hay parametros_cargas_sociales vigentes al %s", f.Format("2006-01-02"))
	}
	out.CargasSociales = make(map[int64]map[int64]ParametroCargaSocial, len(rows))
	for _, r := range rows {
		regimen := int64(0)
		if r.RegimenCCSSID.Valid {
			regimen = r.RegimenCCSSID.Int64
		}
		etiqueta := fmt.Sprintf("carga social %s (regimen_ccss_id=%d)", r.ConceptoCodigo, regimen)
		obr, err := frac(r.PorcentajeObrero, etiqueta+" obrero")
		if err != nil {
			return err
		}
		pat, err := frac(r.PorcentajePatronal, etiqueta+" patronal")
		if err != nil {
			return err
		}
		porRegimen := out.CargasSociales[r.ConceptoSalarialID]
		if porRegimen == nil {
			porRegimen = map[int64]ParametroCargaSocial{}
			out.CargasSociales[r.ConceptoSalarialID] = porRegimen
		}
		if _, dup := porRegimen[regimen]; dup {
			return fmt.Errorf("planilla: %s tiene más de una fila vigente al %s", etiqueta, f.Format("2006-01-02"))
		}
		porRegimen[regimen] = ParametroCargaSocial{ConceptoSalarialID: r.ConceptoSalarialID, RegimenCCSSID: regimen,
			PorcentajePatronal: pat, PorcentajeObrero: obr}
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
		etiqueta := fmt.Sprintf("subsidio %s día %d", r.EntidadCodigo, r.DiaDesde)
		pat, err := frac(r.PorcentajePatronal, etiqueta+" (patrono)")
		if err != nil {
			return err
		}
		ent := decimal.Zero
		if r.PorcentajeEntidad.Valid {
			if ent, err = frac(r.PorcentajeEntidad.Decimal, etiqueta+" (entidad)"); err != nil {
				return err
			}
		}
		out.SubsidiosIncapacidad = append(out.SubsidiosIncapacidad, TramoSubsidio{
			EntidadEmisoraID: r.EntidadEmisoraID, DiaDesde: r.DiaDesde, DiaHasta: r.DiaHasta,
			PctPatronal: pat, PctEntidad: ent})
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

// cargarJornadas: divisor de la hora ordinaria por jornada. Una jornada
// sin fila vigente no falla aquí: falla, con error explícito, el empleado de
// esa jornada que tenga horas extra (MIXTA y NOCTURNA no se siembran).
func (c *CargadorParametros) cargarJornadas(ctx context.Context, f time.Time, out *ParametrosCalculo) error {
	rows, err := c.fuente.ListarParametrosJornadaEnFecha(ctx, f)
	if err != nil {
		return fmt.Errorf("planilla: leyendo parametros_jornada: %w", err)
	}
	out.DivisorPorJornada = make(map[int64]decimal.Decimal, len(rows))
	for _, r := range rows {
		if !r.DivisorHorasMensuales.IsPositive() {
			return fmt.Errorf("planilla: divisor de horas de la jornada %s debe ser > 0, es %s", r.JornadaCodigo, r.DivisorHorasMensuales)
		}
		out.DivisorPorJornada[r.JornadaID] = r.DivisorHorasMensuales
	}
	return nil
}

// ClaveAguinaldoMeses: divisor del aguinaldo (parametros_generales, 000118).
const ClaveAguinaldoMeses = "AGUINALDO_MESES"

// CargarAcumulado completa los parámetros de un período de clase aguinaldo o
// salario_escolar. Faltante → ErrParametroFaltante.
// CargarLiquidacion: parámetros propios de la planilla de liquidación.
// Cotiza sin base mínima CCSS; LIQ_PRESTAMOS_MODO y LIQ_EMBARGO_MONTO_FIJO no
// se exigen aquí (solo los necesita quien tiene préstamos o embargos de monto
// fijo), pero si existen deben ser válidos.
func (c *CargadorParametros) CargarLiquidacion(ctx context.Context, fecha time.Time, out *ParametrosCalculo) error {
	out.SinBaseMinimaCCSS = true
	modo, err := c.textoOpcional(ctx, ClaveLiqPrestamosModo, fecha, ModoPrestamosLiquidacionValido, "cuota | saldo | ninguno")
	if err != nil {
		return err
	}
	out.LiqPrestamosModo = modo
	out.LiqEmbargoMontoFijo, err = c.textoOpcional(ctx, ClaveLiqEmbargoMontoFijo, fecha, ModoEmbargoMontoFijoValido, "mensual | por_periodo")
	return err
}

func (c *CargadorParametros) CargarAcumulado(ctx context.Context, clase ClaseTipoPlanilla, fecha time.Time, out *ParametrosCalculo) error {
	switch clase {
	case ClaseAguinaldo:
		m, err := c.decimalGeneral(ctx, ClaveAguinaldoMeses, fecha)
		if err != nil {
			return err
		}
		if !m.IsPositive() {
			return fmt.Errorf("planilla: %s debe ser > 0, es %s", ClaveAguinaldoMeses, m)
		}
		out.AguinaldoMeses = m
	case ClaseSalarioEscolar:
		pct, err := c.fuente.ObtenerPorcentajeSalarioEscolarEnFecha(ctx, fecha)
		if errors.Is(err, sql.ErrNoRows) {
			return faltante("no hay parametros_salario_escolar vigente al %s", fecha.Format("2006-01-02"))
		}
		if err != nil {
			return fmt.Errorf("planilla: leyendo parametros_salario_escolar: %w", err)
		}
		if !pct.IsPositive() || pct.GreaterThan(decimal.NewFromInt(1)) {
			return fmt.Errorf("planilla: parametros_salario_escolar.porcentaje debe ser una fracción en (0, 1], es %s", pct)
		}
		out.PorcentajeSalarioEscolar = pct
	default:
		return ErrClaseNoAcumulable
	}
	return nil
}

// CargarMetodoRenta: RENTA_METODO_PERIODO, obligatorio solo en un período
// regular que cubre una fracción del mes.
func (c *CargadorParametros) CargarMetodoRenta(ctx context.Context, fecha time.Time, out *ParametrosCalculo) error {
	m, err := c.generales.Texto(ctx, ClaveRentaMetodoPeriodo, fecha)
	var nc parametros.ErrParametroNoConfigurado
	switch {
	case errors.As(err, &nc):
		return fmt.Errorf("%w: %w", ErrParametroFaltante, err)
	case err != nil:
		return err
	case !MetodoRentaValido(m):
		return fmt.Errorf("planilla: %s = %q inválido (%s | %s)", ClaveRentaMetodoPeriodo, m, RentaAcumuladoMes, RentaProyeccion)
	}
	out.RentaMetodoPeriodo = m
	return nil
}

// AjustesEmpleadoLector: valores propios de un empleado vigentes a una fecha
// (empleado_parametros_generales, empleado_parametros_horas_extra y
// empleado_parametros_jornada).
type AjustesEmpleadoLector interface {
	ListarParametrosGeneralesEmpleadoEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]dto.ParametroGeneralEmpleadoRow, error)
	ListarParametrosHorasExtraEmpleadoEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]dto.ParametroHoraExtraEmpleadoRow, error)
	ListarParametrosJornadaEmpleadoEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]dto.ParametroJornadaEmpleadoRow, error)
}

// AjustesTablasEmpleado aplica sobre p los factores de hora extra y los
// divisores de jornada propios del empleado vigentes a `fecha` (sin tocar los
// mapas del período) y devuelve lo aplicado (tabla.columna[clave] -> valor);
// nil si no tiene o no están habilitados.
func (c *CargadorParametros) AjustesTablasEmpleado(ctx context.Context, empleadoID int64, fecha time.Time, p *ParametrosCalculo) (map[string]string, error) {
	if c.ajustes == nil {
		return nil, nil
	}
	he, err := c.ajustes.ListarParametrosHorasExtraEmpleadoEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return nil, fmt.Errorf("planilla: leyendo factores de hora extra propios del empleado_id=%d: %w", empleadoID, err)
	}
	jo, err := c.ajustes.ListarParametrosJornadaEmpleadoEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return nil, fmt.Errorf("planilla: leyendo divisores de jornada propios del empleado_id=%d: %w", empleadoID, err)
	}
	if len(he) == 0 && len(jo) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(he)+len(jo))
	if len(he) > 0 {
		p.FactoresHoraExtraEmpleado = make(map[int64]ParametroHoraExtra, len(he))
		for _, r := range he {
			if r.FactorRecargo.LessThan(decimal.NewFromInt(1)) {
				return nil, fmt.Errorf("planilla: empleado_id=%d, factor propio de hora extra %s < 1", empleadoID, r.FactorRecargo)
			}
			p.FactoresHoraExtraEmpleado[r.TipoHoraExtraID] = ParametroHoraExtra{TipoHoraExtraID: r.TipoHoraExtraID, FactorRecargo: r.FactorRecargo}
			out[fmt.Sprintf("empleado_parametros_horas_extra.factor_recargo[tipo_hora_extra_id=%d]", r.TipoHoraExtraID)] = r.FactorRecargo.String()
		}
	}
	if len(jo) > 0 {
		div := make(map[int64]decimal.Decimal, len(p.DivisorPorJornada)+len(jo))
		for k, v := range p.DivisorPorJornada {
			div[k] = v
		}
		for _, r := range jo {
			if !r.DivisorHorasMensuales.IsPositive() {
				return nil, fmt.Errorf("planilla: empleado_id=%d, divisor propio de la jornada %d debe ser > 0", empleadoID, r.JornadaID)
			}
			div[r.JornadaID] = r.DivisorHorasMensuales
			out[fmt.Sprintf("empleado_parametros_jornada.divisor_horas_mensuales[jornada_id=%d]", r.JornadaID)] = r.DivisorHorasMensuales.String()
		}
		p.DivisorPorJornada = div
	}
	return out, nil
}

// ConAjustesPorEmpleado habilita los parámetros propios de un empleado:
// lector dice si el empleado tiene alguno vigente y paraEmpleado da el lector
// de generales que aplica los generales propios sobre los generales.
func (c *CargadorParametros) ConAjustesPorEmpleado(lector AjustesEmpleadoLector, paraEmpleado func(empleadoID int64) GeneralesLector) *CargadorParametros {
	c.ajustes, c.paraEmpleado = lector, paraEmpleado
	return c
}

// AjustesEmpleado: clave -> valor de los parámetros generales propios del
// empleado vigentes a `fecha`; nil si no tiene o no están habilitados.
func (c *CargadorParametros) AjustesEmpleado(ctx context.Context, empleadoID int64, fecha time.Time) (map[string]string, error) {
	if c.ajustes == nil || c.paraEmpleado == nil {
		return nil, nil
	}
	filas, err := c.ajustes.ListarParametrosGeneralesEmpleadoEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return nil, fmt.Errorf("planilla: leyendo parámetros propios del empleado_id=%d: %w", empleadoID, err)
	}
	if len(filas) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(filas))
	for _, f := range filas {
		out[f.Clave] = f.Valor
	}
	return out, nil
}

// ParametrosEmpleado: los parámetros del período con los generales propios del
// empleado. Vuelve a resolver todo lo que sale de parametros_generales (base,
// liquidación, acumulado y método de renta según la clase); las tablas
// paramétricas se comparten con el período.
func (c *CargadorParametros) ParametrosEmpleado(ctx context.Context, empleadoID int64, clase ClaseTipoPlanilla, fecha time.Time, base ParametrosCalculo) (ParametrosCalculo, error) {
	ce := &CargadorParametros{fuente: c.fuente, generales: c.paraEmpleado(empleadoID)}
	p := base
	if err := ce.cargarGenerales(ctx, fecha, &p); err != nil {
		return ParametrosCalculo{}, err
	}
	switch clase {
	case ClaseAguinaldo, ClaseSalarioEscolar:
		if err := ce.CargarAcumulado(ctx, clase, fecha, &p); err != nil {
			return ParametrosCalculo{}, err
		}
	case ClaseLiquidacion:
		if err := ce.CargarLiquidacion(ctx, fecha, &p); err != nil {
			return ParametrosCalculo{}, err
		}
	default:
		if p.FraccionMensualPeriodo.IsPositive() {
			p.DiasPeriodoCompleto = p.DivisorDiasMes.Mul(p.FraccionMensualPeriodo)
			if p.FraccionMensualPeriodo.LessThan(decimal.NewFromInt(1)) {
				if err := ce.CargarMetodoRenta(ctx, fecha, &p); err != nil {
					return ParametrosCalculo{}, err
				}
			}
		}
	}
	return p, nil
}
