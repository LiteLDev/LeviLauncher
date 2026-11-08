// Package backupmods builds the mods manifest of an instance backup: which
// mod folders belong to which installed LIP package, at which version, so a
// restore reinstalls those packages through lip instead of copying their
// folders. It is the Go port of the window's buildBackupLipPackagesFromSnapshot
// (frontend/src/hooks/useInstanceSettings.ts), with one difference: a
// package's folders come from the files its workspace records (lip 0.38's
// .lip/state.json), which is exact and needs no network, and only fall
// back to name matching when no files are recorded. The window matched mods
// to packages through the online LIP index instead.
package backupmods

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	json "github.com/goccy/go-json"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/lipcore/install"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// LeviLaminaIdentifier is how the manifest names LeviLamina, as the window
// does.
const LeviLaminaIdentifier = "LiteLDev/LeviLamina"

const leviLaminaModName = "levilamina"

// ErrModsStateUnavailable reports that the installed packages could not be
// mapped to mod folders (lip could not be queried, or LeviLamina is
// installed but none of its folders or its version is known). The backup
// would otherwise lose the packages' identity.
var ErrModsStateUnavailable = errors.New("ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE")

// Mod is a mod folder of an instance with its manifest.
type Mod struct {
	Folder  string
	Name    string
	Version string
}

// Package is an installed LIP package.
type Package struct {
	ID       string // package id: "github.com/Owner/Repo"
	Variant  string // "" or e.g. "client"
	Version  string
	Explicit bool
	// Folders are the mod folders the workspace records as the package's
	// files (mods/<folder>/...); nil when it records none.
	Folders []string
}

// Build returns the backup's LIP packages, sorted by identifier
// (case-insensitively). A package whose folders or version cannot be told is
// left out: its folders are then backed up as plain folders. LeviLamina is
// the exception: when it is installed but cannot be mapped, Build fails with
// ErrModsStateUnavailable, as the window does.
func Build(pkgs []Package, mods []Mod) ([]types.InstanceBackupModsLIPPackage, error) {
	modByFolder := map[string]Mod{}
	for _, m := range mods {
		modByFolder[strings.ToLower(m.Folder)] = m
	}
	out := []types.InstanceBackupModsLIPPackage{}
	seen := map[string]bool{}
	for _, p := range pkgs {
		identifier, ok := Identifier(p.ID, p.Variant)
		if !ok {
			continue // not a GitHub package: restore cannot reinstall it
		}
		key := lookupKey(p.ID)
		if seen[key] {
			continue
		}
		isLL := key == lookupKey(LeviLaminaIdentifier)
		folders := existingFolders(p.Folders, modByFolder)
		if len(folders) == 0 && p.Folders == nil && !isLL {
			folders = matchByName(p.ID, mods)
		}
		if isLL {
			identifier = LeviLaminaIdentifier
			for _, m := range mods {
				if strings.EqualFold(strings.TrimSpace(m.Name), leviLaminaModName) {
					folders = append(folders, m.Folder)
				}
			}
			folders = dedupeFolders(folders)
		}
		version := strings.TrimSpace(p.Version)
		if version == "" {
			version = uniqueVersion(folders, modByFolder)
		}
		if len(folders) == 0 || version == "" {
			if isLL {
				return nil, ErrModsStateUnavailable
			}
			continue
		}
		seen[key] = true
		out = append(out, types.InstanceBackupModsLIPPackage{
			Identifier:        identifier,
			Version:           version,
			ExplicitInstalled: p.Explicit,
			Folders:           folders,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Identifier) < strings.ToLower(out[j].Identifier)
	})
	return out, nil
}

// Identifier is the manifest identifier of a GitHub package: "Owner/Repo",
// with "#variant" for a variant other than client. ok is false for a
// package that is not on GitHub.
func Identifier(id, variant string) (string, bool) {
	s := strings.TrimSpace(id)
	low := strings.ToLower(s)
	for _, p := range []string{"https://", "http://"} {
		if strings.HasPrefix(low, p) {
			s, low = s[len(p):], low[len(p):]
		}
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '/' })
	switch {
	case len(parts) == 3 && strings.EqualFold(parts[0], "github.com"):
		parts = parts[1:]
	case len(parts) == 2 && !strings.Contains(parts[0], "."):
	default:
		return "", false
	}
	out := parts[0] + "/" + parts[1]
	if v := strings.TrimSpace(variant); v != "" && !strings.EqualFold(v, "client") {
		out += "#" + v
	}
	return out, true
}

// lookupKey is normalizeIdentifierLookupKey: the lower-case package id with
// "github.com/" for an "Owner/Repo" shorthand.
func lookupKey(id string) string {
	k := strings.ToLower(strings.TrimSpace(id))
	k = strings.TrimPrefix(strings.TrimPrefix(k, "https://"), "http://")
	if i := strings.IndexAny(k, "#@"); i >= 0 {
		k = k[:i]
	}
	parts := strings.FieldsFunc(k, func(r rune) bool { return r == '/' })
	if len(parts) == 2 && !strings.Contains(parts[0], ".") {
		return "github.com/" + strings.Join(parts, "/")
	}
	return strings.Join(parts, "/")
}

func existingFolders(folders []string, mods map[string]Mod) []string {
	var out []string
	for _, f := range folders {
		if m, ok := mods[strings.ToLower(f)]; ok {
			out = append(out, m.Folder)
		}
	}
	return dedupeFolders(out)
}

func dedupeFolders(folders []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range folders {
		f = strings.TrimSpace(f)
		if f == "" || seen[strings.ToLower(f)] {
			continue
		}
		seen[strings.ToLower(f)] = true
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

func normalizeName(s string) string {
	r := strings.NewReplacer("-", "", "_", "", " ", "", ".", "")
	return r.Replace(strings.ToLower(strings.TrimSpace(s)))
}

// matchByName finds the mod folders of a package whose workspace records no
// files: those whose folder or manifest name is the repository name, ignoring
// case, '-', '_', '.' and spaces.
func matchByName(id string, mods []Mod) []string {
	key := lookupKey(id)
	repo := key[strings.LastIndex(key, "/")+1:]
	want := normalizeName(repo)
	if want == "" {
		return nil
	}
	var out []string
	for _, m := range mods {
		if normalizeName(m.Folder) == want || normalizeName(m.Name) == want {
			out = append(out, m.Folder)
		}
	}
	return dedupeFolders(out)
}

// uniqueVersion is the version all the folders' manifests agree on, or "".
func uniqueVersion(folders []string, mods map[string]Mod) string {
	version := ""
	for _, f := range folders {
		v := strings.TrimSpace(mods[strings.ToLower(f)].Version)
		if v == "" {
			continue
		}
		if version != "" && v != version {
			return ""
		}
		version = v
	}
	return version
}

// ScanMods lists the mod folders of modsDir with their manifest (enabled
// manifest.json, or manifest.json.close for a disabled mod). It only reads:
// a missing folder is no mods.
func ScanMods(modsDir string) []Mod {
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil
	}
	var out []Mod
	for _, e := range entries {
		if !e.IsDir() || fsx.IsTempName(e.Name()) {
			continue
		}
		m := Mod{Folder: e.Name()}
		for _, name := range []string{"manifest.json", "manifest.json.close"} {
			b, err := os.ReadFile(filepath.Join(modsDir, e.Name(), name))
			if err != nil {
				continue
			}
			var mf types.ModManifestJson
			if json.Unmarshal(utils.JsonCompatBytes(b), &mf) == nil {
				m.Name, m.Version = strings.TrimSpace(mf.Name), strings.TrimSpace(mf.Version)
				break
			}
		}
		out = append(out, m)
	}
	return out
}

// Seams replaced by tests.
var (
	listPackageStates = lip.ListPackageStates
	loadWorkspace     = pkgfile.LoadWorkspace
)

// InstalledPackages returns the LIP packages installed in instanceDir with
// the mod folders their workspace records. An error means lip could not
// tell (ErrModsStateUnavailable wraps it).
func InstalledPackages(ctx context.Context, instanceDir string) ([]Package, error) {
	states, err := listPackageStates(ctx, instanceDir)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.Join(ErrModsStateUnavailable, err)
	}
	folders := map[string][]string{}
	recorded := map[string]bool{}
	if lf, ok, err := loadWorkspace(instanceDir); err == nil && ok && lf != nil {
		for i := range lf.Packages {
			p := &lf.Packages[i]
			spec, ok := lip.ParsePackageSpec(install.LockedIdentity(p))
			if !ok {
				continue
			}
			k := lip.PackageKey(spec.ID)
			recorded[k] = true
			for _, f := range p.Files {
				if folder, ok := modFolderOf(f); ok {
					folders[k] = append(folders[k], folder)
				}
			}
		}
	}
	keys := make([]string, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Package, 0, len(keys))
	for _, k := range keys {
		s := states[k]
		if !s.Installed {
			continue
		}
		p := Package{ID: s.ID, Variant: s.Variant, Version: s.InstalledVersion, Explicit: s.ExplicitInstalled}
		if recorded[k] {
			p.Folders = append([]string{}, folders[k]...)
		}
		out = append(out, p)
	}
	return out, nil
}

// modFolderOf returns <folder> of a state file "mods/<folder>/...".
func modFolderOf(file string) (string, bool) {
	f := strings.ReplaceAll(strings.TrimSpace(file), `\`, "/")
	parts := strings.Split(f, "/")
	if len(parts) < 3 || !strings.EqualFold(parts[0], "mods") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
