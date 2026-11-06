// Package gitproto is a minimal git smart-HTTP protocol v2 client, porting
// lip 0.37.0 src/Lip.Core/Git/GitRemoteClient.cs and PktLine.cs.
//
// It lists version tags (ls-refs) and fetches packfiles (fetch). Packfile
// decoding lives in package gitpack.
package gitproto

import (
	"context"
	"errors"
	"net/http"

	xsemver "golang.org/x/mod/semver"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("gitproto: not implemented")

// UserAgent is sent with every request, as lip does.
const UserAgent = "git/2.0 (lip)"

// Proxy routes a request URL through a mirror with fallback. fn is called
// with the URL to use; Try returns fn's final error. fetch.ProxyRewriter
// implements it. A nil Proxy means direct access.
type Proxy interface {
	Try(ctx context.Context, rawURL string, fn func(url string) error) error
}

// Client talks to remote repositories such as
// "https://github.com/LiteLDev/LeviLamina.git".
type Client struct {
	// HTTP is the client to use; nil means http.DefaultClient.
	HTTP *http.Client
	// Proxy is optional.
	Proxy Proxy
}

// TagRef is one version tag.
type TagRef struct {
	// Name is the tag name without "refs/tags/", e.g. "v1.2.3".
	Name string
	// Version is the canonical version without "v" (Go semver.Canonical, so
	// "+build" is dropped and "v1.2" becomes "1.2.0").
	Version string
	// Commit is the peeled commit id for annotated tags, else the ref id.
	Commit string
}

// ListTags performs discovery (GET <repo>/info/refs?service=git-upload-pack
// with Git-Protocol: version=2; requires "version 2", ls-refs and fetch,
// object-format sha1 if present; follows redirects, using the final URL as
// the POST endpoint) and "ls-refs" with "peel" and "ref-prefix refs/tags/v".
// Non-semver tags are dropped; when several tags map to the same version,
// "refs/tags/v<version>" wins, otherwise the ordinally smallest name.
// Results are sorted by tag name. Retries per lip (2 retries on 5xx, 429,
// timeouts or IO errors, 500ms*n backoff).
func (c *Client) ListTags(ctx context.Context, repoURL string) ([]TagRef, error) {
	return nil, ErrNotImplemented
}

// FetchOptions configures a fetch command.
type FetchOptions struct {
	// Shallow adds "deepen 1" when the server advertises "shallow".
	Shallow bool
	// BlobNone adds "filter blob:none" when the server advertises "filter".
	BlobNone bool
}

// Fetch runs the v2 "fetch" command (no-progress, ofs-delta, wants, done)
// and returns the raw packfile bytes from side-band channel 1; channel 2 is
// progress (ignored), channel 3 is a fatal error. At most 256 wants are sent
// per request; callers batch larger sets.
func (c *Client) Fetch(ctx context.Context, repoURL string, wants []string, opts FetchOptions) ([]byte, error) {
	return nil, ErrNotImplemented
}

// RepoURL maps a package path to its repository URL ("https://<path>.git").
func RepoURL(packagePath string) string {
	return "https://" + packagePath + ".git"
}

// canonicalTag applies Go semver.Canonical (v1.2 -> v1.2.0, +build dropped);
// "" for invalid tags.
func canonicalTag(tag string) string { return xsemver.Canonical(tag) }
