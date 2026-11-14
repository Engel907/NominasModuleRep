package asistencia

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/db/dto"
)

type feriadoCatalogoRequest struct {
	Nombre               string `json:"nombre" binding:"required,max=100"`
	Tipo                 string `json:"tipo" binding:"required,oneof=fijo movil"`
	Mes                  *int   `json:"mes"`
	Dia                  *int   `json:"dia"`
	DesplazamientoPascua *int   `json:"desplazamiento_pascua"`
	PagoObligatorio      *bool  `json:"pago_obligatorio" binding:"required"`
	FuenteLegal          string `json:"fuente_legal" binding:"required,max=255"`
	AnioDesde            int    `json:"anio_desde" binding:"required"`
	AnioHasta            *int   `json:"anio_hasta"`
	Activo               *bool  `json:"activo" binding:"required"`
}

func (r feriadoCatalogoRequest) aDTO(id int64) dto.FeriadoCatalogo {
	return dto.FeriadoCatalogo{
		ID:                   id,
		Nombre:               r.Nombre,
		Tipo:                 r.Tipo,
		Mes:                  r.Mes,
		Dia:                  r.Dia,
		DesplazamientoPascua: r.DesplazamientoPascua,
		PagoObligatorio:      *r.PagoObligatorio,
		FuenteLegal:          r.FuenteLegal,
		AnioDesde:            r.AnioDesde,
		AnioHasta:            r.AnioHasta,
		Activo:               *r.Activo,
	}
}

func (h *Handler) listarFeriadosCatalogo(c *gin.Context) {
	feriados, err := h.service.ListarFeriadosCatalogo(c.Request.Context())
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": feriados})
}

func (h *Handler) crearFeriadoCatalogo(c *gin.Context) {
	var req feriadoCatalogoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	creado, err := h.service.CrearFeriadoCatalogo(c.Request.Context(), req.aDTO(0), middleware.UsuarioID(c))
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, creado)
}

func (h *Handler) actualizarFeriadoCatalogo(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req feriadoCatalogoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, err)
		return
	}
	actualizado, err := h.service.ActualizarFeriadoCatalogo(c.Request.Context(), req.aDTO(id), middleware.UsuarioID(c))
	if err != nil {
		responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, actualizado)
}
