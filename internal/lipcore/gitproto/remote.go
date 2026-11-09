package gitproto

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
)

// Limits of lip 0.38.0-beta.1's GitRemoteClient.
const (
	maxCachedTrees   = 64
	maxErrorBodyRune = 2000
)

// RemoteClient is a client for the git smart-HTTP protocol version 2
// (read-only, upload-pack), porting lip 0.38.0-beta.1
// src/Lip.Core/Git/GitRemoteClient.cs.
//
// ListTree uses a shallow partial fetch (deepen 1, filter blob:none) so only
// commits and trees are transferred; FetchBlobs then fetches individual
// blobs by object id. A remote that does not support both is not used, so a
// repository is never cloned in full.
//
// Every call tries the GitHub proxy (when GithubProxy is set and the
// repository is on github.com) and then the original URL; nothing is
// retried otherwise, and a failed proxy is tried again by the next call.
// Every failure names what the client was trying to do and with which
// server; the technical detail is kept as the wrapped error.
//
// Measured progress of every pack received is reported to the function set
// with WithPackProgress on the call's context.
//
// The zero value is usable; a RemoteClient is safe for concurrent use and
// must not be copied after first use.
type RemoteClient struct {
	// HTTP is the client to use; nil means defaultHTTPClient.
	HTTP *http.Client
	// GithubProxy is the github_proxy setting (protocol/userspace.md §2),
	// e.g. "https://gh.bibk.top/https://github.com"; "" means none.
	GithubProxy string
	// TempDir is where fetched packs are written while they are read, as
	// <guid>.tmp files deleted on close (lip uses its cache directory); ""
	// means os.TempDir().
	TempDir string

	remotes sync.Map // repo key -> remoteInfo

	treesMu   sync.Mutex
	trees     map[string]*treeCall
	treeOrder []string
}

// TreeEntry is one entry of a commit's tree (lip's GitTreeEntry): a blob or
// symlink; trees and gitlinks are not listed.
type TreeEntry struct {
	// Path is relative to the root tree, with "/" separators.
	Path string
	// Mode is git's octal mode digits read as a decimal number, e.g.
	// 100644, 100755 or 120000.
	Mode int
	// ObjectID is the lower-case blob id.
	ObjectID string
}

// IsRegularFile reports whether the entry is a regular (possibly
// executable) file rather than a symlink: mode 100644 or 100755.
func (e TreeEntry) IsRegularFile() bool { return e.Mode == 100644 || e.Mode == 100755 }

// ArgumentError is an invalid argument (lip's ArgumentException).
type ArgumentError struct{ Message string }

func (e *ArgumentError) Error() string { return e.Message }

// normalizeObjectID is lip's NormalizeObjectId.
func normalizeObjectID(objectID string) (string, error) {
	if !isObjectID(objectID) {
		return "", &ArgumentError{Message: fmt.Sprintf(
			"'%s' is not a git object id; an object id is 40 hexadecimal digits.", objectID)}
	}
	return strings.ToLower(objectID), nil
}

type progressKey struct{}

type progressSink struct {
	fn       func(PackProgress)
	attempts atomic.Int64
}

// WithPackProgress returns a context that makes RemoteClient calls report
// measured progress of every pack they receive to fn (see PackProgress).
// Each pack requested (through the proxy or directly, for every batch) is
// its own attempt, numbered from 1 across the calls made with the context:
// its first update is sent before the request is posted and its last one,
// sent exactly once whether the attempt succeeded or failed, is marked
// Done. Calls to fn are sequential within an attempt.
func WithPackProgress(ctx context.Context, fn func(PackProgress)) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, progressKey{}, &progressSink{fn: fn})
}

// ListRefs lists the refs whose names start with any of the prefixes (all
// refs when there are none) with ls-refs and "peel", so annotated tags
// carry Peeled. Since ref-prefix is only a hint to the server, refs are
// filtered again.
func (c *RemoteClient) ListRefs(ctx context.Context, repoURL string, prefixes []string) ([]RemoteRef, error) {
	w := new(PktLineWriter).
		WriteLine("command=ls-refs").
		WriteDelimiter().
		WriteLine("peel")
	for _, p := range prefixes {
		w.WriteLine("ref-prefix " + p)
	}
	w.WriteFlush()
	body := w.Bytes()

	refs, err := withProxyFallback(c, ctx, repoURL, "list the tags and branches of "+repoURL, "",
		func(url string) ([]RemoteRef, error) {
			remote, err := c.discover(ctx, url)
			if err != nil {
				return nil, err
			}
			var result []RemoteRef
			err = c.postCommand(ctx, remote, body, func(r *PktLineReader) error {
				for {
					typ, err := r.Read()
					if err != nil {
						return err
					}
					if typ == PktFlush {
						return nil
					}
					if typ != PktData {
						return protocolErrorf("Unexpected %s packet in the ls-refs response; expected a ref line or a flush.", typ)
					}
					line := r.PayloadText()
					if err := checkErrLine(line); err != nil {
						return err
					}
					ref, err := parseRef(line)
					if err != nil {
						return err
					}
					result = append(result, ref)
				}
			})
			return result, err
		})
	if err != nil {
		return nil, err
	}

	if len(prefixes) == 0 {
		return refs, nil
	}
	matching := make([]RemoteRef, 0, len(refs))
	for _, r := range refs {
		for _, p := range prefixes {
			if strings.HasPrefix(r.Name, p) {
				matching = append(matching, r)
				break
			}
		}
	}
	return matching, nil
}

// treeCall is one listing of a tree, shared by concurrent callers (lip's
// Lazy<Task>).
type treeCall struct {
	done    chan struct{}
	entries []TreeEntry
	err     error
}

// ListTree recursively lists the blob entries ('/'-separated paths relative
// to the root tree) of a commit or annotated tag id without downloading any
// blob. Gitlinks are excluded; symlinks are listed. Listings are cached for
// the client's lifetime (at most 64, the oldest evicted first) and shared
// by concurrent callers; failures are not cached. The returned slice is
// shared and must not be modified.
func (c *RemoteClient) ListTree(ctx context.Context, repoURL, commitish string) ([]TreeEntry, error) {
	objectID, err := normalizeObjectID(commitish)
	if err != nil {
		return nil, err
	}

	key := RepoKey(repoURL) + "@" + objectID
	for {
		c.treesMu.Lock()
		if c.trees == nil {
			c.trees = map[string]*treeCall{}
		}
		call, ok := c.trees[key]
		if !ok {
			call = &treeCall{done: make(chan struct{})}
			c.trees[key] = call
			c.treeOrder = append(c.treeOrder, key)
			for len(c.treeOrder) > maxCachedTrees {
				delete(c.trees, c.treeOrder[0])
				c.treeOrder = c.treeOrder[1:]
			}
		}
		c.treesMu.Unlock()

		if !ok {
			call.entries, call.err = c.fetchTree(ctx, repoURL, objectID)
			close(call.done)
		} else {
			select {
			case <-call.done:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if call.err == nil {
			return call.entries, nil
		}

		// Do not keep failures cached, so a later call can retry.
		c.treesMu.Lock()
		if c.trees[key] == call {
			delete(c.trees, key)
			if i := slices.Index(c.treeOrder, key); i >= 0 {
				c.treeOrder = slices.Delete(c.treeOrder, i, i+1)
			}
		}
		c.treesMu.Unlock()
		// A listing another caller canceled is not this caller's failure.
		if ok && ctx.Err() == nil && (errors.Is(call.err, context.Canceled) || errors.Is(call.err, context.DeadlineExceeded)) {
			continue
		}
		return nil, call.err
	}
}

func (c *RemoteClient) fetchTree(ctx context.Context, repoURL, objectID string) ([]TreeEntry, error) {
	notFound := fmt.Sprintf("The git repository %s has no commit %s. The commit may"+
		" have been removed from the repository (for example by a force push), or the commit"+
		" id is wrong; check it, or pick another version of the package.", repoURL, objectID)
	purpose := fmt.Sprintf("list the files of commit %s in %s", objectID, repoURL)
	return withProxyFallback(c, ctx, repoURL, purpose, notFound, func(url string) ([]TreeEntry, error) {
		remote, err := c.discover(ctx, url)
		if err != nil {
			return nil, err
		}
		if !remote.supportsShallow || !remote.supportsFilter {
			return nil, &UnsupportedServerError{Message: fmt.Sprintf(
				"The server at %s does not support shallow partial fetches (the 'shallow'"+
					" and 'filter' features of the fetch command).", url)}
		}

		pack, err := c.fetchPack(ctx, url, remote, []string{objectID}, true, true)
		if err != nil {
			return nil, err
		}
		defer pack.Close()
		store := gitpack.NewStore()
		if err := pack.ReadObjects(func(oid string, t gitpack.ObjectType, data []byte) error {
			if t != gitpack.ObjBlob {
				store.Add(oid, gitpack.Object{Type: t, Data: data})
			}
			return nil
		}); err != nil {
			return nil, err
		}

		// Inside the fallback, so an incomplete pack from the proxy is
		// retried directly.
		files, err := store.ListTree(objectID)
		if err != nil {
			return nil, err
		}
		entries := make([]TreeEntry, len(files))
		for i, f := range files {
			mode, _ := strconv.Atoi(f.Mode)
			entries[i] = TreeEntry{Path: f.Path, Mode: mode, ObjectID: f.OID}
		}
		return entries, nil
	})
}

// callbackError marks an error returned by FetchBlobs' onBlob, so it is
// returned as it is.
type callbackError struct{ err error }

func (e *callbackError) Error() string { return e.err.Error() }

// FetchBlobs downloads the given blobs (at most 256 per request) and calls
// onBlob once per object id with its content, sequentially. data stays
// valid after the call but must not be modified. A pack is fully
// downloaded before any callback runs, and a later attempt only asks for
// the blobs still missing, so no callback is ever invoked twice for the
// same blob. An error onBlob returns stops FetchBlobs and is returned as
// it is.
func (c *RemoteClient) FetchBlobs(ctx context.Context, repoURL string, blobObjectIDs []string,
	onBlob func(objectID string, data []byte) error) error {
	remaining := newOrderedSet()
	for _, id := range blobObjectIDs {
		oid, err := normalizeObjectID(id)
		if err != nil {
			return err
		}
		remaining.add(oid)
	}
	if remaining.len() == 0 {
		return nil
	}
	requested := remaining.len()

	var errs []error
	for batch := range slices.Chunk(remaining.list(), MaxWantsPerRequest) {
		batchErr, fatal := c.fetchBlobBatch(ctx, repoURL, batch, remaining, onBlob)
		if fatal != nil {
			return fatal
		}
		if batchErr != nil {
			errs = append(errs, batchErr)
		}
	}

	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	}
	message := fmt.Sprintf("Could not download %d of the %d requested files from %s: %d of the download requests failed.",
		remaining.len(), requested, repoURL, len(errs))
	for _, e := range errs {
		if !isGitProtocolError(e) {
			return &AggregateError{Message: message, Errs: errs}
		}
	}
	return &ProtocolError{Message: message, Err: &AggregateError{
		Message: fmt.Sprintf("The errors of the %d failed download requests.", len(errs)), Errs: errs}}
}

// fetchBlobBatch fetches one batch of blobs, trying the GitHub proxy first
// (if any) and then the original URL. It returns the error if some blobs
// could not be fetched (batchErr), or an error that ends FetchBlobs
// (fatal): onBlob's, a cancellation, or a bad pack whose blobs may already
// have been passed to onBlob.
func (c *RemoteClient) fetchBlobBatch(ctx context.Context, repoURL string, batch []string, remaining *orderedSet,
	onBlob func(string, []byte) error) (batchErr, fatal error) {
	purpose := fmt.Sprintf("download %d files from %s", len(batch), repoURL)
	notFound := fmt.Sprintf("The git repository %s does not have some of the files lip asked"+
		" for; the commit may have been removed from it (for example by a force push). Try"+
		" again, and if this keeps happening, pick another version of the package.", repoURL)
	candidates := c.candidateURLs(repoURL)
	var errs []error

	for _, url := range candidates {
		var wanted []string
		for _, id := range batch {
			if remaining.has(id) {
				wanted = append(wanted, id)
			}
		}

		remote, err := c.discover(ctx, url)
		var pack *gitpack.Pack
		if err == nil {
			pack, err = c.fetchPack(ctx, url, remote, wanted, false, false)
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			errs = append(errs, explain(err, url, repoURL, purpose, notFound))
			continue
		}

		err = pack.ReadObjects(func(oid string, t gitpack.ObjectType, data []byte) error {
			if !remaining.remove(oid) {
				return nil
			}
			if t != gitpack.ObjBlob {
				return &UnusableRepositoryError{Reason: fmt.Sprintf(
					"the object %s was requested as a file (blob) but is a %s", oid, gitpack.TypeName(t))}
			}
			if err := onBlob(oid, data); err != nil {
				return &callbackError{err: err}
			}
			return nil
		})
		pack.Close()
		if err != nil {
			var cb *callbackError
			if errors.As(err, &cb) {
				return nil, cb.err
			}
			if isGitProtocolCause(err) {
				// Callbacks may already have run, so this pack is not
				// retried elsewhere.
				return nil, explain(err, url, repoURL, purpose, notFound)
			}
			return nil, err
		}

		var missing []string
		for _, id := range batch {
			if remaining.has(id) {
				missing = append(missing, id)
			}
		}
		if len(missing) == 0 {
			return nil, nil
		}
		errs = append(errs, explain(protocolErrorf("The server did not send %d of the %d requested blobs, for example %s.",
			len(missing), len(wanted), missing[0]), url, repoURL, purpose, notFound))
	}

	if len(errs) == 1 {
		return errs[0], nil
	}
	return &AggregateError{Message: fallbackSummary(purpose, candidates[0]), Errs: errs}, nil
}

// withProxyFallback runs op against the GitHub proxy (if configured and
// applicable) and falls back to the original URL when the proxied attempt
// fails. purpose says what op does, as in "Could not <purpose>"; notFound,
// if not empty, explains a server error about missing objects.
func withProxyFallback[T any](c *RemoteClient, ctx context.Context, repoURL, purpose, notFound string,
	op func(url string) (T, error)) (T, error) {
	candidates := c.candidateURLs(repoURL)
	var errs []error
	var zero T
	for _, url := range candidates {
		result, err := op(url)
		if err == nil {
			return result, nil
		}
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		errs = append(errs, explain(err, url, repoURL, purpose, notFound))
	}
	if len(errs) == 1 {
		return zero, errs[0]
	}
	return zero, &AggregateError{Message: fallbackSummary(purpose, candidates[0]), Errs: errs}
}

func fallbackSummary(purpose, proxied string) string {
	return fmt.Sprintf("Could not %s: lip tried the GitHub proxy %s and then GitHub directly,"+
		" and both failed.", purpose, urlHost(proxied))
}

// isGitProtocolCause reports whether err is or wraps an error that is a
// GitProtocolException in lip (including corrupt pack data).
func isGitProtocolCause(err error) bool {
	for e := range errorChain(err) {
		if isGitProtocolError(e) {
			return true
		}
	}
	return false
}

// errorChain yields err and every error it wraps, depth first.
func errorChain(err error) func(func(error) bool) {
	return func(yield func(error) bool) {
		var walk func(error) bool
		walk = func(e error) bool {
			if e == nil {
				return true
			}
			if !yield(e) {
				return false
			}
			switch u := e.(type) {
			case interface{ Unwrap() error }:
				return walk(u.Unwrap())
			case interface{ Unwrap() []error }:
				for _, inner := range u.Unwrap() {
					if !walk(inner) {
						return false
					}
				}
			}
			return true
		}
		walk(err)
	}
}

// explain turns a failure of one attempt against url into an error that
// says, in plain words, what went wrong, wrapping the failure (lip's
// GitRemoteClient.Explain). Errors that are not about the server
// (cancellation, local file errors) are returned as they are.
func explain(err error, url, repoURL, purpose, notFound string) error {
	viaProxy := url != repoURL
	host := urlHost(url)
	var server string
	switch {
	case viaProxy:
		server = "the GitHub proxy " + host
	case isGithub(url):
		server = "GitHub"
	default:
		server = "the git server " + host
	}
	subject := upperFirst(server)
	proxyAdvice := ""
	if viaProxy {
		proxyAdvice = " If the GitHub proxy does not support git repositories, remove it with" +
			" 'lip config delete github_proxy'."
	}

	var uriErr *uriFormatError
	// An invalid github_proxy value is accepted by the configuration and
	// fails only here.
	if viaProxy && errors.As(err, &uriErr) {
		return &InvalidProxyError{Message: fmt.Sprintf(
			"The GitHub proxy set in github_proxy does not give a valid URL (%s). Set a valid"+
				" URL with 'lip config set github_proxy <URL>', or remove it with"+
				" 'lip config delete github_proxy'.", url), Err: err}
	}

	var (
		unusable    *UnusableRepositoryError
		format      *UnsupportedObjectFormatError
		serverErr   *ServerError
		unsupported *UnsupportedServerError
		status      *StatusError
		timeout     *timeoutError
		transport   *transportError
	)
	wrap := func(message string) error { return &ProtocolError{Message: message, Err: err} }
	switch {
	case errors.As(err, &unusable):
		return wrap(fmt.Sprintf("The git repository %s contains data lip cannot handle: %s."+
			" Pick another version of the package, or report the problem to the package author.",
			repoURL, unusable.Reason))
	case errors.As(err, &format):
		return wrap(fmt.Sprintf("The git repository %s uses %s object ids, which lip does not"+
			" support yet. Pick another version of the package, or ask the package author to"+
			" publish it from a repository that uses SHA-1 object ids.", repoURL, format.Format))
	case errors.As(err, &serverErr) && notFound != "" && strings.Contains(serverErr.ServerMessage, "not our ref"):
		return wrap(notFound)
	case errors.As(err, &serverErr):
		return wrap(fmt.Sprintf("%s reported an error when lip tried to %s. Try again later.%s",
			subject, purpose, proxyAdvice))
	case errors.As(err, &unsupported):
		return wrap(fmt.Sprintf("%s cannot be used to %s: lip needs a git server that supports version 2"+
			" of the git protocol with shallow and partial fetches, as GitHub does.%s",
			subject, purpose, proxyAdvice))
	case isGitProtocolCause(err):
		return wrap(fmt.Sprintf("%s sent an unexpected response when lip tried to %s. Try again later;"+
			" if this keeps happening, it may not be a git server.%s", subject, purpose, proxyAdvice))
	case errors.As(err, &status):
		return &RequestError{
			Message:    describeStatus(server, subject, purpose, status.StatusCode, isGithub(repoURL)) + proxyAdvice,
			StatusCode: status.StatusCode,
			Err:        err,
		}
	case errors.As(err, &timeout):
		return &RequestError{Message: fmt.Sprintf("%s did not answer in time when lip tried to %s. Check your network"+
			" connection and try again.%s", subject, purpose, proxyAdvice), Err: err}
	case errors.As(err, &transport) && transport.body:
		return &RequestError{Message: fmt.Sprintf("The connection to %s broke off while lip tried to %s. Check your network"+
			" connection and try again.%s", server, purpose, proxyAdvice), Err: err}
	case errors.As(err, &transport), errors.As(err, &uriErr):
		return &RequestError{Message: fmt.Sprintf("Could not connect to %s to %s. Check your network connection and"+
			" try again.%s", server, purpose, proxyAdvice), Err: err}
	}
	return err
}

// describeStatus is lip's GitRemoteClient.DescribeStatus.
func describeStatus(server, subject, purpose string, status int, github bool) string {
	code := fmt.Sprintf("HTTP %d", status)
	switch {
	case status == http.StatusUnauthorized:
		why := "The repository may not exist or may be private"
		if github {
			why = "GitHub answers this way when a repository does not exist or is private"
		}
		return fmt.Sprintf("%s refused to let lip %s (%s). %s: check the spelling of the package, and that its"+
			" repository is public.", subject, purpose, code, why)
	case status == http.StatusForbidden:
		return fmt.Sprintf("%s denied access when lip tried to %s (%s). The repository may be"+
			" private, or %s may be limiting how many requests lip can send; wait a while"+
			" and try again.", subject, purpose, code, server)
	case status == http.StatusNotFound:
		return fmt.Sprintf("%s found no such repository when lip tried to %s (%s). Check the"+
			" spelling of the package, and that its repository is public.", subject, purpose, code)
	case status == http.StatusTooManyRequests:
		return fmt.Sprintf("%s refused to let lip %s because lip sent too many requests (%s)."+
			" Wait a while and try again.", subject, purpose, code)
	case status >= 500:
		return fmt.Sprintf("%s had a problem when lip tried to %s (%s). Try again later.", subject, purpose, code)
	}
	name := ""
	if n, ok := httpStatusNames[status]; ok {
		name = " (" + n + ")"
	}
	return fmt.Sprintf("%s answered %s%s when lip tried to %s. Try again later; if this keeps happening, check that"+
		" the URL points to a git repository.", subject, code, name, purpose)
}

// httpStatusNames are the names of .NET's HttpStatusCode values below 500
// (Enum.ToString), for the statuses describeStatus names.
var httpStatusNames = map[int]string{
	100: "Continue", 101: "SwitchingProtocols", 102: "Processing", 103: "EarlyHints",
	200: "OK", 201: "Created", 202: "Accepted", 203: "NonAuthoritativeInformation", 204: "NoContent",
	205: "ResetContent", 206: "PartialContent", 207: "MultiStatus", 208: "AlreadyReported", 226: "IMUsed",
	300: "MultipleChoices", 301: "MovedPermanently", 302: "Found", 303: "SeeOther", 304: "NotModified",
	305: "UseProxy", 306: "Unused", 307: "RedirectKeepVerb", 308: "PermanentRedirect",
	400: "BadRequest", 401: "Unauthorized", 402: "PaymentRequired", 403: "Forbidden", 404: "NotFound",
	405: "MethodNotAllowed", 406: "NotAcceptable", 407: "ProxyAuthenticationRequired", 408: "RequestTimeout",
	409: "Conflict", 410: "Gone", 411: "LengthRequired", 412: "PreconditionFailed", 413: "RequestEntityTooLarge",
	414: "RequestUriTooLong", 415: "UnsupportedMediaType", 416: "RequestedRangeNotSatisfiable",
	417: "ExpectationFailed", 421: "MisdirectedRequest", 422: "UnprocessableEntity", 423: "Locked",
	424: "FailedDependency", 426: "UpgradeRequired", 428: "PreconditionRequired", 429: "TooManyRequests",
	431: "RequestHeaderFieldsTooLarge", 451: "UnavailableForLegalReasons",
}

func isGithub(rawURL string) bool { return strings.EqualFold(urlHost(rawURL), "github.com") }

// upperFirst is char.ToUpperInvariant(s[0]) + s[1..].
func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// candidateURLs returns the URLs to try for a repository: the GitHub proxy
// (if applicable), then the original URL.
func (c *RemoteClient) candidateURLs(repoURL string) []string {
	if proxied, ok := rewriteGithub(c.GithubProxy, repoURL); ok {
		return []string{proxied, repoURL}
	}
	return []string{repoURL}
}

// rewriteGithub is lip's GithubProxy.Rewrite: the URL of a github.com request
// sent through the proxy, built by appending the request's path segments to
// the proxy URL (like Flurl's AppendPathSegments) and, when the request has
// a query, replacing the proxy's query with it. It reports false when no
// proxy applies: proxy is empty, the host is not github.com (compared
// case-insensitively) or the rewritten URL is the original one.
func rewriteGithub(proxy, rawURL string) (string, bool) {
	if proxy == "" || !isGithub(rawURL) {
		return "", false
	}
	base, query, fragment := splitURL(proxy)
	reqBase, reqQuery, _ := splitURL(rawURL)
	path := reqBase
	if i := strings.Index(path, "://"); i >= 0 {
		path = path[i+3:]
		if j := strings.IndexByte(path, '/'); j >= 0 {
			path = path[j:]
		} else {
			path = ""
		}
	}
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, cleanSegment(s))
		}
	}
	if len(segs) > 0 {
		base = strings.TrimRight(base, "/") + "/" + strings.Join(segs, "/")
	}
	if reqQuery != "" {
		query = reqQuery
	}
	out := base
	if query != "" {
		out += "?" + query
	}
	if fragment != "" {
		out += "#" + fragment
	}
	if out == rawURL {
		return "", false
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

// cleanSegment percent-encodes the bytes of a path segment that may not
// appear in a URL path, keeping existing %XX escapes (Flurl CleanSegment).
func cleanSegment(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			strings.IndexByte("-._~!$&'()*+,;=:@%", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0xF])
	}
	return b.String()
}

// newRequest builds a request for an absolute http(s) URL; any other URL is
// a uriFormatError.
func newRequest(method, rawURL string, body io.Reader) (*http.Request, error) {
	u, err := url.Parse(rawURL)
	if err == nil && (u.Scheme == "" || u.Host == "") {
		err = fmt.Errorf("invalid URI: %s is not an absolute URL", rawURL)
	}
	if err != nil {
		return nil, &uriFormatError{err: err}
	}
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, &uriFormatError{err: err}
	}
	return req, nil
}

func (c *RemoteClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return defaultHTTPClient
}

// discover ports GitRemoteClient.Discover; results are cached per RepoKey.
func (c *RemoteClient) discover(ctx context.Context, repoURL string) (remoteInfo, error) {
	key := RepoKey(repoURL)
	if cached, ok := c.remotes.Load(key); ok {
		return cached.(remoteInfo), nil
	}

	base := strings.TrimRight(repoURL, "/")
	infoRefs := base + "/info/refs?service=git-upload-pack"
	req, err := newRequest(http.MethodGet, infoRefs, nil)
	if err != nil {
		return remoteInfo{}, err
	}
	// Go forwards these headers on redirects, which lip enables explicitly
	// (without Git-Protocol the redirect target answers with protocol v0).
	req.Header.Set("Git-Protocol", gitProtocolHeader)
	req.Header.Set("User-Agent", UserAgent)
	resp, cancel, err := do(ctx, c.httpClient(), req, discoveryTimeout)
	if err != nil {
		return remoteInfo{}, err
	}
	defer cancel()
	defer resp.Body.Close()
	if err := ensureSuccess(ctx, resp, "GET", infoRefs); err != nil {
		return remoteInfo{}, err
	}

	// Follow redirects (e.g. renamed repositories) for subsequent POSTs,
	// like git does.
	endpoint := repoURL
	if resp.Request != nil && resp.Request.URL != nil {
		u := *resp.Request.URL
		u.RawQuery = ""
		u.ForceQuery = false
		u.Fragment = ""
		u.RawFragment = ""
		if p := u.String(); strings.HasSuffix(p, "/info/refs") {
			endpoint = strings.TrimSuffix(p, "/info/refs")
		}
	}

	caps, err := readCapabilities038(NewPktLineReader(resp.Body), repoURL)
	if err != nil {
		return remoteInfo{}, err
	}
	_, hasLsRefs := caps["ls-refs"]
	fetch, hasFetch := caps["fetch"]
	if !hasLsRefs || !hasFetch {
		return remoteInfo{}, &UnsupportedServerError{Message: fmt.Sprintf(
			"The server at %s does not offer the ls-refs and fetch commands of git protocol version 2.", repoURL)}
	}
	if format, ok := caps["object-format"]; ok && (format == nil || *format != "sha1") {
		name := "(unnamed)"
		if format != nil {
			name = *format
		}
		return remoteInfo{}, &UnsupportedObjectFormatError{Format: name}
	}

	info := remoteInfo{endpoint: endpoint}
	if fetch != nil {
		for _, f := range strings.Split(*fetch, " ") {
			switch f {
			case "filter":
				info.supportsFilter = true
			case "shallow":
				info.supportsShallow = true
			}
		}
	}
	c.remotes.Store(key, info)
	return info, nil
}

// readCapabilities038 parses a v2 capability advertisement, skipping an
// optional "# service=..." line and flush. Values are nil for keys without
// "=".
func readCapabilities038(r *PktLineReader, repoURL string) (map[string]*string, error) {
	typ, err := r.Read()
	if err != nil {
		return nil, err
	}
	// Smart HTTP servers may prefix the advertisement with a service line
	// and a flush.
	if typ == PktData && strings.HasPrefix(r.PayloadText(), "# service=") {
		if typ, err = r.Read(); err != nil {
			return nil, err
		}
		if typ == PktFlush {
			if typ, err = r.Read(); err != nil {
				return nil, err
			}
		}
	}
	if typ != PktData || r.PayloadText() != "version 2" {
		return nil, &UnsupportedServerError{Message: fmt.Sprintf(
			"The server at %s does not support version 2 of the git protocol.", repoURL)}
	}

	caps := map[string]*string{}
	for {
		if typ, err = r.Read(); err != nil {
			return nil, err
		}
		if typ != PktData {
			break
		}
		line := r.PayloadText()
		if k, v, ok := strings.Cut(line, "="); ok {
			caps[k] = &v
		} else {
			caps[line] = nil
		}
	}
	if typ != PktFlush {
		return nil, protocolErrorf("Malformed capability advertisement from %s: expected a flush packet after the"+
			" capabilities, got %s.", repoURL, typ)
	}
	return caps, nil
}

// ensureSuccess returns a StatusError for a non-2xx response, keeping the
// start of the body, which often says why (lip's EnsureSuccess).
func ensureSuccess(ctx context.Context, resp *http.Response, verb, url string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	reason := strings.TrimPrefix(resp.Status, fmt.Sprint(resp.StatusCode))
	reason = strings.TrimPrefix(reason, " ")
	e := &StatusError{Verb: verb, URL: url, StatusCode: resp.StatusCode, Reason: reason}
	// Only the start of the body is read, so a large or slow error page
	// does not delay the error.
	br := bufio.NewReader(resp.Body)
	var sb strings.Builder
	n := 0
	var readErr error
	for n <= maxErrorBodyRune {
		r, _, err := br.ReadRune()
		if err != nil {
			if err != io.EOF {
				readErr = err
			}
			break
		}
		if n < maxErrorBodyRune {
			sb.WriteRune(r)
		}
		n++
	}
	switch {
	case readErr != nil && ctx.Err() != nil:
		return ctx.Err()
	case readErr != nil:
		e.Body = "(cannot read: " + readErr.Error() + ")"
	case n > maxErrorBodyRune:
		e.Body = sb.String() + "..."
	default:
		e.Body = sb.String()
	}
	return e
}

// postCommand ports GitRemoteClient.PostCommand.
func (c *RemoteClient) postCommand(ctx context.Context, remote remoteInfo, body []byte, handle func(*PktLineReader) error) error {
	url := strings.TrimRight(remote.endpoint, "/") + "/git-upload-pack"
	req, err := newRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Git-Protocol", gitProtocolHeader)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	resp, cancel, err := do(ctx, c.httpClient(), req, commandTimeout)
	if err != nil {
		return err
	}
	defer cancel()
	defer resp.Body.Close()
	if err := ensureSuccess(ctx, resp, "POST", url); err != nil {
		return err
	}
	return handle(NewPktLineReader(resp.Body))
}

// createPackTemp creates the file a fetched pack is written to in dir;
// removeOnClose is true when PackFile.Close must delete it. Tests replace
// it.
var createPackTemp = func(dir string) (f *os.File, removeOnClose bool, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, false, err
	}
	f, err = createDeleteOnCloseFile(dir, "", ".tmp")
	return f, false, err
}

func (c *RemoteClient) tempDir() string {
	if c.TempDir != "" {
		return c.TempDir
	}
	return os.TempDir()
}

// fetchPack runs the fetch command (no-progress, ofs-delta, [deepen 1],
// [filter blob:none], wants, done), writes the pack to a temporary file and
// opens it. url is the candidate URL, for progress. The caller closes the
// pack, which deletes the file.
func (c *RemoteClient) fetchPack(ctx context.Context, url string, remote remoteInfo, wants []string,
	deepen, filterBlobs bool) (*gitpack.Pack, error) {
	w := new(PktLineWriter).
		WriteLine("command=fetch").
		WriteDelimiter().
		WriteLine("no-progress").
		WriteLine("ofs-delta")
	if deepen {
		w.WriteLine("deepen 1")
	}
	if filterBlobs {
		w.WriteLine("filter blob:none")
	}
	for _, id := range wants {
		w.WriteLine("want " + id)
	}
	w.WriteLine("done").WriteFlush()

	f, remove, err := createPackTemp(c.tempDir())
	if err != nil {
		return nil, err
	}
	file := &PackFile{f: f, remove: remove}
	bw := bufio.NewWriterSize(f, 80<<10)
	var out io.Writer = bw
	var tracker *packTracker
	if sink, ok := ctx.Value(progressKey{}).(*progressSink); ok {
		tracker = newPackTracker(sink.fn, int(sink.attempts.Add(1)), url)
		out = io.MultiWriter(bw, tracker)
	}
	err = c.postCommand(ctx, remote, w.Bytes(), func(r *PktLineReader) error {
		if err := ReadPackfile(r, out); err != nil {
			return err
		}
		return bw.Flush()
	})
	if tracker != nil {
		tracker.finish(err)
	}
	if err == nil {
		err = file.finish()
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	// Open closes file when it fails.
	return gitpack.Open(file, file.Size())
}

// do sends req with a per-request timeout and returns the response whose
// body maps read failures to transport/timeout errors. cancel must be
// called once the body has been consumed.
func do(ctx context.Context, client *http.Client, req *http.Request, timeout time.Duration) (*http.Response, context.CancelFunc, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	req = req.WithContext(reqCtx)
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, nil, classifyNetError(ctx, reqCtx, req.URL.String(), err, false)
	}
	resp.Body = &classifyingBody{rc: resp.Body, parent: ctx, reqCtx: reqCtx, url: req.URL.String()}
	return resp, cancel, nil
}

// orderedSet is a set of strings that keeps insertion order (a .NET
// HashSet without removals in between).
type orderedSet struct {
	order []string
	m     map[string]bool
}

func newOrderedSet() *orderedSet { return &orderedSet{m: map[string]bool{}} }

func (s *orderedSet) add(v string) {
	if !s.m[v] {
		s.m[v] = true
		s.order = append(s.order, v)
	}
}

func (s *orderedSet) has(v string) bool { return s.m[v] }

func (s *orderedSet) remove(v string) bool {
	if !s.m[v] {
		return false
	}
	delete(s.m, v)
	return true
}

func (s *orderedSet) len() int { return len(s.m) }

// list returns the members in insertion order.
func (s *orderedSet) list() []string {
	out := make([]string, 0, len(s.m))
	for _, v := range s.order {
		if s.m[v] {
			out = append(out, v)
		}
	}
	return out
}
