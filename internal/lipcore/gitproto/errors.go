package gitproto

import (
	"fmt"

	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
)

// ProtocolError is returned when a remote misbehaves, reports an error or
// sends malformed data (lip's GitProtocolException). It is never retried.
type ProtocolError = gitpack.ProtocolError

// UnusableRepositoryError is lip's UnusableRepositoryException: the content
// of a repository is something lip cannot handle. Trying again or using
// another server does not help.
type UnusableRepositoryError = gitpack.UnusableRepositoryError

func protocolErrorf(format string, args ...any) *ProtocolError {
	return &ProtocolError{Message: fmt.Sprintf(format, args...)}
}

// ServerError is an error the server reported itself, in an ERR line or on
// side-band channel 3 (lip's GitServerErrorException, a
// GitProtocolException).
type ServerError struct {
	// ServerMessage is the server's own text.
	ServerMessage string
}

func (e *ServerError) Error() string { return "The git server reported: " + e.ServerMessage }

// UnsupportedServerError is a server that works but lacks a feature lip
// needs (lip's UnsupportedServerException, a GitProtocolException).
type UnsupportedServerError struct{ Message string }

func (e *UnsupportedServerError) Error() string { return e.Message }

// UnsupportedObjectFormatError is a repository whose object ids are not
// SHA-1 (lip's UnsupportedObjectFormatException, a GitProtocolException).
type UnsupportedObjectFormatError struct{ Format string }

func (e *UnsupportedObjectFormatError) Error() string {
	return "The repository uses the " + e.Format + " object format; lip supports only sha1."
}

// isGitProtocolError reports whether err itself (not an error it wraps) is
// one of the errors that are a GitProtocolException in lip.
func isGitProtocolError(err error) bool {
	switch err.(type) {
	case *ProtocolError, *UnusableRepositoryError, *ServerError, *UnsupportedServerError,
		*UnsupportedObjectFormatError:
		return true
	}
	return false
}

// StatusError is a response with a non-2xx status (the HttpRequestException
// of lip's GitRemoteClient.EnsureSuccess).
type StatusError struct {
	Verb       string
	URL        string
	StatusCode int
	// Reason is the reason phrase of the status line, if any.
	Reason string
	// Body is the start of the response body (at most 2000 characters, then
	// "..."), or "(cannot read: <error>)" (lip's Data["Response body"]).
	Body string
}

func (e *StatusError) Error() string {
	reason := ""
	if e.Reason != "" {
		reason = " (" + e.Reason + ")"
	}
	return fmt.Sprintf("The server answered HTTP %d%s to %s %s.", e.StatusCode, reason, e.Verb, e.URL)
}

// RequestError is an HTTP failure explained in plain words (the
// HttpRequestException lip's GitRemoteClient.Explain creates). StatusCode
// is 0 when there was no response.
type RequestError struct {
	Message    string
	StatusCode int
	Err        error
}

func (e *RequestError) Error() string { return e.Message }

func (e *RequestError) Unwrap() error { return e.Err }

// InvalidProxyError is a github_proxy value that does not give a valid URL
// (the UriFormatException of lip's GitRemoteClient.Explain).
type InvalidProxyError struct {
	Message string
	Err     error
}

func (e *InvalidProxyError) Error() string { return e.Message }

func (e *InvalidProxyError) Unwrap() error { return e.Err }

// AggregateError holds several failures (lip's AggregateException).
type AggregateError struct {
	Message string
	Errs    []error
}

func (e *AggregateError) Error() string { return e.Message }

func (e *AggregateError) Unwrap() []error { return e.Errs }

// transportError marks a failure of the connection or of reading a
// response body (lip: FlurlHttpException without a response, or
// HttpIOException/IOException while reading the body).
type transportError struct {
	err error
	// body is set when reading the response body failed.
	body bool
}

func (e *transportError) Error() string { return e.err.Error() }

func (e *transportError) Unwrap() error { return e.err }

// timeoutError is a request that hit its per-request timeout
// (FlurlHttpTimeoutException).
type timeoutError struct{ url string }

func (e *timeoutError) Error() string { return "Git request to " + e.url + " timed out" }

// uriFormatError is a request URL that is not a valid absolute URL
// (UriFormatException).
type uriFormatError struct{ err error }

func (e *uriFormatError) Error() string { return e.err.Error() }

func (e *uriFormatError) Unwrap() error { return e.err }
