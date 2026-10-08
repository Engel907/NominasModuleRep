package asistencia

import (
	"context"
	"fmt"
	"time"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

// FeriadoBase describe un feriado antes de resolverse a una fecha concreta del año.
type FeriadoBase struct {
	Nombre          string
	PagoObligatorio bool
	// Fija: mes/día constantes cada año. Móvil (Semana Santa): se calcula desde Pascua.
	Fija         bool
	Mes, Dia     int // usado cuando Fija = true
	OffsetPascua int // usado cuando Fija = false (días respecto al Domingo de Pascua)
}

// feriadosCostaRica está tomado del Código de Trabajo Art. 148 (obligatorios,
// doble pago si se labora) y de las leyes de feriados facultativos (pago no
// obligatorio). Confirmar con Recursos Humanos antes de usarse como fuente
// legal definitiva — aquí se documenta como catálogo de referencia, no como
// texto legal citado.
var feriadosCostaRica = []FeriadoBase{
	{Nombre: "Año Nuevo", PagoObligatorio: true, Fija: true, Mes: 1, Dia: 1},
	{Nombre: "Jueves Santo", PagoObligatorio: true, Fija: false, OffsetPascua: -3},
	{Nombre: "Viernes Santo", PagoObligatorio: true, Fija: false, OffsetPascua: -2},
	{Nombre: "Día de Juan Santamaría", PagoObligatorio: true, Fija: true, Mes: 4, Dia: 11},
	{Nombre: "Día Internacional del Trabajador", PagoObligatorio: true, Fija: true, Mes: 5, Dia: 1},
	{Nombre: "Anexión del Partido de Nicoya", PagoObligatorio: true, Fija: true, Mes: 7, Dia: 25},
	{Nombre: "Virgen de los Ángeles", PagoObligatorio: false, Fija: true, Mes: 8, Dia: 2},
	{Nombre: "Día de la Madre", PagoObligatorio: true, Fija: true, Mes: 8, Dia: 15},
	{Nombre: "Independencia de Costa Rica", PagoObligatorio: true, Fija: true, Mes: 9, Dia: 15},
	{Nombre: "Día de las Culturas", PagoObligatorio: false, Fija: true, Mes: 10, Dia: 12},
	{Nombre: "Navidad", PagoObligatorio: true, Fija: true, Mes: 12, Dia: 25},
}

// pascuaOccidental calcula la fecha del Domingo de Pascua para un año dado
// usando el algoritmo de Meeus/Jones/Butcher (calendario gregoriano).
func pascuaOccidental(anio int) time.Time {
	a := anio % 19
	b := anio / 100
	c := anio % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	mes := (h + l - 7*m + 114) / 31
	dia := (h+l-7*m+114)%31 + 1
	return time.Date(anio, time.Month(mes), dia, 0, 0, 0, 0, time.UTC)
}

// ResolverFeriadosAnio calcula las fechas concretas de los feriados de CR para un
// año dado, a partir del catálogo fijo + móvil.
func ResolverFeriadosAnio(anio int) []dto.CrearFeriadoParams {
	pascua := pascuaOccidental(anio)
	feriados := make([]dto.CrearFeriadoParams, 0, len(feriadosCostaRica))

	for _, f := range feriadosCostaRica {
		var fecha time.Time
		tipo := "fijo"
		if f.Fija {
			fecha = time.Date(anio, time.Month(f.Mes), f.Dia, 0, 0, 0, 0, time.UTC)
		} else {
			fecha = pascua.AddDate(0, 0, f.OffsetPascua)
			tipo = "movil"
		}
		feriados = append(feriados, dto.CrearFeriadoParams{
			Fecha:           fecha,
			Nombre:          f.Nombre,
			Tipo:            tipo,
			PagoObligatorio: f.PagoObligatorio,
			Anio:            int32(anio),
		})
	}
	return feriados
}

// GenerarCalendarioAnual siembra `feriados` y `calendario_laboral_dia` para un
// año completo, todo en UNA transacción auditada (WithAuditContext): o queda el
// año completo o no queda nada. Idempotente: usa ON DUPLICATE KEY UPDATE en las
// queries, así que se puede volver a correr para re-sincronizar sin duplicar
// filas. usuarioID es quien dispara la generación (queda en auditoría).
func (s *Service) GenerarCalendarioAnual(ctx context.Context, anio int, usuarioID int64) error {
	if anio < 2000 || anio > 2100 {
		return fmt.Errorf("%w: %d (permitido 2000–2100)", ErrAnioFueraDeRango, anio)
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		feriadosPorFecha := map[string]int64{}
		for _, params := range ResolverFeriadosAnio(anio) {
			feriado, err := q.CrearFeriado(ctx, params)
			if err != nil {
				return fmt.Errorf("insertando feriado %s: %w", params.Nombre, err)
			}
			feriadosPorFecha[feriado.Fecha.Format("2006-01-02")] = feriado.ID
		}

		inicio := time.Date(anio, time.January, 1, 0, 0, 0, 0, time.UTC)
		fin := time.Date(anio, time.December, 31, 0, 0, 0, 0, time.UTC)

		for d := inicio; !d.After(fin); d = d.AddDate(0, 0, 1) {
			diaSemana := int(d.Weekday())
			if diaSemana == 0 {
				diaSemana = 7 // domingo = 7, para que 1=lunes...7=domingo
			}

			feriadoID, esFeriado := feriadosPorFecha[d.Format("2006-01-02")]
			esHabil := diaSemana <= 5 && !esFeriado // regla base: lunes-viernes, sin feriado

			var feriadoIDParam *int64
			if esFeriado {
				feriadoIDParam = &feriadoID
			}

			if err := q.InsertarDiaCalendario(ctx, dto.InsertarDiaCalendarioParams{
				Fecha:     d,
				Anio:      int32(anio),
				EsHabil:   esHabil,
				EsFeriado: esFeriado,
				FeriadoID: feriadoIDParam,
				DiaSemana: int16(diaSemana),
			}); err != nil {
				return fmt.Errorf("insertando día de calendario %s: %w", d.Format("2006-01-02"), err)
			}
		}
		return nil
	})
}