package mcservice

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/content"
	"github.com/liteldev/LeviLauncher/internal/packages"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	instanceBackupScopeGameData = "gameData"
	instanceBackupScopeMods     = "mods"

	instanceBackupModeSafe = "safe"
	instanceBackupModeFull = "full"

	instanceBackupRestoreChoiceBackup  = "backup"
	instanceBackupRestoreChoiceCurrent = "current"

	instanceBackupFormatVersion = 2
	instanceBackupManifestName  = "backup-manifest.json"
)

var (
	errInstanceBackupReadSource   = errors.New("instance backup read source")
	errInstanceBackupWriteArchive = errors.New("instance backup write archive")
	errInstanceBackupInvalid      = errors.New("instance backup invalid archive")
	errInstanceBackupExtract      = errors.New("instance backup extract archive")
	errInstanceBackupWriteTarget  = errors.New("instance backup write target")
)

var instanceBackupFetchLeviLaminaVersionDB = FetchLeviLaminaVersionDB

func emitInstanceBackupRestoreProgress(
	phase string,
	currentStep int,
	totalSteps int,
	scopeKey string,
	scopeLabel string,
) {
	app := application.Get()
	if app == nil || app.Event == nil {
		return
	}
	app.Event.Emit(EventInstanceBackupRestoreProgress, types.InstanceBackupRestoreProgress{
		Phase:       strings.TrimSpace(phase),
		CurrentStep: currentStep,
		TotalSteps:  totalSteps,
		ScopeKey:    strings.TrimSpace(scopeKey),
		ScopeLabel:  strings.TrimSpace(scopeLabel),
		Ts:          time.Now().UnixMilli(),
	})
}

var instanceBackupSafeBedrockRoots = []string{
	"behavior_packs",
	"development_behavior_packs",
	"development_resource_packs",
	"development_skin_packs",
	"resource_packs",
	"minecraftWorlds",
	"custom_skins",
	"Screenshots",
	"skin_packs",
	"world_templates",
	"config",
}

type instanceBackupContext struct {
	info            types.InstanceBackupInfo
	gameVersion     string
	versionType     string
	enableIsolation bool
	isPreview       bool
	versionDir      string
	gameDataPath    string
	modsPath        string
}

type instanceBackupManifest struct {
	FormatVersion         int                                  `json:"formatVersion"`
	Name                  string                               `json:"name"`
	GameVersion           string                               `json:"gameVersion"`
	Type                  string                               `json:"type"`
	EnableIsolation       bool                                 `json:"enableIsolation"`
	CreatedAt             time.Time                            `json:"createdAt"`
	SelectedScopes        []string                             `json:"selectedScopes"`
	ScopeModes            map[string]string                    `json:"scopeModes,omitempty"`
	BedrockWhitelistRoots []string                             `json:"bedrockWhitelistRoots,omitempty"`
	LIPPackages           []types.InstanceBackupModsLIPPackage `json:"lipPackages,omitempty"`
	RawModFolders         []string                             `json:"rawModFolders,omitempty"`
}

type instanceBackupArchive struct {
	info         types.InstanceBackupArchiveInfo
	manifest     instanceBackupManifest
	gameDataRoot string
	modsRawRoot  string
}

type instanceBackupSourceEntry struct {
	ArchivePath string
	SourcePath  string
	IsDir       bool
}

type instanceBackupPackMetadata struct {
	UUID             string
	Name             string
	Version          string
	MinEngineVersion string
	FolderName       string
	RootName         string
	RelativePath     string
	AbsolutePath     string
}

type instanceBackupWorldMetadata struct {
	FolderName   string
	LevelName    string
	RelativePath string
	AbsolutePath string
}

func GetInstanceBackupInfo(name string) types.InstanceBackupInfo {
	return buildInstanceBackupContext(name).info
}

func BackupInstance(name string, request types.InstanceBackupRequest) types.InstanceBackupResult {
	ctx := buildInstanceBackupContext(name)
	result := types.InstanceBackupResult{
		BackupDir: ctx.info.BackupDir,
	}
	if ctx.info.ErrorCode != "" {
		result.ErrorCode = ctx.info.ErrorCode
		return result
	}

	selectedScopes, hasInvalidScope := normalizeInstanceBackupScopes(request.Scopes)
	if hasInvalidScope {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_INVALID_SCOPE"
		return result
	}
	if len(selectedScopes) == 0 {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_NO_SCOPE"
		return result
	}

	scopeByKey := make(map[string]types.InstanceBackupScope, len(ctx.info.Scopes))
	for _, scope := range ctx.info.Scopes {
		scopeByKey[scope.Key] = scope
	}

	scopeModes := make(map[string]string, len(selectedScopes))
	for _, key := range selectedScopes {
		scope, ok := scopeByKey[key]
		if !ok || !scope.Selectable {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_INVALID_SCOPE"
			return result
		}
		mode, valid := normalizeInstanceBackupScopeMode(scope, request.ScopeModes[key])
		if !valid {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_INVALID_SCOPE_MODE"
			return result
		}
		scopeModes[key] = mode
		if key == instanceBackupScopeGameData && !utils.DirExists(scope.Path) {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_GAME_DATA_NOT_FOUND"
			return result
		}
	}

	if err := utils.CreateDir(ctx.info.BackupDir); err != nil {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_CREATE_DIR"
		return result
	}

	lipPackages := []types.InstanceBackupModsLIPPackage(nil)
	rawModFolders := []string(nil)
	if containsString(selectedScopes, instanceBackupScopeMods) {
		lipPackages, _ = normalizeInstanceBackupModsLIPPackages(request.ModsLIPPackages, ctx.modsPath)
		rawModFolders = collectInstanceBackupRawModFolders(ctx.modsPath, lipPackages)
	}

	now := time.Now()
	safeName := sanitizeInstanceBackupSegment(ctx.info.Name, "instance")
	archivePath := filepath.Join(
		ctx.info.BackupDir,
		fmt.Sprintf("%s_%s.zip", safeName, now.Format("20060102-150405")),
	)
	out, err := os.Create(archivePath)
	if err != nil {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_CREATE_ARCHIVE"
		return result
	}
	zw := zip.NewWriter(out)

	zipClosed := false
	fileClosed := false
	success := false
	defer func() {
		if !zipClosed {
			_ = zw.Close()
		}
		if !fileClosed {
			_ = out.Close()
		}
		if !success {
			_ = os.Remove(archivePath)
		}
	}()

	manifest := instanceBackupManifest{
		FormatVersion:         instanceBackupFormatVersion,
		Name:                  ctx.info.Name,
		GameVersion:           ctx.gameVersion,
		Type:                  ctx.versionType,
		EnableIsolation:       ctx.enableIsolation,
		CreatedAt:             now,
		SelectedScopes:        append([]string{}, selectedScopes...),
		ScopeModes:            copyStringMap(scopeModes),
		BedrockWhitelistRoots: append([]string{}, instanceBackupSafeBedrockRoots...),
		LIPPackages:           cloneInstanceBackupModsLIPPackages(lipPackages),
		RawModFolders:         append([]string{}, rawModFolders...),
	}
	if err := writeInstanceBackupManifestToZip(zw, manifest); err != nil {
		_ = zw.Close()
		zipClosed = true
		result.ErrorCode = mapInstanceBackupErrorCode(err)
		return result
	}

	for _, key := range selectedScopes {
		switch key {
		case instanceBackupScopeGameData:
			mode := scopeModes[key]
			if mode == instanceBackupModeFull {
				if err := writeNamedDirToZip(zw, instanceBackupScopeGameData, ctx.gameDataPath); err != nil {
					_ = zw.Close()
					zipClosed = true
					result.ErrorCode = mapInstanceBackupErrorCode(err)
					return result
				}
				continue
			}
			if err := writeZipDirEntry(zw, instanceBackupScopeGameData); err != nil {
				_ = zw.Close()
				zipClosed = true
				result.ErrorCode = mapInstanceBackupErrorCode(err)
				return result
			}
			entries, _, err := collectInstanceBackupSafeGameDataEntries(ctx.gameDataPath)
			if err != nil {
				_ = zw.Close()
				zipClosed = true
				result.ErrorCode = mapInstanceBackupErrorCode(err)
				return result
			}
			for _, entry := range entries {
				if err := writeInstanceBackupSourceEntryToZip(zw, entry); err != nil {
					_ = zw.Close()
					zipClosed = true
					result.ErrorCode = mapInstanceBackupErrorCode(err)
					return result
				}
			}
		case instanceBackupScopeMods:
			if err := writeZipDirEntry(zw, "mods"); err != nil {
				_ = zw.Close()
				zipClosed = true
				result.ErrorCode = mapInstanceBackupErrorCode(err)
				return result
			}
			if err := writeZipDirEntry(zw, "mods/raw"); err != nil {
				_ = zw.Close()
				zipClosed = true
				result.ErrorCode = mapInstanceBackupErrorCode(err)
				return result
			}
			for _, folder := range rawModFolders {
				srcDir := filepath.Join(ctx.modsPath, folder)
				if !utils.DirExists(srcDir) {
					continue
				}
				if err := writeNamedDirToZip(zw, path.Join("mods", "raw", folder), srcDir); err != nil {
					_ = zw.Close()
					zipClosed = true
					result.ErrorCode = mapInstanceBackupErrorCode(err)
					return result
				}
			}
		default:
			_ = zw.Close()
			zipClosed = true
			result.ErrorCode = "ERR_INSTANCE_BACKUP_INVALID_SCOPE"
			return result
		}
	}

	if err := zw.Close(); err != nil {
		zipClosed = true
		result.ErrorCode = "ERR_INSTANCE_BACKUP_WRITE_ARCHIVE"
		return result
	}
	zipClosed = true
	if err := out.Close(); err != nil {
		fileClosed = true
		result.ErrorCode = "ERR_INSTANCE_BACKUP_WRITE_ARCHIVE"
		return result
	}
	fileClosed = true

	success = true
	result.ArchivePath = archivePath
	result.IncludedScopes = append([]string{}, selectedScopes...)
	return result
}

func InspectInstanceBackupArchive(archivePath string) types.InstanceBackupArchiveInfo {
	trimmedPath := strings.TrimSpace(archivePath)
	info := types.InstanceBackupArchiveInfo{
		ArchivePath: trimmedPath,
		ArchiveName: filepath.Base(trimmedPath),
	}
	if trimmedPath == "" {
		info.ErrorCode = "ERR_INSTANCE_BACKUP_ARCHIVE_OPEN"
		return info
	}

	reader, err := zip.OpenReader(trimmedPath)
	if err != nil {
		info.ErrorCode = "ERR_INSTANCE_BACKUP_ARCHIVE_OPEN"
		return info
	}
	defer reader.Close()

	archive, err := inspectInstanceBackupArchive(trimmedPath, reader.File)
	if err != nil {
		info.ErrorCode = mapInstanceBackupArchiveErrorCode(err)
		return info
	}
	return archive.info
}

func PreviewInstanceBackupRestoreConflicts(name string, request types.InstanceBackupRestoreRequest) types.InstanceBackupRestoreConflictInfo {
	trimmedArchivePath := strings.TrimSpace(request.ArchivePath)
	info := types.InstanceBackupRestoreConflictInfo{
		ArchivePath: trimmedArchivePath,
	}
	if trimmedArchivePath == "" {
		info.ErrorCode = "ERR_INSTANCE_BACKUP_ARCHIVE_OPEN"
		return info
	}

	target := buildInstanceBackupContext(name)
	if target.info.ErrorCode != "" {
		info.ErrorCode = target.info.ErrorCode
		return info
	}

	reader, err := zip.OpenReader(trimmedArchivePath)
	if err != nil {
		info.ErrorCode = "ERR_INSTANCE_BACKUP_ARCHIVE_OPEN"
		return info
	}
	archive, err := inspectInstanceBackupArchive(trimmedArchivePath, reader.File)
	_ = reader.Close()
	if err != nil {
		info.ErrorCode = mapInstanceBackupArchiveErrorCode(err)
		return info
	}

	selectedScopes, errCode := normalizeInstanceBackupRestoreScopes(request.Scopes, archive.info.IncludedScopes)
	if errCode != "" {
		info.ErrorCode = errCode
		return info
	}
	info.IncludedScopes = append([]string{}, selectedScopes...)

	tempDir, err := os.MkdirTemp("", "levilauncher-instance-backup-preview-*")
	if err != nil {
		info.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_EXTRACT_ARCHIVE"
		return info
	}
	defer os.RemoveAll(tempDir)

	if err := extractInstanceBackupArchive(trimmedArchivePath, tempDir); err != nil {
		info.ErrorCode = mapInstanceBackupRestoreErrorCode(err)
		return info
	}

	conflicts, err := collectInstanceBackupRestoreConflicts(tempDir, archive, target, selectedScopes)
	if err != nil {
		if errors.Is(err, errInstanceBackupInvalid) {
			info.ErrorCode = "ERR_INSTANCE_BACKUP_INVALID_ARCHIVE"
		} else {
			info.ErrorCode = mapInstanceBackupRestoreErrorCode(err)
		}
		return info
	}
	info.Conflicts = conflicts
	return info
}

func RestoreInstanceBackup(ctx context.Context, name string, request types.InstanceBackupRestoreRequest) types.InstanceBackupRestoreResult {
	trimmedArchivePath := strings.TrimSpace(request.ArchivePath)
	result := types.InstanceBackupRestoreResult{
		ArchivePath: trimmedArchivePath,
		Status:      "failed",
	}
	if trimmedArchivePath == "" {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_ARCHIVE_OPEN"
		return result
	}

	target := buildInstanceBackupContext(name)
	if target.info.ErrorCode != "" {
		result.ErrorCode = target.info.ErrorCode
		return result
	}

	reader, err := zip.OpenReader(trimmedArchivePath)
	if err != nil {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_ARCHIVE_OPEN"
		return result
	}
	archive, err := inspectInstanceBackupArchive(trimmedArchivePath, reader.File)
	_ = reader.Close()
	if err != nil {
		result.ErrorCode = mapInstanceBackupArchiveErrorCode(err)
		return result
	}

	selectedScopes, errCode := normalizeInstanceBackupRestoreScopes(request.Scopes, archive.info.IncludedScopes)
	if errCode != "" {
		result.ErrorCode = errCode
		return result
	}
	result.IncludedScopes = append([]string{}, selectedScopes...)
	totalSteps := len(selectedScopes) + 3
	emitInstanceBackupRestoreProgress("preparing", 1, totalSteps, "", "")

	tempDir, err := os.MkdirTemp("", "levilauncher-instance-backup-*")
	if err != nil {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_EXTRACT_ARCHIVE"
		return result
	}
	defer os.RemoveAll(tempDir)

	emitInstanceBackupRestoreProgress("extracting", 2, totalSteps, "", "")
	if err := extractInstanceBackupArchive(trimmedArchivePath, tempDir); err != nil {
		result.ErrorCode = mapInstanceBackupRestoreErrorCode(err)
		return result
	}

	conflicts, err := collectInstanceBackupRestoreConflicts(tempDir, archive, target, selectedScopes)
	if err != nil {
		if errors.Is(err, errInstanceBackupInvalid) {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_INVALID_ARCHIVE"
		} else {
			result.ErrorCode = mapInstanceBackupRestoreErrorCode(err)
		}
		return result
	}
	resolutionByID, valid := normalizeInstanceBackupRestoreConflictResolutions(request.ConflictResolutions)
	if !valid {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_INVALID_CONFLICT_RESOLUTION"
		return result
	}
	if len(resolutionByID) > 0 {
		conflictSet := make(map[string]struct{}, len(conflicts))
		for _, conflict := range conflicts {
			conflictSet[conflict.ID] = struct{}{}
		}
		for conflictID := range resolutionByID {
			if _, ok := conflictSet[conflictID]; !ok {
				result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_INVALID_CONFLICT_RESOLUTION"
				return result
			}
		}
		for _, conflict := range conflicts {
			if _, ok := resolutionByID[conflict.ID]; !ok {
				result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_CONFLICT_UNRESOLVED"
				return result
			}
		}
	}

	successCount := 0
	partialCount := 0
	for index, key := range selectedScopes {
		scopeLabel := instanceBackupScopeLabel(key, archive.info.Type)
		emitInstanceBackupRestoreProgress(
			"restoring_scope",
			index+3,
			totalSteps,
			key,
			scopeLabel,
		)
		var scopeResult types.InstanceBackupRestoreScopeResult
		switch key {
		case instanceBackupScopeGameData:
			scopeResult = restoreInstanceBackupGameData(tempDir, archive, target, resolutionByID)
		case instanceBackupScopeMods:
			scopeResult = restoreInstanceBackupMods(ctx, name, tempDir, archive, target, resolutionByID)
		default:
			scopeResult = types.InstanceBackupRestoreScopeResult{
				Key:       key,
				Label:     instanceBackupScopeLabel(key, archive.info.Type),
				Status:    "failed",
				ErrorCode: "ERR_INSTANCE_BACKUP_INVALID_SCOPE",
			}
		}
		result.ScopeResults = append(result.ScopeResults, scopeResult)
		switch scopeResult.Status {
		case "success":
			successCount++
		case "partial":
			partialCount++
		}
	}
	emitInstanceBackupRestoreProgress("finalizing", totalSteps, totalSteps, "", "")

	switch {
	case len(result.ScopeResults) == 0:
		result.Status = "failed"
		result.ErrorCode = "ERR_INSTANCE_BACKUP_NO_SCOPE"
	case successCount == len(result.ScopeResults):
		result.Status = "success"
	case successCount > 0 || partialCount > 0:
		result.Status = "partial"
	default:
		result.Status = "failed"
		for _, scopeResult := range result.ScopeResults {
			if strings.TrimSpace(scopeResult.ErrorCode) != "" {
				result.ErrorCode = scopeResult.ErrorCode
				break
			}
		}
		if result.ErrorCode == "" {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
		}
	}

	return result
}

func buildInstanceBackupContext(name string) instanceBackupContext {
	trimmedName := strings.TrimSpace(name)
	ctx := instanceBackupContext{
		info: types.InstanceBackupInfo{
			Name: trimmedName,
		},
	}
	if trimmedName == "" {
		ctx.info.ErrorCode = "ERR_INSTANCE_BACKUP_INSTANCE_NOT_FOUND"
		return ctx
	}

	ctx.info.BackupDir = filepath.Join(
		apppath.BaseRoot(),
		"backups",
		"instances",
		sanitizeInstanceBackupSegment(trimmedName, "instance"),
	)

	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		ctx.info.ErrorCode = "ERR_ACCESS_VERSIONS_DIR"
		return ctx
	}

	ctx.versionDir = filepath.Join(vdir, trimmedName)
	if !utils.DirExists(ctx.versionDir) {
		ctx.info.ErrorCode = "ERR_INSTANCE_BACKUP_INSTANCE_NOT_FOUND"
		return ctx
	}

	meta := GetVersionMeta(trimmedName)
	roots := GetContentRoots(trimmedName)
	ctx.gameVersion = strings.TrimSpace(meta.GameVersion)
	ctx.versionType = normalizeInstanceBackupVersionType(meta.Type, roots.IsPreview)
	ctx.enableIsolation = roots.IsIsolation
	ctx.isPreview = roots.IsPreview
	ctx.gameDataPath = strings.TrimSpace(roots.Base)
	ctx.modsPath = filepath.Join(ctx.versionDir, "mods")

	gameDataExists := ctx.gameDataPath != "" && utils.DirExists(ctx.gameDataPath)
	safePath := filepath.Join(ctx.gameDataPath, "Users")
	safeExists := gameDataExists && utils.DirExists(safePath)
	fullSize := int64(0)
	safeSize := int64(0)
	if gameDataExists {
		fullSize = utils.DirSize(ctx.gameDataPath)
		if _, size, err := collectInstanceBackupSafeGameDataEntries(ctx.gameDataPath); err == nil {
			safeSize = size
		}
	}
	defaultMode := instanceBackupModeSafe
	if !safeExists && gameDataExists {
		defaultMode = instanceBackupModeFull
	}

	modsExists := utils.DirExists(ctx.modsPath)
	modsSize := int64(0)
	if modsExists {
		modsSize = utils.DirSize(ctx.modsPath)
	}

	ctx.info.Scopes = []types.InstanceBackupScope{
		{
			Key:        instanceBackupScopeGameData,
			Label:      instanceBackupGameDataLabel(roots.IsPreview),
			Path:       ctx.gameDataPath,
			Size:       safeSize,
			Selectable: gameDataExists,
			Exists:     gameDataExists,
			Shared:     !roots.IsIsolation,
			Modes: []types.InstanceBackupScopeMode{
				{
					Key:        instanceBackupModeSafe,
					Path:       safePath,
					Size:       safeSize,
					Selectable: safeExists,
				},
				{
					Key:        instanceBackupModeFull,
					Path:       ctx.gameDataPath,
					Size:       fullSize,
					Selectable: gameDataExists,
					Warning:    "versions.edit.backup.mode.full.warning",
				},
			},
			DefaultMode: defaultMode,
			Warnings:    buildInstanceBackupScopeWarnings(!roots.IsIsolation),
		},
		{
			Key:         instanceBackupScopeMods,
			Label:       instanceBackupScopeMods,
			Path:        ctx.modsPath,
			Size:        modsSize,
			Selectable:  modsExists,
			Exists:      modsExists,
			Shared:      false,
			DefaultMode: "",
		},
	}
	return ctx
}

func normalizeInstanceBackupVersionType(versionType string, isPreview bool) string {
	normalized := strings.ToLower(strings.TrimSpace(versionType))
	if normalized == "preview" || normalized == "release" {
		return normalized
	}
	if isPreview {
		return "preview"
	}
	return "release"
}

func instanceBackupGameDataLabel(isPreview bool) string {
	if isPreview {
		return "Minecraft Bedrock Preview"
	}
	return "Minecraft Bedrock"
}

func instanceBackupScopeLabel(scopeKey string, versionType string) string {
	switch strings.TrimSpace(scopeKey) {
	case instanceBackupScopeGameData:
		return instanceBackupGameDataLabel(strings.EqualFold(strings.TrimSpace(versionType), "preview"))
	case instanceBackupScopeMods:
		return instanceBackupScopeMods
	default:
		return strings.TrimSpace(scopeKey)
	}
}

func buildInstanceBackupScopeWarnings(isShared bool) []string {
	if !isShared {
		return nil
	}
	return []string{"versions.edit.backup.shared_warning"}
}

func normalizeInstanceBackupScopes(scopes []string) ([]string, bool) {
	allowed := map[string]string{
		strings.ToLower(instanceBackupScopeGameData): instanceBackupScopeGameData,
		strings.ToLower(instanceBackupScopeMods):     instanceBackupScopeMods,
	}
	selected := make(map[string]struct{}, len(scopes))
	hasInvalid := false
	for _, raw := range scopes {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" {
			continue
		}
		normalized, ok := allowed[key]
		if !ok {
			hasInvalid = true
			continue
		}
		selected[normalized] = struct{}{}
	}
	ordered := make([]string, 0, len(selected))
	for _, key := range []string{instanceBackupScopeGameData, instanceBackupScopeMods} {
		if _, ok := selected[key]; ok {
			ordered = append(ordered, key)
		}
	}
	return ordered, hasInvalid
}

func normalizeInstanceBackupRestoreScopes(requested []string, available []string) ([]string, string) {
	availableSet := make(map[string]struct{}, len(available))
	for _, key := range available {
		availableSet[strings.TrimSpace(key)] = struct{}{}
	}
	if len(requested) == 0 {
		return append([]string{}, available...), ""
	}
	selected, hasInvalid := normalizeInstanceBackupScopes(requested)
	if hasInvalid {
		return nil, "ERR_INSTANCE_BACKUP_INVALID_SCOPE"
	}
	if len(selected) == 0 {
		return nil, "ERR_INSTANCE_BACKUP_NO_SCOPE"
	}
	for _, key := range selected {
		if _, ok := availableSet[key]; !ok {
			return nil, "ERR_INSTANCE_BACKUP_INVALID_SCOPE"
		}
	}
	return selected, ""
}

func normalizeInstanceBackupScopeMode(scope types.InstanceBackupScope, requested string) (string, bool) {
	if len(scope.Modes) == 0 {
		return "", strings.TrimSpace(requested) == ""
	}
	requested = strings.TrimSpace(requested)
	if requested == "" {
		requested = strings.TrimSpace(scope.DefaultMode)
	}
	for _, mode := range scope.Modes {
		if mode.Key != requested {
			continue
		}
		return requested, mode.Selectable
	}
	return "", false
}

func normalizeInstanceBackupModsLIPPackages(
	packages []types.InstanceBackupModsLIPPackage,
	modsPath string,
) ([]types.InstanceBackupModsLIPPackage, map[string]struct{}) {
	validFolders := make(map[string]struct{})
	if !utils.DirExists(modsPath) || len(packages) == 0 {
		return nil, validFolders
	}

	seen := make(map[string]struct{}, len(packages))
	normalized := make([]types.InstanceBackupModsLIPPackage, 0, len(packages))
	for _, item := range packages {
		identifier := strings.TrimSpace(item.Identifier)
		version := strings.TrimSpace(item.Version)
		if identifier == "" || version == "" {
			continue
		}
		folders := normalizeInstanceBackupFolderList(item.Folders, modsPath)
		if len(folders) == 0 {
			continue
		}
		lookupKey := strings.ToLower(identifier)
		if _, ok := seen[lookupKey]; ok {
			continue
		}
		seen[lookupKey] = struct{}{}
		for _, folder := range folders {
			validFolders[strings.ToLower(folder)] = struct{}{}
		}
		normalized = append(normalized, types.InstanceBackupModsLIPPackage{
			Identifier:        identifier,
			Version:           version,
			ExplicitInstalled: item.ExplicitInstalled,
			Folders:           folders,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		return strings.ToLower(normalized[i].Identifier) < strings.ToLower(normalized[j].Identifier)
	})
	return normalized, validFolders
}

func normalizeInstanceBackupFolderList(folders []string, modsPath string) []string {
	dedup := make(map[string]struct{}, len(folders))
	normalized := make([]string, 0, len(folders))
	for _, raw := range folders {
		folder := strings.TrimSpace(raw)
		if folder == "" || strings.Contains(folder, "/") || strings.Contains(folder, `\`) {
			continue
		}
		lookupKey := strings.ToLower(folder)
		if _, ok := dedup[lookupKey]; ok {
			continue
		}
		if !utils.DirExists(filepath.Join(modsPath, folder)) {
			continue
		}
		dedup[lookupKey] = struct{}{}
		normalized = append(normalized, folder)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return strings.ToLower(normalized[i]) < strings.ToLower(normalized[j])
	})
	return normalized
}

func collectInstanceBackupRawModFolders(
	modsPath string,
	lipPackages []types.InstanceBackupModsLIPPackage,
) []string {
	if !utils.DirExists(modsPath) {
		return nil
	}
	lipFolderSet := make(map[string]struct{})
	for _, item := range lipPackages {
		for _, folder := range item.Folders {
			lipFolderSet[strings.ToLower(strings.TrimSpace(folder))] = struct{}{}
		}
	}
	entries, err := os.ReadDir(modsPath)
	if err != nil {
		return nil
	}
	rawFolders := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		if _, ok := lipFolderSet[strings.ToLower(name)]; ok {
			continue
		}
		hasPayload, err := instanceBackupDirContainsPayload(filepath.Join(modsPath, name))
		if err != nil || !hasPayload {
			continue
		}
		rawFolders = append(rawFolders, name)
	}
	sort.Slice(rawFolders, func(i, j int) bool {
		return strings.ToLower(rawFolders[i]) < strings.ToLower(rawFolders[j])
	})
	return rawFolders
}

func cloneInstanceBackupModsLIPPackages(items []types.InstanceBackupModsLIPPackage) []types.InstanceBackupModsLIPPackage {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]types.InstanceBackupModsLIPPackage, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, types.InstanceBackupModsLIPPackage{
			Identifier:        item.Identifier,
			Version:           item.Version,
			ExplicitInstalled: item.ExplicitInstalled,
			Folders:           append([]string{}, item.Folders...),
		})
	}
	return cloned
}

func inspectInstanceBackupArchive(archivePath string, files []*zip.File) (instanceBackupArchive, error) {
	manifestBytes, err := readInstanceBackupZipFile(files, instanceBackupManifestName)
	if err != nil {
		return instanceBackupArchive{}, wrapInstanceBackupInvalidError(err)
	}

	var manifest instanceBackupManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return instanceBackupArchive{}, wrapInstanceBackupInvalidError(err)
	}
	if manifest.FormatVersion != instanceBackupFormatVersion {
		return instanceBackupArchive{}, wrapInstanceBackupInvalidError(os.ErrInvalid)
	}
	selectedScopes, hasInvalid := normalizeInstanceBackupScopes(manifest.SelectedScopes)
	if hasInvalid || len(selectedScopes) == 0 {
		return instanceBackupArchive{}, wrapInstanceBackupInvalidError(os.ErrInvalid)
	}
	manifest.SelectedScopes = selectedScopes
	if manifest.ScopeModes == nil {
		manifest.ScopeModes = map[string]string{}
	}

	archive := instanceBackupArchive{
		manifest:     manifest,
		gameDataRoot: instanceBackupScopeGameData,
		modsRawRoot:  path.Join("mods", "raw"),
	}
	archive.info = buildInstanceBackupArchiveInfo(archivePath, archive)
	return archive, nil
}

func buildInstanceBackupArchiveInfo(archivePath string, archive instanceBackupArchive) types.InstanceBackupArchiveInfo {
	info := types.InstanceBackupArchiveInfo{
		FormatVersion:         archive.manifest.FormatVersion,
		Name:                  archive.manifest.Name,
		ArchivePath:           archivePath,
		ArchiveName:           filepath.Base(strings.TrimSpace(archivePath)),
		GameVersion:           archive.manifest.GameVersion,
		Type:                  archive.manifest.Type,
		EnableIsolation:       archive.manifest.EnableIsolation,
		CreatedAt:             archive.manifest.CreatedAt.Format(time.RFC3339),
		IncludedScopes:        append([]string{}, archive.manifest.SelectedScopes...),
		ScopeModes:            copyStringMap(archive.manifest.ScopeModes),
		ModsLIPPackages:       cloneInstanceBackupModsLIPPackages(archive.manifest.LIPPackages),
		RawModFolders:         append([]string{}, archive.manifest.RawModFolders...),
		BedrockWhitelistRoots: append([]string{}, archive.manifest.BedrockWhitelistRoots...),
	}
	for _, key := range archive.manifest.SelectedScopes {
		mode := strings.TrimSpace(archive.manifest.ScopeModes[key])
		scopeInfo := types.InstanceBackupArchiveScope{
			Key:   key,
			Label: instanceBackupScopeLabel(key, archive.manifest.Type),
			Mode:  mode,
		}
		if key == instanceBackupScopeGameData && mode == instanceBackupModeFull {
			scopeInfo.Warnings = []string{"versions.edit.backup.mode.full.warning"}
		}
		info.Scopes = append(info.Scopes, scopeInfo)
	}
	return info
}

func collectInstanceBackupSafeGameDataEntries(gameDataPath string) ([]instanceBackupSourceEntry, int64, error) {
	base := strings.TrimSpace(gameDataPath)
	if base == "" {
		return nil, 0, wrapInstanceBackupReadError(os.ErrNotExist)
	}
	usersRoot := filepath.Join(base, "Users")
	if !utils.DirExists(usersRoot) {
		return nil, 0, nil
	}

	userEntries, err := os.ReadDir(usersRoot)
	if err != nil {
		return nil, 0, wrapInstanceBackupReadError(err)
	}

	collected := make([]instanceBackupSourceEntry, 0, len(userEntries)*len(instanceBackupSafeBedrockRoots))
	var totalSize int64
	for _, userEntry := range userEntries {
		if !userEntry.IsDir() {
			continue
		}
		userName := strings.TrimSpace(userEntry.Name())
		if userName == "" {
			continue
		}
		mojangRoot := filepath.Join(usersRoot, userName, "games", "com.mojang")
		if !utils.DirExists(mojangRoot) {
			continue
		}
		for _, rootName := range instanceBackupSafeBedrockRoots {
			sourcePath := filepath.Join(mojangRoot, rootName)
			info, err := os.Stat(sourcePath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, 0, wrapInstanceBackupReadError(err)
			}
			if info.IsDir() {
				hasPayload, err := instanceBackupDirContainsPayload(sourcePath)
				if err != nil {
					return nil, 0, wrapInstanceBackupReadError(err)
				}
				if !hasPayload {
					continue
				}
				collected = append(collected, instanceBackupSourceEntry{
					ArchivePath: path.Join("gameData", "Users", userName, "games", "com.mojang", rootName),
					SourcePath:  sourcePath,
					IsDir:       true,
				})
				totalSize += utils.DirSize(sourcePath)
			} else {
				collected = append(collected, instanceBackupSourceEntry{
					ArchivePath: path.Join("gameData", "Users", userName, "games", "com.mojang", rootName),
					SourcePath:  sourcePath,
					IsDir:       false,
				})
				totalSize += info.Size()
			}
		}
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].ArchivePath < collected[j].ArchivePath
	})
	return collected, totalSize, nil
}

func writeInstanceBackupManifestToZip(zw *zip.Writer, manifest instanceBackupManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return wrapInstanceBackupWriteError(err)
	}
	header := &zip.FileHeader{
		Name:   instanceBackupManifestName,
		Method: zip.Deflate,
	}
	header.SetMode(0o644)
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return wrapInstanceBackupWriteError(err)
	}
	if _, err := writer.Write(data); err != nil {
		return wrapInstanceBackupWriteError(err)
	}
	return nil
}

func writeInstanceBackupSourceEntryToZip(zw *zip.Writer, entry instanceBackupSourceEntry) error {
	info, err := os.Stat(entry.SourcePath)
	if err != nil {
		return wrapInstanceBackupReadError(err)
	}
	if info.IsDir() {
		return writeNamedDirToZipSkippingEmptyDirs(zw, entry.ArchivePath, entry.SourcePath)
	}
	return writeFileToZip(zw, entry.ArchivePath, entry.SourcePath, info)
}

func writeNamedDirToZip(zw *zip.Writer, alias string, srcDir string) error {
	cleanAlias := normalizeInstanceBackupArchiveName(alias)
	cleanSource := filepath.Clean(strings.TrimSpace(srcDir))
	if cleanAlias == "" || cleanSource == "" {
		return wrapInstanceBackupReadError(os.ErrInvalid)
	}
	info, err := os.Stat(cleanSource)
	if err != nil {
		return wrapInstanceBackupReadError(err)
	}
	if !info.IsDir() {
		return wrapInstanceBackupReadError(os.ErrInvalid)
	}

	if err := writeZipDirEntry(zw, cleanAlias); err != nil {
		return err
	}
	return filepath.Walk(cleanSource, func(currentPath string, entry os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return wrapInstanceBackupReadError(walkErr)
		}
		relPath, err := filepath.Rel(cleanSource, currentPath)
		if err != nil {
			return wrapInstanceBackupReadError(err)
		}
		if relPath == "." {
			return nil
		}
		archiveName := path.Join(cleanAlias, filepath.ToSlash(relPath))
		if entry.IsDir() {
			return writeZipDirEntry(zw, archiveName)
		}
		return writeFileToZip(zw, archiveName, currentPath, entry)
	})
}

func writeNamedDirToZipSkippingEmptyDirs(zw *zip.Writer, alias string, srcDir string) error {
	cleanAlias := normalizeInstanceBackupArchiveName(alias)
	cleanSource := filepath.Clean(strings.TrimSpace(srcDir))
	if cleanAlias == "" || cleanSource == "" {
		return wrapInstanceBackupReadError(os.ErrInvalid)
	}
	info, err := os.Stat(cleanSource)
	if err != nil {
		return wrapInstanceBackupReadError(err)
	}
	if !info.IsDir() {
		return wrapInstanceBackupReadError(os.ErrInvalid)
	}

	hasPayload, err := instanceBackupDirContainsPayload(cleanSource)
	if err != nil {
		return wrapInstanceBackupReadError(err)
	}
	if !hasPayload {
		return nil
	}

	if err := writeZipDirEntry(zw, cleanAlias); err != nil {
		return err
	}
	return writeNamedDirContentsToZipSkippingEmptyDirs(zw, cleanAlias, cleanSource)
}

func writeNamedDirContentsToZipSkippingEmptyDirs(zw *zip.Writer, alias string, srcDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return wrapInstanceBackupReadError(err)
	}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		currentPath := filepath.Join(srcDir, name)
		archiveName := path.Join(alias, filepath.ToSlash(name))
		info, err := entry.Info()
		if err != nil {
			return wrapInstanceBackupReadError(err)
		}
		if info.IsDir() {
			hasPayload, err := instanceBackupDirContainsPayload(currentPath)
			if err != nil {
				return wrapInstanceBackupReadError(err)
			}
			if !hasPayload {
				continue
			}
			if err := writeZipDirEntry(zw, archiveName); err != nil {
				return err
			}
			if err := writeNamedDirContentsToZipSkippingEmptyDirs(zw, archiveName, currentPath); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := writeFileToZip(zw, archiveName, currentPath, info); err != nil {
			return err
		}
	}
	return nil
}

func instanceBackupDirContainsPayload(dir string) (bool, error) {
	cleanDir := filepath.Clean(strings.TrimSpace(dir))
	if cleanDir == "" {
		return false, os.ErrInvalid
	}
	info, err := os.Stat(cleanDir)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return info.Mode().IsRegular(), nil
	}
	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		currentPath := filepath.Join(cleanDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return false, err
		}
		if info.IsDir() {
			hasPayload, err := instanceBackupDirContainsPayload(currentPath)
			if err != nil {
				return false, err
			}
			if hasPayload {
				return true, nil
			}
			continue
		}
		if info.Mode().IsRegular() {
			return true, nil
		}
	}
	return false, nil
}

func writeFileToZip(zw *zip.Writer, archiveName string, srcPath string, info os.FileInfo) error {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return wrapInstanceBackupWriteError(err)
	}
	header.Name = normalizeInstanceBackupArchiveName(archiveName)
	header.Method = zip.Deflate
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return wrapInstanceBackupWriteError(err)
	}
	file, err := os.Open(srcPath)
	if err != nil {
		return wrapInstanceBackupReadError(err)
	}
	defer file.Close()
	if _, err := io.Copy(writer, file); err != nil {
		return wrapInstanceBackupWriteError(err)
	}
	return nil
}

func writeZipDirEntry(zw *zip.Writer, archiveName string) error {
	cleanName := normalizeInstanceBackupArchiveName(archiveName)
	if cleanName == "" {
		return wrapInstanceBackupWriteError(os.ErrInvalid)
	}
	header := &zip.FileHeader{
		Name:   cleanName + "/",
		Method: zip.Store,
	}
	header.SetMode(os.ModeDir | 0o755)
	if _, err := zw.CreateHeader(header); err != nil {
		return wrapInstanceBackupWriteError(err)
	}
	return nil
}

func collectInstanceBackupRestoreConflicts(
	tempDir string,
	archive instanceBackupArchive,
	target instanceBackupContext,
	selectedScopes []string,
) ([]types.InstanceBackupRestoreConflict, error) {
	conflicts := make([]types.InstanceBackupRestoreConflict, 0)
	for _, key := range selectedScopes {
		switch key {
		case instanceBackupScopeGameData:
			items, err := collectInstanceBackupGameDataRestoreConflicts(tempDir, archive, target)
			if err != nil {
				return nil, err
			}
			conflicts = append(conflicts, items...)
		case instanceBackupScopeMods:
			items, err := collectInstanceBackupModsRestoreConflicts(tempDir, target)
			if err != nil {
				return nil, err
			}
			conflicts = append(conflicts, items...)
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].ScopeKey != conflicts[j].ScopeKey {
			return conflicts[i].ScopeKey < conflicts[j].ScopeKey
		}
		if conflicts[i].IdentityKind != conflicts[j].IdentityKind {
			return conflicts[i].IdentityKind < conflicts[j].IdentityKind
		}
		return conflicts[i].Path < conflicts[j].Path
	})
	return conflicts, nil
}

func collectInstanceBackupGameDataRestoreConflicts(
	tempDir string,
	archive instanceBackupArchive,
	target instanceBackupContext,
) ([]types.InstanceBackupRestoreConflict, error) {
	sourceRoot := filepath.Join(tempDir, filepath.FromSlash(archive.gameDataRoot))
	if !utils.DirExists(sourceRoot) {
		return nil, wrapInstanceBackupInvalidError(os.ErrInvalid)
	}
	targetRoot := strings.TrimSpace(target.gameDataPath)
	if targetRoot == "" {
		return nil, wrapInstanceBackupWriteTargetError(os.ErrNotExist)
	}

	return collectInstanceBackupGameDataPathConflicts(
		instanceBackupScopeLabel(instanceBackupScopeGameData, archive.info.Type),
		sourceRoot,
		targetRoot,
		"",
		nil,
	)
}

func collectInstanceBackupGameDataPathConflicts(
	scopeLabel string,
	sourcePath string,
	targetPath string,
	relativePath string,
	collected []types.InstanceBackupRestoreConflict,
) ([]types.InstanceBackupRestoreConflict, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return nil, wrapInstanceBackupWriteTargetError(err)
	}
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return collected, nil
		}
		return nil, wrapInstanceBackupWriteTargetError(err)
	}

	normalizedPath := ""
	if strings.TrimSpace(relativePath) != "" {
		normalizedPath = normalizeInstanceBackupRestoreConflictPath(relativePath, sourceInfo.Name())
	}
	if sourceInfo.IsDir() && targetInfo.IsDir() {
		if rootName, ok := matchInstanceBackupGameDataPackRoot(normalizedPath); ok {
			return collectInstanceBackupPackRootConflicts(
				scopeLabel,
				sourcePath,
				targetPath,
				normalizedPath,
				rootName,
				collected,
			)
		}
		if isInstanceBackupGameDataWorldRoot(normalizedPath) {
			return collectInstanceBackupWorldRootConflicts(
				scopeLabel,
				sourcePath,
				targetPath,
				normalizedPath,
				collected,
			)
		}

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return nil, wrapInstanceBackupWriteTargetError(err)
		}
		for _, entry := range entries {
			name := strings.TrimSpace(entry.Name())
			if name == "" {
				continue
			}
			childPath := path.Join(normalizedPath, name)
			var scanErr error
			collected, scanErr = collectInstanceBackupGameDataPathConflicts(
				scopeLabel,
				filepath.Join(sourcePath, name),
				filepath.Join(targetPath, name),
				childPath,
				collected,
			)
			if scanErr != nil {
				return nil, scanErr
			}
		}
		return collected, nil
	}

	collected = append(collected, buildInstanceBackupFileConflict(
		instanceBackupScopeGameData,
		scopeLabel,
		normalizedPath,
		sourceInfo,
		targetInfo,
	))
	return collected, nil
}

func collectInstanceBackupPackRootConflicts(
	scopeLabel string,
	sourceRoot string,
	targetRoot string,
	rootRelativePath string,
	rootName string,
	collected []types.InstanceBackupRestoreConflict,
) ([]types.InstanceBackupRestoreConflict, error) {
	sourceEntries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return nil, wrapInstanceBackupWriteTargetError(err)
	}

	targetByUUID, err := scanInstanceBackupPackRootByUUID(targetRoot, rootName, rootRelativePath)
	if err != nil {
		return nil, err
	}

	for _, entry := range sourceEntries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		sourceChildPath := filepath.Join(sourceRoot, name)
		childRelativePath := path.Join(rootRelativePath, name)
		targetChildPath := filepath.Join(targetRoot, name)

		if !entry.IsDir() {
			targetInfo, err := os.Stat(targetChildPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, wrapInstanceBackupWriteTargetError(err)
			}
			sourceInfo, statErr := os.Stat(sourceChildPath)
			if statErr != nil {
				return nil, wrapInstanceBackupWriteTargetError(statErr)
			}
			collected = append(collected, buildInstanceBackupFileConflict(
				instanceBackupScopeGameData,
				scopeLabel,
				childRelativePath,
				sourceInfo,
				targetInfo,
			))
			continue
		}

		sourceMeta, ok := readInstanceBackupPackMetadata(sourceChildPath, rootName, childRelativePath)
		if !ok {
			var scanErr error
			collected, scanErr = collectInstanceBackupGameDataPathConflicts(
				scopeLabel,
				sourceChildPath,
				targetChildPath,
				childRelativePath,
				collected,
			)
			if scanErr != nil {
				return nil, scanErr
			}
			continue
		}

		currentPath := ""
		var currentMeta *instanceBackupPackMetadata
		if matched, ok := targetByUUID[strings.ToLower(sourceMeta.UUID)]; ok {
			currentPath = matched.AbsolutePath
			metaCopy := matched
			currentMeta = &metaCopy
		}
		if currentPath == "" {
			if _, err := os.Stat(targetChildPath); err == nil {
				currentPath = targetChildPath
				if meta, ok := readInstanceBackupPackMetadata(targetChildPath, rootName, childRelativePath); ok {
					metaCopy := meta
					currentMeta = &metaCopy
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, wrapInstanceBackupWriteTargetError(err)
			}
		}
		if currentPath == "" {
			continue
		}

		targetInfo, err := os.Stat(currentPath)
		if err != nil {
			return nil, wrapInstanceBackupWriteTargetError(err)
		}
		collected = append(collected, buildInstanceBackupPackConflict(
			scopeLabel,
			sourceMeta,
			currentMeta,
			currentPath,
			targetInfo,
			rootRelativePath,
		))
	}

	return collected, nil
}

func collectInstanceBackupWorldRootConflicts(
	scopeLabel string,
	sourceRoot string,
	targetRoot string,
	rootRelativePath string,
	collected []types.InstanceBackupRestoreConflict,
) ([]types.InstanceBackupRestoreConflict, error) {
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return nil, wrapInstanceBackupWriteTargetError(err)
	}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		sourceChildPath := filepath.Join(sourceRoot, name)
		targetChildPath := filepath.Join(targetRoot, name)
		childRelativePath := path.Join(rootRelativePath, name)

		targetInfo, err := os.Stat(targetChildPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, wrapInstanceBackupWriteTargetError(err)
		}

		sourceInfo, err := os.Stat(sourceChildPath)
		if err != nil {
			return nil, wrapInstanceBackupWriteTargetError(err)
		}
		if !entry.IsDir() {
			collected = append(collected, buildInstanceBackupFileConflict(
				instanceBackupScopeGameData,
				scopeLabel,
				childRelativePath,
				sourceInfo,
				targetInfo,
			))
			continue
		}

		sourceMeta := readInstanceBackupWorldMetadata(sourceChildPath, childRelativePath)
		currentMeta := readInstanceBackupWorldMetadata(targetChildPath, childRelativePath)
		collected = append(collected, buildInstanceBackupWorldConflict(
			scopeLabel,
			sourceMeta,
			currentMeta,
			targetInfo,
		))
	}
	return collected, nil
}

func collectInstanceBackupModsRestoreConflicts(
	tempDir string,
	target instanceBackupContext,
) ([]types.InstanceBackupRestoreConflict, error) {
	sourceRawRoot := filepath.Join(tempDir, "mods", "raw")
	if !utils.DirExists(sourceRawRoot) {
		return nil, nil
	}
	conflicts := make([]types.InstanceBackupRestoreConflict, 0)
	entries, err := os.ReadDir(sourceRawRoot)
	if err != nil {
		return nil, wrapInstanceBackupWriteTargetError(err)
	}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		sourcePath := filepath.Join(sourceRawRoot, name)
		targetPath := filepath.Join(target.modsPath, name)
		targetInfo, err := os.Stat(targetPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, wrapInstanceBackupWriteTargetError(err)
		}

		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			return nil, wrapInstanceBackupWriteTargetError(err)
		}
		if !entry.IsDir() {
			conflicts = append(conflicts, buildInstanceBackupFileConflict(
				instanceBackupScopeMods,
				instanceBackupScopeMods,
				name,
				sourceInfo,
				targetInfo,
			))
			continue
		}

		sourceManifest, sourceOK := readInstanceBackupModManifest(sourcePath)
		targetManifest, targetOK := readInstanceBackupModManifest(targetPath)
		conflicts = append(conflicts, buildInstanceBackupModConflict(
			name,
			sourceManifest,
			sourceOK,
			targetManifest,
			targetOK,
			targetInfo,
		))
	}
	return conflicts, nil
}

func matchInstanceBackupGameDataPackRoot(relativePath string) (string, bool) {
	segments := splitInstanceBackupRestorePath(relativePath)
	if len(segments) != 5 {
		return "", false
	}
	if !strings.EqualFold(segments[0], "Users") ||
		!strings.EqualFold(segments[2], "games") ||
		!strings.EqualFold(segments[3], "com.mojang") {
		return "", false
	}
	if !isInstanceBackupPackRootName(segments[4]) {
		return "", false
	}
	return segments[4], true
}

func isInstanceBackupGameDataWorldRoot(relativePath string) bool {
	segments := splitInstanceBackupRestorePath(relativePath)
	return len(segments) == 5 &&
		strings.EqualFold(segments[0], "Users") &&
		strings.EqualFold(segments[2], "games") &&
		strings.EqualFold(segments[3], "com.mojang") &&
		strings.EqualFold(segments[4], "minecraftWorlds")
}

func splitInstanceBackupRestorePath(relativePath string) []string {
	normalized := normalizeInstanceBackupArchiveName(relativePath)
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "/")
}

func isInstanceBackupPackRootName(rootName string) bool {
	switch strings.ToLower(strings.TrimSpace(rootName)) {
	case "resource_packs",
		"behavior_packs",
		"skin_packs",
		"development_resource_packs",
		"development_behavior_packs",
		"development_skin_packs":
		return true
	default:
		return false
	}
}

func scanInstanceBackupPackRootByUUID(
	rootPath string,
	rootName string,
	rootRelativePath string,
) (map[string]instanceBackupPackMetadata, error) {
	result := make(map[string]instanceBackupPackMetadata)
	if !utils.DirExists(rootPath) {
		return result, nil
	}
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, wrapInstanceBackupWriteTargetError(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		relativePath := path.Join(rootRelativePath, name)
		meta, ok := readInstanceBackupPackMetadata(filepath.Join(rootPath, name), rootName, relativePath)
		if !ok {
			continue
		}
		result[strings.ToLower(meta.UUID)] = meta
	}
	return result, nil
}

func findInstanceBackupPackManifestDir(dir string) string {
	root := strings.TrimSpace(dir)
	if root == "" {
		return ""
	}
	if utils.FileExists(filepath.Join(root, "manifest.json")) {
		return root
	}
	if !utils.DirExists(root) {
		return ""
	}

	queue := []string{root}
	seen := map[string]struct{}{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, ok := seen[current]; ok {
			continue
		}
		seen[current] = struct{}{}
		entries, err := os.ReadDir(current)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			child := filepath.Join(current, entry.Name())
			if utils.FileExists(filepath.Join(child, "manifest.json")) {
				return child
			}
			queue = append(queue, child)
		}
	}
	return ""
}

func readInstanceBackupPackMetadata(dir string, rootName string, relativePath string) (instanceBackupPackMetadata, bool) {
	manifestDir := findInstanceBackupPackManifestDir(dir)
	if manifestDir == "" {
		return instanceBackupPackMetadata{}, false
	}
	manifestPath := filepath.Join(manifestDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return instanceBackupPackMetadata{}, false
	}

	var raw packages.RawManifest
	if err := json.Unmarshal(utils.JsonCompatBytes(data), &raw); err != nil {
		return instanceBackupPackMetadata{}, false
	}
	uuid := strings.TrimSpace(raw.Header.UUID)
	if uuid == "" {
		return instanceBackupPackMetadata{}, false
	}

	packInfo := content.ReadPackInfoFromDir(dir)
	name := strings.TrimSpace(packInfo.Name)
	folderName := filepath.Base(strings.TrimSpace(dir))
	if name == "" {
		name = strings.TrimSpace(raw.Header.Name)
	}
	if name == "" {
		name = folderName
	}
	return instanceBackupPackMetadata{
		UUID:             uuid,
		Name:             name,
		Version:          firstNonEmptyString(strings.TrimSpace(packInfo.Version), formatInstanceBackupManifestVersion(raw.Header.Version)),
		MinEngineVersion: firstNonEmptyString(strings.TrimSpace(packInfo.MinEngineVersion), formatInstanceBackupManifestVersion(raw.Header.MinEngineVersion)),
		FolderName:       folderName,
		RootName:         strings.TrimSpace(rootName),
		RelativePath:     normalizeInstanceBackupRestoreConflictPath(relativePath, folderName),
		AbsolutePath:     filepath.Clean(strings.TrimSpace(dir)),
	}, true
}

func readInstanceBackupWorldMetadata(dir string, relativePath string) instanceBackupWorldMetadata {
	cleanDir := filepath.Clean(strings.TrimSpace(dir))
	folderName := filepath.Base(cleanDir)
	levelName := strings.TrimSpace(GetWorldLevelName(cleanDir))
	return instanceBackupWorldMetadata{
		FolderName:   folderName,
		LevelName:    levelName,
		RelativePath: normalizeInstanceBackupRestoreConflictPath(relativePath, folderName),
		AbsolutePath: cleanDir,
	}
}

func formatInstanceBackupManifestVersion(value interface{}) string {
	parts := packages.ParseVersion(value)
	if len(parts) == 0 {
		return ""
	}
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		segments = append(segments, fmt.Sprintf("%d", part))
	}
	return strings.Join(segments, ".")
}

type instanceBackupDiffCandidate struct {
	key     string
	label   string
	backup  string
	current string
}

func buildInstanceBackupDiffFields(candidates ...instanceBackupDiffCandidate) []types.InstanceBackupRestoreConflictDiffField {
	fields := make([]types.InstanceBackupRestoreConflictDiffField, 0, len(candidates))
	for _, candidate := range candidates {
		backupValue := strings.TrimSpace(candidate.backup)
		currentValue := strings.TrimSpace(candidate.current)
		if backupValue == currentValue {
			continue
		}
		if backupValue == "" && currentValue == "" {
			continue
		}
		fields = append(fields, types.InstanceBackupRestoreConflictDiffField{
			Key:          candidate.key,
			Label:        candidate.label,
			BackupValue:  backupValue,
			CurrentValue: currentValue,
		})
	}
	return fields
}

func buildInstanceBackupPackSummary(meta instanceBackupPackMetadata) string {
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = strings.TrimSpace(meta.FolderName)
	}
	version := strings.TrimSpace(meta.Version)
	if name == "" {
		return version
	}
	if version == "" {
		return name
	}
	return fmt.Sprintf("%s @ %s", name, version)
}

func buildInstanceBackupWorldSummary(meta instanceBackupWorldMetadata) string {
	levelName := strings.TrimSpace(meta.LevelName)
	folderName := strings.TrimSpace(meta.FolderName)
	if levelName == "" {
		return folderName
	}
	if folderName == "" || strings.EqualFold(levelName, folderName) {
		return levelName
	}
	return fmt.Sprintf("%s / %s", folderName, levelName)
}

func buildInstanceBackupModSummary(folderName string, manifest types.ModManifestJson, ok bool) string {
	name := strings.TrimSpace(folderName)
	if ok && strings.TrimSpace(manifest.Name) != "" {
		name = strings.TrimSpace(manifest.Name)
	}
	version := ""
	if ok {
		version = strings.TrimSpace(manifest.Version)
	}
	if name == "" {
		return version
	}
	if version == "" {
		return name
	}
	return fmt.Sprintf("%s @ %s", name, version)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildInstanceBackupFileConflict(
	scopeKey string,
	scopeLabel string,
	relativePath string,
	sourceInfo os.FileInfo,
	targetInfo os.FileInfo,
) types.InstanceBackupRestoreConflict {
	normalizedPath := normalizeInstanceBackupRestoreConflictPath(relativePath, "")
	return types.InstanceBackupRestoreConflict{
		ID:             buildInstanceBackupRestoreConflictID(scopeKey, normalizedPath),
		ScopeKey:       scopeKey,
		ScopeLabel:     scopeLabel,
		Path:           normalizedPath,
		SourceType:     instanceBackupEntryKind(sourceInfo),
		TargetType:     instanceBackupEntryKind(targetInfo),
		IdentityKind:   "file_path",
		IdentityKey:    normalizedPath,
		BackupPath:     normalizedPath,
		CurrentPath:    normalizedPath,
		BackupSummary:  normalizedPath,
		CurrentSummary: normalizedPath,
	}
}

func buildInstanceBackupPackConflict(
	scopeLabel string,
	sourceMeta instanceBackupPackMetadata,
	currentMeta *instanceBackupPackMetadata,
	currentPath string,
	targetInfo os.FileInfo,
	rootRelativePath string,
) types.InstanceBackupRestoreConflict {
	currentRelativePath := path.Join(rootRelativePath, strings.TrimSpace(filepath.Base(currentPath)))
	if currentMeta != nil && strings.TrimSpace(currentMeta.RelativePath) != "" {
		currentRelativePath = currentMeta.RelativePath
	}
	displayPath := sourceMeta.RelativePath
	if currentRelativePath != "" && currentRelativePath != sourceMeta.RelativePath {
		displayPath = fmt.Sprintf("%s <-> %s", sourceMeta.RelativePath, currentRelativePath)
	}

	currentSummary := strings.TrimSpace(filepath.Base(currentPath))
	currentUUID := ""
	currentName := ""
	currentVersion := ""
	currentMinEngineVersion := ""
	currentFolder := strings.TrimSpace(filepath.Base(currentPath))
	if currentMeta != nil {
		currentSummary = buildInstanceBackupPackSummary(*currentMeta)
		currentUUID = currentMeta.UUID
		currentName = currentMeta.Name
		currentVersion = currentMeta.Version
		currentMinEngineVersion = currentMeta.MinEngineVersion
		currentFolder = currentMeta.FolderName
	}

	return types.InstanceBackupRestoreConflict{
		ID:             buildInstanceBackupRestoreConflictEntityID(instanceBackupScopeGameData, "pack_uuid", path.Join(rootRelativePath, sourceMeta.UUID), sourceMeta.RelativePath),
		ScopeKey:       instanceBackupScopeGameData,
		ScopeLabel:     scopeLabel,
		Path:           displayPath,
		SourceType:     "dir",
		TargetType:     instanceBackupEntryKind(targetInfo),
		IdentityKind:   "pack_uuid",
		IdentityKey:    sourceMeta.UUID,
		BackupPath:     sourceMeta.RelativePath,
		CurrentPath:    currentRelativePath,
		BackupSummary:  buildInstanceBackupPackSummary(sourceMeta),
		CurrentSummary: currentSummary,
		DiffFields: buildInstanceBackupDiffFields(
			instanceBackupDiffCandidate{key: "folder", label: "versions.edit.backup.restore.diff_field.folder", backup: sourceMeta.FolderName, current: currentFolder},
			instanceBackupDiffCandidate{key: "uuid", label: "versions.edit.backup.restore.diff_field.uuid", backup: sourceMeta.UUID, current: currentUUID},
			instanceBackupDiffCandidate{key: "name", label: "versions.edit.backup.restore.diff_field.name", backup: sourceMeta.Name, current: currentName},
			instanceBackupDiffCandidate{key: "version", label: "versions.edit.backup.restore.diff_field.version", backup: sourceMeta.Version, current: currentVersion},
			instanceBackupDiffCandidate{key: "min_engine_version", label: "versions.edit.backup.restore.diff_field.min_engine_version", backup: sourceMeta.MinEngineVersion, current: currentMinEngineVersion},
		),
	}
}

func buildInstanceBackupWorldConflict(
	scopeLabel string,
	sourceMeta instanceBackupWorldMetadata,
	currentMeta instanceBackupWorldMetadata,
	targetInfo os.FileInfo,
) types.InstanceBackupRestoreConflict {
	return types.InstanceBackupRestoreConflict{
		ID:             buildInstanceBackupRestoreConflictEntityID(instanceBackupScopeGameData, "world_folder", sourceMeta.RelativePath, sourceMeta.RelativePath),
		ScopeKey:       instanceBackupScopeGameData,
		ScopeLabel:     scopeLabel,
		Path:           sourceMeta.RelativePath,
		SourceType:     "dir",
		TargetType:     instanceBackupEntryKind(targetInfo),
		IdentityKind:   "world_folder",
		IdentityKey:    sourceMeta.FolderName,
		BackupPath:     sourceMeta.RelativePath,
		CurrentPath:    currentMeta.RelativePath,
		BackupSummary:  buildInstanceBackupWorldSummary(sourceMeta),
		CurrentSummary: buildInstanceBackupWorldSummary(currentMeta),
		DiffFields: buildInstanceBackupDiffFields(
			instanceBackupDiffCandidate{key: "level_name", label: "versions.edit.backup.restore.diff_field.level_name", backup: sourceMeta.LevelName, current: currentMeta.LevelName},
		),
	}
}

func buildInstanceBackupModConflict(
	relativePath string,
	sourceManifest types.ModManifestJson,
	sourceOK bool,
	targetManifest types.ModManifestJson,
	targetOK bool,
	targetInfo os.FileInfo,
) types.InstanceBackupRestoreConflict {
	folderName := strings.TrimSpace(filepath.Base(relativePath))
	return types.InstanceBackupRestoreConflict{
		ID:             buildInstanceBackupRestoreConflictEntityID(instanceBackupScopeMods, "mod_folder", relativePath, relativePath),
		ScopeKey:       instanceBackupScopeMods,
		ScopeLabel:     instanceBackupScopeMods,
		Path:           relativePath,
		SourceType:     "dir",
		TargetType:     instanceBackupEntryKind(targetInfo),
		IdentityKind:   "mod_folder",
		IdentityKey:    relativePath,
		BackupPath:     relativePath,
		CurrentPath:    relativePath,
		BackupSummary:  buildInstanceBackupModSummary(folderName, sourceManifest, sourceOK),
		CurrentSummary: buildInstanceBackupModSummary(folderName, targetManifest, targetOK),
		DiffFields: buildInstanceBackupDiffFields(
			instanceBackupDiffCandidate{key: "name", label: "versions.edit.backup.restore.diff_field.name", backup: sourceManifest.Name, current: targetManifest.Name},
			instanceBackupDiffCandidate{key: "version", label: "versions.edit.backup.restore.diff_field.version", backup: sourceManifest.Version, current: targetManifest.Version},
			instanceBackupDiffCandidate{key: "entry", label: "versions.edit.backup.restore.diff_field.entry", backup: sourceManifest.Entry, current: targetManifest.Entry},
			instanceBackupDiffCandidate{key: "type", label: "versions.edit.backup.restore.diff_field.type", backup: sourceManifest.Type, current: targetManifest.Type},
		),
	}
}

func normalizeInstanceBackupRestoreConflictResolutions(
	resolutions []types.InstanceBackupRestoreResolution,
) (map[string]string, bool) {
	normalized := make(map[string]string, len(resolutions))
	for _, item := range resolutions {
		conflictID := strings.TrimSpace(item.ConflictID)
		choice := strings.TrimSpace(item.Choice)
		if conflictID == "" {
			return nil, false
		}
		if choice != instanceBackupRestoreChoiceBackup && choice != instanceBackupRestoreChoiceCurrent {
			return nil, false
		}
		normalized[conflictID] = choice
	}
	return normalized, true
}

func normalizeInstanceBackupRestoreConflictPath(relativePath string, fallback string) string {
	normalized := normalizeInstanceBackupArchiveName(relativePath)
	if normalized != "" {
		return normalized
	}
	return normalizeInstanceBackupArchiveName(fallback)
}

func buildInstanceBackupRestoreConflictID(scopeKey string, relativePath string) string {
	return buildInstanceBackupRestoreConflictEntityID(scopeKey, "file_path", relativePath, relativePath)
}

func buildInstanceBackupRestoreConflictEntityID(
	scopeKey string,
	identityKind string,
	identityRef string,
	relativePath string,
) string {
	return fmt.Sprintf(
		"%s::%s::%s::%s",
		strings.TrimSpace(scopeKey),
		strings.TrimSpace(identityKind),
		normalizeInstanceBackupRestoreConflictPath(identityRef, "identity"),
		normalizeInstanceBackupRestoreConflictPath(relativePath, "entry"),
	)
}

func instanceBackupEntryKind(info os.FileInfo) string {
	if info != nil && info.IsDir() {
		return "dir"
	}
	return "file"
}

func resolveInstanceBackupRestoreConflictChoiceByID(conflictID string, resolutionByID map[string]string) (string, bool) {
	if len(resolutionByID) == 0 {
		return "", false
	}
	choice, ok := resolutionByID[strings.TrimSpace(conflictID)]
	return choice, ok
}

func sameInstanceBackupPath(left string, right string) bool {
	leftAbs, _ := filepath.Abs(strings.TrimSpace(left))
	rightAbs, _ := filepath.Abs(strings.TrimSpace(right))
	if leftAbs == "" || rightAbs == "" {
		return false
	}
	return strings.EqualFold(leftAbs, rightAbs)
}

func restoreInstanceBackupPathChoice(
	scopeKey string,
	sourcePath string,
	destPath string,
	relativePath string,
	resolutionByID map[string]string,
) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	_, err = os.Stat(destPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return copyEntryExact(sourcePath, destPath)
		}
		return wrapInstanceBackupWriteTargetError(err)
	}
	conflictID := buildInstanceBackupRestoreConflictID(
		scopeKey,
		normalizeInstanceBackupRestoreConflictPath(relativePath, sourceInfo.Name()),
	)
	choice, ok := resolveInstanceBackupRestoreConflictChoiceByID(conflictID, resolutionByID)
	if !ok {
		return wrapInstanceBackupWriteTargetError(os.ErrInvalid)
	}
	if choice == instanceBackupRestoreChoiceCurrent {
		return nil
	}
	if err := os.RemoveAll(destPath); err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	return copyEntryExact(sourcePath, destPath)
}

func restoreInstanceBackupGameData(
	tempDir string,
	archive instanceBackupArchive,
	target instanceBackupContext,
	resolutionByID map[string]string,
) types.InstanceBackupRestoreScopeResult {
	result := types.InstanceBackupRestoreScopeResult{
		Key:    instanceBackupScopeGameData,
		Label:  instanceBackupScopeLabel(instanceBackupScopeGameData, archive.info.Type),
		Mode:   strings.TrimSpace(archive.manifest.ScopeModes[instanceBackupScopeGameData]),
		Status: "failed",
	}
	targetRoot := strings.TrimSpace(target.gameDataPath)
	if targetRoot == "" {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_GAME_DATA_NOT_FOUND"
		return result
	}

	sourceRoot := filepath.Join(tempDir, filepath.FromSlash(archive.gameDataRoot))
	if !utils.DirExists(sourceRoot) {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_INVALID_ARCHIVE"
		return result
	}

	if result.Mode == instanceBackupModeFull {
		if len(resolutionByID) == 0 && dirHasEntries(targetRoot) {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_FULL_REQUIRES_EMPTY_TARGET"
			return result
		}
		if err := utils.CreateDir(targetRoot); err != nil {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
			return result
		}
		var err error
		if len(resolutionByID) > 0 {
			err = restoreInstanceBackupGameDataWithResolutions(sourceRoot, targetRoot, "", resolutionByID)
		} else {
			err = copyDirContents(sourceRoot, targetRoot)
		}
		if err != nil {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
			return result
		}
		result.Status = "success"
		return result
	}

	var err error
	if len(resolutionByID) > 0 {
		err = restoreInstanceBackupGameDataWithResolutions(sourceRoot, targetRoot, "", resolutionByID)
	} else {
		err = restoreInstanceBackupSafeGameData(sourceRoot, targetRoot)
	}
	if err != nil {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
		return result
	}
	result.Status = "success"
	return result
}

func restoreInstanceBackupSafeGameData(sourceRoot string, targetRoot string) error {
	usersRoot := filepath.Join(sourceRoot, "Users")
	if !utils.DirExists(usersRoot) {
		return nil
	}
	userEntries, err := os.ReadDir(usersRoot)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	for _, userEntry := range userEntries {
		if !userEntry.IsDir() {
			continue
		}
		userName := strings.TrimSpace(userEntry.Name())
		if userName == "" {
			continue
		}
		for _, rootName := range instanceBackupSafeBedrockRoots {
			srcRoot := filepath.Join(usersRoot, userName, "games", "com.mojang", rootName)
			if !utils.FileExists(srcRoot) && !utils.DirExists(srcRoot) {
				continue
			}
			destRoot := filepath.Join(targetRoot, "Users", userName, "games", "com.mojang", rootName)
			if err := restoreInstanceBackupSafeRoot(srcRoot, destRoot, rootName); err != nil {
				return err
			}
		}
	}
	return nil
}

func restoreInstanceBackupSafeGameDataWithResolutions(sourceRoot string, targetRoot string, resolutionByID map[string]string) error {
	return restoreInstanceBackupGameDataWithResolutions(sourceRoot, targetRoot, "", resolutionByID)
}

func restoreInstanceBackupGameDataWithResolutions(
	sourcePath string,
	targetPath string,
	relativePath string,
	resolutionByID map[string]string,
) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return copyEntryExact(sourcePath, targetPath)
		}
		return wrapInstanceBackupWriteTargetError(err)
	}

	normalizedPath := ""
	if strings.TrimSpace(relativePath) != "" {
		normalizedPath = normalizeInstanceBackupRestoreConflictPath(relativePath, sourceInfo.Name())
	}
	if sourceInfo.IsDir() && targetInfo.IsDir() {
		if rootName, ok := matchInstanceBackupGameDataPackRoot(normalizedPath); ok {
			return restoreInstanceBackupPackRootWithResolutions(
				sourcePath,
				targetPath,
				normalizedPath,
				rootName,
				resolutionByID,
			)
		}
		if isInstanceBackupGameDataWorldRoot(normalizedPath) {
			return restoreInstanceBackupWorldRootWithResolutions(
				sourcePath,
				targetPath,
				normalizedPath,
				resolutionByID,
			)
		}
		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return wrapInstanceBackupWriteTargetError(err)
		}
		for _, entry := range entries {
			name := strings.TrimSpace(entry.Name())
			if name == "" {
				continue
			}
			if err := restoreInstanceBackupGameDataWithResolutions(
				filepath.Join(sourcePath, name),
				filepath.Join(targetPath, name),
				path.Join(normalizedPath, name),
				resolutionByID,
			); err != nil {
				return err
			}
		}
		return nil
	}

	return restoreInstanceBackupPathChoice(
		instanceBackupScopeGameData,
		sourcePath,
		targetPath,
		normalizedPath,
		resolutionByID,
	)
}

func restoreInstanceBackupWorldRootWithResolutions(
	sourceRoot string,
	targetRoot string,
	rootRelativePath string,
	resolutionByID map[string]string,
) error {
	if !utils.DirExists(targetRoot) {
		return copyEntryExact(sourceRoot, targetRoot)
	}
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		sourcePath := filepath.Join(sourceRoot, name)
		targetPath := filepath.Join(targetRoot, name)
		relativePath := path.Join(rootRelativePath, name)
		if !entry.IsDir() {
			if err := restoreInstanceBackupPathChoice(
				instanceBackupScopeGameData,
				sourcePath,
				targetPath,
				relativePath,
				resolutionByID,
			); err != nil {
				return err
			}
			continue
		}

		if _, err := os.Stat(targetPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if err := copyEntryExact(sourcePath, targetPath); err != nil {
					return err
				}
				continue
			}
			return wrapInstanceBackupWriteTargetError(err)
		}

		choice, ok := resolveInstanceBackupRestoreConflictChoiceByID(
			buildInstanceBackupRestoreConflictEntityID(instanceBackupScopeGameData, "world_folder", relativePath, relativePath),
			resolutionByID,
		)
		if !ok {
			return wrapInstanceBackupWriteTargetError(os.ErrInvalid)
		}
		if choice == instanceBackupRestoreChoiceCurrent {
			continue
		}
		if err := os.RemoveAll(targetPath); err != nil {
			return wrapInstanceBackupWriteTargetError(err)
		}
		if err := copyEntryExact(sourcePath, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func restoreInstanceBackupPackRootWithResolutions(
	sourceRoot string,
	targetRoot string,
	rootRelativePath string,
	rootName string,
	resolutionByID map[string]string,
) error {
	if !utils.DirExists(targetRoot) {
		return copyEntryExact(sourceRoot, targetRoot)
	}
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		sourcePath := filepath.Join(sourceRoot, name)
		targetPath := filepath.Join(targetRoot, name)
		relativePath := path.Join(rootRelativePath, name)
		if !entry.IsDir() {
			if err := restoreInstanceBackupPathChoice(
				instanceBackupScopeGameData,
				sourcePath,
				targetPath,
				relativePath,
				resolutionByID,
			); err != nil {
				return err
			}
			continue
		}

		sourceMeta, ok := readInstanceBackupPackMetadata(sourcePath, rootName, relativePath)
		if !ok {
			if err := restoreInstanceBackupGameDataWithResolutions(
				sourcePath,
				targetPath,
				relativePath,
				resolutionByID,
			); err != nil {
				return err
			}
			continue
		}
		if err := restoreInstanceBackupPackEntityWithResolutions(
			sourceMeta,
			targetRoot,
			rootRelativePath,
			rootName,
			resolutionByID,
		); err != nil {
			return err
		}
	}
	return nil
}

func restoreInstanceBackupPackEntityWithResolutions(
	sourceMeta instanceBackupPackMetadata,
	targetRoot string,
	rootRelativePath string,
	rootName string,
	resolutionByID map[string]string,
) error {
	targetByUUID, err := scanInstanceBackupPackRootByUUID(targetRoot, rootName, rootRelativePath)
	if err != nil {
		return err
	}

	destPath := filepath.Join(targetRoot, sourceMeta.FolderName)
	currentPath := ""
	if matched, ok := targetByUUID[strings.ToLower(sourceMeta.UUID)]; ok {
		currentPath = matched.AbsolutePath
	}
	if currentPath == "" {
		if _, err := os.Stat(destPath); err == nil {
			currentPath = destPath
		} else if !errors.Is(err, os.ErrNotExist) {
			return wrapInstanceBackupWriteTargetError(err)
		}
	}
	if currentPath == "" {
		return copyEntryExact(sourceMeta.AbsolutePath, destPath)
	}

	conflictID := buildInstanceBackupRestoreConflictEntityID(
		instanceBackupScopeGameData,
		"pack_uuid",
		path.Join(rootRelativePath, sourceMeta.UUID),
		sourceMeta.RelativePath,
	)
	choice, ok := resolveInstanceBackupRestoreConflictChoiceByID(conflictID, resolutionByID)
	if !ok {
		return wrapInstanceBackupWriteTargetError(os.ErrInvalid)
	}
	if choice == instanceBackupRestoreChoiceCurrent {
		return nil
	}
	if err := os.RemoveAll(currentPath); err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	if !sameInstanceBackupPath(currentPath, destPath) && (utils.DirExists(destPath) || utils.FileExists(destPath)) {
		if err := os.RemoveAll(destPath); err != nil {
			return wrapInstanceBackupWriteTargetError(err)
		}
	}
	return copyEntryExact(sourceMeta.AbsolutePath, destPath)
}

func restoreInstanceBackupSafeRoot(sourceRoot string, destRoot string, rootName string) error {
	info, err := os.Stat(sourceRoot)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	if !info.IsDir() {
		if err := utils.CreateDir(filepath.Dir(destRoot)); err != nil {
			return wrapInstanceBackupWriteTargetError(err)
		}
		_, err := copyEntryWithRename(sourceRoot, filepath.Dir(destRoot))
		return err
	}

	if err := utils.CreateDir(destRoot); err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	children, err := os.ReadDir(sourceRoot)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	for _, child := range children {
		srcPath := filepath.Join(sourceRoot, child.Name())
		if rootName == "minecraftWorlds" && child.IsDir() {
			if _, err := restoreInstanceBackupWorldDir(srcPath, destRoot); err != nil {
				return err
			}
			continue
		}
		if _, err := copyEntryWithRename(srcPath, destRoot); err != nil {
			return err
		}
	}
	return nil
}

func restoreInstanceBackupWorldDir(sourceWorldDir string, worldsRoot string) (string, error) {
	if err := utils.CreateDir(worldsRoot); err != nil {
		return "", wrapInstanceBackupWriteTargetError(err)
	}
	sourceFolderName := filepath.Base(sourceWorldDir)
	targetFolderName := sourceFolderName
	if utils.DirExists(filepath.Join(worldsRoot, targetFolderName)) {
		targetFolderName = uniqueEntryName(worldsRoot, sourceFolderName, true)
	}
	targetWorldDir := filepath.Join(worldsRoot, targetFolderName)
	if err := utils.CopyDir(sourceWorldDir, targetWorldDir); err != nil {
		return "", wrapInstanceBackupWriteTargetError(err)
	}
	return targetFolderName, nil
}

func restoreInstanceBackupMods(
	ctx context.Context,
	targetName string,
	tempDir string,
	archive instanceBackupArchive,
	target instanceBackupContext,
	resolutionByID map[string]string,
) types.InstanceBackupRestoreScopeResult {
	result := types.InstanceBackupRestoreScopeResult{
		Key:    instanceBackupScopeMods,
		Label:  instanceBackupScopeMods,
		Status: "failed",
	}
	if err := utils.CreateDir(target.modsPath); err != nil {
		result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
		return result
	}

	successUnits := 0
	sourceRawRoot := filepath.Join(tempDir, filepath.FromSlash(archive.modsRawRoot))
	if utils.DirExists(sourceRawRoot) {
		rawEntries, err := os.ReadDir(sourceRawRoot)
		if err != nil {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
			return result
		}
		for _, entry := range rawEntries {
			var restoreErr error
			if len(resolutionByID) > 0 {
				sourcePath := filepath.Join(sourceRawRoot, entry.Name())
				targetPath := filepath.Join(target.modsPath, entry.Name())
				if entry.IsDir() {
					if _, err := os.Stat(targetPath); err != nil {
						if errors.Is(err, os.ErrNotExist) {
							restoreErr = copyEntryExact(sourcePath, targetPath)
						} else {
							restoreErr = wrapInstanceBackupWriteTargetError(err)
						}
					} else {
						conflictID := buildInstanceBackupRestoreConflictEntityID(
							instanceBackupScopeMods,
							"mod_folder",
							entry.Name(),
							entry.Name(),
						)
						choice, ok := resolveInstanceBackupRestoreConflictChoiceByID(conflictID, resolutionByID)
						if !ok {
							restoreErr = wrapInstanceBackupWriteTargetError(os.ErrInvalid)
						} else if choice == instanceBackupRestoreChoiceBackup {
							if err := os.RemoveAll(targetPath); err != nil {
								restoreErr = wrapInstanceBackupWriteTargetError(err)
							} else {
								restoreErr = copyEntryExact(sourcePath, targetPath)
							}
						}
					}
				} else {
					restoreErr = restoreInstanceBackupPathChoice(
						instanceBackupScopeMods,
						sourcePath,
						targetPath,
						entry.Name(),
						resolutionByID,
					)
				}
			} else {
				_, restoreErr = restoreInstanceBackupRawModEntry(filepath.Join(sourceRawRoot, entry.Name()), target.modsPath)
			}
			if restoreErr != nil {
				result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
				return result
			}
			successUnits++
		}
	}

	lipPackages := cloneInstanceBackupModsLIPPackages(archive.manifest.LIPPackages)
	if len(lipPackages) == 0 {
		result.Status = "success"
		return result
	}

	if supported, definitive := instanceBackupSupportsLeviLamina(target.gameVersion); definitive && !supported {
		result.Warnings = []string{"ERR_LL_NOT_SUPPORTED"}
		result.Details = formatInstanceBackupLIPPackageDetails(lipPackages)
		result.Status = "partial"
		return result
	}

	if !lipIsInstalled() {
		result.Warnings = []string{"ERR_LIP_NOT_INSTALLED"}
		result.Details = formatInstanceBackupLIPPackageDetails(lipPackages)
		if successUnits > 0 {
			result.Status = "partial"
			return result
		}
		result.ErrorCode = "ERR_LIP_NOT_INSTALLED"
		return result
	}

	explicitPackages := make([]types.InstanceBackupModsLIPPackage, 0, len(lipPackages))
	for _, item := range lipPackages {
		if item.ExplicitInstalled {
			explicitPackages = append(explicitPackages, item)
		}
	}
	sort.Slice(explicitPackages, func(i, j int) bool {
		return strings.ToLower(explicitPackages[i].Identifier) < strings.ToLower(explicitPackages[j].Identifier)
	})

	failedInstalls := make([]string, 0)
	for _, item := range explicitPackages {
		if errCode := InstallLIPPackage(ctx, targetName, item.Identifier, item.Version); errCode != "" {
			failedInstalls = append(failedInstalls, fmt.Sprintf("%s@%s (%s)", item.Identifier, item.Version, errCode))
			continue
		}
		successUnits++
	}
	if len(failedInstalls) > 0 {
		result.Warnings = append(result.Warnings, "ERR_INSTANCE_BACKUP_RESTORE_LIP_INSTALL_FAILED")
		result.Details = append(result.Details, failedInstalls...)
	}

	missingPackages := make([]string, 0)
	for _, item := range lipPackages {
		state := GetLIPPackageInstallState(ctx, targetName, item.Identifier)
		if !state.Installed {
			missingPackages = append(missingPackages, fmt.Sprintf("%s@%s", item.Identifier, item.Version))
			continue
		}
		if compareInstanceBackupVersion(state.InstalledVersion, item.Version) != 0 {
			result.Warnings = append(result.Warnings, "ERR_INSTANCE_BACKUP_RESTORE_LIP_VERSION_MISMATCH")
			result.Details = append(result.Details, fmt.Sprintf("%s: expected %s, got %s", item.Identifier, item.Version, strings.TrimSpace(state.InstalledVersion)))
		}
	}
	if len(missingPackages) > 0 {
		result.Warnings = append(result.Warnings, "ERR_INSTANCE_BACKUP_RESTORE_LIP_MISSING")
		result.Details = append(result.Details, missingPackages...)
	}

	result.Warnings = uniqueStrings(result.Warnings)
	result.Details = uniqueStrings(result.Details)
	switch {
	case len(result.Warnings) == 0:
		result.Status = "success"
	case successUnits > 0:
		result.Status = "partial"
	default:
		result.Status = "failed"
		if len(failedInstalls) > 0 {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_LIP_INSTALL_FAILED"
		} else {
			result.ErrorCode = "ERR_INSTANCE_BACKUP_RESTORE_LIP_MISSING"
		}
	}
	return result
}

func instanceBackupSupportsLeviLamina(mcVersion string) (supported bool, definitive bool) {
	trimmedVersion := strings.TrimSpace(mcVersion)
	if trimmedVersion == "" {
		return true, false
	}
	if len(strings.Split(trimmedVersion, ".")) < 3 {
		return true, false
	}

	db, err := instanceBackupFetchLeviLaminaVersionDB()
	if err != nil {
		log.Printf("instance backup restore: failed to resolve LeviLamina support for %s: %v", trimmedVersion, err)
		return true, false
	}
	return len(resolveSupportedLeviLaminaVersions(db, trimmedVersion)) > 0, true
}

func readInstanceBackupZipFile(files []*zip.File, name string) ([]byte, error) {
	for _, file := range files {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, os.ErrNotExist
}

func extractInstanceBackupArchive(archivePath string, destDir string) error {
	reader, err := zip.OpenReader(strings.TrimSpace(archivePath))
	if err != nil {
		return wrapInstanceBackupExtractError(err)
	}
	defer reader.Close()

	if err := utils.CreateDir(destDir); err != nil {
		return wrapInstanceBackupExtractError(err)
	}
	safeRoot, _ := filepath.Abs(destDir)
	safeRoot = strings.ToLower(strings.TrimRight(safeRoot, string(os.PathSeparator)))

	for _, file := range reader.File {
		name := normalizeInstanceBackupZipEntryName(file.Name)
		if name == "" {
			continue
		}
		targetPath := filepath.Join(destDir, filepath.FromSlash(name))
		safeTarget, _ := filepath.Abs(targetPath)
		safeTarget = strings.ToLower(safeTarget)
		if safeTarget != safeRoot && !strings.HasPrefix(safeTarget, safeRoot+string(os.PathSeparator)) {
			return wrapInstanceBackupExtractError(os.ErrInvalid)
		}
		if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return wrapInstanceBackupExtractError(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return wrapInstanceBackupExtractError(err)
		}
		rc, err := file.Open()
		if err != nil {
			return wrapInstanceBackupExtractError(err)
		}
		mode := file.Mode()
		if mode == 0 {
			mode = 0o644
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			rc.Close()
			return wrapInstanceBackupExtractError(err)
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return wrapInstanceBackupExtractError(err)
		}
		if err := out.Close(); err != nil {
			rc.Close()
			return wrapInstanceBackupExtractError(err)
		}
		if err := rc.Close(); err != nil {
			return wrapInstanceBackupExtractError(err)
		}
	}
	return nil
}

func copyDirContents(src string, dst string) error {
	if err := utils.CreateDir(dst); err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	for _, entry := range entries {
		if err := copyEntryExact(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func restoreInstanceBackupRawModEntry(src string, destDir string) (string, error) {
	if err := utils.CreateDir(destDir); err != nil {
		return "", wrapInstanceBackupWriteTargetError(err)
	}
	info, err := os.Stat(src)
	if err != nil {
		return "", wrapInstanceBackupWriteTargetError(err)
	}
	if !info.IsDir() {
		return copyEntryWithRename(src, destDir)
	}

	targetName := info.Name()
	targetPath := filepath.Join(destDir, targetName)
	if !utils.DirExists(targetPath) && !utils.FileExists(targetPath) {
		if err := copyEntryExact(src, targetPath); err != nil {
			return "", err
		}
		return targetName, nil
	}

	sourceManifest, sourceOK := readInstanceBackupModManifest(src)
	targetManifest, targetOK := readInstanceBackupModManifest(targetPath)
	if sourceOK && targetOK && instanceBackupModManifestMatches(sourceManifest, targetManifest) {
		if err := os.RemoveAll(targetPath); err != nil {
			return "", wrapInstanceBackupWriteTargetError(err)
		}
		if err := copyEntryExact(src, targetPath); err != nil {
			return "", err
		}
		return targetName, nil
	}

	return copyEntryWithRename(src, destDir)
}

func copyEntryWithRename(src string, destDir string) (string, error) {
	if err := utils.CreateDir(destDir); err != nil {
		return "", wrapInstanceBackupWriteTargetError(err)
	}
	info, err := os.Stat(src)
	if err != nil {
		return "", wrapInstanceBackupWriteTargetError(err)
	}
	targetName := info.Name()
	if utils.FileExists(filepath.Join(destDir, targetName)) || utils.DirExists(filepath.Join(destDir, targetName)) {
		targetName = uniqueEntryName(destDir, targetName, info.IsDir())
	}
	if err := copyEntryExact(src, filepath.Join(destDir, targetName)); err != nil {
		return "", err
	}
	return targetName, nil
}

func copyEntryExact(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	if info.IsDir() {
		if err := utils.CopyDir(src, dst); err != nil {
			return wrapInstanceBackupWriteTargetError(err)
		}
		return nil
	}
	return copyFileToTarget(src, dst, info.Mode())
}

func copyFileToTarget(src string, dst string, mode os.FileMode) error {
	if err := utils.CreateDir(filepath.Dir(dst)); err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	in, err := os.Open(src)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	defer in.Close()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return wrapInstanceBackupWriteTargetError(err)
	}
	if err := out.Close(); err != nil {
		return wrapInstanceBackupWriteTargetError(err)
	}
	return nil
}

func readInstanceBackupModManifest(dir string) (types.ModManifestJson, bool) {
	root := strings.TrimSpace(dir)
	if root == "" || !utils.DirExists(root) {
		return types.ModManifestJson{}, false
	}

	for _, fileName := range []string{"manifest.json", "manifest.json.close"} {
		path := filepath.Join(root, fileName)
		if !utils.FileExists(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var manifest types.ModManifestJson
		if err := json.Unmarshal(utils.JsonCompatBytes(data), &manifest); err != nil {
			continue
		}
		return manifest, true
	}

	return types.ModManifestJson{}, false
}

func instanceBackupModManifestMatches(left types.ModManifestJson, right types.ModManifestJson) bool {
	compare := func(a string, b string) (matched bool, ok bool) {
		ta := strings.TrimSpace(a)
		tb := strings.TrimSpace(b)
		if ta == "" || tb == "" {
			return false, false
		}
		return strings.EqualFold(ta, tb), true
	}

	strongMatch := false
	for _, pair := range [][2]string{
		{left.Name, right.Name},
		{left.Entry, right.Entry},
	} {
		matched, ok := compare(pair[0], pair[1])
		if !ok {
			continue
		}
		if !matched {
			return false
		}
		strongMatch = true
	}
	if !strongMatch {
		return false
	}

	for _, pair := range [][2]string{
		{left.Type, right.Type},
		{left.Author, right.Author},
	} {
		matched, ok := compare(pair[0], pair[1])
		if ok && !matched {
			return false
		}
	}

	return true
}

func uniqueEntryName(parentDir string, name string, isDir bool) string {
	candidate := strings.TrimSpace(name)
	if candidate == "" {
		if isDir {
			candidate = "item"
		} else {
			candidate = "file"
		}
	}
	if !utils.FileExists(filepath.Join(parentDir, candidate)) && !utils.DirExists(filepath.Join(parentDir, candidate)) {
		return candidate
	}
	base := candidate
	ext := ""
	if !isDir {
		ext = filepath.Ext(candidate)
		base = strings.TrimSuffix(candidate, ext)
	}
	for i := 2; ; i++ {
		next := fmt.Sprintf("%s (%d)", base, i)
		if ext != "" {
			next += ext
		}
		if !utils.FileExists(filepath.Join(parentDir, next)) && !utils.DirExists(filepath.Join(parentDir, next)) {
			return next
		}
	}
}

func formatInstanceBackupLIPPackageDetails(items []types.InstanceBackupModsLIPPackage) []string {
	details := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Identifier) == "" || strings.TrimSpace(item.Version) == "" {
			continue
		}
		details = append(details, fmt.Sprintf("%s@%s", item.Identifier, item.Version))
	}
	return details
}

func compareInstanceBackupVersion(left string, right string) int {
	normalize := func(value string) string {
		return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "v")
	}
	lv := normalize(left)
	rv := normalize(right)
	switch {
	case lv == rv:
		return 0
	case lv < rv:
		return -1
	default:
		return 1
	}
}

func dirHasEntries(path string) bool {
	dir := strings.TrimSpace(path)
	if dir == "" || !utils.DirExists(dir) {
		return false
	}
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func normalizeInstanceBackupArchiveName(name string) string {
	return strings.Trim(filepath.ToSlash(strings.TrimSpace(name)), "/")
}

func normalizeInstanceBackupZipEntryName(name string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(name), `\`, "/")
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return ""
	}
	clean := path.Clean(normalized)
	if clean == "." || clean == "/" || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	return clean
}

func wrapInstanceBackupReadError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errInstanceBackupReadSource, err)
}

func wrapInstanceBackupWriteError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errInstanceBackupWriteArchive, err)
}

func wrapInstanceBackupInvalidError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errInstanceBackupInvalid, err)
}

func wrapInstanceBackupExtractError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errInstanceBackupExtract, err)
}

func wrapInstanceBackupWriteTargetError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errInstanceBackupWriteTarget, err)
}

func mapInstanceBackupErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, os.ErrNotExist) {
		return "ERR_INSTANCE_BACKUP_GAME_DATA_NOT_FOUND"
	}
	if errors.Is(err, errInstanceBackupReadSource) {
		return "ERR_INSTANCE_BACKUP_READ_SOURCE"
	}
	return "ERR_INSTANCE_BACKUP_WRITE_ARCHIVE"
}

func mapInstanceBackupArchiveErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, errInstanceBackupInvalid) {
		return "ERR_INSTANCE_BACKUP_INVALID_ARCHIVE"
	}
	return "ERR_INSTANCE_BACKUP_ARCHIVE_OPEN"
}

func mapInstanceBackupRestoreErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, errInstanceBackupExtract) {
		return "ERR_INSTANCE_BACKUP_RESTORE_EXTRACT_ARCHIVE"
	}
	if errors.Is(err, errInstanceBackupWriteTarget) {
		return "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
	}
	return "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
}

func sanitizeInstanceBackupSegment(name string, fallback string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fallback
	}
	const invalid = "<>:\"/\\|?*"
	var builder strings.Builder
	for _, r := range trimmed {
		if r < 32 || strings.ContainsRune(invalid, r) {
			builder.WriteRune('_')
			continue
		}
		builder.WriteRune(r)
	}
	cleaned := strings.TrimRight(builder.String(), " .")
	if strings.TrimSpace(cleaned) == "" {
		return fallback
	}
	return cleaned
}

func copyStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	dedup := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := dedup[trimmed]; ok {
			continue
		}
		dedup[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
