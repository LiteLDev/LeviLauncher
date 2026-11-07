// Package gitproto is a minimal git smart-HTTP protocol v2 client, porting
// lip 0.38.0-beta.1 src/Lip.Core/Git/GitRemoteClient.cs, GithubProxy.cs,
// PktLine.cs and src/Lip.Core/Sources/GitTreeSource.cs (RemoteClient,
// TreeSource).
//
// Packfile decoding lives in package gitpack.
package gitproto

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
)

// UserAgent is sent with every request, as lip does.
const UserAgent = "git/2.0 (lip)"

// MaxWantsPerRequest is the largest number of wants one fetch request
// sends; FetchBlobs batches larger sets.
const MaxWantsPerRequest = 256

const (
	gitProtocolHeader = "version=2"
	// discoveryTimeout is Flurl's default request timeout (100 s).
	discoveryTimeout = 100 * time.Second
	// commandTimeout is lip's timeout for upload-pack POSTs.
	commandTimeout = 30 * time.Minute

	tagsRefPrefix = "refs/tags/"
	tagPrefix     = tagsRefPrefix + "v"
)

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
	// Version is the rest of the tag name after "v", unparsed.
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

// PackProgress is measured progress of receiving one fetched pack.
type PackProgress struct {
	// Attempt numbers the pack requests made with one WithPackProgress
	// context, from 1 (see WithPackProgress). Callers show
	// each attempt as its own bar; no value decreases within an attempt.
	Attempt int
	// URL is the repository URL of the attempt (the proxied URL when the
	// proxy is used).
	URL string
	// Objects is the number of pack entries received completely.
	Objects int64
	// Total is the object count of the pack header: 0 until the header has
	// arrived, and 0 again (indeterminate, with Objects frozen) when the
	// stream could not be scanned.
	Total int64
	// Bytes is the number of pack bytes received (side-band channel 1).
	Bytes int64
	// Done marks the attempt's last update; Err is the attempt's error,
	// if it failed (or was canceled).
	Done bool
	Err  error
}

// packTracker turns the bytes of one fetch attempt into PackProgress: it
// counts them and feeds them to a gitpack.StreamScanner, which counts the
// entries received. Updates are serialized by mu (the scanner reports from
// its own goroutine) and throttled: object counts at most once per 0.1% of
// the total, byte counts (while no object total is known) once per 64 KiB.
// When the scanner fails the fetch goes on; only the object total is
// dropped.
type packTracker struct {
	fn      func(PackProgress)
	scanner *gitpack.StreamScanner

	mu        sync.Mutex
	p         PackProgress
	failed    bool  // the scanner failed: Total stays 0
	aborting  bool  // the attempt failed: the scanner's error is ignored
	done      bool  // the Done update was sent
	lastBytes int64 // Bytes at the last update
}

// packProgressByteStep is the byte step of updates without an object total.
const packProgressByteStep = 64 << 10

func newPackTracker(fn func(PackProgress), attempt int, url string) *packTracker {
	t := &packTracker{fn: fn, p: PackProgress{Attempt: attempt, URL: url}}
	t.fn(t.p)
	t.scanner = gitpack.NewStreamScanner(t.scanned)
	return t
}

// scanned receives the scanner's counts.
func (t *packTracker) scanned(sp gitpack.StreamProgress) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done || t.failed {
		return
	}
	if sp.Err != nil && t.aborting {
		// The cut stream, not an unscannable pack.
		return
	}
	if sp.Err != nil {
		t.failed = true
		t.p.Total = 0
		t.emitLocked()
		return
	}
	prev := t.p
	t.p.Objects, t.p.Total = sp.Objects, sp.Total
	if prev.Total == 0 || sp.Objects == sp.Total || sp.Objects*1000/sp.Total > prev.Objects*1000/sp.Total {
		t.emitLocked()
	}
}

// Write counts p and passes it to the scanner. It never fails.
func (t *packTracker) Write(p []byte) (int, error) {
	t.mu.Lock()
	t.p.Bytes += int64(len(p))
	if t.p.Total == 0 && t.p.Bytes-t.lastBytes >= packProgressByteStep {
		t.emitLocked()
	}
	t.mu.Unlock()
	// Outside mu: the scanner's callback takes it.
	return t.scanner.Write(p)
}

func (t *packTracker) emitLocked() {
	t.lastBytes = t.p.Bytes
	t.fn(t.p)
}

// finish stops the scanner and sends the Done update. Both Close and Abort
// wait until the scanner has parsed every byte already written (io.Pipe
// hands each Write to the scanner before returning), so the last update
// deterministically counts all bytes received. A failed attempt keeps the
// counts it reached: the scanner's error about the cut stream is ignored.
func (t *packTracker) finish(err error) {
	if err != nil {
		t.mu.Lock()
		t.aborting = true
		t.mu.Unlock()
		t.scanner.Abort(err)
	} else {
		_ = t.scanner.Close()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.done = true
	t.p.Done, t.p.Err = true, err
	t.emitLocked()
}

// PackFile is a fetched packfile in a temporary file that is deleted when
// closed (lip opens it with FileOptions.DeleteOnClose).
type PackFile struct {
	f      *os.File
	remove bool
	size   int64
	once   sync.Once
	err    error
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
// packfile-uris) are skipped. An ERR line or side-band channel 3 is a
// ServerError.
func ReadPackfile(r *PktLineReader, w io.Writer) error {
	for {
		typ, err := r.Read()
		if err != nil {
			return err
		}
		if typ != PktData {
			return protocolErrorf("Unexpected %s packet in the fetch response; expected a section header.", typ)
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
			return protocolErrorf("The fetch response ended after section '%s' without a packfile section.", header)
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
			return protocolErrorf("Unexpected %s packet in the packfile section.", typ)
		}
		payload := r.Payload
		if len(payload) == 0 {
			return protocolErrorf("Empty side-band packet in the packfile section.")
		}
		switch payload[0] {
		case 1:
			if _, err := w.Write(payload[1:]); err != nil {
				return err
			}
		case 2:
		case 3:
			return &ServerError{ServerMessage: gitpack.TrimSpace(gitpack.DecodeUTF8(payload[1:]))}
		default:
			return protocolErrorf("Invalid side-band channel %d in the packfile section.", payload[0])
		}
	}
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

func classifyNetError(parent, reqCtx context.Context, url string, err error, body bool) error {
	if perr := parent.Err(); perr != nil {
		return perr
	}
	if errors.Is(reqCtx.Err(), context.DeadlineExceeded) {
		return &timeoutError{url: url}
	}
	return &transportError{err: err, body: body}
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
		err = classifyNetError(b.parent, b.reqCtx, b.url, err, true)
	}
	return n, err
}

// Close drains a bounded remainder so the connection can be reused.
func (b *classifyingBody) Close() error {
	_, _ = io.CopyN(io.Discard, b.rc, 64<<10)
	return b.rc.Close()
}

func parseRef(line string) (RemoteRef, error) {
	parts := strings.Split(line, " ")
	if len(parts) < 2 || !isObjectID(parts[0]) {
		return RemoteRef{}, protocolErrorf("Malformed ls-refs line '%s'; expected an object id followed by a ref name.", line)
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

// checkErrLine is lip's ThrowIfError.
func checkErrLine(line string) error {
	if rest, ok := strings.CutPrefix(line, "ERR "); ok {
		return &ServerError{ServerMessage: rest}
	}
	return nil
}
