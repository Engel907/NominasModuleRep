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
	"planillas-ccdr/internal/credenciales"
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

// refsEmpleado: catálogos que exige el alta de empleado (se siembran por SQL).
type refsEmpleado struct{ tipoIdent, puesto, tipoContrato, programa int64 }

func cuerpoEmpleado(codigo, ident string, r refsEmpleado, regimenCCSS *int64) map[string]any {
	laborales := map[string]any{
		"puesto_id": r.puesto, "tipo_contrato_id": r.tipoContrato,
		"programa_presupuestario_id": r.programa, "fecha_efectiva": "2026-01-01",
	}
	if regimenCCSS != nil {
		laborales["regimen_ccss_id"] = *regimenCCSS
	}
	return map[string]any{
		"codigo_interno":         codigo,
		"tipo_identificacion_id": r.tipoIdent,
		"numero_identificacion":  ident,
		"nombre_completo":        "Empleado de Humo",
		"fecha_ingreso":          "2026-01-01",
		"datos_personales":       map[string]any{"fecha_efectiva": "2026-01-01"},
		"datos_laborales":        laborales,
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
		App:       config.AppConfig{Env: "development"},
		Server:    config.ServerConfig{MaxBodyBytes: 1 << 20},
		RateLimit: config.RateLimitConfig{AuthPorMin: 1000, AuthBurst: 1000, UsuarioPorMin: 1000, UsuarioBurst: 1000},
		Datos:     config.DatosConfig{LlavesHex: "d1:" + strings.Repeat("cd", 32), HMACHex: strings.Repeat("ef", 32)},
		Auth: config.AuthConfig{
			PasetoLocalKeyHex:          strings.Repeat("ab", 32),
			Issuer:                     "planillas-ccdr-test",
			Audience:                   "planillas-ccdr-test",
			AccessTokenDuration:        20 * time.Minute,
			RefreshTokenDuration:       168 * time.Hour,
			PasswordResetTokenDuration: 30 * time.Minute,
			MaxFailedLogins:            5,
			LockoutDuration:            15 * time.Minute,
		},
	}
	var maker credenciales.TokenMaker
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

	const usuario = "humo.admin"
	pass, err := seedadmin.GenerarPassword(seedadmin.LongitudPassword)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := credenciales.HashPassword(pass)
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
	if _, err := tx.ExecContext(ctx, "SET @auditoria_usuario_id = NULL"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE usuarios SET requiere_cambio_password = 0 WHERE nombre_usuario = ?", usuario); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO usuario_roles (usuario_id, rol_id, fecha_inicio_vigencia)
SELECT u.id, r.id, CURRENT_DATE FROM usuarios u JOIN roles r ON r.codigo = 'RRHH' WHERE u.nombre_usuario = ?`, usuario); err != nil {
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
		st, body := c.llamar("GET", "/readyz", nil)
		if st != http.StatusServiceUnavailable || buscar(body, "status") != "parametros_faltantes" {
			t.Fatalf("readyz: %d %v", st, body)
		}
		if st, _ := c.llamar("GET", "/metrics", nil); st != http.StatusNotFound {
			t.Fatalf("metrics sin token: %d", st)
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
		var r refsEmpleado
		var sem int64
		for q, dst := range map[string]*int64{
			`SELECT id FROM tipos_identificacion WHERE codigo = 'CEDULA_NACIONAL'`: &r.tipoIdent,
			`SELECT id FROM programas_presupuestarios WHERE codigo = 'PR-HUMO'`:    &r.programa,
			`SELECT id FROM regimenes_ccss WHERE codigo = 'SEM'`:                   &sem,
		} {
			if err := sqlDB.QueryRow(q).Scan(dst); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
		res, err := sqlDB.Exec(`INSERT INTO puestos (codigo, nombre, unidad_organizativa_id) SELECT 'PU-HUMO', 'Puesto de humo', id FROM unidades_organizativas WHERE codigo = 'UO-HUMO'`)
		if err != nil {
			t.Fatal(err)
		}
		r.puesto, _ = res.LastInsertId()
		if res, err = sqlDB.Exec(`INSERT INTO tipos_contrato (codigo, nombre) VALUES ('TC-HUMO', 'Tipo de humo')`); err != nil {
			t.Fatal(err)
		}
		r.tipoContrato, _ = res.LastInsertId()

		inexistente := int64(999999)
		if st, body := c.llamar("POST", rutaEmpleados, cuerpoEmpleado("HUMO0", "1-1111-0000", r, &inexistente)); st != http.StatusBadRequest {
			t.Fatalf("régimen inexistente: se esperaba 400, fue %d %v", st, body)
		}
		if st, body := c.llamar("POST", rutaEmpleados, cuerpoEmpleado("HUMO1", "1-1111-1111", r, &sem)); !esExito(st) {
			t.Fatalf("crear empleado: %d %v", st, body)
		}
		if st, body := c.llamar("POST", rutaEmpleados, cuerpoEmpleado("HUMO2", "1-1111-2222", r, nil)); !esExito(st) {
			t.Fatalf("crear empleado sin régimen: %d %v", st, body)
		}
		st, body := c.llamar("GET", rutaEmpleados, nil)
		if st != http.StatusOK {
			t.Fatalf("listar empleados: %d %v", st, body)
		}
		if b, _ := json.Marshal(body); !strings.Contains(string(b), "HUMO1") || !strings.Contains(string(b), "HUMO2") {
			t.Errorf("los empleados creados no aparecen en el listado: %s", b)
		}
		st, body = c.llamar("GET", rutaEmpleados+"?sin_regimen_ccss=true", nil)
		if b, _ := json.Marshal(body); st != http.StatusOK || strings.Contains(string(b), "HUMO1") || !strings.Contains(string(b), "HUMO2") {
			t.Errorf("sin_regimen_ccss: %d %s", st, b)
		}
	})
}
