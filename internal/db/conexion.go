package db

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"

	"planillas-ccdr/internal/config"
)

// Conectar abre el pool de MySQL con la configuración (TLS, límites del pool)
// y verifica la conexión con un ping acotado por PingTimeout.
func Conectar(dbCfg config.DatabaseConfig) (*sql.DB, error) {
	if err := dbCfg.RegistrarTLS(); err != nil {
		return nil, err
	}
	sqlDB, err := sql.Open("mysql", dbCfg.DSN())
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(dbCfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(dbCfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(dbCfg.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), dbCfg.PingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return sqlDB, nil
}
