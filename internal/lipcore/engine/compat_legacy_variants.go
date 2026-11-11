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
//     packages depend on: one package is installed.
//   - Nothing else is rewritten. A dependency written in a lip.toml, and a
//     request, are literal: a lip.toml on a package released with a
//     tooth.json writes "X#client" itself, and a bare "X" gets the label "".
//   - A request whose candidate, read from a lip.toml, has the identity "X"
//     ("X#<y>") replaces an installed "X#client" ("X#client_<y>") in place
//     (roleRecordsReplacedBy): install and update unpin it and drop it from
//     the explicit packages, so the plan uninstalls it and installs "X".
//     When packages still need "X#client" literally (a lip.toml on it), it
//     fails instead of installing both builds (checkReplacedRoles).
//
// Which installed packages were read from a tooth.json: a package file the
// layer records with a dependency on a role variant carries markerKey
// naming the file it was read from (package files ignore unknown keys).
// A record written without the layer is taken as read from a tooth.json
// when its package file has only what compat.Derive writes for the
// environment (shapedDerived).
//
// It can go once no published tooth.json package needs a client build of a
// package released with a lip.toml. To remove it:
//  1. Delete this file and compat_legacy_variants_test.go.
//  2. In internal/lipcore/engine, undo every line marked
//     "compat_legacy_variants.go" (search for it):
//     - engine.go: delete Options.LegacyRoleVariants and the res.compat line
//       of openWorkspace; Install passes unpinned and
//       explicitIdentities(ws.lock) to makePlan again; Install and Update
//       lose replacedRoles, the checkReplacedRoles calls and the second test
//       of Update's "not installed" check;
//       pinnedCandidate passes no fromTooth; inInstallationOrder and
//       lockedOrderNode take env again (inInstallationOrder(remaining,
//       ws.env), lockedOrderNode(p, ws.env)); orderNode loses fromTooth and
//       installationOrder the stripped test.
//     - resolver.go: candidate and walk lose fromTooth and toothDependent,
//       resolver loses compat; newCandidate loses fromTooth and uses
//       r.graphRef(dep); ResolveLazy returns r.resolve(...);
//       resolveArchive loses the stripRole line and records pf.Raw.
//     - doc.go: the sentence on this layer.
//  3. In internal/lipcore/graph, delete Ref.Compat and resolveKey.compat.
//  4. internal/lip/packages.go: newEngine without LegacyRoleVariants.
//  5. The launcher's fallback from "X#client" to "X" (internal/mcservice
//     lipRoleFallback and its uses) is not part of this layer: it picks
//     the identity by the target release and stays.
// State files written with the layer stay valid lip state files: the
// marker is an unknown key, and a dependency it resolved records "X". A
// tooth.json mod whose "X#client" was satisfied by a lip.toml "X" then
// needs "X#client" again, as in lip.

import (
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

// legacyVariantsOn reports whether the layer applies to an environment.
func (e *Engine) legacyVariantsOn(env environment) bool {
	return e.opts.LegacyRoleVariants && env.Side == pkgmodel.SideClient
}

// isRoleVariant reports whether a variant is "client" or "client_<y>".
func isRoleVariant(variant string) bool { return roleCanonical(variant) != variant }

// roleCanonical is the variant R1 looks up for a role variant: "client" ->
// "", "client_<y>" -> "<y>"; any other variant is itself.
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
// candidate: with the layer, a copy carrying markerKey when the package file
// depends on a role variant (the only case where its origin matters).
func (r *resolver) recordedManifest(d *packageDocument, pf *pkgmodel.Document) any {
	if !r.compat || !dependsOnRole(pf) {
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

// #endregion

// #region Installed packages

// recordFromTooth reports whether an installed package was read from a
// tooth.json, as far as the layer needs to know: as its markerKey says; for
// a record without it whose package file depends on a role variant, when
// its package file looks derived (shapedDerived); otherwise false.
func (ws *workspace) recordFromTooth(p *pkgmodel.LockedPackage) bool {
	if !ws.resolver.compat {
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
	pf, err := pkgfile.Parse(o)
	if err != nil || !dependsOnRole(pf) {
		return false
	}
	return shapedDerived(o, pf, ws.env)
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
// requested candidates replace in place: for a candidate read from a
// lip.toml of identity "X" ("X#<y>") that is not installed, an installed
// "X#client" ("X#client_<y>").
func (ws *workspace) roleRecordsReplacedBy(candidates []*candidate) []string {
	if !ws.resolver.compat {
		return nil
	}
	var out []string
	for _, c := range candidates {
		if c.fromTooth || isRoleVariant(c.variant) || installedPackage(ws.lock, c.Identity()) != nil {
			continue
		}
		role := roleOf(c.variant)
		if !ref.IsValidVariant(role) {
			continue
		}
		if p := installedPackage(ws.lock, ref.Identity(c.id, role)); p != nil {
			if identity := install.LockedIdentity(p); !containsIdentity(out, identity) {
				out = append(out, identity)
			}
		}
	}
	return out
}

// checkReplacedRoles fails when a plan keeps a record that requests were to
// replace in place (roleRecordsReplacedBy): installed packages still depend
// on it literally (a lip.toml on "X#client"), and lip would install both
// builds, whose files clash.
func (ws *workspace) checkReplacedRoles(p *plan, replaced []string) error {
	for _, identity := range replaced {
		for _, m := range p.members {
			if !ref.NamesEqual(m.Identity(), identity) {
				continue
			}
			var dependents []string
			for _, d := range p.members {
				for _, dep := range d.deps {
					if dep.Source.Spec && ref.NamesEqual(ref.Identity(dep.Source.Target, dep.Source.Variant), identity) &&
						dep.Source.Compat == "" {
						dependents = append(dependents, "'"+d.Identity()+"'")
					}
				}
			}
			return &Error{Msg: fmt.Sprintf("Cannot replace '%s' with the requested package of the same id:"+
				" installed packages (%s) depend on '%s' itself and still need it. Update or uninstall them"+
				" first.", identity, strings.Join(dependents, ", "), identity)}
		}
	}
	return nil
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
