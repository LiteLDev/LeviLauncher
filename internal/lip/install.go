package lip

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	semver "github.com/Masterminds/semver/v3"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
	winreg "golang.org/x/sys/windows/registry"
)

const (
	EventLipInstallStatus   = "lip_install_status"
	EventLipInstallProgress = "lip_install_progress"
	EventLipInstallDone     = "lip_install_done"
	EventLipInstallError    = "lip_install_error"

	lipMirrorMetadataAPI         = "https://mirrors.cloud.tencent.com/npm/@futrime/lip"
	lipMirrorTarballHost         = "mirrors.cloud.tencent.com"
	lipMirrorTarballPathPrefix   = "/npm/@futrime/lip/"
	lipOfficialMetadataAPI       = "https://registry.npmjs.org/@futrime/lip"
	lipOfficialTarballHost       = "registry.npmjs.org"
	lipOfficialTarballPathPrefix = "/@futrime/lip/-/"
	lipDotNetInstallScriptWin    = "https://dot.net/v1/dotnet-install.ps1"
	lipDotNetInstallScriptUnix   = "https://dot.net/v1/dotnet-install.sh"
	lipDotNetRuntimeName         = "Microsoft.NETCore.App"
	lipDotNetChannel             = "10.0"
	lipDotNetMajor               = 10
	lipBinaryName                = "lipd.exe"
	lipArchiveEntryPath          = "package/win32-x64/lipd.exe"
	lipInstallTimeout            = 3 * time.Minute
	lipDotNetInstallTimeout      = 3 * time.Minute
	lipStatusTimeout             = 20 * time.Second
	lipVersionRPCTimeout         = 15 * time.Second

	lipConfigFormatVersion = 3
	lipConfigFormatUUID    = "289f771f-2c9a-4d73-9f3f-8492495a924d"
	lipConfigGithubProxy   = "https://github.bibk.top"
	lipConfigGoModuleProxy = "https://goproxy.cn"
)

var lipRuntimeConfigMu sync.Mutex

type Status struct {
	Path           string `json:"path"`
	Installed      bool   `json:"installed"`
	UpToDate       bool   `json:"upToDate"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Error          string `json:"error"`
}

type lipPackageDist struct {
	Tarball string `json:"tarball"`
}

type lipPackageVersion struct {
	Dist lipPackageDist `json:"dist"`
}

type lipPackageMetadata struct {
	DistTags map[string]string            `json:"dist-tags"`
	Versions map[string]lipPackageVersion `json:"versions"`
}

type lipPackageSource struct {
	MetadataURL       string
	TarballHost       string
	TarballPathPrefix string
}

func appExeName() string {
	exeName := "levilauncher.exe"
	if exe, err := os.Executable(); err == nil {
		if b := strings.TrimSpace(filepath.Base(exe)); b != "" {
			exeName = strings.ToLower(b)
		}
	}
	return exeName
}

func binDir() string {
	return filepath.Join(apppath.AppData(), appExeName(), "bin")
}

func lipDir() string {
	return filepath.Join(binDir(), "lip")
}

func desiredLipExePath() string {
	return filepath.Join(lipDir(), lipBinaryName)
}

func legacyLipExePath() string {
	return filepath.Join(binDir(), lipBinaryName)
}

func lipConfigPath() string {
	return filepath.Join(apppath.AppData(), "lip", "liprc.json")
}

func desiredLipRuntimeConfig() map[string]any {
	return map[string]any{
		"format_version":  lipConfigFormatVersion,
		"format_uuid":     lipConfigFormatUUID,
		"github_proxy":    lipConfigGithubProxy,
		"go_module_proxy": lipConfigGoModuleProxy,
	}
}

func isZhCnLocaleValue(v string) bool {
	s := strings.ToLower(strings.TrimSpace(v))
	if s == "" {
		return false
	}
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.TrimPrefix(s, "0x")
	if strings.HasPrefix(s, "zh-cn") || s == "zh" || s == "zh-hans" {
		return true
	}
	if strings.Contains(s, "chinese (simplified") {
		return true
	}
	return s == "804" || s == "0804" || s == "00000804"
}

func isWindowsTimeZoneChina() bool {
	k, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\TimeZoneInformation`, winreg.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	for _, field := range []string{"TimeZoneKeyName", "StandardName"} {
		if val, _, err := k.GetStringValue(field); err == nil {
			if strings.EqualFold(strings.TrimSpace(val), "China Standard Time") {
				return true
			}
		}
	}
	return false
}

func isWindowsLocaleZhCN() bool {
	k, err := winreg.OpenKey(winreg.CURRENT_USER, `Control Panel\International`, winreg.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		for _, field := range []string{"LocaleName", "Locale", "sLanguage"} {
			if val, _, err := k.GetStringValue(field); err == nil && isZhCnLocaleValue(val) {
				return true
			}
		}
	}
	for _, key := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		if isZhCnLocaleValue(os.Getenv(key)) {
			return true
		}
	}
	return false
}

func isChinaUser() bool {
	if runtime.GOOS == "windows" {
		return isWindowsTimeZoneChina() || isWindowsLocaleZhCN()
	}

	isTz := false
	isLang := false
	if b, err := os.ReadFile("/etc/timezone"); err == nil {
		s := strings.ToLower(strings.TrimSpace(string(b)))
		if s == "asia/shanghai" || s == "asia/urumqi" {
			isTz = true
		}
	} else if p, err := os.Readlink("/etc/localtime"); err == nil {
		low := strings.ToLower(p)
		if strings.Contains(low, "asia/shanghai") || strings.Contains(low, "asia/urumqi") {
			isTz = true
		}
	}

	for _, key := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "zh_cn") || strings.HasPrefix(v, "zh-cn") || v == "zh" {
			isLang = true
			break
		}
	}

	return isTz || isLang
}

func ensureLipRuntimeConfig() error {
	if !isChinaUser() {
		return nil
	}

	lipRuntimeConfigMu.Lock()
	defer lipRuntimeConfigMu.Unlock()

	configPath := lipConfigPath()
	expected := desiredLipRuntimeConfig()
	config := map[string]any{}

	if content, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(content, &config); err != nil {
			log.Printf("lip: reset invalid runtime config %s: %v", configPath, err)
			config = map[string]any{}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("ERR_LIP_READ_CONFIG: %w", err)
	}

	needsWrite := false
	for key, value := range expected {
		if fmt.Sprint(config[key]) == fmt.Sprint(value) {
			continue
		}
		config[key] = value
		needsWrite = true
	}

	if !needsWrite {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("ERR_LIP_CREATE_CONFIG_DIR: %w", err)
	}

	content, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("ERR_LIP_MARSHAL_CONFIG: %w", err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		return fmt.Errorf("ERR_LIP_WRITE_CONFIG: %w", err)
	}
	return nil
}

func normalizeLipProcPath(path string) string {
	trimmed := strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
	trimmed = strings.TrimPrefix(trimmed, `\\?\`)
	trimmed = strings.TrimPrefix(trimmed, `\??\`)
	return trimmed
}

func killRunningLipdProcesses() error {
	targets := map[string]struct{}{
		normalizeLipProcPath(desiredLipExePath()): {},
		normalizeLipProcPath(legacyLipExePath()):  {},
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil
	}

	var firstErr error
	for {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), lipBinaryName) {
			h, err := windows.OpenProcess(
				windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE,
				false,
				entry.ProcessID,
			)
			if err == nil {
				buf := make([]uint16, 1024)
				size := uint32(len(buf))
				if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err == nil && size > 0 {
					exePath := normalizeLipProcPath(windows.UTF16ToString(buf[:size]))
					if _, ok := targets[exePath]; ok {
						if err := windows.TerminateProcess(h, 1); err != nil && firstErr == nil {
							firstErr = err
						}
					}
				}
				_ = windows.CloseHandle(h)
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return firstErr
}

func restartLipdAndGetVersion(ctx context.Context) (string, error) {
	if err := killRunningLipdProcesses(); err != nil {
		log.Printf("lip: kill running lipd failed: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	return getVersionWithError(ctx)
}

func LipExePath() string {
	if utils.FileExists(desiredLipExePath()) {
		return desiredLipExePath()
	}
	if utils.FileExists(legacyLipExePath()) {
		return legacyLipExePath()
	}
	return desiredLipExePath()
}

func IsInstalled() bool {
	return utils.FileExists(LipExePath()) && isLipDotNetRuntimeInstalled()
}

func normalizeLipVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	return strings.TrimSpace(trimmed)
}

func parseLipVersion(value string) (*semver.Version, error) {
	return semver.NewVersion(normalizeLipVersion(value))
}

func isLipVersionCurrent(localVersion string, latestVersion string) bool {
	local := normalizeLipVersion(localVersion)
	latest := normalizeLipVersion(latestVersion)
	if local == "" || latest == "" {
		return false
	}
	localSemver, localErr := parseLipVersion(local)
	latestSemver, latestErr := parseLipVersion(latest)
	if localErr == nil && latestErr == nil {
		return !localSemver.LessThan(latestSemver)
	}
	return strings.EqualFold(local, latest)
}

func currentLipPackageSource() lipPackageSource {
	if isChinaUser() {
		return lipPackageSource{
			MetadataURL:       lipMirrorMetadataAPI,
			TarballHost:       lipMirrorTarballHost,
			TarballPathPrefix: lipMirrorTarballPathPrefix,
		}
	}

	return lipPackageSource{
		MetadataURL:       lipOfficialMetadataAPI,
		TarballHost:       lipOfficialTarballHost,
		TarballPathPrefix: lipOfficialTarballPathPrefix,
	}
}

func lookupLipPackageVersion(versions map[string]lipPackageVersion, latestTag string) (lipPackageVersion, bool) {
	for _, candidate := range []string{
		strings.TrimSpace(latestTag),
		normalizeLipVersion(latestTag),
	} {
		if candidate == "" {
			continue
		}
		if version, ok := versions[candidate]; ok {
			return version, true
		}
	}
	return lipPackageVersion{}, false
}

func isAllowedLipTarballURL(rawURL string, source lipPackageSource) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}
	if !strings.EqualFold(parsed.Hostname(), source.TarballHost) {
		return false
	}
	return strings.HasPrefix(parsed.Path, source.TarballPathPrefix)
}

func resolveLatestTarball(meta lipPackageMetadata, source lipPackageSource) (string, string, error) {
	latestTag := strings.TrimSpace(meta.DistTags["latest"])
	latestVersion := normalizeLipVersion(latestTag)
	if latestVersion == "" {
		return "", "", fmt.Errorf("ERR_LIP_FETCH_RELEASE: missing dist-tags.latest")
	}

	version, ok := lookupLipPackageVersion(meta.Versions, latestTag)
	if !ok {
		return "", "", fmt.Errorf("ERR_LIP_RELEASE_ASSET_NOT_FOUND: version %q not found in metadata", latestVersion)
	}

	tarballURL := strings.TrimSpace(version.Dist.Tarball)
	if tarballURL == "" {
		return "", "", fmt.Errorf("ERR_LIP_RELEASE_ASSET_NOT_FOUND: missing tarball for %q", latestVersion)
	}
	if !isAllowedLipTarballURL(tarballURL, source) {
		return "", "", fmt.Errorf("ERR_LIP_RELEASE_ASSET_NOT_FOUND: unsupported tarball URL %q", tarballURL)
	}
	return latestVersion, tarballURL, nil
}

func fetchLatestRelease(ctx context.Context) (string, string, error) {
	source := currentLipPackageSource()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.MetadataURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("ERR_LIP_FETCH_RELEASE: %w", err)
	}
	httpx.ApplyDefaultHeaders(req)

	resp, err := httpx.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("ERR_LIP_FETCH_RELEASE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("ERR_LIP_FETCH_RELEASE: HTTP %d", resp.StatusCode)
	}

	var meta lipPackageMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", "", fmt.Errorf("ERR_LIP_FETCH_RELEASE: %w", err)
	}

	return resolveLatestTarball(meta, source)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func lipDotNetDefaultRoot() string {
	if custom := strings.TrimSpace(os.Getenv("DOTNET_INSTALL_DIR")); custom != "" {
		return custom
	}

	homeDir, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		localAppData := firstNonEmpty(
			os.Getenv("LocalAppData"),
			os.Getenv("LOCALAPPDATA"),
		)
		if localAppData == "" && homeDir != "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		if localAppData == "" {
			localAppData = filepath.Join(os.TempDir(), "Microsoft")
		}
		return filepath.Join(localAppData, "Microsoft", "dotnet")
	}

	if homeDir == "" {
		return filepath.Join(os.TempDir(), ".dotnet")
	}
	return filepath.Join(homeDir, ".dotnet")
}

func lipDotNetSharedRuntimeDir(root string) string {
	return filepath.Join(strings.TrimSpace(root), "shared", lipDotNetRuntimeName)
}

func hasLipDotNetMajorInRoot(root string) bool {
	runtimeDir := lipDotNetSharedRuntimeDir(root)
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		return false
	}

	prefix := fmt.Sprintf("%d.", lipDotNetMajor)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func hasLipDotNetMajorInListRuntimesOutput(output string) bool {
	if strings.TrimSpace(output) == "" {
		return false
	}

	prefix := fmt.Sprintf("%s %d.", lipDotNetRuntimeName, lipDotNetMajor)
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return true
		}
	}
	return false
}

func findLipDotNetRoot() string {
	if dotnetRoot := strings.TrimSpace(os.Getenv("DOTNET_ROOT")); dotnetRoot != "" && hasLipDotNetMajorInRoot(dotnetRoot) {
		return dotnetRoot
	}

	defaultRoot := lipDotNetDefaultRoot()
	if hasLipDotNetMajorInRoot(defaultRoot) {
		return defaultRoot
	}

	return ""
}

func upsertEnv(env []string, key string, value string) []string {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return append([]string{}, env...)
	}

	out := append([]string{}, env...)
	prefix := key + "="
	for index, entry := range out {
		name := entry
		if cut := strings.Index(entry, "="); cut >= 0 {
			name = entry[:cut]
		}
		if strings.EqualFold(strings.TrimSpace(name), key) {
			out[index] = prefix + value
			return out
		}
	}
	return append(out, prefix+value)
}

func lipCommandEnv() []string {
	env := os.Environ()
	if dotnetRoot := findLipDotNetRoot(); dotnetRoot != "" {
		return upsertEnv(env, "DOTNET_ROOT", dotnetRoot)
	}
	return env
}

func isLipDotNetRuntimeInstalled() bool {
	if findLipDotNetRoot() != "" {
		return true
	}

	cmd := exec.Command("dotnet", "--list-runtimes")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return hasLipDotNetMajorInListRuntimesOutput(string(output))
}

func lipDotNetInstallScriptURL() string {
	if runtime.GOOS == "windows" {
		return lipDotNetInstallScriptWin
	}
	return lipDotNetInstallScriptUnix
}

func lipDotNetInstallScriptPath() string {
	dir, err := apppath.InstallersDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		dir = filepath.Join(os.TempDir(), "LeviLauncher", "Installers")
	}
	_ = os.MkdirAll(dir, 0o755)

	fileName := "dotnet-install.sh"
	if runtime.GOOS == "windows" {
		fileName = "dotnet-install.ps1"
	}
	return filepath.Join(dir, fileName)
}

func downloadLipDotNetInstallScript(ctx context.Context, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lipDotNetInstallScriptURL(), nil)
	if err != nil {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_DOTNET_INSTALLER: %w", err)
	}
	httpx.ApplyDefaultHeaders(req)

	resp, err := httpx.Do(req)
	if err != nil {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_DOTNET_INSTALLER: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_DOTNET_INSTALLER: HTTP %d", resp.StatusCode)
	}

	tmp := dest + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_DOTNET_INSTALLER: %w", err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("ERR_LIP_DOWNLOAD_DOTNET_INSTALLER: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ERR_LIP_DOWNLOAD_DOTNET_INSTALLER: %w", err)
	}

	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ERR_LIP_DOWNLOAD_DOTNET_INSTALLER: %w", err)
	}
	return nil
}

func resolvePowerShellBinary() string {
	if resolved, err := exec.LookPath("pwsh.exe"); err == nil && strings.TrimSpace(resolved) != "" {
		return resolved
	}
	return "powershell.exe"
}

func ensureLipDotNetRuntimeWithError(ctx context.Context) (bool, error) {
	if isLipDotNetRuntimeInstalled() {
		return false, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	installCtx := ctx
	cancel := func() {}
	if _, hasDeadline := installCtx.Deadline(); !hasDeadline {
		installCtx, cancel = context.WithTimeout(installCtx, lipDotNetInstallTimeout)
	}
	defer cancel()

	scriptPath := lipDotNetInstallScriptPath()
	if err := downloadLipDotNetInstallScript(installCtx, scriptPath); err != nil {
		return false, err
	}

	installRoot := lipDotNetDefaultRoot()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(
			installCtx,
			resolvePowerShellBinary(),
			"-NoLogo",
			"-NoProfile",
			"-NonInteractive",
			"-ExecutionPolicy",
			"Bypass",
			"-File",
			scriptPath,
			"-Runtime",
			"dotnet",
			"-Channel",
			lipDotNetChannel,
			"-InstallDir",
			installRoot,
			"-NoPath",
		)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	} else {
		cmd = exec.CommandContext(
			installCtx,
			"bash",
			scriptPath,
			"--runtime",
			"dotnet",
			"--channel",
			lipDotNetChannel,
			"--install-dir",
			installRoot,
			"--no-path",
		)
	}
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput != "" {
			return true, fmt.Errorf("ERR_LIP_INSTALL_DOTNET_RUNTIME: %w: %s", err, trimmedOutput)
		}
		return true, fmt.Errorf("ERR_LIP_INSTALL_DOTNET_RUNTIME: %w", err)
	}

	if !isLipDotNetRuntimeInstalled() {
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput != "" {
			return true, fmt.Errorf("ERR_LIP_VERIFY_DOTNET_RUNTIME: %s", trimmedOutput)
		}
		return true, fmt.Errorf("ERR_LIP_VERIFY_DOTNET_RUNTIME")
	}

	return true, nil
}

func extractErrCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "ERR_LIP_INSTALL_FAILED"
	}
	if idx := strings.Index(msg, ":"); idx > 0 {
		head := strings.TrimSpace(msg[:idx])
		if strings.HasPrefix(head, "ERR_") {
			return head
		}
	}
	if strings.HasPrefix(msg, "ERR_") {
		return msg
	}
	return "ERR_LIP_INSTALL_FAILED"
}

func ErrorCode(err error) string {
	return extractErrCode(err)
}

func emit(event string, data any) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(event, data)
}

func emitInstallProgress(percentage float64, current int64, total int64) {
	emit(EventLipInstallProgress, map[string]interface{}{
		"percentage": percentage,
		"current":    current,
		"total":      total,
	})
}

func EnsureInstalledWithError() error {
	if !IsInstalled() {
		return fmt.Errorf("ERR_LIP_NOT_INSTALLED")
	}
	return nil
}

func getVersionWithError(ctx context.Context) (string, error) {
	if err := EnsureInstalledWithError(); err != nil {
		return "", err
	}
	versionCtx, cancel := context.WithTimeout(ctx, lipVersionRPCTimeout)
	defer cancel()

	cmd := exec.CommandContext(versionCtx, LipExePath(), "--version")
	cmd.Dir = filepath.Dir(LipExePath())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Env = lipCommandEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput != "" {
			return "", fmt.Errorf("read lip version: %w: %s", err, trimmedOutput)
		}
		return "", fmt.Errorf("read lip version: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return "", fmt.Errorf("read lip version: empty result")
	}

	candidates := []string{raw}
	for _, field := range strings.Fields(raw) {
		candidates = append(candidates, field)
	}
	for _, candidate := range candidates {
		normalized := normalizeLipVersion(candidate)
		if normalized == "" {
			continue
		}
		if _, err := parseLipVersion(normalized); err == nil {
			return normalized, nil
		}
	}

	return "", fmt.Errorf("read lip version: unsupported output %q", raw)
}

func GetVersion() string {
	version, err := getVersionWithError(context.Background())
	if err != nil {
		return ""
	}
	return version
}

func GetLatestVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), lipStatusTimeout)
	defer cancel()

	version, _, err := fetchLatestRelease(ctx)
	if err != nil {
		return "", err
	}
	return version, nil
}

func FetchPackageReadme(projectURL string) (string, error) {
	trimmedURL := strings.TrimSpace(projectURL)
	if trimmedURL == "" {
		return "", nil
	}

	parsed, err := url.Parse(trimmedURL)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") && !strings.EqualFold(parsed.Hostname(), "www.github.com") {
		return "", nil
	}

	parts := strings.Split(parsed.Path, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			segments = append(segments, trimmed)
		}
	}
	if len(segments) < 2 {
		return "", nil
	}

	owner := segments[0]
	repo := strings.TrimSuffix(segments[1], ".git")
	if owner == "" || repo == "" {
		return "", nil
	}

	candidates := []string{
		fmt.Sprintf("https://github.bibk.top/%s/%s/raw/refs/heads/main/README.md", owner, repo),
		fmt.Sprintf("https://github.bibk.top/%s/%s/raw/refs/heads/master/README.md", owner, repo),
		fmt.Sprintf("https://github.bibk.top/%s/%s/raw/refs/heads/main/readme.md", owner, repo),
		fmt.Sprintf("https://github.bibk.top/%s/%s/raw/refs/heads/master/readme.md", owner, repo),
	}

	ctx, cancel := context.WithTimeout(context.Background(), lipStatusTimeout)
	defer cancel()

	var lastErr error
	for _, candidate := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			lastErr = err
			continue
		}
		httpx.ApplyDefaultHeaders(req)

		resp, err := httpx.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				return
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				lastErr = err
				return
			}
			lastErr = nil
			trimmedBody := strings.TrimSpace(string(body))
			if trimmedBody != "" {
				trimmedURL = string(body)
			}
		}()
		if lastErr == nil {
			return trimmedURL, nil
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", nil
}

func downloadArchive(ctx context.Context, rawURL string, dest string) error {
	candidate := strings.TrimSpace(rawURL)
	if candidate == "" {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: empty download URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
	if err != nil {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: %w", err)
	}
	httpx.ApplyDefaultHeaders(req)

	resp, err := httpx.Do(req)
	if err != nil {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: HTTP %d", resp.StatusCode)
	}

	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: %w", err)
	}

	total := resp.ContentLength
	if total < 0 {
		total = 0
	}
	emitInstallProgress(20, 0, total)

	buffer := make([]byte, 128*1024)
	var downloaded int64
	lastEmit := time.Now()

	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			written, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				_ = out.Close()
				_ = os.Remove(tmp)
				return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: %w", writeErr)
			}
			downloaded += int64(written)
			percentage := 50.0
			if total > 0 {
				percentage = 20 + (float64(downloaded)/float64(total))*60
			}
			if percentage > 80 {
				percentage = 80
			}
			if time.Since(lastEmit) >= 120*time.Millisecond {
				emitInstallProgress(percentage, downloaded, total)
				lastEmit = time.Now()
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				_ = out.Close()
				_ = os.Remove(tmp)
				return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: %w", readErr)
			}
			break
		}
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: %w", err)
	}
	emitInstallProgress(80, downloaded, total)

	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: %w", err)
	}
	return nil
}

func extractLipBinary(archivePath string, dest string) error {
	reader, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("ERR_LIP_OPEN_ARCHIVE: %w", err)
	}
	defer reader.Close()

	return extractLipBinaryFromTarGz(reader, dest)
}

func extractLipBinaryFromTarGz(source io.Reader, dest string) error {
	gzipReader, err := gzip.NewReader(source)
	if err != nil {
		return fmt.Errorf("ERR_LIP_OPEN_ARCHIVE: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %w", err)
		}
		if header == nil || !header.FileInfo().Mode().IsRegular() {
			continue
		}
		if !strings.EqualFold(path.Clean(strings.TrimSpace(header.Name)), lipArchiveEntryPath) {
			continue
		}

		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %w", err)
		}

		_, copyErr := io.Copy(out, tarReader)
		closeErr := out.Close()
		if copyErr != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %w", copyErr)
		}
		if closeErr != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %w", closeErr)
		}
		return nil
	}

	return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %s not found", lipArchiveEntryPath)
}

func EnsureLatestWithError(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	installCtx, cancel := context.WithTimeout(ctx, lipInstallTimeout)
	defer cancel()

	emit(EventLipInstallStatus, "checking_runtime")
	emitInstallProgress(12, 0, 0)
	runtimeWasMissing := !isLipDotNetRuntimeInstalled()
	if runtimeWasMissing {
		emit(EventLipInstallStatus, "installing_runtime")
		emitInstallProgress(18, 0, 0)
	}
	if _, err := ensureLipDotNetRuntimeWithError(installCtx); err != nil {
		return err
	}

	latestVersion, archiveURL, err := fetchLatestRelease(installCtx)
	if err != nil {
		return err
	}

	if currentVersion, versionErr := getVersionWithError(installCtx); versionErr == nil && isLipVersionCurrent(currentVersion, latestVersion) {
		return nil
	}

	target := desiredLipExePath()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("ERR_LIP_CREATE_TARGET_DIR: %w", err)
	}

	archivePath := filepath.Join(filepath.Dir(target), "lip-latest-win32-x64.tgz")
	_ = os.Remove(archivePath)
	defer os.Remove(archivePath)

	emit(EventLipInstallStatus, "downloading")
	if runtimeWasMissing {
		emitInstallProgress(20, 0, 0)
	}

	if err := downloadArchive(installCtx, archiveURL, archivePath); err != nil {
		return err
	}

	emit(EventLipInstallStatus, "extracting")
	emitInstallProgress(85, 0, 0)

	tmpTarget := target + ".tmp"
	_ = os.Remove(tmpTarget)
	if err := extractLipBinary(archivePath, tmpTarget); err != nil {
		_ = os.Remove(tmpTarget)
		return err
	}

	if err := killRunningLipdProcesses(); err != nil {
		log.Printf("lip: stop lipd before replace failed: %v", err)
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpTarget)
		return fmt.Errorf("ERR_LIP_REPLACE_FILE: %w", err)
	}
	if err := os.Rename(tmpTarget, target); err != nil {
		_ = os.Remove(tmpTarget)
		return fmt.Errorf("ERR_LIP_REPLACE_FILE: %w", err)
	}

	if legacyPath := legacyLipExePath(); !strings.EqualFold(legacyPath, target) {
		_ = os.Remove(legacyPath)
	}

	installedVersion, err := restartLipdAndGetVersion(installCtx)
	if err != nil {
		return fmt.Errorf("ERR_LIP_VERIFY_INSTALL: %w", err)
	}
	if !isLipVersionCurrent(installedVersion, latestVersion) {
		return fmt.Errorf("ERR_LIP_VERIFY_INSTALL")
	}

	return nil
}

func EnsureLatest(ctx context.Context) {
	if err := EnsureLatestWithError(ctx); err != nil {
		log.Printf("lip: ensure latest failed: %v", err)
	}
}

func CheckStatus() Status {
	out := Status{Path: LipExePath()}
	exeExists := utils.FileExists(out.Path)
	runtimeInstalled := isLipDotNetRuntimeInstalled()

	if !exeExists {
		out.Path = desiredLipExePath()
	}

	if exeExists && !runtimeInstalled {
		out.Error = "ERR_LIP_VERIFY_DOTNET_RUNTIME"
	}

	if exeExists && runtimeInstalled {
		out.Installed = true
		ctx, cancel := context.WithTimeout(context.Background(), lipVersionRPCTimeout)
		currentVersion, versionErr := getVersionWithError(ctx)
		cancel()
		if versionErr != nil {
			log.Printf("lip: read version failed: %v", versionErr)
		} else {
			out.CurrentVersion = currentVersion
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), lipStatusTimeout)
	defer cancel()

	latestVersion, _, err := fetchLatestRelease(ctx)
	if err != nil {
		if out.Error == "" {
			out.Error = err.Error()
		} else {
			out.Error += "; " + err.Error()
		}
		return out
	}
	out.LatestVersion = latestVersion
	if out.Installed && out.CurrentVersion != "" {
		out.UpToDate = isLipVersionCurrent(out.CurrentVersion, out.LatestVersion)
	}
	return out
}

func Install() string {
	emit(EventLipInstallStatus, "checking")
	emitInstallProgress(5, 0, 0)

	status := CheckStatus()
	if status.Installed && status.UpToDate {
		emit(EventLipInstallStatus, "done")
		emitInstallProgress(100, 0, 0)
		emit(EventLipInstallDone, status.CurrentVersion)
		return ""
	}

	emit(EventLipInstallStatus, "preparing")
	emitInstallProgress(10, 0, 0)

	if err := EnsureLatestWithError(context.Background()); err != nil {
		code := extractErrCode(err)
		emit(EventLipInstallError, code)
		return code
	}

	emit(EventLipInstallStatus, "verifying")
	emitInstallProgress(95, 0, 0)

	verifyStatus := CheckStatus()
	if !verifyStatus.Installed || verifyStatus.CurrentVersion == "" || !verifyStatus.UpToDate {
		code := "ERR_LIP_VERIFY_INSTALL"
		emit(EventLipInstallError, code)
		return code
	}

	emit(EventLipInstallStatus, "done")
	emitInstallProgress(100, 0, 0)
	emit(EventLipInstallDone, verifyStatus.CurrentVersion)
	return ""
}
