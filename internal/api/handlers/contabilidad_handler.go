// api/handlers/contabilidad_handler.go
//
// Fase 11B: exposición HTTP del módulo de contabilización (internal/contabilidad).
//
// Endpoints (montados en routes.go, grupo `protegido`):
//
//   - POST /planillas/:id/contabilizar   → asientos_contables.generar
//     Genera y persiste los asientos de un período en estado 'pagada' (uno por
//     tipo_asiento) y lo marca 'contabilizada'. Todo ocurre en UNA transacción
//     auditada (db.WithAuditContext) dentro de
//     contabilidad.Service.ContabilizarPeriodo; cualquier otro estado → 409.
//   - GET  /asientos-contables?periodo_id=&tipo=   → asientos_contables.ver
//     Consulta asientos ya generados, filtrando por período y/o por código de
//     tipo de asiento (al menos un filtro es obligatorio).
//
// Partida doble: este handler NO reimplementa la validación. Service llama a
// GenerarAsientos → ValidarPartidaDoble ANTES de abrir cualquier INSERT; si
// no cuadra (por empleado, por programa o en toda la corrida) retorna
// *DescuadreError, se hace rollback y aquí se traduce a 422 sin que quede
// nada persistido.
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/contabilidad"
)

// ContabilidadServicio es lo que el handler necesita de contabilidad.Service.
// *contabilidad.Service lo satisface tal cual.
type ContabilidadServicio interface {
	ContabilizarPeriodo(ctx context.Context, periodoID, usuarioID int64, detalles []contabilidad.DetallePlanilla) ([]contabilidad.AsientoContable, error)
}

// DetallesCorrida entrega el resultado en memoria de la corrida del período.
//
// Nota: contabilidad.DetallePlanilla exige Calculo.CuotaPatronalCCSS, que NO se
// persiste. contabilidad.DetallesSQL (sql_adapters.go) la deriva de lo
// persistido con la misma fórmula del motor y la valida contra la cuota obrera
// guardada; ver su comentario para los supuestos.
type DetallesCorrida interface {
	DetallesParaContabilizar(ctx context.Context, periodoID int64) ([]contabilidad.DetallePlanilla, error)
}

// LectorAsientos lee asientos ya persistidos (cabecera + líneas). Debe llenar
// TipoAsientoCodigo en cada asiento.
type LectorAsientos interface {
	ListarAsientos(ctx context.Context, filtro contabilidad.FiltroAsientos) ([]contabilidad.AsientoContable, error)
}

// ContabilidadDeps agrupa las dependencias del handler. Se construye en
// cmd/api/main.go (buildDeps).
type ContabilidadDeps struct {
	Servicio ContabilidadServicio
	Detalles DetallesCorrida
	Lector   LectorAsientos
	// UsuarioID extrae el id del usuario autenticado del contexto (el
	// *security.Payload que deja middleware.AuthPASETO). Se inyecta para no
	// acoplar este archivo a la forma del Payload.
	UsuarioID func(c *gin.Context) (int64, bool)
}

type ContabilidadHandler struct {
	d ContabilidadDeps
}

func NewContabilidadHandler(d ContabilidadDeps) *ContabilidadHandler {
	return &ContabilidadHandler{d: d}
}

// ---------------------------------------------------------------------------
// DTOs de respuesta (montos como string con 2 decimales: nunca float).
// ---------------------------------------------------------------------------

type asientoLineaDTO struct {
	ID                       int64  `json:"id"`
	CuentaContableID         int64  `json:"cuenta_contable_id"`
	ProgramaPresupuestarioID int64  `json:"programa_presupuestario_id"`
	Proyecto                 string `json:"proyecto"`
	EmpleadoID               *int64 `json:"empleado_id"`
	ConceptoSalarialID       *int64 `json:"concepto_salarial_id"`
	MontoDebe                string `json:"monto_debe"`
	MontoHaber               string `json:"monto_haber"`
}

type asientoDTO struct {
	ID                int64             `json:"id"`
	PeriodoPlanillaID int64             `json:"periodo_planilla_id"`
	TipoAsientoID     int64             `json:"tipo_asiento_id"`
	TipoAsiento       string            `json:"tipo_asiento,omitempty"`
	NumeroAsiento     string            `json:"numero_asiento"`
	FechaGeneracion   time.Time         `json:"fecha_generacion"`
	Estado            string            `json:"estado"`
	TotalDebe         string            `json:"total_debe"`
	TotalHaber        string            `json:"total_haber"`
	Lineas            []asientoLineaDTO `json:"lineas"`
}

func aAsientoDTO(a contabilidad.AsientoContable) asientoDTO {
	lineas := make([]asientoLineaDTO, 0, len(a.Lineas))
	for _, l := range a.Lineas {
		lineas = append(lineas, asientoLineaDTO{
			ID:                       l.ID,
			CuentaContableID:         l.CuentaContableID,
			ProgramaPresupuestarioID: l.ProgramaPresupuestarioID,
			Proyecto:                 l.Proyecto,
			EmpleadoID:               l.EmpleadoID,
			ConceptoSalarialID:       l.ConceptoSalarialID,
			MontoDebe:                l.MontoDebe.StringFixed(2),
			MontoHaber:               l.MontoHaber.StringFixed(2),
		})
	}
	return asientoDTO{
		ID:                a.ID,
		PeriodoPlanillaID: a.PeriodoPlanillaID,
		TipoAsientoID:     a.TipoAsientoID,
		TipoAsiento:       a.TipoAsientoCodigo,
		NumeroAsiento:     a.NumeroAsiento,
		FechaGeneracion:   a.FechaGeneracion,
		Estado:            a.Estado,
		TotalDebe:         a.TotalDebe().StringFixed(2),
		TotalHaber:        a.TotalHaber().StringFixed(2),
		Lineas:            lineas,
	}
}

func aAsientosDTO(as []contabilidad.AsientoContable) []asientoDTO {
	out := make([]asientoDTO, 0, len(as))
	for _, a := range as {
		out = append(out, aAsientoDTO(a))
	}
	return out
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// Generar: POST /planillas/:id/contabilizar
func (h *ContabilidadHandler) Generar(c *gin.Context) {
	periodoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || periodoID <= 0 {
		contabResponder(c, http.StatusBadRequest, "id_invalido", "el id del período debe ser un entero positivo", nil)
		return
	}
	usuarioID, ok := h.d.UsuarioID(c)
	if !ok {
		contabResponder(c, http.StatusUnauthorized, "sin_sesion", "no se pudo identificar al usuario autenticado", nil)
		return
	}

	detalles, err := h.d.Detalles.DetallesParaContabilizar(c.Request.Context(), periodoID)
	if err != nil {
		contabResponderError(c, err)
		return
	}

	asientos, err := h.d.Servicio.ContabilizarPeriodo(c.Request.Context(), periodoID, usuarioID, detalles)
	if err != nil {
		contabResponderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": aAsientosDTO(asientos)})
}

// Listar: GET /asientos-contables?periodo_id=&tipo=
func (h *ContabilidadHandler) Listar(c *gin.Context) {
	var filtro contabilidad.FiltroAsientos

	if raw := strings.TrimSpace(c.Query("periodo_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			contabResponder(c, http.StatusBadRequest, "periodo_id_invalido", "periodo_id debe ser un entero positivo", nil)
			return
		}
		filtro.PeriodoID = &id
	}
	filtro.TipoCodigo = strings.TrimSpace(c.Query("tipo"))

	if filtro.PeriodoID == nil && filtro.TipoCodigo == "" {
		contabResponder(c, http.StatusBadRequest, "filtro_requerido", "indique al menos periodo_id o tipo", nil)
		return
	}

	asientos, err := h.d.Lector.ListarAsientos(c.Request.Context(), filtro)
	if err != nil {
		contabResponderError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": aAsientosDTO(asientos)})
}

// ---------------------------------------------------------------------------
// Traducción de errores
// ---------------------------------------------------------------------------

func contabResponder(c *gin.Context, status int, codigo, mensaje string, detalle any) {
	body := gin.H{"codigo": codigo, "mensaje": mensaje}
	if detalle != nil {
		body["detalle"] = detalle
	}
	c.AbortWithStatusJSON(status, gin.H{"error": body})
}

// contabResponderError mapea los errores centinela de contabilidad a HTTP.
// Los errores no reconocidos responden 500 genérico (sin filtrar el mensaje
// interno al cliente).
func contabResponderError(c *gin.Context, err error) {
	var desc *contabilidad.DescuadreError
	switch {
	case errors.As(err, &desc):
		contabResponder(c, http.StatusUnprocessableEntity, "asiento_descuadrado",
			"la partida doble no cuadra; no se persistió ningún asiento",
			gin.H{
				"alcance":     desc.Alcance,
				"total_debe":  desc.TotalDebe.StringFixed(2),
				"total_haber": desc.TotalHaber.StringFixed(2),
			})
	case errors.Is(err, contabilidad.ErrPeriodoNoExiste):
		contabResponder(c, http.StatusNotFound, "periodo_no_existe", err.Error(), nil)
	case errors.Is(err, contabilidad.ErrDetalleInconsistente):
		contabResponder(c, http.StatusUnprocessableEntity, "detalle_inconsistente", err.Error(), nil)
	case errors.Is(err, contabilidad.ErrMapeoFaltante):
		contabResponder(c, http.StatusUnprocessableEntity, "mapeo_faltante", err.Error(), nil)
	case errors.Is(err, contabilidad.ErrMontoNegativoOInvalido):
		contabResponder(c, http.StatusUnprocessableEntity, "monto_invalido", err.Error(), nil)
	case errors.Is(err, contabilidad.ErrSinDetalles):
		contabResponder(c, http.StatusUnprocessableEntity, "sin_detalles", err.Error(), nil)
	case errors.Is(err, contabilidad.ErrPeriodoEstadoInvalido):
		contabResponder(c, http.StatusConflict, "periodo_estado_invalido", err.Error(), nil)
	case errors.Is(err, contabilidad.ErrAsientosYaGenerados):
		contabResponder(c, http.StatusConflict, "asientos_ya_generados", err.Error(), nil)
	default:
		// ErrConfigInvalida y cualquier error de infraestructura caen aquí.
		// El detalle va al log (el cliente solo recibe el mensaje genérico).
		slog.Error("contabilidad: error no mapeado", slog.String("error", err.Error()))
		contabResponder(c, http.StatusInternalServerError, "error_interno", "error interno al procesar la contabilización", nil)
	}
}
