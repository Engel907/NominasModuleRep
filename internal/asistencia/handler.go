package asistencia

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/pagination"
)

// Handler expone las rutas HTTP de Fase 4 para turnos, calendario y asistencia.
// Deliberadamente delgado: toda la lógica vive en Service; el handler solo
// parsea input, valida forma y traduce errores a códigos HTTP. Cada rama
// responde UNA sola vez y hace return inmediatamente después.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes cablea las rutas bajo el grupo /api/v1 ya existente en
// cmd/api. La autorización usa RequirePermission con los códigos sembrados en
// la migración 000003 (asistencia.*, calendario.administrar,
// incapacidades.gestionar). Los códigos de permiso NO se tocan en esta fase (F12).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	turnos := rg.Group("/turnos")
	turnos.POST("", middleware.RequirePermission("calendario.administrar"), h.crearTurno)
	turnos.GET("", middleware.RequirePermission("asistencia.ver"), h.listarTurnos)
	turnos.PUT("/:id", middleware.RequirePermission("calendario.administrar"), h.actualizarTurno)
	turnos.POST("/asignar", middleware.RequirePermission("calendario.administrar"), h.asignarTurno)

	calendario := rg.Group("/calendario")
	calendario.POST("/generar/:anio", middleware.RequirePermission("calendario.administrar"), h.generarCalendario)
	calendario.GET("", middleware.RequirePermission("asistencia.ver"), h.listarCalendario)

	asistenciaGrp := rg.Group("/asistencia")
	asistenciaGrp.POST("", middleware.RequirePermission("asistencia.crear"), h.registrarAsistencia)
	asistenciaGrp.GET("/empleado/:empleado_id", middleware.RequirePermission("asistencia.ver"), h.listarAsistenciaEmpleado)

	incapacidades := rg.Group("/incapacidades")
	incapacidades.POST("", middleware.RequirePermission("incapacidades.gestionar"), h.registrarIncapacidad)
	incapacidades.GET("/empleado/:empleado_id", middleware.RequirePermission("incapacidades.gestionar"), h.listarIncapacidadesEmpleado)
}

// responderError traduce un error de Service a respuesta HTTP. Los errores de
// dominio conocidos se mapean aquí (404/409/400/401); todo lo demás va a
// middleware.FailFromError (que no filtra detalles internos al cliente).
// Es el único punto que decide el código HTTP: el llamador siempre hace
// `return` justo después.
func responderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTurnoNoEncontrado):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrTurnoConAsignacionesVigentes):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrRegistroAsistenciaInvalido),
		errors.Is(err, ErrTurnoInvalido),
		errors.Is(err, ErrAnioFueraDeRango):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, db.ErrNilUserID):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario no autenticado"})
	default:
		middleware.FailFromError(c, err)
	}
}

// idParam lee un parámetro de ruta como ID int64 > 0. Si es inválido deja el
// error en el contexto (ErrorHandler lo convierte en 400) y devuelve false:
// el llamador debe hacer return sin escribir nada más.
func idParam(c *gin.Context, nombre string) (int64, bool) {
	id, err := middleware.ParseIDParam(c, nombre)
	if err != nil {
		_ = c.Error(err)
		return 0, false
	}
	return id, true
}

// parseHora parsea "HH:MM".
func parseHora(s string) (time.Time, error) { return time.Parse("15:04", s) }

// parseHoraOpcional parsea un "HH:MM" opcional (nil → nil).
func parseHoraOpcional(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	t, err := parseHora(*s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// --- Turnos ------------------------------------------------------------

type crearTurnoRequest struct {
	Codigo                    string `json:"codigo" binding:"required,max=20"`
	Nombre                    string `json:"nombre" binding:"required,max=100"`
	HoraEntrada               string `json:"hora_entrada" binding:"required"` // "HH:MM"
	HoraSalida                string `json:"hora_salida" binding:"required"`
	DiasLaborables            string `json:"dias_laborables" binding:"required"` // "1,2,3,4,5"
	ToleranciaTardanzaMinutos int16  `json:"tolerancia_tardanza_minutos"`
}

func (h *Handler) crearTurno(c *gin.Context) {
	var req crearTurnoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	horaEntrada, err1 := parseHora(req.HoraEntrada)
	horaSalida, err2 := parseHora(req.HoraSalida)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hora_entrada/hora_salida deben tener formato HH:MM"})
		return
	}

	turno, err := h.service.CrearTurno(c.Request.Context(), CrearTurnoInput{
		Codigo:                    req.Codigo,
		Nombre:                    req.Nombre,
		HoraEntrada:               horaEntrada,
		HoraSalida:                horaSalida,
		DiasLaborables:            req.DiasLaborables,
		ToleranciaTardanzaMinutos: req.ToleranciaTardanzaMinutos,
	}, middleware.UsuarioID(c))
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, turno)
}

func (h *Handler) listarTurnos(c *gin.Context) {
	p := pagination.Desde(c)
	filtro := turnosFiltro(c, p)
	turnos, err := h.service.ListarTurnos(c.Request.Context(), filtro)
	if err != nil {
		responderError(c, err)
		return
	}
	total, err := h.service.ContarTurnos(c.Request.Context(), filtro.Activo)
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": turnos, "meta": pagination.NuevaMeta(p, total)})
}

type actualizarTurnoRequest struct {
	Nombre      string `json:"nombre" binding:"required,max=100"`
	HoraEntrada string `json:"hora_entrada" binding:"required"` // "HH:MM"
	HoraSalida  string `json:"hora_salida" binding:"required"`
}

// actualizarTurno: PUT /turnos/:id. 404 si el turno no existe; 409 si se
// intenta cambiar el horario de un turno con asignaciones vigentes (ver
// Service.ActualizarTurno). El código del turno no es modificable.
func (h *Handler) actualizarTurno(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req actualizarTurnoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	horaEntrada, err1 := parseHora(req.HoraEntrada)
	horaSalida, err2 := parseHora(req.HoraSalida)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hora_entrada/hora_salida deben tener formato HH:MM"})
		return
	}

	turno, err := h.service.ActualizarTurno(c.Request.Context(), ActualizarTurnoInput{
		ID:          id,
		Nombre:      req.Nombre,
		HoraEntrada: horaEntrada,
		HoraSalida:  horaSalida,
	}, middleware.UsuarioID(c))
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, turno)
}

type asignarTurnoRequest struct {
	EmpleadoID    int64  `json:"empleado_id" binding:"required"`
	TurnoID       int64  `json:"turno_id" binding:"required"`
	FechaEfectiva string `json:"fecha_efectiva" binding:"required"` // "YYYY-MM-DD"
}

func (h *Handler) asignarTurno(c *gin.Context) {
	var req asignarTurnoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fecha, err := time.Parse("2006-01-02", req.FechaEfectiva)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fecha_efectiva debe tener formato YYYY-MM-DD"})
		return
	}

	asignacion, err := h.service.AsignarTurnoEmpleado(c.Request.Context(), req.EmpleadoID, req.TurnoID, fecha, middleware.UsuarioID(c))
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, asignacion)
}

// --- Calendario ----------------------------------------------------------

func (h *Handler) generarCalendario(c *gin.Context) {
	anio, ok := idParam(c, "anio")
	if !ok {
		return
	}
	if err := h.service.GenerarCalendarioAnual(c.Request.Context(), int(anio), middleware.UsuarioID(c)); err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "calendario generado", "anio": anio})
}

func (h *Handler) listarCalendario(c *gin.Context) {
	desde, err1 := time.Parse("2006-01-02", c.Query("desde"))
	hasta, err2 := time.Parse("2006-01-02", c.Query("hasta"))
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parámetros 'desde' y 'hasta' son requeridos (YYYY-MM-DD)"})
		return
	}
	dias, err := h.service.ListarCalendario(c.Request.Context(), desde, hasta)
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dias})
}

// --- Asistencia ------------------------------------------------------------

type registrarAsistenciaRequest struct {
	EmpleadoID      int64   `json:"empleado_id" binding:"required"`
	Fecha           string  `json:"fecha" binding:"required"` // YYYY-MM-DD
	HoraEntradaReal *string `json:"hora_entrada_real"`
	HoraSalidaReal  *string `json:"hora_salida_real"`
	Estado          string  `json:"estado" binding:"required,oneof=presente ausente_justificada ausente_injustificada incapacidad vacaciones feriado dia_libre"`
	TipoAusenciaID  *int64  `json:"tipo_ausencia_id"`
	IncapacidadID   *int64  `json:"incapacidad_id"`
	Observaciones   string  `json:"observaciones"`
}

func (h *Handler) registrarAsistencia(c *gin.Context) {
	var req registrarAsistenciaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fecha, err := time.Parse("2006-01-02", req.Fecha)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fecha debe tener formato YYYY-MM-DD"})
		return
	}
	horaEntrada, err := parseHoraOpcional(req.HoraEntradaReal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hora_entrada_real debe tener formato HH:MM"})
		return
	}
	horaSalida, err := parseHoraOpcional(req.HoraSalidaReal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hora_salida_real debe tener formato HH:MM"})
		return
	}

	registro, err := h.service.RegistrarAsistencia(c.Request.Context(), RegistrarAsistenciaInput{
		EmpleadoID:      req.EmpleadoID,
		Fecha:           fecha,
		HoraEntradaReal: horaEntrada,
		HoraSalidaReal:  horaSalida,
		Estado:          req.Estado,
		TipoAusenciaID:  req.TipoAusenciaID,
		IncapacidadID:   req.IncapacidadID,
		Observaciones:   req.Observaciones,
		CreadoPor:       middleware.UsuarioID(c),
	})
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, registro)
}

func (h *Handler) listarAsistenciaEmpleado(c *gin.Context) {
	empleadoID, ok := idParam(c, "empleado_id")
	if !ok {
		return
	}
	p := pagination.Desde(c)
	registros, err := h.service.ListarAsistenciaEmpleado(c.Request.Context(), asistenciaFiltro(c, empleadoID, p))
	if err != nil {
		responderError(c, err)
		return
	}
	total, err := h.service.ContarAsistenciaEmpleado(c.Request.Context(), asistenciaFiltroConteo(c, empleadoID))
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": registros, "meta": pagination.NuevaMeta(p, total)})
}

// --- Incapacidades -----------------------------------------------------

type registrarIncapacidadRequest struct {
	EmpleadoID        int64  `json:"empleado_id" binding:"required"`
	EntidadEmisora    string `json:"entidad_emisora" binding:"required,oneof=CCSS INS"`
	NumeroBoleta      string `json:"numero_boleta" binding:"required"`
	FechaInicio       string `json:"fecha_inicio" binding:"required"`
	FechaFin          string `json:"fecha_fin" binding:"required"`
	DiagnosticoCodigo string `json:"diagnostico_codigo"`
	Observaciones     string `json:"observaciones"`
}

func (h *Handler) registrarIncapacidad(c *gin.Context) {
	var req registrarIncapacidadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fechaInicio, err1 := time.Parse("2006-01-02", req.FechaInicio)
	fechaFin, err2 := time.Parse("2006-01-02", req.FechaFin)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fecha_inicio/fecha_fin deben tener formato YYYY-MM-DD"})
		return
	}

	incapacidad, err := h.service.RegistrarIncapacidad(c.Request.Context(), RegistrarIncapacidadInput{
		EmpleadoID:        req.EmpleadoID,
		EntidadEmisora:    req.EntidadEmisora,
		NumeroBoleta:      req.NumeroBoleta,
		FechaInicio:       fechaInicio,
		FechaFin:          fechaFin,
		DiagnosticoCodigo: req.DiagnosticoCodigo,
		Observaciones:     req.Observaciones,
		CreadoPor:         middleware.UsuarioID(c),
	})
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, incapacidad)
}

func (h *Handler) listarIncapacidadesEmpleado(c *gin.Context) {
	empleadoID, ok := idParam(c, "empleado_id")
	if !ok {
		return
	}
	p := pagination.Desde(c)
	var entidadPtr *string
	if entidad := c.Query("entidad_emisora"); entidad != "" {
		entidadPtr = &entidad
	}

	incapacidades, err := h.service.ListarIncapacidadesEmpleado(c.Request.Context(), buildListarIncapacidadesParams(empleadoID, entidadPtr, p))
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": incapacidades})
}