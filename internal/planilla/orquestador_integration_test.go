//go:build integration

package planilla

import (
	"context"
	"errors"
	"testing"
	"time"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
)

// Adaptadores de planilla/infra re-declarados aquí: infra importa planilla, así
// que este test interno no puede importarlo. Deben reflejar infra tal cual.
type motorRepoSQL struct{ q *dto.Queries }

func (r motorRepoSQL) ObtenerContratoVigenteEnFecha(ctx context.Context, emp int64, f time.Time) (ContratoVigente, error) {
	x, err := r.q.ObtenerContratoVigenteEnFecha(ctx, emp, f)
	return ContratoVigente(x), err
}
func (r motorRepoSQL) ListarConceptosAplicablesEnFecha(ctx context.Context, emp, reg, tipo int64, f time.Time) ([]ConceptoAplicableFila, error) {
	rows, err := r.q.ListarConceptosAplicablesEnFecha(ctx, emp, reg, tipo, f)
	if err != nil {
		return nil, err
	}
	out := make([]ConceptoAplicableFila, 0, len(rows))
	for _, x := range rows {
		c := ConceptoAplicableFila{
			ConceptoOrdenCalculo: ConceptoOrdenCalculo{ConceptoSalarialID: x.ConceptoSalarialID, Codigo: x.Codigo, OrdenCalculo: x.OrdenCalculo,
				AfectaBaseCCSS: x.AfectaBaseCCSS, AfectaBaseRenta: x.AfectaBaseRenta, AfectaBaseAguinaldo: x.AfectaBaseAguinaldo,
				AfectaBaseCesantia: x.AfectaBaseCesantia, AfectaBaseVacaciones: x.AfectaBaseVacaciones, AfectaBaseEmbargo: x.AfectaBaseEmbargo,
				EsDeduccionObligatoria: x.EsDeduccionObligatoria},
			Naturaleza: x.Naturaleza, TipoCalculo: x.TipoCalculo, FormulaID: x.FormulaID, ExpresionFormula: x.ExpresionFormula,
			FormulaEmpleadoID: x.FormulaEmpleadoID,
		}
		if x.MontoFijo.Valid {
			m := x.MontoFijo.Decimal
			c.MontoFijo = &m
		}
		out = append(out, c)
	}
	return out, nil
}
func (r motorRepoSQL) ListarParametrosHorasExtraEnFecha(ctx context.Context, f time.Time) ([]ParametroHoraExtra, error) {
	rows, err := r.q.ListarParametrosHorasExtraEnFecha(ctx, f)
	out := make([]ParametroHoraExtra, 0, len(rows))
	for _, x := range rows {
		out = append(out, ParametroHoraExtra{TipoHoraExtraID: x.TipoHoraExtraID, FactorRecargo: x.FactorRecargo})
	}
	return out, err
}

func (f *fixtureCalculo) sembrarSalarioMinimoInembargable(t *testing.T) {
	t.Helper()
	mustExec(t, f.db, `
INSERT INTO parametros_generales (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, creado_por)
VALUES (?, '300000', 'decimal', 'fixture de prueba', 'FIXTURE DE PRUEBA', '2026-01-01', 1)`, parametros.ClaveSalarioMinimoInembargable)
}

func (f *fixtureCalculo) servicio() *PeriodoService {
	q := dto.New(f.db)
	w := &EscritorDetalle{}
	repo := parametros.NewRepositorioSQL(f.db)
	gen := parametros.NewGenerales(repo, 0).ConValoresPorEmpleado(repo)
	orq := NewOrquestador(NewHidratador(q), NewMotorEmpleado(motorRepoSQL{q}),
		NewCargadorParametros(fuenteCargadorSQL{q}, gen).
			ConAjustesPorEmpleado(q, func(id int64) GeneralesLector { return gen.ParaEmpleado(id) }), w, q, NewVentanas(gen))
	return NewPeriodoService(f.db, PeriodoGanchos{Calculador: orq, Eliminador: w,
		AlAprobar: GanchosAlAprobar{NewAplicadorPrestamos(), NewAplicadorAcumulados(), NewAplicadorVacaciones()}})
}

type fuenteCargadorSQL struct{ *dto.Queries }

func (s fuenteCargadorSQL) ListarBaseMinimaContributivaEnFecha(ctx context.Context, fecha time.Time) ([]BaseMinimaContributivaFila, error) {
	rows, err := s.Queries.ListarBaseMinimaContributivaEnFecha(ctx, fecha)
	out := make([]BaseMinimaContributivaFila, 0, len(rows))
	for _, r := range rows {
		out = append(out, BaseMinimaContributivaFila(r))
	}
	return out, err
}

func (s fuenteCargadorSQL) ListarEscalaEmbargoEnFecha(ctx context.Context, fecha time.Time) ([]TramoEscalaEmbargo, error) {
	rows, err := s.Queries.ListarEscalaEmbargoEnFecha(ctx, fecha)
	out := make([]TramoEscalaEmbargo, 0, len(rows))
	for i, r := range rows {
		out = append(out, TramoEscalaEmbargo{OrdenTramo: int32(i + 1), SalarioDesde: r.DesdeMultiplo, SalarioHasta: r.HastaMultiplo, Porcentaje: r.PorcentajeMaximo})
	}
	return out, err
}

func marzo(t *testing.T, svc *PeriodoService, f *fixtureCalculo) dto.PeriodoCicloRow {
	t.Helper()
	p, err := svc.Crear(context.Background(), f.preparador, f.tipoMensual,
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCicloCompletoTresEmpleados(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	svc := f.servicio()
	ctx := context.Background()
	p := marzo(t, svc, f)

	p, err := svc.Calcular(ctx, f.preparador, p.ID)
	if err != nil {
		var ce *ErrCalculoConErrores
		if errors.As(err, &ce) {
			t.Fatalf("calcular: %+v", ce.Errores)
		}
		t.Fatalf("calcular: %v", err)
	}
	if p.Estado != string(EstadoCalculada) || p.VersionCalculo != 1 {
		t.Fatalf("estado/versión = %s/%d", p.Estado, p.VersionCalculo)
	}
	var cuadra int
	var version string
	var snapshotValido int
	if err := f.db.QueryRow(`SELECT fn_detalle_cuadra(id), version_motor, JSON_VALID(parametros_snapshot) FROM periodos_planilla WHERE id = ?`, p.ID).
		Scan(&cuadra, &version, &snapshotValido); err != nil {
		t.Fatal(err)
	}
	if cuadra != 1 || version != VersionMotor || snapshotValido != 1 {
		t.Fatalf("cuadra=%d version=%q snapshot_valido=%d", cuadra, version, snapshotValido)
	}
	var pctObrero string
	if err := f.db.QueryRow(`SELECT JSON_UNQUOTE(JSON_EXTRACT(pp.parametros_snapshot, CONCAT('$.parametros.CargasSociales."', cs.id, '"."0".PorcentajeObrero')))
		FROM periodos_planilla pp JOIN conceptos_salariales cs ON cs.codigo = 'CCSS_OBR' WHERE pp.id = ?`, p.ID).
		Scan(&pctObrero); err != nil || pctObrero != "0.1083" {
		t.Errorf("snapshot debe guardar el %% obrero en fracción: %q, %v", pctObrero, err)
	}

	n := f.contar(t, p.ID)
	if n["planilla_detalle_empleado"] != 3 || n["planilla_detalle_patronal"] != 3 || n["planilla_detalle_embargo"] != 1 || n["planilla_detalle_prestamo"] != 1 {
		t.Fatalf("filas de detalle: %v", n)
	}
	monto := func(emp int64, codigo string) string {
		var m string
		err := f.db.QueryRow(`
SELECT pdc.monto FROM planilla_detalle_concepto pdc
JOIN planilla_detalle_empleado pde ON pde.id = pdc.planilla_detalle_empleado_id
JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
WHERE pde.periodo_planilla_id = ? AND pde.empleado_id = ? AND cs.codigo = ?`, p.ID, emp, codigo).Scan(&m)
		if err != nil {
			t.Fatalf("línea %s del empleado %d: %v", codigo, emp, err)
		}
		return m
	}
	casos := []struct {
		emp           int64
		codigo, monto string
	}{
		{f.emp[0], "SAL_BASE", "900000.00"}, {f.emp[0], "CCSS_OBR", "97470.00"},
		{f.emp[1], "PRESTAMO", "25000.00"},
		{f.emp[2], "HRS_EXTRA", "26250.00"},
	}
	for _, c := range casos {
		if got := monto(c.emp, c.codigo); got != c.monto {
			t.Errorf("empleado %d %s = %s, se esperaba %s", c.emp, c.codigo, got, c.monto)
		}
	}
	var patronal string
	if err := f.db.QueryRow(`SELECT cuota_patronal_ccss FROM planilla_detalle_empleado WHERE periodo_planilla_id = ? AND empleado_id = ?`, p.ID, f.emp[0]).Scan(&patronal); err != nil || patronal != "241470.00" {
		t.Errorf("cuota patronal persistida = %s, %v (900 000 × 26.83 %%)", patronal, err)
	}

	if p, err = svc.Recalcular(ctx, f.preparador, p.ID); err != nil {
		t.Fatalf("recalcular: %v", err)
	}
	if n := f.contar(t, p.ID); n["planilla_detalle_empleado"] != 3 || n["planilla_detalle_concepto"] == 0 || p.VersionCalculo != 2 {
		t.Fatalf("tras recalcular: %v, versión %d", n, p.VersionCalculo)
	}

	if _, err := svc.Aprobar(ctx, f.preparador, p.ID); !errors.Is(err, ErrSegregacionFunciones) {
		t.Fatalf("quien calculó no puede aprobar: %v", err)
	}
	if _, err := svc.Aprobar(ctx, f.aprobador, p.ID); err != nil {
		t.Fatalf("aprobar: %v", err)
	}
	if p, err = svc.Pagar(ctx, f.pagador, p.ID); err != nil || p.Estado != string(EstadoPagada) {
		t.Fatalf("pagar: %v (estado %s)", err, p.Estado)
	}
}

func TestEmpleadoConErrorNoPersisteNada(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	f.sembrarSalarioMinimoInembargable(t)
	mustExec(t, f.db, `DELETE FROM empleado_datos_laborales WHERE empleado_id = ?`, f.emp[1])
	f.crearIncapacidad(t)
	mustExec(t, f.db, `
INSERT INTO incapacidades (empleado_id, entidad_emisora, numero_boleta, fecha_inicio, fecha_fin, dias, creado_por)
VALUES (?, 'CCSS', 'B-CALC-2', '2026-03-03', '2026-03-05', 3, ?)`, f.emp[2], f.preparador)
	svc := f.servicio()
	p := marzo(t, svc, f)

	_, err := svc.Calcular(context.Background(), f.preparador, p.ID)
	var ce *ErrCalculoConErrores
	if !errors.As(err, &ce) {
		t.Fatalf("se esperaba ErrCalculoConErrores, se obtuvo %v", err)
	}
	codigos := map[int64]string{}
	for _, e := range ce.Errores {
		codigos[e.EmpleadoID] = e.Codigo
	}
	if len(ce.Errores) != 2 || codigos[f.emp[1]] != "empleado_sin_datos_laborales" || codigos[f.emp[2]] != "datos_incompletos" {
		t.Fatalf("errores reportados: %+v", ce.Errores)
	}
	for tabla, c := range f.contar(t, p.ID) {
		if c != 0 {
			t.Errorf("%s tiene %d filas: el cálculo fallido no debe persistir nada", tabla, c)
		}
	}
	var estado string
	var snapshot *string
	if err := f.db.QueryRow(`SELECT estado, parametros_snapshot FROM periodos_planilla WHERE id = ?`, p.ID).Scan(&estado, &snapshot); err != nil {
		t.Fatal(err)
	}
	if estado != string(EstadoBorrador) || snapshot != nil {
		t.Errorf("el período debe quedar intacto: estado=%s snapshot=%v", estado, snapshot)
	}
}

func TestSinSalarioMinimoInembargableEsErrorDeParametros(t *testing.T) {
	f := nuevoFixtureCalculo(t)
	svc := f.servicio()
	p := marzo(t, svc, f)
	_, err := svc.Calcular(context.Background(), f.preparador, p.ID)
	var ec *ErrorCiclo
	if !errors.As(err, &ec) || ec.Codigo != ErrParametrosIncompletos.Codigo {
		t.Fatalf("se esperaba parametros_incompletos, se obtuvo %v", err)
	}
}
