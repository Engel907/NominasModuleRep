// Package dto contiene el acceso a datos de internal/db: DBTX/Queries como
// punto de entrada común, y las funciones y structs de cada módulo (ver
// security.go) escritas a mano con database/sql.
//
// Este proyecto eliminó sqlc por completo (Fase 5B): ya no hay generación de
// código ni queries anotadas `-- name: X :one/:many/:exec` en
// internal/db/queries/*.sql que se compilen a Go. Ese directorio se conserva
// solo como referencia legible de qué hace cada query, pero el código real
// vive aquí y se edita a mano como cualquier otro paquete Go.
//
// Se conserva el boilerplate DBTX/Queries/New/WithTx tal cual porque no
// dependía de sqlc en sí, solo de la forma estándar de envolver *sql.DB o
// *sql.Tx: internal/auth/service.go (y el resto de internal/*) siguen
// pudiendo construir *Queries de la misma manera y pasarlo a una Tx sin
// cambios.
package dto

import (
	"context"
	"database/sql"
)

// DBTX es la interfaz mínima que necesita Queries: la implementan tanto
// *sql.DB como *sql.Tx, así el mismo *Queries sirve fuera y dentro de una
// transacción (ver WithTx).
type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

// Queries agrupa todas las funciones de acceso a datos (ver security.go).
// Es intencionalmente un solo tipo para todo el paquete dto, igual que lo
// era con sqlc, para no forzar a internal/auth/service.go a manejar un
// *Queries por archivo.
type Queries struct {
	db DBTX
}

// WithTx devuelve una copia de Queries que ejecuta contra `tx` en vez de la
// conexión/pool original. Usado para las escrituras de 2 pasos sobre
// usuario_roles (CerrarUsuarioRolVigente + InsertarUsuarioRolVigente), que
// SIEMPRE deben ir en la misma transacción.
func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return &Queries{db: tx}
}