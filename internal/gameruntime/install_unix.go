//go:build darwin

package gameruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

const (
	defaultWineProxy     = "https://gh-proxy.org/https://github.com"
	defaultMetalSharpTag = "v10.5"
)

type runtimeAsset struct {
	stage, name, url, sha string
}

var (
	metalSharpReleasesURL = "https://api.github.com/repos/metalsharp/MetalSharp-Wine/releases?per_page=20"
	metalSharpHTTPClient  = &http.Client{Timeout: 15 * time.Second}
	metalSharpAsset       = runtimeAsset{
		stage: "runtime",
		name:  "MetalSharp-Wine-10.5-x86_64.zip",
		url:   "https://github.com/metalsharp/MetalSharp-Wine/releases/download/v10.5/MetalSharp-Wine-10.5-x86_64.zip",
		sha:   "afd91deb8e745a952f84bc525995bc2d14921a2a999c72fc1790a3cdfa215655",
	}
)

type githubReleaseDTO struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		Size               int64  `json:"size"`
		Digest             string `json:"digest"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func fallbackMetalSharpRelease() types.WineRuntimeRelease {
	version := normalizeMetalSharpVersion(defaultMetalSharpTag)
	return types.WineRuntimeRelease{
		Tag:         defaultMetalSharpTag,
		Version:     version,
		Name:        "MetalSharp Wine " + version,
		AssetName:   metalSharpAsset.name,
		DownloadURL: metalSharpAsset.url,
		SHA256URL:   "https://github.com/metalsharp/MetalSharp-Wine/releases/download/" + defaultMetalSharpTag + "/SHA256SUMS",
		SHA256:      metalSharpAsset.sha,
		IsLatest:    true,
	}
}

func isValidSHA256Hex(raw string) bool {
	if len(raw) != 64 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

func parseGitHubAssetDigest(digest string) string {
	digest = strings.TrimSpace(digest)
	if !strings.HasPrefix(strings.ToLower(digest), "sha256:") {
		return ""
	}
	sum := strings.ToLower(strings.TrimSpace(digest[len("sha256:"):]))
	if !isValidSHA256Hex(sum) {
		return ""
	}
	return sum
}

func parseSHA256Sums(content, filename string) string {
	target := strings.TrimSpace(filename)
	if target == "" {
		return ""
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		if !isValidSHA256Hex(sum) {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		name = strings.TrimPrefix(name, "./")
		if name == target {
			return sum
		}
	}
	return ""
}

func parseMetalSharpReleases(body []byte) ([]types.WineRuntimeRelease, error) {
	var raw []githubReleaseDTO
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	releases := make([]types.WineRuntimeRelease, 0, len(raw))
	for _, item := range raw {
		if item.Draft || item.Prerelease {
			continue
		}
		var (
			zipName, zipURL, zipSHA, sumsURL string
			zipSize                          int64
		)
		for _, asset := range item.Assets {
			if strings.HasPrefix(asset.Name, "MetalSharp-Wine-") && strings.HasSuffix(asset.Name, "-x86_64.zip") {
				zipName = asset.Name
				zipURL = asset.BrowserDownloadURL
				zipSize = asset.Size
				zipSHA = parseGitHubAssetDigest(asset.Digest)
			} else if asset.Name == "SHA256SUMS" {
				sumsURL = asset.BrowserDownloadURL
			}
		}
		if zipName == "" || zipURL == "" {
			continue
		}
		version := normalizeMetalSharpVersion(item.TagName)
		if version == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = "MetalSharp Wine " + version
		}
		releases = append(releases, types.WineRuntimeRelease{
			Tag:         item.TagName,
			Version:     version,
			Name:        name,
			PublishedAt: item.PublishedAt,
			AssetName:   zipName,
			DownloadURL: zipURL,
			SHA256URL:   sumsURL,
			SHA256:      zipSHA,
			SizeBytes:   zipSize,
		})
	}
	if len(releases) > 0 {
		releases[0].IsLatest = true
	}
	return releases, nil
}

// ListMetalSharpReleases fetches official MetalSharp-Wine x86_64 releases from
// GitHub and falls back to the pinned recommended release when offline or rate-limited.
func ListMetalSharpReleases(ctx context.Context) ([]types.WineRuntimeRelease, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, metalSharpReleasesURL, nil)
	if err != nil {
		return []types.WineRuntimeRelease{fallbackMetalSharpRelease()}, nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LeviLauncher")
	resp, err := metalSharpHTTPClient.Do(req)
	if err != nil {
		return []types.WineRuntimeRelease{fallbackMetalSharpRelease()}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []types.WineRuntimeRelease{fallbackMetalSharpRelease()}, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return []types.WineRuntimeRelease{fallbackMetalSharpRelease()}, nil
	}
	releases, err := parseMetalSharpReleases(body)
	if err != nil || len(releases) == 0 {
		return []types.WineRuntimeRelease{fallbackMetalSharpRelease()}, nil
	}
	return releases, nil
}

func fetchReleaseSHA256(ctx context.Context, release types.WineRuntimeRelease, downloadSettings types.WineDownloadSettings) (string, error) {
	if isValidSHA256Hex(strings.ToLower(release.SHA256)) {
		return strings.ToLower(release.SHA256), nil
	}
	if release.SHA256URL == "" {
		return "", errors.New("ERR_WINE_ASSET_CHECKSUM")
	}
	candidates := make([]string, 0, 2)
	if downloadSettings.Mirror {
		if mirror, ok := fetch.RewriteGithub(downloadSettings.GithubProxy, release.SHA256URL); ok {
			candidates = append(candidates, mirror)
		}
	}
	candidates = append(candidates, release.SHA256URL)
	for _, candidate := range candidates {
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, candidate, nil)
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("User-Agent", "LeviLauncher")
		resp, err := metalSharpHTTPClient.Do(req)
		if err != nil {
			cancel()
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		cancel()
		if readErr != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		if sum := parseSHA256Sums(string(body), release.AssetName); sum != "" {
			return sum, nil
		}
	}
	return "", errors.New("ERR_WINE_ASSET_CHECKSUM")
}

func resolveMetalSharpAsset(ctx context.Context, targetTag string, downloadSettings types.WineDownloadSettings) (runtimeAsset, error) {
	targetVersion := normalizeMetalSharpVersion(targetTag)
	if targetVersion == "" || targetVersion == normalizeMetalSharpVersion(defaultMetalSharpTag) {
		return metalSharpAsset, nil
	}
	releases, err := ListMetalSharpReleases(ctx)
	if err != nil {
		return runtimeAsset{}, err
	}
	for _, rel := range releases {
		if strings.EqualFold(rel.Tag, strings.TrimSpace(targetTag)) || rel.Version == targetVersion {
			sha, err := fetchReleaseSHA256(ctx, rel, downloadSettings)
			if err != nil {
				return runtimeAsset{}, err
			}
			return runtimeAsset{
				stage: "runtime",
				name:  rel.AssetName,
				url:   rel.DownloadURL,
				sha:   sha,
			}, nil
		}
	}
	return runtimeAsset{}, errors.New("ERR_WINE_RELEASE_NOT_FOUND")
}

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
	return InstallManagedWineVersion(ctx, consent, "", progress, prepare)
}

func stopWineServerBestEffort(ctx context.Context, settings types.WineSettings) {
	if settings.Executable == "" || settings.Prefix == "" {
		return
	}
	server := filepath.Join(filepath.Dir(settings.Executable), "wineserver")
	info, err := os.Stat(server)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return
	}
	p := Profile{
		App:        settings.App,
		Wine:       settings.Executable,
		Prefix:     settings.Prefix,
		Graphics:   "auto",
		metalSharp: metalSharpRuntime{settings.App},
	}
	killCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	killCmd := exec.CommandContext(killCtx, server, "-k")
	killCmd.Env = p.Environment()
	_ = killCmd.Run()

	waitCtx, waitCancel := context.WithTimeout(ctx, 3*time.Second)
	defer waitCancel()
	waitCmd := exec.CommandContext(waitCtx, server, "-w")
	waitCmd.Env = p.Environment()
	_ = waitCmd.Run()
}

func cleanupStaleMetalSharpArchives(cacheDir, keepName string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if keepName != "" && name == keepName {
			continue
		}
		if strings.HasPrefix(name, "MetalSharp-Wine-") && strings.HasSuffix(name, "-x86_64.zip") {
			_ = os.Remove(filepath.Join(cacheDir, name))
		}
	}
}

func compareMetalSharpVersions(a, b string) int {
	pa := strings.Split(normalizeMetalSharpVersion(a), ".")
	pb := strings.Split(normalizeMetalSharpVersion(b), ".")
	maxLen := len(pa)
	if len(pb) > maxLen {
		maxLen = len(pb)
	}
	for i := 0; i < maxLen; i++ {
		var va, vb int
		if i < len(pa) {
			va, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			vb, _ = strconv.Atoi(pb[i])
		}
		if va != vb {
			if va < vb {
				return -1
			}
			return 1
		}
	}
	return strings.Compare(normalizeMetalSharpVersion(a), normalizeMetalSharpVersion(b))
}

func directorySizeBytes(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func isSameManagedApp(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	return sameRuntimePath(a, b)
}

func discoverManagedMetalSharpApps() []string {
	var apps []string
	seen := map[string]bool{}
	add := func(candidate string) {
		if !isMetalSharpApp(candidate) {
			return
		}
		clean := filepath.Clean(candidate)
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			if seen[resolved] {
				return
			}
			seen[resolved] = true
		} else if seen[clean] {
			return
		} else {
			seen[clean] = true
		}
		apps = append(apps, clean)
	}
	add(managedWineApp())
	if entries, err := os.ReadDir(managedWineVersionsDir()); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			add(filepath.Join(managedWineVersionsDir(), entry.Name(), metalSharpBundle))
		}
	}
	return apps
}

// ListInstalledMetalSharpVersions returns every managed MetalSharp Wine bundle
// installed under Runtimes/MetalSharp (including legacy root and versioned subdirs).
func ListInstalledMetalSharpVersions() ([]types.ManagedWineVersion, error) {
	current, err := LoadSettings()
	if err != nil {
		current = types.WineSettings{}
	}
	apps := discoverManagedMetalSharpApps()
	versions := make([]types.ManagedWineVersion, 0, len(apps))
	for _, app := range apps {
		ver := (metalSharpRuntime{app}).version()
		if ver == "" {
			parent := filepath.Base(filepath.Dir(app))
			if strings.HasPrefix(strings.ToLower(parent), "v") {
				ver = normalizeMetalSharpVersion(parent)
			}
		}
		tag := ver
		if tag != "" && !strings.HasPrefix(strings.ToLower(tag), "v") {
			tag = "v" + tag
		}
		versions = append(versions, types.ManagedWineVersion{
			Tag:       tag,
			Version:   ver,
			Path:      app,
			SizeBytes: directorySizeBytes(app),
			Active:    isSameManagedApp(current.App, app),
		})
	}
	sort.SliceStable(versions, func(i, j int) bool {
		if cmp := compareMetalSharpVersions(versions[i].Version, versions[j].Version); cmp != 0 {
			return cmp > 0
		}
		if versions[i].Active != versions[j].Active {
			return versions[i].Active
		}
		return versions[i].Path < versions[j].Path
	})
	return versions, nil
}

func findInstalledMetalSharpByVersion(version string) string {
	want := normalizeMetalSharpVersion(version)
	if want == "" {
		return ""
	}
	installed, err := ListInstalledMetalSharpVersions()
	if err != nil {
		return ""
	}
	for _, item := range installed {
		if normalizeMetalSharpVersion(item.Version) == want {
			return item.Path
		}
	}
	return ""
}

func mergeManagedSettings(targetApp string, previous types.WineSettings) types.WineSettings {
	settings := managedWineSettings(targetApp)
	if previous.Prefix != "" && isSameManagedApp(previous.Prefix, settings.Prefix) {
		settings = previous
		settings.App = targetApp
		settings.Executable = filepath.Join(targetApp, metalSharpExecutable)
		settings.Prefix = metalSharpPrefix(targetApp)
		settings.Runtime = "metalsharp"
	} else {
		settings.LeftCommandIsCtrl = previous.LeftCommandIsCtrl
	}
	return settings
}

func acquireManagedWineLock() (*os.File, error) {
	base := managedWineRoot()
	if err := os.MkdirAll(base, 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(base, ".install.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.New("ERR_WINE_INSTALL_BUSY")
		}
		return nil, err
	}
	return lock, nil
}

// SwitchManagedWineVersion activates an already-installed managed MetalSharp Wine
// bundle without re-downloading, refreshing the shared prefix via wineboot -u.
func SwitchManagedWineVersion(ctx context.Context, appPath string, prepare func(context.Context, Profile) error) (types.WineSettings, error) {
	if err := CheckManagedWineRequirements(ctx); err != nil {
		return types.WineSettings{}, err
	}
	cleanApp := filepath.Clean(appPath)
	if !isManagedMetalSharpApp(cleanApp) || !isMetalSharpApp(cleanApp) {
		return types.WineSettings{}, errors.New("ERR_WINE_VERSION_INVALID")
	}
	lock, err := acquireManagedWineLock()
	if err != nil {
		return types.WineSettings{}, err
	}
	defer lock.Close()

	previous, err := LoadSettings()
	if err != nil {
		previous = types.WineSettings{}
	}
	settings := mergeManagedSettings(cleanApp, previous)
	switching := !isSameManagedApp(previous.App, cleanApp)
	if switching {
		stopWineServerBestEffort(ctx, previous)
	}
	if err := initializeMetalSharp(ctx, settings, switching); err != nil {
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
		if err := prepare(ctx, p); err != nil {
			return types.WineSettings{}, err
		}
	}
	if err := SaveSettings(settings); err != nil {
		return types.WineSettings{}, err
	}
	if resolved, err := resolveSettings(settings); err == nil {
		settings = resolved
	}
	return settings, nil
}

// RemoveManagedWineVersion deletes an inactive managed MetalSharp Wine version
// and any matching cached archive while leaving the shared Wine prefix intact.
func RemoveManagedWineVersion(appPath string) error {
	cleanApp := filepath.Clean(appPath)
	if !isManagedMetalSharpApp(cleanApp) {
		return errors.New("ERR_WINE_VERSION_INVALID")
	}
	lock, err := acquireManagedWineLock()
	if err != nil {
		return err
	}
	defer lock.Close()

	current, err := readSettings(settingsPath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ERR_WINE_LEGACY_SETTINGS: %w", err)
	}
	if err == nil {
		if resolved, resolveErr := resolveSettings(current); resolveErr == nil {
			current = resolved
		}
	}
	if isSameManagedApp(current.App, cleanApp) {
		return errors.New("ERR_WINE_VERSION_IN_USE")
	}
	if _, statErr := os.Stat(cleanApp); os.IsNotExist(statErr) {
		return nil
	} else if statErr != nil {
		return statErr
	}
	ver := (metalSharpRuntime{cleanApp}).version()
	targetToRemove := cleanApp
	if filepath.Dir(filepath.Dir(cleanApp)) == filepath.Clean(managedWineVersionsDir()) {
		targetToRemove = filepath.Dir(cleanApp)
	}
	if err := os.RemoveAll(targetToRemove); err != nil {
		return err
	}
	if ver != "" {
		archive := filepath.Join(apppath.CacheDir(), "Wine", fmt.Sprintf("MetalSharp-Wine-%s-x86_64.zip", ver))
		_ = os.Remove(archive)
	}
	return nil
}

func replaceAppAtomically(stagingApp, destination string) (backupPath string, err error) {
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return "", err
	}
	if _, statErr := os.Stat(destination); statErr == nil {
		backupPath = filepath.Join(filepath.Dir(destination), fmt.Sprintf(".backup-%d.app", time.Now().UnixNano()))
		if err := os.Rename(destination, backupPath); err != nil {
			return "", fmt.Errorf("backup existing MetalSharp Wine: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}
	if err := os.Rename(stagingApp, destination); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, destination)
		}
		return "", err
	}
	return backupPath, nil
}

func rollbackAppReplacement(destination, backupPath string) {
	if backupPath == "" {
		return
	}
	_ = os.RemoveAll(destination)
	_ = os.Rename(backupPath, destination)
}

func InstallManagedWineVersion(ctx context.Context, consent bool, targetTag string, progress func(types.WineInstallProgress), prepare func(context.Context, Profile) error) (types.WineSettings, error) {
	if progress == nil {
		progress = func(types.WineInstallProgress) {}
	}
	emit := func(stage, status string) { progress(types.WineInstallProgress{Stage: stage, Status: status}) }
	emit("rosetta", "working")
	if err := CheckManagedWineRequirements(ctx); err != nil {
		return types.WineSettings{}, err
	}
	lock, err := acquireManagedWineLock()
	if err != nil {
		return types.WineSettings{}, err
	}
	defer lock.Close()

	if NeedsRosetta(ctx) {
		if !consent {
			return types.WineSettings{}, errors.New("ERR_WINE_ROSETTA_CONSENT")
		}
		emit("rosetta", "installing")
		if err := installRosetta(ctx); err != nil {
			return types.WineSettings{}, err
		}
	}

	previous, err := LoadSettings()
	if err != nil {
		previous = types.WineSettings{}
	}

	desiredVersion := normalizeMetalSharpVersion(targetTag)
	targetApp := ""
	needsDownload := false
	if desiredVersion != "" {
		if existing := findInstalledMetalSharpByVersion(desiredVersion); existing != "" {
			targetApp = existing
		} else {
			targetApp = managedWineAppForVersion(desiredVersion)
			needsDownload = true
		}
	} else {
		if installed, listErr := ListInstalledMetalSharpVersions(); listErr == nil && len(installed) > 0 {
			targetApp = installed[0].Path
			for _, item := range installed {
				if item.Active {
					targetApp = item.Path
					break
				}
			}
		} else {
			targetApp = managedWineAppForVersion(defaultMetalSharpTag)
			needsDownload = true
		}
	}

	settings := mergeManagedSettings(targetApp, previous)
	switchingVersion := needsDownload || !isSameManagedApp(previous.App, targetApp)

	var (
		backupPath  string
		createdApp  bool
		installedOK bool
	)
	defer func() {
		if installedOK {
			return
		}
		if backupPath != "" {
			rollbackAppReplacement(settings.App, backupPath)
		} else if createdApp {
			targetToRemove := settings.App
			if filepath.Dir(filepath.Dir(settings.App)) == filepath.Clean(managedWineVersionsDir()) {
				targetToRemove = filepath.Dir(settings.App)
			}
			_ = os.RemoveAll(targetToRemove)
		}
	}()

	if needsDownload {
		downloadSettings, err := LoadDownloadSettings()
		if err != nil {
			return types.WineSettings{}, err
		}
		asset, err := resolveMetalSharpAsset(ctx, targetTag, downloadSettings)
		if err != nil {
			return types.WineSettings{}, err
		}
		cacheDir := filepath.Join(apppath.CacheDir(), "Wine")
		archive := filepath.Join(cacheDir, asset.name)
		if err := downloadWineAsset(ctx, archive, asset, downloadSettings, progress); err != nil {
			return types.WineSettings{}, err
		}
		emit("extract", "working")
		stagingDir, stagedApp, err := extractMetalSharpToStaging(ctx, archive, managedWineRoot())
		if err != nil {
			return types.WineSettings{}, err
		}
		defer os.RemoveAll(stagingDir)
		if switchingVersion {
			stopWineServerBestEffort(ctx, previous)
		}
		backupPath, err = replaceAppAtomically(stagedApp, settings.App)
		if err != nil {
			return types.WineSettings{}, err
		}
		createdApp = backupPath == ""
	} else {
		if switchingVersion {
			stopWineServerBestEffort(ctx, previous)
		}
		emit("runtime", "skipped")
		emit("extract", "skipped")
	}

	emit("initialize", "working")
	if err := initializeMetalSharp(ctx, settings, switchingVersion); err != nil {
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
	installedOK = true
	if backupPath != "" {
		_ = os.RemoveAll(backupPath)
	}
	if needsDownload {
		cleanupStaleMetalSharpArchives(filepath.Join(apppath.CacheDir(), "Wine"), "")
	}
	if resolved, err := resolveSettings(settings); err == nil {
		settings = resolved
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

func extractMetalSharpToStaging(ctx context.Context, archive, base string) (staging, stagedApp string, err error) {
	if err := os.MkdirAll(base, 0700); err != nil {
		return "", "", err
	}
	staging, err = os.MkdirTemp(base, ".prepare-")
	if err != nil {
		return "", "", err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()
	if out, err := exec.CommandContext(ctx, "/usr/bin/ditto", "-xk", archive, staging).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("extract MetalSharp Wine: %w: %s", err, bytes.TrimSpace(out))
	}
	app := filepath.Join(staging, metalSharpBundle)
	_ = exec.CommandContext(ctx, "/usr/bin/xattr", "-dr", "com.apple.quarantine", app).Run()
	if out, err := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("ERR_WINE_RUNTIME_SIGNATURE: %w: %s", err, bytes.TrimSpace(out))
	}
	if !isMetalSharpApp(app) {
		return "", "", ErrExecutableMissing
	}
	cleanup = false
	return staging, app, nil
}

func initializedMetalSharpPrefix(prefix string) bool {
	for _, name := range []string{"system.reg", "user.reg", "drive_c/windows/system32/cmd.exe"} {
		if info, err := os.Stat(filepath.Join(prefix, name)); err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func initializeMetalSharp(ctx context.Context, settings types.WineSettings, forceRefresh bool) error {
	const ready = ".levilauncher-wineboot"
	if err := os.MkdirAll(settings.Prefix, 0700); err != nil {
		return err
	}
	if err := checkExternalPrefix(settings.App, settings.Prefix); err != nil {
		return err
	}
	marker := filepath.Join(settings.Prefix, ready)
	if !forceRefresh {
		if _, err := os.Stat(marker); err == nil && initializedMetalSharpPrefix(settings.Prefix) {
			return nil
		}
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
	if out, err := boot.CombinedOutput(); err != nil &&
		(ctx.Err() != nil || !errors.Is(err, exec.ErrWaitDelay) || boot.ProcessState == nil || !boot.ProcessState.Success()) {
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
