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
//     of one package, so the solver never takes both (candidate.ClashKey,
//     graph's clashKeyer); it backtracks to another version instead, e.g.
//     to the tooth.json release for every dependent of a mixed range.
//   - In place: a request whose candidate is a lip.toml "X" ("X#<y>")
//     replaces an installed tooth.json "X#client" ("X#client_<y>"), and a
//     request whose candidate is a tooth.json "X#client" replaces an
//     installed lip.toml "X" (roleRecordsReplacedBy): install (also with
//     --no-dependencies) and update unpin it and drop it from the explicit
//     packages, so the plan uninstalls it unless something still needs it.
//   - The launcher's choice of identity (internal/mcservice) learns from
//     firstCandidate why a request "X#client" found nothing: every release
//     looked at is a lip.toml without that variant
//     (ErrClientVariantNotInLipToml), or a tooth.json without it
//     (ErrClientVariantNotInToothJSON).
//
// Which installed packages were read from a tooth.json: every package file
// the layer records carries markerKey naming the file it was read from
// (package files ignore unknown keys). A record written without the layer
// (recordFromTooth): a package file that is not shaped like compat.Derive's
// output is a lip.toml; one that is, and whose origin matters (a role
// variant or a dependency on one), is looked up in its archive (the
// recorded commit, or its local source), and marked in the state file the
// operation writes; when that archive cannot be read it is taken literally
// (as a lip.toml), with a warning. The remaining guess: a derived-shaped
// record whose origin does not matter for R1 (no role variant, no role
// dependency) counts as a tooth.json, so it has no ClashKey.
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
//       second test of its "not installed" check); firstCandidate loses
//       missingRole; pinnedCandidate passes no fromTooth;
//       inInstallationOrder and lockedOrderNode take env again
//       (inInstallationOrder(remaining, ws.env), lockedOrderNode(p,
//       ws.env)); orderNode loses fromTooth and installationOrder the
//       stripped test.
//     - errors.go: Error loses compatKind.
//     - resolver.go: candidate loses compat and fromTooth, walk loses
//       toothDependent and failures (and report its count), resolver loses
//       compat and origins; newCandidate loses fromTooth and uses
//       r.graphRef(dep); ResolveLazy returns r.resolve(...);
//       resolveArchive loses the stripRole and noteMissingRole lines and
//       records pf.Raw.
//     - doc.go: the sentence on this layer.
//  3. In internal/lipcore/graph: delete Ref.Compat, resolveKey.compat,
//     clashKeyer, clashKeyOf, clashBlocker and its call in blocker.
//  4. internal/lip: newEngine without LegacyRoleVariants, and the helpers
//     IsClientVariantNotInLipToml / IsClientVariantNotInToothJSON.
//  5. internal/mcservice: lipRoleRequest falls back to nothing (request
//     the installed identity, or the catalogue's variant, as before).
// WARNING: removing the layer breaks workspaces where R1 satisfied a
// tooth.json mod's "X#client" with a lip.toml "X": lip then needs
// "X#client" again, which no lip.toml release has, so every operation on
// such a workspace fails to solve. Before removing it, make sure no such
// mod is in use (its author moved it to a lip.toml depending on bare "X"),
// or have the launcher tell users to update or uninstall those mods. State
// files stay readable: the marker is an unknown key, and the records name
// the variant installed.

import (
	"errors"
	"fmt"
	"strings"

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

// noteMissingRole counts a release (d) without an entry of the variant
// asked for, when that is a role variant.
func (r *resolver) noteMissingRole(w *walk, d *packageDocument, variant string) {
	if !r.compat || !isRoleVariant(variant) {
		return
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
// one file; otherwise nil.
func (r *resolver) missingRoleKind(w *walk, source ref.Ref) error {
	f := w.failures
	if !r.compat || !isRoleVariant(source.Variant) || f.reports == 0 {
		return nil
	}
	switch f.reports {
	case f.lipToml:
		return ErrClientVariantNotInLipToml
	case f.toothJSON:
		return ErrClientVariantNotInToothJSON
	}
	return nil
}

// #endregion

// #region One build (graph's clashKeyer)

// ClashKey is the package of a tooth.json client build ("X" for a role
// variant "X#client" read from a tooth.json) or of a lip.toml package
// without a role variant (its identity), folded; otherwise "", as always
// without the layer.
func (c *candidate) ClashKey() string {
	if !c.compat {
		return ""
	}
	switch role := isRoleVariant(c.variant); {
	case c.fromTooth && role:
		return ref.Fold(ref.Identity(c.id, roleCanonical(c.variant)))
	case !c.fromTooth && !role:
		return ref.Fold(c.Identity())
	}
	return ""
}

// #endregion

// #region Installed packages

// recordFromTooth reports whether an installed package was read from a
// tooth.json: as its markerKey says, else as found out once (see the file
// comment), and false without the layer. A record found out from its
// archive is marked.
func (ws *workspace) recordFromTooth(p *pkgmodel.LockedPackage) bool {
	r := ws.resolver
	if !r.compat {
		return false
	}
	o, ok := p.Manifest.(*doc.Object)
	if !ok {
		return false
	}
	if v, ok := o.Get(markerKey); ok {
		s, _ := v.(string)
		return s == compat.ToothFileName
	}
	if v, ok := r.origins.Load(o); ok {
		return v.(bool)
	}
	fromTooth, read := ws.classifyRecord(p, o)
	if read {
		m := o.Clone()
		if fromTooth {
			m.Set(markerKey, compat.ToothFileName)
		} else {
			m.Set(markerKey, pkgmodel.PackageFileName)
		}
		p.Manifest = m
	}
	r.origins.Store(o, fromTooth)
	return fromTooth
}

// classifyRecord tells whether a record without markerKey was read from a
// tooth.json, and whether that was read from its archive.
func (ws *workspace) classifyRecord(p *pkgmodel.LockedPackage, o *doc.Object) (fromTooth, read bool) {
	pf, err := pkgfile.Parse(o)
	if err != nil || !shapedDerived(o, pf, ws.env) {
		return false, false
	}
	if !isRoleVariant(p.Variant) && !dependsOnRole(pf) {
		return true, false
	}
	switch d := ws.readRecordArchive(p); {
	case d != nil:
		return d.isLegacy(), true
	default:
		r := ws.resolver
		if r.warnOnce("origin " + install.LockedIdentity(p)) {
			r.onWarning(fmt.Sprintf("Cannot tell whether the installed package '%s' version %s was read from a"+
				" tooth.json: its archive cannot be read. LeviLauncher takes it as written.",
				install.LockedIdentity(p), install.LockedVersion(p)))
		}
		return false, false
	}
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
		switch role := isRoleVariant(c.variant); {
		case !c.fromTooth && !role:
			other = roleOf(c.variant)
		case c.fromTooth && role:
			other = roleCanonical(c.variant)
		default:
			continue
		}
		if !ref.IsValidVariant(other) && other != "" {
			continue
		}
		p := installedPackage(ws.lock, ref.Identity(c.id, other))
		// Only across files: a tooth.json "X#client" and a lip.toml "X".
		if p == nil || ws.recordFromTooth(p) == c.fromTooth {
			continue
		}
		if identity := install.LockedIdentity(p); !containsIdentity(out, identity) {
			out = append(out, identity)
		}
	}
	return out
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
