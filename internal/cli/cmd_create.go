package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/catalog"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// instance create (top-level alias: install) ports the install page's
// orchestration: resolve the build, download its installer if needed,
// extract it into a new instance, save the instance settings, optionally
// copy game data and install LeviLamina, and remove the half-created
// instance when any step after the extraction fails.

func init() {
	Register(&Command{
		Path:    "instance create",
		Summary: "download and install Minecraft as a new instance",
		Args:    "<version|file>",
		Description: `Installs Minecraft into a new instance in one step: downloads the installer
unless it is already downloaded (resuming an unfinished download), extracts
it, saves the instance settings, optionally copies existing game data and
installs LeviLamina. If a step after the extraction fails, or Ctrl-C stops
it, the half-created instance is removed again.

<version> is a build from 'installer search': exact (1.21.50.07), a dotted
prefix (1.21.50 is the newest 1.21.50.x, 1.21.5 does not match it) or latest.
GDK is chosen when a version exists as GDK and UWP; beta builds exist only for
UWP. <file> is a local .msixvc, .appx, .msix, .appxbundle or .msixbundle; its
type comes from the extension.

The instance is named after the game version unless --name is given. It keeps
its own game data unless --shared-data is given (UWP builds before 1.19.70.2
always share it). Installing a GDK build needs a signed-in Microsoft account
that owns Minecraft ('account login').`,
		Examples: []string{
			"install latest --loader --use",
			"instance create 1.21.50 --name Survival --copy-data-from system",
			`install D:\Downloads\Minecraft-UWP-Release-1.21.114.1.appx --shared-data`,
		},
		Flags: []*Flag{
			StringFlag("name", "", "<name>", "instance name (default: the game version)"),
			EnumFlag("channel", "", catalog.Channels, "channel of the build, or of a local .msixvc"),
			EnumFlag("type", "", []string{catalog.TypeGDK, catalog.TypeUWP}, "GDK or UWP build (default GDK when both exist)"),
			BoolFlag("shared-data", "", "use Minecraft's normal shared game data instead of the instance's own"),
			StringFlag("copy-data-from", "", "<system|instance>", "seed the instance's own game data from the installed Minecraft or another GDK instance"),
			BoolFlag("loader", "", "also install LeviLamina (GDK only)"),
			StringFlag("loader-version", "", "<version>", "LeviLamina version (implies --loader; default: newest stable)"),
			StringFlag("mirror", "", "<auto|url>", "GDK download mirror (default auto: the fastest)"),
			BoolFlag("use", "", "make the new instance the default instance"),
		},
		MinArgs: 1, MaxArgs: 1,
		Run: runInstanceCreate,
	})
}

// instanceCreateResult is `instance create --json`: the instance as
// `instance list` describes it, plus the installer used.
type instanceCreateResult struct {
	instanceListItem
	Installer           string `json:"installer"`
	InstallerDownloaded bool   `json:"installerDownloaded"`
}

// giCreate is the state of one instance create run.
type giCreate struct {
	c           *Ctx
	local       bool
	path        string // installer path
	build       catalog.Build
	typ         string
	channel     string // "" while unknown (local GDK file)
	channelSet  bool   // --channel given
	gameVersion string // "" while unknown
	name        string
	isolation   bool
	copyFrom    string // "", "system" or an instance name
	copyChecked bool
	loader      bool
	loaderWant  string // --loader-version
	loaderPick  string // resolved LeviLamina version
	loaderDB    catalog.LoaderDB
	downloaded  bool
	created     bool
}

// giLooksLikeFile reports whether a create argument names a local file
// rather than a catalogue version.
func giLooksLikeFile(arg string) bool {
	if strings.ContainsAny(arg, `/\`) || catalog.IsInstallerFile(arg) {
		return true
	}
	if catalog.ValidSpec(arg) {
		return false
	}
	_, err := os.Stat(arg)
	return err == nil
}

func runInstanceCreate(c *Ctx) error {
	g := &giCreate{c: c, isolation: !c.Bool("shared-data"), loaderWant: strings.TrimSpace(c.String("loader-version"))}
	g.loader = c.Bool("loader") || g.loaderWant != ""
	if c.Changed("loader") && !c.Bool("loader") && g.loaderWant != "" {
		return UsageError("--no-loader and --loader-version exclude each other")
	}
	g.copyFrom = strings.TrimSpace(c.String("copy-data-from"))
	if g.copyFrom != "" && !g.isolation {
		return UsageError("--copy-data-from needs the instance's own game data; drop --shared-data")
	}
	if strings.EqualFold(g.copyFrom, "system") {
		g.copyFrom = "system"
	}
	g.channel, g.channelSet = c.String("channel"), c.String("channel") != ""
	typ := c.String("type")
	arg := strings.TrimSpace(c.Arg(0))

	if giLooksLikeFile(arg) {
		if err := g.planLocal(arg, typ); err != nil {
			return err
		}
	} else {
		if typ == catalog.TypeGDK && g.channel == catalog.ChannelBeta {
			return UsageError("beta builds exist only for UWP").WithHint("use --channel preview, or --type uwp")
		}
		var channels []string
		if g.channelSet {
			channels = []string{g.channel}
		}
		b, err := giResolveBuild(c, arg, channels, typ)
		if err != nil {
			return err
		}
		g.build, g.typ, g.channel, g.gameVersion = b, b.Type, b.Channel, b.Version
	}

	if err := g.checkOptions(); err != nil {
		return err
	}
	if err := g.pickName(arg); err != nil {
		return err
	}
	if g.channel != "" && g.copyFrom != "" {
		if err := g.checkCopySource(); err != nil {
			return err
		}
	}
	if g.loader {
		db, err := giLoaderDB(c)
		if err != nil {
			return err
		}
		g.loaderDB = db
		if g.gameVersion != "" {
			if err := g.pickLoader(); err != nil {
				return err
			}
		}
	}
	if !g.local {
		if err := g.fetchInstaller(); err != nil {
			return err
		}
	}
	if err := g.extract(); err != nil {
		return g.rollback(err)
	}
	if err := g.finish(); err != nil {
		return g.rollback(err)
	}
	return g.report()
}

// planLocal prepares an install from a local installer file.
func (g *giCreate) planLocal(arg, typ string) error {
	abs, err := filepath.Abs(arg)
	if err != nil {
		abs = arg
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return Errorf(ExitNotFound, "ERR_FILE_NOT_FOUND", "no installer file %s", arg).
			WithHint("give a .msixvc, .appx, .msix, .appxbundle or .msixbundle file, or a version: %s installer search", ProgramName)
	}
	if !catalog.IsInstallerFile(abs) {
		return UsageError("%s is not a Minecraft installer (.msixvc, .appx, .msix, .appxbundle or .msixbundle)", filepath.Base(abs))
	}
	if strings.TrimSpace(g.c.String("mirror")) != "" {
		return UsageError("--mirror applies to downloads, not to a local file")
	}
	g.local, g.path, g.typ = true, abs, catalog.TypeGDK
	if catalog.IsAppxInstaller(abs) {
		g.typ = catalog.TypeUWP
	}
	if typ != "" && typ != g.typ {
		return UsageError("%s is a %s installer, not %s", filepath.Base(abs), giTypeLabel(g.typ), giTypeLabel(typ))
	}
	if g.typ == catalog.TypeGDK && g.channel == catalog.ChannelBeta {
		return UsageError("beta builds exist only for UWP").WithHint("use --channel release or preview")
	}
	if id, ok := catalog.IdentityFromPath(abs); ok && id.Type == g.typ {
		g.gameVersion = id.Version
		if !g.channelSet && id.Channel != "" {
			g.channel = id.Channel
		}
	}
	if g.typ == catalog.TypeUWP && g.channel == "" {
		// The package manifest decides release or preview; beta must be named.
		g.channel = catalog.ChannelRelease
	}
	return nil
}

// checkOptions rejects options the package type cannot have.
func (g *giCreate) checkOptions() error {
	if g.typ != catalog.TypeUWP {
		return nil
	}
	if g.copyFrom != "" {
		return UsageError("--copy-data-from works for GDK instances only")
	}
	if g.loader {
		return Errorf(ExitUsage, "ERR_UWP_UNSUPPORTED_FEATURE", "LeviLamina supports GDK instances only").
			WithHint("drop --loader, or install a GDK build")
	}
	if g.isolation && g.gameVersion != "" && !versions.SupportsIsolation(catalog.TypeUWP, g.gameVersion) {
		if g.c.Changed("shared-data") {
			// --no-shared-data asks for own data explicitly.
			return UsageError("UWP builds before %s cannot keep their own game data", versions.UWPIsolationMinVersion).
				WithHint("drop --no-shared-data")
		}
		g.c.Notice("UWP builds before %s use the shared game data.", versions.UWPIsolationMinVersion)
		g.isolation = false
	}
	return nil
}

// pickName sets and validates the instance name.
func (g *giCreate) pickName(arg string) error {
	name := strings.TrimSpace(g.c.String("name"))
	explicit := name != ""
	if !explicit {
		name = g.gameVersion
	}
	if name == "" {
		return UsageError("the version of %s cannot be told from its file name", filepath.Base(arg)).
			WithHint("name the instance: %s install %s --name <name>", ProgramName, quoteArg(arg)).withCode("ERR_NAME_REQUIRED")
	}
	if code := gi.validateName(name); code != "" {
		e := Backend(code)
		if code == "ERR_NAME_EXISTS" {
			e.Message = fmt.Sprintf("an instance named %q already exists", name)
			taken := func(n string) bool { return gi.validateName(n) == "ERR_NAME_EXISTS" }
			if free := catalog.FreeName(name, taken); free != "" {
				e.Hint = fmt.Sprintf("%s install %s --name %s", ProgramName, quoteArg(arg), quoteArg(free))
			}
		} else {
			e.Message = fmt.Sprintf("%s: %q", e.Message, name)
		}
		return e
	}
	g.name = name
	return nil
}

// checkCopySource checks --copy-data-from against the install page's rules.
func (g *giCreate) checkCopySource() error {
	g.copyChecked = true
	preview := g.channel == catalog.ChannelPreview
	if g.copyFrom == "system" {
		dir := gi.systemDataDir(preview)
		if strings.TrimSpace(dir) == "" || !utils.DirExists(dir) {
			return Errorf(ExitNotFound, "ERR_NO_GAME_DATA", "there is no %s Minecraft game data on this PC to copy", g.channel)
		}
		return nil
	}
	src, err := LookupInstance(g.copyFrom)
	if err != nil {
		return err
	}
	all, err := ListInstances()
	if err != nil {
		return err
	}
	sources := make([]catalog.InheritSource, 0, len(all))
	for _, inst := range all {
		sources = append(sources, catalog.InheritSource{Name: inst.Name, GameVersion: inst.Meta.GameVersion,
			Channel: inst.Channel, PackageType: inst.PackageType, Isolated: inst.Meta.EnableIsolation})
	}
	for _, s := range catalog.InheritCandidates(sources, gi.inheritable(g.channel), g.channel) {
		if strings.EqualFold(s.Name, src.Name) {
			g.copyFrom = s.Name
			return nil
		}
	}
	why := "it has no game data yet"
	switch {
	case src.IsUWP():
		why = "it is a UWP instance"
	case !src.Meta.EnableIsolation:
		why = "it uses the shared game data"
	case catalog.NormalizeChannel(src.Channel) != g.channel:
		why = fmt.Sprintf("it is a %s instance and the new one is %s", src.Channel, g.channel)
	}
	return Errorf(ExitUsage, "ERR_INHERIT_SOURCE_INVALID", "cannot copy game data from %s: %s", src.Name, why).
		WithHint("use --copy-data-from system, or a GDK instance of the same channel with its own data")
}

// pickLoader resolves the LeviLamina version for the game version.
func (g *giCreate) pickLoader() error {
	v, err := giPickLoaderVersion(g.c, g.loaderDB, g.gameVersion, g.loaderWant)
	if err != nil {
		return err
	}
	if catalog.IsPrerelease(v) {
		if err := giConfirmPrerelease(g.c, v); err != nil {
			return err
		}
	}
	g.loaderPick = v
	return nil
}

// giPickLoaderVersion picks want (or the newest stable version) among the
// LeviLamina versions supporting gameVersion.
func giPickLoaderVersion(c *Ctx, db catalog.LoaderDB, gameVersion, want string) (string, error) {
	list := db.Versions(gameVersion)
	if len(list) == 0 {
		e := Backend("ERR_LL_NOT_SUPPORTED")
		e.Message = fmt.Sprintf("LeviLamina does not support Minecraft %s", gameVersion)
		if games := db.GameVersions(); len(games) > 0 {
			if len(games) > 5 {
				games = games[:5]
			}
			e.Detail = "supported game versions include " + strings.Join(games, ", ")
			e.Message += " (supported: " + strings.Join(games, ", ") + ", ...)"
		}
		return "", e
	}
	if want != "" {
		v, ok := catalog.FindLoaderVersion(list, want)
		if !ok {
			e := Backend("ERR_LL_VERSION_UNSUPPORTED")
			e.Message = fmt.Sprintf("LeviLamina %s does not support Minecraft %s", want, gameVersion)
			return "", e.WithHint("%s loader versions --game %s --prerelease", ProgramName, gameVersion)
		}
		return v, nil
	}
	if v := catalog.LoaderLatestStable(list); v != "" {
		return v, nil
	}
	return "", Errorf(ExitUsage, "ERR_LL_ONLY_PRERELEASE", "Minecraft %s only has pre-release LeviLamina versions: %s", gameVersion, strings.Join(list, ", ")).
		WithHint("name one explicitly with --loader-version %s", list[0])
}

// giConfirmPrerelease asks before installing a LeviLamina pre-release.
func giConfirmPrerelease(c *Ctx, v string) error {
	return c.Confirm(fmt.Sprintf("LeviLamina %s is a pre-release (release candidate): mods may not work with it and it may be unstable. Install it?", v))
}

// fetchInstaller finds or downloads the catalogue build's installer, after
// checking the account and licence when a GDK download is needed.
func (g *giCreate) fetchInstaller() error {
	if g.build.Type == catalog.TypeGDK {
		_, files, err := giScanInstallers()
		if err != nil {
			return err
		}
		if _, ok := catalog.FindInstaller(files, g.build); !ok {
			if err := giCheckLicence(g.c, g.build.Channel); err != nil {
				return err
			}
		}
	}
	res, err := giEnsureInstaller(g.c, g.build, g.c.String("mirror"), false)
	if err != nil {
		return err
	}
	g.path, g.downloaded = res.Path, res.Downloaded
	if !res.Downloaded {
		g.c.Verbosef("Using the downloaded installer %s", res.Path)
	}
	return nil
}

// giCheckLicence fails early, before a multi-gigabyte download, when no
// Microsoft account is signed in or it does not own the game. A licence
// check that cannot complete is not fatal: the installation checks again.
func giCheckLicence(c *Ctx, channel string) error {
	_ = gi.restoreAccount(c.Context())
	if !gi.accountSaved() {
		e := Backend("ERR_AUTH_INTERACTION_REQUIRED")
		e.Message = "installing a GDK build needs a signed-in Microsoft account that owns Minecraft"
		return e
	}
	p := c.Progress()
	p.Phase("Checking the game licence")
	state := gi.licenceState(c.Context(), channel)
	p.Done()
	if err := c.Context().Err(); err != nil {
		return FromError(err)
	}
	switch state {
	case "authorized":
		return nil
	case "not_entitled":
		return Backend("ERR_LICENSE_NOT_ENTITLED")
	case "trial":
		return Backend("ERR_LICENSE_FULL_REQUIRED")
	}
	c.Verbosef("The game licence could not be checked (%s); continuing", Dash(state))
	return nil
}

// extract creates the instance folder from the installer.
func (g *giCreate) extract() error {
	p := g.c.Progress()
	p.Phase("Extracting %s into instance %s", filepath.Base(g.path), g.name)
	var code string
	if g.typ == catalog.TypeUWP {
		code = gi.extractAppx(g.c.Context(), g.path, g.name, g.channel)
	} else {
		code = gi.extractMsixvc(g.c.Context(), g.path, g.name)
	}
	p.Done()
	if err := giCtxErr(g.c, code); err != nil {
		if e := FromError(err); !g.local && (e.Code == "ERR_NATIVE_MSIXVC" || e.Code == "ERR_MSIXVC_UNSUPPORTED_FORMAT" || e.Code == "ERR_UWP_ARCHIVE") {
			// A damaged download: fetch it again.
			opts := ""
			if g.build.Type == catalog.TypeUWP || g.build.Channel != catalog.ChannelRelease {
				opts = fmt.Sprintf(" --type %s --channel %s", g.build.Type, g.build.Channel)
			}
			e.Hint = fmt.Sprintf("%s installer download %s%s --force", ProgramName, g.build.Version, opts)
			return e
		}
		return err
	}
	g.created = true
	return g.c.Context().Err()
}

// finish runs the steps after the extraction; any error rolls back.
func (g *giCreate) finish() error {
	c := g.c
	ctx := c.Context()
	p := c.Progress()
	p.Phase("Saving the instance settings")
	metaVersion := g.gameVersion
	if g.typ == catalog.TypeUWP {
		m := gi.instanceMeta(g.name)
		if v := strings.TrimSpace(m.GameVersion); v != "" {
			metaVersion = v
		}
		if t := strings.TrimSpace(m.Type); t != "" {
			g.channel = catalog.NormalizeChannel(t)
		}
	} else {
		if id, ok := catalog.ReadGDKIdentity(gi.instanceDir(g.name)); ok {
			if g.channelSet && id.Channel != g.channel {
				c.Warn("the installer is a %s build; saving the instance as %s, not %s", id.Channel, id.Channel, g.channel)
			}
			g.channel = id.Channel
			if g.local || metaVersion == "" {
				metaVersion = id.GameVersion
			}
		}
		if g.channel == "" {
			c.Warn("the channel of %s is unknown; the instance is saved as release (pass --channel to change it)", filepath.Base(g.path))
			g.channel = catalog.ChannelRelease
		}
	}
	if metaVersion == "" {
		// The backend reads the version from the game executable.
		metaVersion = g.name
	}
	isolation := g.isolation && versions.SupportsIsolation(g.typ, metaVersion)
	if g.isolation && !isolation {
		c.Notice("Minecraft %s cannot keep its own game data; it uses the shared data.", metaVersion)
	}
	g.isolation = isolation
	if err := giCtxErr(c, gi.saveMeta(g.name, metaVersion, g.channel, isolation)); err != nil {
		return err
	}
	if v := strings.TrimSpace(gi.instanceMeta(g.name).GameVersion); v != "" {
		g.gameVersion = v
	} else if g.gameVersion == "" && metaVersion != g.name {
		g.gameVersion = metaVersion
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if g.copyFrom != "" && g.typ == catalog.TypeGDK {
		if !isolation {
			return UsageError("cannot copy game data: Minecraft %s uses the shared game data", g.gameVersion)
		}
		if !g.copyChecked {
			if err := g.checkCopySource(); err != nil {
				return err
			}
		}
		from := g.copyFrom
		if from == "system" {
			from = "the installed Minecraft"
		}
		p.Phase("Copying game data from %s", from)
		var code string
		if g.copyFrom == "system" {
			code = gi.copyFromSystem(g.channel == catalog.ChannelPreview, g.name)
		} else {
			code = gi.copyFromInstance(g.copyFrom, g.name)
		}
		p.Done()
		if err := giCtxErr(c, code); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if g.loader && g.typ == catalog.TypeGDK {
		if g.loaderPick == "" {
			if g.gameVersion == "" {
				return Errorf(ExitFailure, "ERR_INVALID_VERSION_FORMAT", "the game version of the new instance is unknown, so LeviLamina cannot be chosen")
			}
			if err := g.pickLoader(); err != nil {
				return err
			}
		}
		p.Phase("Installing LeviLamina %s", g.loaderPick)
		code := gi.installLoader(ctx, g.gameVersion, g.name, g.loaderPick)
		p.Done()
		if err := giCtxErr(c, code); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.Bool("use") {
		if err := gi.setDefault(g.name); err != nil {
			c.Warn("%s", FromError(err).Message)
		}
	}
	return nil
}

// rollback removes the half-created instance and returns cause.
func (g *giCreate) rollback(cause error) error {
	g.c.Progress().Done()
	if !g.created {
		return cause
	}
	if code := gi.deleteInstance(g.name); code != "" {
		g.c.Warn("the half-created instance %s could not be removed (%s); remove it with: %s instance remove %s",
			g.name, code, ProgramName, quoteArg(g.name))
	} else {
		g.c.Notice("Removed the half-created instance %s.", g.name)
	}
	return cause
}

// report prints the created instance.
func (g *giCreate) report() error {
	c := g.c
	defaultName, _, _ := c.DefaultInstance()
	var item instanceListItem
	if inst, err := LookupInstance(g.name); err == nil {
		item = describeInstance(inst, defaultName, "", "", nil)
	} else {
		dir := gi.instanceDir(g.name)
		item = instanceListItem{Name: g.name, GameVersion: g.gameVersion, Channel: g.channel, Type: g.typ,
			Isolated: g.isolation, PIDs: []int{}, Path: dir, IsDefault: strings.EqualFold(defaultName, g.name)}
		if g.loaderPick != "" {
			v := g.loaderPick
			item.LoaderVersion = &v
		}
	}
	out := instanceCreateResult{instanceListItem: item, Installer: g.path, InstallerDownloaded: g.downloaded}
	data := "own data"
	if !item.Isolated {
		data = "shared data"
	}
	extra := ""
	if item.LoaderVersion != nil {
		extra = ", LeviLamina " + *item.LoaderVersion
	}
	c.Notice("Play it: %s play %s", ProgramName, quoteArg(g.name))
	return c.Render(out, []string{g.name}, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "Created instance %q (Minecraft %s, %s, %s, %s%s)\n",
			g.name, Dash(item.GameVersion), giTypeLabel(item.Type), Dash(item.Channel), data, extra)
		return err
	})
}
