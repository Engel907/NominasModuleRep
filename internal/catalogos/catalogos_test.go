package catalogos

import (
	"testing"
)

func str(s string) *string { return &s }

func TestRegistroSinSolaparConParametrosVersionados(t *testing.T) {
	versionados := []string{"concepto_salarial_formula", "orden_calculo_concepto", "regimen_concepto_aplicable",
		"mapeo_concepto_cuenta", "parametros_generales", "feriados_catalogo"}
	r := Registro()
	for _, n := range versionados {
		if _, ok := r[n]; ok {
			t.Errorf("%s no debe administrarse como catálogo simple", n)
		}
	}
	if len(r) == 0 {
		t.Fatal("registro vacío")
	}
}

func TestReglasDeCatalogos(t *testing.T) {
	r := Registro()
	casos := []struct {
		catalogo string
		fila     Fila
		ok       bool
	}{
		{"conceptos_salariales", Fila{"codigo": str("BONO_X")}, true},
		{"conceptos_salariales", Fila{"codigo": str("bono")}, false},
		{"tipos_planilla", Fila{"codigo": str("SEMANAL"), "clase": str("regular"), "fraccion_mensual": str("0.2308")}, true},
		{"tipos_planilla", Fila{"codigo": str("SEMANAL"), "clase": str("regular"), "fraccion_mensual": nil}, false},
		{"tipos_planilla", Fila{"codigo": str("X"), "clase": str("regular"), "fraccion_mensual": str("1.5")}, false},
		{"tipos_planilla", Fila{"codigo": str("AGUI"), "clase": str("aguinaldo"), "fraccion_mensual": nil}, true},
		{"formatos_exportacion_columnas", Fila{"orden": str("1"), "columna_origen": str("salario_bruto")}, true},
		{"formatos_exportacion_columnas", Fila{"orden": str("1"), "columna_origen": str("x; DROP TABLE y")}, false},
		{"formatos_exportacion_columnas", Fila{"orden": str("0"), "columna_origen": str("neto")}, false},
		{"cuentas_contables", Fila{"nivel": str("0")}, false},
	}
	for _, c := range casos {
		err := r[c.catalogo].Validar(c.fila)
		if c.ok != (err == nil) {
			t.Errorf("%s %v: ok esperado %t, err %v", c.catalogo, c.fila, c.ok, err)
		}
	}
}

func TestEnumeracion(t *testing.T) {
	c, _ := Registro()["conceptos_salariales"].campo("naturaleza")
	if err := validarEnumeracion(c, str("ingreso")); err != nil {
		t.Fatal(err)
	}
	if err := validarEnumeracion(c, str("bonificacion")); err == nil {
		t.Fatal("valor fuera del ENUM aceptado")
	}
}

func TestIgualesComparaDecimalesNormalizados(t *testing.T) {
	if !iguales(str("0.5000"), str("0.5")) || iguales(str("1"), str("2")) || !iguales(nil, nil) || iguales(nil, str("1")) {
		t.Fatal("comparación incorrecta")
	}
}
