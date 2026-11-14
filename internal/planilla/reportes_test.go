package planilla

import (
	"strings"
	"testing"
)

func TestNeutralizarCeldaCSV(t *testing.T) {
	casos := map[string]string{
		"":                  "",
		"Ana Mora":          "Ana Mora",
		"-1250.50":          "-1250.50",
		"1250.50":           "1250.50",
		"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")",
		"+SUM(A1)":          "'+SUM(A1)",
		"@cmd":              "'@cmd",
		"-2+3":              "'-2+3",
		"\tinicio":          "'\tinicio",
		"\rinicio":          "'\rinicio",
	}
	for entrada, esperado := range casos {
		if got := neutralizarCeldaCSV(entrada); got != esperado {
			t.Errorf("neutralizarCeldaCSV(%q) = %q, se esperaba %q", entrada, got, esperado)
		}
	}
}

func TestArmarCSVNeutralizaFormulasYSumaNegativos(t *testing.T) {
	nombre, monto := "=1+1", "-10.00"
	otro := "15.00"
	contenido, total, err := armarCSV([]string{"nombre", "monto"}, [][]*string{{&nombre, &monto}, {&nombre, &otro}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total.String() != "5" {
		t.Fatalf("total = %s, se esperaba 5", total)
	}
	if strings.Contains(string(contenido), "\r\n=1+1,") {
		t.Fatalf("celda con fórmula sin neutralizar:\n%s", contenido)
	}
	if !strings.Contains(string(contenido), "'=1+1,-10.00") {
		t.Fatalf("contenido inesperado:\n%s", contenido)
	}
}
