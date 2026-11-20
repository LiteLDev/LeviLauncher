package xbox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type browserTestTransport func(*http.Request) (*http.Response, error)

func (f browserTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func browserTestHTTP(t *testing.T, handler browserTestTransport) {
	t.Helper()
	original := httpClient
	httpClient = &http.Client{Transport: handler}
	t.Cleanup(func() { httpClient = original; ConfigureAccountSelection("", nil) })
}

func browserTestResponse(status int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

func TestBrowserPollingPendingAndDenied(t *testing.T) {
	for _, deny := range []bool{false, true} {
		t.Run(map[bool]string{false: "authorized", true: "denied"}[deny], func(t *testing.T) {
			calls := 0
			browserTestHTTP(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "login.live.com" {
					t.Fatal("unexpected credential destination")
				}
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				if r.Form.Get("device_code") != "test-device" || r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
					t.Fatal("wrong device flow")
				}
				if r.Form.Get("client_id") != "000000004424da1f" {
					t.Fatal("device polling used a client without Store scope authorization")
				}
				calls++
				if calls == 1 {
					return browserTestResponse(400, `{"error":"authorization_pending"}`)
				}
				if deny {
					return browserTestResponse(400, `{"error":"authorization_declined"}`)
				}
				return browserTestResponse(200, `{"access_token":"test-access","refresh_token":"test-refresh","user_id":"test-user","expires_in":3600}`)
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			token, err := pollBrowserToken(ctx, "test-device", time.Millisecond)
			if calls != 2 {
				t.Fatalf("pending response did not poll again: %d", calls)
			}
			if deny && !errors.Is(err, context.Canceled) {
				t.Fatalf("denial not final: %v", err)
			}
			if !deny && (err != nil || token.UserID != "test-user") {
				t.Fatalf("authorization failed: %v", err)
			}
		})
	}
}

func TestBrowserCanceledSignInPreservesAccount(t *testing.T) {
	ConfigureAccountSelection("original", nil)
	original := &session{accountID: "original", xuid: "original-xuid"}
	cached = original
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requests := 0
	browserTestHTTP(t, func(r *http.Request) (*http.Response, error) {
		requests++
		return browserTestResponse(200, `{"user_code":"TEST-CODE","device_code":"private-device","verification_uri":"https://www.microsoft.com/link","expires_in":900,"interval":5}`)
	})
	err := SignInBrowser(ctx, func(code BrowserSignInCode) {
		if code.UserCode != "TEST-CODE" {
			t.Fatal("missing visible challenge")
		}
		cancel()
	}, func(string, string, string) error { t.Fatal("canceled sign-in persisted account"); return nil })
	if !errors.Is(err, context.Canceled) || requests != 1 || preferredAccountID != "original" || cached != original || selectedBrowser != nil {
		t.Fatalf("cancellation replaced previous account: %v", err)
	}
}

func TestBrowserSignInCommitsOnlyAfterXboxAndPersistence(t *testing.T) {
	ConfigureAccountSelection("original", nil)
	paths := []string{}
	browserTestHTTP(t, func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Host+r.URL.Path)
		if r.URL.Host == "login.live.com" {
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("client_id") != "000000004424da1f" {
				t.Fatal("device request and refresh must use the Store-authorized Xbox client")
			}
		}
		switch r.URL.Path {
		case "/oauth20_connect.srf":
			if r.Form.Get("scope") != xblScope {
				t.Fatal("wrong initial Xbox scope")
			}
			return browserTestResponse(200, `{"user_code":"TEST-CODE","device_code":"private-device","verification_uri":"https://www.microsoft.com/link","expires_in":900,"interval":5}`)
		case "/oauth20_token.srf":
			if r.Form.Get("grant_type") == "refresh_token" {
				if r.Form.Get("scope") != "service::www.microsoft.com::MBI_SSL" || r.Form.Get("refresh_token") != "test-refresh" {
					t.Fatal("Store token refresh used a different identity or scope")
				}
				return browserTestResponse(200, `{"access_token":"test-store-ticket","refresh_token":"rotated-refresh","user_id":"test-user","expires_in":3600}`)
			}
			return browserTestResponse(200, `{"access_token":"test-access","refresh_token":"test-refresh","user_id":"test-user","expires_in":3600}`)
		case "/user/authenticate":
			return browserTestResponse(200, `{"Token":"test-user-token"}`)
		case "/xsts/authorize":
			return browserTestResponse(200, `{"Token":"test-xsts","DisplayClaims":{"xui":[{"uhs":"test-hash","xid":"123","gtg":"Test"}]}}`)
		}
		t.Fatal("unexpected endpoint")
		return nil, nil
	})
	persisted := false
	err := SignInBrowser(context.Background(), func(BrowserSignInCode) {}, func(id, previous, next string) error {
		if previous != "" {
			if id != "browser:test-user" || previous != "test-refresh" || next != "rotated-refresh" {
				t.Fatal("incorrect rotated credential persisted")
			}
			return nil
		}
		if next != "test-refresh" {
			t.Fatal("initial refresh token was not saved")
		}
		if preferredAccountID != "original" || selectedBrowser != nil || id != "browser:test-user" {
			t.Fatal("account published before persistence")
		}
		persisted = true
		return nil
	})
	if err != nil || !persisted || len(paths) != 4 || cached == nil || cached.xuid != "123" || selectedBrowser == nil {
		t.Fatalf("browser session was not committed: %v", err)
	}
	if _, _, err := GetStoreTicketForUser(context.Background(), "different-profile"); !errors.Is(err, ErrAccountChanged) || len(paths) != 4 {
		t.Fatalf("Store request must reject a stale profile before sending credentials: %v", err)
	}
	ticket, reference, err := GetStoreTicketForUser(context.Background(), "123")
	if err != nil || ticket != "test-store-ticket" || reference != "test-user" || selectedBrowser.refresh != "rotated-refresh" || selectedBrowser.access != "test-access" {
		t.Fatalf("Store authorization did not preserve the Xbox account and rotate its refresh token: %v", err)
	}
	resetCache()
	id, err := GetLocalUserId()
	if err != nil || id != 123 {
		t.Fatalf("reset lost temporary browser credential: %v", err)
	}
}

func TestBrowserRefreshRejectsAccountSwitchAndStoreScopeDenial(t *testing.T) {
	for _, store := range []bool{false, true} {
		t.Run(map[bool]string{false: "account-changed", true: "store-denied"}[store], func(t *testing.T) {
			ConfigureAccountSelection("browser:chosen", nil)
			selectedBrowser = &browserCredential{userID: "chosen", refresh: "original-refresh"}
			browserTestHTTP(t, func(r *http.Request) (*http.Response, error) {
				if store {
					return browserTestResponse(400, `{"error":"invalid_scope"}`)
				}
				return browserTestResponse(200, `{"access_token":"foreign","refresh_token":"foreign-refresh","user_id":"other","expires_in":3600}`)
			})
			want := ErrAccountChanged
			scope := xblScope
			if store {
				want = ErrBrowserStoreAuthorization
				scope = "service::www.microsoft.com::MBI_SSL"
			}
			_, _, err := requestSelectedTicket(context.Background(), scope, "browser:chosen")
			if !errors.Is(err, want) || selectedBrowser.refresh != "original-refresh" {
				t.Fatalf("credential changed after rejected response: %v", err)
			}
		})
	}
}

func TestBrowserSessionRestoresAndPersistsRotation(t *testing.T) {
	ConfigureAccountSelection("", nil)
	const account = "browser:test-user"
	stored := "saved-refresh"
	browserTestHTTP(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "login.live.com/oauth20_token.srf":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != stored {
				t.Fatal("restore did not use the current saved refresh token")
			}
			return browserTestResponse(200, `{"access_token":"test-access","refresh_token":"rotated","user_id":"test-user","expires_in":3600}`)
		case "user.auth.xboxlive.com/user/authenticate":
			return browserTestResponse(200, `{"Token":"test-user-token"}`)
		case "xsts.auth.xboxlive.com/xsts/authorize":
			return browserTestResponse(200, `{"Token":"test-xsts","DisplayClaims":{"xui":[{"uhs":"test-hash","xid":"123","gtg":"Test"}]}}`)
		}
		t.Fatal("unexpected request during session restoration")
		return nil, nil
	})
	ctx := context.Background()
	load := func() (string, string, error) { return account, stored, nil }
	rotate := func(id, prior, next string) error {
		if id != account || prior != stored || next != "rotated" {
			t.Fatal("wrong refresh token rotation")
		}
		stored = next
		return nil
	}
	for range 2 {
		ConfigureAccountSelection("", nil)
		if err := RestoreAccountSelection(ctx, load, rotate); err != nil {
			t.Fatal(err)
		}
		if id, err := GetLocalUserId(); err != nil || id != 123 {
			t.Fatalf("could not restore Xbox session: %v", err)
		}
		if stored != "rotated" {
			t.Fatal("refresh token rotation was not persisted")
		}
	}
}
