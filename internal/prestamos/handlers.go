package prestamos

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/paginacion"
)

// Handler agrupa los endpoints HTTP del módulo. Es deliberadamente
// delgado: valida forma de entrada, llama al Service, traduce errores
// de dominio a códigos HTTP y responde en el sobre JSON estandarizado.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func respondOK(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{"data": data})
}

func respondOKPaginado(c *gin.Context, data interface{}, meta paginacion.Meta) {
	c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta})
}

func respondFail(c *gin.Context, status int, code, message string) {
	middleware.Fail(c, status, code, message, nil)
}

// payloadInvalido responde 400 con un mensaje fijo o escrito por nosotros.
// Nunca recibe el error crudo de ShouldBindJSON (filtra nombres de structs,
// tags del validador y tipos Go).
func payloadInvalido(c *gin.Context, message string) {
	respondFail(c, http.StatusBadRequest, "invalid_request", message)
}

const msgCuerpoInvalido = "el cuerpo de la solicitud es inválido o está incompleto"

type mapeoError struct {
	err    error
	status int
	code   string
}

var mapeoErrores = []mapeoError{
	{ErrMontoInvalido, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrMontoPrecision, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrCuotaExcedeMonto, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrCuotasInvalidas, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrCuotasIncoherentes, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrFechasInvalidas, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrEmbargoSinBase, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrEmbargoBaseDuplicada, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrPorcentajeInvalido, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrFechaFinRequerida, http.StatusUnprocessableEntity, "unprocessable_entity"},
	{ErrAbonoExcedeSaldo, http.StatusUnprocessableEntity, "unprocessable_entity"},

	{ErrEmpleadoNoEncontrado, http.StatusUnprocessableEntity, "referencia_invalida"},
	{ErrTerceroNoEncontrado, http.StatusUnprocessableEntity, "referencia_invalida"},
	{ErrTipoEmbargoNoEncontrado, http.StatusUnprocessableEntity, "referencia_invalida"},

	{ErrPrestamoNoEncontrado, http.StatusNotFound, "not_found"},
	{ErrEmbargoNoEncontrado, http.StatusNotFound, "not_found"},

	{ErrTransicionInvalida, http.StatusConflict, "conflict"},
	{ErrPrestamoNoActivo, http.StatusConflict, "conflict"},
	{ErrPrestamoYaCancelado, http.StatusConflict, "conflict"},
	{ErrEmpleadoInactivo, http.StatusConflict, "conflict"},
	{ErrEmpleadoSinContratoVigente, http.StatusConflict, "conflict"},
	{ErrTipoEmbargoNoVigente, http.StatusConflict, "conflict"},
	{ErrPrestamoDuplicado, http.StatusConflict, "duplicado"},
}

// respondError centraliza el mapeo de errores de dominio a HTTP. Se responde
// con el mensaje del error de dominio CANÓNICO (m.err), no con err.Error,
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
		respondFail(c, http.StatusUnauthorized, "unauthorized", "no se pudo identificar al usuario autenticado")
	case errors.Is(err, sql.ErrNoRows):
		respondFail(c, http.StatusNotFound, "not_found", "el recurso solicitado no existe")
	default:
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "ocurrió un error inesperado", err)
	}
}

// usuarioAutenticado lee el usuario_id que dejó AuthPASETO. Si no hay
// payload responde 401 y devuelve false (defensa en profundidad: la ruta
// debería estar tras AuthPASETO, que ya habría respondido 401).
func usuarioAutenticado(c *gin.Context) (int64, bool) {
	id, ok := middleware.UsuarioIDDesdeContexto(c)
	if !ok || id <= 0 {
		respondFail(c, http.StatusUnauthorized, "unauthorized", "no se pudo identificar al usuario autenticado")
		return 0, false
	}
	return id, true
}

// idParam parsea un parámetro de ruta como int64 > 0. Si es inválido
// registra el error con c.Error y devuelve false sin escribir respuesta:
// el ErrorHandler lo traduce a 400.
func idParam(c *gin.Context, nombre string) (int64, bool) {
	id, err := middleware.ParseIDParam(c, nombre)
	if err != nil {
		_ = c.Error(err)
		return 0, false
	}
	return id, true
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

// El esquema solo guarda (nombre, tipo); `identificacion` y `cuenta_bancaria`
// no existen en la tabla `terceros` y se eliminaron del contrato.
type crearTerceroRequest struct {
	Nombre      string `json:"nombre" binding:"required"`
	TipoTercero string `json:"tipo_tercero" binding:"required,oneof=juzgado entidad_financiera asociacion_solidarista sindicato aseguradora cooperativa otro"`
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
	pag := paginacion.Desde(c)
	items, total, err := h.svc.ListarTerceros(c.Request.Context(), c.Query("tipo"), c.Query("q"), pag.Limit32(), pag.Offset32())
	if err != nil {
		respondError(c, err)
		return
	}
	respondOKPaginado(c, items, paginacion.NuevaMeta(pag, total))
}

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
	pag := paginacion.Desde(c)
	items, total, err := h.svc.ListarPrestamosPorEmpleado(c.Request.Context(), empleadoID, c.Query("estado"), pag.Limit32(), pag.Offset32())
	if err != nil {
		respondError(c, err)
		return
	}
	respondOKPaginado(c, items, paginacion.NuevaMeta(pag, total))
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

type crearEmbargoRequest struct {
	EmpleadoID       int64   `json:"empleado_id" binding:"required"`
	TipoEmbargoID    int64   `json:"tipo_embargo_id" binding:"required"`
	TerceroID        int64   `json:"tercero_id" binding:"required"`
	NumeroExpediente *string `json:"numero_expediente"`
	Porcentaje       *string `json:"porcentaje"`
	MontoFijo        *string `json:"monto_fijo"`
	TopeMensual      *string `json:"tope_mensual"` // opcional
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
	var topeMensual *decimal.Decimal
	if req.TopeMensual != nil {
		d, ok := parseDecimal(c, "tope_mensual", *req.TopeMensual)
		if !ok {
			return
		}
		topeMensual = &d
	}

	id, err := h.svc.CrearEmbargo(c.Request.Context(), userID, CrearEmbargoInput{
		EmpleadoID:       req.EmpleadoID,
		TipoEmbargoID:    req.TipoEmbargoID,
		TerceroID:        req.TerceroID,
		NumeroExpediente: req.NumeroExpediente,
		Porcentaje:       porcentaje,
		MontoFijo:        montoFijo,
		TopeMensual:      topeMensual,
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
	pag := paginacion.Desde(c)
	items, total, err := h.svc.ListarEmbargosPorEmpleado(c.Request.Context(), empleadoID, c.Query("estado"), pag.Limit32(), pag.Offset32())
	if err != nil {
		respondError(c, err)
		return
	}
	respondOKPaginado(c, items, paginacion.NuevaMeta(pag, total))
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
