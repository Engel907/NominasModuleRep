package planilla

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestTransicionPermitida(t *testing.T) {
	permitidas := map[[2]EstadoPeriodo]bool{
		{EstadoBorrador, EstadoCalculada}:   true,
		{EstadoCalculada, EstadoBorrador}:   true,
		{EstadoCalculada, EstadoAprobada}:   true,
		{EstadoAprobada, EstadoPagada}:      true,
		{EstadoPagada, EstadoContabilizada}: true,
	}
	todos := []EstadoPeriodo{EstadoBorrador, EstadoCalculada, EstadoAprobada, EstadoPagada, EstadoContabilizada}
	for _, d := range todos {
		for _, h := range todos {
			if got, want := TransicionPermitida(d, h), permitidas[[2]EstadoPeriodo{d, h}]; got != want {
				t.Errorf("TransicionPermitida(%s,%s)=%v, want %v", d, h, got, want)
			}
		}
	}
}

func idp(v int64) *int64 { return &v }

func TestVerificarSegregacion(t *testing.T) {
	p := Participantes{Preparo: idp(1), Calculo: idp(2), Aprobo: idp(3), Pago: idp(4)}
	casos := []struct {
		nombre      string
		destino     EstadoPeriodo
		usuario     int64
		quiereError bool
	}{
		{"aprueba quien preparó", EstadoAprobada, 1, true},
		{"aprueba quien calculó", EstadoAprobada, 2, true},
		{"aprueba un tercero", EstadoAprobada, 9, false},
		{"paga quien preparó", EstadoPagada, 1, true},
		{"paga quien calculó", EstadoPagada, 2, true},
		{"paga quien aprobó", EstadoPagada, 3, true},
		{"paga un tercero", EstadoPagada, 9, false},
		{"contabiliza quien pagó", EstadoContabilizada, 4, true},
		{"contabiliza otro", EstadoContabilizada, 1, false},
		{"destino sin regla", EstadoCalculada, 1, false},
	}
	for _, c := range casos {
		err := VerificarSegregacion(c.destino, c.usuario, p)
		if c.quiereError != errors.Is(err, ErrSegregacionFunciones) {
			t.Errorf("%s: err=%v, quiereError=%v", c.nombre, err, c.quiereError)
		}
	}
	// Sin participantes previos nadie queda bloqueado.
	if err := VerificarSegregacion(EstadoAprobada, 1, Participantes{}); err != nil {
		t.Errorf("sin participantes: %v", err)
	}
}

func TestErrorCicloIs(t *testing.T) {
	if !errors.Is(estadoInvalido(EstadoCalculada, "borrador"), ErrEstadoInvalido) {
		t.Error("estadoInvalido debe cumplir errors.Is(ErrEstadoInvalido)")
	}
	if errors.Is(ErrPeriodoTraslapado, ErrEstadoInvalido) {
		t.Error("errores distintos no deben coincidir")
	}
}

func TestTraducirErrorBD(t *testing.T) {
	casos := map[string]error{
		"segregacion_funciones: x": ErrSegregacionFunciones,
		"transicion_invalida: x":   ErrTransicionInvalida,
		"periodo_inmutable: x":     ErrPeriodoInmutable,
		"detalle_descuadrado: x":   ErrDetalleDescuadrado,
	}
	for msg, want := range casos {
		if got := traducirErrorBD(&mysql.MySQLError{Number: 1644, Message: msg}); !errors.Is(got, want) {
			t.Errorf("%q -> %v, want %v", msg, got, want)
		}
	}
	otro := &mysql.MySQLError{Number: 1062, Message: "dup"}
	if got := traducirErrorBD(otro); got != error(otro) {
		t.Errorf("otros errores pasan sin cambio, got %v", got)
	}
}

func TestErrDetalleDescuadrado(t *testing.T) {
	// Un SIGNAL con el prefijo detalle_descuadrado se traduce al error de dominio.
	err := traducirErrorBD(&mysql.MySQLError{Number: 1644, Message: "detalle_descuadrado: el detalle no cuadra"})
	if !errors.Is(err, ErrDetalleDescuadrado) {
		t.Fatalf("errors.Is(err, ErrDetalleDescuadrado)=false, err=%v", err)
	}
	var ec *ErrorCiclo
	if !errors.As(err, &ec) || ec.HTTP != 409 || ec.Codigo != "detalle_descuadrado" {
		t.Errorf("esperaba ErrorCiclo 409 detalle_descuadrado, got %+v", ec)
	}
	// No se confunde con otros errores de dominio del ciclo.
	if errors.Is(err, ErrPeriodoInmutable) || errors.Is(err, ErrPeriodoSinDetalle) {
		t.Error("detalle_descuadrado no debe coincidir con otros errores de ciclo")
	}
	// Con otro número de error MySQL el mensaje no se traduce.
	otro := &mysql.MySQLError{Number: 1062, Message: "detalle_descuadrado: x"}
	if got := traducirErrorBD(otro); errors.Is(got, ErrDetalleDescuadrado) {
		t.Error("solo el error 1644 (SIGNAL) debe traducirse")
	}
}