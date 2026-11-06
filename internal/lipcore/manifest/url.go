package manifest

import "strings"

// NormalizeURL returns the text lip writes for an asset URL: lip stores
// urls as Flurl 4.0 Url objects (Url.Parse on read) and writes
// Url.ToString(), which re-renders the URL through System.Uri. The rules
// reproduced here (verified against Flurl 4.0.0 on .NET 10):
//
//   - surrounding whitespace is trimmed;
//   - for "scheme://authority..." URLs the scheme and ASCII host are
//     lowercased, the port is kept as written, '\' in the path becomes '/',
//     and "." / ".." path segments are removed;
//   - in the path, query and fragment, spaces, non-ASCII bytes, control
//     characters, the backtick and "<>\^{|} are percent-encoded, a '%' not followed
//     by two hex digits becomes "%25", existing escapes are uppercased and
//     escapes of unreserved characters (ALPHA DIGIT - . _ ~) are decoded.
//
// Other inputs (opaque "scheme:..." URIs, file URLs, IPv6 or IDN hosts)
// are only trimmed/escaped; Flurl may render some of them differently, but
// they do not occur in tooth manifests.
func NormalizeURL(s string) string {
	s = strings.TrimSpace(s)
	scheme, rest, ok := cutScheme(s)
	if ok && !strings.HasPrefix(rest, "//") {
		// Opaque URI ("scheme:..."): keep the scheme, escape the rest.
		path, query, hasQuery, frag, hasFrag := splitURLTail(rest)
		var b strings.Builder
		b.WriteString(scheme)
		b.WriteByte(':')
		b.WriteString(escapeURLPart(path))
		writeQueryFragment(&b, query, hasQuery, frag, hasFrag)
		return b.String()
	}
	if ok {
		rest = rest[2:]
		end := strings.IndexAny(rest, "/\\?#")
		if end < 0 {
			end = len(rest)
		}
		authority, tail := rest[:end], rest[end:]
		path, query, hasQuery, frag, hasFrag := splitURLTail(tail)
		path = removeDotSegments(escapeURLPart(strings.ReplaceAll(path, "\\", "/")))
		var b strings.Builder
		b.WriteString(strings.ToLower(scheme))
		b.WriteString("://")
		b.WriteString(normalizeAuthority(authority))
		b.WriteString(path)
		writeQueryFragment(&b, query, hasQuery, frag, hasFrag)
		return b.String()
	}
	path, query, hasQuery, frag, hasFrag := splitURLTail(s)
	var b strings.Builder
	b.WriteString(escapeURLPart(path))
	writeQueryFragment(&b, query, hasQuery, frag, hasFrag)
	return b.String()
}

func writeQueryFragment(b *strings.Builder, query string, hasQuery bool, frag string, hasFrag bool) {
	if hasQuery {
		b.WriteByte('?')
		b.WriteString(escapeURLPart(query))
	}
	if hasFrag {
		b.WriteByte('#')
		b.WriteString(escapeURLPart(frag))
	}
}

// cutScheme splits "scheme:rest" (RFC 3986 scheme, at least 2 characters
// so that Windows drive letters are not taken as schemes).
func cutScheme(s string) (scheme, rest string, ok bool) {
	i := strings.IndexByte(s, ':')
	if i < 2 {
		return "", s, false
	}
	for j := 0; j < i; j++ {
		c := s[j]
		switch {
		case isASCIILetter(c):
		case j > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return "", s, false
		}
	}
	return s[:i], s[i+1:], true
}

func splitURLTail(s string) (path, query string, hasQuery bool, frag string, hasFrag bool) {
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s, frag, hasFrag = s[:i], s[i+1:], true
	}
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s, query, hasQuery = s[:i], s[i+1:], true
	}
	return s, query, hasQuery, frag, hasFrag
}

func normalizeAuthority(a string) string {
	userinfo := ""
	if i := strings.LastIndexByte(a, '@'); i >= 0 {
		userinfo, a = a[:i+1], a[i+1:]
	}
	b := []byte(a)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return userinfo + string(b)
}

const upperHex = "0123456789ABCDEF"

func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func isUnreserved(c byte) bool {
	return isASCIILetter(c) || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~'
}

func escapeURLPart(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' {
			if i+2 < len(s) {
				hi, ok1 := unhex(s[i+1])
				lo, ok2 := unhex(s[i+2])
				if ok1 && ok2 {
					v := hi<<4 | lo
					if isUnreserved(v) {
						b.WriteByte(v)
					} else {
						b.WriteByte('%')
						b.WriteByte(upperHex[hi])
						b.WriteByte(upperHex[lo])
					}
					i += 2
					continue
				}
			}
			b.WriteString("%25")
			continue
		}
		if c <= 0x20 || c >= 0x7F || strings.IndexByte("\"<>\\^`{|}", c) >= 0 {
			b.WriteByte('%')
			b.WriteByte(upperHex[c>>4])
			b.WriteByte(upperHex[c&0xF])
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// removeDotSegments implements RFC 3986 section 5.2.4.
func removeDotSegments(in string) string {
	if !strings.Contains(in, ".") {
		return in
	}
	var out []string
	for in != "" {
		switch {
		case strings.HasPrefix(in, "../"):
			in = in[3:]
		case strings.HasPrefix(in, "./"):
			in = in[2:]
		case strings.HasPrefix(in, "/./"):
			in = in[2:]
		case in == "/.":
			in = "/"
		case strings.HasPrefix(in, "/../"):
			in = in[3:]
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		case in == "/..":
			in = "/"
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		case in == "." || in == "..":
			in = ""
		default:
			start := 0
			if in[0] == '/' {
				start = 1
			}
			end := strings.IndexByte(in[start:], '/')
			if end < 0 {
				end = len(in)
			} else {
				end += start
			}
			out = append(out, in[:end])
			in = in[end:]
		}
	}
	return strings.Join(out, "")
}
