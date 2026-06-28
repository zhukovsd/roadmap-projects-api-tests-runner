package testrun

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/logger"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (TestRun, error) {
	logger.Info("Creating test run", "input", input)

	suite, err := newTestSuite(input.ProjectName, input.DeployBaseURL)
	if err != nil {
		return TestRun{}, fmt.Errorf("service.Create: %w", err)
	}

	testRun, err := s.store.Create(ctx, input)
	if err != nil {
		return TestRun{}, fmt.Errorf("service.Create: %w", err)
	}

	go s.runTestSuite(suite, testRun)

	return testRun, nil
}

func (s *Service) FindByID(ctx context.Context, id uuid.UUID) (TestRun, error) {
	logger.Info("Finding test run by ID", "id", id)

	testRun, err := s.store.FindByID(ctx, id)
	if err != nil {
		return TestRun{}, fmt.Errorf("service.FindByID: %w", err)
	}
	return testRun, nil
}

func (s *Service) List(ctx context.Context, filters filters) ([]TestRun, error) {
	logger.Info("Finding test run by filters", "filters", filters)

	testRuns, err := s.store.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("service.List: %w", err)
	}
	return testRuns, nil
}

func (s *Service) runTestSuite(suite Suite, testRun TestRun) {
	logger.Info("Running test suite", "project", testRun.ProjectName, "telegramUserID", testRun.TelegramUserID)

	testResults := suite.Run()
	completedAt := time.Now().Unix()

	reportBytes, err := json.Marshal(testResults)
	if err != nil {
		logger.Error("Failed to marshal report: %w", err)

		error := "Failed to marshal report"

		err = s.store.Update(context.Background(), testRun.ID, updateParams{
			CompletedAt: &completedAt,
			Status:      StatusCompleted,
			Error:       &error,
		})
		if err != nil {
			logger.Error("Failed to update test run", "error", err, "testRun", testRun)
		}
		return
	}

	report := json.RawMessage(reportBytes)

	err = s.store.Update(context.Background(), testRun.ID, updateParams{
		CompletedAt: &completedAt,
		Status:      StatusCompleted,
		Error:       nil,
		Report:      &report,
	})
	if err != nil {
		logger.Error("Failed to update test run", "error", err, "testRun", testRun)
	}

	logger.Info("Test suite completed", "project", testRun.ProjectName, "telegramUserID", testRun.TelegramUserID)
}
