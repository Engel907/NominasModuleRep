package config

import (
	"fmt"
	"strings"
	"time"
)

// LoadJobsConfig carga la configuración para comandos batch (cmd/jobs): mismas
// variables y defaults que LoadConfig, pero solo exige lo que un job usa (BD,
// logging, caché de parámetros). No requiere PASETO, SMTP ni rate limits.
func LoadJobsConfig() (*Config, error) { return cargarConfig((*Config).validarJobs) }

func (c *Config) validarJobs() error {
	var missing []string
	if c.Database.User == "" {
		missing = append(missing, "DB_USER")
	}
	if c.Database.Name == "" {
		missing = append(missing, "DB_NAME")
	}
	if c.App.IsProduction() && c.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if len(missing) > 0 {
		return fmt.Errorf("config: faltan variables de entorno requeridas: %v", missing)
	}

	switch strings.ToLower(strings.TrimSpace(c.App.LogLevel)) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: LOG_LEVEL debe ser debug|info|warn|error (es %q)", c.App.LogLevel)
	}
	if c.Parametros.CacheTTL < 0 || c.Parametros.CacheTTL > 5*time.Minute {
		return fmt.Errorf("config: PARAMETROS_CACHE_TTL debe estar entre 0 y 5m")
	}

	// TLS a la BD: mismas reglas que validarEndurecimientoHTTP (S9).
	switch c.Database.TLSMode {
	case "disabled", "required", "verify-full":
	default:
		return fmt.Errorf("config: DB_TLS_MODE %q no es válido (disabled|required|verify-full)", c.Database.TLSMode)
	}
	if c.App.IsProduction() && c.Database.TLSMode == "disabled" {
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
