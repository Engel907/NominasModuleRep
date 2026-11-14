package planilla

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"time"

	"planillas-ccdr/internal/db/dto"
)

// TablaReporte: resultado de un reporte de gestión. Cada fila trae un valor
// por columna (nil = sin valor).
type TablaReporte struct {
	Reporte  string
	Columnas []string
	Filas    [][]*string
}

// Objetos: filas como mapas columna → valor (para JSON).
func (t TablaReporte) Objetos() []map[string]*string {
	out := make([]map[string]*string, len(t.Filas))
	for i, f := range t.Filas {
		m := make(map[string]*string, len(t.Columnas))
		for j, c := range t.Columnas {
			m[c] = f[j]
		}
		out[i] = m
	}
	return out
}

// CSV: RFC 4180 con encabezado; las celdas que una hoja de cálculo tomaría
// como fórmula se neutralizan.
func (t TablaReporte) CSV() ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.Write(t.Columnas); err != nil {
		return nil, err
	}
	reg := make([]string, len(t.Columnas))
	for _, f := range t.Filas {
		for i, v := range f {
			reg[i] = ""
			if v != nil {
				reg[i] = neutralizarCeldaCSV(*v)
			}
		}
		if err := w.Write(reg); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

var (
	ErrPeriodoNoCalculado = &ErrorCiclo{"periodo_no_calculado", "el período todavía no está calculado", 409}
	ErrRangoInvalido      = &ErrorCiclo{"rango_invalido", "el año o el mes del reporte no son válidos", 400}
)

// ReportesGestionService: reportes internos de la planilla (no oficiales). Se
// identifican los empleados por código interno: ningún reporte de gestión
// descifra datos personales.
type ReportesGestionService struct{ q *dto.Queries }

func NewReportesGestionService(sqlDB *sql.DB) *ReportesGestionService {
	return &ReportesGestionService{q: dto.New(sqlDB)}
}

func (s *ReportesGestionService) periodo(ctx context.Context, periodoID int64) error {
	existe, conDetalle, err := s.q.PeriodoTieneDetalle(ctx, periodoID)
	switch {
	case errors.Is(err, sql.ErrNoRows) || (err == nil && !existe):
		return ErrPeriodoNoEncontrado
	case err != nil:
		return err
	case !conDetalle:
		return ErrPeriodoNoCalculado
	}
	return nil
}

// ResumenConceptos: total por concepto del período (ingresos, deducciones y
// cargas patronales).
func (s *ReportesGestionService) ResumenConceptos(ctx context.Context, periodoID int64) (TablaReporte, error) {
	if err := s.periodo(ctx, periodoID); err != nil {
		return TablaReporte{}, err
	}
	filas, err := s.q.ReporteResumenConceptos(ctx, periodoID)
	return TablaReporte{Reporte: "resumen_conceptos",
		Columnas: []string{"codigo", "nombre", "naturaleza", "empleados", "cantidad", "monto"}, Filas: filas}, err
}

// CostoPorPrograma: costo de la planilla por programa presupuestario o centro
// de costo.
func (s *ReportesGestionService) CostoPorPrograma(ctx context.Context, periodoID int64) (TablaReporte, error) {
	if err := s.periodo(ctx, periodoID); err != nil {
		return TablaReporte{}, err
	}
	filas, err := s.q.ReporteCostoPorPrograma(ctx, periodoID)
	return TablaReporte{Reporte: "costo_por_programa",
		Columnas: []string{"programa_codigo", "programa_nombre", "empleados", "total_ingresos", "total_deducciones",
			"neto_pagado", "cuota_patronal", "costo_total"}, Filas: filas}, err
}

// AcumuladoEmpleados: lo devengado, cotizado y retenido por empleado en un año
// (mes = 0) o en un mes: base de constancias salariales y de la declaración
// anual de retenciones.
func (s *ReportesGestionService) AcumuladoEmpleados(ctx context.Context, anio, mes int) (TablaReporte, error) {
	desde, hasta, err := rangoReporte(anio, mes)
	if err != nil {
		return TablaReporte{}, err
	}
	filas, err := s.q.ReporteAcumuladoEmpleados(ctx, desde, hasta)
	return TablaReporte{Reporte: "acumulado_empleados",
		Columnas: []string{"codigo_interno", "nombre_completo", "periodos", "salario_bruto", "base_ccss", "cuota_obrera_ccss",
			"cuota_patronal_ccss", "base_renta", "renta_retenida", "creditos_fiscales", "neto_pagado"}, Filas: filas}, err
}

// SaldosVacaciones: saldo de vacaciones de cada empleado activo a la fecha.
func (s *ReportesGestionService) SaldosVacaciones(ctx context.Context, fecha time.Time) (TablaReporte, error) {
	filas, err := s.q.ReporteSaldosVacaciones(ctx, soloFecha(fecha))
	return TablaReporte{Reporte: "saldos_vacaciones",
		Columnas: []string{"codigo_interno", "nombre_completo", "fecha_corte", "dias_acumulados", "dias_disfrutados", "saldo"},
		Filas:    filas}, err
}

// RetencionesTerceros: embargos y préstamos retenidos en el año o el mes, por
// tercero (lo que se debe girar a juzgados y entidades).
func (s *ReportesGestionService) RetencionesTerceros(ctx context.Context, anio, mes int) (TablaReporte, error) {
	desde, hasta, err := rangoReporte(anio, mes)
	if err != nil {
		return TablaReporte{}, err
	}
	filas, err := s.q.ReporteRetencionesTerceros(ctx, desde, hasta)
	return TablaReporte{Reporte: "retenciones_terceros",
		Columnas: []string{"tercero", "tipo_tercero", "origen", "referencia", "codigo_interno", "nombre_completo", "monto"},
		Filas:    filas}, err
}

// rangoReporte: el año completo (mes = 0) o el mes indicado.
func rangoReporte(anio, mes int) (time.Time, time.Time, error) {
	if anio < 1990 || anio > 2100 || mes < 0 || mes > 12 {
		return time.Time{}, time.Time{}, ErrRangoInvalido
	}
	if mes == 0 {
		desde := time.Date(anio, 1, 1, 0, 0, 0, 0, time.UTC)
		return desde, desde.AddDate(1, 0, -1), nil
	}
	desde := time.Date(anio, time.Month(mes), 1, 0, 0, 0, 0, time.UTC)
	return desde, desde.AddDate(0, 1, -1), nil
}
