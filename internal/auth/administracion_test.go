package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"planillas-ccdr/internal/db/dto"
)

type fakeAdminStore struct {
	*fakeStore
	adminUsers map[int64]dto.UsuarioAdministracion
}

func (f *fakeAdminStore) CrearUsuarioAdministracion(_ context.Context, user, name, email string, empleado sql.NullInt64, hash string) (int64, error) {
	f.fakeStore.mu.Lock()
	defer f.fakeStore.mu.Unlock()
	id := f.fakeStore.id()
	f.fakeStore.usuarios[id] = &dto.Usuario{
		ID: id, NombreUsuario: user, Correo: email, PasswordHash: hash,
		Activo: true, RequiereCambioPassword: true, PermisosVersion: 1,
	}
	u := dto.UsuarioAdministracion{
		ID: id, NombreUsuario: user, NombreCompleto: name, Correo: email,
		EmpleadoID: empleado, Activo: true, RequiereCambioPassword: true,
		PermisosVersion: 1, Roles: []string{}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if empleado.Valid {
		v := empleado.Int64
		u.EmpleadoIDJSON = &v
	}
	f.adminUsers[id] = u
	return id, nil
}

func (f *fakeAdminStore) ActualizarUsuarioAdministracion(_ context.Context, id int64, nombre, correo *string, empleadoID **int64) (bool, error) {
	f.fakeStore.mu.Lock()
	defer f.fakeStore.mu.Unlock()
	u, ok := f.adminUsers[id]
	if !ok {
		return false, sql.ErrNoRows
	}
	if nombre != nil {
		u.NombreCompleto = *nombre
	}
	if correo != nil {
		u.Correo = *correo
	}
	if empleadoID != nil {
		u.EmpleadoID = sql.NullInt64{}
		u.EmpleadoIDJSON = nil
		if *empleadoID != nil {
			id := **empleadoID
			u.EmpleadoID = sql.NullInt64{Int64: id, Valid: true}
			u.EmpleadoIDJSON = &id
		}
	}
	f.adminUsers[id] = u
	return true, nil
}

func (f *fakeAdminStore) RestablecerPasswordTemporal(_ context.Context, id int64, hash string) error {
	f.fakeStore.mu.Lock()
	defer f.fakeStore.mu.Unlock()
	u, ok := f.fakeStore.usuarios[id]
	if !ok {
		return sql.ErrNoRows
	}
	u.PasswordHash = hash
	u.RequiereCambioPassword = true
	return nil
}

func (f *fakeAdminStore) UsuarioAdministracionPorID(_ context.Context, id int64) (dto.UsuarioAdministracion, error) {
	f.fakeStore.mu.Lock()
	defer f.fakeStore.mu.Unlock()
	u, ok := f.adminUsers[id]
	if !ok {
		return dto.UsuarioAdministracion{}, sql.ErrNoRows
	}
	return u, nil
}

func (f *fakeAdminStore) ListarUsuariosAdministracion(context.Context, dto.FiltroUsuarios) ([]dto.UsuarioAdministracion, int64, error) {
	return nil, 0, nil
}

func (f *fakeAdminStore) ListarRolesAdministracion(context.Context) ([]dto.RolAdministracion, error) {
	return []dto.RolAdministracion{}, nil
}

func (f *fakeAdminStore) ListarPermisosDeRol(context.Context, int64) ([]dto.PermisoAdministracion, error) {
	return []dto.PermisoAdministracion{}, nil
}

func (f *fakeAdminStore) EmpleadoIDDeUsuario(context.Context, int64) (int64, error) {
	return 0, sql.ErrNoRows
}

func (f *fakeAdminStore) ContarAdministradoresActivos(context.Context) (int64, error) {
	f.fakeStore.mu.Lock()
	defer f.fakeStore.mu.Unlock()
	var count int64
	for id, roles := range f.fakeStore.usuariosRoles {
		if roles[1] && f.fakeStore.usuarios[id] != nil && f.fakeStore.usuarios[id].Activo {
			count++
		}
	}
	return count, nil
}

func (f *fakeAdminStore) BloquearRolPorID(_ context.Context, id int64) (dto.Rol, error) {
	f.fakeStore.mu.Lock()
	defer f.fakeStore.mu.Unlock()
	rol := f.fakeStore.roles[id]
	if rol == nil {
		return dto.Rol{}, sql.ErrNoRows
	}
	return *rol, nil
}

func (f *fakeAdminStore) UsuarioTieneRolVigente(_ context.Context, usuarioID, rolID int64) (bool, error) {
	f.fakeStore.mu.Lock()
	defer f.fakeStore.mu.Unlock()
	return f.fakeStore.usuariosRoles[usuarioID][rolID], nil
}

func (f *fakeAdminStore) CerrarUsuarioRolVigente(_ context.Context, usuarioID, rolID int64, _ time.Time) error {
	f.fakeStore.mu.Lock()
	defer f.fakeStore.mu.Unlock()
	f.fakeStore.usuariosRoles[usuarioID][rolID] = false
	return nil
}

func TestCrearUsuario_EmiteTemporalUnaVezSinDevolverHash(t *testing.T) {
	env := nuevoEntorno(t)
	admin := &fakeAdminStore{fakeStore: env.fs, adminUsers: map[int64]dto.UsuarioAdministracion{}}
	env.s.store = admin
	env.s.txRunner = func(ctx context.Context, _ int64, fn func(context.Context, Store) error) error {
		return fn(ctx, admin)
	}

	resp, err := env.s.CrearUsuario(context.Background(), CrearUsuarioRequest{
		NombreUsuario: "nuevo.usuario", NombreCompleto: "Nuevo Usuario", Correo: "nuevo@example.com",
	}, idAna)
	if err != nil {
		t.Fatal(err)
	}
	if resp.PasswordTemporal == "" || len(resp.PasswordTemporal) < 40 {
		t.Fatalf("contraseña temporal no generada correctamente")
	}
	u, err := admin.GetUsuarioPorID(context.Background(), resp.Usuario.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !u.RequiereCambioPassword || u.PasswordHash == "" || u.PasswordHash == resp.PasswordTemporal {
		t.Fatalf("estado/hash de cuenta incorrecto: %+v", u)
	}
	body, err := json.Marshal(resp.Usuario)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "" || containsAny(string(body), "password_hash", u.PasswordHash, "token") {
		t.Fatalf("la representación pública expone credenciales: %s", body)
	}
}

func TestRestablecerPasswordAdmin_RevocaSesionesYFuerzaCambio(t *testing.T) {
	env := nuevoEntorno(t)
	admin := &fakeAdminStore{fakeStore: env.fs, adminUsers: map[int64]dto.UsuarioAdministracion{}}
	env.s.store = admin
	env.s.txRunner = func(ctx context.Context, _ int64, fn func(context.Context, Store) error) error {
		return fn(ctx, admin)
	}
	_, err := admin.CrearRefreshTokenConFamilia(context.Background(), idAna, "token-hash", time.Now().Add(time.Hour), "family", "", "")
	if err != nil {
		t.Fatal(err)
	}

	password, err := env.s.RestablecerPasswordAdmin(context.Background(), idAna, idBloqueado)
	if err != nil {
		t.Fatal(err)
	}
	u := env.fs.usuario(idAna)
	if password == "" || u.PasswordHash == password || !u.RequiereCambioPassword || u.PermisosVersion != 2 {
		t.Fatalf("reset no actualizó credenciales/estado: %+v", u)
	}
	if !env.fs.refresh[0].Revocado {
		t.Fatal("el reset administrativo no revocó la sesión")
	}
}

func TestCambiarEstadoUsuario_ProhibeAutoDesactivacion(t *testing.T) {
	env := nuevoEntorno(t)
	err := env.s.CambiarEstadoUsuario(context.Background(), idAna, false, idAna)
	if !errors.Is(err, ErrAutoDesactivacion) {
		t.Fatalf("error = %v; quería ErrAutoDesactivacion", err)
	}
	if !env.fs.usuario(idAna).Activo {
		t.Fatal("la cuenta se desactivó pese a ser la cuenta autenticada")
	}
}

func TestRevocarRol_ProtegeAlUltimoAdministrador(t *testing.T) {
	env := nuevoEntorno(t)
	admin := &fakeAdminStore{fakeStore: env.fs, adminUsers: map[int64]dto.UsuarioAdministracion{}}
	admin.roles[1] = &dto.Rol{ID: 1, Codigo: "ADMINISTRADOR_SISTEMA", Activo: true}
	admin.usuariosRoles[idAna] = map[int64]bool{1: true}
	env.s.store = admin
	env.s.txRunner = func(ctx context.Context, _ int64, fn func(context.Context, Store) error) error {
		return fn(ctx, admin)
	}

	err := env.s.RevocarRol(context.Background(), idAna, RevocarRolRequest{RolID: 1}, idAna)
	if !errors.Is(err, ErrUltimoAdministrador) {
		t.Fatalf("error = %v; quería ErrUltimoAdministrador", err)
	}
	if !admin.usuariosRoles[idAna][1] {
		t.Fatal("se revocó el último rol de administración")
	}
}

func containsAny(value string, parts ...string) bool {
	for _, part := range parts {
		if part != "" && strings.Contains(value, part) {
			return true
		}
	}
	return false
}
