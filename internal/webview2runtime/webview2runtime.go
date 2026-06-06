package webview2runtime

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	win "golang.org/x/sys/windows"
	winreg "golang.org/x/sys/windows/registry"
)

const (
	webView2ClientID       = "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
	webView2DownloadURL    = "https://go.microsoft.com/fwlink/p/?LinkId=2124703"
	webView2InstallerName  = "MicrosoftEdgeWebview2Setup.exe"
	webView2HklmClientPath = "SOFTWARE\\WOW6432Node\\Microsoft\\EdgeUpdate\\Clients\\" + webView2ClientID
	webView2HkcuClientPath = "Software\\Microsoft\\EdgeUpdate\\Clients\\" + webView2ClientID
)

const (
	messageBoxOK              = 0x00000000
	messageBoxOKCancel        = 0x00000001
	messageBoxIconError       = 0x00000010
	messageBoxIconInformation = 0x00000040
	messageBoxDefaultButton1  = 0x00000000
	messageBoxResultOK        = 1
)

type webView2RegistryEntry struct {
	root        winreg.Key
	path        string
	accessModes []uint32
}

var (
	webView2RegistryEntries = []webView2RegistryEntry{
		{
			root:        winreg.LOCAL_MACHINE,
			path:        webView2HklmClientPath,
			accessModes: []uint32{winreg.READ | winreg.WOW64_64KEY, winreg.READ},
		},
		{
			root:        winreg.CURRENT_USER,
			path:        webView2HkcuClientPath,
			accessModes: []uint32{winreg.READ | winreg.WOW64_64KEY, winreg.READ},
		},
	}

	user32MessageBox             = win.NewLazySystemDLL("user32.dll")
	procWebView2MessageBoxW      = user32MessageBox.NewProc("MessageBoxW")
	kernel32Locale               = win.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultUILanguage = kernel32Locale.NewProc("GetUserDefaultUILanguage")
)

func messageBox(title, message string, flags uint32) int32 {
	titlePtr, err := win.UTF16PtrFromString(title)
	if err != nil {
		log.Printf("webview2runtime: failed to encode message box title: %v", err)
		return 0
	}
	messagePtr, err := win.UTF16PtrFromString(message)
	if err != nil {
		log.Printf("webview2runtime: failed to encode message box body: %v", err)
		return 0
	}
	r, _, _ := procWebView2MessageBoxW.Call(
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

func webView2StartupRequiredTitle() string {
	if isChineseWindowsUI() {
		return "LeviLauncher - 需要 WebView2 Runtime"
	}
	return "LeviLauncher - WebView2 Runtime Required"
}

func webView2StartupDownloadTitle() string {
	if isChineseWindowsUI() {
		return "LeviLauncher - 正在下载 WebView2 Runtime"
	}
	return "LeviLauncher - Downloading WebView2 Runtime"
}

func webView2StartupInstallPromptMessage() string {
	if isChineseWindowsUI() {
		return "LeviLauncher 需要 Microsoft Edge WebView2 Runtime 才能正常启动。\n\n点击“确定”下载并打开 Microsoft 安装程序。安装完成后，请重新启动 LeviLauncher。"
	}
	return "LeviLauncher requires Microsoft Edge WebView2 Runtime to start correctly.\n\nClick OK to download and open the Microsoft installer. After the installation finishes, restart LeviLauncher."
}

func webView2StartupDownloadMessage() string {
	if isChineseWindowsUI() {
		return "LeviLauncher 将下载 Microsoft Edge WebView2 Runtime 安装程序。请稍候，下载完成后会打开安装程序窗口。"
	}
	return "LeviLauncher will download the Microsoft Edge WebView2 Runtime installer now. Please wait; another installer window will open when the download finishes."
}

func webView2StartupDownloadFailedMessage(err error) string {
	if isChineseWindowsUI() {
		return fmt.Sprintf("下载 Microsoft Edge WebView2 Runtime 失败。\n\n错误:\n%v", err)
	}
	return fmt.Sprintf("Failed to download Microsoft Edge WebView2 Runtime.\n\nError:\n%v", err)
}

func webView2StartupOpenInstallerFailedMessage(installerPath string, err error) string {
	if isChineseWindowsUI() {
		return fmt.Sprintf("打开 Microsoft Edge WebView2 安装程序失败。\n\n安装程序路径:\n%s\n\n错误:\n%v", installerPath, err)
	}
	return fmt.Sprintf("Failed to open the Microsoft Edge WebView2 installer.\n\nInstaller path:\n%s\n\nError:\n%v", installerPath, err)
}

func webView2StartupInstallerOpenedMessage() string {
	if isChineseWindowsUI() {
		return "Microsoft Edge WebView2 安装程序已打开。请完成安装，然后重新启动 LeviLauncher。"
	}
	return "The Microsoft Edge WebView2 installer has been opened. Finish the installation, then restart LeviLauncher."
}

func showWebView2StartupInstallPrompt() bool {
	result := messageBox(
		webView2StartupRequiredTitle(),
		webView2StartupInstallPromptMessage(),
		messageBoxOKCancel|messageBoxIconInformation|messageBoxDefaultButton1,
	)
	return result == messageBoxResultOK
}

func showWebView2StartupDownloadMessage() {
	messageBox(
		webView2StartupDownloadTitle(),
		webView2StartupDownloadMessage(),
		messageBoxOK|messageBoxIconInformation,
	)
}

func showWebView2StartupError(message string) {
	messageBox(
		webView2StartupRequiredTitle(),
		message,
		messageBoxOK|messageBoxIconError,
	)
}

func showWebView2StartupInfo(message string) {
	messageBox(
		webView2StartupRequiredTitle(),
		message,
		messageBoxOK|messageBoxIconInformation,
	)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func webView2InstallerPath() (string, string) {
	dir, _ := apppath.InstallersDir()
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "LeviLauncher", "Installers")
	}
	return dir, filepath.Join(dir, webView2InstallerName)
}

func downloadWebView2Installer(ctx context.Context, destPath string) error {
	if strings.TrimSpace(destPath) == "" {
		return fmt.Errorf("empty WebView2 installer path")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	tmpPath := destPath + ".part"

	req, err := http.NewRequestWithContext(ctx, "GET", webView2DownloadURL, nil)
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
		return fmt.Errorf("download WebView2 installer failed: %s", resp.Status)
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
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
	return nil
}

func runWebView2InstallerVisible(installerPath string) error {
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

func readWebView2RegistryVersion(root winreg.Key, path string, accessModes []uint32) (string, bool) {
	for _, access := range accessModes {
		key, err := winreg.OpenKey(root, path, access)
		if err != nil {
			continue
		}
		version, _, err := key.GetStringValue("pv")
		_ = key.Close()
		if err != nil {
			continue
		}
		return version, true
	}
	return "", false
}

func isWebView2RegistryVersionInstalled(version string) bool {
	version = strings.TrimSpace(version)
	return version != "" && version != "0.0.0.0"
}

func hasInstalledWebView2Registry(entries []webView2RegistryEntry, readVersion func(winreg.Key, string, []uint32) (string, bool)) bool {
	for _, entry := range entries {
		version, ok := readVersion(entry.root, entry.path, entry.accessModes)
		if ok && isWebView2RegistryVersionInstalled(version) {
			return true
		}
	}
	return false
}

func IsInstalled() bool {
	return hasInstalledWebView2Registry(webView2RegistryEntries, readWebView2RegistryVersion)
}

func EnsureStartupInteractive(ctx context.Context) bool {
	if IsInstalled() {
		return true
	}
	if !showWebView2StartupInstallPrompt() {
		return false
	}

	_, installerPath := webView2InstallerPath()
	if !fileExists(installerPath) {
		showWebView2StartupDownloadMessage()
		if err := downloadWebView2Installer(ctx, installerPath); err != nil {
			log.Printf("webview2runtime: startup download failed: %v", err)
			showWebView2StartupError(webView2StartupDownloadFailedMessage(err))
			return false
		}
	}

	if err := runWebView2InstallerVisible(installerPath); err != nil {
		log.Printf("webview2runtime: failed to open visible installer: %v", err)
		showWebView2StartupError(webView2StartupOpenInstallerFailedMessage(installerPath, err))
		return false
	}

	showWebView2StartupInfo(webView2StartupInstallerOpenedMessage())
	return false
}
