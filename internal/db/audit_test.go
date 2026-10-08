package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"planillas-ccdr/internal/db/dto"
)

// --- driver falso: registra lo ejecutado en cada conexión física ---------

type fakeDriver struct {
	mu        sync.Mutex
	conns     []*fakeConn
	failReset bool // si es true, SET @auditoria_usuario_id = NULL falla
}

type fakeConn struct {
	d      *fakeDriver
	mu     sync.Mutex
	log    []string
	closed bool
}

func (d *fakeDriver) Open(string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	c := &fakeConn{d: d}
	d.conns = append(d.conns, c)
	return c, nil
}

func (c *fakeConn) record(s string) {
	c.mu.Lock()
	c.log = append(c.log, s)
	c.mu.Unlock()
}
func (c *fakeConn) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.log...)
}
func (c *fakeConn) isClosed() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.closed }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no soportado") }
func (c *fakeConn) Close() error                        { c.mu.Lock(); c.closed = true; c.mu.Unlock(); return nil }
func (c *fakeConn) Begin() (driver.Tx, error)           { c.record("BEGIN"); return fakeTx{c}, nil }
func (c *fakeConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if q == sqlResetAuditoriaUsuario {
		c.record("RESET")
		if c.d.failReset {
			return nil, errors.New("fallo simulado")
		}
		return driver.RowsAffected(0), nil
	}
	c.record(fmt.Sprintf("EXEC %s %v", q, args[0].Value))
	return driver.RowsAffected(0), nil
}

type fakeTx struct{ c *fakeConn }

func (t fakeTx) Commit() error   { t.c.record("COMMIT"); return nil }
func (t fakeTx) Rollback() error { t.c.record("ROLLBACK"); return nil }

var testDriverSeq int

func nuevaBD(t *testing.T) (*sql.DB, *fakeDriver) {
	t.Helper()
	fd := &fakeDriver{}
	testDriverSeq++
	name := fmt.Sprintf("fakeaudit%d", testDriverSeq)
	sql.Register(name, fd)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // fuerza reutilización: si no se descarta, es la misma conexión
	t.Cleanup(func() { _ = db.Close() })
	return db, fd
}

var setUsuario = "EXEC SET @auditoria_usuario_id = ? 7"

func TestResetEnExito(t *testing.T) {
	db, fd := nuevaBD(t)
	err := WithAuditContext(context.Background(), db, 7, func(context.Context, *dto.Queries) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"BEGIN", setUsuario, "COMMIT", "RESET"}
	if got := fd.conns[0].snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if fd.conns[0].isClosed() {
		t.Fatal("la conexión sana debe volver al pool, no cerrarse")
	}
}

func TestResetEnError(t *testing.T) {
	db, fd := nuevaBD(t)
	boom := errors.New("boom")
	err := WithAuditContext(context.Background(), db, 7, func(context.Context, *dto.Queries) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	want := []string{"BEGIN", setUsuario, "ROLLBACK", "RESET"}
	if got := fd.conns[0].snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResetEnPanicYRelanza(t *testing.T) {
	db, fd := nuevaBD(t)
	func() {
		defer func() {
			if r := recover(); r != "kaboom" {
				t.Fatalf("el panic debía re-lanzarse, recover = %v", r)
			}
		}()
		_ = WithAuditContext(context.Background(), db, 7, func(context.Context, *dto.Queries) error { panic("kaboom") })
	}()
	want := []string{"BEGIN", setUsuario, "ROLLBACK", "RESET"}
	if got := fd.conns[0].snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResetFallidoDescartaConexion(t *testing.T) {
	db, fd := nuevaBD(t)
	fd.failReset = true
	if err := WithAuditContext(context.Background(), db, 7, func(context.Context, *dto.Queries) error { return nil }); err != nil {
		t.Fatalf("un fallo del reset no debe alterar el resultado del commit: %v", err)
	}
	if !fd.conns[0].isClosed() {
		t.Fatal("la conexión con reset fallido debía descartarse")
	}
	fd.failReset = false
	if err := WithAuditContext(context.Background(), db, 8, func(context.Context, *dto.Queries) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(fd.conns) != 2 {
		t.Fatalf("debía abrirse una conexión nueva, conexiones = %d", len(fd.conns))
	}
}

func TestResetConCtxCancelado(t *testing.T) {
	db, fd := nuevaBD(t)
	ctx, cancel := context.WithCancel(context.Background())
	err := WithAuditContext(ctx, db, 7, func(context.Context, *dto.Queries) error {
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	// El pool debe seguir sano: una llamada nueva funciona y ninguna conexión
	// viva queda con la variable sin resetear.
	if err := WithAuditContext(context.Background(), db, 9, func(context.Context, *dto.Queries) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for i, c := range fd.conns {
		log := c.snapshot()
		if c.isClosed() {
			continue
		}
		if len(log) == 0 || log[len(log)-1] != "RESET" {
			t.Fatalf("conexión viva %d sin reset final: %v", i, log)
		}
	}
}

func TestUsuarioRequerido(t *testing.T) {
	db, _ := nuevaBD(t)
	for _, id := range []int64{0, -1} {
		if err := WithAuditContext(context.Background(), db, id, nil); !errors.Is(err, ErrNilUserID) {
			t.Fatalf("id=%d err=%v", id, err)
		}
	}
}