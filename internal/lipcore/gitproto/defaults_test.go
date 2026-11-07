package gitproto

import (
	"net/http"
	"testing"
)

func TestDefaultClientRefusesHTTPSToHTTPRedirect(t *testing.T) {
	c := (&RemoteClient{}).httpClient()
	if c == http.DefaultClient || c.CheckRedirect == nil {
		t.Fatal("zero RemoteClient uses a client without lip's redirect policy")
	}
	next, _ := http.NewRequest(http.MethodGet, "http://example.com/repo.git/info/refs", nil)
	prev, _ := http.NewRequest(http.MethodGet, "https://example.com/repo.git/info/refs", nil)
	if err := c.CheckRedirect(next, []*http.Request{prev}); err != http.ErrUseLastResponse {
		t.Fatalf("https -> http redirect: got %v, want ErrUseLastResponse", err)
	}
}
