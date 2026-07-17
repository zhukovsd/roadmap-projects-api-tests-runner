package testrun

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/tests"
	"gotest.tools/gotestsum/testjson"
)

type Suite struct {
	baseURL         string
	testBin         string
	pkgName         string
	progressHandler *progressHandler
}

type runOpts struct {
	IsDryRun bool
}
type progressHandler struct {
	progress Progress
	mu       sync.Mutex
}

var (
	totalCacheMu sync.Mutex
	totalCache   = make(map[string]int)
)

func newTestSuite(project TestRunProjectName, deployBaseURL string) (*Suite, error) {
	progressHandler := &progressHandler{}
	suite := &Suite{
		baseURL:         deployBaseURL,
		progressHandler: progressHandler,
	}

	switch project {
	case ProjectNameCurrencyExchange:
		suite.pkgName = "currency_exchange"
		suite.testBin = "currencyexchange.test"
	default:
		return nil, fmt.Errorf("unknown project name %q: %w", project, ErrInvalidField)
	}

	total, err := suite.total(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get total tests: %w", err)
	}
	progressHandler.progress.Total = total

	return suite, nil
}

func (s *Suite) Run(ctx context.Context) ([]TestResult, error) {
	stdout, err := s.runTestBin(ctx, runOpts{IsDryRun: false})
	if err != nil {
		return nil, fmt.Errorf("failed to run test binary: %w", err)
	}
	exe, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout:  stdout,
		Handler: s.progressHandler,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan test output: %w", err)
	}
	pkg := exe.Package(s.pkgName)

	results := make([]TestResult, 0)

	appendResults := func(cases []testjson.TestCase, status TestResultStatus) {
		for _, tc := range cases {
			results = append(results, TestResult{
				Name:        tc.Test.Name(),
				Status:      status,
				Output:      strings.Join(pkg.OutputLines(tc), ""),
				Description: tc.Attributes[tests.TestDescriptionAttr],
				Time:        tc.Time.Unix(),
				Elapsed:     tc.Elapsed.Milliseconds(),
			})
		}
	}

	appendResults(pkg.Passed, ResultStatusPassed)
	appendResults(pkg.Failed, ResultStatusFailed)
	appendResults(pkg.Skipped, ResultStatusSkipped)

	return results, nil
}

func (s *Suite) Progress(ctx context.Context) Progress {
	return s.progressHandler.progress
}

func (s *Suite) total(ctx context.Context) (int, error) {
	totalCacheMu.Lock()
	if total, ok := totalCache[s.testBin]; ok {
		totalCacheMu.Unlock()
		return total, nil
	}
	totalCacheMu.Unlock()

	stdout, err := s.runTestBin(ctx, runOpts{IsDryRun: true})
	if err != nil {
		return 0, fmt.Errorf("failed to run test binary: %w", err)
	}
	exe, err := testjson.ScanTestOutput(testjson.ScanConfig{Stdout: stdout})
	if err != nil {
		return 0, fmt.Errorf("failed to scan test output: %w", err)
	}

	total := exe.Total()

	totalCacheMu.Lock()
	totalCache[s.testBin] = total
	totalCacheMu.Unlock()

	return total, nil
}

func (s *Suite) runTestBin(ctx context.Context, opts runOpts) (io.ReadCloser, error) {
	testCmd := exec.CommandContext(
		ctx,
		fmt.Sprintf("./%s", s.testBin),
		fmt.Sprintf("-base-url=%s", s.baseURL),
		fmt.Sprintf("-dry-run=%t", opts.IsDryRun),
		"-test.v=test2json",
	)
	jsonCmd := exec.CommandContext(ctx, "go", "tool", "test2json", "-t", "-p", s.pkgName)

	testStdout, err := testCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get test command stdout pipe: %w", err)
	}
	jsonCmd.Stdin = testStdout

	jsonStdout, err := jsonCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get json command stdout pipe: %w", err)
	}
	if err := testCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start test binary command: %w", err)
	}
	if err := jsonCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start go tool json command: %w", err)
	}
	return jsonStdout, nil
}

func (h *progressHandler) Event(event testjson.TestEvent, execution *testjson.Execution) error {
	if event.Test == "" {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	switch event.Action {
	case testjson.ActionPass:
		h.progress.Passed++
	case testjson.ActionFail:
		h.progress.Failed++
	case testjson.ActionSkip:
		h.progress.Skipped++
	default:
		return nil
	}
	return nil
}

func (h *progressHandler) Err(text string) error {
	return fmt.Errorf("progressHandler Err: %s", text)
}
