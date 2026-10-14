package prestamos

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

// Errores de dominio. Los handlers los traducen a códigos HTTP.
var (
	// Validación de entrada.
	ErrMontoInvalido      = errors.New("el monto debe ser mayor a cero")
	ErrMontoPrecision     = errors.New("los montos admiten máximo 2 decimales y 12 dígitos enteros")
	ErrCuotaExcedeMonto   = errors.New("la cuota periódica no puede ser mayor al monto total")
	ErrCuotasInvalidas    = errors.New("el número de cuotas debe ser mayor a cero")
	ErrCuotasIncoherentes = errors.New("el número de cuotas no es coherente con monto_total / cuota_periodica")
	ErrFechasInvalidas    = errors.New("la fecha de fin no puede ser anterior a la fecha de inicio")

	ErrEmbargoSinBase       = errors.New("un embargo debe definir monto_fijo o porcentaje, no ambos vacíos")
	ErrEmbargoBaseDuplicada = errors.New("un embargo no puede definir monto_fijo y porcentaje simultáneamente")
	ErrPorcentajeInvalido   = errors.New("el porcentaje debe estar en el rango (0, 100] con máximo 2 decimales")
	ErrFechaFinRequerida    = errors.New("fecha_fin_vigencia es obligatoria al finalizar un embargo")

	// Referencias inexistentes.
	ErrEmpleadoNoEncontrado    = errors.New("el empleado no existe")
	ErrTerceroNoEncontrado     = errors.New("el tercero no existe")
	ErrTipoEmbargoNoEncontrado = errors.New("el tipo de embargo no existe")
	ErrPrestamoNoEncontrado    = errors.New("el préstamo no existe")
	ErrEmbargoNoEncontrado     = errors.New("el embargo no existe")

	// Conflictos de estado.
	ErrEmpleadoInactivo           = errors.New("el empleado no está activo")
	ErrEmpleadoSinContratoVigente = errors.New("el empleado no tiene un contrato vigente")
	ErrTipoEmbargoNoVigente       = errors.New("el tipo de embargo ya no está vigente")
	ErrPrestamoDuplicado          = errors.New("ya existe un préstamo idéntico para este empleado")
	ErrPrestamoNoActivo           = errors.New("el préstamo no está activo")
	ErrPrestamoYaCancelado        = errors.New("el préstamo ya está cancelado")
	ErrAbonoExcedeSaldo           = errors.New("el abono excede el saldo pendiente del préstamo")
	ErrTransicionInvalida         = errors.New("transición de estado de embargo no permitida")
)

// toleranciaCuotas es la diferencia máxima (en cuotas) aceptada entre el
// número de cuotas declarado y ceil(monto_total / cuota_periodica). Es una
// holgura OPERATIVA para absorber una última cuota parcial o redondeos; NO
// es un valor legal ni proviene de normativa. Ajustar según la práctica de
// Recursos Humanos.
const toleranciaCuotas int64 = 1

// limiteMonto es 10^12: DECIMAL(14,2) admite hasta 12 dígitos enteros.
var limiteMonto = decimal.New(1, 12)

// Service agrupa la lógica de negocio de Préstamos, Embargos y Terceros.
// Toda escritura pasa por db.WithAuditContext (ADR-000), y TODA lectura que
// precede a una escritura se hace dentro de esa misma transacción con
// SELECT ... FOR UPDATE, para que dos operaciones concurrentes sobre la
// misma fila se serialicen en lugar de pisarse.
type Service struct {
	dbConn *sql.DB
	q      Querier // lecturas de listado, fuera de transacción
}

func NewService(dbConn *sql.DB, q *dto.Queries) *Service {
	return &Service{dbConn: dbConn, q: q}
}

// validarMonto exige un monto > 0 que quepa en DECIMAL(14,2).
func validarMonto(d decimal.Decimal) error {
	if !d.IsPositive() {
		return ErrMontoInvalido
	}
	if !d.Equal(d.Round(2)) || d.GreaterThanOrEqual(limiteMonto) {
		return ErrMontoPrecision
	}
	return nil
}

// =====================================================================
// TERCEROS
// =====================================================================

type CrearTerceroInput struct {
	Nombre string
	Tipo   string // 'juzgado' | 'entidad_financiera' | 'otro'
}

func (s *Service) CrearTercero(ctx context.Context, userID int64, in CrearTerceroInput) (int64, error) {
	var id int64
	err := db.WithAuditContext(ctx, s.dbConn, userID, func(ctx context.Context, qtx *dto.Queries) error {
		var err error
		id, err = qtx.CreateTercero(ctx, dto.CreateTerceroParams{Nombre: in.Nombre, Tipo: in.Tipo})
		return err
	})
	return id, err
}

func (s *Service) ListarTerceros(ctx context.Context, tipo, busqueda string, limit, offset int32) ([]dto.Tercero, int64, error) {
	items, err := s.q.ListTerceros(ctx, dto.ListTercerosParams{Tipo: tipo, Nombre: busqueda, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountTerceros(ctx, dto.CountTercerosParams{Tipo: tipo, Nombre: busqueda})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// =====================================================================
// PRESTAMOS
// =====================================================================

type CrearPrestamoInput struct {
	EmpleadoID       int64
	TerceroID        int64 // prestamos.tercero_id es NOT NULL en el esquema
	MontoTotal       decimal.Decimal
	CuotaPeriodica   decimal.Decimal
	CuotasTotales    *int32 // opcional: solo se contrasta contra monto/cuota, NO se persiste (el esquema no tiene la columna)
	FechaInicio      time.Time
	FechaFinEstimada *time.Time
}

// CrearPrestamo valida coherencia de montos/cuotas fuera de la transacción
// (son puras) y, dentro de ella, bloquea al empleado (FOR UPDATE) para
// verificar empleado activo, contrato vigente, tercero existente y ausencia
// de un préstamo idéntico sin que otra alta concurrente se cuele entre la
// verificación y el INSERT. El préstamo nace 'activo' con saldo = monto_total.
func (s *Service) CrearPrestamo(ctx context.Context, userID int64, in CrearPrestamoInput) (int64, error) {
	if err := validarMonto(in.MontoTotal); err != nil {
		return 0, err
	}
	if err := validarMonto(in.CuotaPeriodica); err != nil {
		return 0, err
	}
	if in.CuotaPeriodica.GreaterThan(in.MontoTotal) {
		return 0, ErrCuotaExcedeMonto
	}
	if in.CuotasTotales != nil {
		if *in.CuotasTotales <= 0 {
			return 0, ErrCuotasInvalidas
		}
		esperadas := in.MontoTotal.Div(in.CuotaPeriodica).Ceil().IntPart()
		diff := int64(*in.CuotasTotales) - esperadas
		if diff < 0 {
			diff = -diff
		}
		if diff > toleranciaCuotas {
			return 0, ErrCuotasIncoherentes
		}
	}
	if in.FechaFinEstimada != nil && in.FechaFinEstimada.Before(in.FechaInicio) {
		return 0, ErrFechasInvalidas // espejo de chk_pr_fechas
	}

	var id int64
	err := db.WithAuditContext(ctx, s.dbConn, userID, func(ctx context.Context, qtx *dto.Queries) error {
		// El lock del empleado va primero: serializa altas concurrentes del
		// mismo empleado, de modo que las lecturas siguientes ven el estado
		// ya confirmado por la alta anterior.
		emp, err := qtx.GetEmpleadoParaActualizar(ctx, in.EmpleadoID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEmpleadoNoEncontrado
		}
		if err != nil {
			return err
		}
		if !emp.Activo {
			return ErrEmpleadoInactivo
		}

		conContrato, err := qtx.ExisteContratoVigente(ctx, in.EmpleadoID)
		if err != nil {
			return err
		}
		if !conContrato {
			return ErrEmpleadoSinContratoVigente
		}

		if _, err := qtx.GetTercero(ctx, in.TerceroID); errors.Is(err, sql.ErrNoRows) {
			return ErrTerceroNoEncontrado
		} else if err != nil {
			return err
		}

		dup, err := qtx.ExistePrestamoDuplicado(ctx, dto.ExistePrestamoDuplicadoParams{
			EmpleadoID:     in.EmpleadoID,
			TerceroID:      in.TerceroID,
			MontoTotal:     in.MontoTotal,
			CuotaPeriodica: in.CuotaPeriodica,
			FechaInicio:    in.FechaInicio,
		})
		if err != nil {
			return err
		}
		if dup {
			return ErrPrestamoDuplicado
		}

		id, err = qtx.CreatePrestamo(ctx, dto.CreatePrestamoParams{
			EmpleadoID:             in.EmpleadoID,
			TerceroID:              in.TerceroID,
			MontoTotal:             in.MontoTotal,
			CuotaPeriodica:         in.CuotaPeriodica,
			SaldoPendiente:         in.MontoTotal, // saldo inicial = monto total
			FechaInicio:            in.FechaInicio,
			FechaFinEstimada:       toNullTime(in.FechaFinEstimada),
			RegistradoPorUsuarioID: sql.NullInt64{Int64: userID, Valid: true},
		})
		return err
	})
	return id, err
}

func (s *Service) ListarPrestamosPorEmpleado(ctx context.Context, empleadoID int64, estado string, limit, offset int32) ([]dto.Prestamo, int64, error) {
	items, err := s.q.ListPrestamosPorEmpleado(ctx, dto.ListPrestamosPorEmpleadoParams{
		EmpleadoID: empleadoID, Estado: estado, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountPrestamosPorEmpleado(ctx, dto.CountPrestamosPorEmpleadoParams{EmpleadoID: empleadoID, Estado: estado})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// AplicarAbono descuenta un monto del saldo pendiente de un préstamo y, si el
// saldo llega a cero, lo cierra. Lee el préstamo con FOR UPDATE dentro de la
// transacción: dos abonos simultáneos se serializan y el segundo ve el saldo
// ya descontado por el primero. Rechaza (no recorta) un abono mayor al saldo;
// quien calcula la cuota (motor de planilla, F23) debe enviar min(cuota, saldo).
func (s *Service) AplicarAbono(ctx context.Context, userID, prestamoID int64, montoAbono decimal.Decimal) error {
	if err := validarMonto(montoAbono); err != nil {
		return err
	}
	return db.WithAuditContext(ctx, s.dbConn, userID, func(ctx context.Context, qtx *dto.Queries) error {
		p, err := qtx.GetPrestamoParaActualizar(ctx, prestamoID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPrestamoNoEncontrado
		}
		if err != nil {
			return err
		}
		if p.Estado != EstadoPrestamoActivo {
			return ErrPrestamoNoActivo
		}
		if montoAbono.GreaterThan(p.SaldoPendiente) {
			return ErrAbonoExcedeSaldo // evita violar chk_pr_saldo (saldo >= 0)
		}
		if err := qtx.AplicarAbonoPrestamo(ctx, dto.AplicarAbonoPrestamoParams{
			SaldoPendiente: montoAbono,
			ID:             prestamoID,
		}); err != nil {
			return err
		}
		// Idempotente: solo cierra si el saldo efectivamente llegó a 0.
		return qtx.CerrarPrestamoSiSaldoEnCero(ctx, prestamoID)
	})
}

// CancelarPrestamo cancela manualmente un préstamo activo o suspendido. La
// lectura del estado es bajo FOR UPDATE para no pisar un cierre concurrente.
func (s *Service) CancelarPrestamo(ctx context.Context, userID, prestamoID int64) error {
	return db.WithAuditContext(ctx, s.dbConn, userID, func(ctx context.Context, qtx *dto.Queries) error {
		p, err := qtx.GetPrestamoParaActualizar(ctx, prestamoID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPrestamoNoEncontrado
		}
		if err != nil {
			return err
		}
		if p.Estado == EstadoPrestamoCancelado {
			return ErrPrestamoYaCancelado
		}
		return qtx.CancelarPrestamoManual(ctx, prestamoID)
	})
}

// =====================================================================
// EMBARGOS
// =====================================================================

// Transiciones de estado permitidas para `embargos` (ENUM del esquema:
// activo | suspendido | finalizado). Recordatorio ADR-000: esta tabla NO usa
// el patrón de vigencia de 2 pasos; el ciclo de vida es puramente por
// `estado`.
var transicionesEmbargoPermitidas = map[string]map[string]bool{
	EstadoEmbargoActivo:     {EstadoEmbargoSuspendido: true, EstadoEmbargoFinalizado: true},
	EstadoEmbargoSuspendido: {EstadoEmbargoActivo: true, EstadoEmbargoFinalizado: true},
	EstadoEmbargoFinalizado: {}, // estado terminal
}

type CrearEmbargoInput struct {
	EmpleadoID       int64
	TipoEmbargoID    int64
	TerceroID        int64
	NumeroExpediente *string
	Porcentaje       *decimal.Decimal
	MontoFijo        *decimal.Decimal
	FechaInicio      time.Time
	FechaFinVigencia *time.Time
}

// CrearEmbargo valida que se defina exactamente una de las dos bases de
// cálculo (monto_fijo XOR porcentaje; "definida" = campo presente, y luego se
// valida su valor), resuelve prioridad_aplicacion desde el tipo de embargo y
// asigna el estado inicial 'activo'.
func (s *Service) CrearEmbargo(ctx context.Context, userID int64, in CrearEmbargoInput) (int64, error) {
	if in.MontoFijo == nil && in.Porcentaje == nil {
		return 0, ErrEmbargoSinBase
	}
	if in.MontoFijo != nil && in.Porcentaje != nil {
		return 0, ErrEmbargoBaseDuplicada
	}
	if in.MontoFijo != nil {
		if err := validarMonto(*in.MontoFijo); err != nil {
			return 0, err
		}
	}
	if in.Porcentaje != nil {
		p := *in.Porcentaje // espejo de chk_emb_porcentaje_rango y DECIMAL(5,2)
		if !p.IsPositive() || p.GreaterThan(decimal.NewFromInt(100)) || !p.Equal(p.Round(2)) {
			return 0, ErrPorcentajeInvalido
		}
	}
	if in.FechaFinVigencia != nil && in.FechaFinVigencia.Before(in.FechaInicio) {
		return 0, ErrFechasInvalidas // espejo de chk_emb_vigencia
	}

	var id int64
	err := db.WithAuditContext(ctx, s.dbConn, userID, func(ctx context.Context, qtx *dto.Queries) error {
		if _, err := qtx.GetEmpleadoParaActualizar(ctx, in.EmpleadoID); errors.Is(err, sql.ErrNoRows) {
			return ErrEmpleadoNoEncontrado
		} else if err != nil {
			return err
		}
		if _, err := qtx.GetTercero(ctx, in.TerceroID); errors.Is(err, sql.ErrNoRows) {
			return ErrTerceroNoEncontrado
		} else if err != nil {
			return err
		}
		tipo, err := qtx.GetTipoEmbargo(ctx, in.TipoEmbargoID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTipoEmbargoNoEncontrado
		}
		if err != nil {
			return err
		}
		if tipo.FechaFinVigencia.Valid {
			return ErrTipoEmbargoNoVigente
		}

		id, err = qtx.CreateEmbargo(ctx, dto.CreateEmbargoParams{
			EmpleadoID:          in.EmpleadoID,
			TerceroID:           in.TerceroID,
			TipoEmbargoID:       in.TipoEmbargoID,
			NumeroExpediente:    toNullString(in.NumeroExpediente),
			Porcentaje:          toNullDecimal(in.Porcentaje),
			MontoFijo:           toNullDecimal(in.MontoFijo),
			PrioridadAplicacion: tipo.PrioridadLegal,
			FechaInicioVigencia: in.FechaInicio,
			FechaFinVigencia:    toNullTime(in.FechaFinVigencia),
		})
		return err
	})
	return id, err
}

func (s *Service) ListarEmbargosPorEmpleado(ctx context.Context, empleadoID int64, estado string, limit, offset int32) ([]dto.Embargo, int64, error) {
	items, err := s.q.ListEmbargosPorEmpleado(ctx, dto.ListEmbargosPorEmpleadoParams{
		EmpleadoID: empleadoID, Estado: estado, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountEmbargosPorEmpleado(ctx, dto.CountEmbargosPorEmpleadoParams{EmpleadoID: empleadoID, Estado: estado})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// EmbargosActivosOrdenados es la consulta que usará el motor de planilla:
// embargos activos y vigentes en fechaCorte, en orden de prioridad ascendente
// (1 = pensión alimentaria primero, según el seed de parametros_tipos_embargo).
func (s *Service) EmbargosActivosOrdenados(ctx context.Context, empleadoID int64, fechaCorte time.Time) ([]dto.ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadRow, error) {
	return s.q.ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridad(ctx, dto.ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadParams{
		EmpleadoID: empleadoID,
		FechaCorte: fechaCorte,
	})
}

// CambiarEstadoEmbargo lee el embargo con FOR UPDATE y valida la transición
// contra la máquina de estados dentro de la misma transacción que escribe.
// fechaFin solo aplica (y es obligatoria) al transicionar a 'finalizado'; en
// las demás transiciones se ignora y se conserva la fecha_fin_vigencia
// existente.
func (s *Service) CambiarEstadoEmbargo(ctx context.Context, userID, embargoID int64, nuevoEstado string, fechaFin *time.Time) error {
	return db.WithAuditContext(ctx, s.dbConn, userID, func(ctx context.Context, qtx *dto.Queries) error {
		actual, err := qtx.GetEmbargoParaActualizar(ctx, embargoID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEmbargoNoEncontrado
		}
		if err != nil {
			return err
		}
		permitidas, ok := transicionesEmbargoPermitidas[actual.Estado]
		if !ok || !permitidas[nuevoEstado] {
			return ErrTransicionInvalida
		}

		if nuevoEstado == EstadoEmbargoFinalizado {
			if fechaFin == nil {
				return ErrFechaFinRequerida
			}
			if fechaFin.Before(actual.FechaInicioVigencia) {
				return ErrFechasInvalidas // espejo de chk_emb_vigencia
			}
		} else {
			fechaFin = nil
		}

		return qtx.ActualizarEstadoEmbargo(ctx, dto.ActualizarEstadoEmbargoParams{
			Estado:           nuevoEstado,
			FechaFinVigencia: toNullTime(fechaFin),
			ID:               embargoID,
		})
	})
}

// =====================================================================
// Helpers de conversión a tipos nullable de database/sql y shopspring/decimal
// =====================================================================

func toNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func toNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func toNullDecimal(d *decimal.Decimal) decimal.NullDecimal {
	if d == nil {
		return decimal.NullDecimal{}
	}
	return decimal.NullDecimal{Decimal: *d, Valid: true}
}