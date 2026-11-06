package engine

import (
	"net/http"

	"github.com/liteldev/LeviLauncher/internal/lipcore/httpclient"
)

// RegistryOption configures NewGitRegistry.
type RegistryOption func(*registryOptions)

type registryOptions struct {
	http *http.Client
}

// WithHTTPClient makes git requests and asset downloads use c, e.g. one
// whose transport follows the system proxy as lip's HttpClient.DefaultProxy
// does. c should have no overall Timeout: downloads use an idle timeout and
// git requests their own contexts. Unless c sets CheckRedirect, lip's
// redirect policy (httpclient.CheckRedirect) is applied.
func WithHTTPClient(c *http.Client) RegistryOption {
	return func(o *registryOptions) { o.http = c }
}

// registryHTTPClient applies opts and returns the client for git and downloads.
func registryHTTPClient(opts []RegistryOption) *http.Client {
	var o registryOptions
	for _, opt := range opts {
		opt(&o)
	}
	return httpclient.WithLipRedirects(o.http)
}
