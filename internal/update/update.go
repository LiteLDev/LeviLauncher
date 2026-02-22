package update

import (
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
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/mouuff/go-rocket-update/pkg/provider"
	"github.com/mouuff/go-rocket-update/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/application"
	winreg "golang.org/x/sys/windows/registry"
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

const (
	EventAppUpdateStatus   = "app_update_status"
	EventAppUpdateProgress = "app_update_progress"
	EventAppUpdateError    = "app_update_error"
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
	apis := []string{
		"https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://cdn.gh-proxy.org/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://edgeone.gh-proxy.org/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://gh-proxy.org/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://hk.gh-proxy.org/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
		"https://ghproxy.vip/https://api.github.com/repos/LiteLDev/LeviLauncher/releases",
	}
	useReverse := isChinaUser()
	var releases []struct {
		TagName    string `json:"tag_name"`
		Body       string `json:"body"`
		Prerelease bool   `json:"prerelease"`
	}

	found := false
	for i := 0; i < len(apis); i++ {
		idx := i
		if useReverse {
			idx = len(apis) - 1 - i
		}
		latestAPI := apis[idx]
		req, err := http.NewRequest("GET", latestAPI, nil)
		if err != nil {
			continue
		}
		httpx.ApplyDefaultHeaders(req)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&releases); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		if len(releases) > 0 {
			found = true
			break
		}
	}

	if !found || len(releases) == 0 {
		return types.CheckUpdate{IsUpdate: false, Version: version}
	}

	cfg, _ := config.Load()
	acceptBeta := cfg.EnableBetaUpdates || isBeta

	cur := strings.TrimPrefix(strings.TrimSpace(version), "v")
	vCur, err1 := semver.NewVersion(cur)

	for _, r := range releases {
		if r.Prerelease && !acceptBeta {
			continue
		}

		tag := strings.TrimPrefix(strings.TrimSpace(r.TagName), "v")
		vRel, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}

		if err1 != nil {
			if r.TagName != version {
				return types.CheckUpdate{IsUpdate: true, Version: r.TagName, Body: r.Body}
			}
			continue
		}

		if vRel.GreaterThan(vCur) {
			return types.CheckUpdate{IsUpdate: true, Version: r.TagName, Body: r.Body}
		}
	}

	return types.CheckUpdate{IsUpdate: false, Version: version}
}

func Update(version string) error {
	exePath, _ := os.Executable()
	instDir := filepath.Dir(exePath)

	cur := strings.TrimSpace(version)
	if cur == "" {
		cur = GetAppVersion()
	}
	cu := CheckUpdate(cur)
	target := strings.TrimSpace(cu.Version)
	if !cu.IsUpdate || target == "" {
		return fmt.Errorf("no update available")
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
			application.Get().Event.Emit(EventAppUpdateError, fmt.Errorf("update requires administrator: %w", err).Error())
			return fmt.Errorf("update requires administrator: %w", err)
		}
		os.Exit(0)
		return nil
	}

	archName := fmt.Sprintf("%s.zip", ver)
	execName := "LeviLauncher"
	if runtime.GOOS == "windows" {
		execName += ".exe"
	}

	base := apppath.BaseRoot()
	updDir := filepath.Join(base, "updates")
	if err := os.MkdirAll(updDir, 0o755); err != nil {
		return fmt.Errorf("create updates dir: %w", err)
	}
	zipPath := filepath.Join(updDir, archName)
	fi, statErr := os.Stat(zipPath)
	if statErr != nil || fi.Size() == 0 {
		cand := []string{
			"https://goproxy.io/github.com/dreamguxiang/!levi!launcher/@v/" + ver + ".zip",
			"https://mirrors.aliyun.com/goproxy/github.com/dreamguxiang/levilauncher/@v/" + ver + ".zip",
		}
		rev := isChinaUser()
		for i := 0; i < len(cand); i++ {
			idx := i
			if rev {
				idx = len(cand) - 1 - i
			}
			u := cand[idx]
			req, err := http.NewRequest("GET", u, nil)
			if err != nil {
				continue
			}
			httpx.ApplyDefaultHeaders(req)
			req.Header.Set("Accept", "application/zip")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				continue
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				resp.Body.Close()
				continue
			}
			application.Get().Event.Emit(EventAppUpdateStatus, "downloading")
			tmp := zipPath + ".part"
			_ = os.Remove(tmp)
			f, ferr := os.Create(tmp)
			if ferr != nil {
				resp.Body.Close()
				continue
			}
			var downloaded int64 = 0
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
			if time.Since(lastEmit) >= 0 {
				application.Get().Event.Emit(EventAppUpdateProgress, AppUpdateProgress{Phase: "download", Downloaded: downloaded, Total: total})
			}
			cerr := f.Close()
			resp.Body.Close()
			if ferr != nil || cerr != nil {
				_ = os.Remove(tmp)
				continue
			}
			if err := os.Rename(tmp, zipPath); err != nil {
				_ = os.Remove(tmp)
				continue
			}
			application.Get().Event.Emit(EventAppUpdateStatus, "downloaded")
			break
		}
	} else {
		application.Get().Event.Emit(EventAppUpdateStatus, "downloaded")
		application.Get().Event.Emit(EventAppUpdateProgress, AppUpdateProgress{Phase: "download", Downloaded: fi.Size(), Total: fi.Size()})
	}

	u := &updater.Updater{
		Provider:       &provider.Zip{Path: zipPath},
		ExecutableName: execName,
		Version:        curVer,
	}

	application.Get().Event.Emit(EventAppUpdateStatus, "installing")
	status, err := u.Update()
	if err != nil {
		application.Get().Event.Emit(EventAppUpdateError, fmt.Errorf("update failed: %w", err).Error())
		return fmt.Errorf("update failed: %w", err)
	}

	_ = u.CleanUp()
	_ = os.RemoveAll(updDir)
	log.Printf("Update status: %+v", status)
	application.Get().Event.Emit(EventAppUpdateStatus, "installed")

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}
	application.Get().Event.Emit(EventAppUpdateStatus, "restarting")
	if err := restartProgram(exe); err != nil {
		return fmt.Errorf("error occurred while restarting program: %w", err)
	}
	return nil
}

func restartProgram(exePath string) error {
	cmd := exec.Command(exePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to restart program: %w", err)
	}
	os.Exit(0)
	return nil
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
	envs := []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"}
	for _, key := range envs {
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
	} else {
		if p, e := os.Readlink("/etc/localtime"); e == nil {
			low := strings.ToLower(p)
			if strings.Contains(low, "asia/shanghai") || strings.Contains(low, "asia/urumqi") {
				isTz = true
			}
		}
	}
	envs := []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"}
	for _, k := range envs {
		v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
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
