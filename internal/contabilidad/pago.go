package contabilidad

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
)

// NetoPrograma: Σ neto pagado del período en un programa presupuestario.
type NetoPrograma struct {
	ProgramaPresupuestarioID int64
	Neto                     decimal.Decimal
}

// GenerarAsientoPago arma (puro) el asiento de pago. ok=false si el neto total
// es cero (no hay nada que registrar).
func GenerarAsientoPago(periodoID int64, netos []NetoPrograma, mapeos []MapeoConceptoCuenta, cfg ConfigGeneracion) (AsientoContable, bool, error) {
	if cfg.ConceptoAnclaNeto == "" || cfg.TipoAsientoPago == "" {
		return AsientoContable{}, false, fmt.Errorf("%w: ConceptoAnclaNeto y TipoAsientoPago son obligatorios", ErrConfigInvalida)
	}
	pasivo := map[int64][]MapeoConceptoCuenta{}
	banco := map[int64][]MapeoConceptoCuenta{}
	var tipoPagoID int64
	for _, m := range mapeos {
		if m.ConceptoCodigo != cfg.ConceptoAnclaNeto || m.TipoMovimiento != MovimientoHaber {
			continue
		}
		if m.TipoAsientoCodigo == cfg.TipoAsientoPago {
			banco[m.ProgramaPresupuestarioID] = append(banco[m.ProgramaPresupuestarioID], m)
			tipoPagoID = m.TipoAsientoID
		} else {
			pasivo[m.ProgramaPresupuestarioID] = append(pasivo[m.ProgramaPresupuestarioID], m)
		}
	}
	sort.Slice(netos, func(i, j int) bool { return netos[i].ProgramaPresupuestarioID < netos[j].ProgramaPresupuestarioID })

	a := AsientoContable{PeriodoPlanillaID: periodoID, TipoAsientoCodigo: cfg.TipoAsientoPago, Estado: EstadoAsientoGenerado}
	for _, n := range netos {
		if n.Neto.IsNegative() {
			return AsientoContable{}, false, fmt.Errorf("%w: neto negativo en programa_presupuestario_id=%d", ErrMontoNegativoOInvalido, n.ProgramaPresupuestarioID)
		}
		monto := n.Neto.Round(dosDecimales)
		if monto.IsZero() {
			continue
		}
		p := n.ProgramaPresupuestarioID
		if len(pasivo[p]) != 1 {
			return AsientoContable{}, false, fmt.Errorf("%w: se esperaba 1 mapeo 'haber' del neto (%q) fuera del tipo %q para el programa %d y hay %d",
				ErrMapeoFaltante, cfg.ConceptoAnclaNeto, cfg.TipoAsientoPago, p, len(pasivo[p]))
		}
		if len(banco[p]) != 1 {
			return AsientoContable{}, false, fmt.Errorf("%w: se esperaba 1 mapeo 'haber' de bancos (%q, tipo %q) para el programa %d y hay %d",
				ErrMapeoFaltante, cfg.ConceptoAnclaNeto, cfg.TipoAsientoPago, p, len(banco[p]))
		}
		pas, ban := pasivo[p][0], banco[p][0]
		cp, cb := pas.ConceptoSalarialID, ban.ConceptoSalarialID
		a.Lineas = append(a.Lineas,
			AsientoContableDetalle{CuentaContableID: pas.CuentaContableID, ProgramaPresupuestarioID: p, Proyecto: pas.Proyecto, ConceptoSalarialID: &cp, MontoDebe: monto, MontoHaber: decimal.Zero},
			AsientoContableDetalle{CuentaContableID: ban.CuentaContableID, ProgramaPresupuestarioID: p, Proyecto: ban.Proyecto, ConceptoSalarialID: &cb, MontoDebe: decimal.Zero, MontoHaber: monto},
		)
	}
	if len(a.Lineas) == 0 {
		return AsientoContable{}, false, nil
	}
	a.TipoAsientoID = tipoPagoID
	if err := ValidarPartidaDoble([]AsientoContable{a}); err != nil {
		return AsientoContable{}, false, err
	}
	return a, true, nil
}

// ganchoPago implementa planilla.GanchoAlPagar.
type ganchoPago struct {
	cfg   ConfigGeneracion
	ahora func() time.Time
}

// NewGanchoPago: gancho AlPagar que persiste el asiento de pago en la misma
// transacción del pago.
func NewGanchoPago(cfg ConfigGeneracion) (planilla.GanchoAlPagar, error) {
	if cfg.ConceptoAnclaNeto == "" || cfg.TipoAsientoPago == "" {
		return nil, fmt.Errorf("%w: ConceptoAnclaNeto y TipoAsientoPago son obligatorios", ErrConfigInvalida)
	}
	return &ganchoPago{cfg: cfg, ahora: time.Now}, nil
}

func (g *ganchoPago) AlPagar(ctx context.Context, q *dto.Queries, periodo dto.PeriodoCicloRow) error {
	cq := q.Contabilidad()
	filas, err := cq.SumarNetoPorPrograma(ctx, periodo.ID)
	if err != nil {
		return fmt.Errorf("contabilidad: neto por programa del período %d: %w", periodo.ID, err)
	}
	netos := make([]NetoPrograma, 0, len(filas))
	for _, f := range filas {
		netos = append(netos, NetoPrograma(f))
	}
	ms, err := (&repoTx{q: cq}).ListarMapeosVigentesEnFecha(ctx, periodo.FechaFin)
	if err != nil {
		return fmt.Errorf("contabilidad: mapeos vigentes al %s: %w", periodo.FechaFin.Format("2006-01-02"), err)
	}
	a, ok, err := GenerarAsientoPago(periodo.ID, netos, ms, g.cfg)
	if err != nil || !ok {
		return err
	}
	a.NumeroAsiento = fmt.Sprintf("PAG-%06d", periodo.ID)
	a.FechaGeneracion = g.ahora()
	if _, err := (&repoTx{q: cq}).GuardarAsiento(ctx, a); err != nil {
		return fmt.Errorf("contabilidad: guardando asiento de pago del período %d: %w", periodo.ID, err)
	}
	return nil
}
