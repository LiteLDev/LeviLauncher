// Package gitproto is a minimal git smart-HTTP protocol v2 client, porting
// lip 0.37.0 src/Lip.Core/Git/GitRemoteClient.cs and PktLine.cs.
//
// It lists version tags (ls-refs) and fetches packfiles (fetch) into
// temporary files. Packfile decoding lives in package gitpack.
package gitproto

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	xsemver "golang.org/x/mod/semver"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("gitproto: not implemented")

// UserAgent is sent with every request, as lip does.
const UserAgent = "git/2.0 (lip)"

// MaxWantsPerRequest is the largest number of wants Fetch sends at once.
const MaxWantsPerRequest = 256

const (
	gitProtocolHeader   = "version=2"
	maxTransientRetries = 2
	retryBaseDelay      = 500 * time.Millisecond
	// discoveryTimeout is Flurl's default request timeout (100 s).
	discoveryTimeout = 100 * time.Second
	// commandTimeout is lip's timeout for upload-pack POSTs.
	commandTimeout = 30 * time.Minute

	tagsRefPrefix = "refs/tags/"
	tagPrefix     = tagsRefPrefix + "v"
)

// Proxy routes a request URL through a mirror with fallback. fn is called
// with the URL to use; Try returns fn's final error. fetch.ProxyRewriter
// implements it. A nil Proxy means direct access.
type Proxy interface {
	Try(ctx context.Context, rawURL string, fn func(url string) error) error
}

// Client talks to remote repositories such as
// "https://github.com/LiteLDev/LeviLamina.git". The zero value is usable and
// safe for concurrent use.
type Client struct {
	// HTTP is the client to use; nil means http.DefaultClient.
	HTTP *http.Client
	// Proxy is optional.
	Proxy Proxy

	// sleep waits between retries; nil means a context-aware timer. Tests
	// replace it.
	sleep func(ctx context.Context, d time.Duration) error

	mu      sync.Mutex
	remotes map[string]remoteInfo
}

// remoteInfo is the result of discovery (lip's RemoteInfo record).
type remoteInfo struct {
	endpoint        string
	supportsFilter  bool
	supportsShallow bool
}

// TagRef is one version tag.
type TagRef struct {
	// Name is the tag name without "refs/tags/", e.g. "v1.2.3".
	Name string
	// Version is the canonical version without "v" (Go semver.Canonical, so
	// "+build" is dropped and "v1.2" becomes "1.2.0").
	Version string
	// Commit is the peeled commit id for annotated tags, else the ref id.
	Commit string
}

// RemoteRef is one ref of an ls-refs response (lip's GitRemoteRef).
type RemoteRef struct {
	// Name is the full ref name, e.g. "refs/tags/v1.2.3".
	Name string
	// ObjectID is the lower-case object id the ref points to.
	ObjectID string
	// Peeled is the lower-case peeled object id of an annotated tag, or "".
	Peeled string
}

// ListTags performs discovery (GET <repo>/info/refs?service=git-upload-pack
// with Git-Protocol: version=2; requires "version 2", ls-refs and fetch,
// object-format sha1 if present; follows redirects, using the final URL as
// the POST endpoint) and "ls-refs" with "peel" and "ref-prefix refs/tags/v".
// Non-semver tags are dropped; when several tags map to the same version,
// "refs/tags/v<version>" wins, otherwise the ordinally smallest name.
// Results are sorted by tag name. Retries per lip (2 retries on 5xx, 429,
// timeouts or IO errors, 500ms*n backoff).
func (c *Client) ListTags(ctx context.Context, repoURL string) ([]TagRef, error) {
	refs, err := c.ListRefs(ctx, repoURL, []string{tagPrefix})
	if err != nil {
		return nil, err
	}
	return versionTags(refs), nil
}

// versionTags ports GitRemotePackageRegistry.ListVersionTags.
func versionTags(refs []RemoteRef) []TagRef {
	sorted := append([]RemoteRef(nil), refs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	byVersion := map[string]RemoteRef{}
	for _, ref := range sorted {
		version, ok := parseTagVersion(ref.Name)
		if !ok {
			continue
		}
		existing, found := byVersion[version]
		if !found || (ref.Name == tagPrefix+version && existing.Name != ref.Name) {
			byVersion[version] = ref
		}
	}

	tags := make([]TagRef, 0, len(byVersion))
	for version, ref := range byVersion {
		commit := ref.Peeled
		if commit == "" {
			commit = ref.ObjectID
		}
		tags = append(tags, TagRef{
			Name:    strings.TrimPrefix(ref.Name, tagsRefPrefix),
			Version: version,
			Commit:  commit,
		})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	return tags
}

// parseTagVersion ports GitRemotePackageRegistry.ParseTagVersion: the ref
// must start with "refs/tags/v" and canonicalize to a valid version.
func parseTagVersion(refName string) (string, bool) {
	if !strings.HasPrefix(refName, tagPrefix) {
		return "", false
	}
	canonical := canonicalTag(refName[len(tagsRefPrefix):])
	if canonical == "" {
		return "", false
	}
	version := canonical[1:]
	// SemVersion.TryParse rejects input longer than its default maxLength.
	if len(version) > 1024 {
		return "", false
	}
	return version, true
}

// ListRefs runs "ls-refs" with "peel" and one "ref-prefix" per prefix. Since
// ref-prefix is only a hint to the server, refs are filtered again.
func (c *Client) ListRefs(ctx context.Context, repoURL string, prefixes []string) ([]RemoteRef, error) {
	w := new(PktLineWriter).
		WriteLine("command=ls-refs").
		WriteDelimiter().
		WriteLine("peel")
	for _, p := range prefixes {
		w.WriteLine("ref-prefix " + p)
	}
	w.WriteFlush()
	body := w.Bytes()

	var refs []RemoteRef
	err := c.withProxyFallback(ctx, repoURL, func(url string) error {
		remote, err := c.discover(ctx, url)
		if err != nil {
			return err
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
					return protocolErrorf("Unexpected %s in ls-refs response.", typ)
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
		if err != nil {
			return err
		}
		refs = result
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(prefixes) == 0 {
		return refs, nil
	}
	filtered := refs[:0]
	for _, r := range refs {
		for _, p := range prefixes {
			if strings.HasPrefix(r.Name, p) {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered, nil
}

// FetchOptions configures a fetch command.
type FetchOptions struct {
	// Shallow adds "deepen 1" when the server advertises "shallow".
	Shallow bool
	// BlobNone adds "filter blob:none" when the server advertises "filter".
	BlobNone bool
	// Want, when set, is asked before each attempt (the GitHub proxy, then
	// the original URL) which wants are still needed, so a fallback only
	// requests what an earlier attempt's handler did not consume, like lip's
	// FetchBlobBatch. An attempt with nothing left to want succeeds without a
	// request.
	Want func(oid string) bool
}

// PackHandler consumes a fetched pack. It runs inside the per-URL proxy
// fallback, so an error it returns (e.g. an incomplete pack) marks the proxy
// failed and retries the original URL, as lip validates packs inside
// WithProxyFallback. The handler owns pack and must Close it, unless it
// keeps it for later use.
type PackHandler func(pack *PackFile) error

// Fetch runs the v2 "fetch" command (no-progress, ofs-delta, wants, done),
// writes the packfile from side-band channel 1 to a temporary file (channel
// 2 is progress and ignored, channel 3 a fatal error) and passes it to
// handle. At most 256 wants are sent per request; callers batch larger sets.
func (c *Client) Fetch(ctx context.Context, repoURL string, wants []string, opts FetchOptions, handle PackHandler) error {
	if len(wants) == 0 {
		return errors.New("gitproto: no wants")
	}
	if len(wants) > MaxWantsPerRequest {
		return fmt.Errorf("gitproto: %d wants exceed the limit of %d per request", len(wants), MaxWantsPerRequest)
	}
	ids := make([]string, len(wants))
	for i, w := range wants {
		if !isObjectID(w) {
			return fmt.Errorf("gitproto: '%s' is not a SHA-1 object id", w)
		}
		ids[i] = strings.ToLower(w)
	}

	return c.withProxyFallback(ctx, repoURL, func(url string) error {
		wanted := ids
		if opts.Want != nil {
			wanted = nil
			for _, id := range ids {
				if opts.Want(id) {
					wanted = append(wanted, id)
				}
			}
			if len(wanted) == 0 {
				return nil
			}
		}
		remote, err := c.discover(ctx, url)
		if err != nil {
			return err
		}
		filtered := opts.BlobNone && remote.supportsFilter
		w := new(PktLineWriter).
			WriteLine("command=fetch").
			WriteDelimiter().
			WriteLine("no-progress").
			WriteLine("ofs-delta")
		if opts.Shallow && remote.supportsShallow {
			w.WriteLine("deepen 1")
		}
		if filtered {
			w.WriteLine("filter blob:none")
		}
		for _, id := range wanted {
			w.WriteLine("want " + id)
		}
		w.WriteLine("done").WriteFlush()

		pack, err := newPackFile()
		if err != nil {
			return err
		}
		bw := bufio.NewWriterSize(pack.f, 80<<10)
		if err := c.postCommand(ctx, remote, w.Bytes(), func(r *PktLineReader) error {
			if err := ReadPackfile(r, bw); err != nil {
				return err
			}
			return bw.Flush()
		}); err != nil {
			pack.Close()
			return err
		}
		if err := pack.finish(); err != nil {
			pack.Close()
			return err
		}
		pack.Filtered = filtered
		return handle(pack)
	})
}

// createTemp creates the file a fetched pack is written to; removeOnClose
// is true when PackFile.Close must delete it (it is not already deleted by
// the system). Tests replace it.
var createTemp = func() (f *os.File, removeOnClose bool, err error) {
	f, err = createDeleteOnCloseTemp()
	return f, false, err
}

// PackFile is a fetched packfile in a temporary file that is deleted when
// closed (lip opens it with FileOptions.DeleteOnClose).
type PackFile struct {
	// Filtered reports that "filter blob:none" was sent, so the pack holds
	// no blobs. A pack fetched without it from a server that does not
	// support filtering holds every blob of the tree.
	Filtered bool

	f      *os.File
	remove bool
	size   int64
	once   sync.Once
	err    error
}

func newPackFile() (*PackFile, error) {
	f, remove, err := createTemp()
	if err != nil {
		return nil, err
	}
	return &PackFile{f: f, remove: remove}, nil
}

// NewPackFile writes the pack read from r to a temporary file.
func NewPackFile(r io.Reader) (*PackFile, error) {
	p, err := newPackFile()
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(p.f, r); err != nil {
		p.Close()
		return nil, err
	}
	if err := p.finish(); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func (p *PackFile) finish() error {
	size, err := p.f.Seek(0, io.SeekEnd)
	p.size = size
	return err
}

// Size returns the pack's length in bytes.
func (p *PackFile) Size() int64 { return p.size }

// ReadAt implements io.ReaderAt.
func (p *PackFile) ReadAt(b []byte, off int64) (int, error) { return p.f.ReadAt(b, off) }

// Close closes and deletes the file. It may be called more than once.
func (p *PackFile) Close() error {
	p.once.Do(func() {
		p.err = p.f.Close()
		if !p.remove {
			return
		}
		if err := os.Remove(p.f.Name()); err != nil && p.err == nil {
			p.err = err
		}
	})
	return p.err
}

// ReadPackfile reads a fetch response up to and including the packfile
// section and writes the demultiplexed pack data (side-band channel 1) to w.
// Sections before "packfile" (acknowledgments, shallow-info, wanted-refs,
// packfile-uris) are skipped.
func ReadPackfile(r *PktLineReader, w io.Writer) error {
	for {
		typ, err := r.Read()
		if err != nil {
			return err
		}
		if typ != PktData {
			return protocolErrorf("Unexpected %s in fetch response.", typ)
		}
		header := r.PayloadText()
		if err := checkErrLine(header); err != nil {
			return err
		}
		if header == "packfile" {
			break
		}
		for {
			typ, err = r.Read()
			if err != nil {
				return err
			}
			if typ != PktData {
				break
			}
			if err := checkErrLine(r.PayloadText()); err != nil {
				return err
			}
		}
		if typ != PktDelimiter {
			return protocolErrorf("Fetch response ended without a packfile (section %s).", header)
		}
	}

	for {
		typ, err := r.Read()
		if err != nil {
			return err
		}
		switch typ {
		case PktFlush, PktResponseEnd:
			return nil
		case PktData:
		default:
			return protocolErrorf("Unexpected %s in packfile section.", typ)
		}
		payload := r.Payload
		if len(payload) == 0 {
			return protocolErrorf("Empty side-band packet.")
		}
		switch payload[0] {
		case 1:
			if _, err := w.Write(payload[1:]); err != nil {
				return err
			}
		case 2:
		case 3:
			return protocolErrorf("Remote error: %s", strings.TrimSpace(string(payload[1:])))
		default:
			return protocolErrorf("Invalid side-band channel %d.", payload[0])
		}
	}
}

// RepoURL maps a package path to its repository URL ("https://<path>.git").
func RepoURL(packagePath string) string {
	return "https://" + packagePath + ".git"
}

// RepoKey identifies a repository for caches (lip's GetRepoKey). GitHub
// resolves owner and repository names case-insensitively and ".git" is
// optional there, so github.com URLs are lower-cased and stripped of it;
// other URLs are kept as they are (minus trailing slashes).
func RepoKey(repoURL string) string {
	key := strings.TrimRight(repoURL, "/")
	if !strings.EqualFold(urlHost(repoURL), "github.com") {
		return key
	}
	key = strings.ToLower(key)
	return strings.TrimSuffix(key, ".git")
}

// urlHost returns the host (without port or userinfo) of an absolute URL.
func urlHost(raw string) string {
	i := strings.Index(raw, "://")
	if i < 0 {
		return ""
	}
	rest := raw[i+3:]
	if j := strings.IndexAny(rest, "/?#"); j >= 0 {
		rest = rest[:j]
	}
	if j := strings.LastIndex(rest, "@"); j >= 0 {
		rest = rest[j+1:]
	}
	if strings.HasPrefix(rest, "[") {
		if j := strings.Index(rest, "]"); j >= 0 {
			return rest[1:j]
		}
	}
	if j := strings.LastIndex(rest, ":"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// canonicalTag applies Go semver.Canonical (v1.2 -> v1.2.0, +build dropped);
// "" for invalid tags.
func canonicalTag(tag string) string { return xsemver.Canonical(tag) }

// withProxyFallback runs op through the proxy (when set), retrying each
// candidate URL on transient errors (lip's WithProxyFallback + WithRetry).
// Discovery and the following POST of one attempt use the same base URL.
func (c *Client) withProxyFallback(ctx context.Context, repoURL string, op func(url string) error) error {
	attempt := func(url string) error {
		return c.withRetry(ctx, func() error { return op(url) })
	}
	if c.Proxy == nil {
		return attempt(repoURL)
	}
	return c.Proxy.Try(ctx, repoURL, attempt)
}

func (c *Client) withRetry(ctx context.Context, op func() error) error {
	for attempt := 0; ; attempt++ {
		err := op()
		if err == nil {
			return nil
		}
		if attempt >= maxTransientRetries || !isTransient(err) || ctx.Err() != nil {
			return err
		}
		if serr := c.doSleep(ctx, retryBaseDelay*time.Duration(attempt+1)); serr != nil {
			return serr
		}
	}
}

func (c *Client) doSleep(ctx context.Context, d time.Duration) error {
	if c.sleep != nil {
		return c.sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// do sends req with a per-request timeout and returns the response whose
// body maps read failures to transport/timeout errors. cancel must be
// called once the body has been consumed.
func (c *Client) do(ctx context.Context, req *http.Request, timeout time.Duration) (*http.Response, context.CancelFunc, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	req = req.WithContext(reqCtx)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		cancel()
		return nil, nil, classifyNetError(ctx, reqCtx, req.URL.String(), err)
	}
	resp.Body = &classifyingBody{rc: resp.Body, parent: ctx, reqCtx: reqCtx, url: req.URL.String()}
	return resp, cancel, nil
}

func classifyNetError(parent, reqCtx context.Context, url string, err error) error {
	if perr := parent.Err(); perr != nil {
		return perr
	}
	if errors.Is(reqCtx.Err(), context.DeadlineExceeded) {
		return &timeoutError{url: url}
	}
	return &transportError{err: err}
}

// classifyingBody turns body read errors (other than a clean EOF) into
// transport errors, like .NET surfaces them as IOException.
type classifyingBody struct {
	rc     io.ReadCloser
	parent context.Context
	reqCtx context.Context
	url    string
}

func (b *classifyingBody) Read(p []byte) (int, error) {
	n, err := b.rc.Read(p)
	if err != nil && err != io.EOF {
		err = classifyNetError(b.parent, b.reqCtx, b.url, err)
	}
	return n, err
}

// Close drains a bounded remainder so the connection can be reused.
func (b *classifyingBody) Close() error {
	_, _ = io.CopyN(io.Discard, b.rc, 64<<10)
	return b.rc.Close()
}

// discover ports GitRemoteClient.Discover; results are cached per RepoKey.
func (c *Client) discover(ctx context.Context, repoURL string) (remoteInfo, error) {
	key := RepoKey(repoURL)
	c.mu.Lock()
	cached, ok := c.remotes[key]
	c.mu.Unlock()
	if ok {
		return cached, nil
	}

	base := strings.TrimRight(repoURL, "/")
	infoRefs := base + "/info/refs?service=git-upload-pack"
	req, err := http.NewRequest(http.MethodGet, infoRefs, nil)
	if err != nil {
		return remoteInfo{}, err
	}
	// Go forwards these headers on redirects, which lip enables explicitly
	// (without Git-Protocol the redirect target answers with protocol v0).
	req.Header.Set("Git-Protocol", gitProtocolHeader)
	req.Header.Set("User-Agent", UserAgent)
	resp, cancel, err := c.do(ctx, req, discoveryTimeout)
	if err != nil {
		return remoteInfo{}, err
	}
	defer cancel()
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return remoteInfo{}, &HTTPError{URL: infoRefs, StatusCode: resp.StatusCode}
	}

	// Follow redirects (e.g. renamed repositories) for subsequent POSTs.
	endpoint := base
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

	caps, err := readCapabilities(NewPktLineReader(resp.Body), repoURL)
	if err != nil {
		return remoteInfo{}, err
	}
	_, hasLsRefs := caps["ls-refs"]
	fetch, hasFetch := caps["fetch"]
	if !hasLsRefs || !hasFetch {
		return remoteInfo{}, protocolErrorf("Remote %s does not support ls-refs and fetch.", repoURL)
	}
	if format, ok := caps["object-format"]; ok && (format == nil || *format != "sha1") {
		f := ""
		if format != nil {
			f = *format
		}
		return remoteInfo{}, protocolErrorf("Remote %s uses unsupported object format %s.", repoURL, f)
	}

	info := remoteInfo{endpoint: endpoint}
	if fetch != nil {
		for _, f := range strings.Fields(*fetch) {
			switch f {
			case "filter":
				info.supportsFilter = true
			case "shallow":
				info.supportsShallow = true
			}
		}
	}
	c.mu.Lock()
	if c.remotes == nil {
		c.remotes = map[string]remoteInfo{}
	}
	c.remotes[key] = info
	c.mu.Unlock()
	return info, nil
}

// readCapabilities parses a v2 capability advertisement, skipping an
// optional "# service=..." line and flush. Values are nil for keys without
// "=".
func readCapabilities(r *PktLineReader, repoURL string) (map[string]*string, error) {
	typ, err := r.Read()
	if err != nil {
		return nil, err
	}
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
		return nil, protocolErrorf("Remote %s does not support git protocol version 2.", repoURL)
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
		return nil, protocolErrorf("Malformed capability advertisement from %s.", repoURL)
	}
	return caps, nil
}

// postCommand ports GitRemoteClient.PostCommand.
func (c *Client) postCommand(ctx context.Context, remote remoteInfo, body []byte, handle func(*PktLineReader) error) error {
	url := remote.endpoint + "/git-upload-pack"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Git-Protocol", gitProtocolHeader)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	resp, cancel, err := c.do(ctx, req, commandTimeout)
	if err != nil {
		return err
	}
	defer cancel()
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{URL: url, StatusCode: resp.StatusCode}
	}
	return handle(NewPktLineReader(resp.Body))
}

func parseRef(line string) (RemoteRef, error) {
	parts := strings.Split(line, " ")
	if len(parts) < 2 || !isObjectID(parts[0]) {
		return RemoteRef{}, protocolErrorf("Malformed ls-refs line '%s'.", line)
	}
	ref := RemoteRef{Name: parts[1], ObjectID: strings.ToLower(parts[0])}
	for _, p := range parts[2:] {
		if v, ok := strings.CutPrefix(p, "peeled:"); ok {
			ref.Peeled = strings.ToLower(v)
			break
		}
	}
	return ref, nil
}

func isObjectID(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func checkErrLine(line string) error {
	if rest, ok := strings.CutPrefix(line, "ERR "); ok {
		return protocolErrorf("Remote error: %s", rest)
	}
	return nil
}
