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

func getExchangeRates(t *testing.T) {
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

func getExchangeRate(t *testing.T) {
	t.Run("success", getExchangeRateSuccess)
	t.Run("not found", getExchangeRateNotFound)
	t.Run("bad request", getExchangeRateBadRequest)
}

func getExchangeRateSuccess(t *testing.T) {
	reqExchangeRate, err := findUsedExchangeRate()
	if err != nil {
		t.Skip("Не удалось найти обменный курс, существующий в API")
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
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
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
		if valid, reason := isValidExchangeRate(exchangeRateObject); !valid {
			t.Skip(reason)
		}
		if exchangeRateDecodeErr != nil {
			t.Skipf("Тело ответа не соответствует ожидаемой схеме: %s", exchangeRateDecodeErr)
		}
		mustMatchExchangeRates(t, respExchangeRate, reqExchangeRate)
	})
}

func getExchangeRateNotFound(t *testing.T) {
	exchangeRate, err := findUnusedExchangeRate()
	if err != nil {
		t.Skip("Не удалось найти обменный курс, несуществующий в API")
	}

	injectCodes := strings.NewReplacer(
		"{base}", exchangeRate.BaseCurrency.Code,
		"{target}", exchangeRate.TargetCurrency.Code,
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
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
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

	base := findOrGenerateUnusedCurrency().Code
	target := findOrGenerateUnusedCurrency().Code

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

func postExchangeRates(t *testing.T) {
	t.Run("success", postExchangeRatesSuccess)
	t.Run("conflict", postExchangeRatesConflict)
	t.Run("bad request", postExchangeRatesBadRequest)
	t.Run("not found", postExchangeRatesNotFound)
}

func postExchangeRatesSuccess(t *testing.T) {
	reqExchangeRate, err := findUnusedExchangeRate()
	if err != nil {
		t.Skip("Не удалось найти обменный курс для вставки")
	}
	reqRate := 0.5

	form := url.Values{
		"baseCurrencyCode":   {reqExchangeRate.BaseCurrency.Code},
		"targetCurrencyCode": {reqExchangeRate.TargetCurrency.Code},
		"rate":               {strconv.FormatFloat(reqRate, 'f', 2, 64)},
	}
	resp, dumps, err := doRequest(t, http.MethodPost, "/exchangeRates", &form)

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

	c.assert("status code is 201", "POST /exchangeRates => HTTP статус код 201", func(t *testing.T) {
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusCreated, resp.StatusCode)
		}
	})
	c.assert("no redirects", "POST /exchangeRates => HTTP статус код не в диапазоне 300-399 (редиректы)", func(t *testing.T) {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Неожиданный статус код редиректа %d", resp.StatusCode)
		}
	})
	c.assert("content type is json", "POST /exchangeRates => HTTP заголовок Content-Type начинается с application/json", func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	exchangeRateObject, isObject := body.(map[string]any)
	c.assert("response is json", "POST /exchangeRates => Тело ответа парсится в JSON объект без ошибок", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	c.assert("object fields are valid", "POST /exchangeRates => JSON Объект содержит id, baseCurrency, targetCurrency, rate поля", func(t *testing.T) {
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
	c.assert("response body matches expected schema", "POST /exchangeRates => JSON тело ответа соответствует схеме в ТЗ", func(t *testing.T) {
		decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&respExchangeRate); err != nil {
			exchangeRateDecodeErr = err
			t.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %s", err)
		}
	})

	c.assert("response echoes requested exchange rate", "POST /exchangeRates => JSON объект содержит ожидаемые id, baseCurrency, targetCurrency, rate поля", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Тело ответа не является JSON-объектом")
		}
		if valid, reason := isValidExchangeRate(exchangeRateObject); !valid {
			t.Skip(reason)
		}
		if exchangeRateDecodeErr != nil {
			t.Skipf("Тело ответа не соответствует ожидаемой схеме: %s", exchangeRateDecodeErr)
		}
		reqExchangeRate := ExchangeRate{
			ID:             respExchangeRate.ID,
			BaseCurrency:   reqExchangeRate.BaseCurrency,
			TargetCurrency: reqExchangeRate.TargetCurrency,
			Rate:           reqRate,
		}
		mustMatchExchangeRates(t, respExchangeRate, reqExchangeRate)
	})

	apiExchangeRates = append(apiExchangeRates, respExchangeRate)
}

func postExchangeRatesConflict(t *testing.T) {
	reqExchangeRate, err := findUsedExchangeRate()
	if err != nil {
		t.Skip("Не удалось найти обменный курс, вызывающий конфликт")
	}

	form := url.Values{
		"baseCurrencyCode":   {reqExchangeRate.BaseCurrency.Code},
		"targetCurrencyCode": {reqExchangeRate.TargetCurrency.Code},
		"rate":               {strconv.FormatFloat(reqExchangeRate.Rate, 'f', 2, 64)},
	}
	resp, dumps, err := doRequest(t, http.MethodPost, "/exchangeRates", &form)

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

	c.assert("status code is 409", "POST /exchangeRates : существующий обменный курс => HTTP статус код 409", func(t *testing.T) {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Ожидался статус код %d, получен %d", http.StatusConflict, resp.StatusCode)
		}
	})
	c.assert("no redirects", "POST /exchangeRates : существующий обменный курс => HTTP статус код не в диапазоне 300-399 (редиректы)", func(t *testing.T) {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Неожиданный статус код редиректа %d", resp.StatusCode)
		}
	})
	c.assert("content type is json", "POST /exchangeRates : существующий обменный курс => HTTP заголовок Content-Type начинается с application/json", func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
		}
	})

	error, isObject := body.(map[string]any)
	c.assert("response is json", "POST /exchangeRates : существующий обменный курс => Тело ответа парсится в JSON объект без ошибок", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Error("Тело ответа не является JSON-объектом")
		}
	})

	c.assert("response contains field message", "POST /exchangeRates : существующий обменный курс => JSON объект содержит поле message", func(t *testing.T) {
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

func postExchangeRatesBadRequest(t *testing.T) {
	base := findOrGenerateUnusedCurrency().Code
	target := findOrGenerateUnusedCurrency().Code
	rate := "0.5"

	type testCase struct {
		subName string
		subDesc string
		form    url.Values
	}

	testCases := []testCase{
		{
			subName: "missing base currency code",
			subDesc: "отсутствует параметр baseCurrencyCode",
			form:    url.Values{"targetCurrencyCode": {target}, "rate": {rate}},
		},
		{
			subName: "blank base currency code",
			subDesc: "параметр baseCurrencyCode пустой",
			form:    url.Values{"baseCurrencyCode": {""}, "targetCurrencyCode": {target}, "rate": {rate}},
		},
		{
			subName: "missing target currency code",
			subDesc: "отсутствует параметр targetCurrencyCode",
			form:    url.Values{"baseCurrencyCode": {base}, "rate": {rate}},
		},
		{
			subName: "blank target currency code",
			subDesc: "параметр targetCurrencyCode пустой",
			form:    url.Values{"baseCurrencyCode": {base}, "targetCurrencyCode": {""}, "rate": {rate}},
		},
		{
			subName: "missing rate",
			subDesc: "отсутствует параметр rate",
			form:    url.Values{"baseCurrencyCode": {base}, "targetCurrencyCode": {target}},
		},
		{
			subName: "blank rate",
			subDesc: "параметр rate пустой",
			form:    url.Values{"baseCurrencyCode": {base}, "targetCurrencyCode": {target}, "rate": {""}},
		},
		{
			subName: "non-numeric rate",
			subDesc: "параметр rate не является числом",
			form:    url.Values{"baseCurrencyCode": {base}, "targetCurrencyCode": {target}, "rate": {"abc"}},
		},
		{
			subName: "negative rate",
			subDesc: "параметр rate отрицательный",
			form:    url.Values{"baseCurrencyCode": {base}, "targetCurrencyCode": {target}, "rate": {"-0.50"}},
		},
		{
			subName: "zero rate",
			subDesc: "параметр rate равен нулю",
			form:    url.Values{"baseCurrencyCode": {base}, "targetCurrencyCode": {target}, "rate": {"0"}},
		},
		{
			subName: "base currency code too short",
			subDesc: "параметр baseCurrencyCode короче допустимой длины",
			form:    url.Values{"baseCurrencyCode": {base[:2]}, "targetCurrencyCode": {target}, "rate": {rate}},
		},
		{
			subName: "base currency code too long",
			subDesc: "параметр baseCurrencyCode превышает допустимую длину",
			form:    url.Values{"baseCurrencyCode": {base + base}, "targetCurrencyCode": {target}, "rate": {rate}},
		},
		{
			subName: "target currency code too short",
			subDesc: "параметр targetCurrencyCode короче допустимой длины",
			form:    url.Values{"baseCurrencyCode": {base}, "targetCurrencyCode": {target[:2]}, "rate": {rate}},
		},
		{
			subName: "target currency code too long",
			subDesc: "параметр targetCurrencyCode превышает допустимую длину",
			form:    url.Values{"baseCurrencyCode": {base}, "targetCurrencyCode": {target + target}, "rate": {rate}},
		},
		{
			subName: "same base and target currency code",
			subDesc: "базовая и целевая валюты совпадают",
			form:    url.Values{"baseCurrencyCode": {base}, "targetCurrencyCode": {base}, "rate": {rate}},
		},
		{
			subName: "lowercase currency codes",
			subDesc: "коды валют в нижнем регистре",
			form:    url.Values{"baseCurrencyCode": {strings.ToLower(base)}, "targetCurrencyCode": {strings.ToLower(target)}, "rate": {rate}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.subName, func(t *testing.T) {
			t.Parallel()
			resp, dumps, err := doRequest(t, http.MethodPost, "/exchangeRates", &tc.form)

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

			c.assert("status code is 400", fmt.Sprintf("POST /exchangeRates : %s => HTTP статус код 400", tc.subDesc), func(t *testing.T) {
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("Ожидался статус код %d, получен %d", http.StatusBadRequest, resp.StatusCode)
				}
			})
			c.assert("content type is json", fmt.Sprintf("POST /exchangeRates : %s => HTTP заголовок Content-Type начинается с application/json", tc.subDesc), func(t *testing.T) {
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
				}
			})

			error, isObject := body.(map[string]any)
			c.assert("response is json", fmt.Sprintf("POST /exchangeRates : %s => Тело ответа парсится в JSON объект без ошибок", tc.subDesc), func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Тело ответа не является JSON-объектом")
				}
			})

			c.assert("response contains field message", fmt.Sprintf("POST /exchangeRates : %s => JSON объект содержит поле message", tc.subDesc), func(t *testing.T) {
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

func postExchangeRatesNotFound(t *testing.T) {
	c, err := findUsedCurrency()
	if err != nil {
		t.Skip("Не удалось найти валюту существующую в API")
	}
	existing := c.Code
	missing := findOrGenerateUnusedCurrency().Code
	rate := "0.50"

	type testCase struct {
		subName string
		subDesc string
		form    url.Values
	}

	testCases := []testCase{
		{
			subName: "unknown base currency code",
			subDesc: "код базовой валюты не существует в системе",
			form:    url.Values{"baseCurrencyCode": {missing}, "targetCurrencyCode": {existing}, "rate": {rate}},
		},
		{
			subName: "unknown target currency code",
			subDesc: "код целевой валюты не существует в системе",
			form:    url.Values{"baseCurrencyCode": {existing}, "targetCurrencyCode": {missing}, "rate": {rate}},
		},
		{
			subName: "unknown base and target currency code",
			subDesc: "код базовой и целевой валюты не существует в системе",
			form:    url.Values{"baseCurrencyCode": {missing}, "targetCurrencyCode": {generateRandomCurrency().Code}, "rate": {rate}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.subName, func(t *testing.T) {
			t.Parallel()
			resp, dumps, err := doRequest(t, http.MethodPost, "/exchangeRates", &tc.form)

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

			c.assert("status code is 404", fmt.Sprintf("POST /exchangeRates : %s => HTTP статус код 404", tc.subDesc), func(t *testing.T) {
				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("Ожидался статус код %d, получен %d", http.StatusNotFound, resp.StatusCode)
				}
			})
			c.assert("content type is json", fmt.Sprintf("POST /exchangeRates : %s => HTTP заголовок Content-Type начинается с application/json", tc.subDesc), func(t *testing.T) {
				contentType := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "application/json") {
					t.Errorf("Ожидался заголовок Content-Type %q, получен %q", "application/json", contentType)
				}
			})

			error, isObject := body.(map[string]any)
			c.assert("response is json", fmt.Sprintf("POST /exchangeRates : %s => Тело ответа парсится в JSON объект без ошибок", tc.subDesc), func(t *testing.T) {
				if decodeErr != nil {
					t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
				}
				if !isObject {
					t.Error("Тело ответа не является JSON-объектом")
				}
			})

			c.assert("response contains field message", fmt.Sprintf("POST /exchangeRates : %s => JSON объект содержит поле message", tc.subDesc), func(t *testing.T) {
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
