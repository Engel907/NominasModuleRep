package planilla

import (
	"bytes"
	"context"
	"fmt"

	"github.com/go-pdf/fpdf"
	"github.com/shopspring/decimal"
)

// ObtenerBoletaPagoPDFAutorizada: la boleta autorizada, renderizada en PDF.
func (s *ExportService) ObtenerBoletaPagoPDFAutorizada(ctx context.Context, periodoID, empleadoID, empleadoUsuarioID int64, permisos []string) ([]byte, error) {
	b, err := s.ObtenerBoletaPagoAutorizada(ctx, periodoID, empleadoID, empleadoUsuarioID, permisos)
	if err != nil {
		return nil, err
	}
	return RenderBoletaPDF(b)
}

// RenderBoletaPDF arma el PDF (A4, fuentes estándar con traducción a cp1252
// para tildes y eñes).
func RenderBoletaPDF(b *BoletaPagoOutput) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("planilla: boleta vacía")
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCreationDate(b.GeneradaEn)
	pdf.SetModificationDate(b.GeneradaEn)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetTitle(tr(fmt.Sprintf("Boleta de pago — período %d", b.Encabezado.PeriodoID)), false)
	pdf.AddPage()

	e := b.Encabezado
	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 8, tr("Boleta de pago"), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Período de planilla: %d", e.PeriodoID)), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Empleado: %s (%s)", e.NombreCompleto, e.Cedula)), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, tr("Salario base del período: "+monto(e.SalarioBasePeriodo)), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	tabla := func(titulo string, filas [][2]string) {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(0, 7, tr(titulo), "B", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		if len(filas) == 0 {
			pdf.CellFormat(0, 6, tr("(sin movimientos)"), "", 1, "L", false, 0, "")
		}
		for _, f := range filas {
			pdf.CellFormat(140, 6, tr(f[0]), "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 6, f[1], "", 1, "R", false, 0, "")
		}
		pdf.Ln(2)
	}
	conceptos := func(cs []ConceptoBoletaPago) [][2]string {
		out := make([][2]string, 0, len(cs))
		for _, c := range cs {
			nombre := c.NombreConcepto
			if !c.Cantidad.IsZero() {
				nombre += " (" + c.Cantidad.String() + ")"
			}
			out = append(out, [2]string{nombre, monto(c.Monto)})
		}
		return out
	}
	tabla("Ingresos", conceptos(b.Ingresos))
	tabla("Deducciones", conceptos(b.Deducciones))

	emb := make([][2]string, 0, len(b.Embargos))
	for _, x := range b.Embargos {
		emb = append(emb, [2]string{fmt.Sprintf("%s (embargo %d, prioridad %d)", x.TipoEmbargoNombre, x.EmbargoID, x.OrdenPrioridad), monto(x.Monto)})
	}
	tabla("Embargos aplicados", emb)

	inc := make([][2]string, 0, len(b.Incapacidades))
	for _, x := range b.Incapacidades {
		inc = append(inc, [2]string{
			fmt.Sprintf("Boleta %s del %s al %s, días %d–%d (%s días); CCSS/INS informativo %s",
				x.NumeroBoleta, x.FechaInicio.Format("02/01/2006"), x.FechaFin.Format("02/01/2006"),
				x.DiaSerieDesde, x.DiaSerieHasta, x.Dias.String(), monto(x.MontoEntidadInformativo)),
			monto(x.MontoPatronal)})
	}
	tabla("Incapacidades (subsidio patronal)", inc)

	pdf.SetFont("Helvetica", "B", 11)
	for _, f := range [][2]string{
		{"Total ingresos", monto(e.TotalIngresos)},
		{"Total deducciones", monto(e.TotalDeducciones)},
		{"Neto pagado", monto(e.NetoPagado)},
	} {
		pdf.CellFormat(140, 7, tr(f[0]), "T", 0, "L", false, 0, "")
		pdf.CellFormat(0, 7, f[1], "T", 1, "R", false, 0, "")
	}
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.CellFormat(0, 5, tr("Generada el "+b.GeneradaEn.Format("02/01/2006 15:04")+". Montos en colones."), "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("planilla: generando PDF de la boleta: %w", err)
	}
	return buf.Bytes(), nil
}

func monto(d decimal.Decimal) string { return d.StringFixed(2) }
