package planilla

import (
	"context"
	"errors"
	"testing"

	"planillas-ccdr/internal/db/dto"
)

type ganchoFalso struct {
	nombre string
	err    error
	orden  *[]string
}

func (g ganchoFalso) AlAprobar(context.Context, *dto.Queries, dto.PeriodoCicloRow) error {
	*g.orden = append(*g.orden, g.nombre)
	return g.err
}

// GanchosAlAprobar corre en orden y corta en el primer error.
func TestGanchosAlAprobarOrdenYCorte(t *testing.T) {
	var orden []string
	fallo := errors.New("falla b")
	gs := GanchosAlAprobar{ganchoFalso{"a", nil, &orden}, nil, ganchoFalso{"b", fallo, &orden}, ganchoFalso{"c", nil, &orden}}
	if err := gs.AlAprobar(context.Background(), nil, dto.PeriodoCicloRow{}); !errors.Is(err, fallo) {
		t.Fatalf("error = %v", err)
	}
	if len(orden) != 2 || orden[0] != "a" || orden[1] != "b" {
		t.Fatalf("orden = %v, esperado [a b]", orden)
	}
}
