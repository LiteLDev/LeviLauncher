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
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	json "github.com/goccy/go-json"

	"github.com/Masterminds/semver/v3"
	buildcfg "github.com/liteldev/LeviLauncher/build"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/mouuff/go-rocket-update/pkg/provider"
	"github.com/mouuff/go-rocket-update/pkg/updater"
)

var (
	appVersion = "0.0.10"
	isBeta     = true

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

func Init() {
	v, b := parseVersionAndBetaFromBuildConfig(buildcfg.ConfigYAML)
	if v != "" {
		appVersion = v
	}
	isBeta = b
}

func parseVersionAndBetaFromBuildConfig(src string) (version string, beta bool) {
	s := strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	inInfo := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "info:" {
			inInfo = true
			continue
		}
		if inInfo {
			if !strings.HasPrefix(lines[i], " ") && strings.Contains(line, ":") {
				break
			}
			if strings.HasPrefix(line, "version:") {
				if m := regexp.MustCompile(`version:\s*"([^"]+)"`).FindStringSubmatch(line); len(m) == 2 {
					version = m[1]
				} else {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						version = strings.TrimSpace(parts[1])
						version = strings.Trim(version, "\"")
					}
				}
			}
			if strings.HasPrefix(line, "beta:") {
				var s string
				if m := regexp.MustCompile(`beta:\s*"([^"]+)"`).FindStringSubmatch(line); len(m) == 2 {
					s = m[1]
				} else {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						s = strings.TrimSpace(parts[1])
						s = strings.Trim(s, "\"")
					}
				}
				ls := strings.ToLower(strings.TrimSpace(s))
				beta = ls == "true" || ls == "yes" || ls == "on" || ls == "1"
			}
		}
	}
	return version, beta
}

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
	return emitUpdateErrorTo(context.Background(), code, err)
}

// emitUpdateErrorTo reports a failed update step to the context's event sink
// and returns it as an error whose text starts with code.
func emitUpdateErrorTo(ctx context.Context, code string, err error) error {
	detail := ""
	if err != nil {
		detail = err.Error()
		log.Printf("update %s: %v", code, err)
	} else {
		log.Printf("update %s", code)
	}
	eventsink.From(ctx).Emit(EventAppUpdateError, makeUpdateErrorPayload(code, detail))
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
	const oriUrl = "https://api.github.com/repos/LiteLDev/LeviLauncher/releases"
	return []string{
		oriUrl,
		"https://ghproxy.vip/" + oriUrl,
		"https://cdn.gh-proxy.org/" + oriUrl,
		"https://v4.gh-proxy.org/" + oriUrl,
		"https://v6.gh-proxy.org/" + oriUrl,
		"https://gh-proxy.org/" + oriUrl,
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
	return selectUpdateReleaseFor(releases, version, getAcceptBeta())
}

func selectUpdateReleaseFor(releases []githubRelease, version string, acceptBeta bool) (githubRelease, bool) {
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
		eventsink.From(ctx).Emit(EventAppUpdateStatus, "downloading")
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
					eventsink.From(ctx).Emit(EventAppUpdateProgress, AppUpdateProgress{Phase: "download", Downloaded: downloaded, Total: total})
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
		// A chunked response has no Content-Length (-1); once the body is fully
		// read the size is known, so report a complete bar instead of an
		// unknown total.
		if ferr == nil && total <= 0 {
			total = downloaded
		}
		eventsink.From(ctx).Emit(EventAppUpdateProgress, AppUpdateProgress{Phase: "download", Downloaded: downloaded, Total: total})
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
	oriUrl := "https://github.com/LiteLDev/LeviLauncher/releases/download/" + ver + "/LeviLauncher.zip"
	if utils.IsMirrorEnabled() {
		return []string{
			"https://gh-proxy.org/" + oriUrl,
			"https://v4.gh-proxy.org/" + oriUrl,
			"https://v6.gh-proxy.org/" + oriUrl,
			"https://gh.bibk.top/" + oriUrl,
			oriUrl,
		}
	} else {
		return []string{
			oriUrl,
		}
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

// errNeedsElevation reports that the install directory is not writable by
// this process.
var errNeedsElevation = errors.New("ERR_UPDATE_ADMIN_REQUIRED: the launcher folder is not writable; run as administrator")

// Update applies the newest release for the GUI: when the install directory
// is not writable it relaunches itself elevated and exits; after installing
// it restarts the launcher and exits.
func Update(version string) error {
	ctx := context.Background()
	exePath, _ := os.Executable()
	res, err := apply(ctx, version, getAcceptBeta())
	if errors.Is(err, errNeedsElevation) {
		eventsink.From(ctx).Emit(EventAppUpdateStatus, "elevating")
		if err := startSelfUpdateElevated(exePath, res.From); err != nil {
			return emitUpdateError("ERR_UPDATE_ADMIN_REQUIRED", err)
		}
		os.Exit(0)
		return nil
	}
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return emitUpdateError("ERR_UPDATE_RESTART_FAILED", err)
	}
	eventsink.From(ctx).Emit(EventAppUpdateStatus, "restarting")
	if err := restartProgram(exe); err != nil {
		return emitUpdateError("ERR_UPDATE_RESTART_FAILED", err)
	}
	return nil
}

// ApplyResult describes an update found or installed by Apply.
type ApplyResult struct {
	// From and To are the versions before and after, with a leading "v".
	// To is empty when no newer release was found.
	From string
	To   string
}

// CheckUpdateErr is CheckUpdate for callers that need to know why a check
// failed: a network or metadata failure is returned as an error starting
// with ERR_UPDATE_RELEASE_UNAVAILABLE instead of "no update". includeBeta
// also accepts prereleases, on top of the beta setting and beta builds.
func CheckUpdateErr(ctx context.Context, version string, includeBeta bool) (types.CheckUpdate, error) {
	cur := strings.TrimSpace(version)
	if cur == "" {
		cur = GetAppVersion()
	}
	releases, err := fetchReleases(ctx)
	if err != nil {
		return types.CheckUpdate{Version: cur}, fmt.Errorf("ERR_UPDATE_RELEASE_UNAVAILABLE: %w", err)
	}
	if release, ok := selectUpdateReleaseFor(releases, cur, includeBeta || getAcceptBeta()); ok {
		return types.CheckUpdate{IsUpdate: true, Version: release.TagName, Body: release.Body}, nil
	}
	return types.CheckUpdate{IsUpdate: false, Version: cur}, nil
}

// Apply downloads the newest release and installs it over the running
// executable. Unlike Update it never elevates, restarts or exits: when this
// process cannot write the install directory it fails with
// ERR_UPDATE_ADMIN_REQUIRED, and the new version runs from the next start.
// Status and progress events go to the context's event sink. Errors start
// with their ERR_UPDATE_* code; ERR_UPDATE_NOT_AVAILABLE means the running
// version is already the newest. An empty version means the running one.
func Apply(ctx context.Context, version string, includeBeta bool) (ApplyResult, error) {
	return apply(ctx, version, includeBeta || getAcceptBeta())
}

func apply(ctx context.Context, version string, acceptBeta bool) (ApplyResult, error) {
	sink := eventsink.From(ctx)
	sink.Emit(EventAppUpdateStatus, "checking")
	exePath, _ := os.Executable()
	instDir := filepath.Dir(exePath)

	cur := strings.TrimSpace(version)
	if cur == "" {
		cur = GetAppVersion()
	}
	curVer := cur
	if !strings.HasPrefix(curVer, "v") {
		curVer = "v" + curVer
	}
	res := ApplyResult{From: curVer}
	releases, err := fetchReleases(ctx)
	if err != nil {
		return res, emitUpdateErrorTo(ctx, "ERR_UPDATE_RELEASE_UNAVAILABLE", err)
	}
	release, ok := selectUpdateReleaseFor(releases, cur, acceptBeta)
	target := strings.TrimSpace(release.TagName)
	if !ok || target == "" {
		return res, emitUpdateErrorTo(ctx, "ERR_UPDATE_NOT_AVAILABLE", fmt.Errorf("no update available"))
	}
	ver := target
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}
	res.To = ver

	if runtime.GOOS == "windows" && !utils.CanWriteDir(instDir) {
		return res, errNeedsElevation
	}

	archName := fmt.Sprintf("%s.zip", ver)

	execName := "LeviLauncher"
	if runtime.GOOS == "windows" {
		execName += ".exe"
	}

	updDir := apppath.UpdatesDir()
	if err := os.MkdirAll(updDir, 0o755); err != nil {
		return res, emitUpdateErrorTo(ctx, "ERR_UPDATE_WRITE_FAILED", err)
	}
	zipPath := filepath.Join(updDir, archName)
	if fi, statErr := os.Stat(zipPath); statErr == nil && fi.Size() > 0 {
		sink.Emit(EventAppUpdateStatus, "downloaded")
		sink.Emit(EventAppUpdateProgress, AppUpdateProgress{Phase: "download", Downloaded: fi.Size(), Total: fi.Size()})
	} else {
		if err := downloadUpdateAsset(ctx, buildSelfUpdateCandidateURLs(ver), zipPath); err != nil {
			return res, emitUpdateErrorTo(ctx, "ERR_UPDATE_DOWNLOAD_FAILED", err)
		}
	}

	u := &updater.Updater{
		Provider:       &provider.Zip{Path: zipPath},
		ExecutableName: execName,
		Version:        curVer,
	}

	sink.Emit(EventAppUpdateStatus, "installing")
	status, err := u.Update()
	if err != nil {
		return res, emitUpdateErrorTo(ctx, "ERR_UPDATE_INSTALL_FAILED", err)
	}

	_ = u.CleanUp()
	_ = os.RemoveAll(updDir)
	log.Printf("Update status: %+v", status)
	sink.Emit(EventAppUpdateStatus, "installed")
	return res, nil
}

func restartProgram(exePath string) error {
	cmd := exec.Command(exePath, "--post-update-restart")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to restart program: %w", err)
	}
	os.Exit(0)
	return nil
}
