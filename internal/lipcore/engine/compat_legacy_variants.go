package engine

// TEMPORARY COMPATIBILITY LAYER for tooth.json dependencies on "X#client".
//
// It is LeviLauncher's, not lip's. With Options.LegacyRoleVariants off (the
// default, and always on server workspaces) none of it applies and the
// engine follows lip exactly. Only internal/lip's newEngine turns it on.
//
// Old releases are read from a tooth.json, where LiteLDev packages keep the
// server build in the label "" and the client build in "client" (or
// "client_<y>" next to "<y>"); mods published with a tooth.json depend on
// "X#client". Packages of LiteLDev and MiracleForest released with a
// lip.toml have no role variants: their client build is an entry of the
// variant "" (or "<y>"). On a client workspace, with the option on:
//
//   - R1: a dependency of a package read from a tooth.json (installed or a
//     candidate) on "X#client" ("X#client_<y>") is looked up as "X"
//     ("X#<y>") in every release of X read from a lip.toml. A release of X
//     read from a tooth.json is taken literally (its "client" label), with
//     no fallback either way. The candidate then has the identity "X", and
//     its recorded source is "X", so it unifies with the bare "X" other
//     packages depend on.
//   - Nothing else is rewritten. A dependency written in a lip.toml, and a
//     request, are literal: a lip.toml on a package released with a
//     tooth.json writes "X#client" itself, and a bare "X" gets the label "".
//   - One build: a tooth.json "X#client" and a lip.toml "X" are two builds
//     of one package, so the solver never adds one next to the other
//     (candidate.ClashKey, graph's clashKeyer); it backtracks to another
//     version instead. Two such packages installed already stay. A failure
//     to solve because of it alone matches ErrRoleBuildClash.
//   - In place: a request whose candidate is a lip.toml "X" ("X#<y>")
//     replaces an installed tooth.json "X#client" ("X#client_<y>"), and a
//     request whose candidate is a tooth.json "X#client" replaces an
//     installed lip.toml "X" (roleRecordsReplacedBy): install (also with
//     --no-dependencies) and update unpin it and drop it from the explicit
//     packages, so the plan uninstalls it unless something still needs it.
//   - An installed "X#client" read from a lip.toml (a real client variant),
//     or one whose archive cannot be read, is never replaced; a request
//     for it whose release has no client variant fails as in lip, without
//     the switch signal, with a note to uninstall it and install "X".
//   - Plan: a dry run returning the planned packages, with which the
//     launcher checks its pin retry before it runs it.
//   - Typed failures for the launcher's choice of identity
//     (internal/mcservice): a request "X#client" whose releases are all
//     read from a lip.toml without that variant fails with
//     ErrClientVariantNotInLipToml (ErrClientVariantNotInToothJSON for a
//     tooth.json without that label); a request "X" over an installed
//     lip.toml "X" whose candidate is a tooth.json release with a client
//     label fails with ErrClientBuildInRoleVariant (checkClientBuild), as
//     it would install that release's "" (usually its server build).
//
// Which installed packages were read from a tooth.json (recordOrigin): every
// package file the layer records carries markerKey naming the file it was
// read from (package files ignore unknown keys). A record written without
// the layer is a lip.toml when its package file is not shaped like
// compat.Derive's output; otherwise its archive (the recorded commit, or
// its local source) is read when the answer is needed, which is: for R1
// (the record depends on a role variant, or is one), for a replacement or
// checkClientBuild, and for ClashKey (lazily, only when a candidate of the
// same package id is checked against it). What a read finds is marked in
// the state file the operation writes. A record whose archive cannot be
// read is taken literally, as lip would: no R1, no ClashKey, no
// replacement, with a warning. Only the installation order of
// --no-dependencies guesses: an unread record counts as a lip.toml there.
//
// It can go once no published tooth.json package needs a client build of a
// package released with a lip.toml. To remove it:
//  1. Delete this file and compat_legacy_variants_test.go.
//  2. In internal/lipcore/engine, undo every line marked
//     "compat_legacy_variants.go" (search for it):
//     - engine.go: delete Options.LegacyRoleVariants and the res.compat line
//       of openWorkspace; Install passes unpinned and
//       explicitIdentities(ws.lock) to makePlan again; Install, Update and
//       installWithoutDependencies lose replacedRoles (and Update the
//       second test of its "not installed" check); requestedCandidates
//       loses checkClientBuild; firstCandidate loses missingRole;
//       pinnedCandidate passes no fromTooth and sets no installed or
//       origin; inInstallationOrder and lockedOrderNode take env again
//       (inInstallationOrder(remaining, ws.env), lockedOrderNode(p,
//       ws.env)); orderNode loses fromTooth and installationOrder the
//       stripped test; Engine loses onPlan, and execute its call.
//     - errors.go: Error loses compatKind.
//     - resolver.go: candidate loses compat, fromTooth, installed and
//       origin, walk loses toothDependent and failures (and report its
//       count), resolver loses compat and origins; newCandidate loses
//       fromTooth and uses r.graphRef(dep); ResolveLazy returns
//       r.resolve(...); resolveArchive loses the stripRole and
//       noteMissingRole lines and records pf.Raw.
//     - doc.go: the sentence on this layer.
//  3. In internal/lipcore/graph: delete Ref.Compat, resolveKey.compat,
//     clashKeyer, ErrBuildClash, clashKeyOf, isInstalled, clashBlocker and
//     its call in blocker, packageIDOf, noteClashFailure and its call in
//     recordFailure, reportsClash, search.clashReasons and clashFailures,
//     and ResolutionError.buildClash and Is.
//  4. internal/lip: newEngine without LegacyRoleVariants, the helpers
//     IsClientVariantNotInLipToml, IsClientVariantNotInToothJSON,
//     IsClientBuildInRoleVariant and IsRoleBuildClash, and PlanPackages,
//     PlannedPackage and planner (the dry run, Plan here).
//  5. internal/mcservice:
//     - lip_package.go: delete lipRoleCanonical, lipSwitchedIdentity and
//       lipRunSwitching. InstallLIPPackage calls lipInstallPackages /
//       lipUpdatePackages directly again (no run wrapper, no pin retry on
//       IsRoleBuildClash, no lipUnpinnedLeviLamina dry run and no
//       lipPlanPackages), with main's "#client" -> bare fallback on
//       isLipNoPackageFoundFor for the request made when the catalogue
//       cannot say (git show 12c663e:internal/mcservice/lip_package.go).
//     - levilamina.go: InstallLeviLamina loses its run wrapper and calls
//       lipUpdatePackages / lipInstallPackages with pkg again, with main's
//       fallback (git show 12c663e:internal/mcservice/levilamina.go).
//     - lip_identity_test.go and levilamina_test.go: delete the tests and
//       helpers (wrapNoClientVariant, clientBuildInRoleVariant,
//       roleBuildClash) that use engine.ErrClientVariantNotIn*,
//       engine.ErrClientBuildInRoleVariant or engine.ErrRoleBuildClash.
// WARNING: removing the layer breaks workspaces where R1 satisfied a
// tooth.json mod's "X#client" with a lip.toml "X": lip then needs
// "X#client" again, which no lip.toml release has, so every operation on
// such a workspace fails to solve. Before removing it, make sure no such
// mod is in use (its author moved it to a lip.toml depending on bare "X"),
// or have the launcher tell users to update or uninstall those mods. State
// files stay readable: the marker is an unknown key, and the records name
// the variant installed.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/liteldev/LeviLauncher/internal/lipcore/compat"
	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/graph"
	"github.com/liteldev/LeviLauncher/internal/lipcore/install"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
)

// roleVariantName is the role variant of the default variant.
const roleVariantName = "client"

// markerKey is the key of a recorded package file naming the file the
// package was read from (compat.ToothFileName or pkgmodel.PackageFileName).
const markerKey = "x-levilauncher-read-from"

// compatToothDependent is graph.Ref.Compat of a dependency, on a role
// variant, of a package read from a tooth.json: R1 applies to it.
const compatToothDependent = "tooth.json"

// ErrClientVariantNotInLipToml matches (errors.Is), with the layer on, the
// ErrNoPackageFound of a request for a role variant ("X#client",
// "X#client_<y>") when every release looked at is read from a lip.toml
// without an entry of that variant for the environment: the release keeps
// the client build in "X" ("X#<y>").
var ErrClientVariantNotInLipToml = errors.New("engine: the lip.toml release has no client variant")

// ErrClientVariantNotInToothJSON is ErrClientVariantNotInLipToml for
// releases read from a tooth.json without that label: their "" may well be
// a server build.
var ErrClientVariantNotInToothJSON = errors.New("engine: the tooth.json release has no client label")

// ErrClientBuildInRoleVariant matches (errors.Is) the failure of a request
// "X" ("X#<y>") over an installed lip.toml "X" whose candidate is a release
// read from a tooth.json with an entry of "X#client" ("X#client_<y>"):
// request that instead (checkClientBuild).
var ErrClientBuildInRoleVariant = errors.New("engine: the tooth.json release keeps its client build in a client variant")

// ErrRoleBuildClash matches (errors.Is) a failure to solve with a
// requirement whose every candidate was refused only because a tooth.json
// "X#client" and a lip.toml "X" cannot both be installed (graph's
// clashKeyer): dropping the "X#client" that blocked them lets the
// requirement have its lip.toml "X".
var ErrRoleBuildClash = graph.ErrBuildClash

// legacyVariantsOn reports whether the layer applies to an environment.
func (e *Engine) legacyVariantsOn(env environment) bool {
	return e.opts.LegacyRoleVariants && env.Side == pkgmodel.SideClient
}

// isRoleVariant reports whether a variant is "client" or "client_<y>".
func isRoleVariant(variant string) bool { return roleCanonical(variant) != variant }

// roleCanonical is the variant R1 looks up for a role variant: "client" ->
// "", "client_<y>" -> "<y>" (a valid variant name); any other variant is
// itself.
func roleCanonical(variant string) string {
	if variant == roleVariantName {
		return ""
	}
	if rest, ok := strings.CutPrefix(variant, roleVariantName+"_"); ok && ref.IsValidVariant(rest) {
		return rest
	}
	return variant
}

// roleOf is the role variant of a variant: "" -> "client", "<y>" ->
// "client_<y>".
func roleOf(variant string) string {
	if variant == "" {
		return roleVariantName
	}
	return roleVariantName + "_" + variant
}

// #region Resolving (R1)

// dependencyRef is a dependency of a package as the solver sees it: one on
// a role variant, of a package read from a tooth.json, is marked for R1.
func (r *resolver) dependencyRef(dep ref.Ref, fromTooth bool) graph.Ref {
	g := r.graphRef(dep)
	if fromTooth && isRoleVariant(dep.Variant) {
		g.Compat = compatToothDependent
	}
	return g
}

// stripRole is the source a candidate is resolved for in a release (d): for
// a dependency R1 applies to, in a release read from a lip.toml, the source
// with the role stripped; otherwise the source.
func (r *resolver) stripRole(w *walk, d *packageDocument, source ref.Ref) ref.Ref {
	if !w.toothDependent || d.isLegacy() || !isRoleVariant(source.Variant) {
		return source
	}
	stripped, err := source.WithVariant(roleCanonical(source.Variant))
	if err != nil {
		return source
	}
	return stripped
}

// recordedManifest is the package file the state file records for a
// candidate: with the layer, a copy carrying markerKey.
func (r *resolver) recordedManifest(d *packageDocument, pf *pkgmodel.Document) any {
	if !r.compat {
		return pf.Raw
	}
	o, ok := pf.Raw.(*doc.Object)
	if !ok {
		return pf.Raw
	}
	m := o.Clone()
	m.Set(markerKey, d.fileName())
	return m
}

// dependsOnRole reports whether an entry of a package file depends on a
// role variant.
func dependsOnRole(pf *pkgmodel.Document) bool {
	for _, v := range pf.Variants {
		for _, e := range v.Entries {
			for _, dep := range e.Deps {
				if source, err := ref.Parse(dep.Key, false); err == nil && isRoleVariant(source.Variant) {
					return true
				}
			}
		}
	}
	return false
}

// roleFailures counts, for one walk, the reports and those that are a
// release without an entry of the role variant asked for, by file.
type roleFailures struct{ reports, lipToml, toothJSON int }

// noteMissingRole counts a release (d, read as pf) without an entry of the
// variant asked for, when that is a role variant the release has no
// variant of at all (not one without an entry for this side or platform).
func (r *resolver) noteMissingRole(w *walk, d *packageDocument, pf *pkgmodel.Document, variant string) {
	if !r.compat || !isRoleVariant(variant) {
		return
	}
	for _, v := range pf.Variants {
		if v.Name == variant {
			return
		}
	}
	if d.isLegacy() {
		w.failures.toothJSON++
	} else {
		w.failures.lipToml++
	}
}

// missingRoleKind is the compatKind of firstCandidate's ErrNoPackageFound
// for a request (source) whose walk found nothing: one of the sentinels
// when every report was a release without the role variant asked for, of
// one file; otherwise nil. A request for an installed "X#client" read from
// a lip.toml (a package with a real client variant, such as a third-party
// one) gets no sentinel, as nothing replaces such a record, but a note
// (appended to the message) on what to do instead.
func (ws *workspace) missingRoleKind(w *walk, source ref.Ref) (error, string) {
	f := w.failures
	if !ws.resolver.compat || !isRoleVariant(source.Variant) || f.reports == 0 {
		return nil, ""
	}
	var kind error
	switch f.reports {
	case f.lipToml:
		kind = ErrClientVariantNotInLipToml
	case f.toothJSON:
		kind = ErrClientVariantNotInToothJSON
	default:
		return nil, ""
	}
	// The launcher switches to "X" on the lip.toml sentinel, which replaces
	// an installed "X#client" only when it was read from a tooth.json.
	identity := ref.Identity(source.Target, source.Variant)
	p := installedPackage(ws.lock, identity)
	if kind != ErrClientVariantNotInLipToml || p == nil {
		return kind, ""
	}
	why := ""
	switch ws.readOrigin(p) {
	case originTooth:
		return kind, ""
	case originLipToml:
		why = "is read from a lip.toml"
	default:
		why = "cannot be told to be read from a tooth.json (its archive cannot be read)"
	}
	canonical := ref.Identity(source.Target, roleCanonical(source.Variant))
	return nil, fmt.Sprintf("\nThe installed '%s' %s, so LeviLauncher does not replace it with '%s'. To move"+
		" to that release, uninstall '%s' and install '%s'.", install.LockedIdentity(p), why, canonical,
		install.LockedIdentity(p), canonical)
}

// PlannedPackage is a package of the state file an operation would write
// (Plan): an InstalledPackage, and whether it is read from a tooth.json.
type PlannedPackage struct {
	InstalledPackage
	FromToothJSON bool
}

// Plan runs Install (or, with update, Update) of packages as a dry run and
// returns the packages of the state file it would write, so that the
// launcher can check what a retry would do before it runs it (its pin
// retry, internal/mcservice). It fails as the operation would.
func (e *Engine) Plan(ctx context.Context, update bool, packages []string) ([]PlannedPackage, error) {
	var out []PlannedPackage
	planned := false
	e.onPlan = func(lf *pkgmodel.LockFile) {
		planned = true
		for i := range lf.Packages {
			p := &lf.Packages[i]
			fromTooth := false
			if o, ok := p.Manifest.(*doc.Object); ok {
				if v, ok := o.Get(markerKey); ok {
					fromTooth = v == compat.ToothFileName
				}
			}
			out = append(out, PlannedPackage{InstalledPackage: InstalledPackage{Identity: install.LockedIdentity(p),
				ID: stringMember(p.Manifest, "id"), Variant: p.Variant, Version: install.LockedVersion(p),
				Explicit: p.Explicit, Ref: p.Ref}, FromToothJSON: fromTooth})
		}
	}
	defer func() { e.onPlan = nil }()
	var err error
	if update {
		err = e.Update(ctx, packages, true, false)
	} else {
		err = e.Install(ctx, packages, true, false, false)
	}
	if err == nil && !planned {
		err = &Error{Msg: "internal error: the dry run planned nothing"}
	}
	return out, err
}

// #endregion

// #region One build (graph's clashKeyer)

// ClashKey is the package of a tooth.json client build ("X" for a role
// variant "X#client" read from a tooth.json) or of a lip.toml package
// without a role variant (its identity), folded; otherwise "", as always
// without the layer and for a record whose origin cannot be read.
func (c *candidate) ClashKey() string {
	if !c.compat {
		return ""
	}
	fromTooth := c.fromTooth
	if c.origin != nil {
		switch c.origin.get() {
		case originTooth:
			fromTooth = true
		case originLipToml:
			fromTooth = false
		default:
			return ""
		}
	}
	switch role := isRoleVariant(c.variant); {
	case fromTooth && role:
		return ref.Fold(ref.Identity(c.id, roleCanonical(c.variant)))
	case !fromTooth && !role:
		return ref.Fold(c.Identity())
	}
	return ""
}

// Installed reports whether the candidate is an installed package (from
// the state file).
func (c *candidate) Installed() bool { return c.installed }

// #endregion

// #region Installed packages

// recordOrigin is the file an installed package was read from.
type recordOrigin int

const (
	originUnknown    recordOrigin = iota // not read yet
	originLipToml                        // a lip.toml
	originTooth                          // a tooth.json
	originUnreadable                     // its archive cannot be read: taken literally
)

// lazyOrigin is a recordOrigin read when first needed.
type lazyOrigin struct {
	once sync.Once
	read func() recordOrigin
	v    recordOrigin
}

func (l *lazyOrigin) get() recordOrigin {
	l.once.Do(func() { l.v = l.read() })
	return l.v
}

// knownOrigin is the origin of a record known without reading its archive:
// its markerKey, an earlier read, or the shape of its package file (not
// shaped like compat.Derive's output: a lip.toml); originUnknown otherwise.
func (ws *workspace) knownOrigin(p *pkgmodel.LockedPackage) recordOrigin {
	r := ws.resolver
	o, ok := p.Manifest.(*doc.Object)
	if !r.compat || !ok {
		return originLipToml
	}
	if v, ok := o.Get(markerKey); ok {
		if s, _ := v.(string); s == compat.ToothFileName {
			return originTooth
		}
		return originLipToml
	}
	if v, ok := r.origins.Load(o); ok {
		return v.(recordOrigin)
	}
	if pf, err := pkgfile.Parse(o); err != nil || !shapedDerived(o, pf, ws.env) {
		return originLipToml
	}
	return originUnknown
}

// readOrigin is the origin of a record, reading its archive when it is not
// known; what it reads is marked on the record.
func (ws *workspace) readOrigin(p *pkgmodel.LockedPackage) recordOrigin {
	if o := ws.knownOrigin(p); o != originUnknown {
		return o
	}
	r := ws.resolver
	key := p.Manifest.(*doc.Object)
	origin := originUnreadable
	if d := ws.readRecordArchive(p); d != nil {
		origin = originLipToml
		if d.isLegacy() {
			origin = originTooth
		}
		m := key.Clone()
		m.Set(markerKey, d.fileName())
		p.Manifest = m
	} else if r.warnOnce("origin " + install.LockedIdentity(p)) {
		r.onWarning(fmt.Sprintf("Cannot tell whether the installed package '%s' version %s was read from a"+
			" tooth.json: its archive cannot be read. LeviLauncher takes it as written.",
			install.LockedIdentity(p), install.LockedVersion(p)))
	}
	r.origins.Store(key, origin)
	return origin
}

// recordFromTooth reports whether a record was read from a tooth.json
// (readOrigin).
func (ws *workspace) recordFromTooth(p *pkgmodel.LockedPackage) bool {
	return ws.resolver.compat && ws.readOrigin(p) == originTooth
}

// pinnedOrigin is the fromTooth of a record's candidate (needed now for R1
// when it is a role variant or depends on one, so read now) and its origin
// for ClashKey (read when first needed).
func (ws *workspace) pinnedOrigin(p *pkgmodel.LockedPackage) (bool, *lazyOrigin) {
	if !ws.resolver.compat {
		return false, nil
	}
	if o := ws.knownOrigin(p); o != originUnknown || isRoleVariant(p.Variant) || ws.recordDependsOnRole(p) {
		o = ws.readOrigin(p)
		l := &lazyOrigin{v: o}
		l.once.Do(func() {})
		return o == originTooth, l
	}
	return false, &lazyOrigin{read: func() recordOrigin { return ws.readOrigin(p) }}
}

// orderFromTooth is orderNode.fromTooth of a record: read when it depends
// on a role variant (strippedIdentity), else as known (an unread record
// counts as a lip.toml).
func (ws *workspace) orderFromTooth(p *pkgmodel.LockedPackage) bool {
	if !ws.resolver.compat {
		return false
	}
	if ws.recordDependsOnRole(p) {
		return ws.readOrigin(p) == originTooth
	}
	return ws.knownOrigin(p) == originTooth
}

// recordDependsOnRole reports whether a record's package file depends on a
// role variant.
func (ws *workspace) recordDependsOnRole(p *pkgmodel.LockedPackage) bool {
	pf, err := pkgfile.Parse(p.Manifest)
	return err == nil && dependsOnRole(pf)
}

// readRecordArchive reads the package document of the archive a record was
// installed from (its recorded commit, or its local source), or nil.
func (ws *workspace) readRecordArchive(p *pkgmodel.LockedPackage) *packageDocument {
	r := ws.resolver
	source, err := parseRecordedSource(p)
	if err != nil {
		return nil
	}
	var h *archiveHandle
	if source.Kind == ref.KindSpec {
		if p.Commit == "" {
			return nil
		}
		h, err = ws.provider.OpenCommit(r.ctx, source.Target, p.Commit)
	} else {
		plain, perr := source.WithVariant("")
		if perr != nil {
			return nil
		}
		h, err = ws.provider.Open(r.ctx, plain)
	}
	if err != nil {
		return nil
	}
	d, err := r.documents.do(h.Key, func() (*packageDocument, error) { return readArchive(h) })
	if err != nil || !ref.NamesEqual(d.id(), stringMember(p.Manifest, "id")) {
		return nil
	}
	return d
}

// shapedDerived reports whether a package file has only what compat.Derive
// writes for the environment: no other top-level keys, and one entry per
// variant, for the environment, without an asset of type "mcmod".
func shapedDerived(o *doc.Object, pf *pkgmodel.Document, env environment) bool {
	for _, k := range o.Keys() {
		switch k {
		case pkgmodel.FormatKey, "id", "version", "info", "variants":
		default:
			return false
		}
	}
	for _, v := range pf.Variants {
		if len(v.Entries) != 1 {
			return false
		}
		e := &v.Entries[0]
		if e.Side != env.Side || !strings.EqualFold(e.Platform, env.Platform) || e.McmodAsset() >= 0 {
			return false
		}
	}
	return true
}

// roleRecordsReplacedBy are the identities of the installed packages that
// requested candidates replace in place, for a candidate whose identity is
// not installed: for a lip.toml "X" ("X#<y>"), an installed "X#client"
// ("X#client_<y>") read from a tooth.json; for a tooth.json "X#client"
// ("X#client_<y>"), an installed "X" ("X#<y>") read from a lip.toml.
func (ws *workspace) roleRecordsReplacedBy(candidates []*candidate) []string {
	if !ws.resolver.compat {
		return nil
	}
	var out []string
	for _, c := range candidates {
		if installedPackage(ws.lock, c.Identity()) != nil {
			continue
		}
		var other string
		want := originTooth
		switch role := isRoleVariant(c.variant); {
		case !c.fromTooth && !role:
			other = roleOf(c.variant)
		case c.fromTooth && role:
			other, want = roleCanonical(c.variant), originLipToml
		default:
			continue
		}
		if !ref.IsValidVariant(other) && other != "" {
			continue
		}
		p := installedPackage(ws.lock, ref.Identity(c.id, other))
		if p == nil || ws.readOrigin(p) != want {
			continue
		}
		if identity := install.LockedIdentity(p); !containsIdentity(out, identity) {
			out = append(out, identity)
		}
	}
	return out
}

// checkClientBuild fails a request whose candidate (c), read from a
// tooth.json, has the identity of an installed lip.toml package ("X") while
// the release keeps its client build in "X#client" ("X#client_<y>"): it
// would install the release's "X" (usually its server build) in place of
// the client build. The launcher then requests "X#client".
func (ws *workspace) checkClientBuild(r request, c *candidate) error {
	if !ws.resolver.compat || !c.fromTooth || isRoleVariant(c.variant) {
		return nil
	}
	role := roleOf(c.variant)
	p := installedPackage(ws.lock, c.Identity())
	if !ref.IsValidVariant(role) || p == nil || ws.readOrigin(p) != originLipToml {
		return nil
	}
	pf, err := pkgfile.Parse(c.manifest)
	if err != nil || pf.SelectEntry(role, ws.env.Side, ws.env.Platform) == nil {
		return nil
	}
	return &Error{Msg: fmt.Sprintf("Cannot use '%s': %s %s is read from a tooth.json, which keeps its client"+
		" build in '%s', while the installed '%s' is read from a lip.toml. Request '%s' instead.", r.arg,
		c.Identity(), c.version, ref.Identity(c.id, role), install.LockedIdentity(p), ref.Identity(c.id, role)),
		compatKind: ErrClientBuildInRoleVariant}
}

// withoutIdentities is list without the identities of remove.
func withoutIdentities(list, remove []string) []string {
	if len(remove) == 0 {
		return list
	}
	var out []string
	for _, id := range list {
		if !containsIdentity(remove, id) {
			out = append(out, id)
		}
	}
	return out
}

// #endregion

// #region Installation order

// strippedIdentity is the identity R1 may satisfy a dependency of the node
// with ("X" for "X#client" of a package read from a tooth.json), or "".
func (n orderNode) strippedIdentity(source ref.Ref) string {
	if !n.fromTooth || !isRoleVariant(source.Variant) {
		return ""
	}
	return ref.Identity(source.Target, roleCanonical(source.Variant))
}

// isStripped reports whether the node is a package of a strippedIdentity
// (one not read from a tooth.json).
func (n orderNode) isStripped(identity string) bool {
	return identity != "" && !n.fromTooth && ref.NamesEqual(n.identity, identity)
}

// #endregion
