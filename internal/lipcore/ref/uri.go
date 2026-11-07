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

// canonUNCHost validates and canonicalizes the host of a UNC file URI or
// path: an IPv6 literal, an IPv4 address, or a (lowercased) name.
func canonUNCHost(host string) (string, bool) {
	if strings.HasPrefix(host, "[") {
		return parseIPv6Literal(host)
	}
	if v4, ok := parseIPv4Host(host); ok {
		return v4, true
	}
	if !isDNSHost(host) && !isUNCName(host) {
		return "", false
	}
	// .NET lowercases the whole name (non-ASCII letters too) only when it has
	// an ASCII uppercase letter.
	if strings.IndexFunc(host, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0 {
		return strings.ToLower(host), true
	}
	return host, true
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

const upperHex = "0123456789ABCDEF"

func isUnreserved(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' ||
		b == '-' || b == '.' || b == '_' || b == '~'
}

// validEscape reports whether s[i] is '%' followed by two hex digits.
func validEscape(s string, i int) bool {
	return i+2 < len(s) && s[i] == '%' && isHex(s[i+1]) && isHex(s[i+2])
}

// utf8Escapes returns the length in bytes of s of the run of escapes at i
// that encodes one valid multi-byte UTF-8 character, or 0.
func utf8Escapes(s string, i int) int {
	lead := unhex(s[i+1])<<4 | unhex(s[i+2])
	n := 0
	switch {
	case lead&0xE0 == 0xC0:
		n = 2
	case lead&0xF0 == 0xE0:
		n = 3
	case lead&0xF8 == 0xF0:
		n = 4
	default:
		return 0
	}
	buf := make([]byte, 0, 4)
	for k := 0; k < n; k++ {
		j := i + 3*k
		if !validEscape(s, j) {
			return 0
		}
		buf = append(buf, unhex(s[j+1])<<4|unhex(s[j+2]))
	}
	if !utf8.Valid(buf) {
		return 0
	}
	return 3 * n
}

// uriPart says where a string being canonicalized sits.
type uriPart int

const (
	partPath         uriPart = iota // the path of a file: URI
	partQuery                       // the query of a file: URI
	partImplicitPath                // an implicit file path (new Uri(@"C:\x"))
)

// escapeByte appends c escaped as .NET escapes it in a file URI.
func escapeByte(b *strings.Builder, s string, i int, part uriPart) {
	c := s[i]
	switch {
	case c == '%':
		if part != partImplicitPath && validEscape(s, i) {
			b.WriteByte('%')
		} else {
			b.WriteString("%25")
		}
		return
	case c < 0x20, c >= 0x7f,
		c == ' ', c == '"', c == '<', c == '>', c == '{', c == '}', c == '|', c == '^', c == '`',
		part == partImplicitPath && (c == '#' || c == '?'),
		part == partQuery && c == '\\':
		b.WriteByte('%')
		b.WriteByte(upperHex[c>>4])
		b.WriteByte(upperHex[c&15])
		return
	}
	b.WriteByte(c)
}

// isUnsafe reports whether .NET escapes the printable ASCII char c wherever
// it appears in a URI.
func isUnsafe(c byte) bool {
	switch c {
	case ' ', '"', '<', '>', '{', '}', '|', '^', '`':
		return true
	}
	return false
}

// needsUnescape reports whether .NET unescapes s (a file URI after "file:",
// or an implicit file path): s has a non-ASCII character, an escape of an
// unreserved or non-ASCII byte, or a '%' followed by two characters that are
// not both hex digits. Otherwise the escapes of a file URI are kept as
// written, and every '%' of an implicit path is data.
func needsUnescape(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x80 {
			return true
		}
		if c != '%' || i+2 >= len(s) {
			continue
		}
		if !isHex(s[i+1]) || !isHex(s[i+2]) {
			return true
		}
		if v := unhex(s[i+1])<<4 | unhex(s[i+2]); isUnreserved(v) || v >= 0x80 {
			return true
		}
		i += 2
	}
	return false
}

// replacementEscape is U+FFFD, which a lone UTF-16 surrogate becomes.
const replacementEscape = "%EF%BF%BD"

// canon unescapes and escapes s as .NET does for a file URI (unescape is
// needsUnescape of the whole URI or path):
//   - in a file: path, %2F and %5C are '/';
//   - without unescape, the other escapes of a file URI are kept as written,
//     and every '%' of an implicit path is data, escaped as "%25";
//   - with unescape, an escaped unreserved character is decoded, an escaped
//     valid multi-byte UTF-8 character and an escaped unsafe character are
//     kept with uppercase hex, and other valid escapes are kept as written in
//     a file URI and have their '%' escaped in an implicit path;
//   - characters a URI does not allow are escaped, and so is a '%' that
//     starts no valid escape. The two UTF-16 chars after such a '%' are only
//     escaped, never decoded; a surrogate pair split there becomes two U+FFFD
//     (the second escaped again in an implicit path).
//
// unescaped reports whether an escape was decoded (even when escaped again),
// after which .NET keeps the whitespace at the end of the string.
func canon(s string, part uriPart, unescape bool) (out string, unescaped bool) {
	implicit := part == partImplicitPath
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '%' || implicit && !unescape {
			escapeByte(&b, s, i, part)
			i++
			continue
		}
		if !validEscape(s, i) {
			b.WriteString("%25")
			i++
			for units := 0; units < 2 && i < len(s); {
				if s[i] >= 0xF0 {
					// A character outside the BMP: two UTF-16 chars.
					_, size := utf8.DecodeRuneInString(s[i:])
					if size == 4 && units == 1 {
						b.WriteString(replacementEscape)
						if implicit {
							b.WriteString("%25EF%25BF%25BD")
						} else {
							b.WriteString(replacementEscape)
						}
						unescaped = true
						i += size
						units = 2
						continue
					}
					for k := 0; k < size; k++ {
						escapeByte(&b, s, i+k, part)
					}
					i += size
					units += 2
					continue
				}
				_, size := utf8.DecodeRuneInString(s[i:])
				for k := 0; k < size; k++ {
					escapeByte(&b, s, i+k, part)
				}
				i += size
				units++
			}
			continue
		}
		v := unhex(s[i+1])<<4 | unhex(s[i+2])
		if part == partPath && (v == '/' || v == '\\') {
			b.WriteByte('/')
			i += 3
			continue
		}
		if !unescape {
			b.WriteString(s[i : i+3])
			i += 3
			continue
		}
		switch {
		case isUnreserved(v):
			b.WriteByte(v)
			unescaped = true
			i += 3
			continue
		case v >= 0x80:
			if n := utf8Escapes(s, i); n > 0 {
				for j := i; j < i+n; j += 3 {
					b.WriteByte('%')
					b.WriteByte(upperHex[unhex(s[j+1])])
					b.WriteByte(upperHex[unhex(s[j+2])])
				}
				unescaped = true
				i += n
				continue
			}
		case isUnsafe(v):
			b.WriteByte('%')
			b.WriteByte(upperHex[v>>4])
			b.WriteByte(upperHex[v&15])
			unescaped = true
			i += 3
			continue
		}
		if implicit {
			b.WriteString("%25")
			b.WriteString(s[i+1 : i+3])
		} else {
			b.WriteString(s[i : i+3])
		}
		i += 3
	}
	return b.String(), unescaped
}

// removeDotSegments is RFC 3986 §5.2.4 on a path starting with '/'.
func removeDotSegments(in string) string {
	var out []string // output segments, each starting with '/'
	for in != "" {
		switch {
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
		default:
			j := strings.IndexByte(in[1:], '/')
			if j < 0 {
				out = append(out, in)
				in = ""
			} else {
				out = append(out, in[:j+1])
				in = in[j+1:]
			}
		}
	}
	return strings.Join(out, "")
}

// driveAt reports whether s starts with a DOS drive "X:/" (or "X|/").
func driveAt(s string) bool {
	return len(s) >= 3 && isASCIILetter(s[0]) && (s[1] == ':' || s[1] == '|') && s[2] == '/'
}

func isASCIILetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

// fileURI is a parsed file URI (or implicit file path).
type fileURI struct {
	// slashes > 0 is a URI of only slashes ("file:////"), kept as written.
	slashes int
	host    string
	// path is escaped and canonical; "" for a UNC URI without a path.
	path  string
	query *string
}

// absoluteURI is Uri.AbsoluteUri.
func (u fileURI) absoluteURI() string {
	var s string
	if u.slashes > 0 {
		s = "file:" + strings.Repeat("/", u.slashes)
	} else {
		path := u.path
		if path == "" {
			path = "/"
		}
		s = "file://" + u.host + path
	}
	if u.query != nil {
		s += "?" + *u.query
	}
	return s
}

// localPath is Uri.LocalPath.
func (u fileURI) localPath() string {
	switch {
	case u.slashes > 0:
		return strings.Repeat("/", u.slashes-2)
	case u.host != "":
		return `\\` + u.host + strings.ReplaceAll(unescapeLocal(u.path), "/", `\`)
	case !unixPaths && len(u.path) >= 4 && isASCIILetter(u.path[1]) && u.path[2] == ':' && u.path[3] == '/':
		return strings.ReplaceAll(unescapeLocal(u.path[1:]), "/", `\`)
	}
	return unescapeLocal(u.path)
}

// normalizePath canonicalizes the path of a file URI and removes its dot
// segments; for a DOS path "/X:/...", the drive is never removed.
func normalizePath(path string, part uriPart, unescape bool) (string, bool) {
	path, unescaped := canon(path, part, unescape)
	if len(path) >= 3 && path[0] == '/' && isASCIILetter(path[1]) && path[2] == ':' {
		return path[:3] + removeDotSegments(path[3:]), unescaped
	}
	return removeDotSegments(path), unescaped
}

// isLWS is UriHelper.IsLWS: the whitespace Uri trims from both ends.
func isLWS(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func trimLeadingLWS(s string) string {
	for len(s) > 0 && isLWS(s[0]) {
		s = s[1:]
	}
	return s
}

func trimTrailingLWS(s string) string {
	for len(s) > 0 && isLWS(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

// withLWS parses s as .NET does with the whitespace at its ends: it trims
// that whitespace, but keeps (and escapes) the whitespace at the end when an
// escape was decoded. Whether to unescape is decided before trimming.
func withLWS(s string, parse func(string, bool) (fileURI, bool, error)) (fileURI, error) {
	s = trimLeadingLWS(s)
	unescape := needsUnescape(s)
	t := trimTrailingLWS(s)
	u, unescaped, err := parse(t, unescape)
	if err == nil && unescaped && t != s {
		u, _, err = parse(s, unescape)
	}
	return u, err
}

// parseFileURI emulates Uri.TryCreate(target, UriKind.Absolute) followed by
// Uri.IsFile for a target starting with "file:" (any ASCII case).
func parseFileURI(target string) (fileURI, error) { return withLWS(target, parseFileURIExact) }

// parseFileURIExact parses a file URI without whitespace at its ends.
// keepLWS reports whether .NET would keep whitespace at the end: when an
// escape was decoded, or, for a non-UNC URI that unescape applies to, when
// it does not start with "file://" (lowercase, two '/') and two or three
// separators.
func parseFileURIExact(target string, unescape bool) (u fileURI, keepLWS bool, err error) {
	if len(target) < len("file:") || !asciiEqualFold(target[:len("file:")], "file:") {
		// A scheme with a non-ASCII char ("fıle:") is no URI scheme.
		return fileURI{}, false, errFormat
	}
	rest := target[len("file:"):]
	unescaped := false
	if q := strings.IndexByte(rest, '?'); q >= 0 {
		qs, qu := canon(rest[q+1:], partQuery, unescape)
		u.query, unescaped = &qs, qu
		rest = rest[:q]
	}
	canonicalPrefix := strings.HasPrefix(target, "file://")
	rest = strings.ReplaceAll(rest, `\`, "/")
	n := 0
	for n < len(rest) && rest[n] == '/' {
		n++
	}
	canonicalPrefix = canonicalPrefix && (n == 2 || n == 3)
	after := rest[n:]
	path := func(p string) (fileURI, bool, error) {
		var pu bool
		u.path, pu = normalizePath(p, partPath, unescape)
		return u, unescaped || pu || unescape && !canonicalPrefix, nil
	}

	if driveAt(after) {
		return path("/" + string(after[0]) + ":" + after[2:])
	}
	if len(after) >= 2 && isASCIILetter(after[0]) && (after[1] == ':' || after[1] == '|') {
		// A drive without a '/' after it.
		return fileURI{}, false, errDosPath
	}

	switch {
	case n < 2:
		return fileURI{}, false, errFormat
	case n == 3:
		return path("/" + after)
	case n > 3 && after == "":
		u.slashes = n
		return u, unescaped, nil
	}

	// UNC: file://host/path (or more slashes before the host).
	authority, p := after, ""
	if i := strings.IndexByte(after, '/'); i >= 0 {
		authority, p = after[:i], after[i:]
	}
	if authority == "" {
		// "file://" or "file://?q".
		u.path = "/"
		return u, unescaped, nil
	}
	host, ok := canonUNCHost(authority)
	if !ok {
		return fileURI{}, false, errHostname
	}
	u.host = host
	if p == "" {
		return u, unescaped, nil
	}
	var pu bool
	u.path, pu = normalizePath(p, partPath, unescape)
	return u, unescaped || pu, nil
}

// fileURIAbsolute is Uri.TryCreate + Uri.IsFile + Uri.AbsoluteUri for a
// target starting with "file:".
func fileURIAbsolute(target string) (string, bool) {
	u, err := parseFileURI(target)
	if err != nil {
		return "", false
	}
	return u.absoluteURI(), true
}

// fileURILocalPath is new Uri(target).LocalPath, which parses the target
// again.
func fileURILocalPath(target string) (string, error) {
	u, err := parseFileURI(target)
	if err != nil {
		return "", err
	}
	return u.localPath(), nil
}

// uriFormatError is a UriFormatException.
type uriFormatError string

func (e uriFormatError) Error() string { return "Invalid URI: " + string(e) }

var (
	errHostname = uriFormatError("The hostname could not be parsed.")
	errFormat   = uriFormatError("The format of the URI could not be determined.")
	errDosPath  = uriFormatError(`A Dos path must be rooted, for example, 'c:\\'.`)
)

// implicitFileURI emulates new Uri(path, UriKind.Absolute).AbsoluteUri for an
// implicit file path: "X:\..." or "\\host\..." on Windows, "/..." elsewhere.
func implicitFileURI(path string) (string, error) {
	if p := trimLeadingLWS(path); len(p) == 2 && isDirSepAny(p[0]) && isDirSepAny(p[1]) {
		// Only the two separators of a UNC path.
		return "", errFormat
	}
	u, err := withLWS(path, parseImplicitExact)
	if err != nil {
		return "", err
	}
	return u.absoluteURI(), nil
}

func parseImplicitExact(path string, unescape bool) (u fileURI, unescaped bool, err error) {
	switch {
	case len(path) >= 2 && isASCIILetter(path[0]) && path[1] == ':':
		if len(path) < 3 || path[2] != '\\' && path[2] != '/' {
			return fileURI{}, false, errDosPath
		}
		u.path, unescaped = normalizePath("/"+strings.ReplaceAll(path, `\`, "/"), partImplicitPath, unescape)
	case len(path) >= 2 && isDirSepAny(path[0]) && isDirSepAny(path[1]):
		p := strings.TrimLeft(strings.ReplaceAll(path, `\`, "/"), "/")
		authority, rest := p, ""
		if i := strings.IndexByte(p, '/'); i >= 0 {
			authority, rest = p[:i], p[i:]
		}
		host, ok := canonUNCHost(authority)
		if !ok {
			return fileURI{}, false, errHostname
		}
		u.host = host
		if rest != "" {
			u.path, unescaped = normalizePath(rest, partImplicitPath, unescape)
		}
	case unixPaths && strings.HasPrefix(path, "/"):
		u.path, unescaped = normalizePath(path, partImplicitPath, unescape)
	default:
		return fileURI{}, false, errFormat
	}
	return u, unescaped, nil
}

func isDirSepAny(c byte) bool { return c == '\\' || c == '/' }

// unescapeLocal decodes the escapes of s for Uri.LocalPath: escapes of ASCII
// characters and of valid UTF-8 sequences are decoded; others are kept.
func unescapeLocal(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if !validEscape(s, i) {
			b.WriteByte(s[i])
			i++
			continue
		}
		v := unhex(s[i+1])<<4 | unhex(s[i+2])
		if v < 0x80 {
			b.WriteByte(v)
			i += 3
			continue
		}
		if n := utf8Escapes(s, i); n > 0 {
			for j := i; j < i+n; j += 3 {
				b.WriteByte(unhex(s[j+1])<<4 | unhex(s[j+2]))
			}
			i += n
			continue
		}
		b.WriteString(s[i : i+3])
		i += 3
	}
	return b.String()
}
