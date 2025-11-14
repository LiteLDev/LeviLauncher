package update

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	buildcfg "github.com/liteldev/LeviLauncher/build"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/mouuff/go-rocket-update/pkg/provider"
	"github.com/mouuff/go-rocket-update/pkg/updater"
)

var (
	appVersion = "0.0.10"
	isBeta     = true
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

func CheckUpdate(version string) types.CheckUpdate {
	const latestAPI = "https://api.github.com/repos/LiteLDev/LeviLauncher/releases/latest"
	req, err := http.NewRequest("GET", latestAPI, nil)
	if err != nil {
		return types.CheckUpdate{IsUpdate: false, Version: version}
	}
	req.Header.Set("User-Agent", "LeviLauncher-Update/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return types.CheckUpdate{IsUpdate: false, Version: version}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return types.CheckUpdate{IsUpdate: false, Version: version}
	}
	var payload struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return types.CheckUpdate{IsUpdate: false, Version: version}
	}
	latestTag := strings.TrimSpace(payload.TagName)
	if latestTag == "" {
		return types.CheckUpdate{IsUpdate: false, Version: version}
	}
	cur := strings.TrimPrefix(strings.TrimSpace(version), "v")
	latest := strings.TrimPrefix(latestTag, "v")
	vCur, err1 := semver.NewVersion(cur)
	vLatest, err2 := semver.NewVersion(latest)
	if err1 != nil || err2 != nil {
		return types.CheckUpdate{IsUpdate: latestTag != version, Version: latestTag, Body: payload.Body}
	}
	if vLatest.GreaterThan(vCur) {
		return types.CheckUpdate{IsUpdate: true, Version: latestTag, Body: payload.Body}
	}
	return types.CheckUpdate{IsUpdate: false, Version: version}
}

func Update(version string) error {
	ver := strings.TrimSpace(version)
	if ver == "" {
		ver = GetAppVersion()
	}
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}

	archName := fmt.Sprintf("LeviLauncher_%s_%s.zip", runtime.GOOS, runtime.GOARCH)
	execName := "LeviLauncher"
	if runtime.GOOS == "windows" {
		execName += ".exe"
	}

	exePath, _ := os.Executable()
	instDir := filepath.Dir(exePath)
	if runtime.GOOS == "windows" && !utils.CanWriteDir(instDir) {
		pwsh := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
			fmt.Sprintf("Start-Process -FilePath '%s' -ArgumentList '--self-update=%s' -Verb RunAs", exePath, strings.TrimPrefix(ver, "v")))
		if err := pwsh.Start(); err != nil {
			return fmt.Errorf("update requires administrator: %w", err)
		}
		os.Exit(0)
		return nil
	}
	u := &updater.Updater{
		Provider: &provider.Github{
			RepositoryURL: "github.com/LiteLDev/LeviLauncher",
			ArchiveName:   archName,
		},
		ExecutableName: execName,
		Version:        ver,
	}
	status, err := u.Update()
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	_ = u.CleanUp()
	log.Printf("Update status: %+v", status)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}
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
