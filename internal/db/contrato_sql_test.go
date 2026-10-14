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
	"internal/db/dto/planilla_export.go": "IN (?,?,...) con estados de período; variantes ejecutadas en TestVariantesDinamicasExport.",
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
				if s, uerr := strconv.Unquote(lit.Value); uerr == nil && pareceSQL(s) {
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