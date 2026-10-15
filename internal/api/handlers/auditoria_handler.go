package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/pagination"
)

type AuditoriaHandler struct {
	queries *dto.Queries
}

func NewAuditoriaHandler(queries *dto.Queries) *AuditoriaHandler {
	return &AuditoriaHandler{queries: queries}
}

func (h *AuditoriaHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/auditoria", middleware.RequirePermission(authz.PermAuditoriaVer), h.listar)
}

func (h *AuditoriaHandler) listar(c *gin.Context) {
	p := pagination.Desde(c)
	filtro := dto.FiltroAuditoria{Tabla: strings.TrimSpace(c.Query("tabla")), Limit: p.Limit32(), Offset: p.Offset32()}
	var err error
	if filtro.RegistroID, err = queryPositiveID(c, "registro_id"); err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_filter", "El filtro registro_id debe ser un entero positivo.", nil)
		return
	}
	if filtro.UsuarioID, err = queryPositiveID(c, "usuario"); err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_filter", "El filtro usuario debe ser un entero positivo.", nil)
		return
	}
	if filtro.Desde, err = parseAuditDate(c.Query("desde"), false); err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_filter", "El filtro desde debe ser YYYY-MM-DD o RFC3339.", nil)
		return
	}
	if filtro.Hasta, err = parseAuditDate(c.Query("hasta"), true); err != nil {
		middleware.Fail(c, http.StatusBadRequest, "invalid_filter", "El filtro hasta debe ser YYYY-MM-DD o RFC3339.", nil)
		return
	}
	if filtro.Desde != nil && filtro.Hasta != nil && filtro.Desde.After(*filtro.Hasta) {
		middleware.Fail(c, http.StatusBadRequest, "invalid_filter", "El rango de fechas es inválido.", nil)
		return
	}
	registros, total, err := h.queries.ListarAuditoria(c.Request.Context(), filtro)
	if err != nil {
		middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "No se pudo consultar la auditoría.", err)
		return
	}
	for i := range registros {
		if registros[i].ValorAnterior, err = redactarJSONSensibles(registros[i].ValorAnterior); err != nil {
			middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "No se pudo leer un registro de auditoría.", err)
			return
		}
		if registros[i].ValorNuevo, err = redactarJSONSensibles(registros[i].ValorNuevo); err != nil {
			middleware.FailFromError(c, http.StatusInternalServerError, "internal_error", "No se pudo leer un registro de auditoría.", err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": registros, "paginacion": pagination.NuevaMeta(p, total)})
}

func queryPositiveID(c *gin.Context, key string) (*int64, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil, errors.New("invalid positive id")
	}
	return &id, nil
}

func parseAuditDate(raw string, endOfDay bool) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	if day, err := time.Parse("2006-01-02", raw); err == nil {
		if endOfDay {
			day = day.AddDate(0, 0, 1).Add(-time.Microsecond)
		}
		return &day, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func redactarJSONSensibles(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return raw, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	scrubJSON(value)
	return json.Marshal(value)
}

func scrubJSON(value any) {
	switch item := value.(type) {
	case map[string]any:
		for key, nested := range item {
			if esCampoSensible(key) {
				item[key] = "***REDACTED***"
			} else {
				scrubJSON(nested)
			}
		}
	case []any:
		for _, nested := range item {
			scrubJSON(nested)
		}
	}
}

func esCampoSensible(key string) bool {
	key = strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	for _, parte := range []string{"password", "passwd", "pass", "hash", "token", "secret", "credential", "authorization", "salt"} {
		if strings.Contains(key, parte) {
			return true
		}
	}
	return false
}
