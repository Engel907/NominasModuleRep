//go:build integration

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/app"
	"planillas-ccdr/internal/config"
	"planillas-ccdr/internal/security"
	"planillas-ccdr/internal/seedadmin"
	"planillas-ccdr/internal/testutil"
)

// ⚠️ AJUSTAR contra los handlers reales (no estaban entre los archivos pasados):
// rutas, nombres de campos JSON y códigos de estado. Cada constante/mapa de
// abajo es lo único que debería tocarse. Lee: auth handler (login), organizacion
// handler (unidad/programa) y empleado handler (campos obligatorios).
const (
	rutaLogin     = "/api/v1/auth/login"
	rutaEmpleados = "/api/v1/empleados"
	rutaUnidades  = "/api/v1/unidades-organizativas"
	rutaProgramas = "/api/v1/programas-presupuestarios"
)

func cuerpoLogin(usuario, pass string) map[string]any {
	return map[string]any{"nombre_usuario": usuario, "password": pass}
}

func cuerpoUnidad() map[string]any {
	return map[string]any{"codigo": "UO-HUMO", "nombre": "Unidad de humo"}
}

func cuerpoPrograma() map[string]any {
	return map[string]any{"codigo": "PR-HUMO", "nombre": "Programa de humo"}
}

func cuerpoEmpleado() map[string]any {
	return map[string]any{
		"numero_identificacion": "1-1111-1111",
		"nombre_completo":       "Empleado de Humo",
		"fecha_nacimiento":      "1990-01-15",
	}
}

type cliente struct {
	t     *testing.T
	base  string
	token string
}

func (c *cliente) llamar(metodo, ruta string, body any) (int, any) {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(metodo, c.base+ruta, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", metodo, ruta, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out any
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = string(raw)
	}
	return resp.StatusCode, out
}

// buscar devuelve el primer valor string de la clave en cualquier nivel.
func buscar(v any, clave string) string {
	switch x := v.(type) {
	case map[string]any:
		if s, ok := x[clave].(string); ok {
			return s
		}
		for _, h := range x {
			if s := buscar(h, clave); s != "" {
				return s
			}
		}
	case []any:
		for _, h := range x {
			if s := buscar(h, clave); s != "" {
				return s
			}
		}
	}
	return ""
}

func esExito(st int) bool { return st == http.StatusOK || st == http.StatusCreated }

func TestHumoAPI(t *testing.T) {
	sqlDB := testutil.NuevaBD(t)
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App: config.AppConfig{Env: "test"},
		Auth: config.AuthConfig{
			PasetoLocalKeyHex:          strings.Repeat("ab", 32),
			AccessTokenDuration:        20 * time.Minute,
			RefreshTokenDuration:       168 * time.Hour,
			PasswordResetTokenDuration: 30 * time.Minute,
			MaxFailedLogins:            5,
			LockoutDuration:            15 * time.Minute,
		},
	}
	var maker security.TokenMaker
	maker, err := app.NewTokenMaker(cfg.Auth)
	if err != nil {
		t.Fatal(err)
	}
	router, err := app.New(cfg, sqlDB, maker)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	defer srv.Close()

	// Admin con el mismo código que cmd/seedadmin + requiere_cambio_password = 0.
	const usuario = "humo.admin"
	pass, err := seedadmin.GenerarPassword(seedadmin.LongitudPassword)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := security.HashPassword(pass)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := seedadmin.CrearAdministrador(ctx, sqlDB, usuario, "Admin de Humo", "humo@ejemplo.cr", hash); err != nil {
		t.Fatalf("seedadmin: %v", err)
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Los triggers de auditoría leen @auditoria_usuario_id en la misma sesión.
	if _, err := tx.ExecContext(ctx, "SET @auditoria_usuario_id = NULL"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE usuarios SET requiere_cambio_password = 0 WHERE nombre_usuario = ?", usuario); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	c := &cliente{t: t, base: srv.URL}

	t.Run("health", func(t *testing.T) {
		if st, body := c.llamar("GET", "/healthz", nil); st != http.StatusOK {
			t.Fatalf("healthz: %d %v", st, body)
		}
	})

	t.Run("login", func(t *testing.T) {
		st, body := c.llamar("POST", rutaLogin, cuerpoLogin(usuario, pass))
		if st != http.StatusOK {
			t.Fatalf("login: %d %v", st, body)
		}
		c.token = buscar(body, "access_token")
		if c.token == "" {
			t.Fatalf("login sin access_token en la respuesta: %v", body)
		}
	})

	t.Run("protegido_sin_y_con_token", func(t *testing.T) {
		con := c.token
		c.token = ""
		if st, body := c.llamar("GET", rutaEmpleados, nil); st != http.StatusUnauthorized {
			t.Errorf("sin token esperaba 401, fue %d %v", st, body)
		}
		c.token = con
		if st, body := c.llamar("GET", rutaEmpleados, nil); st != http.StatusOK {
			t.Errorf("con token esperaba 200, fue %d %v", st, body)
		}
	})

	t.Run("crear_programa_y_unidad", func(t *testing.T) {
		if st, body := c.llamar("POST", rutaProgramas, cuerpoPrograma()); !esExito(st) {
			t.Errorf("crear programa: %d %v", st, body)
		}
		if st, body := c.llamar("POST", rutaUnidades, cuerpoUnidad()); !esExito(st) {
			t.Errorf("crear unidad: %d %v", st, body)
		}
	})

	t.Run("crear_y_listar_empleado", func(t *testing.T) {
		if st, body := c.llamar("POST", rutaEmpleados, cuerpoEmpleado()); !esExito(st) {
			t.Fatalf("crear empleado: %d %v", st, body)
		}
		st, body := c.llamar("GET", rutaEmpleados, nil)
		if st != http.StatusOK {
			t.Fatalf("listar empleados: %d %v", st, body)
		}
		if b, _ := json.Marshal(body); !strings.Contains(string(b), "Empleado de Humo") {
			t.Errorf("el empleado creado no aparece en el listado: %s", b)
		}
	})
}

func TestHumoCrearPeriodoPlanilla(t *testing.T) {
	t.Skip("F19")
}