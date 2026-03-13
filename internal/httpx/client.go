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
