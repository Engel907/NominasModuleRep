//go:build integration

package db

import (
	"strings"
	"testing"

	"planillas-ccdr/internal/testutil"
)

var tablasSinAuditoria = map[string]string{
	"auditoria_cambios":      "es la propia bitácora; solo inserción",
	"registro_accesos_datos": "bitácora de accesos de la Ley 8968; solo inserción",
	"schema_migrations":      "control interno de golang-migrate",
	"refresh_tokens":         "credenciales efímeras de sesión",
	"tokens_reset_password":  "credenciales efímeras de recuperación",
}

func TestTodaTablaTieneAuditoria(t *testing.T) {
	db := testutil.NuevaBD(t)

	filas, err := db.Query(`
SELECT t.table_name,
       COALESCE(SUM(tr.event_manipulation = 'INSERT' AND tr.action_statement LIKE '%auditoria_cambios%'), 0),
       COALESCE(SUM(tr.event_manipulation = 'UPDATE' AND tr.action_statement LIKE '%auditoria_cambios%'), 0),
       COALESCE(SUM(tr.event_manipulation = 'DELETE' AND tr.action_statement LIKE '%auditoria_cambios%'), 0)
FROM information_schema.tables t
LEFT JOIN information_schema.triggers tr
  ON tr.event_object_schema = t.table_schema AND tr.event_object_table = t.table_name
WHERE t.table_schema = DATABASE() AND t.table_type = 'BASE TABLE'
GROUP BY t.table_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer filas.Close()

	var faltantes []string
	for filas.Next() {
		var tabla string
		var ins, upd, del int
		if err := filas.Scan(&tabla, &ins, &upd, &del); err != nil {
			t.Fatal(err)
		}
		if _, exenta := tablasSinAuditoria[tabla]; exenta {
			continue
		}
		if ins == 0 || upd == 0 || del == 0 {
			faltantes = append(faltantes, tabla)
		}
	}
	if err := filas.Err(); err != nil {
		t.Fatal(err)
	}
	if len(faltantes) > 0 {
		t.Fatalf("tablas sin auditoría completa (generar con scripts/gen_auditoria.sh): %s", strings.Join(faltantes, ", "))
	}
}

func TestAuditoriaEsSoloInsercion(t *testing.T) {
	db := testutil.NuevaBD(t)

	if _, err := db.Exec(`INSERT INTO auditoria_cambios (tabla_afectada, registro_id, accion) VALUES ('prueba', 1, 'insert')`); err != nil {
		t.Fatal(err)
	}
	for nombre, sentencia := range map[string]string{
		"update": `UPDATE auditoria_cambios SET accion = 'delete' WHERE tabla_afectada = 'prueba'`,
		"delete": `DELETE FROM auditoria_cambios WHERE tabla_afectada = 'prueba'`,
	} {
		_, err := db.Exec(sentencia)
		if err == nil || !strings.Contains(err.Error(), "registro_inmutable") {
			t.Errorf("%s sobre auditoria_cambios: se esperaba registro_inmutable, se obtuvo %v", nombre, err)
		}
	}
}
