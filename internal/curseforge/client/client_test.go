package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/curseforge/client/types"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type trackingReadCloser struct {
	io.Reader
	closed atomic.Bool
}

func (body *trackingReadCloser) Close() error {
	body.closed.Store(true)
	return nil
}

func TestExecuteRequestClosesResponseBodyAndPreservesRawBody(t *testing.T) {
	t.Parallel()

	const rawBody = `{"data":"description"}`
	body := &trackingReadCloser{Reader: strings.NewReader(rawBody)}
	client := newTestCurseClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       body,
			Header:     make(http.Header),
		}, nil
	}))
	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://example.invalid/test",
		nil,
	)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	var response types.StringResponse
	if err := client.executeRequest(request, "test", &response); err != nil {
		t.Fatalf("execute request: %v", err)
	}
	if !body.closed.Load() {
		t.Fatal("response body was not closed")
	}
	if response.Data != "description" {
		t.Fatalf("data = %q, want description", response.Data)
	}
	if response.RawBody != rawBody {
		t.Fatalf("raw body = %q, want %q", response.RawBody, rawBody)
	}
}

func TestExecuteRequestPreservesNon2xxStatusAndBody(t *testing.T) {
	t.Parallel()

	const rawBody = `{"error":"limited"}`
	body := &trackingReadCloser{Reader: strings.NewReader(rawBody)}
	client := newTestCurseClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       body,
			Header:     make(http.Header),
		}, nil
	}))
	request, err := http.NewRequest(
		http.MethodGet,
		"https://example.invalid/test",
		nil,
	)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	var response types.StringResponse
	err = client.executeRequest(request, "test", &response)
	if err == nil {
		t.Fatal("expected non-2xx error")
	}
	if !body.closed.Load() {
		t.Fatal("response body was not closed")
	}

	var apiError types.CurseforgeAPIError
	if !errors.As(err, &apiError) {
		t.Fatalf("error type = %T, want CurseforgeAPIError: %v", err, err)
	}
	if apiError.Status != http.StatusTooManyRequests ||
		apiError.Message != rawBody {
		t.Fatalf("api error = %#v", apiError)
	}
}

func TestExecuteRequestPreservesDecodeErrorBody(t *testing.T) {
	t.Parallel()

	const rawBody = `{"data":`
	client := newTestCurseClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(rawBody)),
			Header:     make(http.Header),
		}, nil
	}))
	request, err := http.NewRequest(
		http.MethodGet,
		"https://example.invalid/test",
		nil,
	)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	var response types.StringResponse
	err = client.executeRequest(request, "test", &response)
	if err == nil {
		t.Fatal("expected decode error")
	}

	var apiError types.CurseforgeAPIError
	if !errors.As(err, &apiError) {
		t.Fatalf("error type = %T, want CurseforgeAPIError: %v", err, err)
	}
	if apiError.Status != http.StatusOK || apiError.Message != rawBody {
		t.Fatalf("api error = %#v", apiError)
	}
}

func newTestCurseClient(transport http.RoundTripper) *curseClient {
	config := NewDefaultConfig("test-key")
	return &curseClient{
		opt:   *config,
		c:     &http.Client{Transport: transport},
		cache: make(map[string]cachedResponse),
	}
}
