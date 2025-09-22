package infra

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool configures and returns a PostgreSQL connection pool.
func NewPostgresPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, fmt.Errorf("database url is required")
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
