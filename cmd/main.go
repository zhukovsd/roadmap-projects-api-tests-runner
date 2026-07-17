package main

import (
	"net/http"
	"os"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/database"
	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/logger"
	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/middleware"
	testrun "github.com/zhukovsd/roadmap-projects-api-test-runner/internal/test_run"
)

const Addr = ":8080"

func main() {
	connPool, err := database.ConnectionPool()
	if err != nil {
		panic(logger.Fatal("Failed to acquire connection pool for database", "error", err))
	}
	if err := database.RunMigrations(); err != nil {
		panic(logger.Fatal("Failed to run migrations", "error", err))
	}

	handler := testrun.NewHandler(testrun.NewService(testrun.NewStore(connPool), testrun.NewSuiteStore()))

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/tests", handler.Create)
	mux.HandleFunc("GET /api/tests/{id}", handler.FindByID)
	mux.HandleFunc("GET /api/tests", handler.List)

	logger.Info("Starting HTTP server", "address", Addr)

	if err := http.ListenAndServe(Addr, middleware.Auth(mux)); err != nil {
		logger.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}
}
