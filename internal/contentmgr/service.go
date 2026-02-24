package contentmgr

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/content"
	"github.com/liteldev/LeviLauncher/internal/materialbin"
	"github.com/liteldev/LeviLauncher/internal/packages"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

type MaterialCompatResult struct {
	HasMaterialBin      bool   `json:"hasMaterialBin"`
	Compatible          bool   `json:"compatible"`
	NeedsUpdate         bool   `json:"needsUpdate"`
	PackMaterialPath    string `json:"packMaterialPath"`
	PackMaterialVersion uint64 `json:"packMaterialVersion"`
	GameMaterialPath    string `json:"gameMaterialPath"`
	GameMaterialVersion uint64 `json:"gameMaterialVersion"`
	Error               string `json:"error"`
}

type MaterialUpdateResult struct {
	HasMaterialBin bool   `json:"hasMaterialBin"`
	TotalCount     int    `json:"totalCount"`
	UpdatedCount   int    `json:"updatedCount"`
	SkippedCount   int    `json:"skippedCount"`
	FailedCount    int    `json:"failedCount"`
	Error          string `json:"error"`
}

type PackLoader interface {
	LoadPacksForVersion(versionName string, resourcePacksDir string, behaviorPacksDir string, skinPacksDirs ...string) ([]packages.Pack, error)
}

type Deps struct {
	PackLoader         PackLoader
	GetContentRoots    func(name string) types.ContentRoots
	GetVersionGameInfo func(name string) string
	ListDir            func(path string) []types.FileEntry
}

type Manager struct {
	packLoader         PackLoader
	getContentRoots    func(name string) types.ContentRoots
	getVersionGameInfo func(name string) string
	listDir            func(path string) []types.FileEntry
}

func New(deps Deps) *Manager {
	if deps.GetContentRoots == nil {
		deps.GetContentRoots = func(string) types.ContentRoots { return types.ContentRoots{} }
	}
	if deps.GetVersionGameInfo == nil {
		deps.GetVersionGameInfo = func(string) string { return "" }
	}
	if deps.ListDir == nil {
		deps.ListDir = func(string) []types.FileEntry { return nil }
	}
	return &Manager{
		packLoader:         deps.PackLoader,
		getContentRoots:    deps.GetContentRoots,
		getVersionGameInfo: deps.GetVersionGameInfo,
		listDir:            deps.ListDir,
	}
}

func (m *Manager) ListPacksForVersion(versionName string, player string) []packages.Pack {
	if m.packLoader == nil {
		return []packages.Pack{}
	}
	roots := m.getContentRoots(versionName)
	if roots.ResourcePacks == "" && roots.BehaviorPacks == "" {
		return []packages.Pack{}
	}
	var skinPacksDirs []string

	if roots.ResourcePacks != "" {
		sharedSkins := filepath.Join(filepath.Dir(roots.ResourcePacks), "skin_packs")
		if utils.DirExists(sharedSkins) {
			skinPacksDirs = append(skinPacksDirs, sharedSkins)
		}
	}

	if player != "" && roots.UsersRoot != "" {
		userSkins := filepath.Join(roots.UsersRoot, player, "games", "com.mojang", "skin_packs")
		if utils.DirExists(userSkins) {
			skinPacksDirs = append(skinPacksDirs, userSkins)
		}
		userSkinsSimple := filepath.Join(roots.UsersRoot, player, "skin_packs")
		if utils.DirExists(userSkinsSimple) {
			skinPacksDirs = append(skinPacksDirs, userSkinsSimple)
		}
	}

	packs, err := m.packLoader.LoadPacksForVersion(versionName, roots.ResourcePacks, roots.BehaviorPacks, skinPacksDirs...)
	if err != nil {
		return []packages.Pack{}
	}
	return packs
}

func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}
	for i := 0; i < maxLen; i++ {
		var s1, s2 string
		if i < len(parts1) {
			s1 = parts1[i]
		}
		if i < len(parts2) {
			s2 = parts2[i]
		}

		n1, err1 := strconv.Atoi(s1)
		n2, err2 := strconv.Atoi(s2)

		if err1 == nil && err2 == nil {
			if n1 != n2 {
				if n1 > n2 {
					return 1
				}
				return -1
			}
		} else {
			if s1 != s2 {
				if s1 > s2 {
					return 1
				}
				return -1
			}
		}
	}
	return 0
}

func (m *Manager) versionSkinDir(versionName string, roots types.ContentRoots) string {
	if compareVersions(m.getVersionGameInfo(versionName), "1.26.0.0") > 0 {
		return filepath.Join(filepath.Dir(roots.ResourcePacks), "skin_packs")
	}
	return ""
}

func zipDirToBytes(srcDir string) ([]byte, error) {
	src := filepath.Clean(strings.TrimSpace(srcDir))
	if src == "" {
		return nil, os.ErrInvalid
	}
	fi, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, os.ErrInvalid
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err = filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, in)
		_ = in.Close()
		return err
	})
	if err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *Manager) TransferPackToVersion(sourceVersionName string, sourcePackPath string, targetVersionName string, overwrite bool) string {
	srcVer := strings.TrimSpace(sourceVersionName)
	dstVer := strings.TrimSpace(targetVersionName)
	srcPackPath := filepath.Clean(strings.TrimSpace(sourcePackPath))
	if srcVer == "" || dstVer == "" || srcPackPath == "" {
		return "ERR_INVALID_PATH"
	}

	fi, err := os.Stat(srcPackPath)
	if err != nil || !fi.IsDir() {
		return "ERR_INVALID_PATH"
	}

	srcRoots := m.getContentRoots(srcVer)
	dstRoots := m.getContentRoots(dstVer)
	if strings.TrimSpace(dstRoots.ResourcePacks) == "" && strings.TrimSpace(dstRoots.BehaviorPacks) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}

	allowedSourceRoots := []string{
		strings.TrimSpace(srcRoots.ResourcePacks),
		strings.TrimSpace(srcRoots.BehaviorPacks),
	}
	if strings.TrimSpace(srcRoots.ResourcePacks) != "" {
		allowedSourceRoots = append(
			allowedSourceRoots,
			filepath.Join(filepath.Dir(srcRoots.ResourcePacks), "skin_packs"),
		)
	}
	allowed := false
	for _, root := range allowedSourceRoots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		if isChildOfPath(srcPackPath, root) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "ERR_INVALID_PACKAGE"
	}

	data, err := zipDirToBytes(srcPackPath)
	if err != nil || len(data) == 0 {
		return "ERR_OPEN_ZIP"
	}
	skinDir := m.versionSkinDir(dstVer, dstRoots)
	return content.ImportMcpackToDirs2(
		data,
		filepath.Base(srcPackPath),
		dstRoots.ResourcePacks,
		dstRoots.BehaviorPacks,
		skinDir,
		overwrite,
	)
}

func (m *Manager) ImportMcpack(name string, data []byte, overwrite bool) string {
	roots := m.getContentRoots(name)
	skinDir := m.versionSkinDir(name, roots)
	return content.ImportMcpackToDirs2(data, "", roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (m *Manager) ImportMcpackPath(name string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := m.getContentRoots(name)
	skinDir := m.versionSkinDir(name, roots)
	return content.ImportMcpackToDirs2(b, filepath.Base(path), roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (m *Manager) ImportMcaddon(name string, data []byte, overwrite bool) string {
	roots := m.getContentRoots(name)
	skinDir := m.versionSkinDir(name, roots)
	return content.ImportMcaddonToDirs2(data, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (m *Manager) ImportMcaddonPath(name string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := m.getContentRoots(name)
	skinDir := m.versionSkinDir(name, roots)
	return content.ImportMcaddonToDirs2(b, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (m *Manager) ImportMcaddonWithPlayer(name string, player string, data []byte, overwrite bool) string {
	roots := m.getContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	skinDir := ""
	if users != "" && strings.TrimSpace(player) != "" {
		skinDir = filepath.Join(users, player, "games", "com.mojang", "skin_packs")
	}
	return content.ImportMcaddonToDirs2(data, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (m *Manager) ImportMcaddonPathWithPlayer(name string, player string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := m.getContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	skinDir := ""
	if users != "" && strings.TrimSpace(player) != "" {
		skinDir = filepath.Join(users, player, "games", "com.mojang", "skin_packs")
	}
	return content.ImportMcaddonToDirs2(b, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (m *Manager) ImportMcpackWithPlayer(name string, player string, fileName string, data []byte, overwrite bool) string {
	roots := m.getContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	skinDir := ""
	if users != "" && strings.TrimSpace(player) != "" {
		skinDir = filepath.Join(users, player, "games", "com.mojang", "skin_packs")
	}
	return content.ImportMcpackToDirs2(data, fileName, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (m *Manager) ImportMcpackPathWithPlayer(name string, player string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := m.getContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	skinDir := ""
	if users != "" && strings.TrimSpace(player) != "" {
		skinDir = filepath.Join(users, player, "games", "com.mojang", "skin_packs")
	}
	return content.ImportMcpackToDirs2(b, filepath.Base(path), roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (m *Manager) IsMcpackSkinPackPath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return content.IsMcpackSkinPack(b)
}

func (m *Manager) IsMcpackSkinPack(data []byte) bool {
	return content.IsMcpackSkinPack(data)
}

func (m *Manager) ImportMcworld(name string, player string, fileName string, data []byte, overwrite bool) string {
	roots := m.getContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	if users == "" || strings.TrimSpace(player) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	wp := filepath.Join(users, player, "games", "com.mojang", "minecraftWorlds")
	return content.ImportMcworldToDir(data, fileName, wp, overwrite)
}

func (m *Manager) ImportMcworldPath(name string, player string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := m.getContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	if users == "" || strings.TrimSpace(player) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	wp := filepath.Join(users, player, "games", "com.mojang", "minecraftWorlds")
	return content.ImportMcworldToDir(b, filepath.Base(path), wp, overwrite)
}

func (m *Manager) GetPackInfo(dir string) types.PackInfo {
	return content.ReadPackInfoFromDir(dir)
}

func readMaterialBinVersion(path string) (uint64, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return 0, os.ErrNotExist
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return 0, err
	}
	def, _, err := materialbin.ParseAuto(b)
	if err != nil {
		return 0, err
	}
	return def.Version, nil
}

func listPackMaterialBinFiles(packPath string) ([]string, error) {
	base := strings.TrimSpace(packPath)
	if base == "" {
		return nil, os.ErrNotExist
	}
	materialsDirs := []string{
		filepath.Join(base, "renderer", "materials"),
	}

	subpacksRoot := filepath.Join(base, "subpacks")
	if subEntries, err := os.ReadDir(subpacksRoot); err == nil {
		for _, e := range subEntries {
			if !e.IsDir() {
				continue
			}
			subpackName := strings.TrimSpace(e.Name())
			if subpackName == "" {
				continue
			}
			materialsDirs = append(
				materialsDirs,
				filepath.Join(subpacksRoot, subpackName, "renderer", "materials"),
			)
		}
	}

	out := make([]string, 0, 16)
	for _, materialsDir := range materialsDirs {
		entries, err := os.ReadDir(materialsDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.TrimSpace(e.Name())
			if name == "" {
				continue
			}
			if !strings.HasSuffix(strings.ToLower(name), ".material.bin") {
				continue
			}
			out = append(out, filepath.Join(materialsDir, name))
		}
	}
	if len(out) == 0 {
		return nil, os.ErrNotExist
	}
	return out, nil
}

func isChildOfPath(path string, root string) bool {
	p, err1 := filepath.Abs(strings.TrimSpace(path))
	r, err2 := filepath.Abs(strings.TrimSpace(root))
	if err1 != nil || err2 != nil {
		return false
	}
	p = strings.ToLower(filepath.Clean(p))
	r = strings.ToLower(filepath.Clean(r))
	return p != r && strings.HasPrefix(p, r+string(os.PathSeparator))
}

func nextAvailableFolderName(rootDir string, desired string) string {
	base := strings.TrimSpace(desired)
	base = filepath.Clean(base)
	base = filepath.Base(base)
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == string(os.PathSeparator) {
		base = "world"
	}
	seen := map[string]struct{}{}
	if entries, err := os.ReadDir(rootDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			seen[strings.ToLower(strings.TrimSpace(e.Name()))] = struct{}{}
		}
	}

	candidate := base
	if _, ok := seen[strings.ToLower(candidate)]; !ok {
		return candidate
	}
	for i := 1; i < 100000; i++ {
		next := fmt.Sprintf("%s_%d", base, i)
		if _, ok := seen[strings.ToLower(next)]; !ok {
			return next
		}
	}
	return fmt.Sprintf("%s_%d", base, time.Now().Unix())
}

func (m *Manager) UpdateResourcePackMaterialBins(versionName string, packPath string) MaterialUpdateResult {
	result := MaterialUpdateResult{}
	verName := strings.TrimSpace(versionName)
	packDir := strings.TrimSpace(packPath)
	if verName == "" || packDir == "" {
		result.Error = "invalid input"
		return result
	}
	fi, err := os.Stat(packDir)
	if err != nil || !fi.IsDir() {
		result.Error = "ERR_INVALID_PATH"
		return result
	}

	roots := m.getContentRoots(verName)
	if strings.TrimSpace(roots.ResourcePacks) == "" {
		result.Error = "ERR_ACCESS_VERSIONS_DIR"
		return result
	}
	if !isChildOfPath(packDir, roots.ResourcePacks) {
		result.Error = "ERR_INVALID_PACKAGE"
		return result
	}

	files, err := listPackMaterialBinFiles(packDir)
	if err != nil || len(files) == 0 {
		return result
	}
	result.HasMaterialBin = true
	result.TotalCount = len(files)

	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		result.Error = "ERR_ACCESS_VERSIONS_DIR"
		return result
	}
	gameMaterialPath := filepath.Join(vdir, verName, "data", "renderer", "materials", "RenderChunk.material.bin")
	gameBuf, err := os.ReadFile(gameMaterialPath)
	if err != nil {
		result.Error = "ERR_READ_GAME_RENDERCHUNK"
		return result
	}
	_, targetVersion, err := materialbin.ParseAuto(gameBuf)
	if err != nil {
		result.Error = "ERR_READ_GAME_RENDERCHUNK"
		return result
	}

	for _, p := range files {
		raw, err := os.ReadFile(p)
		if err != nil {
			result.FailedCount++
			continue
		}
		def, _, err := materialbin.ParseAuto(raw)
		if err != nil {
			result.FailedCount++
			continue
		}
		rebuilt, err := def.MarshalBinary(targetVersion)
		if err != nil {
			result.FailedCount++
			continue
		}
		if bytes.Equal(raw, rebuilt) {
			result.SkippedCount++
			continue
		}
		mode := os.FileMode(0644)
		if st, err := os.Stat(p); err == nil {
			mode = st.Mode().Perm()
		}
		tmp := p + ".tmp"
		if err := os.WriteFile(tmp, rebuilt, mode); err != nil {
			_ = os.Remove(tmp)
			result.FailedCount++
			continue
		}
		if err := os.Rename(tmp, p); err != nil {
			_ = os.Remove(tmp)
			result.FailedCount++
			continue
		}
		result.UpdatedCount++
	}
	return result
}

func (m *Manager) CheckResourcePackMaterialCompatibility(versionName string, packPath string) MaterialCompatResult {
	result := MaterialCompatResult{
		Compatible: true,
	}
	verName := strings.TrimSpace(versionName)
	packDir := strings.TrimSpace(packPath)
	if verName == "" || packDir == "" {
		result.Error = "invalid input"
		return result
	}
	files, err := listPackMaterialBinFiles(packDir)
	if err != nil || len(files) == 0 {
		// If pack has no renderer/materials/*.material.bin (including subpacks), show nothing.
		return result
	}
	result.HasMaterialBin = true

	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		result.Error = "ERR_ACCESS_VERSIONS_DIR"
		return result
	}
	gameMaterialPath := filepath.Join(vdir, verName, "data", "renderer", "materials", "RenderChunk.material.bin")
	result.GameMaterialPath = gameMaterialPath

	gameVersion, err := readMaterialBinVersion(gameMaterialPath)
	if err != nil {
		result.Error = "ERR_READ_GAME_RENDERCHUNK"
		return result
	}
	result.GameMaterialVersion = gameVersion

	parsedAny := false
	for _, p := range files {
		packVersion, err := readMaterialBinVersion(p)
		if err != nil {
			continue
		}
		if !parsedAny {
			result.PackMaterialPath = p
			result.PackMaterialVersion = packVersion
			parsedAny = true
		}
		if packVersion != gameVersion {
			result.Compatible = false
			result.NeedsUpdate = true
			result.PackMaterialPath = p
			result.PackMaterialVersion = packVersion
			return result
		}
	}
	if !parsedAny {
		result.Error = "ERR_READ_PACK_MATERIALBIN"
	}
	return result
}

func (m *Manager) DeletePack(name string, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return "ERR_INVALID_PATH"
	}
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		return "ERR_INVALID_PATH"
	}
	roots := m.getContentRoots(name)
	allowed := []string{strings.TrimSpace(roots.ResourcePacks), strings.TrimSpace(roots.BehaviorPacks)}
	usersRoot := strings.TrimSpace(roots.UsersRoot)
	if usersRoot != "" {
		ents := m.listDir(usersRoot)
		for _, e := range ents {
			if !e.IsDir {
				continue
			}
			nm := strings.TrimSpace(e.Name)
			if nm == "" || strings.EqualFold(nm, "Shared") {
				continue
			}
			sp := filepath.Join(usersRoot, nm, "games", "com.mojang", "skin_packs")
			allowed = append(allowed, sp)
		}
	}
	absTarget, _ := filepath.Abs(p)
	lowT := strings.ToLower(absTarget)
	ok := false
	for _, r := range allowed {
		if strings.TrimSpace(r) == "" {
			continue
		}
		absRoot, _ := filepath.Abs(r)
		lowR := strings.ToLower(absRoot)
		if lowT != lowR && strings.HasPrefix(lowT, lowR+string(os.PathSeparator)) {
			ok = true
			break
		}
	}
	if !ok {
		return "ERR_INVALID_PACKAGE"
	}
	if err := os.RemoveAll(absTarget); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func (m *Manager) DeleteWorld(name string, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return "ERR_INVALID_PATH"
	}
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		return "ERR_INVALID_PATH"
	}
	roots := m.getContentRoots(name)
	usersRoot := strings.TrimSpace(roots.UsersRoot)
	allowed := []string{}
	if usersRoot != "" {
		ents := m.listDir(usersRoot)
		for _, e := range ents {
			if !e.IsDir {
				continue
			}
			nm := strings.TrimSpace(e.Name)
			if nm == "" || strings.EqualFold(nm, "Shared") {
				continue
			}
			wp := filepath.Join(usersRoot, nm, "games", "com.mojang", "minecraftWorlds")
			allowed = append(allowed, wp)
		}
	}
	absTarget, _ := filepath.Abs(p)
	lowT := strings.ToLower(absTarget)
	ok := false
	for _, r := range allowed {
		if strings.TrimSpace(r) == "" {
			continue
		}
		absRoot, _ := filepath.Abs(r)
		lowR := strings.ToLower(absRoot)
		if lowT != lowR && strings.HasPrefix(lowT, lowR+string(os.PathSeparator)) {
			ok = true
			break
		}
	}
	if !ok {
		return "ERR_INVALID_PATH"
	}
	if err := os.RemoveAll(absTarget); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func (m *Manager) TransferWorldToVersion(sourceVersionName string, sourcePlayer string, sourceWorldPath string, targetVersionName string, targetPlayer string) string {
	srcVer := strings.TrimSpace(sourceVersionName)
	dstVer := strings.TrimSpace(targetVersionName)
	srcPlayer := strings.TrimSpace(sourcePlayer)
	dstPlayer := strings.TrimSpace(targetPlayer)
	srcWorld := filepath.Clean(strings.TrimSpace(sourceWorldPath))
	if srcVer == "" || dstVer == "" || srcPlayer == "" || dstPlayer == "" || srcWorld == "" {
		return "ERR_INVALID_PATH"
	}

	fi, err := os.Stat(srcWorld)
	if err != nil || !fi.IsDir() {
		return "ERR_INVALID_PATH"
	}

	srcRoots := m.getContentRoots(srcVer)
	dstRoots := m.getContentRoots(dstVer)
	srcUsersRoot := strings.TrimSpace(srcRoots.UsersRoot)
	dstUsersRoot := strings.TrimSpace(dstRoots.UsersRoot)
	if srcUsersRoot == "" || dstUsersRoot == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}

	sourceWorldsRoot := filepath.Join(srcUsersRoot, srcPlayer, "games", "com.mojang", "minecraftWorlds")
	if !isChildOfPath(srcWorld, sourceWorldsRoot) {
		return "ERR_INVALID_PATH"
	}

	targetWorldsRoot := filepath.Join(dstUsersRoot, dstPlayer, "games", "com.mojang", "minecraftWorlds")
	if err := os.MkdirAll(targetWorldsRoot, 0755); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}

	targetFolderName := nextAvailableFolderName(targetWorldsRoot, filepath.Base(srcWorld))
	targetPath := filepath.Join(targetWorldsRoot, targetFolderName)

	if err := utils.CopyDir(srcWorld, targetPath); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

type ScreenshotInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Dir         string `json:"dir"`
	CaptureTime int64  `json:"captureTime"`
}

type screenshotMeta struct {
	CaptureTime int64 `json:"captureTime"`
}

func (m *Manager) ListScreenshots(versionName string, player string) []ScreenshotInfo {
	roots := m.getContentRoots(versionName)
	usersRoot := strings.TrimSpace(roots.UsersRoot)
	p := strings.TrimSpace(player)
	if usersRoot == "" || p == "" {
		return []ScreenshotInfo{}
	}
	screenshotsRoot := filepath.Join(usersRoot, p, "games", "com.mojang", "Screenshots")
	if !utils.DirExists(screenshotsRoot) {
		return []ScreenshotInfo{}
	}
	var result []ScreenshotInfo
	subDirs, err := os.ReadDir(screenshotsRoot)
	if err != nil {
		return []ScreenshotInfo{}
	}
	for _, sub := range subDirs {
		if !sub.IsDir() {
			continue
		}
		subPath := filepath.Join(screenshotsRoot, sub.Name())
		entries, err := os.ReadDir(subPath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".jpeg") && !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".png") {
				continue
			}
			baseName := strings.TrimSuffix(name, filepath.Ext(name))
			imgPath := filepath.Join(subPath, name)
			jsonPath := filepath.Join(subPath, baseName+".json")
			var captureTime int64
			if jsonData, err := os.ReadFile(jsonPath); err == nil {
				var meta screenshotMeta
				if json.Unmarshal(jsonData, &meta) == nil {
					captureTime = meta.CaptureTime
				}
			}
			result = append(result, ScreenshotInfo{
				Name:        baseName,
				Path:        imgPath,
				Dir:         subPath,
				CaptureTime: captureTime,
			})
		}
	}
	if result == nil {
		return []ScreenshotInfo{}
	}
	return result
}

func (m *Manager) DeleteScreenshot(versionName string, player string, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return "ERR_INVALID_PATH"
	}
	roots := m.getContentRoots(versionName)
	usersRoot := strings.TrimSpace(roots.UsersRoot)
	pl := strings.TrimSpace(player)
	if usersRoot == "" || pl == "" {
		return "ERR_INVALID_PATH"
	}
	screenshotsRoot := filepath.Join(usersRoot, pl, "games", "com.mojang", "Screenshots")
	if !isChildOfPath(p, screenshotsRoot) {
		return "ERR_INVALID_PATH"
	}
	baseName := strings.TrimSuffix(p, filepath.Ext(p))
	exts := []string{filepath.Ext(p), ".json", ".mc"}
	for _, ext := range exts {
		_ = os.Remove(baseName + ext)
	}
	return ""
}
