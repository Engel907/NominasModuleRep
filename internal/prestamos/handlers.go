package prestamos

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/db"
)

// Handler agrupa los endpoints HTTP del módulo. Es deliberadamente
// delgado: valida forma de entrada, llama al Service, traduce errores
// de dominio a códigos HTTP y responde en el sobre JSON estandarizado.
//
// Convenciones (F05):
//   - Cada camino escribe EXACTAMENTE una respuesta y retorna de inmediato.
//   - Los parámetros de ruta se parsean con middleware.ParseIDParam; si son
//     inválidos el handler hace c.Error(err) + return y el ErrorHandler
//     global responde 400 (NO se escribe respuesta aquí).
//   - Nunca se envía al cliente err.Error() de errores internos (SQL, bind
//     del validador, etc.): se registra con slog y se responde un mensaje
//     genérico. Solo los errores de dominio tipados exponen su mensaje.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ---------------------------------------------------------------------
// Sobre de respuesta estandarizado
// ---------------------------------------------------------------------

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
	Error   *apiError   `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type paginacionMeta struct {
	Total  int64 `json:"total"`
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

func respondOK(c *gin.Context, status int, data interface{}) {
	c.JSON(status, apiResponse{Success: true, Data: data})
}

func respondOKPaginado(c *gin.Context, data interface{}, meta paginacionMeta) {
	c.JSON(http.StatusOK, apiResponse{Success: true, Data: data, Meta: meta})
}

func respondFail(c *gin.Context, status int, code, message string) {
	c.JSON(status, apiResponse{Success: false, Error: &apiError{Code: code, Message: message}})
}

// payloadInvalido responde 400 con un mensaje fijo o escrito por nosotros.
// Nunca recibe el error crudo de ShouldBindJSON (filtra nombres de structs,
// tags del validador y tipos Go).
func payloadInvalido(c *gin.Context, message string) {
	respondFail(c, http.StatusBadRequest, "PAYLOAD_INVALIDO", message)
}

const msgCuerpoInvalido = "el cuerpo de la solicitud es inválido o está incompleto"

// ---------------------------------------------------------------------
// Mapeo de errores de dominio a HTTP
// ---------------------------------------------------------------------

type mapeoError struct {
	err    error
	status int
	code   string
}

var mapeoErrores = []mapeoError{
	// 422 — el dato enviado no cumple una regla de validación.
	{ErrMontoInvalido, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrMontoPrecision, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrCuotaExcedeMonto, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrCuotasInvalidas, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrCuotasIncoherentes, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrFechasInvalidas, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrEmbargoSinBase, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrEmbargoBaseDuplicada, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrPorcentajeInvalido, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrFechaFinRequerida, http.StatusUnprocessableEntity, "VALIDACION"},
	{ErrAbonoExcedeSaldo, http.StatusUnprocessableEntity, "VALIDACION"},

	// 422 — el cuerpo referencia una entidad que no existe.
	{ErrEmpleadoNoEncontrado, http.StatusUnprocessableEntity, "REFERENCIA_INVALIDA"},
	{ErrTerceroNoEncontrado, http.StatusUnprocessableEntity, "REFERENCIA_INVALIDA"},
	{ErrTipoEmbargoNoEncontrado, http.StatusUnprocessableEntity, "REFERENCIA_INVALIDA"},

	// 404 — el recurso de la ruta no existe.
	{ErrPrestamoNoEncontrado, http.StatusNotFound, "NO_ENCONTRADO"},
	{ErrEmbargoNoEncontrado, http.StatusNotFound, "NO_ENCONTRADO"},

	// 409 — el estado actual impide la operación.
	{ErrTransicionInvalida, http.StatusConflict, "ESTADO_INVALIDO"},
	{ErrPrestamoNoActivo, http.StatusConflict, "ESTADO_INVALIDO"},
	{ErrPrestamoYaCancelado, http.StatusConflict, "ESTADO_INVALIDO"},
	{ErrEmpleadoInactivo, http.StatusConflict, "ESTADO_INVALIDO"},
	{ErrEmpleadoSinContratoVigente, http.StatusConflict, "ESTADO_INVALIDO"},
	{ErrTipoEmbargoNoVigente, http.StatusConflict, "ESTADO_INVALIDO"},
	{ErrPrestamoDuplicado, http.StatusConflict, "DUPLICADO"},
}

// respondError centraliza el mapeo de errores de dominio a HTTP. Se responde
// con el mensaje del error de dominio CANÓNICO (m.err), no con err.Error(),
// para que un error envuelto con detalle interno nunca llegue al cliente.
// Todo lo no reconocido es 500 genérico y se registra en el log.
func respondError(c *gin.Context, err error) {
	for _, m := range mapeoErrores {
		if errors.Is(err, m.err) {
			respondFail(c, m.status, m.code, m.err.Error())
			return
		}
	}
	switch {
	case errors.Is(err, db.ErrNilUserID):
		respondFail(c, http.StatusUnauthorized, "NO_AUTENTICADO", "no se pudo identificar al usuario autenticado")
	case errors.Is(err, sql.ErrNoRows):
		respondFail(c, http.StatusNotFound, "NO_ENCONTRADO", "el recurso solicitado no existe")
	default:
		slog.ErrorContext(c.Request.Context(), "prestamos: error interno",
			slog.String("ruta", c.FullPath()),
			slog.String("error", err.Error()))
		respondFail(c, http.StatusInternalServerError, "ERROR_INTERNO", "ocurrió un error inesperado")
	}
}

// ---------------------------------------------------------------------
// Helpers de entrada
// ---------------------------------------------------------------------

// usuarioAutenticado lee el usuario_id que dejó AuthPASETO. Si no hay
// payload responde 401 y devuelve false (defensa en profundidad: la ruta
// debería estar tras AuthPASETO, que ya habría respondido 401).
func usuarioAutenticado(c *gin.Context) (int64, bool) {
	id, ok := middleware.UsuarioIDDesdeContexto(c)
	if !ok || id <= 0 {
		respondFail(c, http.StatusUnauthorized, "NO_AUTENTICADO", "no se pudo identificar al usuario autenticado")
		return 0, false
	}
	return id, true
}

// idParam parsea un parámetro de ruta como int64 > 0. Si es inválido
// registra el error con c.Error y devuelve false SIN escribir respuesta:
// el ErrorHandler lo traduce a 400.
func idParam(c *gin.Context, nombre string) (int64, bool) {
	id, err := middleware.ParseIDParam(c, nombre)
	if err != nil {
		_ = c.Error(err)
		return 0, false
	}
	return id, true
}

func paginacionFromQuery(c *gin.Context) (limit, offset int32) {
	l, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || l <= 0 || l > 100 {
		l = 20
	}
	o, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || o < 0 {
		o = 0
	}
	return int32(l), int32(o)
}

// parseFecha parsea YYYY-MM-DD; si falla responde 400 y devuelve false.
func parseFecha(c *gin.Context, campo, valor string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", valor)
	if err != nil {
		payloadInvalido(c, campo+" inválida, use YYYY-MM-DD")
		return time.Time{}, false
	}
	return t, true
}

// parseDecimal parsea un decimal; si falla responde 400 y devuelve false.
func parseDecimal(c *gin.Context, campo, valor string) (decimal.Decimal, bool) {
	d, err := decimal.NewFromString(valor)
	if err != nil {
		payloadInvalido(c, campo+" inválido")
		return decimal.Decimal{}, false
	}
	return d, true
}

// ---------------------------------------------------------------------
// TERCEROS
// ---------------------------------------------------------------------

// El esquema solo guarda (nombre, tipo); `identificacion` y `cuenta_bancaria`
// no existen en la tabla `terceros` y se eliminaron del contrato.
type crearTerceroRequest struct {
	Nombre      string `json:"nombre" binding:"required"`
	TipoTercero string `json:"tipo_tercero" binding:"required,oneof=juzgado entidad_financiera otro"`
}

func (h *Handler) CrearTercero(c *gin.Context) {
	userID, ok := usuarioAutenticado(c)
	if !ok {
		return
	}
	var req crearTerceroRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		payloadInvalido(c, msgCuerpoInvalido)
		return
	}
	id, err := h.svc.CrearTercero(c.Request.Context(), userID, CrearTerceroInput{
		Nombre: req.Nombre,
		Tipo:   req.TipoTercero,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) ListarTerceros(c *gin.Context) {
	limit, offset := paginacionFromQuery(c)
	items, total, err := h.svc.ListarTerceros(c.Request.Context(), c.Query("tipo"), c.Query("q"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOKPaginado(c, items, paginacionMeta{Total: total, Limit: limit, Offset: offset})
}

// ---------------------------------------------------------------------
// PRESTAMOS
// ---------------------------------------------------------------------

// `descripcion` se eliminó (no existe en `prestamos`). `tercero_id` es
// obligatorio (NOT NULL en el esquema). `cuotas_totales` es opcional: no se
// guarda, solo se contrasta contra monto_total / cuota_periodica.
type crearPrestamoRequest struct {
	EmpleadoID       int64   `json:"empleado_id" binding:"required"`
	TerceroID        int64   `json:"tercero_id" binding:"required"`
	MontoTotal       string  `json:"monto_total" binding:"required"`
	CuotaPeriodica   string  `json:"cuota_periodica" binding:"required"`
	CuotasTotales    *int32  `json:"cuotas_totales"`
	FechaInicio      string  `json:"fecha_inicio" binding:"required"` // YYYY-MM-DD
	FechaFinEstimada *string `json:"fecha_fin_estimada"`              // YYYY-MM-DD, opcional
}

func (h *Handler) CrearPrestamo(c *gin.Context) {
	userID, ok := usuarioAutenticado(c)
	if !ok {
		return
	}
	var req crearPrestamoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		payloadInvalido(c, msgCuerpoInvalido)
		return
	}
	montoTotal, ok := parseDecimal(c, "monto_total", req.MontoTotal)
	if !ok {
		return
	}
	cuota, ok := parseDecimal(c, "cuota_periodica", req.CuotaPeriodica)
	if !ok {
		return
	}
	fechaInicio, ok := parseFecha(c, "fecha_inicio", req.FechaInicio)
	if !ok {
		return
	}
	var fechaFin *time.Time
	if req.FechaFinEstimada != nil {
		t, ok := parseFecha(c, "fecha_fin_estimada", *req.FechaFinEstimada)
		if !ok {
			return
		}
		fechaFin = &t
	}

	id, err := h.svc.CrearPrestamo(c.Request.Context(), userID, CrearPrestamoInput{
		EmpleadoID:       req.EmpleadoID,
		TerceroID:        req.TerceroID,
		MontoTotal:       montoTotal,
		CuotaPeriodica:   cuota,
		CuotasTotales:    req.CuotasTotales,
		FechaInicio:      fechaInicio,
		FechaFinEstimada: fechaFin,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) ListarPrestamosPorEmpleado(c *gin.Context) {
	empleadoID, ok := idParam(c, "empleado_id")
	if !ok {
		return
	}
	limit, offset := paginacionFromQuery(c)
	items, total, err := h.svc.ListarPrestamosPorEmpleado(c.Request.Context(), empleadoID, c.Query("estado"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOKPaginado(c, items, paginacionMeta{Total: total, Limit: limit, Offset: offset})
}

type abonoPrestamoRequest struct {
	Monto string `json:"monto" binding:"required"`
}

func (h *Handler) AplicarAbonoPrestamo(c *gin.Context) {
	userID, ok := usuarioAutenticado(c)
	if !ok {
		return
	}
	prestamoID, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req abonoPrestamoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		payloadInvalido(c, msgCuerpoInvalido)
		return
	}
	monto, ok := parseDecimal(c, "monto", req.Monto)
	if !ok {
		return
	}
	if err := h.svc.AplicarAbono(c.Request.Context(), userID, prestamoID, monto); err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, http.StatusOK, gin.H{"id": prestamoID, "abono_aplicado": monto.String()})
}

func (h *Handler) CancelarPrestamo(c *gin.Context) {
	userID, ok := usuarioAutenticado(c)
	if !ok {
		return
	}
	prestamoID, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.CancelarPrestamo(c.Request.Context(), userID, prestamoID); err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, http.StatusOK, gin.H{"id": prestamoID, "estado": "cancelado"})
}

// ---------------------------------------------------------------------
// EMBARGOS
// ---------------------------------------------------------------------

type crearEmbargoRequest struct {
	EmpleadoID       int64   `json:"empleado_id" binding:"required"`
	TipoEmbargoID    int64   `json:"tipo_embargo_id" binding:"required"`
	TerceroID        int64   `json:"tercero_id" binding:"required"`
	NumeroExpediente *string `json:"numero_expediente"`
	Porcentaje       *string `json:"porcentaje"`
	MontoFijo        *string `json:"monto_fijo"`
	FechaInicio      string  `json:"fecha_inicio_vigencia" binding:"required"`
	FechaFin         *string `json:"fecha_fin_vigencia"`
}

func (h *Handler) CrearEmbargo(c *gin.Context) {
	userID, ok := usuarioAutenticado(c)
	if !ok {
		return
	}
	var req crearEmbargoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		payloadInvalido(c, msgCuerpoInvalido)
		return
	}
	fechaInicio, ok := parseFecha(c, "fecha_inicio_vigencia", req.FechaInicio)
	if !ok {
		return
	}
	var fechaFin *time.Time
	if req.FechaFin != nil {
		t, ok := parseFecha(c, "fecha_fin_vigencia", *req.FechaFin)
		if !ok {
			return
		}
		fechaFin = &t
	}
	var porcentaje, montoFijo *decimal.Decimal
	if req.Porcentaje != nil {
		d, ok := parseDecimal(c, "porcentaje", *req.Porcentaje)
		if !ok {
			return
		}
		porcentaje = &d
	}
	if req.MontoFijo != nil {
		d, ok := parseDecimal(c, "monto_fijo", *req.MontoFijo)
		if !ok {
			return
		}
		montoFijo = &d
	}

	id, err := h.svc.CrearEmbargo(c.Request.Context(), userID, CrearEmbargoInput{
		EmpleadoID:       req.EmpleadoID,
		TipoEmbargoID:    req.TipoEmbargoID,
		TerceroID:        req.TerceroID,
		NumeroExpediente: req.NumeroExpediente,
		Porcentaje:       porcentaje,
		MontoFijo:        montoFijo,
		FechaInicio:      fechaInicio,
		FechaFinVigencia: fechaFin,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) ListarEmbargosPorEmpleado(c *gin.Context) {
	empleadoID, ok := idParam(c, "empleado_id")
	if !ok {
		return
	}
	limit, offset := paginacionFromQuery(c)
	items, total, err := h.svc.ListarEmbargosPorEmpleado(c.Request.Context(), empleadoID, c.Query("estado"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOKPaginado(c, items, paginacionMeta{Total: total, Limit: limit, Offset: offset})
}

// ListarEmbargosActivosOrdenados expone, para consumo del motor de
// planilla u otras vistas administrativas, los embargos activos y vigentes
// de un empleado ordenados por prioridad.
func (h *Handler) ListarEmbargosActivosOrdenados(c *gin.Context) {
	empleadoID, ok := idParam(c, "empleado_id")
	if !ok {
		return
	}
	fecha := time.Now()
	if f := c.Query("fecha_corte"); f != "" {
		parsed, ok := parseFecha(c, "fecha_corte", f)
		if !ok {
			return
		}
		fecha = parsed
	}
	items, err := h.svc.EmbargosActivosOrdenados(c.Request.Context(), empleadoID, fecha)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, http.StatusOK, items)
}

// Estados del ENUM embargos.estado: activo | suspendido | finalizado.
// ('cancelado' no existe en el esquema; era el valor de la versión anterior.)
type cambiarEstadoEmbargoRequest struct {
	Estado   string  `json:"estado" binding:"required,oneof=activo suspendido finalizado"`
	FechaFin *string `json:"fecha_fin_vigencia"`
}

func (h *Handler) CambiarEstadoEmbargo(c *gin.Context) {
	userID, ok := usuarioAutenticado(c)
	if !ok {
		return
	}
	embargoID, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req cambiarEstadoEmbargoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		payloadInvalido(c, msgCuerpoInvalido)
		return
	}
	var fechaFin *time.Time
	if req.FechaFin != nil {
		t, ok := parseFecha(c, "fecha_fin_vigencia", *req.FechaFin)
		if !ok {
			return
		}
		fechaFin = &t
	}
	if err := h.svc.CambiarEstadoEmbargo(c.Request.Context(), userID, embargoID, req.Estado, fechaFin); err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, http.StatusOK, gin.H{"id": embargoID, "estado": req.Estado})
}
