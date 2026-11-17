//go:build darwin || linux

package xbox

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/liteldev/LeviLauncher/internal/httpx"
)

var xboxDirectClient = httpx.NewDirectClient(30 * time.Second)
var xboxUsesSystemProxy = httpx.UsesSystemProxy

// Xbox token exchanges are stateless. Retry only temporary network/server
// failures, never an OAuth refresh or a rejected identity/entitlement.
func doXboxRequest(req *http.Request) (*http.Response, error) {
	if req.URL.Hostname() != "user.auth.xboxlive.com" && req.URL.Hostname() != "xsts.auth.xboxlive.com" {
		return httpClient.Do(req)
	}
	current := req
	client := httpClient
	for attempt := 0; ; attempt++ {
		resp, err := client.Do(current)
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		if attempt == 2 || req.Context().Err() != nil || !browserPollMayRetry(status, err) || (req.Body != nil && req.GetBody == nil) {
			return resp, err
		}
		if attempt == 1 && err != nil && xboxUsesSystemProxy(req) {
			// Keep this within the same verified Xbox host, and use it only
			// after automatic proxy transport failures, never an HTTP rejection.
			client = xboxDirectClient
			log.Printf("Xbox authorization: system proxy connection failed for %s; retrying directly", req.URL.Hostname())
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
