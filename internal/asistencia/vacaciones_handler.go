package asistencia

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
)

// VacacionesHandler expone el flujo de vacaciones. Solo parsea input, valida
// forma y traduce errores a códigos HTTP; las reglas de negocio viven en
// VacacionesService.
type VacacionesHandler struct {
	service   *VacacionesService
	empleados middleware.EmpleadoIDResolver
}

func NewVacacionesHandler(service *VacacionesService, resolver ...middleware.EmpleadoIDResolver) *VacacionesHandler {
	h := &VacacionesHandler{service: service}
	if len(resolver) > 0 {
		h.empleados = resolver[0]
	}
	return h
}

// RegisterRoutes registra las rutas de vacaciones, cada una con su permiso.
func (h *VacacionesHandler) RegisterRoutes(rg *gin.RouterGroup) {
	vac := rg.Group("/vacaciones")
	vac.POST("/solicitudes", middleware.RequirePermission(authz.PermVacacionesSolicitar), h.solicitar)
	vac.POST("/solicitudes/:id/aprobar", middleware.RequirePermission(authz.PermVacacionesAprobar), h.aprobar)
	vac.POST("/solicitudes/:id/rechazar", middleware.RequirePermission(authz.PermVacacionesAprobar), h.rechazar)
	vac.POST("/solicitudes/:id/disfrute", middleware.RequirePermission(authz.PermVacacionesRegistrarDisfrute), h.registrarDisfrute)
	vac.GET("/empleados/:empleado_id/saldo", middleware.RequirePermission(authz.PermVacacionesVer), h.consultarSaldo)
	vac.POST("/empleados/:empleado_id/compensaciones", middleware.RequirePermission(authz.PermVacacionesAprobar), h.registrarCompensacion)
}

type solicitarVacacionesRequest struct {
	EmpleadoID  int64  `json:"empleado_id"`
	FechaInicio string `json:"fecha_inicio" binding:"required"` // YYYY-MM-DD
	FechaFin    string `json:"fecha_fin" binding:"required"`    // YYYY-MM-DD
}

func (h *VacacionesHandler) solicitar(c *gin.Context) {
	var req solicitarVacacionesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	inicio, err1 := time.Parse("2006-01-02", req.FechaInicio)
	fin, err2 := time.Parse("2006-01-02", req.FechaFin)
	if err1 != nil || err2 != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "fecha_inicio/fecha_fin deben tener formato YYYY-MM-DD", nil)
		return
	}
	payload := middleware.PayloadDesdeContexto(c)
	if payload == nil {
		middleware.Fail(c, http.StatusUnauthorized, "missing_token", "Falta el token de autenticación.", nil)
		return
	}
	puedeTerceros := permisoIncluido(payload.Permisos, authz.PermVacacionesSolicitarTerceros)
	empleadoUsuarioID := int64(0)
	if !puedeTerceros {
		resuelto, resolverErr := middleware.EmpleadoIDDelUsuario(c, h.empleados)
		if resolverErr != nil {
			if errors.Is(resolverErr, middleware.ErrEmpleadoIDNoVinculado) {
				middleware.Fail(c, http.StatusUnprocessableEntity, "employee_not_linked", "El usuario no tiene un empleado vinculado.", nil)
			} else {
				middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "No se pudo resolver el empleado del usuario.", resolverErr)
			}
			return
		}
		empleadoUsuarioID = resuelto
		if req.EmpleadoID == 0 {
			req.EmpleadoID = empleadoUsuarioID
		}
	} else if req.EmpleadoID <= 0 {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "Debe indicar empleado_id.", nil)
		return
	}

	solicitud, err := h.service.SolicitarVacaciones(c.Request.Context(), SolicitarVacacionesInput{
		EmpleadoID:             req.EmpleadoID,
		FechaInicio:            inicio,
		FechaFin:               fin,
		CreadoPor:              middleware.UsuarioID(c),
		EmpleadoIDUsuario:      empleadoUsuarioID,
		PuedeSolicitarTerceros: puedeTerceros,
	})
	if err != nil {
		vacResponderError(c, err, "no se pudo crear la solicitud de vacaciones")
		return
	}
	c.JSON(http.StatusCreated, solicitud)
}

func (h *VacacionesHandler) aprobar(c *gin.Context) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	aprobadorEmpleadoID, ok := h.aprobadorEmpleadoID(c)
	if !ok {
		return
	}
	solicitud, err := h.service.ResolverSolicitud(c.Request.Context(), ResolverSolicitudInput{
		SolicitudID:         id,
		Aprobar:             true,
		AprobadoPor:         middleware.UsuarioID(c),
		AprobadorEmpleadoID: aprobadorEmpleadoID,
	})
	if err != nil {
		vacResponderError(c, err, "no se pudo aprobar la solicitud de vacaciones")
		return
	}
	c.JSON(http.StatusOK, solicitud)
}

type rechazarVacacionesRequest struct {
	MotivoRechazo string `json:"motivo_rechazo" binding:"required,max=255"`
}

func (h *VacacionesHandler) rechazar(c *gin.Context) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req rechazarVacacionesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	aprobadorEmpleadoID, ok := h.aprobadorEmpleadoID(c)
	if !ok {
		return
	}
	solicitud, err := h.service.ResolverSolicitud(c.Request.Context(), ResolverSolicitudInput{
		SolicitudID:         id,
		Aprobar:             false,
		MotivoRechazo:       req.MotivoRechazo,
		AprobadoPor:         middleware.UsuarioID(c),
		AprobadorEmpleadoID: aprobadorEmpleadoID,
	})
	if err != nil {
		vacResponderError(c, err, "no se pudo rechazar la solicitud de vacaciones")
		return
	}
	c.JSON(http.StatusOK, solicitud)
}

type registrarDisfruteRequest struct {
	FechaEfectiva string `json:"fecha_efectiva"` // YYYY-MM-DD; opcional (por defecto fecha_inicio de la solicitud)
}

func (h *VacacionesHandler) registrarDisfrute(c *gin.Context) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}

	var req registrarDisfruteRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		middleware.RespondWithError(c, err)
		return
	}
	var fecha time.Time
	if req.FechaEfectiva != "" {
		f, err := time.Parse("2006-01-02", req.FechaEfectiva)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, "invalid_request", "fecha_efectiva debe tener formato YYYY-MM-DD", nil)
			return
		}
		fecha = f
	}

	saldo, err := h.service.RegistrarDisfrute(c.Request.Context(), RegistrarDisfruteInput{
		SolicitudID:   id,
		FechaEfectiva: fecha,
		RegistradoPor: middleware.UsuarioID(c),
	})
	if err != nil {
		vacResponderError(c, err, "no se pudo registrar el disfrute de vacaciones")
		return
	}
	c.JSON(http.StatusOK, saldo)
}

type registrarCompensacionRequest struct {
	Dias          string `json:"dias" binding:"required"` // texto decimal, nunca float
	FechaEfectiva string `json:"fecha_efectiva" binding:"required"`
	Observaciones string `json:"observaciones"`
}

func (h *VacacionesHandler) registrarCompensacion(c *gin.Context) {
	empleadoID, err := middleware.ParseIDParam(c, "empleado_id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req registrarCompensacionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "cuerpo inválido: se requieren dias y fecha_efectiva", nil)
		return
	}
	dias, err := decimal.NewFromString(req.Dias)
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "dias debe ser un número decimal en texto", nil)
		return
	}
	fecha, err := time.Parse("2006-01-02", req.FechaEfectiva)
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_request", "fecha_efectiva debe tener formato YYYY-MM-DD", nil)
		return
	}
	saldo, err := h.service.RegistrarCompensacion(c.Request.Context(), RegistrarCompensacionInput{
		EmpleadoID: empleadoID, Dias: dias, FechaEfectiva: fecha, Observaciones: req.Observaciones,
		RegistradoPor: middleware.UsuarioID(c),
	})
	if err != nil {
		vacResponderError(c, err, "no se pudo registrar la compensación de vacaciones")
		return
	}
	c.JSON(http.StatusCreated, saldo)
}

func (h *VacacionesHandler) consultarSaldo(c *gin.Context) {
	empleadoID, err := middleware.ParseIDParam(c, "empleado_id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	fecha := time.Now()
	if q := c.Query("fecha"); q != "" {
		f, err := time.Parse("2006-01-02", q)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, "invalid_request", "fecha debe tener formato YYYY-MM-DD", nil)
			return
		}
		fecha = f
	}

	saldo, err := h.service.ConsultarSaldo(c.Request.Context(), empleadoID, fecha)
	if err != nil {
		vacResponderError(c, err, "no se pudo consultar el saldo de vacaciones")
		return
	}
	c.JSON(http.StatusOK, saldo)
}

// vacResponderError traduce los errores del servicio a HTTP y escribe
// exactamente una respuesta. Los errores de negocio (4xx) devuelven su
// mensaje, redactado para el usuario; cualquier otro es un 500 genérico para
// no filtrar detalles de BD.
func vacResponderError(c *gin.Context, err error, mensajeGenerico string) {
	var calInc *ErrCalendarioIncompleto
	switch {
	case errors.Is(err, ErrVacSolicitarTercerosNoPermitido), errors.Is(err, ErrVacSolicitudPropiaNoResoluble):
		middleware.Fail(c, http.StatusForbidden, "row_access_denied", "No puede realizar esta operación para otro empleado.", nil)
		return
	case errors.As(err, &calInc):
		fechas := make([]string, len(calInc.Fechas))
		for i, f := range calInc.Fechas {
			fechas[i] = f.Format("2006-01-02")
		}
		middleware.Fail(c, http.StatusUnprocessableEntity, "calendario_incompleto", calInc.Error(), gin.H{"fechas_faltantes": fechas})

	case errors.Is(err, ErrVacEmpleadoNoEncontrado),
		errors.Is(err, ErrVacSolicitudNoEncontrada):
		middleware.Fail(c, http.StatusNotFound, "not_found", err.Error(), nil)

	case errors.Is(err, ErrVacSolicitudNoPendiente),
		errors.Is(err, ErrVacSolicitudNoAprobada),
		errors.Is(err, ErrVacDisfruteYaRegistrado),
		errors.Is(err, ErrAcumulacionYaRegistrada),
		errors.Is(err, ErrVacSolicitudTraslapada),
		errors.Is(err, ErrVacSaldoInsuficiente):
		middleware.Fail(c, http.StatusConflict, "conflict", err.Error(), nil)

	case errors.Is(err, ErrVacRangoFechasInvalido),
		errors.Is(err, ErrVacSinDiasHabiles),
		errors.Is(err, ErrVacEmpleadoInactivo),
		errors.Is(err, ErrVacEscalaNoConfigurada),
		errors.Is(err, ErrVacMotivoRechazoRequerido),
		errors.Is(err, ErrVacMotivoRechazoMuyLargo),
		errors.Is(err, ErrVacPeriodoReferenciaInvalid),
		errors.Is(err, ErrVacDiasCompensacionInvalidos),
		errors.Is(err, ErrVacObservacionMuyLarga):
		middleware.Fail(c, http.StatusUnprocessableEntity, "unprocessable_entity", err.Error(), nil)

	default:
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", mensajeGenerico, err)
	}
}

func (h *VacacionesHandler) aprobadorEmpleadoID(c *gin.Context) (int64, bool) {
	id, err := middleware.EmpleadoIDDelUsuario(c, h.empleados)
	if errors.Is(err, middleware.ErrEmpleadoIDNoVinculado) {
		return 0, true
	}
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "No se pudo resolver el empleado del usuario.", err)
		return 0, false
	}
	return id, true
}

func permisoIncluido(permisos []string, codigo string) bool {
	for _, permiso := range permisos {
		if permiso == codigo {
			return true
		}
	}
	return false
}
