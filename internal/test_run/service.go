package testrun

import (
	"context"
	"fmt"

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

	testRun, err := s.store.Create(ctx, input)
	if err != nil {
		return TestRun{}, fmt.Errorf("service.Create: %w", err)
	}
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

func (s *Service) List(ctx context.Context, filters Filters) ([]TestRun, error) {
	logger.Info("Finding test run by filters", "filters", filters)

	testRuns, err := s.store.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("service.List: %w", err)
	}
	return testRuns, nil
}
