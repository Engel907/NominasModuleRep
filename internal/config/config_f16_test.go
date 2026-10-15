package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// entornoDev deja un entorno de desarrollo válido; cada test pisa lo que necesita.
func entornoDev(t *testing.T) {
	t.Helper()
	validas := map[string]string{
		"APP_ENV":                 "development",
		"LOG_LEVEL":               "info",
		"DB_USER":                 "u",
		"DB_NAME":                 "n",
		"PASETO_LOCAL_KEYS_HEX":   "k1:" + strings.Repeat("ab", 32),
		"RATE_LIMIT_AUTH_PER_MIN": "10",
		"RATE_LIMIT_AUTH_BURST":   "10",
		"RATE_LIMIT_USER_PER_MIN": "300",
		"RATE_LIMIT_USER_BURST":   "60",
	}
	for k, v := range validas {
		t.Setenv(k, v)
	}
	// Variables que podrían venir del shell del desarrollador.
	for _, k := range []string{"DB_TLS_MODE", "DB_TLS_CA_FILE", "DB_PARAMS", "TRUSTED_PROXIES",
		"CORS_ALLOWED_ORIGINS", "MAX_BODY_BYTES", "DB_PASSWORD", "SMTP_HOST", "SMTP_FROM", "URL_BASE_RESET"} {
		t.Setenv(k, "")
	}
}

func entornoProd(t *testing.T) {
	t.Helper()
	entornoDev(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_PASSWORD", "x")
	t.Setenv("DB_TLS_MODE", "verify-full")
	t.Setenv("SMTP_HOST", "smtp.example")
	t.Setenv("SMTP_FROM", "a@b.c")
	t.Setenv("URL_BASE_RESET", "https://app.example/reset")
	t.Setenv("PASETO_LOCAL_KEYS_HEX", "k1:"+strings.Repeat("ab", 32))
}

func TestF16_DefaultsEnDesarrollo(t *testing.T) {
	entornoDev(t)
	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.EqualValues(t, 1<<20, cfg.Server.MaxBodyBytes)
	assert.Equal(t, "disabled", cfg.Database.TLSMode)
	assert.Empty(t, cfg.Server.TrustedProxies)
	assert.Empty(t, cfg.Server.CORSAllowedOrigins)
	assert.Equal(t, 10, cfg.RateLimit.AuthPorMin)
	assert.NotContains(t, cfg.Database.DSN(), "tls=")
}

func TestF16_RateLimitSinDefaultEsObligatorio(t *testing.T) {
	entornoDev(t)
	t.Setenv("RATE_LIMIT_AUTH_PER_MIN", "")
	_, err := LoadConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RATE_LIMIT_AUTH_PER_MIN")

	entornoDev(t)
	t.Setenv("RATE_LIMIT_USER_BURST", "0")
	_, err = LoadConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RATE_LIMIT_USER_BURST")
}

func TestF16_LogLevelInvalido(t *testing.T) {
	entornoDev(t)
	t.Setenv("LOG_LEVEL", "verbose")
	_, err := LoadConfig()
	require.Error(t, err)
}

func TestF16_CORS(t *testing.T) {
	casos := []struct {
		valor string
		ok    bool
	}{
		{"*", false},
		{"https://a.example,*", false},
		{"https://a.example/", false}, // barra final
		{"https://a.example/ruta", false},
		{"ftp://a.example", false},
		{"a.example", false},
		{"https://a.example", true},
		{"https://a.example, http://localhost:5173", true}, // http solo fuera de producción
	}
	for _, tc := range casos {
		entornoDev(t)
		t.Setenv("CORS_ALLOWED_ORIGINS", tc.valor)
		cfg, err := LoadConfig()
		if tc.ok {
			require.NoError(t, err, tc.valor)
			assert.NotEmpty(t, cfg.Server.CORSAllowedOrigins)
		} else {
			assert.Error(t, err, tc.valor)
		}
	}

	entornoProd(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://a.example")
	_, err := LoadConfig()
	assert.Error(t, err, "http:// no se admite en producción")
}

func TestF16_TrustedProxies(t *testing.T) {
	entornoDev(t)
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8, 192.168.1.10")
	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.0/8", "192.168.1.10"}, cfg.Server.TrustedProxies)

	entornoDev(t)
	t.Setenv("TRUSTED_PROXIES", "no-es-una-ip")
	_, err = LoadConfig()
	assert.Error(t, err)

	entornoProd(t)
	t.Setenv("TRUSTED_PROXIES", "0.0.0.0/0")
	_, err = LoadConfig()
	assert.Error(t, err, "confiar en todo Internet no se admite en producción")
}

func TestF16_TLSBaseDeDatos(t *testing.T) {
	// Producción + disabled => falla el arranque.
	entornoProd(t)
	t.Setenv("DB_TLS_MODE", "disabled")
	_, err := LoadConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_TLS_MODE")

	// Producción sin declarar el modo (default disabled) => también falla.
	entornoProd(t)
	t.Setenv("DB_TLS_MODE", "")
	_, err = LoadConfig()
	assert.Error(t, err)

	// Modo desconocido.
	entornoDev(t)
	t.Setenv("DB_TLS_MODE", "preferred")
	_, err = LoadConfig()
	assert.Error(t, err)

	// required / verify-full en producción: ok y el DSN trae tls=.
	for _, modo := range []string{"required", "verify-full"} {
		entornoProd(t)
		t.Setenv("DB_TLS_MODE", modo)
		cfg, err := LoadConfig()
		require.NoError(t, err, modo)
		assert.Contains(t, cfg.Database.DSN(), "&tls="+nombreTLSMySQL)
	}

	// tls= en DB_PARAMS choca con DB_TLS_MODE.
	entornoDev(t)
	t.Setenv("DB_TLS_MODE", "required")
	t.Setenv("DB_PARAMS", "parseTime=true&tls=skip-verify")
	_, err = LoadConfig()
	assert.Error(t, err)

	// CA solo con verify-full.
	entornoDev(t)
	t.Setenv("DB_TLS_MODE", "required")
	t.Setenv("DB_TLS_CA_FILE", "/tmp/ca.pem")
	_, err = LoadConfig()
	assert.Error(t, err)
}

func TestF16_TLSConfig(t *testing.T) {
	d := DatabaseConfig{Host: "db.interno", TLSMode: "disabled"}
	cfg, err := d.TLSConfig()
	require.NoError(t, err)
	assert.Nil(t, cfg)
	assert.NoError(t, d.RegistrarTLS())

	d.TLSMode = "required"
	cfg, err = d.TLSConfig()
	require.NoError(t, err)
	assert.True(t, cfg.InsecureSkipVerify)
	assert.GreaterOrEqual(t, int(cfg.MinVersion), 0x0303) // TLS 1.2+

	d.TLSMode = "verify-full"
	cfg, err = d.TLSConfig()
	require.NoError(t, err)
	assert.False(t, cfg.InsecureSkipVerify)
	assert.Equal(t, "db.interno", cfg.ServerName)
	assert.NoError(t, d.RegistrarTLS())

	// CA inexistente o con basura => error claro.
	d.TLSCAFile = filepath.Join(t.TempDir(), "no-existe.pem")
	_, err = d.TLSConfig()
	assert.Error(t, err)

	basura := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(basura, []byte("esto no es PEM"), 0o600))
	d.TLSCAFile = basura
	_, err = d.TLSConfig()
	assert.Error(t, err)
}

func TestF16_SlogLevel(t *testing.T) {
	assert.Equal(t, "DEBUG", AppConfig{LogLevel: "debug"}.SlogLevel().String())
	assert.Equal(t, "WARN", AppConfig{LogLevel: "WARN"}.SlogLevel().String())
	assert.Equal(t, "INFO", AppConfig{LogLevel: "???"}.SlogLevel().String())
}
