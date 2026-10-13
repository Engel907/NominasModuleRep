// Package db contiene el wrapper transaccional que toda escritura del
// sistema debe usar (ver ADR-000, sección 1 y triggers de auditoría en
// 000001_seguridad_reforzada.up.sql / planillas_ccdr_v11_completo.sql).
//
// REGLA: toda escritura pasa por aquí. Ninguna escritura contra las tablas
// auditadas puede hacerse directo sobre *sql.DB, *sql.Tx propio o un
// *dto.Queries construido fuera de WithAuditContext.
//
// Las tablas auditadas tienen triggers AFTER INSERT/UPDATE/DELETE que leen
// la variable de sesión @auditoria_usuario_id para saber quién hizo el
// cambio (columna usuario_id de auditoria_cambios). Si esa variable no se
// setea antes de escribir, el trigger la registra como NULL y se pierde la
// trazabilidad de "quién".
//
// Las variables de usuario de MySQL son de SESIÓN (conexión), no de
// transacción: sobreviven al COMMIT/ROLLBACK. Por eso WithAuditContext usa
// una conexión dedicada y, antes de devolverla al pool, siempre pone la
// variable en NULL; si no puede garantizarlo, descarta la conexión. Así una
// escritura posterior hecha por error fuera del wrapper nunca queda
// atribuida al usuario de otra operación.
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
	// corre con un contexto NO cancelable por el ctx original.
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
//
// En TODOS los caminos (éxito, error o panic), y antes de devolver la
// conexión al pool, se ejecuta SET @auditoria_usuario_id = NULL sobre esa
// misma conexión con un contexto no cancelado. Si el reset falla, la
// conexión se descarta del pool en lugar de reutilizarse. Un fallo del reset
// no cambia el resultado devuelto (el commit ya ocurrió o no); solo se
// registra en el log.
//
// Uso esperado:
//
//	err := db.WithAuditContext(ctx, sqlDB, usuarioID, func(ctx context.Context, q *dto.Queries) error {
//	    if err := q.AlgunaEscritura(ctx, params); err != nil {
//	        return err
//	    }
//	    return q.OtraEscrituraRelacionada(ctx, otrosParams)
//	})
func WithAuditContext(ctx context.Context, sqlDB *sql.DB, usuarioID int64, fn AuditedFunc) error {
	if usuarioID <= 0 {
		return ErrNilUserID
	}

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("db: no se pudo obtener una conexión dedicada: %w", err)
	}
	// Se ejecuta siempre al salir de esta función, incluso con panic en
	// curso (los defers corren mientras el panic se propaga), y siempre
	// después de que ejecutarEnTx ya hizo commit o rollback.
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
//
// Usa context.WithoutCancel(ctx) porque el ctx original puede estar ya
// cancelado (cliente desconectado, timeout), justo el caso en que más
// importa limpiar la sesión. Requiere Go 1.21+.
func liberarConexionAuditada(ctx context.Context, conn *sql.Conn) {
	resetCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeoutResetAuditoria)
	defer cancel()

	if _, err := conn.ExecContext(resetCtx, sqlResetAuditoriaUsuario); err != nil {
		slog.Error("db: no se pudo limpiar @auditoria_usuario_id; se descarta la conexión del pool",
			slog.String("error", err.Error()))
		// Devolver driver.ErrBadConn desde Raw hace que database/sql cierre
		// la conexión física en vez de reinsertarla en el pool. Si la
		// conexión ya estaba cerrada (sql.ErrConnDone), Raw falla sin
		// efectos y no hay nada más que descartar.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}

	// Tras un descarte, Close devuelve sql.ErrConnDone: es esperado.
	_ = conn.Close()
}
