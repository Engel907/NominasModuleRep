package asistencia

import (
	"context"
	"errors"
	"testing"

	"planillas-ccdr/internal/db/dto"
)

func TestSolicitarVacaciones_EmpleadoNoPuedeSolicitarPorTercero(t *testing.T) {
	tx := &fakeVacTx{}
	_, err := nuevoSvc(&fakeVacQueries{}, tx).SolicitarVacaciones(context.Background(), SolicitarVacacionesInput{
		EmpleadoID: 2, EmpleadoIDUsuario: 1, CreadoPor: 9,
		PuedeSolicitarTerceros: false,
		FechaInicio:            fecha(2026, 10, 5), FechaFin: fecha(2026, 10, 5),
	})
	if !errors.Is(err, ErrVacSolicitarTercerosNoPermitido) {
		t.Fatalf("error = %v; quería ErrVacSolicitarTercerosNoPermitido", err)
	}
	if tx.creadas != 0 {
		t.Fatal("la solicitud de un tercero no debe persistirse")
	}
}

func TestResolverSolicitud_NoPermiteAprobadorSolicitante(t *testing.T) {
	tx := &fakeVacTx{solicitud: dto.VacacionesSolicitud{ID: 3, EmpleadoID: 7, Estado: "pendiente"}}
	_, err := nuevoSvc(&fakeVacQueries{}, tx).ResolverSolicitud(context.Background(), ResolverSolicitudInput{
		SolicitudID: 3, Aprobar: true, AprobadoPor: 11,
		AprobadorEmpleadoID: 7,
	})
	if !errors.Is(err, ErrVacSolicitudPropiaNoResoluble) {
		t.Fatalf("error = %v; quería ErrVacSolicitudPropiaNoResoluble", err)
	}
}
