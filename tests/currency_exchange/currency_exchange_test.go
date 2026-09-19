package currencyexchange_test

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/tests"
)

var baseURL = flag.String("base-url", "http://to_be_provided:8080", "API base URL to test")
var dryRun = flag.Bool("dry-run", false, "Dry run mode, do not actually run tests")
var fatalErrorOccurred = false

//go:embed testdata/currencies.json
var testCurrenciesJSON []byte

//go:embed testdata/exchange_rates.json
var testExchangeRatesJSON []byte

var testCurrencies []Currency
var testExchangeRateInputs []ExchangeRateCreateInput

var apiCurrencies []Currency
var apiExchangeRates []ExchangeRate

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
	resp, dumps, err := doRequest(t, http.MethodGet, "/currencies", nil)

	var bodyBytes []byte
	var body any
	var decodeErr error

	if err != nil {
		err = fmt.Errorf("Не удалось отправить запрос: %w", err)
	} else {
		defer resp.Body.Close()
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
		} else {
			decodeErr = json.Unmarshal(bodyBytes, &body)
		}
	}

	c := &checker{t, dumps, err}

	c.assert("status code is 200", "GET /currencies => HTTP статус код 200", func(t *testing.T) {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusOK, resp.StatusCode)
		}
	})
	c.assert("no redirects", "GET /currencies => HTTP статус код не в диапазоне 300-399 (редиректы)", func(t *testing.T) {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Неожиданный статус код редиректа %d", resp.StatusCode)
		}
	})
	c.assert("content type is json", "GET /currencies => HTTP заголовок Content-Type начинается с application/json", func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	c.assert("response is valid json", "GET /currencies => Тело ответа парсится в JSON без ошибок", func(t *testing.T) {
		if decodeErr != nil {
			t.Errorf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
	})

	currencies, isArray := body.([]any)
	c.assert("response is json array", "GET /currencies => Тело ответа JSON массив", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isArray {
			t.Error("Тело ответа не является JSON-массивом")
		}
	})

	c.assert("response array elements are valid", "GET /currencies => Объект JSON массива содержит id, name, code, sign поля", func(t *testing.T) {
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

	c.assert("response body matches expected schema", "GET /currencies => JSON тело ответа соответствует схеме в ТЗ", func(t *testing.T) {
		decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&apiCurrencies); err != nil {
			t.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %s", err)
		}
	})
}

func TestPostCurrencies(t *testing.T) {
	t.Run("success", postCurrenciesSuccess)
	t.Run("conflict", postCurrenciesConflict)
	t.Run("bad request", postCurrenciesBadRequest)
}

func postCurrenciesSuccess(t *testing.T) {
	reqCurrency, err := findUnusedCurrency()
	if err != nil {
		t.Logf("Не удалось найти реальную валюту для вставки, генерируется случайная")
		reqCurrency = generateUnusedCurrency()
	}

	form := url.Values{"code": {reqCurrency.Code}, "sign": {reqCurrency.Sign}, "name": {reqCurrency.Name}}
	resp, dumps, err := doRequest(t, http.MethodPost, "/currencies", &form)

	var bodyBytes []byte
	var body any
	var decodeErr error

	if err != nil {
		err = fmt.Errorf("Не удалось отправить запрос: %w", err)
	} else {
		defer resp.Body.Close()
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
		} else {
			decodeErr = json.Unmarshal(bodyBytes, &body)
		}
	}

	c := &checker{t, dumps, err}

	c.assert("status code is 201", "POST /currencies => HTTP статус код 201", func(t *testing.T) {
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusCreated, resp.StatusCode)
		}
	})
	c.assert("content type is json", "POST /currencies => HTTP заголовок Content-Type начинается с application/json", func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	currencyObject, isObject := body.(map[string]any)
	c.assert("response is json", "POST /currencies => Тело ответа парсится в JSON объект без ошибок", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	c.assert("object fields are valid", "POST /currencies => JSON Объект содержит id, name, code, sign поля", func(t *testing.T) {
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
	c.assert("response body matches expected schema", "POST /currencies => JSON тело ответа соответствует схеме в ТЗ", func(t *testing.T) {
		decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&respCurrency); err != nil {
			currencyDecodeErr = err
			t.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %s", err)
		}
	})

	c.assert("response echoes request fields", "POST /currencies => JSON объект содержит отправленные name, code, sign поля", func(t *testing.T) {
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
	if !*dryRun && len(apiCurrencies) == 0 {
		t.Skip("Не удалось найти валюту, вызывающую конфликт")
	}
	currency := apiCurrencies[0]

	form := url.Values{"code": {currency.Code}, "sign": {currency.Sign}, "name": {currency.Name}}
	resp, dumps, err := doRequest(t, http.MethodPost, "/currencies", &form)

	var bodyBytes []byte
	var body any
	var decodeErr error

	if err != nil {
		err = fmt.Errorf("Не удалось отправить запрос: %w", err)
	} else {
		defer resp.Body.Close()
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
		} else {
			decodeErr = json.Unmarshal(bodyBytes, &body)
		}
	}

	c := &checker{t, dumps, err}

	c.assert("status code is 409", "POST /currencies : существующая валюта => HTTP статус код 409", func(t *testing.T) {
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusConflict, resp.StatusCode)
		}
	})
	c.assert("content type is json", "POST /currencies => HTTP заголовок Content-Type начинается с application/json", func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	error, isObject := body.(map[string]any)
	c.assert("response is json", "POST /currencies : существующая валюта => Тело ответа парсится в JSON объект без ошибок", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	c.assert("response contains field message", "POST /currencies : существующая валюта => JSON объект содержит поле message", func(t *testing.T) {
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
			t.Parallel()
			resp, dumps, err := doRequest(t, http.MethodPost, "/currencies", &tc.form)

			var bodyBytes []byte
			var body any
			var decodeErr error

			if err != nil {
				err = fmt.Errorf("Не удалось отправить запрос: %w", err)
			} else {
				defer resp.Body.Close()
				bodyBytes, err = io.ReadAll(resp.Body)
				if err != nil {
					err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
				} else {
					decodeErr = json.Unmarshal(bodyBytes, &body)
				}
			}

			c := &checker{t, dumps, err}

			c.assert("status code is 400", fmt.Sprintf("POST /currencies : %s => HTTP статус код 400", tc.subDesc), func(t *testing.T) {
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("Ожидался статус код %d, получен %d", http.StatusBadRequest, resp.StatusCode)
				}
			})
			c.assert("content type is json", fmt.Sprintf("POST /currencies : %s => HTTP заголовок Content-Type начинается с application/json", tc.subDesc), func(t *testing.T) {
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
				}
			})

			error, isObject := body.(map[string]any)
			c.assert("response is json", fmt.Sprintf("POST /currencies : %s => Тело ответа парсится в JSON объект без ошибок", tc.subDesc), func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Тело ответа не является JSON-объектом")
				}
			})

			c.assert("response contains field message", fmt.Sprintf("POST /currencies : %s => JSON объект содержит поле message", tc.subDesc), func(t *testing.T) {
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
		})
	}
}

func TestGetCurrency(t *testing.T) {
	t.Run("success", getCurrencySuccess)
	t.Run("not found", getCurrencyNotFound)
	t.Run("bad request", getCurrencyBadRequest)
}

func getCurrencySuccess(t *testing.T) {
	var reqCurrency Currency

	if !*dryRun {
		if len(apiCurrencies) == 0 {
			t.Skip("Не удалось найти валюту, существующую в API")
		}
		reqCurrency = apiCurrencies[0]
	}

	injectCode := strings.NewReplacer("{code}", reqCurrency.Code).Replace

	resp, dumps, err := doRequest(t, http.MethodGet, injectCode("/currency/{code}"), nil)

	var bodyBytes []byte
	var body any
	var decodeErr error

	if err != nil {
		err = fmt.Errorf("Не удалось отправить запрос: %w", err)
	} else {
		defer resp.Body.Close()
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
		} else {
			decodeErr = json.Unmarshal(bodyBytes, &body)
		}
	}

	c := &checker{t, dumps, err}

	c.assert("status code is 200", injectCode("GET /currency/{code} => HTTP статус код 200"), func(t *testing.T) {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusOK, resp.StatusCode)
		}
	})
	c.assert("no redirects", injectCode("GET /currency/{code} => HTTP статус код не в диапазоне 300-399 (редиректы)"), func(t *testing.T) {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Неожиданный статус код редиректа %d", resp.StatusCode)
		}
	})
	c.assert("content type is json", injectCode("GET /currency/{code} => HTTP заголовок Content-Type начинается с application/json"), func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	currencyObject, isObject := body.(map[string]any)
	c.assert("response is json", injectCode("GET /currency/{code} => Тело ответа парсится в JSON объект без ошибок"), func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	c.assert("object fields are valid", injectCode("GET /currency/{code} => JSON Объект содержит id, name, code, sign поля"), func(t *testing.T) {
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
	c.assert("response body matches expected schema", injectCode("GET /currency/{code} => JSON тело ответа соответствует схеме в ТЗ"), func(t *testing.T) {
		decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&respCurrency); err != nil {
			currencyDecodeErr = err
			t.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %s", err)
		}
	})

	c.assert("response echoes requested currency", injectCode("GET /currency/{code} => JSON объект содержит ожидаемые id, name, code, sign поля"), func(t *testing.T) {
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
		mustMatchCurrencies(t, respCurrency, reqCurrency)
	})
}

func getCurrencyNotFound(t *testing.T) {
	currency, err := findUnusedCurrency()
	if err != nil {
		t.Logf("Не удалось найти реальную валюту, отсутствующую в API, генерируется случайная")
		currency = generateUnusedCurrency()
	}

	injectCode := strings.NewReplacer("{code}", currency.Code).Replace

	resp, dumps, err := doRequest(t, http.MethodGet, injectCode("/currency/{code}"), nil)

	var bodyBytes []byte
	var body any
	var decodeErr error

	if err != nil {
		err = fmt.Errorf("Не удалось отправить запрос: %w", err)
	} else {
		defer resp.Body.Close()
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
		} else {
			decodeErr = json.Unmarshal(bodyBytes, &body)
		}
	}

	c := &checker{t, dumps, err}

	c.assert("status code is 404", injectCode("GET /currency/{code} : несуществующая валюта => HTTP статус код 404"), func(t *testing.T) {
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusNotFound, resp.StatusCode)
		}
	})
	c.assert("no redirects", injectCode("GET /currency/{code} : несуществующая валюта => HTTP статус код не в диапазоне 300-399 (редиректы)"), func(t *testing.T) {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Неожиданный статус код редиректа %d", resp.StatusCode)
		}
	})
	c.assert("content type is json", injectCode("GET /currency/{code} : несуществующая валюта => HTTP заголовок Content-Type начинается с application/json"), func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	error, isObject := body.(map[string]any)
	c.assert("response is json", injectCode("GET /currency/{code} : несуществующая валюта => Тело ответа парсится в JSON объект без ошибок"), func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	c.assert("response contains field message", injectCode("GET /currency/{code} : несуществующая валюта => JSON объект содержит поле message"), func(t *testing.T) {
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

func getCurrencyBadRequest(t *testing.T) {
	type testCase struct {
		name string
		desc string
		code string
	}

	var code string

	if len(apiCurrencies) != 0 {
		code = apiCurrencies[0].Code
	} else {
		t.Logf("Не удалось найти валюту, существующую в API, генерируется случайная")
		code = generateUnusedCurrency().Code
	}
	if *dryRun {
		code = "XDD"
	}

	lowerCode := strings.ToLower(code)
	mixedCode := strings.ToLower(code[:1]) + code[1:]

	testCases := []testCase{
		{
			name: "missing code",
			desc: "код валюты отсутствует в адресе",
			code: "",
		},
		{
			name: "single character",
			desc: "код валюты состоит из одного символа",
			code: prefix(code, 1),
		},
		{
			name: "code too short",
			desc: "код валюты короче допустимой длины",
			code: prefix(code, 2),
		},
		{
			name: "code too long",
			desc: "код валюты превышает допустимую длину",
			code: code + "X",
		},
		{
			name: "code way too long",
			desc: "код валюты значительно превышает допустимую длину",
			code: strings.Repeat(code, 10),
		},
		{
			name: "code with digits",
			desc: "код валюты содержит цифры",
			code: prefix(code, 2) + "1",
		},
		{
			name: "code with special characters",
			desc: "код валюты содержит специальные символы",
			code: prefix(code, 2) + "-",
		},
		{
			name: "whitespace only code",
			desc: "код валюты состоит только из пробелов",
			code: "   ",
		},
		{
			name: "lowercase code",
			desc: "код существующей валюты в нижнем регистре",
			code: lowerCode,
		},
		{
			name: "mixed case code",
			desc: "код существующей валюты в смешанном регистре",
			code: mixedCode,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			injectCode := strings.NewReplacer("{code}", tc.code).Replace

			resp, dumps, err := doRequest(t, http.MethodGet, injectCode("/currency/{code}"), nil)

			var bodyBytes []byte
			var body any
			var decodeErr error

			if err != nil {
				err = fmt.Errorf("Не удалось отправить запрос: %w", err)
			} else {
				defer resp.Body.Close()
				bodyBytes, err = io.ReadAll(resp.Body)
				if err != nil {
					err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
				} else {
					decodeErr = json.Unmarshal(bodyBytes, &body)
				}
			}

			c := &checker{t, dumps, err}

			descPrefix := fmt.Sprintf(injectCode("GET /currency/{code} : %s =>"), tc.desc)

			c.assert("status code is 400", fmt.Sprintf("%s HTTP статус код 400", descPrefix), func(t *testing.T) {
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("Ожидался статус код %d, получен %d", http.StatusBadRequest, resp.StatusCode)
				}
			})
			c.assert("content type is json", fmt.Sprintf("%s HTTP заголовок Content-Type начинается с application/json", descPrefix), func(t *testing.T) {
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
				}
			})

			error, isObject := body.(map[string]any)
			c.assert("response is json", fmt.Sprintf("%s Тело ответа парсится в JSON объект без ошибок", descPrefix), func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Тело ответа не является JSON-объектом")
				}
			})

			c.assert("response contains field message", fmt.Sprintf("%s JSON объект содержит поле message", descPrefix), func(t *testing.T) {
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
		})
	}
}

func TestGetExchangeRates(t *testing.T) {
	resp, dumps, err := doRequest(t, http.MethodGet, "/exchangeRates", nil)

	var bodyBytes []byte
	var body any
	var decodeErr error

	if err != nil {
		err = fmt.Errorf("Не удалось отправить запрос: %w", err)
	} else {
		defer resp.Body.Close()
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
		} else {
			decodeErr = json.Unmarshal(bodyBytes, &body)
		}
	}

	c := &checker{t, dumps, err}

	c.assert("status code is 200", "GET /exchangeRates => HTTP статус код 200", func(t *testing.T) {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusOK, resp.StatusCode)
		}
	})
	c.assert("no redirects", "GET /exchangeRates => HTTP статус код не в диапазоне 300-399 (редиректы)", func(t *testing.T) {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Неожиданный статус код редиректа %d", resp.StatusCode)
		}
	})
	c.assert("content type is json", "GET /exchangeRates => HTTP заголовок Content-Type начинается с application/json", func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	c.assert("response is valid json", "GET /exchangeRates => Тело ответа парсится в JSON без ошибок", func(t *testing.T) {
		if decodeErr != nil {
			t.Errorf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
	})

	exchangeRates, isArray := body.([]any)
	c.assert("response is json array", "GET /exchangeRates => Тело ответа JSON массив", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isArray {
			t.Error("Тело ответа не является JSON-массивом")
		}
	})

	c.assert("response array elements are valid", "GET /exchangeRates => Объект JSON массива содержит id, baseCurrency, targetCurrency, rate поля", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isArray {
			t.Skip("Тело ответа не является JSON-массивом")
		}
		for _, exchangeRate := range exchangeRates {
			er, ok := exchangeRate.(map[string]any)
			if !ok {
				t.Error("Элемент массива не является JSON-объектом")
			}
			if valid, reason := isValidExchangeRate(er); !valid {
				t.Error(reason)
			}
		}
	})

	c.assert("response body matches expected schema", "GET /exchangeRates => JSON тело ответа соответствует схеме в ТЗ", func(t *testing.T) {
		decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&apiExchangeRates); err != nil {
			t.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %s", err)
		}
	})
}

func TestGetExchangeRate(t *testing.T) {
	t.Run("success", getExchangeRateSuccess)
	t.Run("not found", getExchangeRateNotFound)
	t.Run("bad request", getExchangeRateBadRequest)
}

func getExchangeRateSuccess(t *testing.T) {
	var reqExchangeRate ExchangeRate

	if !*dryRun {
		if len(apiExchangeRates) == 0 {
			t.Skip("Не удалось найти обменный курс, существующий в API")
		}
		reqExchangeRate = apiExchangeRates[0]
	}

	injectCodes := strings.NewReplacer(
		"{base}", reqExchangeRate.BaseCurrency.Code,
		"{target}", reqExchangeRate.TargetCurrency.Code,
	).Replace

	resp, dumps, err := doRequest(t, http.MethodGet, injectCodes("/exchangeRate/{base}{target}"), nil)

	var bodyBytes []byte
	var body any
	var decodeErr error

	if err != nil {
		err = fmt.Errorf("Не удалось отправить запрос: %w", err)
	} else {
		defer resp.Body.Close()
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
		} else {
			decodeErr = json.Unmarshal(bodyBytes, &body)
		}
	}

	c := &checker{t, dumps, err}

	c.assert("status code is 200", injectCodes("GET /exchangeRate/{base}{target} => HTTP статус код 200"), func(t *testing.T) {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusOK, resp.StatusCode)
		}
	})
	c.assert("no redirects", injectCodes("GET /exchangeRate/{base}{target} => HTTP статус код не в диапазоне 300-399 (редиректы)"), func(t *testing.T) {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Неожиданный статус код редиректа %d", resp.StatusCode)
		}
	})
	c.assert("content type is json", injectCodes("GET /exchangeRate/{base}{target} => HTTP заголовок Content-Type начинается с application/json"), func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, injectCodes("application/json")) {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", injectCodes("application/json"), contentType)
		}
	})

	exchangeRateObject, isObject := body.(map[string]any)
	c.assert("response is json", injectCodes("GET /exchangeRate/{base}{target} => Тело ответа парсится в JSON объект без ошибок"), func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	c.assert("object fields are valid", injectCodes("GET /exchangeRate/{base}{target} => JSON Объект содержит id, baseCurrency, targetCurrency, rate поля"), func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Тело ответа не является JSON-объектом")
		}
		valid, reason := isValidExchangeRate(exchangeRateObject)
		if !valid {
			t.Error(reason)
		}
	})

	var respExchangeRate ExchangeRate
	var exchangeRateDecodeErr error
	c.assert("response body matches expected schema", injectCodes("GET /exchangeRate/{base}{target} => JSON тело ответа соответствует схеме в ТЗ"), func(t *testing.T) {
		decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&respExchangeRate); err != nil {
			exchangeRateDecodeErr = err
			t.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %s", err)
		}
	})

	c.assert("response echoes requested exchange rate", injectCodes("GET /exchangeRate/{base}{target} => JSON объект содержит ожидаемые id, baseCurrency, targetCurrency, rate поля"), func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Тело ответа не является JSON-объектом")
		}
		if valid, reason := isValidCurrency(exchangeRateObject); !valid {
			t.Skip(reason)
		}
		if exchangeRateDecodeErr != nil {
			t.Skipf("Тело ответа не соответствует ожидаемой схеме: %s", exchangeRateDecodeErr)
		}
		mustMatchExchangeRates(t, respExchangeRate, reqExchangeRate)
	})
}

func getExchangeRateNotFound(t *testing.T) {
	base, target, err := findUnusedExchangeRate()

	if !*dryRun && err != nil {
		t.Skip("Не удалось найти обменный курс, несуществующий в API")
	}

	injectCodes := strings.NewReplacer("{base}", base, "{target}", target).Replace

	resp, dumps, err := doRequest(t, http.MethodGet, injectCodes("/exchangeRate/{base}{target}"), nil)

	var bodyBytes []byte
	var body any
	var decodeErr error

	if err != nil {
		err = fmt.Errorf("Не удалось отправить запрос: %w", err)
	} else {
		defer resp.Body.Close()
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
		} else {
			decodeErr = json.Unmarshal(bodyBytes, &body)
		}
	}

	c := &checker{t, dumps, err}

	c.assert("status code is 404", injectCodes("GET /exchangeRate/{base}{target} : несуществующий курс => HTTP статус код 404"), func(t *testing.T) {
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusNotFound, resp.StatusCode)
		}
	})
	c.assert("no redirects", injectCodes("GET /exchangeRate/{base}{target} : несуществующий курс => HTTP статус код не в диапазоне 300-399 (редиректы)"), func(t *testing.T) {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Неожиданный статус код редиректа %d", resp.StatusCode)
		}
	})
	c.assert("content type is json", injectCodes("GET /exchangeRate/{base}{target} : несуществующий курс => HTTP заголовок Content-Type начинается с application/json"), func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, injectCodes("application/json")) {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", injectCodes("application/json"), contentType)
		}
	})

	error, isObject := body.(map[string]any)
	c.assert("response is json", injectCodes("GET /exchangeRate/{base}{target} : несуществующий курс => Тело ответа парсится в JSON объект без ошибок"), func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	c.assert("response contains field message", injectCodes("GET /exchangeRate/{base}{target} : несуществующий курс => JSON объект содержит поле message"), func(t *testing.T) {
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

func getExchangeRateBadRequest(t *testing.T) {
	type testCase struct {
		name   string
		desc   string
		base   string
		target string
	}

	var base, target string

	if len(apiExchangeRates) == 0 {
		t.Logf("Не удалось найти обменный курс, существующий в API, генерируется случайный")
		base = generateUnusedCurrency().Code
		target = generateUnusedCurrency().Code
	}

	testCases := []testCase{
		{
			name:   "missing codes",
			desc:   "коды валют отсутствуют в адресе",
			base:   "",
			target: "",
		},
		{
			name:   "one code",
			desc:   "один код валюты, второй отсутствует",
			base:   base,
			target: "",
		},
		{
			name:   "single character",
			desc:   "один символ вместо двух кодов валют",
			base:   prefix(base, 1),
			target: "",
		},
		{
			name:   "both codes too short",
			desc:   "оба кода валют короче допустимой длины",
			base:   prefix(base, 2),
			target: prefix(target, 2),
		},
		{
			name:   "base code too short",
			desc:   "код базовой валюты короче допустимой длины",
			base:   prefix(base, 2),
			target: target,
		},
		{
			name:   "target code too short",
			desc:   "код целевой валюты короче допустимой длины",
			base:   base,
			target: prefix(target, 2),
		},
		{
			name:   "base code too long",
			desc:   "код базовой валюты превышает допустимую длину",
			base:   base + prefix(base, 1),
			target: target,
		},
		{
			name:   "target code too long",
			desc:   "код целевой валюты превышает допустимую длину",
			base:   base,
			target: target + prefix(target, 1),
		},
		{
			name:   "codes too long",
			desc:   "оба кода валют значительно превышают допустимую длину",
			base:   strings.Repeat(base, 5),
			target: strings.Repeat(target, 5),
		},
		{
			name:   "codes with digits",
			desc:   "коды валют содержат цифры",
			base:   prefix(base, 2) + "1",
			target: prefix(target, 2) + "2",
		},
		{
			name:   "codes with special characters",
			desc:   "коды валют содержат специальные символы",
			base:   prefix(base, 2) + "-",
			target: prefix(target, 2) + "-",
		},
		{
			name:   "whitespace only codes",
			desc:   "коды валют состоят только из пробелов",
			base:   "   ",
			target: "   ",
		},
		{
			name:   "lowercase codes",
			desc:   "оба кода валют в нижнем регистре",
			base:   strings.ToLower(base),
			target: strings.ToLower(target),
		},
		{
			name:   "lowercase base code",
			desc:   "код базовой валюты в нижнем регистре",
			base:   strings.ToLower(base),
			target: target,
		},
		{
			name:   "lowercase target code",
			desc:   "код целевой валюты в нижнем регистре",
			base:   base,
			target: strings.ToLower(target),
		},
		{
			name:   "mixed case codes",
			desc:   "коды валют в смешанном регистре",
			base:   lowerFirst(base),
			target: lowerFirst(target),
		},
		{
			name:   "duplicate code",
			desc:   "базовая и целевая валюты совпадают",
			base:   base,
			target: base,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			injectCodes := strings.NewReplacer(
				"{base}", tc.base,
				"{target}", tc.target,
			).Replace

			resp, dumps, err := doRequest(
				t,
				http.MethodGet,
				injectCodes("/exchangeRate/{base}{target}"),
				nil,
			)

			var bodyBytes []byte
			var body any
			var decodeErr error

			if err != nil {
				err = fmt.Errorf("Не удалось отправить запрос: %w", err)
			} else {
				defer resp.Body.Close()
				bodyBytes, err = io.ReadAll(resp.Body)
				if err != nil {
					err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
				} else {
					decodeErr = json.Unmarshal(bodyBytes, &body)
				}
			}

			c := &checker{t, dumps, err}

			descPrefix := fmt.Sprintf(injectCodes("GET /exchangeRate/{base}{target} : %s =>"), tc.desc)

			c.assert("status code is 400", fmt.Sprintf("%s HTTP статус код 400", descPrefix), func(t *testing.T) {
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("Ожидался статус код %d, получен %d", http.StatusBadRequest, resp.StatusCode)
				}
			})
			c.assert("content type is json", fmt.Sprintf("%s HTTP заголовок Content-Type начинается с application/json", descPrefix), func(t *testing.T) {
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
				}
			})

			error, isObject := body.(map[string]any)
			c.assert("response is json", fmt.Sprintf("%s Тело ответа парсится в JSON объект без ошибок", descPrefix), func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Тело ответа не является JSON-объектом")
				}
			})

			c.assert("response contains field message", fmt.Sprintf("%s JSON объект содержит поле message", descPrefix), func(t *testing.T) {
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
		})
	}
}

func fatalf(t *testing.T, format string, args ...any) {
	t.Helper()
	fatalErrorOccurred = true
	t.Fatalf(format, args...)
}
