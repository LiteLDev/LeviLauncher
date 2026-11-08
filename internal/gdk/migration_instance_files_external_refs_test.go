package gdk

// Regression tests for group "instance-files-external-refs": GDK
// registrations Windows recorded at the legacy location, after the storage
// migration (internal/apppath/README.md, "从旧布局迁移").

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/storage"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

func migGDKWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// migGDKIsMoved is IsMovedRegistration (wdapp.go) with the InstallLocation
// Windows reports passed in instead of read from the registry.
func migGDKIsMoved(installLocation, folder string) bool {
	if !storage.IsMovedLegacyPath(installLocation) {
		return false
	}
	return normalizeInstallPath(storage.TranslateLegacyPath(installLocation)) == normalizeInstallPath(folder)
}

// migGDKUnregisterMatches is the comparison UnregisterVersionFolder makes.
func migGDKUnregisterMatches(installLocation, folder string) bool {
	return normalizeInstallPath(storage.TranslateLegacyPath(installLocation)) == normalizeInstallPath(filepath.Clean(strings.TrimSpace(folder)))
}

func migGDKPrepare(t *testing.T, want storage.Outcome) {
	t.Helper()
	if o, err := storage.Prepare(); err != nil || o != want {
		t.Fatalf("Prepare = %v, %v; want %v", o, err, want)
	}
}

// P7-registered-packages (GDK): a registration recorded at versions\<n> is
// the moved instance's, in every spelling Windows uses; a registration of a
// folder that a pre-PR rename moved away matches nothing; after a second
// migration the registration at versions\A belongs to A-legacy.
func TestMigration_P7_GDKRegistrationAfterMigration(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(root, "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "Local"))
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	S := apppath.LegacyDir()
	migGDKWrite(t, filepath.Join(S, "config.json"), `{}`)
	migGDKWrite(t, filepath.Join(S, "versions", "A", "version.json"), `{"name":"A","registered":true}`)
	migGDKWrite(t, filepath.Join(S, "versions", "A", "MicrosoftGame.config"), `<Game/>`)
	migGDKWrite(t, filepath.Join(S, "versions", "New", "version.json"), `{"name":"New"}`)
	migGDKPrepare(t, storage.Migrated)
	inst, err := apppath.InstancesDir()
	if err != nil {
		t.Fatal(err)
	}
	instA, instNew := filepath.Join(inst, "A"), filepath.Join(inst, "New")
	if m, err := versions.ReadMeta(instA); err != nil || !m.Registered || m.PackageType != versions.PackageTypeGDK {
		t.Errorf("A meta = %+v, %v", m, err)
	}

	legacyA := filepath.Join(S, "versions", "A")
	for _, loc := range []string{legacyA, strings.ToUpper(legacyA), `\\?\` + legacyA, legacyA + `\`, filepath.ToSlash(legacyA)} {
		if !migGDKIsMoved(loc, instA) {
			t.Errorf("registration at %s is not the moved Instances\\A", loc)
		}
		if migGDKIsMoved(loc, instNew) {
			t.Errorf("registration at %s matches New", loc)
		}
		if !migGDKUnregisterMatches(loc, instA) || migGDKUnregisterMatches(loc, instNew) {
			t.Errorf("UnregisterVersionFolder comparison for %s is wrong", loc)
		}
		// UnregisterIfExists reads the instance where it is now.
		if got := storage.TranslateLegacyPath(loc); versions.DetectPackageType(got, versions.VersionMeta{}) != versions.PackageTypeGDK || !samePathGDK(got, instA) {
			t.Errorf("Translate(%s) = %s", loc, got)
		}
	}
	// Renamed away before the migration (Old -> New): Windows still records
	// versions\Old.
	for _, f := range []string{instA, instNew} {
		if migGDKIsMoved(filepath.Join(S, "versions", "Old"), f) || migGDKUnregisterMatches(filepath.Join(S, "versions", "Old"), f) {
			t.Errorf("stale Old registration matches %s", f)
		}
	}
	// A registration at the current location is not "moved", but it is A's.
	if migGDKIsMoved(instA, instA) {
		t.Error("current-location registration reported as moved")
	}
	if !migGDKUnregisterMatches(instA, instA) {
		t.Error("current-location registration does not match A")
	}

	// Downgrade, then upgrade: the older launcher created and registered a
	// different versions\A; the second migration puts it at A-legacy.
	migGDKWrite(t, filepath.Join(S, "config.json"), `{}`)
	migGDKWrite(t, filepath.Join(S, "versions", "A", "version.json"), `{"name":"A","registered":true,"gameVersion":"1.20.0.1"}`)
	migGDKWrite(t, filepath.Join(S, "versions", "A", "MicrosoftGame.config"), `<Game second/>`)
	migGDKPrepare(t, storage.Migrated)
	instAL := filepath.Join(inst, "A-legacy")
	if b, err := os.ReadFile(filepath.Join(instAL, "MicrosoftGame.config")); err != nil || string(b) != `<Game second/>` {
		t.Fatalf("A-legacy: %q, %v", b, err)
	}
	if migGDKIsMoved(legacyA, instA) || migGDKUnregisterMatches(legacyA, instA) {
		t.Error("the registration of the second versions\\A (now A-legacy) is attributed to Instances\\A")
	}
	if !migGDKIsMoved(legacyA, instAL) || !migGDKUnregisterMatches(legacyA, instAL) {
		t.Error("the registration of versions\\A is not matched to Instances\\A-legacy")
	}
	migGDKPrepare(t, storage.Current)
}

func samePathGDK(a, b string) bool {
	return normalizeInstallPath(a) == normalizeInstallPath(b)
}
