package asistencia

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/paginacion"
)

// Handler expone las rutas HTTP de turnos, calendario y asistencia.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registra las rutas del módulo, cada una con su permiso.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	turnos := rg.Group("/turnos")
	turnos.POST("", middleware.RequirePermission(authz.PermCalendarioAdministrar), h.crearTurno)
	turnos.GET("", middleware.RequirePermission(authz.PermAsistenciaVer), h.listarTurnos)
	turnos.PUT("/:id", middleware.RequirePermission(authz.PermCalendarioAdministrar), h.actualizarTurno)
	turnos.POST("/asignar", middleware.RequirePermission(authz.PermCalendarioAdministrar), h.asignarTurno)

	calendario := rg.Group("/calendario")
	calendario.POST("/generar/:anio", middleware.RequirePermission(authz.PermCalendarioAdministrar), h.generarCalendario)
	calendario.GET("", middleware.RequirePermission(authz.PermAsistenciaVer), h.listarCalendario)
	calendario.GET("/feriados-catalogo", middleware.RequirePermission(authz.PermAsistenciaVer), h.listarFeriadosCatalogo)
	calendario.POST("/feriados-catalogo", middleware.RequirePermission(authz.PermCalendarioAdministrar), h.crearFeriadoCatalogo)
	calendario.PUT("/feriados-catalogo/:id", middleware.RequirePermission(authz.PermCalendarioAdministrar), h.actualizarFeriadoCatalogo)

	asistenciaGrp := rg.Group("/asistencia")
	asistenciaGrp.POST("", middleware.RequirePermission(authz.PermAsistenciaCrear), h.registrarAsistencia)
	asistenciaGrp.GET("/empleado/:empleado_id", middleware.RequirePermission(authz.PermAsistenciaVer), h.listarAsistenciaEmpleado)

	incapacidades := rg.Group("/incapacidades")
	incapacidades.POST("", middleware.RequirePermission(authz.PermIncapacidadesGestionar), h.registrarIncapacidad)
	incapacidades.GET("/empleado/:empleado_id", middleware.RequirePermission(authz.PermIncapacidadesGestionar), h.listarIncapacidadesEmpleado)
}

// responderError traduce un error de Service a respuesta HTTP. Los errores de
// dominio conocidos se mapean aquí (404/409/400/401); todo lo demás va a
// middleware.FailFromError (que no filtra detalles internos al cliente).
// Es el único punto que decide el código HTTP: el llamador siempre hace
// `return` justo después.
func responderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTurnoNoEncontrado), errors.Is(err, ErrFeriadoCatalogoNoEncontrado):
		middleware.Fail(c, http.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, ErrTurnoConAsignacionesVigentes):
		middleware.Fail(c, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, ErrRegistroAsistenciaInvalido),
		errors.Is(err, ErrTurnoInvalido),
		errors.Is(err, ErrAnioFueraDeRango):
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	case errors.Is(err, ErrFranjaNocturnaNoVigente),
		errors.Is(err, ErrCalendarioNoGenerado),
		errors.Is(err, ErrSinJornadaContrato),
		errors.Is(err, ErrFeriadoCatalogoInvalido),
		errors.Is(err, ErrCatalogoFeriadosVacio),
		errors.Is(err, ErrDiasLaboralesNoConfigurados),
		errors.Is(err, ErrDiasLaboralesInvalidos):
		middleware.Fail(c, http.StatusUnprocessableEntity, "unprocessable_entity", err.Error(), nil)
	case errors.Is(err, db.ErrNilUserID):
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "usuario no autenticado", nil)
	default:
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "Error interno del servidor", err)
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

type crearTurnoRequest struct {
	Codigo                    string `json:"codigo" binding:"required,max=20"`
	Nombre                    string `json:"nombre" binding:"required,max=100"`
	HoraEntrada               string `json:"hora_entrada" binding:"required"`
	HoraSalida                string `json:"hora_salida" binding:"required"`
	ToleranciaTardanzaMinutos *int   `json:"tolerancia_tardanza_minutos" binding:"required"`
}

func (h *Handler) crearTurno(c *gin.Context) {
	var req crearTurnoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, err)
		return
	}

	horaEntrada, err1 := parseHora(req.HoraEntrada)
	horaSalida, err2 := parseHora(req.HoraSalida)
	if err1 != nil || err2 != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "hora_entrada/hora_salida deben tener formato HH:MM", nil)
		return
	}

	turno, err := h.service.CrearTurno(c.Request.Context(), CrearTurnoInput{
		Codigo:                    req.Codigo,
		Nombre:                    req.Nombre,
		HoraEntrada:               horaEntrada,
		HoraSalida:                horaSalida,
		ToleranciaTardanzaMinutos: *req.ToleranciaTardanzaMinutos,
	}, middleware.UsuarioID(c))
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, turno)
}

func (h *Handler) listarTurnos(c *gin.Context) {
	p := paginacion.Desde(c)
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
	c.JSON(http.StatusOK, gin.H{"data": turnos, "meta": paginacion.NuevaMeta(p, total)})
}

type actualizarTurnoRequest struct {
	Nombre                    string `json:"nombre" binding:"required,max=100"`
	HoraEntrada               string `json:"hora_entrada" binding:"required"`
	HoraSalida                string `json:"hora_salida" binding:"required"`
	ToleranciaTardanzaMinutos *int   `json:"tolerancia_tardanza_minutos" binding:"required"`
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
		middleware.RespondWithError(c, err)
		return
	}
	horaEntrada, err1 := parseHora(req.HoraEntrada)
	horaSalida, err2 := parseHora(req.HoraSalida)
	if err1 != nil || err2 != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "hora_entrada/hora_salida deben tener formato HH:MM", nil)
		return
	}

	turno, err := h.service.ActualizarTurno(c.Request.Context(), ActualizarTurnoInput{
		ID:                        id,
		Nombre:                    req.Nombre,
		HoraEntrada:               horaEntrada,
		HoraSalida:                horaSalida,
		ToleranciaTardanzaMinutos: *req.ToleranciaTardanzaMinutos,
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
		middleware.RespondWithError(c, err)
		return
	}
	fecha, err := time.Parse("2006-01-02", req.FechaEfectiva)
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "fecha_efectiva debe tener formato YYYY-MM-DD", nil)
		return
	}

	asignacion, err := h.service.AsignarTurnoEmpleado(c.Request.Context(), req.EmpleadoID, req.TurnoID, fecha, middleware.UsuarioID(c))
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, asignacion)
}

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
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "parámetros 'desde' y 'hasta' son requeridos (YYYY-MM-DD)", nil)
		return
	}
	dias, err := h.service.ListarCalendario(c.Request.Context(), desde, hasta)
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dias})
}

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
		middleware.RespondWithError(c, err)
		return
	}
	fecha, err := time.Parse("2006-01-02", req.Fecha)
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "fecha debe tener formato YYYY-MM-DD", nil)
		return
	}
	horaEntrada, err := parseHoraOpcional(req.HoraEntradaReal)
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "hora_entrada_real debe tener formato HH:MM", nil)
		return
	}
	horaSalida, err := parseHoraOpcional(req.HoraSalidaReal)
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "hora_salida_real debe tener formato HH:MM", nil)
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
	p := paginacion.Desde(c)
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
	c.JSON(http.StatusOK, gin.H{"data": registros, "meta": paginacion.NuevaMeta(p, total)})
}

type registrarIncapacidadRequest struct {
	EmpleadoID        int64  `json:"empleado_id" binding:"required"`
	EntidadEmisora    string `json:"entidad_emisora" binding:"required,max=20"`
	NumeroBoleta      string `json:"numero_boleta" binding:"required"`
	FechaInicio       string `json:"fecha_inicio" binding:"required"`
	FechaFin          string `json:"fecha_fin" binding:"required"`
	DiagnosticoCodigo string `json:"diagnostico_codigo"`
	Observaciones     string `json:"observaciones"`
}

func (h *Handler) registrarIncapacidad(c *gin.Context) {
	var req registrarIncapacidadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	fechaInicio, err1 := time.Parse("2006-01-02", req.FechaInicio)
	fechaFin, err2 := time.Parse("2006-01-02", req.FechaFin)
	if err1 != nil || err2 != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "fecha_inicio/fecha_fin deben tener formato YYYY-MM-DD", nil)
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
	p := paginacion.Desde(c)
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
