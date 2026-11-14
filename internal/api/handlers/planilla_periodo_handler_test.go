package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/api/middleware"
	"planillas-ccdr/internal/planilla"
)

func TestFallarCalculoConErrores(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/calcular", func(c *gin.Context) {
		fallar(c, &planilla.ErrCalculoConErrores{Errores: []planilla.EmpleadoErrorOutput{
			{EmpleadoID: 3, Codigo: "empleado_sin_contrato", Mensaje: "sin contrato"},
			{EmpleadoID: 9, Codigo: "calculo_invalido", Mensaje: "otro"},
		}})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/calcular", nil))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, se esperaba 422", w.Code)
	}
	var cuerpo struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				Empleados []planilla.EmpleadoErrorOutput `json:"empleados"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cuerpo); err != nil {
		t.Fatal(err)
	}
	if cuerpo.Error.Code != "calculo_con_errores" || len(cuerpo.Error.Details.Empleados) != 2 || cuerpo.Error.Details.Empleados[0].Codigo != "empleado_sin_contrato" {
		t.Fatalf("cuerpo inesperado: %s", w.Body.String())
	}
}
