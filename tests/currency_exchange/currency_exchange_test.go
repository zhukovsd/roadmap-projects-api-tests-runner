package currencyexchange_test

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/tests"
)

var baseURL = flag.String("base-url", "http://to_be_provided:8080", "API base URL to test")
var dryRun = flag.Bool("dry-run", false, "Dry run mode, do not actually run tests")
var fatalErrorOccurred = false

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

//go:embed testdata/currencies.json
var testCurrenciesJSON []byte

//go:embed testdata/exchange_rates.json
var testExchangeRatesJSON []byte

var testCurrencies []Currency
var testExchangeRateInputs []ExchangeRateCreateInput

var apiCurrencies = make([]Currency, 0)

func init() {
	_ = json.Unmarshal(testCurrenciesJSON, &testCurrencies)
	_ = json.Unmarshal(testExchangeRatesJSON, &testExchangeRateInputs)
}

func TestHostIsReachable(t *testing.T) {
	t.Attr(tests.TestDescriptionAttr, "Деплой доступен для проверки т.е. имеет публичный IP адрес")

	if *dryRun {
		t.Skip()
	}
	u, err := url.Parse(*baseURL)
	if err != nil {
		fatalf(t, "Failed to parse URL: %s", err)
	}
	ip := net.ParseIP(u.Hostname())

	if ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			fatalf(t, "Address %q is not accessible from the public internet", u.Host)
		}
		return
	}

	addrs, err := net.LookupHost(u.Hostname())
	if err != nil {
		fatalf(t, "No addresses found for host: %q", u.Hostname())
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil {
			fatalf(t, "Invalid address %q for host %q", a, u.Hostname())
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			fatalf(t, "Address %q is not accessible from the public internet", u.Host)
		}
	}
}

func TestGetCurrencies(t *testing.T) {
	t.Attr(tests.TestDescriptionAttr, "Контейнер для GET /currencies тестов")

	resp, err := doRequest(t, http.MethodGet, "/currencies", nil)
	if err != nil {
		t.Fatal("Failed to send request")
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Failed to read response body")
	}

	var body any
	decodeErr := json.Unmarshal(bodyBytes, &body)

	assert(t, "status code is 200", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => HTTP статус код 200")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status code %d got %d", http.StatusOK, resp.StatusCode)
		}
	})
	assert(t, "no redirects", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => HTTP статус код не в диапазоне 300-400 (редиректы)")
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Unexpected redirect status code %d", resp.StatusCode)
		}
	})
	assert(t, "content type is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => HTTP заголовок Content-Type начинается с application/json")
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Expected Content-Type header %q got %q", "application/json", contentType)
		}
	})

	assert(t, "response is valid json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => Тело ответа парсится в JSON без ошибок")
		if decodeErr != nil {
			t.Errorf("Response body is not a valid JSON: %s", decodeErr)
		}
	})

	currencies, isArray := body.([]any)
	assert(t, "response is json array", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => Тело ответа JSON массив")
		if decodeErr != nil {
			t.Skipf("Response body is not a valid JSON: %s", decodeErr)
		}
		if !isArray {
			t.Error("Response body is not a JSON array")
		}
	})

	assert(t, "response array elements are valid", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => Объект JSON массива содержит id, name, code, sign поля")
		if decodeErr != nil {
			t.Skipf("Response body is not a valid JSON: %s", decodeErr)
		}
		if !isArray {
			t.Skip("Response body is not a JSON array")
		}
		for _, currency := range currencies {
			c, ok := currency.(map[string]any)
			if !ok {
				t.Error("Array element is not a JSON object")
			}
			if valid, reason := isValidCurrency(c); !valid {
				t.Error(reason)
			}
		}
	})

	assert(t, "response body matches expected schema", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => JSON тело ответа соответствует схеме в ТЗ")
		decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&apiCurrencies); err != nil {
			t.Errorf("Failed to unmarshal response body into struct (POJO/DTO): %s", err)
		}
	})
}

func TestPostCurrencies(t *testing.T) {
	t.Attr(tests.TestDescriptionAttr, "Контейнер для POST /currencies тестов")

	t.Run("success", postCurrenciesSuccess)
	t.Run("conflict", postCurrenciesConflict)
	t.Run("bad request", postCurrenciesBadRequest)
}

func postCurrenciesSuccess(t *testing.T) {
	t.Attr(tests.TestDescriptionAttr, "POST /currencies : успешное создание новой валюты")

	reqCurrency, err := findUnusedCurrency()
	if err != nil {
		t.Logf("Failed to find real currency to insert, generating random one")
		reqCurrency = generateUnusedCurrency()
	}

	form := url.Values{"code": {reqCurrency.Code}, "sign": {reqCurrency.Sign}, "name": {reqCurrency.Name}}
	resp, err := doRequest(t, http.MethodPost, "/currencies", &form)
	if err != nil {
		t.Fatal("Failed to send request")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Failed to read response body")
	}
	defer resp.Body.Close()

	var body any
	decodeErr := json.Unmarshal(bodyBytes, &body)

	assert(t, "status code is 201", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => HTTP статус код 201")
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status code %d got %d", http.StatusCreated, resp.StatusCode)
		}
	})
	assert(t, "content type is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => HTTP заголовок Content-Type начинается с application/json")
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Expected Content-Type header %q got %q", "application/json", contentType)
		}
	})

	currencyObject, isObject := body.(map[string]any)
	assert(t, "response is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => Тело ответа парсится в JSON объект без ошибок")
		if decodeErr != nil {
			t.Skipf("Response body is not a valid JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Response body is not a JSON object")
		}
	})

	assert(t, "object fields are valid", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => JSON Объект содержит id, name, code, sign поля")
		if decodeErr != nil {
			t.Skipf("Response body is not a valid JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Response body is not a JSON object")
		}
		valid, reason := isValidCurrency(currencyObject)
		if !valid {
			t.Error(reason)
		}
	})
	var respCurrency Currency
	var currencyDecodeErr error
	assert(t, "response body matches expected schema", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => JSON тело ответа соответствует схеме в ТЗ")
		decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&respCurrency); err != nil {
			currencyDecodeErr = err
			t.Errorf("Failed to unmarshal response body into struct (POJO/DTO): %s", err)
		}
	})

	assert(t, "response echoes request fields", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => JSON объект содержит отправленные name, code, sign поля")
		if decodeErr != nil {
			t.Skipf("Response body is not a valid JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Response body is not a JSON object")
		}
		if valid, reason := isValidCurrency(currencyObject); !valid {
			t.Skip(reason)
		}
		if currencyDecodeErr != nil {
			t.Skipf("Response body doesn't match expected schema: %s", currencyDecodeErr)
		}
		if respCurrency.Code != reqCurrency.Code {
			t.Errorf("Expected response currency code %q got %q", reqCurrency.Code, respCurrency.Code)
		}
		if respCurrency.Name != reqCurrency.Name {
			t.Errorf("Expected response currency name %q got %q", reqCurrency.Name, respCurrency.Name)
		}
		if respCurrency.Sign != reqCurrency.Sign {
			t.Errorf("Expected response currency sign %q got %q", reqCurrency.Sign, respCurrency.Sign)
		}
	})

	apiCurrencies = append(apiCurrencies, respCurrency)
}

func postCurrenciesConflict(t *testing.T) {
	t.Attr(tests.TestDescriptionAttr, "POST /currencies : существующая валюта")

	if len(apiCurrencies) == 0 {
		t.Skip("Unable to find currency that will trigger conflict")
	}
	currency := apiCurrencies[0]

	form := url.Values{"code": {currency.Code}, "sign": {currency.Sign}, "name": {currency.Name}}
	resp, err := doRequest(t, http.MethodPost, "/currencies", &form)
	if err != nil {
		t.Fatal("Failed to send request")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Failed to read response body")
	}
	defer resp.Body.Close()

	var body any
	decodeErr := json.Unmarshal(bodyBytes, &body)

	assert(t, "status code is 409", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies : существующая валюта => HTTP статус код 409")
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected status code %d got %d", http.StatusConflict, resp.StatusCode)
		}
	})
	assert(t, "content type is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => HTTP заголовок Content-Type начинается с application/json")
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Expected Content-Type header %q got %q", "application/json", contentType)
		}
	})

	error, isObject := body.(map[string]any)
	assert(t, "response is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies : существующая валюта => Тело ответа парсится в JSON объект без ошибок")
		if decodeErr != nil {
			t.Skipf("Response body is not a valid JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Response body is not a JSON object")
		}
	})

	assert(t, "response contains field message", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies : существующая валюта => JSON объект содержит поле message")
		if decodeErr != nil {
			t.Skipf("Response body is not a valid JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Response body is not a JSON object")
		}
		valid, reason := isValidStringField(error, "message")
		if !valid {
			t.Error(reason)
		}
	})
}

func postCurrenciesBadRequest(t *testing.T) {
	t.Attr(tests.TestDescriptionAttr, "POST /currencies : неверные поля в запросе")

	currency, err := findUnusedCurrency()
	if err != nil {
		t.Logf("Failed to find real currency to insert, generating random one")
		currency = generateUnusedCurrency()
	}

	testCases := []struct {
		subName string
		subDesc string
		form    url.Values
	}{
		{
			subName: "missing name",
			subDesc: "отсутствует параметр name",
			form:    url.Values{"code": {currency.Code}, "sign": {currency.Sign}},
		},
		{
			subName: "blank name",
			subDesc: "параметр name пустой",
			form:    url.Values{"code": {currency.Code}, "sign": {currency.Sign}, "name": {""}},
		},
		{
			subName: "missing code",
			subDesc: "отсутствует параметр code",
			form:    url.Values{"sign": {currency.Sign}, "name": {currency.Name}},
		},
		{
			subName: "blank code",
			subDesc: "параметр code пустой",
			form:    url.Values{"code": {""}, "sign": {currency.Sign}, "name": {currency.Name}},
		},
		{
			subName: "missing sign",
			subDesc: "отсутствует параметр sign",
			form:    url.Values{"code": {currency.Code}, "name": {currency.Name}},
		},
		{
			subName: "blank sign",
			subDesc: "параметр sign пустой",
			form:    url.Values{"code": {currency.Code}, "sign": {""}, "name": {currency.Name}},
		},
		{
			subName: "code too long",
			subDesc: "параметр code превышает допустимую длину",
			form:    url.Values{"code": {currency.Code + currency.Code}, "sign": {currency.Sign}, "name": {currency.Name}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.subName, func(t *testing.T) {
			resp, err := doRequest(t, http.MethodPost, "/currencies", &tc.form)
			if err != nil {
				t.Skip("Failed to send request")
				return
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal("Failed to read response body")
			}
			defer resp.Body.Close()

			var body any
			decodeErr := json.Unmarshal(bodyBytes, &body)

			assert(t, "status code is 400", func(t *testing.T) {
				t.Attr(
					tests.TestDescriptionAttr,
					fmt.Sprintf("POST /currencies : %s => HTTP статус код 400", tc.subDesc),
				)
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("Expected status code %d got %d", http.StatusBadRequest, resp.StatusCode)
				}
			})
			assert(t, "content type is json", func(t *testing.T) {
				t.Attr(
					tests.TestDescriptionAttr,
					fmt.Sprintf("POST /currencies %s => HTTP заголовок Content-Type начинается с application/json", tc.subDesc),
				)
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Expected Content-Type header %q got %q", "application/json", contentType)
				}
			})

			error, isObject := body.(map[string]any)
			assert(t, "response is json", func(t *testing.T) {
				t.Attr(
					tests.TestDescriptionAttr,
					fmt.Sprintf("POST /currencies : %s => Тело ответа парсится в JSON объект без ошибок", tc.subDesc))
				if decodeErr != nil {
					t.Skipf("Response body is not a valid JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Response body is not a JSON object")
				}
			})

			assert(t, "response contains field message", func(t *testing.T) {
				t.Attr(
					tests.TestDescriptionAttr,
					fmt.Sprintf("POST /currencies : %s => JSON объект содержит поле message", tc.subDesc),
				)
				if decodeErr != nil {
					t.Skipf("Response body is not a valid JSON: %s", decodeErr)
				}
				if !isObject {
					t.Skip("Response body is not a JSON object")
				}
				valid, reason := isValidStringField(error, "message")
				if !valid {
					t.Error(reason)
				}
			})
			_ = resp.Body.Close()
		})
	}
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
		return false, "Object field 'id' is missing"
	}
	f, ok := id.(float64)
	if !ok {
		return false, fmt.Sprintf("Object field 'id' is not a number: %q", id)
	}
	if f != math.Trunc(f) {
		return false, fmt.Sprintf("Object field 'id' is not an integer: %f", f)
	}
	return true, ""
}

func isValidCode(c map[string]any) (bool, string) {
	codeAny, found := c["code"]
	if !found {
		return false, "Object field 'code' is missing"
	}
	codeStr, ok := codeAny.(string)
	if !ok {
		return false, "Object field 'code' is not a string"
	}
	if len(codeStr) != 3 {
		return false, "Object field 'code' is not a valid currency code"
	}
	return true, ""
}

func isValidStringField(c map[string]any, field string) (bool, string) {
	n, found := c[field]
	if !found {
		return false, fmt.Sprintf("Object field '%s' is missing", field)
	}
	_, ok := n.(string)
	if !ok {
		return false, fmt.Sprintf("Object field '%s' is not a string", field)
	}
	return true, ""
}

func findUnusedCurrency() (Currency, error) {
	for _, avail := range apiCurrencies {
		for _, test := range testCurrencies {
			if avail.Code != test.Code {
				return test, nil
			}
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

func doRequest(t *testing.T, method, path string, form *url.Values) (*http.Response, error) {
	t.Helper()
	if *dryRun {
		return &http.Response{StatusCode: http.StatusTeapot, Header: http.Header{}, Body: http.NoBody}, nil
	}
	var body io.Reader = nil
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(t.Context(), method, *baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %s", err)
	}
	return resp, nil
}

func assert(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if *dryRun || fatalErrorOccurred {
			t.SkipNow()
		}
		fn(t)
	})
}

func fatalf(t *testing.T, format string, args ...any) {
	t.Helper()
	fatalErrorOccurred = true
	t.Fatalf(format, args...)
}
