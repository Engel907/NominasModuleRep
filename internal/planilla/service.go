// Fase 6c — Comprobantes, boletas de pago y exportación bancaria/CCSS.
//
// Este archivo separa, igual que el motor de cálculo (ver service.go), el
// ACCESO A DATOS (ExportRepository, implementado sobre dto.Queries, ver
// NewExportRepository)
// del ARMADO DEL ARCHIVO/RESPUESTA (ExportService), para poder testear el
// formateo del TXT de dispersión y el ensamblado de la boleta sin una BD
// real.
package planilla

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
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
	OrdenCalculo   int32
}

// EmbargoBoletaPago corresponde a una fila de ObtenerEmbargosBoletaPago.
type EmbargoBoletaPago struct {
	EmbargoID                  int64
	TipoEmbargoCodigo          string
	TipoEmbargoNombre          string
	PrioridadLegal             int32
	MontoCalculado             decimal.Decimal
	MontoAplicado              decimal.Decimal
	SaldoPendienteInsatisfecho decimal.Decimal
}

// ExportRepository es el subconjunto de acceso a datos que necesita este
// archivo. Se define como interfaz propia (no se reutiliza
// PayrollRepository de service.go) porque son consultas de LECTURA sobre
// resultados YA calculados y persistidos — no participan del cálculo puro
// ni de la estrategia de bloqueo de periodos_planilla.
type ExportRepository interface {
	// ObtenerDetallesPlanillaParaDispersion retorna una fila por empleado
	// del período, solo si el período está 'calculado' o 'cerrado'. Debe
	// venir ordenado por nombre (ver query) para que el archivo generado
	// sea determinístico entre corridas.
	ObtenerDetallesPlanillaParaDispersion(ctx context.Context, periodoID int64) ([]FilaDispersionBancaria, error)

	// ObtenerDatosBoletaPago retorna el encabezado agregado. Debe
	// retornar un error explícito (nunca una fila vacía/zero-value) si no
	// existe planilla_detalle_empleado para ese periodoID+empleadoID —
	// mismo criterio de "sin defaults implícitos" del motor de cálculo.
	ObtenerDatosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) (EncabezadoBoletaPago, error)

	ObtenerDesgloseConceptosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) ([]ConceptoBoletaPago, error)

	ObtenerEmbargosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) ([]EmbargoBoletaPago, error)
}

// ============================================================================
// Implementación de ExportRepository sobre internal/db/dto
// ============================================================================

// exportQueries es el subconjunto de *dto.Queries que usa el repositorio
// (ver internal/db/dto/planilla_export.go). Se declara como interfaz para
// poder testear el mapeo con un fake, sin BD real.
type exportQueries interface {
	ObtenerDetallesPlanillaParaDispersion(ctx context.Context, periodoID int64) ([]dto.ObtenerDetallesPlanillaParaDispersionRow, error)
	ObtenerDatosBoletaPago(ctx context.Context, periodoID, empleadoID int64) (dto.ObtenerDatosBoletaPagoRow, error)
	ObtenerDesgloseConceptosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]dto.ObtenerDesgloseConceptosBoletaPagoRow, error)
	ObtenerEmbargosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]dto.ObtenerEmbargosBoletaPagoRow, error)
}

// Verificación en compilación: *dto.Queries implementa exportQueries.
var _ exportQueries = (*dto.Queries)(nil)

type exportRepository struct {
	q exportQueries
}

// NewExportRepository conecta ExportService con la base de datos. Uso
// típico en el wiring: planilla.NewExportRepository(dto.New(sqlDB)).
// Como son solo lecturas sobre resultados ya persistidos, no requiere
// transacción ni WithAuditContext.
func NewExportRepository(q exportQueries) ExportRepository {
	return &exportRepository{q: q}
}

func (r *exportRepository) ObtenerDetallesPlanillaParaDispersion(ctx context.Context, periodoID int64) ([]FilaDispersionBancaria, error) {
	rows, err := r.q.ObtenerDetallesPlanillaParaDispersion(ctx, periodoID)
	if err != nil {
		return nil, err
	}
	out := make([]FilaDispersionBancaria, 0, len(rows))
	for _, x := range rows {
		out = append(out, FilaDispersionBancaria{
			EmpleadoID:     x.EmpleadoID,
			Cedula:         x.Cedula,
			NombreCompleto: x.NombreCompleto,
			IBAN:           x.IBAN,
			NetoPagado:     x.NetoPagado,
		})
	}
	return out, nil
}

// ObtenerDatosBoletaPago propaga sql.ErrNoRows (envuelto con %w por
// ExportService) si no existe el detalle: nunca devuelve un
// EncabezadoBoletaPago vacío como si fuera válido.
func (r *exportRepository) ObtenerDatosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) (EncabezadoBoletaPago, error) {
	x, err := r.q.ObtenerDatosBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return EncabezadoBoletaPago{}, err
	}
	return EncabezadoBoletaPago{
		PlanillaDetalleEmpleadoID: x.PlanillaDetalleEmpleadoID,
		EmpleadoID:                x.EmpleadoID,
		PeriodoID:                 x.PeriodoID,
		NombreCompleto:            x.NombreCompleto,
		Cedula:                    x.Cedula,
		SalarioBasePeriodo:        x.SalarioBasePeriodo,
		TotalIngresos:             x.TotalIngresos,
		TotalDeducciones:          x.TotalDeducciones,
		NetoPagado:                x.NetoPagado,
	}, nil
}

func (r *exportRepository) ObtenerDesgloseConceptosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) ([]ConceptoBoletaPago, error) {
	rows, err := r.q.ObtenerDesgloseConceptosBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	out := make([]ConceptoBoletaPago, 0, len(rows))
	for _, x := range rows {
		out = append(out, ConceptoBoletaPago{
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

func (r *exportRepository) ObtenerEmbargosBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) ([]EmbargoBoletaPago, error) {
	rows, err := r.q.ObtenerEmbargosBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, err
	}
	out := make([]EmbargoBoletaPago, 0, len(rows))
	for _, x := range rows {
		out = append(out, EmbargoBoletaPago{
			EmbargoID:                  x.EmbargoID,
			TipoEmbargoCodigo:          x.TipoEmbargoCodigo,
			TipoEmbargoNombre:          x.TipoEmbargoNombre,
			PrioridadLegal:             x.PrioridadLegal,
			MontoCalculado:             x.MontoCalculado,
			MontoAplicado:              x.MontoAplicado,
			SaldoPendienteInsatisfecho: x.SaldoPendienteInsatisfecho,
		})
	}
	return out, nil
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
	// dosDecimalesExport se mantiene separado de la constante homónima de
	// calculations.go (paquete distinto de archivo, mismo paquete
	// `planilla`) solo por claridad de lectura de este archivo.
	dosDecimalesExport = int32(2)

	// separadorDispersion: coma, según lo pedido explícitamente
	// ("TXT delimitado por comas"). ASUNCIÓN a confirmar con el banco real
	// (BCR/BN/Banco Popular, etc. — cada plataforma SINPE/transferencia
	// masiva tiene su propio layout; este formato es el mínimo pedido en
	// la tarea, no un layout oficial verificado).
	separadorDispersion = ","

	// finLineaDispersion: CRLF. ASUNCIÓN — varias plataformas bancarias de
	// procesamiento por lotes en Costa Rica exigen CRLF en archivos planos;
	// si la plataforma destino específica exige LF, cambiar solo esta
	// constante.
	finLineaDispersion = "\r\n"
)

// ErrPlanillaSinFilasParaDispersion se retorna si el período no tiene
// ninguna fila 'calculado'/'cerrado' para dispersar — evita generar y
// entregar un archivo vacío como si fuera válido.
var ErrPlanillaSinFilasParaDispersion = fmt.Errorf("planilla: no hay filas de dispersión bancaria para el período (¿período no cerrado, o sin empleados con datos bancarios vigentes?)")

type ExportService struct {
	repo ExportRepository
}

func NewExportService(repo ExportRepository) *ExportService {
	return &ExportService{repo: repo}
}

// GenerarArchivoDispersionBancaria arma el archivo plano de dispersión
// bancaria del período: una línea por empleado, formato
// "IBAN,Cédula,Nombre,Monto", CRLF entre líneas. Monto siempre con 2
// decimales fijos (StringFixed(2)) — nunca notación científica ni
// separador de miles, para máxima compatibilidad con parsers bancarios
// simples.
//
// Retorna el buffer y el nombre de archivo sugerido
// ("dispersion_bancaria_periodo_<id>_<fecha>.txt"); el handler decide qué
// hacer con ambos (Content-Disposition, etc.).
func (s *ExportService) GenerarArchivoDispersionBancaria(ctx context.Context, periodoID int64) ([]byte, string, error) {
	filas, err := s.repo.ObtenerDetallesPlanillaParaDispersion(ctx, periodoID)
	if err != nil {
		return nil, "", fmt.Errorf("planilla: error obteniendo filas de dispersión para periodo_id=%d: %w", periodoID, err)
	}
	if len(filas) == 0 {
		return nil, "", ErrPlanillaSinFilasParaDispersion
	}

	var buf bytes.Buffer
	for _, fila := range filas {
		if err := validarFilaDispersion(fila); err != nil {
			return nil, "", fmt.Errorf("planilla: empleado_id=%d inválido para dispersión: %w", fila.EmpleadoID, err)
		}

		linea := strings.Join([]string{
			fila.IBAN,
			fila.Cedula,
			sanitizarCampoDispersion(fila.NombreCompleto),
			fila.NetoPagado.StringFixed(dosDecimalesExport),
		}, separadorDispersion)

		buf.WriteString(linea)
		buf.WriteString(finLineaDispersion)
	}

	nombreArchivo := fmt.Sprintf("dispersion_bancaria_periodo_%d_%s.txt", periodoID, time.Now().Format("20060102_150405"))
	return buf.Bytes(), nombreArchivo, nil
}

// validarFilaDispersion evita generar líneas inconsistentes en un archivo
// que un banco va a procesar sin validación humana previa: IBAN/cédula
// vacíos, o neto negativo/cero, se rechazan con error explícito en vez de
// dispersarse silenciosamente (mismo criterio de "nunca defaults
// implícitos" de ADR-000/domain.go).
func validarFilaDispersion(fila FilaDispersionBancaria) error {
	if strings.TrimSpace(fila.IBAN) == "" {
		return fmt.Errorf("IBAN vacío")
	}
	if strings.TrimSpace(fila.Cedula) == "" {
		return fmt.Errorf("cédula vacía")
	}
	if !fila.NetoPagado.IsPositive() {
		return fmt.Errorf("neto_pagado no es positivo (%s)", fila.NetoPagado.StringFixed(dosDecimalesExport))
	}
	return nil
}

// sanitizarCampoDispersion reemplaza el separador y saltos de línea que
// pudieran venir en un nombre (ej. nombres compuestos con coma en algún
// registro mal capturado) para no romper el parseo por columnas del
// archivo. No trunca ni transforma acentos/mayúsculas — eso es decisión
// del layout bancario real, no verificada aquí.
func sanitizarCampoDispersion(campo string) string {
	campo = strings.ReplaceAll(campo, separadorDispersion, " ")
	campo = strings.ReplaceAll(campo, "\r", " ")
	campo = strings.ReplaceAll(campo, "\n", " ")
	return strings.TrimSpace(campo)
}

// ObtenerBoletaPago ensambla la boleta individual de un empleado en un
// período: encabezado + desglose de conceptos (separado en ingresos y
// deducciones por conceptos_salariales.naturaleza) + embargos aplicados.
func (s *ExportService) ObtenerBoletaPago(ctx context.Context, periodoID int64, empleadoID int64) (*BoletaPagoOutput, error) {
	encabezado, err := s.repo.ObtenerDatosBoletaPago(ctx, periodoID, empleadoID)
	if err != nil {
		return nil, fmt.Errorf("planilla: no se encontró planilla_detalle_empleado para periodo_id=%d, empleado_id=%d: %w", periodoID, empleadoID, err)
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
			// Naturaleza desconocida: no se descarta silenciosamente el
			// concepto (perdería visibilidad en la boleta), pero tampoco
			// se asume un lado — se reporta como error explícito para
			// que se corrija el dato en conceptos_salariales.
			return nil, fmt.Errorf("planilla: concepto %q tiene naturaleza desconocida %q", c.CodigoConcepto, c.Naturaleza)
		}
	}

	return &BoletaPagoOutput{
		Encabezado:  encabezado,
		Ingresos:    ingresos,
		Deducciones: deducciones,
		Embargos:    embargos,
		GeneradaEn:  time.Now(),
	}, nil
}