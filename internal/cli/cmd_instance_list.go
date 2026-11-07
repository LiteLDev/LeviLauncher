package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// instanceListItem is one element of `instance list --json`.
type instanceListItem struct {
	Name          string  `json:"name"`
	GameVersion   string  `json:"gameVersion"`
	Channel       string  `json:"channel"`
	Type          string  `json:"type"`
	Isolated      bool    `json:"isolated"`
	Registered    bool    `json:"registered"`
	Running       bool    `json:"running"`
	PIDs          []int   `json:"pids"`
	LoaderVersion *string `json:"loaderVersion"`
	Console       bool    `json:"console"`
	EditorMode    bool    `json:"editorMode"`
	HasLogo       bool    `json:"hasLogo"`
	LogoPath      *string `json:"logoPath"`
	Path          string  `json:"path"`
	IsDefault     bool    `json:"isDefault"`
}

// Seams replaced by tests: registration and process state come from the
// system.
var (
	registeredLocations = mcservice.RegisteredPackageLocations
	minecraftProcesses  = mcservice.ListMinecraftProcesses
)

var channelOrder = map[string]int{"release": 0, "beta": 1, "preview": 2}

func init() {
	Register(&Command{
		Path:    "instance list",
		Summary: "list installed instances",
		Args:    "[<filter>]",
		Description: `The default instance is marked with '*'. <filter> matches part of the name,
game version, channel or type, ignoring case. Instances are sorted newest
game version first, release before beta and preview, then by name.`,
		Examples: []string{"instance list", "instance list --channel preview", "instance list --json", "instance ls -q"},
		Flags: []*Flag{
			EnumsFlag("channel", "", []string{"release", "beta", "preview"}, "only instances of this channel"),
			EnumFlag("type", "", []string{"gdk", "uwp"}, "only GDK or UWP instances"),
		},
		MaxArgs: 1,
		Run:     runInstanceList,
	})
}

func runInstanceList(c *Ctx) error {
	all, err := ListInstances()
	if err != nil {
		return err
	}
	filter := strings.ToLower(strings.TrimSpace(c.Arg(0)))
	channels := c.Strings("channel")
	typ := c.String("type")
	var picked []Instance
	for _, inst := range all {
		if len(channels) > 0 && !containsFold(channels, inst.Channel) {
			continue
		}
		if typ != "" && !strings.EqualFold(typ, inst.PackageType) {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(strings.Join([]string{
			inst.Name, inst.Meta.GameVersion, inst.Channel, inst.PackageType}, "\x00")), filter) {
			continue
		}
		picked = append(picked, inst)
	}
	sort.SliceStable(picked, func(i, j int) bool {
		a, b := picked[i], picked[j]
		if d := versions.CompareGameVersions(a.Meta.GameVersion, b.Meta.GameVersion); d != 0 {
			return d > 0
		}
		if channelOrder[a.Channel] != channelOrder[b.Channel] {
			return channelOrder[a.Channel] < channelOrder[b.Channel]
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	defaultName, _, _ := c.DefaultInstance()
	items := make([]instanceListItem, 0, len(picked))
	if len(picked) > 0 {
		release, preview := registeredLocations()
		pids := map[string][]int{}
		for _, p := range minecraftProcesses() {
			if p.IsLauncher && p.VersionName != "" {
				k := strings.ToLower(p.VersionName)
				pids[k] = append(pids[k], p.Pid)
			}
		}
		for _, inst := range picked {
			items = append(items, describeInstance(inst, defaultName, release, preview, pids[strings.ToLower(inst.Name)]))
		}
	}

	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.Name
	}
	if len(items) == 0 {
		if len(all) == 0 {
			c.Notice("No instance is installed. Install one: %s install latest", ProgramName)
		} else {
			c.Notice("No instance matches.")
		}
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		rows := make([][]string, len(items))
		for i, it := range items {
			mark := " "
			if it.IsDefault {
				mark = "*"
			}
			data := "shared"
			if it.Isolated {
				data = "own"
			}
			status := "-"
			if it.Running {
				status = "running(" + joinInts(it.PIDs) + ")"
			}
			loader := "-"
			if it.LoaderVersion != nil {
				loader = *it.LoaderVersion
			}
			rows[i] = []string{mark + " " + it.Name, Dash(it.GameVersion), Dash(it.Channel), it.Type, data, YesNo(it.Registered), loader, status}
		}
		return Table(w, []string{"  NAME", "GAME VERSION", "CHANNEL", "TYPE", "DATA", "REGISTERED", "LOADER", "STATUS"}, rows)
	})
}

func describeInstance(inst Instance, defaultName, releaseLoc, previewLoc string, pids []int) instanceListItem {
	loc := releaseLoc
	if inst.IsPreview() {
		loc = previewLoc
	}
	it := instanceListItem{
		Name: inst.Name, GameVersion: inst.Meta.GameVersion, Channel: inst.Channel, Type: inst.PackageType,
		Isolated: inst.Meta.EnableIsolation, Console: inst.Meta.EnableConsole, EditorMode: inst.Meta.EnableEditorMode,
		Registered: loc != "" && strings.EqualFold(utils.CanonicalWindowsPath(inst.Dir), loc),
		Running:    len(pids) > 0, PIDs: append([]int{}, pids...), Path: inst.Dir,
		IsDefault: defaultName != "" && strings.EqualFold(defaultName, inst.Name),
	}
	sort.Ints(it.PIDs)
	if v := LoaderVersion(inst); v != "" {
		it.LoaderVersion = &v
	}
	if logo := filepath.Join(inst.Dir, "LargeLogo.png"); utils.FileExists(logo) {
		it.HasLogo, it.LogoPath = true, &logo
	}
	return it
}

// LoaderVersion returns the version of the LeviLamina mod installed in the
// instance (enabled or disabled), "" when there is none. It only reads.
func LoaderVersion(inst Instance) string {
	modsDir := filepath.Join(inst.Dir, "mods")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() || fsx.IsTempName(e.Name()) {
			continue
		}
		for _, name := range []string{"manifest.json", "manifest.json.close"} {
			b, err := os.ReadFile(filepath.Join(modsDir, e.Name(), name))
			if err != nil {
				continue
			}
			var m types.ModManifestJson
			if json.Unmarshal(utils.JsonCompatBytes(b), &m) != nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(m.Name), "LeviLamina") {
				if v := strings.TrimSpace(m.Version); v != "" {
					return v
				}
				return "?"
			}
			break
		}
	}
	return ""
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func joinInts(n []int) string {
	s := make([]string, len(n))
	for i, v := range n {
		s[i] = fmt.Sprint(v)
	}
	return strings.Join(s, ",")
}
