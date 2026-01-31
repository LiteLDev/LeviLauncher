package mods

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
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
		result = append(result, modinfo)
	}
	return result
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
	manifestDir := ""
	manifestName := ""
	var manifestJson types.ModManifestJson
	for _, f := range zr.File {
		nameInZip := strings.TrimPrefix(f.Name, "./")
		if strings.HasSuffix(nameInZip, "/") {
			continue
		}
		if strings.EqualFold(filepath.Base(nameInZip), "manifest.json") {
			dir := filepath.Dir(nameInZip)
			rc, er := f.Open()
			if er == nil {
				b, _ := io.ReadAll(rc)
				_ = rc.Close()
				_ = json.Unmarshal(utils.JsonCompatBytes(b), &manifestJson)
				manifestName = strings.TrimSpace(manifestJson.Name)
			}
			if dir != "." && strings.TrimSpace(dir) != "" {
				manifestDir = dir
			}
			break
		}
	}
	if manifestDir == "" && manifestName == "" {
		return "ERR_MANIFEST_NOT_FOUND"
	}
	modFolderName := ""
	if manifestDir != "" {
		modFolderName = filepath.Base(manifestDir)
	} else {
		modFolderName = manifestName
	}
	if strings.TrimSpace(modFolderName) == "" || modFolderName == "." || modFolderName == string(os.PathSeparator) {
		return "ERR_INVALID_PACKAGE"
	}
	targetRoot := filepath.Join(modsDir, modFolderName)
	if utils.DirExists(targetRoot) {
		if !overwrite {
			return "ERR_DUPLICATE_FOLDER"
		}
	}
	for _, f := range zr.File {
		nameInZip := strings.TrimPrefix(f.Name, "./")
		var relInDir string
		if manifestDir != "" {
			if nameInZip != manifestDir && !strings.HasPrefix(nameInZip, manifestDir+"/") {
				continue
			}
			relInDir = strings.TrimPrefix(strings.TrimPrefix(nameInZip, manifestDir), "/")
		} else {
			relInDir = nameInZip
		}
		target := targetRoot
		if relInDir != "" && relInDir != "/" {
			target = filepath.Join(targetRoot, relInDir)
		}
		safeTarget, _ := filepath.Abs(target)
		safeRoot, _ := filepath.Abs(targetRoot)
		if !strings.HasPrefix(strings.ToLower(safeTarget), strings.ToLower(safeRoot+string(os.PathSeparator))) && strings.ToLower(safeTarget) != strings.ToLower(safeRoot) {
			continue
		}
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") {
			if err := os.MkdirAll(target, 0755); err != nil {
				return "ERR_CREATE_TARGET_DIR"
			}
			continue
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
			rc.Close()
			return "ERR_WRITE_FILE"
		}
		if _, er = io.Copy(out, rc); er != nil {
			out.Close()
			rc.Close()
			return "ERR_WRITE_FILE"
		}
		out.Close()
		rc.Close()
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
