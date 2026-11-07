package cli

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/content"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

// contentHarness is a harness whose instances keep their content in temp
// folders laid out like the real ones.
type contentHarness struct {
	*harness
	roots map[string]types.ContentRoots
	cache string // gamertag cache file
}

func newContentHarness(t *testing.T) *contentHarness {
	h := &contentHarness{harness: newHarness(t), roots: map[string]types.ContentRoots{}}
	h.cache = filepath.Join(t.TempDir(), "gamertags.json")
	savedRoots, savedUsers := contentRootsByName, usersRootOf
	savedBackup, savedPlain, savedNow := worldBackupWithVersion, worldBackupPlain, worldNow
	savedList, savedPing := serverListFn, serverPingFn
	savedXUID, savedCache := playerGamertagByXUID, playerGamertagCachePath
	savedImp, savedDel, savedTr, savedShot := newPackImporter, packDelete, packTransfer, screenshotDelete
	t.Cleanup(func() {
		contentRootsByName, usersRootOf = savedRoots, savedUsers
		worldBackupWithVersion, worldBackupPlain, worldNow = savedBackup, savedPlain, savedNow
		serverListFn, serverPingFn = savedList, savedPing
		playerGamertagByXUID, playerGamertagCachePath = savedXUID, savedCache
		newPackImporter, packDelete, packTransfer, screenshotDelete = savedImp, savedDel, savedTr, savedShot
	})
	contentRootsByName = func(name string) types.ContentRoots {
		for k, r := range h.roots {
			if strings.EqualFold(k, name) {
				return r
			}
		}
		return types.ContentRoots{}
	}
	usersRootOf = func(inst Instance) string { return contentRootsByName(inst.Name).UsersRoot }
	worldBackupWithVersion = func(string, string) string { t.Error("unexpected default backup"); return "" }
	worldBackupPlain = func(string) string { return "" }
	playerGamertagByXUID = func(context.Context, uint64) (string, error) { return "", xbox.ErrInteractionRequired }
	playerGamertagCachePath = func() string { return h.cache }
	gamertagCache = playerReadCache
	serverListFn = func(string, string) ([]types.Server, error) { return nil, nil }
	serverPingFn = func(string) (*mcservice.MotdBEInfo, error) { t.Error("unexpected ping"); return nil, nil }
	return h
}

// addGDK adds an isolated GDK instance with the given player profiles.
func (h *contentHarness) addGDK(name, gameVersion string, players ...string) types.ContentRoots {
	h.add(name, gameVersion, "release", versions.PackageTypeGDK)
	inst := &h.instances[len(h.instances)-1]
	inst.Meta.EnableIsolation = true
	base := filepath.Join(inst.Dir, "Minecraft Bedrock")
	users := filepath.Join(base, "Users")
	shared := filepath.Join(users, "Shared", "games", "com.mojang")
	r := types.ContentRoots{Base: base, PackageType: "gdk", UsersRoot: users, IsIsolation: true,
		ResourcePacks: filepath.Join(shared, "resource_packs"), BehaviorPacks: filepath.Join(shared, "behavior_packs"),
		SkinPacks: filepath.Join(shared, "skin_packs")}
	for _, p := range players {
		mustMkdir(h.t, filepath.Join(users, p, "games", "com.mojang", "minecraftWorlds"))
	}
	mustMkdir(h.t, shared)
	h.roots[name] = r
	return r
}

// addUWP adds a UWP instance; isolated chooses its own data folder.
func (h *contentHarness) addUWP(name, channel string, isolated bool) types.ContentRoots {
	h.add(name, "1.21.0.3", channel, versions.PackageTypeUWP)
	inst := &h.instances[len(h.instances)-1]
	inst.Meta.EnableIsolation = isolated
	cm := filepath.Join(inst.Dir, "data", "games", "com.mojang")
	r := types.ContentRoots{PackageType: "uwp", ComMojangRoot: cm, IsIsolation: isolated, IsPreview: channel == "preview",
		Worlds: filepath.Join(cm, "minecraftWorlds"), ResourcePacks: filepath.Join(cm, "resource_packs"),
		BehaviorPacks: filepath.Join(cm, "behavior_packs"), SkinPacks: filepath.Join(cm, "skin_packs"),
		Screenshots: filepath.Join(cm, "Screenshots")}
	h.roots[name] = r
	return r
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeZip(t *testing.T, path string, files map[string]string) string {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		e, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Write([]byte(files[n])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// makeWorld creates a world folder with levelname.txt and a level.dat.
func makeWorld(t *testing.T, worldsDir, id, name string) string {
	t.Helper()
	dir := filepath.Join(worldsDir, id)
	mustWrite(t, filepath.Join(dir, "levelname.txt"), name)
	root := map[string]any{
		"GameType": int32(0), "Difficulty": int32(2), "LevelName": name, "commandsEnabled": uint8(0),
		"RandomSeed": int64(42), "abilities": map[string]any{"mayfly": uint8(0), "flySpeed": float32(0.05)},
	}
	if err := content.EncodeLevelDat(dir, 10, root); err != nil {
		t.Fatal(err)
	}
	return dir
}

func worldsDirOf(r types.ContentRoots, player string) string {
	return contentPlayerDir(r, player, "minecraftWorlds")
}

func decodeJSON[T any](t *testing.T, s string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad JSON %q: %v", s, err)
	}
	return v
}

func TestContentPlayerDir(t *testing.T) {
	gdk := types.ContentRoots{PackageType: "gdk", UsersRoot: `C:\x\Users`}
	if got := contentPlayerDir(gdk, "123", "minecraftWorlds"); got != filepath.Join(`C:\x\Users`, "123", "games", "com.mojang", "minecraftWorlds") {
		t.Fatalf("gdk: %q", got)
	}
	for _, bad := range []string{"", ".", "..", `a\b`, "a/b", "c:"} {
		if got := contentPlayerDir(gdk, bad, "Screenshots"); got != "" {
			t.Errorf("player %q accepted: %q", bad, got)
		}
	}
	uwp := types.ContentRoots{PackageType: "uwp", ComMojangRoot: `C:\cm`, Worlds: `C:\w`}
	if got := contentPlayerDir(uwp, "", "minecraftWorlds"); got != `C:\w` {
		t.Fatalf("uwp worlds: %q", got)
	}
	if got := contentPlayerDir(uwp, "", "minecraftpe"); got != filepath.Join(`C:\cm`, "minecraftpe") {
		t.Fatalf("uwp minecraftpe: %q", got)
	}
}

func TestContentSharedSkins(t *testing.T) {
	for _, tc := range []struct {
		version, pkg string
		want         bool
	}{
		{"1.21.50.7", "gdk", false},
		{"1.26.0.0", "gdk", false},
		{"1.26.0.2", "gdk", true},
		{"1.26.10.4", "gdk", true},
		{"1.20.0.1", "uwp", true},
	} {
		inst := Instance{PackageType: tc.pkg, Meta: versions.VersionMeta{GameVersion: tc.version}}
		if got := contentSharedSkins(inst, types.ContentRoots{PackageType: tc.pkg}); got != tc.want {
			t.Errorf("%s %s: %v, want %v", tc.pkg, tc.version, got, tc.want)
		}
	}
}

func TestContentCheckTransferTarget(t *testing.T) {
	gdkIso := Instance{Name: "a", PackageType: "gdk", Meta: versions.VersionMeta{EnableIsolation: true}}
	gdkShared := Instance{Name: "b", PackageType: "gdk"}
	uwpRel := Instance{Name: "c", PackageType: "uwp", Channel: "release"}
	uwpPre := Instance{Name: "d", PackageType: "uwp", Channel: "preview"}
	srcGDK := types.ContentRoots{PackageType: "gdk"}
	srcUWPShared := types.ContentRoots{PackageType: "uwp"}
	srcUWPIso := types.ContentRoots{PackageType: "uwp", IsIsolation: true}
	for _, tc := range []struct {
		name  string
		src   Instance
		roots types.ContentRoots
		dst   Instance
		ok    bool
	}{
		{"isolated target", gdkShared, srcGDK, gdkIso, true},
		{"same instance", gdkIso, srcGDK, gdkIso, false},
		{"shared gdk target", gdkIso, srcGDK, gdkShared, false},
		{"gdk to uwp", gdkIso, srcGDK, uwpRel, true},
		{"shared uwp to same channel", Instance{Name: "x", PackageType: "uwp"}, srcUWPShared, uwpRel, false},
		{"shared uwp to other channel", Instance{Name: "x", PackageType: "uwp"}, srcUWPShared, uwpPre, true},
		{"isolated uwp to shared uwp", Instance{Name: "x", PackageType: "uwp"}, srcUWPIso, uwpRel, true},
	} {
		err := contentCheckTransferTarget(tc.src, tc.roots, tc.dst)
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v", tc.name, err)
		}
		if err != nil && FromError(err).Exit != ExitUsage {
			t.Errorf("%s: exit %d", tc.name, FromError(err).Exit)
		}
	}
}

func TestContentRunningPIDs(t *testing.T) {
	h := newContentHarness(t)
	h.addGDK("Iso", "1.21.50.7")
	h.add("Shared", "1.21.50.7", "release", "gdk")
	h.add("Other", "1.21.50.7", "release", "gdk")
	minecraftProcesses = func() []types.ProcessInfo {
		return []types.ProcessInfo{
			{Pid: 10, IsLauncher: true, VersionName: "Iso"},
			{Pid: 20, IsLauncher: true, VersionName: "Other"},
			{Pid: 30},
		}
	}
	iso, _ := LookupInstance("iso")
	shared, _ := LookupInstance("shared")
	if got := contentRunningPIDs(iso); len(got) != 1 || got[0] != 10 {
		t.Fatalf("isolated: %v", got)
	}
	if got := contentRunningPIDs(shared); len(got) != 2 || got[0] != 20 || got[1] != 30 {
		t.Fatalf("shared: %v", got)
	}
	err := contentEnsureStopped(iso, "worlds")
	if e := FromError(err); e == nil || e.Exit != ExitConflict || !strings.Contains(e.Hint, "stop Iso") {
		t.Fatalf("ensure stopped: %+v", e)
	}
}

func TestWorldListAndResolve(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111")
	wd := worldsDirOf(r, "111")
	makeWorld(t, wd, "bbb=", "Beta World")
	makeWorld(t, wd, "aaa=", "alpha")
	makeWorld(t, wd, "ccc=", "Twin")
	makeWorld(t, wd, "ddd=", "twin")

	code, out, errs := h.run("world", "list", "--json")
	if code != 0 {
		t.Fatalf("list: %d %s", code, errs)
	}
	items := decodeJSON[[]worldListItem](t, out)
	var names []string
	for _, it := range items {
		names = append(names, it.Name)
	}
	if strings.Join(names, ",") != "alpha,Beta World,Twin,twin" {
		t.Fatalf("order: %v", names)
	}
	code, out, _ = h.run("world", "ls", "-q", "beta")
	if code != 0 || strings.TrimSpace(out) != "bbb=" {
		t.Fatalf("filter -q: %d %q", code, out)
	}
	code, out, _ = h.run("world", "list", "--size")
	if code != 0 || !strings.Contains(out, "SIZE") || !strings.Contains(out, "Beta World") {
		t.Fatalf("table: %d %q", code, out)
	}

	worlds := worldScan(wd)
	if w, err := worldFind(worlds, "BETA WORLD", "Survival"); err != nil || w.ID != "bbb=" {
		t.Fatalf("by name: %v %v", w, err)
	}
	if w, err := worldFind(worlds, "aaa=", "Survival"); err != nil || w.Name != "alpha" {
		t.Fatalf("by id: %v %v", w, err)
	}
	if _, err := worldFind(worlds, "twin", "Survival"); FromError(err).Exit != ExitUsage || !strings.Contains(err.Error(), "ccc=") {
		t.Fatalf("ambiguous: %v", err)
	}
	if _, err := worldFind(worlds, "alpah", "Survival"); FromError(err).Exit != ExitNotFound || !strings.Contains(FromError(err).Hint, "alpha") {
		t.Fatalf("not found: %+v", FromError(err))
	}
}

func TestWorldNeedsPlayer(t *testing.T) {
	h := newContentHarness(t)
	h.addGDK("Survival", "1.21.50.7", "111", "222")
	code, _, errs := h.run("world", "list")
	if code != ExitUsage || !strings.Contains(errs, "ERR_PLAYER_REQUIRED") {
		t.Fatalf("no player: %d %s", code, errs)
	}
	code, _, errs = h.run("world", "list", "-p", "222")
	if code != 0 {
		t.Fatalf("with -p: %d %s", code, errs)
	}
}

func TestWorldExportImportRoundTrip(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111")
	wd := worldsDirOf(r, "111")
	makeWorld(t, wd, "abc=", "My Survival")
	outDir := t.TempDir()
	file := filepath.Join(outDir, "s.mcworld")

	code, out, errs := h.run("world", "export", "My Survival", "-o", file)
	if code != 0 || !strings.EqualFold(strings.TrimSpace(out), file) {
		t.Fatalf("export: %d %q %s", code, out, errs)
	}
	code, _, errs = h.run("world", "export", "abc=", "-o", file)
	if code != ExitConflict {
		t.Fatalf("export over a file: %d %s", code, errs)
	}
	worldNow = func() time.Time { return time.Date(2024, 5, 1, 14, 3, 22, 0, time.Local) }
	code, out, _ = h.run("world", "export", "abc=", "-o", outDir+`\`)
	if code != 0 || filepath.Base(strings.TrimSpace(out)) != "My Survival_20240501-140322.mcworld" {
		t.Fatalf("export to dir: %d %q", code, out)
	}
	newDir := filepath.Join(outDir, "new folder")
	code, out, _ = h.run("world", "export", "abc=", "-o", newDir)
	if code != 0 || !strings.EqualFold(filepath.Dir(strings.TrimSpace(out)), newDir) {
		t.Fatalf("export to a new folder: %d %q", code, out)
	}
	code, _, _ = h.run("world", "export", "abc=", "abc=", "-o", file)
	if code != ExitConflict {
		t.Fatalf("existing file, repeated world: %d", code)
	}

	code, out, errs = h.run("world", "import", file)
	if code != 0 || !strings.Contains(out, "imported s.mcworld as") {
		t.Fatalf("import: %d %q %s", code, out, errs)
	}
	worlds := worldScan(wd)
	if len(worlds) != 2 || worlds[0].Name != "My Survival" || worlds[1].Name != "My Survival" {
		t.Fatalf("after import: %+v", worlds)
	}
	code, out, _ = h.run("world", "import", file, "-q")
	if code != 0 || len(strings.Fields(out)) != 1 || !utilsDirExists(filepath.Join(wd, strings.TrimSpace(out))) {
		t.Fatalf("import -q: %d %q", code, out)
	}

	code, _, errs = h.run("world", "import", filepath.Join(outDir, "pack.mcpack"))
	if code != ExitUsage || !strings.Contains(errs, "pack import") {
		t.Fatalf("wrong extension: %d %s", code, errs)
	}
	code, _, errs = h.run("world", "import", filepath.Join(outDir, "missing.mcworld"))
	if code != ExitNotFound {
		t.Fatalf("missing file: %d %s", code, errs)
	}
	bad := filepath.Join(outDir, "bad.mcworld")
	mustWrite(t, bad, "not a zip")
	before := len(worldScan(wd))
	code, _, errs = h.run("world", "import", bad)
	if code != ExitFailure || len(worldScan(wd)) != before {
		t.Fatalf("bad archive: %d %s", code, errs)
	}
	// One good file and one bad: partial.
	code, _, errs = h.run("world", "import", file, bad)
	if code != ExitPartial || !strings.Contains(errs, "failed "+bad) {
		t.Fatalf("partial: %d %s", code, errs)
	}
}

func utilsDirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func TestWorldExportDefaultFolder(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111")
	makeWorld(t, worldsDirOf(r, "111"), "abc=", "My Survival")
	var gotInst string
	worldBackupWithVersion = func(dir, inst string) string { gotInst = inst; return "" }
	worldBackupPlain = func(dir string) string { return `C:\backups\x.mcworld` }
	code, out, _ := h.run("world", "export", "abc=", "--json")
	items := decodeJSON[[]contentItem](t, out)
	if code != 0 || gotInst != "Survival" || len(items) != 1 || items[0].Path != `C:\backups\x.mcworld` || items[0].Status != "exported" {
		t.Fatalf("default export: %d %q %q", code, gotInst, out)
	}
}

func TestWorldRemove(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111")
	wd := worldsDirOf(r, "111")
	makeWorld(t, wd, "a=", "One")
	makeWorld(t, wd, "b=", "Two")
	makeWorld(t, wd, "c=", "Three")

	code, _, errs := h.run("world", "remove", "One")
	if code != ExitUsage || !strings.Contains(errs, "ERR_CONFIRMATION_REQUIRED") || !strings.Contains(errs, "--yes") {
		t.Fatalf("no confirmation: %d %s", code, errs)
	}
	h.console, h.stdin = true, "n\n"
	code, _, _ = h.run("world", "remove", "One")
	if code != ExitCancelled || !utilsDirExists(filepath.Join(wd, "a=")) {
		t.Fatalf("declined: %d", code)
	}
	h.stdin = "y\n"
	code, out, errs := h.run("world", "remove", "One")
	if code != 0 || utilsDirExists(filepath.Join(wd, "a=")) || !strings.Contains(out, `removed world "One" (a=)`) {
		t.Fatalf("confirmed: %d %q %s", code, out, errs)
	}
	h.console = false
	code, _, errs = h.run("world", "rm", "Nope", "--yes")
	if code != ExitNotFound {
		t.Fatalf("unknown world: %d %s", code, errs)
	}
	minecraftProcesses = func() []types.ProcessInfo {
		return []types.ProcessInfo{{Pid: 7, IsLauncher: true, VersionName: "Survival"}}
	}
	code, _, errs = h.run("world", "rm", "Two", "--yes")
	if code != ExitConflict || !strings.Contains(errs, "ERR_GAME_RUNNING") {
		t.Fatalf("game running: %d %s", code, errs)
	}
	minecraftProcesses = func() []types.ProcessInfo { return nil }
	code, out, errs = h.run("world", "rm", "Two", "c=", "--yes", "--json")
	items := decodeJSON[[]contentItem](t, out)
	if code != 0 || len(items) != 2 || items[0].Status != "removed" || len(worldScan(wd)) != 0 {
		t.Fatalf("batch: %d %q %s", code, out, errs)
	}
}

func TestWorldRename(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111")
	dir := makeWorld(t, worldsDirOf(r, "111"), "abc=", "Old")
	code, out, errs := h.run("world", "rename", "Old", "New Name")
	if code != 0 || !strings.Contains(out, `to "New Name"`) {
		t.Fatalf("rename: %d %q %s", code, out, errs)
	}
	if got := mcservice.GetWorldLevelName(dir); got != "New Name" {
		t.Fatalf("levelname.txt: %q", got)
	}
	if name, ok, _ := worldLevelNameField(dir); !ok || name != "New Name" {
		t.Fatalf("LevelName: %q %v", name, ok)
	}
	if _, err := os.Stat(filepath.Join(dir, "level.dat_leviold")); err != nil {
		t.Fatalf("backup: %v", err)
	}
	code, out, errs = h.run("world", "rename", "abc=", "New Name", "--json")
	res := decodeJSON[worldRenameResult](t, out)
	if code != 0 || res.Changed {
		t.Fatalf("idempotent: %d %q %s", code, out, errs)
	}
	code, _, _ = h.run("world", "rename", "abc=", "  ")
	if code != ExitUsage {
		t.Fatalf("empty name: %d", code)
	}
}

func TestWorldGetSet(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111")
	dir := makeWorld(t, worldsDirOf(r, "111"), "abc=", "Mine")

	code, out, errs := h.run("world", "get", "Mine", "--json")
	if code != 0 {
		t.Fatalf("get: %d %s", code, errs)
	}
	doc := decodeJSON[worldSettingsDoc](t, out)
	if doc.Version != 10 || len(doc.Fields) != 6 {
		t.Fatalf("doc: %+v", doc)
	}
	byName := map[string]worldSetting{}
	for _, f := range doc.Fields {
		byName[f.Name] = f
	}
	if f := byName["GameType"]; f.Type != "int" || f.Value != float64(0) || f.Label != "Survival/WorldDefault" {
		t.Fatalf("GameType: %+v", f)
	}
	if f := byName["abilities"]; f.Type != "compound" {
		t.Fatalf("abilities: %+v", f)
	}
	code, out, _ = h.run("world", "get", "Mine", "difficulty")
	if code != 0 || out != "2\n" {
		t.Fatalf("single value: %d %q", code, out)
	}
	code, out, _ = h.run("world", "get", "Mine", "abilities")
	if code != 0 || !strings.Contains(out, "mayfly") || !strings.Contains(out, "flySpeed") {
		t.Fatalf("compound: %d %q", code, out)
	}
	code, _, errs = h.run("world", "get", "Mine", "GameTyp")
	if code != ExitNotFound || !strings.Contains(errs, `"GameType"`) {
		t.Fatalf("unknown key: %d %s", code, errs)
	}

	code, out, errs = h.run("world", "set", "Mine", "GameType=creative", "Difficulty=hard", "abilities/mayfly=true", "commandsEnabled=on")
	if code != 0 {
		t.Fatalf("set: %d %s", code, errs)
	}
	for _, want := range []string{"GameType: 0 (Survival/WorldDefault) -> 1 (Creative)", "Difficulty: 2 (Normal) -> 3 (Hard)", "abilities/mayfly: 0 -> 1", "commandsEnabled: 0 -> 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	root, _, err := content.DecodeLevelDat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root["GameType"] != int32(1) || root["Difficulty"] != int32(3) || root["abilities"].(map[string]any)["mayfly"] != uint8(1) || root["LevelName"] != "Mine" {
		t.Fatalf("written: %#v", root)
	}
	code, _, errs = h.run("world", "set", "Mine", "GameType=1")
	if code != 0 || !strings.Contains(errs, "Nothing to change") {
		t.Fatalf("no-op: %d %s", code, errs)
	}
	code, _, errs = h.run("world", "set", "Mine", "newRule=1")
	if code != ExitUsage || !strings.Contains(errs, "ERR_TYPE_REQUIRED") {
		t.Fatalf("new key without type: %d %s", code, errs)
	}
	code, out, errs = h.run("world", "set", "Mine", "newRule=1", "--type", "byte", "--json")
	res := decodeJSON[worldSetResult](t, out)
	if code != 0 || len(res.Changes) != 1 || !res.Changes[0].Added {
		t.Fatalf("add: %d %q %s", code, out, errs)
	}
	code, _, errs = h.run("world", "set", "Mine", "GameType=7.5")
	if code != ExitUsage || !strings.Contains(errs, "ERR_INVALID_VALUE") {
		t.Fatalf("bad value: %d %s", code, errs)
	}
	code, _, errs = h.run("world", "set", "Mine", "nope/x=1", "--type", "int")
	if code != ExitNotFound {
		t.Fatalf("missing compound: %d %s", code, errs)
	}
	code, _, errs = h.run("world", "set", "Mine", "GameType")
	if code != ExitUsage {
		t.Fatalf("no '=': %d %s", code, errs)
	}
	minecraftProcesses = func() []types.ProcessInfo {
		return []types.ProcessInfo{{Pid: 1, IsLauncher: true, VersionName: "Survival"}}
	}
	code, _, _ = h.run("world", "set", "Mine", "GameType=0")
	if code != ExitConflict {
		t.Fatalf("game running: %d", code)
	}
}

func TestWorldParseValue(t *testing.T) {
	for _, tc := range []struct {
		name, tag, raw string
		want           string
		ok             bool
	}{
		{"GameType", "int", "creative", "1", true},
		{"GameType", "int", "Survival", "0", true},
		{"GameType", "int", "spectator", "6", true},
		{"Difficulty", "int", "PEACEFUL", "0", true},
		{"Difficulty", "int", "brutal", "", false},
		{"x", "byte", "true", "1", true},
		{"x", "byte", "off", "0", true},
		{"x", "byte", "300", "", false},
		{"x", "short", "-32768", "-32768", true},
		{"x", "int", "2147483648", "", false},
		{"x", "long", "9007199254740993", "9007199254740993", true},
		{"x", "float", "0.1", "0.1", true},
		{"x", "double", "abc", "", false},
		{"x", "string", " keep spaces ", " keep spaces ", true},
		{"x", "list", "[1, 2]", "[1,2]", true},
		{"x", "list", "{}", "", false},
		{"x", "compound", `{"a": 1}`, `{"a":1}`, true},
		{"x", "compound", "[]", "", false},
		{"x", "unknown", "1", "", false},
	} {
		f, err := worldParseValue(tc.name, tc.tag, tc.raw)
		if (err == nil) != tc.ok {
			t.Errorf("%s %s %q: %v", tc.name, tc.tag, tc.raw, err)
			continue
		}
		if err != nil {
			if FromError(err).Exit != ExitUsage {
				t.Errorf("%s %q: exit %d", tc.tag, tc.raw, FromError(err).Exit)
			}
			continue
		}
		got := f.ValueString
		if tc.tag == "list" || tc.tag == "compound" {
			got = f.ValueJSON
		}
		if got != tc.want {
			t.Errorf("%s %s %q = %q, want %q", tc.name, tc.tag, tc.raw, got, tc.want)
		}
	}
}

func TestWorldSortFields(t *testing.T) {
	fields := []types.LevelDatField{{Name: "zeta"}, {Name: "Alpha"}, {Name: "_x"}, {Name: "beta"}, {Name: "Ordered"}}
	worldSortFields(fields, []string{"Ordered"})
	var got []string
	for _, f := range fields {
		got = append(got, f.Name)
	}
	if strings.Join(got, ",") != "Ordered,Alpha,beta,zeta,_x" {
		t.Fatalf("order: %v", got)
	}
}

func TestWorldCopy(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Src", "1.21.50.7", "111")
	makeWorld(t, worldsDirOf(r, "111"), "w=", "Copied")
	dst := h.addGDK("Dst", "1.21.60.1", "999", "111")
	makeWorld(t, worldsDirOf(dst, "111"), "w=", "Existing")
	h.add("SharedGDK", "1.21.50.7", "release", "gdk")

	code, out, errs := h.run("world", "copy", "Copied", "--to", "dst", "-i", "Src")
	if code != 0 || !strings.Contains(out, "as w=_1") {
		t.Fatalf("copy: %d %q %s", code, out, errs)
	}
	if got := mcservice.GetWorldLevelName(filepath.Join(worldsDirOf(dst, "111"), "w=_1")); got != "Copied" {
		t.Fatalf("copied world: %q", got)
	}
	code, out, _ = h.run("world", "copy", "w=", "--to", "Dst", "-i", "Src", "--to-player", "999", "-q")
	if code != 0 || strings.TrimSpace(out) != "w=" || !utilsDirExists(filepath.Join(worldsDirOf(dst, "999"), "w=")) {
		t.Fatalf("copy to player: %d %q", code, out)
	}
	code, _, errs = h.run("world", "copy", "w=", "--to", "SharedGDK", "-i", "Src")
	if code != ExitUsage || !strings.Contains(errs, "ERR_INVALID_TARGET") {
		t.Fatalf("shared target: %d %s", code, errs)
	}
	code, _, errs = h.run("world", "copy", "w=", "-i", "Src")
	if code != ExitUsage || !strings.Contains(errs, "--to") {
		t.Fatalf("missing --to: %d %s", code, errs)
	}
}

func packManifest(uuid, name, module string) string {
	return `{"format_version":2,"header":{"name":"` + name + `","description":"§aGreen","uuid":"` + uuid +
		`","version":[1,2,3],"min_engine_version":[1,20,0]},"modules":[{"type":"` + module + `","uuid":"m-` + uuid + `","version":[1,0,0]}]}`
}

func TestPackImportListRemove(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111")
	files := t.TempDir()
	rp := writeZip(t, filepath.Join(files, "Faithful.mcpack"), map[string]string{
		"Faithful/manifest.json": packManifest("uuid-rp", "§lFaithful", "resources"), "Faithful/pack_icon.png": "x"})
	bp := writeZip(t, filepath.Join(files, "Mobs.mcpack"), map[string]string{"manifest.json": packManifest("uuid-bp", "Mobs", "data")})

	code, out, errs := h.run("pack", "import", rp, bp)
	if code != 0 || !strings.Contains(out, "imported Faithful.mcpack") || !strings.Contains(out, "imported Mobs.mcpack") {
		t.Fatalf("import: %d %q %s", code, out, errs)
	}
	code, out, errs = h.run("pack", "list", "--json")
	if code != 0 {
		t.Fatalf("list: %d %s", code, errs)
	}
	packs := decodeJSON[[]packEntry](t, out)
	if len(packs) != 2 || packs[0].Kind != "resource" || packs[0].Name != "Faithful" || packs[0].UUID != "uuid-rp" ||
		packs[0].Version != "1.2.3" || packs[0].MinEngineVersion != "1.20.0" || packs[0].Description != "Green" || packs[1].Kind != "behavior" {
		t.Fatalf("packs: %+v", packs)
	}
	if !contentUnder(packs[0].Path, r.ResourcePacks) || packs[0].ID == "" {
		t.Fatalf("pack location: %+v", packs[0])
	}
	code, out, _ = h.run("pack", "list", "--kind", "behavior", "-q")
	if code != 0 || strings.TrimSpace(out) != packs[1].ID {
		t.Fatalf("kind filter: %d %q", code, out)
	}

	// Duplicates: no policy without a console fails with exit 4.
	code, _, errs = h.run("pack", "import", rp)
	if code != ExitConflict || !strings.Contains(errs, "--overwrite") {
		t.Fatalf("duplicate: %d %s", code, errs)
	}
	code, out, _ = h.run("pack", "import", rp, "--skip-existing")
	if code != 0 || !strings.Contains(out, "skipped Faithful.mcpack") {
		t.Fatalf("skip: %d %q", code, out)
	}
	code, out, _ = h.run("pack", "import", rp, "--overwrite")
	if code != 0 || !strings.Contains(out, "overwritten Faithful.mcpack") {
		t.Fatalf("overwrite: %d %q", code, out)
	}
	h.console, h.stdin = true, "a\n"
	code, out, _ = h.run("pack", "import", rp, bp)
	if code != 0 || strings.Count(out, "overwritten") != 2 {
		t.Fatalf("prompt all: %d %q", code, out)
	}
	h.console = false
	newPack := writeZip(t, filepath.Join(files, "New.mcpack"), map[string]string{"manifest.json": packManifest("uuid-new", "New", "resources")})
	code, _, errs = h.run("pack", "import", rp, newPack)
	if code != ExitPartial || !strings.Contains(errs, "1 imported") {
		t.Fatalf("partial: %d %s", code, errs)
	}
	code, _, errs = h.run("pack", "import", rp, "--overwrite", "--skip-existing")
	if code != ExitUsage {
		t.Fatalf("both policies: %d %s", code, errs)
	}
	code, _, errs = h.run("pack", "import", filepath.Join(files, "w.mcworld"))
	if code != ExitUsage || !strings.Contains(errs, "world import") {
		t.Fatalf("mcworld: %d %s", code, errs)
	}

	code, out, errs = h.run("pack", "remove", "faithful", "uuid-bp", "--yes")
	if code != 0 || !strings.Contains(out, `removed resource pack "Faithful"`) || !strings.Contains(out, `removed behavior pack "Mobs"`) {
		t.Fatalf("remove: %d %q %s", code, out, errs)
	}
	if left := contentDirNames(r.ResourcePacks); len(left) != 1 {
		t.Fatalf("resource packs left: %v", left)
	}
	if left := contentDirNames(r.BehaviorPacks); len(left) != 0 {
		t.Fatalf("behavior packs left: %v", left)
	}
	code, _, _ = h.run("pack", "remove", "faithful", "--yes")
	if code != ExitNotFound {
		t.Fatalf("remove again: %d", code)
	}
}

func TestPackRemoveNestedLeavesNoEmptyFolder(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111")
	mustWrite(t, filepath.Join(r.ResourcePacks, "top", "inner", "manifest.json"), packManifest("u1", "Nested", "resources"))
	code, _, errs := h.run("pack", "rm", "top", "--yes")
	if code != 0 || utilsDirExists(filepath.Join(r.ResourcePacks, "top")) {
		t.Fatalf("nested remove: %d %s", code, errs)
	}
}

func TestPackSkinPlayers(t *testing.T) {
	h := newContentHarness(t)
	old := h.addGDK("Old", "1.21.50.7", "111", "222")
	files := t.TempDir()
	skin := writeZip(t, filepath.Join(files, "Skins.mcpack"), map[string]string{"manifest.json": packManifest("uuid-skin", "Skins", "skin_pack")})

	code, _, errs := h.run("pack", "import", skin, "-i", "Old")
	if code != ExitUsage || !strings.Contains(errs, "ERR_PLAYER_REQUIRED") {
		t.Fatalf("skin without player: %d %s", code, errs)
	}
	code, _, errs = h.run("pack", "import", skin, "-i", "Old", "-p", "222")
	if code != 0 {
		t.Fatalf("skin with player: %d %s", code, errs)
	}
	if n := len(contentDirNames(filepath.Join(old.UsersRoot, "222", "games", "com.mojang", "skin_packs"))); n != 1 {
		t.Fatalf("per-player skin dir: %d", n)
	}
	// pack list without -p skips per-player skins with a notice.
	code, out, errs := h.run("pack", "list", "-i", "Old")
	if code != 0 || out != "" || !strings.Contains(errs, "pass -p") {
		t.Fatalf("list without player: %d %q %s", code, out, errs)
	}
	code, out, _ = h.run("pack", "list", "-i", "Old", "-p", "222", "--json")
	if got := decodeJSON[[]packEntry](t, out); code != 0 || len(got) != 1 || got[0].Kind != "skin" {
		t.Fatalf("list with player: %d %q", code, out)
	}
	code, _, errs = h.run("pack", "list", "-i", "Old", "--kind", "skin")
	if code != ExitUsage {
		t.Fatalf("explicit skins without player: %d %s", code, errs)
	}

	newer := h.addGDK("New", "1.26.10.4", "111", "222")
	code, _, errs = h.run("pack", "import", skin, "-i", "New")
	if code != 0 {
		t.Fatalf("shared skins: %d %s", code, errs)
	}
	if n := len(contentDirNames(newer.SkinPacks)); n != 1 {
		t.Fatalf("shared skin dir: %d", n)
	}
	// A shared skin pack copies into another shared-skin instance, not into
	// one that keeps skins per player.
	h.addGDK("New2", "1.26.10.4", "111")
	code, _, errs = h.run("pack", "copy", "Skins", "--to", "New2", "-i", "New")
	if code != 0 {
		t.Fatalf("copy shared skin: %d %s", code, errs)
	}
	code, _, errs = h.run("pack", "copy", "Skins", "--to", "Old", "-i", "New")
	if code != ExitConflict || !strings.Contains(errs, "per player") {
		t.Fatalf("copy skin into per-player instance: %d %s", code, errs)
	}
}

func TestPackCopyAll(t *testing.T) {
	h := newContentHarness(t)
	src := h.addGDK("Src", "1.21.50.7", "111")
	dst := h.addGDK("Dst", "1.21.50.7", "111")
	mustWrite(t, filepath.Join(src.ResourcePacks, "r1", "manifest.json"), packManifest("u-r", "Res", "resources"))
	mustWrite(t, filepath.Join(src.BehaviorPacks, "b1", "manifest.json"), packManifest("u-b", "Beh", "data"))

	code, out, errs := h.run("pack", "copy", "--all", "--to", "Dst", "-i", "Src", "--json")
	items := decodeJSON[[]contentItem](t, out)
	if code != 0 || len(items) != 2 || items[0].Status != "copied" || items[1].Status != "copied" {
		t.Fatalf("copy all: %d %q %s", code, out, errs)
	}
	if len(contentDirNames(dst.ResourcePacks)) != 1 || len(contentDirNames(dst.BehaviorPacks)) != 1 {
		t.Fatal("packs not copied")
	}
	code, out, _ = h.run("pack", "copy", "--all", "--to", "Dst", "-i", "Src", "--skip-existing")
	if code != 0 || strings.Count(out, "skipped") != 2 {
		t.Fatalf("copy again: %d %q", code, out)
	}
	code, _, _ = h.run("pack", "copy", "Res", "--all", "--to", "Dst", "-i", "Src")
	if code != ExitUsage {
		t.Fatalf("names and --all: %d", code)
	}
	code, _, _ = h.run("pack", "copy", "--to", "Dst", "-i", "Src")
	if code != ExitUsage {
		t.Fatalf("nothing named: %d", code)
	}
}

func TestScreenshots(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111")
	shots := filepath.Join(r.UsersRoot, "111", "games", "com.mojang", "Screenshots", "2535400000000000")
	mustWrite(t, filepath.Join(shots, "old.jpeg"), "x")
	mustWrite(t, filepath.Join(shots, "old.json"), `{"captureTime": 1000}`)
	mustWrite(t, filepath.Join(shots, "old.mc"), "x")
	mustWrite(t, filepath.Join(shots, "new.png"), "x")
	mustWrite(t, filepath.Join(shots, "new.json"), `{"captureTime": 2000}`)

	code, out, errs := h.run("screenshot", "list", "--json")
	items := decodeJSON[[]screenshotItem](t, out)
	if code != 0 || len(items) != 2 || items[0].Name != "new" || items[1].TakenAt.Unix() != 1000 {
		t.Fatalf("list: %d %q %s", code, out, errs)
	}
	code, out, _ = h.run("screenshot", "ls", "-q")
	if code != 0 || len(strings.Split(strings.TrimSpace(out), "\n")) != 2 {
		t.Fatalf("-q: %d %q", code, out)
	}
	code, _, _ = h.run("screenshot", "remove")
	if code != ExitUsage {
		t.Fatalf("nothing named: %d", code)
	}
	code, out, errs = h.run("screenshot", "rm", "old", "--yes")
	if code != 0 || !strings.Contains(out, "removed screenshot old") {
		t.Fatalf("remove: %d %q %s", code, out, errs)
	}
	for _, f := range []string{"old.jpeg", "old.json", "old.mc"} {
		if _, err := os.Stat(filepath.Join(shots, f)); !os.IsNotExist(err) {
			t.Fatalf("%s left: %v", f, err)
		}
	}
	code, _, _ = h.run("screenshot", "rm", "old", "--yes")
	if code != ExitNotFound {
		t.Fatalf("remove again: %d", code)
	}
	screenshotDelete = func(string, string, string) string { return "" } // the backend ignores errors
	code, _, errs = h.run("screenshot", "rm", "--all", "--yes")
	if code != ExitFailure || !strings.Contains(errs, "cannot delete screenshot new") {
		t.Fatalf("surfaced failure: %d %s", code, errs)
	}
}

type fakeTimeout struct{}

func (fakeTimeout) Error() string   { return "i/o timeout" }
func (fakeTimeout) Timeout() bool   { return true }
func (fakeTimeout) Temporary() bool { return true }

func TestServerList(t *testing.T) {
	h := newContentHarness(t)
	h.addGDK("Survival", "1.21.50.7", "111")
	var gotPlayer string
	serverListFn = func(inst, player string) ([]types.Server, error) {
		gotPlayer = player
		return []types.Server{
			{Name: "Hive", IP: "geo.hivebedrock.network", Port: "19132", Timestamp: 1700000000},
			{Name: "Gone", IP: "nonexistent.invalid", Port: "19132"},
			{Name: "Quiet", IP: "10.0.0.1", Port: "19133"},
		}, nil
	}
	serverPingFn = func(addr string) (*mcservice.MotdBEInfo, error) {
		switch {
		case strings.HasPrefix(addr, "geo."):
			return &mcservice.MotdBEInfo{Status: "online", Motd: "§bThe Hive", Version: "1.21.50", Agreement: 766, Online: 10, Max: 100, Delay: 42}, nil
		case strings.HasPrefix(addr, "nonexistent."):
			return &mcservice.MotdBEInfo{Status: "offline"}, &net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host", Name: "nonexistent.invalid"}}
		}
		return &mcservice.MotdBEInfo{Status: "offline"}, &net.OpError{Op: "read", Err: fakeTimeout{}}
	}
	code, out, errs := h.run("server", "list", "--json")
	if code != 0 || gotPlayer != "111" {
		t.Fatalf("list: %d %s %q", code, errs, gotPlayer)
	}
	items := decodeJSON[[]serverItem](t, out)
	if len(items) != 3 || items[0].Status != "online" || items[0].Motd != "The Hive" || *items[0].Players != 10 || *items[0].LatencyMs != 42 ||
		items[1].Status != "unreachable" || items[2].Status != "offline" || items[0].Added.Unix() != 1700000000 {
		t.Fatalf("items: %q", out)
	}
	code, out, _ = h.run("server", "list", "hive", "-q")
	if code != 0 || strings.TrimSpace(out) != "geo.hivebedrock.network:19132" {
		t.Fatalf("filter: %d %q", code, out)
	}
	serverPingFn = func(string) (*mcservice.MotdBEInfo, error) { t.Fatal("pinged with --no-ping"); return nil, nil }
	code, out, _ = h.run("server", "list", "--no-ping")
	if code != 0 || !strings.Contains(out, "Quiet") || strings.Contains(out, "online") {
		t.Fatalf("--no-ping: %d %q", code, out)
	}
	serverListFn = func(string, string) ([]types.Server, error) { return nil, errors.New("denied") }
	code, _, _ = h.run("server", "list")
	if code != ExitFailure {
		t.Fatalf("read failure: %d", code)
	}
}

func TestPlayerList(t *testing.T) {
	h := newContentHarness(t)
	r := h.addGDK("Survival", "1.21.50.7", "111", "222", "333")
	mustMkdir(t, filepath.Join(r.UsersRoot, "Shared"))
	mustMkdir(t, filepath.Join(r.UsersRoot, "9556213259376595538"))
	makeWorld(t, worldsDirOf(r, "222"), "w=", "W")
	mustMkdir(t, filepath.Join(r.UsersRoot, "333", "games", "com.mojang", "Screenshots", "2535412345"))
	mustWrite(t, h.cache, `{"111":"Alex","other":"Kept"}`)
	signedInGamertag = func(context.Context) string { return "alex" }

	code, out, errs := h.run("player", "list", "--json")
	if code != 0 {
		t.Fatalf("list: %d %s", code, errs)
	}
	items := decodeJSON[[]playerItem](t, out)
	if len(items) != 3 {
		t.Fatalf("items: %q", out)
	}
	var alex playerItem
	for _, it := range items {
		if it.ID == "111" {
			alex = it
		}
		if it.ID == "222" && it.Worlds != 1 {
			t.Fatalf("worlds: %+v", it)
		}
	}
	if alex.Gamertag != "Alex" || !alex.SignedIn {
		t.Fatalf("alex: %+v", alex)
	}

	code, _, errs = h.run("player", "list", "--resolve-names")
	if code != 0 || !strings.Contains(errs, "not signed in") {
		t.Fatalf("resolve signed out: %d %s", code, errs)
	}
	var asked uint64
	playerGamertagByXUID = func(_ context.Context, xuid uint64) (string, error) { asked = xuid; return "Steve", nil }
	code, out, _ = h.run("player", "list", "--resolve-names")
	if code != 0 || asked != 2535412345 || !strings.Contains(out, "Steve") {
		t.Fatalf("resolve: %d %d %q", code, asked, out)
	}
	cache := playerReadCache()
	if cache["333"] != "Steve" || cache["other"] != "Kept" || cache["111"] != "Alex" {
		t.Fatalf("cache: %v", cache)
	}
	// -p accepts the resolved gamertag.
	code, out, _ = h.run("world", "list", "-p", "steve", "-q")
	if code != 0 || out != "" {
		t.Fatalf("-p gamertag: %d %q", code, out)
	}

	h.addUWP("Legacy", "release", true)
	code, out, _ = h.run("player", "list", "-i", "Legacy", "--json")
	if code != 0 || strings.TrimSpace(out) != "[]" {
		t.Fatalf("uwp json: %d %q", code, out)
	}
	code, out, errs = h.run("player", "list", "-i", "Legacy")
	if code != 0 || out != "" || !strings.Contains(errs, "UWP") {
		t.Fatalf("uwp: %d %q %s", code, out, errs)
	}
}

func TestUWPWorldsAndPacks(t *testing.T) {
	h := newContentHarness(t)
	r := h.addUWP("Legacy", "release", true)
	makeWorld(t, r.Worlds, "u=", "Uwp World")
	code, out, errs := h.run("world", "list", "-q", "-p", "ignored")
	if code != 0 || strings.TrimSpace(out) != "u=" {
		t.Fatalf("uwp worlds: %d %q %s", code, out, errs)
	}
	files := t.TempDir()
	skin := writeZip(t, filepath.Join(files, "S.mcpack"), map[string]string{"manifest.json": packManifest("s", "S", "skin_pack")})
	code, _, errs = h.run("pack", "import", skin)
	if code != 0 || len(contentDirNames(r.SkinPacks)) != 1 {
		t.Fatalf("uwp skin import: %d %s", code, errs)
	}
}

func TestContentCommandsHaveExamples(t *testing.T) {
	for _, c := range Commands() {
		if c.noun == nil {
			continue
		}
		switch c.noun.Name {
		case "world", "pack", "screenshot", "server", "player":
		default:
			continue
		}
		if len(c.Examples) < 2 || len(c.Examples) > 3 {
			t.Errorf("%s: %d examples", c.Path, len(c.Examples))
		}
	}
}
