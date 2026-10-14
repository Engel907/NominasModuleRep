//go:build integration

package planilla_test

import (
	"context"
	"database/sql"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"planillas-ccdr/internal/testutil"
)

var reValorEnum = regexp.MustCompile(`'((?:[^']|'')*)'`)

// enumReal lee de information_schema los valores del ENUM de tabla.columna.
func enumReal(t *testing.T, db *sql.DB, tabla, columna string) []string {
	t.Helper()
	var tipo string
	err := db.QueryRowContext(context.Background(), `
SELECT COLUMN_TYPE
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, tabla, columna).Scan(&tipo)
	if err != nil {
		t.Fatalf("leyendo COLUMN_TYPE de %s.%s: %v", tabla, columna, err)
	}
	if !strings.HasPrefix(strings.ToLower(tipo), "enum(") {
		t.Fatalf("%s.%s no es ENUM: %s", tabla, columna, tipo)
	}
	var out []string
	for _, m := range reValorEnum.FindAllStringSubmatch(tipo, -1) {
		out = append(out, strings.ReplaceAll(m[1], "''", "'"))
	}
	return out
}

// constantesGo devuelve los valores string de las constantes de `archivo`
// cuyo nombre empieza con prefijo (se lee con go/ast: una constante nueva se
// detecta sin tocar el test).
func constantesGo(t *testing.T, archivo, prefijo string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), archivo, nil, 0)
	if err != nil {
		t.Fatalf("parseando %s: %v", archivo, err)
	}
	var out []string
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, sp := range gd.Specs {
			vs := sp.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if !strings.HasPrefix(n.Name, prefijo) || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				v, _ := strconv.Unquote(lit.Value)
				out = append(out, v)
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s: no se encontraron constantes con prefijo %q", archivo, prefijo)
	}
	return out
}

func compararConjuntos(t *testing.T, etiqueta string, bd, goVals []string) {
	t.Helper()
	b := map[string]bool{}
	g := map[string]bool{}
	for _, v := range bd {
		b[v] = true
	}
	for _, v := range goVals {
		g[v] = true
	}
	var soloBD, soloGo []string
	for v := range b {
		if !g[v] {
			soloBD = append(soloBD, v)
		}
	}
	for v := range g {
		if !b[v] {
			soloGo = append(soloGo, v)
		}
	}
	sort.Strings(soloBD)
	sort.Strings(soloGo)
	if len(soloBD)+len(soloGo) > 0 {
		t.Errorf("%s: ENUM de la BD y constantes de Go difieren\n    solo en la BD: %v\n    solo en Go:    %v", etiqueta, soloBD, soloGo)
	}
}

func TestEnumsEstadoCoincidenConConstantes(t *testing.T) {
	bd := testutil.NuevaBD(t)

	t.Run("periodos_planilla.estado", func(t *testing.T) {
		compararConjuntos(t, "periodos_planilla.estado",
			enumReal(t, bd, "periodos_planilla", "estado"),
			constantesGo(t, "estados.go", "Estado"))
	})
	t.Run("prestamos.estado", func(t *testing.T) {
		compararConjuntos(t, "prestamos.estado",
			enumReal(t, bd, "prestamos", "estado"),
			constantesGo(t, "../prestamos/estados.go", "EstadoPrestamo"))
	})
	t.Run("embargos.estado", func(t *testing.T) {
		compararConjuntos(t, "embargos.estado",
			enumReal(t, bd, "embargos", "estado"),
			constantesGo(t, "../prestamos/estados.go", "EstadoEmbargo"))
	})
}