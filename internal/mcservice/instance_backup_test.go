package mcservice

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	json "github.com/goccy/go-json"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

func TestGetInstanceBackupInfoIncludesSafeAndFullModes(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	versionDir := createTestInstance(t, versionsDir, "PreviewIsolation", versions.VersionMeta{
		GameVersion:     "1.21.90.3",
		Type:            "preview",
		EnableIsolation: true,
	})
	gameDataDir := createTestGameDataDir(t, versionDir, appDataDir, true, true)
	if err := os.MkdirAll(filepath.Join(gameDataDir, "Users", "Shared", "games", "com.mojang", "resource_packs"), 0o755); err != nil {
		t.Fatalf("mkdir resource_packs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gameDataDir, "Users", "Shared", "games", "com.mojang", "resource_packs", "pack.txt"), []byte("pack"), 0o644); err != nil {
		t.Fatalf("write pack file: %v", err)
	}

	info := GetInstanceBackupInfo("PreviewIsolation")
	if info.ErrorCode != "" {
		t.Fatalf("unexpected error code: %q", info.ErrorCode)
	}
	if len(info.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(info.Scopes))
	}

	gameData := info.Scopes[0]
	if gameData.Key != instanceBackupScopeGameData {
		t.Fatalf("unexpected game data scope: %+v", gameData)
	}
	if gameData.Label != "Minecraft Bedrock Preview" {
		t.Fatalf("unexpected label: %q", gameData.Label)
	}
	if gameData.DefaultMode != instanceBackupModeSafe {
		t.Fatalf("expected safe default mode, got %q", gameData.DefaultMode)
	}
	if len(gameData.Modes) != 2 {
		t.Fatalf("expected 2 modes, got %+v", gameData.Modes)
	}
	if gameData.Modes[0].Key != instanceBackupModeSafe || gameData.Modes[1].Key != instanceBackupModeFull {
		t.Fatalf("unexpected modes: %+v", gameData.Modes)
	}
	if !gameData.Modes[0].Selectable || !gameData.Modes[1].Selectable {
		t.Fatalf("expected safe/full modes selectable: %+v", gameData.Modes)
	}
	if gameData.Modes[1].Warning != "versions.edit.backup.mode.full.warning" {
		t.Fatalf("unexpected full warning key: %q", gameData.Modes[1].Warning)
	}
}

func TestGetInstanceBackupInfoMarksMissingModsScopeUnavailable(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	versionDir := createTestInstance(t, versionsDir, "NoMods", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, versionDir, appDataDir, false, true)

	info := GetInstanceBackupInfo("NoMods")
	if info.ErrorCode != "" {
		t.Fatalf("unexpected error code: %q", info.ErrorCode)
	}
	if len(info.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(info.Scopes))
	}

	modsScope := info.Scopes[1]
	if modsScope.Key != instanceBackupScopeMods {
		t.Fatalf("unexpected mods scope: %+v", modsScope)
	}
	if modsScope.Exists {
		t.Fatalf("expected missing mods scope, got %+v", modsScope)
	}
	if modsScope.Selectable {
		t.Fatalf("missing mods scope should not be selectable: %+v", modsScope)
	}
}

func TestBackupInstanceWritesSafeArchiveAndLIPMetadata(t *testing.T) {
	baseRoot, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	versionDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	gameDataDir := createTestGameDataDir(t, versionDir, appDataDir, false, true)
	allowedRoot := filepath.Join(gameDataDir, "Users", "PlayerOne", "games", "com.mojang", "minecraftWorlds", "Alpha")
	if err := os.MkdirAll(allowedRoot, 0o755); err != nil {
		t.Fatalf("mkdir allowed root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(allowedRoot, "levelname.txt"), []byte("Demo World"), 0o644); err != nil {
		t.Fatalf("write levelname: %v", err)
	}
	privateRoot := filepath.Join(gameDataDir, "Users", "PlayerOne", "private")
	if err := os.MkdirAll(privateRoot, 0o755); err != nil {
		t.Fatalf("mkdir private root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(privateRoot, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write secret file: %v", err)
	}

	modsDir := filepath.Join(versionDir, "mods")
	if err := os.MkdirAll(filepath.Join(modsDir, "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir raw mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modsDir, "raw_mod", "manifest.json"), []byte(`{"name":"raw","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("write raw manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(modsDir, "lip_mod"), 0o755); err != nil {
		t.Fatalf("mkdir lip mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modsDir, "lip_mod", "manifest.json"), []byte(`{"name":"lip","version":"2.0.0"}`), 0o644); err != nil {
		t.Fatalf("write lip manifest: %v", err)
	}

	result := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData", "mods"},
		ScopeModes: map[string]string{"gameData": "safe"},
		ModsLIPPackages: []types.InstanceBackupModsLIPPackage{
			{
				Identifier:        "LiteLDev/Foo",
				Version:           "2.0.0",
				ExplicitInstalled: true,
				Folders:           []string{"lip_mod"},
			},
		},
	})
	if result.ErrorCode != "" {
		t.Fatalf("unexpected error code: %q", result.ErrorCode)
	}
	if !strings.HasPrefix(result.ArchivePath, filepath.Join(baseRoot, "backups", "instances", "Source")) {
		t.Fatalf("unexpected archive path: %q", result.ArchivePath)
	}

	archiveBytes, err := os.ReadFile(result.ArchivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	names := readZipEntryNames(t, archiveBytes)
	if _, ok := names["gameData/Users/PlayerOne/games/com.mojang/minecraftWorlds/Alpha/levelname.txt"]; !ok {
		t.Fatalf("safe world entry missing: %v", mapKeys(names))
	}
	if _, ok := names["mods/raw/raw_mod/manifest.json"]; !ok {
		t.Fatalf("raw mod entry missing: %v", mapKeys(names))
	}
	if _, ok := names["mods/raw/lip_mod/manifest.json"]; ok {
		t.Fatalf("lip managed mod should not be archived as raw: %v", mapKeys(names))
	}
	for name := range names {
		if strings.Contains(name, "private/secret.txt") {
			t.Fatalf("private path leaked into safe backup: %s", name)
		}
	}

	manifestData := readZipFile(t, archiveBytes, instanceBackupManifestName)
	var manifest instanceBackupManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if bytes.Contains(manifestData, []byte(`"sourcePaths"`)) {
		t.Fatalf("source paths should not be written into manifest: %s", manifestData)
	}
	if manifest.FormatVersion != instanceBackupFormatVersion {
		t.Fatalf("unexpected format version: %d", manifest.FormatVersion)
	}
	if manifest.ScopeModes[instanceBackupScopeGameData] != instanceBackupModeSafe {
		t.Fatalf("unexpected scope modes: %+v", manifest.ScopeModes)
	}
	if len(manifest.LIPPackages) != 1 || manifest.LIPPackages[0].Identifier != "LiteLDev/Foo" {
		t.Fatalf("unexpected lip packages: %+v", manifest.LIPPackages)
	}
	if len(manifest.RawModFolders) != 1 || manifest.RawModFolders[0] != "raw_mod" {
		t.Fatalf("unexpected raw mod folders: %+v", manifest.RawModFolders)
	}
}

func TestBackupInstanceSkipsEmptyUserContentDirsButKeepsRequiredRoots(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	versionDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	gameDataDir := createTestGameDataDir(t, versionDir, appDataDir, false, true)
	emptyResourcePacks := filepath.Join(gameDataDir, "Users", "PlayerOne", "games", "com.mojang", "resource_packs")
	if err := os.MkdirAll(emptyResourcePacks, 0o755); err != nil {
		t.Fatalf("mkdir empty resource_packs: %v", err)
	}
	nonEmptyConfigFile := filepath.Join(gameDataDir, "Users", "PlayerOne", "games", "com.mojang", "config", "settings.json")
	if err := os.MkdirAll(filepath.Dir(nonEmptyConfigFile), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(nonEmptyConfigFile, []byte(`{"demo":true}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	emptyRawModDir := filepath.Join(versionDir, "mods", "empty_mod")
	if err := os.MkdirAll(emptyRawModDir, 0o755); err != nil {
		t.Fatalf("mkdir empty raw mod: %v", err)
	}

	result := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData", "mods"},
		ScopeModes: map[string]string{"gameData": "safe"},
	})
	if result.ErrorCode != "" {
		t.Fatalf("unexpected error code: %q", result.ErrorCode)
	}

	archiveBytes, err := os.ReadFile(result.ArchivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	names := readZipEntryNames(t, archiveBytes)
	if _, ok := names["gameData/"]; !ok {
		t.Fatalf("expected gameData root to be kept: %v", mapKeys(names))
	}
	if _, ok := names["mods/"]; !ok {
		t.Fatalf("expected mods root to be kept: %v", mapKeys(names))
	}
	if _, ok := names["mods/raw/"]; !ok {
		t.Fatalf("expected mods/raw root to be kept: %v", mapKeys(names))
	}
	if _, ok := names["gameData/Users/PlayerOne/games/com.mojang/resource_packs/"]; ok {
		t.Fatalf("empty resource_packs root should be skipped: %v", mapKeys(names))
	}
	if _, ok := names["mods/raw/empty_mod/"]; ok {
		t.Fatalf("empty raw mod directory should be skipped: %v", mapKeys(names))
	}

	manifestData := readZipFile(t, archiveBytes, instanceBackupManifestName)
	var manifest instanceBackupManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if len(manifest.RawModFolders) != 0 {
		t.Fatalf("empty raw mod folders should not be recorded: %+v", manifest.RawModFolders)
	}
}

func TestBackupInstanceRejectsMissingModsScope(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	versionDir := createTestInstance(t, versionsDir, "NoMods", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, versionDir, appDataDir, false, true)

	result := BackupInstance("NoMods", types.InstanceBackupRequest{
		Scopes: []string{"mods"},
	})
	if result.ErrorCode != "ERR_INSTANCE_BACKUP_INVALID_SCOPE" {
		t.Fatalf("unexpected error code: %+v", result)
	}
}

func TestInspectInstanceBackupArchiveRejectsInvalidFormat(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "invalid.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(file)
	if err := writeInstanceBackupManifestToZip(zw, instanceBackupManifest{
		FormatVersion: 1,
		Name:          "Legacy",
		SelectedScopes: []string{
			instanceBackupScopeGameData,
		},
	}); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	info := InspectInstanceBackupArchive(archivePath)
	if info.ErrorCode != "ERR_INSTANCE_BACKUP_INVALID_ARCHIVE" {
		t.Fatalf("unexpected error code: %+v", info)
	}
}

func TestInspectAndRestoreInstanceBackupLegacySourcePaths(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "LegacySource", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	sourceGameData := createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	writeWorld(t, filepath.Join(sourceGameData, "Users", "PlayerOne", "games", "com.mojang", "minecraftWorlds"), "alpha", "Legacy World")

	backup := BackupInstance("LegacySource", types.InstanceBackupRequest{
		Scopes:     []string{"gameData"},
		ScopeModes: map[string]string{"gameData": "safe"},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	legacyArchivePath := createLegacySourcePathsArchive(t, backup.ArchivePath, map[string]string{
		"gameData": filepath.Join(sourceDir, "Minecraft Bedrock"),
		"mods":     filepath.Join(sourceDir, "mods"),
	})

	info := InspectInstanceBackupArchive(legacyArchivePath)
	if info.ErrorCode != "" {
		t.Fatalf("inspect failed: %+v", info)
	}
	if len(info.IncludedScopes) != 1 || info.IncludedScopes[0] != instanceBackupScopeGameData {
		t.Fatalf("unexpected included scopes: %+v", info.IncludedScopes)
	}

	targetDir := createTestInstance(t, versionsDir, "LegacyTarget", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	targetGameData := createTestGameDataDir(t, targetDir, appDataDir, false, true)
	result := RestoreInstanceBackup(context.Background(), "LegacyTarget", types.InstanceBackupRestoreRequest{
		ArchivePath: legacyArchivePath,
		Scopes:      []string{"gameData"},
	})
	if result.Status != "success" {
		t.Fatalf("unexpected restore result: %+v", result)
	}
	if !utils.DirExists(filepath.Join(targetGameData, "Users", "PlayerOne", "games", "com.mojang", "minecraftWorlds", "alpha")) {
		t.Fatalf("expected world restored from legacy archive")
	}
}

func TestRestoreInstanceBackupSafeIgnoresLevelNameForWorldDedup(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	sourceGameData := createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	writeWorld(t, filepath.Join(sourceGameData, "Users", "PlayerOne", "games", "com.mojang", "minecraftWorlds"), "alpha", "Demo World")
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir raw mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "mods", "raw_mod", "manifest.json"), []byte(`{"name":"raw","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("write raw manifest: %v", err)
	}
	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData", "mods"},
		ScopeModes: map[string]string{"gameData": "safe"},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	targetGameData := createTestGameDataDir(t, targetDir, appDataDir, false, true)
	writeWorld(t, filepath.Join(targetGameData, "Users", "PlayerOne", "games", "com.mojang", "minecraftWorlds"), "beta", "Demo World")
	if err := os.MkdirAll(filepath.Join(targetDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir target raw mod: %v", err)
	}

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"gameData", "mods"},
	})
	if result.Status != "success" {
		t.Fatalf("unexpected restore result: %+v", result)
	}

	worldsRoot := filepath.Join(targetGameData, "Users", "PlayerOne", "games", "com.mojang", "minecraftWorlds")
	worldEntries, err := os.ReadDir(worldsRoot)
	if err != nil {
		t.Fatalf("read worlds: %v", err)
	}
	if len(worldEntries) != 2 {
		t.Fatalf("expected 2 worlds, got %d", len(worldEntries))
	}
	names := make([]string, 0, len(worldEntries))
	for _, entry := range worldEntries {
		if !entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "alpha,beta" {
		t.Fatalf("unexpected world folders: %+v", names)
	}

	if !utils.DirExists(filepath.Join(targetDir, "mods", "raw_mod")) || !utils.DirExists(filepath.Join(targetDir, "mods", "raw_mod (2)")) {
		t.Fatalf("expected raw mod conflict rename under target mods")
	}
}

func TestRestoreInstanceBackupModsReplacesSameManifestWithoutRename(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir source raw mod: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "mods", "raw_mod", "manifest.json"),
		[]byte(`{"name":"Same Mod","entry":"main.cpp","version":"2.0.0","type":"mod"}`),
		0o644,
	); err != nil {
		t.Fatalf("write source raw manifest: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "mods", "raw_mod", "payload.txt"),
		[]byte("source"),
		0o644,
	); err != nil {
		t.Fatalf("write source payload: %v", err)
	}

	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes: []string{"mods"},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, targetDir, appDataDir, false, true)
	if err := os.MkdirAll(filepath.Join(targetDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir target raw mod: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(targetDir, "mods", "raw_mod", "manifest.json"),
		[]byte(`{"name":"Same Mod","entry":"main.cpp","version":"1.0.0","type":"mod"}`),
		0o644,
	); err != nil {
		t.Fatalf("write target raw manifest: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(targetDir, "mods", "raw_mod", "old.txt"),
		[]byte("old"),
		0o644,
	); err != nil {
		t.Fatalf("write target old payload: %v", err)
	}

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"mods"},
	})
	if result.Status != "success" {
		t.Fatalf("unexpected restore result: %+v", result)
	}
	if utils.DirExists(filepath.Join(targetDir, "mods", "raw_mod (2)")) {
		t.Fatalf("same manifest mod should not be renamed")
	}
	manifestData, err := os.ReadFile(filepath.Join(targetDir, "mods", "raw_mod", "manifest.json"))
	if err != nil {
		t.Fatalf("read restored manifest: %v", err)
	}
	if !strings.Contains(string(manifestData), `"version":"2.0.0"`) {
		t.Fatalf("expected source manifest restored, got %s", manifestData)
	}
	if utils.FileExists(filepath.Join(targetDir, "mods", "raw_mod", "old.txt")) {
		t.Fatalf("expected previous target mod contents to be replaced")
	}
}

func TestPreviewInstanceBackupRestoreConflictsIncludesModsAndGameData(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	sourceGameData := createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	packDir := filepath.Join(sourceGameData, "Users", "PlayerOne", "games", "com.mojang", "resource_packs", "packA")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatalf("mkdir source resource pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "manifest.json"), []byte(`{"format_version":2,"header":{"name":"PackA","uuid":"pack-a-uuid","version":[2,0,0],"min_engine_version":[1,21,80]},"modules":[{"type":"resources","uuid":"pack-a-module","version":[2,0,0]}]}`), 0o644); err != nil {
		t.Fatalf("write source resource pack manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir source raw mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "mods", "raw_mod", "manifest.json"), []byte(`{"name":"Raw Mod","entry":"main.cpp","version":"1.0.0","type":"mod"}`), 0o644); err != nil {
		t.Fatalf("write source raw mod manifest: %v", err)
	}
	sourceConfigFile := filepath.Join(sourceGameData, "Users", "PlayerOne", "games", "com.mojang", "config", "settings.json")
	if err := os.MkdirAll(filepath.Dir(sourceConfigFile), 0o755); err != nil {
		t.Fatalf("mkdir source config: %v", err)
	}
	if err := os.WriteFile(sourceConfigFile, []byte(`{"source":true}`), 0o644); err != nil {
		t.Fatalf("write source config: %v", err)
	}
	writeWorld(t, filepath.Join(sourceGameData, "Users", "PlayerOne", "games", "com.mojang", "minecraftWorlds"), "alpha", "Backup World")

	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData", "mods"},
		ScopeModes: map[string]string{"gameData": "safe"},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	targetGameData := createTestGameDataDir(t, targetDir, appDataDir, false, true)
	targetPackDir := filepath.Join(targetGameData, "Users", "PlayerOne", "games", "com.mojang", "resource_packs", "packA")
	if err := os.MkdirAll(targetPackDir, 0o755); err != nil {
		t.Fatalf("mkdir target resource pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetPackDir, "manifest.json"), []byte(`{"format_version":2,"header":{"name":"PackA Legacy","uuid":"pack-a-uuid","version":[1,0,0],"min_engine_version":[1,20,80]},"modules":[{"type":"resources","uuid":"pack-a-module-target","version":[1,0,0]}]}`), 0o644); err != nil {
		t.Fatalf("write target resource pack manifest: %v", err)
	}
	targetConfigFile := filepath.Join(targetGameData, "Users", "PlayerOne", "games", "com.mojang", "config", "settings.json")
	if err := os.MkdirAll(filepath.Dir(targetConfigFile), 0o755); err != nil {
		t.Fatalf("mkdir target config: %v", err)
	}
	if err := os.WriteFile(targetConfigFile, []byte(`{"target":true}`), 0o644); err != nil {
		t.Fatalf("write target config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir target raw mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "mods", "raw_mod", "manifest.json"), []byte(`{"name":"Raw Mod","entry":"main.cpp","version":"0.9.0","type":"mod"}`), 0o644); err != nil {
		t.Fatalf("write target raw mod manifest: %v", err)
	}
	writeWorld(t, filepath.Join(targetGameData, "Users", "PlayerOne", "games", "com.mojang", "minecraftWorlds"), "alpha", "Current World")

	preview := PreviewInstanceBackupRestoreConflicts("Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"gameData", "mods"},
	})
	if preview.ErrorCode != "" {
		t.Fatalf("preview failed: %+v", preview)
	}
	modConflict := findConflict(preview.Conflicts, instanceBackupScopeMods, "mod_folder", "raw_mod")
	if modConflict == nil {
		t.Fatalf("expected mods conflict in preview: %+v", preview.Conflicts)
	}
	if !containsConflictDiffKey(modConflict.DiffFields, "version") {
		t.Fatalf("expected mod version diff, got %+v", modConflict)
	}
	packConflict := findConflict(preview.Conflicts, instanceBackupScopeGameData, "pack_uuid", "pack-a-uuid")
	if packConflict == nil {
		t.Fatalf("expected game data conflict in preview: %+v", preview.Conflicts)
	}
	if packConflict.BackupPath != "Users/PlayerOne/games/com.mojang/resource_packs/packA" {
		t.Fatalf("unexpected pack backup path: %+v", packConflict)
	}
	if !containsConflictDiffKey(packConflict.DiffFields, "version") {
		t.Fatalf("expected pack version diff, got %+v", packConflict)
	}
	fileConflict := findConflict(preview.Conflicts, instanceBackupScopeGameData, "file_path", "Users/PlayerOne/games/com.mojang/config/settings.json")
	if fileConflict == nil {
		t.Fatalf("expected file-path config conflict in preview: %+v", preview.Conflicts)
	}
	worldConflict := findConflict(preview.Conflicts, instanceBackupScopeGameData, "world_folder", "alpha")
	if worldConflict == nil {
		t.Fatalf("expected world-folder conflict in preview: %+v", preview.Conflicts)
	}
	if !containsConflictDiffKey(worldConflict.DiffFields, "level_name") {
		t.Fatalf("expected world name diff, got %+v", worldConflict)
	}
}

func TestPreviewAndRestoreInstanceBackupPackConflictUsesUUIDAcrossFolderNames(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	sourceGameData := createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	sourcePackDir := filepath.Join(sourceGameData, "Users", "PlayerOne", "games", "com.mojang", "resource_packs", "new-pack")
	if err := os.MkdirAll(sourcePackDir, 0o755); err != nil {
		t.Fatalf("mkdir source pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePackDir, "manifest.json"), []byte(`{"format_version":2,"header":{"name":"Fancy Pack","uuid":"shared-pack-uuid","version":[3,0,0],"min_engine_version":[1,21,80]},"modules":[{"type":"resources","uuid":"fancy-pack-module","version":[3,0,0]}]}`), 0o644); err != nil {
		t.Fatalf("write source pack manifest: %v", err)
	}

	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData"},
		ScopeModes: map[string]string{"gameData": "safe"},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	targetGameData := createTestGameDataDir(t, targetDir, appDataDir, false, true)
	targetPackDir := filepath.Join(targetGameData, "Users", "PlayerOne", "games", "com.mojang", "resource_packs", "old-pack")
	if err := os.MkdirAll(targetPackDir, 0o755); err != nil {
		t.Fatalf("mkdir target pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetPackDir, "manifest.json"), []byte(`{"format_version":2,"header":{"name":"Fancy Pack Legacy","uuid":"shared-pack-uuid","version":[1,5,0],"min_engine_version":[1,20,30]},"modules":[{"type":"resources","uuid":"fancy-pack-module-old","version":[1,5,0]}]}`), 0o644); err != nil {
		t.Fatalf("write target pack manifest: %v", err)
	}

	preview := PreviewInstanceBackupRestoreConflicts("Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"gameData"},
	})
	if preview.ErrorCode != "" {
		t.Fatalf("preview failed: %+v", preview)
	}
	conflict := findConflict(preview.Conflicts, instanceBackupScopeGameData, "pack_uuid", "shared-pack-uuid")
	if conflict == nil {
		t.Fatalf("expected uuid-based pack conflict, got %+v", preview.Conflicts)
	}
	if conflict.BackupPath != "Users/PlayerOne/games/com.mojang/resource_packs/new-pack" || conflict.CurrentPath != "Users/PlayerOne/games/com.mojang/resource_packs/old-pack" {
		t.Fatalf("unexpected pack paths: %+v", conflict)
	}
	if !containsConflictDiffKey(conflict.DiffFields, "version") {
		t.Fatalf("expected pack version diff: %+v", conflict)
	}

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"gameData"},
		ConflictResolutions: []types.InstanceBackupRestoreResolution{
			{
				ConflictID: conflict.ID,
				Choice:     instanceBackupRestoreChoiceBackup,
			},
		},
	})
	if result.Status != "success" {
		t.Fatalf("unexpected restore result: %+v", result)
	}
	if utils.DirExists(targetPackDir) {
		t.Fatalf("expected old pack directory removed after backup choice")
	}
	restoredManifest, err := os.ReadFile(filepath.Join(sourcePackDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read source pack manifest: %v", err)
	}
	targetManifest, err := os.ReadFile(filepath.Join(targetGameData, "Users", "PlayerOne", "games", "com.mojang", "resource_packs", "new-pack", "manifest.json"))
	if err != nil {
		t.Fatalf("read restored pack manifest: %v", err)
	}
	if string(targetManifest) != string(restoredManifest) {
		t.Fatalf("expected restored pack manifest to match backup version")
	}
}

func TestRestoreInstanceBackupModsUsesConflictChoiceCurrent(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir source raw mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "mods", "raw_mod", "manifest.json"), []byte(`{"name":"Raw Mod","entry":"main.cpp","version":"2.0.0","type":"mod"}`), 0o644); err != nil {
		t.Fatalf("write source raw mod manifest: %v", err)
	}

	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes: []string{"mods"},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, targetDir, appDataDir, false, true)
	if err := os.MkdirAll(filepath.Join(targetDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir target raw mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "mods", "raw_mod", "manifest.json"), []byte(`{"name":"Raw Mod","entry":"main.cpp","version":"1.0.0","type":"mod"}`), 0o644); err != nil {
		t.Fatalf("write target raw mod manifest: %v", err)
	}

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"mods"},
		ConflictResolutions: []types.InstanceBackupRestoreResolution{
			{
				ConflictID: buildInstanceBackupRestoreConflictEntityID(instanceBackupScopeMods, "mod_folder", "raw_mod", "raw_mod"),
				Choice:     instanceBackupRestoreChoiceCurrent,
			},
		},
	})
	if result.Status != "success" {
		t.Fatalf("unexpected restore result: %+v", result)
	}
	manifestData, err := os.ReadFile(filepath.Join(targetDir, "mods", "raw_mod", "manifest.json"))
	if err != nil {
		t.Fatalf("read restored target manifest: %v", err)
	}
	if !strings.Contains(string(manifestData), `"version":"1.0.0"`) {
		t.Fatalf("expected current target manifest kept, got %s", manifestData)
	}
}

func TestRestoreInstanceBackupGameDataUsesConflictChoiceBackup(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	sourceGameData := createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	sourceFile := filepath.Join(sourceGameData, "Users", "PlayerOne", "games", "com.mojang", "config", "settings.json")
	if err := os.MkdirAll(filepath.Dir(sourceFile), 0o755); err != nil {
		t.Fatalf("mkdir source config: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte(`{"from":"backup"}`), 0o644); err != nil {
		t.Fatalf("write source config: %v", err)
	}

	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData"},
		ScopeModes: map[string]string{"gameData": "safe"},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	targetGameData := createTestGameDataDir(t, targetDir, appDataDir, false, true)
	targetFile := filepath.Join(targetGameData, "Users", "PlayerOne", "games", "com.mojang", "config", "settings.json")
	if err := os.MkdirAll(filepath.Dir(targetFile), 0o755); err != nil {
		t.Fatalf("mkdir target config: %v", err)
	}
	if err := os.WriteFile(targetFile, []byte(`{"from":"current"}`), 0o644); err != nil {
		t.Fatalf("write target config: %v", err)
	}

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"gameData"},
		ConflictResolutions: []types.InstanceBackupRestoreResolution{
			{
				ConflictID: buildInstanceBackupRestoreConflictID(instanceBackupScopeGameData, "Users/PlayerOne/games/com.mojang/config/settings.json"),
				Choice:     instanceBackupRestoreChoiceBackup,
			},
		},
	})
	if result.Status != "success" {
		t.Fatalf("unexpected restore result: %+v", result)
	}
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("read restored target file: %v", err)
	}
	if string(content) != `{"from":"backup"}` {
		t.Fatalf("expected backup version restored, got %s", content)
	}
}

func TestRestoreInstanceBackupFullRejectsNonEmptyTarget(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	sourceGameData := createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	if err := os.WriteFile(filepath.Join(sourceGameData, "root.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write source data: %v", err)
	}
	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData"},
		ScopeModes: map[string]string{"gameData": "full"},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	targetGameData := createTestGameDataDir(t, targetDir, appDataDir, false, true)
	if err := os.WriteFile(filepath.Join(targetGameData, "existing.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing data: %v", err)
	}

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"gameData"},
	})
	if result.Status != "failed" {
		t.Fatalf("expected failed result, got %+v", result)
	}
	if result.ErrorCode != "ERR_INSTANCE_BACKUP_RESTORE_FULL_REQUIRES_EMPTY_TARGET" {
		t.Fatalf("unexpected error code: %+v", result)
	}
}

func TestRestoreInstanceBackupModsPartialWhenLipRuntimeMissing(t *testing.T) {
	restoreLipInstalled := lipIsInstalled
	restoreFetchLLDB := instanceBackupFetchLeviLaminaVersionDB
	lipIsInstalled = func() bool { return false }
	instanceBackupFetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{
			"1.21.80.03": {"0.13.0"},
		}, nil
	}
	t.Cleanup(func() {
		lipIsInstalled = restoreLipInstalled
		instanceBackupFetchLeviLaminaVersionDB = restoreFetchLLDB
	})

	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80.03",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir raw mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "mods", "raw_mod", "manifest.json"), []byte(`{"name":"raw","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("write raw manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "lip_mod"), 0o755); err != nil {
		t.Fatalf("mkdir lip mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "mods", "lip_mod", "manifest.json"), []byte(`{"name":"lip","version":"2.0.0"}`), 0o644); err != nil {
		t.Fatalf("write lip manifest: %v", err)
	}
	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes: []string{"mods"},
		ModsLIPPackages: []types.InstanceBackupModsLIPPackage{
			{
				Identifier:        "LiteLDev/Foo",
				Version:           "2.0.0",
				ExplicitInstalled: true,
				Folders:           []string{"lip_mod"},
			},
		},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80.03",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, targetDir, appDataDir, false, true)

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"mods"},
	})
	if result.Status != "partial" {
		t.Fatalf("expected partial result, got %+v", result)
	}
	scopeResult := result.ScopeResults[0]
	if !containsString(scopeResult.Warnings, "ERR_LIP_NOT_INSTALLED") {
		t.Fatalf("expected lip runtime warning, got %+v", scopeResult)
	}
	if !utils.DirExists(filepath.Join(targetDir, "mods", "raw_mod")) {
		t.Fatalf("expected raw mod restored")
	}
	if utils.DirExists(filepath.Join(targetDir, "mods", "lip_mod")) {
		t.Fatalf("lip managed folder should not be restored as raw")
	}
}

func TestRestoreInstanceBackupModsSkipsLipWhenLLUnsupported(t *testing.T) {
	restoreLipInstalled := lipIsInstalled
	restoreFetchLLDB := instanceBackupFetchLeviLaminaVersionDB
	lipChecked := false
	lipIsInstalled = func() bool {
		lipChecked = true
		return true
	}
	instanceBackupFetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{
			"1.21.70.03": {"0.13.0"},
		}, nil
	}
	t.Cleanup(func() {
		lipIsInstalled = restoreLipInstalled
		instanceBackupFetchLeviLaminaVersionDB = restoreFetchLLDB
	})

	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80.03",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir raw mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "mods", "raw_mod", "manifest.json"), []byte(`{"name":"raw","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("write raw manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "lip_mod"), 0o755); err != nil {
		t.Fatalf("mkdir lip mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "mods", "lip_mod", "manifest.json"), []byte(`{"name":"lip","version":"2.0.0"}`), 0o644); err != nil {
		t.Fatalf("write lip manifest: %v", err)
	}

	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes: []string{"mods"},
		ModsLIPPackages: []types.InstanceBackupModsLIPPackage{
			{
				Identifier:        "LiteLDev/Foo",
				Version:           "2.0.0",
				ExplicitInstalled: true,
				Folders:           []string{"lip_mod"},
			},
		},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80.03",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, targetDir, appDataDir, false, true)

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"mods"},
	})
	if result.Status != "partial" {
		t.Fatalf("expected partial result, got %+v", result)
	}
	if lipChecked {
		t.Fatalf("lip runtime should not be checked when LeviLamina is unsupported")
	}
	scopeResult := result.ScopeResults[0]
	if !containsString(scopeResult.Warnings, "ERR_LL_NOT_SUPPORTED") {
		t.Fatalf("expected LL unsupported warning, got %+v", scopeResult)
	}
	if !utils.DirExists(filepath.Join(targetDir, "mods", "raw_mod")) {
		t.Fatalf("expected raw mod restored")
	}
	if utils.DirExists(filepath.Join(targetDir, "mods", "lip_mod")) {
		t.Fatalf("lip managed folder should not be restored as raw")
	}
}

func TestRestoreInstanceBackupModsTreatsLegacyThreePartVersionAsUnsupported(t *testing.T) {
	restoreLipInstalled := lipIsInstalled
	restoreFetchLLDB := instanceBackupFetchLeviLaminaVersionDB
	lipChecked := false
	lipIsInstalled = func() bool {
		lipChecked = true
		return true
	}
	instanceBackupFetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{
			"1.21.80.03": {"0.13.0"},
		}, nil
	}
	t.Cleanup(func() {
		lipIsInstalled = restoreLipInstalled
		instanceBackupFetchLeviLaminaVersionDB = restoreFetchLLDB
	})

	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80.03",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "raw_mod"), 0o755); err != nil {
		t.Fatalf("mkdir raw mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "mods", "raw_mod", "manifest.json"), []byte(`{"name":"raw","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("write raw manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "mods", "lip_mod"), 0o755); err != nil {
		t.Fatalf("mkdir lip mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "mods", "lip_mod", "manifest.json"), []byte(`{"name":"lip","version":"2.0.0"}`), 0o644); err != nil {
		t.Fatalf("write lip manifest: %v", err)
	}

	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes: []string{"mods"},
		ModsLIPPackages: []types.InstanceBackupModsLIPPackage{
			{
				Identifier:        "LiteLDev/Foo",
				Version:           "2.0.0",
				ExplicitInstalled: true,
				Folders:           []string{"lip_mod"},
			},
		},
	})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, targetDir, appDataDir, false, true)

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"mods"},
	})
	if result.Status != "partial" {
		t.Fatalf("expected partial result, got %+v", result)
	}
	if lipChecked {
		t.Fatalf("lip runtime should not be checked for legacy three-part versions")
	}
	scopeResult := result.ScopeResults[0]
	if !containsString(scopeResult.Warnings, "ERR_LL_NOT_SUPPORTED") {
		t.Fatalf("expected LL unsupported warning, got %+v", scopeResult)
	}
	if !utils.DirExists(filepath.Join(targetDir, "mods", "raw_mod")) {
		t.Fatalf("expected raw mod restored")
	}
	if utils.DirExists(filepath.Join(targetDir, "mods", "lip_mod")) {
		t.Fatalf("lip managed folder should not be restored as raw")
	}
}

func setupInstanceBackupEnv(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	baseRoot := filepath.Join(root, "base")
	appDataDir := filepath.Join(root, "appdata")
	apppath.SetBaseRootOverride(baseRoot)
	t.Setenv("APPDATA", appDataDir)
	t.Cleanup(func() {
		apppath.SetBaseRootOverride("")
	})

	versionsDir, err := apppath.VersionsDir()
	if err != nil {
		t.Fatalf("resolve versions dir: %v", err)
	}
	return baseRoot, appDataDir, versionsDir
}

func createTestInstance(t *testing.T, versionsDir string, name string, meta versions.VersionMeta) string {
	t.Helper()
	versionDir := filepath.Join(versionsDir, name)
	meta.Name = name
	if strings.TrimSpace(meta.Type) == "" {
		meta.Type = "release"
	}
	if strings.TrimSpace(meta.GameVersion) == "" {
		meta.GameVersion = "1.21.80"
	}
	if err := versions.WriteMeta(versionDir, meta); err != nil {
		t.Fatalf("write meta for %s: %v", name, err)
	}
	return versionDir
}

func createTestGameDataDir(t *testing.T, versionDir string, appDataDir string, isPreview bool, enableIsolation bool) string {
	t.Helper()
	var gameDataDir string
	if enableIsolation {
		gameDataDir = filepath.Join(versionDir, instanceBackupGameDataLabel(isPreview))
	} else {
		gameDataDir = filepath.Join(appDataDir, instanceBackupGameDataLabel(isPreview))
	}
	if err := os.MkdirAll(gameDataDir, 0o755); err != nil {
		t.Fatalf("mkdir game data dir: %v", err)
	}
	return gameDataDir
}

func writeWorld(t *testing.T, worldsRoot string, folder string, levelName string) {
	t.Helper()
	worldDir := filepath.Join(worldsRoot, folder)
	if err := os.MkdirAll(worldDir, 0o755); err != nil {
		t.Fatalf("mkdir world dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worldDir, "levelname.txt"), []byte(levelName), 0o644); err != nil {
		t.Fatalf("write levelname: %v", err)
	}
}

func readZipEntryNames(t *testing.T, data []byte) map[string]struct{} {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip reader: %v", err)
	}
	names := make(map[string]struct{}, len(reader.File))
	for _, file := range reader.File {
		names[file.Name] = struct{}{}
	}
	return names
}

func readZipFile(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip reader: %v", err)
	}
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip file %s: %v", name, err)
		}
		defer rc.Close()
		content, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read zip file %s: %v", name, err)
		}
		return content
	}
	t.Fatalf("zip file %q not found", name)
	return nil
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsConflictPath(conflicts []types.InstanceBackupRestoreConflict, scopeKey string, conflictPath string) bool {
	for _, conflict := range conflicts {
		if conflict.ScopeKey == scopeKey && conflict.Path == conflictPath {
			return true
		}
	}
	return false
}

func findConflict(conflicts []types.InstanceBackupRestoreConflict, scopeKey string, identityKind string, identityKey string) *types.InstanceBackupRestoreConflict {
	for i := range conflicts {
		conflict := &conflicts[i]
		if conflict.ScopeKey == scopeKey &&
			conflict.IdentityKind == identityKind &&
			conflict.IdentityKey == identityKey {
			return conflict
		}
	}
	return nil
}

func containsConflictDiffKey(fields []types.InstanceBackupRestoreConflictDiffField, key string) bool {
	for _, field := range fields {
		if field.Key == key {
			return true
		}
	}
	return false
}

func createLegacySourcePathsArchive(t *testing.T, archivePath string, sourcePaths map[string]string) string {
	t.Helper()

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()

	legacyArchivePath := filepath.Join(t.TempDir(), "legacy-source-paths.zip")
	file, err := os.Create(legacyArchivePath)
	if err != nil {
		t.Fatalf("create legacy archive: %v", err)
	}
	zw := zip.NewWriter(file)

	for _, entry := range reader.File {
		header := &zip.FileHeader{
			Name:   entry.Name,
			Method: entry.Method,
		}
		header.SetMode(entry.Mode())
		header.SetModTime(entry.Modified)

		writer, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip header for %s: %v", entry.Name, err)
		}
		if entry.FileInfo().IsDir() {
			continue
		}

		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", entry.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", entry.Name, err)
		}

		if entry.Name == instanceBackupManifestName {
			var manifest map[string]any
			if err := json.Unmarshal(content, &manifest); err != nil {
				t.Fatalf("unmarshal manifest: %v", err)
			}
			manifest["sourcePaths"] = sourcePaths
			content, err = json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				t.Fatalf("marshal legacy manifest: %v", err)
			}
		}

		if _, err := writer.Write(content); err != nil {
			t.Fatalf("write zip entry %s: %v", entry.Name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close legacy archive writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close legacy archive file: %v", err)
	}

	return legacyArchivePath
}
