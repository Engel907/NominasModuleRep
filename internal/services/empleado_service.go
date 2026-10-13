// services/empleado_service.go
//
// Fase 3 — Dominio Empleados.
//
// F03 (esta revisión):
//   - Firma real de db.WithAuditContext: callback func(ctx, q) con usuarioID
//     int64. IDs int64 en toda la capa.
//   - Paginación unificada en internal/pagination (20/100).
//   - Toda escritura versionada (cerrar + abrir) toma primero un lock de fila
//     sobre `empleados` (SELECT ... FOR UPDATE) dentro de la misma Tx, de modo
//     que dos escrituras concurrentes sobre el mismo empleado se serialicen y
//     el "leer vigente → cerrar → insertar" no tenga condición de carrera.
//   - Validaciones: fecha efectiva posterior al inicio de la vigencia que se
//     cierra (sin traslapes), salario > 0 con escala DECIMAL(14,2), campos
//     requeridos tras TrimSpace, identificación / código interno duplicados
//     (UNIQUE de la BD traducido a error tipado).
//
// Ver NOTA DE MIGRACIÓN en internal/db/dto/empleados.go para el porqué de la
// forma de los DTOs (campos que viven en `empleados` vs. en los infotypes).
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/pagination"

	"github.com/shopspring/decimal"
)

// ErrEmpleadoNoEncontrado se devuelve cuando el empleado_id no existe.
var ErrEmpleadoNoEncontrado = errors.New("empleado no encontrado")

// -----------------------------------------------------------------------------
// DTOs de entrada/salida
// -----------------------------------------------------------------------------

// --- Empleado (creación de expediente completo) ---

type CrearEmpleadoRequest struct {
	CodigoInterno        string `json:"codigo_interno" binding:"required,max=10"`
	TipoIdentificacionID int64  `json:"tipo_identificacion_id" binding:"required,gt=0"`
	NumeroIdentificacion string `json:"numero_identificacion" binding:"required,max=20"`
	NombreCompleto       string `json:"nombre_completo" binding:"required,max=200"`
	FechaIngreso         string `json:"fecha_ingreso" binding:"required"` // YYYY-MM-DD

	// Los structs anidados se validan siempre (gin/validator desciende por
	// ellos); `required` sobre un struct no hace nada, por eso no se declara.
	DatosPersonales DatosPersonalesInput `json:"datos_personales"`
	DatosLaborales  DatosLaboralesInput  `json:"datos_laborales"`
	DatosBancarios  *DatosBancariosInput `json:"datos_bancarios"` // opcional al crear
}

// DatosPersonalesInput — solo los campos que empleado_datos_personales
// realmente tiene (direccion, telefono, correo_electronico).
type DatosPersonalesInput struct {
	Direccion     string `json:"direccion" binding:"max=255"`
	Telefono      string `json:"telefono" binding:"max=30"`
	Email         string `json:"email" binding:"omitempty,email,max=150"`
	FechaEfectiva string `json:"fecha_efectiva" binding:"required"` // fecha_inicio_vigencia
}

// DatosLaboralesInput — TipoContratoID/ProgramaPresupuestarioID/SindicatoID.
type DatosLaboralesInput struct {
	PuestoID int64 `json:"puesto_id" binding:"required,gt=0"`
	// TODO F22: salario_base_nominal es la fuente de salario que F22 (D5)
	// unifica con contratos.salario_pactado. No tocar la lógica aquí. Se
	// valida en el servicio (> 0, escala 2) porque `required` no detecta el
	// cero de un decimal.Decimal.
	SalarioBaseNominal       decimal.Decimal `json:"salario_base_nominal"`
	TipoContratoID           int64           `json:"tipo_contrato_id" binding:"required,gt=0"`
	ProgramaPresupuestarioID int64           `json:"programa_presupuestario_id" binding:"required,gt=0"`
	SindicatoID              *int64          `json:"sindicato_id" binding:"omitempty,gt=0"` // opcional (afiliación sindical)
	FechaEfectiva            string          `json:"fecha_efectiva" binding:"required"`
}

// DatosBancariosInput — BancoID (FK a `bancos`); tipo_cuenta es ENUM.
type DatosBancariosInput struct {
	BancoID       int64  `json:"banco_id" binding:"required,gt=0"`
	TipoCuenta    string `json:"tipo_cuenta" binding:"required,oneof=corriente ahorros"`
	NumeroCuenta  string `json:"numero_cuenta" binding:"required,max=40"`
	FechaEfectiva string `json:"fecha_efectiva" binding:"required"`
}

// DatosFamiliarInput — declaración agregada (no una persona con nombre
// propio): así es empleado_datos_familiares en el schema real. El conteo no
// es un ID, conserva uint32 (igual que dto.EmpleadoDatosFamiliare).
type DatosFamiliarInput struct {
	AplicaCreditoConyuge    bool   `json:"aplica_credito_conyuge"`
	NumeroHijosDependientes uint32 `json:"numero_hijos_dependientes"`
	FechaEfectiva           string `json:"fecha_efectiva" binding:"required"`
}

type EmpleadoResponse struct {
	ID            int64  `json:"id"`
	CodigoInterno string `json:"codigo_interno"`
	Activo        bool   `json:"activo"`
}

type ListarEmpleadosRequest struct {
	// Paginacion NO se bindea del query con gin: el handler la llena con
	// pagination.Desde(c).
	Paginacion    pagination.Parametros `form:"-"`
	Activo        *bool                 `form:"activo"`
	CodigoInterno *string               `form:"codigo_interno"`
}

type ListarEmpleadosResponse struct {
	Empleados  []EmpleadoResponse `json:"empleados"`
	Paginacion pagination.Meta    `json:"paginacion"`
}

// -----------------------------------------------------------------------------
// Interfaz de servicio
// -----------------------------------------------------------------------------

type EmpleadoService interface {
	CrearExpediente(ctx context.Context, usuarioID int64, req CrearEmpleadoRequest) (*EmpleadoResponse, error)
	ListarEmpleados(ctx context.Context, req ListarEmpleadosRequest) (*ListarEmpleadosResponse, error)
	GetEmpleado(ctx context.Context, empleadoID int64) (*EmpleadoResponse, error)

	ActualizarDatosPersonales(ctx context.Context, usuarioID, empleadoID int64, in DatosPersonalesInput) error
	ActualizarDatosLaborales(ctx context.Context, usuarioID, empleadoID int64, in DatosLaboralesInput) error
	ActualizarDatosBancarios(ctx context.Context, usuarioID, empleadoID int64, in DatosBancariosInput) error

	AgregarFamiliar(ctx context.Context, usuarioID, empleadoID int64, in DatosFamiliarInput) error
	ActualizarFamiliar(ctx context.Context, usuarioID, familiarID int64, in DatosFamiliarInput) error
	ListarFamiliaresVigentes(ctx context.Context, empleadoID int64) ([]dto.EmpleadoDatosFamiliare, error)
}

type empleadoService struct {
	sqlDB *sql.DB
}

func NewEmpleadoService(sqlDB *sql.DB) EmpleadoService {
	return &empleadoService{sqlDB: sqlDB}
}

const layoutFecha = "2006-01-02"

// parseFecha parsea YYYY-MM-DD. El error es un *ErrValidacion (400); no se
// propaga el error crudo de time.Parse.
func parseFecha(campo, valor string) (time.Time, error) {
	t, err := time.Parse(layoutFecha, valor)
	if err != nil {
		return time.Time{}, validacion(campo, "formato de fecha inválido, se espera YYYY-MM-DD")
	}
	return t, nil
}

// bloquearEmpleado toma el lock de fila del empleado dentro de la Tx actual y
// verifica que exista. Debe llamarse al inicio de toda escritura versionada.
func bloquearEmpleado(ctx context.Context, q *dto.Queries, empleadoID int64) error {
	if _, err := q.GetEmpleadoByIDForUpdate(ctx, empleadoID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEmpleadoNoEncontrado
		}
		return fmt.Errorf("bloquear empleado %d: %w", empleadoID, err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// CrearExpediente: alta de empleado + primera versión vigente de sus datos.
// No hay "cerrar" porque es la primera fila — solo INSERT dentro de la
// misma transacción auditada. Las UNIQUE de la BD (codigo_interno,
// (tipo_identificacion_id, numero_identificacion)) son el árbitro de
// duplicados; sus violaciones se traducen a errores tipados.
// -----------------------------------------------------------------------------

func (s *empleadoService) CrearExpediente(ctx context.Context, usuarioID int64, req CrearEmpleadoRequest) (*EmpleadoResponse, error) {
	codigo, err := noVacio("codigo_interno", req.CodigoInterno)
	if err != nil {
		return nil, err
	}
	numeroIdent, err := noVacio("numero_identificacion", req.NumeroIdentificacion)
	if err != nil {
		return nil, err
	}
	nombre, err := noVacio("nombre_completo", req.NombreCompleto)
	if err != nil {
		return nil, err
	}

	fechaIngreso, err := parseFecha("fecha_ingreso", req.FechaIngreso)
	if err != nil {
		return nil, err
	}
	fechaEfectivaPersonales, err := parseFecha("datos_personales.fecha_efectiva", req.DatosPersonales.FechaEfectiva)
	if err != nil {
		return nil, err
	}
	fechaEfectivaLaborales, err := parseFecha("datos_laborales.fecha_efectiva", req.DatosLaborales.FechaEfectiva)
	if err != nil {
		return nil, err
	}
	// TODO F22: salario_base_nominal — fuente única de salario (D5).
	if err := validarMontoSalario("datos_laborales.salario_base_nominal", req.DatosLaborales.SalarioBaseNominal); err != nil {
		return nil, err
	}

	var (
		fechaEfectivaBancarios time.Time
		numeroCuenta           string
	)
	if req.DatosBancarios != nil {
		fechaEfectivaBancarios, err = parseFecha("datos_bancarios.fecha_efectiva", req.DatosBancarios.FechaEfectiva)
		if err != nil {
			return nil, err
		}
		numeroCuenta, err = noVacio("datos_bancarios.numero_cuenta", req.DatosBancarios.NumeroCuenta)
		if err != nil {
			return nil, err
		}
	}

	var respuesta EmpleadoResponse

	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		empleadoID, err := q.CreateEmpleado(ctx, codigo, req.TipoIdentificacionID, numeroIdent, nombre, fechaIngreso)
		if err != nil {
			return traducirErrorBD(err, "crear empleado")
		}

		if _, err := q.CrearVersionDatosPersonales(ctx, empleadoID,
			req.DatosPersonales.Direccion, req.DatosPersonales.Telefono, req.DatosPersonales.Email,
			fechaEfectivaPersonales); err != nil {
			return traducirErrorBD(err, "crear datos personales")
		}

		// TODO F22: aquí se escribe salario_base_nominal.
		if _, err := q.CrearVersionDatosLaborales(ctx, empleadoID, req.DatosLaborales.PuestoID, req.DatosLaborales.SalarioBaseNominal,
			req.DatosLaborales.TipoContratoID, req.DatosLaborales.ProgramaPresupuestarioID, req.DatosLaborales.SindicatoID,
			fechaEfectivaLaborales); err != nil {
			return traducirErrorBD(err, "crear datos laborales")
		}

		if req.DatosBancarios != nil {
			if _, err := q.CrearVersionDatosBancarios(ctx, empleadoID, req.DatosBancarios.BancoID,
				req.DatosBancarios.TipoCuenta, numeroCuenta, fechaEfectivaBancarios); err != nil {
				return traducirErrorBD(err, "crear datos bancarios")
			}
		}

		respuesta = EmpleadoResponse{ID: empleadoID, CodigoInterno: codigo, Activo: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &respuesta, nil
}

func (s *empleadoService) GetEmpleado(ctx context.Context, empleadoID int64) (*EmpleadoResponse, error) {
	q := dto.New(s.sqlDB)
	e, err := q.GetEmpleadoByID(ctx, empleadoID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEmpleadoNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("obtener empleado %d: %w", empleadoID, err)
	}
	return &EmpleadoResponse{ID: e.ID, CodigoInterno: e.CodigoInterno, Activo: e.Activo}, nil
}

func (s *empleadoService) ListarEmpleados(ctx context.Context, req ListarEmpleadosRequest) (*ListarEmpleadosResponse, error) {
	pag := req.Paginacion.Normalizado()
	q := dto.New(s.sqlDB)

	rows, err := q.ListEmpleados(ctx, req.Activo, req.CodigoInterno, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, fmt.Errorf("listar empleados: %w", err)
	}
	total, err := q.CountEmpleados(ctx, req.Activo, req.CodigoInterno)
	if err != nil {
		return nil, fmt.Errorf("contar empleados: %w", err)
	}

	out := make([]EmpleadoResponse, 0, len(rows))
	for _, e := range rows {
		out = append(out, EmpleadoResponse{ID: e.ID, CodigoInterno: e.CodigoInterno, Activo: e.Activo})
	}

	return &ListarEmpleadosResponse{
		Empleados:  out,
		Paginacion: pagination.NuevaMeta(pag, total),
	}, nil
}

// -----------------------------------------------------------------------------
// REGLA DE VIGENCIA DE 2 PASOS
// Cada Actualizar* de abajo, dentro de la MISMA Tx auditada
// (db.WithAuditContext, ADR-000 sección 2):
//   0. bloquea la fila del empleado (FOR UPDATE) y verifica que exista;
//   1. lee la versión vigente, valida que la fecha efectiva sea posterior a su
//      inicio y la cierra con fecha_fin_vigencia = fecha efectiva;
//   2. abre la nueva versión vigente.
// Si no hay versión vigente (primera carga de ese infotype) solo se inserta.
//
// NOTA (convención de fin de vigencia): se conserva la convención previa del
// proyecto — la versión cerrada termina EN la fecha efectiva de la nueva. Si
// el resto del sistema trata fecha_fin_vigencia como inclusiva, ese día queda
// cubierto por ambas versiones; ver informe F03 (punto dudoso).
// -----------------------------------------------------------------------------

func (s *empleadoService) ActualizarDatosPersonales(ctx context.Context, usuarioID, empleadoID int64, in DatosPersonalesInput) error {
	fechaEfectiva, err := parseFecha("fecha_efectiva", in.FechaEfectiva)
	if err != nil {
		return err
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := bloquearEmpleado(ctx, q, empleadoID); err != nil {
			return err
		}

		vigente, err := q.GetDatosPersonalesVigente(ctx, empleadoID)
		switch {
		case err == nil:
			if err := validarNuevaVigencia("fecha_efectiva", fechaEfectiva, vigente.FechaInicioVigencia); err != nil {
				return err
			}
			// Paso 1: cerrar vigente anterior.
			if err := q.CerrarDatosPersonalesVigente(ctx, empleadoID, fechaEfectiva); err != nil {
				return fmt.Errorf("cerrar datos personales vigentes: %w", err)
			}
		case errors.Is(err, sql.ErrNoRows):
			// primera versión: nada que cerrar
		default:
			return fmt.Errorf("obtener datos personales vigentes: %w", err)
		}

		// Paso 2: abrir nueva versión vigente.
		if _, err := q.CrearVersionDatosPersonales(ctx, empleadoID, in.Direccion, in.Telefono, in.Email, fechaEfectiva); err != nil {
			return traducirErrorBD(err, "crear version datos personales")
		}
		return nil
	})
}

func (s *empleadoService) ActualizarDatosLaborales(ctx context.Context, usuarioID, empleadoID int64, in DatosLaboralesInput) error {
	fechaEfectiva, err := parseFecha("fecha_efectiva", in.FechaEfectiva)
	if err != nil {
		return err
	}
	// TODO F22: salario_base_nominal — fuente única de salario (D5).
	if err := validarMontoSalario("salario_base_nominal", in.SalarioBaseNominal); err != nil {
		return err
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := bloquearEmpleado(ctx, q, empleadoID); err != nil {
			return err
		}

		vigente, err := q.GetDatosLaboralesVigente(ctx, empleadoID)
		switch {
		case err == nil:
			if err := validarNuevaVigencia("fecha_efectiva", fechaEfectiva, vigente.FechaInicioVigencia); err != nil {
				return err
			}
			if err := q.CerrarDatosLaboralesVigente(ctx, empleadoID, fechaEfectiva); err != nil {
				return fmt.Errorf("cerrar datos laborales vigentes: %w", err)
			}
		case errors.Is(err, sql.ErrNoRows):
		default:
			return fmt.Errorf("obtener datos laborales vigentes: %w", err)
		}

		// TODO F22: aquí se escribe salario_base_nominal.
		if _, err := q.CrearVersionDatosLaborales(ctx, empleadoID, in.PuestoID, in.SalarioBaseNominal,
			in.TipoContratoID, in.ProgramaPresupuestarioID, in.SindicatoID, fechaEfectiva); err != nil {
			return traducirErrorBD(err, "crear version datos laborales")
		}
		return nil
	})
}

func (s *empleadoService) ActualizarDatosBancarios(ctx context.Context, usuarioID, empleadoID int64, in DatosBancariosInput) error {
	fechaEfectiva, err := parseFecha("fecha_efectiva", in.FechaEfectiva)
	if err != nil {
		return err
	}
	numeroCuenta, err := noVacio("numero_cuenta", in.NumeroCuenta)
	if err != nil {
		return err
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := bloquearEmpleado(ctx, q, empleadoID); err != nil {
			return err
		}

		vigente, err := q.GetDatosBancariosVigente(ctx, empleadoID)
		switch {
		case err == nil:
			if err := validarNuevaVigencia("fecha_efectiva", fechaEfectiva, vigente.FechaInicioVigencia); err != nil {
				return err
			}
			if err := q.CerrarDatosBancariosVigente(ctx, empleadoID, fechaEfectiva); err != nil {
				return fmt.Errorf("cerrar datos bancarios vigentes: %w", err)
			}
		case errors.Is(err, sql.ErrNoRows):
		default:
			return fmt.Errorf("obtener datos bancarios vigentes: %w", err)
		}

		if _, err := q.CrearVersionDatosBancarios(ctx, empleadoID, in.BancoID, in.TipoCuenta, numeroCuenta, fechaEfectiva); err != nil {
			return traducirErrorBD(err, "crear version datos bancarios")
		}
		return nil
	})
}

// -----------------------------------------------------------------------------
// Familiares: empleado_datos_familiares es una declaración AGREGADA por
// empleado (aplica_credito_conyuge + numero_hijos_dependientes), con como
// máximo una fila vigente por empleado_id (uk_edf_una_vigente). Por eso
// AgregarFamiliar sigue el mismo patrón de 2 pasos que el resto de infotypes.
// -----------------------------------------------------------------------------

func (s *empleadoService) AgregarFamiliar(ctx context.Context, usuarioID, empleadoID int64, in DatosFamiliarInput) error {
	fechaEfectiva, err := parseFecha("fecha_efectiva", in.FechaEfectiva)
	if err != nil {
		return err
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := bloquearEmpleado(ctx, q, empleadoID); err != nil {
			return err
		}

		vigentes, err := q.ListDatosFamiliaresVigentes(ctx, empleadoID)
		if err != nil {
			return fmt.Errorf("obtener declaracion familiar vigente: %w", err)
		}
		if len(vigentes) > 0 {
			// uk_edf_una_vigente garantiza a lo sumo una.
			if err := validarNuevaVigencia("fecha_efectiva", fechaEfectiva, vigentes[0].FechaInicioVigencia); err != nil {
				return err
			}
			if err := q.CerrarDatosFamiliarVigentePorEmpleado(ctx, empleadoID, fechaEfectiva); err != nil {
				return fmt.Errorf("cerrar declaracion familiar vigente: %w", err)
			}
		}

		if _, err := q.CrearVersionDatosFamiliares(ctx, empleadoID, in.AplicaCreditoConyuge, in.NumeroHijosDependientes, fechaEfectiva); err != nil {
			return traducirErrorBD(err, "crear declaracion familiar")
		}
		return nil
	})
}

func (s *empleadoService) ActualizarFamiliar(ctx context.Context, usuarioID, familiarID int64, in DatosFamiliarInput) error {
	fechaEfectiva, err := parseFecha("fecha_efectiva", in.FechaEfectiva)
	if err != nil {
		return err
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		// Se lee primero (sin lock) solo para saber a qué empleado pertenece;
		// luego se bloquea al empleado y se RELEE la versión, por si otra Tx la
		// cerró entre ambas lecturas.
		previa, err := q.GetDatosFamiliarVigentePorID(ctx, familiarID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrFamiliarNoEncontrado
		}
		if err != nil {
			return fmt.Errorf("obtener declaracion familiar vigente: %w", err)
		}

		if err := bloquearEmpleado(ctx, q, previa.EmpleadoID); err != nil {
			return err
		}

		familiarVigente, err := q.GetDatosFamiliarVigentePorID(ctx, familiarID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrFamiliarNoEncontrado
		}
		if err != nil {
			return fmt.Errorf("releer declaracion familiar vigente: %w", err)
		}

		if err := validarNuevaVigencia("fecha_efectiva", fechaEfectiva, familiarVigente.FechaInicioVigencia); err != nil {
			return err
		}

		// Paso 1: cerrar la versión puntual de ESTA declaración.
		if err := q.CerrarDatosFamiliarVigentePorID(ctx, familiarID, fechaEfectiva); err != nil {
			return fmt.Errorf("cerrar declaracion familiar vigente: %w", err)
		}

		// Paso 2: abrir nueva versión, bajo el mismo empleado_id original.
		if _, err := q.CrearVersionDatosFamiliares(ctx, familiarVigente.EmpleadoID, in.AplicaCreditoConyuge, in.NumeroHijosDependientes, fechaEfectiva); err != nil {
			return traducirErrorBD(err, "crear version declaracion familiar")
		}
		return nil
	})
}

func (s *empleadoService) ListarFamiliaresVigentes(ctx context.Context, empleadoID int64) ([]dto.EmpleadoDatosFamiliare, error) {
	q := dto.New(s.sqlDB)
	out, err := q.ListDatosFamiliaresVigentes(ctx, empleadoID)
	if err != nil {
		return nil, fmt.Errorf("listar familiares vigentes: %w", err)
	}
	if out == nil {
		out = []dto.EmpleadoDatosFamiliare{} // JSON [] en vez de null
	}
	return out, nil
}
