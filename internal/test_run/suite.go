package testrun

import "fmt"

type TestResult struct {
	Name   string
	Passed bool
	Error  error
}

type Suite interface {
	Run() []TestResult
}

func newTestSuite(project TestRunProjectName, deployBaseURL string) (Suite, error) {
	switch project {
	case ProjectNameCurrencyExchange:
		return CurrencyExchangeSuite{host: deployBaseURL}, nil
	default:
		return nil, fmt.Errorf("unknown project name %q: %w", project, ErrInvalidField)
	}
}
