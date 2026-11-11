package engine

// Tests of the temporary compatibility layer for tooth.json dependencies on
// "X#client" (compat_legacy_variants.go); delete with it.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
)

const (
	llID  = "github.com/compat/ll"
	modID = "github.com/compat/mod"
	newID = "github.com/compat/new"
)

// compatHarness is a client workspace whose engine has the layer on (or
// off).
func compatHarness(t *testing.T, on bool) *harness {
	h := newHarness(t)
	h.side = pkgmodel.SideClient
	h.setCompat(on)
	return h
}

// setCompat restarts the engine with the layer on or off.
func (h *harness) setCompat(on bool) {
	h.restart()
	h.e.opts.LegacyRoleVariants = on
}

// toothLabels is a tooth.json of format 3 with one variant per label:
// (label, dependencies as a JSON object or "", file in the archive, dest)
// quadruples.
func toothLabels(id, version string, labels ...string) string {
	var variants []string
	for i := 0; i+3 < len(labels); i += 4 {
		deps := labels[i+1]
		if deps == "" {
			deps = "{}"
		}
		variants = append(variants, fmt.Sprintf(`{ "label": %q, "dependencies": %s, "assets": [ { "type": "self",`+
			` "placements": [ { "type": "file", "src": %q, "dest": %q } ] } ] }`, labels[i], deps, labels[i+2], labels[i+3]))
	}
	return fmt.Sprintf(`{ "format_version": 3, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
  "tooth": %q, "version": %q, "variants": [ %s ] }`, id, version, strings.Join(variants, ", "))
}

// lipVariants is a lip.toml, written as JSON, with (name, JSON array of
// entries) pairs as its variants.
func lipVariants(id, version string, variants ...string) string {
	var members []string
	for i := 0; i+1 < len(variants); i += 2 {
		members = append(members, fmt.Sprintf("%q: %s", variants[i], variants[i+1]))
	}
	return fmt.Sprintf(`{ "format": 4, "id": %q, "version": %q, "variants": { %s } }`, id, version,
		strings.Join(members, ", "))
}

// entry is an entry, as JSON, placing src at dest, with dependencies (a
// JSON object or "").
func entry(side, src, dest, deps string) string {
	if deps == "" {
		deps = "{}"
	}
	return fmt.Sprintf(`{ "side": %q, "platform": %q, "deps": %s, %s }`, side, testPlatform, deps, selfAsset(src, dest))
}

// addToothLL tags a LeviLamina-like release read from a tooth.json: "" is
// the server build, "client" the client build.
func (h *harness) addToothLL(version string) {
	h.addTag(llID, version, "tooth.json", toothLabels(llID, version,
		"", "", "s.dll", "ll/server-"+version+".dll",
		"client", "", "c.dll", "ll/client-"+version+".dll"), "s.dll", "server", "c.dll", "client")
}

// addLipLL tags a release of it read from a lip.toml: "" has a server and a
// client entry, and there is no client variant.
func (h *harness) addLipLL(version string) {
	h.addTag(llID, version, "lip.toml", lipVariants(llID, version, "", "["+
		entry("server", "s.dll", "ll/server-"+version+".dll", "")+", "+
		entry("client", "c.dll", "ll/client-"+version+".dll", "")+"]"), "s.dll", "server", "c.dll", "client")
}

// addToothMod tags a mod read from a tooth.json whose only label, "",
// depends on deps.
func (h *harness) addToothMod(id, version, deps string) {
	h.addTag(id, version, "tooth.json", toothLabels(id, version, "", deps, "m.dll", "mods/"+baseName(id)+".dll"),
		"m.dll", "mod")
}

// addLipMod tags a mod read from a lip.toml whose client entry depends on
// deps. (Its package file has the shape of a derived one: shapedDerived.)
func (h *harness) addLipMod(id, version, deps string) {
	h.addTag(id, version, "lip.toml", lipVariants(id, version, "", "["+
		entry("client", "m.dll", "mods/"+baseName(id)+".dll", deps)+"]"), "m.dll", "mod")
}

func baseName(id string) string { return id[strings.LastIndexByte(id, '/')+1:] }

func dep(key, rng string) string { return fmt.Sprintf(`{ %q: %q }`, key, rng) }

// recorded is the state file entry of an identity.
func (h *harness) recorded(identity string) *pkgmodel.LockedPackage {
	h.t.Helper()
	lf := h.lock()
	for i := range lf.Packages {
		p := &lf.Packages[i]
		id := stringMember(p.Manifest, "id")
		if p.Variant != "" {
			id += "#" + p.Variant
		}
		if strings.EqualFold(id, identity) {
			return p
		}
	}
	h.t.Fatalf("%s is not recorded: %v", identity, h.summary())
	return nil
}

func (h *harness) marker(identity string) any {
	h.t.Helper()
	v, _ := h.recorded(identity).Manifest.(*doc.Object).Get(markerKey)
	return v
}

// files are the files of the workspace under ll/ and mods/.
func (h *harness) files() []string {
	var out []string
	for rel, content := range h.snapshot() {
		if content != "<dir>" && (strings.HasPrefix(rel, "ll/") || strings.HasPrefix(rel, "mods/")) {
			out = append(out, rel)
		}
	}
	return sortedStrings(out)
}

func sortedStrings(s []string) []string {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return s
}

func mustFail(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil || !strings.Contains(FormatError(err), substr) {
		t.Fatalf("want a failure with %q, got %v", substr, err)
	}
}

func TestRoleCanonical(t *testing.T) {
	for v, want := range map[string]string{"client": "", "client_lua": "lua", "": "", "lua": "lua",
		"clientx": "clientx", "client_": "client_", "server": "server"} {
		if got := roleCanonical(v); got != want {
			t.Errorf("roleCanonical(%q) = %q, want %q", v, got, want)
		}
	}
}

// Off, the engine is lip: "X#client" of a tooth.json needs a client variant,
// and nothing is marked.
func TestCompatOff(t *testing.T) {
	h := compatHarness(t, false)
	h.addLipLL("2.0.0")
	h.addToothMod(modID, "1.0.0", dep(llID+"#client", "2.*"))
	mustFail(t, h.install(modID), `no entry of the variant "client"`)

	h = compatHarness(t, false)
	h.addToothLL("1.0.0")
	h.addToothMod(modID, "1.0.0", dep(llID+"#client", "1.*"))
	h.must(h.install(modID))
	equal(t, h.summary(), []string{llID + "#client@1.0.0 dep", modID + "@1.0.0 explicit"})
	if h.marker(modID) != nil {
		t.Fatal("marked without the layer")
	}
}

// R1: a tooth.json mod's "X#client" gets "" of a lip.toml release, as "X".
func TestR1AgainstLipTomlRelease(t *testing.T) {
	h := compatHarness(t, true)
	h.addLipLL("2.0.0")
	h.addToothMod(modID, "1.0.0", dep(llID+"#client", "2.*"))
	h.must(h.install(modID))
	equal(t, h.summary(), []string{llID + "@2.0.0 dep", modID + "@1.0.0 explicit"})
	equal(t, h.files(), []string{"ll/client-2.0.0.dll", "mods/mod.dll"})
	if p := h.recorded(llID); p.Ref != llID || p.Variant != "" {
		t.Fatalf("recorded %q %q", p.Ref, p.Variant)
	}
	if h.marker(modID) != "tooth.json" || h.marker(llID) != nil {
		t.Fatalf("markers %v %v", h.marker(modID), h.marker(llID))
	}
	// Planning again (a new process: the mod is a record now) keeps it.
	before := h.snapshot()
	h.setCompat(true)
	h.must(h.update())
	equalMaps(t, h.snapshot(), before)
	equal(t, h.summary(), []string{llID + "@2.0.0 dep", modID + "@1.0.0 explicit"})
	// Uninstalling the mod takes the dependency along.
	h.must(h.uninstall(modID))
	equal(t, h.summary(), nil)
	equal(t, h.files(), nil)
}

// R1 with a name after the role: "client_lua" -> "lua"; other variants are
// literal.
func TestR1WithNamedVariant(t *testing.T) {
	h := compatHarness(t, true)
	h.addTag(llID, "2.0.0", "lip.toml", lipVariants(llID, "2.0.0",
		"lua", "["+entry("client", "l.dll", "ll/lua.dll", "")+"]",
		"foo", "["+entry("client", "f.dll", "ll/foo.dll", "")+"]"), "l.dll", "lua", "f.dll", "foo")
	h.addToothMod(modID, "1.0.0", fmt.Sprintf(`{ %q: "2.*", %q: "2.*" }`, llID+"#client_lua", llID+"#foo"))
	h.must(h.install(modID))
	equal(t, h.summary(), []string{llID + "#foo@2.0.0 dep", llID + "#lua@2.0.0 dep", modID + "@1.0.0 explicit"})
	if p := h.recorded(llID + "#lua"); p.Ref != llID+"#lua" {
		t.Fatalf("recorded %q", p.Ref)
	}
}

// A tooth.json release of X is literal: "X#client" is its client label, and
// a release without one is not a candidate (no fallback to "").
func TestToothJSONTargetStaysLiteral(t *testing.T) {
	h := compatHarness(t, true)
	h.addToothLL("1.0.0")
	h.addToothMod(modID, "1.0.0", dep(llID+"#client", "1.*"))
	h.must(h.install(modID))
	equal(t, h.summary(), []string{llID + "#client@1.0.0 dep", modID + "@1.0.0 explicit"})
	equal(t, h.files(), []string{"ll/client-1.0.0.dll", "mods/mod.dll"})

	h = compatHarness(t, true)
	h.addTag(llID, "1.0.0", "tooth.json", toothLabels(llID, "1.0.0", "", "", "s.dll", "ll/server.dll"), "s.dll", "s")
	h.addToothMod(modID, "1.0.0", dep(llID+"#client", "1.*"))
	mustFail(t, h.install(modID), `no entry of the variant "client"`)
}

// A lip.toml's bare "X" against a tooth.json release is literal: the label
// "" (the server build), the author's error.
func TestLipTomlBareDependencyOnToothJSONStaysLiteral(t *testing.T) {
	h := compatHarness(t, true)
	h.addToothLL("1.0.0")
	h.addLipMod(newID, "1.0.0", dep(llID, "1.*"))
	h.must(h.install(newID))
	equal(t, h.summary(), []string{llID + "@1.0.0 dep", newID + "@1.0.0 explicit"})
	equal(t, h.files(), []string{"ll/server-1.0.0.dll", "mods/new.dll"})
}

// A lip.toml's "X#client" against a lip.toml release without a client
// variant fails as in lip; against a tooth.json release it gets the client
// build, and its record is marked so that it stays literal later.
func TestLipTomlClientDependencyIsLiteral(t *testing.T) {
	h := compatHarness(t, true)
	h.addLipLL("2.0.0")
	h.addLipMod(newID, "1.0.0", dep(llID+"#client", "2.*"))
	mustFail(t, h.install(newID), `no entry of the variant "client"`)

	h = compatHarness(t, true)
	h.addToothLL("1.0.0")
	h.addLipMod(newID, "1.0.0", dep(llID+"#client", ">=1.0.0"))
	h.must(h.install(newID))
	equal(t, h.summary(), []string{llID + "#client@1.0.0 dep", newID + "@1.0.0 explicit"})
	if h.marker(newID) != "lip.toml" {
		t.Fatalf("marker %v", h.marker(newID))
	}
	// A lip.toml release of X appears: the lip.toml mod still needs the
	// client variant, so X#client cannot be replaced by it (lip would install
	// both, whose files would clash).
	h.addLipLL("2.0.0")
	h.setCompat(true)
	before := h.snapshot()
	mustFail(t, h.update(llID+"@2.0.0"), "still need it")
	mustFail(t, h.install(llID+"@2.0.0"), "still need it")
	equalMaps(t, h.snapshot(), before)
}

// Unification: the tooth.json mod's "X#client" and the lip.toml mod's "X"
// are one package.
func TestUnificationInstallsOnePackage(t *testing.T) {
	h := compatHarness(t, true)
	h.addLipLL("2.0.0")
	h.addToothMod(modID, "1.0.0", dep(llID+"#client", "2.*"))
	h.addLipMod(newID, "1.0.0", dep(llID, "2.*"))
	h.must(h.install(modID, newID))
	equal(t, h.summary(), []string{llID + "@2.0.0 dep", modID + "@1.0.0 explicit", newID + "@1.0.0 explicit"})
	equal(t, h.files(), []string{"ll/client-2.0.0.dll", "mods/mod.dll", "mods/new.dll"})

	// In either order, and over an installed "X".
	h = compatHarness(t, true)
	h.addLipLL("2.0.0")
	h.addToothMod(modID, "1.0.0", dep(llID+"#client", "2.*"))
	h.addLipMod(newID, "1.0.0", dep(llID, "2.*"))
	h.must(h.install(newID))
	h.setCompat(true)
	h.must(h.install(modID))
	equal(t, h.summary(), []string{llID + "@2.0.0 dep", modID + "@1.0.0 explicit", newID + "@1.0.0 explicit"})
}

// An old state file (written without the layer, so without markers): the
// record "X#client" of a tooth.json release is updated in place to a
// lip.toml release "X", by update or install of "X", explicit or not.
func TestOldClientRecordUpdatedInPlace(t *testing.T) {
	for _, tc := range []struct {
		name     string
		explicit bool
		op       func(h *harness) error
	}{
		{"update explicit", true, func(h *harness) error { return h.update(llID + "@2.0.0") }},
		{"install explicit", true, func(h *harness) error { return h.install(llID + "@2.0.0") }},
		{"install dependency", false, func(h *harness) error { return h.install(llID + "@2.0.0") }},
		{"update dependency", false, func(h *harness) error { return h.update(llID) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := compatHarness(t, false)
			h.addToothLL("1.0.0")
			h.addToothMod(modID, "1.0.0", dep(llID+"#client", ">=1.0.0"))
			if tc.explicit {
				h.must(h.install(llID+"#client", modID))
			} else {
				h.must(h.install(modID))
			}
			h.addLipLL("2.0.0")
			h.setCompat(true)
			h.must(tc.op(h))
			equal(t, h.summary(), []string{llID + "@2.0.0 explicit", modID + "@1.0.0 explicit"})
			equal(t, h.files(), []string{"ll/client-2.0.0.dll", "mods/mod.dll"})
			if p := h.recorded(llID); p.Ref != llID && p.Ref != llID+"@2.0.0" {
				t.Fatalf("recorded %q", p.Ref)
			}
			// It stays.
			before := h.snapshot()
			h.setCompat(true)
			h.must(h.update())
			equalMaps(t, h.snapshot(), before)
		})
	}
}

// Without the layer, the same update fails as in lip; with it, a request
// read from a tooth.json does not replace anything.
func TestOldClientRecordWithoutCompat(t *testing.T) {
	h := compatHarness(t, false)
	h.addToothLL("1.0.0")
	h.addToothMod(modID, "1.0.0", dep(llID+"#client", ">=1.0.0"))
	h.must(h.install(llID+"#client", modID))
	h.addLipLL("2.0.0")
	h.setCompat(false)
	mustFail(t, h.update(llID+"@2.0.0"), "is not installed")

	// The bare id of a tooth.json release is its server build, which is not
	// the client record's replacement.
	h = compatHarness(t, false)
	h.addToothLL("1.0.0")
	h.must(h.install(llID + "#client"))
	h.addToothLL("1.1.0")
	h.setCompat(true)
	mustFail(t, h.update(llID+"@1.1.0"), "is not installed")
}

// Old records are uninstalled by their recorded identity, and installing the
// recorded identity again (a backup restore) gets the same package back.
func TestUninstallAndRestoreOldRecords(t *testing.T) {
	h := compatHarness(t, false)
	h.addToothLL("1.0.0")
	h.addToothMod(modID, "1.0.0", dep(llID+"#client", "1.*"))
	h.must(h.install(llID+"#client", modID))
	h.setCompat(true)
	h.must(h.uninstall(modID))
	equal(t, h.summary(), []string{llID + "#client@1.0.0 explicit"})
	h.must(h.uninstall(llID + "#client"))
	equal(t, h.summary(), nil)
	equal(t, h.files(), nil)

	h.must(h.install(llID+"#client@1.0.0", modID+"@1.0.0"))
	equal(t, h.summary(), []string{llID + "#client@1.0.0 explicit", modID + "@1.0.0 explicit"})
	equal(t, h.files(), []string{"ll/client-1.0.0.dll", "mods/mod.dll"})
}

// The server side is untouched with the layer on.
func TestServerSideUntouched(t *testing.T) {
	h := newHarness(t)
	h.e.opts.LegacyRoleVariants = true
	h.addLipLL("2.0.0")
	h.addTag(modID, "1.0.0", "tooth.json", toothLabels(modID, "1.0.0", "", dep(llID+"#client", "2.*"),
		"m.dll", "mods/mod.dll"), "m.dll", "mod")
	mustFail(t, h.install(modID), `no entry of the variant "client"`)

	h = newHarness(t)
	h.e.opts.LegacyRoleVariants = true
	h.addToothLL("1.0.0")
	h.addTag(modID, "1.0.0", "tooth.json", toothLabels(modID, "1.0.0", "", dep(llID, "1.*"),
		"m.dll", "mods/mod.dll"), "m.dll", "mod")
	h.must(h.install(modID))
	equal(t, h.summary(), []string{llID + "@1.0.0 dep", modID + "@1.0.0 explicit"})
	if h.marker(modID) != nil {
		t.Fatal("marked on a server workspace")
	}
}

// With --no-dependencies, a tooth.json mod is ordered after the lip.toml
// package R1 satisfies its dependency with.
func TestInstallationOrderFollowsR1(t *testing.T) {
	h := compatHarness(t, true)
	h.addLipLL("2.0.0")
	h.addToothMod("github.com/compat/aaa", "1.0.0", dep(llID+"#client", "2.*"))
	ctx := context.Background()
	h.must(h.e.Install(ctx, []string{"github.com/compat/aaa"}, false, false, true))
	h.setCompat(true)
	h.must(h.e.Install(ctx, []string{llID}, false, false, true))
	var order []string
	for _, p := range h.lock().Packages {
		order = append(order, stringMember(p.Manifest, "id"))
	}
	equal(t, order, []string{llID, "github.com/compat/aaa"})
}

// The shape of a package file derived from a tooth.json; a lip.toml with
// server entries, or other keys, does not have it.
func TestShapedDerived(t *testing.T) {
	env := environment{Side: pkgmodel.SideClient, Platform: testPlatform}
	check := func(s string, want bool) {
		t.Helper()
		v, err := doc.ParseJSON([]byte(s), "x")
		if err != nil {
			t.Fatal(err)
		}
		pf, err := pkgfile.Parse(v)
		if err != nil {
			t.Fatal(err)
		}
		if got := shapedDerived(v.(*doc.Object), pf, env); got != want {
			t.Fatalf("shapedDerived(%s) = %v", s, got)
		}
	}
	check(lipVariants(llID, "1.0.0", "", "["+entry("client", "s.dll", "s.dll", "")+"]",
		"client", "["+entry("client", "c.dll", "c.dll", "")+"]"), true)
	check(lipVariants(llID, "1.0.0", "", "["+entry("server", "s.dll", "s.dll", "")+", "+
		entry("client", "c.dll", "c.dll", "")+"]"), false)
	check(strings.Replace(lipVariants(llID, "1.0.0", "", "["+entry("client", "s.dll", "s.dll", "")+"]"),
		`"format": 4,`, `"format": 4, "x": 1,`, 1), false)
}
