package currencyexchange_test

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/tests"
)

type dumps struct {
	req  []byte
	resp []byte
}

type checker struct {
	t     *testing.T
	dumps *dumps
	err   error
}

type Currency struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
	Sign string `json:"sign"`
}

type ExchangeRate struct {
	ID             int64    `json:"id"`
	BaseCurrency   Currency `json:"baseCurrency"`
	TargetCurrency Currency `json:"targetCurrency"`
	Rate           float64  `json:"rate"`
}

type ExchangeRateCreateInput struct {
	BaseCurrencyCode   string  `json:"baseCurrencyCode"`
	TargetCurrencyCode string  `json:"targetCurrencyCode"`
	Rate               float64 `json:"rate"`
}

func doRequest(t *testing.T, method, path string, form *url.Values) (*http.Response, *dumps, error) {
	t.Helper()
	if *dryRun {
		return &http.Response{StatusCode: http.StatusTeapot, Header: http.Header{}, Body: http.NoBody}, nil, nil
	}
	var body io.Reader = nil
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(t.Context(), method, *baseURL+path, body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %s", err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	reqDump, _ := httputil.DumpRequestOut(req, true)

	resp, err := client.Do(req)
	if err != nil {
		return nil, &dumps{req: reqDump}, fmt.Errorf("failed to send request: %s", err)
	}

	respDump, _ := httputil.DumpResponse(resp, true)

	return resp, &dumps{req: reqDump, resp: respDump}, nil
}

func (c *checker) assert(name string, desc string, fn func(t *testing.T)) {
	c.t.Helper()

	c.t.Run(name, func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, desc)

		if c.dumps != nil {
			t.Log(tests.TestRequestStartMarker)
			t.Log(string(c.dumps.req))
			t.Log(tests.TestRequestEndMarker)

			t.Log(tests.TestResponseStartMarker)
			t.Log(string(c.dumps.resp))
			t.Log(tests.TestResponseEndMarker)
		}
		if c.err != nil {
			t.Skip(c.err)
		}
		if *dryRun || fatalErrorOccurred {
			t.SkipNow()
		}
		fn(t)
	})
}

func isValidExchangeRate(er map[string]any) (bool, string) {
	errors := make([]string, 0)

	if valid, err := isValidID(er); !valid {
		errors = append(errors, err)
	}

	validateCurrency := func(field string) {
		f, found := er[field]
		if !found {
			errors = append(errors, fmt.Sprintf("Отсутствует поле объекта `%s`", field))
			return
		}
		c, ok := f.(map[string]any)
		if !ok {
			errors = append(errors, fmt.Sprintf("Элемент `%s` не является JSON-объектом", field))
			return
		}
		if valid, reason := isValidCurrency(c); !valid {
			errors = append(errors, reason)
		}
	}

	validateCurrency("baseCurrency")
	validateCurrency("targetCurrency")

	rate, found := er["rate"]
	if !found {
		errors = append(errors, "Отсутствует поле объекта `rate`")
	}
	_, ok := rate.(float64)
	if !ok {
		errors = append(errors, fmt.Sprintf("Поле объекта `rate` не является числом: %q", rate))
	}

	return len(errors) == 0, strings.Join(errors, "; ")
}

func isValidCurrency(c map[string]any) (bool, string) {
	errors := make([]string, 0)
	if valid, err := isValidID(c); !valid {
		errors = append(errors, err)
	}
	if valid, err := isValidStringField(c, "name"); !valid {
		errors = append(errors, err)
	}
	if valid, err := isValidStringField(c, "sign"); !valid {
		errors = append(errors, err)
	}
	if valid, err := isValidCode(c); !valid {
		errors = append(errors, err)
	}
	return len(errors) == 0, strings.Join(errors, "; ")
}

func isValidID(c map[string]any) (bool, string) {
	id, found := c["id"]
	if !found {
		return false, "Отсутствует поле объекта `id`"
	}
	f, ok := id.(float64)
	if !ok {
		return false, fmt.Sprintf("Поле объекта `id` не является числом: %q", id)
	}
	if f != math.Trunc(f) {
		return false, fmt.Sprintf("Поле объекта `id` не является целым числом: %f", f)
	}
	return true, ""
}

func isValidCode(c map[string]any) (bool, string) {
	codeAny, found := c["code"]
	if !found {
		return false, "Отсутствует поле объекта `code`"
	}
	codeStr, ok := codeAny.(string)
	if !ok {
		return false, "Поле объекта `code` не является строкой"
	}
	if len(codeStr) != 3 {
		return false, "Поле объекта `code` не является допустимым кодом валюты"
	}
	return true, ""
}

func isValidStringField(c map[string]any, field string) (bool, string) {
	n, found := c[field]
	if !found {
		return false, fmt.Sprintf("Отсутствует поле объекта `%s`", field)
	}
	_, ok := n.(string)
	if !ok {
		return false, fmt.Sprintf("Поле объекта `%s` не является строкой", field)
	}
	return true, ""
}
