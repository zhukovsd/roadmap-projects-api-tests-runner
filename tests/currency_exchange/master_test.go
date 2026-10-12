package currencyexchange_test

import (
	_ "embed"
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/tests"
)

var baseURL = flag.String("base-url", "http://to_be_provided:8080", "API base URL to test")
var dryRun = flag.Bool("dry-run", false, "Dry run mode, do not actually run tests")
var deployAvailable = true

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

func TestMaster(t *testing.T) {
	t.Run("deploy is reachable", deployIsReachable)

	t.Run("currencies", func(t *testing.T) {
		t.Run("get", getCurrencies)
		t.Run("post", postCurrencies)
	})
	t.Run("currency", func(t *testing.T) {
		t.Run("get", getCurrency)
	})

	t.Run("exchange rates", func(t *testing.T) {
		t.Run("get", getExchangeRates)
		t.Run("post", postExchangeRates)
	})
	t.Run("exchange rate", func(t *testing.T) {
		t.Run("get", getExchangeRate)
		t.Run("patch", patchExchangeRate)
	})
	t.Run("exchange", func(t *testing.T) {
		t.Run("get", getExchange)
	})
}

func deployIsReachable(t *testing.T) {
	t.Attr(tests.TestDescriptionAttr, "Деплой доступен для проверки т.е. имеет публичный IP адрес")

	setDeployUnavailable := func(format string, args ...any) {
		deployAvailable = false
		t.Fatalf(format, args...)
	}

	if *dryRun {
		t.Skip()
	}
	u, err := url.Parse(*baseURL)
	if err != nil {
		setDeployUnavailable("Не удалось разобрать URL: %v", err)
	}

	addrs, err := net.LookupHost(u.Hostname())
	if err != nil {
		setDeployUnavailable("Не найдено адресов для хоста %q: %v", u.Hostname(), err)
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil {
			setDeployUnavailable("Недопустимый адрес %q для хоста %q", a, u.Hostname())
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
			setDeployUnavailable("Адрес %q недоступен из публичного интернета", a)
		}
	}

	if _, _, err = doRequest(t, http.MethodGet, "", nil); err != nil {
		setDeployUnavailable("%v", err)
	}
}
