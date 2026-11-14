package security

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/google/uuid"
)

var (
	ErrTokenInvalido = errors.New("security: token inválido, corrupto o de tipo incorrecto")
	ErrTokenExpirado = errors.New("security: token expirado")
)

// TamanoClave es el largo en bytes de la clave simétrica PASETO v4.local.
const TamanoClave = 32

// Tolerancia de reloj para nbf/iat entre instancias.
const toleranciaReloj = 5 * time.Second

// TipoToken distingue access de refresh para que VerificarToken rechace un
// refresh token presentado donde se espera un access token (y viceversa).
type TipoToken string

const (
	TipoTokenAcceso   TipoToken = "access"
	TipoTokenRefresco TipoToken = "refresh"
)

// Payload es el contenido verificado de un token PASETO v4 local.
//
// F15 (S7/S17): el token lleva iss, aud, iat, nbf, exp, jti (registrados) y los
// claims propios uid, usr, perm, rcp, pv, tipo. El footer lleva el kid de la
// clave con que se cifró. "pv" es la versión de permisos del usuario al emitir:
// el middleware la compara contra la BD (con caché corta) para que una
// revocación de rol, la desactivación o un cambio de contraseña surtan efecto
// en <= AUTH_CACHE_TTL y no al expirar el access token.
//
// El refresh token NO lleva Permisos ni pv: sus permisos se re-resuelven en BD.
type Payload struct {
	ID            uuid.UUID
	Tipo          TipoToken
	UsuarioID     int64
	NombreUsuario string
	Permisos      []string
	// RequiereCambioPassword (claim "rcp", F14).
	RequiereCambioPassword bool
	// PermisosVersion (claim "pv", F15).
	PermisosVersion uint32
	// KID de la clave con que se cifró (footer).
	KID       string
	EmitidoEn time.Time
	NoAntesDe time.Time
	ExpiraEn  time.Time
}

// TokenMaker abstrae la creación/verificación de tokens para que
// internal/auth no dependa directamente de la librería PASETO.
type TokenMaker interface {
	CrearAccessToken(usuarioID int64, nombreUsuario string, permisos []string, requiereCambioPassword bool, permisosVersion uint32, duracion time.Duration) (token string, payload *Payload, err error)
	CrearRefreshToken(usuarioID int64, duracion time.Duration) (token string, payload *Payload, err error)
	VerificarToken(token string, tipoEsperado TipoToken) (*Payload, error)
}

// ConfigPaseto agrupa lo que necesita el maker. Claves: kid -> 32 bytes.
type ConfigPaseto struct {
	Claves    map[string][]byte
	KIDActivo string
	Issuer    string
	Audience  string
}

// PasetoMaker implementa TokenMaker con PASETO v4 local (XChaCha20 +
// BLAKE2b-MAC) usando aidanwoods.dev/go-paseto. Firma solo con la clave
// activa y valida con cualquiera del llavero (rotación de claves).
type PasetoMaker struct {
	claves   map[string]paseto.V4SymmetricKey
	activa   string
	issuer   string
	audience string
	ahora    func() time.Time // inyectable en pruebas
}

type footerToken struct {
	KID string `json:"kid"`
}

func NuevoPasetoMaker(cfg ConfigPaseto) (*PasetoMaker, error) {
	if cfg.Issuer == "" || cfg.Audience == "" {
		return nil, errors.New("security: issuer y audience son obligatorios")
	}
	if len(cfg.Claves) == 0 {
		return nil, errors.New("security: se requiere al menos una clave PASETO")
	}
	if _, ok := cfg.Claves[cfg.KIDActivo]; !ok {
		return nil, fmt.Errorf("security: el kid activo %q no está en el llavero", cfg.KIDActivo)
	}
	claves := make(map[string]paseto.V4SymmetricKey, len(cfg.Claves))
	for kid, raw := range cfg.Claves {
		if len(raw) != TamanoClave {
			return nil, fmt.Errorf("security: la clave %q debe tener exactamente %d bytes", kid, TamanoClave)
		}
		k, err := paseto.V4SymmetricKeyFromBytes(raw)
		if err != nil {
			return nil, fmt.Errorf("security: clave %q inválida: %w", kid, err)
		}
		claves[kid] = k
	}
	return &PasetoMaker{
		claves: claves, activa: cfg.KIDActivo,
		issuer: cfg.Issuer, audience: cfg.Audience,
		ahora: time.Now,
	}, nil
}

func (m *PasetoMaker) crearToken(tipo TipoToken, usuarioID int64, nombreUsuario string, permisos []string, rcp bool, pv uint32, duracion time.Duration) (string, *Payload, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", nil, err
	}
	ahora := m.ahora().UTC().Truncate(time.Second)
	exp := ahora.Add(duracion)

	t := paseto.NewToken()
	t.SetIssuer(m.issuer)
	t.SetAudience(m.audience)
	t.SetIssuedAt(ahora)
	t.SetNotBefore(ahora)
	t.SetExpiration(exp)
	t.SetJti(id.String())
	t.SetString("tipo", string(tipo))
	for k, v := range map[string]any{"uid": usuarioID, "usr": nombreUsuario, "rcp": rcp, "pv": pv} {
		if err := t.Set(k, v); err != nil {
			return "", nil, err
		}
	}
	if len(permisos) > 0 {
		if err := t.Set("perm", permisos); err != nil {
			return "", nil, err
		}
	}
	footer, err := json.Marshal(footerToken{KID: m.activa})
	if err != nil {
		return "", nil, err
	}
	t.SetFooter(footer)

	token := t.V4Encrypt(m.claves[m.activa], nil)
	return token, &Payload{
		ID: id, Tipo: tipo, UsuarioID: usuarioID, NombreUsuario: nombreUsuario,
		Permisos: permisos, RequiereCambioPassword: rcp, PermisosVersion: pv,
		KID: m.activa, EmitidoEn: ahora, NoAntesDe: ahora, ExpiraEn: exp,
	}, nil
}

func (m *PasetoMaker) CrearAccessToken(usuarioID int64, nombreUsuario string, permisos []string, rcp bool, pv uint32, duracion time.Duration) (string, *Payload, error) {
	return m.crearToken(TipoTokenAcceso, usuarioID, nombreUsuario, permisos, rcp, pv, duracion)
}

// CrearRefreshToken no recibe permisos ni pv — ver comentario en Payload.
func (m *PasetoMaker) CrearRefreshToken(usuarioID int64, duracion time.Duration) (string, *Payload, error) {
	return m.crearToken(TipoTokenRefresco, usuarioID, "", nil, false, 0, duracion)
}

// VerificarToken valida: kid conocido, cifrado/MAC, iss, aud, nbf/iat, exp y
// tipo. Los tokens sin esos claims se rechazan. La revocación efectiva
// (usuario inactivo / pv distinto) la aplica el middleware, no este método.
func (m *PasetoMaker) VerificarToken(token string, tipoEsperado TipoToken) (*Payload, error) {
	// Parser sin chequeo de expiración automático: se valida a mano para poder
	// distinguir ErrTokenExpirado. Las reglas de iss/aud sí son del parser.
	parser := paseto.NewParserWithoutExpiryCheck()
	parser.AddRule(paseto.IssuedBy(m.issuer), paseto.ForAudience(m.audience))

	footerRaw, err := parser.UnsafeParseFooter(paseto.V4Local, token)
	if err != nil {
		return nil, ErrTokenInvalido
	}
	var f footerToken
	if err := json.Unmarshal(footerRaw, &f); err != nil || f.KID == "" {
		return nil, ErrTokenInvalido
	}
	clave, ok := m.claves[f.KID]
	if !ok {
		return nil, ErrTokenInvalido
	}
	t, err := parser.ParseV4Local(clave, token, nil)
	if err != nil {
		return nil, ErrTokenInvalido
	}

	ahora := m.ahora()
	iat, e1 := t.GetIssuedAt()
	nbf, e2 := t.GetNotBefore()
	exp, e3 := t.GetExpiration()
	jti, e4 := t.GetJti()
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return nil, ErrTokenInvalido
	}
	id, err := uuid.Parse(jti)
	if err != nil {
		return nil, ErrTokenInvalido
	}
	if nbf.After(ahora.Add(toleranciaReloj)) || iat.After(ahora.Add(toleranciaReloj)) {
		return nil, ErrTokenInvalido
	}
	if !exp.After(ahora) {
		return nil, ErrTokenExpirado
	}

	tipo, err := t.GetString("tipo")
	if err != nil || TipoToken(tipo) != tipoEsperado {
		return nil, ErrTokenInvalido
	}
	p := &Payload{
		ID: id, Tipo: TipoToken(tipo), KID: f.KID,
		EmitidoEn: iat, NoAntesDe: nbf, ExpiraEn: exp,
	}
	if err := t.Get("uid", &p.UsuarioID); err != nil || p.UsuarioID <= 0 {
		return nil, ErrTokenInvalido
	}
	if err := t.Get("usr", &p.NombreUsuario); err != nil {
		return nil, ErrTokenInvalido
	}
	if err := t.Get("rcp", &p.RequiereCambioPassword); err != nil {
		return nil, ErrTokenInvalido
	}
	if err := t.Get("pv", &p.PermisosVersion); err != nil {
		return nil, ErrTokenInvalido
	}
	// "perm" es opcional (refresh y usuarios sin permisos no lo llevan).
	_ = t.Get("perm", &p.Permisos)
	return p, nil
}