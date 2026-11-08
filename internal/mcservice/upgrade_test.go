package mcservice

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestSwapInstanceKeepsUserDataAndTakesNewBuild(t *testing.T) {
	root := t.TempDir()
	inst := filepath.Join(root, "Instances", "A")
	newDir := filepath.Join(root, "Upgrades", "A_new")
	oldDir := filepath.Join(root, "Upgrades", "A_old")

	// Old instance: game files, user data, launcher state.
	writeTestFile(t, filepath.Join(inst, "Minecraft.Windows.exe"), "old exe")
	writeTestFile(t, filepath.Join(inst, "removed.dll"), "old dll")
	writeTestFile(t, filepath.Join(inst, "data", "resource.pak"), "old pak")
	writeTestFile(t, filepath.Join(inst, "Minecraft Bedrock", "Users", "x", "world.dat"), "world")
	writeTestFile(t, filepath.Join(inst, "mods", "LeviLamina", "a.dll"), "mod")
	writeTestFile(t, filepath.Join(inst, ".lip", "state.json"), "lip")
	writeTestFile(t, filepath.Join(inst, apppath.InstanceStateDirName, apppath.InstanceMetaFileName), "meta")
	writeTestFile(t, filepath.Join(inst, apppath.InstanceStateDirName, apppath.InstanceAuthKeyMarker), "old marker")
	writeTestFile(t, filepath.Join(inst, apppath.LegacyInstanceAuthKeyMarker), "legacy marker")
	writeTestFile(t, filepath.Join(inst, "notes.txt"), "user file")

	// New build, as the installer leaves it.
	writeTestFile(t, filepath.Join(newDir, "Minecraft.Windows.exe"), "new exe")
	writeTestFile(t, filepath.Join(newDir, "data", "resource.pak"), "new pak")
	writeTestFile(t, filepath.Join(newDir, apppath.InstanceStateDirName, apppath.InstanceAuthKeyMarker), "new marker")

	if code := swapInstance(inst, newDir, oldDir); code != "" {
		t.Fatalf("swapInstance: %s", code)
	}
	checks := map[string]string{
		"Minecraft.Windows.exe":               "new exe",
		"data/resource.pak":                   "new pak",
		"Minecraft Bedrock/Users/x/world.dat": "world",
		"mods/LeviLamina/a.dll":               "mod",
		".lip/state.json":                     "lip",
		apppath.InstanceStateDirName + "/" + apppath.InstanceMetaFileName:  "meta",
		apppath.InstanceStateDirName + "/" + apppath.InstanceAuthKeyMarker: "new marker",
		"notes.txt": "user file",
	}
	for rel, want := range checks {
		if got := readTestFile(t, filepath.Join(inst, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	for _, gone := range []string{"removed.dll", apppath.LegacyInstanceAuthKeyMarker} {
		if _, err := os.Stat(filepath.Join(inst, gone)); !os.IsNotExist(err) {
			t.Errorf("%s should not be carried over", gone)
		}
	}
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		t.Errorf("staging dir should be gone")
	}
}

func TestSwapInstanceRollsBackWhenNewBuildCannotMove(t *testing.T) {
	root := t.TempDir()
	inst := filepath.Join(root, "Instances", "A")
	newDir := filepath.Join(root, "Upgrades", "A_new")
	oldDir := filepath.Join(root, "Upgrades", "A_old")
	writeTestFile(t, filepath.Join(inst, "Minecraft.Windows.exe"), "old exe")
	writeTestFile(t, filepath.Join(inst, "mods", "m.dll"), "mod")
	writeTestFile(t, filepath.Join(newDir, "Minecraft.Windows.exe"), "new exe")

	// Hold a handle inside the new build so Windows refuses to rename it.
	f, err := os.Open(filepath.Join(newDir, "Minecraft.Windows.exe"))
	if err != nil {
		t.Fatal(err)
	}
	code := swapInstance(inst, newDir, oldDir)
	f.Close()
	if code == "" {
		t.Skip("filesystem allowed renaming a directory with an open file")
	}
	if got := readTestFile(t, filepath.Join(inst, "Minecraft.Windows.exe")); got != "old exe" {
		t.Errorf("exe = %q, want old exe restored", got)
	}
	if got := readTestFile(t, filepath.Join(inst, "mods", "m.dll")); got != "mod" {
		t.Errorf("mods not restored: %q", got)
	}
}

// A lip-installed LeviLamina keeps the unpatched executable as
// Minecraft.Windows.exe.bak and renames it back on uninstall. The old build's
// .bak must not survive the upgrade, or the next LeviLamina update restores
// the old executable over the new game.
func TestSwapInstanceReplacesExecutableBackups(t *testing.T) {
	root := t.TempDir()
	inst := filepath.Join(root, "Instances", "A")
	newDir := filepath.Join(root, "Upgrades", "A_new")
	oldDir := filepath.Join(root, "Upgrades", "A_old")

	writeTestFile(t, filepath.Join(inst, "Minecraft.Windows.exe"), "old exe patched")
	writeTestFile(t, filepath.Join(inst, "Minecraft.Windows.exe.bak"), "old exe")
	writeTestFile(t, filepath.Join(inst, "Minecraft.Windows.exe.orig"), "old exe before import patch")
	writeTestFile(t, filepath.Join(inst, "Removed.exe.bak"), "backup of a dropped exe")
	writeTestFile(t, filepath.Join(inst, "notes.bak"), "user file")
	writeTestFile(t, filepath.Join(newDir, "Minecraft.Windows.exe"), "new exe")

	if code := swapInstance(inst, newDir, oldDir); code != "" {
		t.Fatalf("swapInstance: %s", code)
	}
	checks := map[string]string{
		"Minecraft.Windows.exe":     "new exe",
		"Minecraft.Windows.exe.bak": "new exe",
		"notes.bak":                 "user file",
	}
	for rel, want := range checks {
		if got := readTestFile(t, filepath.Join(inst, rel)); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	for _, gone := range []string{"Minecraft.Windows.exe.orig", "Removed.exe.bak"} {
		if _, err := os.Stat(filepath.Join(inst, gone)); !os.IsNotExist(err) {
			t.Errorf("%s should not be carried over", gone)
		}
	}
}

// Without a lip backup in the old instance no .bak is invented.
func TestSwapInstanceWritesNoBackupWithoutLip(t *testing.T) {
	root := t.TempDir()
	inst := filepath.Join(root, "Instances", "A")
	newDir := filepath.Join(root, "Upgrades", "A_new")
	oldDir := filepath.Join(root, "Upgrades", "A_old")

	writeTestFile(t, filepath.Join(inst, "Minecraft.Windows.exe"), "old exe")
	writeTestFile(t, filepath.Join(newDir, "Minecraft.Windows.exe"), "new exe")

	if code := swapInstance(inst, newDir, oldDir); code != "" {
		t.Fatalf("swapInstance: %s", code)
	}
	if _, err := os.Stat(filepath.Join(inst, "Minecraft.Windows.exe.bak")); !os.IsNotExist(err) {
		t.Errorf("no .bak should be written when the old instance had none")
	}
}
