package testrun

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/logger"
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

func init() {
	projects := []TestRunProjectName{ProjectNameCurrencyExchange}

	for _, project := range projects {
		suite, err := newTestSuite(project, "")
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
		testResults, err := suite.run(context.Background(), runOpts{IsDryRun: true})
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}

		actual := len(testResults)
		expected := suite.progressHandler.progress.Total

		if expected != actual {
			testNames := make([]string, actual)
			for i, t := range testResults {
				testNames[i] = t.Name
			}
			logger.Error(
				"Number of total asserts don't match",
				"project", project,
				"expected", expected,
				"actual", actual,
				"testNames", testNames,
			)
			os.Exit(1)
		}
	}
}

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
		progressHandler.progress.Total = currencyExchangeTotal
	default:
		return nil, fmt.Errorf("unknown project name %q: %w", project, ErrInvalidField)
	}

	return suite, nil
}

func (s *Suite) Run(ctx context.Context) ([]TestResult, error) {
	return s.run(ctx, runOpts{IsDryRun: false})
}

func (s *Suite) Progress(ctx context.Context) Progress {
	return s.progressHandler.progress
}

func (s *Suite) run(ctx context.Context, runOpts runOpts) ([]TestResult, error) {
	stdout, wait, err := s.runTestBin(ctx, runOpts)
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
	if err := wait(); err != nil {
		return []TestResult{}, fmt.Errorf("test binary failed: %w", err)
	}
	pkg := exe.Package(s.pkgName)

	results := make([]TestResult, 0)

	appendResults := func(cases []testjson.TestCase, status TestResultStatus) {
		for _, tc := range cases {
			if tc.Attributes[tests.TestDescriptionAttr] == "" {
				continue
			}
			results = append(results, TestResult{
				Name:        stripMasterPrefix(tc.Test.Name()),
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

func (s *Suite) runTestBin(ctx context.Context, opts runOpts) (io.ReadCloser, func() error, error) {
	testCmd := exec.CommandContext(
		ctx,
		fmt.Sprintf("./%s", s.testBin),
		fmt.Sprintf("-base-url=%s", s.baseURL),
		fmt.Sprintf("-dry-run=%t", opts.IsDryRun),
		"-test.v=test2json",
	)
	jsonCmd := exec.CommandContext(ctx, "go", "tool", "test2json", "-t", "-p", s.pkgName)

	var stderr bytes.Buffer
	testCmd.Stderr = &stderr

	testStdout, err := testCmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get test stdout pipe: %w", err)
	}
	jsonCmd.Stdin = testStdout

	jsonStdout, err := jsonCmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get json stdout pipe: %w", err)
	}
	if err := testCmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start test binary: %w", err)
	}
	if err := jsonCmd.Start(); err != nil {
		_ = testCmd.Process.Kill()
		_ = testCmd.Wait()
		return nil, nil, fmt.Errorf("failed to start test2json: %w", err)
	}

	wait := func() error {
		jsonErr := jsonCmd.Wait()
		testErr := testCmd.Wait()

		if testErr != nil {
			if strings.Contains(stderr.String(), "panic") {
				return fmt.Errorf("test binary failed: %w\nstderr:\n%s", testErr, stderr.String())
			}
		}
		if jsonErr != nil {
			return fmt.Errorf("test2json failed: %w", jsonErr)
		}

		return nil
	}

	return jsonStdout, wait, nil
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

// helper_test.go:42: some message -> some message
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

func stripMasterPrefix(testName string) string {
	if after, found := strings.CutPrefix(testName, "TestMaster/"); found {
		return after
	}
	return testName
}
