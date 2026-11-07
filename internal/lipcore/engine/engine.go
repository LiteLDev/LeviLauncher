package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/compat"
	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/graph"
	"github.com/liteldev/LeviLauncher/internal/lipcore/httpclient"
	"github.com/liteldev/LeviLauncher/internal/lipcore/install"
	"github.com/liteldev/LeviLauncher/internal/lipcore/lipdirs"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// Options configure an Engine.
type Options struct {
	// GithubProxy is the github_proxy setting (protocol/userspace.md §2);
	// "" means none. The caller resolves the configuration and any override.
	GithubProxy string
	// HTTPClient carries git requests and downloads; nil is the default
	// client with httpclient.CheckRedirect.
	HTTPClient *http.Client
	// Platform is the platform of the workspace; "" is "client", the
	// platform of LeviLauncher's workspaces.
	Platform string
	// Runtime overrides the runtime of the environment (lip's --rid); ""
	// is the runtime the workspace file records, else CurrentRID().
	Runtime string
}

// InstalledPackage is a package of the workspace file.
type InstalledPackage struct {
	// Identity is the package id, then '#' and the variant when not "".
	Identity string
	ID       string
	Variant  string
	Version  string
	// Explicit is whether a request named it (as opposed to a dependency).
	Explicit bool
	// Ref is the source it was installed from: the source of the request or
	// the key of the dependency it was resolved for.
	Ref string
}

// Engine is McmodService (src/Lip.Core/Mcmod/McmodService.cs,
// v0.38.0-beta.1) for one workspace: install, update, uninstall and list
// (protocol/workspace.md §3-§7, protocol/cli.md §5).
type Engine struct {
	// Dir is the absolute workspace root.
	Dir string

	opts     Options
	platform string
	rep      report.Reporter
	client   *http.Client
	cache    *fetch.HTTPCache
	git      gitRemote
	inst     *install.Installer
	// newID returns the id of a new progress bar.
	newID func() string
}

// New returns an engine for the workspace dir.
func New(dir string, opts Options, r report.Reporter) *Engine {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	rep := report.OrNop(r)
	client := httpclient.WithLipRedirects(opts.HTTPClient)
	platform := opts.Platform
	if platform == "" {
		platform = pkgmodel.PlatformClient
	}
	e := &Engine{
		Dir:      abs,
		opts:     opts,
		platform: platform,
		rep:      rep,
		client:   client,
		cache:    fetch.DefaultHTTPCache(rep),
		inst:     &install.Installer{Reporter: rep},
		newID:    newGUID,
	}
	e.git = &remoteGit{
		client: &gitproto.RemoteClient{HTTP: client, GithubProxy: opts.GithubProxy, TempDir: lipdirs.CacheDir()},
		rep:    rep,
		ids:    func() string { return e.newID() },
	}
	return e
}

// HTTPClient is the client of the engine's git requests and downloads.
func (e *Engine) HTTPClient() *http.Client { return e.client }

// Close releases the engine. It must not be used afterwards.
func (e *Engine) Close() error { return nil }

// CurrentRID returns the .NET runtime identifier of this process
// (RuntimeInformation.RuntimeIdentifier), e.g. "win-x64", "linux-arm64".
func CurrentRID() string {
	goos := runtime.GOOS
	switch goos {
	case "windows":
		goos = "win"
	case "darwin":
		goos = "osx"
	}
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x64"
	case "386":
		arch = "x86"
	}
	return goos + "-" + arch
}

// newGUID returns a random version 4 GUID in .NET "D" format.
func newGUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func (e *Engine) info(msg string)    { e.rep.Info(msg) }
func (e *Engine) warning(msg string) { e.rep.Warning(msg) }

// operation is OperationOptions.
type operation struct {
	dryRun, ignoreScripts, noDependencies bool
}

// request is a package argument and the request it makes (PackageRequest,
// whose range is always "any version" for a package argument).
type request struct {
	arg    string
	source ref.Ref
}

// workspace is McmodService.Workspace: what an operation works on.
type workspace struct {
	root     string
	lock     *pkgmodel.LockFile
	env      environment
	provider archiveProvider
	resolver *resolver
}

// newProvider returns the archive provider of one operation.
func (e *Engine) newProvider() archiveProvider {
	return newProvider(e.git, e.cache, e.opts.GithubProxy, e.client)
}

// Install is McmodService.Install (protocol/cli.md §5.1).
func (e *Engine) Install(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error {
	op := operation{dryRun: dryRun, ignoreScripts: ignoreScripts, noDependencies: noDependencies}
	release, err := e.beginOperation(op)
	if err != nil {
		return err
	}
	defer release()
	ws, err := e.openWorkspace(ctx)
	if err != nil {
		return err
	}
	requests, err := e.parseRequests(packages)
	if err != nil {
		return err
	}

	if noDependencies {
		return e.installWithoutDependencies(ctx, ws, requests, op)
	}

	candidates, err := e.requestedCandidates(ctx, ws, requests, false)
	if err != nil {
		return err
	}
	for i, c := range candidates {
		identity := c.Identity()
		for j := range ws.lock.Packages {
			p := &ws.lock.Packages[j]
			if p.Explicit && ref.NamesEqual(install.LockedIdentity(p), identity) {
				return &Error{Msg: fmt.Sprintf("Cannot install '%s': package '%s' is already explicitly"+
					" installed (version %s). To change its version, run 'lip update %s' instead.",
					requests[i].arg, identity, install.LockedVersion(p), requests[i].arg),
					kind: ErrAlreadyExplicitlyInstalled}
			}
		}
	}

	unpinned := make([]string, len(candidates))
	for i, c := range candidates {
		unpinned[i] = c.Identity()
	}
	p, err := e.makePlan(ctx, ws, requests, unpinned, explicitIdentities(ws.lock))
	if err != nil {
		return err
	}
	return e.execute(ctx, ws, op, e.changes(ws, op, p))
}

// Uninstall is McmodService.Uninstall (protocol/cli.md §5.2).
func (e *Engine) Uninstall(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error {
	op := operation{dryRun: dryRun, ignoreScripts: ignoreScripts, noDependencies: noDependencies}
	release, err := e.beginOperation(op)
	if err != nil {
		return err
	}
	defer release()
	ws, err := e.openWorkspace(ctx)
	if err != nil {
		return err
	}

	var targets []int
	for _, arg := range packages {
		matched, err := e.matchInstalled(ws, arg)
		if err != nil {
			return err
		}
		if len(matched) == 0 {
			return &Error{Msg: fmt.Sprintf("Package '%s' is not installed in this workspace, so it cannot be"+
				" uninstalled. Run 'lip list' to see the installed packages and their identities.", arg)}
		}
		if !noDependencies {
			for _, i := range matched {
				if p := &ws.lock.Packages[i]; !p.Explicit {
					return &Error{Msg: fmt.Sprintf("Package '%s' was installed as a dependency of other"+
						" packages, not requested by you, so lip uninstalls it by itself once no package"+
						" needs it. To uninstall it anyway, pass --no-dependencies (the packages that need"+
						" it may then stop working).", install.LockedIdentity(p))}
				}
			}
		}
		for _, i := range matched {
			if !slices.Contains(targets, i) {
				targets = append(targets, i)
			}
		}
	}

	if noDependencies {
		e.warning("--no-dependencies is enabled. This operation may result in a broken workspace.")
		var changes []change
		for i := len(ws.lock.Packages) - 1; i >= 0; i-- {
			if slices.Contains(targets, i) {
				changes = append(changes, change{kind: uninstallChange, origin: i})
			}
		}
		return e.execute(ctx, ws, op, changeSet{changes: changes})
	}

	removed := map[string]bool{}
	for _, i := range targets {
		removed[ref.Fold(install.LockedIdentity(&ws.lock.Packages[i]))] = true
	}
	var explicit []string
	for _, id := range explicitIdentities(ws.lock) {
		if !removed[ref.Fold(id)] {
			explicit = append(explicit, id)
		}
	}
	p, err := e.makePlan(ctx, ws, nil, nil, explicit)
	if err != nil {
		return err
	}
	for _, kept := range p.members {
		if removed[ref.Fold(kept.Identity())] {
			e.info(fmt.Sprintf("Package '%s' is still required by other packages; keeping it as a dependency.",
				kept.Identity()))
		}
	}
	return e.execute(ctx, ws, op, e.changes(ws, op, p))
}

// Update is McmodService.Update (protocol/cli.md §5.3).
func (e *Engine) Update(ctx context.Context, packages []string, dryRun, ignoreScripts bool) error {
	op := operation{dryRun: dryRun, ignoreScripts: ignoreScripts}
	release, err := e.beginOperation(op)
	if err != nil {
		return err
	}
	defer release()
	ws, err := e.openWorkspace(ctx)
	if err != nil {
		return err
	}

	var requests []request
	if len(packages) == 0 {
		// Every explicitly installed package, requested again by its source: a
		// remote repository at its newest version, a branch at its head, a tag
		// where it points now, a rev as it is, a URL downloaded again and a
		// local path with its current content.
		for i := range ws.lock.Packages {
			p := &ws.lock.Packages[i]
			if !p.Explicit {
				continue
			}
			source, err := parseRecordedSource(p)
			if err != nil {
				return err
			}
			if source.Kind == ref.KindFile {
				path, err := source.LocalPath()
				if err != nil {
					return err
				}
				if _, err := os.Stat(path); err != nil {
					e.warning(fmt.Sprintf("Keeping package '%s' as it is: %s, which it was installed from,"+
						" does not exist any more.", install.LockedIdentity(p), path))
					continue
				}
			}
			requests = append(requests, request{arg: install.LockedIdentity(p), source: source})
		}
	} else {
		if requests, err = e.parseRequests(packages); err != nil {
			return err
		}
	}

	// A URL is downloaded again (protocol/workspace.md §5 "Updating").
	for _, r := range requests {
		if r.source.Kind == ref.KindURL {
			if err := ws.provider.Forget(r.source.Target); err != nil {
				return err
			}
		}
	}

	candidates, err := e.requestedCandidates(ctx, ws, requests, true)
	if err != nil {
		return err
	}
	var identities []string
	var requested []request
	for i, c := range candidates {
		identity := c.Identity()
		installed := false
		for j := range ws.lock.Packages {
			if ref.NamesEqual(install.LockedIdentity(&ws.lock.Packages[j]), identity) {
				installed = true
				break
			}
		}
		if !installed {
			return &Error{Msg: fmt.Sprintf("Cannot update '%s': package '%s' is not installed in this"+
				" workspace. To install it, run 'lip install %s'.", requests[i].arg, identity, requests[i].arg)}
		}
		if containsIdentity(identities, identity) {
			continue
		}
		identities = append(identities, identity)
		requested = append(requested, requests[i])
	}

	p, err := e.makePlan(ctx, ws, requested, identities, explicitIdentities(ws.lock))
	if err != nil {
		return err
	}
	return e.execute(ctx, ws, op, e.changes(ws, op, p))
}

// List returns the packages of the workspace file as "<identity>@<version>",
// in installation order, explicitly installed packages and dependency
// packages apart (protocol/cli.md §5.4; LipClient.List).
func (e *Engine) List(ctx context.Context) (explicit, implicit []string, err error) {
	packages, err := e.Packages(ctx)
	if err != nil {
		return nil, nil, err
	}
	explicit, implicit = []string{}, []string{}
	for _, p := range packages {
		s := p.Identity + "@" + p.Version
		if p.Explicit {
			if !slices.Contains(explicit, s) {
				explicit = append(explicit, s)
			}
		} else if !slices.Contains(implicit, s) {
			implicit = append(implicit, s)
		}
	}
	return explicit, implicit, nil
}

// Packages is McmodService.List: the packages of the workspace file. The
// workspace file derived from a tooth_lock.json is only read, for the
// environment a workspace operation would give it, without the commits it
// would record.
func (e *Engine) Packages(ctx context.Context) ([]InstalledPackage, error) {
	lf, exists, err := pkgfile.LoadWorkspace(e.Dir)
	if err != nil {
		return nil, err
	}
	if !exists {
		// DetermineEnvironment(null, null): list takes no runtime override.
		env := environment{Platform: e.platform, Runtime: CurrentRID()}
		if lf, err = e.readToothLock(ctx, e.Dir, env, nil, false); err != nil {
			return nil, err
		}
	}
	out := []InstalledPackage{}
	if lf == nil {
		return out, nil
	}
	for i := range lf.Packages {
		p := &lf.Packages[i]
		out = append(out, InstalledPackage{
			Identity: install.LockedIdentity(p),
			ID:       stringMember(p.Manifest, "id"),
			Variant:  p.Variant,
			Version:  install.LockedVersion(p),
			Explicit: p.Explicit,
			Ref:      p.Ref,
		})
	}
	return out, nil
}

// CacheClean deletes lip's cache (protocol/cli.md §5.9), reporting one
// cache_clean bar: entries deleted out of the entries counted by a walk
// done before deleting, ended with Done whether the clean succeeded or not.
func (e *Engine) CacheClean(ctx context.Context) error {
	var bar *report.Progress
	err := e.cache.CleanProgress(ctx, func(done, total int64) {
		if bar == nil {
			bar = &report.Progress{ID: "cache_clean", Stage: report.StageCacheClean,
				Message: "Cleaning the download cache", Unit: report.UnitFiles}
		}
		bar.Current, bar.Total = done, total
		report.Report(e.rep, *bar)
	})
	if bar != nil {
		bar.Done = true
		report.Report(e.rep, *bar)
	}
	return err
}

// #region Planning

// openWorkspace is OpenWorkspace.
func (e *Engine) openWorkspace(ctx context.Context) (*workspace, error) {
	root := e.Dir
	recorded, exists, err := pkgfile.LoadWorkspace(root)
	if err != nil {
		return nil, err
	}
	if !exists {
		recorded = nil
	}
	env, err := e.determineEnvironment(recorded)
	if err != nil {
		return nil, err
	}
	p := e.newProvider()
	if recorded == nil {
		if recorded, err = e.readToothLock(ctx, root, env, p, true); err != nil {
			return nil, err
		}
	}
	// A workspace without a workspace file gets the environment of its first
	// operation.
	lf := recorded
	if lf == nil {
		lf = &pkgmodel.LockFile{Format: pkgmodel.Format, Platform: env.Platform, Runtime: env.Runtime}
	}
	res := newResolver(ctx, p, env, e.warning)
	for i := range lf.Packages {
		pkg := &lf.Packages[i]
		if pkg.Commit == "" {
			continue
		}
		if source, _, ok := ref.TryParse(pkg.Ref, true); ok {
			res.PinCommit(source, pkg.Commit)
		}
	}
	return &workspace{root: root, lock: lf, env: env, provider: p, resolver: res}, nil
}

// readToothLock is ReadToothLock: the workspace file
// protocol/compatibility.md §3 derives from the tooth_lock.json of a
// workspace without a workspace file, for its environment; nil when it has
// none that Section 3 reads. With resolveCommits, the commit of each package
// is that of the tag "v<version>" of its remote repository, and it fails
// when that cannot be obtained.
func (e *Engine) readToothLock(ctx context.Context, root string, env environment, p archiveProvider,
	resolveCommits bool) (*pkgmodel.LockFile, error) {
	tl, ok := compat.ReadToothLock(root)
	if !ok {
		return nil, nil
	}
	var commitOf compat.CommitOf
	if resolveCommits {
		e.info("Migrating " + compat.ToothLockFileName + " of lip v0.x to the workspace file.")
		commitOf = func(repo, tag string) (string, error) {
			g, _, ok := ref.TryParseGitRef(strings.TrimPrefix(tag, "v"))
			if !ok {
				return "", nil
			}
			commit, err := p.ResolveGitRef(ctx, repo, g)
			if err != nil {
				if isUnavailable(err) {
					return "", nil
				}
				return "", err
			}
			return commit, nil
		}
	}
	return compat.DeriveWorkspace(tl, root, env.Platform, env.Runtime, commitOf)
}

// beginOperation is BeginOperation: takes the workspace lock, then finishes
// or undoes what an interrupted operation left in the journal. A dry run
// writes nothing, so it takes no lock and only reports the journal.
func (e *Engine) beginOperation(op operation) (release func(), err error) {
	root := e.Dir
	if op.dryRun {
		if err := e.recoverInterrupted(root, op); err != nil {
			return nil, err
		}
		return func() {}, nil
	}
	if err := txn.EnsureLipDir(root); err != nil {
		return nil, err
	}
	mutex, err := txn.Lock(root)
	if err != nil {
		return nil, err
	}
	if err := e.recoverInterrupted(root, op); err != nil {
		mutex.Close()
		return nil, err
	}
	return func() { mutex.Close() }, nil
}

func (e *Engine) recoverInterrupted(root string, op operation) error {
	if !txn.HasJournal(root) {
		return nil
	}
	journal := filepath.Join(root, txn.LipDirectory, txn.JournalFile)
	if op.dryRun {
		e.warning("An interrupted lip operation left changes in this workspace, which the next install," +
			" uninstall or update finishes or undoes. The dry run plans with the workspace as it is now.")
		return nil
	}

	result, err := txn.RecoverWith(root, pkgfile.RecoverHooks(root))
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}
	started := ""
	if result.Started != "" {
		started = fmt.Sprintf(" (it started at %s)", result.Started)
	}
	var errs strings.Builder
	for _, err := range result.Errors {
		errs.WriteString("\n  - " + err.Error())
	}
	if result.Committed {
		e.info(fmt.Sprintf("Finished an interrupted lip operation that had completed%s: deleted the"+
			" backups it left.", started))
		if len(result.Errors) > 0 {
			e.warning("Some of them could not be deleted:" + errs.String())
		}
		return nil
	}

	if len(result.Errors) > 0 {
		// The errors behind the list, so that the report and the hints see them.
		var cause error = result.Errors[0]
		if len(result.Errors) > 1 {
			cause = &fetch.AggregateError{Message: "The changes that could not be undone.", Errs: result.Errors}
		}
		return &Error{Msg: fmt.Sprintf("A lip operation on this workspace was interrupted%s, and lip"+
			" cannot undo all of its changes yet:%s\nFix the problem (e.g. stop the server and other"+
			" programs using these files), then run lip again; it retries the rest. Do not delete"+
			" '%s': it records what is left to undo.", started, errs.String(), journal), Err: cause}
	}

	e.warning(fmt.Sprintf("A lip operation on this workspace was interrupted%s; undid its %d changes."+
		" The workspace and its workspace file are as they were before it.", started, result.Changes))
	return nil
}

// determineEnvironment is DetermineEnvironment (protocol/cli.md §4): the
// platform of the engine, which a workspace file must record when it has
// one; the runtime is the override, then the workspace file's, then the RID
// the engine runs on.
func (e *Engine) determineEnvironment(lf *pkgmodel.LockFile) (environment, error) {
	if lf != nil && lf.Platform != e.platform {
		var msg string
		if e.platform == pkgmodel.PlatformServer {
			msg = fmt.Sprintf("The workspace file %s records the platform \"%s\" in its \"platform\" field,"+
				" but lip manages server workspaces only (\"%s\"). Client workspaces are managed by"+
				" launchers such as LeviLauncher.", pkgmodel.WorkspaceFile, lf.Platform, e.platform)
		} else {
			msg = fmt.Sprintf("The workspace file %s records the platform \"%s\" in its \"platform\" field,"+
				" but LeviLauncher manages %s workspaces only (\"%s\"). Server workspaces are managed"+
				" by lip.", pkgmodel.WorkspaceFile, lf.Platform, e.platform, e.platform)
		}
		return environment{}, &Error{Msg: msg}
	}
	runtime := e.opts.Runtime
	if runtime == "" && lf != nil {
		runtime = lf.Runtime
	}
	if runtime == "" {
		runtime = CurrentRID()
	}
	return environment{Platform: e.platform, Runtime: runtime}, nil
}

// parseRequests parses package arguments; local paths are relative to the
// workspace root.
func (e *Engine) parseRequests(packages []string) ([]request, error) {
	out := make([]request, 0, len(packages))
	for _, p := range packages {
		source, err := ref.ParsePackageArgument(p, e.Dir)
		if err != nil {
			return nil, err
		}
		out = append(out, request{arg: p, source: source})
	}
	return out, nil
}

// requestedCandidates is RequestedCandidates: the first candidate of every
// request. Fails when a request has no candidate, or, unless
// allowSameIdentity, when two requests name the same package
// (protocol/cli.md §5.1).
func (e *Engine) requestedCandidates(ctx context.Context, ws *workspace, requests []request,
	allowSameIdentity bool) ([]*candidate, error) {
	var bar *engineBar
	if len(requests) > 0 {
		bar = e.newBar(report.StageResolve, report.UnitPackages, "Resolving packages", int64(len(requests)))
		defer bar.done()
	}
	var out []*candidate
	for _, r := range requests {
		bar.update(int64(len(out)), r.arg)
		c, err := e.firstCandidate(ctx, ws, r)
		if err != nil {
			return nil, err
		}
		identity := c.Identity()
		earlier := -1
		for i, o := range out {
			if ref.NamesEqual(o.Identity(), identity) {
				earlier = i
				break
			}
		}
		if !allowSameIdentity && earlier >= 0 {
			return nil, &Error{Msg: fmt.Sprintf("The arguments '%s' and '%s' both name the package '%s'."+
				" Request each package once.", requests[earlier].arg, r.arg, identity)}
		}
		out = append(out, c)
	}
	bar.update(int64(len(out)), "")
	return out, nil
}

func (e *Engine) firstCandidate(ctx context.Context, ws *workspace, r request) (*candidate, error) {
	e.info("Resolving " + r.arg + "...")
	var diagnostics []graph.Diagnostic
	seq := ws.resolver.resolve(r.source, nil, false, func(d graph.Diagnostic) {
		diagnostics = append(diagnostics, d)
	})
	c, err := seq.nextCandidate(ctx)
	if err != nil {
		return nil, err
	}
	if c != nil {
		return c, nil
	}

	const shown = 10
	summary := fmt.Sprintf("No package found for '%s' that can be installed in this workspace (%s, %s).",
		r.arg, ws.env.Platform, ws.env.Runtime)
	message := summary
	if len(diagnostics) > 0 {
		message += " Every place lip looked failed:"
		for _, d := range diagnostics[:min(shown, len(diagnostics))] {
			message += "\n  - " + d.Message
		}
	}
	if len(diagnostics) > shown {
		message += fmt.Sprintf("\n  ... and %d more", len(diagnostics)-shown)
	}
	// The errors behind the diagnostics are kept as causes, so that the
	// failure report shows them and their hints apply.
	var causes []error
	for _, d := range diagnostics {
		if d.Cause != nil {
			causes = append(causes, d.Cause)
		}
	}
	if len(causes) == 0 {
		// When the package was found, the reasons above say what is wrong;
		// spelling advice fits only when nothing (or a repository without
		// versions, maybe the wrong one) was found.
		notFound := true
		for _, d := range diagnostics {
			if !strings.Contains(d.Message, "has no version tags") {
				notFound = false
				break
			}
		}
		if notFound {
			message += "\nCheck the spelling of the package and the version you asked for."
		}
		return nil, &Error{Msg: message}
	}
	return nil, &Error{Msg: message, Err: &fetch.AggregateError{Message: summary, Errs: causes}}
}

// plan is McmodService.Plan: the new package set in installation order, and
// the folded identities of its explicitly installed members.
type plan struct {
	members  []*candidate
	explicit map[string]bool
}

// makePlan is MakePlan.
func (e *Engine) makePlan(ctx context.Context, ws *workspace, requests []request, unpinned, explicitIdentities []string) (*plan, error) {
	unpinnedSet := foldSet(unpinned)
	explicitSet := foldSet(explicitIdentities)

	var pinnedAll []graph.Candidate
	for i := range ws.lock.Packages {
		p := &ws.lock.Packages[i]
		if unpinnedSet[ref.Fold(install.LockedIdentity(p))] {
			continue
		}
		c, err := e.pinnedCandidate(ctx, ws, p)
		if err != nil {
			return nil, err
		}
		pinnedAll = append(pinnedAll, c)
	}
	greqs := make([]graph.Request, len(requests))
	for i, r := range requests {
		greqs[i] = graph.Request{Source: ws.resolver.graphRef(r.source)}
	}

	solution, err := e.solve(ctx, ws, greqs, pinnedAll)
	if withInstalled, ok := err.(*graph.ResolutionError); ok {
		var pinnedExplicit []graph.Candidate
		for _, c := range pinnedAll {
			if explicitSet[ref.Fold(c.Identity())] {
				pinnedExplicit = append(pinnedExplicit, c)
			}
		}
		if len(pinnedExplicit) == len(pinnedAll) {
			return nil, err
		}
		lines := strings.Split(withInstalled.Message, "\n")
		reason := strings.TrimRight(withInstalled.Message, ".")
		if len(lines) > 1 {
			reason = strings.Trim(lines[1], " -:.")
		}
		e.info(fmt.Sprintf("The installed dependency packages do not fit the request (%s); choosing their"+
			" versions again.", reason))
		solution, err = e.solve(ctx, ws, greqs, pinnedExplicit)
	}
	if err != nil {
		return nil, err
	}

	// Keep what the requests and the explicitly installed packages need.
	needed := map[int]bool{}
	var pending []int
	for _, i := range solution.RequestSatisfiers {
		if i >= 0 {
			pending = append(pending, i)
		}
	}
	for i, c := range solution.Order {
		if explicitSet[ref.Fold(c.Identity())] {
			pending = append(pending, i)
		}
	}
	for len(pending) > 0 {
		index := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if !needed[index] {
			needed[index] = true
			pending = append(pending, solution.Dependencies[index]...)
		}
	}

	finalExplicit := map[string]bool{}
	for k := range explicitSet {
		finalExplicit[k] = true
	}
	for _, i := range solution.RequestSatisfiers {
		if i >= 0 {
			finalExplicit[ref.Fold(solution.Order[i].Identity())] = true
		}
	}
	p := &plan{explicit: finalExplicit}
	for i, c := range solution.Order {
		if needed[i] {
			p.members = append(p.members, c.(*candidate))
		}
	}
	return p, nil
}

// solve runs the graph solver under an indeterminate resolve bar counting
// the package files read.
func (e *Engine) solve(ctx context.Context, ws *workspace, requests []graph.Request, pinned []graph.Candidate) (*graph.Solution, error) {
	bar := e.newBar(report.StageResolve, report.UnitPackages, "Resolving dependencies", 0)
	defer bar.done()
	read := int64(0)
	ws.resolver.onRead = func(key string) {
		read++
		bar.update(read, key)
	}
	defer func() { ws.resolver.onRead = nil }()
	return graph.Solve(ctx, graph.Problem{Requests: requests, Pinned: pinned, Resolver: ws.resolver})
}

// #endregion

// #region Applying

type changeKind int

const (
	uninstallChange changeKind = iota
	installChange
	markExplicitChange
)

// change is a Change: uninstalling or marking an installed package (origin,
// its index in the workspace file), or installing a candidate.
type change struct {
	kind      changeKind
	origin    int
	candidate *candidate
	explicit  bool
}

// changeSet is ChangeSet: what an operation changes, in order, and how the
// workspace file entries left at the end are ordered and marked (finish;
// nil keeps them as they are).
type changeSet struct {
	changes []change
	finish  func(remaining []pkgmodel.LockedPackage) []pkgmodel.LockedPackage
}

// changes is Changes: from the installed packages to the planned package
// set, uninstalling what leaves or changes (in reverse installation order),
// then installing what enters (in installation order).
func (e *Engine) changes(ws *workspace, op operation, p *plan) changeSet {
	members := map[string]*candidate{}
	for _, c := range p.members {
		members[ref.Fold(c.Identity())] = c
	}

	// The identities of the packages that stay exactly as installed.
	kept := map[string]bool{}
	for i := range ws.lock.Packages {
		pkg := &ws.lock.Packages[i]
		key := ref.Fold(install.LockedIdentity(pkg))
		if m, ok := members[key]; ok && isSamePackage(pkg, m) {
			kept[key] = true
		}
	}

	var changes []change
	for i := len(ws.lock.Packages) - 1; i >= 0; i-- {
		if !kept[ref.Fold(install.LockedIdentity(&ws.lock.Packages[i]))] {
			changes = append(changes, change{kind: uninstallChange, origin: i})
		}
	}
	for _, c := range p.members {
		key := ref.Fold(c.Identity())
		if !kept[key] {
			changes = append(changes, change{kind: installChange, candidate: c, explicit: p.explicit[key]})
		}
	}

	finish := func(remaining []pkgmodel.LockedPackage) []pkgmodel.LockedPackage {
		// The workspace file lists the members of the new set in installation
		// order.
		var final []pkgmodel.LockedPackage
		for _, c := range p.members {
			var pkg pkgmodel.LockedPackage
			for _, r := range remaining {
				if ref.NamesEqual(install.LockedIdentity(&r), c.Identity()) {
					pkg = r
					break
				}
			}
			key := ref.Fold(install.LockedIdentity(&pkg))
			isExplicit := p.explicit[key]
			if isExplicit != pkg.Explicit && kept[key] {
				verb := "Marking"
				if op.dryRun {
					verb = "Dry run: would mark"
				}
				state := "not explicitly installed."
				if isExplicit {
					state = "explicitly installed."
				}
				e.info(fmt.Sprintf("%s package '%s' version %s as %s", verb, install.LockedIdentity(&pkg),
					install.LockedVersion(&pkg), state))
			}
			entry := pkg
			entry.Explicit = isExplicit
			// A kept package requested by another source, e.g.
			// "<repo>@<version>" for a package installed from "<repo>",
			// records the source it is requested by now.
			if m, ok := members[key]; ok && isExplicit && kept[key] && m.source.String() != pkg.Ref {
				entry.Ref = m.source.String()
			}
			final = append(final, entry)
		}
		return final
	}
	return changeSet{changes: changes, finish: finish}
}

// installWithoutDependencies is InstallWithoutDependencies (protocol/cli.md
// §5.1 with --no-dependencies).
func (e *Engine) installWithoutDependencies(ctx context.Context, ws *workspace, requests []request, op operation) error {
	e.warning("--no-dependencies is enabled. This operation may result in a broken workspace.")
	candidates, err := e.requestedCandidates(ctx, ws, requests, false)
	if err != nil {
		return err
	}
	var changes []change
	for _, c := range candidates {
		existing := -1
		for i := range ws.lock.Packages {
			if ref.NamesEqual(install.LockedIdentity(&ws.lock.Packages[i]), c.Identity()) {
				existing = i
				break
			}
		}
		if existing >= 0 && isSamePackage(&ws.lock.Packages[existing], c) {
			changes = append(changes, change{kind: markExplicitChange, origin: existing})
			continue
		}
		if existing >= 0 {
			changes = append(changes, change{kind: uninstallChange, origin: existing})
		}
		changes = append(changes, change{kind: installChange, candidate: c, explicit: true})
	}
	return e.execute(ctx, ws, op, changeSet{changes: changes})
}

// remainingPackage is an entry of the workspace file as the planned changes
// leave it: origin is its index in the workspace file, or -1 for a package
// the operation installs.
type remainingPackage struct {
	pkg    pkgmodel.LockedPackage
	origin int
}

// plannedChange is a planned uninstall or install and its message.
type plannedChange struct {
	message string
	pkg     string // "<identity>@<version>", for progress
	plan    *install.Plan
}

// execute is Execute: plans every change against a view of the workspace as
// the earlier changes leave it, so that every check and every fetch happens
// before anything changes; then applies the plans in one transaction and
// writes the workspace file last. A failure while applying (or
// cancellation) undoes every change and leaves the workspace file as it was
// (protocol/workspace.md §6). In a dry run, reports the plans instead.
func (e *Engine) execute(ctx context.Context, ws *workspace, op operation, cs changeSet) error {
	var layout *install.LayoutPlan
	for _, c := range cs.changes {
		if c.kind != markExplicitChange {
			var err error
			if layout, err = install.PlanLayout(ws.root); err != nil {
				return err
			}
			if op.dryRun {
				for _, m := range layout.DryRunMessages() {
					e.info(m)
				}
			}
			break
		}
	}

	view := install.NewView(layout)
	remaining := make([]remainingPackage, len(ws.lock.Packages))
	for i, p := range ws.lock.Packages {
		remaining[i] = remainingPackage{pkg: p, origin: i}
	}
	installed := func() []pkgmodel.LockedPackage {
		out := make([]pkgmodel.LockedPackage, len(remaining))
		for i, r := range remaining {
			out[i] = r.pkg
		}
		return out
	}
	options := func() install.Options {
		return install.Options{Root: ws.root, Installed: installed(), DryRun: op.dryRun,
			IgnoreScripts: op.ignoreScripts}
	}

	var plans []plannedChange
	for _, c := range cs.changes {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch c.kind {
		case uninstallChange:
			pkg := ws.lock.Packages[c.origin]
			what := fmt.Sprintf(" package '%s' version %s", install.LockedIdentity(&pkg), install.LockedVersion(&pkg))
			if op.dryRun {
				e.info("Dry run: would uninstall" + what)
			}
			entry, err := lockedEntry(&pkg, ws.env)
			if err != nil {
				return err
			}
			pl, err := e.inst.PlanUninstall(ctx, view, options(), &pkg, entry)
			if err != nil {
				return err
			}
			plans = append(plans, plannedChange{message: "Uninstalling" + what,
				pkg: install.LockedIdentity(&pkg) + "@" + install.LockedVersion(&pkg), plan: pl})
			for i, r := range remaining {
				if r.origin == c.origin {
					remaining = append(remaining[:i:i], remaining[i+1:]...)
					break
				}
			}
		case installChange:
			cand := c.candidate
			what := fmt.Sprintf(" package '%s' version %s from %s", cand.Identity(), cand.version, cand.archive.Key)
			if op.dryRun {
				e.info("Dry run: would install" + what)
			}
			pl, err := e.inst.PlanInstall(ctx, view, options(), cand.installPackage())
			if err != nil {
				return err
			}
			plans = append(plans, plannedChange{message: "Installing" + what,
				pkg: cand.Identity() + "@" + cand.version.String(), plan: pl})
			remaining = append(remaining, remainingPackage{pkg: locked(cand, c.explicit, pl.OwnedFiles), origin: -1})
		case markExplicitChange:
			pkg := ws.lock.Packages[c.origin]
			if !pkg.Explicit {
				verb := "Marking"
				if op.dryRun {
					verb = "Dry run: would mark"
				}
				e.info(fmt.Sprintf("%s package '%s' version %s as explicitly installed.", verb,
					install.LockedIdentity(&pkg), install.LockedVersion(&pkg)))
			}
			for i := range remaining {
				if remaining[i].origin == c.origin {
					remaining[i].pkg.Explicit = true
					break
				}
			}
		}
	}

	finalPackages := installed()
	if cs.finish != nil {
		finalPackages = cs.finish(finalPackages)
	}
	final := lockWith(ws, finalPackages)
	if op.dryRun {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if len(plans) == 0 {
		return pkgfile.SaveWorkspace(ws.root, final)
	}

	var reserved []string
	for _, p := range plans {
		reserved = append(reserved, p.plan.WrittenPaths()...)
	}
	commitStaged, discardStaged := pkgfile.WorkspaceStore{}.StagedHooks(ws.root)
	tx, err := txn.Begin(ws.root, txn.Options{Reserved: reserved, Context: ctx,
		CommitStaged: commitStaged, DiscardStaged: discardStaged})
	if err != nil {
		return err
	}
	defer tx.Close()

	bar := e.newBar(report.StageTask, report.UnitPackages, "Applying package changes", int64(len(plans)))
	applied := int64(0)
	defer func() { bar.doneAt(applied) }()
	err = func() error {
		if layout != nil {
			if err := install.ApplyLayout(tx, layout); err != nil {
				return err
			}
		}
		for _, p := range plans {
			e.info(p.message)
			bar.step(applied, p.pkg, p.message)
			if err := e.inst.Apply(ctx, tx, p.plan); err != nil {
				return err
			}
			applied++
			bar.update(applied, "")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// The commit point: the journal records the commit, then the
		// workspace file moves into place.
		if err := pkgfile.StageWorkspace(ws.root, final); err != nil {
			return err
		}
		if err := tx.MarkCommitted(); err != nil {
			return err
		}
		return pkgfile.CommitStaged(ws.root)
	}()
	if err != nil {
		if isContextError(err) {
			e.warning("Cancelled; undoing its changes to the workspace.")
		} else {
			e.warning("The operation failed; undoing its changes to the workspace.")
		}
		errs := tx.Rollback(func() error {
			// A staged workspace file is never read, and the next save
			// replaces it.
			_ = pkgfile.DiscardStaged(ws.root)
			return nil
		})
		for _, u := range errs {
			e.warning(u.Error())
		}
		if len(errs) > 0 {
			e.warning("Some changes could not be undone. Stop the server and other programs using the" +
				" workspace; the next install, uninstall or update of this workspace tries again.")
		}
		return err
	}

	for _, w := range tx.Complete() {
		e.warning(w.Error())
	}
	return nil
}

// locked is the workspace file entry of an installed candidate.
func locked(c *candidate, explicit bool, files []string) pkgmodel.LockedPackage {
	return pkgmodel.LockedPackage{
		Manifest: c.manifest,
		Variant:  c.variant,
		Explicit: explicit,
		Ref:      c.source.String(),
		Commit:   commitOf(c),
		Files:    append([]string{}, files...),
	}
}

// lockWith is LockWith: the workspace file with the given packages and the
// environment.
func lockWith(ws *workspace, packages []pkgmodel.LockedPackage) *pkgmodel.LockFile {
	if packages == nil {
		packages = []pkgmodel.LockedPackage{}
	}
	return &pkgmodel.LockFile{
		Format:   ws.lock.Format,
		Platform: ws.env.Platform,
		Runtime:  ws.env.Runtime,
		Packages: packages,
	}
}

// #endregion

// #region Installed packages

// pinnedCandidate is PinnedCandidate: rebuilds an installed package from the
// workspace file (protocol/workspace.md §5, "Reinstalling"), with the
// package file it records, and the archive of the commit it records for a
// remote repository, otherwise the archive its source names now, opened when
// first needed.
func (e *Engine) pinnedCandidate(ctx context.Context, ws *workspace, p *pkgmodel.LockedPackage) (*candidate, error) {
	c, err := func() (*candidate, error) {
		version, err := semver.ParseStrict(install.LockedVersion(p))
		if err != nil {
			return nil, err
		}
		entry, err := lockedEntry(p, ws.env)
		if err != nil {
			return nil, err
		}
		source, err := parseRecordedSource(p)
		if err != nil {
			return nil, err
		}
		spec := source.Kind == ref.KindSpec
		if spec && p.Commit == "" {
			return nil, &pkgfile.LockFormatError{Message: fmt.Sprintf("The entry of '%s' has no \"commit\""+
				" field, which every package from a remote repository (%s) must have.",
				install.LockedIdentity(p), source)}
		}
		commit := ""
		if spec {
			commit = p.Commit
		}
		provider := ws.provider
		src := archive.Defer(func() (archive.Source, error) {
			var h *archiveHandle
			var err error
			if spec {
				h, err = provider.OpenCommit(ctx, source.Target, commit)
			} else {
				plain, perr := source.WithVariant("")
				if perr != nil {
					return nil, perr
				}
				h, err = provider.Open(ctx, plain)
			}
			if err != nil {
				return nil, err
			}
			return h.Source, nil
		})
		a := &archiveHandle{Key: p.Ref + " (installed)", Source: src, Commit: commit}
		return ws.resolver.newCandidate(stringMember(p.Manifest, "id"), p.Variant, version, entry, p.Manifest, a, source), nil
	}()
	if err == nil {
		return c, nil
	}
	var ie *install.Error
	var lfe *pkgfile.LockFormatError
	extra := " If the file was edited by hand, undo the edit."
	switch {
	case errors.As(err, &ie) && err == error(ie) && ws.env.Runtime != ws.lock.Runtime:
		extra = fmt.Sprintf(" The workspace was installed for the runtime %s; run lip without --rid, or"+
			" with that runtime.", ws.lock.Runtime)
	case errors.As(err, &lfe) && err == error(lfe):
		extra = ""
	}
	return nil, &Error{Msg: fmt.Sprintf("Cannot use the installed package '%s' as the workspace file %s"+
		" records it, in the environment (%s, %s).%s", install.LockedIdentity(p), pkgmodel.WorkspaceFile,
		ws.env.Platform, ws.env.Runtime, extra), Err: err}
}

// lockedEntry is McmodInstaller.LockedEntry: the entry of the variant of an
// installed package in its recorded package file for the environment.
func lockedEntry(p *pkgmodel.LockedPackage, env environment) (*pkgmodel.Entry, error) {
	identity, version := install.LockedIdentity(p), install.LockedVersion(p)
	doc, err := pkgfile.Parse(p.Manifest)
	if err != nil {
		return nil, &install.Error{Msg: fmt.Sprintf("The package file that the workspace file (%s) records"+
			" for %s %s is invalid. If you edited the workspace file by hand, undo the change.",
			pkgmodel.WorkspaceFile, identity, version), Err: err}
	}
	variant := "the default variant"
	if p.Variant != "" {
		variant = fmt.Sprintf("the variant \"%s\"", p.Variant)
	}
	entry := doc.SelectEntry(p.Variant, env.Platform, env.Runtime)
	if entry == nil {
		return nil, &install.Error{Msg: fmt.Sprintf("The package file that the workspace file (%s) records"+
			" for %s %s has no entry of %s for the platform %s and the runtime %s. If you passed --rid,"+
			" pass the runtime the workspace was installed for instead.",
			pkgmodel.WorkspaceFile, identity, version, variant, env.Platform, env.Runtime)}
	}
	return entry, nil
}

// parseRecordedSource is ParseRecordedSource: the source an entry of the
// workspace file records.
func parseRecordedSource(p *pkgmodel.LockedPackage) (ref.Ref, error) {
	source, err := ref.Parse(p.Ref, true)
	if err != nil {
		return ref.Ref{}, &pkgfile.LockFormatError{Message: fmt.Sprintf("The workspace file %s records \"%s\""+
			" in the \"ref\" field of package '%s', which is not a valid package source. If the file was"+
			" edited by hand, undo the edit.", pkgmodel.WorkspaceFile, p.Ref, install.LockedIdentity(p)), Err: err}
	}
	return source, nil
}

// matchInstalled is MatchInstalled (protocol/cli.md §5.2): the indexes of
// the installed packages an uninstall argument names: by identity (ignoring
// ASCII case), or else by source, with the locations compared ignoring ASCII
// case; an argument without a variant matches every variant, and one without
// a ref every ref.
func (e *Engine) matchInstalled(ws *workspace, arg string) ([]int, error) {
	var byIdentity []int
	for i := range ws.lock.Packages {
		if ref.NamesEqual(install.LockedIdentity(&ws.lock.Packages[i]), arg) {
			byIdentity = append(byIdentity, i)
		}
	}
	if len(byIdentity) > 0 {
		return byIdentity, nil
	}

	wanted, err := ref.ParsePackageArgument(arg, ws.root)
	if err != nil {
		return nil, err
	}
	var out []int
	for i := range ws.lock.Packages {
		source, _, ok := ref.TryParse(ws.lock.Packages[i].Ref, true)
		if !ok || source.Kind != wanted.Kind || !ref.NamesEqual(source.Target, wanted.Target) {
			continue
		}
		if wanted.Variant != "" && source.Variant != wanted.Variant {
			continue
		}
		if wanted.GitRef != nil && (source.GitRef == nil || *source.GitRef != *wanted.GitRef) {
			continue
		}
		out = append(out, i)
	}
	return out, nil
}

// #endregion

func explicitIdentities(lf *pkgmodel.LockFile) []string {
	var out []string
	for i := range lf.Packages {
		if lf.Packages[i].Explicit {
			out = append(out, install.LockedIdentity(&lf.Packages[i]))
		}
	}
	return out
}

// isSamePackage reports whether a candidate is the installed package
// (protocol/workspace.md §5): same identity, version and commit.
func isSamePackage(p *pkgmodel.LockedPackage, c *candidate) bool {
	return ref.NamesEqual(install.LockedIdentity(p), c.Identity()) &&
		install.LockedVersion(p) == c.version.String() &&
		strings.EqualFold(p.Commit, commitOf(c))
}

// commitOf is the commit of a candidate from a remote repository (lowercase
// hex), or "".
func commitOf(c *candidate) string {
	if c.source.Kind != ref.KindSpec {
		return ""
	}
	return strings.ToLower(c.archive.Commit)
}

func stringMember(manifest any, name string) string {
	type getter interface{ Get(string) (any, bool) }
	if o, ok := manifest.(getter); ok {
		if v, ok := o.Get(name); ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func foldSet(names []string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[ref.Fold(n)] = true
	}
	return m
}

func containsIdentity(list []string, identity string) bool {
	for _, v := range list {
		if ref.NamesEqual(v, identity) {
			return true
		}
	}
	return false
}
