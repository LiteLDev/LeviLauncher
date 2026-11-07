// Package httpclient holds the HTTP client defaults lipcore shares with
// lip's Flurl configuration.
package httpclient

import (
	"errors"
	"net/http"
)

// maxRedirects is Flurl's default Redirects.MaxAutoRedirects.
const maxRedirects = 10

// CheckRedirect is Flurl 4's default redirect policy
// (Redirects.AllowSecureToInsecure = false): an https request is never
// redirected to plain http. The 3xx response is returned instead, so the
// caller's status check fails as lip's does.
func CheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errors.New("stopped after 10 redirects")
	}
	if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
		return http.ErrUseLastResponse
	}
	return nil
}

// Default is used when a caller does not supply a client: Go's default
// transport (proxy from HTTP(S)_PROXY) with CheckRedirect.
var Default = &http.Client{CheckRedirect: CheckRedirect}

// WithLipRedirects returns c, or a copy of it using CheckRedirect when c has
// no redirect policy of its own. A nil c yields Default.
func WithLipRedirects(c *http.Client) *http.Client {
	if c == nil {
		return Default
	}
	if c.CheckRedirect != nil {
		return c
	}
	cp := *c
	cp.CheckRedirect = CheckRedirect
	return &cp
}
