package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// fakeSystemProxy replaces the proxy environment (with none), the IE config
// and the PAC/WPAD lookup, and resets the budgeted resolver's shared state
// around the test.
func fakeSystemProxy(t *testing.T, cfg ieProxyConfig, lookup func(*url.URL, ieProxyConfig) (*url.URL, bool, error)) {
	t.Helper()
	oldEnv, oldLoad, oldLookup := envProxy, loadIEProxyConfig, lookupAutoProxyURL
	envProxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	loadIEProxyConfig = func() (ieProxyConfig, error) { return cfg, nil }
	lookupAutoProxyURL = lookup
	resetAutoProxyLookupState()
	t.Cleanup(func() {
		envProxy, loadIEProxyConfig, lookupAutoProxyURL = oldEnv, oldLoad, oldLookup
		resetAutoProxyLookupState()
	})
}

func resetAutoProxyLookupState() {
	autoProxyLookupMu.Lock()
	defer autoProxyLookupMu.Unlock()
	autoProxyLookupActive = false
	autoProxyLookupTimedOut = false
	autoProxyLookupDisabledTill = time.Time{}
}

func pacProxy(t *testing.T) *url.URL {
	t.Helper()
	return mustParseURL(t, "http://pac-proxy:8080")
}

func newProxyRequest(t *testing.T, ctx context.Context) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/LiteLDev/LeviLamina", nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// A PAC lookup slower than the UI budget is still waited for, and the
// budget's 30 s disable window does not apply.
func TestBlockingProxyWaitsForSlowPAC(t *testing.T) {
	want := pacProxy(t)
	fakeSystemProxy(t, ieProxyConfig{autoConfig: "http://wpad/proxy.pac"}, func(*url.URL, ieProxyConfig) (*url.URL, bool, error) {
		time.Sleep(autoProxyLookupTimeout + 250*time.Millisecond)
		return want, true, nil
	})
	req := newProxyRequest(t, context.Background())

	if got, err := proxyFromWindowsSystem(req); err != nil || got != nil {
		t.Fatalf("budgeted resolver = %v, %v; want direct after timeout", got, err)
	}
	for i := 0; i < 2; i++ {
		got, err := proxyFromWindowsSystemBlocking(req)
		if err != nil || got == nil || got.String() != want.String() {
			t.Fatalf("blocking resolver = %v, %v; want %s", got, err, want)
		}
	}
}

// Concurrent requests each resolve instead of skipping the lookup while
// another one is in flight.
func TestBlockingProxyResolvesDuringConcurrentLookup(t *testing.T) {
	want := pacProxy(t)
	fakeSystemProxy(t, ieProxyConfig{autoDetect: true}, func(*url.URL, ieProxyConfig) (*url.URL, bool, error) {
		return want, true, nil
	})
	if !beginAutoProxyLookup() {
		t.Fatal("could not mark a lookup in flight")
	}
	req := newProxyRequest(t, context.Background())

	if got, _ := proxyFromWindowsSystem(req); got != nil {
		t.Fatalf("budgeted resolver = %v; want skipped lookup", got)
	}
	if got, err := proxyFromWindowsSystemBlocking(req); err != nil || got == nil || got.String() != want.String() {
		t.Fatalf("blocking resolver = %v, %v; want %s", got, err, want)
	}
}

// A failed PAC/WPAD lookup falls back to the manual proxy.
func TestBlockingProxyFallsBackToManualOnLookupError(t *testing.T) {
	fakeSystemProxy(t, ieProxyConfig{autoDetect: true, proxy: "manual-proxy:3128"}, func(*url.URL, ieProxyConfig) (*url.URL, bool, error) {
		return nil, false, errors.New("ERROR_WINHTTP_AUTODETECTION_FAILED")
	})
	got, err := proxyFromWindowsSystemBlocking(newProxyRequest(t, context.Background()))
	if err != nil || got == nil || got.String() != "http://manual-proxy:3128" {
		t.Fatalf("blocking resolver = %v, %v; want manual proxy", got, err)
	}
}

// A lookup that outlives the request's context fails the request.
func TestBlockingProxyHonorsRequestContext(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	fakeSystemProxy(t, ieProxyConfig{autoDetect: true}, func(*url.URL, ieProxyConfig) (*url.URL, bool, error) {
		<-release
		return nil, true, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := proxyFromWindowsSystemBlocking(newProxyRequest(t, ctx)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v; want deadline exceeded", err)
	}
}

func TestNewBlockingProxyTransport(t *testing.T) {
	rt, ok := NewBlockingProxyTransport().(*defaultHeaderTransport)
	if !ok {
		t.Fatal("transport does not apply default headers")
	}
	base, ok := rt.base.(*http.Transport)
	if !ok || base == sharedTransport.(*defaultHeaderTransport).base {
		t.Fatal("transport must have its own *http.Transport")
	}
	want := pacProxy(t)
	fakeSystemProxy(t, ieProxyConfig{autoDetect: true}, func(*url.URL, ieProxyConfig) (*url.URL, bool, error) {
		return want, true, nil
	})
	beginAutoProxyLookup() // the budgeted resolver would skip the lookup now
	if got, err := base.Proxy(newProxyRequest(t, context.Background())); err != nil || got == nil || got.String() != want.String() {
		t.Fatalf("Proxy = %v, %v; want %s", got, err, want)
	}
}
