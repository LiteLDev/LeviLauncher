package gitproto

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
)

// TreeSource is a source backed by the tree of a remote git commit, porting
// lip 0.38.0-beta.1 src/Lip.Core/Sources/GitTreeSource.cs. The tree is
// listed once per instance (again after a failure) without downloading any
// blob, and only the blobs of the keys read are downloaded, never through a
// cache on disk: Open fetches its blob on every call unless Prefetch has
// downloaded it into this instance's memory. Blob content is verified
// against its object id.
//
// Keys are '/'-separated paths relative to the root tree. Only regular files
// (modes 100644/100755) are exposed, as in a git archive
// (protocol/package.md §1): symlinks, gitlinks and other modes are skipped.
//
// It satisfies archive.Source (Keys, Open, Size). Open uses the context the
// source was created with; OpenContext takes one.
type TreeSource struct {
	client    treeClient
	ctx       context.Context
	repoURL   string
	commitish string

	listMu  sync.Mutex // serializes listings
	mu      sync.Mutex
	entries *treeEntries
	// prefetched holds the blobs downloaded by Prefetch, by object id.
	prefetched map[string][]byte
}

// treeClient is the part of RemoteClient a TreeSource uses (lip's
// IGitRemoteClient).
type treeClient interface {
	ListTree(ctx context.Context, repoURL, commitish string) ([]TreeEntry, error)
	FetchBlobs(ctx context.Context, repoURL string, blobObjectIDs []string, onBlob func(objectID string, data []byte) error) error
}

// treeEntries are the regular files of a tree in tree order.
type treeEntries struct {
	keys  []string
	byKey map[string]string // key -> object id
}

// NewTreeSource returns a source for commitish (a commit or annotated tag
// id) of repoURL whose tree is listed on first access. ctx is used by Keys
// and Open.
func (c *RemoteClient) NewTreeSource(ctx context.Context, repoURL, commitish string) *TreeSource {
	return newTreeSource(c, ctx, repoURL, commitish)
}

func newTreeSource(c treeClient, ctx context.Context, repoURL, commitish string) *TreeSource {
	return &TreeSource{client: c, ctx: ctx, repoURL: repoURL, commitish: commitish, prefetched: map[string][]byte{}}
}

// CreateTreeSource is GitTreeSource.Create: it returns a source whose tree
// is listed up front, so a missing commit fails here rather than on first
// access, and Keys does not block. ctx is kept for Open.
func (c *RemoteClient) CreateTreeSource(ctx context.Context, repoURL, commitish string) (*TreeSource, error) {
	return createTreeSource(c, ctx, repoURL, commitish)
}

func createTreeSource(c treeClient, ctx context.Context, repoURL, commitish string) (*TreeSource, error) {
	s := newTreeSource(c, ctx, repoURL, commitish)
	if _, err := s.getEntries(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Keys returns the paths of the regular files in tree order. It lists the
// tree on first access unless it was listed already (CreateTreeSource);
// if that listing fails, Keys returns nil, like an empty tree.
func (s *TreeSource) Keys() []string {
	e, err := s.getEntries(s.ctx)
	if err != nil {
		return nil
	}
	return append([]string(nil), e.keys...)
}

// Size returns the size of a blob downloaded by Prefetch; git trees do not
// record sizes, so for other keys it reports false.
func (s *TreeSource) Size(key string) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		return 0, false
	}
	oid, ok := s.entries.byKey[key]
	if !ok {
		return 0, false
	}
	data, ok := s.prefetched[oid]
	return int64(len(data)), ok
}

// Open is OpenContext with the context the source was created with.
func (s *TreeSource) Open(key string) (io.ReadCloser, error) { return s.OpenContext(s.ctx, key) }

// OpenContext returns the content of key: from memory when Prefetch
// downloaded it, else downloaded now (and not kept).
func (s *TreeSource) OpenContext(ctx context.Context, key string) (io.ReadCloser, error) {
	oid, err := s.objectID(ctx, key)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	data, ok := s.prefetched[oid]
	s.mu.Unlock()
	if ok {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	blobs, err := s.fetchBlobs(ctx, []string{oid})
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(blobs[oid])), nil
}

// Prefetch downloads all blobs of the given keys that are not downloaded
// yet with a single FetchBlobs call (one request per 256 blobs) into
// memory, where Open and Size find them.
func (s *TreeSource) Prefetch(ctx context.Context, keys []string) error {
	e, err := s.getEntries(ctx)
	if err != nil {
		return err
	}
	ids := newOrderedSet()
	s.mu.Lock()
	for _, k := range keys {
		oid, ok := e.byKey[k]
		if !ok {
			s.mu.Unlock()
			return s.notInCommit(k)
		}
		if _, done := s.prefetched[oid]; !done {
			ids.add(oid)
		}
	}
	s.mu.Unlock()
	if ids.len() == 0 {
		return nil
	}

	blobs, err := s.fetchBlobs(ctx, ids.list())
	if err != nil {
		return err
	}
	s.mu.Lock()
	for oid, data := range blobs {
		if _, ok := s.prefetched[oid]; !ok {
			s.prefetched[oid] = data
		}
	}
	s.mu.Unlock()
	return nil
}

// fetchBlobs fetches the given blobs with a single FetchBlobs call and
// verifies each against its object id.
func (s *TreeSource) fetchBlobs(ctx context.Context, objectIDs []string) (map[string][]byte, error) {
	e, err := s.getEntries(ctx)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(objectIDs))
	for _, oid := range objectIDs {
		wanted[oid] = true
	}
	blobs := map[string][]byte{}
	err = s.client.FetchBlobs(ctx, s.repoURL, objectIDs, func(oid string, data []byte) error {
		if _, done := blobs[oid]; !wanted[oid] || done {
			return nil
		}
		if err := s.verifyBlob(oid, data, e); err != nil {
			return err
		}
		blobs[oid] = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	var missing []string
	for _, oid := range objectIDs {
		if _, ok := blobs[oid]; !ok {
			missing = append(missing, oid)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("The git server did not send %d of the files lip asked for from commit"+
			" %s in %s: %s. Try again later.", len(missing), s.commitish, s.repoURL,
			strings.Join(pathsOf(missing, e), ", "))
	}
	return blobs, nil
}

func (s *TreeSource) objectID(ctx context.Context, key string) (string, error) {
	e, err := s.getEntries(ctx)
	if err != nil {
		return "", err
	}
	oid, ok := e.byKey[key]
	if !ok {
		return "", s.notInCommit(key)
	}
	return oid, nil
}

// NotInCommitError is a key that is not a regular file of the commit (lip's
// ArgumentException); it matches fs.ErrNotExist.
type NotInCommitError struct {
	Key     string
	Message string
}

func (e *NotInCommitError) Error() string { return e.Message }

func (e *NotInCommitError) Is(target error) bool { return target == fs.ErrNotExist }

func (s *TreeSource) notInCommit(key string) error {
	return &NotInCommitError{Key: key, Message: fmt.Sprintf("There is no file '%s' in commit %s of %s.", key, s.commitish, s.repoURL)}
}

// pathsOf returns the quoted paths of the given blobs, or the object ids of
// blobs not in the tree.
func pathsOf(objectIDs []string, e *treeEntries) []string {
	out := make([]string, len(objectIDs))
	for i, oid := range objectIDs {
		out[i] = oid
		for _, k := range e.keys {
			if strings.EqualFold(e.byKey[k], oid) {
				out[i] = "'" + k + "'"
				break
			}
		}
	}
	return out
}

// getEntries lists the tree at most once per instance unless it fails, in
// which case the next access retries.
func (s *TreeSource) getEntries(ctx context.Context) (*treeEntries, error) {
	s.mu.Lock()
	e := s.entries
	s.mu.Unlock()
	if e != nil {
		return e, nil
	}

	s.listMu.Lock()
	defer s.listMu.Unlock()
	s.mu.Lock()
	e = s.entries
	s.mu.Unlock()
	if e != nil {
		return e, nil
	}

	tree, err := s.client.ListTree(ctx, s.repoURL, s.commitish)
	if err != nil {
		return nil, err
	}
	e = &treeEntries{byKey: map[string]string{}}
	for _, entry := range tree {
		if !entry.IsRegularFile() {
			continue
		}
		if _, dup := e.byKey[entry.Path]; dup {
			// Enumerable.ToDictionary rejects duplicate keys.
			return nil, &ArgumentError{Message: "An item with the same key has already been added. Key: " + entry.Path}
		}
		e.keys = append(e.keys, entry.Path)
		e.byKey[entry.Path] = entry.ObjectID
	}
	s.mu.Lock()
	s.entries = e
	s.mu.Unlock()
	return e, nil
}

// verifyBlob checks that the git object hash of the blob ("blob <size>\0" +
// content) matches its SHA-1 object id.
func (s *TreeSource) verifyBlob(objectID string, content []byte, e *treeEntries) error {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	actual := hex.EncodeToString(h.Sum(nil))
	if strings.EqualFold(actual, objectID) {
		return nil
	}
	return &CorruptBlobError{Message: fmt.Sprintf("The content of %s downloaded from commit"+
		" %s in %s is corrupt: its git object id is %s, but it should"+
		" be %s. The data was damaged on the way. Try again; if you use a GitHub"+
		" proxy, try without it ('lip config delete github_proxy'); if this keeps happening,"+
		" report it as a lip bug.", pathsOf([]string{objectID}, e)[0], s.commitish, s.repoURL, actual, objectID)}
}

// CorruptBlobError is a downloaded blob whose content does not match its
// object id (lip's InvalidDataException).
type CorruptBlobError struct{ Message string }

func (e *CorruptBlobError) Error() string { return e.Message }

// Version tags and refs (lip 0.38.0-beta.1 ArchiveProvider).

const headsRefPrefix = "refs/heads/"

// ListVersionTags lists the refs below "refs/tags/v" in the server's order.
// Name is the tag without "refs/tags/" (e.g. "v1.2.3"), Version the rest of
// the name after "v", unparsed (callers keep the tags whose Version is a
// strict SemVer 2.0 version, protocol/package.md §2.1), and Commit the
// peeled commit of an annotated tag, else the ref's object id.
func (c *RemoteClient) ListVersionTags(ctx context.Context, repoURL string) ([]TagRef, error) {
	refs, err := c.ListRefs(ctx, repoURL, []string{tagPrefix})
	if err != nil {
		return nil, err
	}
	tags := make([]TagRef, len(refs))
	for i, r := range refs {
		commit := r.Peeled
		if commit == "" {
			commit = r.ObjectID
		}
		tags[i] = TagRef{Name: r.Name[len(tagsRefPrefix):], Version: r.Name[len(tagPrefix):], Commit: commit}
	}
	return tags, nil
}

// RefKind is the kind of a ref of a remote repository (protocol/package.md
// §2.1).
type RefKind int

const (
	// RefVersion is a version, resolved as the tag "v<version>".
	RefVersion RefKind = iota
	// RefBranch is a branch name.
	RefBranch
	// RefTag is a tag name.
	RefTag
	// RefRev is a full commit id.
	RefRev
)

// RefNotFoundError is a branch or tag the repository does not have.
type RefNotFoundError struct {
	RepoURL string
	// RefName is the full ref name looked up, e.g. "refs/heads/main".
	RefName string
}

func (e *RefNotFoundError) Error() string {
	return fmt.Sprintf("The git repository %s has no %s.", e.RepoURL, e.RefName)
}

// ResolveRef resolves a ref to a commit id, like lip's
// ArchiveProvider.ResolveGitRef: a rev must be a full commit id and is
// returned lower-cased without asking the server (whether the commit
// exists is found out when its tree is listed, as lip does); a version,
// branch or tag is looked up with ls-refs by its exact full name, and an
// annotated tag is peeled. A ref the repository does not have is a
// RefNotFoundError.
func (c *RemoteClient) ResolveRef(ctx context.Context, repoURL string, kind RefKind, name string) (string, error) {
	var refName string
	switch kind {
	case RefRev:
		return normalizeObjectID(name)
	case RefVersion:
		refName = tagPrefix + name
	case RefBranch:
		refName = headsRefPrefix + name
	case RefTag:
		refName = tagsRefPrefix + name
	default:
		return "", fmt.Errorf("gitproto: unknown ref kind %d", kind)
	}

	refs, err := c.ListRefs(ctx, repoURL, []string{refName})
	if err != nil {
		return "", err
	}
	// ref-prefix also matches longer names (e.g. "main" matches "main-old").
	for _, r := range refs {
		if r.Name == refName {
			if r.Peeled != "" {
				return r.Peeled, nil
			}
			return r.ObjectID, nil
		}
	}
	return "", &RefNotFoundError{RepoURL: repoURL, RefName: refName}
}

// OpenCommit returns a TreeSource for a full commit id, with its tree listed
// up front (lip's ArchiveProvider.OpenCommit).
func (c *RemoteClient) OpenCommit(ctx context.Context, repoURL, commit string) (*TreeSource, error) {
	normalized, err := normalizeObjectID(commit)
	if err != nil {
		return nil, err
	}
	return c.CreateTreeSource(ctx, repoURL, normalized)
}
