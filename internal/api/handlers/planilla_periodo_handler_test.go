package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"planillas-ccdr/internal/planilla"
)

// ErrCalculoConErrores se traduce a 422 con la lista completa de empleados.
func TestFallarCalculoConErrores(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	fallar(c, &planilla.ErrCalculoConErrores{Errores: []planilla.EmpleadoErrorOutput{
		{EmpleadoID: 3, Codigo: "empleado_sin_contrato", Mensaje: "sin contrato"},
		{EmpleadoID: 9, Codigo: "calculo_invalido", Mensaje: "otro"},
	}})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, se esperaba 422", w.Code)
	}
	var cuerpo struct {
		Error struct {
			Codigo    string                         `json:"codigo"`
			Empleados []planilla.EmpleadoErrorOutput `json:"empleados"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cuerpo); err != nil {
		t.Fatal(err)
	}
	if cuerpo.Error.Codigo != "calculo_con_errores" || len(cuerpo.Error.Empleados) != 2 || cuerpo.Error.Empleados[0].Codigo != "empleado_sin_contrato" {
		t.Fatalf("cuerpo inesperado: %s", w.Body.String())
	}
}
