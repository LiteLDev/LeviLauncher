//go:build darwin

package gameruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

const defaultWineProxy = "https://gh-proxy.org/https://github.com"

type runtimeAsset struct {
	stage, name, url, sha string
}

var (
	templateAsset = runtimeAsset{"template", "Template-1.0.27.tar.xz",
		"https://github.com/Sikarugir-App/Template/releases/download/v1.0/Template-1.0.27.tar.xz",
		"dd0778f7b553d63df9b0f4e2cda3f3c6c005aabb75588c84d604afe80cdedbb9"}
	engineAsset = runtimeAsset{"engine", "WS12WineSikarugir11.0_1.tar.xz",
		"https://github.com/Sikarugir-App/Engines/releases/download/v1.0/WS12WineSikarugir11.0_1.tar.xz",
		"67e29fb3d74f363af39c69ba11f9b13a79812c5db07cf4748672658e4a200a0e"}
	wineInstallMu sync.Mutex
)

func downloadSettingsPath() string { return filepath.Join(apppath.ConfigDir(), "wine-download.json") }

func LoadDownloadSettings() (types.WineDownloadSettings, error) {
	defaults := types.WineDownloadSettings{GithubProxy: defaultWineProxy}
	data, err := os.ReadFile(downloadSettingsPath())
	if os.IsNotExist(err) {
		defaults.Mirror = utils.IsMirrorEnabled()
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	if err := json.Unmarshal(data, &defaults); err != nil {
		return defaults, err
	}
	return defaults, nil
}

func mainlandLocale(locale string) bool {
	return utils.IsMainlandChinaLocale(locale)
}

func SaveDownloadSettings(settings types.WineDownloadSettings) error {
	raw := strings.TrimRight(strings.TrimSpace(settings.GithubProxy), "/")
	if raw == "" {
		raw = defaultWineProxy
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("ERR_WINE_INVALID_GITHUB_PROXY")
	}
	if u.Path == "" {
		host := strings.ToLower(u.Hostname())
		for _, fullURLHost := range []string{"gh-proxy.org", "ghproxy.vip", "gh.bibk.top"} {
			if host == fullURLHost || strings.HasSuffix(host, "."+fullURLHost) {
				raw += "/https://github.com"
				break
			}
		}
	}
	settings.GithubProxy = raw
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(downloadSettingsPath(), data, 0600)
}

func NeedsRosetta(ctx context.Context) bool {
	if runtime.GOARCH != "arm64" {
		return false
	}
	// Check the receipt first so opening Wine settings cannot trigger an
	// Intel execution prompt on a Mac that has never installed Rosetta.
	if exec.CommandContext(ctx, "/usr/sbin/pkgutil", "--pkg-info", "com.apple.pkg.RosettaUpdateAuto").Run() != nil {
		return true
	}
	return exec.CommandContext(ctx, "/usr/bin/arch", "-x86_64", "/usr/bin/true").Run() != nil
}

func installRosetta(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	const arguments = "/usr/sbin/softwareupdate --install-rosetta --agree-to-license"
	output, err := exec.CommandContext(ctx, "/usr/sbin/softwareupdate", "--install-rosetta", "--agree-to-license").CombinedOutput()
	if !NeedsRosetta(ctx) {
		return nil
	}
	message := strings.ToLower(string(output))
	if err != nil && (strings.Contains(message, "administrator") || strings.Contains(message, "root privileges") ||
		strings.Contains(message, "must be root") || strings.Contains(message, "not authorized")) {
		output, err = exec.CommandContext(ctx, "/usr/bin/osascript", "-e",
			`do shell script "`+arguments+`" with administrator privileges`).CombinedOutput()
		if !NeedsRosetta(ctx) {
			return nil
		}
	}
	return fmt.Errorf("ERR_WINE_ROSETTA_INSTALL: %s", strings.TrimSpace(string(output)))
}

func managedWineSettings(app string) types.WineSettings {
	support := filepath.Join(app, "Contents", "SharedSupport")
	graphics := "auto"
	if runtime.GOARCH == "arm64" {
		graphics = "d3dmetal"
	}
	return types.WineSettings{App: app, Prefix: filepath.Join(support, "prefix"),
		Executable: filepath.Join(support, "wine", "bin", "wine"), Graphics: graphics, Sync: "auto"}
}

func InstallManagedWine(ctx context.Context, consent bool, progress func(types.WineInstallProgress)) (types.WineSettings, error) {
	if !wineInstallMu.TryLock() {
		return types.WineSettings{}, errors.New("ERR_WINE_INSTALL_BUSY")
	}
	defer wineInstallMu.Unlock()
	if progress == nil {
		progress = func(types.WineInstallProgress) {}
	}
	emit := func(stage, status string) { progress(types.WineInstallProgress{Stage: stage, Status: status}) }
	emit("rosetta", "working")
	if NeedsRosetta(ctx) {
		if !consent {
			return types.WineSettings{}, errors.New("ERR_WINE_ROSETTA_CONSENT")
		}
		emit("rosetta", "installing")
		if err := installRosetta(ctx); err != nil {
			return types.WineSettings{}, err
		}
	}

	base := filepath.Join(apppath.LocalDir(), "Runtimes", "Sikarugir")
	settings := managedWineSettings(filepath.Join(base, "Levi Wine.app"))
	if _, err := os.Stat(settings.App); os.IsNotExist(err) {
		downloadSettings, err := LoadDownloadSettings()
		if err != nil {
			return types.WineSettings{}, err
		}
		cache := filepath.Join(apppath.CacheDir(), "Wine")
		for _, asset := range []runtimeAsset{templateAsset, engineAsset} {
			if err := downloadWineAsset(ctx, filepath.Join(cache, asset.name), asset, downloadSettings, progress); err != nil {
				return types.WineSettings{}, err
			}
		}
		emit("container", "working")
		if err := assembleWineContainer(ctx, base, cache, settings); err != nil {
			return types.WineSettings{}, err
		}
	} else if err != nil {
		return types.WineSettings{}, err
	} else {
		for _, stage := range []string{"template", "engine", "container"} {
			emit(stage, "skipped")
		}
	}

	emit("initialize", "working")
	if err := initializeWineContainer(ctx, base, settings); err != nil {
		return types.WineSettings{}, err
	}
	emit("select", "working")
	if _, err := CheckSettings(ctx, settings); err != nil {
		return types.WineSettings{}, err
	}
	if err := ctx.Err(); err != nil {
		return types.WineSettings{}, err
	}
	if err := SaveSettings(settings); err != nil {
		return types.WineSettings{}, err
	}
	emit("select", "done")
	return settings, nil
}

func downloadWineAsset(ctx context.Context, dest string, asset runtimeAsset, settings types.WineDownloadSettings, progress func(types.WineInstallProgress)) error {
	update := func(status string, count, total int64) {
		progress(types.WineInstallProgress{Stage: asset.stage, Status: status, Downloaded: count, Total: total})
	}
	verify := func(path string) error {
		update("verifying", 0, 0)
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		digest := sha256.New()
		if _, err := io.Copy(digest, file); err != nil {
			return err
		}
		if fmt.Sprintf("%x", digest.Sum(nil)) != asset.sha {
			return errors.New("ERR_WINE_ASSET_CHECKSUM")
		}
		return nil
	}
	if _, err := os.Stat(dest); err == nil && verify(dest) == nil {
		return nil
	}
	download := func(source string) error {
		transfer, cancel := context.WithCancel(ctx)
		defer cancel()
		const idleTimeout = 30 * time.Second
		watchdog := time.AfterFunc(idleTimeout, cancel)
		defer watchdog.Stop()
		err := downloader.Download(transfer, source, dest, downloader.SyncOptions{
			Verify: verify,
			Progress: func(p downloader.DownloadProgress) {
				watchdog.Reset(idleTimeout)
				update("downloading", p.Downloaded, p.Total)
			},
		})
		if err != nil && transfer.Err() != nil && ctx.Err() == nil {
			return errors.New("ERR_WINE_DOWNLOAD_STALLED")
		}
		return err
	}
	update("downloading", 0, 0)
	if settings.Mirror {
		if mirror, ok := fetch.RewriteGithub(settings.GithubProxy, asset.url); ok {
			if err := download(mirror); err == nil {
				return nil
			} else if ctx.Err() != nil || errors.Is(err, downloader.ErrBusy) {
				return err
			}
			update("fallback", 0, 0)
		}
	}
	return download(asset.url)
}

func assembleWineContainer(ctx context.Context, base, cache string, settings types.WineSettings) error {
	if err := os.MkdirAll(base, 0700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(base, ".prepare-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	for _, asset := range []runtimeAsset{templateAsset, engineAsset} {
		out, err := exec.CommandContext(ctx, "/usr/bin/tar", "-xf", filepath.Join(cache, asset.name), "-C", staging).CombinedOutput()
		if err != nil {
			return fmt.Errorf("extract %s: %w: %s", asset.name, err, bytes.TrimSpace(out))
		}
	}
	app := filepath.Join(staging, "Template-1.0.27.app")
	support := filepath.Join(app, "Contents", "SharedSupport")
	if err := os.Rename(filepath.Join(staging, "wswine.bundle"), filepath.Join(support, "wine")); err != nil {
		return err
	}
	plist := filepath.Join(app, "Contents", "Info.plist")
	for _, field := range []struct{ name, kind, value string }{
		{"CFBundleIdentifier", "-string", "org.levimc.levilauncher.managedwine"},
		{"CFBundleName", "-string", "Levi Wine"},
		{"Skip Mono", "-integer", "1"}, {"Skip Gecko", "-integer", "1"},
		{"Symlinks In User Folder", "-integer", "0"},
	} {
		if err := exec.CommandContext(ctx, "/usr/bin/plutil", "-replace", field.name, field.kind, field.value, plist).Run(); err != nil {
			return fmt.Errorf("configure %s: %w", field.name, err)
		}
	}
	if settings.Graphics == "d3dmetal" {
		if err := exec.CommandContext(ctx, "/usr/bin/plutil", "-replace", "D3DMETAL", "-integer", "1", plist).Run(); err != nil {
			return err
		}
	}
	return os.Rename(app, settings.App)
}

func initializeWineContainer(ctx context.Context, base string, settings types.WineSettings) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	temp, err := os.MkdirTemp(base, "verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	script := filepath.Join(temp, "init.cmd")
	if err := os.WriteFile(script, []byte("@echo off\r\nver > \"%~dp0ready.txt\"\r\n"), 0600); err != nil {
		return err
	}
	launcher := filepath.Join(settings.App, "Contents", "MacOS", "wineskinlauncher")
	cmd := exec.CommandContext(ctx, launcher, script)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key == "HOME" || key == "USER" || key == "LOGNAME" || key == "TMPDIR" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "PATH=/usr/bin:/bin:/usr/sbin:/sbin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("initialize Wine: %w: %s", err, bytes.TrimSpace(output))
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		marker, _ := os.ReadFile(filepath.Join(temp, "ready.txt"))
		_, systemErr := os.Stat(filepath.Join(settings.Prefix, "system.reg"))
		_, userErr := os.Stat(filepath.Join(settings.Prefix, "user.reg"))
		_, commandErr := os.Stat(filepath.Join(settings.Prefix, "drive_c", "windows", "system32", "cmd.exe"))
		if bytes.Contains(marker, []byte("Microsoft Windows")) && systemErr == nil && userErr == nil && commandErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("ERR_WINE_CONTAINER_INIT: %w: %s", ctx.Err(), bytes.TrimSpace(output))
		case <-ticker.C:
		}
	}
}
