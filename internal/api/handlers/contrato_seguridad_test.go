package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/authz"
	"planillas-ccdr/internal/security"
	"planillas-ccdr/internal/services"
)

func TestContratoSeguro_EnmascaraSalarioSinPermisoSensible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(middleware.CtxKeyPayload, &security.Payload{Permisos: []string{"contratos.leer"}})

	body, err := json.Marshal(contratoSeguro(c, services.ContratoResponse{
		SalarioPactado: decimal.RequireFromString("123456.78"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "123456.78") || !strings.Contains(string(body), "***REDACTED***") {
		t.Fatalf("salario no enmascarado: %s", body)
	}

	c.Set(middleware.CtxKeyPayload, &security.Payload{Permisos: []string{authz.PermDatosPersonalesVerSensible}})
	body, err = json.Marshal(contratoSeguro(c, services.ContratoResponse{
		SalarioPactado: decimal.RequireFromString("123456.78"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "123456.78") {
		t.Fatalf("el permiso sensible debe conservar el salario: %s", body)
	}
}
