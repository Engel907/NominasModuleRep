package asistencia

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
)

// ClaveDiasLaboralesSemana: días ISO hábiles de la semana separados por coma
// (1 = lunes … 7 = domingo), p. ej. "1,2,3,4,5" o "1,2,3,4,5,6".
const ClaveDiasLaboralesSemana = "DIAS_LABORALES_SEMANA"

const (
	feriadoFijo  = "fijo"
	feriadoMovil = "movil"

	largoMaximoNombreFeriado = 100
)

var (
	ErrDiasLaboralesNoConfigurados = fmt.Errorf("falta el parámetro %s: defina los días hábiles de la semana antes de generar el calendario", ClaveDiasLaboralesSemana)
	ErrDiasLaboralesInvalidos      = fmt.Errorf("%s debe listar días del 1 (lunes) al 7 (domingo) separados por coma, sin repetir", ClaveDiasLaboralesSemana)
	ErrCatalogoFeriadosVacio       = errors.New("el catálogo de feriados no tiene feriados activos para el año solicitado")
	ErrFeriadoCatalogoInvalido     = errors.New("feriado de catálogo inválido")
	ErrFeriadoCatalogoNoEncontrado = errors.New("feriado de catálogo no encontrado")
)

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

// ParsearDiasLaborales convierte el valor de DIAS_LABORALES_SEMANA en el
// conjunto de días ISO hábiles.
func ParsearDiasLaborales(valor string) (map[int]bool, error) {
	dias := map[int]bool{}
	for _, parte := range strings.Split(valor, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(parte))
		if err != nil || n < 1 || n > 7 || dias[n] {
			return nil, ErrDiasLaboralesInvalidos
		}
		dias[n] = true
	}
	return dias, nil
}

// ValidarFeriadoCatalogo exige la forma propia de cada tipo: un feriado fijo
// tiene mes y día válidos; uno móvil, solo el desplazamiento desde Pascua.
func ValidarFeriadoCatalogo(f dto.FeriadoCatalogo) error {
	invalido := func(motivo string) error { return fmt.Errorf("%w: %s", ErrFeriadoCatalogoInvalido, motivo) }
	nombre := strings.TrimSpace(f.Nombre)
	if nombre == "" || utf8.RuneCountInString(nombre) > largoMaximoNombreFeriado {
		return invalido("nombre requerido, máximo 100 caracteres")
	}
	if strings.TrimSpace(f.FuenteLegal) == "" {
		return invalido("fuente_legal es requerida")
	}
	if f.AnioDesde < 1900 || (f.AnioHasta != nil && *f.AnioHasta < f.AnioDesde) {
		return invalido("rango de años inválido")
	}
	switch f.Tipo {
	case feriadoFijo:
		if f.Mes == nil || f.Dia == nil || f.DesplazamientoPascua != nil {
			return invalido("un feriado fijo requiere mes y día, sin desplazamiento_pascua")
		}
		fecha := time.Date(2024, time.Month(*f.Mes), *f.Dia, 0, 0, 0, 0, time.UTC)
		if *f.Mes < 1 || *f.Mes > 12 || fecha.Day() != *f.Dia {
			return invalido("mes y día no forman una fecha válida")
		}
	case feriadoMovil:
		if f.DesplazamientoPascua == nil || f.Mes != nil || f.Dia != nil {
			return invalido("un feriado móvil requiere desplazamiento_pascua, sin mes ni día")
		}
		if *f.DesplazamientoPascua < -70 || *f.DesplazamientoPascua > 70 {
			return invalido("desplazamiento_pascua fuera de rango (-70 a 70)")
		}
	default:
		return invalido("tipo debe ser fijo o movil")
	}
	return nil
}

// ResolverFeriadosAnio fija la fecha de cada feriado del catálogo en el año.
// Dos feriados que caen el mismo día se fusionan: nombres unidos y pago
// obligatorio si lo es cualquiera de ellos.
func ResolverFeriadosAnio(anio int, catalogo []dto.FeriadoCatalogo) ([]dto.CrearFeriadoParams, error) {
	pascua := pascuaOccidental(anio)
	porFecha := map[time.Time]*dto.CrearFeriadoParams{}
	for _, f := range catalogo {
		if err := ValidarFeriadoCatalogo(f); err != nil {
			return nil, fmt.Errorf("feriado %q del catálogo: %w", f.Nombre, err)
		}
		fecha := pascua.AddDate(0, 0, intOCero(f.DesplazamientoPascua))
		if f.Tipo == feriadoFijo {
			if *f.Mes == 2 && *f.Dia == 29 && !esBisiesto(anio) {
				continue
			}
			fecha = time.Date(anio, time.Month(*f.Mes), *f.Dia, 0, 0, 0, 0, time.UTC)
		}
		if previo, ok := porFecha[fecha]; ok {
			previo.Nombre = recortar(previo.Nombre+" / "+f.Nombre, largoMaximoNombreFeriado)
			previo.PagoObligatorio = previo.PagoObligatorio || f.PagoObligatorio
			if f.Tipo == feriadoFijo {
				previo.Tipo = feriadoFijo
			}
			continue
		}
		porFecha[fecha] = &dto.CrearFeriadoParams{
			Fecha:           fecha,
			Nombre:          strings.TrimSpace(f.Nombre),
			Tipo:            f.Tipo,
			PagoObligatorio: f.PagoObligatorio,
			Anio:            int32(anio),
		}
	}
	out := make([]dto.CrearFeriadoParams, 0, len(porFecha))
	for _, p := range porFecha {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Fecha.Before(out[j].Fecha) })
	return out, nil
}

func intOCero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func esBisiesto(anio int) bool {
	return anio%4 == 0 && (anio%100 != 0 || anio%400 == 0)
}

func recortar(s string, maximo int) string {
	if utf8.RuneCountInString(s) <= maximo {
		return s
	}
	return string([]rune(s)[:maximo])
}

func (s *Service) diasLaborales(ctx context.Context, fecha time.Time) (map[int]bool, error) {
	valor, err := s.parametros.Texto(ctx, ClaveDiasLaboralesSemana, fecha)
	var noConfigurado parametros.ErrParametroNoConfigurado
	if errors.As(err, &noConfigurado) {
		return nil, ErrDiasLaboralesNoConfigurados
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", ClaveDiasLaboralesSemana, err)
	}
	return ParsearDiasLaborales(valor)
}

// GenerarCalendarioAnual siembra `feriados` y `calendario_laboral_dia` del año
// a partir del catálogo de feriados y de DIAS_LABORALES_SEMANA vigentes al 1
// de enero, en una sola transacción auditada. Regenerar un año lo sincroniza:
// los feriados que ya no están en el catálogo se eliminan.
func (s *Service) GenerarCalendarioAnual(ctx context.Context, anio int, usuarioID int64) error {
	if anio < 2000 || anio > 2100 {
		return fmt.Errorf("%w: %d (permitido 2000–2100)", ErrAnioFueraDeRango, anio)
	}
	inicio := time.Date(anio, time.January, 1, 0, 0, 0, 0, time.UTC)
	fin := time.Date(anio, time.December, 31, 0, 0, 0, 0, time.UTC)
	habiles, err := s.diasLaborales(ctx, inicio)
	if err != nil {
		return err
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		catalogo, err := q.ListarFeriadosCatalogoAnio(ctx, anio)
		if err != nil {
			return fmt.Errorf("leyendo catálogo de feriados: %w", err)
		}
		if len(catalogo) == 0 {
			return ErrCatalogoFeriadosVacio
		}
		resueltos, err := ResolverFeriadosAnio(anio, catalogo)
		if err != nil {
			return err
		}

		feriadosPorFecha := map[string]int64{}
		vigentes := make([]int64, 0, len(resueltos))
		for _, params := range resueltos {
			feriado, err := q.CrearFeriado(ctx, params)
			if err != nil {
				return fmt.Errorf("insertando feriado %s: %w", params.Nombre, err)
			}
			feriadosPorFecha[feriado.Fecha.Format(time.DateOnly)] = feriado.ID
			vigentes = append(vigentes, feriado.ID)
		}

		for d := inicio; !d.After(fin); d = d.AddDate(0, 0, 1) {
			diaSemana := int(d.Weekday())
			if diaSemana == 0 {
				diaSemana = 7
			}
			feriadoID, esFeriado := feriadosPorFecha[d.Format(time.DateOnly)]
			var feriadoIDParam *int64
			if esFeriado {
				feriadoIDParam = &feriadoID
			}
			if err := q.InsertarDiaCalendario(ctx, dto.InsertarDiaCalendarioParams{
				Fecha:     d,
				Anio:      int32(anio),
				EsHabil:   habiles[diaSemana] && !esFeriado,
				EsFeriado: esFeriado,
				FeriadoID: feriadoIDParam,
				DiaSemana: int16(diaSemana),
			}); err != nil {
				return fmt.Errorf("insertando día de calendario %s: %w", d.Format(time.DateOnly), err)
			}
		}
		if err := q.EliminarFeriadosAnioExcepto(ctx, int32(anio), vigentes); err != nil {
			return fmt.Errorf("eliminando feriados que ya no están en el catálogo: %w", err)
		}
		return nil
	})
}

func (s *Service) ListarFeriadosCatalogo(ctx context.Context) ([]dto.FeriadoCatalogo, error) {
	return s.queries.ListarFeriadosCatalogo(ctx)
}

func (s *Service) CrearFeriadoCatalogo(ctx context.Context, f dto.FeriadoCatalogo, usuarioID int64) (dto.FeriadoCatalogo, error) {
	f.Nombre, f.FuenteLegal = strings.TrimSpace(f.Nombre), strings.TrimSpace(f.FuenteLegal)
	if err := ValidarFeriadoCatalogo(f); err != nil {
		return dto.FeriadoCatalogo{}, err
	}
	var creado dto.FeriadoCatalogo
	err := db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		id, err := q.CrearFeriadoCatalogo(ctx, f, usuarioID)
		if err != nil {
			return err
		}
		creado, err = q.ObtenerFeriadoCatalogo(ctx, id)
		return err
	})
	return creado, err
}

func (s *Service) ActualizarFeriadoCatalogo(ctx context.Context, f dto.FeriadoCatalogo, usuarioID int64) (dto.FeriadoCatalogo, error) {
	f.Nombre, f.FuenteLegal = strings.TrimSpace(f.Nombre), strings.TrimSpace(f.FuenteLegal)
	if err := ValidarFeriadoCatalogo(f); err != nil {
		return dto.FeriadoCatalogo{}, err
	}
	var actualizado dto.FeriadoCatalogo
	err := db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if existe, err := q.ExisteFeriadoCatalogo(ctx, f.ID); err != nil {
			return err
		} else if !existe {
			return ErrFeriadoCatalogoNoEncontrado
		}
		if _, err := q.ActualizarFeriadoCatalogo(ctx, f); err != nil {
			return err
		}
		var err error
		actualizado, err = q.ObtenerFeriadoCatalogo(ctx, f.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrFeriadoCatalogoNoEncontrado
		}
		return err
	})
	return actualizado, err
}
