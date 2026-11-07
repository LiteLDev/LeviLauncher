package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// Ports src/Lip.Core/Mcmod/Archives.cs and Archives/ArchiveProvider.cs
// (v0.38.0-beta.1).

const headsRefPrefix = "refs/heads/"

// archiveHandle is ArchiveHandle: an opened archive, a key naming it in
// messages and caches, its file tree, and, for a git archive, the commit
// (lowercase hex) it is the tree of.
type archiveHandle struct {
	Key    string
	Source archive.Source
	Commit string
}

// specVersion is SpecVersion: a version of a remote repository, its version
// tag "v" + version.
type specVersion struct {
	Version semver.Version
	Tag     string
}

// archiveProvider is IArchiveProvider: obtains archives of sources and
// assets (protocol/package.md §2, §4 step 1). Failures to obtain an archive
// are *archive.UnavailableError (ArchiveUnavailableException); other
// errors (cancellation, failures of lip itself) are not.
type archiveProvider interface {
	// Open opens the archive of a URL or a local path.
	Open(ctx context.Context, target ref.Ref) (*archiveHandle, error)
	// ListVersions lists the version tags of a remote repository, highest
	// precedence first, at equal precedence the ordinally greater tag first.
	ListVersions(ctx context.Context, spec string) ([]specVersion, error)
	// OpenVersion opens the git archive of one version; its key is
	// "spec@version".
	OpenVersion(ctx context.Context, spec string, version specVersion) (*archiveHandle, error)
	// ResolveGitRef resolves a ref of a remote repository to a commit id.
	ResolveGitRef(ctx context.Context, spec string, gitRef ref.GitRef) (string, error)
	// OpenCommit opens the git archive of a commit; its key is
	// "spec@rev:commit".
	OpenCommit(ctx context.Context, spec, commit string) (*archiveHandle, error)
	// OpenAsset downloads the ZIP archive of an asset from the first of its
	// URLs that can be obtained.
	OpenAsset(ctx context.Context, urls []string) (archive.Source, error)
	// Forget forgets the cached download of the archive of a URL.
	Forget(url string) error
}

// gitRemote is what ArchiveProvider needs of IGitRemoteClient: ls-refs, and
// the tree of a commit as a source whose tree is listed up front
// (GitTreeSource.Create). key names the archive for progress.
type gitRemote interface {
	ListRefs(ctx context.Context, repoURL string, prefixes []string) ([]gitproto.RemoteRef, error)
	OpenTree(ctx context.Context, repoURL, commit, key string) (archive.Source, error)
}

// repoURL is ArchiveProvider.GetRepoUrl: the git repository URL of a remote
// repository, without any GitHub proxy applied.
func repoURL(spec string) string { return "https://" + spec + ".git" }

// taggedVersion is a version tag and the commit it points to.
type taggedVersion struct {
	Version specVersion
	Commit  string
}

// provider is ArchiveProvider.
type provider struct {
	git    gitRemote
	cache  *fetch.HTTPCache
	proxy  string
	client *http.Client

	tags memo[[]taggedVersion]
}

func newProvider(git gitRemote, cache *fetch.HTTPCache, proxy string, client *http.Client) *provider {
	return &provider{git: git, cache: cache, proxy: proxy, client: client}
}

func (p *provider) Open(ctx context.Context, target ref.Ref) (*archiveHandle, error) {
	switch target.Kind {
	case ref.KindURL:
		return p.openURL(ctx, target.Target)
	case ref.KindFile:
		path, err := target.LocalPath()
		if err != nil {
			return nil, err
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return openDirectory(target.Target, path)
		}
		return openFile(target.Target, path)
	default:
		return nil, fmt.Errorf("Internal error: %s is a remote repository, which lip opens by version or"+
			" commit, not as a URL or local path.", target)
	}
}

func (p *provider) ResolveGitRef(ctx context.Context, spec string, gitRef ref.GitRef) (string, error) {
	if gitRef.Kind == ref.GitRefRev {
		return gitRef.Name, nil
	}
	repo := repoURL(spec)
	var refName, what string
	switch gitRef.Kind {
	case ref.GitRefVersion:
		refName = semver.VersionTagRefPrefix + gitRef.Name
		what = fmt.Sprintf("version %s (tag v%s)", gitRef.Name, gitRef.Name)
	case ref.GitRefBranch:
		refName = headsRefPrefix + gitRef.Name
		what = fmt.Sprintf("branch '%s'", gitRef.Name)
	default:
		refName = semver.TagsRefPrefix + gitRef.Name
		what = fmt.Sprintf("tag '%s'", gitRef.Name)
	}
	refs, err := p.git.ListRefs(ctx, repo, []string{refName})
	if err != nil {
		if !isUnavailableCause(err) {
			return "", err
		}
		return "", unavailablef(err, "Could not look up the %s of %s (%s).", what, spec, repo)
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
	return "", unavailablef(nil, "%s has no %s. Check the spelling, and that it exists in the repository %s.",
		spec, what, repo)
}

func (p *provider) OpenCommit(ctx context.Context, spec, commit string) (*archiveHandle, error) {
	g, err := ref.RevGitRef(commit)
	if err != nil {
		return nil, err
	}
	return p.openGit(ctx, spec+"@rev:"+g.Name, spec, g.Name)
}

func (p *provider) ListVersions(ctx context.Context, spec string) ([]specVersion, error) {
	tagged, err := p.taggedVersions(ctx, spec)
	if err != nil {
		return nil, err
	}
	out := make([]specVersion, len(tagged))
	for i, t := range tagged {
		out[i] = t.Version
	}
	return out, nil
}

func (p *provider) OpenVersion(ctx context.Context, spec string, version specVersion) (*archiveHandle, error) {
	tagged, err := p.taggedVersions(ctx, spec)
	if err != nil {
		return nil, err
	}
	for _, t := range tagged {
		if t.Version.Tag == version.Tag {
			return p.openGit(ctx, spec+"@"+version.Version.String(), spec, strings.ToLower(t.Commit))
		}
	}
	return nil, unavailablef(nil, "%s has no version tag %s (any more). Check the version.", spec, version.Tag)
}

func (p *provider) Forget(url string) error { return p.cache.Forget(url) }

func (p *provider) OpenAsset(ctx context.Context, urls []string) (archive.Source, error) {
	var opened string
	path, err := p.cache.DownloadZipNamed(ctx, urls, p.proxy, p.client, validateZip)
	if err != nil {
		return nil, archiveError(err)
	}
	// The URL whose download it is names it, as lip's ZipSource does.
	for _, u := range urls {
		if base, _, _ := strings.Cut(u, "#"); p.cache.Path(fetch.HTTPKey(base)) == path {
			opened = u
			break
		}
	}
	return archive.NewZipFileSource(path, fmt.Sprintf("the download of %s (%s)", opened, path))
}

func (p *provider) openURL(ctx context.Context, url string) (*archiveHandle, error) {
	path, err := p.cache.DownloadZipURLNamed(ctx, url, p.proxy, p.client, validateZip)
	if err != nil {
		return nil, archiveError(err)
	}
	src, err := archive.NewZipFileSource(path, fmt.Sprintf("the download of %s (%s)", url, path))
	if err != nil {
		return nil, err
	}
	return &archiveHandle{Key: url, Source: src}, nil
}

// validateZip is ZipValidator.Validate of a file of the cache.
func validateZip(path, subject string) error { return archive.ValidateZipFile(path, subject) }

// openDirectory opens a local directory.
func openDirectory(key, path string) (*archiveHandle, error) {
	full, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	src, err := archive.NewDirectorySource(full)
	if err != nil {
		return nil, err
	}
	return &archiveHandle{Key: key, Source: src}, nil
}

// openFile opens a local ZIP file, read once into memory.
func openFile(key, path string) (*archiveHandle, error) {
	src, err := archive.OpenLocalZip(path)
	if err != nil {
		return nil, err
	}
	return &archiveHandle{Key: key, Source: src}, nil
}

func (p *provider) openGit(ctx context.Context, key, spec, commit string) (*archiveHandle, error) {
	repo := repoURL(spec)
	src, err := p.git.OpenTree(ctx, repo, commit, key)
	if err != nil {
		if !isUnavailableCause(err) {
			return nil, err
		}
		return nil, unavailablef(err, "Could not get the files of %s (the commit %s of %s).", key, commit, repo)
	}
	return &archiveHandle{Key: key, Source: src, Commit: commit}, nil
}

// taggedVersions is GetTaggedVersions: the version tags of a repository,
// listed once (failures are not kept, so later calls retry).
func (p *provider) taggedVersions(ctx context.Context, spec string) ([]taggedVersion, error) {
	return p.tags.do(spec, func() ([]taggedVersion, error) { return p.listTaggedVersions(ctx, spec) })
}

func (p *provider) listTaggedVersions(ctx context.Context, spec string) ([]taggedVersion, error) {
	repo := repoURL(spec)
	refs, err := p.git.ListRefs(ctx, repo, []string{semver.VersionTagRefPrefix})
	if err != nil {
		if !isUnavailableCause(err) {
			return nil, err
		}
		return nil, unavailablef(err, "Could not list the versions of %s (the tags of %s).", spec, repo)
	}
	var versions []taggedVersion
	for _, r := range refs {
		v, ok := semver.ParseVersionTag(r.Name)
		if !ok {
			continue
		}
		commit := r.Peeled
		if commit == "" {
			commit = r.ObjectID
		}
		versions = append(versions, taggedVersion{
			Version: specVersion{Version: v, Tag: r.Name[len(semver.TagsRefPrefix):]},
			Commit:  commit,
		})
	}
	slices.SortStableFunc(versions, func(a, b taggedVersion) int {
		return semver.CompareTagVersions(
			semver.TagVersion{Version: a.Version.Version, Tag: a.Version.Tag},
			semver.TagVersion{Version: b.Version.Version, Tag: b.Version.Tag})
	})
	return versions, nil
}

// unavailablef is a new ArchiveUnavailableException.
func unavailablef(cause error, format string, args ...any) error {
	return &archive.UnavailableError{Message: fmt.Sprintf(format, args...), Err: cause}
}

// archiveError makes the ArchiveUnavailableException of package fetch one of
// package archive, which the installer and the resolver look for.
func archiveError(err error) error {
	if fe, ok := err.(*fetch.UnavailableError); ok {
		return &archive.UnavailableError{Message: fe.Message, Err: fe.Err}
	}
	return err
}

// isUnavailable reports whether err is an ArchiveUnavailableException: an
// archive or an asset that cannot be obtained or is invalid.
func isUnavailable(err error) bool {
	switch err.(type) {
	case *archive.UnavailableError, *archive.NotPackageError, *fetch.UnavailableError:
		return true
	}
	return false
}

// isUnavailableCause is ArchiveProvider.IsUnavailable: everything but
// cancellation and ArchiveUnavailableException makes an archive unavailable.
func isUnavailableCause(err error) bool {
	return !isContextError(err) && !isUnavailable(err)
}

// contextErrorOf is the context error in err's chain, or nil.
func contextErrorOf(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	}
	return nil
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// memo shares one call per key; failures are not kept, so that a transient
// failure is retried by the next caller (Resolver.Cached).
type memo[T any] struct {
	mu sync.Mutex
	m  map[string]*memoEntry[T]
}

type memoEntry[T any] struct {
	done chan struct{}
	val  T
	err  error
}

func (c *memo[T]) do(key string, fn func() (T, error)) (T, error) {
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string]*memoEntry[T]{}
	}
	if e, ok := c.m[key]; ok {
		c.mu.Unlock()
		<-e.done
		return e.val, e.err
	}
	e := &memoEntry[T]{done: make(chan struct{})}
	c.m[key] = e
	c.mu.Unlock()
	ok := false
	defer func() {
		// A failure (or a panic) is not kept, and waiters never hang.
		if !ok || e.err != nil {
			if !ok {
				e.err = fmt.Errorf("internal error: the computation of %q did not finish", key)
			}
			c.mu.Lock()
			if c.m[key] == e {
				delete(c.m, key)
			}
			c.mu.Unlock()
		}
		close(e.done)
	}()
	e.val, e.err = fn()
	ok = true
	return e.val, e.err
}

// remoteGit is the gitRemote of lip: gitproto.RemoteClient, reporting the
// packs it receives as git_fetch bars.
type remoteGit struct {
	client *gitproto.RemoteClient
	rep    report.Reporter
	ids    func() string
}

func (g *remoteGit) ListRefs(ctx context.Context, repoURL string, prefixes []string) ([]gitproto.RemoteRef, error) {
	return g.client.ListRefs(ctx, repoURL, prefixes)
}

// OpenTree lists the tree of the commit up front (GitTreeSource.Create): the
// pack of commits and trees is a bar "Listing files of <key>"; the packs of
// the blobs read later are bars "Fetching files of <key>".
func (g *remoteGit) OpenTree(ctx context.Context, repoURL, commit, key string) (archive.Source, error) {
	t := &treeSource{key: key, git: g}
	t.message.Store(ptr("Listing files of " + key))
	src, err := g.client.CreateTreeSource(gitproto.WithPackProgress(ctx, t.progress(&t.message)), repoURL, commit)
	if err != nil {
		return nil, err
	}
	t.message.Store(ptr("Fetching files of " + key))
	t.TreeSource = src
	return t, nil
}

// treeSource is a git archive: a gitproto.TreeSource whose blob fetches
// report progress.
type treeSource struct {
	*gitproto.TreeSource
	key     string
	git     *remoteGit
	message atomic.Pointer[string]
}

// Prefetch downloads the blobs of keys, reporting the packs received.
func (t *treeSource) Prefetch(ctx context.Context, keys []string) error {
	var msg atomic.Pointer[string]
	msg.Store(ptr("Fetching files of " + t.key))
	return t.TreeSource.Prefetch(gitproto.WithPackProgress(ctx, t.progress(&msg)), keys)
}

func ptr[T any](v T) *T { return &v }

// progress reports each pack request (through the proxy, directly, every
// batch) as its own git_fetch bar with a fresh id, so that a bar never goes
// backwards: objects received out of the pack header's count, indeterminate
// until the header arrives or when the pack cannot be scanned (the message
// then counts the bytes received). gitproto ends every attempt with one Done
// update.
func (t *treeSource) progress(message *atomic.Pointer[string]) func(gitproto.PackProgress) {
	rep := report.OrNop(t.git.rep)
	var mu sync.Mutex
	attempt := 0
	var id string
	return func(p gitproto.PackProgress) {
		mu.Lock()
		defer mu.Unlock()
		if p.Attempt != attempt {
			attempt, id = p.Attempt, t.git.ids()
		}
		msg := *message.Load()
		if p.Total <= 0 && p.Bytes > 0 {
			msg = fmt.Sprintf("%s (%d bytes received)", msg, p.Bytes)
		}
		reportBar(rep, report.Progress{
			ID: id, Stage: report.StageGitFetch, Package: t.key, Message: msg,
			Current: p.Objects, Total: p.Total, Unit: report.UnitObjects, Item: p.URL, Done: p.Done,
		}, p.Err != nil, false)
	}
}
