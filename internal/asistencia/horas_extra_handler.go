package asistencia

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/paginacion"
)

// HorasExtraHandler expone /horas-extra.
type HorasExtraHandler struct{ svc *HorasExtraService }

func NewHorasExtraHandler(svc *HorasExtraService) *HorasExtraHandler {
	return &HorasExtraHandler{svc: svc}
}

func (h *HorasExtraHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/horas-extra")
	g.POST("/autorizaciones", middleware.RequirePermission(authz.PermAsistenciaCrear), h.solicitar)
	g.GET("/autorizaciones", middleware.RequirePermission(authz.PermAsistenciaVer), h.listar)
	g.POST("/autorizaciones/:id/autorizar", middleware.RequirePermission(authz.PermHorasExtraAutorizar), h.autorizar)
	g.POST("/autorizaciones/:id/rechazar", middleware.RequirePermission(authz.PermHorasExtraAutorizar), h.rechazar)
	g.GET("/candidatas", middleware.RequirePermission(authz.PermAsistenciaVer), h.candidatas)
}

type autorizacionResponse struct {
	ID               int64      `json:"id"`
	EmpleadoID       int64      `json:"empleado_id"`
	Fecha            string     `json:"fecha"`
	HorasAutorizadas string     `json:"horas_autorizadas"`
	TipoHoraExtraID  *int64     `json:"tipo_hora_extra_id"`
	Motivo           string     `json:"motivo"`
	Estado           string     `json:"estado"`
	SolicitadoPor    int64      `json:"solicitado_por"`
	FechaSolicitud   time.Time  `json:"fecha_solicitud"`
	ResueltoPor      *int64     `json:"resuelto_por"`
	FechaResolucion  *time.Time `json:"fecha_resolucion"`
	MotivoResolucion *string    `json:"motivo_resolucion"`
}

func aAutorizacionResponse(r dto.HoraExtraAutorizacionRow) autorizacionResponse {
	return autorizacionResponse{
		ID: r.ID, EmpleadoID: r.EmpleadoID, Fecha: r.Fecha.Format("2006-01-02"),
		HorasAutorizadas: r.HorasAutorizadas.StringFixed(2), TipoHoraExtraID: r.TipoHoraExtraID,
		Motivo: r.Motivo, Estado: r.Estado, SolicitadoPor: r.SolicitadoPor, FechaSolicitud: r.FechaSolicitud,
		ResueltoPor: r.ResueltoPor, FechaResolucion: r.FechaResolucion, MotivoResolucion: r.MotivoResolucion,
	}
}

// fallarHE responde los errores de dominio; el resto va al ErrorHandler global
// (que nunca expone el texto interno).
func fallarHE(c *gin.Context, err error) {
	var e *ErrorHorasExtra
	if errors.As(err, &e) {
		middleware.Fail(c, e.HTTP, e.Codigo, e.Mensaje, nil)
		return
	}
	_ = c.Error(err)
}

func invalido(c *gin.Context, mensaje string) {
	middleware.Fail(c, http.StatusBadRequest, "solicitud_invalida", mensaje, nil)
}

type solicitarHorasExtraRequest struct {
	EmpleadoID       int64  `json:"empleado_id" binding:"required"`
	Fecha            string `json:"fecha" binding:"required"`             // YYYY-MM-DD
	HorasAutorizadas string `json:"horas_autorizadas" binding:"required"` // texto decimal, ej. "3.50"
	TipoHoraExtraID  *int64 `json:"tipo_hora_extra_id"`
	Motivo           string `json:"motivo" binding:"required"`
}

func (h *HorasExtraHandler) solicitar(c *gin.Context) {
	usuarioID := middleware.UsuarioID(c)
	var req solicitarHorasExtraRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.EmpleadoID <= 0 {
		invalido(c, "cuerpo inválido: se requieren empleado_id, fecha, horas_autorizadas (texto decimal) y motivo")
		return
	}
	fecha, err := time.Parse("2006-01-02", req.Fecha)
	if err != nil {
		invalido(c, "fecha debe tener formato YYYY-MM-DD")
		return
	}
	horas, err := decimal.NewFromString(req.HorasAutorizadas)
	if err != nil {
		fallarHE(c, ErrHorasAutorizadasInvalida)
		return
	}
	r, err := h.svc.Solicitar(c.Request.Context(), usuarioID, SolicitudHorasExtra{
		EmpleadoID: req.EmpleadoID, Fecha: fecha, HorasAutorizadas: horas, TipoHoraExtraID: req.TipoHoraExtraID, Motivo: req.Motivo,
	})
	if err != nil {
		fallarHE(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": aAutorizacionResponse(r)})
}

func (h *HorasExtraHandler) autorizar(c *gin.Context) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	r, err := h.svc.Autorizar(c.Request.Context(), middleware.UsuarioID(c), id)
	if err != nil {
		fallarHE(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": aAutorizacionResponse(r)})
}

type rechazarRequest struct {
	Motivo string `json:"motivo" binding:"required"`
}

func (h *HorasExtraHandler) rechazar(c *gin.Context) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req rechazarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fallarHE(c, ErrMotivoRequerido)
		return
	}
	r, err := h.svc.Rechazar(c.Request.Context(), middleware.UsuarioID(c), id, req.Motivo)
	if err != nil {
		fallarHE(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": aAutorizacionResponse(r)})
}

func fechaQueryHE(c *gin.Context, nombre string) (*time.Time, bool) {
	v := c.Query(nombre)
	if v == "" {
		return nil, true
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		invalido(c, nombre+" debe tener formato YYYY-MM-DD")
		return nil, false
	}
	return &t, true
}

func empleadoQueryHE(c *gin.Context) (*int64, bool) {
	v := c.Query("empleado_id")
	if v == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		invalido(c, "empleado_id inválido")
		return nil, false
	}
	return &id, true
}

func (h *HorasExtraHandler) listar(c *gin.Context) {
	var f dto.FiltroHorasExtraAutorizaciones
	var ok bool
	if f.EmpleadoID, ok = empleadoQueryHE(c); !ok {
		return
	}
	if v := c.Query("estado"); v != "" {
		if v != EstadoHEPendiente && v != EstadoHEAutorizada && v != EstadoHERechazada {
			invalido(c, "estado inválido (pendiente, autorizada, rechazada)")
			return
		}
		f.Estado = &v
	}
	if f.Desde, ok = fechaQueryHE(c, "desde"); !ok {
		return
	}
	if f.Hasta, ok = fechaQueryHE(c, "hasta"); !ok {
		return
	}
	pag := paginacion.Desde(c)
	filas, total, err := h.svc.Listar(c.Request.Context(), f, pag)
	if err != nil {
		fallarHE(c, err)
		return
	}
	datos := make([]autorizacionResponse, len(filas))
	for i, r := range filas {
		datos[i] = aAutorizacionResponse(r)
	}
	c.JSON(http.StatusOK, gin.H{"data": datos, "meta": paginacion.NuevaMeta(pag, total)})
}

func (h *HorasExtraHandler) candidatas(c *gin.Context) {
	empleadoID, ok := empleadoQueryHE(c)
	if !ok {
		return
	}
	desde, ok := fechaQueryHE(c, "desde")
	if !ok {
		return
	}
	hasta, ok := fechaQueryHE(c, "hasta")
	if !ok {
		return
	}
	if desde == nil || hasta == nil {
		fallarHE(c, ErrRangoFechasInvalido)
		return
	}
	filas, err := h.svc.Candidatas(c.Request.Context(), empleadoID, *desde, *hasta)
	if err != nil {
		fallarHE(c, err)
		return
	}
	type candidata struct {
		EmpleadoID       int64  `json:"empleado_id"`
		Fecha            string `json:"fecha"`
		HorasRegistradas string `json:"horas_registradas"`
	}
	datos := make([]candidata, len(filas))
	for i, r := range filas {
		datos[i] = candidata{r.EmpleadoID, r.Fecha.Format("2006-01-02"), r.HorasRegistradas.StringFixed(2)}
	}
	c.JSON(http.StatusOK, gin.H{"data": datos})
}
