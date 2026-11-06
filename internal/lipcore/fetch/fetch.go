// Package fetch provides lip 0.37.0's download plumbing: the on-disk cache
// (Services/CacheService.cs), GitHub proxy rewriting with fallback
// (Services/SourceService.cs), the HTTP file downloader with progress
// (Infrastructure/FileDownloader.cs) and archive asset sources
// (Sources/ArchiveSource.cs, SingleFileSource.cs).
package fetch

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("fetch: not implemented")

// Cache is lip's download cache. Entries live at
// <Root>/<EscapeKey(key)>; writes go to <TempRoot>/<random> then are moved
// into place.
type Cache struct {
	Root     string
	TempRoot string
}

// DefaultCache returns the cache shared with lip:
// %LOCALAPPDATA%\lip\cache and %LOCALAPPDATA%\lip\temp.
func DefaultCache() *Cache {
	panic("fetch: not implemented")
}

// EscapeKey escapes key like .NET Uri.EscapeDataString: every byte of the
// UTF-8 encoding except RFC 3986 unreserved characters (A-Z a-z 0-9 - _ . ~)
// becomes %XX with uppercase hex.
func EscapeKey(key string) string {
	panic("fetch: not implemented")
}

// BlobKey is the cache key for a git blob: "git-blob:<oid>".
func BlobKey(oid string) string { return "git-blob:" + oid }

// Path returns the file path for key (whether or not it exists).
func (c *Cache) Path(key string) string {
	panic("fetch: not implemented")
}

// Get returns the cached file path for key if present.
func (c *Cache) Get(key string) (path string, ok bool) {
	return "", false
}

// Put writes an entry via write (to a temp file, then moved into place)
// and returns its path. A failed write leaves no entry.
func (c *Cache) Put(key string, write func(w io.Writer) error) (string, error) {
	return "", ErrNotImplemented
}

// Clean deletes the cache directory, clearing read-only attributes first.
// A missing directory is not an error.
func (c *Cache) Clean() error {
	return ErrNotImplemented
}

// ProxyRewriter implements the GitHub proxy: for URLs whose host is exactly
// "github.com", the proxied URL is Proxy + original path (+ query and
// fragment). Try uses the proxied URL first; on failure it retries the
// direct URL and, for the rest of the process, skips the proxy for that
// repository (owner/repo). Implements gitproto.Proxy.
type ProxyRewriter struct {
	// Proxy is a prefix like "https://github.bibk.top" (no trailing slash);
	// empty disables proxying.
	Proxy string
	// Implementation fields may be added.
}

// NewProxyRewriter returns a rewriter for proxy (may be empty).
func NewProxyRewriter(proxy string) *ProxyRewriter {
	return &ProxyRewriter{Proxy: proxy}
}

// Rewrite returns the proxied URL and true, or rawURL and false when the
// URL is not eligible.
func (p *ProxyRewriter) Rewrite(rawURL string) (string, bool) {
	return rawURL, false
}

// Try calls fn with the proxied URL, falling back to rawURL.
func (p *ProxyRewriter) Try(ctx context.Context, rawURL string, fn func(url string) error) error {
	return ErrNotImplemented
}

// ProgressFunc receives download progress in [0, 100].
type ProgressFunc func(percentage float64)

// Downloader downloads asset URLs into the cache, keyed by the original
// (unproxied) URL. The GET must return 200.
type Downloader struct {
	HTTP  *http.Client
	Proxy *ProxyRewriter
	Cache *Cache
}

// Fetch returns the cached file for rawURL, downloading it first if needed.
// progress may be nil.
func (d *Downloader) Fetch(ctx context.Context, rawURL string, progress ProgressFunc) (string, error) {
	return "", ErrNotImplemented
}

// AssetSource exposes the files of one asset by key. Keys are archive entry
// names used verbatim (no "./" stripping; directories excluded); an
// uncompressed asset has the single key "".
type AssetSource interface {
	Keys() []string
	Open(key string) (io.ReadCloser, error)
	Close() error
}

// OpenArchive opens a downloaded asset file of the given manifest asset type
// ("zip", "tar", "tgz" or "uncompressed").
func OpenArchive(assetType string, path string) (AssetSource, error) {
	return nil, ErrNotImplemented
}
