package planilla

import "github.com/shopspring/decimal"

// reglaPrestamo (prestamo): cuotas de los préstamos activos del empleado.
type reglaPrestamo struct{}

func (reglaPrestamo) Aplicar(rc *ContextoRegla) error {
	st := rc.St
	st.usados[formulaPrestamo] = true
	st.conceptoPrestamoID = rc.Concepto.ConceptoSalarialID
	monto := CalcularPrestamos(rc.In.Prestamos)
	st.res.DeduccionesPrestamos = monto
	for _, p := range rc.In.Prestamos {
		cuota := decimal.Max(decimal.Min(p.CuotaPeriodica, p.SaldoPendiente), decimal.Zero).Round(dosDecimales)
		st.prestamos = append(st.prestamos, LineaPrestamo{PrestamoID: p.PrestamoID, CuotaCalculada: cuota, MontoRetenido: cuota})
	}
	if monto.IsPositive() {
		st.agregar(rc.Concepto, monto, decimal.Zero)
	}
	return nil
}
