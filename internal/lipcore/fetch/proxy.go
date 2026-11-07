package fetch

import (
	"net/url"
	"strings"
)

// RewriteGithub is lip 0.38's GithubProxy.Rewrite (src/Lip.Core/Git/
// GithubProxy.cs): when proxy is set and rawURL's host is github.com
// (compared case-insensitively) it returns proxy with rawURL's path
// segments appended and rawURL's query (when non-empty) in place of the
// proxy's, built like Flurl's Clone().AppendPathSegments(url.PathSegments).
// The original host is dropped and the URL's fragment is not carried over.
// It returns rawURL and false when no proxy applies, including when the
// result equals rawURL. Download candidates are then [proxied, rawURL].
func RewriteGithub(proxy, rawURL string) (string, bool) {
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
