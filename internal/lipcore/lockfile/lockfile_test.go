package lockfile

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/stjson"
)

// useFakes makes the manifest hooks independent of packages semver/glob:
// versions and ranges are returned unchanged and every glob is valid.
func useFakes(t *testing.T) {
	t.Helper()
	nv, nr, vg := manifest.NormalizeVersion, manifest.NormalizeRange, manifest.ValidateGlob
	nl := stjson.NewLine
	t.Cleanup(func() {
		manifest.NormalizeVersion, manifest.NormalizeRange, manifest.ValidateGlob = nv, nr, vg
		stjson.NewLine = nl
	})
	manifest.NormalizeVersion = func(s string) (string, error) {
		if s == "" {
			return "", errors.New("empty version")
		}
		return s, nil
	}
	manifest.NormalizeRange = func(s string) (string, error) {
		if s == "" {
			return "", errors.New("empty range")
		}
		return s, nil
	}
	manifest.ValidateGlob = func(string) error { return nil }
	// lip on Windows writes CRLF.
	stjson.NewLine = "\r\n"
}

// testdata/tooth_lock.json is a real lock file written by lip 0.37.0
// (LeviLamina client install), with the user name in absolute paths
// replaced. Load -> Save must reproduce it byte for byte.
func TestGoldenRoundTrip(t *testing.T) {
	useFakes(t)
	want, err := os.ReadFile(filepath.Join("testdata", FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(want, []byte("\r\n")) {
		t.Fatal("golden file lost its CRLF line endings (check .gitattributes)")
	}
	s, err := Load("testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Packages) != 6 {
		t.Fatalf("got %d packages, want 6", len(s.Packages))
	}
	ll := s.Packages[s.Find(manifest.PackageID{Path: "github.com/LiteLDev/LeviLamina", Variant: "client"})]
	if !ll.Locked || ll.Spec().String() != "github.com/LiteLDev/LeviLamina#client@26.51.9" {
		t.Errorf("LeviLamina package = %+v", ll.Spec())
	}
	if !strings.Contains(ll.Manifest.Variants[1].Scripts.PostUninstall[0], " && ") {
		t.Errorf("script not unescaped: %q", ll.Manifest.Variants[1].Scripts.PostUninstall[0])
	}

	dir := t.TempDir()
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("round trip differs at byte %d", firstDiff(got, want))
	}
}

func firstDiff(a, b []byte) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

func TestLoadMissing(t *testing.T) {
	s, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if s.FormatVersion != 3 || s.FormatUUID != manifest.FormatUUID || s.Packages == nil || len(s.Packages) != 0 {
		t.Errorf("Load(missing) = %+v", s)
	}
}

const manifestJSON = `{"format_version": 3, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d", "tooth": "example.com/x", "version": "1.0.0"}`

func TestLoadMalformed(t *testing.T) {
	useFakes(t)
	pkg := func(body string) string { return `{"packages": [` + body + `]}` }
	tests := map[string]string{
		"not json":             `{`,
		"null":                 `null`,
		"missing packages":     `{"format_version": 3}`,
		"bad format version":   `{"format_version": 2, "packages": []}`,
		"bad format uuid":      `{"format_uuid": "x", "packages": []}`,
		"package missing file": pkg(`{"locked": true, "manifest": ` + manifestJSON + `, "variant": ""}`),
		"locked null":          pkg(`{"files": [], "locked": null, "manifest": ` + manifestJSON + `, "variant": ""}`),
		"locked string":        pkg(`{"files": [], "locked": "true", "manifest": ` + manifestJSON + `, "variant": ""}`),
		"missing manifest":     pkg(`{"files": [], "locked": true, "variant": ""}`),
		"bad manifest":         pkg(`{"files": [], "locked": true, "manifest": {}, "variant": ""}`),
		"missing variant":      pkg(`{"files": [], "locked": true, "manifest": ` + manifestJSON + `}`),
		"bad variant":          pkg(`{"files": [], "locked": true, "manifest": ` + manifestJSON + `, "variant": "A"}`),
	}
	for name, doc := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, FileName), []byte(doc), 0o644); err != nil {
				t.Fatal(err)
			}
			s, err := Load(dir)
			if err == nil {
				t.Fatal("Load succeeded")
			}
			if s == nil || len(s.Packages) != 0 || s.FormatVersion != 3 {
				t.Errorf("Load must return an empty state on error, got %+v", s)
			}
		})
	}
}

func TestUnmarshalOptionalFormatAndBOM(t *testing.T) {
	useFakes(t)
	s, err := Unmarshal([]byte("\xef\xbb\xbf" + `{"packages": [{"files": ["a"], "locked": true, "manifest": ` + manifestJSON + `, "variant": "client"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Packages) != 1 || s.Packages[0].Spec().String() != "example.com/x#client@1.0.0" {
		t.Fatalf("got %+v", s.Packages)
	}
	if s.Find(manifest.PackageID{Path: "example.com/x"}) != -1 || s.Find(manifest.PackageID{Path: "example.com/x", Variant: "client"}) != 0 {
		t.Error("Find mismatch")
	}
}

// A List running alongside an install may have tooth_lock.json open while
// Save replaces it; on Windows that rename fails until the reader closes.
func TestSaveWhileReaderHasFileOpen(t *testing.T) {
	useFakes(t)
	dir := t.TempDir()
	if err := New().Save(dir); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(60 * time.Millisecond)
		f.Close()
	}()
	if err := New().Save(dir); err != nil {
		t.Fatalf("Save with a concurrent reader: %v", err)
	}
}

// List does not take the per-instance lock, so Load can run while Save
// renames over the file; on Windows opening it then fails briefly.
func TestLoadDuringConcurrentSaves(t *testing.T) {
	useFakes(t)
	dir := t.TempDir()
	if err := New().Save(dir); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	saveErr := make(chan error, 1)
	go func() {
		defer close(saveErr)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := New().Save(dir); err != nil {
				saveErr <- err
				return
			}
		}
	}()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := Load(dir); err != nil {
			close(stop)
			t.Fatalf("Load during Save: %v", err)
		}
	}
	close(stop)
	if err := <-saveErr; err != nil {
		t.Fatalf("Save during Load: %v", err)
	}
}

func TestSaveRetriesFailedRename(t *testing.T) {
	useFakes(t)
	oldRename, oldBackoff := rename, renameBackoff
	t.Cleanup(func() { rename, renameBackoff = oldRename, oldBackoff })
	renameBackoff = 0
	failures := 3
	rename = func(from, to string) error {
		if failures > 0 {
			failures--
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: os.ErrPermission}
		}
		return os.Rename(from, to)
	}
	dir := t.TempDir()
	if err := New().Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1", len(entries))
	}

	rename = func(from, to string) error {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: os.ErrPermission}
	}
	if err := New().Save(dir); err == nil {
		t.Fatal("Save succeeded although every rename failed")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temp file left behind: %d entries", len(entries))
	}

	// Other errors are permanent and returned at once.
	calls := 0
	rename = func(from, to string) error {
		calls++
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: os.ErrNotExist}
	}
	if err := New().Save(dir); err == nil || calls != 1 {
		t.Fatalf("Save = %v after %d renames, want an error after 1", err, calls)
	}
}

func TestSaveNewState(t *testing.T) {
	useFakes(t)
	stjson.NewLine = "\n"
	s := &State{Packages: []Package{{
		Manifest: manifest.Manifest{Tooth: "example.com/x", Version: "1.0.0"},
	}}}
	dir := filepath.Join(t.TempDir(), "nested", "instance")
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "format_version": 3,
  "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
  "packages": [
    {
      "files": [],
      "locked": false,
      "manifest": {
        "format_version": 3,
        "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
        "tooth": "example.com/x",
        "version": "1.0.0",
        "info": {
          "name": "",
          "description": "",
          "tags": [],
          "avatar_url": ""
        },
        "variants": []
      },
      "variant": ""
    }
  ]
}`
	if string(got) != want {
		t.Errorf("Save wrote\n%s\nwant\n%s", got, want)
	}
	// Saving again replaces the file and leaves no temp files behind.
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1", len(entries))
	}

	empty, err := New().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != "{\n  \"format_version\": 3,\n  \"format_uuid\": \"289f771f-2c9a-4d73-9f3f-8492495a924d\",\n  \"packages\": []\n}" {
		t.Errorf("New().Marshal() = %s", empty)
	}
}
