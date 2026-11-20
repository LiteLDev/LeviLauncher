//go:build darwin

package xbox

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestXboxExchangeFallsBackAfterSystemProxyDisconnects(t *testing.T) {
	oldDirect, oldPolicy := xboxDirectClient, xboxUsesSystemProxy
	t.Cleanup(func() { xboxDirectClient = oldDirect; xboxUsesSystemProxy = oldPolicy })
	xboxUsesSystemProxy = func(*http.Request) bool { return true }
	normal, direct := 0, 0
	browserTestHTTP(t, func(r *http.Request) (*http.Response, error) { normal++; return nil, io.EOF })
	xboxDirectClient = &http.Client{Transport: browserTestTransport(func(r *http.Request) (*http.Response, error) {
		direct++
		if r.URL.Hostname() != "user.auth.xboxlive.com" {
			t.Fatal("unexpected credential destination")
		}
		b, _ := io.ReadAll(r.Body)
		if string(b) != `{"test":"payload"}` {
			t.Fatal("fallback changed exchange payload")
		}
		return browserTestResponse(200, `{"ok":true}`)
	})}
	var result struct {
		OK bool `json:"ok"`
	}
	err := postJSONContext(context.Background(), "https://user.auth.xboxlive.com/user/authenticate", "1", "", map[string]string{"test": "payload"}, &result)
	if err != nil || !result.OK || normal != 2 || direct != 1 {
		t.Fatalf("fallback failed: normal=%d direct=%d err=%v", normal, direct, err)
	}
}

func TestXboxExchangeDoesNotBypassExplicitProxy(t *testing.T) {
	oldDirect, oldPolicy := xboxDirectClient, xboxUsesSystemProxy
	t.Cleanup(func() { xboxDirectClient = oldDirect; xboxUsesSystemProxy = oldPolicy })
	xboxUsesSystemProxy = func(*http.Request) bool { return false }
	calls := 0
	browserTestHTTP(t, func(r *http.Request) (*http.Response, error) { calls++; return nil, io.EOF })
	xboxDirectClient = &http.Client{Transport: browserTestTransport(func(r *http.Request) (*http.Response, error) {
		t.Fatal("explicit proxy was bypassed")
		return nil, io.EOF
	})}
	var result map[string]any
	if err := postJSONContext(context.Background(), "https://xsts.auth.xboxlive.com/xsts/authorize", "1", "", map[string]string{}, &result); err == nil || calls != 3 {
		t.Fatalf("unexpected retry result: %d %v", calls, err)
	}
}

func TestXboxExchangeRetriesWithSameBodyAndHeaders(t *testing.T) {
	calls := 0
	var first string
	browserTestHTTP(t, func(r *http.Request) (*http.Response, error) {
		calls++
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if calls == 1 {
			first = string(b)
		} else if string(b) != first {
			t.Fatal("retry changed the token-exchange payload")
		}
		if r.Header.Get("Authorization") != "test-auth" {
			t.Fatal("missing auth header")
		}
		if calls == 1 {
			return browserTestResponse(503, `service unavailable`)
		}
		return browserTestResponse(200, `{"ok":true}`)
	})
	var result struct {
		OK bool `json:"ok"`
	}
	err := postJSONContext(context.Background(), "https://xsts.auth.xboxlive.com/xsts/authorize", "1", "test-auth", map[string]string{"test": "payload"}, &result)
	if err != nil || !result.OK || calls != 2 {
		t.Fatalf("exchange did not recover: calls=%d err=%v", calls, err)
	}
}

func TestXboxExchangeDoesNotRetryAuthorizationRejection(t *testing.T) {
	calls := 0
	browserTestHTTP(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return browserTestResponse(401, `{"error":"unauthorized"}`)
	})
	var result map[string]any
	err := postJSONContext(context.Background(), "https://xsts.auth.xboxlive.com/xsts/authorize", "1", "", map[string]string{}, &result)
	if err == nil || calls != 1 {
		t.Fatalf("authorization rejection was retried: calls=%d err=%v", calls, err)
	}
}
