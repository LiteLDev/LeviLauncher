package cli

import (
	"io"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipindex"
)

// modListItem is one element of `mod list --json`: a local mod, or the
// mods installed from one catalogue package.
type modListItem struct {
	// Ref is what other mod commands accept for this entry: the package
	// (owner/repo) or the folder.
	Ref      string `json:"ref"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Latest   string `json:"latest"`
	Outdated bool   `json:"outdated"`
	// Source is "catalog" for mods installed from the catalogue, else
	// "local".
	Source  string          `json:"source"`
	Package *modPackageInfo `json:"package"`
	Enabled bool            `json:"enabled"`
	// State is on, off or partial (some folders of a package disabled).
	State   string          `json:"state"`
	Folders []string        `json:"folders"`
	Mods    []modFolderInfo `json:"mods"`
}

type modPackageInfo struct {
	ID               string `json:"id"`
	Installed        bool   `json:"installed"`
	Explicit         bool   `json:"explicit"`
	InstalledVersion string `json:"installedVersion"`
}

type modFolderInfo struct {
	Folder  string `json:"folder"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
	Entry   string `json:"entry"`
	Author  string `json:"author"`
	Enabled bool   `json:"enabled"`
}

func init() {
	Register(&Command{
		Path:    "mod list",
		Summary: "list the mods of an instance",
		Args:    "[<filter>]",
		Description: `Mods installed from the LeviLamina mod catalogue are shown once per package,
with the newest stable version the catalogue offers; mods added from files
are shown by folder. LeviLamina itself is managed with 'loader'.

<filter> matches part of a mod's name, version, folder or package, ignoring
case. The catalogue is cached for 30 minutes; --refresh fetches it again.
Without network the list falls back to local information with a warning.`,
		Examples: []string{"mod list", "mod list --outdated", "mod list -i Survival --json"},
		Flags: []*Flag{
			BoolFlag("enabled", "", "only mods that are turned on"),
			BoolFlag("outdated", "", "only mods with a newer catalogue version"),
			EnumFlag("source", "", []string{"local", "catalog"}, "only mods added from files (local) or from the catalogue"),
			BoolFlag("refresh", "", "fetch the mod catalogue again instead of using the cached copy"),
		},
		MaxArgs: 1,
		Run:     runModList,
	})
}

func runModList(c *Ctx) error {
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	v, err := modLoadView(c, inst, !inst.IsUWP(), c.Bool("refresh"))
	if err != nil {
		return err
	}
	if v.catErr != nil {
		c.Warn("%s; showing local information only", v.catErr.Message)
	} else if v.stateErr != "" && !inst.IsUWP() {
		c.Warn("%s; showing local information only", modBackendError(v.stateErr).Message)
	}
	items := modListItems(v)
	filter := strings.ToLower(strings.TrimSpace(c.Arg(0)))
	picked := []modListItem{}
	for _, it := range items {
		if c.Bool("enabled") && !it.Enabled {
			continue
		}
		if c.Bool("outdated") && !it.Outdated {
			continue
		}
		if s := c.String("source"); s != "" && s != it.Source {
			continue
		}
		if filter != "" && !modItemMatches(it, filter) {
			continue
		}
		picked = append(picked, it)
	}
	ids := make([]string, len(picked))
	for i, it := range picked {
		ids[i] = it.Ref
	}
	return c.Render(picked, ids, func(w io.Writer) error {
		if len(picked) == 0 {
			if len(items) == 0 {
				c.Notice("No mods in %s. Find some with: %s mod search", inst.Name, ProgramName)
			} else {
				c.Notice("No mods match.")
			}
			return nil
		}
		rows := make([][]string, 0, len(picked))
		for _, it := range picked {
			src := it.Source
			if it.Package != nil && it.Source == "catalog" {
				src = lipShortID(it.Package.ID)
				if !it.Package.Explicit {
					src += " (dependency)"
				}
			}
			latest := Dash(it.Latest)
			if it.Latest != "" && !it.Outdated {
				latest = "up to date"
			}
			rows = append(rows, []string{it.Name, Dash(it.Version), latest, src, it.State, strings.Join(it.Folders, ", ")})
		}
		return Table(w, []string{"MOD", "VERSION", "LATEST", "SOURCE", "STATE", "FOLDER"}, rows)
	})
}

func modItemMatches(it modListItem, filter string) bool {
	hay := []string{it.Name, it.Version, it.Ref}
	hay = append(hay, it.Folders...)
	if it.Package != nil {
		hay = append(hay, it.Package.ID)
	}
	for _, h := range hay {
		if strings.Contains(strings.ToLower(h), filter) {
			return true
		}
	}
	return false
}

func modState(all, any bool) string {
	switch {
	case all:
		return "on"
	case any:
		return "partial"
	}
	return "off"
}

// modOutdated is whether mod update would update a mod: never one newer than
// the catalogue's latest stable version, which mod update holds back.
func modOutdated(canUpdate bool, installed, target string) bool {
	return canUpdate && !modUpdateWouldDowngrade(installed, target)
}

// modListItems turns a view into list entries: packages first, then local
// mods, each sorted by name.
func modListItems(v *modView) []modListItem {
	out := []modListItem{}
	for _, g := range v.listing.Groups {
		it := modListItem{
			Ref: lipShortID(g.Identifier), Name: g.PackageName, Version: g.InstalledVersion,
			Latest: g.TargetVersion, Outdated: modOutdated(g.CanUpdate, g.InstalledVersion, g.TargetVersion), Source: "catalog",
			Package: &modPackageInfo{ID: g.Identifier, Installed: true, Explicit: g.Explicit, InstalledVersion: g.InstalledVersion},
			Enabled: g.AllEnabled, State: modState(g.AllEnabled, g.AnyEnabled),
			Folders: append([]string{}, g.Folders...), Mods: []modFolderInfo{},
		}
		for _, m := range g.Mods {
			it.Mods = append(it.Mods, modFolderOf(v, m.Folder))
		}
		out = append(out, it)
	}
	for _, item := range v.listing.Items {
		f := modFolderOf(v, item.Folder)
		it := modListItem{
			Ref: item.Folder, Name: modFirstNonEmpty(item.Mod.Name, item.Folder), Version: item.Mod.Version,
			Source: "local", Enabled: f.Enabled, State: modState(f.Enabled, f.Enabled),
			Folders: []string{item.Folder}, Mods: []modFolderInfo{f},
		}
		if item.State.Source == lipindex.SourceUnique {
			it.Latest, it.Outdated = item.State.TargetVersion, modOutdated(item.State.CanUpdate, item.Mod.Version, item.State.TargetVersion)
			it.Package = &modPackageInfo{ID: item.State.Identifier}
		}
		out = append(out, it)
	}
	return out
}

func modFolderOf(v *modView, folder string) modFolderInfo {
	m, _ := v.modByFolder(folder)
	return modFolderInfo{Folder: folder, Name: m.Name, Version: m.Version, Type: m.Type, Entry: m.Entry,
		Author: m.Author, Enabled: v.enabled[folder]}
}
