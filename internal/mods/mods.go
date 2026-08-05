package mods

import (
	"archive/zip"
	"bytes"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func GetMods(mcname string) (result []types.ModInfo) {
	name := strings.TrimSpace(mcname)
	if name == "" {
		return result
	}
	vroot, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vroot) == "" {
		return result
	}
	root := filepath.Join(vroot, name)
	modsDir := filepath.Join(root, "mods")
	if !utils.DirExists(modsDir) {
		_ = os.MkdirAll(modsDir, 0755)
	}
	modDirs := utils.GetDirNames(modsDir)
	for _, modDir := range modDirs {
		jsonfile := filepath.Join(modsDir, modDir, "manifest.json")
		jsonClosed := jsonfile + ".close"
		var manifestPath string
		if utils.FileExists(jsonfile) {
			manifestPath = jsonfile
		} else if utils.FileExists(jsonClosed) {
			manifestPath = jsonClosed
		} else {
			continue
		}
		var ManifestJson types.ModManifestJson
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		if err = json.Unmarshal(utils.JsonCompatBytes(data), &ManifestJson); err != nil {
			continue
		}
		var modinfo types.ModInfo
		modinfo.Name = ManifestJson.Name
		modinfo.Entry = ManifestJson.Entry
		modinfo.Version = ManifestJson.Version
		modinfo.Type = ManifestJson.Type
		modinfo.Author = ManifestJson.Author
		modinfo.Folder = modDir
		result = append(result, modinfo)
	}
	return result
}

func HasNamedMod(mcname string, targetName string) bool {
	name := strings.TrimSpace(mcname)
	if name == "" || strings.TrimSpace(targetName) == "" {
		return false
	}

	vroot, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vroot) == "" {
		return false
	}

	modsDir := filepath.Join(vroot, name, "mods")
	if !utils.DirExists(modsDir) {
		_ = os.MkdirAll(modsDir, 0o755)
	}

	for _, modDir := range utils.GetDirNames(modsDir) {
		manifestPath := filepath.Join(modsDir, modDir, "manifest.json")
		disabledManifestPath := manifestPath + ".close"
		switch {
		case utils.FileExists(manifestPath):
		case utils.FileExists(disabledManifestPath):
			manifestPath = disabledManifestPath
		default:
			continue
		}

		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var manifest types.ModManifestJson
		if err := json.Unmarshal(utils.JsonCompatBytes(data), &manifest); err != nil {
			continue
		}
		if strings.EqualFold(manifest.Name, targetName) {
			return true
		}
	}

	return false
}

func DeleteMod(mcname string, modFolder string) string {
	name := strings.TrimSpace(mcname)
	mod := strings.TrimSpace(modFolder)
	if name == "" || mod == "" {
		return "ERR_INVALID_NAME"
	}
	vroot, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vroot) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	root := filepath.Join(vroot, name)
	modsDir := filepath.Join(root, "mods")
	target := filepath.Join(modsDir, mod)
	absTarget, _ := filepath.Abs(target)
	absMods, _ := filepath.Abs(modsDir)
	lowT := strings.ToLower(absTarget)
	lowM := strings.ToLower(absMods)
	if lowT != lowM && !strings.HasPrefix(lowT, lowM+string(os.PathSeparator)) {
		return "ERR_INVALID_PACKAGE"
	}
	if !utils.DirExists(target) {
		return "ERR_INVALID_PACKAGE"
	}
	if err := os.RemoveAll(target); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func DisableMod(mcname string, modFolder string) string {
	name := strings.TrimSpace(mcname)
	mod := strings.TrimSpace(modFolder)
	if name == "" || mod == "" {
		return "ERR_INVALID_NAME"
	}
	vroot, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vroot) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	root := filepath.Join(vroot, name)
	modsDir := filepath.Join(root, "mods")
	targetRoot := filepath.Join(modsDir, mod)
	absTarget, _ := filepath.Abs(targetRoot)
	absMods, _ := filepath.Abs(modsDir)
	lowT := strings.ToLower(absTarget)
	lowM := strings.ToLower(absMods)
	if lowT != lowM && !strings.HasPrefix(lowT, lowM+string(os.PathSeparator)) {
		return "ERR_INVALID_PACKAGE"
	}
	if !utils.DirExists(targetRoot) {
		return "ERR_INVALID_PACKAGE"
	}
	mfile := filepath.Join(targetRoot, "manifest.json")
	closed := mfile + ".close"
	if utils.FileExists(closed) {
		return ""
	}
	if !utils.FileExists(mfile) {
		return "ERR_INVALID_PACKAGE"
	}
	if err := os.Rename(mfile, closed); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func EnableMod(mcname string, modFolder string) string {
	name := strings.TrimSpace(mcname)
	mod := strings.TrimSpace(modFolder)
	if name == "" || mod == "" {
		return "ERR_INVALID_NAME"
	}
	vroot, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vroot) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	root := filepath.Join(vroot, name)
	modsDir := filepath.Join(root, "mods")
	targetRoot := filepath.Join(modsDir, mod)
	absTarget, _ := filepath.Abs(targetRoot)
	absMods, _ := filepath.Abs(modsDir)
	lowT := strings.ToLower(absTarget)
	lowM := strings.ToLower(absMods)
	if lowT != lowM && !strings.HasPrefix(lowT, lowM+string(os.PathSeparator)) {
		return "ERR_INVALID_PACKAGE"
	}
	if !utils.DirExists(targetRoot) {
		return "ERR_INVALID_PACKAGE"
	}
	mfile := filepath.Join(targetRoot, "manifest.json")
	closed := mfile + ".close"
	if utils.FileExists(mfile) && !utils.FileExists(closed) {
		return ""
	}
	if !utils.FileExists(closed) {
		return "ERR_INVALID_PACKAGE"
	}
	if err := os.Rename(closed, mfile); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func IsModEnabled(mcname string, modFolder string) bool {
	name := strings.TrimSpace(mcname)
	mod := strings.TrimSpace(modFolder)
	if name == "" || mod == "" {
		return false
	}
	vroot, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vroot) == "" {
		return false
	}
	root := filepath.Join(vroot, name)
	modsDir := filepath.Join(root, "mods")
	targetRoot := filepath.Join(modsDir, mod)
	absTarget, _ := filepath.Abs(targetRoot)
	absMods, _ := filepath.Abs(modsDir)
	lowT := strings.ToLower(absTarget)
	lowM := strings.ToLower(absMods)
	if lowT != lowM && !strings.HasPrefix(lowT, lowM+string(os.PathSeparator)) {
		return false
	}
	if !utils.DirExists(targetRoot) {
		return false
	}
	mfile := filepath.Join(targetRoot, "manifest.json")
	if utils.FileExists(mfile + ".close") {
		return false
	}
	return utils.FileExists(mfile)
}

func UpdateModManifest(mcname string, modFolder string, modName string, version string, modType string, entry string, author string) string {
	name := strings.TrimSpace(mcname)
	mod := strings.TrimSpace(modFolder)
	if name == "" || mod == "" || strings.TrimSpace(modName) == "" {
		return "ERR_INVALID_NAME"
	}
	if mod == "." || strings.ContainsAny(mod, `/\`) {
		return "ERR_INVALID_NAME"
	}
	vroot, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vroot) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	root := filepath.Join(vroot, name)
	modsDir := filepath.Join(root, "mods")
	targetRoot := filepath.Join(modsDir, mod)
	absTarget, _ := filepath.Abs(targetRoot)
	absMods, _ := filepath.Abs(modsDir)
	lowT := strings.ToLower(absTarget)
	lowM := strings.ToLower(absMods)
	if lowT != lowM && !strings.HasPrefix(lowT, lowM+string(os.PathSeparator)) {
		return "ERR_INVALID_PACKAGE"
	}
	if !utils.DirExists(targetRoot) {
		return "ERR_INVALID_PACKAGE"
	}

	manifestPath := filepath.Join(targetRoot, "manifest.json")
	closedPath := manifestPath + ".close"
	manifestExists := utils.FileExists(manifestPath)
	closedExists := utils.FileExists(closedPath)
	if !manifestExists && !closedExists {
		return "ERR_MANIFEST_NOT_FOUND"
	}

	readPath := manifestPath
	if !manifestExists {
		readPath = closedPath
	}
	data, err := os.ReadFile(readPath)
	if err != nil {
		return "ERR_INVALID_PACKAGE"
	}

	manifest := map[string]interface{}{}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(utils.JsonCompatBytes(data), &manifest); err != nil {
			return "ERR_INVALID_PACKAGE"
		}
	}

	manifest["name"] = strings.TrimSpace(modName)
	manifest["version"] = strings.TrimSpace(version)
	manifest["type"] = strings.TrimSpace(modType)
	manifest["entry"] = strings.TrimSpace(entry)
	if strings.TrimSpace(author) == "" {
		delete(manifest, "author")
	} else {
		manifest["author"] = strings.TrimSpace(author)
	}

	mbytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "ERR_WRITE_FILE"
	}

	targetPaths := []string{}
	if manifestExists {
		targetPaths = append(targetPaths, manifestPath)
	}
	if closedExists {
		targetPaths = append(targetPaths, closedPath)
	}
	for _, path := range targetPaths {
		tmp := path + ".tmp"
		f, er := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if er != nil {
			return "ERR_WRITE_FILE"
		}
		if _, er = f.Write(mbytes); er != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return "ERR_WRITE_FILE"
		}
		_ = f.Sync()
		_ = f.Close()
		_ = os.Remove(path)
		if er = os.Rename(tmp, path); er != nil {
			_ = os.Remove(tmp)
			return "ERR_WRITE_FILE"
		}
	}
	return ""
}

func ImportZipToMods(mcname string, data []byte, overwrite bool) string {
	name := strings.TrimSpace(mcname)
	if name == "" {
		return "ERR_INVALID_NAME"
	}
	vroot, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vroot) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	root := filepath.Join(vroot, name)
	modsDir := filepath.Join(root, "mods")
	if !utils.DirExists(modsDir) {
		if er := os.MkdirAll(modsDir, 0755); er != nil {
			return "ERR_CREATE_TARGET_DIR"
		}
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "ERR_OPEN_ZIP"
	}

	manifestDir, manifest, errCode := validateModArchive(zr)
	if errCode != "" {
		return errCode
	}
	modFolderName := strings.TrimSpace(manifest.Name)
	if manifestDir != "" {
		modFolderName = path.Base(manifestDir)
	}
	targetRoot := filepath.Join(modsDir, modFolderName)
	if utils.DirExists(targetRoot) {
		if !overwrite {
			return "ERR_DUPLICATE_FOLDER"
		}
	}

	stagingRoot, err := os.MkdirTemp(modsDir, ".mod-import-*")
	if err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	defer os.RemoveAll(stagingRoot)

	for _, f := range zr.File {
		relInDir, selected, valid := modArchiveRelativePath(f.Name, manifestDir)
		if !valid {
			return "ERR_INVALID_PACKAGE"
		}
		if !selected || relInDir == "" {
			continue
		}
		target := filepath.Join(stagingRoot, filepath.FromSlash(relInDir))
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") {
			if err := os.MkdirAll(target, 0755); err != nil {
				return "ERR_CREATE_TARGET_DIR"
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return "ERR_INVALID_PACKAGE"
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "ERR_CREATE_TARGET_DIR"
		}
		rc, err := f.Open()
		if err != nil {
			return "ERR_READ_ZIP_ENTRY"
		}
		out, er := os.Create(target)
		if er != nil {
			_ = rc.Close()
			return "ERR_WRITE_FILE"
		}
		_, copyErr := io.Copy(out, rc)
		closeOutErr := out.Close()
		closeReadErr := rc.Close()
		if copyErr != nil || closeOutErr != nil {
			return "ERR_WRITE_FILE"
		}
		if closeReadErr != nil {
			return "ERR_READ_ZIP_ENTRY"
		}
	}

	return replaceImportedMod(stagingRoot, targetRoot, overwrite)
}

func validateModArchive(zr *zip.Reader) (string, types.ModManifestJson, string) {
	manifestDir := ""
	var manifest types.ModManifestJson
	manifestFound := false

	for _, file := range zr.File {
		normalized, valid := normalizeModArchivePath(file.Name)
		if !valid {
			return "", manifest, "ERR_INVALID_PACKAGE"
		}
		if normalized == "" || file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
			continue
		}
		if !strings.EqualFold(path.Base(normalized), "manifest.json") {
			continue
		}

		manifestFound = true
		manifestDir = path.Dir(normalized)
		if manifestDir == "." {
			manifestDir = ""
		}
		rc, err := file.Open()
		if err != nil {
			return "", manifest, "ERR_READ_ZIP_ENTRY"
		}
		content, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil || closeErr != nil {
			return "", manifest, "ERR_READ_ZIP_ENTRY"
		}
		if err := json.Unmarshal(utils.JsonCompatBytes(content), &manifest); err != nil {
			return "", manifest, "ERR_INVALID_MANIFEST"
		}
		break
	}

	if !manifestFound {
		return "", manifest, "ERR_MANIFEST_NOT_FOUND"
	}
	if strings.TrimSpace(manifest.Name) == "" ||
		strings.TrimSpace(manifest.Entry) == "" ||
		strings.TrimSpace(manifest.Version) == "" ||
		strings.TrimSpace(manifest.Type) == "" {
		return "", manifest, "ERR_INVALID_MANIFEST"
	}
	if manifestDir == "" && !isSafeModFolderName(manifest.Name) {
		return "", manifest, "ERR_INVALID_MANIFEST"
	}

	entryPath, valid := normalizeModArchivePath(manifest.Entry)
	if !valid || entryPath == "" {
		return "", manifest, "ERR_INVALID_MANIFEST"
	}
	for _, file := range zr.File {
		relative, selected, valid := modArchiveRelativePath(file.Name, manifestDir)
		if !valid {
			return "", manifest, "ERR_INVALID_PACKAGE"
		}
		if selected && !file.FileInfo().IsDir() && strings.EqualFold(relative, entryPath) {
			return manifestDir, manifest, ""
		}
	}
	return "", manifest, "ERR_MANIFEST_ENTRY_NOT_FOUND"
}

func normalizeModArchivePath(value string) (string, bool) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), `\`, "/")
	normalized = strings.TrimPrefix(normalized, "./")
	if normalized == "" {
		return "", true
	}
	if strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") {
		return "", false
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." {
		return "", true
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
}

func modArchiveRelativePath(name string, manifestDir string) (string, bool, bool) {
	normalized, valid := normalizeModArchivePath(name)
	if !valid {
		return "", false, false
	}
	if manifestDir == "" {
		return normalized, true, true
	}
	if normalized == manifestDir {
		return "", true, true
	}
	prefix := manifestDir + "/"
	if !strings.HasPrefix(normalized, prefix) {
		return "", false, true
	}
	return strings.TrimPrefix(normalized, prefix), true, true
}

func isSafeModFolderName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" &&
		trimmed != "." &&
		trimmed != ".." &&
		!strings.ContainsAny(trimmed, `/\:`)
}

func replaceImportedMod(stagingRoot string, targetRoot string, overwrite bool) string {
	if !utils.DirExists(targetRoot) {
		if err := os.Rename(stagingRoot, targetRoot); err != nil {
			return "ERR_WRITE_FILE"
		}
		return ""
	}
	if !overwrite {
		return "ERR_DUPLICATE_FOLDER"
	}

	backupRoot, err := os.MkdirTemp(filepath.Dir(targetRoot), ".mod-backup-*")
	if err != nil {
		return "ERR_WRITE_FILE"
	}
	if err := os.Remove(backupRoot); err != nil {
		return "ERR_WRITE_FILE"
	}
	if err := os.Rename(targetRoot, backupRoot); err != nil {
		return "ERR_WRITE_FILE"
	}
	if err := os.Rename(stagingRoot, targetRoot); err != nil {
		if rollbackErr := os.Rename(backupRoot, targetRoot); rollbackErr != nil {
			log.Printf(
				"mods: failed to roll back import for %s: import error: %v; rollback error: %v",
				targetRoot,
				err,
				rollbackErr,
			)
		}
		return "ERR_WRITE_FILE"
	}
	if err := os.RemoveAll(backupRoot); err != nil {
		log.Printf("mods: imported %s but failed to remove backup %s: %v", targetRoot, backupRoot, err)
	}
	return ""
}

func ImportDllToMods(mcname string, dllFileName string, data []byte, modName string, modType string, version string, overwrite bool) string {
	name := strings.TrimSpace(mcname)
	if name == "" {
		return "ERR_INVALID_NAME"
	}
	vroot, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vroot) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	root := filepath.Join(vroot, name)
	modsDir := filepath.Join(root, "mods")
	if !utils.DirExists(modsDir) {
		if er := os.MkdirAll(modsDir, 0755); er != nil {
			return "ERR_CREATE_TARGET_DIR"
		}
	}
	base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(dllFileName)), filepath.Ext(dllFileName))
	finalName := strings.TrimSpace(modName)
	if finalName == "" {
		finalName = base
	}
	if finalName == "" {
		return "ERR_INVALID_NAME"
	}
	if strings.TrimSpace(modType) == "" {
		modType = "preload-native"
	}
	if strings.TrimSpace(version) == "" {
		version = "0.0.0"
	}
	targetRoot := filepath.Join(modsDir, finalName)
	if utils.DirExists(targetRoot) {
		if !overwrite {
			return "ERR_DUPLICATE_FOLDER"
		}
	}
	if err := os.MkdirAll(targetRoot, 0755); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	dllTarget := filepath.Join(targetRoot, filepath.Base(dllFileName))
	dllTmp := dllTarget + ".tmp"
	f, er := os.OpenFile(dllTmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if er != nil {
		return "ERR_WRITE_FILE"
	}
	if _, er = f.Write(data); er != nil {
		_ = f.Close()
		return "ERR_WRITE_FILE"
	}
	_ = f.Sync()
	_ = f.Close()
	_ = os.Remove(dllTarget)
	if er = os.Rename(dllTmp, dllTarget); er != nil {
		_ = os.Remove(dllTmp)
		return "ERR_WRITE_FILE"
	}

	manifest := types.ModManifestJson{Name: finalName, Entry: filepath.Base(dllFileName), Version: version, Type: modType}
	mbytes, _ := json.MarshalIndent(manifest, "", "  ")
	mpath := filepath.Join(targetRoot, "manifest.json")
	mtmp := mpath + ".tmp"
	mf, me := os.OpenFile(mtmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if me != nil {
		return "ERR_WRITE_FILE"
	}
	if _, me = mf.Write(mbytes); me != nil {
		_ = mf.Close()
		return "ERR_WRITE_FILE"
	}
	_ = mf.Sync()
	_ = mf.Close()
	_ = os.Remove(mpath)
	if me = os.Rename(mtmp, mpath); me != nil {
		_ = os.Remove(mtmp)
		return "ERR_WRITE_FILE"
	}
	return ""
}
