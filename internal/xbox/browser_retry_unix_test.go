//go:build darwin

package xbox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestDeviceCodePollingRecoversFromTemporaryProxyFailure(t *testing.T) {
	calls := 0
	browserTestHTTP(t, func(r *http.Request) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			return nil, io.ErrUnexpectedEOF
		case 2:
			return browserTestResponse(503, `<html>Service unavailable</html>`)
		default:
			return browserTestResponse(200, `{"access_token":"test-access","refresh_token":"test-refresh","user_id":"test-user","expires_in":3600}`)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	token, err := pollBrowserToken(ctx, "test-device", time.Millisecond)
	if err != nil || token.UserID != "test-user" || calls != 3 {
		t.Fatalf("temporary failure aborted polling: calls=%d error=%v", calls, err)
	}
}

func TestDeviceCodePollingDoesNotRetryDenialAndHasFailureLimit(t *testing.T) {
	for _, denied := range []bool{true, false} {
		t.Run(map[bool]string{true: "denied", false: "proxy-down"}[denied], func(t *testing.T) {
			calls := 0
			browserTestHTTP(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if denied {
					return browserTestResponse(400, `{"error":"authorization_declined"}`)
				}
				return nil, io.EOF
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := pollBrowserToken(ctx, "test-device", time.Millisecond)
			if denied && (calls != 1 || !errors.Is(err, context.Canceled)) {
				t.Fatalf("denial was retried: %d %v", calls, err)
			}
			if !denied && (calls != 4 || !errors.Is(err, io.EOF)) {
				t.Fatalf("unexpected retry limit: %d %v", calls, err)
			}
		})
	}
}
