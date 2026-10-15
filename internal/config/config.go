// Package config agrupa la configuración de la aplicación (viper), cargada
// desde variables de entorno o .env. Antes vivía como paquete "utils" en la
// raíz del módulo; se renombró y se movió bajo internal/ para que el nombre
// del paquete describa su contenido y quede protegido de imports externos
// como el resto de la app (ver ORGANIZACION.md).
package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

// Config es la configuración completa de la aplicación, cargada desde
// variables de entorno (o un archivo .env en desarrollo). Ver
// .env.example para la lista completa de claves y su significado.
type Config struct {
	App        AppConfig
	Server     ServerConfig
	Database   DatabaseConfig
	Auth       AuthConfig
	SMTP       SMTPConfig
	Asistencia AsistenciaConfig
	RateLimit  RateLimitConfig
	Parametros ParametrosConfig
}

type AppConfig struct {
	Env      string // development | staging | production
	LogLevel string
}

func (a AppConfig) IsProduction() bool {
	return a.Env == "production"
}

// SlogLevel traduce LOG_LEVEL (debug|info|warn|error) a slog.Level. validate()
// ya rechaza valores inválidos; ante uno desconocido devuelve Info.
func (a AppConfig) SlogLevel() slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.TrimSpace(a.LogLevel))); err != nil {
		return slog.LevelInfo
	}
	return l
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration

	// F16 (S8/S15). MaxBodyBytes acota el cuerpo de toda petición.
	// TrustedProxies son las IP/CIDR de los proxies que SÍ pueden informar la IP
	// real vía X-Forwarded-For (vacío = ninguno). CORSAllowedOrigins es la lista
	// blanca exacta de orígenes (vacía = ninguno; "*" no se admite).
	MaxBodyBytes       int64
	TrustedProxies     []string
	CORSAllowedOrigins []string
}

// RateLimitConfig (F16/S8): límites por minuto y ráfaga. NO tienen valor por
// defecto: un default inseguro (o un "0 = sin límite") pasaría desapercibido,
// así que deben declararse y ser > 0 (ver .env.example).
type RateLimitConfig struct {
	// Por IP y ruta, en login, refresh, logout y reset de contraseña.
	AuthPorMin int
	AuthBurst  int
	// Por usuario autenticado, sobre todas las rutas protegidas.
	UsuarioPorMin int
	UsuarioBurst  int
}

type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	ExtraParams     string // p.ej. "parseTime=true&loc=UTC&charset=utf8mb4"
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
	PingTimeout     time.Duration

	// F16 (S9). TLSMode: disabled | required | verify-full.
	//   disabled    sin TLS (prohibido con APP_ENV=production).
	//   required    TLS obligatorio, cifra el canal pero NO verifica el
	//               certificado del servidor (no protege contra un MITM activo).
	//   verify-full TLS verificando la cadena (TLSCAFile o raíces del sistema) y
	//               que el certificado corresponda a Host.
	// TLSCAFile (PEM) solo aplica a verify-full.
	TLSMode   string
	TLSCAFile string
}

// nombreTLSMySQL es el nombre con el que se registra la config TLS en el driver
// (parámetro tls=<nombre> del DSN).
const nombreTLSMySQL = "planillas-db"

// TLSConfig arma el *tls.Config según TLSMode (nil si disabled).
func (d DatabaseConfig) TLSConfig() (*tls.Config, error) {
	switch d.TLSMode {
	case "", "disabled":
		return nil, nil
	case "required":
		// Cifrado sin verificación de certificado: es exactamente lo que
		// significa "required" (ver arriba); se documenta y se advierte al
		// arrancar en producción.
		return &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}, nil // #nosec G402
	case "verify-full":
		cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: d.Host}
		if d.TLSCAFile != "" {
			pem, err := os.ReadFile(d.TLSCAFile)
			if err != nil {
				return nil, fmt.Errorf("config: no se pudo leer DB_TLS_CA_FILE: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("config: DB_TLS_CA_FILE no contiene certificados PEM válidos")
			}
			cfg.RootCAs = pool
		}
		return cfg, nil
	}
	return nil, fmt.Errorf("config: DB_TLS_MODE %q no es válido (disabled|required|verify-full)", d.TLSMode)
}

// RegistrarTLS registra la config TLS en el driver MySQL bajo el nombre que
// usa DSN(). Debe llamarse ANTES de sql.Open. No hace nada si TLSMode=disabled.
func (d DatabaseConfig) RegistrarTLS() error {
	cfg, err := d.TLSConfig()
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}
	return mysql.RegisterTLSConfig(nombreTLSMySQL, cfg)
}

// DSN arma el Data Source Name para github.com/go-sql-driver/mysql.
func (d DatabaseConfig) DSN() string {
	params := d.ExtraParams
	if params == "" {
		params = "parseTime=true&loc=UTC&charset=utf8mb4"
	}
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?%s&timeout=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, params, d.ConnectTimeout,
	)
	if d.TLSMode != "" && d.TLSMode != "disabled" {
		dsn += "&tls=" + nombreTLSMySQL
	}
	return dsn
}

// SMTPConfig es el canal de correo del reset de contraseña (F14 / S2).
// Con Host vacío no hay SMTP: solo se permite en APP_ENV=development, donde
// auth.LogNotificador registra el token en el log.
type SMTPConfig struct {
	Host string
	Port string // 465 = TLS implícito; cualquier otro = STARTTLS obligatorio
	User string
	Pass string
	From string // p.ej. "Planillas CCDR <no-responder@ccdrliberia.go.cr>"
	// URLBaseReset es la URL del frontend a la que se agrega ?token=...
	URLBaseReset string
}

type AsistenciaConfig struct {
	ToleranciaTardanzaMin int
}

// ParametrosConfig (F18A): caché en memoria del servicio parametros.Generales.
// CacheTTL es la latencia máxima con la que un cambio en parametros_generales
// se ve en un proceso que no llamó a Invalidar() (0 = sin caché).
type ParametrosConfig struct {
	CacheTTL time.Duration
}

type AuthConfig struct {
	// PasetoLocalKeyHex (LEGADO, F15): una sola clave v4.local en hex (64
	// caracteres). Se acepta solo si PasetoLocalKeysHex está vacío; equivale a
	// una clave con kid "k1". Preferir PasetoLocalKeysHex.
	PasetoLocalKeyHex string
	// PasetoLocalKeysHex (F15): llavero "kid1:hex,kid2:hex". Se firma con
	// PasetoActiveKID y se valida con cualquiera de la lista (rotación).
	PasetoLocalKeysHex string
	PasetoActiveKID    string
	// CacheTTL (F15): vigencia de la caché del verificador de sesión. Es la
	// cota superior de latencia de una revocación (0 = sin caché).
	CacheTTL            time.Duration
	AccessTokenDuration time.Duration
	Issuer              string
	Audience            string

	// Sesión / refresh tokens.
	RefreshTokenDuration time.Duration

	// Reset de contraseña: vigencia del token de un solo uso.
	PasswordResetTokenDuration time.Duration

	// Política de bloqueo por intentos fallidos de login: tras
	// MaxFailedLogins intentos consecutivos la cuenta queda bloqueada
	// durante LockoutDuration.
	MaxFailedLogins int
	LockoutDuration time.Duration
}

// LoadConfig lee la configuración desde (en orden de precedencia) variables
// de entorno del proceso y, si existe, un archivo .env en el directorio
// actual. No falla si el .env no existe (útil en producción, donde las
// variables ya vienen inyectadas por el orquestador).
func LoadConfig() (*Config, error) {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config: error leyendo .env: %w", err)
		}
		// .env no encontrado: seguimos solo con env vars + defaults.
	}

	cfg := &Config{
		App: AppConfig{
			Env:      v.GetString("APP_ENV"),
			LogLevel: v.GetString("LOG_LEVEL"),
		},
		Server: ServerConfig{
			Port:            v.GetString("SERVER_PORT"),
			ReadTimeout:     v.GetDuration("SERVER_READ_TIMEOUT_SECONDS") * time.Second,
			WriteTimeout:    v.GetDuration("SERVER_WRITE_TIMEOUT_SECONDS") * time.Second,
			IdleTimeout:     v.GetDuration("SERVER_IDLE_TIMEOUT_SECONDS") * time.Second,
			ShutdownTimeout: v.GetDuration("SERVER_SHUTDOWN_TIMEOUT_SECONDS") * time.Second,

			MaxBodyBytes:       v.GetInt64("MAX_BODY_BYTES"),
			TrustedProxies:     listaCSV(v.GetString("TRUSTED_PROXIES")),
			CORSAllowedOrigins: listaCSV(v.GetString("CORS_ALLOWED_ORIGINS")),
		},
		Database: DatabaseConfig{
			Host:            v.GetString("DB_HOST"),
			Port:            v.GetString("DB_PORT"),
			User:            v.GetString("DB_USER"),
			Password:        v.GetString("DB_PASSWORD"),
			Name:            v.GetString("DB_NAME"),
			ExtraParams:     v.GetString("DB_PARAMS"),
			MaxOpenConns:    v.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    v.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: v.GetDuration("DB_CONN_MAX_LIFETIME_MINUTES") * time.Minute,
			ConnMaxIdleTime: v.GetDuration("DB_CONN_MAX_IDLE_TIME_MINUTES") * time.Minute,
			ConnectTimeout:  v.GetDuration("DB_CONNECT_TIMEOUT_SECONDS") * time.Second,
			PingTimeout:     v.GetDuration("DB_PING_TIMEOUT_SECONDS") * time.Second,
			TLSMode:         strings.ToLower(strings.TrimSpace(v.GetString("DB_TLS_MODE"))),
			TLSCAFile:       strings.TrimSpace(v.GetString("DB_TLS_CA_FILE")),
		},
		RateLimit: RateLimitConfig{
			AuthPorMin:    v.GetInt("RATE_LIMIT_AUTH_PER_MIN"),
			AuthBurst:     v.GetInt("RATE_LIMIT_AUTH_BURST"),
			UsuarioPorMin: v.GetInt("RATE_LIMIT_USER_PER_MIN"),
			UsuarioBurst:  v.GetInt("RATE_LIMIT_USER_BURST"),
		},
		Asistencia: AsistenciaConfig{ToleranciaTardanzaMin: v.GetInt("ASISTENCIA_TOLERANCIA_TARDANZA_MIN")},
		Parametros: ParametrosConfig{CacheTTL: v.GetDuration("PARAMETROS_CACHE_TTL")},
		SMTP: SMTPConfig{
			Host:         v.GetString("SMTP_HOST"),
			Port:         v.GetString("SMTP_PORT"),
			User:         v.GetString("SMTP_USER"),
			Pass:         v.GetString("SMTP_PASS"),
			From:         v.GetString("SMTP_FROM"),
			URLBaseReset: v.GetString("URL_BASE_RESET"),
		},
		Auth: AuthConfig{
			PasetoLocalKeyHex:   v.GetString("PASETO_LOCAL_KEY_HEX"),
			PasetoLocalKeysHex:  v.GetString("PASETO_LOCAL_KEYS_HEX"),
			PasetoActiveKID:     v.GetString("PASETO_ACTIVE_KID"),
			CacheTTL:            v.GetDuration("AUTH_CACHE_TTL"),
			AccessTokenDuration: v.GetDuration("PASETO_ACCESS_TOKEN_MINUTES") * time.Minute,
			Issuer:              v.GetString("PASETO_ISSUER"),
			Audience:            v.GetString("PASETO_AUDIENCE"),

			RefreshTokenDuration:       v.GetDuration("AUTH_REFRESH_TOKEN_HOURS") * time.Hour,
			PasswordResetTokenDuration: v.GetDuration("AUTH_PASSWORD_RESET_TOKEN_MINUTES") * time.Minute,
			MaxFailedLogins:            v.GetInt("AUTH_MAX_FAILED_LOGINS"),
			LockoutDuration:            v.GetDuration("AUTH_LOCKOUT_MINUTES") * time.Minute,
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("LOG_LEVEL", "debug")

	v.SetDefault("SERVER_PORT", "8080")
	v.SetDefault("SERVER_READ_TIMEOUT_SECONDS", 10)
	v.SetDefault("SERVER_WRITE_TIMEOUT_SECONDS", 15)
	v.SetDefault("SERVER_IDLE_TIMEOUT_SECONDS", 60)
	v.SetDefault("SERVER_SHUTDOWN_TIMEOUT_SECONDS", 15)

	v.SetDefault("DB_HOST", "127.0.0.1")
	v.SetDefault("DB_PORT", "3306")
	v.SetDefault("DB_PARAMS", "parseTime=true&loc=UTC&charset=utf8mb4")
	v.SetDefault("DB_MAX_OPEN_CONNS", 25)
	v.SetDefault("DB_MAX_IDLE_CONNS", 25)
	v.SetDefault("DB_CONN_MAX_LIFETIME_MINUTES", 5)
	v.SetDefault("DB_CONN_MAX_IDLE_TIME_MINUTES", 2)
	v.SetDefault("DB_CONNECT_TIMEOUT_SECONDS", 5)
	v.SetDefault("DB_PING_TIMEOUT_SECONDS", 5)
	v.SetDefault("DB_TLS_MODE", "disabled") // con APP_ENV=production validate() lo rechaza

	v.SetDefault("MAX_BODY_BYTES", 1<<20) // 1 MiB
	// RATE_LIMIT_*: sin default a propósito (ver RateLimitConfig).

	v.SetDefault("PASETO_ACCESS_TOKEN_MINUTES", 20)
	v.SetDefault("AUTH_CACHE_TTL", "5s")
	v.SetDefault("PASETO_ISSUER", "planillas-ccdr")
	v.SetDefault("PASETO_AUDIENCE", "planillas-ccdr-api")

	v.SetDefault("AUTH_REFRESH_TOKEN_HOURS", 168) // 7 días
	v.SetDefault("AUTH_PASSWORD_RESET_TOKEN_MINUTES", 30)
	v.SetDefault("AUTH_MAX_FAILED_LOGINS", 5)
	v.SetDefault("AUTH_LOCKOUT_MINUTES", 15)
	v.SetDefault("ASISTENCIA_TOLERANCIA_TARDANZA_MIN", 0)
	v.SetDefault("PARAMETROS_CACHE_TTL", "30s")

	v.SetDefault("SMTP_PORT", "587")
}

// validate atrapa temprano los casos de configuración incompleta que de
// otra forma solo se notarían al primer request (DB o token inválido).
func (c *Config) validate() error {
	var missing []string

	if c.Database.User == "" {
		missing = append(missing, "DB_USER")
	}
	if c.Database.Name == "" {
		missing = append(missing, "DB_NAME")
	}
	// F15: llavero PASETO (validado completo; falla en el arranque).
	if c.Auth.PasetoLocalKeysHex == "" && c.Auth.PasetoLocalKeyHex == "" {
		missing = append(missing, "PASETO_LOCAL_KEYS_HEX")
	} else {
		claves, _, err := c.Auth.ClavesPaseto()
		if err != nil {
			return err
		}
		if c.App.IsProduction() {
			for kid, k := range claves {
				if strings.Trim(hex.EncodeToString(k), "0") == "" {
					return fmt.Errorf("config: la clave PASETO %q es la de ejemplo (todo ceros); no se permite con APP_ENV=production", kid)
				}
			}
		}
	}
	if c.Auth.Issuer == "" {
		missing = append(missing, "PASETO_ISSUER")
	}
	if c.Auth.Audience == "" {
		missing = append(missing, "PASETO_AUDIENCE")
	}
	if c.Auth.CacheTTL < 0 || c.Auth.CacheTTL > time.Minute {
		return fmt.Errorf("config: AUTH_CACHE_TTL debe estar entre 0 y 1m (es la latencia máxima de una revocación)")
	}
	if c.Parametros.CacheTTL < 0 || c.Parametros.CacheTTL > 5*time.Minute {
		return fmt.Errorf("config: PARAMETROS_CACHE_TTL debe estar entre 0 y 5m")
	}
	if c.App.IsProduction() && c.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}

	// Reset de contraseña por correo (F14). En producción SMTP no es opcional:
	// sin él el flujo de reset no entregaría el token a nadie. Fuera de
	// producción, SMTP_HOST vacío se acepta solo en development (LogNotificador).
	if c.App.IsProduction() {
		if c.SMTP.Host == "" {
			missing = append(missing, "SMTP_HOST")
		}
		if c.SMTP.From == "" {
			missing = append(missing, "SMTP_FROM")
		}
		if c.SMTP.URLBaseReset == "" {
			missing = append(missing, "URL_BASE_RESET")
		} else if !strings.HasPrefix(c.SMTP.URLBaseReset, "https://") {
			return fmt.Errorf("config: URL_BASE_RESET debe empezar con https:// en producción")
		}
	} else if c.SMTP.Host != "" {
		if c.SMTP.From == "" {
			missing = append(missing, "SMTP_FROM")
		}
		if c.SMTP.URLBaseReset == "" {
			missing = append(missing, "URL_BASE_RESET")
		}
	} else if c.App.Env != "development" {
		return fmt.Errorf("config: APP_ENV=%s requiere SMTP_HOST (el LogNotificador solo existe en development)", c.App.Env)
	}

	if c.Auth.AccessTokenDuration <= 0 {
		return fmt.Errorf("config: PASETO_ACCESS_TOKEN_MINUTES debe ser > 0")
	}
	if c.Auth.RefreshTokenDuration <= c.Auth.AccessTokenDuration {
		return fmt.Errorf("config: AUTH_REFRESH_TOKEN_HOURS debe ser mayor que la vigencia del access token")
	}
	if c.Auth.PasswordResetTokenDuration <= 0 {
		return fmt.Errorf("config: AUTH_PASSWORD_RESET_TOKEN_MINUTES debe ser > 0")
	}
	if c.Auth.MaxFailedLogins <= 0 {
		return fmt.Errorf("config: AUTH_MAX_FAILED_LOGINS debe ser > 0")
	}
	if c.Auth.LockoutDuration <= 0 {
		return fmt.Errorf("config: AUTH_LOCKOUT_MINUTES debe ser > 0")
	}

	if err := c.validarEndurecimientoHTTP(&missing); err != nil {
		return err
	}

	if len(missing) > 0 {
		return fmt.Errorf("config: faltan variables de entorno requeridas: %v", missing)
	}

	return nil
}

// validarEndurecimientoHTTP valida lo agregado en F16: logging, límite de
// cuerpo, rate limiting, CORS, proxies de confianza y TLS a la BD.
func (c *Config) validarEndurecimientoHTTP(missing *[]string) error {
	prod := c.App.IsProduction()

	switch strings.ToLower(strings.TrimSpace(c.App.LogLevel)) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: LOG_LEVEL debe ser debug|info|warn|error (es %q)", c.App.LogLevel)
	}

	if c.Server.MaxBodyBytes <= 0 {
		return fmt.Errorf("config: MAX_BODY_BYTES debe ser > 0")
	}

	for _, r := range []struct {
		nombre string
		valor  int
	}{
		{"RATE_LIMIT_AUTH_PER_MIN", c.RateLimit.AuthPorMin},
		{"RATE_LIMIT_AUTH_BURST", c.RateLimit.AuthBurst},
		{"RATE_LIMIT_USER_PER_MIN", c.RateLimit.UsuarioPorMin},
		{"RATE_LIMIT_USER_BURST", c.RateLimit.UsuarioBurst},
	} {
		if r.valor <= 0 {
			*missing = append(*missing, r.nombre+" (debe ser > 0)")
		}
	}

	// CORS: lista blanca exacta. Nunca "*".
	for _, o := range c.Server.CORSAllowedOrigins {
		if o == "*" {
			return fmt.Errorf("config: CORS_ALLOWED_ORIGINS no admite \"*\"; liste los orígenes explícitos")
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return fmt.Errorf("config: CORS_ALLOWED_ORIGINS: origen %q inválido (formato esperado: https://host[:puerto], sin ruta ni barra final)", o)
		}
		if prod && u.Scheme != "https" {
			return fmt.Errorf("config: CORS_ALLOWED_ORIGINS: origen %q debe ser https:// en producción", o)
		}
	}

	// Proxies de confianza: IP o CIDR. En producción no se admite confiar en
	// todo Internet (permitiría falsificar X-Forwarded-For y evadir el rate limit).
	for _, p := range c.Server.TrustedProxies {
		if net.ParseIP(p) != nil {
			continue
		}
		_, red, err := net.ParseCIDR(p)
		if err != nil {
			return fmt.Errorf("config: TRUSTED_PROXIES: %q no es una IP ni un CIDR válido", p)
		}
		if ones, _ := red.Mask.Size(); prod && ones == 0 {
			return fmt.Errorf("config: TRUSTED_PROXIES: %q confía en todo Internet; no se permite con APP_ENV=production", p)
		}
	}

	// TLS a la BD (S9).
	switch c.Database.TLSMode {
	case "disabled", "required", "verify-full":
	default:
		return fmt.Errorf("config: DB_TLS_MODE %q no es válido (disabled|required|verify-full)", c.Database.TLSMode)
	}
	if prod && c.Database.TLSMode == "disabled" {
		return fmt.Errorf("config: DB_TLS_MODE=disabled no se permite con APP_ENV=production (use required o, mejor, verify-full)")
	}
	if c.Database.TLSCAFile != "" && c.Database.TLSMode != "verify-full" {
		return fmt.Errorf("config: DB_TLS_CA_FILE solo aplica con DB_TLS_MODE=verify-full")
	}
	for _, par := range strings.Split(c.Database.ExtraParams, "&") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(par)), "tls=") {
			return fmt.Errorf("config: no incluya tls= en DB_PARAMS; el TLS se controla con DB_TLS_MODE")
		}
	}
	return nil
}

// listaCSV separa "a, b,c" en ["a","b","c"] ignorando vacíos.
func listaCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var kidValido = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// ClavesPaseto decodifica el llavero y devuelve (kid -> 32 bytes, kid activo).
// PASETO_LOCAL_KEYS_HEX tiene precedencia; si falta se usa la clave legada
// PASETO_LOCAL_KEY_HEX con kid "k1". Falla si hay kids repetidos o inválidos,
// claves de largo != 32 bytes, o no hay kid activo determinable.
func (a AuthConfig) ClavesPaseto() (map[string][]byte, string, error) {
	claves := map[string][]byte{}
	activo := strings.TrimSpace(a.PasetoActiveKID)

	if strings.TrimSpace(a.PasetoLocalKeysHex) == "" {
		k, err := decodificarClave("PASETO_LOCAL_KEY_HEX", a.PasetoLocalKeyHex)
		if err != nil {
			return nil, "", err
		}
		if activo != "" && activo != "k1" {
			return nil, "", fmt.Errorf("config: PASETO_ACTIVE_KID=%q no existe; con PASETO_LOCAL_KEY_HEX el único kid es \"k1\"", activo)
		}
		return map[string][]byte{"k1": k}, "k1", nil
	}

	for _, par := range strings.Split(a.PasetoLocalKeysHex, ",") {
		kid, h, ok := strings.Cut(strings.TrimSpace(par), ":")
		if !ok || !kidValido.MatchString(kid) {
			return nil, "", fmt.Errorf("config: PASETO_LOCAL_KEYS_HEX debe tener formato kid:hex,kid:hex (kid = letras, dígitos, _ o -, máx. 32)")
		}
		if _, dup := claves[kid]; dup {
			return nil, "", fmt.Errorf("config: kid %q repetido en PASETO_LOCAL_KEYS_HEX", kid)
		}
		k, err := decodificarClave("PASETO_LOCAL_KEYS_HEX["+kid+"]", h)
		if err != nil {
			return nil, "", err
		}
		claves[kid] = k
	}
	if activo == "" {
		if len(claves) != 1 {
			return nil, "", fmt.Errorf("config: PASETO_ACTIVE_KID es obligatorio con más de una clave")
		}
		for kid := range claves {
			activo = kid
		}
	}
	if _, ok := claves[activo]; !ok {
		kids := make([]string, 0, len(claves))
		for k := range claves {
			kids = append(kids, k)
		}
		sort.Strings(kids)
		return nil, "", fmt.Errorf("config: PASETO_ACTIVE_KID=%q no está en PASETO_LOCAL_KEYS_HEX (kids: %v)", activo, kids)
	}
	return claves, activo, nil
}

func decodificarClave(nombre, h string) ([]byte, error) {
	if len(h) != 64 {
		return nil, fmt.Errorf("config: %s debe tener 64 caracteres hex (32 bytes), tiene %d", nombre, len(h))
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, fmt.Errorf("config: %s no es hex válido: %w", nombre, err)
	}
	return b, nil
}