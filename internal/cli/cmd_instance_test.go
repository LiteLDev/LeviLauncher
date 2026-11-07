package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// instFakes replaces every system-changing seam of the instance commands
// and records the calls.
type instFakes struct {
	h          *harness
	saved      []func()
	calls      []string
	registered map[string]string // slot -> canonical path of the holder
	devMode    bool
	desktop    string
	systemData string
	running    map[string][]int // instance name -> pids
	setDefault []string
	saveMeta   func(name, gv, typ string, iso, console, editor bool, args, env string) string
	register   func(name string) string
}

func instSave[T any](f *instFakes, p *T) {
	old := *p
	f.saved = append(f.saved, func() { *p = old })
}

func newInstFakes(t *testing.T) *instFakes {
	h := newHarness(t)
	f := &instFakes{h: h, registered: map[string]string{}, devMode: true, desktop: t.TempDir(),
		systemData: filepath.Join(t.TempDir(), "Minecraft Bedrock"), running: map[string][]int{}}
	instSave(f, &instSaveMeta)
	instSave(f, &instSaveLogo)
	instSave(f, &instRemoveLogo)
	instSave(f, &instValidateName)
	instSave(f, &instRenameFolder)
	instSave(f, &instDeleteFolder)
	instSave(f, &instCreateShortcut)
	instSave(f, &instShortcutPath)
	instSave(f, &instSetDefault)
	instSave(f, &instRegister)
	instSave(f, &instUnregister)
	instSave(f, &instDevModeEnabled)
	instSave(f, &instSystemDataPath)
	instSave(f, &instContentRoots)
	instSave(f, &instOpenFolder)
	t.Cleanup(func() {
		for i := len(f.saved) - 1; i >= 0; i-- {
			f.saved[i]()
		}
	})
	record := func(format string, a ...any) { f.calls = append(f.calls, fmt.Sprintf(format, a...)) }
	instSaveMeta = func(name, gv, typ string, iso, console, editor bool, args, env string) string {
		record("save %s gv=%s type=%s iso=%v console=%v editor=%v args=%q env=%q", name, gv, typ, iso, console, editor, args, env)
		if f.saveMeta != nil {
			return f.saveMeta(name, gv, typ, iso, console, editor, args, env)
		}
		return ""
	}
	instSaveLogo = func(name, path string) string { record("logo %s %s", name, filepath.Base(path)); return "" }
	instRemoveLogo = func(name string) string { record("unlogo %s", name); return "" }
	instValidateName = func(name string) string {
		for _, i := range h.instances {
			if strings.EqualFold(i.Name, name) {
				return "ERR_NAME_EXISTS"
			}
		}
		if strings.ContainsAny(name, `<>:"/\|?*`) {
			return "ERR_NAME_INVALID_CHAR"
		}
		return ""
	}
	instRenameFolder = func(old, nn string) string {
		record("rename %s %s", old, nn)
		for i := range h.instances {
			if h.instances[i].Name == old {
				h.instances[i].Name = nn
			}
		}
		return ""
	}
	instDeleteFolder = func(name string) string {
		record("delete %s", name)
		out := h.instances[:0]
		for _, i := range h.instances {
			if i.Name != name {
				out = append(out, i)
			}
		}
		h.instances = out
		return ""
	}
	instShortcutPath = func(name string) string { return filepath.Join(f.desktop, "Minecraft - "+name+".lnk") }
	instCreateShortcut = func(name string) string {
		record("shortcut %s", name)
		return instWriteFile(t, instShortcutPath(name), "lnk")
	}
	instSetDefault = func(name string) error { f.setDefault = append(f.setDefault, name); h.current = name; return nil }
	instRegister = func(_ context.Context, name string, preview bool) string {
		record("register %s preview=%v", name, preview)
		if f.register != nil {
			if code := f.register(name); code != "" {
				return code
			}
		}
		inst, _ := lookupIn(h.instances, name)
		slot := "release"
		if preview {
			slot = "preview"
		}
		f.registered[slot] = utils.CanonicalWindowsPath(inst.Dir)
		return ""
	}
	instUnregister = func(_ context.Context, name string) string {
		record("unregister %s", name)
		inst, _ := lookupIn(h.instances, name)
		for k, v := range f.registered {
			if strings.EqualFold(v, utils.CanonicalWindowsPath(inst.Dir)) {
				delete(f.registered, k)
			}
		}
		return ""
	}
	instDevModeEnabled = func() bool { return f.devMode }
	instSystemDataPath = func(bool) string { return f.systemData }
	instContentRoots = instFakeRoots
	instOpenFolder = func(dir string) error { record("open %s", dir); return nil }
	usersRootOf = func(inst Instance) string { return instFakeRoots(inst).UsersRoot }
	registeredLocations = func() (string, string) { return f.registered["release"], f.registered["preview"] }
	minecraftProcesses = func() []types.ProcessInfo {
		var out []types.ProcessInfo
		for name, pids := range f.running {
			for _, p := range pids {
				out = append(out, types.ProcessInfo{Pid: p, IsLauncher: true, VersionName: name, ExePath: name + `\Minecraft.Windows.exe`})
			}
		}
		return out
	}
	return f
}

// add creates an instance folder with its own data.
func (f *instFakes) add(name, gv, channel, pkg string, isolated bool) Instance {
	inst := f.h.add(name, gv, channel, pkg)
	if err := os.MkdirAll(inst.Dir, 0o755); err != nil {
		f.h.t.Fatal(err)
	}
	f.h.instances[len(f.h.instances)-1].Meta.EnableIsolation = isolated
	inst.Meta.EnableIsolation = isolated
	return inst
}

// instFakeRoots lays out content like an instance with its own data.
func instFakeRoots(inst Instance) types.ContentRoots {
	base := filepath.Join(inst.Dir, "Minecraft Bedrock")
	if inst.IsUWP() {
		cm := filepath.Join(base, "games", "com.mojang")
		return types.ContentRoots{Base: base, PackageType: "uwp", ComMojangRoot: cm, Worlds: filepath.Join(cm, "minecraftWorlds"),
			ResourcePacks: filepath.Join(cm, "resource_packs"), BehaviorPacks: filepath.Join(cm, "behavior_packs"),
			SkinPacks: filepath.Join(cm, "skin_packs"), Screenshots: filepath.Join(cm, "Screenshots"), IsIsolation: true}
	}
	users := filepath.Join(base, "Users")
	shared := filepath.Join(users, "Shared", "games", "com.mojang")
	return types.ContentRoots{Base: base, PackageType: "gdk", UsersRoot: users, ResourcePacks: filepath.Join(shared, "resource_packs"),
		BehaviorPacks: filepath.Join(shared, "behavior_packs"), SkinPacks: filepath.Join(shared, "skin_packs"), IsIsolation: true}
}

func instWriteFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return ""
}

func instMkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInstApplyEnv(t *testing.T) {
	got, err := instApplyEnv([]string{"A=1", "Path=x", "B=2"}, false, []string{"path"}, []string{"a=9", "C=3=x"})
	if err != nil || !reflect.DeepEqual(got, []string{"a=9", "B=2", "C=3=x"}) {
		t.Fatalf("env = %q, %v", got, err)
	}
	got, err = instApplyEnv([]string{"A=1"}, true, nil, []string{"D="})
	if err != nil || !reflect.DeepEqual(got, []string{"D="}) {
		t.Fatalf("clear then set = %q, %v", got, err)
	}
	for _, bad := range [][]string{{"NOVALUE"}, {"=x"}} {
		if _, err := instApplyEnv(nil, false, nil, bad); err == nil || FromError(err).Exit != ExitUsage {
			t.Errorf("--env %q accepted", bad)
		}
	}
	if _, err := instApplyEnv(nil, false, []string{"A=1"}, nil); err == nil {
		t.Error("--unset-env with = accepted")
	}
	if d := instEnvDiff([]string{"A=1", "B=2"}, []string{"A=2", "C=3"}); d != "env +1 ~1 -1" {
		t.Errorf("diff = %q", d)
	}
	if d := instEnvDiff([]string{"A=1"}, []string{"A=1"}); d != "" {
		t.Errorf("no-op diff = %q", d)
	}
	if got := instEnvLines("A=1\r\n\n  B=2  \n"); !reflect.DeepEqual(got, []string{"A=1", "B=2"}) {
		t.Errorf("lines = %q", got)
	}
}

func TestInstKindPath(t *testing.T) {
	gdk := Instance{Name: "g", Dir: `C:\i\g`, PackageType: "gdk", Channel: "release", Meta: instTestMeta("1.21.50.07")}
	r := instFakeRoots(gdk)
	users := r.UsersRoot
	for kind, want := range map[string]string{
		"root":           gdk.Dir,
		"data":           r.Base,
		"mods":           filepath.Join(gdk.Dir, "mods"),
		"worlds":         filepath.Join(users, "123", "games", "com.mojang", "minecraftWorlds"),
		"screenshots":    filepath.Join(users, "123", "games", "com.mojang", "Screenshots"),
		"servers":        filepath.Join(users, "123", "games", "com.mojang", "minecraftpe"),
		"skin-packs":     filepath.Join(users, "123", "games", "com.mojang", "skin_packs"),
		"resource-packs": r.ResourcePacks,
	} {
		if got := instKindPath(gdk, r, kind, "123"); got != want {
			t.Errorf("gdk %s = %q, want %q", kind, got, want)
		}
	}
	for _, bad := range []string{"", "..", `a\b`, "c:"} {
		if got := instKindPath(gdk, r, "worlds", bad); got != "" {
			t.Errorf("player %q gave %q", bad, got)
		}
	}
	if !instKindIsPerPlayer(gdk, "skin-packs") || instKindIsPerPlayer(gdk, "resource-packs") {
		t.Error("per-player kinds of old GDK")
	}
	newGDK := gdk
	newGDK.Meta = instTestMeta("1.26.0.1")
	if got, want := instKindPath(newGDK, r, "skin-packs", "123"), filepath.Join(users, "Shared", "games", "com.mojang", "skin_packs"); got != want {
		t.Errorf("shared skins = %q, want %q", got, want)
	}
	if instKindIsPerPlayer(newGDK, "skin-packs") {
		t.Error("skins after 1.26 are shared")
	}
	uwp := Instance{Name: "u", Dir: `C:\i\u`, PackageType: "uwp", Channel: "release"}
	ur := instFakeRoots(uwp)
	if got := instKindPath(uwp, ur, "worlds", "ignored"); got != ur.Worlds {
		t.Errorf("uwp worlds = %q", got)
	}
	if got := instKindPath(uwp, ur, "servers", ""); got != filepath.Join(ur.ComMojangRoot, "minecraftpe") {
		t.Errorf("uwp servers = %q", got)
	}
	if instKindIsPerPlayer(uwp, "worlds") {
		t.Error("uwp has no players")
	}
}

func instTestMeta(gv string) versions.VersionMeta { return versions.VersionMeta{GameVersion: gv} }

func TestInstanceShowCountsContent(t *testing.T) {
	f := newInstFakes(t)
	inst := f.add("Survival", "1.21.50.07", "release", "gdk", true)
	f.h.current = "Survival"
	r := instFakeRoots(inst)
	player := filepath.Join(r.UsersRoot, "111", "games", "com.mojang")
	instMkdirs(t, filepath.Join(player, "minecraftWorlds", "w1"), filepath.Join(player, "minecraftWorlds", "w2"),
		filepath.Join(player, "skin_packs", "s1"), filepath.Join(r.UsersRoot, "98765"), filepath.Join(r.UsersRoot, "Shared"),
		filepath.Join(r.ResourcePacks, "rp"), filepath.Join(inst.Dir, "mods", "LeviLamina"))
	instWriteFile(t, filepath.Join(player, "Screenshots", "a.png"), "x")
	instWriteFile(t, filepath.Join(player, "Screenshots", "sub", "b.jpeg"), "x")
	instWriteFile(t, filepath.Join(player, "Screenshots", "sub", "b.json"), "{}")
	instWriteFile(t, filepath.Join(player, "minecraftpe", "external_servers.txt"), "1:A:a.net:19132:1\n2:B:b.net:19132:2\nbad\n")
	instWriteFile(t, filepath.Join(inst.Dir, "mods", "LeviLamina", "manifest.json"), `{"name":"LeviLamina","version":"1.4.2"}`)
	f.h.instances[0].Meta.EnvVars = "A=1\nB=2"
	gamertagCache = func() map[string]string { return map[string]string{"111": "Steve"} }

	code, out, errOut := f.h.run("instance", "show", "--json")
	var it instShowItem
	if code != 0 || json.Unmarshal([]byte(out), &it) != nil {
		t.Fatalf("show: %d %q %q", code, out, errOut)
	}
	if it.Name != "Survival" || !it.IsDefault || it.ModCount != 1 || it.LoaderVersion == nil || *it.LoaderVersion != "1.4.2" ||
		it.ResourcePacks != 1 || len(it.EnvVars) != 2 || len(it.Players) != 2 || it.Content != nil || it.SharedSkinPacks != nil {
		t.Fatalf("item = %+v", it)
	}
	var steve instShowPlayer
	for _, p := range it.Players {
		if p.ID == "111" {
			steve = p
		}
	}
	if steve.Gamertag == nil || *steve.Gamertag != "Steve" || steve.Worlds != 2 || steve.Screenshots != 2 || steve.Servers != 2 ||
		steve.SkinPacks == nil || *steve.SkinPacks != 1 {
		t.Fatalf("player = %+v", steve)
	}
	// "instance <name>" is "instance show <name>"; -p limits the players.
	code, out, _ = f.h.run("instance", "survival", "-p", "Steve")
	if code != 0 || !strings.Contains(out, "LeviLamina 1.4.2") || !strings.Contains(out, "Steve") || strings.Contains(out, "98765") ||
		!strings.Contains(out, "editor mode:") {
		t.Fatalf("human show: %d\n%s", code, out)
	}
	code, _, errOut = f.h.run("instance", "show", "nope")
	if code != ExitNotFound {
		t.Fatalf("unknown instance: %d %s", code, errOut)
	}
}

func TestInstanceShowUWP(t *testing.T) {
	f := newInstFakes(t)
	inst := f.add("Old", "1.20.81.1", "beta", "uwp", false)
	r := instFakeRoots(inst)
	instMkdirs(t, filepath.Join(r.Worlds, "w"), filepath.Join(r.SkinPacks, "s"))
	f.registered["release"] = utils.CanonicalWindowsPath(inst.Dir)
	code, out, _ := f.h.run("--json", "instance", "show", "Old")
	var it instShowItem
	if code != 0 || json.Unmarshal([]byte(out), &it) != nil {
		t.Fatalf("show: %d %q", code, out)
	}
	if it.Content == nil || it.Content.Worlds != 1 || it.Content.SkinPacks == nil || *it.Content.SkinPacks != 1 ||
		!it.Registered || it.Slot != "release" || it.EditorSupported {
		t.Fatalf("uwp item = %+v content %+v", it, it.Content)
	}
}

func TestInstanceSet(t *testing.T) {
	f := newInstFakes(t)
	f.add("G", "1.21.50.07", "release", "gdk", false)
	f.add("U", "1.19.60.3", "release", "uwp", false)
	f.add("P", "1.19.80.20", "preview", "gdk", true)

	code, _, errOut := f.h.run("instance", "set", "G")
	if code != ExitUsage || !strings.Contains(errOut, "nothing to change") {
		t.Fatalf("no flags: %d %s", code, errOut)
	}
	code, out, errOut := f.h.run("instance", "set", "G", "--console", "--editor", "--own-data", "--args", `-a "b c"`, "--env", "X=1", "--json")
	if code != 0 || len(f.calls) != 1 ||
		f.calls[0] != `save G gv=1.21.50.07 type=release iso=true console=true editor=true args="-a \"b c\"" env="X=1"` {
		t.Fatalf("set: %d %q %v %s", code, out, f.calls, errOut)
	}
	var item instanceListItem
	if json.Unmarshal([]byte(out), &item) != nil || item.Name != "G" {
		t.Fatalf("json: %q", out)
	}
	code, _, errOut = f.h.run("instance", "set", "G", "--no-console")
	if code != 0 || len(f.calls) != 1 || !strings.Contains(errOut, "Nothing to change") {
		t.Fatalf("no-op must not save: %d %v %s", code, f.calls, errOut)
	}
	for _, tc := range []struct {
		args []string
		exit int
		msg  string
	}{
		{[]string{"U", "--editor"}, ExitUsage, "editor mode needs game 1.21.50"},
		{[]string{"U", "--own-data"}, ExitUsage, "needs game 1.19.70.2"},
		{[]string{"U", "--args", "x"}, ExitUsage, "not supported on UWP"},
		{[]string{"U", "--env", "A=1"}, ExitUsage, "not supported on UWP"},
		{[]string{"G", "--own-data", "--shared-data"}, ExitUsage, "contradict"},
		{[]string{"G", "--env", "novalue"}, ExitUsage, "KEY=VALUE"},
		{[]string{"G", "--logo", "x.png", "--clear-logo"}, ExitUsage, "contradict"},
		{[]string{"G", "--logo", filepath.Join(t.TempDir(), "missing.png")}, ExitNotFound, "no file"},
	} {
		code, _, errOut := f.h.run(append([]string{"instance", "set"}, tc.args...)...)
		if code != tc.exit || !strings.Contains(errOut, tc.msg) {
			t.Errorf("%v: %d %s", tc.args, code, errOut)
		}
	}
	// Preview editor mode starts at 1.19.80.20; the game must not run.
	f.running["P"] = []int{9}
	code, _, errOut = f.h.run("instance", "set", "P", "--editor")
	if code != ExitConflict || !strings.Contains(errOut, "stop P") {
		t.Fatalf("running: %d %s", code, errOut)
	}
	delete(f.running, "P")
	code, _, errOut = f.h.run("instance", "set", "P", "--shared-data", "--editor")
	if code != 0 || !strings.Contains(f.calls[len(f.calls)-1], "iso=false console=false editor=true") ||
		!strings.Contains(errOut, "data own → shared") {
		t.Fatalf("preview editor: %d %v %s", code, f.calls, errOut)
	}
	f.saveMeta = func(string, string, string, bool, bool, bool, string, string) string { return "ERR_PREPARE_EXE" }
	code, _, errOut = f.h.run("instance", "set", "G", "--console")
	if code != ExitFailure || !strings.Contains(errOut, "[ERR_PREPARE_EXE]") {
		t.Fatalf("backend error: %d %s", code, errOut)
	}
}

func TestInstanceSetLogo(t *testing.T) {
	f := newInstFakes(t)
	inst := f.add("G", "1.21.50.07", "release", "gdk", false)
	dir := t.TempDir()
	writePNG := func(name string, w, h int) string {
		p := filepath.Join(dir, name)
		file, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
			t.Fatal(err)
		}
		return p
	}
	code, _, errOut := f.h.run("instance", "set", "G", "--logo", writePNG("wide.png", 4, 2))
	if code != ExitUsage || !strings.Contains(errOut, "must be square (got 4×2)") {
		t.Fatalf("not square: %d %s", code, errOut)
	}
	code, _, errOut = f.h.run("instance", "set", "G", "--logo", writePNG("ok.png", 3, 3))
	if code != 0 || len(f.calls) != 1 || f.calls[0] != "logo G ok.png" {
		t.Fatalf("logo: %d %v %s", code, f.calls, errOut)
	}
	instWriteFile(t, filepath.Join(inst.Dir, "LargeLogo.png"), "x")
	code, _, _ = f.h.run("instance", "set", "G", "--clear-logo")
	if code != 0 || f.calls[len(f.calls)-1] != "unlogo G" {
		t.Fatalf("clear logo: %d %v", code, f.calls)
	}
}

func TestInstanceRename(t *testing.T) {
	f := newInstFakes(t)
	a := f.add("A", "1.21.50.07", "release", "gdk", false)
	f.add("B", "1.21.50.07", "release", "gdk", false)
	f.h.current = "a"
	instWriteFile(t, instShortcutPath("A"), "lnk")

	code, _, errOut := f.h.run("instance", "rename", "A", "b")
	if code != ExitConflict || !strings.Contains(errOut, `"b" already exists`) {
		t.Fatalf("exists: %d %s", code, errOut)
	}
	code, _, errOut = f.h.run("instance", "rename", "A", "x:y")
	if code != ExitUsage || !strings.Contains(errOut, "must not contain") {
		t.Fatalf("invalid: %d %s", code, errOut)
	}
	f.registered["release"] = utils.CanonicalWindowsPath(a.Dir)
	code, _, errOut = f.h.run("instance", "rename", "A", "C")
	if code != ExitConflict || !strings.Contains(errOut, "instance unregister A") {
		t.Fatalf("registered: %d %s", code, errOut)
	}
	delete(f.registered, "release")
	f.running["A"] = []int{5}
	if code, _, _ = f.h.run("instance", "rename", "A", "C"); code != ExitConflict {
		t.Fatalf("running: %d", code)
	}
	delete(f.running, "A")
	code, out, errOut := f.h.run("instance", "rename", "A", "Survival", "--json")
	var res instRenameResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || !res.ShortcutUpdated || !res.DefaultUpdated || res.To != "Survival" {
		t.Fatalf("rename: %d %q %s", code, out, errOut)
	}
	if !reflect.DeepEqual(f.setDefault, []string{"Survival"}) || utils.FileExists(filepath.Join(f.desktop, "Minecraft - A.lnk")) ||
		!utils.FileExists(filepath.Join(f.desktop, "Minecraft - Survival.lnk")) {
		t.Fatalf("follow-ups: %v %v", f.setDefault, f.calls)
	}
	// Case-only renames skip the exists check.
	code, _, errOut = f.h.run("instance", "rename", "survival", "SURVIVAL")
	if code != 0 || !strings.Contains(strings.Join(f.calls, ";"), "rename Survival SURVIVAL") {
		t.Fatalf("case rename: %d %v %s", code, f.calls, errOut)
	}
	// The folder itself takes the new case.
	g := f.add("Gamma", "1.21.50.07", "release", "gdk", false)
	if code, _, errOut = f.h.run("instance", "rename", "Gamma", "GAMMA"); code != 0 {
		t.Fatalf("case rename on disk: %d %s", code, errOut)
	}
	es, err := os.ReadDir(filepath.Dir(g.Dir))
	if err != nil || len(es) != 1 || es[0].Name() != "GAMMA" {
		t.Fatalf("folder after case rename: %v %v", es, err)
	}
}

func TestInstanceRemoveStopsOnInterrupt(t *testing.T) {
	f := newInstFakes(t)
	for _, n := range []string{"A", "B", "C"} {
		f.add(n, "1.21.50.07", "release", "gdk", false)
	}
	sigs := make(chan chan<- os.Signal, 1)
	notifyInterrupt = func(ch chan<- os.Signal) func() { sigs <- ch; return func() {} }
	cancelled := make(chan struct{})
	interruptCancelled = func() { close(cancelled) }
	t.Cleanup(func() { interruptCancelled = func() {} })
	del := instDeleteFolder
	instDeleteFolder = func(name string) string {
		if name == "A" {
			// Ctrl-C while A is being deleted.
			(<-sigs) <- os.Interrupt
			select {
			case <-cancelled:
			case <-time.After(10 * time.Second):
				t.Error("the interrupt never cancelled the context")
			}
		}
		return del(name)
	}
	code, out, errOut := f.h.run("instance", "remove", "A", "B", "C", "-y")
	if code != ExitInterrupted || out != "Removed A\n" || !strings.Contains(errOut, "not removed: B, C") ||
		!strings.Contains(errOut, "instance remove B C") {
		t.Fatalf("interrupted: %d %q %s", code, out, errOut)
	}
	if strings.Contains(strings.Join(f.calls, ";"), "delete B") {
		t.Fatalf("deleted after Ctrl-C: %v", f.calls)
	}
}

func TestInstanceRemove(t *testing.T) {
	f := newInstFakes(t)
	a := f.add("A", "1.21.50.07", "release", "gdk", true)
	f.add("B", "1.21.50.07", "release", "gdk", false)
	f.add("C", "1.21.50.07", "preview", "gdk", false)
	f.h.current = "A"
	instWriteFile(t, instShortcutPath("A"), "lnk")

	code, _, errOut := f.h.run("instance", "remove", "A")
	if code != ExitUsage || !strings.Contains(errOut, "--yes") || len(f.calls) != 0 {
		t.Fatalf("needs confirmation: %d %s", code, errOut)
	}
	code, _, errOut = f.h.run("instance", "remove", "A", "nope", "-y")
	if code != ExitNotFound || len(f.calls) != 0 {
		t.Fatalf("unknown name first: %d %s", code, errOut)
	}
	// Interactive confirmation lists the folder and the worlds it holds.
	r := instFakeRoots(a)
	instMkdirs(t, filepath.Join(r.UsersRoot, "1", "games", "com.mojang", "minecraftWorlds", "w"))
	f.h.console, f.h.stdin = true, "n\n"
	code, _, errOut = f.h.run("instance", "rm", "A")
	if code != ExitCancelled || !strings.Contains(errOut, "includes 1 worlds") {
		t.Fatalf("declined: %d %s", code, errOut)
	}
	f.h.console, f.h.stdin = false, ""
	code, out, _ := f.h.run("instance", "rm", "A", "--yes")
	if code != 0 || out != "Removed A\n" || !reflect.DeepEqual(f.setDefault, []string{""}) || utils.FileExists(instShortcutPath("A")) {
		t.Fatalf("remove: %d %q %v", code, out, f.setDefault)
	}
	c := f.h.instances[1]
	f.registered["preview"] = utils.CanonicalWindowsPath(c.Dir)
	code, _, errOut = f.h.run("instance", "remove", "C", "-y")
	if code != ExitConflict || !strings.Contains(errOut, "instance unregister C") {
		t.Fatalf("registered: %d %s", code, errOut)
	}
	code, out, errOut = f.h.run("instance", "remove", "B", "C", "-y")
	if code != ExitPartial || out != "Removed B\n" || !strings.Contains(errOut, "error: C:") {
		t.Fatalf("partial: %d %q %s", code, out, errOut)
	}
}

func TestInstanceRegister(t *testing.T) {
	f := newInstFakes(t)
	other := f.add("Other", "1.21.50.07", "release", "gdk", false)
	f.add("New", "1.21.60.10", "release", "gdk", false)
	f.add("U", "1.20.81.1", "release", "uwp", false)
	f.registered["release"] = utils.CanonicalWindowsPath(other.Dir)

	code, _, errOut := f.h.run("instance", "register", "New")
	if code != ExitUsage || !strings.Contains(errOut, "replacing instance Other") || len(f.calls) != 0 {
		t.Fatalf("displacement: %d %s", code, errOut)
	}
	// A running holder would be closed by Windows: refused even with --yes.
	f.running["Other"] = []int{77}
	code, _, errOut = f.h.run("instance", "register", "New", "--yes")
	if code != ExitConflict || !strings.Contains(errOut, "would close it") || !strings.Contains(errOut, "stop Other") || len(f.calls) != 0 {
		t.Fatalf("running holder: %d %s", code, errOut)
	}
	delete(f.running, "Other")
	f.devMode = false
	code, _, errOut = f.h.run("instance", "register", "New", "--yes")
	if code != ExitSystem || !strings.Contains(errOut, "Developer Mode") || len(f.calls) != 0 {
		t.Fatalf("dev mode: %d %s", code, errOut)
	}
	f.devMode = true
	code, out, errOut := f.h.run("instance", "register", "New", "--yes", "--json")
	var res instRegisterResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || !res.Registered || !res.Changed || res.Displaced == nil || *res.Displaced != "Other" {
		t.Fatalf("register: %d %q %s", code, out, errOut)
	}
	code, _, errOut = f.h.run("instance", "register", "New")
	if code != 0 || len(f.calls) != 1 || !strings.Contains(errOut, "already registered") {
		t.Fatalf("idempotent: %d %v %s", code, f.calls, errOut)
	}
	// UWP does not need Developer Mode up front; verification catches a
	// registration Windows did not keep.
	f.devMode = false
	instRegister = func(_ context.Context, name string, _ bool) string {
		f.calls = append(f.calls, "register "+name)
		return ""
	}
	code, _, errOut = f.h.run("instance", "register", "U", "-y")
	if code != ExitFailure || !strings.Contains(errOut, "[ERR_UWP_NOT_REGISTERED]") {
		t.Fatalf("verification: %d %s", code, errOut)
	}
	instRegister = func(context.Context, string, bool) string { return "ERR_UWP_DEVELOPER_MODE: 0x80073cff" }
	code, _, errOut = f.h.run("instance", "register", "U", "-y", "-v")
	if code != ExitSystem || !strings.Contains(errOut, "detail: 0x80073cff") {
		t.Fatalf("backend code: %d %s", code, errOut)
	}
}

func TestInstanceUnregister(t *testing.T) {
	f := newInstFakes(t)
	a := f.add("A", "1.21.50.07", "preview", "gdk", false)
	code, _, errOut := f.h.run("instance", "unregister", "A")
	if code != 0 || len(f.calls) != 0 || !strings.Contains(errOut, "not registered") {
		t.Fatalf("idempotent: %d %s", code, errOut)
	}
	f.registered["preview"] = utils.CanonicalWindowsPath(a.Dir)
	code, _, errOut = f.h.run("instance", "unregister", "A")
	if code != 0 || len(f.calls) != 1 || f.registered["preview"] != "" {
		t.Fatalf("unregister: %d %v %s", code, f.calls, errOut)
	}
}

func TestInstanceCopyData(t *testing.T) {
	f := newInstFakes(t)
	target := f.add("T", "1.21.50.07", "release", "gdk", true)
	f.add("Shared", "1.21.50.07", "release", "gdk", false)
	src := f.add("S", "1.21.40.07", "release", "gdk", true)
	f.add("Pre", "1.21.60.20", "preview", "gdk", true)
	f.add("U", "1.20.81.1", "release", "uwp", true)
	instWriteFile(t, filepath.Join(f.systemData, "Users", "1", "games", "com.mojang", "minecraftWorlds", "w", "level.dat"), "12345")
	instWriteFile(t, filepath.Join(f.systemData, "options.txt"), "ab")

	for _, tc := range []struct {
		args []string
		exit int
		msg  string
	}{
		{[]string{"T"}, ExitUsage, "--from is required"},
		{[]string{"Shared", "--from", "system"}, ExitUsage, "--own-data"},
		{[]string{"U", "--from", "system"}, ExitUsage, "UWP"},
		{[]string{"T", "--from", "Pre"}, ExitUsage, "same channel"},
		{[]string{"T", "--from", "Shared"}, ExitUsage, "--from system"},
		{[]string{"T", "--from", "S"}, ExitNotFound, "no game data"},
		{[]string{"T", "--from", "T"}, ExitUsage, "into itself"},
		{[]string{"T", "--from", "nope"}, ExitNotFound, "nope"},
	} {
		code, _, errOut := f.h.run(append([]string{"instance", "copy-data"}, tc.args...)...)
		if code != tc.exit || !strings.Contains(errOut, tc.msg) {
			t.Errorf("%v: %d %s", tc.args, code, errOut)
		}
	}
	code, out, errOut := f.h.run("instance", "copy-data", "T", "--from", "system", "--json")
	var res instCopyDataResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || res.Files != 2 || res.Bytes != 7 {
		t.Fatalf("copy: %d %q %s", code, out, errOut)
	}
	if b, _ := os.ReadFile(filepath.Join(target.Dir, "Minecraft Bedrock", "options.txt")); string(b) != "ab" {
		t.Fatalf("copied content %q", b)
	}
	// Copying into data that exists asks first.
	instWriteFile(t, filepath.Join(src.Dir, "Minecraft Bedrock", "options.txt"), "zzz")
	code, _, errOut = f.h.run("instance", "copy-data", "T", "--from", "S")
	if code != ExitUsage || !strings.Contains(errOut, "overwriting files") {
		t.Fatalf("confirmation: %d %s", code, errOut)
	}
	code, _, errOut = f.h.run("instance", "copy-data", "T", "--from", "S", "-y")
	if b, _ := os.ReadFile(filepath.Join(target.Dir, "Minecraft Bedrock", "options.txt")); code != 0 || string(b) != "zzz" {
		t.Fatalf("merge: %d %q %s", code, b, errOut)
	}
	// Ctrl-C stops the copy.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := instCopyTree(ctx, f.systemData, t.TempDir(), func(int64, int64) {}); err == nil {
		t.Fatal("cancelled copy succeeded")
	}
}

func TestInstanceShortcutAndPath(t *testing.T) {
	f := newInstFakes(t)
	inst := f.add("A", "1.21.50.07", "release", "gdk", true)
	code, out, _ := f.h.run("instance", "shortcut", "A")
	lnk := instShortcutPath("A")
	if code != 0 || out != lnk+"\n" || !utils.FileExists(lnk) {
		t.Fatalf("shortcut: %d %q", code, out)
	}
	code, _, errOut := f.h.run("instance", "shortcut", "A")
	if code != 0 || !strings.Contains(errOut, "Replaced") {
		t.Fatalf("replace: %d %s", code, errOut)
	}
	for i := 0; i < 2; i++ {
		if code, _, _ = f.h.run("instance", "shortcut", "A", "--remove"); code != 0 || utils.FileExists(lnk) {
			t.Fatalf("remove %d: %d", i, code)
		}
	}

	code, out, _ = f.h.run("instance", "path")
	if code != 0 || out != inst.Dir+"\n" {
		t.Fatalf("root: %d %q", code, out)
	}
	r := instFakeRoots(inst)
	worlds := filepath.Join(r.UsersRoot, "77", "games", "com.mojang", "minecraftWorlds")
	code, _, errOut = f.h.run("instance", "path", "worlds")
	if code != ExitNotFound || !strings.Contains(errOut, "no player profiles") {
		t.Fatalf("no players: %d %s", code, errOut)
	}
	instMkdirs(t, filepath.Join(r.UsersRoot, "77", "games", "com.mojang"))
	code, _, errOut = f.h.run("instance", "path", "A", "worlds")
	if code != ExitNotFound || !strings.Contains(errOut, "for this player") {
		t.Fatalf("missing folder: %d %s", code, errOut)
	}
	instMkdirs(t, worlds)
	code, out, _ = f.h.run("instance", "path", "worlds", "--open")
	if code != 0 || out != worlds+"\n" || f.calls[len(f.calls)-1] != "open "+worlds {
		t.Fatalf("worlds: %d %q %v", code, out, f.calls)
	}
	code, out, _ = f.h.run("--json", "instance", "path", "A", "data")
	var res instPathResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || res.Path != r.Base || res.Kind != "data" || !res.Exists {
		t.Fatalf("json: %d %q", code, out)
	}
	code, _, errOut = f.h.run("instance", "path", "A", "logs")
	if code != ExitUsage || !strings.Contains(errOut, "unknown folder kind") {
		t.Fatalf("bad kind: %d %s", code, errOut)
	}
}

func TestUse(t *testing.T) {
	f := newInstFakes(t)
	f.add("A", "1.21.50.07", "release", "gdk", false)
	code, out, _ := f.h.run("use")
	if code != 0 || out != "No default instance (using the only instance: A)\n" {
		t.Fatalf("only: %d %q", code, out)
	}
	f.add("B", "1.21.50.07", "release", "gdk", false)
	code, out, _ = f.h.run("use", "--json")
	if code != 0 || !strings.Contains(out, `"instance": null`) || !strings.Contains(out, `"source": "none"`) {
		t.Fatalf("none: %d %q", code, out)
	}
	code, _, errOut := f.h.run("use", "b")
	if code != 0 || !reflect.DeepEqual(f.setDefault, []string{"B"}) || !strings.Contains(errOut, "Default instance: B") {
		t.Fatalf("set: %d %v %s", code, f.setDefault, errOut)
	}
	code, _, errOut = f.h.run("use", "B")
	if code != 0 || len(f.setDefault) != 1 || !strings.Contains(errOut, "already") {
		t.Fatalf("idempotent: %d %v %s", code, f.setDefault, errOut)
	}
	code, out, _ = f.h.run("use", "-q")
	if code != 0 || out != "B\n" {
		t.Fatalf("show: %d %q", code, out)
	}
	f.h.env[EnvInstance] = "A"
	code, out, errOut = f.h.run("use", "B")
	if code != 0 || !strings.Contains(errOut, "overrides") {
		t.Fatalf("env warning: %d %s", code, errOut)
	}
	delete(f.h.env, EnvInstance)
	if code, _, _ = f.h.run("use", "--clear"); code != 0 || f.h.current != "" {
		t.Fatalf("clear: %d %q", code, f.h.current)
	}
	if code, _, _ = f.h.run("use", "nope"); code != ExitNotFound {
		t.Fatalf("unknown: %d", code)
	}
}
