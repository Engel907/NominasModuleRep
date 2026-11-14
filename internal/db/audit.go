// Package db contiene la conexión y el wrapper transaccional auditado que
// toda escritura del sistema debe usar.
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"planillas-ccdr/internal/db/dto"
)

// ErrNilUserID se devuelve si se intenta auditar una escritura sin un
// usuario asociado. Si en el futuro aparece un caso legítimo de escritura
// "del sistema" (job batch, migración de datos, etc.) sin usuario humano,
// resolverlo con un usuario técnico real en la tabla `usuarios`, no
// pasando 0 o un valor mágico acá.
var ErrNilUserID = errors.New("db: usuarioID es requerido para WithAuditContext")

const (
	sqlSetAuditoriaUsuario   = "SET @auditoria_usuario_id = ?"
	sqlResetAuditoriaUsuario = "SET @auditoria_usuario_id = NULL"

	// timeoutResetAuditoria acota el reset de la variable de sesión, que
	// corre con un contexto no cancelable por el ctx original.
	timeoutResetAuditoria = 5 * time.Second
)

// AuditedFunc es el trabajo que corre dentro de la transacción auditada.
// Recibe *dto.Queries ya ligado a la transacción (no al *sql.DB), y debe
// devolver un error para forzar rollback, o nil para permitir el commit.
// No debe hacer su propio Commit/Rollback ni abrir otra transacción.
type AuditedFunc func(ctx context.Context, q *dto.Queries) error

// WithAuditContext toma una conexión dedicada del pool, abre una
// transacción sobre ella, establece @auditoria_usuario_id (de forma que los
// triggers de auditoría capturen el usuario correcto), ejecuta fn, y hace
// commit si fn no devolvió error, o rollback en caso contrario (incluyendo
// panics, que se re-lanzan después del rollback).
func WithAuditContext(ctx context.Context, sqlDB *sql.DB, usuarioID int64, fn AuditedFunc) error {
	if usuarioID <= 0 {
		return ErrNilUserID
	}

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("db: no se pudo obtener una conexión dedicada: %w", err)
	}
	defer liberarConexionAuditada(ctx, conn)

	return ejecutarEnTx(ctx, conn, usuarioID, fn)
}

// ejecutarEnTx corre fn dentro de una transacción abierta sobre `conn`.
// No libera la conexión: eso es responsabilidad de WithAuditContext.
func ejecutarEnTx(ctx context.Context, conn *sql.Conn, usuarioID int64, fn AuditedFunc) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: no se pudo iniciar la transacción: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if _, execErr := tx.ExecContext(ctx, sqlSetAuditoriaUsuario, usuarioID); execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("db: no se pudo establecer @auditoria_usuario_id: %w", execErr)
	}

	q := dto.New(tx)

	if fnErr := fn(ctx, q); fnErr != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("db: fn falló (%w) y el rollback también falló: %v", fnErr, rbErr)
		}
		return fnErr
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("db: no se pudo hacer commit: %w", commitErr)
	}

	return nil
}

// liberarConexionAuditada limpia la variable de sesión y devuelve la
// conexión al pool; si la limpieza falla, la descarta.
func liberarConexionAuditada(ctx context.Context, conn *sql.Conn) {
	resetCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeoutResetAuditoria)
	defer cancel()

	if _, err := conn.ExecContext(resetCtx, sqlResetAuditoriaUsuario); err != nil {
		slog.Error("db: no se pudo limpiar @auditoria_usuario_id; se descarta la conexión del pool",
			slog.String("error", err.Error()))
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}

	_ = conn.Close()
}
