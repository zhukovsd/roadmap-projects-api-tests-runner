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
			if tc.Attributes[tests.TestDescriptionAttr] == "" {
				continue
			}
			results = append(results, TestResult{
				Name:        tc.Test.Name(),
				Status:      status,
				Output:      cleanOutput(pkg.OutputLines(tc)),
				Description: tc.Attributes[tests.TestDescriptionAttr],
				Time:        tc.Time.Unix(),
				Elapsed:     tc.Elapsed.Milliseconds(),
				Request:     extractRequest(pkg.OutputLines(tc)),
				Response:    extractResponse(pkg.OutputLines(tc)),
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

	total := 0
	for _, tc := range exe.Skipped() {
		if tc.Attributes[tests.TestDescriptionAttr] != "" {
			total++
		}
	}

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

	var cases []testjson.TestCase

	pkg := execution.Package(event.Package)

	switch event.Action {
	case testjson.ActionPass:
		cases = pkg.Passed
	case testjson.ActionFail:
		cases = pkg.Failed
	case testjson.ActionSkip:
		cases = pkg.Skipped
	default:
		return nil
	}
	if len(cases) == 0 || cases[len(cases)-1].Attributes[tests.TestDescriptionAttr] == "" {
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

func cleanOutput(lines []string) string {
	var out []string

	skipping := false
	for _, line := range lines {
		line = stripCallerPrefix(strings.TrimSpace(strings.TrimRight(line, "\n")))

		if skipping {
			if strings.HasPrefix(line, tests.TestRequestEndMarker) || strings.HasPrefix(line, tests.TestResponseEndMarker) {
				skipping = false
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, tests.TestRequestStartMarker),
			strings.HasPrefix(line, tests.TestResponseStartMarker):
			skipping = true
			continue
		case strings.HasPrefix(line, "=== RUN"):
			continue
		case strings.HasPrefix(line, "=== PAUSE"):
			continue
		case strings.HasPrefix(line, "=== CONT"):
			continue
		case strings.HasPrefix(line, "=== ATTR"):
			continue
		case strings.HasPrefix(line, "--- PASS:"):
			continue
		case strings.HasPrefix(line, "--- FAIL:"):
			continue
		case strings.HasPrefix(line, "--- SKIP:"):
			continue
		}

		if line != "" {
			out = append(out, line)
		}
	}

	return strings.Join(out, "\n")
}

func extractRequest(lines []string) string {
	var out []string

	reading := false
	for _, line := range lines {
		line = stripCallerPrefix(strings.TrimSpace(strings.TrimRight(line, "\n")))

		if strings.HasPrefix(line, tests.TestRequestStartMarker) {
			reading = true
			continue
		}
		if strings.HasPrefix(line, tests.TestRequestEndMarker) {
			break
		}
		if reading {
			out = append(out, line)
		}

	}

	return strings.Join(out, "\n")
}

func extractResponse(lines []string) string {
	var out []string

	reading := false
	for _, line := range lines {
		line = stripCallerPrefix(strings.TrimSpace(strings.TrimRight(line, "\n")))

		if strings.HasPrefix(line, tests.TestResponseStartMarker) {
			reading = true
			continue
		}
		if strings.HasPrefix(line, tests.TestResponseEndMarker) {
			break
		}
		if reading {
			out = append(out, line)
		}
	}

	return strings.Join(out, "\n")
}

func stripCallerPrefix(line string) string {
	_, after, ok := strings.Cut(line, ".go:")
	if !ok {
		return line
	}
	_, after, ok = strings.Cut(after, ":")
	if !ok {
		return line
	}
	return strings.TrimPrefix(after, " ")
}
