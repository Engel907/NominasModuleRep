package auth

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/security"
)

// ----------------------------------------------------------------------------
// Fakes
// ----------------------------------------------------------------------------

// fakeStore implementa Store en memoria. Cada método toma el mutex entero, lo
// que le da las mismas garantías de atomicidad que un UPDATE condicional en BD
// (es lo que RevocarRefreshTokenSiVigente / ConsumirTokenResetPassword prometen).
type fakeStore struct {
	mu            sync.Mutex
	usuarios      map[int64]*dto.Usuario
	roles         map[int64]*dto.Rol
	usuariosRoles map[int64]map[int64]bool
	permisosRol   map[int64]map[string]bool
	refresh       []*dto.RefreshToken
	resets        []*dto.TokenResetPassword
	sigID         int64
	fallos        int // llamadas a RegistrarIntentoFallidoAtomico
}

func nuevoFakeStore() *fakeStore {
	return &fakeStore{
		usuarios:      map[int64]*dto.Usuario{},
		roles:         map[int64]*dto.Rol{},
		usuariosRoles: map[int64]map[int64]bool{},
		permisosRol:   map[int64]map[string]bool{},
		sigID:         100,
	}
}

func (f *fakeStore) id() int64 { f.sigID++; return f.sigID }

func (f *fakeStore) GetUsuarioPorNombreUsuario(_ context.Context, nombre string) (dto.Usuario, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.usuarios {
		if u.NombreUsuario == nombre {
			return *u, nil
		}
	}
	return dto.Usuario{}, sql.ErrNoRows
}

func (f *fakeStore) GetUsuarioPorID(_ context.Context, id int64) (dto.Usuario, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.usuarios[id]; ok {
		return *u, nil
	}
	return dto.Usuario{}, sql.ErrNoRows
}

func (f *fakeStore) RegistrarIntentoFallidoAtomico(_ context.Context, id int64, max int, hasta time.Time) (dto.EstadoIntentos, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fallos++
	u := f.usuarios[id]
	if int(u.IntentosFallidos)+1 >= max {
		u.BloqueadoHasta = sql.NullTime{Time: hasta, Valid: true}
	}
	u.IntentosFallidos++
	return dto.EstadoIntentos{IntentosFallidos: u.IntentosFallidos, BloqueadoHasta: u.BloqueadoHasta}, nil
}

func (f *fakeStore) ResetearIntentosFallidos(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u := f.usuarios[id]
	u.IntentosFallidos, u.BloqueadoHasta = 0, sql.NullTime{}
	return nil
}

func (f *fakeStore) ActualizarPasswordHash(_ context.Context, id int64, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u := f.usuarios[id]
	u.PasswordHash, u.RequiereCambioPassword = hash, false
	return nil
}

func (f *fakeStore) RehashPasswordHash(_ context.Context, id int64, anterior, nuevo string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u := f.usuarios[id]; u.PasswordHash == anterior {
		u.PasswordHash = nuevo // NO toca RequiereCambioPassword
	}
	return nil
}

func (f *fakeStore) CrearRefreshTokenConFamilia(_ context.Context, usuarioID int64, hash string, exp time.Time, familia, _, _ string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.id()
	f.refresh = append(f.refresh, &dto.RefreshToken{
		ID: id, UsuarioID: usuarioID, TokenHash: hash, FechaExpiracion: exp,
		FamiliaID: sql.NullString{String: familia, Valid: familia != ""},
	})
	return id, nil
}

func (f *fakeStore) GetRefreshTokenPorHash(_ context.Context, hash string) (dto.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.refresh {
		if r.TokenHash == hash {
			return *r, nil
		}
	}
	return dto.RefreshToken{}, sql.ErrNoRows
}

func (f *fakeStore) RevocarRefreshTokenPorHash(_ context.Context, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.refresh {
		if r.TokenHash == hash {
			r.Revocado = true
		}
	}
	return nil
}

func (f *fakeStore) RevocarRefreshTokenSiVigente(_ context.Context, id int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.refresh {
		if r.ID == id && !r.Revocado {
			r.Revocado = true
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) RevocarFamiliaRefreshTokens(_ context.Context, familia string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, r := range f.refresh {
		if r.FamiliaID.String == familia && !r.Revocado {
			r.Revocado = true
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) RevocarTodosLosRefreshTokensDeUsuario(_ context.Context, usuarioID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.refresh {
		if r.UsuarioID == usuarioID {
			r.Revocado = true
		}
	}
	return nil
}

func (f *fakeStore) CrearTokenResetPassword(_ context.Context, usuarioID int64, hash string, exp time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.id()
	f.resets = append(f.resets, &dto.TokenResetPassword{ID: id, UsuarioID: usuarioID, TokenHash: hash, FechaExpiracion: exp})
	return id, nil
}

func (f *fakeStore) GetTokenResetPasswordPorHash(_ context.Context, hash string) (dto.TokenResetPassword, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.resets {
		if r.TokenHash == hash {
			return *r, nil
		}
	}
	return dto.TokenResetPassword{}, sql.ErrNoRows
}

func (f *fakeStore) ConsumirTokenResetPassword(_ context.Context, id int64, ahora time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.resets {
		if r.ID == id && !r.Usado && r.FechaExpiracion.After(ahora) {
			r.Usado = true
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) InvalidarTokensResetPasswordPendientesDeUsuario(_ context.Context, usuarioID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.resets {
		if r.UsuarioID == usuarioID {
			r.Usado = true
		}
	}
	return nil
}

func (f *fakeStore) GetRolPorID(_ context.Context, id int64) (dto.Rol, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rol, ok := f.roles[id]; ok {
		return *rol, nil
	}
	return dto.Rol{}, sql.ErrNoRows
}

func (f *fakeStore) GetPermisosVigentesDeUsuario(context.Context, int64) ([]string, error) {
	return []string{"empleados.leer"}, nil
}

func (f *fakeStore) IncrementarPermisosVersion(_ context.Context, usuarioID int64) (uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.usuarios[usuarioID]
	if !ok {
		return 0, sql.ErrNoRows
	}
	u.PermisosVersion++
	return u.PermisosVersion, nil
}

func (f *fakeStore) IncrementarPermisosVersionPorRol(_ context.Context, rolID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for usuarioID, roles := range f.usuariosRoles {
		if roles[rolID] {
			if u, ok := f.usuarios[usuarioID]; ok {
				u.PermisosVersion++
			}
		}
	}
	return nil
}

func (f *fakeStore) SetUsuarioActivo(_ context.Context, usuarioID int64, activo bool) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.usuarios[usuarioID]
	if !ok {
		return false, sql.ErrNoRows
	}
	if u.Activo == activo {
		return false, nil
	}
	u.Activo = activo
	return true, nil
}

func (f *fakeStore) SetRolActivo(_ context.Context, rolID int64, activo bool) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rol, ok := f.roles[rolID]
	if !ok {
		return false, sql.ErrNoRows
	}
	if rol.Activo == activo {
		return false, nil
	}
	rol.Activo = activo
	return true, nil
}

func (f *fakeStore) ReemplazarPermisosRol(_ context.Context, rolID int64, codigos []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.roles[rolID]; !ok {
		return sql.ErrNoRows
	}
	permisos := make(map[string]bool, len(codigos))
	for _, codigo := range codigos {
		permisos[codigo] = true
	}
	f.permisosRol[rolID] = permisos
	return nil
}

// Acceso directo (con lock) para aserciones.
func (f *fakeStore) usuario(id int64) dto.Usuario {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.usuarios[id]
}

func (f *fakeStore) todasRevocadas(usuarioID int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.refresh {
		if r.UsuarioID == usuarioID && !r.Revocado {
			return false
		}
	}
	return true
}

type fakeNotificador struct {
	mu     sync.Mutex
	envios []envioReset
}

type envioReset struct{ correo, nombre, token string }

func (n *fakeNotificador) EnviarResetPassword(_ context.Context, correo, nombre, token string, _ time.Time) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.envios = append(n.envios, envioReset{correo, nombre, token})
	return nil
}

func (n *fakeNotificador) copia() []envioReset {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]envioReset(nil), n.envios...)
}

// ----------------------------------------------------------------------------
// Entorno de prueba
// ----------------------------------------------------------------------------

const (
	claveValida = "Correcta-clave-2026!"
	claveNueva  = "caballo-azul-nube-47"

	idAna       = 1
	idInactivo  = 2
	idBloqueado = 3
	idTemporal  = 4
)

var (
	hashClaveOnce sync.Once
	hashClave     string
)

// hashDeClave calcula UNA vez el argon2id de claveValida: cada hash cuesta
// ~64 MiB y cientos de ms, y todos los usuarios de prueba comparten clave.
func hashDeClave(t *testing.T) string {
	t.Helper()
	hashClaveOnce.Do(func() {
		h, err := security.HashPassword(claveValida)
		if err != nil {
			panic(err)
		}
		hashClave = h
	})
	return hashClave
}

type entorno struct {
	s         *Servicio
	fs        *fakeStore
	notif     *fakeNotificador
	maker     *security.PasetoMaker
	verifCall atomic.Int64 // veces que Login/Cambiar verificaron una contraseña
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	h := hashDeClave(t)

	fs := nuevoFakeStore()
	fs.usuarios[idAna] = &dto.Usuario{ID: idAna, NombreUsuario: "ana.lopez", Correo: "ana@ccdr.go.cr", PasswordHash: h, Activo: true, PermisosVersion: 1}
	fs.usuarios[idInactivo] = &dto.Usuario{ID: idInactivo, NombreUsuario: "inactivo", Correo: "inactivo@ccdr.go.cr", PasswordHash: h, Activo: false, PermisosVersion: 1}
	fs.usuarios[idBloqueado] = &dto.Usuario{ID: idBloqueado, NombreUsuario: "bloqueado", Correo: "bloqueado@ccdr.go.cr", PasswordHash: h, Activo: true, PermisosVersion: 1,
		BloqueadoHasta: sql.NullTime{Time: time.Now().Add(10 * time.Minute), Valid: true}}
	fs.usuarios[idTemporal] = &dto.Usuario{ID: idTemporal, NombreUsuario: "temporal", Correo: "temporal@ccdr.go.cr", PasswordHash: h, Activo: true, PermisosVersion: 1,
		RequiereCambioPassword: true}

	maker, err := security.NuevoPasetoMaker(security.ConfigPaseto{
		Claves:    map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")},
		KIDActivo: "k1",
		Issuer:    "planillas-test",
		Audience:  "planillas-test-api",
	})
	if err != nil {
		t.Fatal(err)
	}
	e := &entorno{fs: fs, notif: &fakeNotificador{}, maker: maker}

	s := NuevoServicio(fs, nil, maker, Politica{
		MaxIntentosFallidos:  3,
		MinutosBloqueo:       15 * time.Minute,
		DuracionAccessToken:  20 * time.Minute,
		DuracionRefreshToken: 168 * time.Hour,
		DuracionTokenReset:   30 * time.Minute,
	}).ConNotificador(e.notif).ConLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.txRunner = func(ctx context.Context, _ int64, fn func(context.Context, Store) error) error {
		return fn(ctx, fs)
	}
	orig := s.verificar
	s.verificar = func(p, h string) (bool, error) {
		e.verifCall.Add(1)
		return orig(p, h)
	}
	e.s = s
	return e
}

func (e *entorno) login(t *testing.T, usuario, pass string) *LoginResponse {
	t.Helper()
	resp, err := e.s.Login(context.Background(), LoginRequest{NombreUsuario: usuario, Password: pass})
	if err != nil {
		t.Fatalf("login de %q: %v", usuario, err)
	}
	return resp
}

func (e *entorno) refresh(tok string) (*RefreshResponse, error) {
	return e.s.Refresh(context.Background(), RefreshRequest{RefreshToken: tok})
}

// ----------------------------------------------------------------------------
// LOGIN (S5)
// ----------------------------------------------------------------------------

func TestLogin_RespuestasUniformes(t *testing.T) {
	casos := []struct {
		nombre, usuario, pass string
		cuentaFallo           bool // ¿debe sumar al contador de intentos?
	}{
		{"usuario inexistente", "no.existe", claveValida, false},
		{"contraseña incorrecta", "ana.lopez", "otra-clave-cualquiera", true},
		{"usuario inactivo (clave correcta)", "inactivo", claveValida, false},
		{"usuario inactivo (clave mala)", "inactivo", "otra-clave-cualquiera", false},
		{"usuario bloqueado (clave correcta)", "bloqueado", claveValida, false},
		{"usuario bloqueado (clave mala)", "bloqueado", "otra-clave-cualquiera", false},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			e := nuevoEntorno(t)
			resp, err := e.s.Login(context.Background(), LoginRequest{NombreUsuario: tc.usuario, Password: tc.pass})

			// Mismo valor de error en todos los casos => el handler produce
			// idéntico código y cuerpo HTTP.
			if err != ErrCredencialesInvalidas { //nolint:errorlint // se exige identidad, no solo errors.Is
				t.Fatalf("err = %v, se esperaba exactamente ErrCredencialesInvalidas", err)
			}
			if resp != nil {
				t.Fatal("no debe devolverse sesión")
			}
			// Todos los caminos pagan UNA verificación argon2id (la del usuario
			// o la del hash señuelo): el tiempo no distingue los casos.
			if n := e.verifCall.Load(); n != 1 {
				t.Fatalf("verificaciones de contraseña = %d, se esperaba 1", n)
			}
			if got := e.fs.fallos == 1; got != tc.cuentaFallo {
				t.Fatalf("registró fallo = %v, se esperaba %v", got, tc.cuentaFallo)
			}
		})
	}
}

func TestLogin_Exito_ClaimRcp(t *testing.T) {
	e := nuevoEntorno(t)

	normal := e.login(t, "ana.lopez", claveValida)
	if normal.RequiereCambioPassword {
		t.Fatal("ana no requiere cambio de contraseña")
	}
	p, err := e.maker.VerificarToken(normal.AccessToken, security.TipoTokenAcceso)
	if err != nil || p.RequiereCambioPassword {
		t.Fatalf("access token de ana: rcp debe ser false (err=%v)", err)
	}

	temporal := e.login(t, "temporal", claveValida)
	if !temporal.RequiereCambioPassword {
		t.Fatal("la respuesta debe indicar requiere_cambio_password")
	}
	p, err = e.maker.VerificarToken(temporal.AccessToken, security.TipoTokenAcceso)
	if err != nil || !p.RequiereCambioPassword {
		t.Fatalf("access token de cuenta temporal: rcp debe ser true (err=%v)", err)
	}
	r, _ := e.maker.VerificarToken(temporal.RefreshToken, security.TipoTokenRefresco)
	if r.RequiereCambioPassword {
		t.Fatal("el refresh token no lleva rcp")
	}
}

func TestLogin_BloqueoTrasUmbral(t *testing.T) {
	e := nuevoEntorno(t)
	for i := 0; i < 3; i++ { // MaxIntentosFallidos = 3
		if _, err := e.s.Login(context.Background(), LoginRequest{NombreUsuario: "ana.lopez", Password: "mala"}); err != ErrCredencialesInvalidas { //nolint:errorlint
			t.Fatalf("intento %d: %v", i+1, err)
		}
	}
	if !e.fs.usuario(idAna).BloqueadoHasta.Valid {
		t.Fatal("la cuenta debe quedar bloqueada al llegar al umbral")
	}
	// Con la clave CORRECTA sigue rechazada (y con el mismo error).
	if _, err := e.s.Login(context.Background(), LoginRequest{NombreUsuario: "ana.lopez", Password: claveValida}); err != ErrCredencialesInvalidas { //nolint:errorlint
		t.Fatalf("cuenta bloqueada: err = %v", err)
	}
	if e.fs.fallos != 3 {
		t.Fatalf("un intento sobre cuenta bloqueada no debe sumar: fallos = %d", e.fs.fallos)
	}
}

// Regresión: el re-hash perezoso bcrypt→argon2id borraba requiere_cambio_password.
func TestLogin_RehashBcryptNoLimpiaElCambioObligatorio(t *testing.T) {
	e := nuevoEntorno(t)
	bc, err := bcrypt.GenerateFromPassword([]byte(claveValida), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	e.fs.usuarios[idTemporal].PasswordHash = string(bc)

	resp := e.login(t, "temporal", claveValida)
	if !resp.RequiereCambioPassword {
		t.Fatal("el login debe seguir exigiendo el cambio de contraseña")
	}
	u := e.fs.usuario(idTemporal)
	if !u.RequiereCambioPassword {
		t.Fatal("el re-hash NO debe limpiar requiere_cambio_password")
	}
	if u.PasswordHash == string(bc) {
		t.Fatal("el hash bcrypt debió migrarse a argon2id")
	}
}

// ----------------------------------------------------------------------------
// REFRESH (S6)
// ----------------------------------------------------------------------------

func TestRefresh_RotaYDetectaReutilizacion(t *testing.T) {
	e := nuevoEntorno(t)
	p1 := e.login(t, "ana.lopez", claveValida)

	p2, err := e.refresh(p1.RefreshToken)
	if err != nil {
		t.Fatalf("la primera rotación debe funcionar: %v", err)
	}
	if p2.RefreshToken == p1.RefreshToken {
		t.Fatal("la rotación debe emitir un refresh token distinto")
	}

	// Reutilizar el token viejo => 401 genérico...
	if _, err := e.refresh(p1.RefreshToken); !errors.Is(err, ErrRefreshTokenInvalido) {
		t.Fatalf("reutilización: err = %v", err)
	}
	// ...y la familia entera queda revocada: el token NUEVO (p2) ya no sirve.
	if _, err := e.refresh(p2.RefreshToken); !errors.Is(err, ErrRefreshTokenInvalido) {
		t.Fatalf("tras la reutilización la familia debe estar revocada: err = %v", err)
	}
	if !e.fs.todasRevocadas(idAna) {
		t.Fatal("ninguna sesión de la familia debe quedar viva")
	}
}

func TestRefresh_ReutilizacionNoAfectaOtraFamilia(t *testing.T) {
	e := nuevoEntorno(t)
	a := e.login(t, "ana.lopez", claveValida) // sesión 1 (familia A)
	b := e.login(t, "ana.lopez", claveValida) // sesión 2, otro dispositivo (familia B)

	if _, err := e.refresh(a.RefreshToken); err != nil {
		t.Fatal(err)
	}
	_, _ = e.refresh(a.RefreshToken) // reutilización: solo cae la familia A

	if _, err := e.refresh(b.RefreshToken); err != nil {
		t.Fatalf("la familia B debe seguir viva: %v", err)
	}
}

func TestRefresh_Concurrente_SoloUnoGana(t *testing.T) {
	e := nuevoEntorno(t)
	sesion := e.login(t, "ana.lopez", claveValida)

	const n = 8
	var ok, invalidos atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := e.refresh(sesion.RefreshToken)
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, ErrRefreshTokenInvalido):
				invalidos.Add(1)
			default:
				t.Errorf("error inesperado: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if ok.Load() != 1 || invalidos.Load() != n-1 {
		t.Fatalf("ok=%d invalidos=%d; con el mismo token debe ganar exactamente 1 de %d", ok.Load(), invalidos.Load(), n)
	}
}

func TestRefresh_FilaDeOtroUsuarioSeRechaza(t *testing.T) {
	e := nuevoEntorno(t)
	sesion := e.login(t, "ana.lopez", claveValida)
	e.fs.refresh[0].UsuarioID = idTemporal // fila y payload dejan de coincidir

	if _, err := e.refresh(sesion.RefreshToken); !errors.Is(err, ErrRefreshTokenInvalido) {
		t.Fatalf("err = %v", err)
	}
}

func TestRefresh_UsuarioInactivoSeRechaza(t *testing.T) {
	e := nuevoEntorno(t)
	sesion := e.login(t, "ana.lopez", claveValida)
	e.fs.usuarios[idAna].Activo = false

	if _, err := e.refresh(sesion.RefreshToken); !errors.Is(err, ErrRefreshTokenInvalido) {
		t.Fatalf("err = %v", err)
	}
}

func TestRefresh_MantieneElClaimRcp(t *testing.T) {
	e := nuevoEntorno(t)
	sesion := e.login(t, "temporal", claveValida)

	r, err := e.refresh(sesion.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := e.maker.VerificarToken(r.AccessToken, security.TipoTokenAcceso)
	if !p.RequiereCambioPassword {
		t.Fatal("refrescar no debe sacar a la cuenta del cambio obligatorio")
	}
}

// ----------------------------------------------------------------------------
// CAMBIO DE CONTRASEÑA (S1)
// ----------------------------------------------------------------------------

func TestCambiarPassword(t *testing.T) {
	e := nuevoEntorno(t)
	vieja := e.login(t, "temporal", claveValida)
	ctx := context.Background()

	// Contraseña actual incorrecta: error específico y cuenta como fallo.
	if _, err := e.s.CambiarPassword(ctx, idTemporal, CambiarPasswordRequest{PasswordActual: "equivocada", NuevaPassword: claveNueva}); !errors.Is(err, ErrPasswordActualIncorrecta) {
		t.Fatalf("actual incorrecta: err = %v", err)
	}
	if e.fs.usuario(idTemporal).IntentosFallidos != 1 {
		t.Fatal("una contraseña actual incorrecta debe sumar al contador de intentos")
	}

	// Nueva contraseña que incumple la política.
	var pe *ErrPoliticaPassword
	if _, err := e.s.CambiarPassword(ctx, idTemporal, CambiarPasswordRequest{PasswordActual: claveValida, NuevaPassword: "password1234"}); !errors.As(err, &pe) || pe.Motivo != MotivoComun {
		t.Fatalf("política: err = %v", err)
	}
	// Igual a la actual.
	if _, err := e.s.CambiarPassword(ctx, idTemporal, CambiarPasswordRequest{PasswordActual: claveValida, NuevaPassword: claveValida}); !errors.As(err, &pe) || pe.Motivo != MotivoIgualActual {
		t.Fatalf("igual a la actual: err = %v", err)
	}

	nueva, err := e.s.CambiarPassword(ctx, idTemporal, CambiarPasswordRequest{PasswordActual: claveValida, NuevaPassword: claveNueva})
	if err != nil {
		t.Fatalf("cambio válido: %v", err)
	}
	if nueva.RequiereCambioPassword {
		t.Fatal("tras el cambio, rcp debe ser false")
	}
	p, _ := e.maker.VerificarToken(nueva.AccessToken, security.TipoTokenAcceso)
	if p.RequiereCambioPassword {
		t.Fatal("el access token nuevo no debe llevar rcp")
	}

	u := e.fs.usuario(idTemporal)
	if u.RequiereCambioPassword || u.IntentosFallidos != 0 {
		t.Fatalf("flag/contador sin limpiar: %+v", u)
	}
	// Sesión anterior revocada; la nueva sí funciona.
	if _, err := e.refresh(vieja.RefreshToken); !errors.Is(err, ErrRefreshTokenInvalido) {
		t.Fatalf("la sesión anterior debe estar revocada: %v", err)
	}
	// (esa reutilización revocó la familia vieja, no la nueva)
	if _, err := e.refresh(nueva.RefreshToken); err != nil {
		t.Fatalf("la sesión nueva debe funcionar: %v", err)
	}
	// La clave vieja ya no entra; la nueva sí.
	if _, err := e.s.Login(ctx, LoginRequest{NombreUsuario: "temporal", Password: claveValida}); err != ErrCredencialesInvalidas { //nolint:errorlint
		t.Fatalf("clave vieja: err = %v", err)
	}
	e.login(t, "temporal", claveNueva)
}

// ----------------------------------------------------------------------------
// RESET POR CORREO (S2)
// ----------------------------------------------------------------------------

func TestResetPassword_FlujoCompleto(t *testing.T) {
	e := nuevoEntorno(t)
	ctx := context.Background()
	sesion := e.login(t, "ana.lopez", claveValida)
	e.fs.usuarios[idAna].IntentosFallidos = 2
	e.fs.usuarios[idAna].BloqueadoHasta = sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true}

	if err := e.s.SolicitarResetPassword(ctx, SolicitarResetPasswordRequest{NombreUsuario: "ana.lopez"}); err != nil {
		t.Fatal(err)
	}
	e.s.EsperarEnvios()
	envios := e.notif.copia()
	if len(envios) != 1 || envios[0].correo != "ana@ccdr.go.cr" || len(envios[0].token) != 64 {
		t.Fatalf("envíos = %+v", envios)
	}
	token := envios[0].token

	// En BD solo existe el hash, nunca el token en claro.
	for _, r := range e.fs.resets {
		if r.TokenHash == token || r.TokenHash != hashToken(token) {
			t.Fatal("solo debe persistirse hashToken(token)")
		}
	}

	// Política incumplida: no quema el token.
	var pe *ErrPoliticaPassword
	if err := e.s.ConfirmarResetPassword(ctx, ConfirmarResetPasswordRequest{Token: token, NuevaPassword: "corta"}); !errors.As(err, &pe) {
		t.Fatalf("política: err = %v", err)
	}

	if err := e.s.ConfirmarResetPassword(ctx, ConfirmarResetPasswordRequest{Token: token, NuevaPassword: claveNueva}); err != nil {
		t.Fatalf("confirmar: %v", err)
	}
	// Un solo uso.
	if err := e.s.ConfirmarResetPassword(ctx, ConfirmarResetPasswordRequest{Token: token, NuevaPassword: claveNueva + "x"}); !errors.Is(err, ErrTokenResetInvalido) {
		t.Fatalf("segundo uso: err = %v", err)
	}
	// Revoca sesiones, limpia el bloqueo y la clave nueva funciona.
	if _, err := e.refresh(sesion.RefreshToken); !errors.Is(err, ErrRefreshTokenInvalido) {
		t.Fatalf("las sesiones deben quedar revocadas: %v", err)
	}
	if u := e.fs.usuario(idAna); u.BloqueadoHasta.Valid || u.IntentosFallidos != 0 {
		t.Fatalf("el reset debe limpiar el bloqueo: %+v", u)
	}
	e.login(t, "ana.lopez", claveNueva)
}

func TestResetPassword_ConfirmacionesConcurrentes_GanaUna(t *testing.T) {
	e := nuevoEntorno(t)
	ctx := context.Background()
	_ = e.s.SolicitarResetPassword(ctx, SolicitarResetPasswordRequest{NombreUsuario: "ana.lopez"})
	e.s.EsperarEnvios()
	token := e.notif.copia()[0].token

	const n = 3 // cada confirmación calcula un argon2id de 64 MiB antes de la Tx
	var ok atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if e.s.ConfirmarResetPassword(ctx, ConfirmarResetPasswordRequest{Token: token, NuevaPassword: claveNueva}) == nil {
				ok.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if ok.Load() != 1 {
		t.Fatalf("confirmaciones exitosas = %d, se esperaba 1", ok.Load())
	}
}

func TestResetPassword_SolicitudNoRevelaExistencia(t *testing.T) {
	e := nuevoEntorno(t)
	for _, usuario := range []string{"no.existe", "inactivo"} {
		if err := e.s.SolicitarResetPassword(context.Background(), SolicitarResetPasswordRequest{NombreUsuario: usuario}); err != nil {
			t.Fatalf("%s: debe devolver nil igual que un usuario real, err = %v", usuario, err)
		}
	}
	e.s.EsperarEnvios()
	if n := len(e.notif.copia()); n != 0 {
		t.Fatalf("no debe enviarse nada, envíos = %d", n)
	}
	if len(e.fs.resets) != 0 {
		t.Fatal("no debe persistirse ningún token")
	}
}

func TestResetPassword_TokenExpiradoOInexistente(t *testing.T) {
	e := nuevoEntorno(t)
	ctx := context.Background()
	_ = e.s.SolicitarResetPassword(ctx, SolicitarResetPasswordRequest{NombreUsuario: "ana.lopez"})
	e.s.EsperarEnvios()
	token := e.notif.copia()[0].token
	e.fs.resets[0].FechaExpiracion = time.Now().Add(-time.Minute)

	if err := e.s.ConfirmarResetPassword(ctx, ConfirmarResetPasswordRequest{Token: token, NuevaPassword: claveNueva}); !errors.Is(err, ErrTokenResetInvalido) {
		t.Fatalf("expirado: err = %v", err)
	}
	if err := e.s.ConfirmarResetPassword(ctx, ConfirmarResetPasswordRequest{Token: "inventado", NuevaPassword: claveNueva}); !errors.Is(err, ErrTokenResetInvalido) {
		t.Fatalf("inexistente: err = %v", err)
	}
}

func TestLogNotificador_SoloEnDesarrollo(t *testing.T) {
	for _, env := range []string{"production", "staging", ""} {
		if _, err := NuevoLogNotificador(env, "", nil); !errors.Is(err, ErrLogNotificadorSoloDesarrollo) {
			t.Fatalf("APP_ENV=%q debe rechazarse, err = %v", env, err)
		}
	}
	if _, err := NuevoLogNotificador("development", "", nil); err != nil {
		t.Fatalf("development debe permitirse: %v", err)
	}
}

func TestCambiarEstadoUsuario_InvalidaSesionesEnLaMismaTransaccion(t *testing.T) {
	e := nuevoEntorno(t)
	ctx := context.Background()
	sesion := e.login(t, "ana.lopez", claveValida)

	if err := e.s.CambiarEstadoUsuario(ctx, idAna, false, 99); err != nil {
		t.Fatal(err)
	}
	if u := e.fs.usuario(idAna); u.Activo || u.PermisosVersion != 2 {
		t.Fatalf("al desactivar debe cambiar estado y subir pv: %+v", u)
	}
	if !e.fs.todasRevocadas(idAna) {
		t.Fatal("desactivar el usuario debe revocar sus refresh tokens")
	}
	if _, err := e.refresh(sesion.RefreshToken); !errors.Is(err, ErrRefreshTokenInvalido) {
		t.Fatalf("el refresh token anterior debe quedar revocado: %v", err)
	}

	if err := e.s.CambiarEstadoUsuario(ctx, idAna, false, 99); err != nil {
		t.Fatal(err)
	}
	if u := e.fs.usuario(idAna); u.PermisosVersion != 2 {
		t.Fatalf("repetir el mismo estado no debe volver a subir pv: %+v", u)
	}
}

func TestCambiarEstadoRol_InvalidaSesionesDeUsuariosAsignados(t *testing.T) {
	e := nuevoEntorno(t)
	e.fs.roles[7] = &dto.Rol{ID: 7, Codigo: "RRHH", Activo: true}
	e.fs.usuariosRoles[idAna] = map[int64]bool{7: true}

	if err := e.s.CambiarEstadoRol(context.Background(), 7, false, 99); err != nil {
		t.Fatal(err)
	}
	if u := e.fs.usuario(idAna); u.PermisosVersion != 2 {
		t.Fatalf("desactivar el rol debe subir pv del usuario asignado: %+v", u)
	}
}

func TestReemplazarPermisosRol_ValidaCatalogoYInvalidaSesiones(t *testing.T) {
	e := nuevoEntorno(t)
	e.fs.roles[7] = &dto.Rol{ID: 7, Codigo: "RRHH", Activo: true}
	e.fs.usuariosRoles[idAna] = map[int64]bool{7: true}

	err := e.s.ReemplazarPermisosRol(context.Background(), 7, []string{authz.PermEmpleadosLeer}, 99)
	if err != nil {
		t.Fatal(err)
	}
	if !e.fs.permisosRol[7][authz.PermEmpleadosLeer] {
		t.Fatalf("no se guardó el permiso: %+v", e.fs.permisosRol[7])
	}
	if u := e.fs.usuario(idAna); u.PermisosVersion != 2 {
		t.Fatalf("cambiar permisos del rol debe subir pv del usuario asignado: %+v", u)
	}

	if err := e.s.ReemplazarPermisosRol(context.Background(), 7, []string{"permiso.inexistente"}, 99); !errors.Is(err, ErrPermisoInvalido) {
		t.Fatalf("permiso fuera de catálogo: err = %v", err)
	}
	if err := e.s.ReemplazarPermisosRol(context.Background(), 7, []string{authz.PermEmpleadosLeer, authz.PermEmpleadosLeer}, 99); !errors.Is(err, ErrPermisoDuplicado) {
		t.Fatalf("permiso duplicado: err = %v", err)
	}
	if u := e.fs.usuario(idAna); u.PermisosVersion != 2 {
		t.Fatalf("entrada inválida no debe subir pv: %+v", u)
	}
}
