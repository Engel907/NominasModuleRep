//go:build integration

package catalogos_test

import (
	"context"
	"errors"
	"testing"

	"planillas-ccdr/internal/catalogos"
	"planillas-ccdr/internal/testutil"
)

func TestCatalogosCoincidenConElEsquema(t *testing.T) {
	db := testutil.NuevaBD(t)
	for nombre, c := range catalogos.Registro() {
		for _, campo := range append([]catalogos.Campo{{}}, c.Campos...) {
			col := campo.Nombre
			if col == "" {
				col = "id"
			}
			var n int
			if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
				nombre, col).Scan(&n); err != nil || n != 1 {
				t.Errorf("%s.%s no existe en el esquema (%v)", nombre, col, err)
			}
		}
	}
}

func TestCatalogoCicloCompletoAuditado(t *testing.T) {
	db := testutil.NuevaBD(t)
	ctx := context.Background()
	res, err := db.Exec(`INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash) VALUES ('cfg', 'Configuración', 'cfg@example.test', 'x')`)
	if err != nil {
		t.Fatal(err)
	}
	usuario, _ := res.LastInsertId()
	svc := catalogos.NewService(db, catalogos.Registro())

	for _, nombre := range catalogos.Nombres(catalogos.Registro()) {
		if _, _, err := svc.Listar(ctx, nombre, 50, 0); err != nil {
			t.Errorf("listar %s: %v", nombre, err)
		}
	}

	creado, err := svc.Crear(ctx, usuario, "entidades_emisoras_incapacidad", []byte(`{"codigo":"ASEG_PRIV","nombre":"Aseguradora privada","activo":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if *creado["codigo"] != "ASEG_PRIV" || *creado["activo"] != "1" {
		t.Fatalf("creado %v", creado)
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM entidades_emisoras_incapacidad WHERE codigo = 'ASEG_PRIV'`).Scan(&id); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Crear(ctx, usuario, "entidades_emisoras_incapacidad", []byte(`{"codigo":"ASEG_PRIV","nombre":"Otra","activo":true}`)); !errors.Is(err, catalogos.ErrDuplicado) {
		t.Fatalf("duplicado: %v", err)
	}
	if _, err := svc.Actualizar(ctx, usuario, "entidades_emisoras_incapacidad", id, []byte(`{"codigo":"OTRO"}`)); !errors.Is(err, catalogos.ErrCampoInmutable) {
		t.Fatalf("código inmutable: %v", err)
	}
	actualizado, err := svc.Actualizar(ctx, usuario, "entidades_emisoras_incapacidad", id, []byte(`{"codigo":"ASEG_PRIV","activo":false}`))
	if err != nil || *actualizado["activo"] != "0" || *actualizado["nombre"] != "Aseguradora privada" {
		t.Fatalf("actualizado %v (%v)", actualizado, err)
	}
	if _, err := svc.Actualizar(ctx, usuario, "entidades_emisoras_incapacidad", id, []byte(`{"campo_falso":1}`)); !errors.Is(err, catalogos.ErrDatosInvalidos) {
		t.Fatalf("campo desconocido: %v", err)
	}
	if err := svc.Borrar(ctx, usuario, "entidades_emisoras_incapacidad", id); !errors.Is(err, catalogos.ErrBorradoNoPermitido) {
		t.Fatalf("borrar catálogo sin borrado: %v", err)
	}
	if _, err := svc.Crear(ctx, usuario, "usuarios", []byte(`{}`)); !errors.Is(err, catalogos.ErrCatalogoNoSoportado) {
		t.Fatalf("tabla fuera de la lista blanca: %v", err)
	}

	var tipoPlanilla, concepto int64
	if err := db.QueryRow(`SELECT id FROM tipos_planilla WHERE clase = 'regular' ORDER BY id LIMIT 1`).Scan(&tipoPlanilla); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM conceptos_salariales WHERE codigo = 'VIATICOS'`).Scan(&concepto); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crear(ctx, usuario, "tipo_planilla_concepto", []byte(`{"tipo_planilla_id":999999,"concepto_salarial_id":1}`)); !errors.Is(err, catalogos.ErrDatosInvalidos) {
		t.Fatalf("referencia inexistente: %v", err)
	}
	var vinculo int64
	if err := db.QueryRow(`SELECT id FROM tipo_planilla_concepto WHERE tipo_planilla_id = ? AND concepto_salarial_id = ?`, tipoPlanilla, concepto).Scan(&vinculo); err != nil {
		t.Fatal(err)
	}
	if err := svc.Borrar(ctx, usuario, "tipo_planilla_concepto", vinculo); err != nil {
		t.Fatal(err)
	}

	var auditadas int
	if err := db.QueryRow(`SELECT COUNT(*) FROM auditoria_cambios WHERE usuario_id = ? AND tabla_afectada IN ('entidades_emisoras_incapacidad', 'tipo_planilla_concepto')`, usuario).Scan(&auditadas); err != nil || auditadas != 3 {
		t.Fatalf("auditoría = %d (%v), se esperaban 3 (alta, cambio y baja)", auditadas, err)
	}
}
