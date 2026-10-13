package asistencia

import (
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/pagination"
)

// parseFechaQuery lee un query param de fecha (YYYY-MM-DD); devuelve nil si
// está vacío o mal formado, para que la query SQL use COALESCE(?, columna) y
// el filtro quede "sin efecto" en vez de fallar.
func parseFechaQuery(c *gin.Context, nombre string) *time.Time {
	valor := c.Query(nombre)
	if valor == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", valor)
	if err != nil {
		return nil
	}
	return &t
}

func parseEstadoQuery(c *gin.Context) *string {
	estado := c.Query("estado")
	if estado == "" {
		return nil
	}
	return &estado
}

func parseActivoQuery(c *gin.Context) *bool {
	valor := c.Query("activo")
	if valor == "" {
		return nil
	}
	activo := valor == "true" || valor == "1"
	return &activo
}

func turnosFiltro(c *gin.Context, p pagination.Parametros) dto.ListarTurnosParams {
	return dto.ListarTurnosParams{
		Activo: parseActivoQuery(c),
		Limit:  int32(p.Tamano),
		Offset: int32(p.Offset),
	}
}

func asistenciaFiltro(c *gin.Context, empleadoID int64, p pagination.Parametros) dto.ListarAsistenciaEmpleadoParams {
	return dto.ListarAsistenciaEmpleadoParams{
		EmpleadoID: empleadoID,
		Estado:     parseEstadoQuery(c),
		FechaDesde: parseFechaQuery(c, "desde"),
		FechaHasta: parseFechaQuery(c, "hasta"),
		Limit:      int32(p.Tamano),
		Offset:     int32(p.Offset),
	}
}

func asistenciaFiltroConteo(c *gin.Context, empleadoID int64) dto.ContarAsistenciaEmpleadoParams {
	return dto.ContarAsistenciaEmpleadoParams{
		EmpleadoID: empleadoID,
		Estado:     parseEstadoQuery(c),
		FechaDesde: parseFechaQuery(c, "desde"),
		FechaHasta: parseFechaQuery(c, "hasta"),
	}
}

func buildListarIncapacidadesParams(empleadoID int64, entidad *string, p pagination.Parametros) dto.ListarIncapacidadesEmpleadoParams {
	return dto.ListarIncapacidadesEmpleadoParams{
		EmpleadoID:     empleadoID,
		EntidadEmisora: entidad,
		Limit:          int32(p.Tamano),
		Offset:         int32(p.Offset),
	}
}
