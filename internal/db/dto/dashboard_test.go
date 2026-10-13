package dto

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// Driver mínimo que registra la consulta y los argumentos y devuelve una fila
// fija. No ejecuta SQL: sirve para verificar que placeholders y argumentos
// calzan y que el SQL ya no referencia columnas/estados inexistentes.
type capturaDriver struct {
	mu    sync.Mutex
	query string
	args  []driver.NamedValue
}

func (d *capturaDriver) Open(string) (driver.Conn, error) { return &capturaConn{d: d}, nil }

type capturaConn struct{ d *capturaDriver }

func (c *capturaConn) Prepare(q string) (driver.Stmt, error) { return &capturaStmt{c: c, q: q}, nil }
func (c *capturaConn) Close() error                          { return nil }
func (c *capturaConn) Begin() (driver.Tx, error)             { return nil, errors.New("no soportado") }

type capturaStmt struct {
	c *capturaConn
	q string
}

func (s *capturaStmt) Close() error  { return nil }
func (s *capturaStmt) NumInput() int { return -1 }
func (s *capturaStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("no soportado")
}
func (s *capturaStmt) Query([]driver.Value) (driver.Rows, error) {
	return nil, errors.New("usar QueryContext")
}
func (s *capturaStmt) QueryContext(_ context.Context, a []driver.NamedValue) (driver.Rows, error) {
	s.c.d.mu.Lock()
	defer s.c.d.mu.Unlock()
	s.c.d.query, s.c.d.args = s.q, a
	return &filaFija{}, nil
}

type filaFija struct{ leida bool }

func (r *filaFija) Columns() []string {
	return []string{"a", "b", "c", "d", "e"}
}
func (r *filaFija) Close() error { return nil }
func (r *filaFija) Next(dest []driver.Value) error {
	if r.leida {
		return io.EOF
	}
	r.leida = true
	dest[0], dest[1], dest[2], dest[3], dest[4] = int64(12), int64(9), "1500000.50", int64(3), int64(2)
	return nil
}

func abrirCaptura(t *testing.T) (*Queries, *capturaDriver) {
	t.Helper()
	d := &capturaDriver{}
	nombre := "captura_" + t.Name()
	sql.Register(nombre, d)
	db, err := sql.Open(nombre, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db), d
}

func TestObtenerMetricasDashboard_SQLYArgumentos(t *testing.T) {
	q, cap := abrirCaptura(t)
	// 23:30 en UTC-6 = día siguiente en UTC: la fecha enviada debe ser la del
	// reloj local del llamador, no la convertida.
	cr := time.FixedZone("CR", -6*3600)
	fecha := time.Date(2026, 10, 13, 23, 30, 0, 0, cr)

	r, err := q.ObtenerMetricasDashboard(context.Background(), fecha, []string{"pagada", "contabilizada"})
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalEmpleadosActivos != 12 || !r.UltimoPeriodoCerradoID.Valid || r.UltimoPeriodoCerradoID.Int64 != 9 ||
		r.CostoUltimaPlanillaCerrada.String() != "1500000.5" || r.SolicitudesVacacionesPendientes != 3 || r.IncapacidadesVigentes != 2 {
		t.Errorf("fila mal mapeada: %+v", r)
	}

	// Un '?' por argumento: 2 usos × 2 estados + 2 fechas = 6.
	if n := strings.Count(cap.query, "?"); n != len(cap.args) || n != 6 {
		t.Fatalf("placeholders=%d args=%d (esperaba 6 y 6)", n, len(cap.args))
	}
	want := []any{"pagada", "contabilizada", "pagada", "contabilizada", "2026-10-13", "2026-10-13"}
	for i, w := range want {
		if cap.args[i].Value != w {
			t.Errorf("arg[%d]=%v, esperaba %v", i, cap.args[i].Value, w)
		}
	}
	for _, prohibido := range []string{"calculo_finalizado_en", "'cerrado'", "pde.periodo_id ", "solicitudes_vacaciones "} {
		if strings.Contains(cap.query, prohibido) {
			t.Errorf("el SQL aún contiene %q", prohibido)
		}
	}
	for _, requerido := range []string{"pde.periodo_planilla_id", "ORDER BY pp.fecha_fin DESC, pp.id DESC", "vacaciones_solicitudes", "pp.estado IN (?,?)"} {
		if !strings.Contains(cap.query, requerido) {
			t.Errorf("el SQL no contiene %q", requerido)
		}
	}
}

func TestObtenerMetricasDashboard_SinEstadosFalla(t *testing.T) {
	q, cap := abrirCaptura(t)
	_, err := q.ObtenerMetricasDashboard(context.Background(), time.Now(), nil)
	if !errors.Is(err, ErrSinEstadosCerrados) {
		t.Fatalf("err=%v", err)
	}
	if cap.query != "" {
		t.Error("no debía ejecutar ninguna consulta")
	}
}
