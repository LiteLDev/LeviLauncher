package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
)

// Installer file states.
const (
	StateComplete    = "complete"
	StatePartial     = "partial"
	StateDownloading = "downloading"
)

var appxExt = regexp.MustCompile(`(?i)\.(?:appx|msix|appxbundle|msixbundle)$`)

// IsAppxInstaller reports whether path is a UWP package (.appx, .msix,
// .appxbundle, .msixbundle).
func IsAppxInstaller(path string) bool { return appxExt.MatchString(path) }

// IsInstallerFile reports whether path has an installer extension (.msixvc
// or a UWP package extension).
func IsInstallerFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".msixvc") || IsAppxInstaller(path)
}

// GDKFileName is the file a GDK build is downloaded to:
// "Release 1.21.50.07.msixvc".
func GDKFileName(channel, version string) string {
	return ChannelLabel(channel) + " " + strings.TrimSpace(version) + ".msixvc"
}

// FileName is the installer file name of a build. UWP names come from the
// UWP downloader ("Minecraft-UWP-Release-1.21.114.1.appx").
func FileName(b Build) (string, error) {
	if b.Type == TypeUWP {
		return uwpdownload.Filename(b.Version, b.Channel)
	}
	if strings.TrimSpace(b.Version) == "" {
		return "", fmt.Errorf("empty version")
	}
	return GDKFileName(b.Channel, b.Version), nil
}

// Identity is what an installer file name says about its build. Channel is
// "" when the name does not tell (a bare "1.21.50.07.msixvc").
type Identity struct {
	Version string
	Channel string
	Type    string
}

var (
	// installerIdentityFromPath of the window: "Release 1.21.50.07.msixvc",
	// "Minecraft-Preview-1.21.60.21.msixvc", "Minecraft-UWP-Beta-1.16.0.51.appx".
	namedInstaller = regexp.MustCompile(`(?i)^(?:Minecraft[- ])?(?:(UWP)[- ])?(Preview|Release|Beta)[- ](.+)\.(msixvc|appx|msix|appxbundle|msixbundle)$`)
	bareInstaller  = regexp.MustCompile(`(?i)^(\d+(?:\.\d+){2,3})\.(msixvc|appx|msix|appxbundle|msixbundle)$`)
	// Package files as the CDN names them:
	// "Microsoft.MinecraftUWP_1.21.12001.0_x64__8wekyb3d8bbwe.msixvc".
	storeInstaller = regexp.MustCompile(`(?i)^Microsoft\.(MinecraftUWP|MinecraftWindowsBeta)_(\d+\.\d+\.\d+\.\d+)_[A-Za-z0-9]+__[a-z0-9]+\.(msixvc|appx|msix|appxbundle|msixbundle)$`)
)

// IdentityFromPath reads the build an installer file name stands for. ok is
// false for names that say nothing.
func IdentityFromPath(path string) (Identity, bool) {
	name := filepath.Base(strings.TrimSpace(path))
	if m := namedInstaller.FindStringSubmatch(name); m != nil {
		typ := TypeGDK
		if m[1] != "" || !strings.EqualFold(m[4], "msixvc") {
			typ = TypeUWP
		}
		return Identity{Version: strings.TrimSpace(m[3]), Channel: NormalizeChannel(m[2]), Type: typ}, true
	}
	if m := storeInstaller.FindStringSubmatch(name); m != nil {
		channel := ChannelRelease
		if strings.EqualFold(m[1], "MinecraftWindowsBeta") {
			channel = ChannelPreview
		}
		if strings.EqualFold(m[3], "msixvc") {
			if v, ok := PackageToGameVersion(m[2]); ok {
				return Identity{Version: v, Channel: channel, Type: TypeGDK}, true
			}
			return Identity{}, false
		}
		if v, ok := packageToUWPVersion(m[2]); ok {
			return Identity{Version: v, Channel: channel, Type: TypeUWP}, true
		}
		return Identity{}, false
	}
	if m := bareInstaller.FindStringSubmatch(name); m != nil {
		typ := TypeGDK
		if !strings.EqualFold(m[2], "msixvc") {
			typ = TypeUWP
		}
		return Identity{Version: m[1], Type: typ}, true
	}
	return Identity{}, false
}

func packageNumbers(v string) ([4]int, bool) {
	var n [4]int
	parts := strings.Split(strings.TrimSpace(v), ".")
	if len(parts) != 4 {
		return n, false
	}
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return n, false
		}
		n[i] = x
	}
	return n, true
}

// PackageToGameVersion converts a GDK package version to the game version
// the catalogue uses: the third component packs patch and revision, so
// 1.21.12001.0 is 1.21.120.01.
func PackageToGameVersion(pkg string) (string, bool) {
	n, ok := packageNumbers(pkg)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d.%02d", n[0], n[1], n[2]/100, n[2]%100), true
}

// packageToUWPVersion converts a UWP package version (1.21.11401.0) to the
// UWP catalogue's form (1.21.114.1).
func packageToUWPVersion(pkg string) (string, bool) {
	n, ok := packageNumbers(pkg)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d.%d", n[0], n[1], n[2]/100, n[2]%100), true
}

// Matches reports whether an installer file with identity id holds build b.
// A file whose name gives no channel matches any channel.
func (id Identity) Matches(b Build) bool {
	if id.Type != b.Type || !VersionEqual(id.Version, b.Version) {
		return false
	}
	return id.Channel == "" || id.Channel == b.Channel
}

// InstallerFile is a file in the installers folder.
type InstallerFile struct {
	Path string
	Name string // file name of the finished installer (without .partial.tmp)
	Size int64
	// State is complete, partial (an unfinished download) or downloading
	// (an unfinished download another process is writing).
	State    string
	Identity Identity
	Known    bool // Identity was read from the name
}

// ScanInstallers lists the installer files in dir, finished and unfinished,
// sorted newest version first, then channel, type and name. Lock and other
// files are skipped. A missing dir is no error.
func ScanInstallers(dir string) ([]InstallerFile, error) {
	out := []InstallerFile{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	present := map[string]bool{}
	for _, e := range entries {
		present[strings.ToLower(e.Name())] = true
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		final, state := name, StateComplete
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".partial.tmp") {
			final, state = name[:len(name)-len(".partial.tmp")], StatePartial
			if present[strings.ToLower(final)+".lock"] {
				state = StateDownloading
			}
		}
		if !IsInstallerFile(final) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		f := InstallerFile{Path: filepath.Join(dir, name), Name: final, Size: info.Size(), State: state}
		f.Identity, f.Known = IdentityFromPath(final)
		if !f.Known {
			f.Identity.Type = TypeGDK
			if IsAppxInstaller(final) {
				f.Identity.Type = TypeUWP
			}
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if c := CompareVersions(a.Identity.Version, b.Identity.Version); c != 0 {
			return c > 0
		}
		if a.Identity.Channel != b.Identity.Channel {
			return channelRank[a.Identity.Channel] < channelRank[b.Identity.Channel]
		}
		if a.Identity.Type != b.Identity.Type {
			return a.Identity.Type == TypeGDK
		}
		if a.Name != b.Name {
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		return a.State == StateComplete
	})
	return out, nil
}

// FindInstaller returns the finished installer of b among files, preferring
// the file the launcher would download it to. ok is false when there is
// none.
func FindInstaller(files []InstallerFile, b Build) (InstallerFile, bool) {
	want, _ := FileName(b)
	var found InstallerFile
	ok := false
	for _, f := range files {
		if f.State != StateComplete || !f.Known || !f.Identity.Matches(b) || f.Size == 0 {
			continue
		}
		if strings.EqualFold(f.Name, want) {
			return f, true
		}
		if !ok {
			found, ok = f, true
		}
	}
	return found, ok
}

// PartialOf returns the unfinished download of b among files.
func PartialOf(files []InstallerFile, b Build) (InstallerFile, bool) {
	for _, f := range files {
		if f.State != StateComplete && f.Known && f.Identity.Matches(b) {
			return f, true
		}
	}
	return InstallerFile{}, false
}

// PartialPath is where a download to dest is staged.
func PartialPath(dest string) string { return fsx.PartialPath(dest) }
