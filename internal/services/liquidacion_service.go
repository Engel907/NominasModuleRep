// Package services — Fase 7: Acumulados (Aguinaldo, Salario Escolar) y
// Liquidaciones Laborales por cesación de contrato.
//
// Referencias legales (Costa Rica, Código de Trabajo):
//   - Aguinaldo: Ley de Aguinaldo (Ley N.° 2412 y reformas). Suma de
//     salarios brutos devengados del 1-dic del año anterior al 30-nov del
//     año actual, entre 12.
//   - Preaviso: Art. 28 Código de Trabajo.
//   - Auxilio de cesantía: Art. 29 Código de Trabajo, tabla reformada por
//     la Ley de Protección al Trabajador (Ley N.° 7983, vigente desde el
//     1-mar-2001). Tabla verificada contra el pronunciamiento oficial
//     DAJ-AE-083-09 del MTSS (17-abr-2009).
//
// Principio de diseño (igual que internal/planilla/service.go): separar la
// ORQUESTACIÓN (acceso a BD, transacciones, auditoría — funciones
// Proyectar*/Procesar*) del CÁLCULO PURO (funciones Calcular*, que solo
// operan sobre decimal.Decimal/time.Time ya en memoria y no hacen I/O). Los
// Calcular* son lo que se testea exhaustivamente con tablas de casos.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
)

// ============================================================================
// Constantes y tablas legales
// ============================================================================

// DivisorDiasMes es el divisor usado para obtener un "día de salario" a
// partir de un salario mensual (salario_diario = salario_mensual / DivisorDiasMes).
//
// Se mantiene como constante Go y NO como parámetro de BD porque db/schema.sql
// no tiene ninguna tabla ni columna para este valor (revisado: las únicas
// parametros_* existentes son cargas sociales, renta, créditos fiscales,
// tipos de embargo, horas extra, vacaciones, cesantía, preaviso, base mínima
// contributiva, subsidios de incapacidad y salario mínimo), y no varía por
// régimen laboral ni por vigencia.
//
// Fundamento: NO es un divisor fijado en un artículo del Código de Trabajo.
// Es el criterio administrativo del MTSS (Dirección de Asuntos Jurídicos)
// para trabajadores con pago mensual, quincenal o semanal en actividad
// comercial: salario promedio mensual / 30 (criterio citado como
// DAJ-AER-OFP-363-2018 en fuentes secundarias; VERIFICAR el número exacto
// del dictamen contra el texto oficial del MTSS antes de citarlo en
// documentos legales). Para pago semanal en actividad no comercial el MTSS
// usa 26, caso que este motor no modela. El factor 30.42 (365/12) que
// usan algunas calculadoras no es el criterio del MTSS.
const DivisorDiasMes = 30

// EscalonCesantia es un escalón de parametros_cesantia_escala ya resuelto
// para un régimen laboral y una fecha (vigencia). Reemplaza al antiguo mapa
// tablaCesantiaArt29 (valores en Go): la escala ahora SIEMPRE llega como
// parámetro, resuelta desde SQL por regimen_laboral_id + fecha.
//
// El escalón cubre los años enteros N tales que
// AniosDesde <= N < AniosHasta (AniosHasta == nil: sin tope superior).
//
// TopeAnios es parametros_cesantia_escala.tope_anios (nil = sin tope): el
// máximo de años de servicio reconocidos para cesantía. Ver
// topeAniosCesantia.
type EscalonCesantia struct {
	AniosDesde  decimal.Decimal
	AniosHasta  *decimal.Decimal
	DiasPorAnio decimal.Decimal
	TopeAnios   *decimal.Decimal
}

// EscalonPreaviso es un escalón de parametros_preaviso_escala resuelto para
// un régimen y una fecha. Cubre antigüedades M (en meses completos) con
// MesesDesde <= M < MesesHasta (MesesHasta == nil: último tramo, sin tope).
type EscalonPreaviso struct {
	MesesDesde   int
	MesesHasta   *int
	DiasPreaviso int
}

// topeAniosCesantia deriva el tope de años reconocidos desde
// tope_anios de la escala vigente. Todos los escalones que declaren tope
// deben coincidir (es un atributo de la escala completa, no de un tramo);
// si difieren se retorna error en vez de elegir uno. nil = la escala no
// define tope.
func topeAniosCesantia(escala []EscalonCesantia) (*int, error) {
	var tope *decimal.Decimal
	for _, e := range escala {
		if e.TopeAnios == nil {
			continue
		}
		if tope != nil && !tope.Equal(*e.TopeAnios) {
			return nil, fmt.Errorf("liquidacion: la escala de cesantía tiene valores distintos de tope_anios (%s y %s)", tope.String(), e.TopeAnios.String())
		}
		t := *e.TopeAnios
		tope = &t
	}
	if tope == nil {
		return nil, nil
	}
	anios := int(tope.Floor().IntPart())
	return &anios, nil
}

// diasPorAnioCesantia devuelve los días de salario que corresponden al año
// de servicio `anio` según la escala recibida. Sin defaults implícitos: si
// ningún escalón cubre el año, o lo cubre más de uno (vigencias/escalones
// solapados), retorna error en vez de asumir un valor.
func diasPorAnioCesantia(anio int, escala []EscalonCesantia) (decimal.Decimal, error) {
	if anio < 1 {
		return decimal.Zero, nil
	}
	n := decimal.NewFromInt(int64(anio))
	encontrado := false
	dias := decimal.Zero
	for _, e := range escala {
		if n.LessThan(e.AniosDesde) {
			continue
		}
		if e.AniosHasta != nil && n.GreaterThanOrEqual(*e.AniosHasta) {
			continue
		}
		if encontrado {
			return decimal.Zero, fmt.Errorf("liquidacion: la escala de cesantía tiene más de un escalón para el año de servicio %d (escalones solapados)", anio)
		}
		encontrado = true
		dias = e.DiasPorAnio
	}
	if !encontrado {
		return decimal.Zero, fmt.Errorf("liquidacion: la escala de cesantía no tiene escalón para el año de servicio %d (¿falta parametros_cesantia_escala vigente para el régimen?)", anio)
	}
	return dias, nil
}

// ============================================================================
// Helpers de fecha (puros)
// ============================================================================

// mesesCompletosEntre calcula la antigüedad en meses completos entre
// fechaIngreso y fechaCorte (ambas inclusive del día), más los días
// sobrantes del mes en curso. No usa librerías de "diferencia de fecha
// calendario" externas para mantener el cálculo 100% determinista y
// auditable: es aritmética simple de año/mes/día.
func mesesCompletosEntre(fechaIngreso, fechaCorte time.Time) (mesesTotales int, diasSobrantes int) {
	if fechaCorte.Before(fechaIngreso) {
		return 0, 0
	}
	anios := fechaCorte.Year() - fechaIngreso.Year()
	meses := int(fechaCorte.Month()) - int(fechaIngreso.Month())
	dias := fechaCorte.Day() - fechaIngreso.Day()

	if dias < 0 {
		meses--
		// días del mes anterior a fechaCorte
		prevMonthLastDay := time.Date(fechaCorte.Year(), fechaCorte.Month(), 0, 0, 0, 0, 0, fechaCorte.Location()).Day()
		dias += prevMonthLastDay
	}
	if meses < 0 {
		anios--
		meses += 12
	}

	mesesTotales = anios*12 + meses
	return mesesTotales, dias
}

// ============================================================================
// CÁLCULO PURO: Aguinaldo
// ============================================================================

// PeriodoAguinaldo resuelve el rango legal del aguinaldo (1-dic del año
// anterior a fechaReferencia hasta 30-nov de fechaReferencia), o hasta
// fechaCorte si esta cae dentro del rango (aguinaldo proporcional por
// cesación, o cálculo antes de que cierre el periodo).
func PeriodoAguinaldo(anio int, fechaCorte *time.Time) (desde, hasta time.Time) {
	desde = time.Date(anio-1, time.December, 1, 0, 0, 0, 0, time.UTC)
	hasta = time.Date(anio, time.November, 30, 0, 0, 0, 0, time.UTC)
	if fechaCorte != nil && fechaCorte.Before(hasta) {
		hasta = *fechaCorte
	}
	return desde, hasta
}

// CalcularAguinaldo implementa la Ley de Aguinaldo: suma de salarios
// brutos devengados en el periodo / 12. totalSalariosBrutos debe venir ya
// agregado por el repositorio (SumarSalariosBrutosPeriodo) — esta función
// no hace I/O.
func CalcularAguinaldo(totalSalariosBrutos decimal.Decimal) decimal.Decimal {
	return totalSalariosBrutos.DivRound(decimal.NewFromInt(12), 2)
}

// ============================================================================
// CÁLCULO PURO: Salario Escolar
// ============================================================================

// CalcularSalarioEscolar aplica el porcentaje vigente sobre el total de
// salarios brutos del periodo anual (1-ene a 31-dic, o hasta fechaCorte si
// es proporcional).
func CalcularSalarioEscolar(totalSalariosBrutos, porcentaje decimal.Decimal) decimal.Decimal {
	return totalSalariosBrutos.Mul(porcentaje).Round(2)
}

// ============================================================================
// CÁLCULO PURO: Liquidación
// ============================================================================

// LiquidacionInput agrupa todo lo que CalcularLiquidacion necesita, ya
// hidratado desde el repositorio (ver PrepararLiquidacionInput más abajo).
type LiquidacionInput struct {
	FechaIngreso time.Time
	FechaSalida  time.Time

	MotivoSalida string

	// AplicaResponsabilidadPatronal gobierna preaviso Y cesantía. Es
	// explícito y NO se infiere automáticamente de MotivoSalida (ver nota
	// en la migración): el enunciado de negocio lo llama "pagoPreaviso".
	AplicaResponsabilidadPatronal bool

	// SalariosBrutosUltimos6Meses: montos de total_ingresos de los últimos
	// periodos calculados (más reciente primero), ya filtrados/curados por
	// el orquestador. Puede tener menos de 6 elementos si el empleado
	// tiene menos de 6 meses de planillas calculadas — el promedio se
	// calcula sobre los disponibles (no es un error: el motor no rellena
	// con ceros ni con supuestos).
	SalariosBrutosUltimos6Meses []decimal.Decimal

	DiasVacacionesPendientes decimal.Decimal

	// EscalaCesantia: escalones de parametros_cesantia_escala del régimen
	// laboral vigente del empleado en FechaSalida (resuelto en
	// prepararInput). Solo se hidrata si AplicaResponsabilidadPatronal.
	EscalaCesantia []EscalonCesantia

	// EscalaPreaviso: escalones de parametros_preaviso_escala del mismo
	// régimen y fecha. Se hidrata junto con EscalaCesantia.
	EscalaPreaviso []EscalonPreaviso

	// TotalSalariosBrutosAguinaldoProporcional: suma de salarios brutos
	// desde el último 1-dic hasta FechaSalida (para el aguinaldo
	// proporcional), ya agregada por el repositorio.
	TotalSalariosBrutosAguinaldoProporcional decimal.Decimal
	PeriodoAguinaldoDesde                    time.Time
}

// LiquidacionResultado es la salida completa y auditable del cálculo de
// liquidación. Se persiste 1:1 en la tabla `liquidaciones` únicamente
// cuando se invoca ProcesarLiquidacion (NUNCA en ProyectarLiquidacion).
type LiquidacionResultado struct {
	AntiguedadMeses int

	SalarioPromedio6Meses  decimal.Decimal
	SalarioDiarioPromedio  decimal.Decimal

	DiasPreaviso  int
	MontoPreaviso decimal.Decimal

	DiasCesantia  decimal.Decimal
	MontoCesantia decimal.Decimal

	DiasVacacionesPendientes      decimal.Decimal
	MontoVacacionesProporcionales decimal.Decimal

	PeriodoAguinaldoDesde                    time.Time
	TotalSalariosBrutosAguinaldoProporcional decimal.Decimal
	MontoAguinaldoProporcional               decimal.Decimal

	TotalLiquidacion decimal.Decimal

	// Advertencias no fatales, ej. "menos de 6 periodos calculados
	// disponibles para el promedio salarial; se usó el promedio de N
	// periodos" — visibles para revisión humana antes de procesar.
	Advertencias []string
}

// CalcularDiasPreaviso implementa el Art. 28 del Código de Trabajo a partir
// de la escala de parametros_preaviso_escala del régimen laboral vigente en
// la fecha de cálculo (ver prepararInput). Sin defaults implícitos: si
// ningún escalón cubre la antigüedad, o la cubre más de uno, retorna error.
func CalcularDiasPreaviso(antiguedadMeses int, escala []EscalonPreaviso) (int, error) {
	encontrado := false
	dias := 0
	for _, e := range escala {
		if antiguedadMeses < e.MesesDesde {
			continue
		}
		if e.MesesHasta != nil && antiguedadMeses >= *e.MesesHasta {
			continue
		}
		if encontrado {
			return 0, fmt.Errorf("liquidacion: la escala de preaviso tiene más de un escalón para %d meses de antigüedad (escalones solapados)", antiguedadMeses)
		}
		encontrado = true
		dias = e.DiasPreaviso
	}
	if !encontrado {
		return 0, fmt.Errorf("liquidacion: la escala de preaviso no tiene escalón para %d meses de antigüedad (¿falta parametros_preaviso_escala vigente para el régimen?)", antiguedadMeses)
	}
	return dias, nil
}

// CalcularDiasCesantia implementa el Art. 29 del Código de Trabajo.
//
// Reglas (verificadas contra DAJ-AE-083-09):
//   - < 3 meses continuos: sin derecho a cesantía (0 días).
//   - 3 a 6 meses: 7 días.
//   - 6 a 12 meses: 14 días.
//   - > 1 año: suma de la escala recibida por cada año completo, más un
//     año adicional si la fracción restante supera 6 meses (criterio
//     administrativo del MTSS en la misma resolución, "regla de
//     fracción").
//   - Tope legal de años reconocidos (inciso 4): parametros_cesantia_escala.tope_anios
//     (sembrado en 8 para Código de Trabajo).
//
// ASUNCIÓN A CONFIRMAR (documentada explícitamente, no silenciada): el
// texto legal dice "no podrá indemnizar... más que los últimos ocho años
// de la relación laboral". Esta función lo implementa como "los PRIMEROS 8
// años según la tabla ascendente" (interpretación más común en
// calculadoras públicas consultadas), NO como "los últimos 8 años
// cronológicos de la relación" (que para tenencias >13 años daría un
// resultado distinto, ya que la tabla es simétrica: sube hasta el año 7-9
// y luego vuelve a bajar). Para empleados con antigüedad mayor a 13 años
// las dos interpretaciones divergen; confirmar con asesoría legal antes de
// procesar liquidaciones de personal con esa antigüedad.
//
// escala son los escalones de parametros_cesantia_escala del régimen
// laboral vigente del empleado en la fecha de cálculo (ver prepararInput).
// Solo se consulta para antigüedad >= 12 meses; si en ese caso no cubre
// algún año requerido, se retorna error explícito.
func CalcularDiasCesantia(antiguedadMeses int, escala []EscalonCesantia) (decimal.Decimal, error) {
	switch {
	case antiguedadMeses < 3:
		return decimal.Zero, nil
	case antiguedadMeses < 6:
		return decimal.NewFromInt(7), nil
	case antiguedadMeses < 12:
		return decimal.NewFromInt(14), nil
	}

	aniosCompletos := antiguedadMeses / 12
	mesesFraccion := antiguedadMeses % 12

	// Tope de años reconocidos: parametros_cesantia_escala.tope_anios.
	topeAnios, err := topeAniosCesantia(escala)
	if err != nil {
		return decimal.Zero, err
	}

	aniosAPagar := aniosCompletos
	if topeAnios != nil && aniosAPagar > *topeAnios {
		aniosAPagar = *topeAnios
	}

	total := decimal.Zero
	for anio := 1; anio <= aniosAPagar; anio++ {
		dias, err := diasPorAnioCesantia(anio, escala)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(dias)
	}

	// Fracción superior a 6 meses reconocida como un año adicional
	// completo, a la tarifa del año siguiente (criterio MTSS), siempre que
	// no se haya alcanzado ya el tope de 8 años.
	if mesesFraccion > 6 && (topeAnios == nil || aniosCompletos < *topeAnios) {
		dias, err := diasPorAnioCesantia(aniosCompletos+1, escala)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(dias)
	}

	return total, nil
}

// promedioDecimal calcula el promedio simple de una lista de montos. No
// rellena con ceros los meses faltantes: si input tiene menos de 6
// elementos, promedia sobre los que hay (ver Advertencias en el resultado).
func promedioDecimal(valores []decimal.Decimal) decimal.Decimal {
	if len(valores) == 0 {
		return decimal.Zero
	}
	suma := decimal.Zero
	for _, v := range valores {
		suma = suma.Add(v)
	}
	return suma.DivRound(decimal.NewFromInt(int64(len(valores))), 2)
}

// CalcularLiquidacion es el núcleo PURO del motor de liquidaciones: no
// hace I/O, no conoce el repositorio. Recibe el input ya hidratado y
// retorna el resultado completo, listo para mostrarse (proyectar) o
// persistirse (procesar).
func CalcularLiquidacion(input LiquidacionInput) (*LiquidacionResultado, error) {
	if input.FechaSalida.Before(input.FechaIngreso) {
		return nil, fmt.Errorf("liquidacion: fecha_salida (%s) es anterior a fecha_ingreso (%s)",
			input.FechaSalida.Format("2006-01-02"), input.FechaIngreso.Format("2006-01-02"))
	}

	antiguedadMeses, _ := mesesCompletosEntre(input.FechaIngreso, input.FechaSalida)

	resultado := &LiquidacionResultado{
		AntiguedadMeses:        antiguedadMeses,
		PeriodoAguinaldoDesde:  input.PeriodoAguinaldoDesde,
	}

	// --- Salario promedio y salario diario (últimos 6 meses) ---
	if len(input.SalariosBrutosUltimos6Meses) == 0 {
		return nil, errors.New("liquidacion: no hay periodos de planilla calculados para promediar el salario; " +
			"no se asume un salario por defecto (directiva de no-defaults implícitos)")
	}
	if len(input.SalariosBrutosUltimos6Meses) < 6 {
		resultado.Advertencias = append(resultado.Advertencias, fmt.Sprintf(
			"promedio salarial calculado con %d periodo(s) calculado(s) disponible(s), no 6 — revisar antes de procesar",
			len(input.SalariosBrutosUltimos6Meses)))
	}
	resultado.SalarioPromedio6Meses = promedioDecimal(input.SalariosBrutosUltimos6Meses)
	resultado.SalarioDiarioPromedio = resultado.SalarioPromedio6Meses.DivRound(decimal.NewFromInt(DivisorDiasMes), 4)

	// --- Preaviso y Cesantía (solo si aplica responsabilidad patronal) ---
	if input.AplicaResponsabilidadPatronal {
		diasPreaviso, err := CalcularDiasPreaviso(antiguedadMeses, input.EscalaPreaviso)
		if err != nil {
			return nil, err
		}
		resultado.DiasPreaviso = diasPreaviso
		resultado.MontoPreaviso = resultado.SalarioDiarioPromedio.
			Mul(decimal.NewFromInt(int64(resultado.DiasPreaviso))).Round(2)

		diasCesantia, err := CalcularDiasCesantia(antiguedadMeses, input.EscalaCesantia)
		if err != nil {
			return nil, err
		}
		resultado.DiasCesantia = diasCesantia
		resultado.MontoCesantia = resultado.SalarioDiarioPromedio.
			Mul(resultado.DiasCesantia).Round(2)
	} else {
		resultado.DiasPreaviso = 0
		resultado.MontoPreaviso = decimal.Zero
		resultado.DiasCesantia = decimal.Zero
		resultado.MontoCesantia = decimal.Zero
		resultado.Advertencias = append(resultado.Advertencias,
			"aplica_responsabilidad_patronal=false: no se calculó preaviso ni cesantía (ver Art. 28/29 CT)")
	}

	// --- Vacaciones proporcionales (saldo materializado x salario diario) ---
	resultado.DiasVacacionesPendientes = input.DiasVacacionesPendientes
	resultado.MontoVacacionesProporcionales = resultado.SalarioDiarioPromedio.
		Mul(input.DiasVacacionesPendientes).Round(2)

	// --- Aguinaldo proporcional (desde el último 1-dic hasta fecha_salida) ---
	resultado.TotalSalariosBrutosAguinaldoProporcional = input.TotalSalariosBrutosAguinaldoProporcional
	resultado.MontoAguinaldoProporcional = CalcularAguinaldo(input.TotalSalariosBrutosAguinaldoProporcional)

	// --- Total ---
	resultado.TotalLiquidacion = resultado.MontoPreaviso.
		Add(resultado.MontoCesantia).
		Add(resultado.MontoVacacionesProporcionales).
		Add(resultado.MontoAguinaldoProporcional)

	if resultado.TotalLiquidacion.IsNegative() {
		// No debería ocurrir dado que todos los sumandos son >= 0, pero se
		// verifica explícitamente en vez de truncar a cero en silencio
		// (misma directiva que aplicarEmbargos en el motor de planilla).
		return nil, fmt.Errorf("liquidacion: total calculado es negativo (%s), revisar inputs",
			resultado.TotalLiquidacion.String())
	}

	return resultado, nil
}

// ============================================================================
// ORQUESTACIÓN: acceso a BD, hidratación de inputs, persistencia y auditoría
// ============================================================================

// ErrSinDatosLaborales se retorna si el empleado no tiene datos_laborales o
// contrato vigente (no se puede resolver fecha_ingreso).
var ErrSinDatosLaborales = errors.New("liquidacion: el empleado no tiene contrato/datos_laborales vigente")

// LiquidacionRepository es el subconjunto de dto.Queries que el servicio
// necesita, expuesto como interfaz para poder testear el orquestador con un
// fake sin BD real (mismo criterio de testabilidad que PayrollRepository
// en internal/planilla).
type LiquidacionRepository interface {
	GetContratoVigente(ctx context.Context, empleadoID int64) (dto.Contrato, error)
	SumarSalariosBrutosPeriodo(ctx context.Context, arg dto.SumarSalariosBrutosPeriodoParams) (dto.SumarSalariosBrutosPeriodoRow, error)
	ListSalariosBrutosUltimosPeriodos(ctx context.Context, arg dto.ListSalariosBrutosUltimosPeriodosParams) ([]dto.ListSalariosBrutosUltimosPeriodosRow, error)
	GetSaldoVacacionesDisponibles(ctx context.Context, empleadoID int64) (decimal.Decimal, error)
	GetPorcentajeSalarioEscolarVigente(ctx context.Context) (dto.ParametrosSalarioEscolar, error)

	// ListarEscalaCesantiaEnFecha retorna los escalones de
	// parametros_cesantia_escala del régimen indicado, vigentes a `fecha`
	// (fecha_inicio_vigencia <= fecha AND (fecha_fin_vigencia IS NULL OR
	// fecha_fin_vigencia >= fecha)), ordenados por anios_servicio_desde.
	ListarEscalaCesantiaEnFecha(ctx context.Context, regimenLaboralID int64, fecha time.Time) ([]EscalonCesantia, error)

	// ListarEscalaPreavisoEnFecha: ídem sobre parametros_preaviso_escala,
	// ordenado por antiguedad_meses_desde.
	ListarEscalaPreavisoEnFecha(ctx context.Context, regimenLaboralID int64, fecha time.Time) ([]EscalonPreaviso, error)
}

// RegimenResolver resuelve el contrato (y por tanto el régimen laboral)
// vigente de un empleado en una fecha. Es exactamente la lectura que usa
// evaluarConcepto en la fase 9A (planilla.MotorRepository.
// ObtenerContratoVigenteEnFecha), por lo que planilla.NewMotorRepository(...)
// satisface esta interfaz sin adaptadores: una sola fuente de verdad para
// "régimen vigente en una fecha".
type RegimenResolver interface {
	ObtenerContratoVigenteEnFecha(ctx context.Context, empleadoID int64, fecha time.Time) (planilla.ContratoVigente, error)
}

// LiquidacionService orquesta la hidratación, el cálculo puro y la
// persistencia de Fase 7. sqlDB se usa directamente para /procesar (que
// necesita WithAuditContext); repo se usa para lecturas fuera de
// transacción en /proyectar.
type LiquidacionService struct {
	sqlDB   *sql.DB
	repo    LiquidacionRepository
	regimen RegimenResolver
}

// NewLiquidacionService: regimen típicamente planilla.NewMotorRepository(dto.New(sqlDB)).
func NewLiquidacionService(sqlDB *sql.DB, repo LiquidacionRepository, regimen RegimenResolver) *LiquidacionService {
	return &LiquidacionService{sqlDB: sqlDB, repo: repo, regimen: regimen}
}

// ProyectarSolicitud agrupa los parámetros que el llamador (handler HTTP)
// debe proveer explícitamente. AplicaResponsabilidadPatronal es
// obligatorio y NO se infiere de MotivoSalida (ver nota en
// LiquidacionInput) — es responsabilidad de quien llama (RRHH/Legal)
// decidirlo caso a caso.
type ProyectarSolicitud struct {
	EmpleadoID                    int64
	FechaSalida                   time.Time
	MotivoSalida                  string
	AplicaResponsabilidadPatronal bool
}

// prepararInput hidrata un LiquidacionInput completo desde el repositorio.
// Compartido por Proyectar y Procesar para que ambos calculen exactamente
// igual (proyectar no debe poder divergir del resultado que luego procesar
// persiste).
func (s *LiquidacionService) prepararInput(ctx context.Context, sol ProyectarSolicitud) (*LiquidacionInput, error) {
	contrato, err := s.repo.GetContratoVigente(ctx, sol.EmpleadoID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSinDatosLaborales, err)
	}

	// Últimos 6 periodos calculados hasta (e incluyendo) fecha_salida.
	filas, err := s.repo.ListSalariosBrutosUltimosPeriodos(ctx, dto.ListSalariosBrutosUltimosPeriodosParams{
		EmpleadoID: sol.EmpleadoID,
		FechaCorte: sol.FechaSalida,
		Limit:      6,
	})
	if err != nil {
		return nil, fmt.Errorf("liquidacion: error consultando salarios últimos 6 meses: %w", err)
	}
	salarios := make([]decimal.Decimal, 0, len(filas))
	for _, f := range filas {
		salarios = append(salarios, f.TotalIngresos)
	}

	// Escala de cesantía: régimen laboral vigente en fecha_salida (misma
	// resolución que evaluarConcepto, fase 9A) -> escala de ese régimen
	// vigente a esa fecha. Solo aplica si hay responsabilidad patronal.
	var escalaCesantia []EscalonCesantia
	var escalaPreaviso []EscalonPreaviso
	if sol.AplicaResponsabilidadPatronal {
		contratoEnFecha, err := s.regimen.ObtenerContratoVigenteEnFecha(ctx, sol.EmpleadoID, sol.FechaSalida)
		if err != nil {
			return nil, fmt.Errorf("%w: empleado_id=%d sin contrato vigente al %s (no se asume régimen): %v",
				ErrSinDatosLaborales, sol.EmpleadoID, sol.FechaSalida.Format("2006-01-02"), err)
		}
		escalaCesantia, err = s.repo.ListarEscalaCesantiaEnFecha(ctx, contratoEnFecha.RegimenLaboralID, sol.FechaSalida)
		if err != nil {
			return nil, fmt.Errorf("liquidacion: error resolviendo escala de cesantía (regimen_laboral_id=%d): %w",
				contratoEnFecha.RegimenLaboralID, err)
		}
		escalaPreaviso, err = s.repo.ListarEscalaPreavisoEnFecha(ctx, contratoEnFecha.RegimenLaboralID, sol.FechaSalida)
		if err != nil {
			return nil, fmt.Errorf("liquidacion: error resolviendo escala de preaviso (regimen_laboral_id=%d): %w",
				contratoEnFecha.RegimenLaboralID, err)
		}
	}

	saldoVacaciones, err := s.repo.GetSaldoVacacionesDisponibles(ctx, sol.EmpleadoID)
	if err != nil {
		return nil, fmt.Errorf("liquidacion: error consultando saldo de vacaciones: %w", err)
	}

	// Periodo de aguinaldo proporcional: desde el último 1-dic hasta fecha_salida.
	anioAguinaldo := sol.FechaSalida.Year()
	if sol.FechaSalida.Month() == time.December {
		anioAguinaldo++ // si sale en diciembre, el "año de pago" del aguinaldo es el próximo
	}
	periodoDesde, _ := PeriodoAguinaldo(anioAguinaldo, &sol.FechaSalida)

	aguinaldoAgg, err := s.repo.SumarSalariosBrutosPeriodo(ctx, dto.SumarSalariosBrutosPeriodoParams{
		EmpleadoID:  sol.EmpleadoID,
		FechaDesde:  periodoDesde,
		FechaHasta:  sol.FechaSalida,
	})
	if err != nil {
		return nil, fmt.Errorf("liquidacion: error consultando salarios brutos para aguinaldo proporcional: %w", err)
	}

	return &LiquidacionInput{
		FechaIngreso:                              contrato.FechaInicioContrato,
		FechaSalida:                                sol.FechaSalida,
		MotivoSalida:                                sol.MotivoSalida,
		AplicaResponsabilidadPatronal:              sol.AplicaResponsabilidadPatronal,
		SalariosBrutosUltimos6Meses:                salarios,
		DiasVacacionesPendientes:                    saldoVacaciones,
		EscalaCesantia:                              escalaCesantia,
		EscalaPreaviso:                              escalaPreaviso,
		TotalSalariosBrutosAguinaldoProporcional:    aguinaldoAgg.TotalSalariosBrutos,
		PeriodoAguinaldoDesde:                        periodoDesde,
	}, nil
}

// ProyectarLiquidacion calcula el resultado completo SIN alterar estado
// alguno: no persiste en `liquidaciones`, no cierra contrato, no
// desactiva el expediente. Seguro de llamar repetidamente (ej. RRHH
// simulando distintas fechas de salida).
func (s *LiquidacionService) ProyectarLiquidacion(ctx context.Context, sol ProyectarSolicitud) (*LiquidacionResultado, error) {
	input, err := s.prepararInput(ctx, sol)
	if err != nil {
		return nil, err
	}
	return CalcularLiquidacion(*input)
}

// ProcesarLiquidacion recalcula (reutilizando prepararInput, para no
// confiar en un resultado ya proyectado que pudo quedar stale) y, dentro
// de UNA transacción auditada:
//  1. Persiste el resultado en `liquidaciones` (estado='procesada').
//  2. Cierra el contrato vigente (CerrarContratoVigente, Fase 3).
//  3. Cierra empleado_datos_laborales vigente (CerrarDatosLaboralesVigente).
//  4. Desactiva el expediente (SetEmpleadoActivo activo=0).
//
// Si cualquier paso falla, toda la transacción hace rollback — un
// empleado NUNCA debe quedar con contrato cerrado pero liquidación no
// persistida, o viceversa.
func (s *LiquidacionService) ProcesarLiquidacion(ctx context.Context, sol ProyectarSolicitud, usuarioID int64) (*LiquidacionResultado, int64, error) {
	input, err := s.prepararInput(ctx, sol)
	if err != nil {
		return nil, 0, err
	}
	resultado, err := CalcularLiquidacion(*input)
	if err != nil {
		return nil, 0, err
	}

	var liquidacionID int64
	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		id, err := q.CrearLiquidacion(ctx, dto.CrearLiquidacionParams{
			EmpleadoID:                                sol.EmpleadoID,
			FechaSalida:                                sol.FechaSalida,
			MotivoSalida:                                sol.MotivoSalida,
			AplicaResponsabilidadPatronal:              sol.AplicaResponsabilidadPatronal,
			FechaIngreso:                                input.FechaIngreso,
			AntiguedadMeses:                             int32(resultado.AntiguedadMeses),
			SalarioPromedio6meses:                       resultado.SalarioPromedio6Meses,
			SalarioDiarioPromedio:                       resultado.SalarioDiarioPromedio,
			DiasPreaviso:                                int16(resultado.DiasPreaviso),
			MontoPreaviso:                               resultado.MontoPreaviso,
			DiasCesantia:                                resultado.DiasCesantia,
			MontoCesantia:                               resultado.MontoCesantia,
			DiasVacacionesPendientes:                    resultado.DiasVacacionesPendientes,
			MontoVacacionesProporcionales:                resultado.MontoVacacionesProporcionales,
			PeriodoAguinaldoDesde:                        resultado.PeriodoAguinaldoDesde,
			TotalSalariosBrutosAguinaldoProporcional:     resultado.TotalSalariosBrutosAguinaldoProporcional,
			MontoAguinaldoProporcional:                   resultado.MontoAguinaldoProporcional,
			TotalLiquidacion:                             resultado.TotalLiquidacion,
			ProcesadoPor:                                 usuarioID,
		})
		if err != nil {
			return fmt.Errorf("crear liquidacion: %w", err)
		}
		liquidacionID = id

		fechaFinVigencia := sol.FechaSalida
		if err := q.CerrarContratoVigente(ctx, dto.CerrarContratoVigenteParams{
			EmpleadoID:      sol.EmpleadoID,
			FechaFinVigencia: fechaFinVigencia,
		}); err != nil {
			return fmt.Errorf("cerrar contrato vigente: %w", err)
		}

		if err := q.CerrarDatosLaboralesVigente(ctx, dto.CerrarDatosLaboralesVigenteParams{
			EmpleadoID:      sol.EmpleadoID,
			FechaFinVigencia: fechaFinVigencia,
		}); err != nil {
			return fmt.Errorf("cerrar datos laborales vigentes: %w", err)
		}

		if err := q.SetEmpleadoActivo(ctx, dto.SetEmpleadoActivoParams{
			ID:     sol.EmpleadoID,
			Activo: false,
		}); err != nil {
			return fmt.Errorf("desactivar expediente: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return resultado, liquidacionID, nil
}