package lip

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
)

// oldStateJSON is the state file a launcher before v1.1.0-beta.4 wrote: the
// side is "platform" and the platform "runtime", both at the root and in the
// entries of the package files it records.
const oldStateJSON = `{
  "format": 4,
  "platform": "client",
  "runtime": "win-x64",
  "packages": [
    {
      "manifest": {
        "format": 4,
        "id": "github.com/example/alpha",
        "version": "1.2.3",
        "info": {
          "name": "Alpha",
          "description": "the first test package",
          "tags": ["test"]
        },
        "variants": {
          "": [
            {
              "platform": "client",
              "runtime": "win-x64",
              "assets": [{"type": "self"}]
            },
            {
              "platform": "client",
              "runtime": "win-arm64",
              "assets": [{"type": "self"}]
            }
          ]
        }
      },
      "variant": "",
      "explicit": true,
      "ref": "github.com/example/alpha",
      "commit": "0123456789abcdef0123456789abcdef01234567",
      "files": ["mods/alpha/alpha.dll"]
    },
    {
      "manifest": {
        "format": 4,
        "id": "github.com/example/beta",
        "version": "2.0.0",
        "variants": {
          "client": [
            {
              "platform": "client",
              "runtime": "win-x64",
              "assets": [{
                "type": "zip",
                "urls": ["https://example.com/beta.zip"],
                "place": [{"src": "beta.dll", "dest": "mods/beta/beta.dll"}]
              }]
            }
          ]
        }
      },
      "variant": "client",
      "explicit": false,
      "ref": "github.com/example/beta#client",
      "commit": "fedcba9876543210fedcba9876543210fedcba98",
      "files": []
    }
  ]
}
`

func writeWorkspaceState(t *testing.T, root, content string) string {
	t.Helper()
	dir := filepath.Join(root, ".lip")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMigrateWorkspaceStateConvertsOldKeys(t *testing.T) {
	root := t.TempDir()
	path := writeWorkspaceState(t, root, oldStateJSON)

	migrated, err := migrateWorkspaceState(root)
	if err != nil {
		t.Fatalf("migrateWorkspaceState: %v", err)
	}
	if !migrated {
		t.Fatal("state file with the old keys was not migrated")
	}

	lf, exists, err := pkgfile.LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace after the migration: %v", err)
	}
	if !exists {
		t.Fatal("state file missing after the migration")
	}
	if lf.Side != "client" || lf.Platform != "win-x64" {
		t.Fatalf("workspace side/platform = %q/%q, want client/win-x64", lf.Side, lf.Platform)
	}
	if len(lf.Packages) != 2 {
		t.Fatalf("%d packages, want 2", len(lf.Packages))
	}

	alpha, err := pkgfile.Parse(lf.Packages[0].Manifest)
	if err != nil {
		t.Fatalf("alpha manifest: %v", err)
	}
	entries, ok := alpha.EntriesOf("")
	if !ok || len(entries) != 2 {
		t.Fatalf("alpha variant \"\": %+v", alpha.Variants)
	}
	if entries[0].Side != "client" || entries[0].Platform != "win-x64" ||
		entries[1].Side != "client" || entries[1].Platform != "win-arm64" {
		t.Fatalf("alpha entries = %+v", entries)
	}
	beta, err := pkgfile.Parse(lf.Packages[1].Manifest)
	if err != nil {
		t.Fatalf("beta manifest: %v", err)
	}
	if entries, ok := beta.EntriesOf("client"); !ok || len(entries) != 1 ||
		entries[0].Side != "client" || entries[0].Platform != "win-x64" {
		t.Fatalf("beta variant \"client\" = %+v", beta.Variants)
	}

	content := readFile(t, path)
	if strings.Contains(content, `"runtime"`) {
		t.Fatalf("migrated state file still has a runtime key:\n%s", content)
	}
	for _, key := range []string{`"format": 4`, `"side": "client"`, `"platform": "win-x64"`} {
		if !strings.Contains(content, key) {
			t.Errorf("migrated state file has no %s:\n%s", key, content)
		}
	}
	// The member order of lip's state file is kept.
	if strings.Index(content, `"side"`) > strings.Index(content, `"platform"`) {
		t.Errorf("side comes after platform:\n%s", content)
	}
	// No temporary file is left behind (the workspace lock file stays, as
	// lip keeps it).
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".tmp") {
			t.Errorf("temporary file left in .lip: %s", f.Name())
		}
	}

	// The conversion is done once.
	migrated, err = migrateWorkspaceState(root)
	if err != nil || migrated {
		t.Fatalf("second migrateWorkspaceState = %v, %v; want false, nil", migrated, err)
	}
}

func TestMigrateWorkspaceStateConvertsPackageFilesOnly(t *testing.T) {
	root := t.TempDir()
	// The root already uses the keys of lip v0.40, the recorded package file
	// does not.
	content := `{
  "format": 4,
  "side": "client",
  "platform": "win-x64",
  "packages": [{
    "manifest": {
      "format": 4,
      "id": "github.com/example/alpha",
      "version": "1.2.3",
      "variants": {"": [{"platform": "client", "runtime": "win-x64"}]}
    },
    "variant": "",
    "explicit": true,
    "ref": "github.com/example/alpha",
    "files": []
  }]
}
`
	path := writeWorkspaceState(t, root, content)
	migrated, err := migrateWorkspaceState(root)
	if err != nil || !migrated {
		t.Fatalf("migrateWorkspaceState = %v, %v; want true, nil", migrated, err)
	}
	if got := readFile(t, path); strings.Contains(got, `"runtime"`) {
		t.Fatalf("migrated state file still has a runtime key:\n%s", got)
	}
	if _, _, err := pkgfile.LoadWorkspace(root); err != nil {
		t.Fatalf("LoadWorkspace after the migration: %v", err)
	}
}

func TestMigrateWorkspaceStateLeavesNewStateAlone(t *testing.T) {
	root := t.TempDir()
	content := `{
  "format": 4,
  "side": "client",
  "platform": "win-x64",
  "packages": []
}
`
	path := writeWorkspaceState(t, root, content)
	migrated, err := migrateWorkspaceState(root)
	if err != nil || migrated {
		t.Fatalf("migrateWorkspaceState = %v, %v; want false, nil", migrated, err)
	}
	if got := readFile(t, path); got != content {
		t.Fatalf("new state file changed:\n%s", got)
	}
}

func TestMigrateWorkspaceStateLeavesUnreadableStateAlone(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"not JSON", "format = 4\n"},
		{"old keys but not a valid workspace", `{"format": 4, "platform": "client", "runtime": "win-x64"}`},
		{"old keys but invalid manifest", `{
  "format": 4,
  "platform": "client",
  "runtime": "win-x64",
  "packages": [{
    "manifest": {"format": 4, "id": "github.com/example/alpha", "version": "not a version", "variants": {}},
    "variant": "",
    "explicit": true,
    "ref": "github.com/example/alpha",
    "files": []
  }]
}
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := writeWorkspaceState(t, root, tc.content)
			migrated, err := migrateWorkspaceState(root)
			if err != nil || migrated {
				t.Fatalf("migrateWorkspaceState = %v, %v; want false, nil", migrated, err)
			}
			if got := readFile(t, path); got != tc.content {
				t.Fatalf("state file changed:\n%s", got)
			}
		})
	}
}

func TestMigrateWorkspaceStateWithoutStateFile(t *testing.T) {
	migrated, err := migrateWorkspaceState(t.TempDir())
	if err != nil || migrated {
		t.Fatalf("migrateWorkspaceState = %v, %v; want false, nil", migrated, err)
	}
}

func TestMigrateWorkspaceStateRunsBeforeList(t *testing.T) {
	root := t.TempDir()
	path := writeWorkspaceState(t, root, oldStateJSON)
	state := useFakeEngine(t)

	_, err := ListPackageStates(context.Background(), root)
	if err != nil {
		t.Fatalf("ListPackageStates: %v", err)
	}
	if got := readFile(t, path); strings.Contains(got, `"runtime"`) {
		t.Fatalf("ListPackageStates did not migrate the state file:\n%s", got)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.calls) != 1 || state.calls[0].method != "List" {
		t.Fatalf("engine calls = %+v", state.calls)
	}
}

func TestRenameOldKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
		// changed is false for an object already using the new keys.
		changed bool
	}{
		{
			name:    "old entry",
			in:      `{"platform":"client","runtime":"win-x64","assets":[]}`,
			want:    `{"side":"client","platform":"win-x64","assets":[]}`,
			changed: true,
		},
		{
			name:    "old side only",
			in:      `{"platform":"server","assets":[]}`,
			want:    `{"side":"server","assets":[]}`,
			changed: true,
		},
		{
			name:    "old platform only",
			in:      `{"runtime":"win-x64","assets":[]}`,
			want:    `{"platform":"win-x64","assets":[]}`,
			changed: true,
		},
		{
			name:    "new keys win over a left-over runtime",
			in:      `{"side":"client","platform":"win-x64","runtime":"win-x64","assets":[]}`,
			want:    `{"side":"client","platform":"win-x64","assets":[]}`,
			changed: true,
		},
		{
			name:    "new keys are kept",
			in:      `{"side":"client","platform":"win-x64","assets":[]}`,
			want:    `{"side":"client","platform":"win-x64","assets":[]}`,
			changed: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := doc.ParseJSONFile([]byte(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			got, changed := renameOldKeys(v.(*doc.Object))
			if changed != tc.changed {
				t.Fatalf("changed = %v, want %v", changed, tc.changed)
			}
			if text := string(doc.MarshalJSONCompact(got)); text != tc.want {
				t.Fatalf("renameOldKeys = %s, want %s", text, tc.want)
			}
		})
	}
}
