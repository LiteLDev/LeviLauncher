package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/lipindex"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// The catalogue fixtures are for Windows x64, the launcher's platform;
// the tests run on any platform.
func init() { lipindex.TargetPlatform = "win-x64" }

// Catalogue documents for the mod command tests.
var modTestDetails = map[string]string{
	"github.com/LiteLDev/LegacyMoney": `{"id":"github.com/LiteLDev/LegacyMoney","name":"LegacyMoney","owner":"LiteLDev","stars":50,
		"description":"Economy","tags":["money"],"license":"MIT","updated":"2026-01-01T00:00:00Z",
		"side":["server","client"],"platform":["win-x64"],"gameVersions":["1.26.60","1.26.51"],"versions":[
		{"version":"0.9.2","variants":{
			"":[{"side":"server","platform":"win-x64","deps":{"github.com/LiteLDev/LeviLamina":"26.60.*"}}],
			"client":[{"side":"client","platform":"win-x64","deps":{"github.com/LiteLDev/LeviLamina#client":"26.60.*"}}]}},
		{"version":"0.9.1","variants":{
			"client":[{"side":"client","platform":"win-x64","deps":{"github.com/LiteLDev/LeviLamina#client":"26.51.*"}}]}}]}`,
	"github.com/Some/Chat": `{"id":"github.com/Some/Chat","name":"BetterChat","owner":"Some","stars":5,
		"side":["client"],"platform":["win-x64"],"versions":[
		{"version":"2.0.0","variants":{"":[{"side":"client","platform":"win-x64","deps":{"github.com/Some/Lib":"^1.0.0"}}]}}]}`,
}

type fakeModWorld struct {
	t         *testing.T
	h         *harness
	states    map[string]map[string]types.LIPPackageInstallState // instance -> lookup key -> state
	calls     []string
	catErr    error
	dbErr     error
	results   map[string]string // call prefix -> backend result
	installed map[string]string // lookup key -> version installed by fake installs
}

func newModWorld(t *testing.T) *fakeModWorld {
	h := newHarness(t)
	w := &fakeModWorld{t: t, h: h, states: map[string]map[string]types.LIPPackageInstallState{}, results: map[string]string{}}
	saved := modOps
	t.Cleanup(func() { modOps = saved })
	// In a fixed order: the catalogue's index order is the default order.
	details := func() []*lipindex.Detail {
		ids := make([]string, 0, len(modTestDetails))
		for id := range modTestDetails {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var out []*lipindex.Detail
		for _, id := range ids {
			var d lipindex.Detail
			if err := json.Unmarshal([]byte(modTestDetails[id]), &d); err != nil {
				t.Fatal(err)
			}
			out = append(out, &d)
		}
		return out
	}
	result := func(call string) string {
		w.calls = append(w.calls, call)
		for prefix, r := range w.results {
			if strings.HasPrefix(call, prefix) {
				return r
			}
		}
		return ""
	}
	modOps = modBackend{
		catalog: func(ctx context.Context, refresh bool) (*lipindex.Index, time.Time, bool, error) {
			if w.catErr != nil {
				return nil, time.Time{}, false, w.catErr
			}
			return lipindex.Build(details()), time.Now(), false, nil
		},
		fetchDetail: func(ctx context.Context, id string) (*lipindex.Detail, error) {
			for k, doc := range modTestDetails {
				if k == id {
					var d lipindex.Detail
					_ = json.Unmarshal([]byte(doc), &d)
					return &d, nil
				}
			}
			if w.catErr != nil {
				return nil, w.catErr
			}
			return nil, lipindex.ErrPackageNotFound
		},
		loaderDB: func() (map[string][]string, error) {
			if w.dbErr != nil {
				return nil, w.dbErr
			}
			return map[string][]string{"1.26.51.01": {"26.51.0", "26.51.1"}, "1.26.60.05": {"26.60.0", "26.60.4", "26.60.5-rc.1"}}, nil
		},
		installStates: func(ctx context.Context, inst Instance, ids []string) []types.LIPPackageInstallStateEntry {
			var out []types.LIPPackageInstallStateEntry
			for _, id := range ids {
				out = append(out, types.LIPPackageInstallStateEntry{IdentifierKey: id, State: w.states[inst.Name][lipindex.LookupKey(id)]})
			}
			return out
		},
		listPackages: func(ctx context.Context, inst Instance) (map[string]lip.PackageInstallState, error) {
			out := map[string]lip.PackageInstallState{}
			for k, s := range w.states[inst.Name] {
				out[k] = lip.PackageInstallState{ID: k, Installed: s.Installed, ExplicitInstalled: s.ExplicitInstalled}
			}
			return out, nil
		},
		installPackage: func(ctx context.Context, inst Instance, id, version string) string {
			return result(fmt.Sprintf("install %s %s@%s", inst.Name, id, version))
		},
		uninstallPackage: func(ctx context.Context, inst Instance, id string) string {
			return result(fmt.Sprintf("uninstall %s %s", inst.Name, id))
		},
		installLoader: func(ctx context.Context, inst Instance, game, version string) string {
			return result(fmt.Sprintf("loader %s %s %s", inst.Name, game, version))
		},
		readme: func(string) (string, error) { return "# fetched readme", nil },
		enable: func(inst Instance, folder string) string {
			m := filepath.Join(inst.Dir, "mods", folder, "manifest.json")
			if r := result("enable " + folder); r != "" {
				return r
			}
			return modRenameIf(m+".close", m)
		},
		disable: func(inst Instance, folder string) string {
			m := filepath.Join(inst.Dir, "mods", folder, "manifest.json")
			if r := result("disable " + folder); r != "" {
				return r
			}
			return modRenameIf(m, m+".close")
		},
		remove: func(inst Instance, folder string) string {
			if r := result("remove " + folder); r != "" {
				return r
			}
			_ = os.RemoveAll(filepath.Join(inst.Dir, "mods", folder))
			return ""
		},
		setManifest: func(inst Instance, folder, name, version, typ, entry, author string) string {
			r := result(fmt.Sprintf("set %s name=%s version=%s type=%s entry=%s author=%s", folder, name, version, typ, entry, author))
			if r == "" {
				modWriteMod(w.t, inst, folder, types.ModManifestJson{Name: name, Version: version, Type: typ, Entry: entry, Author: author}, true)
			}
			return r
		},
		importZip: func(inst Instance, data []byte, overwrite bool) string {
			return result(fmt.Sprintf("zip %s %s overwrite=%v", inst.Name, data, overwrite))
		},
		importDll: func(inst Instance, file string, data []byte, name, typ, version string, overwrite bool) string {
			return result(fmt.Sprintf("dll %s %s name=%s type=%s version=%s overwrite=%v", inst.Name, file, name, typ, version, overwrite))
		},
		openFolder: func(string) error { result("open"); return nil },
	}
	return w
}

func modRenameIf(from, to string) string {
	if _, err := os.Stat(from); err != nil {
		return ""
	}
	if err := os.Rename(from, to); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func modWriteMod(t *testing.T, inst Instance, folder string, m types.ModManifestJson, enabled bool) {
	t.Helper()
	dir := filepath.Join(inst.Dir, "mods", folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(m)
	name := "manifest.json"
	if !enabled {
		name += ".close"
	}
	_ = os.Remove(filepath.Join(dir, "manifest.json"))
	_ = os.Remove(filepath.Join(dir, "manifest.json.close"))
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// survival is a GDK instance with LeviLamina, LegacyMoney from the
// catalogue and two local mods.
func (w *fakeModWorld) survival() Instance {
	inst := w.h.add("Survival", "1.26.60.05", "release", versions.PackageTypeGDK)
	modWriteMod(w.t, inst, "LeviLamina", types.ModManifestJson{Name: "LeviLamina", Version: "26.60.4"}, true)
	modWriteMod(w.t, inst, "LegacyMoney", types.ModManifestJson{Name: "LegacyMoney", Version: "0.9.1", Entry: "LegacyMoney.dll", Type: "preload-native"}, true)
	modWriteMod(w.t, inst, "BetterChatLocal", types.ModManifestJson{Name: "Chatty", Version: "1.0", Entry: "chat.dll", Type: "preload-native", Author: "me"}, false)
	modWriteMod(w.t, inst, "Tweaks", types.ModManifestJson{Name: "Tweaks", Version: "0.1", Entry: "t.dll", Type: "preload-native"}, true)
	w.states["Survival"] = map[string]types.LIPPackageInstallState{
		"github.com/liteldev/levilamina":  {Installed: true, InstalledVersion: "26.60.4"},
		"github.com/liteldev/legacymoney": {Installed: true, ExplicitInstalled: true, InstalledVersion: "0.9.1"},
	}
	return inst
}

func TestModList(t *testing.T) {
	w := newModWorld(t)
	w.survival()
	code, out, errs := w.h.run("mod", "list", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	var items []modListItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	var refs []string
	for _, it := range items {
		refs = append(refs, it.Ref)
	}
	if !reflect.DeepEqual(refs, []string{"LiteLDev/LegacyMoney", "BetterChatLocal", "Tweaks"}) {
		t.Fatalf("refs = %v", refs)
	}
	money := items[0]
	if money.Source != "catalog" || money.Latest != "0.9.2" || !money.Outdated || money.Package == nil || !money.Package.Explicit || money.State != "on" {
		t.Fatalf("money = %+v", money)
	}
	if chat := items[1]; chat.Enabled || chat.State != "off" || chat.Source != "local" || chat.Mods[0].Author != "me" {
		t.Fatalf("chat = %+v", chat)
	}
	code, out, _ = w.h.run("mod", "ls", "-q", "--outdated")
	if code != 0 || out != "LiteLDev/LegacyMoney\n" {
		t.Fatalf("outdated -q = %d %q", code, out)
	}
	code, out, _ = w.h.run("mod", "list", "--enabled", "--source", "local", "-q")
	if code != 0 || out != "Tweaks\n" {
		t.Fatalf("enabled local = %d %q", code, out)
	}
	code, out, _ = w.h.run("mod", "list")
	if code != 0 || !strings.Contains(out, "LiteLDev/LegacyMoney") || !strings.Contains(out, "MOD") {
		t.Fatalf("table = %s", out)
	}
	// Offline: local information with a warning.
	w.catErr = errors.New("dial tcp: no route")
	code, out, errs = w.h.run("mod", "list")
	if code != 0 || !strings.Contains(errs, "warning:") || !strings.Contains(out, "LegacyMoney") || strings.Contains(out, "LiteLDev/") {
		t.Fatalf("offline = %d %q %q", code, out, errs)
	}
}

func TestModToggleSetAndPath(t *testing.T) {
	w := newModWorld(t)
	inst := w.survival()
	code, out, errs := w.h.run("mod", "enable", "Chatty", "Tweaks")
	if code != 0 || !strings.Contains(out, "enabled Chatty (folder BetterChatLocal)") || !strings.Contains(out, "Tweaks is already enabled") {
		t.Fatalf("enable = %d %q %q", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(inst.Dir, "mods", "BetterChatLocal", "manifest.json")); err != nil {
		t.Fatal("not enabled on disk")
	}
	code, _, _ = w.h.run("mod", "disable", "LiteLDev/LegacyMoney")
	if code != 0 || !reflect.DeepEqual(w.calls[len(w.calls)-1], "disable LegacyMoney") {
		t.Fatalf("disable by package = %d %v", code, w.calls)
	}
	code, _, errs = w.h.run("mod", "disable", "Twaeks")
	if code != ExitNotFound || !strings.Contains(errs, `did you mean "Tweaks"?`) {
		t.Fatalf("unknown = %d %q", code, errs)
	}
	code, _, errs = w.h.run("mod", "disable", "LeviLamina")
	if code != ExitUsage || !strings.Contains(errs, "loader") {
		t.Fatalf("LeviLamina = %d %q", code, errs)
	}
	code, _, _ = w.h.run("mod", "enable")
	if code != ExitUsage {
		t.Fatalf("no mods = %d", code)
	}
	w.results["enable Tweaks"] = "ERR_WRITE_FILE"
	w.h.run("mod", "disable", "Tweaks")
	code, _, errs = w.h.run("mod", "enable", "--all")
	if code != ExitPartial || !strings.Contains(errs, "1 of 3 failed") {
		t.Fatalf("partial = %d %q", code, errs)
	}

	code, out, errs = w.h.run("mod", "set", "Tweaks", "--mod-version", "0.2", "--author", "Ann")
	if code != 0 || out != "version: 0.1 -> 0.2\nauthor: \"\" -> Ann\n" {
		t.Fatalf("set = %d %q %q", code, out, errs)
	}
	if last := w.calls[len(w.calls)-1]; last != "set Tweaks name=Tweaks version=0.2 type=preload-native entry=t.dll author=Ann" {
		t.Fatalf("set call = %q", last)
	}
	if code, _, _ = w.h.run("mod", "set", "Tweaks"); code != ExitUsage {
		t.Fatalf("set without fields = %d", code)
	}
	if code, out, _ = w.h.run("mod", "set", "Tweaks", "--author", "Ann"); code != 0 || out != "" {
		t.Fatalf("set unchanged = %d %q", code, out)
	}

	code, out, _ = w.h.run("mod", "path", "Chatty")
	if code != 0 || strings.TrimSpace(out) != filepath.Join(inst.Dir, "mods", "BetterChatLocal") {
		t.Fatalf("path = %d %q", code, out)
	}
	code, out, _ = w.h.run("mod", "path")
	if code != 0 || strings.TrimSpace(out) != filepath.Join(inst.Dir, "mods") {
		t.Fatalf("mods path = %d %q", code, out)
	}
}

func TestModImport(t *testing.T) {
	w := newModWorld(t)
	w.survival()
	dir := t.TempDir()
	file := func(name, data string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	dll, pkg := file("Tool.dll", "MZ"), file("Chat.mcmod", "PK")
	code, out, errs := w.h.run("mod", "import", dll, "--mod-version", "1.2.0")
	if code != 0 || out != "imported Tool.dll\n" {
		t.Fatalf("dll = %d %q %q", code, out, errs)
	}
	if last := w.calls[len(w.calls)-1]; last != "dll Survival Tool.dll name=Tool type=preload-native version=1.2.0 overwrite=false" {
		t.Fatalf("dll call = %q", last)
	}
	if code, _, errs = w.h.run("mod", "import", dll, pkg, "--name", "X"); code != ExitUsage {
		t.Fatalf("--name with two files = %d %q", code, errs)
	}
	// A .dll name must be one folder name inside mods.
	n := len(w.calls)
	for _, name := range []string{"../../evil", `..\evil`, "a/b", "..", "C:x", "x."} {
		if code, _, errs = w.h.run("mod", "import", dll, "--name", name); code != ExitUsage || !strings.Contains(errs, "one folder name") {
			t.Fatalf("--name %q = %d %q", name, code, errs)
		}
	}
	if code, _, errs = w.h.run("mod", "import", file("..dll", "MZ")); code != ExitUsage || !strings.Contains(errs, ProgramName+" mod import ") || !strings.Contains(errs, "--name <name>") {
		t.Fatalf("bad file name = %d %q", code, errs)
	}
	if len(w.calls) != n {
		t.Fatalf("bad names reached the backend: %v", w.calls[n:])
	}
	// An empty --name falls back to the file name.
	if code, _, errs = w.h.run("mod", "import", dll, "--name", " "); code != 0 || !strings.Contains(w.calls[len(w.calls)-1], "name=Tool ") {
		t.Fatalf("--name empty = %d %q %v", code, errs, w.calls[n:])
	}
	if code, _, errs = w.h.run("mod", "import", file("a.mcpack", "")); code != ExitUsage || !strings.Contains(errs, "pack import") {
		t.Fatalf("mcpack = %d %q", code, errs)
	}
	w.results["zip "] = "ERR_DUPLICATE_FOLDER"
	if code, _, errs = w.h.run("mod", "import", pkg); code != ExitConflict || !strings.Contains(errs, "--overwrite") {
		t.Fatalf("duplicate = %d %q", code, errs)
	}
	if code, out, _ = w.h.run("mod", "import", pkg, "--skip-existing"); code != 0 || !strings.HasPrefix(out, "skipped") {
		t.Fatalf("skip = %d %q", code, out)
	}
	w.results = map[string]string{"zip Survival PK overwrite=false": "ERR_DUPLICATE_FOLDER"}
	w.h.console, w.h.stdin = true, "y\n"
	if code, out, _ = w.h.run("mod", "import", pkg); code != 0 || out != "overwritten Chat.mcmod\n" {
		t.Fatalf("prompted overwrite = %d %q", code, out)
	}
	w.h.console, w.h.stdin = false, ""
	w.results = map[string]string{"zip ": "ERR_OPEN_ZIP"}
	code, _, errs = w.h.run("mod", "import", dll, pkg, filepath.Join(dir, "missing.zip"))
	if code != ExitPartial || !strings.Contains(errs, "not a valid mod package") || !strings.Contains(errs, "2 of 3 failed") {
		t.Fatalf("batch = %d %q", code, errs)
	}
}

func TestModRemove(t *testing.T) {
	w := newModWorld(t)
	w.survival()
	code, _, errs := w.h.run("mod", "remove", "Tweaks")
	if code != ExitUsage || !strings.Contains(errs, "--yes") {
		t.Fatalf("unconfirmed = %d %q", code, errs)
	}
	code, out, errs := w.h.run("mod", "rm", "Tweaks", "LegacyMoney", "-y")
	if code != 0 || !strings.Contains(out, "removed Tweaks") || !strings.Contains(out, "removed LegacyMoney (LiteLDev/LegacyMoney)") {
		t.Fatalf("remove = %d %q %q", code, out, errs)
	}
	if !modContains(w.calls, "remove Tweaks") || !modContains(w.calls, "uninstall Survival github.com/LiteLDev/LegacyMoney") {
		t.Fatalf("calls = %v", w.calls)
	}
	w.results["uninstall"] = "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY"
	code, out, errs = w.h.run("mod", "rm", "LiteLDev/LegacyMoney", "-y")
	if code != 0 || !strings.Contains(errs, "dependency") || !strings.Contains(out, "kept") {
		t.Fatalf("demoted = %d %q %q", code, out, errs)
	}
	w.states["Survival"]["github.com/liteldev/legacymoney"] = types.LIPPackageInstallState{Installed: true, InstalledVersion: "0.9.1"}
	code, _, errs = w.h.run("mod", "rm", "LiteLDev/LegacyMoney", "-y")
	if code != ExitConflict || !strings.Contains(errs, "need") {
		t.Fatalf("required = %d %q", code, errs)
	}
	if code, _, _ = w.h.run("mod", "rm", "nothing", "-y"); code != ExitNotFound {
		t.Fatalf("unknown = %d", code)
	}
	// Offline with an explicit catalogue package: refuse to guess.
	w.states["Survival"]["github.com/liteldev/legacymoney"] = types.LIPPackageInstallState{Installed: true, ExplicitInstalled: true}
	w.catErr = errors.New("offline")
	if code, _, errs = w.h.run("mod", "rm", "BetterChatLocal", "-y"); code != ExitNetwork {
		t.Fatalf("offline guard = %d %q", code, errs)
	}
}

func TestModInstall(t *testing.T) {
	w := newModWorld(t)
	inst := w.survival()
	w.h.current = inst.Name
	// LegacyMoney 0.9.1 is installed; 0.9.2 fits LeviLamina 26.60.4.
	code, out, errs := w.h.run("mod", "install", "LiteLDev/LegacyMoney")
	if code != 0 || out != "upgraded LegacyMoney 0.9.1 -> 0.9.2\n" {
		t.Fatalf("upgrade = %d %q %q", code, out, errs)
	}
	// Installed already: changed in place, whatever its variant.
	if last := w.calls[len(w.calls)-1]; last != "install Survival github.com/LiteLDev/LegacyMoney@0.9.2" {
		t.Fatalf("call = %q", last)
	}
	// Same version again: nothing to do.
	w.states["Survival"]["github.com/liteldev/legacymoney"] = types.LIPPackageInstallState{Installed: true, ExplicitInstalled: true, InstalledVersion: "0.9.2"}
	n := len(w.calls)
	code, out, _ = w.h.run("mod", "install", "LiteLDev/LegacyMoney")
	if code != 0 || !strings.Contains(out, "already installed") || len(w.calls) != n {
		t.Fatalf("idempotent = %d %q %v", code, out, w.calls[n:])
	}
	// A downgrade needs confirmation, and the older version needs another
	// LeviLamina for this game: incompatible.
	code, _, errs = w.h.run("mod", "install", "LiteLDev/LegacyMoney@0.9.1")
	if code != ExitUsage || !strings.Contains(errs, "does not run on Minecraft 1.26.60.05") {
		t.Fatalf("incompatible = %d %q", code, errs)
	}
	// No LeviLamina installed: it is installed first.
	delete(w.states["Survival"], "github.com/liteldev/levilamina")
	delete(w.states["Survival"], "github.com/liteldev/legacymoney")
	code, out, errs = w.h.run("mod", "install", "LiteLDev/LegacyMoney", "--json")
	if code != 0 || !modContains(w.calls, "loader Survival 1.26.60.05 26.60.4") {
		t.Fatalf("with loader = %d %q %q %v", code, out, errs, w.calls)
	}
	var res []modBatchResult
	if err := json.Unmarshal([]byte(out), &res); err != nil || len(res) != 1 || res[0].Status != "install" || res[0].To != "0.9.2" {
		t.Fatalf("json = %q", out)
	}
	if code, _, errs = w.h.run("mod", "install", "LiteLDev/LegacyMoney", "--no-loader-change"); code != ExitConflict {
		t.Fatalf("--no-loader-change = %d %q", code, errs)
	}
	// Replacing an installed LeviLamina asks first.
	w.states["Survival"]["github.com/liteldev/levilamina"] = types.LIPPackageInstallState{Installed: true, InstalledVersion: "26.51.1"}
	if code, _, errs = w.h.run("mod", "install", "LiteLDev/LegacyMoney"); code != ExitUsage || !strings.Contains(errs, "--yes") {
		t.Fatalf("replace loader = %d %q", code, errs)
	}
	if code, _, errs = w.h.run("mod", "install", "LiteLDev/LegacyMoney", "--loader-version", "1.0.0", "-y"); code != ExitNotFound {
		t.Fatalf("bad loader version = %d %q", code, errs)
	}
	if code, _, errs = w.h.run("mod", "install", "nobody/nothing"); code != ExitNotFound {
		t.Fatalf("unknown package = %d %q", code, errs)
	}
	if code, _, _ = w.h.run("mod", "install", "notapackage"); code != ExitUsage {
		t.Fatalf("bad reference = %d", code)
	}
	if code, _, _ = w.h.run("mod", "install", "LiteLDev/LeviLamina"); code != ExitUsage {
		t.Fatalf("LeviLamina = %d", code)
	}
	w.results["install"] = "ERR_LIP_PACKAGE_INSTALL_FAILED"
	code, _, errs = w.h.run("mod", "install", "LiteLDev/LegacyMoney", "Some/Chat", "-y")
	if code != ExitFailure || !strings.Contains(errs, "2 of 2 failed") {
		t.Fatalf("failed batch = %d %q", code, errs)
	}
	uwp := w.h.add("Old", "1.20.0.1", "release", versions.PackageTypeUWP)
	if code, _, errs = w.h.run("mod", "install", "Some/Chat", "-i", uwp.Name); code != ExitConflict || !strings.Contains(errs, "GDK") {
		t.Fatalf("uwp = %d %q", code, errs)
	}
}

func TestModUpdate(t *testing.T) {
	w := newModWorld(t)
	w.survival()
	code, out, errs := w.h.run("mod", "update", "--all", "--dry-run", "--json")
	if code != 0 {
		t.Fatalf("dry run = %d %q", code, errs)
	}
	var plan []modUpdatePlanItem
	if err := json.Unmarshal([]byte(out), &plan); err != nil || len(plan) != 1 || plan[0].Package != "LiteLDev/LegacyMoney" || plan[0].To != "0.9.2" {
		t.Fatalf("plan = %q", out)
	}
	if code, _, _ = w.h.run("mod", "update", "--all"); code != ExitUsage {
		t.Fatalf("unconfirmed = %d", code)
	}
	code, out, errs = w.h.run("mod", "update", "LegacyMoney", "-y")
	if code != 0 || out != "updated LegacyMoney 0.9.1 -> 0.9.2\n" || w.calls[len(w.calls)-1] != "install Survival github.com/LiteLDev/LegacyMoney@0.9.2" {
		t.Fatalf("update = %d %q %q %v", code, out, errs, w.calls)
	}
	// An installed pre-release newer than the latest stable is not
	// "updated" back to it.
	w.states["Survival"]["github.com/liteldev/legacymoney"] = types.LIPPackageInstallState{Installed: true, ExplicitInstalled: true, InstalledVersion: "1.0.0-beta.1"}
	n := len(w.calls)
	code, out, errs = w.h.run("mod", "update", "--all", "-y")
	if code != 0 || out != "" || !strings.Contains(errs, "newer than the catalogue's latest stable 0.9.2") || !strings.Contains(errs, "mod install LiteLDev/LegacyMoney@0.9.2") || len(w.calls) != n {
		t.Fatalf("pre-release = %d %q %q %v", code, out, errs, w.calls[n:])
	}
	// --json still says why.
	code, _, errs = w.h.run("mod", "update", "LegacyMoney", "--json", "-y")
	if code != 0 || !strings.Contains(errs, "warning: LegacyMoney 1.0.0-beta.1 is newer") || len(w.calls) != n {
		t.Fatalf("pre-release --json = %d %q %v", code, errs, w.calls[n:])
	}
	if modOutdated(true, "1.0.0-beta.1", "0.9.2") || !modOutdated(true, "0.9.1", "0.9.2") {
		t.Fatal("modOutdated")
	}
	if !modUpdateWouldDowngrade("2.0.0-beta.2", "1.9.0") || modUpdateWouldDowngrade("1.0.0-rc.1", "1.0.0") || modUpdateWouldDowngrade("1.0", "0.9.2") {
		t.Fatal("modUpdateWouldDowngrade")
	}
	code, _, errs = w.h.run("mod", "update", "Tweaks")
	if code != 0 || !strings.Contains(errs, "not installed from the mod catalogue") {
		t.Fatalf("local = %d %q", code, errs)
	}
	if code, _, _ = w.h.run("mod", "update"); code != ExitUsage {
		t.Fatalf("nothing named = %d", code)
	}
}

func TestModSearchAndShow(t *testing.T) {
	w := newModWorld(t)
	inst := w.survival()
	w.h.current = inst.Name
	// The default instance's game filters out mods that do not run on it
	// (the catalogue's game versions).
	code, out, errs := w.h.run("mod", "search")
	if code != 0 || !strings.Contains(out, "LiteLDev/LegacyMoney") || strings.Contains(out, "Some/Chat") || !strings.Contains(errs, "Minecraft 1.26.60 (instance Survival)") {
		t.Fatalf("search = %d %q %q", code, out, errs)
	}
	// A default instance no mod runs on filters nothing, and says so.
	w.h.current = w.h.add("Alpha", "1.21.50.07", "release", versions.PackageTypeGDK).Name
	code, out, errs = w.h.run("mod", "search")
	if code != 0 || !strings.Contains(out, "Some/Chat") || !strings.Contains(errs, "no mod of the catalogue runs on Minecraft 1.21.50 (instance Alpha); showing mods for every game version") {
		t.Fatalf("unsupported default = %d %q %q", code, out, errs)
	}
	w.h.current = ""
	w.h.add("Other", "1.26.51.01", "release", versions.PackageTypeGDK)
	code, out, _ = w.h.run("mod", "search", "--game", "1.26.51", "-q")
	if code != 0 || out != "LiteLDev/LegacyMoney\n" {
		t.Fatalf("by game = %d %q", code, out)
	}
	// Relevance without a search is the catalogue's order; the site's sorts
	// only (no name sort, no reverse order).
	code, out, _ = w.h.run("mod", "search", "--any-version", "-q")
	if code != 0 || out != "LiteLDev/LegacyMoney\nSome/Chat\n" {
		t.Fatalf("relevance = %d %q", code, out)
	}
	code, out, _ = w.h.run("mod", "search", "--any-version", "--sort", "stars", "-q")
	if code != 0 || out != "LiteLDev/LegacyMoney\nSome/Chat\n" {
		t.Fatalf("by stars = %d %q", code, out)
	}
	for _, args := range [][]string{{"--sort", "name"}, {"--reverse"}, {"--license", "oss"}, {"--license", "WTFPL"}} {
		if code, _, _ = w.h.run(append([]string{"mod", "search", "--any-version"}, args...)...); code != ExitUsage {
			t.Fatalf("%v = %d", args, code)
		}
	}
	// Tags (aliases group under featured tags), licenses, dependencies, authors.
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--tag", "economy"}, "LiteLDev/LegacyMoney\n"},
		{[]string{"--tag", "-money"}, "Some/Chat\n"},
		{[]string{"--license", "permissive"}, "LiteLDev/LegacyMoney\n"},
		{[]string{"--license", "none,GPL-3.0"}, "Some/Chat\n"},
		{[]string{"--license", "Permissive"}, "LiteLDev/LegacyMoney\n"},
		{[]string{"--license", "mit,NONE"}, "LiteLDev/LegacyMoney\nSome/Chat\n"},
		{[]string{"--dep", "Some/Lib"}, "Some/Chat\n"},
		{[]string{"--author", "liteldev", "--author", "nobody"}, "LiteLDev/LegacyMoney\n"},
	} {
		code, out, _ = w.h.run(append([]string{"mod", "search", "--any-version", "-q"}, tc.args...)...)
		if code != 0 || out != tc.want {
			t.Fatalf("%v = %d %q, want %q", tc.args, code, out, tc.want)
		}
	}
	if code, _, _ = w.h.run("mod", "search", "--game", "9.9"); code != ExitNotFound {
		t.Fatalf("unknown game = %d", code)
	}
	code, out, _ = w.h.run("mod", "search", "econ", "--json")
	var found []modSearchItem
	if code != 0 || json.Unmarshal([]byte(out), &found) != nil || len(found) != 1 || found[0].Latest != "0.9.2" {
		t.Fatalf("keyword = %d %q", code, out)
	}
	code, out, errs = w.h.run("mod", "show", "LiteLDev/LegacyMoney", "--versions", "-i", "Survival")
	if code != 0 || !strings.Contains(out, "0.9.1") || !strings.Contains(out, "1.26.51.01") || !strings.Contains(out, "Installed in Survival") {
		t.Fatalf("show = %d %q %q", code, out, errs)
	}
	code, out, _ = w.h.run("mod", "show", "LiteLDev/LegacyMoney", "--readme")
	if code != 0 || out != "# fetched readme\n" {
		t.Fatalf("readme = %d %q", code, out)
	}
	if code, _, _ = w.h.run("mod", "show", "LiteLDev/LegacyMoney#server"); code != ExitNotFound {
		t.Fatalf("missing variant = %d", code)
	}
	w.catErr = errors.New("ERR_LIP_INDEX_TIMEOUT: slow")
	if code, _, errs = w.h.run("mod", "search"); code != ExitNetwork || !strings.Contains(errs, "ERR_LIP_INDEX_TIMEOUT") {
		t.Fatalf("timeout = %d %q", code, errs)
	}
}

func TestModBackendErrors(t *testing.T) {
	cases := map[string]int{
		"ERR_UWP_UNSUPPORTED_FEATURE":            ExitConflict,
		"ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS": ExitConflict,
		"ERR_LIP_PACKAGE_INSTALL_FAILED":         ExitFailure,
		"ERR_FETCH_LL_DB":                        ExitNetwork,
		"ERR_DUPLICATE_FOLDER":                   ExitConflict,
		"ERR_LIP_PACKAGE_INVALID_IDENTIFIER":     ExitUsage,
	}
	for code, exit := range cases {
		if e := modBackendError(code); e.Exit != exit || e.Code != code || e.Message == "" {
			t.Errorf("%s = %+v", code, e)
		}
	}
	if e := modBackendError("ERR_OPEN_ZIP: zip: not a valid zip file"); e.Detail != "zip: not a valid zip file" || !strings.Contains(e.Message, "not a valid mod package") {
		t.Errorf("detail kept = %+v", e)
	}
	if modBackendError("") != nil {
		t.Error("empty result is success")
	}
	if e := modCatalogError(errors.New("boom")); e.Code != "ERR_LIP_INDEX_UNAVAILABLE" || e.Exit != ExitNetwork {
		t.Errorf("catalogError = %+v", e)
	}
}

func modContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
