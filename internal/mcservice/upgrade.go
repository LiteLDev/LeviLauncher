package mcservice

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/leviloader"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/uwp"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// UpgradeOptions controls UpgradeInstance.
type UpgradeOptions struct {
	// Installer is an .msixvc (GDK) or .appx/.msix (UWP) path, absolute or a
	// file name inside the installers directory.
	Installer string
	// GameVersion is the target version reported by the catalog. GDK
	// installers carry no readable version, so it is used for the metadata;
	// UWP takes the version from the package manifest.
	GameVersion string
	// Backup zips the whole instance into Backups\Instances first.
	Backup bool
	// Unregister releases the instance's Windows registration before its
	// folder is swapped. It is supplied by the caller to avoid an import cycle.
	Unregister func() string
	// DeltaUpdateID, for a UWP instance, upgrades from Windows Update without
	// a downloaded installer: only the blocks that differ from the instance's
	// current files are fetched (see uwp.InstallDelta). Installer is ignored.
	DeltaUpdateID string
}

// EventUpgradeDelta carries uwp.DeltaStats while a delta upgrade runs.
const EventUpgradeDelta = "upgrade.delta"

// UpgradeResult is returned to the frontend.
type UpgradeResult struct {
	ErrorCode   string `json:"errorCode"`
	BackupPath  string `json:"backupPath"`
	FromVersion string `json:"fromVersion"`
	ToVersion   string `json:"toVersion"`
	// WasRegistered reports that the instance was registered with Windows
	// before the upgrade; the registration is released for the swap.
	WasRegistered bool   `json:"wasRegistered"`
	PackageType   string `json:"packageType"`
	IsPreview     bool   `json:"isPreview"`
}

// upgradeDropExt marks top-level game binaries that belong to the old build:
// files with these extensions that the new package no longer ships are not
// carried over, so an old DLL cannot shadow the new game's.
var upgradeDropExt = map[string]bool{".exe": true, ".dll": true, ".pdb": true, ".winmd": true}

func upgradeInstallerPath(name string) (string, error) {
	p := strings.TrimSpace(name)
	if p == "" {
		return "", errors.New("ERR_UPGRADE_INSTALLER_NOT_SPECIFIED")
	}
	if !filepath.IsAbs(p) {
		if filepath.Base(p) != p {
			return "", errors.New("ERR_PATH_ESCAPE")
		}
		dir, err := apppath.InstallersDir()
		if err != nil {
			return "", errors.New("ERR_ACCESS_INSTALLERS_DIR")
		}
		cand := filepath.Join(dir, p)
		if !utils.FileExists(cand) && utils.FileExists(cand+".msixvc") {
			cand += ".msixvc"
		}
		p = cand
	}
	if !utils.FileExists(p) {
		return "", errors.New("ERR_UPGRADE_INSTALLER_NOT_FOUND")
	}
	return p, nil
}

func isUWPInstaller(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".appx", ".msix", ".appxbundle", ".msixbundle":
		return true
	}
	return false
}

// UpgradeInstance replaces the game files of an existing instance with
// another build of the same channel and package type, keeping everything the
// user added: isolated game data, mods, lip state and launcher settings.
//
// The new build is extracted next to the instances (Cache\Upgrades, same
// volume), the instance's own entries are moved into it, and the two folders
// are swapped with renames. Every move is recorded and undone on failure, so
// the instance is either fully upgraded or left as it was.
func UpgradeInstance(ctx context.Context, name string, opts UpgradeOptions) (res UpgradeResult) {
	fail := func(code string) UpgradeResult {
		res.ErrorCode = code
		eventsink.From(ctx).Emit(EventExtractError, code)
		return res
	}
	name = strings.TrimSpace(name)
	if name == "" || versions.ValidateFolderName(name) != "" {
		return fail("ERR_INVALID_FOLDER_NAME")
	}
	vdir, err := apppath.InstancesDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return fail("ERR_ACCESS_VERSIONS_DIR")
	}
	instDir := filepath.Join(vdir, name)
	meta, err := versions.ReadMeta(instDir)
	if err != nil {
		return fail("ERR_UPGRADE_INSTANCE_NOT_FOUND")
	}
	res.FromVersion = meta.GameVersion
	pkgType := versions.DetectPackageType(instDir, meta)
	res.PackageType = pkgType
	res.IsPreview = strings.EqualFold(strings.TrimSpace(meta.Type), "preview")
	delta := strings.TrimSpace(opts.DeltaUpdateID) != ""
	installer := ""
	if delta {
		if pkgType != versions.PackageTypeUWP {
			return fail("ERR_UPGRADE_PACKAGE_MISMATCH")
		}
	} else {
		installer, err = upgradeInstallerPath(opts.Installer)
		if err != nil {
			return fail(err.Error())
		}
		if (pkgType == versions.PackageTypeUWP) != isUWPInstaller(installer) {
			return fail("ERR_UPGRADE_PACKAGE_MISMATCH")
		}
	}

	if opts.Backup {
		info := GetInstanceBackupInfo(name)
		if info.ErrorCode != "" {
			return fail(info.ErrorCode)
		}
		req := types.InstanceBackupRequest{ScopeModes: map[string]string{}}
		// The whole instance: its isolated game data in full mode (not only
		// Users) and its mods. Game data shared with the system install is
		// not part of the instance and is left to the user's own backups.
		for _, s := range info.Scopes {
			if !s.Selectable || !s.Exists || s.Shared {
				continue
			}
			req.Scopes = append(req.Scopes, s.Key)
			mode := s.DefaultMode
			for _, m := range s.Modes {
				if m.Key == instanceBackupModeFull && m.Selectable {
					mode = m.Key
				}
			}
			if mode != "" {
				req.ScopeModes[s.Key] = mode
			}
		}
		if len(req.Scopes) > 0 {
			br := BackupInstance(name, req)
			if br.ErrorCode != "" {
				return fail(br.ErrorCode)
			}
			res.BackupPath = br.ArchivePath
		}
	}

	stageRoot := filepath.Join(apppath.CacheDir(), apppath.UpgradesDirName)
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return fail("ERR_CREATE_TARGET_DIR")
	}
	stamp := time.Now().Format("20060102-150405")
	newDir := filepath.Join(stageRoot, name+"_"+stamp)
	oldDir := filepath.Join(stageRoot, name+"_"+stamp+".old")
	defer os.RemoveAll(newDir)

	progress := func(cur, total, fileCur, fileTotal int64, file string) {
		eventsink.From(ctx).Emit(EventExtractProgress, types.ExtractProgress{
			Dir: instDir, Bytes: fileCur, TotalBytes: fileTotal,
			GlobalCurrent: cur, GlobalTotal: total, CurrentFile: file, Ts: time.Now().UnixMilli(),
		})
	}

	newVersion := strings.TrimSpace(opts.GameVersion)
	channel := strings.ToLower(strings.TrimSpace(meta.Type))
	if pkgType == versions.PackageTypeUWP {
		var last time.Time
		uwpOpts := uwp.Options{
			Prepare: func(stage string, m uwp.Manifest) error {
				if (channel == "preview") != m.IsPreview() {
					return &uwp.Error{Code: "ERR_UPGRADE_CHANNEL_MISMATCH", Cause: errors.New("channel differs")}
				}
				newVersion = m.GameVersion()
				if err := leviloader.EnsureForVersion(ctx, stage); err != nil {
					return &uwp.Error{Code: "ERR_UWP_PREPARE", Cause: err}
				}
				return nil
			},
			Progress: func(p uwp.Progress) {
				if p.Current < p.Total && time.Since(last) < 100*time.Millisecond {
					return
				}
				last = time.Now()
				progress(p.Current, p.Total, p.FileCurrent, p.FileTotal, p.File)
			},
		}
		if delta {
			pkg, rerr := uwpdownload.ResolvePackage(ctx, strings.TrimSpace(opts.DeltaUpdateID))
			if rerr != nil {
				log.Printf("UpgradeInstance: resolve %s: %v", opts.DeltaUpdateID, rerr)
				return fail("ERR_UWP_DOWNLOAD_URL")
			}
			var lastStats time.Time
			_, err = uwp.InstallDelta(ctx, pkg.URL, instDir, newDir, uwp.DeltaOptions{
				Options: uwpOpts,
				Client:  httpx.NewClient(5 * time.Minute),
				Stats: func(s uwp.DeltaStats) {
					if s.Phase == "download" && time.Since(lastStats) < 200*time.Millisecond {
						return
					}
					lastStats = time.Now()
					eventsink.From(ctx).Emit(EventUpgradeDelta, s)
				},
			})
		} else {
			_, err = uwp.Install(ctx, installer, newDir, uwpOpts)
		}
		if err != nil {
			log.Printf("UpgradeInstance: UWP extract failed for %s: %v", name, err)
			return fail(uwp.ErrorCode(err))
		}
	} else {
		_, err = nativeinstall.Install(ctx, installer, newDir, nativeinstall.Options{
			CacheDir:           apppath.AccountsDir(),
			RequireFullLicense: true,
			Prepare:            func(staging string) error { return leviloader.EnsureForVersion(ctx, staging) },
			Progress: func(p nativeinstall.Progress) {
				progress(p.Current, p.Total, p.FileCurrent, p.FileTotal, p.File)
			},
		})
		if err != nil {
			code := "ERR_NATIVE_MSIXVC"
			var failure *nativeinstall.Error
			if errors.As(err, &failure) {
				code = failure.Code
			}
			if errors.Is(err, context.Canceled) {
				code = "ERR_CANCELED"
			}
			log.Printf("UpgradeInstance: GDK extract failed for %s: %v", name, err)
			return fail(code)
		}
		if v, ok := readExeFileVersion(filepath.Join(newDir, "Minecraft.Windows.exe")); ok && newVersion == "" {
			newVersion = v
		}
	}
	if newVersion == "" {
		newVersion = meta.GameVersion
	}
	res.ToVersion = newVersion

	if meta.Registered && opts.Unregister != nil {
		if code := opts.Unregister(); code != "" && code != "ERR_NOT_REGISTERED_THIS_VERSION" {
			return fail(code)
		}
		res.WasRegistered = true
		// Unregister rewrites the meta; keep the rest of it as read above.
		if m, err := versions.ReadMeta(instDir); err == nil {
			meta = m
		}
	}

	if code := swapInstance(instDir, newDir, oldDir); code != "" {
		return fail(code)
	}

	meta.GameVersion = newVersion
	meta.Registered = false
	if pkgType == versions.PackageTypeUWP && !versions.SupportsIsolation(pkgType, newVersion) {
		meta.EnableIsolation = false
	}
	if err := versions.WriteMeta(instDir, meta); err != nil {
		log.Printf("UpgradeInstance: write meta for %s: %v", name, err)
	}
	if err := os.RemoveAll(oldDir); err != nil {
		log.Printf("UpgradeInstance: remove old game files %s: %v", oldDir, err)
	}
	eventsink.From(ctx).Emit(EventExtractDone, instDir)
	return res
}

// swapInstance moves instDir aside to oldDir, moves the user's entries from
// it into newDir, and renames newDir to instDir. On failure every step is
// undone and an error code is returned.
func swapInstance(instDir, newDir, oldDir string) string {
	if err := os.Rename(instDir, oldDir); err != nil {
		log.Printf("UpgradeInstance: move instance aside: %v", err)
		return "ERR_UPGRADE_IN_USE"
	}
	var done [][2]string // {from, to}
	undo := func() {
		for i := len(done) - 1; i >= 0; i-- {
			if err := os.Rename(done[i][1], done[i][0]); err != nil {
				log.Printf("UpgradeInstance: undo %s: %v", done[i][1], err)
			}
		}
		if err := os.Rename(oldDir, instDir); err != nil {
			log.Printf("UpgradeInstance: restore instance: %v", err)
		}
	}
	plan, err := upgradeMoves(oldDir, newDir)
	if err != nil {
		log.Printf("UpgradeInstance: plan moves: %v", err)
		undo()
		return "ERR_UPGRADE_SWAP_FAILED"
	}
	for _, m := range plan {
		if err := os.Rename(m[0], m[1]); err != nil {
			log.Printf("UpgradeInstance: move %s: %v", m[0], err)
			undo()
			return "ERR_UPGRADE_IN_USE"
		}
		done = append(done, m)
	}
	if err := os.Rename(newDir, instDir); err != nil {
		log.Printf("UpgradeInstance: move new build in place: %v", err)
		// newDir still holds the moved entries; undo moves them back.
		undo()
		return "ERR_UPGRADE_SWAP_FAILED"
	}
	return ""
}

// upgradeMoves lists the {from, to} renames that carry the instance's own
// entries from oldDir into the new build at newDir:
//   - entries the new build ships are left behind (they are game files);
//   - top-level binaries the new build no longer ships are left behind;
//   - .levilauncher is merged entry by entry and the new build wins, because
//     the UWP installer writes the Xbox auth-key marker of the new executable
//     there; the legacy root marker describes the old executable and is
//     dropped for the same reason;
//   - everything else (isolated game data, mods, .lip, user files) moves.
func upgradeMoves(oldDir, newDir string) ([][2]string, error) {
	exists := func(p string) bool { _, err := os.Lstat(p); return err == nil }
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return nil, err
	}
	var plan [][2]string
	for _, e := range entries {
		n := e.Name()
		from, to := filepath.Join(oldDir, n), filepath.Join(newDir, n)
		switch {
		case strings.EqualFold(n, apppath.LegacyInstanceAuthKeyMarker):
			continue
		case strings.EqualFold(n, apppath.InstanceStateDirName) && e.IsDir() && exists(to):
			children, err := os.ReadDir(from)
			if err != nil {
				return nil, err
			}
			for _, c := range children {
				if cto := filepath.Join(to, c.Name()); !exists(cto) {
					plan = append(plan, [2]string{filepath.Join(from, c.Name()), cto})
				}
			}
		case exists(to):
			continue
		case !e.IsDir() && upgradeDropExt[strings.ToLower(filepath.Ext(n))]:
			continue
		default:
			plan = append(plan, [2]string{from, to})
		}
	}
	return plan, nil
}

// UpgradeTargetError describes why a catalog version cannot be used to
// upgrade the instance, or "" when it can. Downgrades are allowed; the
// frontend warns about them.
func UpgradeTargetError(name, version, channel, packageType string) string {
	vdir, err := apppath.InstancesDir()
	if err != nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	dir := filepath.Join(vdir, strings.TrimSpace(name))
	meta, err := versions.ReadMeta(dir)
	if err != nil {
		return "ERR_UPGRADE_INSTANCE_NOT_FOUND"
	}
	if versions.DetectPackageType(dir, meta) != versions.NormalizePackageType(packageType) {
		return "ERR_UPGRADE_PACKAGE_MISMATCH"
	}
	isPrev := func(c string) bool { return strings.EqualFold(strings.TrimSpace(c), "preview") }
	if isPrev(meta.Type) != isPrev(channel) {
		return "ERR_UPGRADE_CHANNEL_MISMATCH"
	}
	if strings.TrimSpace(version) == strings.TrimSpace(meta.GameVersion) {
		return "ERR_UPGRADE_SAME_VERSION"
	}
	return ""
}
