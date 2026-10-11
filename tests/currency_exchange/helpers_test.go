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

type Exchange struct {
	BaseCurrency    Currency `json:"baseCurrency"`
	TargetCurrency  Currency `json:"targetCurrency"`
	Rate            float64  `json:"rate"`
	Amount          float64  `json:"amount"`
	ConvertedAmount float64  `json:"convertedAmount"`
}

func doRequest(t *testing.T, method, path string, form *url.Values) (*http.Response, *dumps, error) {
	t.Helper()
	if *dryRun {
		return &http.Response{StatusCode: http.StatusTeapot, Header: http.Header{}, Body: http.NoBody}, nil, nil
	}
	if !deployAvailable {
		return nil, nil, fmt.Errorf("деплой недоступен: %s", *baseURL)
	}
	var body io.Reader = nil
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(t.Context(), method, *baseURL+path, body)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка при создании запроса: %s", err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	reqDump, _ := httputil.DumpRequestOut(req, true)

	resp, err := client.Do(req)
	if err != nil {
		return nil, &dumps{req: reqDump}, fmt.Errorf("ошибка при отправке запроса: %s", err)
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
		if *dryRun || !deployAvailable {
			t.SkipNow()
		}
		fn(t)
	})
}

func findOrGenerateUnusedCurrency() Currency {
	for _, test := range testCurrencies {
		used := false
		for _, avail := range apiCurrencies {
			if avail.Code == test.Code {
				used = true
				break
			}
		}
		if !used {
			return test
		}
	}

	return generateRandomCurrency()
}

func generateRandomCurrency() Currency {
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

func findUsedCurrency() (Currency, error) {
	if len(apiCurrencies) == 0 {
		if *dryRun {
			return Currency{}, nil
		}
		return Currency{}, fmt.Errorf("no used currencies found")
	}
	return apiCurrencies[0], nil
}

func findUnusedExchangeRate() (ExchangeRate, error) {
	if *dryRun {
		return ExchangeRate{}, nil
	}

	existingRates := make(map[string]ExchangeRate)

	for _, rate := range apiExchangeRates {
		existingRates[rate.BaseCurrency.Code+rate.TargetCurrency.Code] = rate
	}

	for _, base := range apiCurrencies {
		for _, target := range apiCurrencies {
			if target.Code == base.Code {
				continue
			}
			if _, found := existingRates[base.Code+target.Code]; !found {
				return ExchangeRate{BaseCurrency: base, TargetCurrency: target}, nil
			}
		}
	}

	return ExchangeRate{}, fmt.Errorf("no unused exchange rates found")
}

func findUsedExchangeRate() (ExchangeRate, error) {
	if len(apiExchangeRates) == 0 {
		if *dryRun {
			return ExchangeRate{322, Currency{}, Currency{}, 0.333333}, nil
		}
		return ExchangeRate{}, fmt.Errorf("no used exchange rates found")
	}
	return apiExchangeRates[0], nil
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
		t.Errorf("Ожидался обменный курс %v, получен %v", want.Rate, got.Rate)
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

	if valid, err := isValidFloatField(er, "rate"); !valid {
		errors = append(errors, err)
	}

	return len(errors) == 0, strings.Join(errors, "; ")
}

func isValidExchange(er map[string]any) (bool, string) {
	errors := make([]string, 0)

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

	if valid, err := isValidFloatField(er, "rate"); !valid {
		errors = append(errors, err)
	}
	if valid, err := isValidFloatField(er, "amount"); !valid {
		errors = append(errors, err)
	}
	if valid, err := isValidFloatField(er, "convertedAmount"); !valid {
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
		return false, fmt.Sprintf("Поле объекта `id` не является целым числом: %v", f)
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
		return false, fmt.Sprintf("Поле объекта `code` не является строкой: %q", codeAny)
	}
	if len(codeStr) != 3 {
		return false, fmt.Sprintf("Поле объекта `code` не является допустимым кодом валюты: %q", codeStr)
	}
	return true, ""
}

func isValidFloatField(c map[string]any, field string) (bool, string) {
	f, found := c[field]
	if !found {
		return false, fmt.Sprintf("Отсутствует поле объекта `%s`", field)
	}
	float, ok := f.(float64)
	if !ok {
		return false, fmt.Sprintf("Поле объекта `%s` не является числом: %q", field, f)
	}
	if float < 0 {
		return false, fmt.Sprintf("Поле объекта `%s` не может быть отрицательным: %v", field, float)
	}
	return true, ""
}

func isValidStringField(c map[string]any, field string) (bool, string) {
	f, found := c[field]
	if !found {
		return false, fmt.Sprintf("Отсутствует поле объекта `%s`", field)
	}
	_, ok := f.(string)
	if !ok {
		return false, fmt.Sprintf("Поле объекта `%s` не является строкой", field)
	}
	return true, ""
}

// Required rounding is two decimal places, e.g. 11.22
//
// For values starting with 64.32..., only 64.32 or 64.33 are possible
// regardless of input precision or rounding mode.
func isValidRounding(actual, expected float64) error {
	roundDown := math.Floor(expected*100) / 100
	roundUp := math.Ceil(expected*100) / 100

	if actual == roundDown || actual == roundUp {
		return nil
	}

	return fmt.Errorf(
		"Ожидалось корректное округление до двух десятичных знаков: %v или %v, получено %v",
		roundDown, roundUp, actual,
	)
}

func prefix(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
