package currencyexchange_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

var baseURL = flag.String("base-url", "http://to_be_provided:8080", "API base URL to test")
var client = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func TestHostIsReachable(t *testing.T) {
	u, err := url.Parse(*baseURL)
	if err != nil {
		t.Errorf("Failed to parse URL: %s", err)
	}
	ip := net.ParseIP(u.Hostname())

	if ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			t.Errorf("Address %q is not accessible from the public internet", u.Host)
		}
		return
	}

	addrs, err := net.LookupHost(u.Hostname())
	if err != nil {
		t.Errorf("No addresses found for host: %q", u.Hostname())
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil {
			t.Errorf("Invalid address %q for host %q", a, u.Hostname())
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			t.Errorf("Address %q is not accessible from the public internet", u.Host)
		}
	}
}

func TestGetCurrencies(t *testing.T) {
	req, err := http.NewRequestWithContext(t.Context(), "GET", *baseURL+"/currencies", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %s", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %s", err)
	}
	defer resp.Body.Close()

	var body any
	decodeErr := json.NewDecoder(resp.Body).Decode(&body)

	t.Run("status_code", func(t *testing.T) {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status code %d got %d", http.StatusOK, resp.StatusCode)
		}
	})
	t.Run("no_redirect", func(t *testing.T) {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("Unexpected redirect status code %d", resp.StatusCode)
		}
	})
	t.Run("content_type", func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Expected Content-Type header %q got %q", "application/json", contentType)
		}
	})

	t.Run("response_is_valid_JSON", func(t *testing.T) {
		if decodeErr != nil {
			t.Errorf("Response body is not a valid JSON: %s", decodeErr)
		}
	})

	currencies, isArray := body.([]any)
	t.Run("response_is_array", func(t *testing.T) {
		if decodeErr != nil {
			t.Skipf("Response body is not a valid JSON: %s", decodeErr)
		}
		if !isArray {
			t.Error("Response body is not a JSON array")
		}
	})

	t.Run("array_elements_are_valid", func(t *testing.T) {
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
			valid, reason := isValidID(c)
			if !valid {
				t.Error(reason)
			}
			valid, reason = isValidStringField(c, "name")
			if !valid {
				t.Error(reason)
			}
			valid, reason = isValidStringField(c, "sign")
			if !valid {
				t.Error(reason)
			}
		}
	})
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
