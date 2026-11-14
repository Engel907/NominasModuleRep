package handlers

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/catalogos"
	"planillas-ccdr/internal/paginacion"
)

type CatalogosHandler struct{ svc *catalogos.Service }

func NewCatalogosHandler(svc *catalogos.Service) *CatalogosHandler {
	return &CatalogosHandler{svc: svc}
}

func (h *CatalogosHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/configuracion/catalogos")
	ver := middleware.RequirePermission(authz.PermConfiguracionVer)
	administrar := middleware.RequirePermission(authz.PermConfiguracionAdministrar)
	g.GET("", ver, h.catalogos)
	g.GET("/:catalogo", ver, h.listar)
	g.GET("/:catalogo/:id", ver, h.obtener)
	g.POST("/:catalogo", administrar, h.crear)
	g.PATCH("/:catalogo/:id", administrar, h.actualizar)
	g.DELETE("/:catalogo/:id", administrar, h.borrar)
}

func (h *CatalogosHandler) catalogos(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.svc.Catalogos()})
}

func (h *CatalogosHandler) listar(c *gin.Context) {
	pag := paginacion.Desde(c)
	filas, total, err := h.svc.Listar(c.Request.Context(), c.Param("catalogo"), pag.Limit32(), pag.Offset32())
	if err != nil {
		fallarCatalogos(c, err)
		return
	}
	if filas == nil {
		filas = []map[string]*string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": filas, "meta": paginacion.NuevaMeta(pag, total)})
}

func (h *CatalogosHandler) obtener(c *gin.Context) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	fila, err := h.svc.Obtener(c.Request.Context(), c.Param("catalogo"), id)
	if err != nil {
		fallarCatalogos(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": fila})
}

func cuerpoCrudo(c *gin.Context) ([]byte, bool) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		middleware.RespondWithError(c, err)
		return nil, false
	}
	return raw, true
}

func (h *CatalogosHandler) crear(c *gin.Context) {
	usuarioID := middleware.UsuarioID(c)
	raw, ok := cuerpoCrudo(c)
	if !ok {
		return
	}
	fila, err := h.svc.Crear(c.Request.Context(), usuarioID, c.Param("catalogo"), raw)
	if err != nil {
		fallarCatalogos(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": fila})
}

func (h *CatalogosHandler) actualizar(c *gin.Context) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	raw, ok := cuerpoCrudo(c)
	if !ok {
		return
	}
	fila, err := h.svc.Actualizar(c.Request.Context(), middleware.UsuarioID(c), c.Param("catalogo"), id, raw)
	if err != nil {
		fallarCatalogos(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": fila})
}

func (h *CatalogosHandler) borrar(c *gin.Context) {
	id, err := middleware.ParseIDParam(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	if err := h.svc.Borrar(c.Request.Context(), middleware.UsuarioID(c), c.Param("catalogo"), id); err != nil {
		fallarCatalogos(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func fallarCatalogos(c *gin.Context, err error) {
	switch {
	case errors.Is(err, catalogos.ErrCatalogoNoSoportado):
		middleware.Fail(c, http.StatusNotFound, "catalogo_no_soportado", err.Error(), nil)
	case errors.Is(err, catalogos.ErrRegistroNoExiste):
		middleware.Fail(c, http.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, catalogos.ErrDatosInvalidos):
		middleware.Fail(c, http.StatusUnprocessableEntity, "datos_invalidos", err.Error(), nil)
	case errors.Is(err, catalogos.ErrCampoInmutable):
		middleware.Fail(c, http.StatusConflict, "campo_inmutable", err.Error(), nil)
	case errors.Is(err, catalogos.ErrBorradoNoPermitido):
		middleware.Fail(c, http.StatusConflict, "borrado_no_permitido", err.Error(), nil)
	case errors.Is(err, catalogos.ErrDuplicado):
		middleware.Fail(c, http.StatusConflict, "duplicado", err.Error(), nil)
	case errors.Is(err, catalogos.ErrEnUso):
		middleware.Fail(c, http.StatusConflict, "en_uso", err.Error(), nil)
	default:
		middleware.RespondWithError(c, err)
	}
}
