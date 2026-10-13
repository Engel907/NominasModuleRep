package contabilidad

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/planilla"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// IDs de prueba
const (
	cSalBase, cHrsExtra, cComisiones   int64 = 1, 2, 3
	cCcssObr, cRenta, cCcssPat, cInsRT int64 = 10, 11, 16, 17
	tGasto, tPasivo                    int64 = 1, 2
	prog                               int64 = 1
)

func cfg() ConfigGeneracion {
	return ConfigGeneracion{ConceptoAnclaNeto: "SAL_BASE", ConceptoCuotaPatronalCCSS: "CCSS_PAT"}
}

func m(conceptoID int64, cod string, tipo int64, cuenta int64, mov string) MapeoConceptoCuenta {
	tc := "DEVENGO_GASTO_PLANILLA"
	if tipo == tPasivo {
		tc = "DEVENGO_PASIVO_PLANILLA"
	}
	return MapeoConceptoCuenta{ConceptoSalarialID: conceptoID, ConceptoCodigo: cod, ProgramaPresupuestarioID: prog,
		TipoAsientoID: tipo, TipoAsientoCodigo: tc, CuentaContableID: cuenta, TipoMovimiento: mov, Proyecto: "99"}
}

// Réplica de la forma del seed de mapeo_concepto_cuenta (5101-5106 / 2101-2106).
func mapeosSeed() []MapeoConceptoCuenta {
	return []MapeoConceptoCuenta{
		m(cSalBase, "SAL_BASE", tGasto, 5101, MovimientoDebe),
		m(cHrsExtra, "HRS_EXTRA", tGasto, 5102, MovimientoDebe),
		m(cCcssPat, "CCSS_PAT", tGasto, 5106, MovimientoDebe),
		m(cInsRT, "INS_RT", tGasto, 5106, MovimientoDebe), // sin contrapartida 'haber' en el seed
		m(cCcssObr, "CCSS_OBR", tPasivo, 2101, MovimientoHaber),
		m(cCcssPat, "CCSS_PAT", tPasivo, 2101, MovimientoHaber),
		m(cRenta, "IMP_RENTA", tPasivo, 2102, MovimientoHaber),
		m(cSalBase, "SAL_BASE", tPasivo, 2106, MovimientoHaber), // neto
	}
}

func linea(id int64, cod, monto string) planilla.ConceptoCalculadoOutput {
	return planilla.ConceptoCalculadoOutput{ConceptoSalarialID: id, Codigo: cod, Monto: d(monto)}
}

// Mismos números que TestCalcularEmpleado_FlujoCompleto de motor_calculo_test.go.
func detalleEjemplo(emp int64) DetallePlanilla {
	return DetallePlanilla{ProgramaPresupuestarioID: prog, Calculo: &planilla.CalculoEmpleadoDetalle{
		ResultadoEmpleadoOutput: planilla.ResultadoEmpleadoOutput{
			EmpleadoID: emp, PeriodoID: 5,
			TotalIngresos: d("1062500"), TotalDeducciones: d("129518.75"), NetoPagado: d("932981.25"),
			CuotaPatronalCCSS: d("285068.75"),
			DesglosePorConcepto: []planilla.ConceptoCalculadoOutput{
				linea(cSalBase, "SAL_BASE", "1000000"), linea(cHrsExtra, "HRS_EXTRA", "62500"),
				linea(cCcssObr, "CCSS_OBR", "115068.75"), linea(cRenta, "IMP_RENTA", "14450"),
			},
		}}}
}

func TestGenerarAsientos_CuadraYAgrupa(t *testing.T) {
	as, err := GenerarAsientos(5, []DetallePlanilla{detalleEjemplo(7), detalleEjemplo(8)}, mapeosSeed(), cfg())
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 2 || as[0].TipoAsientoID != tGasto || as[1].TipoAsientoID != tPasivo {
		t.Fatalf("se esperaban 2 asientos (gasto, pasivo): %+v", as)
	}
	// Por empleado: gasto = 1.062.500 + 285.068,75 = 1.347.568,75 ; x2 empleados
	if !as[0].TotalDebe().Equal(d("2695137.50")) || !as[0].TotalHaber().IsZero() {
		t.Errorf("gasto debe=%s haber=%s", as[0].TotalDebe(), as[0].TotalHaber())
	}
	if !as[1].TotalHaber().Equal(d("2695137.50")) || !as[1].TotalDebe().IsZero() {
		t.Errorf("pasivo debe=%s haber=%s", as[1].TotalDebe(), as[1].TotalHaber())
	}
	// Consolidado: 3 líneas de gasto (5101, 5102, 5106) y 4 de pasivo (CCSS_OBR y CCSS_PAT en 2101 como conceptos distintos, 2102, 2106)
	if len(as[0].Lineas) != 3 || len(as[1].Lineas) != 4 {
		t.Errorf("líneas gasto=%d pasivo=%d", len(as[0].Lineas), len(as[1].Lineas))
	}
	for _, l := range as[1].Lineas {
		if l.CuentaContableID == 2106 && !l.MontoHaber.Equal(d("1865962.50")) {
			t.Errorf("neto consolidado = %s", l.MontoHaber)
		}
		if l.EmpleadoID != nil {
			t.Errorf("las líneas consolidadas deben llevar empleado_id NULL")
		}
	}
	// SAL_BASE en el desglose NO debe duplicar la línea de neto 2106.
	if err := ValidarPartidaDoble(as); err != nil {
		t.Fatal(err)
	}
}

func TestGenerarAsientos_MapeoFaltanteFalla(t *testing.T) {
	det := detalleEjemplo(7)
	det.Calculo.DesglosePorConcepto = append(det.Calculo.DesglosePorConcepto, linea(cComisiones, "COMISIONES", "1000"))
	_, err := GenerarAsientos(5, []DetallePlanilla{det}, mapeosSeed(), cfg())
	if !errors.Is(err, ErrMapeoFaltante) {
		t.Fatalf("esperaba ErrMapeoFaltante, obtuve %v", err)
	}
}

func TestGenerarAsientos_ConceptoSinContrapartidaDescuadra(t *testing.T) {
	// INS_RT (patronal manual) solo tiene mapeo 'debe' en el seed: debe fallar, no ajustarse.
	det := detalleEjemplo(7)
	det.Calculo.DesglosePorConcepto = append(det.Calculo.DesglosePorConcepto, linea(cInsRT, "INS_RT", "5000"))
	_, err := GenerarAsientos(5, []DetallePlanilla{det}, mapeosSeed(), cfg())
	var de *DescuadreError
	if !errors.Is(err, ErrDescuadre) || !errors.As(err, &de) || de.Alcance != "empleado_id=7" {
		t.Fatalf("esperaba DescuadreError del empleado 7, obtuve %v", err)
	}
}

func TestGenerarAsientos_SinMapeoDeNetoFalla(t *testing.T) {
	var sinNeto []MapeoConceptoCuenta
	for _, x := range mapeosSeed() {
		if !(x.ConceptoCodigo == "SAL_BASE" && x.TipoMovimiento == MovimientoHaber) {
			sinNeto = append(sinNeto, x)
		}
	}
	_, err := GenerarAsientos(5, []DetallePlanilla{detalleEjemplo(7)}, sinNeto, cfg())
	if !errors.Is(err, ErrMapeoFaltante) {
		t.Fatalf("esperaba ErrMapeoFaltante, obtuve %v", err)
	}
}

func TestGenerarAsientos_ValidacionesDeEntrada(t *testing.T) {
	if _, err := GenerarAsientos(5, []DetallePlanilla{detalleEjemplo(7)}, mapeosSeed(), ConfigGeneracion{}); !errors.Is(err, ErrConfigInvalida) {
		t.Errorf("config vacía: %v", err)
	}
	if _, err := GenerarAsientos(5, nil, mapeosSeed(), cfg()); !errors.Is(err, ErrSinDetalles) {
		t.Errorf("sin detalles: %v", err)
	}
	sinProg := detalleEjemplo(7)
	sinProg.ProgramaPresupuestarioID = 0
	if _, err := GenerarAsientos(5, []DetallePlanilla{sinProg}, mapeosSeed(), cfg()); err == nil {
		t.Error("esperaba error por programa faltante")
	}
	if _, err := GenerarAsientos(5, []DetallePlanilla{detalleEjemplo(7), detalleEjemplo(7)}, mapeosSeed(), cfg()); err == nil {
		t.Error("esperaba error por empleado duplicado")
	}
	otroPeriodo := detalleEjemplo(7)
	otroPeriodo.Calculo.PeriodoID = 99
	if _, err := GenerarAsientos(5, []DetallePlanilla{otroPeriodo}, mapeosSeed(), cfg()); err == nil {
		t.Error("esperaba error por período distinto")
	}
}

func TestValidarPartidaDoble(t *testing.T) {
	ok := []AsientoContable{
		{TipoAsientoID: 1, Lineas: []AsientoContableDetalle{{ProgramaPresupuestarioID: 1, MontoDebe: d("10")}}},
		{TipoAsientoID: 2, Lineas: []AsientoContableDetalle{{ProgramaPresupuestarioID: 1, MontoHaber: d("10")}}},
	}
	if err := ValidarPartidaDoble(ok); err != nil {
		t.Fatal(err)
	}
	desc := []AsientoContable{
		{TipoAsientoID: 1, Lineas: []AsientoContableDetalle{{ProgramaPresupuestarioID: 1, MontoDebe: d("10")}}},
		{TipoAsientoID: 2, Lineas: []AsientoContableDetalle{{ProgramaPresupuestarioID: 1, MontoHaber: d("9.99")}}},
	}
	if err := ValidarPartidaDoble(desc); !errors.Is(err, ErrDescuadre) {
		t.Errorf("descuadre de 0.01 no detectado: %v", err)
	}
	// Cuadra en total pero no por programa
	porProg := []AsientoContable{
		{TipoAsientoID: 1, Lineas: []AsientoContableDetalle{{ProgramaPresupuestarioID: 1, MontoDebe: d("10")}}},
		{TipoAsientoID: 2, Lineas: []AsientoContableDetalle{{ProgramaPresupuestarioID: 2, MontoHaber: d("10")}}},
	}
	if err := ValidarPartidaDoble(porProg); !errors.Is(err, ErrDescuadre) {
		t.Errorf("descuadre por programa no detectado: %v", err)
	}
	// Línea que violaría el CHECK de la BD
	ambas := []AsientoContable{{TipoAsientoID: 1, Lineas: []AsientoContableDetalle{{ProgramaPresupuestarioID: 1, MontoDebe: d("5"), MontoHaber: d("5")}}}}
	if err := ValidarPartidaDoble(ambas); !errors.Is(err, ErrMontoNegativoOInvalido) {
		t.Errorf("línea debe+haber no detectada: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Servicio con repositorio fake (verifica que un fallo no persiste nada)
// ---------------------------------------------------------------------------

type fakeRepo struct {
	periodo    PeriodoContable
	existentes int64
	mapeos     []MapeoConceptoCuenta
	guardados  []AsientoContable
	marcado    bool
	marcadoDe  string // estado desde el que se marcó
	usuarioTx  int64  // usuario con el que se abrió la tx auditada
	txAbiertas int
}

func (f *fakeRepo) EjecutarEnTx(ctx context.Context, usuarioID int64, fn func(RepositorioTx) error) error {
	f.txAbiertas++
	f.usuarioTx = usuarioID
	if err := fn(f); err != nil {
		f.guardados, f.marcado = nil, false // simula rollback
		return err
	}
	return nil
}
func (f *fakeRepo) ObtenerPeriodoParaContabilizar(context.Context, int64) (PeriodoContable, error) {
	return f.periodo, nil
}
func (f *fakeRepo) ContarAsientosDelPeriodo(context.Context, int64) (int64, error) {
	return f.existentes, nil
}
func (f *fakeRepo) ListarMapeosVigentesEnFecha(context.Context, time.Time) ([]MapeoConceptoCuenta, error) {
	return f.mapeos, nil
}
func (f *fakeRepo) GuardarAsiento(_ context.Context, a AsientoContable) (int64, error) {
	f.guardados = append(f.guardados, a)
	return int64(100 + len(f.guardados)), nil
}
func (f *fakeRepo) MarcarPeriodoContabilizado(_ context.Context, _ int64, estadoActual string) error {
	f.marcado, f.marcadoDe = true, estadoActual
	return nil
}

func TestContabilizarPeriodo(t *testing.T) {
	base := func() *fakeRepo {
		return &fakeRepo{periodo: PeriodoContable{ID: 5, FechaFin: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), Estado: "pagada"}, mapeos: mapeosSeed()}
	}
	svc := func(r *fakeRepo) *Service { s, _ := NewService(r, cfg()); return s }

	t.Run("ok persiste 2 asientos y marca el período", func(t *testing.T) {
		r := base()
		as, err := svc(r).ContabilizarPeriodo(context.Background(), 5, 42, []DetallePlanilla{detalleEjemplo(7)})
		if err != nil || len(as) != 2 || len(r.guardados) != 2 || !r.marcado {
			t.Fatalf("err=%v as=%d guardados=%d marcado=%v", err, len(as), len(r.guardados), r.marcado)
		}
		// Misma transacción auditada: una sola tx, con el usuario real, y la
		// transición sale de 'pagada'.
		if r.txAbiertas != 1 || r.usuarioTx != 42 || r.marcadoDe != "pagada" {
			t.Errorf("tx=%d usuario=%d marcadoDe=%q", r.txAbiertas, r.usuarioTx, r.marcadoDe)
		}
		if as[0].NumeroAsiento == "" || as[0].ID == 0 || as[0].Lineas[0].AsientoContableID != as[0].ID {
			t.Errorf("asiento sin folio/id: %+v", as[0])
		}
	})
	t.Run("descuadre no inserta nada", func(t *testing.T) {
		r := base()
		det := detalleEjemplo(7)
		det.Calculo.DesglosePorConcepto = append(det.Calculo.DesglosePorConcepto, linea(cInsRT, "INS_RT", "5000"))
		_, err := svc(r).ContabilizarPeriodo(context.Background(), 5, 1, []DetallePlanilla{det})
		if !errors.Is(err, ErrDescuadre) || len(r.guardados) != 0 || r.marcado {
			t.Fatalf("err=%v guardados=%d marcado=%v", err, len(r.guardados), r.marcado)
		}
	})
	// D4: solo 'pagada' se contabiliza. Cualquier otro estado → error de estado
	// y NADA persistido ni marcado.
	for _, estado := range []string{"borrador", "calculada", "aprobada", "contabilizada", "cerrado", ""} {
		estado := estado
		t.Run("estado inválido: "+estado, func(t *testing.T) {
			r := base()
			r.periodo.Estado = estado
			_, err := svc(r).ContabilizarPeriodo(context.Background(), 5, 1, []DetallePlanilla{detalleEjemplo(7)})
			if !errors.Is(err, ErrPeriodoEstadoInvalido) || len(r.guardados) != 0 || r.marcado {
				t.Fatalf("err=%v guardados=%d marcado=%v", err, len(r.guardados), r.marcado)
			}
		})
	}
	t.Run("asientos ya generados", func(t *testing.T) {
		r := base()
		r.existentes = 2
		if _, err := svc(r).ContabilizarPeriodo(context.Background(), 5, 1, []DetallePlanilla{detalleEjemplo(7)}); !errors.Is(err, ErrAsientosYaGenerados) {
			t.Fatalf("err=%v", err)
		}
	})
}

// Neto que no corresponde a ingresos - deducciones: la partida doble lo debe
// detectar (el motor/BD podrían traer un neto alterado) y no persistir nada.
func TestContabilizarPeriodo_NetoAlteradoDescuadra(t *testing.T) {
	r := &fakeRepo{periodo: PeriodoContable{ID: 5, FechaFin: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), Estado: "pagada"}, mapeos: mapeosSeed()}
	s, _ := NewService(r, cfg())
	det := detalleEjemplo(7)
	det.Calculo.NetoPagado = d("932981.26") // +0.01
	_, err := s.ContabilizarPeriodo(context.Background(), 5, 1, []DetallePlanilla{det})
	var de *DescuadreError
	if !errors.As(err, &de) || !errors.Is(err, ErrDescuadre) {
		t.Fatalf("esperaba DescuadreError, obtuve %v", err)
	}
	if !de.TotalHaber.Sub(de.TotalDebe).Equal(d("0.01")) {
		t.Errorf("diferencia = %s, esperaba 0.01", de.TotalHaber.Sub(de.TotalDebe))
	}
	if len(r.guardados) != 0 || r.marcado {
		t.Errorf("no debía persistir nada: guardados=%d marcado=%v", len(r.guardados), r.marcado)
	}
}

// Los montos con más de 2 decimales se redondean ANTES de validar/persistir
// (las columnas son DECIMAL(14,2)): ninguna línea puede llevar más precisión
// que la que la base va a conservar.
func TestGenerarAsientos_RedondeaADosDecimales(t *testing.T) {
	det := detalleEjemplo(7)
	det.Calculo.DesglosePorConcepto[1] = linea(cHrsExtra, "HRS_EXTRA", "62500.004")
	det.Calculo.CuotaPatronalCCSS = d("285068.754")
	det.Calculo.NetoPagado = d("932981.254")
	as, err := GenerarAsientos(5, []DetallePlanilla{det}, mapeosSeed(), cfg())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		for _, l := range a.Lineas {
			if !l.MontoDebe.Equal(l.MontoDebe.Round(2)) || !l.MontoHaber.Equal(l.MontoHaber.Round(2)) {
				t.Errorf("línea con más de 2 decimales: debe=%s haber=%s", l.MontoDebe, l.MontoHaber)
			}
		}
	}
	if !as[0].TotalDebe().Equal(d("1347568.75")) || !as[1].TotalHaber().Equal(d("1347568.75")) {
		t.Errorf("totales gasto=%s pasivo=%s", as[0].TotalDebe(), as[1].TotalHaber())
	}
}

// Un monto que redondea a cero no genera línea (violaría el CHECK > 0).
func TestGenerarAsientos_MontoQueRedondeaACeroSeOmite(t *testing.T) {
	det := detalleEjemplo(7)
	det.Calculo.DesglosePorConcepto[1] = linea(cHrsExtra, "HRS_EXTRA", "0.004")
	det.Calculo.TotalIngresos = d("1000000")
	det.Calculo.NetoPagado = d("870481.25") // 1.000.000 - 129.518,75
	as, err := GenerarAsientos(5, []DetallePlanilla{det}, mapeosSeed(), cfg())
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range as[0].Lineas {
		if l.CuentaContableID == 5102 {
			t.Errorf("no debía existir línea de HRS_EXTRA: %+v", l)
		}
	}
}

// La cuota patronal negativa se rechaza aunque redondee a cero.
func TestGenerarAsientos_PatronalNegativaFalla(t *testing.T) {
	det := detalleEjemplo(7)
	det.Calculo.CuotaPatronalCCSS = d("-0.004")
	if _, err := GenerarAsientos(5, []DetallePlanilla{det}, mapeosSeed(), cfg()); !errors.Is(err, ErrMontoNegativoOInvalido) {
		t.Fatalf("esperaba ErrMontoNegativoOInvalido, obtuve %v", err)
	}
}
