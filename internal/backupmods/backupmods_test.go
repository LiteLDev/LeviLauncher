package backupmods

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
)

func TestIdentifier(t *testing.T) {
	cases := []struct {
		id, variant, want string
		ok                bool
	}{
		{"github.com/LiteLDev/LegacyMoney", "", "LiteLDev/LegacyMoney", true},
		{"github.com/LiteLDev/LeviLamina", "client", "LiteLDev/LeviLamina", true},
		{"github.com/a/b", "server", "a/b#server", true},
		{"https://github.com/a/b", "", "a/b", true},
		{"a/b", "", "a/b", true},
		{"example.com/a/b", "", "", false},
		{"gitlab.com/a/b", "", "", false},
	}
	for _, c := range cases {
		got, ok := Identifier(c.id, c.variant)
		if got != c.want || ok != c.ok {
			t.Errorf("Identifier(%q, %q) = %q, %v; want %q, %v", c.id, c.variant, got, ok, c.want, c.ok)
		}
	}
}

func TestBuildFromRecordedFiles(t *testing.T) {
	mods := []Mod{
		{Folder: "LegacyMoney", Name: "LegacyMoney", Version: "0.9.0"},
		{Folder: "LeviLamina", Name: "LeviLamina", Version: "1.4.0"},
		{Folder: "MyOwnMod", Name: "MyOwnMod", Version: "1.0.0"},
		{Folder: "zeta", Name: "Zeta", Version: "2.0.0"},
	}
	pkgs := []Package{
		{ID: "github.com/LiteLDev/LegacyMoney", Version: "0.9.1", Explicit: true, Folders: []string{"LegacyMoney", "LegacyMoney"}},
		{ID: "github.com/LiteLDev/LeviLamina", Variant: "client", Version: "1.4.0", Folders: []string{}},
		// No recorded files: matched by repository name, version from the manifest.
		{ID: "github.com/someone/Zeta"},
		// A package with recorded files of which none is a mod folder.
		{ID: "github.com/x/Tool", Version: "1.0.0", Folders: []string{}},
		// Not on GitHub: left to the plain-folder backup.
		{ID: "example.com/a/b", Version: "1", Folders: []string{"MyOwnMod"}},
	}
	got, err := Build(pkgs, mods)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id, version string
		explicit    bool
		folders     []string
	}
	var rows []row
	for _, p := range got {
		rows = append(rows, row{p.Identifier, p.Version, p.ExplicitInstalled, p.Folders})
	}
	want := []row{
		{"LiteLDev/LegacyMoney", "0.9.1", true, []string{"LegacyMoney"}},
		{"LiteLDev/LeviLamina", "1.4.0", false, []string{"LeviLamina"}},
		{"someone/Zeta", "2.0.0", false, []string{"zeta"}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("Build = %+v\nwant %+v", rows, want)
	}
}

func TestBuildLeviLaminaUnmappable(t *testing.T) {
	_, err := Build([]Package{{ID: "github.com/LiteLDev/LeviLamina", Version: "1.0.0", Folders: []string{}}}, nil)
	if !errors.Is(err, ErrModsStateUnavailable) {
		t.Fatalf("err = %v, want ErrModsStateUnavailable", err)
	}
	// Installed through the mods folder but without a version anywhere.
	_, err = Build([]Package{{ID: "github.com/LiteLDev/LeviLamina"}}, []Mod{{Folder: "LeviLamina", Name: "LeviLamina"}})
	if !errors.Is(err, ErrModsStateUnavailable) {
		t.Fatalf("err = %v, want ErrModsStateUnavailable", err)
	}
}

func TestBuildVersionNeedsAgreement(t *testing.T) {
	mods := []Mod{{Folder: "a", Version: "1"}, {Folder: "b", Version: "2"}}
	got, err := Build([]Package{{ID: "github.com/o/r", Folders: []string{"a", "b"}}}, mods)
	if err != nil || len(got) != 0 {
		t.Fatalf("Build = %+v, %v; want none (no single version)", got, err)
	}
}

func TestModFolderOf(t *testing.T) {
	for in, want := range map[string]string{
		"mods/LegacyMoney/LegacyMoney.dll": "LegacyMoney",
		`mods\X\manifest.json`:             "X",
		"mods/onlyfile":                    "",
		"plugins/a/b":                      "",
	} {
		got, ok := modFolderOf(in)
		if got != want || ok != (want != "") {
			t.Errorf("modFolderOf(%q) = %q, %v", in, got, ok)
		}
	}
}

func TestScanMods(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, data string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("A/manifest.json", `{"name":"Alpha","version":"1.2.3"}`)
	write("B/manifest.json.close", `{"name":"Beta","version":"0.1"}`)
	write("C/readme.txt", "x")
	write("D.tmp/manifest.json", `{"name":"tmp"}`)
	got := ScanMods(dir)
	want := []Mod{{"A", "Alpha", "1.2.3"}, {"B", "Beta", "0.1"}, {"C", "", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ScanMods = %+v, want %+v", got, want)
	}
	if ScanMods(filepath.Join(dir, "missing")) != nil {
		t.Fatal("a missing mods folder must be no mods")
	}
}

func TestInstalledPackages(t *testing.T) {
	savedList, savedLoad := listPackageStates, loadWorkspace
	t.Cleanup(func() { listPackageStates, loadWorkspace = savedList, savedLoad })

	listPackageStates = func(context.Context, string) (map[string]lip.PackageInstallState, error) {
		return map[string]lip.PackageInstallState{
			lip.PackageKey("github.com/LiteLDev/LegacyMoney"): {ID: "github.com/LiteLDev/LegacyMoney", Installed: true, ExplicitInstalled: true, InstalledVersion: "0.9.1"},
			lip.PackageKey("github.com/o/legacy"):             {ID: "github.com/o/legacy", Installed: true, InstalledVersion: "1.0.0"},
		}, nil
	}
	manifest := doc.NewObject()
	manifest.Set("id", "github.com/LiteLDev/LegacyMoney")
	manifest.Set("version", "0.9.1")
	loadWorkspace = func(string) (*pkgmodel.LockFile, bool, error) {
		return &pkgmodel.LockFile{Packages: []pkgmodel.LockedPackage{{
			Manifest: manifest, Explicit: true,
			Files: []string{"mods/LegacyMoney/LegacyMoney.dll", "mods/LegacyMoney/manifest.json", "config/x.json"},
		}}}, true, nil
	}
	got, err := InstalledPackages(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d packages", len(got))
	}
	byID := map[string]Package{}
	for _, p := range got {
		byID[p.ID] = p
	}
	if f := byID["github.com/LiteLDev/LegacyMoney"].Folders; !reflect.DeepEqual(f, []string{"LegacyMoney", "LegacyMoney"}) {
		t.Errorf("recorded folders = %v", f)
	}
	if f := byID["github.com/o/legacy"].Folders; f != nil {
		t.Errorf("a package without recorded files must have nil folders, got %v", f)
	}

	listPackageStates = func(context.Context, string) (map[string]lip.PackageInstallState, error) {
		return nil, errors.New("broken workspace")
	}
	if _, err := InstalledPackages(context.Background(), t.TempDir()); !errors.Is(err, ErrModsStateUnavailable) {
		t.Fatalf("err = %v, want ErrModsStateUnavailable", err)
	}
}
