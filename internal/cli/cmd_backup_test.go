package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/backupmods"
	"github.com/liteldev/LeviLauncher/internal/types"
)

func gdkBackupInfo(name string) types.InstanceBackupInfo {
	return types.InstanceBackupInfo{Name: name, BackupDir: `C:\B\` + name, Scopes: []types.InstanceBackupScope{
		{Key: "gameData", Selectable: true, Exists: true, DefaultMode: "safe",
			Modes: []types.InstanceBackupScopeMode{{Key: "safe", Selectable: true}, {Key: "full", Selectable: true}}},
		{Key: "mods", Selectable: true, Exists: true},
	}}
}

func TestPlanBackupScopes(t *testing.T) {
	info := gdkBackupInfo("S")
	keys, modes, err := planBackupScopes(info, nil, false)
	if err != nil || !reflect.DeepEqual(keys, []string{"gameData", "mods"}) || !reflect.DeepEqual(modes, map[string]string{"gameData": "safe"}) {
		t.Fatalf("default: %v %v %v", keys, modes, err)
	}
	keys, modes, err = planBackupScopes(info, []string{"game-data"}, true)
	if err != nil || !reflect.DeepEqual(keys, []string{"gameData"}) || modes["gameData"] != "full" {
		t.Fatalf("--full: %v %v %v", keys, modes, err)
	}
	if _, _, err := planBackupScopes(info, []string{"mods"}, true); FromError(err).Exit != ExitUsage {
		t.Fatalf("--full without game data: %v", err)
	}
	uwp := info
	uwp.Scopes = []types.InstanceBackupScope{info.Scopes[0], {Key: "mods"}}
	if _, _, err := planBackupScopes(uwp, []string{"mods"}, false); FromError(err).Exit != ExitConflict {
		t.Fatalf("mods on UWP: %v", err)
	}
	empty := types.InstanceBackupInfo{Name: "E", Scopes: []types.InstanceBackupScope{{Key: "gameData"}, {Key: "mods"}}}
	if _, _, err := planBackupScopes(empty, nil, false); FromError(err).Exit != ExitNotFound {
		t.Fatalf("nothing to back up: %v", err)
	}
}

type fakeBackup struct {
	req         types.InstanceBackupRequest
	restoreReq  types.InstanceBackupRestoreRequest
	restored    int
	conflicts   []types.InstanceBackupRestoreConflict
	status      string
	gameRunning bool
	pkgErr      error
}

func installFakeBackup(t *testing.T) *fakeBackup {
	t.Helper()
	f := &fakeBackup{status: "success"}
	saved := []any{bkInfo, bkCreate, bkInspect, bkPreview, bkRestore, bkPackages, bkScanMods, bkGameRunning}
	t.Cleanup(func() {
		bkInfo = saved[0].(func(string) types.InstanceBackupInfo)
		bkCreate = saved[1].(func(string, types.InstanceBackupRequest) types.InstanceBackupResult)
		bkInspect = saved[2].(func(string) types.InstanceBackupArchiveInfo)
		bkPreview = saved[3].(func(string, types.InstanceBackupRestoreRequest) types.InstanceBackupRestoreConflictInfo)
		bkRestore = saved[4].(func(context.Context, string, types.InstanceBackupRestoreRequest) types.InstanceBackupRestoreResult)
		bkPackages = saved[5].(func(context.Context, string) ([]backupmods.Package, error))
		bkScanMods = saved[6].(func(string) []backupmods.Mod)
		bkGameRunning = saved[7].(func(string) bool)
	})
	bkInfo = gdkBackupInfo
	bkCreate = func(name string, req types.InstanceBackupRequest) types.InstanceBackupResult {
		f.req = req
		return types.InstanceBackupResult{ArchivePath: `C:\B\` + name + `\x.zip`, IncludedScopes: req.Scopes}
	}
	bkInspect = func(p string) types.InstanceBackupArchiveInfo {
		return types.InstanceBackupArchiveInfo{Name: "Survival", ArchivePath: p, IncludedScopes: []string{"gameData", "mods"},
			CreatedAt: "2026-01-02T03:04:05Z", GameVersion: "1.21.50"}
	}
	bkPreview = func(name string, req types.InstanceBackupRestoreRequest) types.InstanceBackupRestoreConflictInfo {
		scopes := req.Scopes
		if len(scopes) == 0 {
			scopes = []string{"gameData", "mods"}
		}
		return types.InstanceBackupRestoreConflictInfo{IncludedScopes: scopes, Conflicts: f.conflicts}
	}
	bkRestore = func(_ context.Context, name string, req types.InstanceBackupRestoreRequest) types.InstanceBackupRestoreResult {
		f.restored++
		f.restoreReq = req
		r := types.InstanceBackupRestoreResult{Status: f.status, IncludedScopes: req.Scopes}
		for _, s := range req.Scopes {
			sr := types.InstanceBackupRestoreScopeResult{Key: s, Status: "success"}
			if f.status == "partial" && s == "mods" {
				sr.Status, sr.Warnings = "partial", []string{"ERR_INSTANCE_BACKUP_RESTORE_LIP_INSTALL_FAILED"}
			}
			r.ScopeResults = append(r.ScopeResults, sr)
		}
		return r
	}
	bkPackages = func(context.Context, string) ([]backupmods.Package, error) {
		if f.pkgErr != nil {
			return nil, f.pkgErr
		}
		return []backupmods.Package{{ID: "github.com/LiteLDev/LegacyMoney", Version: "0.9.1", Explicit: true, Folders: []string{"LegacyMoney"}}}, nil
	}
	bkScanMods = func(string) []backupmods.Mod {
		return []backupmods.Mod{{Folder: "LegacyMoney", Name: "LegacyMoney", Version: "0.9.1"}}
	}
	bkGameRunning = func(string) bool { return f.gameRunning }
	return f
}

func TestBackupCreate(t *testing.T) {
	h := newHarness(t)
	f := installFakeBackup(t)
	h.add("Survival", "1.21.50", "release", "gdk")
	code, out, errOut := h.run("backup", "create")
	if code != 0 || out != `C:\B\Survival\x.zip`+"\n" {
		t.Fatalf("create: %d %q %q", code, out, errOut)
	}
	if !reflect.DeepEqual(f.req.Scopes, []string{"gameData", "mods"}) || len(f.req.ModsLIPPackages) != 1 ||
		f.req.ModsLIPPackages[0].Identifier != "LiteLDev/LegacyMoney" || f.req.ScopeModes["gameData"] != "safe" {
		t.Fatalf("request: %+v", f.req)
	}
	code, out, _ = h.run("backup", "create", "Survival", "--scope", "game-data", "--full", "--json")
	var r backupCreateResult
	if code != 0 || json.Unmarshal([]byte(out), &r) != nil || !reflect.DeepEqual(r.IncludedScopes, []string{"game-data"}) || f.req.ScopeModes["gameData"] != "full" {
		t.Fatalf("--full --json: %d %q %+v", code, out, f.req)
	}
	f.pkgErr = backupmods.ErrModsStateUnavailable
	if code, _, errOut := h.run("backup", "create"); code != ExitConflict || !strings.Contains(errOut, "--scope game-data") {
		t.Fatalf("mods state unavailable: %d %q", code, errOut)
	}
	if code, _, _ := h.run("backup", "create", "Creative"); code != ExitNotFound {
		t.Fatalf("unknown instance: %d", code)
	}
}

func worldConflict(id string) types.InstanceBackupRestoreConflict {
	return types.InstanceBackupRestoreConflict{ID: id, ScopeKey: "gameData", IdentityKind: "world_folder", Path: "minecraftWorlds/" + id,
		BackupSummary: "Old World", CurrentSummary: "New World",
		DiffFields: []types.InstanceBackupRestoreConflictDiffField{{Key: "level_name", BackupValue: "Old", CurrentValue: "New"}}}
}

func TestDecideConflicts(t *testing.T) {
	cs := []restoreConflict{{ID: "a::b=c"}, {ID: "x"}, {ID: "y"}}
	if err := decideConflicts(cs, []string{"a::b=c=current", "x=backup"}, ""); err != nil {
		t.Fatal(err)
	}
	if cs[0].Decision != "current" || cs[1].Decision != "backup" || cs[2].Decision != "" {
		t.Fatalf("%+v", cs)
	}
	if err := decideConflicts(cs, nil, "backup"); err != nil || cs[2].Decision != "backup" || cs[0].Decision != "current" {
		t.Fatalf("--prefer must not override --resolve: %+v", cs)
	}
	for _, bad := range []string{"x", "x=both", "nope=backup"} {
		if err := decideConflicts(cs, []string{bad}, ""); FromError(err).Exit != ExitUsage {
			t.Errorf("--resolve %q: %v", bad, err)
		}
	}
}

func TestBackupRestore(t *testing.T) {
	h := newHarness(t)
	f := installFakeBackup(t)
	h.add("Survival", "1.21.50", "release", "gdk")
	archive := filepath.Join(t.TempDir(), "Survival_1.zip")
	if err := os.WriteFile(archive, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.conflicts = []types.InstanceBackupRestoreConflict{worldConflict("w1"), worldConflict("w2")}

	// --dry-run lists the conflicts and restores nothing; the instance comes
	// from the archive.
	code, out, errOut := h.run("backup", "restore", archive, "--dry-run", "--json")
	var plan restorePlan
	if code != 0 || json.Unmarshal([]byte(out), &plan) != nil || len(plan.Conflicts) != 2 || plan.Instance != "Survival" ||
		plan.Conflicts[0].Kind != "world" || plan.Conflicts[0].Differs["level_name"] != "Old -> New" || f.restored != 0 {
		t.Fatalf("dry run: %d %q %q", code, out, errOut)
	}
	if _, ok := plan.Resolutions["w1"]; !ok {
		t.Fatalf("resolutions template: %+v", plan.Resolutions)
	}

	// Undecided conflicts without a console.
	if code, _, errOut := h.run("backup", "restore", archive, "--yes"); code != ExitUsage || !strings.Contains(errOut, "ERR_INSTANCE_BACKUP_RESTORE_CONFLICT_UNRESOLVED") {
		t.Fatalf("unresolved: %d %q", code, errOut)
	}
	// Decided but not confirmed.
	if code, _, errOut := h.run("backup", "restore", archive, "--prefer", "backup"); code != ExitUsage || !strings.Contains(errOut, "ERR_CONFIRMATION_REQUIRED") || f.restored != 0 {
		t.Fatalf("no --yes: %d %q", code, errOut)
	}
	code, out, errOut = h.run("backup", "restore", "Survival", archive, "--prefer", "backup", "--resolve", "w2=current", "--yes")
	if code != 0 || f.restored != 1 || !strings.Contains(out, "game-data") {
		t.Fatalf("restore: %d %q %q", code, out, errOut)
	}
	want := []types.InstanceBackupRestoreResolution{{ConflictID: "w1", Choice: "backup"}, {ConflictID: "w2", Choice: "current"}}
	if !reflect.DeepEqual(f.restoreReq.ConflictResolutions, want) {
		t.Fatalf("resolutions: %+v", f.restoreReq.ConflictResolutions)
	}

	// Partial restore: exit 7 with the warning in English.
	f.status = "partial"
	code, out, _ = h.run("backup", "restore", archive, "--prefer", "current", "--yes")
	if code != ExitPartial || !strings.Contains(out, "could not be reinstalled") {
		t.Fatalf("partial: %d %q", code, out)
	}
	f.status = "success"

	// The game is running.
	f.gameRunning = true
	if code, _, _ := h.run("backup", "restore", archive, "--prefer", "current", "--yes"); code != ExitConflict {
		t.Fatalf("game running: %d", code)
	}
	f.gameRunning = false
	// Another game on the shared game data blocks a game-data restore, but
	// not a mods-only one.
	minecraftProcesses = func() []types.ProcessInfo {
		return []types.ProcessInfo{{Pid: 77, IsLauncher: true, VersionName: "Other"}}
	}
	if code, _, errOut := h.run("backup", "restore", archive, "--prefer", "current", "--yes"); code != ExitConflict || !strings.Contains(errOut, "PID 77") {
		t.Fatalf("shared data in use: %d %q", code, errOut)
	}
	if code, _, errOut := h.run("backup", "restore", archive, "--scope", "mods", "--prefer", "current", "--yes"); code != 0 || strings.Contains(errOut, "PID 77") {
		t.Fatalf("mods only: %d %q", code, errOut)
	}
	minecraftProcesses = func() []types.ProcessInfo { return nil }
	if code, _, _ := h.run("backup", "restore", filepath.Join(t.TempDir(), "missing.zip")); code != ExitNotFound {
		t.Fatalf("missing archive: %d", code)
	}
}

func TestBackupRestorePrompts(t *testing.T) {
	h := newHarness(t)
	f := installFakeBackup(t)
	h.add("Survival", "1.21.50", "release", "gdk")
	archive := filepath.Join(t.TempDir(), "a.zip")
	if err := os.WriteFile(archive, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.conflicts = []types.InstanceBackupRestoreConflict{worldConflict("w1"), worldConflict("w2"), worldConflict("w3")}
	h.console = true
	h.stdin = "x\nc\nB\ny\n" // invalid, current, backup for the rest, confirm
	code, _, errOut := h.run("backup", "restore", archive)
	if code != 0 || f.restored != 1 {
		t.Fatalf("prompted restore: %d %q", code, errOut)
	}
	got := map[string]string{}
	for _, r := range f.restoreReq.ConflictResolutions {
		got[r.ConflictID] = r.Choice
	}
	if !reflect.DeepEqual(got, map[string]string{"w1": "current", "w2": "backup", "w3": "backup"}) {
		t.Fatalf("decisions: %v", got)
	}
}

func TestBackupList(t *testing.T) {
	h := newHarness(t)
	installFakeBackup(t)
	root := sysSandbox(t)
	apppath.SetBaseRootOverride(filepath.Join(root, "Data"))
	write := func(p string, mod time.Time) {
		writeSized(t, p, 3)
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	write(filepath.Join(apppath.InstanceBackupsDir(), "Survival", "Survival_20260102-030405.zip"), old)
	write(filepath.Join(apppath.WorldBackupsDir(), "Survival", "abc_My World", "My World_20260101-000000.mcworld"), time.Now())
	write(filepath.Join(apppath.WorldBackupsDir(), "Creative", "def_Flat", "Flat_20260101-000000.mcworld"), time.Now())

	code, out, _ := h.run("backup", "list", "--json")
	var items []backupItem
	if code != 0 || json.Unmarshal([]byte(out), &items) != nil || len(items) != 3 {
		t.Fatalf("list: %d %q", code, out)
	}
	code, out, _ = h.run("backup", "list", "survival", "--json")
	items = nil
	if code != 0 || json.Unmarshal([]byte(out), &items) != nil || len(items) != 2 {
		t.Fatalf("list survival: %d %q", code, out)
	}
	if items[0].Kind != "world" || items[0].World != "My World" || items[1].Kind != "instance" || items[1].Created != "2026-01-02T03:04:05Z" ||
		!reflect.DeepEqual(items[1].Scopes, []string{"game-data", "mods"}) {
		t.Fatalf("items: %+v", items)
	}
	if code, out, _ := h.run("backup", "ls", "-i", "Creative", "-q"); code != 0 || !strings.HasSuffix(out, "Flat_20260101-000000.mcworld\n") {
		t.Fatalf("-q: %d %q", code, out)
	}
	if code, out, errOut := h.run("backup", "list", "Nobody"); code != 0 || out != "" || !strings.Contains(errOut, "No backups of Nobody") {
		t.Fatalf("none: %d %q %q", code, out, errOut)
	}
}
