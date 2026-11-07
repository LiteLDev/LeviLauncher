package fetch

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// ProxyRewriter implements the GitHub proxy: for URLs whose host is
// "github.com" (compared case-insensitively, as System.Uri lower-cases it),
// the proxied URL is Proxy + the original non-empty path segments (+ the
// original query and fragment when non-empty), built like Flurl's
// AppendPathSegments.
//
// Try is GitRemoteClient.WithProxyFallback: it uses the proxied URL first;
// on failure it retries the direct URL and, for the rest of the rewriter's
// lifetime, skips the proxy for that repository. lip keys repositories by
// GetRepoKey(repoUrl) (the github.com repository URL lower-cased without a
// trailing "/" or ".git"); this port keys them by the lower-cased first two
// path segments (owner/repo, ".git" stripped), which is identical for
// repository URLs and also groups sub-URLs of one repository. Try does not
// retry transient errors itself; lip retries inside each candidate.
// Implements gitproto.Proxy.
//
// Asset downloads (Downloader.Fetch, lip SourceService.GetDownloadUrls) use
// Rewrite only: the proxy then the direct URL, every time, with no skip.
type ProxyRewriter struct {
	// Proxy is a prefix like "https://github.bibk.top" (no trailing slash);
	// empty disables proxying.
	Proxy string

	failed sync.Map // repo key -> struct{}
}

// NewProxyRewriter returns a rewriter for proxy (may be empty).
func NewProxyRewriter(proxy string) *ProxyRewriter {
	return &ProxyRewriter{Proxy: proxy}
}

// Rewrite returns the proxied URL and true, or rawURL and false when the
// URL is not eligible (no proxy, unparsable URL, host other than
// github.com, or a rewrite that yields the same URL).
func (p *ProxyRewriter) Rewrite(rawURL string) (string, bool) {
	if p == nil {
		return rawURL, false
	}
	return rewriteGithub(p.Proxy, rawURL, true)
}

// RewriteGithub is lip 0.38's GithubProxy.Rewrite (src/Lip.Core/Git/
// GithubProxy.cs): when proxy is set and rawURL's host is github.com
// (compared case-insensitively) it returns proxy with rawURL's path
// segments appended and rawURL's query (when non-empty) in place of the
// proxy's, built like Flurl's Clone().AppendPathSegments(url.PathSegments).
// The original host is dropped and, unlike 0.37, the URL's fragment is not
// carried over. It returns rawURL and false when no proxy applies,
// including when the result equals rawURL. Download candidates are then
// [proxied, rawURL].
func RewriteGithub(proxy, rawURL string) (string, bool) {
	return rewriteGithub(proxy, rawURL, false)
}

// rewriteGithub is the rewrite shared by 0.37 (keepFragment: the URL's
// fragment replaces the proxy's) and 0.38 (the proxy's own is kept).
func rewriteGithub(proxy, rawURL string, keepFragment bool) (string, bool) {
	if proxy == "" {
		return rawURL, false
	}
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Hostname(), "github.com") {
		return rawURL, false
	}

	base, pq, pf := splitURL(proxy)
	var segs []string
	for _, s := range strings.Split(rawPath(rawURL), "/") {
		if s != "" {
			segs = append(segs, cleanSegment(s))
		}
	}
	if len(segs) > 0 {
		base = joinPath(base, strings.Join(segs, "/"))
	}
	if u.RawQuery != "" {
		pq = u.RawQuery
	}
	if f := u.EscapedFragment(); keepFragment && f != "" {
		pf = f
	}
	out := base
	if pq != "" {
		out += "?" + pq
	}
	if pf != "" {
		out += "#" + pf
	}
	if out == rawURL {
		return rawURL, false
	}
	return out, true
}

// splitURL splits s into the part before the query, the query and the
// fragment (without their '?' and '#').
func splitURL(s string) (base, query, fragment string) {
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s, fragment = s[:i], s[i+1:]
	}
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s, query = s[:i], s[i+1:]
	}
	return s, query, fragment
}

// rawPath returns the path of an absolute URL exactly as written (between
// the authority and the query or fragment), so that escapes such as %2F
// survive.
func rawPath(rawURL string) string {
	s, _, _ := splitURL(rawURL)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		if j := strings.IndexByte(s, '/'); j >= 0 {
			return s[j:]
		}
		return ""
	}
	return s
}

// cleanSegment percent-encodes the bytes of a path segment that may not
// appear in a URL path, keeping existing %XX escapes (Flurl CleanSegment).
func cleanSegment(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) || strings.IndexByte("!$&'()*+,;=:@%", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0xF])
	}
	return b.String()
}

// joinPath appends path to base with exactly one '/' between them.
func joinPath(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + path
}

// downloadURLs is lip's SourceService.GetDownloadUrls: the proxied URL (if
// any) then rawURL.
func (p *ProxyRewriter) downloadURLs(rawURL string) []string {
	if proxied, ok := p.Rewrite(rawURL); ok {
		return []string{proxied, rawURL}
	}
	return []string{rawURL}
}

// repoKey identifies the github.com repository of rawURL.
func repoKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	var segs []string
	for _, s := range strings.Split(u.Path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
		if len(segs) == 2 {
			break
		}
	}
	key := strings.ToLower(strings.Join(segs, "/"))
	return strings.TrimSuffix(key, ".git")
}

// NoFallbackError marks an error of an attempt that is not the remote's
// fault, such as a failure to store what was fetched (lip raises these
// outside FetchBlobBatch's try). Try returns Err unchanged, without marking
// the proxy failed or trying the original URL.
type NoFallbackError struct{ Err error }

func (e *NoFallbackError) Error() string { return e.Err.Error() }

func (e *NoFallbackError) Unwrap() error { return e.Err }

// NoFallback marks err (nil stays nil) as a NoFallbackError.
func NoFallback(err error) error {
	if err == nil {
		return nil
	}
	return &NoFallbackError{Err: err}
}

// StripNoFallback returns the error marked by NoFallback, or err when it is
// not marked. Only err itself is checked, so a marked error wrapped or
// joined with others keeps them.
func StripNoFallback(err error) error {
	if nf, ok := err.(*NoFallbackError); ok {
		return nf.Err
	}
	return err
}

// Try calls fn with the proxied URL, falling back to rawURL. Once the
// proxied attempt for a repository fails, later calls for that repository
// go directly to rawURL. A failure caused by ctx being done, or marked by
// NoFallback, is returned without falling back.
func (p *ProxyRewriter) Try(ctx context.Context, rawURL string, fn func(url string) error) error {
	proxied, ok := p.Rewrite(rawURL)
	if !ok {
		return StripNoFallback(fn(rawURL))
	}
	key := repoKey(rawURL)
	if _, skip := p.failed.Load(key); skip {
		return StripNoFallback(fn(rawURL))
	}
	perr := fn(proxied)
	if perr == nil {
		return nil
	}
	if nf, ok := perr.(*NoFallbackError); ok {
		return nf.Err
	}
	if ctx != nil && ctx.Err() != nil {
		return perr
	}
	p.failed.Store(key, struct{}{})
	derr := fn(rawURL)
	if derr == nil {
		return nil
	}
	if nf, ok := derr.(*NoFallbackError); ok {
		// lip throws it out of the direct attempt, dropping the proxy's error.
		return nf.Err
	}
	return fmt.Errorf("git request for %s failed both via the GitHub proxy and directly: %w",
		rawURL, errors.Join(perr, derr))
}
