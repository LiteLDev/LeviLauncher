package mcservice

// Regression tests for group "instance-files-external-refs": desktop
// shortcuts that launchers before the current layout created, repaired after
// the storage migration (internal/apppath/README.md, "迁移之后").

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/storage"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func migSCWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func migSCPowerShell(t *testing.T, script string) string {
	t.Helper()
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell: %v\n%s", err, out)
	}
	return string(out)
}

type migShortcut struct {
	path, icon, args string
}

func migSCq(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// migSCCreate writes shortcuts as CreateDesktopShortcut of v0.0.15 to v1.0.7
// did: TargetPath the launcher, Arguments --launch=<name>, IconLocation into
// the instance folder.
func migSCCreate(t *testing.T, target string, links ...migShortcut) {
	t.Helper()
	var b strings.Builder
	b.WriteString("$s = New-Object -ComObject WScript.Shell; ")
	for _, l := range links {
		b.WriteString("$l = $s.CreateShortcut(" + migSCq(l.path) + "); $l.TargetPath = " + migSCq(target) +
			"; $l.Arguments = " + migSCq(l.args) + "; $l.WorkingDirectory = " + migSCq(filepath.Dir(target)) +
			"; $l.IconLocation = " + migSCq(l.icon) + "; $l.Save(); ")
	}
	migSCPowerShell(t, b.String())
}

// migSCRead returns IconLocation (without the index) and Arguments of each
// shortcut.
func migSCRead(t *testing.T, paths ...string) map[string][2]string {
	t.Helper()
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = migSCq(p)
	}
	out := migSCPowerShell(t, "$s = New-Object -ComObject WScript.Shell; foreach ($p in @("+strings.Join(quoted, ",")+")) { $l = $s.CreateShortcut($p); [Console]::Out.WriteLine($p + '|' + $l.IconLocation + '|' + $l.Arguments) }")
	res := map[string][2]string{}
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n") {
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			continue
		}
		icon := parts[1]
		if i := strings.LastIndex(icon, ","); i > 0 {
			icon = icon[:i]
		}
		res[strings.ToLower(parts[0])] = [2]string{icon, parts[2]}
	}
	return res
}

func migSCSame(a, b string) bool {
	return utils.NormalizeWindowsPath(a) == utils.NormalizeWindowsPath(b)
}

// P8-desktop-shortcuts: every LeviLauncher shortcut on every desktop an
// older launcher may have used gets its icon pointed at the moved instance;
// other shortcuts and .cmd files are untouched; the repair runs once per
// migration; a shortcut whose instance the migration placed under "-legacy"
// keeps launching that instance.
func TestMigration_P8_DesktopShortcuts(t *testing.T) {
	if _, err := exec.LookPath("powershell"); err != nil {
		t.Skip("powershell not available")
	}
	root := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(root, "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "Local"))
	home := filepath.Join(root, "home")
	oneDrive := filepath.Join(root, "OneDriveEnv")
	public := filepath.Join(root, "Public")
	t.Setenv("USERPROFILE", home)
	t.Setenv("OneDrive", oneDrive)
	t.Setenv("OneDriveConsumer", "")
	t.Setenv("OneDriveCommercial", "")
	t.Setenv("PUBLIC", public)
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	t.Cleanup(func() {
		apppath.SetLegacyMode("")
		apppath.SetBaseRootOverride("")
	})
	desktops := map[string]string{
		"profile":  filepath.Join(home, "Desktop"),
		"onedrive": filepath.Join(home, "OneDrive", "Desktop"),
		"odenv":    filepath.Join(oneDrive, "Desktop"),
		"public":   filepath.Join(public, "Desktop"),
	}
	for _, d := range desktops {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	S := apppath.LegacyDir()
	vA := filepath.Join(S, "versions", "A")
	vB := filepath.Join(S, "versions", "B")
	png := "\x89PNG\r\n\x1a\nfake-png"
	migSCWrite(t, filepath.Join(vA, "version.json"), `{"name":"A"}`)
	migSCWrite(t, filepath.Join(vA, "LargeLogo.png"), png)
	migSCWrite(t, filepath.Join(vA, "LargeLogo.ico"), "old-ico")
	migSCWrite(t, filepath.Join(vA, "Minecraft.Windows.exe"), "exe-A")
	migSCWrite(t, filepath.Join(vB, "version.json"), `{"name":"B"}`)
	migSCWrite(t, filepath.Join(vB, "Minecraft.Windows.exe"), "exe-B")
	migSCWrite(t, filepath.Join(S, "config.json"), `{}`)
	launcher := filepath.Join(root, "bin", "LeviLauncher.exe")
	migSCWrite(t, launcher, "launcher")
	cmdFile := filepath.Join(desktops["profile"], "Minecraft - A.cmd")
	const cmdText = "@echo off\r\nstart \"\" \"launcher\" --launch=A\r\n"
	migSCWrite(t, cmdFile, cmdText)

	links := []migShortcut{
		{filepath.Join(desktops["profile"], "Minecraft - A.lnk"), filepath.Join(vA, "LargeLogo.ico"), "--launch=A"},
		{filepath.Join(desktops["onedrive"], "Minecraft - B.lnk"), filepath.Join(vB, "Minecraft.Windows.exe"), "--launch=B"},
		{filepath.Join(desktops["odenv"], "Minecraft - A.lnk"), filepath.Join(vA, "Minecraft.Windows.exe"), "--launch=A"},
		{filepath.Join(desktops["public"], "Minecraft - A.lnk"), filepath.Join(vA, "LargeLogo.ico"), "--launch=A"},
	}
	unrelated := migShortcut{filepath.Join(desktops["profile"], "Minecraft - Store.lnk"), `C:\Windows\System32\shell32.dll`, ""}
	migSCCreate(t, launcher, append(links, unrelated)...)

	if o, err := storage.Prepare(); err != nil || o != storage.Migrated {
		t.Fatalf("Prepare = %v, %v", o, err)
	}
	inst, _ := apppath.InstancesDir()
	nA, nB := filepath.Join(inst, "A"), filepath.Join(inst, "B")

	RepairMigratedShortcuts()
	var all []string
	for _, l := range links {
		all = append(all, l.path)
	}
	all = append(all, unrelated.path)
	got := migSCRead(t, all...)
	want := map[string]string{
		links[0].path:  filepath.Join(nA, ".levilauncher", "icon.ico"),
		links[1].path:  filepath.Join(nB, "Minecraft.Windows.exe"),
		links[2].path:  filepath.Join(nA, ".levilauncher", "icon.ico"),
		links[3].path:  filepath.Join(nA, ".levilauncher", "icon.ico"),
		unrelated.path: unrelated.icon,
	}
	for p, w := range want {
		g, ok := got[strings.ToLower(p)]
		if !ok {
			t.Errorf("%s not read back", p)
			continue
		}
		if !migSCSame(g[0], w) {
			t.Errorf("%s icon = %s, want %s", p, g[0], w)
		}
		if _, err := os.Stat(g[0]); err != nil && p != unrelated.path {
			t.Errorf("%s icon %s does not exist", p, g[0])
		}
	}
	if b, err := os.ReadFile(cmdFile); err != nil || string(b) != cmdText {
		t.Errorf(".cmd changed: %q, %v", b, err)
	}
	if b, _ := os.ReadFile(filepath.Join(nA, "LargeLogo.ico")); string(b) != "old-ico" {
		t.Error("LargeLogo.ico not kept")
	}
	if r := storage.LoadRecord(); r == nil || !r.ShortcutsRepaired {
		t.Fatalf("ShortcutsRepaired not set: %+v", r)
	}

	// Once per migration: a shortcut pointing at the legacy layout again is
	// left alone by the next start.
	migSCCreate(t, launcher, links[0])
	RepairMigratedShortcuts()
	if g := migSCRead(t, links[0].path)[strings.ToLower(links[0].path)]; !migSCSame(g[0], links[0].icon) {
		t.Errorf("repair ran again: icon = %s", g[0])
	}

	// Downgrade, then upgrade: the older launcher created another versions\A
	// and its shortcut "Minecraft - A" (replacing the one of the same name);
	// the next migration places that instance at A-legacy.
	migSCWrite(t, filepath.Join(S, "config.json"), `{}`)
	migSCWrite(t, filepath.Join(vA, "version.json"), `{"name":"A","gameVersion":"1.20.0.1"}`)
	migSCWrite(t, filepath.Join(vA, "LargeLogo.png"), png+"-second")
	migSCWrite(t, filepath.Join(vA, "Minecraft.Windows.exe"), "exe-A-second")
	migSCCreate(t, launcher, links[0])
	if o, err := storage.Prepare(); err != nil || o != storage.Migrated {
		t.Fatalf("second Prepare = %v, %v", o, err)
	}
	nAL := filepath.Join(inst, "A-legacy")
	if b, _ := os.ReadFile(filepath.Join(nAL, "Minecraft.Windows.exe")); string(b) != "exe-A-second" {
		t.Fatalf("second A not at A-legacy: %q", b)
	}
	if r := storage.LoadRecord(); r == nil || r.ShortcutsRepaired {
		t.Fatalf("a new migration must repair shortcuts again: %+v", r)
	}
	RepairMigratedShortcuts()
	g := migSCRead(t, links[0].path)[strings.ToLower(links[0].path)]
	t.Logf("Minecraft - A.lnk after the second migration: icon=%s args=%s", g[0], g[1])
	if !migSCSame(g[0], filepath.Join(nAL, ".levilauncher", "icon.ico")) {
		t.Errorf("icon = %s, want A-legacy's", g[0])
	}
	// The shortcut was made for the instance now at A-legacy: its icon says
	// so, and it must launch that instance, not Instances\A.
	launch := strings.Trim(strings.TrimPrefix(strings.TrimSpace(g[1]), "--launch="), `"`)
	iconInstance := filepath.Base(filepath.Dir(filepath.Dir(g[0])))
	if !strings.EqualFold(launch, iconInstance) {
		t.Errorf("Minecraft - A.lnk shows the icon of %s but launches --launch=%s (Instances\\%s, a different instance)", iconInstance, launch, launch)
	}
	if r := storage.LoadRecord(); r == nil || !r.ShortcutsRepaired {
		t.Errorf("ShortcutsRepaired not set after the second repair: %+v", r)
	}
	if o, err := storage.Prepare(); err != nil || o != storage.Current {
		t.Errorf("third Prepare = %v, %v; want Current", o, err)
	}
}
