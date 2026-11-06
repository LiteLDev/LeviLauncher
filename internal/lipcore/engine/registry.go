package engine

import (
	"context"

	"github.com/liteldev/LeviLauncher/internal/lipcore/engine/workspace"
	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// GitClient is the subset of gitproto.Client the registry uses.
type GitClient interface {
	ListTags(ctx context.Context, repoURL string) ([]gitproto.TagRef, error)
	Fetch(ctx context.Context, repoURL string, wants []string, opts gitproto.FetchOptions) ([]byte, error)
}

// GitRegistry is lip's GitRemotePackageRegistry + SourceService:
// versions come from tags of https://<path>.git; a version's manifest is the
// root tooth.json of the tag commit (migrated with migrate.ToV3, parsed,
// tooth/version overwritten with a warning when they differ from the
// request); "self" assets are the tag's git tree (blobs fetched lazily in
// batches and cached as git-blob:<oid>, SHA-1 verified); other assets are
// downloaded through Downloader (tried URL by URL) and opened with
// fetch.OpenArchive. Results are memoized per process.
type GitRegistry struct {
	Git        GitClient
	Cache      *fetch.Cache
	Downloader *fetch.Downloader
	Reporter   report.Reporter
	RID        string
	// Implementation fields (memo caches, hooks) may be added.
}

// NewGitRegistry wires a registry with the real gitproto client, the shared
// lip cache and the given GitHub proxy prefix (may be empty).
func NewGitRegistry(githubProxy string, r report.Reporter) *GitRegistry {
	panic("engine: not implemented")
}

func (g *GitRegistry) Versions(ctx context.Context, id manifest.PackageID) ([]string, error) {
	return nil, ErrNotImplemented
}

func (g *GitRegistry) Manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error) {
	return nil, ErrNotImplemented
}

func (g *GitRegistry) OpenAsset(ctx context.Context, spec manifest.PackageSpec, asset manifest.Asset) (workspace.Source, error) {
	return nil, ErrNotImplemented
}
