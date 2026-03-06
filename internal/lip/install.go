package lip

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	semver "github.com/Masterminds/semver/v3"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

const (
	EventLipInstallStatus   = "lip_install_status"
	EventLipInstallProgress = "lip_install_progress"
	EventLipInstallDone     = "lip_install_done"
	EventLipInstallError    = "lip_install_error"

	lipReleaseAPI        = "https://api.github.com/repos/futrime/lip/releases/latest"
	lipBinaryName        = "lipd.exe"
	lipArchiveSuffix     = "-win-x64.zip"
	lipInstallTimeout    = 5 * time.Minute
	lipStatusTimeout     = 20 * time.Second
	lipVersionRPCTimeout = 15 * time.Second
)

type Status struct {
	Path           string `json:"path"`
	Installed      bool   `json:"installed"`
	UpToDate       bool   `json:"upToDate"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Error          string `json:"error"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type releaseResponse struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
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
	return utils.FileExists(LipExePath())
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

func releaseAPIs() []string {
	return []string{
		lipReleaseAPI,
		"https://cdn.gh-proxy.org/" + lipReleaseAPI,
		"https://edgeone.gh-proxy.org/" + lipReleaseAPI,
		"https://gh-proxy.org/" + lipReleaseAPI,
		"https://hk.gh-proxy.org/" + lipReleaseAPI,
		"https://ghproxy.vip/" + lipReleaseAPI,
	}
}

func mirroredDownloadURLs(rawURL string) []string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}
	return []string{
		trimmed,
		"https://cdn.gh-proxy.org/" + trimmed,
		"https://edgeone.gh-proxy.org/" + trimmed,
		"https://gh-proxy.org/" + trimmed,
		"https://hk.gh-proxy.org/" + trimmed,
		"https://ghproxy.vip/" + trimmed,
	}
}

func fetchLatestRelease(ctx context.Context) (string, string, error) {
	var lastErr error
	for _, api := range releaseAPIs() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
		if err != nil {
			lastErr = err
			continue
		}
		httpx.ApplyDefaultHeaders(req)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		var rel releaseResponse
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				return
			}
			if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
				lastErr = err
				return
			}
		}()
		if resp.StatusCode != http.StatusOK {
			continue
		}

		latestVersion := normalizeLipVersion(rel.TagName)
		if latestVersion == "" {
			lastErr = fmt.Errorf("empty tag_name")
			continue
		}

		expectedName := fmt.Sprintf("lip-%s%s", latestVersion, lipArchiveSuffix)
		for _, asset := range rel.Assets {
			if strings.EqualFold(strings.TrimSpace(asset.Name), expectedName) {
				return latestVersion, strings.TrimSpace(asset.BrowserDownloadURL), nil
			}
		}
		for _, asset := range rel.Assets {
			if strings.HasSuffix(strings.ToLower(strings.TrimSpace(asset.Name)), lipArchiveSuffix) {
				return latestVersion, strings.TrimSpace(asset.BrowserDownloadURL), nil
			}
		}
		lastErr = fmt.Errorf("asset %s not found", expectedName)
	}
	if lastErr != nil {
		if strings.Contains(strings.ToLower(lastErr.Error()), "asset") {
			return "", "", fmt.Errorf("ERR_LIP_RELEASE_ASSET_NOT_FOUND: %w", lastErr)
		}
		return "", "", fmt.Errorf("ERR_LIP_FETCH_RELEASE: %w", lastErr)
	}
	return "", "", fmt.Errorf("ERR_LIP_FETCH_RELEASE")
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

		resp, err := http.DefaultClient.Do(req)
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
	var lastErr error
	for _, candidate := range mirroredDownloadURLs(rawURL) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			lastErr = err
			continue
		}
		httpx.ApplyDefaultHeaders(req)

		resp, err := http.DefaultClient.Do(req)
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

			tmp := dest + ".tmp"
			out, err := os.Create(tmp)
			if err != nil {
				lastErr = err
				return
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
						lastErr = writeErr
						return
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
						lastErr = readErr
						return
					}
					break
				}
			}

			if err := out.Close(); err != nil {
				_ = os.Remove(tmp)
				lastErr = err
				return
			}
			emitInstallProgress(80, downloaded, total)

			_ = os.Remove(dest)
			if err := os.Rename(tmp, dest); err != nil {
				_ = os.Remove(tmp)
				lastErr = err
				return
			}
			lastErr = nil
		}()

		if lastErr == nil {
			return nil
		}
	}
	if lastErr != nil {
		return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE: %w", lastErr)
	}
	return fmt.Errorf("ERR_LIP_DOWNLOAD_ARCHIVE")
}

func extractLipBinary(archivePath string, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("ERR_LIP_OPEN_ARCHIVE: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Base(file.Name), lipBinaryName) {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %w", err)
		}

		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			rc.Close()
			return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %w", err)
		}

		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rcErr := rc.Close()
		if copyErr != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %w", copyErr)
		}
		if closeErr != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %w", closeErr)
		}
		if rcErr != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %w", rcErr)
		}
		return nil
	}

	return fmt.Errorf("ERR_LIP_EXTRACT_BINARY: %s not found", lipBinaryName)
}

func EnsureLatestWithError(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	installCtx, cancel := context.WithTimeout(ctx, lipInstallTimeout)
	defer cancel()

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

	archivePath := filepath.Join(filepath.Dir(target), "lip-latest-win-x64.zip")
	_ = os.Remove(archivePath)
	defer os.Remove(archivePath)

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
	if !utils.FileExists(out.Path) {
		out.Path = desiredLipExePath()
	}

	if utils.FileExists(LipExePath()) {
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
	emit(EventLipInstallStatus, "downloading")

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
