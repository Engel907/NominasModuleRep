// Hidratador: arma el CalculoEmpleadoInput de cada empleado de un período a
// partir de la BD. Solo lecturas; no calcula ni persiste. El orquestador (L8)
// lo invoca dentro de la transacción del cálculo.
package planilla

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/shared/porcentaje"
)

// Códigos del catálogo tipos_credito_fiscal_renta (seed de 000000) que se
// derivan de empleado_datos_familiares.
const (
	codigoCreditoConyuge = "CONYUGE"
	codigoCreditoHijo    = "HIJO"
)

// PeriodoParaCalculo: datos del período que necesita la hidratación.
// DiasPeriodoCompleto = DivisorDiasMes × FraccionMensual (lo arma L8 con los
// parámetros ya cargados; el hidratador no conoce el divisor).
type PeriodoParaCalculo struct {
	ID                  int64
	Inicio, Fin         time.Time
	Clase               string
	TipoPlanillaID      int64
	FraccionMensual     decimal.Decimal
	DiasPeriodoCompleto decimal.Decimal
}

// EntradaEmpleado: input del motor más los datos que el escritor de detalle
// necesita y que el motor no usa.
type EntradaEmpleado struct {
	Input                    CalculoEmpleadoInput
	ContratoID               int64
	ProgramaPresupuestarioID int64
}

// FuenteHidratacion: subconjunto de dto.Queries que usa el hidratador.
type FuenteHidratacion interface {
	ListarEmpleadosActivosEnRango(ctx context.Context, desde, hasta time.Time) ([]int64, error)
	ObtenerContratoVigenteEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (dto.ContratoVigenteEnFechaRow, error)
	ObtenerContextoLaboral(ctx context.Context, empleadoID int64, fecha time.Time) (dto.ContextoLaboralRow, error)
	SumarHorasExtraPeriodo(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]dto.HorasExtraPeriodoRow, error)
	ListarIncapacidadesSolapadas(ctx context.Context, empleadoID int64, desde, hasta time.Time) ([]dto.IncapacidadSolapadaRow, error)
	ListarPrestamosActivosEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]dto.PrestamoActivoRow, error)
	ListarEmbargosActivosEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) ([]dto.EmbargoActivoRow, error)
	ObtenerDatosFamiliaresEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (dto.DatosFamiliaresEnFechaRow, error)
	ListarCreditosFiscalesEnFecha(ctx context.Context, fecha time.Time) ([]dto.CreditoFiscalVigenteRow, error)
	ListarDevengosManualesEmpleado(ctx context.Context, periodoID, empleadoID int64) ([]dto.DevengoManualEmpleadoRow, error)
}

var _ FuenteHidratacion = (*dto.Queries)(nil)

// ErrEmpleadoSinContrato: el empleado entra en el período pero no tiene
// contrato vigente a la fecha efectiva (no se asume régimen ni salario).
type ErrEmpleadoSinContrato struct{ EmpleadoID int64 }

func (e ErrEmpleadoSinContrato) Error() string {
	return fmt.Sprintf("planilla: empleado_id=%d sin contrato vigente en el período", e.EmpleadoID)
}

// ErrEmpleadoSinDatosLaborales: no hay versión de empleado_datos_laborales
// vigente a la fecha efectiva (sin programa presupuestario no se puede
// imputar el gasto).
type ErrEmpleadoSinDatosLaborales struct{ EmpleadoID int64 }

func (e ErrEmpleadoSinDatosLaborales) Error() string {
	return fmt.Sprintf("planilla: empleado_id=%d sin datos laborales vigentes en el período", e.EmpleadoID)
}

type Hidratador struct{ f FuenteHidratacion }

func NewHidratador(f FuenteHidratacion) *Hidratador { return &Hidratador{f: f} }

// EmpleadosDelPeriodo: empleados activos con algún contrato que se solape con
// el período, ordenados por id.
func (h *Hidratador) EmpleadosDelPeriodo(ctx context.Context, p PeriodoParaCalculo) ([]int64, error) {
	ids, err := h.f.ListarEmpleadosActivosEnRango(ctx, p.Inicio, p.Fin)
	if err != nil {
		return nil, fmt.Errorf("planilla: error listando empleados del período %d: %w", p.ID, err)
	}
	return ids, nil
}

// Hidratar arma la entrada de un empleado. La fecha efectiva es el fin del
// período (contrato, datos laborales, préstamos, embargos y familia se
// resuelven a esa fecha).
func (h *Hidratador) Hidratar(ctx context.Context, p PeriodoParaCalculo, empleadoID int64) (EntradaEmpleado, error) {
	if !p.DiasPeriodoCompleto.IsPositive() {
		return EntradaEmpleado{}, errors.New("planilla: DiasPeriodoCompleto del período no definido para hidratar")
	}
	fecha := p.Fin

	// TODO L10: un cese o ingreso a mitad de período deja al empleado sin
	// contrato a fecha_fin o con días parciales; L10 segmenta por vigencias.
	contrato, err := h.f.ObtenerContratoVigenteEnFecha(ctx, empleadoID, fecha)
	if errors.Is(err, sql.ErrNoRows) {
		return EntradaEmpleado{}, ErrEmpleadoSinContrato{EmpleadoID: empleadoID}
	}
	if err != nil {
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d, contrato: %w", empleadoID, err)
	}

	lab, err := h.f.ObtenerContextoLaboral(ctx, empleadoID, fecha)
	if errors.Is(err, sql.ErrNoRows) {
		return EntradaEmpleado{}, ErrEmpleadoSinDatosLaborales{EmpleadoID: empleadoID}
	}
	if err != nil {
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d, datos laborales: %w", empleadoID, err)
	}

	in := CalculoEmpleadoInput{
		EmpleadoID:         empleadoID,
		PeriodoID:          p.ID,
		FechaEfectiva:      fecha,
		FechaInicioPeriodo: p.Inicio,
		FechaFinPeriodo:    p.Fin,
		// SalarioBaseNominal NO se llena (D5: la fuente es contratos.salario_pactado).
		// TODO L10: días comerciales reales según vigencias; provisional = período completo.
		DiasTrabajados: p.DiasPeriodoCompleto,
	}
	if lab.ClasificacionPuestoID != nil {
		in.ClasificacionPuestoID = *lab.ClasificacionPuestoID
	}
	if lab.RegimenCCSSID != nil {
		in.RegimenCCSSID = *lab.RegimenCCSSID // L15 crea la columna; hasta entonces queda en 0
	}

	if err := h.hidratarAsistencia(ctx, p, empleadoID, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarDeducciones(ctx, empleadoID, fecha, &in); err != nil {
		return EntradaEmpleado{}, err
	}
	if err := h.hidratarCreditos(ctx, empleadoID, fecha, &in); err != nil {
		return EntradaEmpleado{}, err
	}

	devs, err := h.f.ListarDevengosManualesEmpleado(ctx, p.ID, empleadoID)
	if err != nil {
		return EntradaEmpleado{}, fmt.Errorf("planilla: empleado_id=%d, devengos manuales: %w", empleadoID, err)
	}
	for _, d := range devs {
		dev := ConceptoDevengoManualInput{ConceptoSalarialID: d.ConceptoSalarialID, Monto: d.Monto}
		if d.Observacion != nil {
			dev.Descripcion = *d.Observacion
		}
		in.DevengosManuales = append(in.DevengosManuales, dev)
	}

	return EntradaEmpleado{
		Input:                    in,
		ContratoID:               contrato.ContratoID,
		ProgramaPresupuestarioID: lab.ProgramaPresupuestarioID,
	}, nil
}

func (h *Hidratador) hidratarAsistencia(ctx context.Context, p PeriodoParaCalculo, empleadoID int64, in *CalculoEmpleadoInput) error {
	horas, err := h.f.SumarHorasExtraPeriodo(ctx, empleadoID, p.Inicio, p.Fin)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, horas extra: %w", empleadoID, err)
	}
	for _, x := range horas {
		in.HorasExtra = append(in.HorasExtra, HoraExtraInput{TipoHoraExtraID: x.TipoHoraExtraID, CantidadHoras: x.Horas})
	}

	incs, err := h.f.ListarIncapacidadesSolapadas(ctx, empleadoID, p.Inicio, p.Fin)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, incapacidades: %w", empleadoID, err)
	}
	for _, x := range incs {
		// SUPUESTO A VALIDAR (L11): días naturales de la boleta dentro del
		// período; la numeración es continua desde el inicio de la boleta,
		// aunque haya empezado en un período anterior.
		desde, hasta := maxFecha(x.FechaInicio, p.Inicio), minFecha(x.FechaFin, p.Fin)
		in.Incapacidades = append(in.Incapacidades, IncapacidadInput{
			EntidadEmisoraID: x.EntidadEmisoraID,
			DiaDesde:         int32(diasEntre(x.FechaInicio, desde)) + 1,
			DiasIncapacidad:  decimal.NewFromInt(diasEntre(desde, hasta) + 1),
		})
	}
	return nil
}

func (h *Hidratador) hidratarDeducciones(ctx context.Context, empleadoID int64, fecha time.Time, in *CalculoEmpleadoInput) error {
	prestamos, err := h.f.ListarPrestamosActivosEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, préstamos: %w", empleadoID, err)
	}
	for _, x := range prestamos {
		in.Prestamos = append(in.Prestamos, PrestamoDeduccionInput{
			PrestamoID: x.PrestamoID, CuotaPeriodica: x.CuotaPeriodica, SaldoPendiente: x.SaldoPendiente,
		})
	}

	embargos, err := h.f.ListarEmbargosActivosEnFecha(ctx, empleadoID, fecha)
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, embargos: %w", empleadoID, err)
	}
	for _, x := range embargos {
		e := EmbargoInput{EmbargoID: x.EmbargoID, TipoEmbargoID: x.TipoEmbargoID, Prioridad: x.PrioridadLegal}
		if x.Porcentaje.Valid {
			pct, err := porcentaje.DesdePorcentaje(x.Porcentaje.Decimal)
			if err != nil {
				return fmt.Errorf("planilla: embargo_id=%d: %w", x.EmbargoID, err)
			}
			e.Porcentaje = pct.Fraccion()
		}
		if x.MontoFijo.Valid {
			e.MontoFijo = x.MontoFijo.Decimal
		}
		if e.Porcentaje.IsZero() && e.MontoFijo.IsZero() {
			return fmt.Errorf("planilla: embargo_id=%d sin porcentaje ni monto fijo", x.EmbargoID)
		}
		in.Embargos = append(in.Embargos, e)
	}
	return nil
}

// hidratarCreditos traduce la declaración familiar vigente a créditos
// fiscales por tipo. Sin declaración vigente = sin créditos (no es error).
func (h *Hidratador) hidratarCreditos(ctx context.Context, empleadoID int64, fecha time.Time, in *CalculoEmpleadoInput) error {
	fam, err := h.f.ObtenerDatosFamiliaresEnFecha(ctx, empleadoID, fecha)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("planilla: empleado_id=%d, datos familiares: %w", empleadoID, err)
	}
	if !fam.AplicaCreditoConyuge && fam.NumeroHijosDependientes <= 0 {
		return nil
	}

	tipos, err := h.f.ListarCreditosFiscalesEnFecha(ctx, fecha)
	if err != nil {
		return fmt.Errorf("planilla: error leyendo créditos fiscales vigentes: %w", err)
	}
	idPorCodigo := make(map[string]int64, len(tipos))
	for _, t := range tipos {
		idPorCodigo[t.TipoCodigo] = t.TipoCreditoFiscalID
	}
	agregar := func(codigo string, cantidad int32) error {
		id, ok := idPorCodigo[codigo]
		if !ok {
			return fmt.Errorf("planilla: empleado_id=%d declara crédito %s pero no hay parámetro de crédito fiscal vigente al %s",
				empleadoID, codigo, fecha.Format("2006-01-02"))
		}
		in.CreditosFiscales = append(in.CreditosFiscales, CreditoFiscalInput{TipoCreditoFiscalID: id, Cantidad: cantidad})
		return nil
	}
	if fam.AplicaCreditoConyuge {
		if err := agregar(codigoCreditoConyuge, 1); err != nil {
			return err
		}
	}
	if fam.NumeroHijosDependientes > 0 {
		if err := agregar(codigoCreditoHijo, fam.NumeroHijosDependientes); err != nil {
			return err
		}
	}
	return nil
}

// diasEntre: días naturales de a a b (b - a), comparando solo la fecha.
func diasEntre(a, b time.Time) int64 {
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int64(db.Sub(da).Hours() / 24)
}

func maxFecha(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minFecha(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
