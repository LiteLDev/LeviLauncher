package engine

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/compat"
	"github.com/liteldev/LeviLauncher/internal/lipcore/graph"
	"github.com/liteldev/LeviLauncher/internal/lipcore/install"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// Ports src/Lip.Core/Mcmod/Resolution/Resolver.cs and the Candidate,
// AssetHandle and Package records of Contracts.cs and Packages.cs
// (v0.38.0-beta.1).

// environment is LipEnvironment: the (platform, runtime) of a workspace.
type environment struct {
	Platform string
	Runtime  string
}

// versionRange is a VersionRange as a constraint of the solver: its text, as
// written, names it.
type versionRange struct{ r semver.VersionRange }

func (v versionRange) Contains(x semver.Version) bool { return v.r.Contains(x) }
func (v versionRange) String() string                 { return v.r.String() }
func (v versionRange) Key() string                    { return "range:" + v.r.String() }

// assetHandle is AssetHandle: an asset of a candidate, opened on first use.
type assetHandle struct {
	urls []string
	self bool
	once sync.Once
	open func() (archive.Source, error)
	src  archive.Source
	err  error
}

func (a *assetHandle) Open() (archive.Source, error) {
	a.once.Do(func() { a.src, a.err = a.open() })
	return a.src, a.err
}

// candidate is Candidate (protocol/package.md §4): a package with its
// selected entry, the archive it is in, the source it was resolved for, and
// the assets of its entry, in order. It implements graph.Candidate.
type candidate struct {
	id       string
	variant  string
	version  semver.Version
	entry    *pkgmodel.Entry
	manifest any // the package file the workspace file records
	archive  *archiveHandle
	source   ref.Ref
	gsource  graph.Ref
	assets   []*assetHandle
	deps     []graph.Dependency

	modOnce sync.Once
	modName string
	modErr  error
}

func (c *candidate) Identity() string                 { return ref.Identity(c.id, c.variant) }
func (c *candidate) Version() semver.Version          { return c.version }
func (c *candidate) Variant() string                  { return c.variant }
func (c *candidate) Commit() string                   { return c.archive.Commit }
func (c *candidate) ArchiveKey() string               { return c.archive.Key }
func (c *candidate) Source() graph.Ref                { return c.gsource }
func (c *candidate) Derived() bool                    { return c.entry.Derived }
func (c *candidate) Dependencies() []graph.Dependency { return c.deps }

// ModName is GetModManifest: the mod name of a mod, read from the
// manifest.json of its asset of type "mcmod" (obtaining it), or "" for a
// package that is not a mod. The result is kept.
func (c *candidate) ModName(ctx context.Context) (string, error) {
	index := c.entry.McmodAsset()
	if index < 0 {
		return "", nil
	}
	c.modOnce.Do(func() {
		label := c.Identity() + " " + c.version.String()
		src, err := c.assets[index].Open()
		if err != nil {
			if isContextError(err) {
				c.modErr = err
				return
			}
			c.modErr = &install.Error{Msg: fmt.Sprintf("Cannot install %s: lip could not read the %s of"+
				" its asset of type \"mcmod\" to tell the mod name.", label, pkgmodel.ModManifestFileName), Err: err}
			return
		}
		c.modName, c.modErr = install.ReadModName(src, label)
	})
	return c.modName, c.modErr
}

// installPackage is the candidate as the installer takes it.
func (c *candidate) installPackage() *install.Package {
	return &install.Package{
		ID:      c.id,
		Variant: c.variant,
		Version: c.version.String(),
		Entry:   c.entry,
		Archive: c.archive.Source,
		OpenAsset: func(urls []string) (archive.Source, error) {
			for _, a := range c.assets {
				if !a.self && slices.Equal(a.urls, urls) {
					return a.Open()
				}
			}
			return nil, fmt.Errorf("internal error: the entry of %s has no asset with the URLs %v", c.Identity(), urls)
		},
		ModName: c.ModName,
	}
}

// newCandidate is Candidate.Create: the candidate of a package in an
// archive, whose assets are downloaded when first needed.
func (r *resolver) newCandidate(id, variant string, version semver.Version, entry *pkgmodel.Entry,
	manifest any, a *archiveHandle, source ref.Ref) *candidate {
	c := &candidate{
		id: id, variant: variant, version: version, entry: entry, manifest: manifest,
		archive: a, source: source, gsource: r.graphRef(source),
	}
	for _, asset := range entry.Assets {
		h := &assetHandle{urls: asset.URLs}
		if asset.Type == pkgmodel.AssetSelf {
			h.self = true
			h.open = func() (archive.Source, error) { return a.Source, nil }
		} else {
			urls := asset.URLs
			h.open = func() (archive.Source, error) { return r.provider.OpenAsset(r.ctx, urls) }
		}
		c.assets = append(c.assets, h)
	}
	for _, d := range entry.Deps {
		dep, err := ref.Parse(d.Key, false)
		if err != nil {
			continue // the package file parser accepts only valid keys
		}
		rng, err := semver.ParseVersionRange(d.Range)
		if err != nil {
			continue
		}
		c.deps = append(c.deps, graph.Dependency{
			Source: r.graphRef(dep), Range: versionRange{rng}, ClientCompat: entry.Derived})
	}
	return c
}

// resolver is Resolver: Resolve(source, range, environment) of
// protocol/package.md §4, reading an archive without a package file as
// protocol/compatibility.md §2 does. It caches archives, version lists,
// package files and ref commits for one operation.
type resolver struct {
	ctx       context.Context
	provider  archiveProvider
	env       environment
	onWarning func(string)
	// onRead is told the key of every archive whose package file is read.
	onRead func(key string)

	warnedMu sync.Mutex
	warned   map[string]bool

	archives  memo[*archiveHandle]
	versions  memo[[]specVersion]
	documents memo[*packageDocument]
	// commits of refs ("<folded spec>@branch:name"), so that a branch
	// resolves to one commit for the whole operation.
	commits memo[string]

	refsMu sync.Mutex
	refs   map[string]ref.Ref
}

func newResolver(ctx context.Context, p archiveProvider, env environment, onWarning func(string)) *resolver {
	if onWarning == nil {
		onWarning = func(string) {}
	}
	return &resolver{ctx: ctx, provider: p, env: env, onWarning: onWarning,
		warned: map[string]bool{}, refs: map[string]ref.Ref{}}
}

// graphRef is a source as the solver sees it; the resolver remembers the
// source for its text.
func (r *resolver) graphRef(s ref.Ref) graph.Ref {
	text := s.String()
	r.refsMu.Lock()
	r.refs[text] = s
	r.refsMu.Unlock()
	return graph.Ref{Text: text, Spec: s.Kind == ref.KindSpec, Pinned: s.GitRef != nil,
		Target: s.Target, Variant: s.Variant}
}

func (r *resolver) sourceOf(g graph.Ref) (ref.Ref, error) {
	r.refsMu.Lock()
	s, ok := r.refs[g.Text]
	r.refsMu.Unlock()
	if ok {
		return s, nil
	}
	return ref.Parse(g.Text, true)
}

// PinCommit makes a version ref resolve to the commit a workspace recorded
// for it, as updating it gives the same commit even when its tag has moved
// since (protocol/workspace.md §5, "Updating").
func (r *resolver) PinCommit(source ref.Ref, commit string) {
	if source.Kind != ref.KindSpec || source.GitRef == nil || source.GitRef.Kind != ref.GitRefVersion {
		return
	}
	commit = strings.ToLower(commit)
	key := ref.Fold(source.Target) + "@" + source.GitRef.String()
	_, _ = r.commits.do(key, func() (string, error) { return commit, nil })
}

// ResolveLazy implements graph.Resolver.
func (r *resolver) ResolveLazy(ctx context.Context, source graph.Ref, rng graph.Constraint, clientCompat bool,
	onDiagnostic func(graph.Diagnostic)) graph.Sequence {
	s, err := r.sourceOf(source)
	if err != nil {
		return &errSequence{err}
	}
	return r.resolve(s, rng, clientCompat, onDiagnostic)
}

type errSequence struct{ err error }

func (e *errSequence) Next(context.Context) (graph.Candidate, bool, error) { return nil, false, e.err }

// walk holds the parameters shared by a whole Resolve call.
type walk struct {
	source       ref.Ref
	rng          graph.Constraint
	clientCompat bool
	sink         func(graph.Diagnostic)
}

func (w *walk) report(archiveName, reason string, cause error) {
	if w.sink != nil {
		w.sink(graph.Diagnostic{Message: archiveName + ": " + reason, Cause: cause})
	}
}

// resolve is ResolveLazyWithCauses: the candidate sequence of a source,
// produced lazily; every path that produces no candidate is reported.
func (r *resolver) resolve(source ref.Ref, rng graph.Constraint, clientCompat bool,
	onDiagnostic func(graph.Diagnostic)) *sequence {
	return &sequence{r: r, w: walk{source: source, rng: rng, clientCompat: clientCompat, sink: onDiagnostic}}
}

// sequence is the lazily produced candidate sequence of one Resolve call.
type sequence struct {
	r        *resolver
	w        walk
	started  bool
	done     bool
	matching []specVersion
	next     int
}

func (s *sequence) Next(ctx context.Context) (graph.Candidate, bool, error) {
	c, err := s.nextCandidate(ctx)
	if err != nil || c == nil {
		return nil, false, err
	}
	return c, true, nil
}

func (s *sequence) nextCandidate(ctx context.Context) (*candidate, error) {
	if s.done {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r, w := s.r, &s.w
	source := w.source
	if !s.started {
		s.started = true
		// Step 1: the archives the source names.
		if source.Kind != ref.KindSpec {
			s.done = true
			plain, err := source.WithVariant("")
			if err != nil {
				return nil, err
			}
			a, err := r.archives.do(source.Target, func() (*archiveHandle, error) {
				return r.provider.Open(r.ctx, plain)
			})
			if err != nil {
				if !isUnavailable(err) {
					return nil, err
				}
				w.report(source.String(), err.Error(), err)
				return nil, nil
			}
			return r.resolveArchive(w, a, nil)
		}
		if source.GitRef != nil {
			s.done = true
			return r.resolvePinned(w, *source.GitRef)
		}

		versions, err := r.versions.do(source.Target, func() ([]specVersion, error) {
			return r.provider.ListVersions(r.ctx, source.Target)
		})
		if err != nil {
			s.done = true
			if !isUnavailable(err) {
				return nil, err
			}
			w.report(source.String(), err.Error(), err)
			return nil, nil
		}
		for _, v := range versions {
			if w.rng == nil || w.rng.Contains(v.Version) {
				s.matching = append(s.matching, v)
			}
		}
		if len(s.matching) == 0 {
			s.done = true
			if len(versions) == 0 {
				w.report(source.String(), "the repository has no version tags (tags named like 'v1.2.3'),"+
					" so it has no version to install; a branch, tag or commit can be named with"+
					" '@branch:<name>', '@tag:<name>' or '@rev:<commit>'", nil)
			} else {
				w.report(source.String(), fmt.Sprintf("no version tag satisfies %s; the repository has the"+
					" versions %s", w.rng, listVersions(versions)), nil)
			}
			return nil, nil
		}
	}

	for s.next < len(s.matching) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		v := s.matching[s.next]
		s.next++
		a, err := r.archives.do(source.Target+"@"+v.Tag, func() (*archiveHandle, error) {
			return r.provider.OpenVersion(r.ctx, source.Target, v)
		})
		if err != nil {
			if !isUnavailable(err) {
				return nil, err
			}
			w.report(source.Target+"@"+v.Tag, err.Error(), err)
			continue
		}
		version := v.Version
		c, err := r.resolveArchive(w, a, &version)
		if err != nil || c != nil {
			return c, err
		}
	}
	s.done = true
	return nil, nil
}

// resolvePinned is step 1 for a remote repository with a ref
// (protocol/package.md §2.1): the single archive of the commit the ref
// selects. Only a version tag gives it a tag version.
func (r *resolver) resolvePinned(w *walk, gitRef ref.GitRef) (*candidate, error) {
	source := w.source
	commit, err := r.commits.do(ref.Fold(source.Target)+"@"+gitRef.String(), func() (string, error) {
		return r.provider.ResolveGitRef(r.ctx, source.Target, gitRef)
	})
	var a *archiveHandle
	if err == nil {
		if gitRef.Kind == ref.GitRefVersion {
			// The archive of a version ref is named like that of the same
			// version without a ref.
			a, err = r.archives.do(source.Target+"@"+gitRef.String()+" "+commit, func() (*archiveHandle, error) {
				h, err := r.provider.OpenCommit(r.ctx, source.Target, commit)
				if err != nil {
					return nil, err
				}
				named := *h
				named.Key = source.Target + "@" + gitRef.String()
				return &named, nil
			})
		} else {
			a, err = r.archives.do(source.Target+"@rev:"+commit, func() (*archiveHandle, error) {
				return r.provider.OpenCommit(r.ctx, source.Target, commit)
			})
		}
	}
	if err != nil {
		if !isUnavailable(err) {
			return nil, err
		}
		w.report(source.String(), err.Error(), err)
		return nil, nil
	}

	var tagVersion *semver.Version
	switch gitRef.Kind {
	case ref.GitRefVersion:
		if v, err := semver.ParseStrict(gitRef.Name); err == nil {
			tagVersion = &v
		}
	case ref.GitRefTag:
		if v, ok := semver.ParseVersionTag(semver.TagsRefPrefix + gitRef.Name); ok {
			tagVersion = &v
		}
	}
	return r.resolveArchive(w, a, tagVersion)
}

// resolveArchive is step 2 for one archive: the candidate of the package,
// or nil.
func (r *resolver) resolveArchive(w *walk, a *archiveHandle, tagVersion *semver.Version) (*candidate, error) {
	source := w.source
	d, err := r.documents.do(a.Key, func() (*packageDocument, error) {
		if r.onRead != nil {
			r.onRead(a.Key)
		}
		return readArchive(a)
	})
	if err != nil {
		if !isUnavailable(err) {
			return nil, err
		}
		w.report(a.Key, err.Error(), err)
		return nil, nil
	}

	// protocol/package.md §2.1: the git archive of a remote repository is a
	// package of its own package id, whose version is the tag version, if any.
	fileName := d.fileName()
	if source.Kind == ref.KindSpec && !ref.NamesEqual(d.id(), source.Target) {
		w.report(a.Key, fmt.Sprintf("its %s declares the package id %s, not %s, so it is not a package"+
			" of this repository", fileName, d.id(), source.Target), nil)
		return nil, nil
	}

	version := d.version
	if tagVersion != nil && !version.Equal(*tagVersion) {
		if !d.isLegacy() {
			w.report(a.Key, fmt.Sprintf("its %s declares the version %s, not the tag version %s; the"+
				" package author must make them match", fileName, version, tagVersion), nil)
			return nil, nil
		}
		// protocol/compatibility.md §2: the version of a package read from a
		// tooth.json is its tag version.
		if semver.ComparePrecedence(version, *tagVersion) != 0 && r.warnOnce(a.Key) {
			r.onWarning(fmt.Sprintf("%s: its tooth.json declares the version %s, but it is tagged as"+
				" version %s; using the tag version %s.", a.Key, version, tagVersion, tagVersion))
		}
		version = *tagVersion
	}

	pf, err := d.readFor(r.env.Platform, r.env.Runtime, &version)
	if err != nil {
		if _, ok := err.(*compat.FormatError); !ok {
			return nil, err
		}
		w.report(a.Key, err.Error(), err)
		return nil, nil
	}

	// protocol/compatibility.md §2: a dependency of a tooth.json on the
	// variant "client" of a package that has a package file names its
	// variant "".
	variant := source.Variant
	if w.clientCompat && variant == compat.ClientLabel && !d.isLegacy() {
		variant = ""
	}

	entry := pf.SelectEntry(variant, r.env.Platform, r.env.Runtime)
	if entry == nil {
		r.reportNoEntry(w, a, pf, fileName, variant)
		return nil, nil
	}

	if w.rng != nil && !w.rng.Contains(version) {
		w.report(a.Key, fmt.Sprintf("version %s does not satisfy %s", version, w.rng), nil)
		return nil, nil
	}

	return r.newCandidate(pf.ID, variant, version, entry, pf.Raw, a, source), nil
}

func (r *resolver) warnOnce(key string) bool {
	r.warnedMu.Lock()
	defer r.warnedMu.Unlock()
	if r.warned[key] {
		return false
	}
	r.warned[key] = true
	return true
}

// reportNoEntry reports that a package file (named fileName) has no entry
// of a variant for the environment, with the entries it has instead.
func (r *resolver) reportNoEntry(w *walk, a *archiveHandle, pf *pkgmodel.Document, fileName, variant string) {
	env := r.env
	variantName := "the default variant"
	if variant != "" {
		variantName = fmt.Sprintf("the variant \"%s\"", variant)
	}
	wanted := fmt.Sprintf("no entry of %s for the platform %s and the runtime %s", variantName,
		env.Platform, env.Runtime)
	entries, ok := pf.EntriesOf(variant)
	if !ok || len(entries) == 0 {
		if len(pf.Variants) == 0 {
			w.report(a.Key, fmt.Sprintf("its %s has %s; it has no entries at all", fileName, wanted), nil)
			return
		}
		names := make([]string, len(pf.Variants))
		for i, v := range pf.Variants {
			if v.Name == "" {
				names[i] = "the default variant"
			} else {
				names[i] = "\"" + v.Name + "\""
			}
		}
		w.report(a.Key, fmt.Sprintf("its %s has %s; its variants are %s", fileName, wanted,
			strings.Join(names, ", ")), nil)
		return
	}

	available := make([]string, len(entries))
	samePlatform := false
	for i, e := range entries {
		available[i] = fmt.Sprintf("(%s, %s)", e.Platform, e.Runtime)
		if e.Platform == env.Platform {
			samePlatform = true
		}
	}
	// An entry for another runtime of the same platform: the package does not
	// support the runtime lip installs for. Whether --rid can help depends on
	// the workspace, which is not known here, so the advice is conditional.
	runtimeNote := ""
	if samePlatform {
		runtimeNote = fmt.Sprintf("; it does not support the runtime %s (if the server runs another"+
			" runtime and the workspace has no packages yet, pass it with --rid)", env.Runtime)
	}
	w.report(a.Key, fmt.Sprintf("its %s has %s; it has entries for %s%s", fileName, wanted,
		strings.Join(available, ", "), runtimeNote), nil)
}

// listVersions is the versions of a version list, newest first, cut short
// when long.
func listVersions(versions []specVersion) string {
	const shown = 10
	var distinct []string
	seen := map[string]bool{}
	for _, v := range versions {
		s := v.Version.String()
		if !seen[s] {
			seen[s] = true
			distinct = append(distinct, s)
		}
	}
	text := strings.Join(distinct[:min(shown, len(distinct))], ", ")
	if len(distinct) > shown {
		text += fmt.Sprintf(" and %d older ones", len(distinct)-shown)
	}
	return text + " (newest first)"
}
