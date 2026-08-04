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
		fatalf(t, "Не удалось разобрать URL: %s", err)
	}
	ip := net.ParseIP(u.Hostname())

	if ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			fatalf(t, "Адрес %q недоступен из публичного интернета", u.Host)
		}
		return
	}

	addrs, err := net.LookupHost(u.Hostname())
	if err != nil {
		fatalf(t, "Не найдено адресов для хоста: %q", u.Hostname())
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil {
			fatalf(t, "Недопустимый адрес %q для хоста %q", a, u.Hostname())
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			fatalf(t, "Адрес %q недоступен из публичного интернета", u.Host)
		}
	}
}

func TestGetCurrencies(t *testing.T) {
	t.Attr(tests.TestDescriptionAttr, "Контейнер для GET /currencies тестов")

	resp, err := doRequest(t, http.MethodGet, "/currencies", nil)
	if err != nil {
		t.Fatal("Не удалось отправить запрос")
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Не удалось прочитать тело ответа")
	}

	var body any
	decodeErr := json.Unmarshal(bodyBytes, &body)

	assert(t, "status code is 200", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => HTTP статус код 200")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Ожидался код статуса %d, получен %d", http.StatusOK, resp.StatusCode)
		}
	})
	assert(t, "no redirects", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => HTTP статус код не в диапазоне 300-400 (редиректы)")
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Неожиданный код статуса редиректа %d", resp.StatusCode)
		}
	})
	assert(t, "content type is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => HTTP заголовок Content-Type начинается с application/json")
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	assert(t, "response is valid json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => Тело ответа парсится в JSON без ошибок")
		if decodeErr != nil {
			t.Errorf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
	})

	currencies, isArray := body.([]any)
	assert(t, "response is json array", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => Тело ответа JSON массив")
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isArray {
			t.Error("Тело ответа не является JSON-массивом")
		}
	})

	assert(t, "response array elements are valid", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "GET /currencies => Объект JSON массива содержит id, name, code, sign поля")
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isArray {
			t.Skip("Тело ответа не является JSON-массивом")
		}
		for _, currency := range currencies {
			c, ok := currency.(map[string]any)
			if !ok {
				t.Error("Элемент массива не является JSON-объектом")
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
			t.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %s", err)
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
		t.Logf("Не удалось найти реальную валюту для вставки, генерируется случайная")
		reqCurrency = generateUnusedCurrency()
	}

	form := url.Values{"code": {reqCurrency.Code}, "sign": {reqCurrency.Sign}, "name": {reqCurrency.Name}}
	resp, err := doRequest(t, http.MethodPost, "/currencies", &form)
	if err != nil {
		t.Fatal("Не удалось отправить запрос")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Не удалось прочитать тело ответа")
	}
	defer resp.Body.Close()

	var body any
	decodeErr := json.Unmarshal(bodyBytes, &body)

	assert(t, "status code is 201", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => HTTP статус код 201")
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Ожидался код статуса %d, получен %d", http.StatusCreated, resp.StatusCode)
		}
	})
	assert(t, "content type is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => HTTP заголовок Content-Type начинается с application/json")
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	currencyObject, isObject := body.(map[string]any)
	assert(t, "response is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => Тело ответа парсится в JSON объект без ошибок")
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	assert(t, "object fields are valid", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => JSON Объект содержит id, name, code, sign поля")
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Тело ответа не является JSON-объектом")
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
			t.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %s", err)
		}
	})

	assert(t, "response echoes request fields", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => JSON объект содержит отправленные name, code, sign поля")
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Тело ответа не является JSON-объектом")
		}
		if valid, reason := isValidCurrency(currencyObject); !valid {
			t.Skip(reason)
		}
		if currencyDecodeErr != nil {
			t.Skipf("Тело ответа не соответствует ожидаемой схеме: %s", currencyDecodeErr)
		}
		if respCurrency.Code != reqCurrency.Code {
			t.Errorf("Ожидался код валюты %q, получен %q", reqCurrency.Code, respCurrency.Code)
		}
		if respCurrency.Name != reqCurrency.Name {
			t.Errorf("Ожидалось название валюты %q, получено %q", reqCurrency.Name, respCurrency.Name)
		}
		if respCurrency.Sign != reqCurrency.Sign {
			t.Errorf("Ожидался знак валюты %q, получен %q", reqCurrency.Sign, respCurrency.Sign)
		}
	})

	apiCurrencies = append(apiCurrencies, respCurrency)
}

func postCurrenciesConflict(t *testing.T) {
	t.Attr(tests.TestDescriptionAttr, "POST /currencies : существующая валюта")

	if len(apiCurrencies) == 0 {
		t.Skip("Не удалось найти валюту, вызывающую конфликт")
	}
	currency := apiCurrencies[0]

	form := url.Values{"code": {currency.Code}, "sign": {currency.Sign}, "name": {currency.Name}}
	resp, err := doRequest(t, http.MethodPost, "/currencies", &form)
	if err != nil {
		t.Fatal("Не удалось отправить запрос")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Не удалось прочитать тело ответа")
	}
	defer resp.Body.Close()

	var body any
	decodeErr := json.Unmarshal(bodyBytes, &body)

	assert(t, "status code is 409", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies : существующая валюта => HTTP статус код 409")
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Ожидался код статуса %d, получен %d", http.StatusConflict, resp.StatusCode)
		}
	})
	assert(t, "content type is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies => HTTP заголовок Content-Type начинается с application/json")
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	error, isObject := body.(map[string]any)
	assert(t, "response is json", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies : существующая валюта => Тело ответа парсится в JSON объект без ошибок")
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	assert(t, "response contains field message", func(t *testing.T) {
		t.Attr(tests.TestDescriptionAttr, "POST /currencies : существующая валюта => JSON объект содержит поле message")
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Тело ответа не является JSON-объектом")
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
		t.Logf("Не удалось найти реальную валюту для вставки, генерируется случайная")
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
				t.Skip("Не удалось отправить запрос")
				return
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal("Не удалось прочитать тело ответа")
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
					t.Errorf("Ожидался код статуса %d, получен %d", http.StatusBadRequest, resp.StatusCode)
				}
			})
			assert(t, "content type is json", func(t *testing.T) {
				t.Attr(
					tests.TestDescriptionAttr,
					fmt.Sprintf("POST /currencies %s => HTTP заголовок Content-Type начинается с application/json", tc.subDesc),
				)
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
				}
			})

			error, isObject := body.(map[string]any)
			assert(t, "response is json", func(t *testing.T) {
				t.Attr(
					tests.TestDescriptionAttr,
					fmt.Sprintf("POST /currencies : %s => Тело ответа парсится в JSON объект без ошибок", tc.subDesc))
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Тело ответа не является JSON-объектом")
				}
			})

			assert(t, "response contains field message", func(t *testing.T) {
				t.Attr(
					tests.TestDescriptionAttr,
					fmt.Sprintf("POST /currencies : %s => JSON объект содержит поле message", tc.subDesc),
				)
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Skip("Тело ответа не является JSON-объектом")
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
		return false, "Отсутствует поле объекта 'id'"
	}
	f, ok := id.(float64)
	if !ok {
		return false, fmt.Sprintf("Поле объекта 'id' не является числом: %q", id)
	}
	if f != math.Trunc(f) {
		return false, fmt.Sprintf("Поле объекта 'id' не является целым числом: %f", f)
	}
	return true, ""
}

func isValidCode(c map[string]any) (bool, string) {
	codeAny, found := c["code"]
	if !found {
		return false, "Отсутствует поле объекта 'code'"
	}
	codeStr, ok := codeAny.(string)
	if !ok {
		return false, "Поле объекта 'code' не является строкой"
	}
	if len(codeStr) != 3 {
		return false, "Поле объекта 'code' не является допустимым кодом валюты"
	}
	return true, ""
}

func isValidStringField(c map[string]any, field string) (bool, string) {
	n, found := c[field]
	if !found {
		return false, fmt.Sprintf("Отсутствует поле объекта '%s'", field)
	}
	_, ok := n.(string)
	if !ok {
		return false, fmt.Sprintf("Поле объекта '%s' не является строкой", field)
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
