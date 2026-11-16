//go:build windows

package xbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BrowserSignInCode contains only the user-facing device challenge. Credentials
// and the device_code stay in the backend, never in bindings or event payloads.
type BrowserSignInCode struct {
	UserCode        string `json:"userCode"`
	VerificationURL string `json:"verificationURL"`
	ExpiresIn       int    `json:"expiresIn"`
}

type browserCredential struct {
	userID, refresh, access string
	expires                 time.Time
}

// Protected by sessionGate, like cached and preferredAccountID. These temporary
// compatibility credentials live only until the launcher process closes.
var selectedBrowser *browserCredential

// The Minecraft Launcher client used by native WAM is not pre-authorized for
// Store OAuth scopes. This Xbox public client is used by Xodus and supports
// both Xbox Live and service::www.microsoft.com::MBI_SSL device authorization.
// Use the same client for the device request, polling and every refresh.
const browserClientID = "000000004424da1f"

type browserToken struct {
	Access    string `json:"access_token"`
	Refresh   string `json:"refresh_token"`
	UserID    string `json:"user_id"`
	ExpiresIn int    `json:"expires_in"`
	Error     string `json:"error"`
}

var ErrBrowserStoreAuthorization = errors.New("ERR_AUTH_BROWSER_STORE_UNAVAILABLE")

func browserPost(ctx context.Context, endpoint string, values url.Values, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	defer clear(data)
	if err := json.Unmarshal(data, out); err != nil {
		return resp.StatusCode, fmt.Errorf("Microsoft OAuth returned an invalid response (HTTP %d)", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func browserOAuthError(code string) error {
	switch code {
	case "authorization_declined", "access_denied":
		return context.Canceled
	case "expired_token", "code_expired":
		return context.DeadlineExceeded
	case "invalid_grant":
		return ErrInteractionRequired
	case "invalid_scope":
		return ErrBrowserStoreAuthorization
	default:
		return ErrAuthenticationFailed
	}
}

func pollBrowserToken(ctx context.Context, deviceCode string, interval time.Duration) (browserToken, error) {
	for {
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return browserToken{}, ctx.Err()
		case <-timer.C:
		}
		var token browserToken
		status, err := browserPost(ctx, "https://login.live.com/oauth20_token.srf", url.Values{
			"client_id": {browserClientID}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "device_code": {deviceCode},
		}, &token)
		if err != nil {
			return browserToken{}, err
		}
		switch token.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "":
			if status == http.StatusOK && token.Access != "" && token.Refresh != "" && token.UserID != "" && token.ExpiresIn > 0 {
				return token, nil
			}
		}
		return browserToken{}, browserOAuthError(token.Error)
	}
}

// SignInBrowser uses Microsoft's device authorization grant without native WAM.
// Like native sign-in, it commits identity only after Xbox auth and persistence.
func SignInBrowser(ctx context.Context, notify func(BrowserSignInCode), persist func(string) error) error {
	return signInSession(ctx, func() (*session, error) {
		var challenge struct {
			UserCode        string `json:"user_code"`
			DeviceCode      string `json:"device_code"`
			VerificationURL string `json:"verification_uri"`
			ExpiresIn       int    `json:"expires_in"`
			Interval        int    `json:"interval"`
			Error           string `json:"error"`
		}
		status, err := browserPost(ctx, "https://login.live.com/oauth20_connect.srf", url.Values{
			"client_id": {browserClientID}, "scope": {xblScope}, "response_type": {"device_code"},
		}, &challenge)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK || challenge.Error != "" {
			return nil, browserOAuthError(challenge.Error)
		}
		if challenge.UserCode == "" || challenge.DeviceCode == "" || challenge.VerificationURL != "https://www.microsoft.com/link" || challenge.ExpiresIn <= 0 || challenge.ExpiresIn > 3600 {
			return nil, ErrAuthenticationFailed
		}
		waitCtx, cancel := context.WithTimeout(ctx, time.Duration(challenge.ExpiresIn)*time.Second)
		defer cancel()
		notify(BrowserSignInCode{challenge.UserCode, challenge.VerificationURL, challenge.ExpiresIn})
		interval := max(challenge.Interval, 5)
		token, err := pollBrowserToken(waitCtx, challenge.DeviceCode, time.Duration(interval)*time.Second)
		if err != nil {
			return nil, err
		}
		userToken, err := xblUserAuth(ctx, token.Access)
		if err != nil {
			return nil, err
		}
		s, err := xblXSTS(ctx, userToken)
		if err != nil {
			return nil, err
		}
		if s.xuid == "" {
			return nil, ErrAuthenticationFailed
		}
		s.accountID = "browser:" + token.UserID
		s.browser = &browserCredential{userID: token.UserID, refresh: token.Refresh, access: token.Access, expires: time.Now().Add(time.Duration(token.ExpiresIn)*time.Second - sessionSkew)}
		return s, nil
	}, persist)
}

func requestSelectedTicket(ctx context.Context, scope, expectedAccount string) (string, string, error) {
	if !strings.HasPrefix(expectedAccount, "browser:") {
		return requestWAMTicket(ctx, scope, expectedAccount)
	}
	c := selectedBrowser
	if c == nil || expectedAccount != "browser:"+c.userID {
		return "", "", ErrInteractionRequired
	}
	if scope == xblScope && time.Now().Before(c.expires) {
		return c.access, c.userID, nil
	}
	var token browserToken
	status, err := browserPost(ctx, "https://login.live.com/oauth20_token.srf", url.Values{
		"client_id": {browserClientID}, "scope": {scope}, "grant_type": {"refresh_token"}, "refresh_token": {c.refresh},
	}, &token)
	if err != nil {
		return "", "", err
	}
	if status != http.StatusOK || token.Error != "" {
		return "", "", browserOAuthError(token.Error)
	}
	if token.UserID != "" && token.UserID != c.userID {
		return "", "", ErrAccountChanged
	}
	if token.Access == "" {
		return "", "", ErrAuthenticationFailed
	}
	if token.Refresh != "" {
		c.refresh = token.Refresh
	}
	if scope == xblScope {
		c.access = token.Access
		c.expires = time.Now().Add(time.Duration(token.ExpiresIn)*time.Second - sessionSkew)
	}
	return token.Access, c.userID, nil
}
