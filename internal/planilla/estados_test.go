package planilla

import (
	"reflect"
	"testing"
)

func TestValido(t *testing.T) {
	for _, e := range []EstadoPeriodo{EstadoBorrador, EstadoCalculada, EstadoAprobada, EstadoPagada, EstadoContabilizada} {
		if !e.Valido() {
			t.Errorf("%q debería ser válido", e)
		}
	}
	for _, e := range []EstadoPeriodo{"", "abierta", "cerrada", "APROBADA"} {
		if e.Valido() {
			t.Errorf("%q no debería ser válido", e)
		}
	}
}

func TestConjuntos(t *testing.T) {
	if got := EstadosParaDispersion(); !reflect.DeepEqual(got, []EstadoPeriodo{"aprobada", "pagada"}) {
		t.Errorf("dispersion = %v", got)
	}
	if got := EstadosParaAcumulados(); !reflect.DeepEqual(got, []EstadoPeriodo{"aprobada", "pagada", "contabilizada"}) {
		t.Errorf("acumulados = %v", got)
	}
	if got := EstadosCerrados(); !reflect.DeepEqual(got, []EstadoPeriodo{"pagada", "contabilizada"}) {
		t.Errorf("cerrados = %v", got)
	}
	EstadosCerrados()[0] = "x" // mutar la copia no afecta llamadas futuras
	if EstadosCerrados()[0] != EstadoPagada {
		t.Error("los conjuntos no deben compartir estado")
	}
}

func TestClausulaIN(t *testing.T) {
	in, args := ClausulaIN(EstadosParaAcumulados())
	if in != "IN (?,?,?)" || !reflect.DeepEqual(args, []interface{}{"aprobada", "pagada", "contabilizada"}) {
		t.Errorf("got %q %v", in, args)
	}
	if in, args := ClausulaIN(nil); in != "IN (NULL)" || len(args) != 0 {
		t.Errorf("vacío: %q %v", in, args)
	}
}
