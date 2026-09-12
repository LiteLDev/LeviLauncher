package mcservice

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

func TestUWPStatusSeparatesPackageAndChannel(t *testing.T) {
	_, _, versionDir := setupInstanceBackupEnv(t)
	createTestInstance(t, versionDir, "uwp", versions.VersionMeta{GameVersion: "1.21.80.3", Type: "beta", PackageType: "uwp"})
	dir, err := apppath.InstallersDir()
	if err != nil {
		t.Fatal(err)
	}
	name, _ := uwpdownload.Filename("1.21.80.3", "beta")
	if err := os.WriteFile(filepath.Join(dir, name), []byte("archive"), 0600); err != nil {
		t.Fatal(err)
	}
	u := GetVersionStatusForPackage("1.21.80.3", "beta", "uwp")
	if !u.IsDownloaded || !u.IsInstalled {
		t.Fatalf("missing UWP status: %+v", u)
	}
	for _, target := range [][2]string{{"gdk", "beta"}, {"uwp", "preview"}, {"uwp", "release"}} {
		s := GetVersionStatusForPackage("1.21.80.3", target[1], target[0])
		if s.IsDownloaded || s.IsInstalled {
			t.Fatalf("cross-package/channel status: %+v", s)
		}
	}
	if code := DeleteDownloadedUWP("1.21.80.3", "beta"); code != "" {
		t.Fatal(code)
	}
	if got := ResolveDownloadedUWP("1.21.80.3", "beta"); got != "" {
		t.Fatal(got)
	}
}

func TestGDKCompletedDownloadStatusUsesBareVersion(t *testing.T) {
	setupInstanceBackupEnv(t)
	dir, err := apppath.InstallersDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Release 1.21.120.0.msixvc"), []byte("gdk"), 0600); err != nil {
		t.Fatal(err)
	}
	if s := GetVersionStatusForPackage("1.21.120.0", "release", "gdk"); !s.IsDownloaded {
		t.Fatalf("completion refresh lost downloaded state: %+v", s)
	}
	if s := GetVersionStatusForPackage("1.21.120.0", "preview", "gdk"); s.IsDownloaded {
		t.Fatal("release cache matched preview")
	}
	if s := GetVersionStatusForPackage("1.21.120.0", "release", "uwp"); s.IsDownloaded {
		t.Fatal("GDK cache matched UWP")
	}
}

func TestUWPSaveSettingsDoesNotPatchExecutable(t *testing.T) {
	_, _, versionDir := setupInstanceBackupEnv(t)
	dir := createTestInstance(t, versionDir, "uwp", versions.VersionMeta{GameVersion: "1.21.80.3", Type: "release", PackageType: "uwp"})
	exe := filepath.Join(dir, "Minecraft.Windows.exe")
	if err := os.WriteFile(exe, []byte("untouched UWP executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := SaveVersionMeta("uwp", "1.21.80.3", "release", true, true, true, "--editor", "TEST=1"); code != "" {
		t.Fatal(code)
	}
	m, err := versions.ReadMeta(dir)
	if err != nil || m.EnableIsolation || m.EnableConsole || m.EnableEditorMode || m.LaunchArgs != "" || m.EnvVars != "" || m.PackageType != "uwp" {
		t.Fatalf("UWP settings: %+v %v", m, err)
	}
	if data, err := os.ReadFile(exe); err != nil || string(data) != "untouched UWP executable" {
		t.Fatalf("modified executable: %q %v", data, err)
	}
}

func TestUWPBackupRoundTripAndPackageBoundary(t *testing.T) {
	_, _, versionDir := setupInstanceBackupEnv(t)
	t.Setenv("LOCALAPPDATA", t.TempDir())
	createTestInstance(t, versionDir, "uwp", versions.VersionMeta{GameVersion: "1.21.80.3", Type: "release", PackageType: "uwp"})
	createTestInstance(t, versionDir, "gdk", versions.VersionMeta{GameVersion: "1.21.80.3", Type: "release", PackageType: "gdk"})
	roots := GetContentRoots("uwp")
	if err := os.MkdirAll(filepath.Join(roots.Worlds, "world"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roots.Worlds, "world", "level.dat"), []byte("world"), 0600); err != nil {
		t.Fatal(err)
	}
	backup := BackupInstance("uwp", types.InstanceBackupRequest{Scopes: []string{"gameData"}})
	if backup.ErrorCode != "" {
		t.Fatal(backup.ErrorCode)
	}
	r, err := zip.OpenReader(backup.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range r.File {
		if f.Name == "gameData/games/com.mojang/minecraftWorlds/world/level.dat" {
			found = true
		}
	}
	r.Close()
	if !found {
		t.Fatal("UWP world missing or archived under GDK Users")
	}
	request := types.InstanceBackupRestoreRequest{ArchivePath: backup.ArchivePath, Scopes: []string{"gameData"}}
	preview := PreviewInstanceBackupRestoreConflicts("gdk", request)
	if preview.ErrorCode != "ERR_INSTANCE_BACKUP_PACKAGE_TYPE_MISMATCH" {
		t.Fatalf("cross-package preview: %+v", preview)
	}
	result := RestoreInstanceBackup(context.Background(), "gdk", request)
	if result.ErrorCode != "ERR_INSTANCE_BACKUP_PACKAGE_TYPE_MISMATCH" {
		t.Fatalf("cross-package restore: %+v", result)
	}
	// Change the sandbox LocalState root to restore without touching real data.
	t.Setenv("LOCALAPPDATA", t.TempDir())
	result = RestoreInstanceBackup(context.Background(), "uwp", request)
	if result.Status != "success" {
		t.Fatalf("UWP restore: %+v", result)
	}
	data, err := os.ReadFile(filepath.Join(GetContentRoots("uwp").Worlds, "world", "level.dat"))
	if err != nil || string(data) != "world" {
		t.Fatalf("restored world: %q %v", data, err)
	}
}

func TestBackupLayoutUsesPackageTypeWithMixedDirectories(t *testing.T) {
	base := t.TempDir()
	for _, prefix := range []string{"", "Users/player"} {
		folder := filepath.Join(base, filepath.FromSlash(prefix), "games", "com.mojang", "minecraftWorlds", "world")
		if err := os.MkdirAll(folder, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "level.dat"), []byte("world"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, platform := range []string{"gdk", "uwp"} {
		entries, _, err := collectInstanceBackupSafeGameDataEntries(base, platform)
		if err != nil || len(entries) != 1 {
			t.Fatalf("%s: %+v %v", platform, entries, err)
		}
		want := "gameData/games/com.mojang/minecraftWorlds"
		if platform == "gdk" {
			want = "gameData/Users/player/games/com.mojang/minecraftWorlds"
		}
		if entries[0].ArchivePath != want {
			t.Fatalf("%s selected %q", platform, entries[0].ArchivePath)
		}
		target := t.TempDir()
		if err := restoreInstanceBackupSafeGameData(base, target, platform); err != nil {
			t.Fatal(err)
		}
		prefix := ""
		if platform == "gdk" {
			prefix = "Users/player"
		}
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(prefix), "games", "com.mojang", "minecraftWorlds", "world", "level.dat")); err != nil {
			t.Fatal(err)
		}
	}
}
