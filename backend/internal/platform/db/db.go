// Package db provides the PostgreSQL connection pool and migration runner.
//
// Design: We use pgxpool (connection pooling built into pgx v5) rather than
// database/sql for better performance and pgx-native features (e.g. COPY, LISTEN).
// The pool is configured from application config, not hardcoded.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-org/job-scheduler/internal/config"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// Pool is a type alias for pgxpool.Pool, used for dependency injection.
type Pool = pgxpool.Pool

// Connect creates and validates a connection pool to PostgreSQL.
// It retries up to maxAttempts times with exponential backoff to handle
// container startup ordering (Postgres may not be ready immediately).
func Connect(ctx context.Context, cfg config.PostgresConfig, log *logger.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = 10 * time.Minute
	poolCfg.HealthCheckPeriod = 1 * time.Minute

	const maxAttempts = 10
	var pool *pgxpool.Pool
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolCfg)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				break
			} else {
				pool.Close()
				err = pingErr
			}
		}
		backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
		log.Warn("postgres not ready, retrying",
			logger.Int("attempt", attempt),
			logger.Int("max_attempts", maxAttempts),
			logger.Err(err),
		)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	if err != nil {
		return nil, fmt.Errorf("connect to postgres after %d attempts: %w", maxAttempts, err)
	}

	log.Info("postgres connection pool established",
		logger.String("host", cfg.Host),
		logger.Int("max_conns", cfg.MaxOpenConns),
	)
	return pool, nil
}

// RunMigrations applies all pending SQL migrations from the given source path.
// Uses golang-migrate with the file:// source driver.
func RunMigrations(databaseURL, migrationsPath string, log *logger.Logger) error {
	source := fmt.Sprintf("file://%s", migrationsPath)
	m, err := migrate.New(source, databaseURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		log.Warn("could not read migration version", logger.Err(err))
	} else {
		log.Info("migrations applied",
			logger.Int("version", int(version)),
			logger.Bool("dirty", dirty),
		)
	}
	return nil
}
