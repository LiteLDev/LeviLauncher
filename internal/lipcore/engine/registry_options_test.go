package engine

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
)

func TestNewGitRegistryUsesHTTPClient(t *testing.T) {
	proxied := &url.URL{Scheme: "http", Host: "127.0.0.1:7890"}
	transport := &http.Transport{Proxy: http.ProxyURL(proxied)}
	reg := NewGitRegistry("", nil, WithHTTPClient(&http.Client{Transport: transport}))

	git, ok := reg.Git.(*gitproto.Client)
	if !ok {
		t.Fatalf("Git = %T", reg.Git)
	}
	for name, c := range map[string]*http.Client{"git": git.HTTP, "downloader": reg.Downloader.HTTP} {
		if c == nil || c.Transport != transport {
			t.Errorf("%s client does not use the supplied transport", name)
			continue
		}
		next, _ := http.NewRequest(http.MethodGet, "http://example.com/b", nil)
		prev, _ := http.NewRequest(http.MethodGet, "https://example.com/a", nil)
		if c.CheckRedirect == nil || c.CheckRedirect(next, []*http.Request{prev}) != http.ErrUseLastResponse {
			t.Errorf("%s client follows https -> http redirects", name)
		}
	}
}

func TestNewGitRegistryDefaultClientRefusesDowngrade(t *testing.T) {
	reg := NewGitRegistry("", nil)
	c := reg.Downloader.HTTP
	if c == nil || c == http.DefaultClient || c.CheckRedirect == nil {
		t.Fatal("default downloader client lacks lip's redirect policy")
	}
}
