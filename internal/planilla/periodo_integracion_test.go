package planilla

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/db/dto"
)

type calculadorNulo struct{}

func (calculadorNulo) Calcular(context.Context, *dto.Queries, dto.PeriodoCicloRow, int64) error {
	return nil
}
func (calculadorNulo) EliminarDetalle(context.Context, *dto.Queries, int64) error { return nil }

type fixtureCiclo struct {
	db                         *sql.DB
	svc                        *PeriodoService
	tipoID, empleadoID, progID int64
	u                          [3]int64
}

func nuevoFixtureCiclo(t *testing.T) fixtureCiclo {
	t.Helper()
	dsn := os.Getenv("PLANILLAS_TEST_DSN")
	if dsn == "" {
		t.Skip("PLANILLAS_TEST_DSN no definido")
	}
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	f := fixtureCiclo{db: sqlDB}
	uno := func(q string) int64 {
		var id int64
		if err := sqlDB.QueryRow(q).Scan(&id); err != nil {
			t.Skipf("falta dato de prueba (%s): %v", q, err)
		}
		return id
	}
	f.tipoID = uno(`SELECT id FROM tipos_planilla ORDER BY id LIMIT 1`)
	f.empleadoID = uno(`SELECT id FROM empleados ORDER BY id LIMIT 1`)
	f.progID = uno(`SELECT id FROM programas_presupuestarios ORDER BY id LIMIT 1`)
	rows, err := sqlDB.Query(`SELECT id FROM usuarios ORDER BY id LIMIT 3`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() && i < 3 {
		if err := rows.Scan(&f.u[i]); err != nil {
			t.Fatal(err)
		}
		i++
	}
	if i < 3 {
		t.Skip("se necesitan al menos 3 usuarios")
	}
	f.svc = NewPeriodoService(sqlDB, PeriodoGanchos{Calculador: calculadorNulo{}, Eliminador: calculadorNulo{}})
	return f
}

func fechasAleatorias(dia1, dia2 int) (time.Time, time.Time) {
	y := 2200 + rand.Intn(6000)
	return time.Date(y, 1, dia1, 0, 0, 0, 0, time.UTC), time.Date(y, 1, dia2, 0, 0, 0, 0, time.UTC)
}

// periodoCalculado crea un período, le inserta una línea (en borrador, antes
// de calcular) y lo calcula con u[0].
func (f fixtureCiclo) periodoCalculado(t *testing.T) dto.PeriodoCicloRow {
	t.Helper()
	ctx := context.Background()
	ini, fin := fechasAleatorias(1, 15)
	p, err := f.svc.Crear(ctx, f.u[0], f.tipoID, ini, fin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO planilla_detalle_empleado
		(periodo_planilla_id, empleado_id, programa_presupuestario_id, salario_base_periodo) VALUES (?,?,?,0)`,
		p.ID, f.empleadoID, f.progID); err != nil {
		t.Fatal(err)
	}
	p, err = f.svc.Calcular(ctx, f.u[0], p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Estado != "calculada" || p.PreparadoPorUsuarioID == nil || *p.PreparadoPorUsuarioID != f.u[0] {
		t.Fatalf("período mal calculado: %+v", p)
	}
	return p
}

func esTrigger(err error, prefijo string) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1644 && len(me.Message) >= len(prefijo) && me.Message[:len(prefijo)] == prefijo
}

func TestIntegracionTraslape(t *testing.T) {
	f := nuevoFixtureCiclo(t)
	ctx := context.Background()
	ini, fin := fechasAleatorias(1, 15)
	if _, err := f.svc.Crear(ctx, f.u[0], f.tipoID, ini, fin); err != nil {
		t.Fatal(err)
	}
	ini2, fin2 := ini.AddDate(0, 0, 9), ini.AddDate(0, 0, 19)
	if _, err := f.svc.Crear(ctx, f.u[0], f.tipoID, ini2, fin2); !errors.Is(err, ErrPeriodoTraslapado) {
		t.Fatalf("traslape parcial: err=%v", err)
	}
	if _, err := f.svc.Crear(ctx, f.u[0], f.tipoID, fin.AddDate(0, 0, 1), fin.AddDate(0, 0, 15)); err != nil {
		t.Fatalf("período contiguo: %v", err)
	}
	if _, err := f.svc.Crear(ctx, f.u[0], f.tipoID, fin, ini); !errors.Is(err, ErrFechasInvalidas) {
		t.Fatalf("fechas invertidas: err=%v", err)
	}
}

func TestIntegracionSegregacion(t *testing.T) {
	f := nuevoFixtureCiclo(t)
	ctx := context.Background()
	p := f.periodoCalculado(t)

	if _, err := f.svc.Aprobar(ctx, f.u[0], p.ID); !errors.Is(err, ErrSegregacionFunciones) {
		t.Fatalf("aprobar quien preparó: err=%v", err)
	}
	if _, err := f.svc.Aprobar(ctx, f.u[1], p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Pagar(ctx, f.u[1], p.ID); !errors.Is(err, ErrSegregacionFunciones) {
		t.Fatalf("pagar quien aprobó: err=%v", err)
	}
	if _, err := f.svc.Pagar(ctx, f.u[0], p.ID); !errors.Is(err, ErrSegregacionFunciones) {
		t.Fatalf("pagar quien preparó: err=%v", err)
	}
	pagada, err := f.svc.Pagar(ctx, f.u[2], p.ID)
	if err != nil || pagada.Estado != "pagada" {
		t.Fatalf("pagar con tercero: %v %+v", err, pagada)
	}
	var estadoLinea string
	if err := f.db.QueryRow(`SELECT estado FROM planilla_detalle_empleado WHERE periodo_planilla_id=?`, p.ID).Scan(&estadoLinea); err != nil || estadoLinea != "pagado" {
		t.Fatalf("línea no sincronizada: %q %v", estadoLinea, err)
	}
}

func TestIntegracionTriggerSegregacionDirecta(t *testing.T) {
	f := nuevoFixtureCiclo(t)
	p := f.periodoCalculado(t)
	_, err := f.db.Exec(`UPDATE periodos_planilla SET estado='aprobada', aprobado_por_usuario_id=?, fecha_aprobacion=NOW() WHERE id=?`, f.u[0], p.ID)
	if !esTrigger(err, "segregacion_funciones") {
		t.Fatalf("esperaba segregacion_funciones, got %v", err)
	}
}

func TestIntegracionDobleAprobacionConcurrente(t *testing.T) {
	f := nuevoFixtureCiclo(t)
	p := f.periodoCalculado(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, u := range []int64{f.u[1], f.u[2]} {
		wg.Add(1)
		go func(i int, u int64) {
			defer wg.Done()
			_, errs[i] = f.svc.Aprobar(context.Background(), u, p.ID)
		}(i, u)
	}
	wg.Wait()
	ok, conflicto := 0, 0
	for _, e := range errs {
		switch {
		case e == nil:
			ok++
		case errors.Is(e, ErrEstadoInvalido):
			conflicto++
		default:
			t.Errorf("error inesperado: %v", e)
		}
	}
	if ok != 1 || conflicto != 1 {
		t.Fatalf("ok=%d conflicto=%d, esperaba 1 y 1", ok, conflicto)
	}
}

func TestIntegracionSinRetrocesoNiMutacion(t *testing.T) {
	f := nuevoFixtureCiclo(t)
	ctx := context.Background()
	p := f.periodoCalculado(t)
	if _, err := f.svc.Aprobar(ctx, f.u[1], p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Descartar(ctx, f.u[0], p.ID); !errors.Is(err, ErrEstadoInvalido) {
		t.Fatalf("descartar aprobada: err=%v", err)
	}
	if _, err := f.db.Exec(`UPDATE periodos_planilla SET estado='calculada' WHERE id=?`, p.ID); !esTrigger(err, "transicion_invalida") {
		t.Fatalf("retroceso directo: err=%v", err)
	}
	if _, err := f.db.Exec(`UPDATE planilla_detalle_empleado SET salario_base_periodo=1 WHERE periodo_planilla_id=?`, p.ID); !esTrigger(err, "periodo_inmutable") {
		t.Fatalf("modificar línea aprobada: err=%v", err)
	}
	if _, err := f.db.Exec(`DELETE FROM planilla_detalle_empleado WHERE periodo_planilla_id=?`, p.ID); !esTrigger(err, "periodo_inmutable") {
		t.Fatalf("borrar línea aprobada: err=%v", err)
	}
	if _, err := f.db.Exec(`DELETE FROM periodos_planilla WHERE id=?`, p.ID); !esTrigger(err, "periodo_inmutable") {
		t.Fatalf("borrar período aprobado: err=%v", err)
	}
}
