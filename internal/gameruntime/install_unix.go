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
	"strings"
	"syscall"
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
	metalSharpAsset = runtimeAsset{"runtime", "MetalSharp-Wine-10.0-x86_64.zip",
		"https://github.com/metalsharp/MetalSharp-Wine/releases/download/v10.0/MetalSharp-Wine-10.0-x86_64.zip",
		"8c90c9c2fdb01e6d5f1a5ea40d3b18b388649ddc7238b9061fb4657b43ebf737"}
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
	if !appleSilicon() {
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
	return types.WineSettings{App: app, Prefix: metalSharpPrefix(app),
		Executable: filepath.Join(app, metalSharpExecutable), Graphics: "auto", Sync: "auto", Runtime: "metalsharp"}
}

func InstallManagedWine(ctx context.Context, consent bool, progress func(types.WineInstallProgress), prepare func(context.Context, Profile) error) (types.WineSettings, error) {

	if progress == nil {
		progress = func(types.WineInstallProgress) {}
	}
	emit := func(stage, status string) { progress(types.WineInstallProgress{Stage: stage, Status: status}) }
	emit("rosetta", "working")
	if err := CheckManagedWineRequirements(ctx); err != nil {
		return types.WineSettings{}, err
	}
	app := managedWineApp()
	base := filepath.Dir(app)
	if err := os.MkdirAll(base, 0700); err != nil {
		return types.WineSettings{}, err
	}
	lock, err := os.OpenFile(filepath.Join(base, ".install.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return types.WineSettings{}, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return types.WineSettings{}, errors.New("ERR_WINE_INSTALL_BUSY")
		}
		return types.WineSettings{}, err
	}

	if NeedsRosetta(ctx) {
		if !consent {
			return types.WineSettings{}, errors.New("ERR_WINE_ROSETTA_CONSENT")
		}
		emit("rosetta", "installing")
		if err := installRosetta(ctx); err != nil {
			return types.WineSettings{}, err
		}
	}

	settings := managedWineSettings(app)
	previous, err := LoadSettings()
	if err != nil {
		previous = types.WineSettings{}
	}
	if previous.App == settings.App && previous.Prefix == settings.Prefix {
		settings = previous
	} else {
		settings.LeftCommandIsCtrl = previous.LeftCommandIsCtrl
	}
	if _, err := os.Stat(settings.App); os.IsNotExist(err) {
		downloadSettings, err := LoadDownloadSettings()
		if err != nil {
			return types.WineSettings{}, err
		}
		archive := filepath.Join(apppath.CacheDir(), "Wine", metalSharpAsset.name)
		if err := downloadWineAsset(ctx, archive, metalSharpAsset, downloadSettings, progress); err != nil {
			return types.WineSettings{}, err
		}
		emit("extract", "working")
		if err := extractMetalSharp(ctx, archive, settings.App); err != nil {
			return types.WineSettings{}, err
		}
	} else if err != nil {
		return types.WineSettings{}, err
	} else {
		emit("runtime", "skipped")
		emit("extract", "skipped")
	}

	emit("initialize", "working")
	if err := initializeMetalSharp(ctx, settings); err != nil {
		return types.WineSettings{}, err
	}
	p, err := profileFromSettings(settings)
	if err != nil {
		return types.WineSettings{}, err
	}
	if _, err := p.CheckVersion(ctx); err != nil {
		return types.WineSettings{}, err
	}
	if prepare != nil {
		emit("initialize", "dependencies")
		if err := prepare(ctx, p); err != nil {
			return types.WineSettings{}, err
		}
	}
	if err := migrateLegacyWineAccount(previous, p.Prefix); err != nil {
		return types.WineSettings{}, fmt.Errorf("ERR_WINE_ACCOUNT_MIGRATION: %w", err)
	}
	emit("select", "working")
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
		const idleTimeout = 60 * time.Second
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

func extractMetalSharp(ctx context.Context, archive, destination string) error {
	base := filepath.Dir(destination)
	if err := os.MkdirAll(base, 0700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(base, ".prepare-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if out, err := exec.CommandContext(ctx, "/usr/bin/ditto", "-xk", archive, staging).CombinedOutput(); err != nil {
		return fmt.Errorf("extract MetalSharp Wine: %w: %s", err, bytes.TrimSpace(out))
	}
	app := filepath.Join(staging, metalSharpBundle)
	_ = exec.CommandContext(ctx, "/usr/bin/xattr", "-dr", "com.apple.quarantine", app).Run()
	if out, err := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		return fmt.Errorf("ERR_WINE_RUNTIME_SIGNATURE: %w: %s", err, bytes.TrimSpace(out))
	}
	if !isMetalSharpApp(app) {
		return ErrExecutableMissing
	}
	return os.Rename(app, destination)
}

func initializedMetalSharpPrefix(prefix string) bool {
	for _, name := range []string{"system.reg", "user.reg", "drive_c/windows/system32/cmd.exe"} {
		if info, err := os.Stat(filepath.Join(prefix, name)); err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func initializeMetalSharp(ctx context.Context, settings types.WineSettings) error {
	const ready = ".levilauncher-wineboot"
	if err := os.MkdirAll(settings.Prefix, 0700); err != nil {
		return err
	}
	if err := checkExternalPrefix(settings.App, settings.Prefix); err != nil {
		return err
	}
	marker := filepath.Join(settings.Prefix, ready)
	if _, err := os.Stat(marker); err == nil && initializedMetalSharpPrefix(settings.Prefix) {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	p := Profile{App: settings.App, Wine: settings.Executable, Prefix: settings.Prefix,
		Graphics: "auto", metalSharp: metalSharpRuntime{settings.App}}
	// Initialize at the final path: Wine keys its server socket off the prefix.
	boot := p.Command(ctx, "wineboot", "-u")
	boot.WaitDelay = 2 * time.Second
	// A successfully exited wineboot can leave its output open in Wine
	// services. The separate wineserver wait below checks their lifetime.
	if out, err := boot.CombinedOutput(); err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		return fmt.Errorf("ERR_WINE_CONTAINER_INIT: %w: %s", err, bytes.TrimSpace(out))
	}
	server := exec.CommandContext(ctx, filepath.Join(filepath.Dir(p.Wine), "wineserver"), "-w")
	server.Env = p.Environment()
	if out, err := server.CombinedOutput(); err != nil {
		return fmt.Errorf("ERR_WINE_CONTAINER_INIT: %w: %s", err, bytes.TrimSpace(out))
	}
	if !initializedMetalSharpPrefix(p.Prefix) {
		return errors.New("ERR_WINE_CONTAINER_INIT: missing prefix files")
	}
	return fsx.WriteFileAtomic(marker, []byte("1\n"), 0600)
}
