package versions

// Regression tests for scenario J6 of the group "journal-crash-downgrade":
// while the launcher runs on the legacy layout (apppath.SetLegacyMode, the
// migration could not complete), instance metadata stays in version.json,
// which is all an older launcher reads (internal/apppath/README.md, "以旧布局
// 运行时"). A downgrade after such a run must still see every instance.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

// legacyModeInstance runs on the legacy layout of a fresh %APPDATA% and
// returns <legacy>\versions\<name>, created.
func legacyModeInstance(t *testing.T, name string) (string, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(root, "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "Local"))
	apppath.SetBaseRootOverride("")
	legacy := apppath.LegacyDir()
	apppath.SetLegacyMode(legacy)
	t.Cleanup(func() {
		apppath.SetLegacyMode("")
		apppath.SetBaseRootOverride("")
	})
	versionsDir, err := apppath.InstancesDir()
	if err != nil {
		t.Fatal(err)
	}
	if !samePathFold(versionsDir, filepath.Join(legacy, "versions")) {
		t.Fatalf("legacy InstancesDir = %s", versionsDir)
	}
	dir := filepath.Join(versionsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return versionsDir, dir
}

func samePathFold(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestMigration_J6_LegacyModeReadMetaKeepsVersionJSON(t *testing.T) {
	versionsDir, a := legacyModeInstance(t, "A")
	const meta = `{"name":"A","gameVersion":"1.21.0","type":"release"}`
	legacyMeta := filepath.Join(a, "version.json")
	if err := os.WriteFile(legacyMeta, []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := ReadMeta(a)
	if err != nil || m.Name != "A" || m.GameVersion != "1.21.0" {
		t.Fatalf("ReadMeta = %+v, %v", m, err)
	}
	ms, err := ScanVersions(versionsDir)
	if err != nil || len(ms) != 1 || ms[0].Name != "A" {
		t.Fatalf("ScanVersions = %v, %v", ms, err)
	}
	// v1.0.x ReadMeta reads <instance>\version.json only.
	if got := mustReadFile(t, legacyMeta); got != meta {
		t.Errorf("version.json = %q, want it unchanged", got)
	}
	if _, err := os.Stat(apppath.InstanceStateDir(a)); err == nil {
		t.Error("a legacy-mode read created .levilauncher")
	}

	// Leaving legacy mode (the next start migrated): the first read moves
	// the content, unchanged, to .levilauncher\instance.json.
	apppath.SetLegacyMode("")
	if m, err := ReadMeta(a); err != nil || m.Name != "A" {
		t.Fatalf("ReadMeta after legacy mode = %+v, %v", m, err)
	}
	if got := mustReadFile(t, filepath.Join(a, ".levilauncher", "instance.json")); got != meta {
		t.Errorf("instance.json = %q", got)
	}
}

// An instance a newer start already gave instance.json, then changed by the
// older launcher (newer version.json): in legacy mode the newer version.json
// is read and both files are kept.
func TestMigration_J6_LegacyModeReadsNewerVersionJSONAndKeepsBoth(t *testing.T) {
	_, a := legacyModeInstance(t, "A")
	state := filepath.Join(a, ".levilauncher")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	const older = `{"name":"A","gameVersion":"1.20.0"}`
	const newer = `{"name":"A","gameVersion":"1.21.0"}`
	cur := filepath.Join(state, "instance.json")
	leg := filepath.Join(a, "version.json")
	if err := os.WriteFile(cur, []byte(older), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(leg, []byte(newer), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(cur, past, past); err != nil {
		t.Fatal(err)
	}

	m, err := ReadMeta(a)
	if err != nil || m.GameVersion != "1.21.0" {
		t.Fatalf("ReadMeta = %+v, %v; want the newer version.json", m, err)
	}
	if got := mustReadFile(t, leg); got != newer {
		t.Errorf("version.json = %q", got)
	}
	if got := mustReadFile(t, cur); got != older {
		t.Errorf("instance.json = %q", got)
	}
}

// Writes in legacy mode go to version.json, so that an older launcher sees
// them, and keep an existing instance.json in step.
func TestMigration_J6_LegacyModeWriteMetaWritesVersionJSON(t *testing.T) {
	versionsDir, a := legacyModeInstance(t, "A")
	if err := WriteMeta(a, VersionMeta{Name: "A", GameVersion: "1.21.0"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a, "version.json")); err != nil {
		t.Fatalf("WriteMeta in legacy mode did not write version.json: %v", err)
	}
	if _, err := os.Stat(apppath.InstanceStateDir(a)); err == nil {
		t.Error("WriteMeta in legacy mode created .levilauncher")
	}

	// A folder renamed outside the launcher: the folder name is persisted
	// where the older launcher reads it.
	b := filepath.Join(versionsDir, "B")
	if err := os.Rename(a, b); err != nil {
		t.Fatal(err)
	}
	ms, err := ScanVersions(versionsDir)
	if err != nil || len(ms) != 1 || ms[0].Name != "B" {
		t.Fatalf("ScanVersions = %v, %v", ms, err)
	}
	var stored VersionMeta
	data := mustReadFile(t, filepath.Join(b, "version.json"))
	if err := json.Unmarshal([]byte(data), &stored); err != nil || stored.Name != "B" {
		t.Errorf("version.json = %q (%v), want name B", data, err)
	}
	if _, err := os.Stat(apppath.InstanceStateDir(b)); err == nil {
		t.Error("a legacy-mode scan created .levilauncher")
	}
}
