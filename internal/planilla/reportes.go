package planilla

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

// ReporteOficial: valores de formatos_exportacion.reporte_oficial.
type ReporteOficial string

const (
	ReporteCCSS     ReporteOficial = "ccss"
	ReporteINS      ReporteOficial = "ins"
	ReporteHacienda ReporteOficial = "hacienda"
)

// vistaReporte: vista de origen y columnas permitidas (000138).
type vistaReporte struct {
	vista     string
	columnas  map[string]bool
	numericas map[string]bool // las que pueden ser columna_total
}

var vistasReporte = map[ReporteOficial]vistaReporte{
	ReporteCCSS: {vista: "v_reporte_ccss",
		columnas:  set("identificacion", "nombre_completo", "salario_bruto", "base_ccss", "cuota_obrera_ccss", "cuota_patronal_ccss"),
		numericas: set("salario_bruto", "base_ccss", "cuota_obrera_ccss", "cuota_patronal_ccss")},
	ReporteINS: {vista: "v_reporte_ins",
		columnas:  set("identificacion", "nombre_completo", "salario_bruto", "base_ccss"),
		numericas: set("salario_bruto", "base_ccss")},
	ReporteHacienda: {vista: "v_reporte_renta",
		columnas:  set("identificacion", "nombre_completo", "base_renta", "renta_bruta", "creditos_fiscales_aplicados", "renta_neta"),
		numericas: set("base_renta", "renta_bruta", "creditos_fiscales_aplicados", "renta_neta")},
}

func set(xs ...string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

var (
	ErrReporteDesconocido       = &ErrorCiclo{"reporte_desconocido", "el reporte oficial no existe (ccss | ins | hacienda)", 404}
	ErrReportePeriodoNoAprobado = &ErrorCiclo{"periodo_no_aprobado", "el reporte solo se genera para períodos aprobados, pagados o contabilizados", 409}
	ErrFormatoReporte           = &ErrorCiclo{"formato_reporte_invalido", "no hay un formato de exportación válido para el reporte", 422}
	ErrFormatoSinColumnas       = &ErrorCiclo{"formato_sin_columnas", "el formato no tiene columnas configuradas", 422}
	ErrMesInvalido              = &ErrorCiclo{"mes_invalido", "anio y mes deben ser un mes válido (mes 1-12)", 400}
	ErrMesSinPeriodos           = &ErrorCiclo{"mes_sin_periodos", "el mes no tiene períodos aprobados, pagados o contabilizados", 409}
)

// columnaIdentificacion: la vista la entrega cifrada (hex + llave) y el
// servicio la descifra; el layout la sigue llamando "identificacion".
const (
	columnaIdentificacion   = "identificacion"
	columnaIdentificacionHx = "identificacion_cifrado_hex"
	columnaIdentificacionKd = "identificacion_kid"
	campoIdentificacion     = "empleados.numero_identificacion"
)

// ReportesService genera los reportes oficiales.
type ReportesService struct {
	db          *sql.DB
	descifrador Descifrador
	now         func() time.Time
}

func NewReportesService(sqlDB *sql.DB, descifrador Descifrador) *ReportesService {
	return &ReportesService{db: sqlDB, descifrador: descifrador, now: time.Now}
}

// leerFilas consulta la vista (con consultar) y reemplaza la identificación
// cifrada por su valor descifrado.
func (s *ReportesService) leerFilas(nombres []string, consultar func(cols []string) ([][]*string, error)) ([][]*string, error) {
	sqlCols := append([]string(nil), nombres...)
	idx := -1
	for i, n := range sqlCols {
		if n == columnaIdentificacion {
			sqlCols[i], idx = columnaIdentificacionHx, i
		}
	}
	if idx >= 0 {
		sqlCols = append(sqlCols, columnaIdentificacionKd)
	}
	filas, err := consultar(sqlCols)
	if err != nil || idx < 0 {
		return filas, err
	}
	if s.descifrador == nil {
		return nil, errors.New("planilla: no hay llavero para descifrar la identificación del reporte")
	}
	for _, f := range filas {
		kid := f[len(f)-1]
		if f[idx] != nil && kid != nil {
			cifrado, err := hex.DecodeString(*f[idx])
			if err != nil {
				return nil, fmt.Errorf("planilla: identificación cifrada inválida en el reporte: %w", err)
			}
			valor, err := s.descifrador.Descifrar(campoIdentificacion, cifrado, *kid)
			if err != nil {
				return nil, fmt.Errorf("planilla: descifrando la identificación del reporte: %w", err)
			}
			f[idx] = &valor
		}
	}
	for i := range filas {
		filas[i] = filas[i][:len(filas[i])-1]
	}
	return filas, nil
}

// ArchivoReporte: contenido y metadatos del archivo generado.
type ArchivoReporte struct {
	Contenido     []byte
	NombreArchivo string
	HashSHA256    string
	Filas         int
}

// alcanceReporte: un período (PeriodoID) o un mes calendario (Anio, Mes).
type alcanceReporte struct {
	PeriodoID int64
	Anio, Mes int
}

// Generar arma el CSV del reporte para el período con el formato indicado
// (formatoCodigo vacío = el único formato configurado para ese reporte) y lo
// registra en exportaciones_generadas.
func (s *ReportesService) Generar(ctx context.Context, periodoID int64, reporte ReporteOficial, formatoCodigo string, usuarioID int64) (ArchivoReporte, error) {
	return s.generar(ctx, alcanceReporte{PeriodoID: periodoID}, reporte, formatoCodigo, usuarioID)
}

// GenerarMensual: el mismo reporte consolidado por empleado para todos los
// períodos aprobados, pagados o contabilizados que terminan en el mes (las
// declaraciones a la CCSS, al INS y a Hacienda son mensuales).
func (s *ReportesService) GenerarMensual(ctx context.Context, anio, mes int, reporte ReporteOficial, formatoCodigo string, usuarioID int64) (ArchivoReporte, error) {
	if mes < 1 || mes > 12 || anio < 1990 || anio > 2100 {
		return ArchivoReporte{}, ErrMesInvalido
	}
	return s.generar(ctx, alcanceReporte{Anio: anio, Mes: mes}, reporte, formatoCodigo, usuarioID)
}

func (s *ReportesService) generar(ctx context.Context, alcance alcanceReporte, reporte ReporteOficial, formatoCodigo string, usuarioID int64) (ArchivoReporte, error) {
	v, ok := vistasReporte[reporte]
	if !ok {
		return ArchivoReporte{}, ErrReporteDesconocido
	}
	if usuarioID <= 0 {
		return ArchivoReporte{}, ErrUsuarioExportRequerido
	}
	var out ArchivoReporte
	err := db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		var desde, hasta time.Time
		if alcance.PeriodoID > 0 {
			p, err := q.ObtenerPeriodoCiclo(ctx, alcance.PeriodoID)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPeriodoNoEncontrado
			}
			if err != nil {
				return err
			}
			switch EstadoPeriodo(p.Estado) {
			case EstadoAprobada, EstadoPagada, EstadoContabilizada:
			default:
				return ErrReportePeriodoNoAprobado
			}
		} else {
			desde = time.Date(alcance.Anio, time.Month(alcance.Mes), 1, 0, 0, 0, 0, time.UTC)
			hasta = desde.AddDate(0, 1, -1)
		}
		formatos, err := q.ListarFormatosReporte(ctx, string(reporte), formatoCodigo)
		if err != nil {
			return err
		}
		if len(formatos) != 1 {
			return &ErrorCiclo{Codigo: ErrFormatoReporte.Codigo, HTTP: ErrFormatoReporte.HTTP,
				Mensaje: fmt.Sprintf("se esperaba 1 formato para el reporte %s (código %q) y hay %d", reporte, formatoCodigo, len(formatos))}
		}
		f := formatos[0]
		cols, err := q.ListarColumnasFormato(ctx, f.ID)
		if err != nil {
			return err
		}
		if len(cols) == 0 {
			return ErrFormatoSinColumnas
		}
		nombres := make([]string, len(cols))
		encabezados := make([]string, len(cols))
		idxTotal := -1
		for i, c := range cols {
			if !v.columnas[c.ColumnaOrigen] {
				return &ErrorCiclo{Codigo: ErrFormatoReporte.Codigo, HTTP: ErrFormatoReporte.HTTP,
					Mensaje: fmt.Sprintf("la columna %q no existe en el reporte %s", c.ColumnaOrigen, reporte)}
			}
			nombres[i], encabezados[i] = c.ColumnaOrigen, c.Encabezado
			if f.ColumnaTotal != nil && *f.ColumnaTotal == c.ColumnaOrigen {
				idxTotal = i
			}
		}
		if f.ColumnaTotal != nil && !v.numericas[*f.ColumnaTotal] {
			return &ErrorCiclo{Codigo: ErrFormatoReporte.Codigo, HTTP: ErrFormatoReporte.HTTP,
				Mensaje: fmt.Sprintf("columna_total %q no es numérica en el reporte %s", *f.ColumnaTotal, reporte)}
		}
		filas, err := s.leerFilas(nombres, func(sqlCols []string) ([][]*string, error) {
			if alcance.PeriodoID > 0 {
				return q.ListarFilasReporte(ctx, v.vista, sqlCols, alcance.PeriodoID)
			}
			return q.ListarFilasReporteMensual(ctx, v.vista, sqlCols, v.numericas, desde, hasta)
		})
		if err != nil {
			return err
		}
		if alcance.PeriodoID == 0 && len(filas) == 0 {
			return ErrMesSinPeriodos
		}
		contenido, total, err := armarCSV(encabezados, filas, idxTotal)
		if err != nil {
			return err
		}
		suma := sha256.Sum256(contenido)
		hash := hex.EncodeToString(suma[:])
		ambito := fmt.Sprintf("periodo_%d", alcance.PeriodoID)
		if alcance.PeriodoID == 0 {
			ambito = fmt.Sprintf("mes_%04d_%02d", alcance.Anio, alcance.Mes)
		}
		nombre := fmt.Sprintf("reporte_%s_%s_%s.csv", reporte, ambito, s.now().Format("20060102_150405"))
		if _, err := q.InsertarExportacionGenerada(ctx, dto.InsertarExportacionGeneradaParams{
			PeriodoID: alcance.PeriodoID, Anio: int32(alcance.Anio), Mes: int32(alcance.Mes),
			FormatoCodigo: f.Codigo, NombreArchivo: nombre, HashSHA256: hash,
			CantidadFilas: int64(len(filas)), TotalMonto: total, UsuarioID: usuarioID,
		}); err != nil {
			return fmt.Errorf("planilla: registrando la exportación del reporte %s: %w", reporte, err)
		}
		out = ArchivoReporte{Contenido: contenido, NombreArchivo: nombre, HashSHA256: hash, Filas: len(filas)}
		return nil
	})
	return out, err
}

// armarCSV: RFC 4180 (CRLF), encabezado primero. Si idxTotal >= 0 suma esa
// columna (decimal; un valor no numérico es error).
func armarCSV(encabezados []string, filas [][]*string, idxTotal int) ([]byte, decimal.Decimal, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.Write(encabezados); err != nil {
		return nil, decimal.Zero, err
	}
	total := decimal.Zero
	reg := make([]string, len(encabezados))
	for _, f := range filas {
		for i, v := range f {
			reg[i] = ""
			if v != nil {
				reg[i] = neutralizarCeldaCSV(*v)
			}
		}
		if idxTotal >= 0 && f[idxTotal] != nil {
			d, err := decimal.NewFromString(*f[idxTotal])
			if err != nil {
				return nil, decimal.Zero, fmt.Errorf("planilla: valor no numérico en la columna total: %q", *f[idxTotal])
			}
			total = total.Add(d)
		}
		if err := w.Write(reg); err != nil {
			return nil, decimal.Zero, err
		}
	}
	w.Flush()
	return buf.Bytes(), total, w.Error()
}

// neutralizarCeldaCSV antepone un apóstrofo a una celda que una hoja de
// cálculo interpretaría como fórmula. Un número negativo se deja intacto.
func neutralizarCeldaCSV(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '@', '\t', '\r':
		return "'" + v
	case '-':
		if _, err := decimal.NewFromString(v); err != nil {
			return "'" + v
		}
	}
	return v
}
