//go:build integration

package db_test

import (
	"context"
	"database/sql"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"planillas-ccdr/internal/contabilidad"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/planilla"
	"planillas-ccdr/internal/testutil"
)

// listaBlancaDinamicas: archivos (ruta relativa a la raíz, con /) que
// construyen SQL dinámicamente (concatenación, fmt, IN armados). Esas consultas
// no se pueden preparar tal cual; cada entrada DEBE explicar qué test
// específico ejecuta sus variantes reales. Una consulta dinámica en un archivo
// que no esté aquí hace fallar el test de contrato.
var listaBlancaDinamicas = map[string]string{
	"internal/db/dto/planilla_export.go":          "IN (?,?,...) con estados de período; variantes ejecutadas en TestVariantesDinamicasExport.",
	"internal/contabilidad/sql_adapters.go":       "IN (?,?,...) de líneas de asientos; ejecutada en TestVariantesDinamicasSQL.",
	"internal/db/dto/acumulados_liquidaciones.go": "Filtro IN de estados de período; ejecutado en TestVariantesDinamicasSQL.",
	"internal/db/dto/dashboard.go":                "Placeholders variables según estados cerrados; ejecutados en TestVariantesDinamicasSQL.",
	"internal/db/dto/planilla_ciclo.go":           "Filtros opcionales de período; ejecutados en TestVariantesDinamicasSQL.",
	"internal/db/dto/planilla_periodo.go":         "Columna de base elegida por método y query agregada; ejecutadas en TestVariantesDinamicasSQL.",
	"internal/db/dto/usuarios_admin.go":           "Filtros opcionales de usuario/auditoría y UPDATE selectivo; ejecutados en TestVariantesDinamicasSQL.",
}

type consulta struct {
	archivo  string // ruta relativa con /
	linea    int
	sql      string
	dinamica bool
}

var (
	verbosSQL    = map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true, "WITH": true}
	reEstructura = regexp.MustCompile(`(?i)\b(from|into|set|where|values)\b`)
	reFormato    = regexp.MustCompile(`%[sdvq]`)
)

func quitarComentarios(s string) string {
	for {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "--"):
			i := strings.IndexByte(s, '\n')
			if i < 0 {
				return ""
			}
			s = s[i+1:]
		case strings.HasPrefix(s, "/*"):
			i := strings.Index(s, "*/")
			if i < 0 {
				return ""
			}
			s = s[i+2:]
		default:
			return s
		}
	}
}

func pareceSQL(s string) bool {
	s = quitarComentarios(s)
	f := strings.Fields(s)
	if len(f) < 2 {
		return false
	}
	tok := strings.TrimLeft(f[0], "(")
	if !verbosSQL[strings.ToUpper(tok)] {
		return false
	}
	return tok == strings.ToUpper(tok) || reEstructura.MatchString(s)
}

func esDinamica(padre ast.Node, s string) bool {
	if reFormato.MatchString(s) {
		return true
	}
	switch p := padre.(type) {
	case *ast.BinaryExpr:
		return p.Op == token.ADD
	case *ast.CallExpr:
		if sel, ok := p.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fmt" {
				return true
			}
		}
	}
	return false
}

func extraerConsultas(t *testing.T, raiz string) []consulta {
	t.Helper()
	var out []consulta
	err := filepath.WalkDir(filepath.Join(raiz, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(raiz, path)
		rel = filepath.ToSlash(rel)

		var pila []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				pila = pila[:len(pila)-1]
				return true
			}
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, uerr := strconv.Unquote(lit.Value); uerr == nil && pareceSQL(s) && !esFragmentoSQL(pila, rel) {
					var padre ast.Node
					if len(pila) > 0 {
						padre = pila[len(pila)-1]
					}

					out = append(out, consulta{
						archivo:  rel,
						linea:    fset.Position(lit.Pos()).Line,
						sql:      s,
						dinamica: esDinamica(padre, s),
					})
				}
			}
			pila = append(pila, n)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("recorriendo internal/: %v", err)
	}
	return out
}

func esFragmentoSQL(pila []ast.Node, archivo string) bool {
	if archivo != "internal/db/dto/usuarios_admin.go" || len(pila) == 0 {
		return false
	}
	valor, ok := pila[len(pila)-1].(*ast.ValueSpec)
	if !ok {
		return false
	}
	for _, nombre := range valor.Names {
		if nombre.Name == "usuarioAdminSelect" {
			return true
		}
	}
	return false
}

// TestContratoSQL prepara CADA consulta SQL embebida en Go contra el esquema
// real. Un error de columna/tabla inexistente (u otro error de preparación)
// falla el test con archivo, línea y mensaje. No se detiene en el primero.
func TestContratoSQL(t *testing.T) {
	sqlDB := testutil.NuevaBD(t)
	consultas := extraerConsultas(t, testutil.RaizRepo(t))
	if len(consultas) == 0 {
		t.Fatal("no se extrajo ninguna consulta SQL: el extractor está roto")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var preparadas int
	var dinamicas []consulta
	for _, c := range consultas {
		if c.dinamica {
			dinamicas = append(dinamicas, c)
			continue
		}
		stmt, err := sqlDB.PrepareContext(ctx, c.sql)
		if err != nil {
			t.Errorf("%s:%d: %v\n    SQL: %s", c.archivo, c.linea, err, resumen(c.sql))
			continue
		}
		_ = stmt.Close()
		preparadas++
	}

	t.Logf("consultas: %d preparadas OK, %d dinámicas, %d total", preparadas, len(dinamicas), len(consultas))
	for _, d := range dinamicas {
		t.Logf("DINÁMICA %s:%d  %s", d.archivo, d.linea, resumen(d.sql))
		if _, ok := listaBlancaDinamicas[d.archivo]; !ok {
			t.Errorf("%s:%d: consulta dinámica fuera de la lista blanca: agrégala a listaBlancaDinamicas con su test específico", d.archivo, d.linea)
		}
	}
}

func resumen(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 140 {
		s = s[:140] + "…"
	}
	return s
}

func aStrings(es []planilla.EstadoPeriodo) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = string(e)
	}
	return out
}

// TestVariantesDinamicasExport ejecuta las variantes reales del IN armado en
// internal/db/dto/planilla_export.go y de planilla.ClausulaIN con los estados
// de estados.go. Un estado fuera del ENUM no da error en MySQL (solo compara),
// por eso el contrato del ENUM vive en planilla/estados_enum_test.go.
func TestVariantesDinamicasExport(t *testing.T) {
	sqlDB := testutil.NuevaBD(t)
	ctx := context.Background()
	q := dto.New(sqlDB)

	todos := []planilla.EstadoPeriodo{
		planilla.EstadoBorrador, planilla.EstadoCalculada, planilla.EstadoAprobada,
		planilla.EstadoPagada, planilla.EstadoContabilizada,
	}

	variantes := map[string][]planilla.EstadoPeriodo{
		"dispersion": planilla.EstadosParaDispersion(),
		"acumulados": planilla.EstadosParaAcumulados(),
		"cerrados":   planilla.EstadosCerrados(),
	}
	for n := 1; n <= len(todos); n++ {
		variantes["primeros_"+strconv.Itoa(n)] = todos[:n]
	}

	for nombre, estados := range variantes {
		t.Run(nombre, func(t *testing.T) {
			ss := aStrings(estados)
			if _, err := q.ObtenerDetallesPlanillaParaDispersion(ctx, -1, ss); err != nil {
				t.Errorf("ObtenerDetallesPlanillaParaDispersion: %v", err)
			}
			if _, err := q.ObtenerDatosBoletaPago(ctx, -1, -1, ss); err != nil && !errors.Is(err, sql.ErrNoRows) {
				t.Errorf("ObtenerDatosBoletaPago: %v", err)
			}
			in, args := planilla.ClausulaIN(estados)
			rows, err := sqlDB.QueryContext(ctx, "SELECT id FROM periodos_planilla WHERE estado "+in, args...)
			if err != nil {
				t.Errorf("ClausulaIN: %v", err)
				return
			}
			_ = rows.Close()
		})
	}

	t.Run("clausula_vacia", func(t *testing.T) {
		in, args := planilla.ClausulaIN(nil)
		rows, err := sqlDB.QueryContext(ctx, "SELECT id FROM periodos_planilla WHERE estado "+in, args...)
		if err != nil {
			t.Fatalf("IN (NULL): %v", err)
		}
		_ = rows.Close()
	})
}

func TestVariantesDinamicasSQL(t *testing.T) {
	sqlDB := testutil.NuevaBD(t)
	ctx := context.Background()
	q := dto.New(sqlDB)
	desde := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hasta := desde.AddDate(1, 0, 0)

	if _, err := q.SumarSalariosBrutosPeriodo(ctx, 1, desde, hasta); err != nil {
		t.Fatalf("SumarSalariosBrutosPeriodo: %v", err)
	}
	if _, err := q.ListSalariosBrutosUltimosPeriodos(ctx, 1, hasta, 5); err != nil {
		t.Fatalf("ListSalariosBrutosUltimosPeriodos: %v", err)
	}
	if _, err := q.ObtenerMetricasDashboard(ctx, hasta, []string{"aprobada", "pagada", "contabilizada"}); err != nil {
		t.Fatalf("ObtenerMetricasDashboard: %v", err)
	}
	filtroCiclo := dto.FiltroPeriodosCiclo{Estado: "borrador"}
	if _, err := q.ListarPeriodosCiclo(ctx, filtroCiclo, 10, 0); err != nil {
		t.Fatalf("ListarPeriodosCiclo: %v", err)
	}
	if _, err := q.ContarPeriodosCiclo(ctx, filtroCiclo); err != nil {
		t.Fatalf("ContarPeriodosCiclo: %v", err)
	}
	if _, err := q.SumarBaseAguinaldoPorEmpleado(ctx, desde, hasta); err != nil {
		t.Fatalf("SumarBaseAguinaldoPorEmpleado: %v", err)
	}
	if _, err := q.SumarBaseSalarioEscolarPorEmpleado(ctx, desde, hasta); err != nil {
		t.Fatalf("SumarBaseSalarioEscolarPorEmpleado: %v", err)
	}

	usuarioID, err := q.CrearUsuarioAdministracion(ctx, "contrato_sql", "Contrato SQL", "contrato-sql@example.test", sql.NullInt64{}, "hash-de-prueba")
	if err != nil {
		t.Fatalf("CrearUsuarioAdministracion: %v", err)
	}
	nombre := "Usuario contrato actualizado"
	correo := "contrato-actualizado@example.test"
	if _, err := q.ActualizarUsuarioAdministracion(ctx, usuarioID, &nombre, &correo, nil); err != nil {
		t.Fatalf("ActualizarUsuarioAdministracion: %v", err)
	}
	if _, _, err := q.ListarUsuariosAdministracion(ctx, dto.FiltroUsuarios{Texto: "contrato", Activo: boolPuntero(true), Rol: "admin", Limit: 10}); err != nil {
		t.Fatalf("ListarUsuariosAdministracion: %v", err)
	}
	if _, _, err := q.ListarAuditoria(ctx, dto.FiltroAuditoria{Tabla: "usuarios", Limit: 10}); err != nil {
		t.Fatalf("ListarAuditoria: %v", err)
	}

	var periodoTipoID, tipoAsientoID int64
	if err := sqlDB.QueryRowContext(ctx, `SELECT id FROM tipos_planilla LIMIT 1`).Scan(&periodoTipoID); errors.Is(err, sql.ErrNoRows) {
		res, err := sqlDB.ExecContext(ctx, `INSERT INTO tipos_planilla (codigo, nombre) VALUES (?, ?)`, "contrato_sql", "Tipo contrato SQL")
		if err != nil {
			t.Fatalf("creando tipo de planilla de prueba: %v", err)
		}
		periodoTipoID, err = res.LastInsertId()
		if err != nil {
			t.Fatalf("leyendo id de tipo de planilla de prueba: %v", err)
		}
	} else if err != nil {
		t.Fatalf("buscando tipo de planilla de prueba: %v", err)
	}
	if err := sqlDB.QueryRowContext(ctx, `SELECT id FROM tipos_asiento LIMIT 1`).Scan(&tipoAsientoID); err != nil {
		t.Fatalf("buscando tipo de asiento de prueba: %v", err)
	}
	res, err := sqlDB.ExecContext(ctx, `INSERT INTO periodos_planilla (tipo_planilla_id, fecha_inicio, fecha_fin) VALUES (?, ?, ?)`, periodoTipoID, desde, hasta)
	if err != nil {
		t.Fatalf("creando período para contrato de contabilidad: %v", err)
	}
	periodoID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("leyendo id de período de prueba: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO asientos_contables (periodo_planilla_id, tipo_asiento_id, numero_asiento, fecha_generacion) VALUES (?, ?, ?, ?)`, periodoID, tipoAsientoID, "CONTRATO-1", hasta); err != nil {
		t.Fatalf("creando asiento para contrato de contabilidad: %v", err)
	}
	if _, err := contabilidad.NewLectorSQL(sqlDB).ListarAsientos(ctx, contabilidad.FiltroAsientos{PeriodoID: &periodoID}); err != nil {
		t.Fatalf("ListarAsientos: %v", err)
	}
}

func boolPuntero(valor bool) *bool {
	return &valor
}
