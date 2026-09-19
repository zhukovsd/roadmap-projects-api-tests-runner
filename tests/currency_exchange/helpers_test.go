package currencyexchange_test

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

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

var client = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
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

func findUnusedCurrency() (Currency, error) {
	for _, test := range testCurrencies {
		used := false
		for _, avail := range apiCurrencies {
			if avail.Code == test.Code {
				used = true
				break
			}
		}
		if !used {
			return test, nil
		}
	}
	return Currency{}, fmt.Errorf("no unused currencies found")
}

func generateUnusedCurrency() Currency {
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	var currency Currency
uniqueCurrencies:
	for {
		codeBytes := make([]byte, 3)
		for i := range codeBytes {
			codeBytes[i] = letters[rand.Intn(len(letters))]
		}
		code := string(codeBytes)

		for _, avail := range apiCurrencies {
			if avail.Code == code {
				continue uniqueCurrencies
			}
		}

		currency.Sign = string(code[0])
		currency.Code = code
		currency.Name = "Test currency " + code

		break
	}

	return currency
}

func mustMatchCurrencies(t *testing.T, got, want Currency) {
	t.Helper()

	if got.ID != want.ID {
		t.Errorf("Ожидался ID валюты %d, получен %d", want.ID, got.ID)
	}
	if got.Code != want.Code {
		t.Errorf("Ожидался код валюты %q, получен %q", want.Code, got.Code)
	}
	if got.Name != want.Name {
		t.Errorf("Ожидалось название валюты %q, получено %q", want.Name, got.Name)
	}
	if got.Sign != want.Sign {
		t.Errorf("Ожидался знак валюты %q, получен %q", want.Sign, got.Sign)
	}
}

func mustMatchExchangeRates(t *testing.T, got, want ExchangeRate) {
	t.Helper()

	if got.ID != want.ID {
		t.Errorf("Ожидался ID обменного курса %d, получен %d", want.ID, got.ID)
	}

	mustMatchCurrencies(t, got.BaseCurrency, want.BaseCurrency)
	mustMatchCurrencies(t, got.TargetCurrency, want.TargetCurrency)

	if got.Rate != want.Rate {
		t.Errorf("Ожидался обменный курс %f, получен %f", want.Rate, got.Rate)
	}
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

	if valid, err := isValidRate(er); !valid {
		errors = append(errors, err)
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

func isValidRate(c map[string]any) (bool, string) {
	rate, found := c["rate"]
	if !found {
		return false, "Отсутствует поле объекта `rate`"
	}
	_, ok := rate.(float64)
	if !ok {
		return false, fmt.Sprintf("Поле объекта `rate` не является числом: %q", rate)
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
