package security

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/stretchr/testify/require"
)

func clave(b byte) []byte { return bytes.Repeat([]byte{b}, TamanoClave) }

func nuevoMaker(t *testing.T, claves map[string][]byte, activa, iss, aud string) *PasetoMaker {
	t.Helper()
	m, err := NuevoPasetoMaker(ConfigPaseto{Claves: claves, KIDActivo: activa, Issuer: iss, Audience: aud})
	require.NoError(t, err)
	return m
}

func base(t *testing.T) *PasetoMaker {
	return nuevoMaker(t, map[string][]byte{"k1": clave(1)}, "k1", "iss", "aud")
}

func TestAccessTokenIdaYVuelta(t *testing.T) {
	m := base(t)
	tok, _, err := m.CrearAccessToken(7, "ana", []string{"a.b", "c.d"}, true, 3, time.Minute)
	require.NoError(t, err)
	p, err := m.VerificarToken(tok, TipoTokenAcceso)
	require.NoError(t, err)
	require.EqualValues(t, 7, p.UsuarioID)
	require.Equal(t, "ana", p.NombreUsuario)
	require.Equal(t, []string{"a.b", "c.d"}, p.Permisos)
	require.True(t, p.RequiereCambioPassword)
	require.EqualValues(t, 3, p.PermisosVersion)
	require.Equal(t, "k1", p.KID)
}

func TestTipoIncorrecto(t *testing.T) {
	m := base(t)
	tok, _, _ := m.CrearRefreshToken(7, time.Hour)
	_, err := m.VerificarToken(tok, TipoTokenAcceso)
	require.ErrorIs(t, err, ErrTokenInvalido)
}

func TestExpirado(t *testing.T) {
	m := base(t)
	tok, _, err := m.CrearAccessToken(1, "x", nil, false, 1, time.Minute)
	require.NoError(t, err)
	m.ahora = func() time.Time { return time.Now().Add(2 * time.Minute) }
	_, err = m.VerificarToken(tok, TipoTokenAcceso)
	require.ErrorIs(t, err, ErrTokenExpirado)
}

func TestNbfFuturo(t *testing.T) {
	m := base(t)
	m.ahora = func() time.Time { return time.Now().Add(time.Hour) }
	tok, _, err := m.CrearAccessToken(1, "x", nil, false, 1, 2*time.Hour)
	require.NoError(t, err)
	m.ahora = time.Now // el verificador "vive" una hora antes del nbf
	_, err = m.VerificarToken(tok, TipoTokenAcceso)
	require.ErrorIs(t, err, ErrTokenInvalido)
}

func TestIssuerYAudienceDistintos(t *testing.T) {
	cl := map[string][]byte{"k1": clave(1)}
	emisor := nuevoMaker(t, cl, "k1", "iss", "aud")
	tok, _, _ := emisor.CrearAccessToken(1, "x", nil, false, 1, time.Minute)

	_, err := nuevoMaker(t, cl, "k1", "OTRO", "aud").VerificarToken(tok, TipoTokenAcceso)
	require.ErrorIs(t, err, ErrTokenInvalido)
	_, err = nuevoMaker(t, cl, "k1", "iss", "OTRA").VerificarToken(tok, TipoTokenAcceso)
	require.ErrorIs(t, err, ErrTokenInvalido)
}

// Token bien cifrado y con kid válido pero SIN aud (ni iss): ya no se acepta.
func TestTokenSinAudNiIss(t *testing.T) {
	m := base(t)
	tk := paseto.NewToken()
	tk.SetIssuedAt(time.Now())
	tk.SetNotBefore(time.Now())
	tk.SetExpiration(time.Now().Add(time.Minute))
	tk.SetJti("5c1f6f7e-6a64-4a7e-9c1e-2b1f1b1f1b1f")
	tk.SetString("tipo", "access")
	require.NoError(t, tk.Set("uid", 1))
	foot, _ := json.Marshal(footerToken{KID: "k1"})
	tk.SetFooter(foot)
	tok := tk.V4Encrypt(m.claves["k1"], nil)

	_, err := m.VerificarToken(tok, TipoTokenAcceso)
	require.ErrorIs(t, err, ErrTokenInvalido)
}

func TestKidDesconocido(t *testing.T) {
	a := nuevoMaker(t, map[string][]byte{"k9": clave(9)}, "k9", "iss", "aud")
	tok, _, _ := a.CrearAccessToken(1, "x", nil, false, 1, time.Minute)
	_, err := base(t).VerificarToken(tok, TipoTokenAcceso)
	require.ErrorIs(t, err, ErrTokenInvalido)
}

func TestClaveIncorrectaConMismoKid(t *testing.T) {
	a := nuevoMaker(t, map[string][]byte{"k1": clave(2)}, "k1", "iss", "aud")
	tok, _, _ := a.CrearAccessToken(1, "x", nil, false, 1, time.Minute)
	_, err := base(t).VerificarToken(tok, TipoTokenAcceso)
	require.ErrorIs(t, err, ErrTokenInvalido)
}

func TestRotacionDeClaves(t *testing.T) {
	viejo := nuevoMaker(t, map[string][]byte{"k1": clave(1)}, "k1", "iss", "aud")
	tokViejo, _, _ := viejo.CrearAccessToken(1, "x", nil, false, 1, time.Minute)

	// Despliegue con ambas claves, activa k2.
	rot := nuevoMaker(t, map[string][]byte{"k1": clave(1), "k2": clave(2)}, "k2", "iss", "aud")
	_, err := rot.VerificarToken(tokViejo, TipoTokenAcceso)
	require.NoError(t, err, "el token firmado con el kid viejo sigue válido")
	tokNuevo, p, _ := rot.CrearAccessToken(1, "x", nil, false, 1, time.Minute)
	require.Equal(t, "k2", p.KID)

	// Se retira k1: el viejo deja de valer, el nuevo sigue.
	soloK2 := nuevoMaker(t, map[string][]byte{"k2": clave(2)}, "k2", "iss", "aud")
	_, err = soloK2.VerificarToken(tokViejo, TipoTokenAcceso)
	require.ErrorIs(t, err, ErrTokenInvalido)
	_, err = soloK2.VerificarToken(tokNuevo, TipoTokenAcceso)
	require.NoError(t, err)
}

func TestConstructorValida(t *testing.T) {
	_, err := NuevoPasetoMaker(ConfigPaseto{Claves: map[string][]byte{"k1": []byte("corta")}, KIDActivo: "k1", Issuer: "i", Audience: "a"})
	require.Error(t, err)
	_, err = NuevoPasetoMaker(ConfigPaseto{Claves: map[string][]byte{"k1": clave(1)}, KIDActivo: "zz", Issuer: "i", Audience: "a"})
	require.Error(t, err)
	_, err = NuevoPasetoMaker(ConfigPaseto{Claves: map[string][]byte{"k1": clave(1)}, KIDActivo: "k1", Issuer: "", Audience: "a"})
	require.Error(t, err)
}

func TestBasuraYTokenV2Antiguo(t *testing.T) {
	m := base(t)
	for _, s := range []string{"", "x", "v2.local.AAAA", "v4.local.AAAA"} {
		_, err := m.VerificarToken(s, TipoTokenAcceso)
		require.ErrorIs(t, err, ErrTokenInvalido)
	}
}