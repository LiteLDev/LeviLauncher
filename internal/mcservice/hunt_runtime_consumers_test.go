package mcservice

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/storage"
)

func huntWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func huntPS(t *testing.T, script string) string {
	t.Helper()
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("powershell: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// A pre-PR launcher created instance X and its desktop shortcut
// ("Minecraft - X.lnk", Arguments --launch=X, icon <instance>\LargeLogo.ico).
// The migration merges it into a current layout that already has an X, so
// it is kept as X-legacy. RepairMigratedShortcuts points the icon at
// X-legacy but leaves --launch=X: the shortcut now shows one instance and
// launches another, and the instance it was made for is no longer reachable
// from it.
func TestHunt_runtime_consumers_ShortcutOfLegacyRenamedInstanceLaunchesOther(t *testing.T) {
	root := t.TempDir()
	roaming, local, profile, public := filepath.Join(root, "Roaming"), filepath.Join(root, "Local"), filepath.Join(root, "Profile"), filepath.Join(root, "Public")
	for _, d := range []string{roaming, local, filepath.Join(profile, "Desktop"), filepath.Join(public, "Desktop")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("APPDATA", roaming)
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("USERPROFILE", profile)
	t.Setenv("PUBLIC", public)
	for _, k := range []string{"OneDrive", "OneDriveConsumer", "OneDriveCommercial"} {
		t.Setenv(k, "")
	}
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	legacy := apppath.LegacyDir()

	// First upgrade: X moves to the current layout.
	huntWrite(t, filepath.Join(legacy, "config.json"), `{}`)
	huntWrite(t, filepath.Join(legacy, "versions", "X", "version.json"), `{"name":"X"}`)
	huntWrite(t, filepath.Join(legacy, "versions", "X", "Minecraft.Windows.exe"), "exe")
	if outcome, err := storage.Prepare(); err != nil || outcome != storage.Migrated {
		t.Fatalf("first Prepare = %v, %v", outcome, err)
	}
	// Downgrade: the old launcher creates X again, with a desktop shortcut.
	huntWrite(t, filepath.Join(legacy, "config.json"), `{}`)
	oldX := filepath.Join(legacy, "versions", "X")
	huntWrite(t, filepath.Join(oldX, "version.json"), `{"name":"X"}`)
	huntWrite(t, filepath.Join(oldX, "Minecraft.Windows.exe"), "exe")
	huntWrite(t, filepath.Join(oldX, "LargeLogo.ico"), "ico")
	lnk := filepath.Join(profile, "Desktop", "Minecraft - X.lnk")
	esc := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
	huntPS(t, "$s = New-Object -ComObject WScript.Shell; $l = $s.CreateShortcut('"+esc(lnk)+"'); "+
		"$l.TargetPath = '"+esc(filepath.Join(root, "LeviLauncher.exe"))+"'; $l.Arguments = '--launch=X'; "+
		"$l.IconLocation = '"+esc(filepath.Join(oldX, "LargeLogo.ico"))+",0'; $l.Save()")
	// Upgrade again: X is kept as X-legacy.
	if outcome, err := storage.Prepare(); err != nil || outcome != storage.Migrated {
		t.Fatalf("second Prepare = %v, %v", outcome, err)
	}
	instances := filepath.Join(local, "LeviLauncher", "Instances")
	if _, err := os.Stat(filepath.Join(instances, "X-legacy", "version.json")); err != nil {
		t.Fatalf("setup: X-legacy missing: %v", err)
	}

	RepairMigratedShortcuts()

	out := huntPS(t, "$s = New-Object -ComObject WScript.Shell; $l = $s.CreateShortcut('"+esc(lnk)+"'); "+
		"[Console]::Out.WriteLine($l.IconLocation + '|' + $l.Arguments)")
	icon, args, _ := strings.Cut(out, "|")
	iconInstance := ""
	if rel, err := filepath.Rel(strings.ToLower(instances), strings.ToLower(icon)); err == nil && !strings.HasPrefix(rel, "..") {
		iconInstance = strings.SplitN(rel, string(filepath.Separator), 2)[0]
	}
	launched := strings.Trim(strings.TrimPrefix(strings.TrimSpace(args), "--launch="), `"`)
	if iconInstance == "" {
		t.Fatalf("setup: the shortcut icon was not repaired: %q", icon)
	}
	if !strings.EqualFold(iconInstance, launched) {
		t.Errorf("repaired shortcut shows instance %q (icon %s) but launches %q (Arguments %q): the legacy X it was made for is now X-legacy", iconInstance, icon, launched, args)
	}
}
