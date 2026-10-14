package dto_test

// Prueba de integración del contador atómico (F14 / S5). Se omite salvo que
// TEST_DATABASE_URL apunte a una BD MySQL con todas las migraciones aplicadas
// (incluida la 000114), p. ej.:
//
//	TEST_DATABASE_URL='user:pass@tcp(127.0.0.1:3306)/planillas_test?parseTime=true&loc=UTC&charset=utf8mb4'
//
// Crea un usuario propio y lo borra al terminar. Supuesto: el constructor de
// *Queries es dto.New(db) (convención sqlc); ajustar si el proyecto usa otro.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/db/dto"
)

func TestRegistrarIntentoFallidoAtomico_DosGoroutines(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite la prueba de integración")
	}
	sqlDB, err := sql.Open("mysql", url)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(8)

	ctx := context.Background()
	sufijo := time.Now().UnixNano()
	res, err := sqlDB.ExecContext(ctx, `
INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash, activo)
VALUES (?, 'Prueba contador atómico', ?, 'no-es-un-hash', 1)`,
		fmt.Sprintf("t_cnt_%d", sufijo), fmt.Sprintf("t_cnt_%d@prueba.local", sufijo))
	if err != nil {
		t.Fatalf("no se pudo crear el usuario de prueba: %v", err)
	}
	id, _ := res.LastInsertId()
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM usuarios WHERE id = ?`, id) })

	q := dto.New(sqlDB)
	const (
		goroutines  = 2
		porGorutina = 10
		umbral      = 15 // entre 10 y 20: solo se cruza si NO se pierde ninguna suma
	)
	hasta := time.Now().Add(15 * time.Minute)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < porGorutina; i++ {
				if _, err := q.RegistrarIntentoFallidoAtomico(ctx, id, umbral, hasta); err != nil {
					t.Errorf("RegistrarIntentoFallidoAtomico: %v", err)
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	u, err := q.GetUsuarioPorID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if want := uint16(goroutines * porGorutina); u.IntentosFallidos != want {
		t.Fatalf("intentos_fallidos = %d, se esperaba %d (se perdieron sumas: no es atómico)", u.IntentosFallidos, want)
	}
	if !u.BloqueadoHasta.Valid || !u.BloqueadoHasta.Time.After(time.Now()) {
		t.Fatalf("al cruzar el umbral debe quedar bloqueado hacia el futuro: %+v", u.BloqueadoHasta)
	}
}

// El UPDATE fija bloqueado_hasta con el contador ANTES de incrementarse: con
// umbral N, el intento N-1 no bloquea y el N sí (ver el comentario sobre el
// orden de los SET en RegistrarIntentoFallidoAtomico).
func TestRegistrarIntentoFallidoAtomico_BloqueaExactamenteEnElUmbral(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite la prueba de integración")
	}
	sqlDB, err := sql.Open("mysql", url)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	ctx := context.Background()
	sufijo := time.Now().UnixNano()
	res, err := sqlDB.ExecContext(ctx, `
INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash, activo)
VALUES (?, 'Prueba umbral', ?, 'no-es-un-hash', 1)`,
		fmt.Sprintf("t_umb_%d", sufijo), fmt.Sprintf("t_umb_%d@prueba.local", sufijo))
	if err != nil {
		t.Fatalf("no se pudo crear el usuario de prueba: %v", err)
	}
	id, _ := res.LastInsertId()
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM usuarios WHERE id = ?`, id) })

	q := dto.New(sqlDB)
	const umbral = 3
	hasta := time.Now().Add(15 * time.Minute)

	for i := 1; i <= umbral; i++ {
		e, err := q.RegistrarIntentoFallidoAtomico(ctx, id, umbral, hasta)
		if err != nil {
			t.Fatal(err)
		}
		if bloqueado := e.BloqueadoHasta.Valid; bloqueado != (i == umbral) {
			t.Fatalf("intento %d de %d: bloqueado = %v", i, umbral, bloqueado)
		}
	}
}
