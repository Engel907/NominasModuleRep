//go:build integration

package planilla

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/testutil"
)

func TestReportesOficiales(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	svc := f.servicio()
	ctx := context.Background()
	p := marzo(t, svc, f)
	if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
		t.Fatal(err)
	}
	rep := NewReportesService(f.db, testutil.Llavero(t))

	if _, err := rep.Generar(ctx, p.ID, ReporteCCSS, "", f.preparador); !errors.Is(err, ErrReportePeriodoNoAprobado) {
		t.Fatalf("período calculado: %v", err)
	}
	if _, err := svc.Aprobar(ctx, f.aprobador, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := rep.Generar(ctx, p.ID, ReporteCCSS, "", f.preparador); !errors.Is(err, ErrFormatoSinColumnas) {
		t.Fatalf("formato sin columnas: %v", err)
	}
	mustExec(t, f.db, `
INSERT INTO formatos_exportacion_columnas (formato_exportacion_id, orden, columna_origen, encabezado)
SELECT fe.id, v.orden, v.col, v.enc FROM formatos_exportacion fe
JOIN (SELECT 1 orden, 'identificacion' col, 'CEDULA' enc
      UNION ALL SELECT 2, 'base_ccss', 'SALARIO'
      UNION ALL SELECT 3, 'cuota_obrera_ccss', 'OBRERO'
      UNION ALL SELECT 4, 'cuota_patronal_ccss', 'PATRONO') v ON 1 = 1
WHERE fe.codigo = 'REPORTE_CCSS'`)
	a, err := rep.Generar(ctx, p.ID, ReporteCCSS, "", f.preparador)
	if err != nil {
		t.Fatal(err)
	}
	lineas := strings.Split(strings.TrimRight(string(a.Contenido), "\r\n"), "\r\n")
	if len(lineas) != 4 || lineas[0] != "CEDULA,SALARIO,OBRERO,PATRONO" || a.Filas != 3 {
		t.Fatalf("CSV inesperado (%d filas):\n%s", a.Filas, a.Contenido)
	}
	if !strings.HasPrefix(lineas[1], "CALC-0000,900000.00,97470.00,") {
		t.Errorf("primera fila: %q", lineas[1])
	}
	var hash, total string
	var filas int
	if err := f.db.QueryRow(`
SELECT eg.hash_verificacion, eg.cantidad_filas, eg.total_monto FROM exportaciones_generadas eg
JOIN formatos_exportacion fe ON fe.id = eg.formato_exportacion_id
WHERE eg.periodo_planilla_id = ? AND fe.codigo = 'REPORTE_CCSS'`, p.ID).Scan(&hash, &filas, &total); err != nil {
		t.Fatal(err)
	}
	if hash != a.HashSHA256 || filas != 3 || total != "2426250.00" {
		t.Fatalf("registro: hash %s (archivo %s), filas %d, total %s", hash, a.HashSHA256, filas, total)
	}
	if _, err := rep.Generar(ctx, p.ID, ReporteHacienda, "", f.preparador); !errors.Is(err, ErrFormatoSinColumnas) {
		t.Fatalf("hacienda sin columnas: %v", err)
	}

	// El consolidado del mes: una fila por empleado con los montos sumados.
	m, err := rep.GenerarMensual(ctx, 2026, 3, ReporteCCSS, "", f.preparador)
	if err != nil {
		t.Fatal(err)
	}
	if m.Filas != 3 || !strings.Contains(string(m.Contenido), "CALC-0000,900000.00,97470.00,") {
		t.Fatalf("mensual (%d filas):\n%s", m.Filas, m.Contenido)
	}
	if _, err := rep.GenerarMensual(ctx, 2026, 4, ReporteCCSS, "", f.preparador); !errors.Is(err, ErrMesSinPeriodos) {
		t.Fatalf("mes sin períodos: %v", err)
	}
	if _, err := rep.GenerarMensual(ctx, 2026, 13, ReporteCCSS, "", f.preparador); !errors.Is(err, ErrMesInvalido) {
		t.Fatalf("mes inválido: %v", err)
	}

	q := dto.New(f.db)
	exp := NewExportService(exportRepoSQL{q, testutil.Llavero(t)}, nil)
	pdf, err := exp.ObtenerBoletaPagoPDFAutorizada(ctx, p.ID, f.emp[0], f.emp[0], []string{authz.PermPlanillaVerBoletaPropia})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("no es un PDF: %q", pdf[:8])
	}
	if _, err := exp.ObtenerBoletaPagoPDFAutorizada(ctx, p.ID, f.emp[1], f.emp[0], []string{authz.PermPlanillaVerBoletaPropia}); !errors.Is(err, ErrBoletaNoPropia) {
		t.Fatalf("boleta ajena: %v", err)
	}
	b, err := exp.ObtenerBoletaPago(ctx, p.ID, f.emp[0])
	if err != nil || len(b.Embargos) != 1 {
		t.Fatalf("boleta: %+v %v", b, err)
	}
}

func TestBoletaConIncapacidad(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	f.crearIncapacidad(t)
	svc := f.servicio()
	ctx := context.Background()
	p := marzo(t, svc, f)
	if _, err := svc.Calcular(ctx, f.preparador, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Aprobar(ctx, f.aprobador, p.ID); err != nil {
		t.Fatal(err)
	}
	exp := NewExportService(exportRepoSQL{dto.New(f.db), testutil.Llavero(t)}, nil)
	b, err := exp.ObtenerBoletaPago(ctx, p.ID, f.emp[2])
	if err != nil || len(b.Incapacidades) == 0 || b.Incapacidades[0].NumeroBoleta != "B-CALC-1" {
		t.Fatalf("incapacidades en la boleta: %+v %v", b, err)
	}
	pdf, err := RenderBoletaPDF(b)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("pdf: %v", err)
	}
}

// exportRepoSQL: adaptador mínimo de ExportRepository para las pruebas del
// paquete (infra importa planilla: no se puede usar aquí).
type exportRepoSQL struct {
	q *dto.Queries
	d Descifrador
}

var estadosBoletaPrueba = []string{"aprobada", "pagada", "contabilizada"}

func (r exportRepoSQL) ObtenerDetallesPlanillaParaDispersion(context.Context, int64) ([]FilaDispersionBancaria, error) {
	return nil, errors.New("no usado")
}
func (r exportRepoSQL) ObtenerDatosBoletaPago(ctx context.Context, periodoID, empleadoID int64) (EncabezadoBoletaPago, error) {
	x, err := r.q.ObtenerDatosBoletaPago(ctx, periodoID, empleadoID, estadosBoletaPrueba)
	if err != nil {
		return EncabezadoBoletaPago{}, err
	}
	cedula, err := r.d.Descifrar(dto.CampoIdentificacion.Campo(), x.Cedula.Cifrado, x.Cedula.Kid.String)
	return EncabezadoBoletaPago{PlanillaDetalleEmpleadoID: x.PlanillaDetalleEmpleadoID, EmpleadoID: x.EmpleadoID, PeriodoID: x.PeriodoID,
		NombreCompleto: x.NombreCompleto, Cedula: cedula, SalarioBasePeriodo: x.SalarioBasePeriodo,
		TotalIngresos: x.TotalIngresos, TotalDeducciones: x.TotalDeducciones, NetoPagado: x.NetoPagado}, err
}
func (r exportRepoSQL) ObtenerDesgloseConceptosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]ConceptoBoletaPago, error) {
	rows, err := r.q.ObtenerDesgloseConceptosBoletaPago(ctx, periodoID, empleadoID)
	out := make([]ConceptoBoletaPago, 0, len(rows))
	for _, x := range rows {
		out = append(out, ConceptoBoletaPago{CodigoConcepto: x.CodigoConcepto, NombreConcepto: x.NombreConcepto,
			Naturaleza: x.Naturaleza, Monto: x.Monto, Cantidad: x.Cantidad, OrdenCalculo: x.OrdenCalculo})
	}
	return out, err
}
func (r exportRepoSQL) ObtenerEmbargosBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]EmbargoBoletaPago, error) {
	rows, err := r.q.ObtenerEmbargosBoletaPago(ctx, periodoID, empleadoID)
	out := make([]EmbargoBoletaPago, 0, len(rows))
	for _, x := range rows {
		out = append(out, EmbargoBoletaPago{EmbargoID: x.EmbargoID, TipoEmbargoCodigo: x.TipoEmbargoCodigo,
			TipoEmbargoNombre: x.TipoEmbargoNombre, OrdenPrioridad: x.OrdenPrioridad, Monto: x.Monto})
	}
	return out, err
}
func (r exportRepoSQL) ObtenerIncapacidadesBoletaPago(ctx context.Context, periodoID, empleadoID int64) ([]IncapacidadBoletaPago, error) {
	rows, err := r.q.ObtenerIncapacidadesBoletaPago(ctx, periodoID, empleadoID)
	out := make([]IncapacidadBoletaPago, 0, len(rows))
	for _, x := range rows {
		out = append(out, IncapacidadBoletaPago(x))
	}
	return out, err
}
