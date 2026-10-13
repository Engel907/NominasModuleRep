package security

import (
	"errors"
	"time"

	"github.com/aead/chacha20poly1305"
	"github.com/google/uuid"
	"github.com/o1egl/paseto"
)

var (
	ErrTokenInvalido = errors.New("security: token inválido, corrupto o de tipo incorrecto")
	ErrTokenExpirado = errors.New("security: token expirado")
)

// TipoToken distingue access de refresh para que VerificarToken rechace un
// refresh token presentado donde se espera un access token (y viceversa).
type TipoToken string

const (
	TipoTokenAcceso   TipoToken = "access"
	TipoTokenRefresco TipoToken = "refresh"
)

// Payload es el contenido cifrado del token PASETO v2 local.
//
// Ver ADR-000 §3: el payload del access token es liviano a propósito
// (usuario_id, nombre_usuario, permisos ya resueltos vía
// usuarios → usuario_roles(es_vigente=1) → roles(activo=1) → rol_permisos →
// permisos) para evitar ese join en cada request. Trade-off aceptado: una
// revocación de rol/permiso no toma efecto hasta que expira el access token
// (15-30 min); `refresh_tokens` sí permite revocar la sesión de inmediato.
//
// El refresh token deliberadamente NO lleva `Permisos` — solo lo necesario
// para ubicar su fila en `refresh_tokens` (por hash) y validar que no esté
// revocada. Los permisos se re-resuelven contra la BD en cada refresh.
type Payload struct {
	ID            uuid.UUID `json:"id"`
	Tipo          TipoToken `json:"tipo"`
	UsuarioID     int64     `json:"usuario_id"`
	NombreUsuario string    `json:"nombre_usuario"`
	Permisos      []string  `json:"permisos,omitempty"`
	EmitidoEn     time.Time `json:"emitido_en"`
	ExpiraEn      time.Time `json:"expira_en"`
}

// Valid implementa la interfaz que paseto usa para validar expiración.
//
// F14: el Payload no lleva issuer/audience aunque config.AuthConfig los
// define (PASETO_ISSUER / PASETO_AUDIENCE); hoy no se emiten ni se validan.
func (p *Payload) Valid() error {
	if time.Now().After(p.ExpiraEn) {
		return ErrTokenExpirado
	}
	return nil
}

// TokenMaker abstrae la creación/verificación de tokens para que
// internal/auth no dependa directamente de la librería PASETO — facilita
// mockear en tests y aislar un eventual cambio de estrategia de firma.
type TokenMaker interface {
	CrearAccessToken(usuarioID int64, nombreUsuario string, permisos []string, duracion time.Duration) (token string, payload *Payload, err error)
	CrearRefreshToken(usuarioID int64, duracion time.Duration) (token string, payload *Payload, err error)
	VerificarToken(token string, tipoEsperado TipoToken) (*Payload, error)
}

// PasetoMaker implementa TokenMaker con PASETO v2 local (XChaCha20-Poly1305
// autenticado). La clave simétrica sale de config.LoadConfig() (Fase 1) y
// debe tener exactamente 32 bytes — NuevoPasetoMaker falla rápido si no.
type PasetoMaker struct {
	paseto         *paseto.V2
	claveSimetrica []byte
}

func NuevoPasetoMaker(claveSimetrica string) (*PasetoMaker, error) {
	if len(claveSimetrica) != chacha20poly1305.KeySize {
		return nil, errors.New("security: la clave PASETO debe tener exactamente 32 bytes")
	}
	return &PasetoMaker{
		paseto:         paseto.NewV2(),
		claveSimetrica: []byte(claveSimetrica),
	}, nil
}

func (m *PasetoMaker) crearToken(tipo TipoToken, usuarioID int64, nombreUsuario string, permisos []string, duracion time.Duration) (string, *Payload, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", nil, err
	}
	ahora := time.Now()
	payload := &Payload{
		ID:            id,
		Tipo:          tipo,
		UsuarioID:     usuarioID,
		NombreUsuario: nombreUsuario,
		Permisos:      permisos,
		EmitidoEn:     ahora,
		ExpiraEn:      ahora.Add(duracion),
	}
	token, err := m.paseto.Encrypt(m.claveSimetrica, payload, nil)
	if err != nil {
		return "", nil, err
	}
	return token, payload, nil
}

func (m *PasetoMaker) CrearAccessToken(usuarioID int64, nombreUsuario string, permisos []string, duracion time.Duration) (string, *Payload, error) {
	return m.crearToken(TipoTokenAcceso, usuarioID, nombreUsuario, permisos, duracion)
}

// CrearRefreshToken no recibe `permisos` — ver comentario en Payload.
func (m *PasetoMaker) CrearRefreshToken(usuarioID int64, duracion time.Duration) (string, *Payload, error) {
	return m.crearToken(TipoTokenRefresco, usuarioID, "", nil, duracion)
}

// F15: VerificarToken solo valida firma, expiración y tipo. Un access token
// sigue siendo válido hasta expirar aunque el usuario se desactive o se
// revoque su sesión (ver trade-off documentado en Payload).
func (m *PasetoMaker) VerificarToken(token string, tipoEsperado TipoToken) (*Payload, error) {
	payload := &Payload{}
	if err := m.paseto.Decrypt(token, m.claveSimetrica, payload, nil); err != nil {
		return nil, ErrTokenInvalido
	}
	if err := payload.Valid(); err != nil {
		return nil, err
	}
	if payload.Tipo != tipoEsperado {
		return nil, ErrTokenInvalido
	}
	return payload, nil
}
