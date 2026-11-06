package gitproto

import (
	"context"
	"errors"
	"fmt"

	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
)

// ProtocolError is returned when a remote misbehaves, reports an error or
// sends malformed data (lip's GitProtocolException). It is never retried.
type ProtocolError struct {
	Message string
	Err     error
}

func (e *ProtocolError) Error() string { return e.Message }

func (e *ProtocolError) Unwrap() error { return e.Err }

func protocolErrorf(format string, args ...any) *ProtocolError {
	return &ProtocolError{Message: fmt.Sprintf(format, args...)}
}

// HTTPError is a non-2xx response (lip's HttpRequestException with a status
// code). 5xx and 429 are retried.
type HTTPError struct {
	URL        string
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Git request to %s failed: HTTP %d", e.URL, e.StatusCode)
}

// transportError marks a failure of the connection or of reading a
// response body (lip: HttpRequestException without status or IOException).
type transportError struct{ err error }

func (e *transportError) Error() string { return e.err.Error() }

func (e *transportError) Unwrap() error { return e.err }

// timeoutError is a request that hit its per-request timeout
// (FlurlHttpTimeoutException).
type timeoutError struct{ url string }

func (e *timeoutError) Error() string { return "Git request to " + e.url + " timed out" }

// isTransient mirrors lip's GitRemoteClient.IsTransient: network failures
// without a response, timeouts, IO errors, 5xx and 429.
func isTransient(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	// A pack handler's own failure is not retried: lip's FetchBlobBatch
	// retries only the download.
	if _, ok := err.(*fetch.NoFallbackError); ok {
		return false
	}
	var pe *ProtocolError
	if errors.As(err, &pe) {
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return he.StatusCode >= 500 || he.StatusCode == 429
	}
	var te *transportError
	if errors.As(err, &te) {
		return true
	}
	var to *timeoutError
	return errors.As(err, &to)
}
