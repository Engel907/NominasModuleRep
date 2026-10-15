// Fase 6c / F09 — Comprobantes, boletas de pago y exportación bancaria.
//
// Este archivo separa el ACCESO A DATOS (ExportRepository y ExportUnitOfWork,
// cuya implementación sobre dto.Queries vive en export_repository.go) del
// ARMADO DEL ARCHIVO/RESPUESTA (ExportService), para poder testear el
// formateo del TXT de dispersión y el ensamblado de la boleta sin una BD
// real. Este archivo NO importa internal/db ni internal/db/dto.
package planilla

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/authz"
)

// ============================================================================
// Contratos de lectura (implementados en internal/db/dto/planilla_export.go)
// ============================================================================

// FilaDispersionBancaria es la fila que necesita el archivo plano de
// dispersión. Corresponde 1:1 a ObtenerDetallesPlanillaParaDispersion.
type FilaDispersionBancaria struct {
	EmpleadoID     int64
	Cedula         string
	NombreCompleto string
	IBAN           string
	NetoPagado     decimal.Decimal
}

// EncabezadoBoletaPago corresponde a ObtenerDatosBoletaPago (:one).
type EncabezadoBoletaPago struct {
	PlanillaDetalleEmpleadoID int64
	EmpleadoID                int64
	PeriodoID                 int64
	NombreCompleto            string
	Cedula                    string
	SalarioBasePeriodo        decimal.Decimal
	TotalIngresos             decimal.Decimal
	TotalDeducciones          decimal.Decimal
	NetoPagado                decimal.Decimal
}

// ConceptoBoletaPago corresponde a una fila de
// ObtenerDesgloseConceptosBoletaPago.
type ConceptoBoletaPago struct {
	CodigoConcepto string
	NombreConcepto string
	Naturaleza     string // 'ingreso' | 'deduccion' — ver conceptos_salariales.naturaleza
	Monto          decimal.Decimal
	Cantidad       decimal.Decimal
	// OrdenCalculo viene de orden_calculo_concepto vigente a la fecha fin
	// del período; 0 si el concepto no tiene orden vigente.
	OrdenCalculo int32
}

// EmbargoBoletaPago corresponde a una fila de ObtenerEmbargosBoletaPago.
//
// F20: planilla_detalle_embargo hoy solo guarda orden_prioridad y monto
// (monto efectivamente retenido). F20 agregará monto_calculado,
// monto_aplicado y saldo_pendiente_insatisfecho; entonces se amplía este struct.
type EmbargoBoletaPago struct {
	EmbargoID         int64
	TipoEmbargoCodigo string
	TipoEmbargoNombre string
	OrdenPrioridad    int32 // planilla_detalle_embargo.orden_prioridad
	Monto             decimal.Decimal
}

// ExportRepository es el subconjunto de acceso a datos de LECTURA que
// necesita este archivo, sobre resultados YA calculados y persistidos.
type ExportRepository interface {
	// ObtenerDetallesPlanillaParaDispersion retorna una fila por empleado
	// del período, solo si el período está en EstadosParaDispersion()
	// (aprobada|pagada). Viene ordenado por nombre e id para que el archivo
	// sea determinístico entre corridas.
	ObtenerDetallesPlanillaParaDispersion(ctx context.Context, periodoID int64) ([]FilaDispersionBancaria, error)

	// ObtenerDatosBoletaPago retorna el encabezado agregado, solo si el
	// período está aprobada|pagada|contabilizada. Retorna error explícito
	// (sql.ErrNoRows envuelto) si no existe: nunca un zero-value.
	ObtenerDatosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) (EncabezadoBoletaPago, error)

	ObtenerDesgloseConceptosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) ([]ConceptoBoletaPago, error)

	ObtenerEmbargosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) ([]EmbargoBoletaPago, error)
}

// ============================================================================
// Trazabilidad (S14): registro de exportaciones en exportaciones_generadas
// ============================================================================

// FormatoDispersionTXT es formatos_exportacion.codigo del TXT de dispersión
// delimitado por comas (lo siembra la migración 000109).
const FormatoDispersionTXT = "BANCO_TXT_COMAS"

// ExportacionRegistro es lo que se persiste por cada archivo generado.
type ExportacionRegistro struct {
	PeriodoID     int64
	FormatoCodigo string
	NombreArchivo string
	HashSHA256    string // hex en minúsculas, 64 caracteres
	CantidadFilas int64
	TotalMonto    decimal.Decimal
	UsuarioID     int64
}

// ExportRegistrador inserta la fila de trazabilidad.
type ExportRegistrador interface {
	RegistrarExportacion(ctx context.Context, r ExportacionRegistro) (int64, error)
}

// ExportTx es lo que ve el servicio dentro de la transacción auditada:
// lecturas y registro comparten la MISMA transacción.
type ExportTx interface {
	ExportRepository
	ExportRegistrador
}

// ExportUnitOfWork ejecuta fn dentro de una transacción auditada
// (db.WithAuditContext). Si fn devuelve error se hace rollback: no queda
// registro de un archivo que no se entregó.
type ExportUnitOfWork interface {
	EjecutarAuditado(ctx context.Context, usuarioID int64, fn func(ctx context.Context, tx ExportTx) error) error
}

// ============================================================================
// Salida: boleta de pago (para serializar como JSON en el handler)
// ============================================================================

// BoletaPagoOutput es la estructura completa de la boleta individual.
type BoletaPagoOutput struct {
	Encabezado  EncabezadoBoletaPago
	Ingresos    []ConceptoBoletaPago
	Deducciones []ConceptoBoletaPago
	Embargos    []EmbargoBoletaPago
	GeneradaEn  time.Time
}

// ============================================================================
// ExportService
// ============================================================================

const (
	dosDecimalesExport = int32(2)

	// separadorDispersion: coma ("TXT delimitado por comas"). ASUNCIÓN a
	// confirmar con el banco real: cada plataforma tiene su propio layout.
	separadorDispersion = ","

	// finLineaDispersion: CRLF. ASUNCIÓN: varias plataformas bancarias por
	// lotes en Costa Rica exigen CRLF; si la destino exige LF, cambiar aquí.
	finLineaDispersion = "\r\n"
)

// ErrPlanillaSinFilasParaDispersion: el período no tiene filas dispersables.
var ErrPlanillaSinFilasParaDispersion = errors.New("planilla: no hay filas de dispersión bancaria para el período (¿período no aprobado/pagado, o sin empleados con datos bancarios vigentes?)")

// ErrUsuarioExportRequerido: la exportación exige un usuario identificado.
var ErrUsuarioExportRequerido = errors.New("planilla: usuarioID es requerido para exportar")

var (
	ErrBoletaSinPermiso           = errors.New("planilla: el usuario no tiene permiso para ver boletas")
	ErrBoletaNoPropia             = errors.New("planilla: solo se permite consultar la boleta propia")
	ErrBoletaSinEmpleadoVinculado = errors.New("planilla: el usuario no tiene empleado vinculado")
)

// Motivos (controlados) por los que una fila de dispersión se rechaza. Son
// seguros de mostrar al operador: no contienen datos bancarios.
const (
	MotivoIBANVacio      = "IBAN vacío"
	MotivoIBANInvalido   = "IBAN con caracteres no permitidos"
	MotivoCedulaVacia    = "cédula vacía"
	MotivoCedulaInvalida = "cédula con caracteres no permitidos"
	MotivoNetoNoPositivo = "neto a pagar no es positivo"
	MotivoNetoDecimales  = "neto a pagar con más de 2 decimales"
)

// FilaDispersionError identifica qué empleado impide generar el archivo y
// por qué (Motivo es uno de las constantes Motivo*).
type FilaDispersionError struct {
	EmpleadoID int64
	Motivo     string
}

func (e *FilaDispersionError) Error() string {
	return fmt.Sprintf("planilla: empleado_id=%d inválido para dispersión: %s", e.EmpleadoID, e.Motivo)
}

type ExportService struct {
	repo ExportRepository
	uow  ExportUnitOfWork
	now  func() time.Time
}

// NewExportService: repo atiende las boletas (solo lectura); uow atiende la
// dispersión (lectura + registro en una transacción auditada).
func NewExportService(repo ExportRepository, uow ExportUnitOfWork) *ExportService {
	return &ExportService{repo: repo, uow: uow, now: time.Now}
}

// GenerarArchivoDispersionBancaria arma el archivo plano de dispersión del
// período ("IBAN,Cédula,Nombre,Monto", CRLF) y registra la exportación
// (quién, cuándo, período, formato, nombre, SHA-256, filas y total) en
// exportaciones_generadas en la MISMA transacción auditada. Si algo falla,
// no se entrega archivo ni queda registro.
//
// Devuelve el contenido y el nombre de archivo sugerido.
func (s *ExportService) GenerarArchivoDispersionBancaria(ctx context.Context, periodoID int64, usuarioID int64) ([]byte, string, error) {
	if usuarioID <= 0 {
		return nil, "", ErrUsuarioExportRequerido
	}

	var (
		contenido     []byte
		nombreArchivo string
	)
	err := s.uow.EjecutarAuditado(ctx, usuarioID, func(ctx context.Context, tx ExportTx) error {
		filas, err := tx.ObtenerDetallesPlanillaParaDispersion(ctx, periodoID)
		if err != nil {
			return fmt.Errorf("planilla: error obteniendo filas de dispersión para periodo_id=%d: %w", periodoID, err)
		}
		if len(filas) == 0 {
			return ErrPlanillaSinFilasParaDispersion
		}

		buf, total, err := armarArchivoDispersion(filas)
		if err != nil {
			return err
		}

		nombre := nombreArchivoDispersion(periodoID, s.now())
		suma := sha256.Sum256(buf)
		if _, err := tx.RegistrarExportacion(ctx, ExportacionRegistro{
			PeriodoID:     periodoID,
			FormatoCodigo: FormatoDispersionTXT,
			NombreArchivo: nombre,
			HashSHA256:    hex.EncodeToString(suma[:]),
			CantidadFilas: int64(len(filas)),
			TotalMonto:    total,
			UsuarioID:     usuarioID,
		}); err != nil {
			return fmt.Errorf("planilla: no se pudo registrar la exportación del periodo_id=%d: %w", periodoID, err)
		}

		contenido, nombreArchivo = buf, nombre
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return contenido, nombreArchivo, nil
}

// armarArchivoDispersion es la parte pura: valida cada fila, arma el TXT y
// suma el total. Cualquier fila inválida aborta todo (nunca un archivo parcial).
func armarArchivoDispersion(filas []FilaDispersionBancaria) ([]byte, decimal.Decimal, error) {
	var buf bytes.Buffer
	total := decimal.Zero
	for _, fila := range filas {
		if err := validarFilaDispersion(fila); err != nil {
			return nil, decimal.Zero, err
		}
		linea := strings.Join([]string{
			strings.TrimSpace(fila.IBAN),
			strings.TrimSpace(fila.Cedula),
			sanitizarCampoDispersion(fila.NombreCompleto),
			fila.NetoPagado.StringFixed(dosDecimalesExport),
		}, separadorDispersion)
		buf.WriteString(linea)
		buf.WriteString(finLineaDispersion)
		total = total.Add(fila.NetoPagado)
	}
	return buf.Bytes(), total, nil
}

func nombreArchivoDispersion(periodoID int64, t time.Time) string {
	return fmt.Sprintf("dispersion_bancaria_periodo_%d_%s.txt", periodoID, t.Format("20060102_150405"))
}

// campoTieneCaracterProhibido: separador o cualquier carácter de control
// (incluye CR, LF, TAB, NUL) rompería la estructura de la línea.
func campoTieneCaracterProhibido(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || strings.ContainsRune(separadorDispersion, r)
	}) >= 0
}

// validarFilaDispersion evita generar líneas inconsistentes en un archivo que
// un banco procesa sin validación humana previa. Devuelve *FilaDispersionError.
func validarFilaDispersion(fila FilaDispersionBancaria) error {
	rechazar := func(motivo string) error {
		return &FilaDispersionError{EmpleadoID: fila.EmpleadoID, Motivo: motivo}
	}
	iban := strings.TrimSpace(fila.IBAN)
	if iban == "" {
		return rechazar(MotivoIBANVacio)
	}
	if campoTieneCaracterProhibido(iban) {
		return rechazar(MotivoIBANInvalido)
	}
	cedula := strings.TrimSpace(fila.Cedula)
	if cedula == "" {
		return rechazar(MotivoCedulaVacia)
	}
	if campoTieneCaracterProhibido(cedula) {
		return rechazar(MotivoCedulaInvalida)
	}
	if !fila.NetoPagado.IsPositive() {
		return rechazar(MotivoNetoNoPositivo)
	}
	// StringFixed(2) redondearía en silencio: un monto con más de 2 decimales
	// indica un dato corrupto, no algo que haya que "arreglar" al pagar.
	if !fila.NetoPagado.Equal(fila.NetoPagado.Round(dosDecimalesExport)) {
		return rechazar(MotivoNetoDecimales)
	}
	return nil
}

// sanitizarCampoDispersion reemplaza por espacio el separador y cualquier
// carácter de control (CR, LF, TAB, NUL...) y recorta los extremos. No trunca
// ni transforma acentos/mayúsculas: eso depende del layout bancario real.
func sanitizarCampoDispersion(campo string) string {
	campo = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(separadorDispersion, r) {
			return ' '
		}
		return r
	}, campo)
	return strings.TrimSpace(campo)
}

// ObtenerBoletaPago ensambla la boleta individual de un empleado en un
// período: encabezado + desglose (ingresos/deducciones por naturaleza) +
// embargos aplicados. Solo períodos aprobada|pagada|contabilizada.
func (s *ExportService) ObtenerBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) (*BoletaPagoOutput, error) {
	encabezado, err := s.repo.ObtenerDatosBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, fmt.Errorf("planilla: boleta no disponible para periodo_id=%d, empleado_id=%d (inexistente o período no aprobado): %w", periodoID, empleadoID, err)
	}

	conceptos, err := s.repo.ObtenerDesgloseConceptosBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, fmt.Errorf("planilla: error obteniendo desglose de conceptos: %w", err)
	}

	embargos, err := s.repo.ObtenerEmbargosBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, fmt.Errorf("planilla: error obteniendo embargos aplicados: %w", err)
	}

	ingresos := make([]ConceptoBoletaPago, 0, len(conceptos))
	deducciones := make([]ConceptoBoletaPago, 0, len(conceptos))
	for _, c := range conceptos {
		switch c.Naturaleza {
		case "ingreso":
			ingresos = append(ingresos, c)
		case "deduccion":
			deducciones = append(deducciones, c)
		default:
			return nil, fmt.Errorf("planilla: concepto %q tiene naturaleza desconocida %q", c.CodigoConcepto, c.Naturaleza)
		}
	}

	return &BoletaPagoOutput{
		Encabezado:  encabezado,
		Ingresos:    ingresos,
		Deducciones: deducciones,
		Embargos:    embargos,
		GeneradaEn:  s.now(),
	}, nil
}

// ObtenerBoletaPagoAutorizada aplica autorización por fila en el servicio,
// independientemente de la ruta que haya invocado el handler.
func (s *ExportService) ObtenerBoletaPagoAutorizada(ctx context.Context, periodoID, empleadoID, empleadoUsuarioID int64, permisos []string) (*BoletaPagoOutput, error) {
	if tienePermisoBoleta(permisos, authz.PermPlanillaVerBoleta) {
		return s.ObtenerBoletaPago(ctx, periodoID, empleadoID)
	}
	if !tienePermisoBoleta(permisos, authz.PermPlanillaVerBoletaPropia) {
		return nil, ErrBoletaSinPermiso
	}
	if empleadoUsuarioID <= 0 {
		return nil, ErrBoletaSinEmpleadoVinculado
	}
	if empleadoID != empleadoUsuarioID {
		return nil, ErrBoletaNoPropia
	}
	return s.ObtenerBoletaPago(ctx, periodoID, empleadoID)
}

func tienePermisoBoleta(permisos []string, buscado string) bool {
	for _, permiso := range permisos {
		if permiso == buscado {
			return true
		}
	}
	return false
}
