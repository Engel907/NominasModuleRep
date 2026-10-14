package authz

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Test de coherencia catálogo <-> migraciones (F12, test b).
//
// Reconstruye, leyendo db/migrations/*.up.sql en orden, el conjunto de códigos
// que quedan en la tabla `permisos` (INSERT agrega, DELETE FROM permisos quita)
// y lo compara con authz.Todos() en ambas direcciones:
//   - un código de authz sin sembrar  => las rutas darían 403 para siempre;
//   - un código sembrado fuera de authz => permiso huérfano que nadie controla.
//
// Reglas para escribir migraciones de permisos (lo que este parser entiende):
//   - sentencias "INSERT [IGNORE] INTO permisos ..." con los códigos como
//     literales 'dominio.accion' dentro de la misma sentencia;
//   - sentencias "DELETE FROM permisos WHERE codigo IN (...)" con literales;
//   - sin ';' dentro de strings ni de la sentencia salvo el terminador.

var (
	reComentarioLinea  = regexp.MustCompile(`(?m)--[^\n]*`)
	reComentarioBloque = regexp.MustCompile(`(?s)/\*.*?\*/`)
	reSentenciaPermiso = regexp.MustCompile("(?is)(INSERT\\s+(?:IGNORE\\s+)?INTO|DELETE\\s+FROM)\\s+`?permisos`?\\b[^;]*;")
	reCodigoLiteral    = regexp.MustCompile(`'([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*)'`)
	reHashArgon2       = regexp.MustCompile(`\$argon2(id|i|d)\$`)
)

const dirMigraciones = "../../db/migrations" // relativo a internal/authz

func archivosUp(t *testing.T) []string {
	t.Helper()
	archivos, err := filepath.Glob(filepath.Join(dirMigraciones, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(archivos) == 0 {
		t.Fatalf("no se encontraron migraciones *.up.sql en %s (el test no puede pasar en vacío)", dirMigraciones)
	}
	sort.Strings(archivos)
	return archivos
}

func sinComentarios(sql string) string {
	sql = reComentarioBloque.ReplaceAllString(sql, "")
	return reComentarioLinea.ReplaceAllString(sql, "")
}

// codigosSembrados devuelve el conjunto neto de códigos tras aplicar todas las migraciones.
func codigosSembrados(t *testing.T) map[string]string {
	t.Helper()
	neto := map[string]string{} // codigo -> archivo que lo sembró
	for _, archivo := range archivosUp(t) {
		crudo, err := os.ReadFile(archivo)
		if err != nil {
			t.Fatal(err)
		}
		sql := sinComentarios(string(crudo))
		for _, sent := range reSentenciaPermiso.FindAllString(sql, -1) {
			esBorrado := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sent)), "DELETE")
			for _, m := range reCodigoLiteral.FindAllStringSubmatch(sent, -1) {
				if esBorrado {
					delete(neto, m[1])
				} else {
					neto[m[1]] = filepath.Base(archivo)
				}
			}
		}
	}
	return neto
}

func TestMigraciones_SiembranExactamenteElCatalogoDeAuthz(t *testing.T) {
	sembrados := codigosSembrados(t)
	catalogo := map[string]bool{}
	for _, c := range Codigos() {
		catalogo[c] = true
	}

	var sinSembrar, huerfanos []string
	for c := range catalogo {
		if _, ok := sembrados[c]; !ok {
			sinSembrar = append(sinSembrar, c)
		}
	}
	for c, archivo := range sembrados {
		if !catalogo[c] {
			huerfanos = append(huerfanos, c+" (sembrado en "+archivo+")")
		}
	}
	sort.Strings(sinSembrar)
	sort.Strings(huerfanos)

	if len(sinSembrar) > 0 {
		t.Errorf("códigos de authz.Todos() que ninguna migración siembra:\n  %s", strings.Join(sinSembrar, "\n  "))
	}
	if len(huerfanos) > 0 {
		t.Errorf("códigos sembrados por migraciones que no están en authz.Todos():\n  %s", strings.Join(huerfanos, "\n  "))
	}
}

// Criterio de aceptación: ningún archivo del repositorio contiene un hash de contraseña.
func TestMigraciones_NoContienenHashesDePassword(t *testing.T) {
	for _, archivo := range archivosUp(t) {
		crudo, err := os.ReadFile(archivo)
		if err != nil {
			t.Fatal(err)
		}
		if reHashArgon2.Match(crudo) {
			t.Errorf("%s contiene un hash argon2: las credenciales no van en el repositorio (usar cmd/seedadmin)", filepath.Base(archivo))
		}
	}
}

func TestTodos_CatalogoWellFormed(t *testing.T) {
	modulosValidos := map[string]bool{
		"empleados": true, "asistencia": true, "parametros": true, "planilla": true,
		"contabilizacion": true, "integraciones": true, "auditoria": true,
	}
	vistos := map[string]bool{}
	for _, p := range Todos() {
		if !FormatoCodigo.MatchString(p.Codigo) {
			t.Errorf("código con formato inválido: %q", p.Codigo)
		}
		if len(p.Codigo) > 60 { // permisos.codigo VARCHAR(60)
			t.Errorf("código demasiado largo (%d > 60): %q", len(p.Codigo), p.Codigo)
		}
		if vistos[p.Codigo] {
			t.Errorf("código duplicado: %q", p.Codigo)
		}
		vistos[p.Codigo] = true
		if strings.TrimSpace(p.Nombre) == "" {
			t.Errorf("%s: nombre vacío", p.Codigo)
		}
		if !modulosValidos[p.Modulo] {
			t.Errorf("%s: módulo %q fuera del vocabulario de permisos.modulo", p.Codigo, p.Modulo)
		}
	}
}