package testrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/logger"
)

type Currency struct {
}

type CurrencyExchangeSuite struct {
	url      string
	total    int
	finished atomic.Int32
	client   *http.Client
}

func NewCurrencyExchangeSuite(url string) *CurrencyExchangeSuite {
	return &CurrencyExchangeSuite{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (s *CurrencyExchangeSuite) Run(ctx context.Context) []TestResult {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	testCases := []func(context.Context) TestResult{
		s.testHostIsReachable,
		s.testGetCurrenciesStatus,
		s.testGetCurrenciesContentType,
		s.testGetCurrenciesNoRedirect,
		s.testGetCurrenciesJSONBody,
		s.testGetCurrenciesJSONArray,
		s.testGetCurrenciesJSONItemID,
		s.testGetCurrenciesJSONItemName,
		s.testGetCurrenciesJSONItemCode,
		s.testGetCurrenciesJSONItemSign,
	}

	s.total = len(testCases)

	testResults := make([]TestResult, len(testCases))

	var wg sync.WaitGroup
	s.finished.Store(0)

	logger.Debug("Running test suite")

	for i, tc := range testCases {
		wg.Add(1)
		go func(i int, fn func(context.Context) TestResult) {
			defer wg.Done()
			res := fn(ctx)
			testResults[i] = res
			if res.fatal {
				cancel(fmt.Errorf("failed fatal test: %s", res.Name))
			}
			s.finished.Add(1)
		}(i, tc)
	}

	wg.Wait()

	if err := context.Cause(ctx); err != nil {
		logger.Info("Test suite cancelled", "cause", err)
	}

	logger.Debug("Test suite completed")

	return testResults
}

func (s *CurrencyExchangeSuite) Progress() (finished, total int) {
	return int(s.finished.Load()), s.total
}

func (s *CurrencyExchangeSuite) testHostIsReachable(ctx context.Context) TestResult {
	name := "Host is reachable"

	logger.Debug("Running test case", "name", name)

	u, err := url.Parse(s.url)
	if err != nil {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Failed to parse URL: %s", err),
			fatal:  true,
		}
	}
	ip := net.ParseIP(u.Hostname())

	if ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  fmt.Sprintf("Address %q is not accessible from the public internet", u.Host),
				fatal:  true,
			}
		}
		return TestResult{Name: name, Status: ResultStatusPassed}
	}

	addrs, err := net.LookupHost(u.Hostname())
	if err != nil {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("No addresses found for host: %q", u.Hostname()),
			fatal:  true,
		}
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  fmt.Sprintf("Invalid address %q for host %q", a, u.Hostname()),
				fatal:  true,
			}
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  fmt.Sprintf("Address %q is not accessible from the public internet", u.Host),
				fatal:  true,
			}
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func (s *CurrencyExchangeSuite) testGetCurrenciesStatus(ctx context.Context) TestResult {
	name := "GET /currencies => Response status code is 200"

	logger.Debug("Running test case", "name", name)

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/currencies", nil)
	if err != nil {
		return handleError(ctx, err, name)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return handleError(ctx, err, name)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Expected %d got %d", http.StatusOK, resp.StatusCode),
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func (s *CurrencyExchangeSuite) testGetCurrenciesContentType(ctx context.Context) TestResult {
	name := "GET /currencies => Header 'Content-Type' starts with 'application/json'"

	logger.Debug("Running test case", "name", name)

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/currencies", nil)
	if err != nil {
		return handleError(ctx, err, name)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return handleError(ctx, err, name)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	applicationJSON := "application/json"

	if !strings.HasPrefix(contentType, applicationJSON) {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Expected %q got %q", applicationJSON, contentType),
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func (s *CurrencyExchangeSuite) testGetCurrenciesNoRedirect(ctx context.Context) TestResult {
	name := "GET /currencies => Response status code is not one of redirect status codes (30x)"

	logger.Debug("Running test case", "name", name)

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/currencies", nil)
	if err != nil {
		return handleError(ctx, err, name)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return handleError(ctx, err, name)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Unexpected redirect %d", resp.StatusCode),
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func (s *CurrencyExchangeSuite) testGetCurrenciesJSONBody(ctx context.Context) TestResult {
	name := "GET /currencies => Response body is a valid JSON"

	logger.Debug("Running test case", "name", name)

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/currencies", nil)
	if err != nil {
		return handleError(ctx, err, name)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return handleError(ctx, err, name)
	}
	defer resp.Body.Close()

	var body any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Response body is not a valid JSON: %s", err),
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func (s *CurrencyExchangeSuite) testGetCurrenciesJSONArray(ctx context.Context) TestResult {
	name := "GET /currencies => Response body contains JSON array"

	logger.Debug("Running test case", "name", name)

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/currencies", nil)
	if err != nil {
		return handleError(ctx, err, name)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return handleError(ctx, err, name)
	}
	defer resp.Body.Close()

	var body any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Response body is not a valid JSON: %s", err),
		}
	}
	_, ok := body.([]any)
	if !ok {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Response body is not a JSON array: %s", err),
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func (s *CurrencyExchangeSuite) testGetCurrenciesJSONItemID(ctx context.Context) TestResult {
	name := "GET /currencies => Response body contains JSON array of objects with 'id' fields that are integers"

	logger.Debug("Running test case", "name", name)

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/currencies", nil)
	if err != nil {
		return handleError(ctx, err, name)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return handleError(ctx, err, name)
	}
	defer resp.Body.Close()

	var body any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Response body is not a valid JSON: %s", err),
		}
	}
	currencies, ok := body.([]any)
	if !ok {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  "Response body is not a JSON array",
		}
	}
	for _, currency := range currencies {
		c, ok := currency.(map[string]any)
		if !ok {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Array element is not a JSON object",
			}
		}
		id, found := c["id"]
		if !found {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Object field 'id' is missing",
			}
		}
		f, ok := id.(float64)
		if !ok {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Object field 'id' is not a number",
			}
		}
		if f != math.Trunc(f) {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Object field 'id' is not an integer",
			}
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func (s *CurrencyExchangeSuite) testGetCurrenciesJSONItemName(ctx context.Context) TestResult {
	name := "GET /currencies => Response body contains JSON array of objects with 'name' fields that are strings"

	logger.Debug("Running test case", "name", name)

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/currencies", nil)
	if err != nil {
		return handleError(ctx, err, name)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return handleError(ctx, err, name)
	}
	defer resp.Body.Close()

	var body any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Response body is not a valid JSON: %s", err),
		}
	}
	currencies, ok := body.([]any)
	if !ok {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  "Response body is not a JSON array",
		}
	}
	for _, currency := range currencies {
		c, ok := currency.(map[string]any)
		if !ok {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Array element is not a JSON object",
			}
		}
		n, found := c["name"]
		if !found {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Object field 'name' is missing",
			}
		}
		_, ok = n.(string)
		if !ok {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Object field 'name' is not a string",
			}
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func (s *CurrencyExchangeSuite) testGetCurrenciesJSONItemCode(ctx context.Context) TestResult {
	name := "GET /currencies => Response body contains JSON array of objects with 'code' fields that are strings"

	logger.Debug("Running test case", "name", name)

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/currencies", nil)
	if err != nil {
		return handleError(ctx, err, name)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return handleError(ctx, err, name)
	}
	defer resp.Body.Close()

	var body any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Response body is not a valid JSON: %s", err),
		}
	}
	currencies, ok := body.([]any)
	if !ok {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  "Response body is not a JSON array",
		}
	}
	for _, currency := range currencies {
		c, ok := currency.(map[string]any)
		if !ok {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Array element is not a JSON object",
			}
		}
		code, found := c["code"]
		if !found {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Object field 'code' is missing",
			}
		}
		_, ok = code.(string)
		if !ok {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Object field 'code' is not a string",
			}
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func (s *CurrencyExchangeSuite) testGetCurrenciesJSONItemSign(ctx context.Context) TestResult {
	name := "GET /currencies => Response body contains JSON array of objects with 'sign' fields that are strings"

	logger.Debug("Running test case", "name", name)

	time.Sleep(30 * time.Second)

	req, err := http.NewRequestWithContext(ctx, "GET", s.url+"/currencies", nil)
	if err != nil {
		return handleError(ctx, err, name)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return handleError(ctx, err, name)
	}
	defer resp.Body.Close()

	var body any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  fmt.Sprintf("Response body is not a valid JSON: %s", err),
		}
	}
	currencies, ok := body.([]any)
	if !ok {
		return TestResult{
			Name:   name,
			Status: ResultStatusFailed,
			Error:  "Response body is not a JSON array",
		}
	}
	for _, currency := range currencies {
		c, ok := currency.(map[string]any)
		if !ok {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Array element is not a JSON object",
			}
		}
		sign, found := c["sign"]
		if !found {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Object field 'sign' is missing",
			}
		}
		_, ok = sign.(string)
		if !ok {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Object field 'sign' is not a string",
			}
		}
	}
	return TestResult{Name: name, Status: ResultStatusPassed}
}

func handleError(ctx context.Context, err error, name string) TestResult {
	if cause := context.Cause(ctx); cause != nil {
		return TestResult{
			Name:   name,
			Status: ResultStatusCancelled,
			Error:  fmt.Sprintf("Cancelled due to: %s", cause),
		}
	}
	if err, found := errors.AsType[*url.Error](err); found {
		if err.Timeout() {
			return TestResult{
				Name:   name,
				Status: ResultStatusFailed,
				Error:  "Timeout",
			}
		}
	}
	logger.Error("Unknown error", "error", err, "test", name)

	return TestResult{
		Name:   name,
		Status: ResultStatusFailed,
		Error:  "Unknown error",
	}
}
