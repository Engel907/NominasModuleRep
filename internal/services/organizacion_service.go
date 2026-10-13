// services/organizacion_service.go
//
// Fase 3 — Puestos, Unidades Organizativas, Programas Presupuestarios y
// Clasificaciones de Puesto. Son catálogos SIN patrón de vigencia (no están
// en la lista de 19 tablas del ADR-000 sección 2), así que su CRUD es
// directo. Las escrituras van envueltas en db.WithAuditContext porque los
// triggers de auditoría (trg_aud_*) aplican a nivel de tabla y leen
// @auditoria_usuario_id.
//
// FASE 3B — cableado a funciones reales de internal/db/dto contra columnas
// reales de db/schema.sql:
//   - `puestos` no tiene columna `descripcion`.
//   - `unidades_organizativas` no tiene programa_presupuestario_id (vive en
//     empleado_datos_laborales).
//   - clasificaciones_puesto / programas_presupuestarios no tienen
//     created_at/updated_at (unidades_organizativas sí).
//
// F03 (esta revisión):
//   - Firma real de db.WithAuditContext (callback con ctx, usuarioID int64);
//     IDs int64; paginación en internal/pagination (20/100).
//   - `Activo` en los *UpdateInput pasa a *bool con binding:"required": con
//     `bool` plano, un PUT que omitía "activo" DESACTIVABA el registro en
//     silencio. Cambio de contrato: ahora es obligatorio en el PUT.
//   - Unidades: se valida unidad_padre_id (existe, no es ella misma, no
//     genera ciclos en la jerarquía).
//   - Violaciones de UNIQUE/FK de MySQL se traducen a errores tipados.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/pagination"
)

var ErrNoEncontrado = errors.New("recurso no encontrado")

// profundidadMaximaUnidades acota el recorrido ascendente de la jerarquía de
// unidades (defensa ante un ciclo preexistente en la BD).
const profundidadMaximaUnidades = 100

// ----------------------------- DTOs -----------------------------------------

type ClasificacionPuestoInput struct {
	Codigo string `json:"codigo" binding:"required,max=20"`
	Nombre string `json:"nombre" binding:"required,max=100"`
}

type ClasificacionPuestoUpdateInput struct {
	Nombre string `json:"nombre" binding:"required,max=100"`
	Activo *bool  `json:"activo" binding:"required"`
}

type ProgramaPresupuestarioInput struct {
	Codigo string `json:"codigo" binding:"required,max=20"`
	Nombre string `json:"nombre" binding:"required,max=150"`
}

type ProgramaPresupuestarioUpdateInput struct {
	Nombre string `json:"nombre" binding:"required,max=150"`
	Activo *bool  `json:"activo" binding:"required"`
}

type UnidadOrganizativaInput struct {
	Codigo        string `json:"codigo" binding:"required,max=20"`
	Nombre        string `json:"nombre" binding:"required,max=150"`
	UnidadPadreID *int64 `json:"unidad_padre_id" binding:"omitempty,gt=0"`
}

type UnidadOrganizativaUpdateInput struct {
	Nombre        string `json:"nombre" binding:"required,max=150"`
	UnidadPadreID *int64 `json:"unidad_padre_id" binding:"omitempty,gt=0"`
	Activo        *bool  `json:"activo" binding:"required"`
}

type PuestoInput struct {
	Codigo                string `json:"codigo" binding:"required,max=20"`
	Nombre                string `json:"nombre" binding:"required,max=150"`
	ClasificacionPuestoID int64  `json:"clasificacion_puesto_id" binding:"required,gt=0"`
	UnidadOrganizativaID  int64  `json:"unidad_organizativa_id" binding:"required,gt=0"`
}

type PuestoUpdateInput struct {
	Nombre                string `json:"nombre" binding:"required,max=150"`
	ClasificacionPuestoID int64  `json:"clasificacion_puesto_id" binding:"required,gt=0"`
	UnidadOrganizativaID  int64  `json:"unidad_organizativa_id" binding:"required,gt=0"`
	Activo                *bool  `json:"activo" binding:"required"`
}

// Los *ListRequest se bindean del query string con gin; Paginacion NO (el
// handler la llena con pagination.Desde(c)).

type CatalogoListRequest struct {
	Paginacion pagination.Parametros `form:"-"`
	Activo     *bool                 `form:"activo"`
	Nombre     *string               `form:"nombre"`
}

type PuestoListRequest struct {
	Paginacion            pagination.Parametros `form:"-"`
	Activo                *bool                 `form:"activo"`
	Nombre                *string               `form:"nombre"`
	UnidadOrganizativaID  *int64                `form:"unidad_organizativa_id"`
	ClasificacionPuestoID *int64                `form:"clasificacion_puesto_id"`
}

type UnidadOrganizativaListRequest struct {
	Paginacion    pagination.Parametros `form:"-"`
	Activo        *bool                 `form:"activo"`
	Nombre        *string               `form:"nombre"`
	UnidadPadreID *int64                `form:"unidad_padre_id"`
}

// requerirTexto recorta espacios y exige no vacío (binding:"required" deja
// pasar "   ").
func requerirTexto(campo, valor string) (string, error) { return noVacio(campo, valor) }

// ----------------------------- Servicio: Clasificaciones de Puesto ---------

type ClasificacionPuestoService interface {
	Crear(ctx context.Context, usuarioID int64, in ClasificacionPuestoInput) (dto.ClasificacionesPuesto, error)
	Actualizar(ctx context.Context, usuarioID, id int64, in ClasificacionPuestoUpdateInput) error
	Get(ctx context.Context, id int64) (dto.ClasificacionesPuesto, error)
	List(ctx context.Context, req CatalogoListRequest) ([]dto.ClasificacionesPuesto, pagination.Meta, error)
}

type clasificacionPuestoService struct{ sqlDB *sql.DB }

func NewClasificacionPuestoService(sqlDB *sql.DB) ClasificacionPuestoService {
	return &clasificacionPuestoService{sqlDB: sqlDB}
}

func (s *clasificacionPuestoService) Crear(ctx context.Context, usuarioID int64, in ClasificacionPuestoInput) (dto.ClasificacionesPuesto, error) {
	codigo, err := requerirTexto("codigo", in.Codigo)
	if err != nil {
		return dto.ClasificacionesPuesto{}, err
	}
	nombre, err := requerirTexto("nombre", in.Nombre)
	if err != nil {
		return dto.ClasificacionesPuesto{}, err
	}

	var creado dto.ClasificacionesPuesto
	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		id, err := q.CreateClasificacionPuesto(ctx, codigo, nombre)
		if err != nil {
			return traducirErrorBD(err, "crear clasificacion de puesto")
		}
		creado, err = q.GetClasificacionPuestoByID(ctx, id)
		if err != nil {
			return fmt.Errorf("releer clasificacion de puesto %d: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return dto.ClasificacionesPuesto{}, err
	}
	return creado, nil
}

func (s *clasificacionPuestoService) Actualizar(ctx context.Context, usuarioID, id int64, in ClasificacionPuestoUpdateInput) error {
	nombre, err := requerirTexto("nombre", in.Nombre)
	if err != nil {
		return err
	}
	if in.Activo == nil {
		return validacion("activo", "es requerido")
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if _, err := q.GetClasificacionPuestoByID(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNoEncontrado
		} else if err != nil {
			return fmt.Errorf("obtener clasificacion de puesto %d: %w", id, err)
		}
		if err := q.UpdateClasificacionPuesto(ctx, id, nombre, *in.Activo); err != nil {
			return traducirErrorBD(err, "actualizar clasificacion de puesto")
		}
		return nil
	})
}

func (s *clasificacionPuestoService) Get(ctx context.Context, id int64) (dto.ClasificacionesPuesto, error) {
	q := dto.New(s.sqlDB)
	c, err := q.GetClasificacionPuestoByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.ClasificacionesPuesto{}, ErrNoEncontrado
	}
	if err != nil {
		return dto.ClasificacionesPuesto{}, fmt.Errorf("obtener clasificacion de puesto %d: %w", id, err)
	}
	return c, nil
}

func (s *clasificacionPuestoService) List(ctx context.Context, req CatalogoListRequest) ([]dto.ClasificacionesPuesto, pagination.Meta, error) {
	pag := req.Paginacion.Normalizado()
	q := dto.New(s.sqlDB)
	rows, err := q.ListClasificacionesPuesto(ctx, req.Activo, req.Nombre, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("listar clasificaciones de puesto: %w", err)
	}
	total, err := q.CountClasificacionesPuesto(ctx, req.Activo, req.Nombre)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("contar clasificaciones de puesto: %w", err)
	}
	if rows == nil {
		rows = []dto.ClasificacionesPuesto{}
	}
	return rows, pagination.NuevaMeta(pag, total), nil
}

// ----------------------------- Servicio: Programas Presupuestarios ---------

type ProgramaPresupuestarioService interface {
	Crear(ctx context.Context, usuarioID int64, in ProgramaPresupuestarioInput) (dto.ProgramasPresupuestario, error)
	Actualizar(ctx context.Context, usuarioID, id int64, in ProgramaPresupuestarioUpdateInput) error
	Get(ctx context.Context, id int64) (dto.ProgramasPresupuestario, error)
	List(ctx context.Context, req CatalogoListRequest) ([]dto.ProgramasPresupuestario, pagination.Meta, error)
}

type programaPresupuestarioService struct{ sqlDB *sql.DB }

func NewProgramaPresupuestarioService(sqlDB *sql.DB) ProgramaPresupuestarioService {
	return &programaPresupuestarioService{sqlDB: sqlDB}
}

func (s *programaPresupuestarioService) Crear(ctx context.Context, usuarioID int64, in ProgramaPresupuestarioInput) (dto.ProgramasPresupuestario, error) {
	codigo, err := requerirTexto("codigo", in.Codigo)
	if err != nil {
		return dto.ProgramasPresupuestario{}, err
	}
	nombre, err := requerirTexto("nombre", in.Nombre)
	if err != nil {
		return dto.ProgramasPresupuestario{}, err
	}

	var creado dto.ProgramasPresupuestario
	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		id, err := q.CreateProgramaPresupuestario(ctx, codigo, nombre)
		if err != nil {
			return traducirErrorBD(err, "crear programa presupuestario")
		}
		creado, err = q.GetProgramaPresupuestarioByID(ctx, id)
		if err != nil {
			return fmt.Errorf("releer programa presupuestario %d: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return dto.ProgramasPresupuestario{}, err
	}
	return creado, nil
}

func (s *programaPresupuestarioService) Actualizar(ctx context.Context, usuarioID, id int64, in ProgramaPresupuestarioUpdateInput) error {
	nombre, err := requerirTexto("nombre", in.Nombre)
	if err != nil {
		return err
	}
	if in.Activo == nil {
		return validacion("activo", "es requerido")
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if _, err := q.GetProgramaPresupuestarioByID(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNoEncontrado
		} else if err != nil {
			return fmt.Errorf("obtener programa presupuestario %d: %w", id, err)
		}
		if err := q.UpdateProgramaPresupuestario(ctx, id, nombre, *in.Activo); err != nil {
			return traducirErrorBD(err, "actualizar programa presupuestario")
		}
		return nil
	})
}

func (s *programaPresupuestarioService) Get(ctx context.Context, id int64) (dto.ProgramasPresupuestario, error) {
	q := dto.New(s.sqlDB)
	p, err := q.GetProgramaPresupuestarioByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.ProgramasPresupuestario{}, ErrNoEncontrado
	}
	if err != nil {
		return dto.ProgramasPresupuestario{}, fmt.Errorf("obtener programa presupuestario %d: %w", id, err)
	}
	return p, nil
}

func (s *programaPresupuestarioService) List(ctx context.Context, req CatalogoListRequest) ([]dto.ProgramasPresupuestario, pagination.Meta, error) {
	pag := req.Paginacion.Normalizado()
	q := dto.New(s.sqlDB)
	rows, err := q.ListProgramasPresupuestarios(ctx, req.Activo, req.Nombre, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("listar programas presupuestarios: %w", err)
	}
	total, err := q.CountProgramasPresupuestarios(ctx, req.Activo, req.Nombre)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("contar programas presupuestarios: %w", err)
	}
	if rows == nil {
		rows = []dto.ProgramasPresupuestario{}
	}
	return rows, pagination.NuevaMeta(pag, total), nil
}

// ----------------------------- Servicio: Unidades Organizativas ------------

type UnidadOrganizativaService interface {
	Crear(ctx context.Context, usuarioID int64, in UnidadOrganizativaInput) (dto.UnidadesOrganizativa, error)
	Actualizar(ctx context.Context, usuarioID, id int64, in UnidadOrganizativaUpdateInput) error
	Get(ctx context.Context, id int64) (dto.UnidadesOrganizativa, error)
	List(ctx context.Context, req UnidadOrganizativaListRequest) ([]dto.UnidadesOrganizativa, pagination.Meta, error)
}

type unidadOrganizativaService struct{ sqlDB *sql.DB }

func NewUnidadOrganizativaService(sqlDB *sql.DB) UnidadOrganizativaService {
	return &unidadOrganizativaService{sqlDB: sqlDB}
}

// validarPadreUnidad verifica que la unidad padre exista y que asignarla no
// cree un ciclo. id == 0 en una creación (la unidad aún no existe, no puede
// formar parte de un ciclo). Recorre la cadena de ancestros de la unidad
// padre; si aparece `id` (o la cadena excede la profundidad máxima) es un
// ciclo. Corre dentro de la Tx. Dos Actualizar concurrentes sobre unidades
// distintas podrían igualmente cerrar un ciclo entre sí (ver informe F03).
func validarPadreUnidad(ctx context.Context, q *dto.Queries, id int64, padreID *int64) error {
	if padreID == nil {
		return nil
	}
	if id != 0 && *padreID == id {
		return ErrJerarquiaCiclica
	}

	actual := *padreID
	for i := 0; i < profundidadMaximaUnidades; i++ {
		u, err := q.GetUnidadOrganizativaByID(ctx, actual)
		if errors.Is(err, sql.ErrNoRows) {
			if i == 0 {
				return validacion("unidad_padre_id", "la unidad padre no existe")
			}
			return nil // cadena rota por datos inconsistentes; la FK impide llegar aquí
		}
		if err != nil {
			return fmt.Errorf("recorrer jerarquía de unidades: %w", err)
		}
		if !u.UnidadPadreID.Valid {
			return nil // llegó a la raíz sin encontrar `id`
		}
		actual = u.UnidadPadreID.Int64
		if id != 0 && actual == id {
			return ErrJerarquiaCiclica
		}
	}
	return ErrJerarquiaCiclica
}

func (s *unidadOrganizativaService) Crear(ctx context.Context, usuarioID int64, in UnidadOrganizativaInput) (dto.UnidadesOrganizativa, error) {
	codigo, err := requerirTexto("codigo", in.Codigo)
	if err != nil {
		return dto.UnidadesOrganizativa{}, err
	}
	nombre, err := requerirTexto("nombre", in.Nombre)
	if err != nil {
		return dto.UnidadesOrganizativa{}, err
	}

	var creado dto.UnidadesOrganizativa
	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if err := validarPadreUnidad(ctx, q, 0, in.UnidadPadreID); err != nil {
			return err
		}
		id, err := q.CreateUnidadOrganizativa(ctx, codigo, nombre, in.UnidadPadreID)
		if err != nil {
			return traducirErrorBD(err, "crear unidad organizativa")
		}
		creado, err = q.GetUnidadOrganizativaByID(ctx, id)
		if err != nil {
			return fmt.Errorf("releer unidad organizativa %d: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return dto.UnidadesOrganizativa{}, err
	}
	return creado, nil
}

func (s *unidadOrganizativaService) Actualizar(ctx context.Context, usuarioID, id int64, in UnidadOrganizativaUpdateInput) error {
	nombre, err := requerirTexto("nombre", in.Nombre)
	if err != nil {
		return err
	}
	if in.Activo == nil {
		return validacion("activo", "es requerido")
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if _, err := q.GetUnidadOrganizativaByID(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNoEncontrado
		} else if err != nil {
			return fmt.Errorf("obtener unidad organizativa %d: %w", id, err)
		}
		if err := validarPadreUnidad(ctx, q, id, in.UnidadPadreID); err != nil {
			return err
		}
		if err := q.UpdateUnidadOrganizativa(ctx, id, nombre, in.UnidadPadreID, *in.Activo); err != nil {
			return traducirErrorBD(err, "actualizar unidad organizativa")
		}
		return nil
	})
}

func (s *unidadOrganizativaService) Get(ctx context.Context, id int64) (dto.UnidadesOrganizativa, error) {
	q := dto.New(s.sqlDB)
	u, err := q.GetUnidadOrganizativaByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.UnidadesOrganizativa{}, ErrNoEncontrado
	}
	if err != nil {
		return dto.UnidadesOrganizativa{}, fmt.Errorf("obtener unidad organizativa %d: %w", id, err)
	}
	return u, nil
}

func (s *unidadOrganizativaService) List(ctx context.Context, req UnidadOrganizativaListRequest) ([]dto.UnidadesOrganizativa, pagination.Meta, error) {
	pag := req.Paginacion.Normalizado()
	q := dto.New(s.sqlDB)
	rows, err := q.ListUnidadesOrganizativas(ctx, req.Activo, req.Nombre, req.UnidadPadreID, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("listar unidades organizativas: %w", err)
	}
	total, err := q.CountUnidadesOrganizativas(ctx, req.Activo, req.Nombre, req.UnidadPadreID)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("contar unidades organizativas: %w", err)
	}
	if rows == nil {
		rows = []dto.UnidadesOrganizativa{}
	}
	return rows, pagination.NuevaMeta(pag, total), nil
}

// ----------------------------- Servicio: Puestos ----------------------------

type PuestoService interface {
	Crear(ctx context.Context, usuarioID int64, in PuestoInput) (dto.Puesto, error)
	Actualizar(ctx context.Context, usuarioID, id int64, in PuestoUpdateInput) error
	Get(ctx context.Context, id int64) (dto.Puesto, error)
	List(ctx context.Context, req PuestoListRequest) ([]dto.Puesto, pagination.Meta, error)
}

type puestoService struct{ sqlDB *sql.DB }

func NewPuestoService(sqlDB *sql.DB) PuestoService {
	return &puestoService{sqlDB: sqlDB}
}

func (s *puestoService) Crear(ctx context.Context, usuarioID int64, in PuestoInput) (dto.Puesto, error) {
	codigo, err := requerirTexto("codigo", in.Codigo)
	if err != nil {
		return dto.Puesto{}, err
	}
	nombre, err := requerirTexto("nombre", in.Nombre)
	if err != nil {
		return dto.Puesto{}, err
	}

	var creado dto.Puesto
	err = db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		id, err := q.CreatePuesto(ctx, codigo, nombre, in.ClasificacionPuestoID, in.UnidadOrganizativaID)
		if err != nil {
			return traducirErrorBD(err, "crear puesto")
		}
		creado, err = q.GetPuestoByID(ctx, id)
		if err != nil {
			return fmt.Errorf("releer puesto %d: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return dto.Puesto{}, err
	}
	return creado, nil
}

func (s *puestoService) Actualizar(ctx context.Context, usuarioID, id int64, in PuestoUpdateInput) error {
	nombre, err := requerirTexto("nombre", in.Nombre)
	if err != nil {
		return err
	}
	if in.Activo == nil {
		return validacion("activo", "es requerido")
	}

	return db.WithAuditContext(ctx, s.sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if _, err := q.GetPuestoByID(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNoEncontrado
		} else if err != nil {
			return fmt.Errorf("obtener puesto %d: %w", id, err)
		}
		if err := q.UpdatePuesto(ctx, id, nombre, in.ClasificacionPuestoID, in.UnidadOrganizativaID, *in.Activo); err != nil {
			return traducirErrorBD(err, "actualizar puesto")
		}
		return nil
	})
}

func (s *puestoService) Get(ctx context.Context, id int64) (dto.Puesto, error) {
	q := dto.New(s.sqlDB)
	p, err := q.GetPuestoByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.Puesto{}, ErrNoEncontrado
	}
	if err != nil {
		return dto.Puesto{}, fmt.Errorf("obtener puesto %d: %w", id, err)
	}
	return p, nil
}

func (s *puestoService) List(ctx context.Context, req PuestoListRequest) ([]dto.Puesto, pagination.Meta, error) {
	pag := req.Paginacion.Normalizado()
	q := dto.New(s.sqlDB)
	rows, err := q.ListPuestos(ctx, req.Activo, req.Nombre, req.UnidadOrganizativaID, req.ClasificacionPuestoID, pag.Limit32(), pag.Offset32())
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("listar puestos: %w", err)
	}
	total, err := q.CountPuestos(ctx, req.Activo, req.Nombre, req.UnidadOrganizativaID, req.ClasificacionPuestoID)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("contar puestos: %w", err)
	}
	if rows == nil {
		rows = []dto.Puesto{}
	}
	return rows, pagination.NuevaMeta(pag, total), nil
}
