package currencyexchange_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func getExchange(t *testing.T) {
	t.Run("success", getExchangeSuccess)
	t.Run("bad request", getExchangeBadRequest)
	t.Run("not found", getExchangeNotFound)
}

func getExchangeSuccess(t *testing.T) {
	type testCase struct {
		name   string
		desc   string
		amount float64
		rateFn func() (ExchangeRate, error)
	}

	testCases := []testCase{
		{
			name:   "direct",
			desc:   "прямой обменный курс между валютами",
			amount: 10.0,
			rateFn: findDirectExchangeRate,
		},
		{
			name:   "direct with zero amount",
			desc:   "прямой обменный курс между валютами и параметр amount равен нулю",
			amount: 0.0,
			rateFn: findDirectExchangeRate,
		},
		{
			name:   "inverse",
			desc:   "обратный обменный курс между валютами",
			amount: 10.0,
			rateFn: findInverseExchangeRate,
		},
		{
			name:   "cross",
			desc:   "кросс-курс через USD для валют без прямого или обратного курса",
			amount: 10.0,
			rateFn: findUSDCrossExchangeRate,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reqExchangeRate, err := tc.rateFn()

			if err != nil {
				err = fmt.Errorf("Не удалось найти обменный курс подходящий для обмена в API: %w", err)
			}

			path := "/exchange"
			if err == nil {
				urlParams := url.Values{
					"from":   {reqExchangeRate.BaseCurrency.Code},
					"to":     {reqExchangeRate.TargetCurrency.Code},
					"amount": {strconv.FormatFloat(tc.amount, 'f', 2, 64)},
				}
				path += "?" + urlParams.Encode()
			}

			var (
				resp  *http.Response
				dumps *dumps
			)
			if err == nil {
				resp, dumps, err = doRequest(t, http.MethodGet, path, nil)
			}

			var (
				bodyBytes []byte
				body      any
				decodeErr error
			)
			if err == nil {
				defer resp.Body.Close()
				bodyBytes, err = io.ReadAll(resp.Body)
				if err != nil {
					err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
				} else {
					decodeErr = json.Unmarshal(bodyBytes, &body)
				}
			}

			descPrefix := fmt.Sprintf("GET %s : %s => ", path, tc.desc)

			c := &checker{t, dumps, err}

			c.assert("status code is 200", descPrefix+"HTTP статус код 200", func(t *testing.T) {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Ожидался статус код %d, получен %d", http.StatusOK, resp.StatusCode)
				}
			})
			c.assert("no redirects", descPrefix+"HTTP статус код не в диапазоне 300-399 (редиректы)", func(t *testing.T) {
				if resp.StatusCode >= 300 && resp.StatusCode < 400 {
					t.Errorf("Неожиданный статус код редиректа %d", resp.StatusCode)
				}
			})
			c.assert("content type is json", descPrefix+"HTTP заголовок Content-Type начинается с application/json", func(t *testing.T) {
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
				}
			})

			c.assert("response is valid json", descPrefix+"Тело ответа парсится в JSON без ошибок", func(t *testing.T) {
				if decodeErr != nil {
					t.Errorf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
			})

			exchangeObject, isObject := body.(map[string]any)
			c.assert("response is json", descPrefix+"Тело ответа парсится в JSON объект без ошибок", func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Тело ответа не является JSON-объектом")
				}
			})

			var exchangeInvalidErr error
			c.assert("object fields are valid", descPrefix+"JSON Объект содержит baseCurrency, targetCurrency, rate, amount, convertedAmount поля", func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Skip("Тело ответа не является JSON-объектом")
				}
				valid, reason := isValidExchange(exchangeObject)
				if !valid {
					exchangeInvalidErr = fmt.Errorf("Объект JSON не является валидным объектом `Exchange`: %s", reason)
					t.Error(exchangeInvalidErr)
				}
			})

			var respExchange Exchange
			c.assert("response body matches expected schema", descPrefix+"JSON тело ответа соответствует схеме в ТЗ", func(t *testing.T) {
				if exchangeInvalidErr != nil {
					t.Skip(exchangeInvalidErr)
				}
				decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&respExchange); err != nil {
					decodeErr = fmt.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %w", err)
					t.Error(decodeErr)
				}
			})

			var roundingErr error
			c.assert("converted amount is rounded", descPrefix+"Результат конвертации корректно округлен", func(t *testing.T) {
				if exchangeInvalidErr != nil {
					t.Skip(exchangeInvalidErr)
				}
				if decodeErr != nil {
					t.Skip(decodeErr)
				}
				s := strconv.FormatFloat(respExchange.ConvertedAmount, 'f', -1, 64)
				_, decimalPart, found := strings.Cut(s, ".")

				if found && len(decimalPart) > 2 {
					roundingErr = fmt.Errorf("Ожидалось округление `convertedAmount` до двух десятичных знаков, получено %q", s)
					t.Error(roundingErr)
				}
			})

			var exchangeRateErr error
			c.assert("matches requested exchange rate", descPrefix+"Возвращенный обменный курс соответствует запрошенному курсу", func(t *testing.T) {
				if exchangeInvalidErr != nil {
					t.Skip(exchangeInvalidErr)
				}
				if decodeErr != nil {
					t.Skip(decodeErr)
				}

				decimalPart := 0
				s := strconv.FormatFloat(respExchange.Rate, 'f', -1, 64)
				if _, dp, found := strings.Cut(s, "."); found {
					decimalPart = len(dp)
				}
				if err := isValidRounding(respExchange.Rate, reqExchangeRate.Rate, decimalPart); err != nil {
					exchangeRateErr = fmt.Errorf("Некорректный обменный курс: %w", err)
					t.Error(exchangeRateErr)
				}
			})

			c.assert("conversion is correct", descPrefix+"Конвертация математически корректна", func(t *testing.T) {
				if exchangeInvalidErr != nil {
					t.Skip(exchangeInvalidErr)
				}
				if decodeErr != nil {
					t.Skip(decodeErr)
				}
				if roundingErr != nil {
					t.Skip(roundingErr)
				}
				if exchangeRateErr != nil {
					t.Skip(exchangeRateErr)
				}
				if err := isValidRounding(respExchange.ConvertedAmount, tc.amount*respExchange.Rate, 2); err != nil {
					t.Error(err)
				}
			})
		})
	}
}

func getExchangeBadRequest(t *testing.T) {
	type testCase struct {
		name   string
		desc   string
		params url.Values
	}

	testCases := []testCase{
		{
			name:   "missing from",
			desc:   "отсутствует параметр from",
			params: url.Values{"to": {"USD"}, "amount": {"10.00"}},
		},
		{
			name:   "missing to",
			desc:   "отсутствует параметр to",
			params: url.Values{"from": {"EUR"}, "amount": {"10.00"}},
		},
		{
			name:   "missing amount",
			desc:   "отсутствует параметр amount",
			params: url.Values{"from": {"EUR"}, "to": {"USD"}},
		},
		{
			name:   "blank from",
			desc:   "параметр from пустой",
			params: url.Values{"from": {""}, "to": {"USD"}, "amount": {"10.00"}},
		},
		{
			name:   "blank to",
			desc:   "параметр to пустой",
			params: url.Values{"from": {"EUR"}, "to": {""}, "amount": {"10.00"}},
		},
		{
			name:   "blank amount",
			desc:   "параметр amount пустой",
			params: url.Values{"from": {"EUR"}, "to": {"USD"}, "amount": {""}},
		},
		{
			name:   "non-numeric amount",
			desc:   "параметр amount не является числом",
			params: url.Values{"from": {"EUR"}, "to": {"USD"}, "amount": {"abc"}},
		},
		{
			name:   "negative amount",
			desc:   "параметр amount отрицательный",
			params: url.Values{"from": {"EUR"}, "to": {"USD"}, "amount": {"-10.00"}},
		},
		{
			name:   "whitespace only amount",
			desc:   "параметр amount состоит только из пробелов",
			params: url.Values{"from": {"EUR"}, "to": {"USD"}, "amount": {"   "}},
		},
		{
			name:   "same currencies",
			desc:   "исходная и целевая валюты совпадают",
			params: url.Values{"from": {"USD"}, "to": {"USD"}, "amount": {"10.00"}},
		},
		{
			name:   "currency codes too short",
			desc:   "коды валют короче допустимой длины",
			params: url.Values{"from": {"EU"}, "to": {"US"}, "amount": {"10.00"}},
		},
		{
			name:   "currency codes too long",
			desc:   "коды валют превышают допустимую длину",
			params: url.Values{"from": {"EURO"}, "to": {"USDD"}, "amount": {"10.00"}},
		},
		{
			name:   "currency codes with digits",
			desc:   "коды валют содержат цифры",
			params: url.Values{"from": {"EU1"}, "to": {"US2"}, "amount": {"10.00"}},
		},
		{
			name:   "currency codes with special characters",
			desc:   "коды валют содержат специальные символы",
			params: url.Values{"from": {"EU-"}, "to": {"US~"}, "amount": {"10.00"}},
		},
		{
			name:   "whitespace only currency codes",
			desc:   "коды валют состоят только из пробелов",
			params: url.Values{"from": {"   "}, "to": {"   "}, "amount": {"10.00"}},
		},
		{
			name:   "lowercase currency codes",
			desc:   "оба кода валют в нижнем регистре",
			params: url.Values{"from": {"eur"}, "to": {"usd"}, "amount": {"10.00"}},
		},
		{
			name:   "lowercase from",
			desc:   "код исходной валюты в нижнем регистре",
			params: url.Values{"from": {"eur"}, "to": {"USD"}, "amount": {"10.00"}},
		},
		{
			name:   "lowercase to",
			desc:   "код целевой валюты в нижнем регистре",
			params: url.Values{"from": {"EUR"}, "to": {"usd"}, "amount": {"10.00"}},
		},
		{
			name:   "mixed case currency codes",
			desc:   "коды валют в смешанном регистре",
			params: url.Values{"from": {"eUr"}, "to": {"uSd"}, "amount": {"10.00"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := "/exchange?" + tc.params.Encode()

			resp, dumps, err := doRequest(t, http.MethodGet, path, nil)

			var (
				bodyBytes []byte
				body      any
				decodeErr error
			)

			if err == nil {
				defer resp.Body.Close()
				bodyBytes, err = io.ReadAll(resp.Body)
				if err != nil {
					err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
				} else {
					decodeErr = json.Unmarshal(bodyBytes, &body)
				}
			}

			c := &checker{t, dumps, err}

			descPrefix := fmt.Sprintf("GET %s : %s => ", path, tc.desc)

			c.assert("status code is 400", descPrefix+"HTTP статус код 400", func(t *testing.T) {
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("Ожидался статус код %d, получен %d", http.StatusBadRequest, resp.StatusCode)
				}
			})

			c.assert("content type is json", descPrefix+"HTTP заголовок Content-Type начинается с application/json", func(t *testing.T) {
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
				}
			})

			errorObject, isObject := body.(map[string]any)
			c.assert("response is json", descPrefix+"Тело ответа парсится в JSON объект без ошибок", func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Тело ответа не является JSON-объектом")
				}
			})

			c.assert("response contains field message", descPrefix+"JSON объект содержит поле message", func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Skip("Тело ответа не является JSON-объектом")
				}
				valid, reason := isValidStringField(errorObject, "message")
				if !valid {
					t.Error(reason)
				}
			})
		})
	}
}

func getExchangeNotFound(t *testing.T) {
	type testCase struct {
		name   string
		desc   string
		err    error
		params url.Values
	}

	missing := findOrGenerateUnusedCurrency()
	existing, existingErr := findUsedCurrency()

	if existingErr != nil {
		existingErr = fmt.Errorf("Не удалось найти валюту существующую в API: %w", existingErr)
	}

	baseWithoutExchangePath, targetWithoutExchangePath, exchangePathErr := findCurrenciesWithoutExchangePath()

	if exchangePathErr != nil {
		exchangePathErr = fmt.Errorf("Не удалось найти валюты без возможности обмена: %w", exchangePathErr)
	}

	testCases := []testCase{
		{
			name:   "base currency does not exist",
			desc:   "базовой валюты не существует",
			err:    existingErr,
			params: url.Values{"from": {missing.Code}, "to": {existing.Code}, "amount": {"10.00"}},
		},
		{
			name:   "target currency does not exist",
			desc:   "целевой валюты не существует",
			err:    existingErr,
			params: url.Values{"from": {existing.Code}, "to": {missing.Code}, "amount": {"10.00"}},
		},
		{
			name: "exchange rate does not exist",
			desc: "обменный курс между валютами отсутствует",
			err:  exchangePathErr,
			params: url.Values{
				"from":   {baseWithoutExchangePath.Code},
				"to":     {targetWithoutExchangePath.Code},
				"amount": {"10.00"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				resp  *http.Response
				dumps *dumps
				err   = tc.err
			)

			path := "/exchange?" + tc.params.Encode()

			if err == nil {
				resp, dumps, err = doRequest(t, http.MethodGet, path, nil)
			}

			var (
				bodyBytes []byte
				body      any
				decodeErr error
			)

			if err == nil {
				defer resp.Body.Close()
				bodyBytes, err = io.ReadAll(resp.Body)
				if err != nil {
					err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
				} else {
					decodeErr = json.Unmarshal(bodyBytes, &body)
				}
			}

			c := &checker{t, dumps, err}

			descPrefix := fmt.Sprintf("GET %s : %s => ", path, tc.desc)

			c.assert("status code is 404", descPrefix+"HTTP статус код 404", func(t *testing.T) {
				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("Ожидался статус код %d, получен %d", http.StatusNotFound, resp.StatusCode)
				}
			})

			c.assert("content type is json", descPrefix+"HTTP заголовок Content-Type начинается с application/json", func(t *testing.T) {
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
				}
			})

			errorObject, isObject := body.(map[string]any)

			c.assert("response is json", descPrefix+"Тело ответа парсится в JSON объект без ошибок", func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Тело ответа не является JSON-объектом")
				}
			})

			c.assert("response contains field message", descPrefix+"JSON объект содержит поле message", func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Skip("Тело ответа не является JSON-объектом")
				}
				valid, reason := isValidStringField(errorObject, "message")
				if !valid {
					t.Error(reason)
				}
			})
		})
	}
}
