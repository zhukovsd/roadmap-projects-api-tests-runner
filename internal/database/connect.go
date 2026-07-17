// Package database provides PostgreSQL connection and migration utilities.
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/config"
	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/logger"
)

func ConnectionPool() (*pgxpool.Pool, error) {
	logger.Info("Acquiring database connection pool")

	dbPool, err := pgxpool.New(context.Background(), dbURL())

	if err != nil {
		return nil, fmt.Errorf("failed to acquire database connection pool: %w", err)
	}

	logger.Info("Database connection pool acquired successfully")

	return dbPool, nil
}

func dbURL() string {
	env := config.Get()

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		env.PostgresUser,
		env.PostgresPassword,
		env.PostgresHost,
		env.PostgresPort,
		env.PostgresDB,
	)

}
