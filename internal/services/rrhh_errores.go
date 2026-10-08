// services/rrhh_errores.go
//
// F03 — Errores de negocio tipados (D8) y helpers de validación compartidos
// por empleado_service.go, contrato_service.go y organizacion_service.go.
//
// Regla: los servicios devuelven SOLO errores de este archivo (o de los
// sentinelas ya existentes: ErrEmpleadoNoEncontrado, ErrContratoNoEncontrado,
// ErrNoEncontrado, ErrSalarioInferiorAlMinimo) para condiciones de negocio, y
// errores envueltos con contexto para fallos técnicos. Los handlers traducen
// los de negocio a HTTP con errors.Is/errors.As (ver
// api/handlers/rrhh_errores.go); el resto sale por middleware.FailFromError.
// Ningún handler debe volcar err.Error() al cliente.
package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
)

// ----------------------------- Sentinelas ------------------------------------

var (
	ErrPuestoNoEncontrado    = errors.New("el puesto no existe")
	ErrPuestoInactivo        = errors.New("el puesto está inactivo")
	ErrFamiliarNoEncontrado  = errors.New("la declaración familiar vigente no existe")
	ErrContratoVigenteExiste = errors.New("el empleado ya tiene un contrato vigente; use actualizar contrato")

	// ErrSalarioMinimoNoParametrizado: no hay fila vigente en
	// parametros_salario_minimo para la clasificación del puesto (o el puesto
	// no tiene clasificación). NO se asume ningún valor por defecto: el alta
	// o cambio de salario se rechaza hasta que se cargue el parámetro.
	ErrSalarioMinimoNoParametrizado = errors.New("no hay salario mínimo vigente parametrizado para la clasificación del puesto")

	ErrVigenciaTraslapada      = errors.New("la fecha efectiva traslapa o precede la vigencia actual")
	ErrIdentificacionDuplicada = errors.New("ya existe un empleado con ese tipo y número de identificación")
	ErrCodigoInternoDuplicado  = errors.New("ya existe un empleado con ese código interno")
	ErrRegistroDuplicado       = errors.New("ya existe un registro con ese código")
	ErrReferenciaInvalida      = errors.New("una referencia (FK) apunta a un registro que no existe")
	ErrJerarquiaCiclica        = errors.New("la unidad padre indicada generaría un ciclo en la jerarquía")
)

// ----------------------------- Tipados ---------------------------------------

// ErrValidacion es un error de entrada (campo + motivo) que el handler
// responde como 400. El mensaje lo redactamos nosotros: es seguro de exponer.
type ErrValidacion struct {
	Campo   string
	Mensaje string
}

func (e *ErrValidacion) Error() string { return e.Campo + ": " + e.Mensaje }

func validacion(campo, mensaje string) error {
	return &ErrValidacion{Campo: campo, Mensaje: mensaje}
}

// SalarioInferiorError detalla un incumplimiento del salario mínimo.
// errors.Is(err, ErrSalarioInferiorAlMinimo) == true.
type SalarioInferiorError struct {
	PuestoID int64
	Pactado  decimal.Decimal
	Minimo   decimal.Decimal
}

func (e *SalarioInferiorError) Error() string {
	return fmt.Sprintf("%s: pactado %s, mínimo vigente %s",
		ErrSalarioInferiorAlMinimo.Error(), e.Pactado.String(), e.Minimo.String())
}
func (e *SalarioInferiorError) Unwrap() error { return ErrSalarioInferiorAlMinimo }

// SalarioMinimoNoParametrizadoError: errors.Is(err, ErrSalarioMinimoNoParametrizado) == true.
// ClasificacionPuestoID es nil si el puesto no tiene clasificación asignada.
type SalarioMinimoNoParametrizadoError struct {
	PuestoID              int64
	ClasificacionPuestoID *int64
}

func (e *SalarioMinimoNoParametrizadoError) Error() string {
	if e.ClasificacionPuestoID == nil {
		return fmt.Sprintf("%s (el puesto %d no tiene clasificación)", ErrSalarioMinimoNoParametrizado.Error(), e.PuestoID)
	}
	return fmt.Sprintf("%s (puesto %d, clasificación %d)", ErrSalarioMinimoNoParametrizado.Error(), e.PuestoID, *e.ClasificacionPuestoID)
}
func (e *SalarioMinimoNoParametrizadoError) Unwrap() error { return ErrSalarioMinimoNoParametrizado }

// VigenciaTraslapadaError: errors.Is(err, ErrVigenciaTraslapada) == true.
type VigenciaTraslapadaError struct {
	Campo          string
	FechaEfectiva  time.Time
	InicioVigencia time.Time // inicio de la versión vigente actual
}

func (e *VigenciaTraslapadaError) Error() string {
	return fmt.Sprintf("%s: la fecha efectiva %s debe ser posterior al inicio de la vigencia actual (%s)",
		e.Campo, e.FechaEfectiva.Format(layoutFecha), e.InicioVigencia.Format(layoutFecha))
}
func (e *VigenciaTraslapadaError) Unwrap() error { return ErrVigenciaTraslapada }

// ----------------------------- Helpers ---------------------------------------

// soloFecha normaliza un time.Time a medianoche UTC conservando año/mes/día,
// para comparar DATEs sin que influya la zona horaria con la que el driver
// materializó la columna.
func soloFecha(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// validarNuevaVigencia aplica la regla de vigencia de 2 pasos antes de
// cerrar la versión vigente: la nueva fecha efectiva debe ser POSTERIOR al
// inicio de la versión que se va a cerrar. Si no, el cierre dejaría
// fecha_fin < fecha_inicio (viola chk_*_vigencia) o dejaría una versión de
// duración cero / traslapada. El invariante "una sola vigente" y "las
// versiones cerradas terminan antes del inicio de la vigente" hace que
// comparar contra la vigente sea suficiente.
func validarNuevaVigencia(campo string, fechaEfectiva, inicioVigenteActual time.Time) error {
	fe, ini := soloFecha(fechaEfectiva), soloFecha(inicioVigenteActual)
	if !fe.After(ini) {
		return &VigenciaTraslapadaError{Campo: campo, FechaEfectiva: fe, InicioVigencia: ini}
	}
	return nil
}

var maxMontoSalario = decimal.New(1, 12) // DECIMAL(14,2): < 10^12

// validarMontoSalario valida un salario/monto contra DECIMAL(14,2): > 0,
// máximo 2 decimales y dentro de rango.
func validarMontoSalario(campo string, monto decimal.Decimal) error {
	if !monto.IsPositive() {
		return validacion(campo, "debe ser mayor que 0")
	}
	if !monto.Equal(monto.Round(2)) {
		return validacion(campo, "admite como máximo 2 decimales")
	}
	if monto.GreaterThanOrEqual(maxMontoSalario) {
		return validacion(campo, "excede el máximo permitido")
	}
	return nil
}

// noVacio recorta espacios y exige un valor no vacío.
func noVacio(campo, valor string) (string, error) {
	v := strings.TrimSpace(valor)
	if v == "" {
		return "", validacion(campo, "es requerido")
	}
	return v, nil
}

// validarRangoFechas valida fecha_fin_contrato >= fecha_inicio_contrato
// (chk_contrato_fechas) con un error de negocio en vez del 3819 de MySQL.
func validarRangoFechas(campoFin string, inicio, fin time.Time) error {
	if soloFecha(fin).Before(soloFecha(inicio)) {
		return validacion(campoFin, "no puede ser anterior a la fecha de inicio")
	}
	return nil
}

// Códigos de error de MySQL relevantes.
const (
	mysqlErrDuplicado    = 1062 // ER_DUP_ENTRY
	mysqlErrFKHijoFalla  = 1452 // ER_NO_REFERENCED_ROW_2
	mysqlErrCheckFallida = 3819 // ER_CHECK_CONSTRAINT_VIOLATED
)

// traducirErrorBD convierte violaciones de UNIQUE/FK conocidas de MySQL en
// errores de negocio tipados; el resto se envuelve con contexto. La UNIQUE
// de la BD es el árbitro final (no hay pre-chequeo racy).
func traducirErrorBD(err error, contexto string) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		switch me.Number {
		case mysqlErrDuplicado:
			switch {
			case strings.Contains(me.Message, "uk_empleado_identificacion"):
				return fmt.Errorf("%s: %w", contexto, ErrIdentificacionDuplicada)
			case strings.Contains(me.Message, "codigo_interno"):
				return fmt.Errorf("%s: %w", contexto, ErrCodigoInternoDuplicado)
			default:
				return fmt.Errorf("%s: %w", contexto, ErrRegistroDuplicado)
			}
		case mysqlErrFKHijoFalla:
			return fmt.Errorf("%s: %w", contexto, ErrReferenciaInvalida)
		}
	}
	return fmt.Errorf("%s: %w", contexto, err)
}