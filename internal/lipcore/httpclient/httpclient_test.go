package httpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckRedirectRefusesHTTPSToHTTP(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("plain"))
	}))
	defer plain.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/same":
			_, _ = w.Write([]byte("secure"))
		case "/hop":
			http.Redirect(w, r, "/same", http.StatusFound)
		default:
			http.Redirect(w, r, plain.URL+"/x", http.StatusMovedPermanently)
		}
	}))
	defer secure.Close()

	c := WithLipRedirects(secure.Client())
	resp, err := c.Get(secure.URL + "/down")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("https->http redirect followed: status %d", resp.StatusCode)
	}

	resp, err = c.Get(secure.URL + "/hop")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("https->https redirect not followed: status %d", resp.StatusCode)
	}
}

func TestCheckRedirectAllowsHTTPToHTTPS(t *testing.T) {
	up, _ := http.NewRequest(http.MethodGet, "https://example.com/b", nil)
	insecure, _ := http.NewRequest(http.MethodGet, "http://example.com/a", nil)
	if err := CheckRedirect(up, []*http.Request{insecure}); err != nil {
		t.Fatalf("http->https refused: %v", err)
	}
	down, _ := http.NewRequest(http.MethodGet, "http://example.com/b", nil)
	secure, _ := http.NewRequest(http.MethodGet, "https://example.com/a", nil)
	if err := CheckRedirect(down, []*http.Request{secure}); err != http.ErrUseLastResponse {
		t.Fatalf("https->http: got %v, want ErrUseLastResponse", err)
	}
}

func TestWithLipRedirectsKeepsCallerPolicy(t *testing.T) {
	if WithLipRedirects(nil) != Default {
		t.Fatal("nil client should map to Default")
	}
	own := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return nil }}
	if WithLipRedirects(own) != own {
		t.Fatal("a client with its own policy should be returned unchanged")
	}
	bare := &http.Client{}
	got := WithLipRedirects(bare)
	if got == bare || got.CheckRedirect == nil || bare.CheckRedirect != nil {
		t.Fatal("a bare client should be copied with CheckRedirect set")
	}
}
