//go:build darwin || linux

package httpx

import (
	"context"
	"errors"
	"golang.org/x/net/http/httpproxy"
	"net/http"
	"testing"
	"time"
)

func TestMacProxySettings(t *testing.T) {
	settings := map[string]string{"HTTPEnable": "1", "HTTPProxy": "127.0.0.1", "HTTPPort": "7897", "HTTPSEnable": "1", "HTTPSProxy": "::1", "HTTPSPort": "7898"}
	if p := proxyFromMacSettings("http", settings); p == nil || p.String() != "http://127.0.0.1:7897" {
		t.Fatalf("HTTP proxy: %v", p)
	}
	if p := proxyFromMacSettings("https", settings); p == nil || p.String() != "http://[::1]:7898" {
		t.Fatalf("HTTPS proxy: %v", p)
	}
	settings["HTTPSPort"] = "invalid"
	if p := proxyFromMacSettings("https", settings); p != nil {
		t.Fatalf("invalid port accepted: %v", p)
	}
	settings["HTTPEnable"] = "0"
	if p := proxyFromMacSettings("http", settings); p != nil {
		t.Fatalf("disabled proxy accepted: %v", p)
	}
}

func TestSystemProxyRefreshesAfterChange(t *testing.T) {
	now := time.Now()
	port := "7897"
	loads := 0
	cache := unixProxyCache{now: func() time.Time { return now }, load: func() (map[string]string, error) {
		loads++
		return map[string]string{"HTTPSEnable": "1", "HTTPSProxy": "127.0.0.1", "HTTPSPort": port}, nil
	}}
	first, err := cache.get()
	if err != nil {
		t.Fatal(err)
	}
	port = "7898"
	cached, _ := cache.get()
	if cached["HTTPSPort"] != "7897" || loads != 1 {
		t.Fatal("cache did not reuse the current snapshot")
	}
	now = now.Add(6 * time.Second)
	changed, err := cache.get()
	if err != nil || changed["HTTPSPort"] != "7898" || first["HTTPSPort"] != "7897" {
		t.Fatalf("proxy was not refreshed: %v %v", changed, err)
	}
}

func TestUnixProxyHonorsBypassBeforeSystemFallback(t *testing.T) {
	t.Setenv("ALL_PROXY", "")
	t.Setenv("all_proxy", "")
	settings := parseUnixProxySettings(`<dictionary> {
  ExceptionsList : <array> {
    0 : *.example.com
  }
  HTTPSEnable : 1
  HTTPSProxy : 127.0.0.1
  HTTPSPort : 7897
  ExcludeSimpleHostnames : 1
 }`)
	for _, host := range []string{"api.example.com", "auth.internal.test", "10.2.3.4", "intranet"} {
		req, _ := http.NewRequest(http.MethodGet, "https://"+host, nil)
		proxy, err := proxyFromUnixSettings(req, settings, &httpproxy.Config{NoProxy: "internal.test,10.0.0.0/8"})
		if err != nil || proxy != nil {
			t.Errorf("bypass %s: %v %v", host, proxy, err)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, "https://login.live.com", nil)
	proxy, err := proxyFromUnixSettings(req, settings, &httpproxy.Config{})
	if err != nil || proxy == nil || proxy.String() != "http://127.0.0.1:7897" {
		t.Fatalf("system proxy not used: %v %v", proxy, err)
	}
	proxy, err = proxyFromUnixSettings(req, settings, &httpproxy.Config{HTTPSProxy: "http://127.0.0.1:7898"})
	if err != nil || proxy == nil || proxy.String() != "http://127.0.0.1:7898" {
		t.Fatalf("explicit proxy not used: %v %v", proxy, err)
	}
}

func TestUnixProxyReadFailureDoesNotBecomeCachedDirectConnection(t *testing.T) {
	cache := unixProxyCache{now: time.Now, load: func() (map[string]string, error) { return nil, context.DeadlineExceeded }}
	for range 2 {
		if _, err := cache.get(); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("proxy lookup failure was suppressed: %v", err)
		}
	}
}
