package contabilidad

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// Armado puro de asientos (sin I/O)
// ============================================================================

type claveLinea struct {
	tipoAsientoID int64
	programaID    int64
	cuentaID      int64
	proyecto      string
	conceptoID    int64
	movimiento    string
}

type partida struct {
	mapeo      MapeoConceptoCuenta
	conceptoID int64
	monto      decimal.Decimal
}

// GenerarAsientos convierte los resultados calculados de la corrida en
// asientos contables (uno por tipo_asiento presente en el mapeo), agrupando
// por tipo_asiento_id y programa_presupuestario_id. Es pura: recibe los
// mapeos ya resueltos a la fecha efectiva.
//
// Cómo se resuelve cada monto (siempre vía mapeo, nunca cuentas en Go):
//   - Cada línea de DesglosePorConcepto genera una línea por cada mapeo
//     vigente de (concepto, programa del empleado).
//   - La cuota patronal de CCSS (fuera del desglose) se imputa al concepto
//     cfg.ConceptoCuotaPatronalCCSS.
//   - El neto pagado se registra con el/los mapeo 'haber' del concepto ancla
//     cfg.ConceptoAnclaNeto (debe existir exactamente uno por programa).
//   - Un concepto con monto > 0 sin mapeo falla con ErrMapeoFaltante: omitirlo
//     desbalancearía el asiento.
//
// Antes de devolver, valida partida doble por empleado, por programa y de toda
// la corrida (ValidarPartidaDoble). Ante cualquier descuadre retorna error y
// no devuelve asientos.
func GenerarAsientos(periodoID int64, detalles []DetallePlanilla, mapeos []MapeoConceptoCuenta, cfg ConfigGeneracion) ([]AsientoContable, error) {
	if strings.TrimSpace(cfg.ConceptoAnclaNeto) == "" || strings.TrimSpace(cfg.ConceptoCuotaPatronalCCSS) == "" {
		return nil, fmt.Errorf("%w: ConceptoAnclaNeto y ConceptoCuotaPatronalCCSS son obligatorios", ErrConfigInvalida)
	}
	if len(detalles) == 0 {
		return nil, ErrSinDetalles
	}

	porConcepto := map[int64]map[int64][]MapeoConceptoCuenta{}
	idPorCodigo := map[string]int64{}
	for _, m := range mapeos {
		if porConcepto[m.ConceptoSalarialID] == nil {
			porConcepto[m.ConceptoSalarialID] = map[int64][]MapeoConceptoCuenta{}
		}
		porConcepto[m.ConceptoSalarialID][m.ProgramaPresupuestarioID] = append(porConcepto[m.ConceptoSalarialID][m.ProgramaPresupuestarioID], m)
		idPorCodigo[m.ConceptoCodigo] = m.ConceptoSalarialID
	}

	acum := map[claveLinea]decimal.Decimal{}
	tipoCodigo := map[int64]string{}
	vistos := map[int64]bool{}

	for _, d := range detalles {
		if d.Calculo == nil {
			return nil, fmt.Errorf("contabilidad: detalle de planilla sin cálculo")
		}
		emp := d.Calculo.EmpleadoID
		if vistos[emp] {
			return nil, fmt.Errorf("contabilidad: empleado_id=%d aparece más de una vez en la corrida", emp)
		}
		vistos[emp] = true
		if d.Calculo.PeriodoID != 0 && d.Calculo.PeriodoID != periodoID {
			return nil, fmt.Errorf("contabilidad: empleado_id=%d fue calculado para periodo_id=%d, no para %d", emp, d.Calculo.PeriodoID, periodoID)
		}
		if d.ProgramaPresupuestarioID <= 0 {
			return nil, fmt.Errorf("contabilidad: empleado_id=%d sin programa_presupuestario_id (no se asume programa por defecto)", emp)
		}

		partidas, err := partidasEmpleado(d, porConcepto, idPorCodigo, cfg)
		if err != nil {
			return nil, err
		}

		debe, haber := decimal.Zero, decimal.Zero
		for _, p := range partidas {
			if p.mapeo.TipoMovimiento == MovimientoDebe {
				debe = debe.Add(p.monto)
			} else {
				haber = haber.Add(p.monto)
			}
			k := claveLinea{p.mapeo.TipoAsientoID, d.ProgramaPresupuestarioID, p.mapeo.CuentaContableID, p.mapeo.Proyecto, p.conceptoID, p.mapeo.TipoMovimiento}
			acum[k] = acum[k].Add(p.monto)
			tipoCodigo[p.mapeo.TipoAsientoID] = p.mapeo.TipoAsientoCodigo
		}
		if !debe.Equal(haber) {
			return nil, &DescuadreError{Alcance: fmt.Sprintf("empleado_id=%d", emp), TotalDebe: debe, TotalHaber: haber}
		}
	}

	porTipo := map[int64]*AsientoContable{}
	for k, monto := range acum {
		a := porTipo[k.tipoAsientoID]
		if a == nil {
			a = &AsientoContable{PeriodoPlanillaID: periodoID, TipoAsientoID: k.tipoAsientoID, TipoAsientoCodigo: tipoCodigo[k.tipoAsientoID], Estado: EstadoAsientoGenerado}
			porTipo[k.tipoAsientoID] = a
		}
		cid := k.conceptoID
		l := AsientoContableDetalle{
			CuentaContableID: k.cuentaID, ProgramaPresupuestarioID: k.programaID, Proyecto: k.proyecto,
			ConceptoSalarialID: &cid, // EmpleadoID nil: línea consolidada
		}
		if k.movimiento == MovimientoDebe {
			l.MontoDebe = monto
		} else {
			l.MontoHaber = monto
		}
		a.Lineas = append(a.Lineas, l)
	}

	asientos := make([]AsientoContable, 0, len(porTipo))
	for _, a := range porTipo {
		ls := a.Lineas
		sort.Slice(ls, func(i, j int) bool {
			x, y := ls[i], ls[j]
			if x.ProgramaPresupuestarioID != y.ProgramaPresupuestarioID {
				return x.ProgramaPresupuestarioID < y.ProgramaPresupuestarioID
			}
			if x.MontoDebe.IsPositive() != y.MontoDebe.IsPositive() {
				return x.MontoDebe.IsPositive() // debe primero
			}
			if x.CuentaContableID != y.CuentaContableID {
				return x.CuentaContableID < y.CuentaContableID
			}
			return *x.ConceptoSalarialID < *y.ConceptoSalarialID
		})
		asientos = append(asientos, *a)
	}
	sort.Slice(asientos, func(i, j int) bool { return asientos[i].TipoAsientoID < asientos[j].TipoAsientoID })

	if err := ValidarPartidaDoble(asientos); err != nil {
		return nil, err
	}
	return asientos, nil
}

func partidasEmpleado(d DetallePlanilla, porConcepto map[int64]map[int64][]MapeoConceptoCuenta, idPorCodigo map[string]int64, cfg ConfigGeneracion) ([]partida, error) {
	emp, prog := d.Calculo.EmpleadoID, d.ProgramaPresupuestarioID
	var out []partida

	mapeosDe := func(conceptoID int64, codigo string) ([]MapeoConceptoCuenta, error) {
		ms := porConcepto[conceptoID][prog]
		if len(ms) == 0 {
			return nil, fmt.Errorf("%w: concepto %q (id=%d), programa_presupuestario_id=%d, empleado_id=%d", ErrMapeoFaltante, codigo, conceptoID, prog, emp)
		}
		return ms, nil
	}

	for _, l := range d.Calculo.DesglosePorConcepto {
		if l.Monto.IsNegative() {
			return nil, fmt.Errorf("%w: concepto %q de empleado_id=%d con monto negativo %s", ErrMontoNegativoOInvalido, l.Codigo, emp, l.Monto)
		}
		if l.Monto.IsZero() {
			continue
		}
		ms, err := mapeosDe(l.ConceptoSalarialID, l.Codigo)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			if m.ConceptoCodigo == cfg.ConceptoAnclaNeto && m.TipoMovimiento == MovimientoHaber {
				continue // esa línea es el neto, se agrega abajo
			}
			out = append(out, partida{m, l.ConceptoSalarialID, l.Monto})
		}
	}

	if pat := d.Calculo.CuotaPatronalCCSS; pat.IsNegative() {
		return nil, fmt.Errorf("%w: cuota patronal negativa en empleado_id=%d", ErrMontoNegativoOInvalido, emp)
	} else if pat.IsPositive() {
		id, ok := idPorCodigo[cfg.ConceptoCuotaPatronalCCSS]
		if !ok {
			return nil, fmt.Errorf("%w: concepto %q (cuota patronal) sin ningún mapeo vigente", ErrMapeoFaltante, cfg.ConceptoCuotaPatronalCCSS)
		}
		ms, err := mapeosDe(id, cfg.ConceptoCuotaPatronalCCSS)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			out = append(out, partida{m, id, pat})
		}
	}

	if neto := d.Calculo.NetoPagado; neto.IsNegative() {
		return nil, fmt.Errorf("%w: neto negativo en empleado_id=%d", ErrMontoNegativoOInvalido, emp)
	} else if neto.IsPositive() {
		id, ok := idPorCodigo[cfg.ConceptoAnclaNeto]
		if !ok {
			return nil, fmt.Errorf("%w: concepto ancla del neto %q sin ningún mapeo vigente", ErrMapeoFaltante, cfg.ConceptoAnclaNeto)
		}
		ms, err := mapeosDe(id, cfg.ConceptoAnclaNeto)
		if err != nil {
			return nil, err
		}
		var haberes []MapeoConceptoCuenta
		for _, m := range ms {
			if m.TipoMovimiento == MovimientoHaber {
				haberes = append(haberes, m)
			}
		}
		if len(haberes) != 1 {
			return nil, fmt.Errorf("%w: se esperaba exactamente 1 mapeo 'haber' para el neto (concepto %q, programa %d) y hay %d", ErrMapeoFaltante, cfg.ConceptoAnclaNeto, prog, len(haberes))
		}
		out = append(out, partida{haberes[0], id, neto})
	}
	return out, nil
}

// ValidarPartidaDoble verifica sobre el juego completo de asientos de una
// corrida: (1) cada línea es débito XOR crédito con monto > 0 (espejo del
// CHECK de la BD, para fallar antes de insertar); (2) ningún asiento vacío;
// (3) total debe == total haber por programa presupuestario y en toda la
// corrida.
//
// Nota: con el seed actual el devengo se reparte en dos tipos de asiento
// (gasto = solo debe, pasivo = solo haber), así que la igualdad se exige
// sobre el JUEGO de asientos de la corrida, no sobre cada uno aislado.
func ValidarPartidaDoble(asientos []AsientoContable) error {
	if len(asientos) == 0 {
		return ErrSinDetalles
	}
	type tot struct{ debe, haber decimal.Decimal }
	porPrograma := map[int64]*tot{}
	total := tot{decimal.Zero, decimal.Zero}

	for _, a := range asientos {
		if len(a.Lineas) == 0 {
			return fmt.Errorf("contabilidad: asiento tipo_asiento_id=%d sin líneas", a.TipoAsientoID)
		}
		for _, l := range a.Lineas {
			okDebe := l.MontoDebe.IsPositive() && l.MontoHaber.IsZero()
			okHaber := l.MontoHaber.IsPositive() && l.MontoDebe.IsZero()
			if !okDebe && !okHaber {
				return fmt.Errorf("%w: línea (cuenta_contable_id=%d) debe=%s haber=%s; debe ser débito XOR crédito > 0",
					ErrMontoNegativoOInvalido, l.CuentaContableID, l.MontoDebe, l.MontoHaber)
			}
			t := porPrograma[l.ProgramaPresupuestarioID]
			if t == nil {
				t = &tot{decimal.Zero, decimal.Zero}
				porPrograma[l.ProgramaPresupuestarioID] = t
			}
			t.debe, t.haber = t.debe.Add(l.MontoDebe), t.haber.Add(l.MontoHaber)
			total.debe, total.haber = total.debe.Add(l.MontoDebe), total.haber.Add(l.MontoHaber)
		}
	}
	ids := make([]int64, 0, len(porPrograma))
	for id := range porPrograma {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if t := porPrograma[id]; !t.debe.Equal(t.haber) {
			return &DescuadreError{Alcance: fmt.Sprintf("programa_presupuestario_id=%d", id), TotalDebe: t.debe, TotalHaber: t.haber}
		}
	}
	if !total.debe.Equal(total.haber) {
		return &DescuadreError{Alcance: "corrida", TotalDebe: total.debe, TotalHaber: total.haber}
	}
	return nil
}

// ============================================================================
// Orquestación
// ============================================================================

// Estados de periodos_planilla desde los que se puede contabilizar el
// devengo. ASUNCIÓN a confirmar con Contabilidad: aprobada o pagada.
const (
	estadoPeriodoAprobada = "aprobada"
	estadoPeriodoPagada   = "pagada"
)

type PeriodoContable struct {
	ID       int64
	FechaFin time.Time
	Estado   string
}

// Repositorio abre una transacción y entrega el acceso transaccional.
type Repositorio interface {
	EjecutarEnTx(ctx context.Context, fn func(tx RepositorioTx) error) error
}

// RepositorioTx: todo lo que ocurre dentro de la transacción de contabilización.
type RepositorioTx interface {
	// FijarUsuarioAuditoria fija @auditoria_usuario_id que leen los triggers de auditoría.
	FijarUsuarioAuditoria(ctx context.Context, usuarioID int64) error
	// ObtenerPeriodoParaContabilizar lee el período bloqueándolo (FOR UPDATE).
	ObtenerPeriodoParaContabilizar(ctx context.Context, periodoID int64) (PeriodoContable, error)
	ContarAsientosDelPeriodo(ctx context.Context, periodoID int64) (int64, error)
	ListarMapeosVigentesEnFecha(ctx context.Context, fecha time.Time) ([]MapeoConceptoCuenta, error)
	// GuardarAsiento inserta cabecera y líneas; devuelve el id del asiento.
	GuardarAsiento(ctx context.Context, a AsientoContable) (int64, error)
	// MarcarPeriodoContabilizado pasa el período de estadoActual a 'contabilizada'.
	MarcarPeriodoContabilizado(ctx context.Context, periodoID int64, estadoActual string) error
}

type Service struct {
	repo  Repositorio
	cfg   ConfigGeneracion
	ahora func() time.Time
}

func NewService(repo Repositorio, cfg ConfigGeneracion) (*Service, error) {
	if strings.TrimSpace(cfg.ConceptoAnclaNeto) == "" || strings.TrimSpace(cfg.ConceptoCuotaPatronalCCSS) == "" {
		return nil, fmt.Errorf("%w: ConceptoAnclaNeto y ConceptoCuotaPatronalCCSS son obligatorios", ErrConfigInvalida)
	}
	return &Service{repo: repo, cfg: cfg, ahora: time.Now}, nil
}

// ContabilizarPeriodo genera y persiste, en UNA transacción, los asientos del
// período y lo marca 'contabilizada'. Si algo falla (mapeo faltante,
// descuadre, período en mal estado, asientos ya existentes) se hace rollback
// y no queda nada insertado.
//
// detalles debe ser el resultado en memoria de la corrida (ver DetallePlanilla).
// Los mapeos se resuelven a periodos_planilla.fecha_fin.
func (s *Service) ContabilizarPeriodo(ctx context.Context, periodoID, usuarioID int64, detalles []DetallePlanilla) ([]AsientoContable, error) {
	var generados []AsientoContable

	err := s.repo.EjecutarEnTx(ctx, func(tx RepositorioTx) error {
		if err := tx.FijarUsuarioAuditoria(ctx, usuarioID); err != nil {
			return fmt.Errorf("contabilidad: fijando usuario de auditoría: %w", err)
		}
		periodo, err := tx.ObtenerPeriodoParaContabilizar(ctx, periodoID)
		if err != nil {
			return fmt.Errorf("contabilidad: leyendo periodo_id=%d: %w", periodoID, err)
		}
		if periodo.Estado != estadoPeriodoAprobada && periodo.Estado != estadoPeriodoPagada {
			return fmt.Errorf("%w: periodo_id=%d está %q (se requiere %q o %q)", ErrPeriodoEstadoInvalido, periodoID, periodo.Estado, estadoPeriodoAprobada, estadoPeriodoPagada)
		}
		n, err := tx.ContarAsientosDelPeriodo(ctx, periodoID)
		if err != nil {
			return fmt.Errorf("contabilidad: verificando asientos existentes: %w", err)
		}
		if n > 0 {
			return fmt.Errorf("%w: periodo_id=%d tiene %d asiento(s)", ErrAsientosYaGenerados, periodoID, n)
		}
		mapeos, err := tx.ListarMapeosVigentesEnFecha(ctx, periodo.FechaFin)
		if err != nil {
			return fmt.Errorf("contabilidad: leyendo mapeo_concepto_cuenta vigente al %s: %w", periodo.FechaFin.Format("2006-01-02"), err)
		}

		asientos, err := GenerarAsientos(periodoID, detalles, mapeos, s.cfg)
		if err != nil {
			return err
		}

		ahora := s.ahora()
		for i := range asientos {
			a := &asientos[i]
			a.NumeroAsiento = numeroAsiento(periodoID, a.TipoAsientoID)
			a.FechaGeneracion = ahora
			id, err := tx.GuardarAsiento(ctx, *a)
			if err != nil {
				return fmt.Errorf("contabilidad: guardando asiento tipo_asiento_id=%d: %w", a.TipoAsientoID, err)
			}
			a.ID = id
			for j := range a.Lineas {
				a.Lineas[j].AsientoContableID = id
			}
		}
		if err := tx.MarcarPeriodoContabilizado(ctx, periodoID, periodo.Estado); err != nil {
			return fmt.Errorf("contabilidad: marcando periodo_id=%d como contabilizada: %w", periodoID, err)
		}
		generados = asientos
		return nil
	})
	if err != nil {
		return nil, err
	}
	return generados, nil
}

// numeroAsiento: folio determinístico por (período, tipo). ASUNCIÓN: el
// correlativo contable oficial no está definido en el esquema; ajustar aquí
// si Contabilidad exige otro formato.
func numeroAsiento(periodoID, tipoAsientoID int64) string {
	return fmt.Sprintf("PLA-%06d-%02d", periodoID, tipoAsientoID)
}