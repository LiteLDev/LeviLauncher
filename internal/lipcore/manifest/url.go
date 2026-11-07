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
//   - in the userinfo, path, query and fragment, spaces, non-ASCII bytes,
//     control characters, the backtick and "<>\^{|} are percent-encoded
//     and a '%' not followed by two hex digits becomes "%25";
//   - when the URL triggers .NET's IRI processing (see iriTriggered), the
//     escapes in those components are normalized as
//     IriHelper.EscapeUnescapeIri does (see escapeURLPart): escapes of
//     unreserved characters (ALPHA DIGIT - . _ ~) are decoded; escapes of
//     the other ASCII characters that are neither reserved nor unsafe
//     (%3c %3e %5e %7b %7c %7d, and %20 %22 %60) and runs of escapes
//     forming a valid UTF-8 multi-byte sequence whose code point is in the
//     IRI range (see iriInRange) are uppercased; every other escape
//     (reserved or unsafe ASCII such as %2f or %5c, stray or invalid UTF-8
//     bytes such as %e9, code points outside the IRI range such as
//     %c2%80) keeps the case it was written in. Without the trigger every
//     escape is kept as written.
//
// Other inputs (opaque "scheme:..." URIs, file URLs, IPv6 or IDN hosts)
// are only trimmed/escaped; Flurl may render some of them differently, but
// they do not occur in tooth manifests. Neither does a userinfo whose
// length changes (a decoded or added escape): .NET then drops the
// character after the host (https://u%41@h.com/a renders as
// https://uA@h.coma), which is not reproduced here.
func NormalizeURL(s string) string {
	s = strings.TrimSpace(s)
	iri := iriTriggered(s)
	scheme, rest, ok := cutScheme(s)
	if ok && !strings.HasPrefix(rest, "//") {
		// Opaque URI ("scheme:..."): keep the scheme, escape the rest.
		path, query, hasQuery, frag, hasFrag := splitURLTail(rest)
		var b strings.Builder
		b.WriteString(scheme)
		b.WriteByte(':')
		b.WriteString(escapeURLPart(path, false, iri))
		writeQueryFragment(&b, query, hasQuery, frag, hasFrag, iri)
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
		path = removeDotSegments(escapeURLPart(strings.ReplaceAll(path, "\\", "/"), false, iri))
		var b strings.Builder
		b.WriteString(strings.ToLower(scheme))
		b.WriteString("://")
		b.WriteString(normalizeAuthority(authority, iri))
		b.WriteString(path)
		writeQueryFragment(&b, query, hasQuery, frag, hasFrag, iri)
		return b.String()
	}
	path, query, hasQuery, frag, hasFrag := splitURLTail(s)
	var b strings.Builder
	b.WriteString(escapeURLPart(path, false, iri))
	writeQueryFragment(&b, query, hasQuery, frag, hasFrag, iri)
	return b.String()
}

func writeQueryFragment(b *strings.Builder, query string, hasQuery bool, frag string, hasFrag, iri bool) {
	if hasQuery {
		b.WriteByte('?')
		b.WriteString(escapeURLPart(query, true, iri))
	}
	if hasFrag {
		b.WriteByte('#')
		b.WriteString(escapeIRIPart(frag, false, iri, iri))
	}
}

// iriTriggered reports whether System.Uri runs IRI processing
// (IriHelper.EscapeUnescapeIri) on the components of s. Port of
// Uri.CheckForUnicodeOrEscapedUnreserved in dotnet/runtime
// src/libraries/System.Private.Uri/src/System/UriExt.cs. It scans the
// whole URL, so an escape in one component changes how the others render.
// The trigger is any non-ASCII character, or any '%' followed by at least
// two more characters whose decoded value (UriHelper.DecodeHexChars) is
// non-ASCII, an unreserved character, or invalid hex (c_DummyChar, so
// "%zz" counts). A '%' near the end of s is skipped, and the two
// characters after an escape of any other ASCII value are not rescanned.
func iriTriggered(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			return true
		}
		if c != '%' || i+2 >= len(s) {
			continue
		}
		hi, ok1 := unhex(s[i+1])
		lo, ok2 := unhex(s[i+2])
		if !ok1 || !ok2 {
			return true
		}
		if v := hi<<4 | lo; v >= utf8.RuneSelf || isUnreserved(v) {
			return true
		}
		i += 2
	}
	return false
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

// normalizeAuthority lowercases the ASCII host and escapes the userinfo
// like the path: Uri.CheckAuthorityHelper (Uri.cs) runs
// IriHelper.EscapeUnescapeIri on it with UriComponents.UserInfo, which
// uses the non-query IRI range and the same reserved set.
func normalizeAuthority(a string, iri bool) string {
	userinfo := ""
	if i := strings.LastIndexByte(a, '@'); i >= 0 {
		userinfo, a = escapeURLPart(a[:i], false, iri)+"@", a[i+1:]
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

// iriKeepsEscaped reports whether IriHelper.EscapeUnescapeIri keeps the
// escape of the ASCII byte v as written. That is the case for
// UriHelper.IsNotSafeForUnescape: controls, DEL, '%', '\' and the RFC 3986
// reserved set UriHelper.RFC3986ReservedMarks. IriHelper.CheckIsReserved
// uses the same set for every component of an absolute URI.
func iriKeepsEscaped(v byte) bool {
	return v <= 0x1F || v == 0x7F || strings.IndexByte(";/?:@&=+$,#[]!'()*%\\", v) >= 0
}

// escapeURLPart escapes one URL component; isQuery is true only for the
// query, which admits a wider IRI range than the path and fragment, and
// iri is iriTriggered for the whole URL. With iri, escapes are normalized
// as IriHelper.EscapeUnescapeIri (IriHelper.cs) does:
//   - an ASCII escape that iriKeepsEscaped rejects is decoded, and System.Uri
//     then re-escapes it in uppercase unless it is unreserved;
//   - an escaped UTF-8 sequence in the IRI range is decoded and
//     re-escaped in uppercase;
//   - every other escape is copied as written.
func escapeURLPart(s string, isQuery, iri bool) string {
	return escapeIRIPart(s, isQuery, iri, false)
}

// escapeIRIPart is escapeURLPart; with trimSpaces it also drops the trailing
// whitespace the IRI pass produced, as System.Uri does for the fragment
// (GetLengthWithoutTrailingSpaces right after EscapeUnescapeIri, Uri.cs). An
// escaped space copied verbatim (no IRI, or skipped after invalid hex) stays.
func escapeIRIPart(s string, isQuery, iri, trimSpaces bool) string {
	var b strings.Builder
	keep := 0 // output length up to the last character that is not trimmed
	// plain counts characters the IRI pass skips: EscapeUnescapeIri copies a
	// '%' with invalid hex and the two characters after it as they are
	// (IriHelper.cs, DecodeHexChars == c_DummyChar), so only the later
	// escaping applies to them.
	plain := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		useIRI := iri && plain == 0
		if plain > 0 {
			plain--
		}
		// A decoded or literal space (System.Uri.IsLWS) is trimmable.
		trimmable := false
		if c == '%' {
			if i+2 < len(s) {
				hi, ok1 := unhex(s[i+1])
				lo, ok2 := unhex(s[i+2])
				if ok1 && ok2 {
					v := hi<<4 | lo
					plain = 0
					switch {
					case !useIRI || v < utf8.RuneSelf && iriKeepsEscaped(v):
						b.WriteString(s[i : i+3])
						i += 2
					case isUnreserved(v):
						b.WriteByte(v)
						i += 2
					case v < utf8.RuneSelf:
						trimmable = v == ' '
						b.WriteByte('%')
						b.WriteByte(upperHex[v>>4])
						b.WriteByte(upperHex[v&0xF])
						i += 2
					default:
						if n := escapedUTF8Len(s[i:], isQuery); n > 0 {
							b.WriteString(strings.ToUpper(s[i : i+n]))
							i += n - 1
						} else {
							b.WriteString(s[i : i+3])
							i += 2
						}
					}
					goto next
				}
			}
			if useIRI && i+2 < len(s) {
				plain = 2
			}
			b.WriteString("%25")
			goto next
		}
		if c <= 0x20 || c >= 0x7F || strings.IndexByte("\"<>\\^`{|}", c) >= 0 {
			trimmable = useIRI && (c == ' ' || c == '\t' || c == '\r' || c == '\n')
			b.WriteByte('%')
			b.WriteByte(upperHex[c>>4])
			b.WriteByte(upperHex[c&0xF])
			goto next
		}
		b.WriteByte(c)
	next:
		if !trimmable {
			keep = b.Len()
		}
	}
	if trimSpaces {
		return b.String()[:keep]
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
