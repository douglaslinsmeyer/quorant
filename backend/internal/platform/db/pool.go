// Package db provides database connection pool utilities for the Quorant backend.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/platform/config"
)

// NewPool creates a PostgreSQL connection pool from the given config.
// The caller is responsible for closing the pool via pool.Close() when done.
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("db: failed to parse connection config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: failed to connect to database: %w", err)
	}

	return pool, nil
}
