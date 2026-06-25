package testrun

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/logger"
)

type CurrencyExchangeSuite struct {
	host string
}

func (s CurrencyExchangeSuite) Run() []TestResult {
	testCases := []func() TestResult{
		s.testGetCurrenciesStatus,
		s.testGetCurrenciesContentType,
	}

	testResults := make([]TestResult, len(testCases))

	var wg sync.WaitGroup

	logger.Debug("Running test suite")

	for i, tc := range testCases {
		wg.Add(1)
		go func(i int, fn func() TestResult) {
			defer wg.Done()
			testResults[i] = fn()
		}(i, tc)
	}

	wg.Wait()

	logger.Debug("Test suite completed")

	return testResults
}

func (s CurrencyExchangeSuite) testGetCurrenciesStatus() TestResult {
	name := "GET /currencies => status code 200"

	logger.Debug("Running test case", "name", name)

	resp, err := http.Get(s.host + "/currencies")
	if err != nil {
		return TestResult{Name: name, Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TestResult{Name: name, Error: fmt.Errorf("expected 200 got %d", resp.StatusCode)}
	}
	return TestResult{Name: name, Passed: true}
}

func (s CurrencyExchangeSuite) testGetCurrenciesContentType() TestResult {
	name := "GET /currencies => Content-Type: application/json"

	logger.Debug("Running test case", "name", name)

	resp, err := http.Get(s.host + "/currencies")
	if err != nil {
		return TestResult{Name: name, Error: err}
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")

	if !strings.HasPrefix(contentType, "application/json") {
		return TestResult{Name: name, Error: fmt.Errorf("expected \"application/json\" got %q", contentType)}
	}
	return TestResult{Name: name, Passed: true}
}
