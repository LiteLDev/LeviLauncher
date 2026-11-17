//go:build darwin || linux

package xbox

import (
	"net/http"
	"strconv"
	"time"
)

// Xbox token exchanges are stateless. Retry only temporary network/server
// failures, never an OAuth refresh or a rejected identity/entitlement.
func doXboxRequest(req *http.Request) (*http.Response, error) {
	if req.URL.Hostname() != "user.auth.xboxlive.com" && req.URL.Hostname() != "xsts.auth.xboxlive.com" {
		return httpClient.Do(req)
	}
	current := req
	for attempt := 0; ; attempt++ {
		resp, err := httpClient.Do(current)
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		if attempt == 2 || req.Context().Err() != nil || !browserPollMayRetry(status, err) || (req.Body != nil && req.GetBody == nil) {
			return resp, err
		}
		delay := time.Duration(attempt+1) * 250 * time.Millisecond
		if resp != nil {
			if seconds, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && seconds > 0 {
				delay = time.Duration(seconds) * time.Second
			}
			resp.Body.Close()
		}
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
		current = req.Clone(req.Context())
		if req.GetBody != nil {
			body, e := req.GetBody()
			if e != nil {
				return nil, e
			}
			current.Body = body
		}
	}
}
