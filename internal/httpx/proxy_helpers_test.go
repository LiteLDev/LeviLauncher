package httpx

import (
	"errors"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
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

func TestRunWithSoftTimeoutReturnsCompletedResult(t *testing.T) {
	var finished atomic.Int32
	var timedOut atomic.Int32

	value, resolved, err, timeout := runWithSoftTimeout(
		200*time.Millisecond,
		func() (int, bool, error) {
			return 42, true, nil
		},
		func(err error) {
			if err != nil {
				t.Fatalf("unexpected finish error: %v", err)
			}
			finished.Add(1)
		},
		func() {
			timedOut.Add(1)
		},
	)
	if timeout {
		t.Fatal("expected lookup to complete before timeout")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved {
		t.Fatal("expected resolved result")
	}
	if value != 42 {
		t.Fatalf("unexpected value: %d", value)
	}
	if finished.Load() != 1 {
		t.Fatalf("expected onFinish to be called once, got %d", finished.Load())
	}
	if timedOut.Load() != 0 {
		t.Fatalf("expected onTimeout not to be called, got %d", timedOut.Load())
	}
}

func TestRunWithSoftTimeoutFallsBackQuickly(t *testing.T) {
	finishCh := make(chan error, 1)
	var timedOut atomic.Int32

	startedAt := time.Now()
	value, resolved, err, timeout := runWithSoftTimeout(
		40*time.Millisecond,
		func() (int, bool, error) {
			time.Sleep(120 * time.Millisecond)
			return 7, true, errors.New("late failure")
		},
		func(err error) {
			finishCh <- err
		},
		func() {
			timedOut.Add(1)
		},
	)
	elapsed := time.Since(startedAt)

	if !timeout {
		t.Fatal("expected lookup to time out")
	}
	if elapsed >= 100*time.Millisecond {
		t.Fatalf("expected timeout fallback to return quickly, elapsed=%s", elapsed)
	}
	if err != nil {
		t.Fatalf("expected timeout fallback to suppress late error, got %v", err)
	}
	if resolved {
		t.Fatal("expected unresolved result after timeout")
	}
	if value != 0 {
		t.Fatalf("expected zero value after timeout, got %d", value)
	}
	if timedOut.Load() != 1 {
		t.Fatalf("expected onTimeout to be called once, got %d", timedOut.Load())
	}

	select {
	case finishErr := <-finishCh:
		if finishErr == nil || finishErr.Error() != "late failure" {
			t.Fatalf("unexpected finish error: %v", finishErr)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected onFinish to run after timeout")
	}
}
