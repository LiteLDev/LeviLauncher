package mcservice

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

func recordInstanceBackupRestoreProgress(t *testing.T) *[]types.InstanceBackupRestoreProgress {
	t.Helper()
	events := &[]types.InstanceBackupRestoreProgress{}
	previous := emitInstanceBackupRestoreProgressEvent
	emitInstanceBackupRestoreProgressEvent = func(payload types.InstanceBackupRestoreProgress) {
		*events = append(*events, payload)
	}
	t.Cleanup(func() {
		emitInstanceBackupRestoreProgressEvent = previous
	})
	return events
}

func recordInstanceBackupCreateProgress(t *testing.T) *[]types.InstanceBackupCreateProgress {
	t.Helper()
	events := &[]types.InstanceBackupCreateProgress{}
	previous := emitInstanceBackupCreateProgressEvent
	emitInstanceBackupCreateProgressEvent = func(payload types.InstanceBackupCreateProgress) {
		*events = append(*events, payload)
	}
	t.Cleanup(func() {
		emitInstanceBackupCreateProgressEvent = previous
	})
	return events
}

func writeProgressTestFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	data := make([]byte, size)
	seed := uint32(len(path)) + 1
	for i := range data {
		seed = seed*1664525 + 1013904223
		data[i] = byte(seed >> 24)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// instanceBackupArchiveCounts measures a written backup archive: the regular
// entries under prefix (all entries when prefix is empty) and their sizes.
func instanceBackupArchiveCounts(t *testing.T, archivePath string, prefix string, skipManifest bool) (int, int64) {
	t.Helper()
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()
	files := 0
	var size int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
			continue
		}
		if skipManifest && file.Name == instanceBackupManifestName {
			continue
		}
		if !strings.HasPrefix(file.Name, prefix) {
			continue
		}
		files++
		size += int64(file.UncompressedSize64)
	}
	return files, size
}

// createProgressTestSource builds a "Source" instance with safe gameData
// content (one multi-MiB world file plus small packs) and two raw mods.
func createProgressTestSource(t *testing.T, versionsDir string, appDataDir string) string {
	t.Helper()
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	gameDataDir := createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	mojang := filepath.Join(gameDataDir, "Users", "PlayerOne", "games", "com.mojang")
	writeProgressTestFile(t, filepath.Join(mojang, "minecraftWorlds", "Alpha", "db", "big.ldb"), 3<<20+123)
	writeProgressTestFile(t, filepath.Join(mojang, "minecraftWorlds", "Alpha", "levelname.txt"), 10)
	for i := 0; i < 5; i++ {
		writeProgressTestFile(t, filepath.Join(mojang, "resource_packs", "pack", "file"+string(rune('a'+i))+".txt"), 100+i)
	}
	writeProgressTestFile(t, filepath.Join(gameDataDir, "Users", "PlayerOne", "private", "secret.txt"), 50)
	writeProgressTestFile(t, filepath.Join(sourceDir, "mods", "raw_a", "manifest.json"), 40)
	writeProgressTestFile(t, filepath.Join(sourceDir, "mods", "raw_a", "lib", "a.dll"), 70000)
	writeProgressTestFile(t, filepath.Join(sourceDir, "mods", "raw_b", "manifest.json"), 30)
	writeProgressTestFile(t, filepath.Join(sourceDir, "mods", "lip_a", "manifest.json"), 30)
	writeProgressTestFile(t, filepath.Join(sourceDir, "mods", "lip_b", "manifest.json"), 30)
	writeProgressTestFile(t, filepath.Join(sourceDir, "mods", "lip_dep", "manifest.json"), 30)
	return sourceDir
}

func progressTestLIPPackages() []types.InstanceBackupModsLIPPackage {
	return []types.InstanceBackupModsLIPPackage{
		{Identifier: "LiteLDev/Bravo", Version: "1.0.0", ExplicitInstalled: true, Folders: []string{"lip_b"}},
		{Identifier: "LiteLDev/Alpha", Version: "2.0.0", ExplicitInstalled: true, Folders: []string{"lip_a"}},
		{Identifier: "LiteLDev/Dep", Version: "3.0.0", ExplicitInstalled: false, Folders: []string{"lip_dep"}},
	}
}

func assertInstanceBackupCreateProgress(t *testing.T, events []types.InstanceBackupCreateProgress, wantFiles int, wantBytes int64) {
	t.Helper()
	if len(events) < 3 {
		t.Fatalf("too few create progress events: %+v", events)
	}
	if events[0].Phase != "scanning" {
		t.Fatalf("first event should be scanning, got %+v", events[0])
	}
	last := events[len(events)-1]
	if last.Phase != "done" {
		t.Fatalf("last event should be done, got %+v", last)
	}
	if last.Files != wantFiles || last.TotalFiles != wantFiles || last.Bytes != wantBytes || last.TotalBytes != wantBytes {
		t.Fatalf("done event = %+v, want %d files / %d bytes", last, wantFiles, wantBytes)
	}
	doneCount := 0
	prevFiles, prevBytes := 0, int64(0)
	for i, event := range events[1:] {
		switch event.Phase {
		case "done":
			doneCount++
		case "writing":
			if event.TotalFiles != wantFiles || event.TotalBytes != wantBytes {
				t.Fatalf("writing event %d totals = %d/%d, want %d/%d (pre-scan must match the archive)", i, event.TotalFiles, event.TotalBytes, wantFiles, wantBytes)
			}
		default:
			t.Fatalf("unexpected phase after scanning: %+v", event)
		}
		if event.Files < prevFiles || event.Bytes < prevBytes {
			t.Fatalf("create progress went backwards at %d: %+v after %d/%d", i, event, prevFiles, prevBytes)
		}
		if event.Files > event.TotalFiles || event.Bytes > event.TotalBytes {
			t.Fatalf("create progress beyond total: %+v", event)
		}
		prevFiles, prevBytes = event.Files, event.Bytes
	}
	if doneCount != 1 {
		t.Fatalf("want exactly one done event, got %d", doneCount)
	}
	first := events[1]
	if first.Phase != "writing" || first.Files != 0 || first.Bytes != 0 {
		t.Fatalf("first writing event should start at zero: %+v", first)
	}
	// Throttled: at most one update per 1% of files plus one per byte step,
	// plus the opening and closing events.
	if len(events) > 2*100+4 {
		t.Fatalf("create progress not throttled: %d events", len(events))
	}
}

func TestBackupInstanceEmitsMeasuredCreateProgress(t *testing.T) {
	for _, mode := range []string{instanceBackupModeSafe, instanceBackupModeFull} {
		t.Run(mode, func(t *testing.T) {
			_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
			createProgressTestSource(t, versionsDir, appDataDir)
			events := recordInstanceBackupCreateProgress(t)

			result := BackupInstance("Source", types.InstanceBackupRequest{
				Scopes:          []string{"gameData", "mods"},
				ScopeModes:      map[string]string{"gameData": mode},
				ModsLIPPackages: progressTestLIPPackages(),
			})
			if result.ErrorCode != "" {
				t.Fatalf("backup failed: %q", result.ErrorCode)
			}
			wantFiles, wantBytes := instanceBackupArchiveCounts(t, result.ArchivePath, "", true)
			if wantBytes < 3<<20 {
				t.Fatalf("archive unexpectedly small: %d bytes", wantBytes)
			}
			assertInstanceBackupCreateProgress(t, *events, wantFiles, wantBytes)

			// The big world file spans many copy chunks, so byte progress must
			// arrive in between file boundaries.
			midFile := 0
			for _, event := range *events {
				if event.Phase == "writing" && strings.HasSuffix(event.CurrentFile, "big.ldb") {
					midFile++
				}
			}
			if midFile < 2 {
				t.Fatalf("expected byte-level updates while writing big.ldb, got %d", midFile)
			}
		})
	}
}

func TestBackupInstanceEmitsNoCreateProgressAfterValidationFailure(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	createProgressTestSource(t, versionsDir, appDataDir)
	events := recordInstanceBackupCreateProgress(t)

	result := BackupInstance("Source", types.InstanceBackupRequest{Scopes: []string{"bogus"}})
	if result.ErrorCode == "" {
		t.Fatalf("expected an error")
	}
	if len(*events) != 0 {
		t.Fatalf("validation failures must not emit progress: %+v", *events)
	}
}

func filterInstanceBackupSubProgress(
	events []types.InstanceBackupRestoreProgress,
	phase string,
	scopeKey string,
	subID string,
	unit string,
) []types.InstanceBackupRestoreProgress {
	var out []types.InstanceBackupRestoreProgress
	for _, event := range events {
		if event.SubTotal > 0 && event.Phase == phase && event.ScopeKey == scopeKey && event.SubID == subID && event.SubUnit == unit {
			out = append(out, event)
		}
	}
	return out
}

func assertInstanceBackupSubProgress(t *testing.T, name string, series []types.InstanceBackupRestoreProgress, wantTotal int64) {
	t.Helper()
	if len(series) == 0 {
		t.Fatalf("%s: no sub-progress events", name)
	}
	if series[0].SubCurrent != 0 {
		t.Fatalf("%s: first event should start at 0, got %+v", name, series[0])
	}
	atTotal := 0
	prev := int64(0)
	for i, event := range series {
		if event.SubTotal != wantTotal {
			t.Fatalf("%s: event %d total = %d, want %d", name, i, event.SubTotal, wantTotal)
		}
		if event.SubCurrent < prev || event.SubCurrent > event.SubTotal {
			t.Fatalf("%s: event %d current %d not monotonic within total (prev %d): %+v", name, i, event.SubCurrent, prev, event)
		}
		if event.SubCurrent == event.SubTotal {
			atTotal++
		}
		prev = event.SubCurrent
	}
	if last := series[len(series)-1]; last.SubCurrent != wantTotal {
		t.Fatalf("%s: final value %d, want %d", name, last.SubCurrent, wantTotal)
	}
	if atTotal != 1 {
		t.Fatalf("%s: want exactly one final event, got %d", name, atTotal)
	}
}

func stubInstanceBackupLIP(t *testing.T, failing string) *[]string {
	t.Helper()
	installed := &[]string{}
	previousInstall := instanceBackupInstallLIPPackage
	previousState := instanceBackupGetLIPPackageInstallState
	previousFetch := instanceBackupFetchLeviLaminaVersionDB
	versionByID := map[string]string{}
	instanceBackupInstallLIPPackage = func(ctx context.Context, targetName string, identifier string, version string) string {
		*installed = append(*installed, identifier)
		if identifier == failing {
			return "ERR_FAKE_INSTALL"
		}
		versionByID[identifier] = version
		return ""
	}
	instanceBackupGetLIPPackageInstallState = func(ctx context.Context, targetName string, identifier string) types.LIPPackageInstallState {
		version, ok := versionByID[identifier]
		return types.LIPPackageInstallState{Identifier: identifier, Installed: ok, InstalledVersion: version}
	}
	instanceBackupFetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{"1.21.80": {"1.0.0"}}, nil
	}
	t.Cleanup(func() {
		instanceBackupInstallLIPPackage = previousInstall
		instanceBackupGetLIPPackageInstallState = previousState
		instanceBackupFetchLeviLaminaVersionDB = previousFetch
	})
	return installed
}

func TestRestoreInstanceBackupEmitsMeasuredSubProgress(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	createProgressTestSource(t, versionsDir, appDataDir)
	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:          []string{"gameData", "mods"},
		ScopeModes:      map[string]string{"gameData": instanceBackupModeSafe},
		ModsLIPPackages: progressTestLIPPackages(),
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
	installed := stubInstanceBackupLIP(t, "LiteLDev/Bravo")
	events := recordInstanceBackupRestoreProgress(t)

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"gameData", "mods"},
	})
	if result.Status != "partial" {
		t.Fatalf("expected partial result (one lip install fails), got %+v", result)
	}

	// Step semantics are unchanged: the step events carry no sub-progress.
	var steps []string
	for _, event := range *events {
		if event.SubTotal == 0 {
			steps = append(steps, event.Phase+":"+event.ScopeKey)
			if event.TotalSteps != 5 {
				t.Fatalf("unexpected total steps: %+v", event)
			}
		}
	}
	wantSteps := []string{"preparing:", "extracting:", "restoring_scope:gameData", "restoring_scope:mods", "finalizing:"}
	if strings.Join(steps, ",") != strings.Join(wantSteps, ",") {
		t.Fatalf("step events = %v, want %v", steps, wantSteps)
	}

	_, archiveBytes := instanceBackupArchiveCounts(t, backup.ArchivePath, "", false)
	extract := filterInstanceBackupSubProgress(*events, "extracting", "", "extract", instanceBackupProgressUnitBytes)
	assertInstanceBackupSubProgress(t, "extract", extract, archiveBytes)
	if len(extract) < 3 || len(extract) > 100+3 {
		t.Fatalf("extract progress should be throttled to ~1%% steps, got %d events", len(extract))
	}
	for _, event := range extract {
		if event.CurrentStep != 2 {
			t.Fatalf("extract sub-progress must stay on step 2: %+v", event)
		}
	}

	gameDataFiles, _ := instanceBackupArchiveCounts(t, backup.ArchivePath, "gameData/", false)
	gameData := filterInstanceBackupSubProgress(*events, "restoring_scope", "gameData", "gameData:files", instanceBackupProgressUnitFiles)
	assertInstanceBackupSubProgress(t, "gameData files", gameData, int64(gameDataFiles))

	modFiles, _ := instanceBackupArchiveCounts(t, backup.ArchivePath, "mods/raw/", false)
	if modFiles != 3 {
		t.Fatalf("expected 3 raw mod files in archive, got %d", modFiles)
	}
	mods := filterInstanceBackupSubProgress(*events, "restoring_scope", "mods", "mods:files", instanceBackupProgressUnitFiles)
	assertInstanceBackupSubProgress(t, "mods files", mods, int64(modFiles))

	packages := filterInstanceBackupSubProgress(*events, "restoring_scope", "mods", "mods:packages", instanceBackupProgressUnitPackages)
	assertInstanceBackupSubProgress(t, "lip packages", packages, 2)
	var labels []string
	for _, event := range packages {
		labels = append(labels, event.SubLabel)
	}
	wantLabels := "LiteLDev/Alpha,LiteLDev/Alpha,LiteLDev/Bravo,LiteLDev/Bravo"
	if strings.Join(labels, ",") != wantLabels {
		t.Fatalf("package labels = %v, want %s", labels, wantLabels)
	}
	if strings.Join(*installed, ",") != "LiteLDev/Alpha,LiteLDev/Bravo" {
		t.Fatalf("unexpected installs: %v", *installed)
	}
	for _, event := range packages {
		if event.CurrentStep != 4 {
			t.Fatalf("package sub-progress must stay on the mods step: %+v", event)
		}
	}

	// The install-state check runs over every recorded package (explicit and
	// dependency) as its own bar, after the install bar has closed.
	verify := filterInstanceBackupSubProgress(*events, "restoring_scope", "mods", "mods:verify", instanceBackupProgressUnitPackages)
	assertInstanceBackupSubProgress(t, "lip verify", verify, 3)

	// Every sub-progress event carries a bar id, and within the mods step the
	// bars run one after another: files, packages, verify, never interleaved.
	var modsBars []string
	for _, event := range *events {
		if event.SubTotal == 0 {
			continue
		}
		if event.SubID == "" {
			t.Fatalf("sub-progress event without SubID: %+v", event)
		}
		if event.ScopeKey != "mods" {
			continue
		}
		if len(modsBars) == 0 || modsBars[len(modsBars)-1] != event.SubID {
			modsBars = append(modsBars, event.SubID)
		}
	}
	if got := strings.Join(modsBars, ","); got != "mods:files,mods:packages,mods:verify" {
		t.Fatalf("mods bars = %s, want mods:files,mods:packages,mods:verify", got)
	}
}

func TestPreviewInstanceBackupRestoreConflictsEmitsExtractProgress(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	createProgressTestSource(t, versionsDir, appDataDir)
	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData", "mods"},
		ScopeModes: map[string]string{"gameData": instanceBackupModeSafe},
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

	restoreEvents := recordInstanceBackupRestoreProgress(t)
	var events []types.InstanceBackupRestoreProgress
	previous := emitInstanceBackupPreviewProgressEvent
	emitInstanceBackupPreviewProgressEvent = func(payload types.InstanceBackupRestoreProgress) {
		events = append(events, payload)
	}
	t.Cleanup(func() { emitInstanceBackupPreviewProgressEvent = previous })

	info := PreviewInstanceBackupRestoreConflicts("Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"gameData", "mods"},
	})
	if info.ErrorCode != "" {
		t.Fatalf("preview failed: %q", info.ErrorCode)
	}
	if len(*restoreEvents) != 0 {
		t.Fatalf("preview must not emit restore progress: %+v", *restoreEvents)
	}
	if len(events) == 0 || events[0].SubTotal != 0 || events[0].Phase != "extracting" || events[0].CurrentStep != 1 || events[0].TotalSteps != 1 {
		t.Fatalf("expected an opening extracting step 1/1 event, got %+v", events)
	}
	_, archiveBytes := instanceBackupArchiveCounts(t, backup.ArchivePath, "", false)
	extract := filterInstanceBackupSubProgress(events, "extracting", "", "extract", instanceBackupProgressUnitBytes)
	assertInstanceBackupSubProgress(t, "preview extract", extract, archiveBytes)
	if len(extract)+1 != len(events) {
		t.Fatalf("preview should only emit the step and its byte bar, got %+v", events)
	}
}

// A file reached through a symlink is copied from its target (os.Open follows
// links), so the dry run must count the target size, not the size of the
// link itself, or Bytes would overrun TotalBytes.
func TestInstanceBackupDryRunCountsSymlinkTargetSize(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.bin")
	writeProgressTestFile(t, targetPath, 4096)
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	// Stand-in for the Lstat FileInfo filepath.Walk hands over for a link:
	// symlink mode and size 0, as Windows reports for a file symlink.
	linkInfo := progressTestFileInfo{FileInfo: targetInfo, mode: os.ModeSymlink | 0o777, size: 0}
	if got := instanceBackupFileContentSize(targetPath, linkInfo); got != 4096 {
		t.Fatalf("content size through link = %d, want 4096", got)
	}
	if got := instanceBackupFileContentSize(targetPath, targetInfo); got != 4096 {
		t.Fatalf("content size of regular file = %d, want 4096", got)
	}
	dangling := filepath.Join(dir, "missing.bin")
	if got := instanceBackupFileContentSize(dangling, linkInfo); got != 0 {
		t.Fatalf("dangling link should fall back to the Lstat size, got %d", got)
	}
}

// End to end through filepath.Walk with a real symlink, when the platform
// allows the test to create one: the dry-run totals equal what is written.
func TestInstanceBackupDryRunMatchesWriteThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	external := filepath.Join(dir, "build", "MyMod.dll")
	writeProgressTestFile(t, external, 3*1024*1024+7)
	src := filepath.Join(dir, "mods", "MyMod")
	writeProgressTestFile(t, filepath.Join(src, "manifest.json"), 120)
	if err := os.Symlink(external, filepath.Join(src, "MyMod.dll")); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}

	dry := &instanceBackupZipSink{}
	if err := writeNamedDirToZip(dry, "mods/MyMod", src); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.files != 2 || dry.bytes != 3*1024*1024+7+120 {
		t.Fatalf("dry run counted %d files / %d bytes", dry.files, dry.bytes)
	}

	events := recordInstanceBackupCreateProgress(t)
	out, err := os.Create(filepath.Join(dir, "out.zip"))
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer out.Close()
	sink := &instanceBackupZipSink{zw: zip.NewWriter(out), progress: newInstanceBackupCreateProgress(dry.files, dry.bytes)}
	if err := writeNamedDirToZip(sink, "mods/MyMod", src); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := sink.zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if sink.files != dry.files || sink.bytes != dry.bytes {
		t.Fatalf("written %d/%d, dry run %d/%d", sink.files, sink.bytes, dry.files, dry.bytes)
	}
	if len(*events) == 0 {
		t.Fatalf("expected writing events")
	}
	for i, event := range *events {
		if event.TotalBytes != dry.bytes || event.TotalFiles != dry.files {
			t.Fatalf("event %d totals moved off the scan: %+v", i, event)
		}
		if event.Bytes > event.TotalBytes {
			t.Fatalf("event %d overran total: %+v", i, event)
		}
	}
}

type progressTestFileInfo struct {
	os.FileInfo
	mode os.FileMode
	size int64
}

func (f progressTestFileInfo) Mode() os.FileMode { return f.mode }
func (f progressTestFileInfo) Size() int64       { return f.size }

func TestRestoreInstanceBackupFilesProgressCountsKeptConflicts(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createTestInstance(t, versionsDir, "Source", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, sourceDir, appDataDir, false, true)
	writeProgressTestFile(t, filepath.Join(sourceDir, "mods", "raw_mod", "manifest.json"), 40)
	writeProgressTestFile(t, filepath.Join(sourceDir, "mods", "raw_mod", "a.dll"), 400)
	writeProgressTestFile(t, filepath.Join(sourceDir, "mods", "other_mod", "manifest.json"), 40)
	backup := BackupInstance("Source", types.InstanceBackupRequest{Scopes: []string{"mods"}})
	if backup.ErrorCode != "" {
		t.Fatalf("backup failed: %q", backup.ErrorCode)
	}

	targetDir := createTestInstance(t, versionsDir, "Target", versions.VersionMeta{
		GameVersion:     "1.21.80",
		Type:            "release",
		EnableIsolation: true,
	})
	_ = createTestGameDataDir(t, targetDir, appDataDir, false, true)
	writeProgressTestFile(t, filepath.Join(targetDir, "mods", "raw_mod", "manifest.json"), 41)
	events := recordInstanceBackupRestoreProgress(t)

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"mods"},
		ConflictResolutions: []types.InstanceBackupRestoreResolution{{
			ConflictID: buildInstanceBackupRestoreConflictEntityID(instanceBackupScopeMods, "mod_folder", "raw_mod", "raw_mod"),
			Choice:     instanceBackupRestoreChoiceCurrent,
		}},
	})
	if result.Status != "success" {
		t.Fatalf("unexpected restore result: %+v", result)
	}
	mods := filterInstanceBackupSubProgress(*events, "restoring_scope", "mods", "mods:files", instanceBackupProgressUnitFiles)
	assertInstanceBackupSubProgress(t, "mods files", mods, 3)
	// The kept folder's two files are counted together when it is skipped.
	sawSkip := false
	for i := 1; i < len(mods); i++ {
		if mods[i].SubLabel == "raw_mod" && mods[i].SubCurrent-mods[i-1].SubCurrent == 2 {
			sawSkip = true
		}
	}
	if !sawSkip {
		t.Fatalf("expected the kept raw_mod folder to be counted as handled: %+v", mods)
	}
}

func TestRestoreInstanceBackupFullGameDataFilesProgress(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	createProgressTestSource(t, versionsDir, appDataDir)
	backup := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData"},
		ScopeModes: map[string]string{"gameData": instanceBackupModeFull},
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
	events := recordInstanceBackupRestoreProgress(t)

	result := RestoreInstanceBackup(context.Background(), "Target", types.InstanceBackupRestoreRequest{
		ArchivePath: backup.ArchivePath,
		Scopes:      []string{"gameData"},
	})
	if result.Status != "success" {
		t.Fatalf("unexpected restore result: %+v", result)
	}
	gameDataFiles, _ := instanceBackupArchiveCounts(t, backup.ArchivePath, "gameData/", false)
	gameData := filterInstanceBackupSubProgress(*events, "restoring_scope", "gameData", "gameData:files", instanceBackupProgressUnitFiles)
	assertInstanceBackupSubProgress(t, "full gameData files", gameData, int64(gameDataFiles))
	// Files are counted as they are copied, not in one jump at the end.
	if len(gameData) < 3 {
		t.Fatalf("expected per-file updates, got %+v", gameData)
	}
}

func TestExtractInstanceBackupArchiveStopsProgressOnError(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "bad.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zw := zip.NewWriter(file)
	for _, entry := range []struct{ name, content string }{{"a.txt", "AAAAA"}, {"b.txt", "BBBBB"}} {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: entry.name, Method: zip.Store})
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := w.Write([]byte(entry.content)); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	// Corrupt a.txt's stored bytes so its checksum fails during extraction.
	raw, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	index := strings.Index(string(raw), "AAAAA")
	if index < 0 {
		t.Fatalf("stored content not found")
	}
	raw[index] = 'X'
	if err := os.WriteFile(archivePath, raw, 0o644); err != nil {
		t.Fatalf("rewrite archive: %v", err)
	}

	var values [][2]int64
	err = extractInstanceBackupArchive(archivePath, filepath.Join(dir, "out"), func(total int64) *instanceBackupMeasuredProgress {
		return newInstanceBackupMeasuredProgress(total, 1, func(current int64, total int64, label string) {
			values = append(values, [2]int64{current, total})
		})
	})
	if !errors.Is(err, errInstanceBackupExtract) {
		t.Fatalf("expected extract error, got %v", err)
	}
	if len(values) == 0 || values[0] != [2]int64{0, 10} {
		t.Fatalf("expected an opening 0/10 update, got %v", values)
	}
	// Extraction stops at the failing entry; the bar never claims completion.
	if last := values[len(values)-1]; last[0] >= last[1] {
		t.Fatalf("failed extraction must not reach the total: %v", values)
	}
}

func TestInstanceBackupMeasuredProgressThrottlesAndFinishes(t *testing.T) {
	if p := newInstanceBackupMeasuredProgress(0, 1, func(int64, int64, string) {}); p != nil {
		t.Fatalf("zero total must not open a bar")
	}
	var nilProgress *instanceBackupMeasuredProgress
	nilProgress.add(1, "x")
	nilProgress.announce("x")
	nilProgress.skipEntry(t.TempDir())
	nilProgress.finish(true)

	var got []int64
	p := newInstanceBackupMeasuredProgress(1000, instanceBackupByteProgressThreshold(1000), func(current int64, total int64, label string) {
		if total != 1000 {
			t.Fatalf("total changed: %d", total)
		}
		got = append(got, current)
	})
	p.announce("")
	for i := 0; i < 1000; i++ {
		p.add(1, "f")
	}
	p.finish(false)
	if len(got) != 101 || got[0] != 0 || got[len(got)-1] != 1000 {
		t.Fatalf("expected 0 plus 100 one-percent steps, got %d values ending %v", len(got), got[len(got)-1])
	}

	got = nil
	p = newInstanceBackupMeasuredProgress(10, 4, func(current int64, total int64, label string) {
		got = append(got, current)
	})
	p.add(3, "a")
	p.finish(true)
	p.finish(true)
	if len(got) != 1 || got[0] != 10 {
		t.Fatalf("finish(true) should emit the total exactly once, got %v", got)
	}

	if step := instanceBackupByteProgressThreshold(10 << 30); step != instanceBackupProgressMaxByteStep {
		t.Fatalf("byte threshold should cap at 1 MiB, got %d", step)
	}
}
