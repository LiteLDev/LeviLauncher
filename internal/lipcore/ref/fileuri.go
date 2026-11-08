package ref

import (
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"
)

// The file: URIs of local paths [RFC8089], as lip's Ref.PathToFileUri and
// Ref.FileUriToPath write and read them (lip a68921d, 93826f0, ddb96ca): every
// octet a path segment does not allow as it is is percent-encoded, so that a
// '%', '#' or '?' in a path is part of a name.

var onWindows = runtime.GOOS == "windows"

// pathToFileURI is Ref.PathToFileUri: the file: URI [RFC8089] of an absolute
// local path, with every octet a path segment does not allow as it is
// percent-encoded, or false when the path is not absolute. On Windows, it is
// "file:///" drive "/" ... for a path on a drive and "file://" host "/"
// share "/" ... for a UNC path.
func pathToFileURI(path string) (string, bool) {
	if strings.ContainsRune(path, 0) {
		return "", false
	}
	if !onWindows {
		if !strings.HasPrefix(path, "/") {
			return "", false
		}
		segments := strings.Split(path, "/")
		for i, s := range segments {
			segments[i] = escapeSegment(s, false)
		}
		return "file://" + strings.Join(segments, "/"), true
	}

	segments := splitAny(path, `\/`)
	if len(path) >= 2 && isDirSepAny(path[0]) && isDirSepAny(path[1]) {
		if len(segments) < 4 || segments[2] == "" || segments[3] == "" {
			return "", false
		}
		escaped := make([]string, 0, len(segments)-3)
		for _, s := range segments[3:] {
			escaped = append(escaped, escapeSegment(s, false))
		}
		return "file://" + escapeSegment(segments[2], true) + "/" + strings.Join(escaped, "/"), true
	}
	if len(segments) < 2 || !isDrive(segments[0]) {
		return "", false
	}
	escaped := make([]string, 0, len(segments)-1)
	for _, s := range segments[1:] {
		escaped = append(escaped, escapeSegment(s, false))
	}
	return "file:///" + segments[0] + "/" + strings.Join(escaped, "/"), true
}

// fileURIToPath is Ref.FileUriToPath: the absolute local path a file: URI
// [RFC8089] names, the reverse of pathToFileURI, or false when it names none:
// when it has a query or fragment, is no valid URI, or names a remote host
// other than on Windows, or no drive on Windows.
func fileURIToPath(uri string) (string, bool) {
	if !hasFileScheme(uri) {
		return "", false
	}
	path := uri[len("file:"):]
	host := ""
	if strings.HasPrefix(path, "//") {
		slash := strings.IndexByte(path[2:], '/')
		if slash < 0 {
			return "", false
		}
		host = path[2 : 2+slash]
		path = path[2+slash:]
	}

	// The host is a reg-name: no userinfo ('@') and no port (':').
	if !strings.HasPrefix(path, "/") || !uriTextRegex.MatchString(host+path) ||
		strings.ContainsAny(host+path, "?#[]") || strings.ContainsAny(host, ":@") {
		return "", false
	}

	separator := "/"
	if onWindows {
		separator = `\`
	}
	var names []string
	for _, segment := range strings.Split(path[1:], "/") {
		name, ok := unescapeComponent(segment)
		if !ok || strings.ContainsAny(name, "/"+separator+"\x00") {
			return "", false
		}
		names = append(names, name)
	}

	hostName, ok := unescapeComponent(host)
	if !ok || strings.ContainsAny(hostName, "/\\\x00") {
		return "", false
	}

	local := hostName == "" || asciiEqualFold(hostName, "localhost")
	if !onWindows {
		if !local {
			return "", false
		}
		return "/" + strings.Join(names, "/"), true
	}

	// A UNC path on the host localhost, such as \\localhost\C$\x, is not on a
	// drive.
	if !local || (hostName != "" && !isDrive(names[0])) {
		if names[0] == "" {
			return "", false
		}
		return `\\` + hostName + `\` + strings.Join(names, `\`), true
	}
	if !isDrive(names[0]) {
		return "", false
	}
	if len(names) == 1 {
		return names[0] + `\`, true
	}
	return strings.Join(names, `\`), true
}

// splitAny is string.Split with several separator characters.
func splitAny(s, separators string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(separators, s[i]) >= 0 {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// isDrive is Ref.IsDrive: whether a Windows path segment is a drive, such as
// "C:".
func isDrive(segment string) bool {
	return len(segment) == 2 && isASCIILetter(segment[0]) && segment[1] == ':'
}

// escapeSegment is Ref.Escape: percent-encodes the UTF-8 octets of a path
// segment but those RFC 3986 allows as they are (pchar). A host (reg-name)
// allows neither ':' nor '@', which would start a port or end userinfo.
func escapeSegment(segment string, host bool) string {
	var b strings.Builder
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		if c < 0x80 && (isASCIIAlnum(rune(c)) || strings.IndexByte("-._~!$&'()*+,;=", c) >= 0 ||
			(!host && (c == ':' || c == '@'))) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// unescapeComponent is Ref.Unescape: decodes the percent-encoded octets of an
// ASCII URI component with every '%' starting one, as UTF-8; false when they
// are no valid UTF-8.
func unescapeComponent(component string) (string, bool) {
	var bytes []byte
	for i := 0; i < len(component); i++ {
		if component[i] == '%' && i+2 < len(component) && isHex(component[i+1]) && isHex(component[i+2]) {
			bytes = append(bytes, unhex(component[i+1])<<4|unhex(component[i+2]))
			i += 2
		} else {
			bytes = append(bytes, component[i])
		}
	}
	if !utf8.Valid(bytes) {
		return "", false
	}
	return string(bytes), true
}
