package mcservice

// Regression tests for the group "conflicts-existing-layout" at the level
// the launcher uses instances: after the storage migration placed a legacy
// instance under a "-legacy" name (K2), or migrated instances named like
// directories of the layout (K4), every instance is listed under its own
// name, and looking it up or reconciling it touches its own folder only.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/storage"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

type celMcEnv struct{ roaming, local, legacy string }

func celMcSetup(t *testing.T) celMcEnv {
	t.Helper()
	root := t.TempDir()
	e := celMcEnv{roaming: filepath.Join(root, "Roaming"), local: filepath.Join(root, "Local")}
	for _, d := range []string{e.roaming, e.local} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("APPDATA", e.roaming)
	t.Setenv("LOCALAPPDATA", e.local)
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	t.Cleanup(func() {
		apppath.SetLegacyMode("")
		apppath.SetBaseRootOverride("")
	})
	e.legacy = apppath.LegacyDir()
	return e
}

func celMcWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func celMcRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// celMcStartup does what main.go does: Prepare, EnsureDirs, config.Load.
func celMcStartup(t *testing.T, want storage.Outcome) {
	t.Helper()
	if o, err := storage.Prepare(); err != nil || o != want {
		t.Fatalf("Prepare = %v, %v; want %v", o, err, want)
	}
	if err := apppath.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Reload(); err != nil {
		t.Fatal(err)
	}
}

func TestMigration_K2_ListAndReconcileAfterNameCollision(t *testing.T) {
	e := celMcSetup(t)
	// First start of the new version migrates A.
	celMcWrite(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
	celMcWrite(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"name":"A","gameVersion":"1.21.0","type":"release"}`)
	celMcWrite(t, filepath.Join(e.legacy, "versions", "A", "x"), "new")
	celMcStartup(t, storage.Migrated)
	// Downgrade: the old version installs and registers A again.
	celMcWrite(t, filepath.Join(e.legacy, "config.json"), `{"base_root":""}`)
	celMcWrite(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"name":"A","gameVersion":"1.20.0","type":"release","registered":true}`)
	celMcWrite(t, filepath.Join(e.legacy, "versions", "A", "x"), "old")
	celMcStartup(t, storage.Migrated)

	vdir, err := apppath.InstancesDir()
	if err != nil {
		t.Fatal(err)
	}
	if !celMcSamePath(vdir, filepath.Join(e.local, "LeviLauncher", "Instances")) {
		t.Fatalf("InstancesDir = %s", vdir)
	}
	if got := celMcRead(t, filepath.Join(vdir, "A", "x")); got != "new" {
		t.Errorf("Instances\\A\\x = %q", got)
	}
	if got := celMcRead(t, filepath.Join(vdir, "A-legacy", "x")); got != "old" {
		t.Errorf("Instances\\A-legacy\\x = %q", got)
	}

	metas := ListVersionMetas()
	byName := map[string]versions.VersionMeta{}
	for _, m := range metas {
		t.Logf("listed: name=%q gameVersion=%q registered=%v", m.Name, m.GameVersion, m.Registered)
		if _, dup := byName[m.Name]; dup {
			t.Errorf("two listed instances are named %q", m.Name)
		}
		byName[m.Name] = m
	}
	if len(metas) != 2 {
		t.Fatalf("listed %d instances, want 2", len(metas))
	}
	if byName["A"].GameVersion != "1.21.0" || byName["A-legacy"].GameVersion != "1.20.0" {
		t.Errorf("listed %v", byName)
	}
	if got := GetVersionMeta("A-legacy"); got.Name != "A-legacy" || got.GameVersion != "1.20.0" {
		t.Errorf("GetVersionMeta(A-legacy) = %+v", got)
	}
	if got := GetVersionMeta("A"); got.Name != "A" || got.GameVersion != "1.21.0" {
		t.Errorf("GetVersionMeta(A) = %+v", got)
	}

	// Run at every start: it writes each instance's metadata back to the
	// folder of the listed name.
	ReconcileRegisteredFlags()
	a, err := versions.ReadMeta(filepath.Join(vdir, "A"))
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "A" || a.GameVersion != "1.21.0" {
		t.Errorf("Instances\\A metadata = %+v after ReconcileRegisteredFlags; want A's own (1.21.0)", a)
	}
	l, err := versions.ReadMeta(filepath.Join(vdir, "A-legacy"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Name != "A-legacy" || l.GameVersion != "1.20.0" {
		t.Errorf("Instances\\A-legacy metadata = %+v", l)
	}
	// The old version registered its A at versions\A: that path now names
	// the -legacy folder.
	if got := storage.TranslateLegacyPath(filepath.Join(e.legacy, "versions", "A")); !celMcSamePath(got, filepath.Join(vdir, "A-legacy")) {
		t.Errorf("TranslateLegacyPath(versions\\A) = %s, want %s", got, filepath.Join(vdir, "A-legacy"))
	}
	if got := celMcRead(t, filepath.Join(vdir, "A", "x")); got != "new" {
		t.Errorf("Instances\\A\\x = %q after the startup", got)
	}

	// The next start is a no-op.
	if o, err := storage.Prepare(); err != nil || o != storage.Current {
		t.Errorf("next Prepare = %v, %v; want Current", o, err)
	}
}

func TestMigration_K4_ListInstancesNamedLikeLayoutDirs(t *testing.T) {
	e := celMcSetup(t)
	names := []string{"Instances", "Cache", "Backups", ".levilauncher", "Instances-legacy", ".uwp-install-123", ".msixvc-staging-9"}
	celMcWrite(t, filepath.Join(e.legacy, "config.json"), `{}`)
	for _, n := range names {
		celMcWrite(t, filepath.Join(e.legacy, "versions", n, "version.json"), `{"name":"`+n+`","gameVersion":"1.21.0","type":"release"}`)
		celMcWrite(t, filepath.Join(e.legacy, "versions", n, "payload"), "data-"+n)
	}
	celMcStartup(t, storage.Migrated)

	listed := map[string]bool{}
	for _, m := range ListVersionMetas() {
		listed[m.Name] = true
	}
	for _, n := range names {
		if !listed[n] {
			t.Errorf("instance %q not listed: %v", n, listed)
		}
		if got := GetVersionMeta(n); got.Name != n {
			t.Errorf("GetVersionMeta(%q).Name = %q", n, got.Name)
		}
		vdir, _ := apppath.InstancesDir()
		if got := celMcRead(t, filepath.Join(vdir, n, "payload")); got != "data-"+n {
			t.Errorf("%s payload = %q", n, got)
		}
	}
	if len(listed) != len(names) {
		t.Errorf("listed %d instances, want %d", len(listed), len(names))
	}
	ReconcileRegisteredFlags()
	vdir, _ := apppath.InstancesDir()
	for _, n := range names {
		if m, err := versions.ReadMeta(filepath.Join(vdir, n)); err != nil || m.Name != n {
			t.Errorf("ReadMeta(%s) = %+v, %v", n, m, err)
		}
	}
	if o, err := storage.Prepare(); err != nil || o != storage.Current {
		t.Errorf("next Prepare = %v, %v; want Current", o, err)
	}
}

func celMcSamePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b) || celMcEqualFold(a, b)
}

func celMcEqualFold(a, b string) bool {
	ca, cb := filepath.Clean(a), filepath.Clean(b)
	if len(ca) != len(cb) {
		return false
	}
	for i := 0; i < len(ca); i++ {
		x, y := ca[i], cb[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
