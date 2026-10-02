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
	er, err := findUsedExchangeRate()
	if err != nil {
		t.Skip("Не удалось найти обменный курс, существующий в API")
	}

	amount := 10.00

	urlParams := url.Values{
		"from":   {er.BaseCurrency.Code},
		"to":     {er.TargetCurrency.Code},
		"amount": {strconv.FormatFloat(amount, 'f', 2, 64)},
	}

	path := "/exchange?" + urlParams.Encode()

	resp, dumps, err := doRequest(t, http.MethodGet, path, nil)

	var bodyBytes []byte
	var body any
	var decodeErr error

	if err == nil {
		defer resp.Body.Close()
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("Не удалось прочитать тело ответа: %w", err)
		} else {
			decodeErr = json.Unmarshal(bodyBytes, &body)
		}
	}

	descPrefix := fmt.Sprintf("GET %s => ", path)

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

	c.assert("object fields are valid", descPrefix+"JSON Объект содержит baseCurrency, targetCurrency, rate, amount, convertedAmount поля", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Тело ответа не является валидным JSON: %s", decodeErr)
		}
		if !isObject {
			t.Skip("Тело ответа не является JSON-объектом")
		}
		valid, reason := isValidExchange(exchangeObject)
		if !valid {
			t.Error(reason)
		}
	})

	var respExchange Exchange
	c.assert("response body matches expected schema", descPrefix+"JSON тело ответа соответствует схеме в ТЗ", func(t *testing.T) {
		decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&respExchange); err != nil {
			t.Errorf("Не удалось разобрать тело ответа в структуру (POJO/DTO): %s", err)
		}

		s := strconv.FormatFloat(respExchange.ConvertedAmount, 'f', -1, 64)
		_, decimalPart, found := strings.Cut(s, ".")

		if found && len(decimalPart) > 2 {
			t.Errorf("Ожидалось корректное округление `convertedAmount` до двух десятичных знаков, получено %q", s)
		}
	})
}
