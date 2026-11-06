package manifest

import (
	"strings"
	"unicode/utf8"
)

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
//     by two hex digits becomes "%25", escapes of unreserved characters
//     (ALPHA DIGIT - . _ ~) are decoded, runs of escapes forming a valid
//     UTF-8 multi-byte sequence whose code point .NET's IRI parsing
//     decodes (see iriInRange) are uppercased and every other escape
//     (reserved ASCII such as %2f, stray or invalid UTF-8 bytes such as
//     %e9, code points outside the IRI range such as %c2%80) keeps the
//     case it was written in.
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
		b.WriteString(escapeURLPart(path, false))
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
		path = removeDotSegments(escapeURLPart(strings.ReplaceAll(path, "\\", "/"), false))
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
	b.WriteString(escapeURLPart(path, false))
	writeQueryFragment(&b, query, hasQuery, frag, hasFrag)
	return b.String()
}

func writeQueryFragment(b *strings.Builder, query string, hasQuery bool, frag string, hasFrag bool) {
	if hasQuery {
		b.WriteByte('?')
		b.WriteString(escapeURLPart(query, true))
	}
	if hasFrag {
		b.WriteByte('#')
		b.WriteString(escapeURLPart(frag, false))
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

// escapeURLPart escapes one URL component; isQuery is true only for the
// query, which admits a wider IRI range than the path and fragment.
func escapeURLPart(s string, isQuery bool) string {
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
						i += 2
					} else if n := escapedUTF8Len(s[i:], isQuery); n > 0 {
						b.WriteString(strings.ToUpper(s[i : i+n]))
						i += n - 1
					} else {
						b.WriteString(s[i : i+3])
						i += 2
					}
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

// escapedUTF8Len returns the length of the run of %XX escapes at the start
// of s when together they encode one valid non-ASCII UTF-8 character that
// passes iriInRange, and 0 otherwise.
func escapedUTF8Len(s string, isQuery bool) (n int) {
	var buf [utf8.UTFMax]byte
	for k := 0; k < utf8.UTFMax && n+2 < len(s) && s[n] == '%'; k++ {
		hi, ok1 := unhex(s[n+1])
		lo, ok2 := unhex(s[n+2])
		if !ok1 || !ok2 {
			break
		}
		buf[k] = hi<<4 | lo
		n += 3
		if k == 0 && buf[0] < utf8.RuneSelf {
			return 0
		}
		if r, size := utf8.DecodeRune(buf[:k+1]); size > 1 && size == k+1 {
			if iriInRange(r, isQuery) {
				return n
			}
			return 0
		}
	}
	return 0
}

// iriInRange reports whether System.Uri's IRI parsing decodes an escaped
// r (and so re-escapes it in uppercase); escapes of other code points are
// copied as written. Port of IriHelper.CheckIriUnicodeRange(uint, bool) in
// dotnet/runtime src/libraries/System.Private.Uri/src/System/IriHelper.cs,
// which Uri calls with isQuery true only for the query component:
//
//   - BMP: U+00A0-U+D7FF, U+F900-U+FDCF and U+FDF0-U+FFEF, plus the
//     private-use U+E000-U+F8FF in the query only (so C1 controls, the
//     noncharacters U+FDD0-U+FDEF and U+FFF0-U+FFFF are excluded);
//   - supplementary planes: everything except U+xFFFE/U+xFFFF, the tag and
//     variation-selector block U+E0000-U+E0FFF and, outside the query, the
//     private-use planes 15-16 (U+F0000 and up).
func iriInRange(r rune, isQuery bool) bool {
	if r <= 0xFFFF {
		return r >= 0x00A0 && r <= 0xD7FF ||
			r >= 0xF900 && r <= 0xFDCF ||
			r >= 0xFDF0 && r <= 0xFFEF ||
			isQuery && r >= 0xE000 && r <= 0xF8FF
	}
	return r&0xFFFF < 0xFFFE &&
		!(r >= 0xE0000 && r <= 0xE0FFF) &&
		(isQuery || r < 0xF0000)
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
