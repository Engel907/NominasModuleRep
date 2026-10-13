package prestamos

import (
	"context"

	"planillas-ccdr/internal/db/dto"
)

// Querier es el conjunto de consultas de dto que usa este módulo. La
// aserción de abajo hace que el compilador verifique, método por método
// (nombres, argumentos, tipos y retornos), que *dto.Queries la satisface:
// cualquier desajuste futuro rompe la compilación en vez de fallar en runtime.
type Querier interface {
	// Terceros
	CreateTercero(ctx context.Context, arg dto.CreateTerceroParams) (int64, error)
	GetTercero(ctx context.Context, id int64) (dto.Tercero, error)
	ListTerceros(ctx context.Context, arg dto.ListTercerosParams) ([]dto.Tercero, error)
	CountTerceros(ctx context.Context, arg dto.CountTercerosParams) (int64, error)

	// Empleados / contratos (solo lectura)
	GetEmpleadoParaActualizar(ctx context.Context, id int64) (dto.EmpleadoEstado, error)
	ExisteContratoVigente(ctx context.Context, empleadoID int64) (bool, error)

	// Préstamos
	CreatePrestamo(ctx context.Context, arg dto.CreatePrestamoParams) (int64, error)
	GetPrestamoParaActualizar(ctx context.Context, id int64) (dto.Prestamo, error)
	ExistePrestamoDuplicado(ctx context.Context, arg dto.ExistePrestamoDuplicadoParams) (bool, error)
	ListPrestamosPorEmpleado(ctx context.Context, arg dto.ListPrestamosPorEmpleadoParams) ([]dto.Prestamo, error)
	CountPrestamosPorEmpleado(ctx context.Context, arg dto.CountPrestamosPorEmpleadoParams) (int64, error)
	AplicarAbonoPrestamo(ctx context.Context, arg dto.AplicarAbonoPrestamoParams) error
	CerrarPrestamoSiSaldoEnCero(ctx context.Context, id int64) error
	CancelarPrestamoManual(ctx context.Context, id int64) error

	// Embargos
	GetTipoEmbargo(ctx context.Context, id int64) (dto.TipoEmbargo, error)
	CreateEmbargo(ctx context.Context, arg dto.CreateEmbargoParams) (int64, error)
	GetEmbargoParaActualizar(ctx context.Context, id int64) (dto.Embargo, error)
	ListEmbargosPorEmpleado(ctx context.Context, arg dto.ListEmbargosPorEmpleadoParams) ([]dto.Embargo, error)
	CountEmbargosPorEmpleado(ctx context.Context, arg dto.CountEmbargosPorEmpleadoParams) (int64, error)
	ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridad(ctx context.Context, arg dto.ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadParams) ([]dto.ListEmbargosActivosPorEmpleadoOrdenadosPorPrioridadRow, error)
	ActualizarEstadoEmbargo(ctx context.Context, arg dto.ActualizarEstadoEmbargoParams) error
}

var _ Querier = (*dto.Queries)(nil)
