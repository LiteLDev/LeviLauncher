package vcruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/google/go-github/v30/github"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/wailsapp/wails/v3/pkg/application"
	win "golang.org/x/sys/windows"
	winreg "golang.org/x/sys/windows/registry"
)

const (
	EventEnsureStart    = "vcruntime.ensure.start"
	EventEnsureProgress = "vcruntime.ensure.progress"
	EventEnsureDone     = "vcruntime.ensure.done"

	EventDownloadStart    = "vcruntime.download.start"
	EventDownloadProgress = "vcruntime.download.progress"
	EventDownloadDone     = "vcruntime.download.done"
	EventDownloadError    = "vcruntime.download.error"
	EventMissing          = "vcruntime.missing"
)

type EnsureProgress struct {
	Downloaded int64
	Total      int64
}

type vcRuntimeRegistryState struct {
	Installed    uint64
	HasInstalled bool
	Version      string
	Major        uint64
	HasMajor     bool
}

var (
	mu       sync.Mutex
	ensuring bool

	vcRuntimeRegistryPaths = []string{
		"SOFTWARE\\Microsoft\\VisualStudio\\14.0\\VC\\Runtimes\\x64",
	}
	vcRuntimeUninstallRootPath = "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall"

	vcRuntimeX64SystemDLLs = []string{
		"msvcp140.dll",
		"msvcp140_1.dll",
		"mfc140.dll",
		"mfc140u.dll",
		"mfcm140.dll",
		"mfcm140u.dll",
		"vcruntime140.dll",
		"vcruntime140_1.dll",
	}
)

const (
	vcRuntimeForceNoInstalledPath = "C:\\vc_force_noinstalled"
	vcRuntimeDownloadURL          = "https://aka.ms/vc14/vc_redist.x64.exe"
	vcRuntimeInstallerName        = "vc_redist.x64.exe"
)

const (
	messageBoxOK              = 0x00000000
	messageBoxOKCancel        = 0x00000001
	messageBoxIconError       = 0x00000010
	messageBoxIconInformation = 0x00000040
	messageBoxDefaultButton1  = 0x00000000
	messageBoxDefaultButton2  = 0x00000100
	messageBoxResultOK        = 1
	messageBoxResultCancel    = 2
)

var (
	user32MessageBox             = win.NewLazySystemDLL("user32.dll")
	procVcRuntimeMessageBoxW     = user32MessageBox.NewProc("MessageBoxW")
	kernel32Locale               = win.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultUILanguage = kernel32Locale.NewProc("GetUserDefaultUILanguage")
)

//go:embed vcruntime140_1.dll
var embeddedVcruntime []byte

func bytesSHA256(b []byte) []byte { h := sha256.Sum256(b); return h[:] }

func fileSHA256(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

type vcRuntimeDownloadCallbacks struct {
	Start    func(total int64)
	Progress func(downloaded int64, total int64)
	Done     func()
}

func vcRuntimeInstallerPath() (string, string) {
	dir, _ := apppath.InstallersDir()
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "LeviLauncher", "Installers")
	}
	return dir, filepath.Join(dir, vcRuntimeInstallerName)
}

func downloadVcRuntimeInstaller(ctx context.Context, destPath string, callbacks vcRuntimeDownloadCallbacks) error {
	if strings.TrimSpace(destPath) == "" {
		return fmt.Errorf("empty VC Runtime installer path")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	tmpPath := destPath + ".part"

	req, err := http.NewRequestWithContext(ctx, "GET", vcRuntimeDownloadURL, nil)
	if err != nil {
		return err
	}
	httpx.ApplyDefaultHeaders(req)
	resp, err := httpx.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download VC Runtime installer failed: %s", resp.Status)
	}

	if callbacks.Start != nil {
		callbacks.Start(resp.ContentLength)
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	var downloaded int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = f.Close()
				_ = os.Remove(tmpPath)
				return werr
			}
			downloaded += int64(n)
			if callbacks.Progress != nil {
				callbacks.Progress(downloaded, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(destPath)
		if err := os.Rename(tmpPath, destPath); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
	}

	if callbacks.Done != nil {
		callbacks.Done()
	}
	return nil
}

func messageBox(title, message string, flags uint32) int32 {
	titlePtr, err := win.UTF16PtrFromString(title)
	if err != nil {
		log.Printf("vcruntime: failed to encode message box title: %v", err)
		return 0
	}
	messagePtr, err := win.UTF16PtrFromString(message)
	if err != nil {
		log.Printf("vcruntime: failed to encode message box body: %v", err)
		return 0
	}
	r, _, _ := procVcRuntimeMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(flags),
	)
	return int32(r)
}

func isChineseWindowsUI() bool {
	langID, _, err := procGetUserDefaultUILanguage.Call()
	if langID == 0 || err != nil && err != syscall.Errno(0) {
		return false
	}
	primaryLangID := uint16(langID) & 0x03ff
	return primaryLangID == 0x04
}

func vcStartupRequiredTitle() string {
	if isChineseWindowsUI() {
		return "LeviLauncher - 需要 Visual C++ Runtime"
	}
	return "LeviLauncher - Visual C++ Runtime Required"
}

func vcStartupDownloadTitle() string {
	if isChineseWindowsUI() {
		return "LeviLauncher - 正在下载 Runtime"
	}
	return "LeviLauncher - Downloading Runtime"
}

func vcStartupInstallPromptMessage() string {
	if isChineseWindowsUI() {
		return "LeviLauncher 需要 Microsoft Visual C++ 2015-2022 Redistributable (x64) 才能正常启动。\n\n点击“确定”下载并打开 Microsoft 安装程序。安装完成后，请重新启动 LeviLauncher。"
	}
	return "LeviLauncher requires Microsoft Visual C++ 2015-2022 Redistributable (x64) to start correctly.\n\nClick OK to download and open the Microsoft installer. After the installation finishes, restart LeviLauncher."
}

func vcStartupDownloadMessage() string {
	if isChineseWindowsUI() {
		return "LeviLauncher 将下载 Microsoft Visual C++ 2015-2022 Redistributable (x64) 安装程序。请稍候，下载完成后会打开安装程序窗口。"
	}
	return "LeviLauncher will download the Microsoft Visual C++ 2015-2022 Redistributable (x64) installer now. Please wait; another installer window will open when the download finishes."
}

func vcStartupDownloadFailedMessage(err error) string {
	if isChineseWindowsUI() {
		return fmt.Sprintf("下载 Microsoft Visual C++ 2015-2022 Redistributable (x64) 失败。\n\n错误:\n%v", err)
	}
	return fmt.Sprintf("Failed to download Microsoft Visual C++ 2015-2022 Redistributable (x64).\n\nError:\n%v", err)
}

func vcStartupOpenInstallerFailedMessage(installerPath string, err error) string {
	if isChineseWindowsUI() {
		return fmt.Sprintf("打开 Microsoft Visual C++ 安装程序失败。\n\n安装程序路径:\n%s\n\n错误:\n%v", installerPath, err)
	}
	return fmt.Sprintf("Failed to open the Microsoft Visual C++ installer.\n\nInstaller path:\n%s\n\nError:\n%v", installerPath, err)
}

func vcStartupInstallerOpenedMessage() string {
	if isChineseWindowsUI() {
		return "Microsoft Visual C++ 安装程序已打开。请完成安装，然后重新启动 LeviLauncher。"
	}
	return "The Microsoft Visual C++ installer has been opened. Finish the installation, then restart LeviLauncher."
}

func showVCStartupInstallPrompt() bool {
	result := messageBox(
		vcStartupRequiredTitle(),
		vcStartupInstallPromptMessage(),
		messageBoxOKCancel|messageBoxIconInformation|messageBoxDefaultButton1,
	)
	return result == messageBoxResultOK
}

func showVCStartupDownloadMessage() {
	messageBox(
		vcStartupDownloadTitle(),
		vcStartupDownloadMessage(),
		messageBoxOK|messageBoxIconInformation,
	)
}

func showVCStartupError(message string) {
	messageBox(
		vcStartupRequiredTitle(),
		message,
		messageBoxOK|messageBoxIconError,
	)
}

func showVCStartupInfo(message string) {
	messageBox(
		vcStartupRequiredTitle(),
		message,
		messageBoxOK|messageBoxIconInformation,
	)
}

func runVCInstallerVisible(installerPath string) error {
	verb, err := win.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := win.UTF16PtrFromString(installerPath)
	if err != nil {
		return err
	}
	return win.ShellExecute(0, verb, file, nil, nil, win.SW_SHOWNORMAL)
}

func EnsureStartupInteractive(ctx context.Context) bool {
	if IsInstalled() {
		return true
	}
	if !showVCStartupInstallPrompt() {
		return false
	}

	_, installerPath := vcRuntimeInstallerPath()
	if !fileExists(installerPath) {
		showVCStartupDownloadMessage()
		if err := downloadVcRuntimeInstaller(ctx, installerPath, vcRuntimeDownloadCallbacks{}); err != nil {
			log.Printf("vcruntime: startup download failed: %v", err)
			showVCStartupError(vcStartupDownloadFailedMessage(err))
			return false
		}
	}

	if err := runVCInstallerVisible(installerPath); err != nil {
		log.Printf("vcruntime: failed to open visible installer: %v", err)
		showVCStartupError(vcStartupOpenInstallerFailedMessage(installerPath, err))
		return false
	}

	showVCStartupInfo(vcStartupInstallerOpenedMessage())
	return false
}

func readVcRuntimeRegistryState(path string) (vcRuntimeRegistryState, bool) {
	accessModes := []uint32{winreg.READ | winreg.WOW64_64KEY, winreg.READ}
	for _, access := range accessModes {
		key, err := winreg.OpenKey(winreg.LOCAL_MACHINE, path, access)
		if err != nil {
			continue
		}
		defer key.Close()

		state := vcRuntimeRegistryState{}
		if installed, _, err := key.GetIntegerValue("Installed"); err == nil {
			state.Installed = installed
			state.HasInstalled = true
		}
		if version, _, err := key.GetStringValue("Version"); err == nil {
			state.Version = strings.TrimSpace(version)
		}
		if major, _, err := key.GetIntegerValue("Major"); err == nil {
			state.Major = major
			state.HasMajor = true
		}
		return state, true
	}
	return vcRuntimeRegistryState{}, false
}

func isVcRuntimeRegistryStateInstalled(state vcRuntimeRegistryState) bool {
	if !state.HasInstalled || state.Installed != 1 {
		return false
	}
	if strings.TrimSpace(state.Version) != "" {
		return true
	}
	return state.HasMajor && state.Major >= 14
}

func hasInstalledVcRuntimeRegistry(paths []string, readState func(string) (vcRuntimeRegistryState, bool)) bool {
	for _, path := range paths {
		state, ok := readState(path)
		if ok && isVcRuntimeRegistryStateInstalled(state) {
			return true
		}
	}
	return false
}

func readVCUninstallDisplayNames(rootPath string) []string {
	accessModes := []uint32{winreg.READ | winreg.WOW64_64KEY, winreg.READ}
	for _, access := range accessModes {
		key, err := winreg.OpenKey(winreg.LOCAL_MACHINE, rootPath, access)
		if err != nil {
			continue
		}
		names, err := key.ReadSubKeyNames(-1)
		if err != nil {
			_ = key.Close()
			continue
		}

		displayNames := make([]string, 0, len(names))
		for _, name := range names {
			sub, err := winreg.OpenKey(key, name, winreg.QUERY_VALUE)
			if err != nil {
				continue
			}
			displayName, _, err := sub.GetStringValue("DisplayName")
			_ = sub.Close()
			if err != nil {
				continue
			}
			displayNames = append(displayNames, displayName)
		}
		_ = key.Close()
		return displayNames
	}
	return nil
}

func isVC2015To2022X64RedistributableDisplayName(displayName string) bool {
	displayName = strings.ToLower(strings.TrimSpace(displayName))
	if displayName == "" {
		return false
	}
	if !strings.Contains(displayName, "microsoft visual c++") {
		return false
	}
	if !strings.Contains(displayName, "2015") || !strings.Contains(displayName, "2022") {
		return false
	}
	if !strings.Contains(displayName, "redistributable") {
		return false
	}
	return strings.Contains(displayName, "(x64)") || strings.Contains(displayName, " x64 ") || strings.HasSuffix(displayName, " x64")
}

func hasVC2015To2022X64UninstallEntry(readDisplayNames func(string) []string) bool {
	for _, displayName := range readDisplayNames(vcRuntimeUninstallRootPath) {
		if isVC2015To2022X64RedistributableDisplayName(displayName) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func vcRuntimeSystem32Dir() string {
	systemRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	return filepath.Join(systemRoot, "System32")
}

func hasVC2015To2022X64SystemFiles(system32Dir string, exists func(string) bool) bool {
	system32Dir = strings.TrimSpace(system32Dir)
	if system32Dir == "" {
		return false
	}
	for _, dll := range vcRuntimeX64SystemDLLs {
		if !exists(filepath.Join(system32Dir, dll)) {
			return false
		}
	}
	return true
}

func isVC2015To2022X64Installed(forceNoInstalled, hasRegistry, hasUninstallEntry, hasSystemFiles bool) bool {
	if forceNoInstalled {
		return false
	}
	return hasRegistry || hasUninstallEntry || hasSystemFiles
}

func IsInstalled() bool {
	if fileExists(vcRuntimeForceNoInstalledPath) {
		return false
	}
	if hasInstalledVcRuntimeRegistry(vcRuntimeRegistryPaths, readVcRuntimeRegistryState) {
		return true
	}
	if hasVC2015To2022X64UninstallEntry(readVCUninstallDisplayNames) {
		return true
	}
	return hasVC2015To2022X64SystemFiles(vcRuntimeSystem32Dir(), fileExists)
}

func EnsureInteractive(ctx context.Context) {
	mu.Lock()
	if ensuring {
		mu.Unlock()
		return
	}
	ensuring = true
	mu.Unlock()
	defer func() {
		mu.Lock()
		ensuring = false
		mu.Unlock()
	}()

	if IsInstalled() {
		return
	}

	application.Get().Event.Emit(EventEnsureStart, struct{}{})

	_, dlPath := vcRuntimeInstallerPath()
	if err := downloadVcRuntimeInstaller(ctx, dlPath, vcRuntimeDownloadCallbacks{
		Start: func(total int64) {
			application.Get().Event.Emit(EventDownloadStart, total)
		},
		Progress: func(downloaded int64, total int64) {
			application.Get().Event.Emit(EventDownloadProgress, EnsureProgress{Downloaded: downloaded, Total: total})
		},
		Done: func() {
			application.Get().Event.Emit(EventDownloadDone, struct{}{})
		},
	}); err != nil {
		application.Get().Event.Emit(EventDownloadError, err.Error())
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}

	if err := runVCInstallerVisible(dlPath); err != nil {
		log.Println("failed to open VC Runtime installer:", err)
		application.Get().Event.Emit(EventDownloadError, err.Error())
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}

	installed := false
	for i := 0; i < 30; i++ {
		if IsInstalled() {
			installed = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	log.Println("VC Runtime installed:", installed)
	application.Get().Event.Emit(EventEnsureDone, installed)
}

func EnsureForVersion(ctx context.Context, versionDir string) bool {
	if strings.TrimSpace(versionDir) == "" {
		return false
	}
	dest := filepath.Join(versionDir, "vcruntime140_1.dll")

	if len(embeddedVcruntime) > 0 {
		needWrite := true
		if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
			if fh, err := fileSHA256(dest); err == nil {
				if bytes.Equal(fh, bytesSHA256(embeddedVcruntime)) {
					needWrite = false
				}
			}
		}
		if needWrite {
			_ = os.MkdirAll(versionDir, 0755)
			tmp := dest + ".tmp"
			if err := os.WriteFile(tmp, embeddedVcruntime, 0644); err != nil {
				_ = os.Remove(tmp)
				return false
			}
			if err := os.Rename(tmp, dest); err != nil {
				_ = os.Remove(tmp)
				return false
			}
		}
		return true
	}

	if _, err := os.Stat(dest); err == nil {
		return true
	}
	return false
}

func EnsureLatest(ctx context.Context, contentDir string) {
	if strings.TrimSpace(contentDir) == "" {
		return
	}
	dest := filepath.Join(contentDir, "vcruntime140_1.dll")
	tmp := dest + ".tmp"
	if _, err := os.Stat(dest); err == nil {
		application.Get().Event.Emit(EventEnsureStart, struct{}{})
		application.Get().Event.Emit(EventEnsureDone, true)

		return
	}
	if _, err := os.Stat(tmp); err == nil {
		if err := os.Rename(tmp, dest); err != nil {
			if in, e1 := os.Open(tmp); e1 == nil {
				defer in.Close()
				if out, e2 := os.Create(dest); e2 == nil {
					if _, e3 := io.Copy(out, in); e3 == nil {
						out.Close()
						_ = os.Remove(tmp)
						application.Get().Event.Emit(EventEnsureStart, struct{}{})
						application.Get().Event.Emit(EventEnsureDone, true)
						return
					}
					out.Close()
					_ = os.Remove(dest)
				}
			}
		} else {
			application.Get().Event.Emit(EventEnsureStart, struct{}{})
			application.Get().Event.Emit(EventEnsureDone, true)
			return
		}
	}
	application.Get().Event.Emit(EventEnsureStart, struct{}{})
	c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var downloadURL string
	client := github.NewClient(httpx.DefaultClient())
	rel, _, err := client.Repositories.GetLatestRelease(c, "LiteLDev", "vcproxy")
	if err == nil && rel != nil {
		for _, asset := range rel.Assets {
			if strings.EqualFold(asset.GetName(), "vcruntime140_1.dll") {
				downloadURL = asset.GetBrowserDownloadURL()
				break
			}
		}
	} else if err != nil {
		log.Printf("vcruntime.EnsureLatest: 获取最新 release 失败: %v", err)
	}
	if downloadURL == "" {
		downloadURL = "https://github.com/LiteLDev/vcproxy/releases/download/v1.0.0/vcruntime140_1.dll"
	}
	req, err := http.NewRequestWithContext(c, "GET", downloadURL, nil)
	if err != nil {
		log.Printf("vcruntime.EnsureLatest: 构造请求失败: %v", err)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	httpx.ApplyDefaultHeaders(req)
	resp, err := httpx.Do(req)
	if err != nil {
		log.Printf("vcruntime.EnsureLatest: 请求失败: %v", err)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("vcruntime.EnsureLatest: HTTP %s", resp.Status)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	application.Get().Event.Emit(EventEnsureProgress, EnsureProgress{Downloaded: 0, Total: resp.ContentLength})
	_ = os.Remove(tmp)
	f, err := os.Create(tmp)
	if err != nil {
		log.Printf("vcruntime.EnsureLatest: 创建文件失败: %v", err)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	defer f.Close()
	var downloaded int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				log.Printf("vcruntime.EnsureLatest: 写入失败: %v", werr)
				application.Get().Event.Emit(EventEnsureDone, false)
				return
			}
			downloaded += int64(n)
			application.Get().Event.Emit(EventEnsureProgress, EnsureProgress{Downloaded: downloaded, Total: resp.ContentLength})
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			log.Printf("vcruntime.EnsureLatest: 读取失败: %v", rerr)
			application.Get().Event.Emit(EventEnsureDone, false)
			return
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		log.Printf("vcruntime.EnsureLatest: 移动到目标失败: %v", err)
		if in, e1 := os.Open(tmp); e1 == nil {
			defer in.Close()
			if out, e2 := os.Create(dest); e2 == nil {
				if _, e3 := io.Copy(out, in); e3 == nil {
					out.Close()
					_ = os.Remove(tmp)
					log.Printf("vcruntime.EnsureLatest: 复制回退成功: %s", dest)
					application.Get().Event.Emit(EventEnsureDone, true)
					return
				}
				out.Close()
				_ = os.Remove(dest)
			}
		}
		_ = os.Remove(tmp)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	log.Printf("vcruntime.EnsureLatest: 已下载 vcruntime140_1.dll 到 %s", dest)
	application.Get().Event.Emit(EventEnsureDone, true)
}

func EnsureEmbedded(contentDir string, embedded []byte) {
	if strings.TrimSpace(contentDir) == "" {
		return
	}
	dest := filepath.Join(contentDir, "vcruntime140_1.dll")
	if _, err := os.Stat(dest); err == nil {
		return
	}
	if len(embedded) == 0 {
		return
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, embedded, 0o644); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return
	}
}
