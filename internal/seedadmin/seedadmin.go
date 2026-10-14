// Package seedadmin contiene la lógica del alta inicial del administrador,
// extraída de cmd/seedadmin para poder reutilizarla en pruebas de integración.
// Garantías (ver cmd/seedadmin): contraseña aleatoria con crypto/rand, solo se
// guarda su hash, requiere_cambio_password=1, idempotente (se niega si ya hay
// un administrador activo) y transacción propia con @auditoria_usuario_id = NULL.
package seedadmin

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/go-sql-driver/mysql"
)

const (
	RolAdministrador = "ADMINISTRADOR_SISTEMA"
	LongitudPassword = 24 // mínimo exigido: 20

	letrasMin = "abcdefghijkmnopqrstuvwxyz" // sin l
	letrasMay = "ABCDEFGHJKLMNPQRSTUVWXYZ"  // sin I, O
	digitos   = "23456789"                  // sin 0, 1
	simbolos  = "!#%*-_=+@"
	alfabeto  = letrasMin + letrasMay + digitos + simbolos
)

var ErrAdminYaExiste = errors.New("ya existe un administrador activo: seedadmin solo sirve para el alta inicial")

// CrearAdministrador inserta usuario + rol vigente en una transacción propia.
// hash es el hash argon2id ya calculado (security.HashPassword).
func CrearAdministrador(ctx context.Context, db *sql.DB, usuario, nombre, correo, hash string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // tras Commit devuelve ErrTxDone; no importa

	// Alta inicial sin autor previo: NULL explícito (no heredar el valor de otra sesión).
	if _, err := tx.ExecContext(ctx, "SET @auditoria_usuario_id = NULL"); err != nil {
		return fmt.Errorf("fijar @auditoria_usuario_id: %w", err)
	}

	// Bloquear la fila del rol serializa dos ejecuciones simultáneas.
	var rolID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE codigo = ? FOR UPDATE", RolAdministrador).Scan(&rolID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("el rol %s no existe: aplica primero la migración 000112", RolAdministrador)
	}
	if err != nil {
		return fmt.Errorf("buscar el rol %s: %w", RolAdministrador, err)
	}

	var existentes int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM usuario_roles ur
		JOIN usuarios u ON u.id = ur.usuario_id
		WHERE ur.rol_id = ? AND ur.fecha_fin_vigencia IS NULL AND u.activo = 1`, rolID).Scan(&existentes)
	if err != nil {
		return fmt.Errorf("verificar administradores existentes: %w", err)
	}
	if existentes > 0 {
		return ErrAdminYaExiste
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO usuarios (nombre_usuario, nombre_completo, correo, password_hash, empleado_id, activo, requiere_cambio_password)
		VALUES (?, ?, ?, ?, NULL, 1, 1)`, usuario, nombre, correo, hash)
	if err != nil {
		return traducirErrorInsert(err)
	}
	usuarioID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("obtener id del usuario creado: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO usuario_roles (usuario_id, rol_id, creado_por, fecha_inicio_vigencia, fecha_fin_vigencia)
		VALUES (?, ?, NULL, CURDATE(), NULL)`, usuarioID, rolID); err != nil {
		return fmt.Errorf("asignar el rol %s: %w", RolAdministrador, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar la transacción: %w", err)
	}
	return nil
}

func traducirErrorInsert(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return errors.New("ya existe un usuario con ese nombre de usuario o correo (si venía de la migración 000006, el usuario 'admin' sigue ocupando su nombre: elige otro)")
	}
	return fmt.Errorf("crear el usuario: %w", err)
}

// GenerarPassword devuelve n caracteres aleatorios (crypto/rand, sin sesgo de
// módulo) con al menos una minúscula, una mayúscula, un dígito y un símbolo.
func GenerarPassword(n int) (string, error) {
	if n < 20 {
		return "", errors.New("la contraseña debe tener al menos 20 caracteres")
	}
	tope := big.NewInt(int64(len(alfabeto)))
	for {
		b := make([]byte, n)
		for i := range b {
			x, err := rand.Int(rand.Reader, tope)
			if err != nil {
				return "", err
			}
			b[i] = alfabeto[x.Int64()]
		}
		if cumpleComposicion(string(b)) {
			return string(b), nil
		}
	}
}

func cumpleComposicion(p string) bool {
	return strings.ContainsAny(p, letrasMin) &&
		strings.ContainsAny(p, letrasMay) &&
		strings.ContainsAny(p, digitos) &&
		strings.ContainsAny(p, simbolos)
}