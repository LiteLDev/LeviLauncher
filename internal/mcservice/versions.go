package mcservice

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/icons"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"golang.org/x/sys/windows"
)

type ContentCounts struct {
	Worlds        int `json:"worlds"`
	ResourcePacks int `json:"resourcePacks"`
	BehaviorPacks int `json:"behaviorPacks"`
}

var (
	versionDLL                  = windows.NewLazySystemDLL("version.dll")
	procGetFileVersionInfoSizeW = versionDLL.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = versionDLL.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = versionDLL.NewProc("VerQueryValueW")
	fourPartVersionMatchRe      = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
	fourPartVersionExactMatchRe = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
)

type vsFixedFileInfo struct {
	Signature        uint32
	StrucVersion     uint32
	FileVersionMS    uint32
	FileVersionLS    uint32
	ProductVersionMS uint32
	ProductVersionLS uint32
	FileFlagsMask    uint32
	FileFlags        uint32
	FileOS           uint32
	FileType         uint32
	FileSubtype      uint32
	FileDateMS       uint32
	FileDateLS       uint32
}

func normalizeBedrockGameVersion(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	match := s
	if !fourPartVersionExactMatchRe.MatchString(s) {
		m := fourPartVersionMatchRe.FindString(s)
		if m == "" {
			return s
		}
		match = m
	}
	parts := strings.Split(match, ".")
	if len(parts) != 4 {
		return s
	}
	nums := make([]int, 4)
	for i := 0; i < 4; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return s
		}
		nums[i] = n
	}
	return fmt.Sprintf("%d.%d.%d.%02d", nums[0], nums[1], nums[2], nums[3])
}

func isFourPartNumericVersion(s string) bool {
	return fourPartVersionExactMatchRe.MatchString(strings.TrimSpace(s))
}

func readExeFileVersion(exePath string) (string, bool) {
	p := strings.TrimSpace(exePath)
	if p == "" || !utils.FileExists(p) {
		return "", false
	}
	p16, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return "", false
	}
	var handle uint32
	r1, _, _ := procGetFileVersionInfoSizeW.Call(
		uintptr(unsafe.Pointer(p16)),
		uintptr(unsafe.Pointer(&handle)),
	)
	if r1 == 0 {
		return "", false
	}
	sz := uint32(r1)
	buf := make([]byte, sz)
	r1, _, _ = procGetFileVersionInfoW.Call(
		uintptr(unsafe.Pointer(p16)),
		0,
		uintptr(sz),
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if r1 == 0 {
		return "", false
	}
	root16, _ := windows.UTF16PtrFromString(`\`)
	var block unsafe.Pointer
	var blockLen uint32
	r1, _, _ = procVerQueryValueW.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(root16)),
		uintptr(unsafe.Pointer(&block)),
		uintptr(unsafe.Pointer(&blockLen)),
	)
	if r1 == 0 || block == nil || blockLen < uint32(unsafe.Sizeof(vsFixedFileInfo{})) {
		return "", false
	}
	info := (*vsFixedFileInfo)(block)
	if info.Signature != 0xFEEF04BD {
		return "", false
	}
	major := int(info.FileVersionMS >> 16)
	minor := int(info.FileVersionMS & 0xFFFF)
	build := int(info.FileVersionLS >> 16)
	rev := int(info.FileVersionLS & 0xFFFF)
	return fmt.Sprintf("%d.%d.%d.%d", major, minor, build, rev), true
}

func GetContentRoots(name string) types.ContentRoots {
	roots := types.ContentRoots{Base: "", UsersRoot: "", ResourcePacks: "", BehaviorPacks: "", IsIsolation: false, IsPreview: false}
	verName := strings.TrimSpace(name)
	isPreview := false
	isIsolation := false
	if verName != "" {
		if vdir, err := apppath.VersionsDir(); err == nil && strings.TrimSpace(vdir) != "" {
			dir := filepath.Join(vdir, verName)
			if m, merr := versions.ReadMeta(dir); merr == nil {
				isIsolation = m.EnableIsolation
				isPreview = strings.EqualFold(strings.TrimSpace(m.Type), "preview")
			}
		}
	}
	roots.IsIsolation = isIsolation
	roots.IsPreview = isPreview
	gameDirName := "Minecraft Bedrock"
	if isPreview {
		gameDirName = "Minecraft Bedrock Preview"
	}
	base := ""
	if isIsolation && verName != "" {
		if vdir, err := apppath.VersionsDir(); err == nil && strings.TrimSpace(vdir) != "" {
			base = filepath.Join(vdir, verName, gameDirName)
		}
	} else {
		base = utils.GetMinecraftGDKDataPath(isPreview)
		base = strings.TrimSpace(base)
	}
	roots.Base = base
	if strings.TrimSpace(base) == "" {
		return roots
	}
	users := filepath.Join(base, "Users")
	shared := filepath.Join(users, "Shared", "games", "com.mojang")
	roots.UsersRoot = users
	roots.ResourcePacks = filepath.Join(shared, "resource_packs")
	roots.BehaviorPacks = filepath.Join(shared, "behavior_packs")
	return roots
}

func GetContentCounts(name string) ContentCounts {
	roots := GetContentRoots(name)
	countDirs := func(path string) int {
		p := strings.TrimSpace(path)
		if p == "" {
			return 0
		}
		ents, err := os.ReadDir(p)
		if err != nil {
			return 0
		}
		n := 0
		for _, e := range ents {
			if e.IsDir() {
				n++
			}
		}
		return n
	}
	res := countDirs(roots.ResourcePacks)
	bp := countDirs(roots.BehaviorPacks)
	worlds := 0
	usersRoot := strings.TrimSpace(roots.UsersRoot)
	if usersRoot != "" {
		ents, _ := os.ReadDir(usersRoot)
		var firstPlayer string
		for _, e := range ents {
			if e.IsDir() {
				nm := strings.TrimSpace(e.Name())
				if nm != "" && !strings.EqualFold(nm, "Shared") {
					firstPlayer = nm
					break
				}
			}
		}
		if firstPlayer != "" {
			wp := filepath.Join(usersRoot, firstPlayer, "games", "com.mojang", "minecraftWorlds")
			worlds = countDirs(wp)
		}
	}
	return ContentCounts{Worlds: worlds, ResourcePacks: res, BehaviorPacks: bp}
}

func SaveVersionMeta(name string, gameVersion string, typeStr string, enableIsolation bool, enableConsole bool, enableEditorMode bool, enableRenderDragon bool, launchArgs string, envVars string) string {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	n := strings.TrimSpace(name)
	dir := filepath.Join(vdir, n)

	gvRaw := strings.TrimSpace(gameVersion)
	gv := normalizeBedrockGameVersion(gvRaw)
	if gv == "" {
		gv = gvRaw
	}
	shouldDetect := gv == "" || (strings.EqualFold(gv, n) && !isFourPartNumericVersion(gv))
	if shouldDetect {
		if fv, ok := readExeFileVersion(filepath.Join(dir, "Minecraft.Windows.exe")); ok {
			gv = normalizeBedrockGameVersion(fv)
		}
		if strings.EqualFold(strings.TrimSpace(gv), n) {
			gv = ""
		}
	}

	// Try read old
	oldMeta, _ := versions.ReadMeta(dir)

	meta := versions.VersionMeta{
		Name:               n,
		GameVersion:        strings.TrimSpace(gv),
		Type:               strings.TrimSpace(typeStr),
		EnableIsolation:    enableIsolation,
		EnableConsole:      enableConsole,
		EnableEditorMode:   enableEditorMode,
		EnableRenderDragon: enableRenderDragon,
		LaunchArgs:         launchArgs,
		EnvVars:            envVars,
		CreatedAt:          time.Now(),
	}
	if !oldMeta.CreatedAt.IsZero() {
		meta.CreatedAt = oldMeta.CreatedAt
	}
	if oldMeta.Registered {
		meta.Registered = true
	}

	if _, err := peeditor.PrepareExecutableForLaunch(context.Background(), dir, enableConsole); err != nil {
		return "ERR_PREPARE_EXE"
	}
	if err := versions.WriteMeta(dir, meta); err != nil {
		return "ERR_WRITE_TARGET"
	}

	return ""
}

func ListVersionMetas() []versions.VersionMeta {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return []versions.VersionMeta{}
	}
	metas, err := versions.ScanVersions(vdir)
	if err != nil {
		return []versions.VersionMeta{}
	}
	return metas
}

func GetVersionMeta(name string) versions.VersionMeta {
	var m versions.VersionMeta
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return m
	}
	dir := filepath.Join(vdir, strings.TrimSpace(name))
	mm, er := versions.ReadMeta(dir)
	if er != nil {
		return m
	}
	return mm
}

func ReconcileRegisteredFlags() {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return
	}
	metas, err := versions.ScanVersions(vdir)
	if err != nil || len(metas) == 0 {
		return
	}
	getLoc := func(name string) string {
		if info, e := registry.GetAppxInfo(name); e == nil && info != nil {
			return strings.ToLower(filepath.Clean(strings.TrimSpace(info.InstallLocation)))
		}
		return ""
	}
	releaseLoc := getLoc("MICROSOFT.MINECRAFTUWP")
	previewLoc := getLoc("Microsoft.MinecraftWindowsBeta")
	normalize := func(p string) string {
		s := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
		s = strings.TrimPrefix(s, `\\?\`)
		s = strings.TrimPrefix(s, `\??\`)
		return s
	}
	releaseLoc = normalize(releaseLoc)
	previewLoc = normalize(previewLoc)
	for _, m := range metas {
		isPreview := strings.EqualFold(strings.TrimSpace(m.Type), "preview")
		dir := normalize(filepath.Join(vdir, strings.TrimSpace(m.Name)))
		want := releaseLoc
		if isPreview {
			want = previewLoc
		}
		flag := want != "" && dir == want
		if m.Registered != flag {
			m.Registered = flag
			_ = versions.WriteMeta(filepath.Join(vdir, strings.TrimSpace(m.Name)), m)
		}
	}
}

func ListInheritableVersionNames(versionType string) []string {
	vt := strings.ToLower(strings.TrimSpace(versionType))
	if vt != "release" && vt != "preview" {
		vt = "release"
	}
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return []string{}
	}
	metas, err := versions.ScanVersions(vdir)
	if err != nil || len(metas) == 0 {
		return []string{}
	}
	names := make([]string, 0, len(metas))
	for _, m := range metas {
		if !m.EnableIsolation {
			continue
		}
		mt := strings.ToLower(strings.TrimSpace(m.Type))
		if mt != vt {
			continue
		}
		gameDirName := "Minecraft Bedrock"
		if mt == "preview" {
			gameDirName = "Minecraft Bedrock Preview"
		}
		base := filepath.Join(vdir, m.Name, gameDirName)
		if !utils.DirExists(base) {
			continue
		}
		if utils.DirSize(base) <= 10*1024 {
			continue
		}
		names = append(names, m.Name)
	}
	return names
}

func CopyVersionDataFromVersion(sourceName string, targetName string) string {
	s := strings.TrimSpace(sourceName)
	t := strings.TrimSpace(targetName)
	if s == "" || t == "" {
		return "ERR_TARGET_DIR_NOT_SPECIFIED"
	}
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	sdir := filepath.Join(vdir, s)
	tdir := filepath.Join(vdir, t)
	sm, serr := versions.ReadMeta(sdir)
	if serr != nil {
		return "ERR_INHERIT_SOURCE_NOT_FOUND"
	}
	tm, terr := versions.ReadMeta(tdir)
	if terr != nil {
		return "ERR_INHERIT_TARGET_NOT_FOUND"
	}
	st := strings.ToLower(strings.TrimSpace(sm.Type))
	tt := strings.ToLower(strings.TrimSpace(tm.Type))
	if st == "" || tt == "" || st != tt {
		return "ERR_INHERIT_TYPE_MISMATCH"
	}
	gameDirName := "Minecraft Bedrock"
	if st == "preview" {
		gameDirName = "Minecraft Bedrock Preview"
	}
	srcBase := filepath.Join(sdir, gameDirName)
	dstBase := filepath.Join(tdir, gameDirName)
	if !utils.DirExists(srcBase) {
		return "ERR_INHERIT_SOURCE_NOT_FOUND"
	}
	if err := os.MkdirAll(dstBase, 0755); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	if err := utils.CopyDir(srcBase, dstBase); err != nil {
		return "ERR_INHERIT_COPY_FAILED"
	}
	return ""
}

func CopyVersionDataFromGDK(isPreview bool, targetName string) string {
	t := strings.TrimSpace(targetName)
	if t == "" {
		return "ERR_TARGET_DIR_NOT_SPECIFIED"
	}
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	tdir := filepath.Join(vdir, t)
	tm, terr := versions.ReadMeta(tdir)
	if terr != nil {
		return "ERR_INHERIT_TARGET_NOT_FOUND"
	}
	tt := strings.ToLower(strings.TrimSpace(tm.Type))
	if (isPreview && tt != "preview") || (!isPreview && tt != "release") {
		return "ERR_INHERIT_TYPE_MISMATCH"
	}
	gameDirName := "Minecraft Bedrock"
	if isPreview {
		gameDirName = "Minecraft Bedrock Preview"
	}
	srcBase := utils.GetMinecraftGDKDataPath(isPreview)
	if strings.TrimSpace(srcBase) == "" || !utils.DirExists(srcBase) {
		return "ERR_NO_GAME_DATA"
	}
	dstBase := filepath.Join(tdir, gameDirName)
	if err := os.MkdirAll(dstBase, 0755); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	if err := utils.CopyDir(srcBase, dstBase); err != nil {
		return "ERR_INHERIT_COPY_FAILED"
	}
	return ""
}

func SaveVersionLogoDataUrl(name string, dataUrl string) string {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	dir := filepath.Join(vdir, strings.TrimSpace(name))
	if !utils.DirExists(dir) {
		return "ERR_VERSION_NOT_FOUND"
	}
	du := strings.TrimSpace(dataUrl)
	lower := strings.ToLower(du)
	if !strings.HasPrefix(lower, "data:image/") || !strings.Contains(lower, ";base64,") {
		return "ERR_ICON_FORMAT"
	}
	parts := strings.SplitN(du, ",", 2)
	if len(parts) != 2 {
		return "ERR_ICON_FORMAT"
	}
	b64 := parts[1]
	raw, er := base64.StdEncoding.DecodeString(b64)
	if er != nil {
		return "ERR_ICON_DECODE"
	}
	cfg, _, er := image.DecodeConfig(bytes.NewReader(raw))
	if er != nil {
		return "ERR_ICON_DECODE"
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width != cfg.Height {
		return "ERR_ICON_NOT_SQUARE"
	}
	img, _, er := image.Decode(bytes.NewReader(raw))
	if er != nil {
		return "ERR_ICON_DECODE"
	}
	outPath := filepath.Join(dir, "LargeLogo.png")
	f, ferr := os.Create(outPath)
	if ferr != nil {
		return "ERR_WRITE_TARGET"
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "ERR_WRITE_TARGET"
	}
	return ""
}

func SaveVersionLogoFromPath(name string, filePath string) string {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	dir := filepath.Join(vdir, strings.TrimSpace(name))
	if !utils.DirExists(dir) {
		return "ERR_VERSION_NOT_FOUND"
	}
	p := strings.TrimSpace(filePath)
	if p == "" || !utils.FileExists(p) {
		return "ERR_ICON_DECODE"
	}
	raw, er := os.ReadFile(p)
	if er != nil {
		return "ERR_ICON_DECODE"
	}
	cfg, _, er := image.DecodeConfig(bytes.NewReader(raw))
	if er != nil {
		return "ERR_ICON_DECODE"
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width != cfg.Height {
		return "ERR_ICON_NOT_SQUARE"
	}
	img, _, er := image.Decode(bytes.NewReader(raw))
	if er != nil {
		return "ERR_ICON_DECODE"
	}
	outPath := filepath.Join(dir, "LargeLogo.png")
	f, ferr := os.Create(outPath)
	if ferr != nil {
		return "ERR_WRITE_TARGET"
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "ERR_WRITE_TARGET"
	}
	return ""
}

func GetVersionLogoDataUrl(name string) string {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return ""
	}
	p := filepath.Join(vdir, strings.TrimSpace(name), "LargeLogo.png")
	if !utils.FileExists(p) {
		return ""
	}
	b, er := os.ReadFile(p)
	if er != nil {
		return ""
	}
	enc := base64.StdEncoding.EncodeToString(b)
	return "data:image/png;base64," + enc
}

func RemoveVersionLogo(name string) string {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	p := filepath.Join(vdir, strings.TrimSpace(name), "LargeLogo.png")
	if utils.FileExists(p) {
		if er := os.Remove(p); er != nil {
			return "ERR_WRITE_TARGET"
		}
	}
	return ""
}

func ValidateVersionFolderName(name string) string {
	n := strings.TrimSpace(name)
	if msg := versions.ValidateFolderName(n); msg != "" {
		return msg
	}
	lower := strings.ToLower(n)
	reserved := map[string]struct{}{
		"con": {}, "prn": {}, "aux": {}, "nul": {},
		"com1": {}, "com2": {}, "com3": {}, "com4": {}, "com5": {}, "com6": {}, "com7": {}, "com8": {}, "com9": {},
		"lpt1": {}, "lpt2": {}, "lpt3": {}, "lpt4": {}, "lpt5": {}, "lpt6": {}, "lpt7": {}, "lpt8": {}, "lpt9": {},
	}
	if _, ok := reserved[lower]; ok {
		return "ERR_NAME_RESERVED"
	}
	vdir, err := apppath.VersionsDir()
	if err != nil || vdir == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	entries, err := os.ReadDir(vdir)
	if err != nil {
		return "ERR_READ_VERSIONS_DIR"
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), n) {
			return "ERR_NAME_EXISTS"
		}
	}
	return ""
}

func RenameVersionFolder(oldName string, newName string) string {
	on := strings.TrimSpace(oldName)
	nn := strings.TrimSpace(newName)
	if on == "" || nn == "" {
		return "ERR_INVALID_NAME"
	}
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	oldPath := filepath.Join(vdir, on)
	newPath := filepath.Join(vdir, nn)
	if !utils.DirExists(oldPath) {
		return "ERR_NOT_FOUND_OLD"
	}
	if strings.EqualFold(on, nn) {
		m, _ := versions.ReadMeta(oldPath)
		m.Name = nn
		if er := versions.WriteMeta(oldPath, m); er != nil {
			return "ERR_WRITE_TARGET"
		}
		return ""
	}
	if utils.DirExists(newPath) {
		return "ERR_NAME_EXISTS"
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return "ERR_RENAME_FAILED"
	}
	m, _ := versions.ReadMeta(newPath)
	m.Name = nn
	if er := versions.WriteMeta(newPath, m); er != nil {
		return "ERR_WRITE_TARGET"
	}
	return ""
}

func IsProcessRunningAtPath(exePath string) bool {
	p := strings.ToLower(filepath.Clean(strings.TrimSpace(exePath)))
	if p == "" {
		return false
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return false
	}
	for {
		pid := pe.ProcessID
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err == nil {
			buf := make([]uint16, 1024)
			size := uint32(len(buf))
			if e := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); e == nil && size > 0 {
				path := windows.UTF16ToString(buf[:size])
				_ = windows.CloseHandle(h)
				norm := strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
				norm = strings.TrimPrefix(norm, `\\?\`)
				norm = strings.TrimPrefix(norm, `\??\`)
				if norm == p {
					return true
				}
			} else {
				_ = windows.CloseHandle(h)
			}
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			break
		}
	}
	return false
}

func DeleteVersionFolder(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "ERR_INVALID_NAME"
	}
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	dir := filepath.Join(vdir, n)
	if !utils.DirExists(dir) {
		return "ERR_NOT_FOUND_OLD"
	}
	exe := filepath.Join(dir, "Minecraft.Windows.exe")
	if utils.FileExists(exe) && IsProcessRunningAtPath(exe) {
		return "ERR_GAME_ALREADY_RUNNING"
	}
	if err := os.RemoveAll(dir); err != nil {
		return "ERR_DELETE_FAILED"
	}
	return ""
}

func CreateDesktopShortcut(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "ERR_NAME_REQUIRED"
	}
	exePath, _ := os.Executable()
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return "ERR_SHORTCUT_CREATE_FAILED"
	}

	findDesktopDir := func() string {
		candidates := []string{}
		home, _ := os.UserHomeDir()
		home = strings.TrimSpace(home)
		if home == "" {
			home = strings.TrimSpace(os.Getenv("USERPROFILE"))
		}
		if home != "" {
			candidates = append(candidates, filepath.Join(home, "Desktop"))
			candidates = append(candidates, filepath.Join(home, "OneDrive", "Desktop"))
		}
		for _, k := range []string{"OneDrive", "OneDriveConsumer", "OneDriveCommercial"} {
			v := strings.TrimSpace(os.Getenv(k))
			if v != "" {
				candidates = append(candidates, filepath.Join(v, "Desktop"))
			}
		}
		pub := strings.TrimSpace(os.Getenv("PUBLIC"))
		if pub != "" {
			candidates = append(candidates, filepath.Join(pub, "Desktop"))
		}
		for _, p := range candidates {
			if strings.TrimSpace(p) == "" {
				continue
			}
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				return p
			}
		}
		return ""
	}

	desktop := findDesktopDir()
	if desktop == "" {
		return "ERR_SHORTCUT_CREATE_FAILED"
	}

	safeName := strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		}
		if r < 32 {
			return '_'
		}
		return r
	}, n)
	safeName = strings.TrimSpace(safeName)
	safeName = strings.TrimRight(safeName, ". ")
	if safeName == "" {
		safeName = "Minecraft"
	}

	lnk := filepath.Join(desktop, "Minecraft - "+safeName+".lnk")
	args := "--launch=" + n
	if strings.ContainsAny(n, " \t") {
		esc := strings.ReplaceAll(n, `"`, `\"`)
		args = `--launch="` + esc + `"`
	}
	workdir := filepath.Dir(exePath)
	iconPath := exePath
	if vdir, err := apppath.VersionsDir(); err == nil && strings.TrimSpace(vdir) != "" {
		dir := filepath.Join(vdir, n)
		e := filepath.Join(dir, "Minecraft.Windows.exe")
		isPreview := false
		if m, er := versions.ReadMeta(dir); er == nil {
			isPreview = strings.EqualFold(strings.TrimSpace(m.Type), "preview")
		}
		if p := icons.EnsureVersionIcon(dir, isPreview); strings.TrimSpace(p) != "" {
			lp := strings.ToLower(p)
			if strings.HasSuffix(lp, ".ico") || strings.HasSuffix(lp, ".exe") {
				iconPath = p
			} else {
				if utils.FileExists(e) {
					iconPath = e
				}
			}
		} else if utils.FileExists(e) {
			iconPath = e
		}
	}
	esc := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
	script := "$WshShell = New-Object -ComObject WScript.Shell; " +
		"$Shortcut = $WshShell.CreateShortcut('" + esc(lnk) + "'); " +
		"$Shortcut.TargetPath = '" + esc(exePath) + "'; " +
		"$Shortcut.Arguments = '" + esc(args) + "'; " +
		"$Shortcut.WorkingDirectory = '" + esc(workdir) + "'; " +
		"$Shortcut.IconLocation = '" + esc(iconPath) + "'; " +
		"$Shortcut.Save()"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		cmdFile := filepath.Join(desktop, "Minecraft - "+safeName+".cmd")
		data := "@echo off\r\nstart \"\" \"" + exePath + "\" " + args + "\r\n"
		if werr := os.WriteFile(cmdFile, []byte(data), 0o644); werr != nil {
			return "ERR_SHORTCUT_CREATE_FAILED"
		}
	}
	return ""
}
