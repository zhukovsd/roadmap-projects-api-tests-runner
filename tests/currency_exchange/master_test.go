package currencyexchange_test

import (
	_ "embed"
	"encoding/json"
	"flag"
	"net"
	"net/url"
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

func TestMaster(t *testing.T) {
	t.Run("host is reachable", hostIsReachable)

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
	})
}

func hostIsReachable(t *testing.T) {
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
