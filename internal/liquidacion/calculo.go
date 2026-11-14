// Package liquidacion contiene el cálculo puro de la liquidación laboral, sin
// I/O. Proyectar y procesar usan la misma función, de modo que lo proyectado
// es lo que luego se persiste.
package liquidacion

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/planilla"
)

// Rubro identifica un componente pagable de la liquidación. Son las claves de
// expresion_formula {"tipo":"liquidacion","rubro":...} de los conceptos LIQ_*.
type Rubro string

const (
	RubroPreaviso       Rubro = "preaviso"
	RubroCesantia       Rubro = "cesantia"
	RubroVacaciones     Rubro = "vacaciones"
	RubroAguinaldo      Rubro = "aguinaldo"
	RubroSalarioEscolar Rubro = "salario_escolar"
)

// Rubros devuelve todos los rubros en orden estable.
func Rubros() []Rubro {
	return []Rubro{RubroPreaviso, RubroCesantia, RubroVacaciones, RubroAguinaldo, RubroSalarioEscolar}
}

// ReglaMotivo: fila vigente de parametros_motivos_salida para el motivo.
type ReglaMotivo struct {
	PagaPreaviso       bool
	PagaCesantia       bool
	PagaVacaciones     bool
	PagaAguinaldo      bool
	PagaSalarioEscolar bool
}

// EscalonCesantia: fila de parametros_cesantia_escala del régimen a la fecha.
type EscalonCesantia struct {
	AniosDesde  decimal.Decimal
	AniosHasta  *decimal.Decimal
	DiasPorAnio decimal.Decimal
	TopeAnios   *decimal.Decimal
}

// EscalonPreaviso: fila de parametros_preaviso_escala; cubre antigüedades M
// (meses completos) con MesesDesde <= M < MesesHasta (nil = sin tope).
type EscalonPreaviso struct {
	MesesDesde   int
	MesesHasta   *int
	DiasPreaviso int
}

// BasePromedio: Σ de una base persistida (base_cesantia, base_vacaciones) de
// los períodos regulares de la ventana y Σ de su fraccion_mensual (meses que
// cubren). Promedio mensual = Total / Meses.
type BasePromedio struct {
	Total    decimal.Decimal
	Meses    decimal.Decimal
	Periodos int
	// MesesVentana: tamaño nominal de la ventana en meses (para advertir si
	// faltan períodos). Cero = no se advierte.
	MesesVentana decimal.Decimal
}

// EntradaLiquidacion: datos del empleado ya hidratados.
type EntradaLiquidacion struct {
	// FechaIngreso: empleados.fecha_ingreso (continuidad entre contratos).
	FechaIngreso time.Time
	FechaSalida  time.Time
	// AplicaResponsabilidadPatronal: decisión explícita de RR. HH./Legal. El
	// preaviso y la cesantía se pagan solo si el motivo los admite Y esta
	// bandera es verdadera.
	AplicaResponsabilidadPatronal bool

	BaseCesantia   BasePromedio // promedio de preaviso y cesantía
	BaseVacaciones BasePromedio // promedio de vacaciones

	DiasVacacionesPendientes decimal.Decimal

	BaseAguinaldo  decimal.Decimal
	AguinaldoDesde time.Time

	BaseSalarioEscolar  decimal.Decimal
	SalarioEscolarDesde time.Time
}

// ParametrosLiquidacion: parámetros vigentes a la fecha de salida.
type ParametrosLiquidacion struct {
	Motivo         ReglaMotivo
	EscalaCesantia []EscalonCesantia
	EscalaPreaviso []EscalonPreaviso
	// DivisorDiasMes: DIVISOR_DIAS_MES.
	DivisorDiasMes int
	// FraccionMesesUmbral: LIQ_FRACCION_MESES_UMBRAL (la fracción de año cuenta
	// como año completo si sus meses son estrictamente mayores).
	FraccionMesesUmbral int
	// MetodoCesantia: LIQ_CESANTIA_METODO (solo si se paga cesantía).
	MetodoCesantia MetodoCesantia
	// AguinaldoMeses: AGUINALDO_MESES (solo si el motivo paga aguinaldo).
	AguinaldoMeses decimal.Decimal
	// PorcentajeSalarioEscolar: fracción (solo si el motivo paga salario escolar).
	PorcentajeSalarioEscolar decimal.Decimal
}

// RubrosLiquidacion: resultado completo y auditable.
type RubrosLiquidacion struct {
	AntiguedadMeses int `json:"antiguedad_meses"`

	SalarioPromedioMensual decimal.Decimal `json:"salario_promedio_mensual"`
	SalarioDiarioPromedio  decimal.Decimal `json:"salario_diario_promedio"`

	SalarioPromedioVacaciones decimal.Decimal `json:"salario_promedio_vacaciones"`
	SalarioDiarioVacaciones   decimal.Decimal `json:"salario_diario_vacaciones"`

	DiasPreaviso  int             `json:"dias_preaviso"`
	MontoPreaviso decimal.Decimal `json:"monto_preaviso"`

	DiasCesantia  decimal.Decimal `json:"dias_cesantia"`
	MontoCesantia decimal.Decimal `json:"monto_cesantia"`

	DiasVacacionesPendientes      decimal.Decimal `json:"dias_vacaciones_pendientes"`
	MontoVacacionesProporcionales decimal.Decimal `json:"monto_vacaciones_proporcionales"`

	PeriodoAguinaldoDesde      time.Time       `json:"periodo_aguinaldo_desde"`
	BaseAguinaldoProporcional  decimal.Decimal `json:"base_aguinaldo_proporcional"`
	MontoAguinaldoProporcional decimal.Decimal `json:"monto_aguinaldo_proporcional"`

	PeriodoSalarioEscolarDesde      time.Time       `json:"periodo_salario_escolar_desde"`
	BaseSalarioEscolarProporcional  decimal.Decimal `json:"base_salario_escolar_proporcional"`
	MontoSalarioEscolarProporcional decimal.Decimal `json:"monto_salario_escolar_proporcional"`

	TotalLiquidacion decimal.Decimal `json:"total_liquidacion"`

	Advertencias []string `json:"advertencias"`
}

// Montos: monto de cada rubro (claves de Rubros).
func (r RubrosLiquidacion) Montos() map[Rubro]decimal.Decimal {
	return map[Rubro]decimal.Decimal{
		RubroPreaviso:       r.MontoPreaviso,
		RubroCesantia:       r.MontoCesantia,
		RubroVacaciones:     r.MontoVacacionesProporcionales,
		RubroAguinaldo:      r.MontoAguinaldoProporcional,
		RubroSalarioEscolar: r.MontoSalarioEscolarProporcional,
	}
}

// Errores tipados del cálculo.
var (
	// ErrResponsabilidadNoAdmitida: se pidió responsabilidad patronal para un
	// motivo cuya regla vigente no paga preaviso ni cesantía.
	ErrResponsabilidadNoAdmitida = errors.New("liquidacion: el motivo de salida no admite responsabilidad patronal según parametros_motivos_salida")
	// ErrSinPeriodosParaPromedio: no hay períodos regulares en la ventana.
	ErrSinPeriodosParaPromedio = errors.New("liquidacion: no hay períodos de planilla aprobados en la ventana del promedio salarial")
	// ErrFechasInvalidas: salida anterior al ingreso.
	ErrFechasInvalidas = errors.New("liquidacion: la fecha de salida es anterior a la fecha de ingreso")
)

const (
	dosDecimales    = 2
	cuatroDecimales = 4
)

// CalcularRubros calcula todos los rubros. Pura y determinista.
func CalcularRubros(in EntradaLiquidacion, p ParametrosLiquidacion) (RubrosLiquidacion, error) {
	if in.FechaSalida.Before(in.FechaIngreso) {
		return RubrosLiquidacion{}, fmt.Errorf("%w (salida %s, ingreso %s)", ErrFechasInvalidas,
			in.FechaSalida.Format("2006-01-02"), in.FechaIngreso.Format("2006-01-02"))
	}
	pagaResponsabilidad := p.Motivo.PagaPreaviso || p.Motivo.PagaCesantia
	if in.AplicaResponsabilidadPatronal && !pagaResponsabilidad {
		return RubrosLiquidacion{}, ErrResponsabilidadNoAdmitida
	}

	antiguedad := MesesCompletosEntre(in.FechaIngreso, in.FechaSalida)
	r := RubrosLiquidacion{
		AntiguedadMeses:                 antiguedad,
		MontoPreaviso:                   decimal.Zero,
		DiasCesantia:                    decimal.Zero,
		MontoCesantia:                   decimal.Zero,
		DiasVacacionesPendientes:        decimal.Zero,
		MontoVacacionesProporcionales:   decimal.Zero,
		PeriodoAguinaldoDesde:           in.AguinaldoDesde,
		BaseAguinaldoProporcional:       decimal.Zero,
		MontoAguinaldoProporcional:      decimal.Zero,
		PeriodoSalarioEscolarDesde:      in.SalarioEscolarDesde,
		BaseSalarioEscolarProporcional:  decimal.Zero,
		MontoSalarioEscolarProporcional: decimal.Zero,
		SalarioPromedioMensual:          decimal.Zero,
		SalarioDiarioPromedio:           decimal.Zero,
		SalarioPromedioVacaciones:       decimal.Zero,
		SalarioDiarioVacaciones:         decimal.Zero,
		Advertencias:                    []string{},
	}

	pagaPreaviso := p.Motivo.PagaPreaviso && in.AplicaResponsabilidadPatronal
	pagaCesantia := p.Motivo.PagaCesantia && in.AplicaResponsabilidadPatronal
	if pagaResponsabilidad && !in.AplicaResponsabilidadPatronal {
		r.Advertencias = append(r.Advertencias,
			"el motivo admite preaviso/cesantía pero aplica_responsabilidad_patronal=false: no se calcularon")
	}

	if pagaPreaviso || pagaCesantia {
		mensual, diario, err := promedio(in.BaseCesantia, p.DivisorDiasMes, "preaviso y cesantía", &r.Advertencias)
		if err != nil {
			return RubrosLiquidacion{}, err
		}
		r.SalarioPromedioMensual, r.SalarioDiarioPromedio = mensual, diario
	}
	if pagaPreaviso {
		dias, err := CalcularDiasPreaviso(antiguedad, p.EscalaPreaviso)
		if err != nil {
			return RubrosLiquidacion{}, err
		}
		r.DiasPreaviso = dias
		r.MontoPreaviso = r.SalarioDiarioPromedio.Mul(decimal.NewFromInt(int64(dias))).Round(dosDecimales)
	}
	if pagaCesantia {
		dias, err := CalcularDiasCesantia(antiguedad, p.EscalaCesantia, p.FraccionMesesUmbral, p.MetodoCesantia)
		if err != nil {
			return RubrosLiquidacion{}, err
		}
		r.DiasCesantia = dias
		r.MontoCesantia = r.SalarioDiarioPromedio.Mul(dias).Round(dosDecimales)
	}

	if p.Motivo.PagaVacaciones {
		if in.DiasVacacionesPendientes.IsNegative() {
			return RubrosLiquidacion{}, fmt.Errorf("liquidacion: saldo de vacaciones negativo (%s)", in.DiasVacacionesPendientes)
		}
		r.DiasVacacionesPendientes = in.DiasVacacionesPendientes
		if in.DiasVacacionesPendientes.IsPositive() {
			mensual, diario, err := promedio(in.BaseVacaciones, p.DivisorDiasMes, "vacaciones", &r.Advertencias)
			if err != nil {
				return RubrosLiquidacion{}, err
			}
			r.SalarioPromedioVacaciones, r.SalarioDiarioVacaciones = mensual, diario
			r.MontoVacacionesProporcionales = diario.Mul(in.DiasVacacionesPendientes).Round(dosDecimales)
		}
	}

	if p.Motivo.PagaAguinaldo {
		m, err := planilla.CalcularAguinaldo(in.BaseAguinaldo, p.AguinaldoMeses)
		if err != nil {
			return RubrosLiquidacion{}, fmt.Errorf("liquidacion: aguinaldo proporcional: %w", err)
		}
		r.BaseAguinaldoProporcional = in.BaseAguinaldo
		r.MontoAguinaldoProporcional = m
	}

	if p.Motivo.PagaSalarioEscolar {
		m, err := planilla.CalcularSalarioEscolar(in.BaseSalarioEscolar, p.PorcentajeSalarioEscolar)
		if err != nil {
			return RubrosLiquidacion{}, fmt.Errorf("liquidacion: salario escolar proporcional: %w", err)
		}
		r.BaseSalarioEscolarProporcional = in.BaseSalarioEscolar
		r.MontoSalarioEscolarProporcional = m
	}

	total := decimal.Zero
	for _, m := range r.Montos() {
		total = total.Add(m)
	}
	r.TotalLiquidacion = total
	if total.IsNegative() {
		return RubrosLiquidacion{}, fmt.Errorf("liquidacion: total negativo (%s), revisar entradas", total)
	}
	return r, nil
}

// promedio: (mensual = Total / Meses, diario = mensual / divisor).
func promedio(b BasePromedio, divisor int, uso string, adv *[]string) (decimal.Decimal, decimal.Decimal, error) {
	if divisor <= 0 {
		return decimal.Zero, decimal.Zero, fmt.Errorf("liquidacion: DIVISOR_DIAS_MES inválido (%d)", divisor)
	}
	if b.Periodos == 0 || !b.Meses.IsPositive() {
		return decimal.Zero, decimal.Zero, fmt.Errorf("%w (%s)", ErrSinPeriodosParaPromedio, uso)
	}
	if b.Total.IsNegative() {
		return decimal.Zero, decimal.Zero, fmt.Errorf("liquidacion: base del promedio de %s negativa (%s)", uso, b.Total)
	}
	if b.MesesVentana.IsPositive() && b.Meses.LessThan(b.MesesVentana) {
		*adv = append(*adv, fmt.Sprintf(
			"promedio de %s calculado con %s mes(es) de planilla, la ventana es de %s: revisar antes de procesar",
			uso, b.Meses.String(), b.MesesVentana.String()))
	}
	mensual := b.Total.DivRound(b.Meses, dosDecimales)
	diario := mensual.DivRound(decimal.NewFromInt(int64(divisor)), cuatroDecimales)
	return mensual, diario, nil
}

// MesesCompletosEntre: antigüedad en meses completos entre ingreso y corte.
// Aritmética de año/mes/día, sin librerías externas.
func MesesCompletosEntre(ingreso, corte time.Time) int {
	if corte.Before(ingreso) {
		return 0
	}
	anios := corte.Year() - ingreso.Year()
	meses := int(corte.Month()) - int(ingreso.Month())
	if corte.Day() < ingreso.Day() {
		meses--
	}
	if meses < 0 {
		anios--
		meses += 12
	}
	return anios*12 + meses
}

// CalcularDiasPreaviso (Art. 28 CT) desde la escala; sin tramo o con tramos
// solapados → error.
func CalcularDiasPreaviso(antiguedadMeses int, escala []EscalonPreaviso) (int, error) {
	encontrado, dias := false, 0
	for _, e := range escala {
		if antiguedadMeses < e.MesesDesde || (e.MesesHasta != nil && antiguedadMeses >= *e.MesesHasta) {
			continue
		}
		if encontrado {
			return 0, fmt.Errorf("liquidacion: la escala de preaviso tiene más de un escalón para %d meses (solapados)", antiguedadMeses)
		}
		encontrado, dias = true, e.DiasPreaviso
	}
	if !encontrado {
		return 0, fmt.Errorf("liquidacion: la escala de preaviso no tiene escalón para %d meses de antigüedad", antiguedadMeses)
	}
	return dias, nil
}

// MetodoCesantia: interpretación del Art. 29 CT para antigüedades de un año o
// más. Ambas se sostienen en la práctica, por eso la elige cada empleador.
type MetodoCesantia string

const (
	// MetodoCesantiaAcumulado: Σ de los días del escalón de cada año laborado
	// (año 1, año 2, …) hasta el tope.
	MetodoCesantiaAcumulado MetodoCesantia = "acumulado_por_anio"
	// MetodoCesantiaAntiguedadTotal: los días del escalón que corresponde a la
	// antigüedad total, multiplicados por los años laborados hasta el tope.
	MetodoCesantiaAntiguedadTotal MetodoCesantia = "tasa_antiguedad_total"
)

// ClaveMetodoCesantia: parametros_generales (texto).
const ClaveMetodoCesantia = "LIQ_CESANTIA_METODO"

// MetodoCesantiaValido indica si el valor es uno de los métodos admitidos.
func MetodoCesantiaValido(m string) bool {
	return MetodoCesantia(m) == MetodoCesantiaAcumulado || MetodoCesantia(m) == MetodoCesantiaAntiguedadTotal
}

// CalcularDiasCesantia (Art. 29 CT) desde la escala:
//   - < 12 meses: el tramo de menos de un año que cubre meses/12 (total de días).
//   - >= 12 meses: años laborados (la fracción cuenta como año si sus meses
//     superan umbralMeses), con el tope de años de la escala, valorados según
//     el método.
func CalcularDiasCesantia(antiguedadMeses int, escala []EscalonCesantia, umbralMeses int, metodo MetodoCesantia) (decimal.Decimal, error) {
	if antiguedadMeses < 0 {
		return decimal.Zero, fmt.Errorf("liquidacion: antigüedad negativa (%d)", antiguedadMeses)
	}
	if umbralMeses < 0 || umbralMeses >= 12 {
		return decimal.Zero, fmt.Errorf("liquidacion: LIQ_FRACCION_MESES_UMBRAL inválido (%d)", umbralMeses)
	}
	if antiguedadMeses < 12 {
		return diasTramoMenorAnio(antiguedadMeses, escala)
	}
	if !MetodoCesantiaValido(string(metodo)) {
		return decimal.Zero, fmt.Errorf("liquidacion: %s = %q inválido (%s | %s)", ClaveMetodoCesantia, metodo, MetodoCesantiaAcumulado, MetodoCesantiaAntiguedadTotal)
	}
	completos, fraccion := antiguedadMeses/12, antiguedadMeses%12
	tope, err := topeAnios(escala)
	if err != nil {
		return decimal.Zero, err
	}
	if metodo == MetodoCesantiaAntiguedadTotal {
		anios := completos
		if fraccion > umbralMeses {
			anios++
		}
		tasa, err := diasPorAnio(anios, escala)
		if err != nil {
			return decimal.Zero, err
		}
		if tope != nil && anios > *tope {
			anios = *tope
		}
		return tasa.Mul(decimal.NewFromInt(int64(anios))), nil
	}
	aPagar := completos
	if tope != nil && aPagar > *tope {
		aPagar = *tope
	}
	total := decimal.Zero
	for anio := 1; anio <= aPagar; anio++ {
		d, err := diasPorAnio(anio, escala)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(d)
	}
	if fraccion > umbralMeses && (tope == nil || completos < *tope) {
		d, err := diasPorAnio(completos+1, escala)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(d)
	}
	return total, nil
}

var unAnio = decimal.NewFromInt(1)

func esTramoMenorAnio(e EscalonCesantia) bool {
	return e.AniosHasta != nil && e.AniosHasta.LessThanOrEqual(unAnio)
}

func diasTramoMenorAnio(meses int, escala []EscalonCesantia) (decimal.Decimal, error) {
	x := decimal.NewFromInt(int64(meses)).Div(decimal.NewFromInt(12))
	encontrado, dias := false, decimal.Zero
	for _, e := range escala {
		if !esTramoMenorAnio(e) || x.LessThan(e.AniosDesde) || x.GreaterThanOrEqual(*e.AniosHasta) {
			continue
		}
		if encontrado {
			return decimal.Zero, fmt.Errorf("liquidacion: la escala de cesantía tiene más de un tramo para %d meses (solapados)", meses)
		}
		encontrado, dias = true, e.DiasPorAnio
	}
	if !encontrado {
		return decimal.Zero, fmt.Errorf("liquidacion: la escala de cesantía no tiene tramo para %d meses de antigüedad", meses)
	}
	return dias, nil
}

func diasPorAnio(anio int, escala []EscalonCesantia) (decimal.Decimal, error) {
	n := decimal.NewFromInt(int64(anio))
	encontrado, dias := false, decimal.Zero
	for _, e := range escala {
		if esTramoMenorAnio(e) || n.LessThan(e.AniosDesde) || (e.AniosHasta != nil && n.GreaterThanOrEqual(*e.AniosHasta)) {
			continue
		}
		if encontrado {
			return decimal.Zero, fmt.Errorf("liquidacion: la escala de cesantía tiene más de un escalón para el año %d (solapados)", anio)
		}
		encontrado, dias = true, e.DiasPorAnio
	}
	if !encontrado {
		return decimal.Zero, fmt.Errorf("liquidacion: la escala de cesantía no tiene escalón para el año de servicio %d", anio)
	}
	return dias, nil
}

// topeAnios: tope_anios común a la escala (todos los que lo declaran deben
// coincidir); nil = sin tope.
func topeAnios(escala []EscalonCesantia) (*int, error) {
	var tope *decimal.Decimal
	for _, e := range escala {
		if e.TopeAnios == nil {
			continue
		}
		if tope != nil && !tope.Equal(*e.TopeAnios) {
			return nil, fmt.Errorf("liquidacion: la escala de cesantía tiene tope_anios distintos (%s y %s)", tope, e.TopeAnios)
		}
		t := *e.TopeAnios
		tope = &t
	}
	if tope == nil {
		return nil, nil
	}
	n := int(tope.Floor().IntPart())
	return &n, nil
}
