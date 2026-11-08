package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/catalog"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/gdkversions"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"github.com/liteldev/LeviLauncher/internal/xbox"
	"golang.org/x/sys/windows"
)

// This file holds the "installer" commands and the backend plumbing shared
// by the game-install commands (installer, instance create, loader): the
// build catalogue, installer downloads, the LeviLamina version database and
// the backend calls, all behind giBackend so tests run without network,
// accounts or real instances.

// gameInstallBackend is every backend call the game-install commands make.
type gameInstallBackend struct {
	// Catalogue and downloads.
	fetchGDK      func(ctx context.Context) gdkversions.Catalog
	loadUWP       func() ([]uwpdownload.Version, error)
	catalogCache  func() string
	installersDir func() (string, error)
	probeMirrors  func(ctx context.Context, urls []string, timeout time.Duration) []catalog.MirrorResult
	download      func(ctx context.Context, src, dest string, o downloader.SyncOptions) error
	resolveUWP    func(ctx context.Context, updateID string) (src string, verify func(string) error, err error)
	removeFile    func(path string) error
	// Account and licence (GDK).
	restoreAccount func(ctx context.Context) error
	accountSaved   func() bool
	licenceState   func(ctx context.Context, channel string) string // authorized, trial, not_entitled, error
	// Instances.
	validateName     func(name string) string
	extractMsixvc    func(ctx context.Context, path, name string) string
	extractAppx      func(ctx context.Context, path, name, channel string) string
	instanceMeta     func(name string) versions.VersionMeta
	instanceDir      func(name string) string
	saveMeta         func(name, gameVersion, channel string, isolation bool) string
	inheritable      func(channel string) []string
	systemDataDir    func(preview bool) string
	copyFromSystem   func(preview bool, name string) string
	copyFromInstance func(source, name string) string
	deleteInstance   func(name string) string
	setDefault       func(name string) error
	// LeviLamina.
	fetchLoaderDB   func(ctx context.Context) (map[string][]string, error)
	installLoader   func(ctx context.Context, gameVersion, name, version string) string
	uninstallLoader func(ctx context.Context, name string) string
	loaderStates    func(ctx context.Context, dir string) (map[string]lip.PackageInstallState, error)
}

// gi is the game-install backend; tests replace it.
var gi = realGameInstallBackend()

func realGameInstallBackend() gameInstallBackend {
	gdkClient := gdkversions.NewClient(httpx.NewClient(5*time.Second), xbox.GetPackageUpdateAuthorization)
	return gameInstallBackend{
		fetchGDK: func(ctx context.Context) gdkversions.Catalog {
			return gdkClient.Fetch(ctx, utils.IsMirrorEnabled(), apppath.GDKPendingVersionsPath())
		},
		loadUWP:       uwpdownload.LoadCatalog,
		catalogCache:  func() string { return filepath.Join(apppath.CacheDir(), "catalog", "gdk-versions.json") },
		installersDir: apppath.InstallersDir,
		probeMirrors: func(ctx context.Context, urls []string, timeout time.Duration) []catalog.MirrorResult {
			return catalog.ProbeMirrors(ctx, httpx.NewClient(timeout), urls, timeout, httpx.ApplyDefaultHeaders)
		},
		download: downloader.Download,
		resolveUWP: func(ctx context.Context, id string) (string, func(string) error, error) {
			p, err := uwpdownload.ResolvePackage(ctx, id)
			if err != nil {
				return "", nil, err
			}
			return p.URL, p.Verify, nil
		},
		removeFile:     os.Remove,
		restoreAccount: RestoreAccount,
		accountSaved: func() bool {
			return utils.FileExists(filepath.Join(apppath.AccountsDir(), "wam-account.dpapi"))
		},
		licenceState: func(ctx context.Context, channel string) string {
			id, err := xbox.GetLocalUserId()
			if err != nil {
				return "error"
			}
			l := nativeinstall.CheckGameLicenses(ctx, apppath.AccountsDir(), strconv.FormatUint(id, 10))
			if catalog.NormalizeChannel(channel) == catalog.ChannelPreview {
				return l.Preview
			}
			return l.Release
		},
		validateName: mcservice.ValidateVersionFolderName,
		extractMsixvc: func(ctx context.Context, path, name string) string {
			return mcservice.InstallExtractMsixvc(ctx, path, name, false)
		},
		extractAppx:  mcservice.InstallExtractAppx,
		instanceMeta: mcservice.GetVersionMeta,
		instanceDir: func(name string) string {
			dir, err := apppath.InstancesDir()
			if err != nil {
				return ""
			}
			return filepath.Join(dir, name)
		},
		saveMeta: func(name, gameVersion, channel string, isolation bool) string {
			return mcservice.SaveVersionMeta(name, gameVersion, channel, isolation, false, false, "", "")
		},
		inheritable:      mcservice.ListInheritableVersionNames,
		systemDataDir:    utils.GetMinecraftGDKDataPath,
		copyFromSystem:   mcservice.CopyVersionDataFromGDK,
		copyFromInstance: mcservice.CopyVersionDataFromVersion,
		deleteInstance:   mcservice.DeleteVersionFolder,
		setDefault:       giSetDefaultInstance,
		fetchLoaderDB: func(ctx context.Context) (map[string][]string, error) {
			type result struct {
				db  map[string][]string
				err error
			}
			ch := make(chan result, 1)
			go func() {
				db, err := mcservice.FetchLeviLaminaVersionDB()
				ch <- result{db, err}
			}()
			select {
			case r := <-ch:
				return r.db, r.err
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		installLoader:   mcservice.InstallLeviLamina,
		uninstallLoader: mcservice.UninstallLeviLamina,
		loaderStates:    lip.ListPackageStates,
	}
}

func init() {
	RegisterBackendErrorsFor([]string{"installer", "instance create", "loader"}, map[string]BackendError{
		// Installer files and downloads.
		"ERR_MSIXVC_NOT_FOUND":          {Exit: ExitNotFound, Message: "the installer file was not found"},
		"ERR_MSIXVC_NOT_SPECIFIED":      {Exit: ExitUsage, Message: "no installer file was given"},
		"ERR_UWP_PACKAGE_NOT_FOUND":     {Exit: ExitNotFound, Message: "the installer file was not found"},
		"ERR_UWP_NOT_SPECIFIED":         {Exit: ExitUsage, Message: "no installer file was given"},
		"ERR_ACCESS_INSTALLERS_DIR":     {Exit: ExitFailure, Message: "cannot access the installers folder"},
		"ERR_WRITE_TARGET":              {Exit: ExitFailure, Message: "cannot write the target file or folder"},
		"ERR_UWP_DOWNLOAD_URL":          {Exit: ExitNetwork, Message: "cannot get the download address from Windows Update"},
		"ERR_UWP_INTEGRITY":             {Exit: ExitFailure, Message: "the downloaded package is corrupted (digest mismatch)", Hint: "run the same command again"},
		"ERR_UWP_INVALID_VERSION":       {Exit: ExitUsage, Message: "not a valid UWP build"},
		"ERR_CATALOG_UNAVAILABLE":       {Exit: ExitNetwork, Message: "the Minecraft build catalogue is unreachable", Hint: "check the network connection and try again"},
		"ERR_INSTALLER_IN_USE":          {Exit: ExitConflict, Message: "the installer file is in use"},
		"ERR_INSTALLER_NOT_FOUND":       {Exit: ExitNotFound, Message: "no downloaded installer matches"},
		"ERR_BUILD_NOT_FOUND":           {Exit: ExitNotFound, Message: "no Minecraft build matches"},
		"ERR_DOWNLOAD_FAILED":           {Exit: ExitNetwork, Message: "the download failed"},
		"ERR_TARGET_EXISTS":             {Exit: ExitConflict, Message: "the instance folder already exists"},
		"ERR_INVALID_FOLDER_NAME":       {Exit: ExitUsage, Message: "invalid instance name"},
		"ERR_NATIVE_MSIXVC":             {Exit: ExitFailure, Message: "extracting the GDK installer failed"},
		"ERR_MSIXVC_UNSUPPORTED_FORMAT": {Exit: ExitFailure, Message: "the installer is not a supported GDK package (GDK installs also need 64-bit Windows)"},
		// Instance names.
		"ERR_NAME_REQUIRED":           {Exit: ExitUsage, Message: "the instance name is empty"},
		"ERR_NAME_TOO_LONG":           {Exit: ExitUsage, Message: "the instance name is longer than 64 characters"},
		"ERR_NAME_TRAILING_DOT_SPACE": {Exit: ExitUsage, Message: "the instance name must not end with a dot or a space"},
		"ERR_NAME_INVALID_CHAR":       {Exit: ExitUsage, Message: `the instance name must not contain any of < > : " / \ | ? *`},
		"ERR_NAME_CONTROL_CHAR":       {Exit: ExitUsage, Message: "the instance name must not contain control characters"},
		"ERR_NAME_RESERVED":           {Exit: ExitUsage, Message: "the instance name is reserved by Windows"},
		"ERR_NAME_EXISTS":             {Exit: ExitConflict, Message: "an instance with this name already exists"},
		"ERR_READ_VERSIONS_DIR":       {Exit: ExitFailure, Message: "cannot read the instances folder"},
		"ERR_PREPARE_EXE":             {Exit: ExitFailure, Message: "cannot prepare the game executable"},
		// Account and licence.
		"ERR_AUTH_ACCOUNT_SELECTION": {Exit: ExitAuth, Message: "the saved Microsoft account cannot be used", Hint: ProgramName + " account login"},
		"ERR_AUTH_DEVICE_CACHE":      {Exit: ExitAuth, Message: "the device licence cache cannot be used"},
		"ERR_AUTH_DEVICE_PROVISION":  {Exit: ExitAuth, Message: "this device could not be registered for game licences"},
		"ERR_AUTH_DEVICE_TOKEN":      {Exit: ExitNetwork, Message: "Microsoft did not issue a device licence token", Hint: "turn off any proxy or network accelerator, check the system clock, then try again"},
		"ERR_LICENSE_NOT_ENTITLED": {Exit: ExitAuth, Message: "the signed-in Microsoft account does not own Minecraft for Windows",
			Hint: ProgramName + " account login (with the account that bought the game)"},
		"ERR_LICENSE_FULL_REQUIRED":   {Exit: ExitAuth, Message: "the signed-in account only has a trial licence; the full game is needed"},
		"ERR_LICENSE_DEVICE_MISMATCH": {Exit: ExitAuth, Message: "the game licence belongs to another device"},
		"ERR_LICENSE_MISSING_KEY":     {Exit: ExitAuth, Message: "no decryption key was granted for this build"},
		"ERR_LICENSE_UNSUPPORTED":     {Exit: ExitAuth, Message: "this build's licence type is not supported"},
		"ERR_LICENSE_CATALOG":         {Exit: ExitNetwork, Message: "cannot read the Microsoft Store catalogue"},
		"ERR_LICENSE_HTTP":            {Exit: ExitNetwork, Message: "cannot reach the Microsoft licence service"},
		// UWP extraction.
		"ERR_UWP_CHANNEL":         {Exit: ExitUsage, Message: "unknown UWP channel"},
		"ERR_UWP_ARCHIVE":         {Exit: ExitFailure, Message: "the UWP package cannot be read"},
		"ERR_UWP_UNSAFE_ARCHIVE":  {Exit: ExitFailure, Message: "the UWP package contains unsafe paths"},
		"ERR_UWP_MANIFEST":        {Exit: ExitFailure, Message: "the UWP package has no valid manifest"},
		"ERR_UWP_NOT_MINECRAFT":   {Exit: ExitUsage, Message: "the package is not Minecraft"},
		"ERR_UWP_RESOURCE_BUNDLE": {Exit: ExitFailure, Message: "the UWP bundle has no x64 Minecraft package"},
		"ERR_UWP_ARCHITECTURE":    {Exit: ExitFailure, Message: "the UWP package is not for x64"},
		"ERR_UWP_INSTALL":         {Exit: ExitFailure, Message: "extracting the UWP package failed"},
		// Copying game data.
		"ERR_INHERIT_SOURCE_NOT_FOUND": {Exit: ExitNotFound, Message: "the instance to copy game data from has no game data"},
		"ERR_INHERIT_TARGET_NOT_FOUND": {Exit: ExitNotFound, Message: "the new instance was not found"},
		"ERR_INHERIT_TYPE_MISMATCH":    {Exit: ExitUsage, Message: "game data can only be copied between GDK instances of the same channel"},
		"ERR_INHERIT_COPY_FAILED":      {Exit: ExitFailure, Message: "copying the game data failed"},
		"ERR_NO_GAME_DATA":             {Exit: ExitNotFound, Message: "the installed Minecraft has no game data on this PC"},
		"ERR_CREATE_TARGET_DIR":        {Exit: ExitFailure, Message: "cannot create the game data folder"},
		"ERR_TARGET_DIR_NOT_SPECIFIED": {Exit: ExitUsage, Message: "no instance was given"},
		// LeviLamina.
		"ERR_FETCH_LL_DB": {Exit: ExitNetwork, Message: "cannot download the LeviLamina version list"},
		"ERR_LL_NOT_SUPPORTED": {Exit: ExitUsage, Message: "LeviLamina does not support this game version",
			Hint: ProgramName + " installer search --loader"},
		"ERR_LL_VERSION_UNSUPPORTED":  {Exit: ExitNotFound, Message: "this LeviLamina version does not support the game version"},
		"ERR_INVALID_VERSION_FORMAT":  {Exit: ExitUsage, Message: "the instance's game version is not known"},
		"ERR_UWP_UNSUPPORTED_FEATURE": {Exit: ExitConflict, Message: "LeviLamina needs a GDK instance; UWP instances are not supported"},
		"ERR_LIP_INSTALL_FAILED":      {Exit: ExitFailure, Message: "installing LeviLamina failed", Hint: "run with -v for the package manager's log"},
		"ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS": {Exit: ExitConflict,
			Message: "LeviLamina is required by installed mods and cannot be removed on its own"},
		"ERR_LIP_PACKAGE_UNINSTALL_FAILED": {Exit: ExitFailure, Message: "removing LeviLamina failed", Hint: "run with -v for the package manager's log"},
		"ERR_TARGET_NOT_FOUND":             {Exit: ExitNotFound, Message: "the instance folder was not found"},
	})
}

// giCtxErr turns a backend result into an error, reporting an interrupted
// operation as such rather than as the backend's cancellation code.
func giCtxErr(c *Ctx, result string) error {
	if strings.TrimSpace(result) == "" {
		return nil
	}
	if err := c.Context().Err(); err != nil {
		return FromError(err)
	}
	return Backend(result)
}

// giSetDefaultInstance makes name the default instance.
func giSetDefaultInstance(name string) error {
	if err := config.SetCurrentInstance(name); err != nil {
		return Errorf(ExitFailure, "ERR_CONFIG_WRITE_FAILED", "cannot save the default instance: %v", err)
	}
	return nil
}

// ---- catalogue ----

// giLoadCatalog reads the build catalogue of typ ("" for both).
func giLoadCatalog(c *Ctx, typ string) (catalog.Loaded, error) {
	wantGDK, wantUWP := typ != catalog.TypeUWP, typ != catalog.TypeGDK
	p := c.Progress()
	p.Phase("Reading the Minecraft build catalogue")
	if wantGDK && gi.restoreAccount != nil {
		// The Store's newest builds are found with the saved account, if any.
		_ = gi.restoreAccount(c.Context())
	}
	loaded, err := catalog.Load(c.Context(), catalog.Source{
		FetchGDK: gi.fetchGDK, LoadUWP: gi.loadUWP, CachePath: gi.catalogCache(),
	}, wantGDK, wantUWP)
	p.Done()
	if err != nil {
		if errors.Is(err, catalog.ErrUnavailable) {
			return loaded, Backend("ERR_CATALOG_UNAVAILABLE")
		}
		return loaded, FromError(err)
	}
	switch {
	case loaded.GDKStale:
		c.Warn("the GDK build list could not be refreshed; using the copy from %s", loaded.CachedAt.Local().Format("2006-01-02 15:04"))
	case loaded.GDKMissing && wantUWP:
		c.Warn("the GDK build list is unreachable; only UWP builds are listed")
	case loaded.GDKMissing:
		c.Warn("the GDK build list is unreachable; only the newest Store builds are listed")
	}
	return loaded, nil
}

// giChannelsAndType reads --channel and --type and rejects beta on GDK.
func giChannelsAndType(c *Ctx) ([]string, string, error) {
	channels, typ := c.Strings("channel"), c.String("type")
	if typ == catalog.TypeGDK && containsFold(channels, catalog.ChannelBeta) {
		return nil, "", UsageError("beta builds exist only for UWP").WithHint("use --channel preview, or --type uwp")
	}
	return channels, typ, nil
}

// giResolveBuild resolves a version spec against the catalogue.
func giResolveBuild(c *Ctx, spec string, channels []string, typ string) (catalog.Build, error) {
	spec = strings.TrimSpace(spec)
	if !catalog.ValidSpec(spec) {
		return catalog.Build{}, UsageError("%q is not a Minecraft version", spec).
			WithHint("%s installer search, or give a version like 1.21.50 or latest", ProgramName)
	}
	loaded, err := giLoadCatalog(c, typ)
	if err != nil {
		return catalog.Build{}, err
	}
	if loaded.GDKMissing && typ != catalog.TypeUWP {
		return catalog.Build{}, Errorf(ExitNetwork, "ERR_CATALOG_UNAVAILABLE", "the GDK build catalogue is unreachable").
			WithHint("check the network connection and try again, or pass --type uwp")
	}
	b, err := catalog.Resolve(loaded.Builds, spec, catalog.Filter{Channels: channels, Type: typ})
	if err != nil {
		e := Errorf(ExitNotFound, "ERR_BUILD_NOT_FOUND", "no Minecraft build matches %s%s", spec, giFilterText(channels, typ))
		if near := catalog.Nearest(catalog.Select(loaded.Builds, catalog.Filter{Channels: channels, Type: typ}), spec, 3); len(near) > 0 && !strings.EqualFold(spec, catalog.Latest) {
			return catalog.Build{}, e.WithHint("did you mean %s? See %s installer search %s", strings.Join(near, ", "), ProgramName, giSearchPrefix(spec))
		}
		return catalog.Build{}, e.WithHint("%s installer search %s", ProgramName, giSearchPrefix(spec))
	}
	if !catalog.VersionEqual(b.Version, spec) {
		c.Notice("Resolved %s to %s (%s, %s)", spec, b.Version, giTypeLabel(b.Type), b.Channel)
	}
	return b, nil
}

func giSearchPrefix(spec string) string {
	if strings.EqualFold(spec, catalog.Latest) {
		return ""
	}
	parts := strings.Split(spec, ".")
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, ".")
}

func giFilterText(channels []string, typ string) string {
	var parts []string
	if len(channels) > 0 {
		parts = append(parts, strings.Join(channels, "/"))
	}
	if typ != "" {
		parts = append(parts, giTypeLabel(typ))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func giTypeLabel(t string) string { return strings.ToUpper(catalog.NormalizeType(t)) }

// giLoaderDB downloads the LeviLamina version database.
func giLoaderDB(c *Ctx) (catalog.LoaderDB, error) {
	raw, err := gi.fetchLoaderDB(c.Context())
	if err != nil {
		if c.Context().Err() != nil {
			return nil, FromError(c.Context().Err())
		}
		return nil, Backend("ERR_FETCH_LL_DB").WithDetail(err.Error())
	}
	return catalog.NormalizeLoaderDB(raw), nil
}

// giInstalledAs lists the instances installed from build b.
func giInstalledAs(all []Instance, b catalog.Build) []string {
	out := []string{}
	for _, inst := range all {
		if catalog.NormalizeType(inst.PackageType) == b.Type && catalog.NormalizeChannel(inst.Channel) == b.Channel &&
			catalog.VersionEqual(inst.Meta.GameVersion, b.Version) {
			out = append(out, inst.Name)
		}
	}
	return out
}

// giScanInstallers lists the installers folder.
func giScanInstallers() (string, []catalog.InstallerFile, error) {
	dir, err := gi.installersDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return "", nil, Backend("ERR_ACCESS_INSTALLERS_DIR")
	}
	files, err := catalog.ScanInstallers(dir)
	if err != nil {
		return dir, nil, Backend("ERR_ACCESS_INSTALLERS_DIR").WithDetail(err.Error())
	}
	return dir, files, nil
}

// giNotRunning fails with exit 4 when inst's game is running.
func giNotRunning(inst Instance, doing string) error {
	if pids := instRunningPIDs(inst); len(pids) > 0 {
		e := instRunningError(inst, pids)
		e.Message += "; close it before " + doing
		return e
	}
	return nil
}

// ---- downloads ----

// giPickMirror chooses the URL a GDK build is downloaded from: mirror when
// given ("auto" or "" measures the mirrors and takes the best).
func giPickMirror(c *Ctx, b catalog.Build, mirror string) (string, error) {
	mirror = strings.TrimSpace(mirror)
	if mirror != "" && !strings.EqualFold(mirror, "auto") {
		u, err := url.Parse(mirror)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return "", UsageError("--mirror must be auto or an http(s) URL").
				WithHint("%s installer mirrors %s", ProgramName, b.Version)
		}
		known := false
		for _, m := range b.URLs {
			if strings.EqualFold(strings.TrimSpace(m), mirror) {
				known = true
			}
		}
		if !known {
			c.Warn("%s is not one of the listed mirrors of %s; the download is still checked against the catalogue", catalog.MirrorHost(mirror), b.Version)
		}
		return mirror, nil
	}
	switch len(b.URLs) {
	case 0:
		return "", Errorf(ExitNetwork, "ERR_NO_MIRROR", "the catalogue lists no download address for %s", b.Version)
	case 1:
		return b.URLs[0], nil
	}
	p := c.Progress()
	p.Phase("Testing %d download mirrors", len(b.URLs))
	results := gi.probeMirrors(c.Context(), b.URLs, catalog.DefaultProbeTimeout)
	p.Done()
	if err := c.Context().Err(); err != nil {
		return "", FromError(err)
	}
	best, ok := catalog.BestMirror(results)
	if !ok {
		return b.URLs[0], nil
	}
	if !best.OK {
		c.Warn("no download mirror answered; trying %s", best.Host)
	} else {
		c.Verbosef("Using mirror %s (%d ms)", best.Host, best.LatencyMs)
	}
	return best.URL, nil
}

// giInstallerResult describes an installer made available for a build.
type giInstallerResult struct {
	Path       string
	Downloaded bool   // false when the file was already there
	Mirror     string // the URL downloaded from (GDK)
}

// giEnsureInstaller returns b's installer, downloading it unless it is
// already there (force downloads again). A cancelled download keeps its
// partial file for the next run.
func giEnsureInstaller(c *Ctx, b catalog.Build, mirror string, force bool) (giInstallerResult, error) {
	if b.Type == catalog.TypeUWP && strings.TrimSpace(mirror) != "" && !strings.EqualFold(mirror, "auto") {
		return giInstallerResult{}, UsageError("--mirror applies to GDK builds only; UWP builds come from Windows Update")
	}
	dir, files, err := giScanInstallers()
	if err != nil {
		return giInstallerResult{}, err
	}
	if f, ok := catalog.FindInstaller(files, b); ok && !force {
		return giInstallerResult{Path: f.Path}, nil
	}
	name, err := catalog.FileName(b)
	if err != nil {
		return giInstallerResult{}, Backend("ERR_UWP_INVALID_VERSION").WithDetail(err.Error())
	}
	dest := filepath.Join(dir, name)
	for _, f := range files {
		if f.State == catalog.StateDownloading && strings.EqualFold(f.Name, name) {
			return giInstallerResult{}, Errorf(ExitConflict, "ERR_DOWNLOAD_BUSY", "%s is being downloaded by another process", name).
				WithHint("wait for it to finish, then run the command again")
		}
	}
	if force && utils.FileExists(dest) {
		if err := gi.removeFile(dest); err != nil {
			return giInstallerResult{}, giRemoveError(name, err)
		}
	}

	opts := downloader.SyncOptions{}
	src, from := "", ""
	if b.Type == catalog.TypeGDK {
		if src, err = giPickMirror(c, b, mirror); err != nil {
			return giInstallerResult{}, err
		}
		opts.MD5, from = b.MD5, " from "+catalog.MirrorHost(src)
		if b.MD5 == "" {
			c.Verbosef("The catalogue has no checksum for %s; the download is not verified", b.Version)
		}
	} else {
		p := c.Progress()
		p.Phase("Asking Windows Update for %s", b.Version)
		var verify func(string) error
		src, verify, err = gi.resolveUWP(c.Context(), b.UUID)
		p.Done()
		if err != nil {
			if c.Context().Err() != nil {
				return giInstallerResult{}, FromError(c.Context().Err())
			}
			return giInstallerResult{}, Backend("ERR_UWP_DOWNLOAD_URL").WithDetail(err.Error())
		}
		opts.Verify = verify
	}
	if catalog.PartialPath(dest) != "" && utils.FileExists(catalog.PartialPath(dest)) {
		c.Notice("Resuming the unfinished download of %s", name)
	}
	p := c.Progress()
	p.Phase("Downloading %s%s", name, from)
	opts.Progress = p.ProgressFunc()
	opts.Status = func(s string) {
		if s == "verifying" {
			p.Phase("Verifying %s", name)
		}
	}
	err = gi.download(c.Context(), src, dest, opts)
	p.Done()
	if err != nil {
		if c.Context().Err() != nil || errors.Is(err, context.Canceled) {
			c.Notice("The unfinished download is kept; run the same command to resume it.")
			return giInstallerResult{}, FromError(context.Canceled)
		}
		return giInstallerResult{}, giDownloadError(name, err)
	}
	return giInstallerResult{Path: dest, Downloaded: true, Mirror: src}, nil
}

// giDownloadError maps a download failure to an exit code.
func giDownloadError(name string, err error) *Error {
	if errors.Is(err, downloader.ErrBusy) {
		return Backend("ERR_DOWNLOAD_BUSY").WithHint("wait for the other download to finish, then run the command again")
	}
	msg := err.Error()
	if code, _, ok := SplitBackendCode(msg); ok {
		return Backend(code).WithDetail(msg)
	}
	if strings.Contains(msg, "MD5 mismatch") {
		return Backend("ERR_MD5_MISMATCH").WithDetail(msg)
	}
	exit := ExitNetwork
	var pathErr *os.PathError
	if errors.As(err, &pathErr) || strings.Contains(msg, "MD5 calculation") {
		exit = ExitFailure
	}
	return Errorf(exit, "ERR_DOWNLOAD_FAILED", "downloading %s failed: %s", name, msg).
		WithHint("run the same command again to resume")
}

// giRemoveError maps a failed file removal: a file another process holds
// open is in use (exit 4).
func giRemoveError(name string, err error) *Error {
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return Errorf(ExitConflict, "ERR_INSTALLER_IN_USE", "%s is in use by another program", name).
			WithHint("wait until the download or installation using it ends")
	}
	if errors.Is(err, os.ErrNotExist) {
		return Errorf(ExitNotFound, "ERR_INSTALLER_NOT_FOUND", "%s no longer exists", name)
	}
	return Errorf(ExitFailure, "ERR_WRITE_TARGET", "cannot remove %s: %v", name, err)
}

// ---- installer search ----

type installerSearchItem struct {
	Version        string     `json:"version"`
	Channel        string     `json:"channel"`
	Type           string     `json:"type"`
	Released       *time.Time `json:"released"`
	MD5            string     `json:"md5"`
	UUID           string     `json:"uuid,omitempty"`
	URLs           []string   `json:"urls"`
	Downloaded     bool       `json:"downloaded"`
	Partial        bool       `json:"partial"`
	InstallerPath  *string    `json:"installerPath"`
	InstalledAs    []string   `json:"installedAs"`
	LoaderVersions []string   `json:"loaderVersions"`
}

func giChannelFlag(usage string) *Flag {
	return EnumsFlag("channel", "", catalog.Channels, usage)
}

func init() {
	Register(&Command{
		Path:    "installer search",
		Summary: "list the Minecraft builds that can be installed",
		Args:    "[<version>]",
		Description: `Lists installable builds newest first: GDK builds (msixvc) and UWP builds
(appx). Shows which installers are downloaded, which instances use each build
and the LeviLamina versions that support it. <version> matches part of the
version, e.g. 1.21.5 or 1.21.50.

Only release builds are listed unless --channel says otherwise. Beta builds
exist only for UWP. On a console at most 20 rows are shown; --limit 0 shows all.`,
		Examples: []string{"installer search 1.21", "installer search --channel preview --type gdk", "installer search --loader --json"},
		Flags: []*Flag{
			giChannelFlag("channel to list (default release)"),
			EnumFlag("type", "", []string{catalog.TypeGDK, catalog.TypeUWP}, "only GDK or UWP builds (default both)"),
			BoolFlag("downloaded", "", "only builds whose installer is downloaded"),
			BoolFlag("not-downloaded", "", "only builds whose installer is not downloaded"),
			BoolFlag("loader", "", "only GDK builds LeviLamina supports"),
			IntFlag("limit", "", "<n>", "show at most n builds (0 = all; default 20 on a console)"),
		},
		MaxArgs: 1,
		Run:     runInstallerSearch,
	})
}

func runInstallerSearch(c *Ctx) error {
	channels, typ, err := giChannelsAndType(c)
	if err != nil {
		return err
	}
	if len(channels) == 0 {
		channels = []string{catalog.ChannelRelease}
	}
	if c.Bool("downloaded") && c.Bool("not-downloaded") {
		return UsageError("--downloaded and --not-downloaded exclude each other")
	}
	onlyLoader := c.Bool("loader")
	if onlyLoader {
		if typ == catalog.TypeUWP {
			return UsageError("LeviLamina supports GDK builds only; drop --type uwp")
		}
		typ = catalog.TypeGDK
	}
	limit := 0
	if c.Changed("limit") {
		if limit = c.Int("limit"); limit < 0 {
			return UsageError("--limit must be 0 or more")
		}
	} else if c.StdoutIsConsole() && !c.G.JSON && !c.G.Quiet {
		limit = 20
	}

	// The LeviLamina list is fetched alongside the catalogue.
	type dbResult struct {
		db  catalog.LoaderDB
		err error
	}
	dbCh := make(chan dbResult, 1)
	if typ != catalog.TypeUWP {
		go func() {
			raw, err := gi.fetchLoaderDB(c.Context())
			dbCh <- dbResult{catalog.NormalizeLoaderDB(raw), err}
		}()
	} else {
		dbCh <- dbResult{}
	}
	loaded, err := giLoadCatalog(c, typ)
	if err != nil {
		return err
	}
	dbr := <-dbCh
	if dbr.err != nil {
		if c.Context().Err() != nil {
			return FromError(c.Context().Err())
		}
		if onlyLoader {
			return Backend("ERR_FETCH_LL_DB").WithDetail(dbr.err.Error())
		}
		c.Warn("the LeviLamina version list is unreachable; the LOADER column is empty")
	}
	_, files, err := giScanInstallers()
	if err != nil {
		return err
	}
	insts, err := ListInstances()
	if err != nil {
		return err
	}

	want := c.Bool("downloaded")
	wantSet := c.Changed("downloaded") && want
	notSet := c.Changed("not-downloaded") && c.Bool("not-downloaded")
	items := []installerSearchItem{}
	for _, b := range catalog.Select(loaded.Builds, catalog.Filter{Channels: channels, Type: typ, Query: c.Arg(0)}) {
		ll := []string{}
		if b.Type == catalog.TypeGDK && dbr.db != nil {
			ll = append(ll, dbr.db.Versions(b.Version)...)
		}
		if onlyLoader && len(ll) == 0 {
			continue
		}
		f, downloaded := catalog.FindInstaller(files, b)
		if (wantSet && !downloaded) || (notSet && downloaded) {
			continue
		}
		it := installerSearchItem{
			Version: b.Version, Channel: b.Channel, Type: b.Type, MD5: b.MD5, UUID: b.UUID,
			URLs: append([]string{}, b.URLs...), Downloaded: downloaded, InstalledAs: giInstalledAs(insts, b), LoaderVersions: ll,
		}
		if b.Timestamp > 0 {
			t := time.Unix(b.Timestamp, 0).UTC()
			it.Released = &t
		}
		if downloaded {
			p := f.Path
			it.InstallerPath = &p
		} else if _, ok := catalog.PartialOf(files, b); ok {
			it.Partial = true
		}
		items = append(items, it)
	}
	total := len(items)
	if limit > 0 && total > limit {
		items = items[:limit]
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.Version
	}
	if total == 0 {
		c.Notice("No build matches.")
	} else if len(items) < total {
		c.Notice("Showing %d of %d builds; use --limit 0 for all.", len(items), total)
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		rows := make([][]string, len(items))
		for i, it := range items {
			released := "-"
			if it.Released != nil {
				released = it.Released.Local().Format("2006-01-02")
			}
			dl := "no"
			switch {
			case it.Downloaded:
				dl = "yes"
			case it.Partial:
				dl = "partial"
			}
			loader := "-"
			if len(it.LoaderVersions) > 0 {
				loader = it.LoaderVersions[0]
				if s := catalog.LoaderLatestStable(it.LoaderVersions); s != "" {
					loader = s
				}
			}
			rows[i] = []string{it.Version, it.Channel, giTypeLabel(it.Type), released, dl, Dash(strings.Join(it.InstalledAs, ", ")), loader}
		}
		return Table(w, []string{"VERSION", "CHANNEL", "TYPE", "RELEASED", "DOWNLOADED", "INSTALLED-AS", "LOADER"}, rows)
	})
}

// ---- installer download ----

type installerDownloadResult struct {
	Version    string `json:"version"`
	Channel    string `json:"channel"`
	Type       string `json:"type"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Downloaded bool   `json:"downloaded"`
	Mirror     string `json:"mirror,omitempty"`
}

func init() {
	Register(&Command{
		Path:    "installer download",
		Summary: "download a build's installer without installing it",
		Args:    "<version>",
		Description: `Downloads the installer of a build and verifies it, without creating an
instance. <version> is an exact build, a dotted prefix (1.21.50 is the newest
1.21.50.x) or latest. GDK builds come from the fastest mirror unless --mirror
names one; UWP builds come from Windows Update.

Ctrl-C keeps the unfinished download; running the same command resumes it.
An installer that is already downloaded is not downloaded again unless
--force is given. The installer's path is printed.`,
		Examples: []string{"installer download latest", "installer download 1.21.50 --channel preview", "installer download 1.21.114.1 --type uwp"},
		Flags: []*Flag{
			giChannelFlag("channel of the build (release, beta or preview)"),
			EnumFlag("type", "", []string{catalog.TypeGDK, catalog.TypeUWP}, "GDK or UWP build (default GDK when both exist)"),
			StringFlag("mirror", "", "<auto|url>", "GDK download mirror (default auto: the fastest)"),
			BoolFlag("force", "", "download again even if the installer is already downloaded"),
		},
		MinArgs: 1, MaxArgs: 1,
		Run: runInstallerDownload,
	})
}

func runInstallerDownload(c *Ctx) error {
	channels, typ, err := giChannelsAndType(c)
	if err != nil {
		return err
	}
	b, err := giResolveBuild(c, c.Arg(0), channels, typ)
	if err != nil {
		return err
	}
	res, err := giEnsureInstaller(c, b, c.String("mirror"), c.Bool("force"))
	if err != nil {
		return err
	}
	if !res.Downloaded {
		c.Notice("%s is already downloaded.", filepath.Base(res.Path))
	}
	out := installerDownloadResult{Version: b.Version, Channel: b.Channel, Type: b.Type, Path: res.Path, Downloaded: res.Downloaded, Mirror: res.Mirror}
	if info, err := os.Stat(res.Path); err == nil {
		out.Size = info.Size()
	}
	return c.Render(out, []string{res.Path}, func(w io.Writer) error {
		_, err := fmt.Fprintln(w, res.Path)
		return err
	})
}

// ---- installer mirrors ----

type installerMirrorItem struct {
	Host      string `json:"host"`
	URL       string `json:"url"`
	LatencyMs *int64 `json:"latencyMs"`
	OK        bool   `json:"ok"`
	Best      bool   `json:"best"`
}

func init() {
	Register(&Command{
		Path:    "installer mirrors",
		Summary: "measure the download mirrors of a GDK build",
		Args:    "<version>",
		Description: `Sends a request to every download mirror of a GDK build and lists them
fastest first. The mirror marked '*' is the one 'installer download' and
'install' use by default; pass another URL to their --mirror flag to use it.`,
		Examples: []string{"installer mirrors latest", "installer mirrors 1.21.50 --timeout 3s", "installer mirrors 1.21.60 --channel preview -q"},
		Flags: []*Flag{
			EnumsFlag("channel", "", []string{catalog.ChannelRelease, catalog.ChannelPreview}, "channel of the build"),
			DurationFlag("timeout", "", "time to wait for each mirror").WithDefault("7s"),
		},
		MinArgs: 1, MaxArgs: 1,
		Run: runInstallerMirrors,
	})
}

func runInstallerMirrors(c *Ctx) error {
	timeout := c.Duration("timeout")
	if timeout <= 0 {
		return UsageError("--timeout must be more than 0")
	}
	b, err := giResolveBuild(c, c.Arg(0), c.Strings("channel"), catalog.TypeGDK)
	if err != nil {
		return err
	}
	if len(b.URLs) == 0 {
		return Errorf(ExitNetwork, "ERR_NO_MIRROR", "the catalogue lists no download address for %s", b.Version)
	}
	p := c.Progress()
	p.Phase("Testing %d download mirrors", len(b.URLs))
	results := gi.probeMirrors(c.Context(), b.URLs, timeout)
	p.Done()
	if err := c.Context().Err(); err != nil {
		return FromError(err)
	}
	best, hasBest := catalog.BestMirror(results)
	items := []installerMirrorItem{}
	for _, r := range catalog.RankMirrors(results) {
		it := installerMirrorItem{Host: r.Host, URL: r.URL, OK: r.OK, Best: hasBest && r.URL == best.URL}
		if r.Measured {
			ms := r.LatencyMs
			it.LatencyMs = &ms
		}
		items = append(items, it)
	}
	var ids []string
	if hasBest {
		ids = []string{best.URL}
	}
	return c.Render(items, ids, func(w io.Writer) error {
		rows := make([][]string, len(items))
		for i, it := range items {
			mark := " "
			if it.Best {
				mark = "*"
			}
			lat := "-"
			if it.LatencyMs != nil {
				lat = fmt.Sprintf("%d ms", *it.LatencyMs)
			}
			rows[i] = []string{mark + " " + it.Host, lat, YesNo(it.OK), it.URL}
		}
		return Table(w, []string{"  HOST", "LATENCY", "OK", "URL"}, rows)
	})
}

// ---- installer list ----

type installerFileItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Channel string `json:"channel"`
	Type    string `json:"type"`
	Size    int64  `json:"size"`
	State   string `json:"state"`
	Path    string `json:"path"`
}

func init() {
	Register(&Command{
		Path:    "installer list",
		Summary: "list downloaded installers, including unfinished downloads",
		Description: `Lists the installer files in the launcher's installers folder, newest
version first. STATE is complete, partial (an unfinished download that the
same download command resumes) or downloading (another process is writing
it). Remove files with 'installer remove'.`,
		Examples: []string{"installer list", "installer ls --json", "installer list -q"},
		Run:      runInstallerList,
	})
}

func giFileItem(f catalog.InstallerFile) installerFileItem {
	return installerFileItem{Name: f.Name, Version: f.Identity.Version, Channel: f.Identity.Channel, Type: f.Identity.Type,
		Size: f.Size, State: f.State, Path: f.Path}
}

func runInstallerList(c *Ctx) error {
	dir, files, err := giScanInstallers()
	if err != nil {
		return err
	}
	items := make([]installerFileItem, 0, len(files))
	ids := make([]string, 0, len(files))
	for _, f := range files {
		items = append(items, giFileItem(f))
		ids = append(ids, f.Path)
	}
	if len(items) == 0 {
		c.Notice("No installer is downloaded (folder: %s).", dir)
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		rows := make([][]string, len(items))
		for i, it := range items {
			rows[i] = []string{Dash(it.Version), Dash(it.Channel), giTypeLabel(it.Type), Size(it.Size), it.State, it.Path}
		}
		return Table(w, []string{"VERSION", "CHANNEL", "TYPE", "SIZE", "STATE", "PATH"}, rows)
	})
}

// ---- installer remove ----

type installerRemoveItem struct {
	installerFileItem
	Removed bool   `json:"removed"`
	Error   string `json:"error,omitempty"`
}

func init() {
	Register(&Command{
		Path:    "installer remove",
		Summary: "delete downloaded installers to free disk space",
		Args:    "<version|file>...",
		Description: `Deletes installer files from the installers folder. Instances already
installed from them are not affected. A version must match exactly (no
prefixes); --channel and --type pick one build when a version exists in
several. A file name or path must be a file in the installers folder.

Unfinished downloads are left alone unless --partial is given. Asks for
confirmation; --yes skips it.`,
		Examples: []string{"installer remove 1.21.50.07", "installer remove 1.21.60.21 --channel preview --partial", `installer rm "Release 1.21.50.07.msixvc" -y`},
		Flags: []*Flag{
			giChannelFlag("only the build of this channel"),
			EnumFlag("type", "", []string{catalog.TypeGDK, catalog.TypeUWP}, "only the GDK or the UWP build"),
			BoolFlag("partial", "", "remove the unfinished download instead of the complete file"),
		},
		MinArgs: 1, MaxArgs: Unlimited,
		Run: runInstallerRemove,
	})
}

// giMatchInstallers resolves one installer remove argument.
func giMatchInstallers(files []catalog.InstallerFile, arg string, channels []string, typ string, partial bool) ([]catalog.InstallerFile, error) {
	arg = strings.TrimSpace(arg)
	stateOK := func(f catalog.InstallerFile) bool { return (f.State == catalog.StateComplete) != partial }
	var out []catalog.InstallerFile
	isPath := strings.ContainsAny(arg, `/\`) || catalog.IsInstallerFile(arg) || strings.HasSuffix(strings.ToLower(arg), ".partial.tmp")
	if isPath {
		abs, _ := filepath.Abs(arg)
		bare := !strings.ContainsAny(arg, `/\`)
		for _, f := range files {
			exact := strings.EqualFold(f.Path, abs) || (bare && strings.EqualFold(filepath.Base(f.Path), arg))
			named := (strings.EqualFold(f.Name, arg) || strings.EqualFold(filepath.Join(filepath.Dir(f.Path), f.Name), abs)) && stateOK(f)
			if exact || named {
				out = append(out, f)
			}
		}
		if len(out) == 0 {
			return nil, Errorf(ExitNotFound, "ERR_INSTALLER_NOT_FOUND", "%s is not a file in the installers folder", arg).
				WithHint("%s installer list", ProgramName)
		}
		return out, nil
	}
	if !catalog.ValidSpec(arg) || strings.EqualFold(arg, catalog.Latest) {
		return nil, UsageError("%q is neither a version nor an installer file", arg).WithHint("%s installer list", ProgramName)
	}
	otherState := false
	for _, f := range files {
		if !f.Known || !catalog.VersionEqual(f.Identity.Version, arg) {
			continue
		}
		if typ != "" && f.Identity.Type != typ {
			continue
		}
		if len(channels) > 0 && f.Identity.Channel != "" && !containsFold(channels, f.Identity.Channel) {
			continue
		}
		if !stateOK(f) {
			otherState = true
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		e := Errorf(ExitNotFound, "ERR_INSTALLER_NOT_FOUND", "no downloaded installer of %s%s", arg, giFilterText(channels, typ))
		switch {
		case otherState && !partial:
			return nil, e.WithHint("only an unfinished download exists; add --partial to remove it")
		case otherState:
			return nil, e.WithHint("the download is complete; drop --partial to remove it")
		}
		return nil, e.WithHint("%s installer list", ProgramName)
	}
	builds := map[string]bool{}
	for _, f := range out {
		builds[f.Identity.Type+":"+f.Identity.Channel] = true
	}
	if len(builds) > 1 {
		names := make([]string, len(out))
		for i, f := range out {
			names[i] = f.Name
		}
		return nil, UsageError("%s matches several installers: %s", arg, strings.Join(names, ", ")).
			WithHint("add --type gdk|uwp or --channel release|beta|preview").withCode("ERR_AMBIGUOUS")
	}
	return out, nil
}

func runInstallerRemove(c *Ctx) error {
	channels, typ, err := giChannelsAndType(c)
	if err != nil {
		return err
	}
	_, files, err := giScanInstallers()
	if err != nil {
		return err
	}
	var targets []catalog.InstallerFile
	seen := map[string]bool{}
	failures := 0
	var firstErr error
	results := []installerRemoveItem{}
	for _, arg := range c.Args {
		matched, err := giMatchInstallers(files, arg, channels, typ, c.Bool("partial"))
		if err != nil {
			if len(c.Args) == 1 {
				return err
			}
			failures++
			if firstErr == nil {
				firstErr = err
			}
			c.errf("error: %s\n", FromError(err).Message)
			continue
		}
		for _, f := range matched {
			if !seen[strings.ToLower(f.Path)] {
				seen[strings.ToLower(f.Path)] = true
				targets = append(targets, f)
			}
		}
	}
	if len(targets) > 0 {
		var total int64
		names := make([]string, len(targets))
		for i, f := range targets {
			total += f.Size
			names[i] = filepath.Base(f.Path)
		}
		q := fmt.Sprintf("Remove %s (%s)?", strings.Join(names, ", "), Size(total))
		if len(targets) > 1 {
			q = fmt.Sprintf("Remove %d installer files (%s): %s?", len(targets), Size(total), strings.Join(names, ", "))
		}
		if err := c.Confirm(q); err != nil {
			return err
		}
	}
	var skipped []string
	for i, f := range targets {
		// Ctrl-C stops the batch between files.
		if c.Context().Err() != nil {
			for _, t := range targets[i:] {
				skipped = append(skipped, filepath.Base(t.Path))
			}
			break
		}
		it := installerRemoveItem{installerFileItem: giFileItem(f)}
		var err error
		if f.State == catalog.StateDownloading {
			err = Errorf(ExitConflict, "ERR_INSTALLER_IN_USE", "%s is being downloaded", filepath.Base(f.Path)).
				WithHint("wait for the download to finish or stop it first")
		} else if rerr := gi.removeFile(f.Path); rerr != nil {
			err = giRemoveError(filepath.Base(f.Path), rerr)
		}
		if err != nil {
			failures++
			if firstErr == nil {
				firstErr = err
			}
			it.Error = FromError(err).Message
			if len(c.Args) > 1 || len(targets) > 1 {
				c.errf("error: %s\n", it.Error)
			}
		} else {
			it.Removed = true
		}
		results = append(results, it)
	}
	removed := 0
	ids := []string{}
	for _, r := range results {
		if r.Removed {
			removed++
			ids = append(ids, r.Path)
		}
	}
	if err := c.Render(results, ids, func(w io.Writer) error {
		for _, r := range results {
			if r.Removed {
				fmt.Fprintf(w, "Removed %s (%s)\n", filepath.Base(r.Path), Size(r.Size))
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if len(skipped) > 0 {
		return Errorf(ExitInterrupted, "ERR_INTERRUPTED", "interrupted; not removed: %s", strings.Join(skipped, ", "))
	}
	switch {
	case failures == 0:
		return nil
	case removed == 0 && len(c.Args) == 1 && len(targets) == 1:
		return firstErr
	case removed == 0:
		c.Notice("No installer was removed.")
		return ExitCode(ExitFailure)
	}
	c.Notice("Removed %d installer files; %d failed.", removed, failures)
	return ExitCode(ExitPartial)
}
