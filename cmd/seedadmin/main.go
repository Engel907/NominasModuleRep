// Command seedadmin crea el usuario administrador inicial del sistema.
//
// Reemplaza al usuario con contraseña conocida que sembraba la migración 000006
// (F12). La lógica vive en internal/seedadmin (reutilizada por las pruebas de
// integración, F13). Garantías:
//   - la contraseña es ALEATORIA (crypto/rand, 24 caracteres) y se imprime UNA
//     sola vez por stdout; no se escribe en ningún archivo ni log;
//   - se guarda solo su hash argon2id (security.HashPassword);
//   - el usuario queda con requiere_cambio_password=1 y el rol
//     ADMINISTRADOR_SISTEMA;
//   - es idempotente: si ya existe un administrador activo, se niega.
//
// Uso:
//
//	go run ./cmd/seedadmin -usuario jperez -correo jperez@ejemplo.cr
//
// También se aceptan SEEDADMIN_USUARIO, SEEDADMIN_CORREO y SEEDADMIN_NOMBRE.
// La conexión sale de config.LoadConfig() (mismas variables DB_* y .env que la
// API; también exige las variables PASETO_*).
//
// Salida: stdout contiene SOLO la contraseña (se puede capturar con $(...));
// los mensajes para la persona van a stderr.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/config"
	"planillas-ccdr/internal/security"
	"planillas-ccdr/internal/seedadmin"
)

var reUsuario = regexp.MustCompile(`^[A-Za-z0-9._-]{3,60}$`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seedadmin:", err)
		os.Exit(1)
	}
}

func run() error {
	usuario := flag.String("usuario", os.Getenv("SEEDADMIN_USUARIO"), "nombre de usuario (login) del administrador")
	correo := flag.String("correo", os.Getenv("SEEDADMIN_CORREO"), "correo del administrador")
	nombre := flag.String("nombre", valorOPorDefecto(os.Getenv("SEEDADMIN_NOMBRE"), "Administrador del Sistema"), "nombre completo")
	flag.Parse()

	if err := validarEntrada(*usuario, *nombre, *correo); err != nil {
		return err
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	password, err := seedadmin.GenerarPassword(seedadmin.LongitudPassword)
	if err != nil {
		return fmt.Errorf("generar contraseña: %w", err)
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return fmt.Errorf("calcular hash de la contraseña: %w", err)
	}

	db, err := sql.Open("mysql", cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("abrir conexión: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("conectar a la base de datos: %w", err)
	}

	if err := seedadmin.CrearAdministrador(ctx, db, *usuario, *nombre, *correo, hash); err != nil {
		return err
	}

	// Solo después del COMMIT: si algo falló arriba, no se imprime nada.
	fmt.Fprintf(os.Stderr, "Administrador %q creado con el rol %s. Debe cambiar la contraseña en el primer inicio de sesión.\n", *usuario, seedadmin.RolAdministrador)
	fmt.Fprintln(os.Stderr, "Contraseña temporal (se muestra UNA sola vez, no se guarda en ningún lado):")
	fmt.Println(password)
	return nil
}

func validarEntrada(usuario, nombre, correo string) error {
	if !reUsuario.MatchString(usuario) {
		return errors.New("usuario inválido: 3 a 60 caracteres entre letras, dígitos, punto, guion y guion bajo (flag -usuario)")
	}
	if n := len([]rune(strings.TrimSpace(nombre))); n == 0 || n > 200 {
		return errors.New("nombre inválido: entre 1 y 200 caracteres (flag -nombre)")
	}
	dir, err := mail.ParseAddress(correo)
	if err != nil || dir.Address != correo || len(correo) > 150 {
		return errors.New("correo inválido (flag -correo): debe ser una dirección simple, máximo 150 caracteres")
	}
	return nil
}

func valorOPorDefecto(v, porDefecto string) string {
	if strings.TrimSpace(v) == "" {
		return porDefecto
	}
	return v
}