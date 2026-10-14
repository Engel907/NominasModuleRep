// Package config agrupa la configuración de la aplicación (viper), cargada
// desde variables de entorno o .env. Antes vivía como paquete "utils" en la
// raíz del módulo; se renombró y se movió bajo internal/ para que el nombre
// del paquete describa su contenido y quede protegido de imports externos
// como el resto de la app (ver ORGANIZACION.md).
package config

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

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
}

type AppConfig struct {
	Env      string // development | staging | production
	LogLevel string
}

func (a AppConfig) IsProduction() bool {
	return a.Env == "production"
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
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
}

// DSN arma el Data Source Name para github.com/go-sql-driver/mysql.
func (d DatabaseConfig) DSN() string {
	params := d.ExtraParams
	if params == "" {
		params = "parseTime=true&loc=UTC&charset=utf8mb4"
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?%s&timeout=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, params, d.ConnectTimeout,
	)
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
		},
		Asistencia: AsistenciaConfig{ToleranciaTardanzaMin: v.GetInt("ASISTENCIA_TOLERANCIA_TARDANZA_MIN")},
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

	v.SetDefault("PASETO_ACCESS_TOKEN_MINUTES", 20)
	v.SetDefault("AUTH_CACHE_TTL", "5s")
	v.SetDefault("PASETO_ISSUER", "planillas-ccdr")
	v.SetDefault("PASETO_AUDIENCE", "planillas-ccdr-api")

	v.SetDefault("AUTH_REFRESH_TOKEN_HOURS", 168) // 7 días
	v.SetDefault("AUTH_PASSWORD_RESET_TOKEN_MINUTES", 30)
	v.SetDefault("AUTH_MAX_FAILED_LOGINS", 5)
	v.SetDefault("AUTH_LOCKOUT_MINUTES", 15)
	v.SetDefault("ASISTENCIA_TOLERANCIA_TARDANZA_MIN", 0)

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

	if len(missing) > 0 {
		return fmt.Errorf("config: faltan variables de entorno requeridas: %v", missing)
	}

	return nil
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