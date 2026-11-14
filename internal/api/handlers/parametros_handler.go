package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/paginacion"
	"planillas-ccdr/internal/parametros"
)

type ParametrosHandler struct{ svc *parametros.Propuestas }

func NewParametrosHandler(svc *parametros.Propuestas) *ParametrosHandler {
	return &ParametrosHandler{svc: svc}
}

func (h *ParametrosHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/parametros")
	ver := middleware.RequirePermission(authz.PermParametrosVer)
	g.GET("/tablas", ver, h.tablas)
	g.GET("/tablas/:tabla", ver, h.vigentes)
	g.GET("/propuestas", ver, h.listar)
	g.POST("/propuestas", middleware.RequirePermission(authz.PermParametrosProponer), h.proponer)
	g.POST("/propuestas/:id/aprobar", middleware.RequirePermission(authz.PermParametrosAprobar), h.aprobar)
	g.POST("/propuestas/:id/rechazar", middleware.RequirePermission(authz.PermParametrosAprobar), h.rechazar)
}

func (h *ParametrosHandler) tablas(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.svc.Tablas()})
}

func (h *ParametrosHandler) vigentes(c *gin.Context) {
	fecha := time.Now().UTC()
	if s := c.Query("fecha"); s != "" {
		f, err := time.Parse("2006-01-02", s)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, "fecha_invalida", "fecha debe tener formato YYYY-MM-DD", nil)
			return
		}
		fecha = f
	}
	filas, err := h.svc.Vigentes(c.Request.Context(), c.Param("tabla"), fecha)
	if err != nil {
		fallarParametros(c, err)
		return
	}
	if filas == nil {
		filas = []map[string]*string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": filas, "fecha": fecha.Format("2006-01-02")})
}

func (h *ParametrosHandler) listar(c *gin.Context) {
	pag := paginacion.Desde(c)
	filas, total, err := h.svc.Listar(c.Request.Context(), c.Query("estado"), c.Query("tabla"), pag.Limit32(), pag.Offset32())
	if err != nil {
		fallarParametros(c, err)
		return
	}
	if filas == nil {
		filas = []parametros.Propuesta{}
	}
	c.JSON(http.StatusOK, gin.H{"data": filas, "meta": paginacion.NuevaMeta(pag, total)})
}

type proponerBody struct {
	Tabla               string          `json:"tabla" binding:"required"`
	FechaInicioVigencia string          `json:"fecha_inicio_vigencia" binding:"required"`
	FuenteLegal         string          `json:"fuente_legal" binding:"required"`
	Payload             json.RawMessage `json:"payload" binding:"required"`
}

func (h *ParametrosHandler) proponer(c *gin.Context) {
	usuarioID := middleware.UsuarioID(c)
	if usuarioID <= 0 {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "No se pudo determinar el usuario autenticado", nil)
		return
	}
	var b proponerBody
	if err := c.ShouldBindJSON(&b); err != nil {
		middleware.Fail(c, http.StatusBadRequest, "cuerpo_invalido", "se requieren tabla, fecha_inicio_vigencia, fuente_legal y payload", nil)
		return
	}
	f, err := time.Parse("2006-01-02", b.FechaInicioVigencia)
	if err != nil {
		middleware.Fail(c, http.StatusBadRequest, "fecha_invalida", "fecha_inicio_vigencia debe tener formato YYYY-MM-DD", nil)
		return
	}
	p, err := h.svc.Proponer(c.Request.Context(), usuarioID, parametros.NuevaPropuesta{
		Tabla: b.Tabla, Payload: b.Payload, FechaInicioVigencia: f, FuenteLegal: b.FuenteLegal,
	})
	if err != nil {
		fallarParametros(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": p})
}

func (h *ParametrosHandler) aprobar(c *gin.Context) {
	id, usuarioID, ok := idYUsuario(c)
	if !ok {
		return
	}
	p, err := h.svc.Aprobar(c.Request.Context(), usuarioID, id)
	if err != nil {
		fallarParametros(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": p})
}

func (h *ParametrosHandler) rechazar(c *gin.Context) {
	id, usuarioID, ok := idYUsuario(c)
	if !ok {
		return
	}
	var b struct {
		Motivo string `json:"motivo"`
	}
	if err := c.ShouldBindJSON(&b); err != nil {
		middleware.Fail(c, http.StatusBadRequest, "cuerpo_invalido", "se requiere motivo", nil)
		return
	}
	p, err := h.svc.Rechazar(c.Request.Context(), usuarioID, id, b.Motivo)
	if err != nil {
		fallarParametros(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": p})
}

func idYUsuario(c *gin.Context) (int64, int64, bool) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return 0, 0, false
	}
	usuarioID := middleware.UsuarioID(c)
	if usuarioID <= 0 {
		middleware.Fail(c, http.StatusUnauthorized, "unauthorized", "No se pudo determinar el usuario autenticado", nil)
		return 0, 0, false
	}
	return id, usuarioID, true
}

// fallarParametros: errores de dominio con su código; el resto, 500 sin el
// texto interno.
func fallarParametros(c *gin.Context, err error) {
	switch {
	case errors.Is(err, parametros.ErrTablaNoSoportada):
		middleware.Fail(c, http.StatusNotFound, "tabla_no_soportada", err.Error(), nil)
	case errors.Is(err, parametros.ErrPropuestaNoEncontrada):
		middleware.Fail(c, http.StatusNotFound, "propuesta_no_encontrada", err.Error(), nil)
	case errors.Is(err, parametros.ErrPropuestaInvalida):
		middleware.Fail(c, http.StatusUnprocessableEntity, "propuesta_invalida", err.Error(), nil)
	case errors.Is(err, parametros.ErrCuatroOjos):
		middleware.Fail(c, http.StatusForbidden, "segregacion_funciones", err.Error(), nil)
	case errors.Is(err, parametros.ErrPropuestaResuelta):
		middleware.Fail(c, http.StatusConflict, "propuesta_resuelta", err.Error(), nil)
	case errors.Is(err, parametros.ErrVigenciaNoPosterior):
		middleware.Fail(c, http.StatusConflict, "vigencia_no_posterior", err.Error(), nil)
	case errors.Is(err, parametros.ErrSerieSinVigente):
		middleware.Fail(c, http.StatusConflict, "serie_sin_vigente", err.Error(), nil)
	default:
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "Error interno del servidor", err)
	}
}
