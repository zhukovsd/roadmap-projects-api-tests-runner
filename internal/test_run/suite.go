package testrun

import (
	"context"
	"fmt"
)

type Suite interface {
	Run(ctx context.Context) []TestResult
	Progress() (finished, total int)
}

func newTestSuite(project TestRunProjectName, deployBaseURL string) (Suite, error) {
	switch project {
	case ProjectNameCurrencyExchange:
		return NewCurrencyExchangeSuite(deployBaseURL), nil
	default:
		return nil, fmt.Errorf("unknown project name %q: %w", project, ErrInvalidField)
	}
}
