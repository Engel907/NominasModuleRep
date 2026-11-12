package planilla

import "github.com/shopspring/decimal"

// reglaPrestamo (prestamo): cuotas de los préstamos activos del empleado.
type reglaPrestamo struct{}

func (reglaPrestamo) Aplicar(rc *ContextoRegla) error {
	st := rc.St
	st.usados[formulaPrestamo] = true
	monto := CalcularPrestamos(rc.In.Prestamos)
	st.res.DeduccionesPrestamos = monto
	if monto.IsPositive() {
		st.agregar(rc.Concepto, monto, decimal.Zero)
	}
	return nil
}
