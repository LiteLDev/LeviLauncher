package versions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

// Legacy-mode writes keep both files in step, an older launcher's later
// edit of version.json wins and is moved in, and a "-legacy" folder name is
// synced into the metadata.
func TestHunt_runtime_consumers_MetaAcrossModes(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "X-legacy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version.json"), []byte(`{"name":"X","launchArgs":"a"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ms, err := ScanVersions(root)
	if err != nil || len(ms) != 1 || ms[0].Name != "X-legacy" {
		t.Fatalf("ScanVersions = %+v, %v", ms, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "version.json")); !os.IsNotExist(err) {
		t.Fatal("version.json not moved to instance.json")
	}

	apppath.SetLegacyMode(root)
	t.Cleanup(func() { apppath.SetLegacyMode("") })
	m, _ := ReadMeta(dir)
	m.LaunchArgs = "b"
	if err := WriteMeta(dir, m); err != nil {
		t.Fatal(err)
	}
	cur, _ := os.ReadFile(filepath.Join(dir, ".levilauncher", "instance.json"))
	leg, _ := os.ReadFile(filepath.Join(dir, "version.json"))
	if string(cur) != string(leg) || !strings.Contains(string(leg), `"b"`) {
		t.Fatalf("legacy-mode write out of step: %s / %s", cur, leg)
	}
	apppath.SetLegacyMode("")

	// An older launcher edits version.json afterwards.
	later := time.Now().Add(time.Minute)
	if err := os.WriteFile(filepath.Join(dir, "version.json"), []byte(`{"name":"X-legacy","launchArgs":"c"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(filepath.Join(dir, "version.json"), later, later)
	m, err = ReadMeta(dir)
	if err != nil || m.LaunchArgs != "c" {
		t.Fatalf("ReadMeta = %+v, %v; want the newer version.json", m, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "version.json")); !os.IsNotExist(err) {
		t.Fatal("newer version.json not moved in")
	}
}
