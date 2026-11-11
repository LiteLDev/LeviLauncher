package engine

import (
	"errors"
	"io/fs"
	"net"
	"net/url"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/graph"
	"github.com/liteldev/LeviLauncher/internal/lipcore/install"
)

// ErrAlreadyExplicitlyInstalled matches (errors.Is) the failure of an install
// that requests a package already explicitly installed ("... is already
// explicitly installed ... run 'lip update ...' instead").
var ErrAlreadyExplicitlyInstalled = errors.New("engine: package already explicitly installed")

// ErrNoPackageFound matches (errors.Is) the failure of an operation because
// a package argument resolves to no package that can be installed in the
// workspace ("No package found for '...' that can be installed in this
// workspace ...").
var ErrNoPackageFound = errors.New("engine: no package found for a package argument")

// Error is the InvalidOperationException (or ArgumentException) McmodService
// throws: an operation that cannot be carried out. Error() is lip's message;
// the cause, if any, is reachable through errors.Unwrap.
type Error struct {
	Msg string
	Err error
	// kind is the sentinel the error matches with errors.Is, if any.
	kind error
	// compatKind: compat_legacy_variants.go.
	compatKind error
}

func (e *Error) Error() string { return e.Msg }

func (e *Error) Unwrap() error { return e.Err }

// Is: e.compatKind is compat_legacy_variants.go.
func (e *Error) Is(target error) bool {
	return e.kind != nil && target == e.kind || e.compatKind != nil && target == e.compatKind
}

// FormatError tells why an operation failed in plain words, as lip's
// ErrorReporter (src/Lip.Cli/ErrorReporter.cs) prints it: the message of each
// failure followed by the messages of its causes that add something
// ("caused by: ..."), with credentials in URLs masked, then a line
// "hint: ..." for every hint that applies.
func FormatError(err error) string {
	if err == nil {
		return ""
	}
	failures := unwrapFailures(err)
	var b strings.Builder
	b.WriteString(redact(summarize(failures)))
	var hints []string
	for _, f := range failures {
		for _, c := range chain(f) {
			for _, h := range hintsOf(c) {
				if !slices.Contains(hints, h) {
					hints = append(hints, h)
				}
			}
		}
	}
	for _, h := range hints {
		b.WriteString("\nhint: " + h)
	}
	return b.String()
}

// unwrapFailures is ErrorReporter.Unwrap: the failures inside aggregates.
func unwrapFailures(err error) []error {
	switch agg := err.(type) {
	case *fetch.AggregateError:
		return flatten(agg.Errs)
	case *gitproto.AggregateError:
		return flatten(agg.Errs)
	}
	return []error{err}
}

func flatten(errs []error) []error {
	var out []error
	for _, e := range errs {
		out = append(out, unwrapFailures(e)...)
	}
	return out
}

// chain is an error and its causes, outermost first; an error with several
// causes is followed by the chains of each of them.
func chain(err error) []error {
	var out []error
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		out = append(out, cur)
		if multi, ok := cur.(interface{ Unwrap() []error }); ok {
			for _, inner := range multi.Unwrap() {
				out = append(out, chain(inner)...)
			}
			break
		}
	}
	return out
}

// summarize is ErrorReporter.Summarize.
func summarize(failures []error) string {
	var b strings.Builder
	for _, f := range failures {
		if len(failures) > 1 {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString("• ")
		}
		shown := ""
		for _, cause := range chain(f) {
			message := strings.TrimSpace(cause.Error())
			if message == "" || strings.Contains(shown, message) {
				continue
			}
			if shown == "" {
				b.WriteString(message)
			} else {
				b.WriteString("\n  caused by: " + message)
			}
			shown += message
		}
	}
	return b.String()
}

const networkHint = "lip could not reach the server. Check your network connection. If GitHub is slow or" +
	" blocked where you are, set a GitHub proxy with 'lip config set github_proxy <URL>'."

// hintsOf is ErrorReporter.Hints: what the user can do about an error.
func hintsOf(err error) []string {
	status, isHTTP := httpStatus(err)
	switch {
	case isHTTP && (status == 404 || status == 401):
		return []string{"The repository or file does not exist, or is private. Check the package name and" +
			" version for typos, and that the repository is public."}
	case isHTTP && (status == 403 || status == 429):
		return []string{"The server refused the request, possibly because too many requests were sent." +
			" Wait a while and try again, or set a GitHub proxy with 'lip config set github_proxy <URL>'."}
	case isHTTP && status >= 500:
		return []string{"The server had a problem. Try again later."}
	case isHTTP:
		return []string{networkHint}
	}
	switch e := err.(type) {
	case *url.Error, *net.OpError, *net.DNSError:
		return []string{networkHint}
	case *install.CommandFailedError:
		return []string{"A hook command of a package failed. To install or uninstall without running hook" +
			" commands, pass --ignore-scripts (the package may then need manual setup)."}
	case *graph.ResolutionError:
		return []string{"If the reasons above are about versions that do not fit together, request other" +
			" versions, or install with --no-dependencies to choose them yourself."}
	case syscall.Errno:
		return errnoHints(e)
	}
	return nil
}

// httpStatus is the status of an HttpRequestException: 0 when there was no
// response.
func httpStatus(err error) (int, bool) {
	switch e := err.(type) {
	case *gitproto.StatusError:
		return e.StatusCode, true
	case *gitproto.RequestError:
		return e.StatusCode, true
	case *fetch.DownloadError:
		return e.StatusCode, true
	}
	return 0, false
}

func errnoHints(errno syscall.Errno) []string {
	switch {
	case errors.Is(errno, fs.ErrPermission):
		return []string{"lip is not allowed to access a file. Check that it is not read-only and that you" +
			" may write to the workspace."}
	case runtime.GOOS == "windows" && (errno == 32 || errno == 33):
		// ERROR_SHARING_VIOLATION, ERROR_LOCK_VIOLATION.
		return []string{"A file is in use by another program. Stop the server and other programs that use" +
			" the workspace, then try again."}
	case runtime.GOOS == "windows" && (errno == 0x70 || errno == 0x27), runtime.GOOS != "windows" && errno == 28:
		// ERROR_DISK_FULL, ERROR_HANDLE_DISK_FULL; ENOSPC.
		return []string{"The disk is full. Free some space, then try again."}
	}
	return nil
}

var (
	urlPattern      = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9+.-]*://[^\s'"<>]+`)
	userInfoPattern = regexp.MustCompile(`^([^:]+://)[^/?#@]+@`)
	queryPattern    = regexp.MustCompile(`([?&][^=&#]+)=([^&#]*)`)
)

// redact is ErrorReporter.Redact: masks credentials in the URLs of a text,
// the user info (https://user:token@host/) and the values of query
// parameters (?token=...), except the harmless git 'service' parameter.
func redact(text string) string {
	return urlPattern.ReplaceAllStringFunc(text, func(u string) string {
		u = userInfoPattern.ReplaceAllString(u, "${1}***@")
		return queryPattern.ReplaceAllStringFunc(u, func(p string) string {
			m := queryPattern.FindStringSubmatch(p)
			if m[1] == "?service" || m[1] == "&service" || m[2] == "" {
				return p
			}
			return m[1] + "=***"
		})
	})
}
