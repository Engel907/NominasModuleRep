package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/security"
)

var (
	ErrUsuarioAdministracionNoEncontrado = errors.New("auth: usuario no encontrado")
	ErrRolAdministracionNoEncontrado     = errors.New("auth: rol no encontrado")
	ErrUltimoAdministrador               = errors.New("auth: no se puede quitar el último rol de administración")
	ErrNoEmpleadoVinculado               = errors.New("auth: el usuario autenticado no tiene un empleado vinculado")
	ErrDatosAdministracionInvalidos      = errors.New("auth: datos de administración inválidos")
)

type CrearUsuarioRequest struct {
	NombreUsuario  string `json:"nombre_usuario" binding:"required,max=60"`
	NombreCompleto string `json:"nombre_completo" binding:"required,max=200"`
	Correo         string `json:"correo" binding:"required,email,max=150"`
	EmpleadoID     *int64 `json:"empleado_id"`
}

type ActualizarUsuarioRequest struct {
	Nombre         *string         `json:"nombre"`
	NombreCompleto *string         `json:"nombre_completo"`
	Correo         *string         `json:"correo"`
	EmpleadoID     json.RawMessage `json:"empleado_id"`
}

type UsuarioPublico struct {
	ID                     int64     `json:"id"`
	NombreUsuario          string    `json:"nombre_usuario"`
	NombreCompleto         string    `json:"nombre_completo"`
	Correo                 string    `json:"correo"`
	EmpleadoID             *int64    `json:"empleado_id"`
	Activo                 bool      `json:"activo"`
	RequiereCambioPassword bool      `json:"requiere_cambio_password"`
	Roles                  []string  `json:"roles"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type CrearUsuarioResponse struct {
	Usuario          UsuarioPublico `json:"usuario"`
	PasswordTemporal string         `json:"password_temporal"`
}

type UsuariosAdminStore interface {
	CrearUsuarioAdministracion(context.Context, string, string, string, sql.NullInt64, string) (int64, error)
	ActualizarUsuarioAdministracion(context.Context, int64, *string, *string, **int64) (bool, error)
	RestablecerPasswordTemporal(context.Context, int64, string) error
	UsuarioAdministracionPorID(context.Context, int64) (dto.UsuarioAdministracion, error)
	ListarUsuariosAdministracion(context.Context, dto.FiltroUsuarios) ([]dto.UsuarioAdministracion, int64, error)
	ListarRolesAdministracion(context.Context) ([]dto.RolAdministracion, error)
	ListarPermisosDeRol(context.Context, int64) ([]dto.PermisoAdministracion, error)
	EmpleadoIDDeUsuario(context.Context, int64) (int64, error)
}

type ultimoAdminStore interface {
	ContarAdministradoresActivos(context.Context) (int64, error)
}

type bloquearRolStore interface {
	BloquearRolPorID(context.Context, int64) (dto.Rol, error)
	UsuarioTieneRolVigente(context.Context, int64, int64) (bool, error)
	CerrarUsuarioRolVigente(context.Context, int64, int64, time.Time) error
	IncrementarPermisosVersion(context.Context, int64) (uint32, error)
}

var (
	_ UsuariosAdminStore = (*dto.Queries)(nil)
	_ ultimoAdminStore   = (*dto.Queries)(nil)
	_ bloquearRolStore   = (*dto.Queries)(nil)
)

func (s *Servicio) adminStore() (UsuariosAdminStore, error) {
	st, ok := s.store.(UsuariosAdminStore)
	if !ok {
		return nil, errors.New("auth: el store no implementa administración de usuarios")
	}
	return st, nil
}

func usuarioPublico(u dto.UsuarioAdministracion) UsuarioPublico {
	return UsuarioPublico{
		ID: u.ID, NombreUsuario: u.NombreUsuario, NombreCompleto: u.NombreCompleto,
		Correo: u.Correo, EmpleadoID: u.EmpleadoIDJSON, Activo: u.Activo,
		RequiereCambioPassword: u.RequiereCambioPassword, Roles: u.Roles,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func generarPasswordTemporal() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: generando contraseña temporal: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *Servicio) CrearUsuario(ctx context.Context, req CrearUsuarioRequest, actorID int64) (CrearUsuarioResponse, error) {
	if actorID <= 0 {
		return CrearUsuarioResponse{}, dbNilActorError()
	}
	req.NombreUsuario = strings.TrimSpace(req.NombreUsuario)
	req.NombreCompleto = strings.TrimSpace(req.NombreCompleto)
	req.Correo = strings.TrimSpace(req.Correo)
	if req.NombreUsuario == "" || len(req.NombreUsuario) > 60 ||
		req.NombreCompleto == "" || len(req.NombreCompleto) > 200 ||
		req.Correo == "" || len(req.Correo) > 150 {
		return CrearUsuarioResponse{}, ErrDatosAdministracionInvalidos
	}
	email, err := mail.ParseAddress(req.Correo)
	if err != nil || email.Address != req.Correo {
		return CrearUsuarioResponse{}, ErrDatosAdministracionInvalidos
	}
	password, err := generarPasswordTemporal()
	if err != nil {
		return CrearUsuarioResponse{}, err
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return CrearUsuarioResponse{}, fmt.Errorf("auth: generando hash temporal: %w", err)
	}
	var empleado sql.NullInt64
	if req.EmpleadoID != nil {
		if *req.EmpleadoID <= 0 {
			return CrearUsuarioResponse{}, ErrDatosAdministracionInvalidos
		}
		empleado = sql.NullInt64{Int64: *req.EmpleadoID, Valid: true}
	}
	var usuario dto.UsuarioAdministracion
	err = s.ejecutarTx(ctx, actorID, func(ctx context.Context, st Store) error {
		adminStore, ok := st.(UsuariosAdminStore)
		if !ok {
			return errors.New("auth: el store transaccional no implementa administración de usuarios")
		}
		id, createErr := adminStore.CrearUsuarioAdministracion(ctx, req.NombreUsuario,
			strings.TrimSpace(req.NombreCompleto), strings.TrimSpace(req.Correo), empleado, hash)
		if createErr != nil {
			return createErr
		}
		var err error
		usuario, err = adminStore.UsuarioAdministracionPorID(ctx, id)
		return err
	})
	if err != nil {
		return CrearUsuarioResponse{}, err
	}
	return CrearUsuarioResponse{Usuario: usuarioPublico(usuario), PasswordTemporal: password}, nil
}

func dbNilActorError() error {
	return errors.New("auth: actor autenticado requerido")
}

func (s *Servicio) ListarUsuarios(ctx context.Context, filtro dto.FiltroUsuarios) ([]dto.UsuarioAdministracion, int64, error) {
	store, err := s.adminStore()
	if err != nil {
		return nil, 0, err
	}
	return store.ListarUsuariosAdministracion(ctx, filtro)
}

func (s *Servicio) ObtenerUsuarioAdministracion(ctx context.Context, id int64) (dto.UsuarioAdministracion, error) {
	return s.adminUserByID(ctx, id)
}

func (s *Servicio) adminUserByID(ctx context.Context, id int64) (dto.UsuarioAdministracion, error) {
	store, err := s.adminStore()
	if err != nil {
		return dto.UsuarioAdministracion{}, err
	}
	u, err := store.UsuarioAdministracionPorID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.UsuarioAdministracion{}, ErrUsuarioAdministracionNoEncontrado
	}
	return u, err
}

func (s *Servicio) ActualizarUsuario(ctx context.Context, id int64, req ActualizarUsuarioRequest, actorID int64) error {
	if req.NombreCompleto != nil && req.Nombre != nil {
		return ErrDatosAdministracionInvalidos
	}
	if req.NombreCompleto == nil {
		req.NombreCompleto = req.Nombre
	}
	var empleadoID **int64
	if len(req.EmpleadoID) > 0 {
		empleadoID = new(*int64)
		if string(req.EmpleadoID) != "null" {
			var value int64
			if err := json.Unmarshal(req.EmpleadoID, &value); err != nil || value <= 0 {
				return ErrDatosAdministracionInvalidos
			}
			*empleadoID = &value
		}
	}
	if req.NombreCompleto == nil && req.Correo == nil && empleadoID == nil {
		return ErrDatosAdministracionInvalidos
	}
	if req.NombreCompleto != nil {
		nombre := strings.TrimSpace(*req.NombreCompleto)
		if nombre == "" || len(nombre) > 200 {
			return ErrDatosAdministracionInvalidos
		}
		req.NombreCompleto = &nombre
	}
	if req.Correo != nil {
		correo := strings.TrimSpace(*req.Correo)
		email, err := mail.ParseAddress(correo)
		if correo == "" || len(correo) > 150 || err != nil || email.Address != correo {
			return ErrDatosAdministracionInvalidos
		}
		req.Correo = &correo
	}
	return s.ejecutarTx(ctx, actorID, func(ctx context.Context, st Store) error {
		store, ok := st.(UsuariosAdminStore)
		if !ok {
			return errors.New("auth: el store transaccional no implementa administración de usuarios")
		}
		_, err := store.ActualizarUsuarioAdministracion(ctx, id, req.NombreCompleto, req.Correo, empleadoID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUsuarioAdministracionNoEncontrado
		}
		return err
	})
}

func (s *Servicio) RestablecerPasswordAdmin(ctx context.Context, id, actorID int64) (string, error) {
	password, err := generarPasswordTemporal()
	if err != nil {
		return "", err
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("auth: generando hash temporal: %w", err)
	}
	err = s.ejecutarTx(ctx, actorID, func(ctx context.Context, st Store) error {
		store, ok := st.(UsuariosAdminStore)
		if !ok {
			return errors.New("auth: el store transaccional no implementa administración de usuarios")
		}
		if err := store.RestablecerPasswordTemporal(ctx, id, hash); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUsuarioAdministracionNoEncontrado
			}
			return err
		}
		if err := st.RevocarTodosLosRefreshTokensDeUsuario(ctx, id); err != nil {
			return err
		}
		_, err := st.IncrementarPermisosVersion(ctx, id)
		return err
	})
	if err != nil {
		return "", err
	}
	return password, nil
}

func (s *Servicio) Roles(ctx context.Context) ([]dto.RolAdministracion, error) {
	store, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return store.ListarRolesAdministracion(ctx)
}

func (s *Servicio) PermisosDeRol(ctx context.Context, id int64) ([]dto.PermisoAdministracion, error) {
	store, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	_, err = s.store.GetRolPorID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRolAdministracionNoEncontrado
	}
	if err != nil {
		return nil, err
	}
	permisos, err := store.ListarPermisosDeRol(ctx, id)
	if err != nil {
		return nil, err
	}
	if permisos == nil {
		return []dto.PermisoAdministracion{}, nil
	}
	return permisos, nil
}

func (s *Servicio) EmpleadoIDDeUsuario(ctx context.Context, usuarioID int64) (int64, error) {
	store, err := s.adminStore()
	if err != nil {
		return 0, err
	}
	id, err := store.EmpleadoIDDeUsuario(ctx, usuarioID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNoEmpleadoVinculado
	}
	return id, err
}

func (s *Servicio) RevocarRolSeguro(ctx context.Context, usuarioID int64, req RevocarRolRequest, actorID int64) error {
	return s.ejecutarTx(ctx, actorID, func(ctx context.Context, st Store) error {
		q, ok := st.(bloquearRolStore)
		if !ok {
			return errors.New("auth: el store transaccional no permite validar la revocación del rol")
		}
		if _, err := st.GetUsuarioPorID(ctx, usuarioID); err != nil {
			return err
		}
		rol, err := q.BloquearRolPorID(ctx, req.RolID)
		if err != nil {
			return err
		}
		if rol.Codigo == "ADMINISTRADOR_SISTEMA" {
			tieneRol, err := q.UsuarioTieneRolVigente(ctx, usuarioID, req.RolID)
			if err != nil {
				return err
			}
			if !tieneRol {
				return nil
			}
			countStore, ok := st.(ultimoAdminStore)
			if !ok {
				return errors.New("auth: el store transaccional no permite verificar administradores activos")
			}
			n, err := countStore.ContarAdministradoresActivos(ctx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return ErrUltimoAdministrador
			}
		}
		if err := q.CerrarUsuarioRolVigente(ctx, usuarioID, req.RolID, time.Now()); err != nil {
			return err
		}
		_, err = q.IncrementarPermisosVersion(ctx, usuarioID)
		return err
	})
}
