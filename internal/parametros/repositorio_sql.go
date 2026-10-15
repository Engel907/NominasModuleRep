package parametros

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// RepositorioSQL implementa Repositorio sobre database/sql (MySQL).
type RepositorioSQL struct{ db *sql.DB }

func NewRepositorioSQL(db *sql.DB) *RepositorioSQL { return &RepositorioSQL{db: db} }

const queryVigente = `
SELECT clave, valor, tipo
  FROM parametros_generales
 WHERE clave = ?
   AND fecha_inicio_vigencia <= ?
   AND (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= ?)
 ORDER BY fecha_inicio_vigencia DESC
 LIMIT 1`

func (r *RepositorioSQL) BuscarVigente(ctx context.Context, clave string, fecha time.Time) (Registro, error) {
	f := soloFecha(fecha).Format(formatoFecha)
	var reg Registro
	err := r.db.QueryRowContext(ctx, queryVigente, clave, f, f).Scan(&reg.Clave, &reg.Valor, &reg.Tipo)
	if errors.Is(err, sql.ErrNoRows) {
		return Registro{}, ErrNoEncontrado
	}
	return reg, err
}