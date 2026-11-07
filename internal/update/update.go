package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	json "github.com/goccy/go-json"

	"github.com/Masterminds/semver/v3"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/mouuff/go-rocket-update/pkg/provider"
	"github.com/mouuff/go-rocket-update/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// devVersion is reported by builds that were not stamped with a version.
const devVersion = "0.0.0-dev"

// appVersion is stamped at build time from the release tag via
// -ldflags "-X github.com/liteldev/LeviLauncher/internal/update.appVersion=<version>".
var (
	appVersion = devVersion
	isBeta     bool
)

var (
	shell32            = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW  = shell32.NewProc("ShellExecuteW")
	swShowNormal       = uintptr(1)
	shellExecuteErrCut = uintptr(32)
)

type AppUpdateProgress struct {
	Phase      string `json:"phase"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
}

type githubRelease struct {
	TagName    string               `json:"tag_name"`
	Body       string               `json:"body"`
	Prerelease bool                 `json:"prerelease"`
	Assets     []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

const (
	EventAppUpdateStatus   = "app_update_status"
	EventAppUpdateProgress = "app_update_progress"
	EventAppUpdateError    = "app_update_error"
	updateErrorSeparator   = "::"
)

const (
	updateMetadataTimeout = 20 * time.Second
	updateChecksumTimeout = 20 * time.Second
	updateDownloadTimeout = 20 * time.Minute
)

func init() { setAppVersion(appVersion) }

// Init is kept for callers; the version is resolved once at package init.
func Init() {}

// setAppVersion normalizes a stamped version (trimming whitespace and a
// leading "v") and derives isBeta from its SemVer prerelease component.
// Anything that is not strict SemVer falls back to devVersion so that update
// checks always compare against a well-formed version.
func setAppVersion(raw string) {
	v := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	sv, err := semver.StrictNewVersion(v)
	if err != nil {
		if v != "" {
			log.Printf("invalid build version %q, using %s: %v", raw, devVersion, err)
		}
		v = devVersion
		sv = semver.MustParse(devVersion)
	}
	appVersion = v
	isBeta = sv.Prerelease() != ""
}

// IsBeta reports whether the running build is a prerelease (the stamped
// version has a SemVer prerelease component, e.g. "1.2.3-beta.1").
func IsBeta() bool { return isBeta }

func GetAppVersion() string {
	return appVersion
}

func makeUpdateErrorPayload(code string, detail string) string {
	if strings.TrimSpace(detail) == "" {
		return code
	}
	return code + updateErrorSeparator + detail
}

func emitUpdateError(code string, err error) error {
	detail := ""
	if err != nil {
		detail = err.Error()
		log.Printf("update %s: %v", code, err)
	} else {
		log.Printf("update %s", code)
	}
	application.Get().Event.Emit(EventAppUpdateError, makeUpdateErrorPayload(code, detail))
	if err != nil {
		return fmt.Errorf("%s: %w", code, err)
	}
	return errors.New(code)
}

func newTimedRequest(parent context.Context, method string, target string, timeout time.Duration) (*http.Request, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	httpx.ApplyDefaultHeaders(req)
	return req, cancel, nil
}

func getReleaseAPIURLs() []string {
	return []string{
		"https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://cdn.gh-proxy.org/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://edgeone.gh-proxy.org/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://gh-proxy.org/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://hk.gh-proxy.org/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://ghproxy.vip/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
	}
}

func fetchReleases(ctx context.Context) ([]githubRelease, error) {
	apis := getReleaseAPIURLs()
	useReverse := utils.IsMirrorEnabled()
	var lastErr error
	for i := 0; i < len(apis); i++ {
		idx := i
		if useReverse {
			idx = len(apis) - 1 - i
		}
		target := apis[idx]
		req, cancel, err := newTimedRequest(ctx, http.MethodGet, target, updateMetadataTimeout)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := httpx.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			cancel()
			lastErr = fmt.Errorf("HTTP %s", resp.Status)
			continue
		}
		var releases []githubRelease
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&releases)
		resp.Body.Close()
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if len(releases) > 0 {
			return releases, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no release metadata available")
	}
	return nil, lastErr
}

func getAcceptBeta() bool {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("config.Load failed while checking updates: %v", err)
	}
	return cfg.EnableBetaUpdates || isBeta
}

func selectUpdateRelease(releases []githubRelease, version string) (githubRelease, bool) {
	acceptBeta := getAcceptBeta()
	cur := strings.TrimPrefix(strings.TrimSpace(version), "v")
	vCur, err1 := semver.NewVersion(cur)
	for _, release := range releases {
		if release.Prerelease && !acceptBeta {
			continue
		}
		tag := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
		vRel, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if err1 != nil {
			if release.TagName != version {
				return release, true
			}
			continue
		}
		if vRel.GreaterThan(vCur) {
			return release, true
		}
	}
	return githubRelease{}, false
}

func findAssetByName(release githubRelease, name string) *githubReleaseAsset {
	for i := range release.Assets {
		if strings.EqualFold(strings.TrimSpace(release.Assets[i].Name), strings.TrimSpace(name)) {
			return &release.Assets[i]
		}
	}
	return nil
}

func expectedZipAssetName() string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("LeviLauncher_windows_%s.zip", runtime.GOARCH)
	}
	return fmt.Sprintf("LeviLauncher_%s_%s.zip", runtime.GOOS, runtime.GOARCH)
}

func buildAssetCandidateURLs(target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	proxies := []string{
		"https://edgeone.gh-proxy.org/",
		"https://gh-proxy.org/",
		"https://hk.gh-proxy.org/",
		"https://cdn.gh-proxy.org/",
	}
	out := make([]string, 0, len(proxies)+1)
	if utils.IsMirrorEnabled() {
		for _, prefix := range proxies {
			out = append(out, prefix+target)
		}
	}
	out = append(out, target)
	if !utils.IsMirrorEnabled() {
		for _, prefix := range proxies[:1] {
			out = append(out, prefix+target)
		}
	}
	return out
}

func downloadTextWithCandidates(ctx context.Context, urls []string, timeout time.Duration) (string, error) {
	var lastErr error
	for _, target := range urls {
		req, cancel, err := newTimedRequest(ctx, http.MethodGet, target, timeout)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := httpx.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			cancel()
			lastErr = fmt.Errorf("HTTP %s", resp.Status)
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		return string(data), nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("download text failed")
	}
	return "", lastErr
}

func parseSHA256Checksum(content string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(content))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty checksum content")
	}
	hash := strings.TrimSpace(fields[0])
	if len(hash) != 64 {
		return "", fmt.Errorf("invalid sha256 length")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", err
	}
	return strings.ToLower(hash), nil
}

func calculateSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifyFileSHA256(path string, expected string) error {
	actual, err := calculateSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(expected)) {
		return fmt.Errorf("sha256 mismatch: expected %s got %s", expected, actual)
	}
	return nil
}

func downloadUpdateAsset(ctx context.Context, urls []string, dest string) error {
	var lastErr error
	tmp := fsx.PartialPath(dest)
	for _, target := range urls {
		_ = os.Remove(tmp)
		req, cancel, err := newTimedRequest(ctx, http.MethodGet, target, updateDownloadTimeout)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Accept", "application/zip")
		resp, err := httpx.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			cancel()
			lastErr = fmt.Errorf("HTTP %s", resp.Status)
			continue
		}
		application.Get().Event.Emit(EventAppUpdateStatus, "downloading")
		f, ferr := os.Create(tmp)
		if ferr != nil {
			resp.Body.Close()
			cancel()
			lastErr = ferr
			continue
		}
		var downloaded int64
		total := resp.ContentLength
		buf := make([]byte, 32*1024)
		lastEmit := time.Now().Add(-time.Second)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				wn, werr := f.Write(buf[:n])
				if werr != nil {
					ferr = werr
					break
				}
				downloaded += int64(wn)
				if time.Since(lastEmit) >= 250*time.Millisecond {
					application.Get().Event.Emit(EventAppUpdateProgress, AppUpdateProgress{Phase: "download", Downloaded: downloaded, Total: total})
					lastEmit = time.Now()
				}
			}
			if rerr != nil {
				if rerr == io.EOF {
					break
				}
				ferr = rerr
				break
			}
		}
		application.Get().Event.Emit(EventAppUpdateProgress, AppUpdateProgress{Phase: "download", Downloaded: downloaded, Total: total})
		cerr := f.Close()
		resp.Body.Close()
		cancel()
		if ferr != nil || cerr != nil {
			_ = os.Remove(tmp)
			if ferr != nil {
				lastErr = ferr
			} else {
				lastErr = cerr
			}
			continue
		}
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("download failed")
	}
	return lastErr
}

func buildSelfUpdateCandidateURLs(ver string) []string {
	return []string{
		"https://goproxy.io/github.com/dreamguxiang/!levi!launcher/@v/" + ver + ".zip",
		"https://mirrors.aliyun.com/goproxy/github.com/dreamguxiang/levilauncher/@v/" + ver + ".zip",
	}
}

func startSelfUpdateElevated(exePath string, curVer string) error {
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}
	arg := "--self-update=" + strings.TrimPrefix(strings.TrimSpace(curVer), "v")
	args, err := syscall.UTF16PtrFromString(arg)
	if err != nil {
		return err
	}
	ret, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(args)),
		0,
		swShowNormal,
	)
	if ret <= shellExecuteErrCut {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("ShellExecuteW failed with code %d", ret)
	}
	return nil
}

func CheckUpdate(version string) types.CheckUpdate {
	releases, err := fetchReleases(context.Background())
	if err != nil || len(releases) == 0 {
		if err != nil {
			log.Printf("CheckUpdate failed: %v", err)
		}
		return types.CheckUpdate{IsUpdate: false, Version: version}
	}
	release, ok := selectUpdateRelease(releases, version)
	if ok {
		return types.CheckUpdate{IsUpdate: true, Version: release.TagName, Body: release.Body}
	}

	return types.CheckUpdate{IsUpdate: false, Version: version}
}

func Update(version string) error {
	ctx := context.Background()
	application.Get().Event.Emit(EventAppUpdateStatus, "checking")
	exePath, _ := os.Executable()
	instDir := filepath.Dir(exePath)

	cur := strings.TrimSpace(version)
	if cur == "" {
		cur = GetAppVersion()
	}
	releases, err := fetchReleases(ctx)
	if err != nil {
		return emitUpdateError("ERR_UPDATE_RELEASE_UNAVAILABLE", err)
	}
	release, ok := selectUpdateRelease(releases, cur)
	target := strings.TrimSpace(release.TagName)
	if !ok || target == "" {
		return emitUpdateError("ERR_UPDATE_NOT_AVAILABLE", fmt.Errorf("no update available"))
	}
	ver := target
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}
	curVer := cur
	if !strings.HasPrefix(curVer, "v") {
		curVer = "v" + curVer
	}

	if runtime.GOOS == "windows" && !utils.CanWriteDir(instDir) {
		application.Get().Event.Emit(EventAppUpdateStatus, "elevating")
		if err := startSelfUpdateElevated(exePath, curVer); err != nil {
			return emitUpdateError("ERR_UPDATE_ADMIN_REQUIRED", err)
		}
		os.Exit(0)
		return nil
	}

	archName := fmt.Sprintf("%s.zip", ver)

	execName := "LeviLauncher"
	if runtime.GOOS == "windows" {
		execName += ".exe"
	}

	updDir := apppath.UpdatesDir()
	if err := os.MkdirAll(updDir, 0o755); err != nil {
		return emitUpdateError("ERR_UPDATE_WRITE_FAILED", err)
	}
	zipPath := filepath.Join(updDir, archName)
	if fi, statErr := os.Stat(zipPath); statErr == nil && fi.Size() > 0 {
		application.Get().Event.Emit(EventAppUpdateStatus, "downloaded")
		application.Get().Event.Emit(EventAppUpdateProgress, AppUpdateProgress{Phase: "download", Downloaded: fi.Size(), Total: fi.Size()})
	} else {
		if err := downloadUpdateAsset(ctx, buildSelfUpdateCandidateURLs(ver), zipPath); err != nil {
			return emitUpdateError("ERR_UPDATE_DOWNLOAD_FAILED", err)
		}
	}

	u := &updater.Updater{
		Provider:       &provider.Zip{Path: zipPath},
		ExecutableName: execName,
		Version:        curVer,
	}

	application.Get().Event.Emit(EventAppUpdateStatus, "installing")
	status, err := u.Update()
	if err != nil {
		return emitUpdateError("ERR_UPDATE_INSTALL_FAILED", err)
	}

	_ = u.CleanUp()
	_ = os.RemoveAll(updDir)
	log.Printf("Update status: %+v", status)
	application.Get().Event.Emit(EventAppUpdateStatus, "installed")

	exe, err := os.Executable()
	if err != nil {
		return emitUpdateError("ERR_UPDATE_RESTART_FAILED", err)
	}
	application.Get().Event.Emit(EventAppUpdateStatus, "restarting")
	if err := restartProgram(exe); err != nil {
		return emitUpdateError("ERR_UPDATE_RESTART_FAILED", err)
	}
	return nil
}

func restartProgram(exePath string) error {
	cmd := exec.Command(exePath, "--post-update-restart")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to restart program: %w", err)
	}
	os.Exit(0)
	return nil
}
