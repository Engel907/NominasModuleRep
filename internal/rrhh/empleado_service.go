package rrhh

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"planillas-ccdr/internal/cripto"
	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/paginacion"

	"github.com/shopspring/decimal"
)

// ErrEmpleadoNoEncontrado se devuelve cuando el empleado_id no existe.
var ErrEmpleadoNoEncontrado = errors.New("empleado no encontrado")

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

// DatosPersonalesInput: dirección, teléfono y correo del empleado.
type DatosPersonalesInput struct {
	Direccion     string `json:"direccion" binding:"max=255"`
	Telefono      string `json:"telefono" binding:"max=30"`
	Email         string `json:"email" binding:"omitempty,email,max=150"`
	FechaEfectiva string `json:"fecha_efectiva" binding:"required"` // fecha_inicio_vigencia
}

// salarioDerivadoPorBD: valor que se envía en salario_base_nominal (NOT NULL).
// trg_edl_salario_derivado_bi lo reemplaza por el salario del contrato vigente;
// si aún no hay contrato, trg_contratos_sync_salario_ai lo actualiza al crearlo.
var salarioDerivadoPorBD = decimal.Zero

// DatosLaboralesInput: datos laborales versionados del empleado.
type DatosLaboralesInput struct {
	PuestoID int64 `json:"puesto_id" binding:"required,gt=0"`
	// El salario no se captura aquí: la fuente única es
	// contratos.salario_pactado; empleado_datos_laborales.salario_base_nominal
	// es una copia derivada que mantiene la BD (triggers de 000124).
	TipoContratoID           int64  `json:"tipo_contrato_id" binding:"required,gt=0"`
	ProgramaPresupuestarioID int64  `json:"programa_presupuestario_id" binding:"required,gt=0"`
	SindicatoID              *int64 `json:"sindicato_id" binding:"omitempty,gt=0"` // opcional (afiliación sindical)
	// RegimenCCSSID: régimen CCSS para la base mínima contributiva.
	// Opcional en la API (la asignación es dato humano), pero sin él el
	// cálculo de planilla del empleado falla con empleado_sin_regimen_ccss.
	RegimenCCSSID *int64 `json:"regimen_ccss_id" binding:"omitempty,gt=0"`
	FechaEfectiva string `json:"fecha_efectiva" binding:"required"`
}

// DatosBancariosInput: cuenta bancaria del empleado.
type DatosBancariosInput struct {
	BancoID       int64  `json:"banco_id" binding:"required,gt=0"`
	TipoCuenta    string `json:"tipo_cuenta" binding:"required,oneof=corriente ahorros"`
	NumeroCuenta  string `json:"numero_cuenta" binding:"required,max=40"`
	FechaEfectiva string `json:"fecha_efectiva" binding:"required"`
}

// DatosFamiliarInput: declaración familiar agregada (cónyuge e hijos
// dependientes) para los créditos fiscales.
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
	// Paginacion no se bindea del query con gin: el handler la llena con
	// paginacion.Desde(c).
	Paginacion    paginacion.Parametros `form:"-"`
	Activo        *bool                 `form:"activo"`
	CodigoInterno *string               `form:"codigo_interno"`
	// SinRegimenCCSS: solo empleados sin regimen_ccss_id vigente.
	SinRegimenCCSS bool `form:"sin_regimen_ccss"`
}

type ListarEmpleadosResponse struct {
	Empleados  []EmpleadoResponse `json:"empleados"`
	Paginacion paginacion.Meta    `json:"paginacion"`
}

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

	// DatosSensibles (Ley 8968): revela con permiso (y registra el
	// acceso) o enmascara.
	DatosSensibles(ctx context.Context, sol SolicitudDatosSensibles, empleadoID int64) (*DatosSensiblesResponse, error)
}

type empleadoService struct {
	sqlDB   *sql.DB
	llavero *cripto.Llavero
}

// NewEmpleadoService: llavero cifra los datos personales al escribir.
// Sin llavero, escribir un dato sensible falla (nunca se guarda solo en claro
// en silencio).
func NewEmpleadoService(sqlDB *sql.DB, llavero *cripto.Llavero) EmpleadoService {
	return &empleadoService{sqlDB: sqlDB, llavero: llavero}
}

// valorCifrado cifra un dato personal y calcula su hash de búsqueda.
func (s *empleadoService) valorCifrado(c dto.CampoSensible, valor string) (dto.ValorCifrado, error) {
	if s.llavero == nil {
		return dto.ValorCifrado{}, cripto.ErrLlaveroNoProvisto
	}
	ct, kid, err := s.llavero.Cifrar(c.Campo(), valor)
	if err != nil {
		return dto.ValorCifrado{}, fmt.Errorf("cifrar %s: %w", c.Campo(), err)
	}
	return dto.ValorCifrado{Cifrado: ct, Hash: s.llavero.HashBusqueda(c.Campo(), valor), Kid: kid}, nil
}

// cifrar guarda X_cifrado/X_hash/X_kid de la fila recién escrita. Valor vacío
// = nada que cifrar.
func (s *empleadoService) cifrar(ctx context.Context, q *dto.Queries, c dto.CampoSensible, id int64, valor string) error {
	if valor == "" {
		return nil
	}
	v, err := s.valorCifrado(c, valor)
	if err != nil {
		return err
	}
	if err := q.GuardarCifrado(ctx, c, id, v); err != nil {
		return fmt.Errorf("guardar cifrado de %s: %w", c.Campo(), err)
	}
	return nil
}

var (
	campoIdentificacion = dto.CampoIdentificacion
	campoDireccion      = dto.CampoDireccion
	campoTelefono       = dto.CampoTelefono
	campoCorreo         = dto.CampoCorreo
	campoCuenta         = dto.CampoCuenta
)

func (s *empleadoService) cifrarPersonales(ctx context.Context, q *dto.Queries, id int64, in DatosPersonalesInput) error {
	for _, x := range []struct {
		c dto.CampoSensible
		v string
	}{{campoDireccion, in.Direccion}, {campoTelefono, in.Telefono}, {campoCorreo, in.Email}} {
		if err := s.cifrar(ctx, q, x.c, id, x.v); err != nil {
			return err
		}
	}
	return nil
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

	identCifrada, err := s.valorCifrado(campoIdentificacion, numeroIdent)
	if err != nil {
		return nil, err
	}
	var cuentaCifrada dto.ValorCifrado
	if req.DatosBancarios != nil {
		if cuentaCifrada, err = s.valorCifrado(campoCuenta, numeroCuenta); err != nil {
			return nil, err
		}
	}

	var respuesta EmpleadoResponse

	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		empleadoID, err := q.CreateEmpleado(ctx, codigo, req.TipoIdentificacionID, identCifrada, nombre, fechaIngreso)
		if err != nil {
			return traducirErrorBD(err, "crear empleado")
		}

		personalesID, err := q.CrearVersionDatosPersonales(ctx, empleadoID, fechaEfectivaPersonales)
		if err != nil {
			return traducirErrorBD(err, "crear datos personales")
		}
		if err := s.cifrarPersonales(ctx, q, personalesID, req.DatosPersonales); err != nil {
			return err
		}

		if err := validarRegimenCCSS(ctx, q, "datos_laborales.regimen_ccss_id", req.DatosLaborales.RegimenCCSSID); err != nil {
			return err
		}
		if _, err := q.CrearVersionDatosLaborales(ctx, empleadoID, req.DatosLaborales.PuestoID, salarioDerivadoPorBD,
			req.DatosLaborales.TipoContratoID, req.DatosLaborales.ProgramaPresupuestarioID, req.DatosLaborales.SindicatoID,
			req.DatosLaborales.RegimenCCSSID, fechaEfectivaLaborales); err != nil {
			return traducirErrorBD(err, "crear datos laborales")
		}

		if req.DatosBancarios != nil {
			if _, err := q.CrearVersionDatosBancarios(ctx, empleadoID, req.DatosBancarios.BancoID,
				req.DatosBancarios.TipoCuenta, cuentaCifrada, fechaEfectivaBancarios); err != nil {
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

	rows, err := q.ListEmpleados(ctx, req.Activo, req.CodigoInterno, req.SinRegimenCCSS, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, fmt.Errorf("listar empleados: %w", err)
	}
	total, err := q.CountEmpleados(ctx, req.Activo, req.CodigoInterno, req.SinRegimenCCSS)
	if err != nil {
		return nil, fmt.Errorf("contar empleados: %w", err)
	}

	out := make([]EmpleadoResponse, 0, len(rows))
	for _, e := range rows {
		out = append(out, EmpleadoResponse{ID: e.ID, CodigoInterno: e.CodigoInterno, Activo: e.Activo})
	}

	return &ListarEmpleadosResponse{
		Empleados:  out,
		Paginacion: paginacion.NuevaMeta(pag, total),
	}, nil
}

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
			if err := q.CerrarDatosPersonalesVigente(ctx, empleadoID, fechaEfectiva); err != nil {
				return fmt.Errorf("cerrar datos personales vigentes: %w", err)
			}
		case errors.Is(err, sql.ErrNoRows):
		default:
			return fmt.Errorf("obtener datos personales vigentes: %w", err)
		}

		id, err := q.CrearVersionDatosPersonales(ctx, empleadoID, fechaEfectiva)
		if err != nil {
			return traducirErrorBD(err, "crear version datos personales")
		}
		return s.cifrarPersonales(ctx, q, id, in)
	})
}

func (s *empleadoService) ActualizarDatosLaborales(ctx context.Context, usuarioID, empleadoID int64, in DatosLaboralesInput) error {
	fechaEfectiva, err := parseFecha("fecha_efectiva", in.FechaEfectiva)
	if err != nil {
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

		if err := validarRegimenCCSS(ctx, q, "regimen_ccss_id", in.RegimenCCSSID); err != nil {
			return err
		}
		if _, err := q.CrearVersionDatosLaborales(ctx, empleadoID, in.PuestoID, salarioDerivadoPorBD,
			in.TipoContratoID, in.ProgramaPresupuestarioID, in.SindicatoID, in.RegimenCCSSID, fechaEfectiva); err != nil {
			return traducirErrorBD(err, "crear version datos laborales")
		}
		return nil
	})
}

// validarRegimenCCSS: si viene, el régimen debe existir y estar activo (400).
// nil se acepta y queda NULL (la asignación es dato humano).
func validarRegimenCCSS(ctx context.Context, q *dto.Queries, campo string, id *int64) error {
	if id == nil {
		return nil
	}
	ok, err := q.ExisteRegimenCCSSActivo(ctx, *id)
	if err != nil {
		return fmt.Errorf("validar regimen_ccss_id: %w", err)
	}
	if !ok {
		return validacion(campo, "el régimen CCSS no existe o no está activo")
	}
	return nil
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
	cuentaCifrada, err := s.valorCifrado(campoCuenta, numeroCuenta)
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

		if _, err := q.CrearVersionDatosBancarios(ctx, empleadoID, in.BancoID, in.TipoCuenta, cuentaCifrada, fechaEfectiva); err != nil {
			return traducirErrorBD(err, "crear version datos bancarios")
		}
		return nil
	})
}

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

		if err := q.CerrarDatosFamiliarVigentePorID(ctx, familiarID, fechaEfectiva); err != nil {
			return fmt.Errorf("cerrar declaracion familiar vigente: %w", err)
		}

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
		out = []dto.EmpleadoDatosFamiliare{}
	}
	return out, nil
}

// SolicitudDatosSensibles: quién pide y si tiene datos_personales.ver_sensible.
type SolicitudDatosSensibles struct {
	UsuarioID   int64
	VerSensible bool
	Recurso     string
	IP          string
	RequestID   string
}

// DatosSensiblesResponse: valores revelados o enmascarados.
type DatosSensiblesResponse struct {
	EmpleadoID           int64  `json:"empleado_id"`
	Revelado             bool   `json:"revelado"`
	NumeroIdentificacion string `json:"numero_identificacion"`
	Direccion            string `json:"direccion"`
	Telefono             string `json:"telefono"`
	CorreoElectronico    string `json:"correo_electronico"`
	NumeroCuenta         string `json:"numero_cuenta"`
}

// DatosSensibles lee y descifra los datos vigentes. Sin permiso devuelve máscaras y no
// registra acceso; con permiso revela e inserta una fila en
// registro_accesos_datos con los campos revelados.
func (s *empleadoService) DatosSensibles(ctx context.Context, sol SolicitudDatosSensibles, empleadoID int64) (*DatosSensiblesResponse, error) {
	if sol.UsuarioID <= 0 {
		return nil, validacion("usuario", "usuario autenticado requerido")
	}
	row, err := dto.New(s.sqlDB).LeerDatosSensibles(ctx, empleadoID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEmpleadoNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("leer datos sensibles del empleado %d: %w", empleadoID, err)
	}
	out := &DatosSensiblesResponse{EmpleadoID: empleadoID, Revelado: sol.VerSensible}
	var revelados []string
	for _, x := range []struct {
		c    dto.CampoSensible
		v    dto.ValorSensible
		tipo cripto.TipoDato
		dst  *string
	}{
		{campoIdentificacion, row.NumeroIdentificacion, cripto.DatoIdentificacion, &out.NumeroIdentificacion},
		{campoDireccion, row.Direccion, cripto.DatoDireccion, &out.Direccion},
		{campoTelefono, row.Telefono, cripto.DatoTelefono, &out.Telefono},
		{campoCorreo, row.Correo, cripto.DatoCorreo, &out.CorreoElectronico},
		{campoCuenta, row.NumeroCuenta, cripto.DatoCuenta, &out.NumeroCuenta},
	} {
		valor, err := s.valorSensible(x.c, x.v)
		if err != nil {
			return nil, err
		}
		if valor == "" {
			continue
		}
		if sol.VerSensible {
			*x.dst = valor
			revelados = append(revelados, x.c.Columna)
		} else {
			*x.dst = cripto.Enmascarar(x.tipo, valor)
		}
	}
	if sol.VerSensible && len(revelados) > 0 {
		err := db.WithAuditContext(ctx, s.sqlDB, sol.UsuarioID, func(ctx context.Context, q *dto.Queries) error {
			return q.InsertarRegistroAcceso(ctx, dto.RegistroAcceso{
				UsuarioID: sol.UsuarioID, EmpleadoID: empleadoID, Recurso: sol.Recurso,
				Campos: strings.Join(revelados, ","), IP: sol.IP, RequestID: sol.RequestID,
			})
		})
		if err != nil {
			return nil, fmt.Errorf("registrar acceso a datos sensibles: %w", err)
		}
	}
	return out, nil
}

func (s *empleadoService) valorSensible(c dto.CampoSensible, v dto.ValorSensible) (string, error) {
	if len(v.Cifrado) == 0 {
		return "", nil
	}
	if s.llavero == nil {
		return "", cripto.ErrLlaveroNoProvisto
	}
	valor, err := s.llavero.Descifrar(c.Campo(), v.Cifrado, v.Kid.String)
	if err != nil {
		return "", fmt.Errorf("descifrar %s: %w", c.Campo(), err)
	}
	return valor, nil
}
