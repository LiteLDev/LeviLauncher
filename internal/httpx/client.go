package httpx

import (
	"net/http"
	"time"
)

type defaultHeaderTransport struct {
	base http.RoundTripper
}

func (t *defaultHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ApplyDefaultHeaders(req)
	return t.base.RoundTrip(req)
}

var sharedTransport http.RoundTripper = &defaultHeaderTransport{base: newBaseTransport()}

var sharedClient = &http.Client{
	Transport: sharedTransport,
}

func DefaultClient() *http.Client {
	return sharedClient
}

// NewBlockingProxyTransport returns a transport with its own connection
// pool that applies the default headers and follows the system proxy like
// the shared one, but waits for PAC/WPAD resolution instead of falling back
// to the manual proxy or a direct connection when the lookup is slow or
// already in flight. Use it for background work that must reach the
// network the way .NET's HttpClient.DefaultProxy would.
func NewBlockingProxyTransport() http.RoundTripper {
	return &defaultHeaderTransport{base: newBlockingBaseTransport()}
}

func NewClient(timeout time.Duration) *http.Client {
	client := &http.Client{
		Transport: sharedTransport,
	}
	if timeout > 0 {
		client.Timeout = timeout
	}
	return client
}

func Do(req *http.Request) (*http.Response, error) {
	ApplyDefaultHeaders(req)
	return sharedClient.Do(req)
}

func Get(target string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	return Do(req)
}
