package httpx

import (
	"net/url"
	"testing"
)

func TestManualProxyConfigUsesSchemeSpecificTargets(t *testing.T) {
	cfg := newManualProxyConfig("http=proxy-http:8080;https=proxy-https:8443", "")

	httpURL, err := cfg.proxyForURL(mustParseURL(t, "http://example.com"))
	if err != nil {
		t.Fatalf("http proxy lookup failed: %v", err)
	}
	if got := httpURL.String(); got != "http://proxy-http:8080" {
		t.Fatalf("unexpected http proxy: %s", got)
	}

	httpsURL, err := cfg.proxyForURL(mustParseURL(t, "https://example.com"))
	if err != nil {
		t.Fatalf("https proxy lookup failed: %v", err)
	}
	if got := httpsURL.String(); got != "http://proxy-https:8443" {
		t.Fatalf("unexpected https proxy: %s", got)
	}
}

func TestManualProxyConfigBypassRules(t *testing.T) {
	cfg := newManualProxyConfig("proxy.local:7890", "<local>;*.example.com;127.*")

	cases := []string{
		"http://printer",
		"http://api.example.com",
		"http://127.0.0.1",
	}

	for _, target := range cases {
		proxyURL, err := cfg.proxyForURL(mustParseURL(t, target))
		if err != nil {
			t.Fatalf("proxy lookup failed for %s: %v", target, err)
		}
		if proxyURL != nil {
			t.Fatalf("expected direct connection for %s, got %s", target, proxyURL.String())
		}
	}
}

func TestParseAutoProxyURL(t *testing.T) {
	cases := map[string]string{
		"PROXY corp-proxy:8080; DIRECT": "http://corp-proxy:8080",
		"HTTPS secure-proxy:8443":       "https://secure-proxy:8443",
		"SOCKS socks-proxy:1080":        "socks5://socks-proxy:1080",
	}

	for raw, want := range cases {
		proxyURL, err := parseAutoProxyURL(raw)
		if err != nil {
			t.Fatalf("parseAutoProxyURL(%q) failed: %v", raw, err)
		}
		if proxyURL == nil || proxyURL.String() != want {
			t.Fatalf("parseAutoProxyURL(%q) = %v, want %s", raw, proxyURL, want)
		}
	}

	proxyURL, err := parseAutoProxyURL("DIRECT")
	if err != nil {
		t.Fatalf("parseAutoProxyURL(DIRECT) failed: %v", err)
	}
	if proxyURL != nil {
		t.Fatalf("expected DIRECT to resolve without proxy, got %s", proxyURL.String())
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) failed: %v", raw, err)
	}
	return parsed
}
