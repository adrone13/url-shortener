package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pool against dsn, with MaxConns set explicitly rather
// than via a "pool_max_conns" DSN query param — that param is a pgxpool-only
// convention that other tools sharing the same DSN (migrate, psql) don't
// understand and choke on, so pool sizing is kept out of the DSN entirely.
func Connect(ctx context.Context, dsn string, maxConns int32, logger *slog.Logger, role string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}
	poolCfg.MaxConns = maxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	logger.Info("connected to postgres", slog.String("role", role))

	return pool, nil
}
