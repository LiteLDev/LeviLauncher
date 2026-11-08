package ref

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file emulates the parts of .NET's System.Uri that Ref.cs relies on:
// Uri.TryCreate(..., UriKind.Absolute) for https URLs (validity only; the
// URL is kept as written), Uri.AbsoluteUri of file: URIs and of implicit file
// paths (new Uri(@"C:\x")), and Uri.LocalPath of file URIs. The rules were
// taken from .NET 10's behavior and checked differentially against it.

var uriTextRegex = regexp.MustCompile(`^(?:[A-Za-z0-9\-._~:/?#\[\]@!$&'()*+,;=]|%[0-9A-Fa-f]{2})*$`)

// isURIText ports Ref.IsUriText: s has only the characters RFC 3986 allows in
// a URI, with every '%' starting a percent-encoded octet, and '[' and ']'
// only before the path.
func isURIText(s string) bool {
	authority := strings.Index(s, "://") + 3
	pathStart := -1
	if authority <= len(s) {
		if i := strings.IndexAny(s[authority:], "/?#"); i >= 0 {
			pathStart = authority + i
		}
	}
	return uriTextRegex.MatchString(s) &&
		(pathStart < 0 || !strings.ContainsAny(s[pathStart:], "[]"))
}

// validHTTPSURL emulates Uri.TryCreate(target, UriKind.Absolute) and the
// scheme check of Ref.TryParse, for a target that starts with "https://" and
// passed isURIText. Such a URI always has a host: TryCreate fails on an
// empty one.
func validHTTPSURL(target string) bool {
	const httpsInvalid = false
	const httpsOK = true
	rest := target[len("https://"):]
	authority := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		authority = rest[:i]
	}
	if at := strings.IndexByte(authority, '@'); at >= 0 {
		authority = authority[at+1:]
	}
	host, port := authority, ""
	hasPort := false
	if strings.HasPrefix(authority, "[") {
		end := strings.IndexByte(authority, ']')
		if end < 0 {
			return httpsInvalid
		}
		host = authority[:end+1]
		// After the literal, a ':' starts the port; anything else starts the
		// path (.NET reads "https://[::1]x/" as host [::1] and path "x/").
		if after := authority[end+1:]; after != "" && after[0] == ':' {
			port, hasPort = after[1:], true
		}
		if _, ok := parseIPv6Literal(host); !ok {
			return httpsInvalid
		}
	} else {
		if i := strings.IndexByte(authority, ':'); i >= 0 {
			host, port, hasPort = authority[:i], authority[i+1:], true
		}
		if host == "" {
			return httpsInvalid
		}
		if _, ok := parseIPv4Host(host); !ok && !isDNSHost(host) && !isUNCName(host) {
			return httpsInvalid
		}
	}
	if hasPort && !validPort(port) {
		return httpsInvalid
	}
	return httpsOK
}

func validPort(port string) bool {
	if port == "" {
		return true
	}
	n := 0
	for i := 0; i < len(port); i++ {
		c := port[i]
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
		if n > 65535 {
			return false
		}
	}
	return true
}

func isASCIIAlnum(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

// isDNSHost is DomainNameHelper.IsValid (with IRI parsing): labels of ASCII
// letters, digits, '-', '_' and non-ASCII characters, each starting with an
// ASCII letter or digit or a non-ASCII character and at most 63 UTF-16 chars
// long; only the last label may be empty, after a trailing dot.
func isDNSHost(host string) bool {
	if host == "" {
		return false
	}
	labels := strings.Split(host, ".")
	for i, l := range labels {
		if l == "" {
			if i == len(labels)-1 && i > 0 {
				continue
			}
			return false
		}
		if first, _ := utf8.DecodeRuneInString(l); !isASCIIAlnum(first) && first < 0xA0 {
			return false
		}
		units := 0
		for _, r := range l {
			switch {
			case isASCIIAlnum(r), r == '-', r == '_', r >= 0xA0 && r != utf8.RuneError:
			default:
				return false
			}
			units++
			if r > 0xFFFF {
				units++
			}
		}
		if units > 63 {
			return false
		}
	}
	return true
}

// isUNCName is UncNameHelper.IsValid, the names .NET accepts that are no
// DNS names: the first segment has letters, digits, '-' and '_', and not
// only digits; each later segment starts with a letter or digit (the second
// may also start with '-' or '_'); only the last segment may be empty. A
// letter is a BMP char.IsLetter.
func isUNCName(host string) bool {
	if host == "" {
		return false
	}
	isLetter := func(r rune) bool { return r <= 0xFFFF && unicode.IsLetter(r) }
	valid := false
	prevDot := false
	i := 0
	for i < len(host) {
		r, size := utf8.DecodeRuneInString(host[i:])
		if r == '.' {
			i++
			prevDot = true
			break
		}
		switch {
		case isLetter(r), r == '-', r == '_':
			valid = true
		case r < '0' || r > '9':
			return false
		}
		i += size
	}
	if !valid {
		return false
	}
	for i < len(host) {
		r, size := utf8.DecodeRuneInString(host[i:])
		switch {
		case r == '.':
			if !valid || prevDot {
				return false
			}
			valid = false
		case r == '-', r == '_':
			if !valid {
				return false
			}
		case isLetter(r), r >= '0' && r <= '9':
			valid = true
		default:
			return false
		}
		prevDot = r == '.'
		i += size
	}
	if strings.HasSuffix(host, ".") {
		valid = true
	}
	return valid
}

// parseIPv6Literal validates "[...]" and returns its canonical form as .NET
// prints it (without a scope id).
func parseIPv6Literal(lit string) (string, bool) {
	if len(lit) < 2 || lit[0] != '[' || lit[len(lit)-1] != ']' {
		return "", false
	}
	inner := lit[1 : len(lit)-1]
	if !strings.Contains(inner, ":") {
		return "", false
	}
	addr, err := netip.ParseAddr(inner)
	if err != nil || !addr.Is6() {
		return "", false
	}
	return "[" + addr.WithZone("").String() + "]", true
}

// parseIPv4Host canonicalizes a host that .NET reads as an IPv4 address
// (the inet_aton forms: one to four parts, each decimal, octal with a
// leading 0 or hexadecimal with 0x, the last filling the remaining bytes).
func parseIPv4Host(host string) (string, bool) {
	parts := strings.Split(host, ".")
	if len(parts) > 4 {
		return "", false
	}
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		var v uint64
		var err error
		switch {
		case p == "":
			return "", false
		case len(p) > 1 && (p[:2] == "0x" || p[:2] == "0X"):
			if len(p) == 2 {
				return "", false
			}
			v, err = strconv.ParseUint(p[2:], 16, 64)
		case len(p) > 1 && p[0] == '0':
			v, err = strconv.ParseUint(p[1:], 8, 64)
		default:
			v, err = strconv.ParseUint(p, 10, 64)
		}
		if err != nil || strings.ContainsAny(p, "+-") {
			return "", false
		}
		vals[i] = v
	}
	var n uint64
	for i, v := range vals[:len(vals)-1] {
		if v > 255 {
			return "", false
		}
		n |= v << (8 * (3 - i))
	}
	last := vals[len(vals)-1]
	if last >= 1<<(8*(5-len(vals))) {
		return "", false
	}
	n |= last
	return strconv.FormatUint(n>>24, 10) + "." + strconv.FormatUint(n>>16&255, 10) + "." +
		strconv.FormatUint(n>>8&255, 10) + "." + strconv.FormatUint(n&255, 10), true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

func isASCIILetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

func isDirSepAny(c byte) bool { return c == '\\' || c == '/' }
