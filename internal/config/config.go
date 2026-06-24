// Package config provides application configuration loaded from environment variables.
package config

import (
	"os"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/logger"
)

type Config struct {
	RestAPIKey       string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresHost     string
	PostgresPort     string
}

const RESTAPIKey = "REST_API_KEY"

const PostgresUser = "POSTGRES_USER"
const PostgresPassword = "POSTGRES_PASSWORD"
const PostgresDB = "POSTGRES_DB"
const PostgresHost = "POSTGRES_HOST"
const PostgresPort = "POSTGRES_PORT"

var cfg Config

func init() {
	logger.Info("Initialising configuration")

	cfg.RestAPIKey = requireEnv(RESTAPIKey)
	cfg.PostgresUser = requireEnv(PostgresUser)
	cfg.PostgresPassword = requireEnv(PostgresPassword)
	cfg.PostgresDB = requireEnv(PostgresDB)
	cfg.PostgresHost = requireEnv(PostgresHost)
	cfg.PostgresPort = requireEnv(PostgresPort)

	logger.Info("Configuration initialised successfully")
}

func requireEnv(key string) string {
	env, found := os.LookupEnv(key)
	if !found {
		panic(logger.Fatal("Environment variable is not set", "variable", key))
	}
	return env
}

func Get() Config {
	return cfg
}
