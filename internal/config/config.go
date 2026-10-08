// Package config agrupa la configuración de la aplicación (viper), cargada
// desde variables de entorno o .env. Antes vivía como paquete "utils" en la
// raíz del módulo; se renombró y se movió bajo internal/ para que el nombre
// del paquete describa su contenido y quede protegido de imports externos
// como el resto de la app (ver ORGANIZACION.md).
package config

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config es la configuración completa de la aplicación, cargada desde
// variables de entorno (o un archivo .env en desarrollo). Ver
// .env.example para la lista completa de claves y su significado.
type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
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

type AuthConfig struct {
	// PasetoLocalKeyHex es la clave simétrica v2.local en hex (32 bytes / 64
	// caracteres hex). Ver internal/security.NuevoPasetoMaker
	// (github.com/o1egl/paseto) y cmd/api/main.go:newTokenMaker, que hace
	// el hex-decode antes de pasarla.
	PasetoLocalKeyHex   string
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
		Auth: AuthConfig{
			PasetoLocalKeyHex:   v.GetString("PASETO_LOCAL_KEY_HEX"),
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
	v.SetDefault("PASETO_ISSUER", "planillas-ccdr")
	v.SetDefault("PASETO_AUDIENCE", "planillas-ccdr-api")

	v.SetDefault("AUTH_REFRESH_TOKEN_HOURS", 168) // 7 días
	v.SetDefault("AUTH_PASSWORD_RESET_TOKEN_MINUTES", 30)
	v.SetDefault("AUTH_MAX_FAILED_LOGINS", 5)
	v.SetDefault("AUTH_LOCKOUT_MINUTES", 15)
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
	if c.Auth.PasetoLocalKeyHex == "" {
		missing = append(missing, "PASETO_LOCAL_KEY_HEX")
	} else {
		if len(c.Auth.PasetoLocalKeyHex) != 64 {
			return fmt.Errorf("config: PASETO_LOCAL_KEY_HEX debe tener 64 caracteres hex (32 bytes), tiene %d", len(c.Auth.PasetoLocalKeyHex))
		}
		if _, err := hex.DecodeString(c.Auth.PasetoLocalKeyHex); err != nil {
			return fmt.Errorf("config: PASETO_LOCAL_KEY_HEX no es hex válido: %w", err)
		}
		// La clave del .env.example es todo ceros: en producción sería un
		// despliegue con una clave pública y conocida.
		if c.App.IsProduction() && strings.Trim(c.Auth.PasetoLocalKeyHex, "0") == "" {
			return fmt.Errorf("config: PASETO_LOCAL_KEY_HEX es la clave de ejemplo (todo ceros); no se permite con APP_ENV=production")
		}
	}
	if c.App.IsProduction() && c.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
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