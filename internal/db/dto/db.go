// Package dto contiene el acceso a datos con database/sql: DBTX y Queries
// como punto de entrada común, y las consultas de cada módulo.
package dto

import (
	"context"
	"database/sql"
)

// DBTX es la interfaz mínima que necesita Queries: la implementan tanto
// *sql.DB como *sql.Tx, así el mismo *Queries sirve fuera y dentro de una
// transacción.
type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

// Queries agrupa todas las funciones de acceso a datos.
type Queries struct {
	db DBTX
}

// WithTx devuelve una copia de Queries que ejecuta contra `tx` en vez de la
// conexión/pool original. Usado para las escrituras de 2 pasos sobre
// usuario_roles (CerrarUsuarioRolVigente + InsertarUsuarioRolVigente), que
// siempre deben ir en la misma transacción.
func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return &Queries{db: tx}
}
