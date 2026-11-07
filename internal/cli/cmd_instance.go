package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/instancereg"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	winreg "github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// This file holds the instance commands other than list and create: show,
// set, rename, remove, register, unregister, copy-data, shortcut and path,
// plus the top-level "use".

// Seams replaced by tests: everything that changes the system or reads
// state outside the instance folder.
var (
	instSaveMeta       = mcservice.SaveVersionMeta
	instSaveLogo       = mcservice.SaveVersionLogoFromPath
	instRemoveLogo     = mcservice.RemoveVersionLogo
	instValidateName   = mcservice.ValidateVersionFolderName
	instRenameFolder   = mcservice.RenameVersionFolder
	instDeleteFolder   = mcservice.DeleteVersionFolder
	instCreateShortcut = mcservice.CreateDesktopShortcut
	instShortcutPath   = mcservice.DesktopShortcutPath
	instSetDefault     = config.SetCurrentInstance
	instRegister       = instancereg.Register
	instUnregister     = instancereg.Unregister
	instDevModeEnabled = winreg.IsDevModeEnabled
	instSystemDataPath = utils.GetMinecraftGDKDataPath
	instContentRoots   = func(inst Instance) types.ContentRoots { return mcservice.GetContentRoots(inst.Name) }
	instOpenFolder     = instExplorer
)

func init() {
	RegisterBackendErrorsFor([]string{"instance", "launch", "use"}, map[string]BackendError{
		"ERR_NAME_REQUIRED":            {Exit: ExitUsage, Message: "the name must not be empty"},
		"ERR_NAME_TOO_LONG":            {Exit: ExitUsage, Message: "the name is longer than 64 characters"},
		"ERR_NAME_TRAILING_DOT_SPACE":  {Exit: ExitUsage, Message: "the name must not end with a dot or a space"},
		"ERR_NAME_INVALID_CHAR":        {Exit: ExitUsage, Message: `the name must not contain any of < > : " / \ | ? *`},
		"ERR_NAME_CONTROL_CHAR":        {Exit: ExitUsage, Message: "the name must not contain control characters"},
		"ERR_NAME_RESERVED":            {Exit: ExitUsage, Message: "the name is reserved by Windows (CON, PRN, AUX, NUL, COM1-9, LPT1-9)"},
		"ERR_NAME_EXISTS":              {Exit: ExitConflict, Message: "an instance with that name already exists", Hint: "LeviLauncher.exe instance list"},
		"ERR_READ_VERSIONS_DIR":        {Exit: ExitFailure, Message: "cannot read the instances folder"},
		"ERR_NOT_FOUND_OLD":            {Exit: ExitNotFound, Message: "the instance folder does not exist", Hint: "LeviLauncher.exe instance list"},
		"ERR_VERSION_NOT_FOUND":        {Exit: ExitNotFound, Message: "the instance folder does not exist", Hint: "LeviLauncher.exe instance list"},
		"ERR_RENAME_FAILED":            {Exit: ExitFailure, Message: "renaming the instance folder failed", Hint: "close programs that use files in the instance folder and try again"},
		"ERR_DELETE_FAILED":            {Exit: ExitFailure, Message: "deleting the instance folder failed", Hint: "close programs that use files in the instance folder and try again"},
		"ERR_WRITE_TARGET":             {Exit: ExitFailure, Message: "cannot write to the instance folder"},
		"ERR_PREPARE_EXE":              {Exit: ExitFailure, Message: "cannot prepare the game executable of the instance"},
		"ERR_ICON_DECODE":              {Exit: ExitUsage, Message: "the logo is not a readable PNG, JPEG or GIF image"},
		"ERR_ICON_FORMAT":              {Exit: ExitUsage, Message: "the logo is not a supported image"},
		"ERR_ICON_NOT_SQUARE":          {Exit: ExitUsage, Message: "the logo must be a square image"},
		"ERR_SHORTCUT_CREATE_FAILED":   {Exit: ExitFailure, Message: "creating the desktop shortcut failed"},
		"ERR_TARGET_DIR_NOT_SPECIFIED": {Exit: ExitUsage, Message: "no instance was given"},
		"ERR_DEV_MODE_REQUIRED": {Exit: ExitSystem, Message: "registering a GDK instance needs Windows Developer Mode",
			Hint: "turn on Developer Mode in Windows Settings > System > For developers, then run the command again"},
		"ERR_UWP_DEVELOPER_MODE": {Exit: ExitSystem, Message: "registering a UWP instance needs Windows Developer Mode",
			Hint: "turn on Developer Mode in Windows Settings > System > For developers, then run the command again"},
		"ERR_REGISTER_FAILED":             {Exit: ExitFailure, Message: "Windows refused to register the instance"},
		"ERR_UNREGISTER_FAILED":           {Exit: ExitFailure, Message: "Windows refused to release the registration"},
		"ERR_NOT_REGISTERED_THIS_VERSION": {Exit: ExitFailure, Message: "the instance is not registered with Windows"},
		"ERR_UWP_NOT_REGISTERED":          {Exit: ExitFailure, Message: "the instance is not registered with Windows"},
		"ERR_UWP_REGISTER":                {Exit: ExitFailure, Message: "Windows refused to register the instance"},
		"ERR_UWP_UNREGISTER":              {Exit: ExitFailure, Message: "Windows refused to release the registration"},
		"ERR_UWP_REGISTER_ROLLBACK":       {Exit: ExitFailure, Message: "registering failed and the previous registration could not be restored"},
		"ERR_UWP_PACKAGE_CONFLICT":        {Exit: ExitConflict, Message: "another Minecraft installation that the launcher does not manage blocks the registration"},
		"ERR_UWP_DEPENDENCY":              {Exit: ExitFailure, Message: "a package this Minecraft build depends on is missing from Windows"},
		"ERR_UWP_SYSTEM_VERSION":          {Exit: ExitFailure, Message: "this Windows version is too old for this Minecraft build"},
		"ERR_UWP_QUERY":                   {Exit: ExitFailure, Message: "cannot read the registered Minecraft packages from Windows"},
		"ERR_UWP_MANIFEST":                {Exit: ExitFailure, Message: "the instance's package manifest is missing or damaged", Hint: "reinstall the instance"},
		"ERR_INHERIT_TYPE_MISMATCH":       {Exit: ExitUsage, Message: "game data can only be copied between GDK instances of the same channel"},
		"ERR_INHERIT_SOURCE_NOT_FOUND":    {Exit: ExitNotFound, Message: "the source has no game data to copy"},
		"ERR_INHERIT_TARGET_NOT_FOUND":    {Exit: ExitNotFound, Message: "the target instance does not exist"},
		"ERR_NO_GAME_DATA":                {Exit: ExitNotFound, Message: "the system Minecraft has no game data to copy"},
		"ERR_INHERIT_COPY_FAILED":         {Exit: ExitFailure, Message: "copying the game data failed"},
		"ERR_CREATE_TARGET_DIR":           {Exit: ExitFailure, Message: "cannot create the instance's data folder"},
	})

	Register(&Command{
		Path:    "instance show",
		Summary: "show settings, registration, loader, players and content counts of an instance",
		Args:    "[<instance>]",
		Description: `Shows everything about one instance. 'instance <name>' is the same as
'instance show <name>'. Content is counted per player profile on GDK
instances; -p limits the table to one profile.`,
		Examples: []string{"instance show", "instance Survival", "instance show 1.21.50 --json"},
		MaxArgs:  1,
		Run:      runInstanceShow,
	})
	Register(&Command{
		Path:    "instance set",
		Summary: "change instance settings (console, editor mode, own data, arguments, environment, logo)",
		Args:    "[<instance>]",
		Description: `Only the settings you pass change. Launch arguments and environment
variables are GDK only. Editor mode needs game 1.21.50 or newer (Preview
1.19.80.20); own data on UWP needs 1.19.70.2 or newer. The game must not be
running.`,
		Examples: []string{
			"instance set Survival --console",
			"instance set 1.21.50 --env MC_LOG=1 --args \"-dev\"",
			"instance set Creative --logo D:\\logo.png --no-editor",
		},
		Flags: []*Flag{
			BoolFlag("console", "", "open the game's debug console (--no-console to turn it off)"),
			BoolFlag("editor", "", "start the game in editor mode (--no-editor to turn it off)"),
			BoolFlag("own-data", "", "keep worlds and settings inside the instance"),
			BoolFlag("shared-data", "", "use the system Minecraft data (worlds and settings)"),
			StringFlag("args", "", "<string>", "extra launch arguments, quote-aware; '' clears them (GDK only)"),
			StringsFlag("env", "", "<KEY=VALUE>", "set a game environment variable (GDK only)"),
			StringsFlag("unset-env", "", "<KEY>", "remove an environment variable"),
			BoolFlag("clear-env", "", "remove all environment variables"),
			StringFlag("logo", "", "<image>", "set the logo from a square PNG, JPEG or GIF image"),
			BoolFlag("clear-logo", "", "remove the logo"),
		},
		MaxArgs: 1,
		Run:     runInstanceSet,
	})
	Register(&Command{
		Path:    "instance rename",
		Summary: "rename an instance",
		Args:    "<instance> <new-name>",
		Description: `The default instance and the desktop shortcut follow the new name. A
registered instance must be unregistered first, and the game must not be
running. Changing only the letter case is allowed.`,
		Examples: []string{"instance rename 1.21.50 Survival", "instance rename survival Survival"},
		MinArgs:  2, MaxArgs: 2,
		Run: runInstanceRename,
	})
	Register(&Command{
		Path:    "instance remove",
		Summary: "permanently delete instances and everything in their folders",
		Args:    "<instance>...",
		Description: `Deletes the instance folders, including worlds kept inside instances with
their own data. Registered or running instances are refused. An instance
that links to a folder elsewhere loses only the link. The default instance
setting and desktop shortcut of a removed instance are removed too.`,
		Examples: []string{"instance remove 1.20.80", "instance rm Old Test --yes"},
		MinArgs:  1, MaxArgs: Unlimited,
		Run: runInstanceRemove,
	})
	Register(&Command{
		Path:    "instance register",
		Summary: "make Windows treat this instance as the installed Minecraft of its channel",
		Args:    "[<instance>]",
		Description: `Start menu, minecraft:// links and Xbox features then use this instance.
It replaces the current holder of the release or preview slot (a Store
install or another instance), whose data is kept; that needs confirmation
or --yes. Game data is never touched. 'launch' registers UWP instances
automatically.`,
		Examples: []string{"instance register Survival", "instance register 1.21.50 --yes"},
		MaxArgs:  1,
		Run:      runInstanceRegister,
	})
	Register(&Command{
		Path:     "instance unregister",
		Summary:  "release the instance's Windows registration (game data is kept)",
		Args:     "[<instance>]",
		Examples: []string{"instance unregister Survival", "instance unregister --json"},
		MaxArgs:  1,
		Run:      runInstanceUnregister,
	})
	Register(&Command{
		Path:    "instance copy-data",
		Summary: "copy game data (worlds, settings) into an instance with its own data",
		Args:    "[<instance>] --from system|<instance>",
		Description: `Merges the game data of the system Minecraft of the same channel, or of
another GDK instance with its own data, into a GDK instance with its own
data. Files with the same name are overwritten; nothing is deleted.`,
		Examples: []string{"instance copy-data Survival --from system", "instance copy-data 1.21.50 --from 1.21.44 --yes"},
		Flags:    []*Flag{StringFlag("from", "", "<system|instance>", "where to copy from: 'system' or another instance")},
		MaxArgs:  1,
		Run:      runInstanceCopyData,
	})
	Register(&Command{
		Path:    "instance shortcut",
		Summary: "create a desktop shortcut that starts the instance, or remove it",
		Args:    "[<instance>]",
		Description: `Creates (or replaces) "Minecraft - <name>.lnk" on the desktop and prints
its path. The shortcut opens the launcher and starts the game.`,
		Examples: []string{"instance shortcut Survival", "instance shortcut Survival --remove"},
		Flags:    []*Flag{BoolFlag("remove", "", "delete the shortcut instead")},
		MaxArgs:  1,
		Run:      runInstanceShortcut,
	})
	Register(&Command{
		Path:    "instance path",
		Summary: "print a folder of an instance (and optionally open it)",
		Args:    "[<instance>] [<kind>]",
		Description: `<kind> is one of: ` + strings.Join(instPathKinds, ", ") + ` (default root).
data is the game-data folder (own or shared). worlds, screenshots, servers
and (before game 1.26) skin-packs belong to a player profile on GDK
instances; pick one with -p. When the instance is omitted, a kind word is
still recognised: 'instance path worlds'.`,
		Examples: []string{"instance path", "instance path Survival worlds -p Steve", "instance path Survival mods --open"},
		Flags:    []*Flag{BoolFlag("open", "", "also open the folder in Explorer")},
		MaxArgs:  2,
		Run:      runInstancePath,
	})
	Register(&Command{
		Path:    "use",
		Summary: "set or show the default instance",
		Args:    "[<instance>]",
		Description: `Commands use the default instance when none is named (after -i and
$LEVILAUNCHER_INSTANCE). With no argument, shows the current default and
where it comes from.`,
		Examples: []string{"use Survival", "use", "use --clear"},
		Flags:    []*Flag{BoolFlag("clear", "", "remove the default instance setting")},
		MaxArgs:  1,
		Run:      runUse,
	})
}

// ---------------------------------------------------------------------------
// Shared helpers.

// instRunningPIDs returns the PIDs of the Minecraft processes started from
// the instance: by the launcher for it, or from an executable inside its
// folder. Every instance command (and loader, instance create) uses it.
func instRunningPIDs(inst Instance) []int {
	var out []int
	dir := ""
	if inst.Dir != "" {
		dir = strings.ToLower(filepath.Clean(inst.Dir)) + string(filepath.Separator)
	}
	for _, p := range minecraftProcesses() {
		if (p.IsLauncher && p.VersionName != "" && strings.EqualFold(p.VersionName, inst.Name)) ||
			(dir != "" && p.ExePath != "" && strings.HasPrefix(strings.ToLower(filepath.Clean(p.ExePath)), dir)) {
			out = append(out, p.Pid)
		}
	}
	sort.Ints(out)
	return out
}

// instRunningError is the "game running" conflict for instance commands.
func instRunningError(inst Instance, pids []int) *Error {
	return Errorf(ExitConflict, "ERR_GAME_ALREADY_RUNNING", "Minecraft is running from instance %s (pid %s)", inst.Name, joinInts(pids)).
		WithHint("%s stop %s", ProgramName, quoteArg(inst.Name))
}

// instSlot is the registration slot of an instance: preview has its own,
// release and beta share the retail one.
func instSlot(inst Instance) string {
	if inst.IsPreview() {
		return "preview"
	}
	return "release"
}

// instRegState describes the Windows registration of an instance's slot.
type instRegState struct {
	Registered bool   // the slot holds this instance
	Holder     string // canonical path of the slot's holder, "" when free
	HolderName string // the holder's instance name, "" when not an instance
}

func instRegistration(inst Instance, all []Instance) instRegState {
	release, preview := registeredLocations()
	loc := release
	if inst.IsPreview() {
		loc = preview
	}
	if loc == "" {
		return instRegState{}
	}
	if strings.EqualFold(utils.CanonicalWindowsPath(inst.Dir), loc) {
		return instRegState{Registered: true, Holder: loc, HolderName: inst.Name}
	}
	st := instRegState{Holder: loc}
	for _, o := range all {
		if strings.EqualFold(utils.CanonicalWindowsPath(o.Dir), loc) {
			st.HolderName = o.Name
			break
		}
	}
	return st
}

// displacedLabel is the JSON value naming a displaced holder: its instance
// name, or "store" for an installation the launcher does not manage.
func (s instRegState) displacedLabel() string {
	if s.HolderName != "" {
		return s.HolderName
	}
	return "store"
}

// instHolderPIDs returns the game processes running from the slot's
// holder.
func instHolderPIDs(st instRegState, all []Instance) []int {
	if st.HolderName != "" {
		for _, o := range all {
			if o.Name == st.HolderName {
				return instRunningPIDs(o)
			}
		}
	}
	prefix := strings.ToLower(st.Holder) + string(filepath.Separator)
	var out []int
	for _, p := range minecraftProcesses() {
		if p.ExePath != "" && strings.HasPrefix(strings.ToLower(utils.CanonicalWindowsPath(p.ExePath)), prefix) {
			out = append(out, p.Pid)
		}
	}
	sort.Ints(out)
	return out
}

func (s instRegState) describeHolder() string {
	if s.HolderName != "" {
		return "instance " + s.HolderName
	}
	if strings.Contains(strings.ToLower(s.Holder), `\windowsapps\`) {
		return "the Microsoft Store Minecraft"
	}
	return "the Minecraft installed at " + s.Holder
}

// instRegisterResult is the outcome of registering, also the JSON document
// of instance register/unregister.
type instRegisterResult struct {
	Instance   string  `json:"instance"`
	Channel    string  `json:"channel"`
	Registered bool    `json:"registered"`
	Displaced  *string `json:"displaced"`
	Changed    bool    `json:"changed"`
}

// instRegisterFlow registers inst unless it already holds its slot. It
// asks before displacing another holder, checks Developer Mode for GDK and
// verifies the result with Windows. Shared by instance register and launch.
func instRegisterFlow(c *Ctx, inst Instance) (instRegisterResult, error) {
	all, err := ListInstances()
	if err != nil {
		return instRegisterResult{}, err
	}
	res := instRegisterResult{Instance: inst.Name, Channel: instSlot(inst)}
	st := instRegistration(inst, all)
	if st.Registered {
		res.Registered = true
		return res, nil
	}
	if pids := instRunningPIDs(inst); len(pids) > 0 {
		return res, instRunningError(inst, pids)
	}
	if st.Holder != "" {
		// Windows removes the holder's package to register this one, which
		// closes its running game.
		if pids := instHolderPIDs(st, all); len(pids) > 0 {
			e := Errorf(ExitConflict, "ERR_GAME_ALREADY_RUNNING", "%s is running (pid %s); registering %s would close it",
				st.describeHolder(), joinInts(pids), inst.Name)
			if st.HolderName != "" {
				return res, e.WithHint("%s stop %s", ProgramName, quoteArg(st.HolderName))
			}
			return res, e.WithHint("%s process stop %s", ProgramName, strings.Trim(fmt.Sprint(pids), "[]"))
		}
		d := st.displacedLabel()
		res.Displaced = &d
		q := fmt.Sprintf("Register %s as Minecraft (%s), replacing %s (its data is kept)?",
			inst.Name, res.Channel, st.describeHolder())
		if err := c.Confirm(q); err != nil {
			return res, err
		}
	}
	if !inst.IsUWP() && !instDevModeEnabled() {
		if !c.CanPrompt() {
			return res, Backend("ERR_DEV_MODE_REQUIRED")
		}
		c.Notice("Windows Developer Mode is off; Windows will ask to turn it on")
	}
	p := c.Progress()
	p.Phase("Registering %s as Minecraft (%s)", inst.Name, res.Channel)
	code := instRegister(c.Context(), inst.Name, inst.IsPreview())
	p.Done()
	if err := c.Context().Err(); err != nil {
		return res, err
	}
	if e := Backend(code); e != nil {
		return res, e
	}
	if !instRegistration(inst, all).Registered {
		code := "ERR_REGISTER_FAILED"
		if inst.IsUWP() {
			code = "ERR_UWP_NOT_REGISTERED"
		}
		return res, Backend(code).WithDetail("Windows does not list the instance folder as the registered package after registering")
	}
	res.Registered, res.Changed = true, true
	return res, nil
}

// instIsLinked reports whether dir is a junction or symbolic link (an
// instance that points at a game folder elsewhere).
func instIsLinked(dir string) bool {
	fi, err := os.Lstat(dir)
	if err != nil {
		return false
	}
	if fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return true
	}
	if d, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		return d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	return false
}

// instExplorer opens dir in a new Explorer window.
func instExplorer(dir string) error {
	cmd := exec.Command("explorer.exe", dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // explorer.exe exits 1 even on success
	return nil
}

// instCountDirs counts the folders directly inside dir (following
// junctions); 0 when dir is missing.
func instCountDirs(dir string) int {
	if strings.TrimSpace(dir) == "" {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || utils.ResolvesToDir(filepath.Join(dir, e.Name())) {
			n++
		}
	}
	return n
}

// instCountScreenshots counts the images in a Screenshots folder and its direct
// subfolders, as the screenshot list does.
func instCountScreenshots(dir string) int {
	if strings.TrimSpace(dir) == "" {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	dirs := []string{dir}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(dir, e.Name()))
		}
	}
	n := 0
	for _, d := range dirs {
		files, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			switch strings.ToLower(filepath.Ext(f.Name())) {
			case ".png", ".jpg", ".jpeg":
				n++
			}
		}
	}
	return n
}

// instCountServers counts the entries of a minecraftpe/external_servers.txt
// ("index:name:ip:port:timestamp" lines).
func instCountServers(minecraftpeDir string) int {
	if strings.TrimSpace(minecraftpeDir) == "" {
		return 0
	}
	f, err := os.Open(filepath.Join(minecraftpeDir, "external_servers.txt"))
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if len(strings.Split(sc.Text(), ":")) >= 5 {
			n++
		}
	}
	return n
}

// instCountMods counts the mod folders of an instance without creating mods/.
func instCountMods(inst Instance) int {
	entries, err := os.ReadDir(filepath.Join(inst.Dir, "mods"))
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() && !fsx.IsTempName(e.Name()) {
			n++
		}
	}
	return n
}

// instSharedSkinMin: GDK games newer than this keep skin packs next to
// the shared resource packs instead of per player (contentmgr.versionSkinDir).
const instSharedSkinMin = "1.26.0.0"

// instSharedSkins reports whether skin packs are shared by every player.
func instSharedSkins(inst Instance) bool {
	return inst.IsUWP() || versions.CompareGameVersions(inst.Meta.GameVersion, instSharedSkinMin) > 0
}

// The kinds of folder 'instance path' prints.
var instPathKinds = []string{"root", "data", "mods", "worlds", "resource-packs", "behavior-packs", "skin-packs", "screenshots", "servers"}

// instKindIsPerPlayer reports whether a kind of content belongs to one
// player profile of the instance.
func instKindIsPerPlayer(inst Instance, kind string) bool {
	if inst.IsUWP() {
		return false
	}
	switch kind {
	case "worlds", "screenshots", "servers":
		return true
	case "skin-packs":
		return !instSharedSkins(inst)
	}
	return false
}

// instKindPath is a port of the window's resolveContentPath plus the
// launcher's other instance folders. player is the profile folder ID for
// per-player kinds of GDK instances. "" means the folder cannot be located
// (for example a GDK instance without a data folder yet).
func instKindPath(inst Instance, roots types.ContentRoots, kind, player string) string {
	comMojang := func(sub string) string {
		if roots.PackageType == versions.PackageTypeUWP {
			if roots.ComMojangRoot == "" {
				return ""
			}
			return filepath.Join(roots.ComMojangRoot, sub)
		}
		player = strings.TrimSpace(player)
		if roots.UsersRoot == "" || player == "" || player == "." || player == ".." || strings.ContainsAny(player, `\/:`) {
			return ""
		}
		return filepath.Join(roots.UsersRoot, player, "games", "com.mojang", sub)
	}
	pick := func(direct, sub string) string {
		if roots.PackageType == versions.PackageTypeUWP && direct != "" {
			return direct
		}
		return comMojang(sub)
	}
	switch kind {
	case "root":
		return inst.Dir
	case "data":
		return roots.Base
	case "mods":
		return filepath.Join(inst.Dir, "mods")
	case "resource-packs":
		return roots.ResourcePacks
	case "behavior-packs":
		return roots.BehaviorPacks
	case "worlds":
		return pick(roots.Worlds, "minecraftWorlds")
	case "screenshots":
		return pick(roots.Screenshots, "Screenshots")
	case "servers":
		return comMojang("minecraftpe")
	case "skin-packs":
		if roots.PackageType == versions.PackageTypeUWP {
			return pick(roots.SkinPacks, "skin_packs")
		}
		if instSharedSkins(inst) {
			if roots.ResourcePacks == "" {
				return ""
			}
			return filepath.Join(filepath.Dir(roots.ResourcePacks), "skin_packs")
		}
		return comMojang("skin_packs")
	}
	return ""
}

// instEnvLines splits stored environment variables (one KEY=VALUE per line).
func instEnvLines(s string) []string {
	out := []string{}
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func instEnvKey(line string) string {
	k, _, _ := strings.Cut(line, "=")
	return strings.TrimSpace(k)
}

// instApplyEnv applies --clear-env, then --unset-env, then --env to the
// stored lines. Keys compare case-insensitively, as Windows does; a set
// replaces an existing key in place.
func instApplyEnv(lines []string, clear bool, unset, set []string) ([]string, error) {
	out := append([]string{}, lines...)
	if clear {
		out = []string{}
	}
	for _, k := range unset {
		k = strings.TrimSpace(k)
		if k == "" || strings.Contains(k, "=") {
			return nil, UsageError("--unset-env takes a variable name, got %q", k)
		}
		kept := out[:0]
		for _, l := range out {
			if !strings.EqualFold(instEnvKey(l), k) {
				kept = append(kept, l)
			}
		}
		out = kept
	}
	for _, kv := range set {
		k, v, ok := strings.Cut(kv, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" || strings.ContainsAny(kv, "\r\n") {
			return nil, UsageError("--env takes KEY=VALUE, got %q", kv)
		}
		line := k + "=" + v
		replaced := false
		for i, l := range out {
			if strings.EqualFold(instEnvKey(l), k) {
				out[i], replaced = line, true
				break
			}
		}
		if !replaced {
			out = append(out, line)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// instance show

type instContentCounts struct {
	Worlds      int  `json:"worlds"`
	Screenshots int  `json:"screenshots"`
	Servers     int  `json:"servers"`
	SkinPacks   *int `json:"skinPacks"` // null when skin packs are shared
}

type instShowPlayer struct {
	ID       string  `json:"id"`
	Gamertag *string `json:"gamertag"`
	instContentCounts
}

type instShowItem struct {
	Name               string             `json:"name"`
	Path               string             `json:"path"`
	DataPath           *string            `json:"dataPath"`
	GameVersion        string             `json:"gameVersion"`
	Channel            string             `json:"channel"`
	Type               string             `json:"type"`
	Isolated           bool               `json:"isolated"`
	IsolationSupported bool               `json:"isolationSupported"`
	Linked             bool               `json:"linked"`
	Registered         bool               `json:"registered"`
	Slot               string             `json:"slot"`
	Running            bool               `json:"running"`
	PIDs               []int              `json:"pids"`
	Console            bool               `json:"console"`
	EditorMode         bool               `json:"editorMode"`
	EditorSupported    bool               `json:"editorSupported"`
	EditorMinVersion   string             `json:"editorMinVersion"`
	LaunchArgs         string             `json:"launchArgs"`
	EnvVars            []string           `json:"envVars"`
	HasLogo            bool               `json:"hasLogo"`
	LogoPath           *string            `json:"logoPath"`
	LoaderVersion      *string            `json:"loaderVersion"`
	ModCount           int                `json:"modCount"`
	CreatedAt          *time.Time         `json:"createdAt"`
	IsDefault          bool               `json:"isDefault"`
	Shortcut           *string            `json:"shortcut"`
	ResourcePacks      int                `json:"resourcePacks"`
	BehaviorPacks      int                `json:"behaviorPacks"`
	SharedSkinPacks    *int               `json:"sharedSkinPacks"`
	Content            *instContentCounts `json:"content"` // UWP: the instance's only data
	Players            []instShowPlayer   `json:"players"`
}

func runInstanceShow(c *Ctx) error {
	inst, err := c.ResolveInstance(c.Arg(0))
	if err != nil {
		// "instance lst" reaches show through the noun's default verb: a
		// word close to a verb is a mistyped command, not an instance.
		if e := FromError(err); e.Exit == ExitNotFound && c.Arg(0) != "" && c.Cmd.noun != nil {
			var verbs []string
			for _, v := range c.Cmd.noun.visibleVerbs() {
				verbs = append(verbs, v.name)
			}
			if s := suggest(c.Arg(0), verbs); s != "" {
				return UsageError("unknown command %q", c.Cmd.noun.Name+" "+c.Arg(0)).
					WithHint("did you mean %q?", c.Cmd.noun.Name+" "+s)
			}
		}
		return err
	}
	all, err := ListInstances()
	if err != nil {
		return err
	}
	var players []Player
	if !inst.IsUWP() {
		if strings.TrimSpace(c.G.Player) != "" || strings.TrimSpace(c.Getenv(EnvPlayer)) != "" {
			p, err := c.ResolvePlayer(inst)
			if err != nil {
				return err
			}
			players = []Player{p}
		} else if players, err = ListPlayers(inst); err != nil {
			return Errorf(ExitFailure, "ERR_ACCESS_PLAYERS", "cannot read the player profiles of %s: %v", inst.Name, err)
		}
	} else if strings.TrimSpace(c.G.Player) != "" {
		c.Notice("--player is ignored: UWP instances have no player profiles")
	}
	item := instBuildShow(c, inst, all, players)
	return c.Render(item, []string{item.Name}, func(w io.Writer) error { return instWriteShow(w, item) })
}

func instBuildShow(c *Ctx, inst Instance, all []Instance, players []Player) instShowItem {
	defaultName, _, _ := c.DefaultInstance()
	pids := instRunningPIDs(inst)
	reg := instRegistration(inst, all)
	m := inst.Meta
	it := instShowItem{
		Name: inst.Name, Path: inst.Dir, GameVersion: m.GameVersion, Channel: inst.Channel, Type: inst.PackageType,
		Isolated: m.EnableIsolation, IsolationSupported: versions.SupportsIsolation(inst.PackageType, m.GameVersion),
		Linked: instIsLinked(inst.Dir), Registered: reg.Registered, Slot: instSlot(inst),
		Running: len(pids) > 0, PIDs: append([]int{}, pids...),
		Console: m.EnableConsole, EditorMode: m.EnableEditorMode,
		EditorSupported: versions.SupportsEditorMode(m.GameVersion, inst.Channel), EditorMinVersion: versions.EditorMinVersion(inst.Channel),
		LaunchArgs: m.LaunchArgs, EnvVars: instEnvLines(m.EnvVars), ModCount: instCountMods(inst),
		IsDefault: defaultName != "" && strings.EqualFold(defaultName, inst.Name),
		Players:   []instShowPlayer{},
	}
	if !m.CreatedAt.IsZero() {
		t := m.CreatedAt
		it.CreatedAt = &t
	}
	if v := LoaderVersion(inst); v != "" {
		it.LoaderVersion = &v
	}
	if logo := filepath.Join(inst.Dir, "LargeLogo.png"); utils.FileExists(logo) {
		it.HasLogo, it.LogoPath = true, &logo
	}
	if lnk := instShortcutPath(inst.Name); lnk != "" && utils.FileExists(lnk) {
		it.Shortcut = &lnk
	}
	roots := instContentRoots(inst)
	if roots.Base != "" {
		base := roots.Base
		it.DataPath = &base
	}
	it.ResourcePacks = instCountDirs(roots.ResourcePacks)
	it.BehaviorPacks = instCountDirs(roots.BehaviorPacks)
	counts := func(player string) instContentCounts {
		cc := instContentCounts{
			Worlds:      instCountDirs(instKindPath(inst, roots, "worlds", player)),
			Screenshots: instCountScreenshots(instKindPath(inst, roots, "screenshots", player)),
			Servers:     instCountServers(instKindPath(inst, roots, "servers", player)),
		}
		if instKindIsPerPlayer(inst, "skin-packs") {
			n := instCountDirs(instKindPath(inst, roots, "skin-packs", player))
			cc.SkinPacks = &n
		}
		return cc
	}
	if inst.IsUWP() {
		cc := counts("")
		n := instCountDirs(instKindPath(inst, roots, "skin-packs", ""))
		cc.SkinPacks = &n
		it.Content = &cc
		return it
	}
	if instSharedSkins(inst) {
		n := instCountDirs(instKindPath(inst, roots, "skin-packs", ""))
		it.SharedSkinPacks = &n
	}
	for _, p := range players {
		row := instShowPlayer{ID: p.ID, instContentCounts: counts(p.ID)}
		if p.Gamertag != "" {
			tag := p.Gamertag
			row.Gamertag = &tag
		}
		it.Players = append(it.Players, row)
	}
	return it
}

func instOnOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func instWriteShow(w io.Writer, it instShowItem) error {
	data := "shared (system Minecraft data)"
	if it.Isolated {
		data = "own"
	}
	if it.DataPath != nil {
		data += ", " + *it.DataPath
	}
	reg := "no"
	if it.Registered {
		reg = "yes, as Minecraft (" + it.Slot + ")"
	}
	running := "no"
	if it.Running {
		running = "yes (pid " + joinInts(it.PIDs) + ")"
	}
	editor := instOnOff(it.EditorMode)
	if !it.EditorSupported {
		editor = "unsupported (needs game " + it.EditorMinVersion + " or newer)"
	}
	loader := "none"
	if it.LoaderVersion != nil {
		loader = "LeviLamina " + *it.LoaderVersion
	}
	logo := "none"
	if it.LogoPath != nil {
		logo = *it.LogoPath
	}
	shortcut := "none"
	if it.Shortcut != nil {
		shortcut = *it.Shortcut
	}
	created := "-"
	if it.CreatedAt != nil {
		created = Ago(*it.CreatedAt)
	}
	env := "none"
	if len(it.EnvVars) > 0 {
		env = strings.Join(it.EnvVars, "; ")
	}
	name := it.Name
	if it.IsDefault {
		name += " (default)"
	}
	path := it.Path
	if it.Linked {
		path += " (linked folder)"
	}
	pairs := []string{
		"name", name,
		"path", path,
		"game version", Dash(it.GameVersion),
		"channel", Dash(it.Channel),
		"type", strings.ToUpper(it.Type),
		"data", data,
		"registered", reg,
		"running", running,
		"console", instOnOff(it.Console),
		"editor mode", editor,
	}
	if it.Type != versions.PackageTypeUWP {
		pairs = append(pairs, "launch args", Dash(it.LaunchArgs), "environment", env)
	}
	pairs = append(pairs,
		"loader", loader,
		"mods", fmt.Sprint(it.ModCount),
		"logo", logo,
		"shortcut", shortcut,
		"created", created,
		"resource packs", fmt.Sprint(it.ResourcePacks),
		"behavior packs", fmt.Sprint(it.BehaviorPacks),
	)
	if it.SharedSkinPacks != nil {
		pairs = append(pairs, "skin packs", fmt.Sprint(*it.SharedSkinPacks))
	}
	if it.Content != nil {
		pairs = append(pairs,
			"worlds", fmt.Sprint(it.Content.Worlds),
			"screenshots", fmt.Sprint(it.Content.Screenshots),
			"servers", fmt.Sprint(it.Content.Servers),
			"skin packs", fmt.Sprint(instIntOr(it.Content.SkinPacks)),
		)
	}
	if err := KV(w, pairs...); err != nil {
		return err
	}
	if it.Type == versions.PackageTypeUWP {
		return nil
	}
	if len(it.Players) == 0 {
		fmt.Fprintln(w, "\nNo player profiles yet (they appear after the first start of the game).")
		return nil
	}
	fmt.Fprintln(w)
	header := []string{"PLAYER", "GAMERTAG", "WORLDS", "SCREENSHOTS", "SERVERS"}
	perPlayerSkins := it.SharedSkinPacks == nil
	if perPlayerSkins {
		header = append(header, "SKINS")
	}
	rows := make([][]string, 0, len(it.Players))
	for _, p := range it.Players {
		tag := "-"
		if p.Gamertag != nil {
			tag = *p.Gamertag
		}
		row := []string{p.ID, tag, fmt.Sprint(p.Worlds), fmt.Sprint(p.Screenshots), fmt.Sprint(p.Servers)}
		if perPlayerSkins {
			row = append(row, fmt.Sprint(instIntOr(p.SkinPacks)))
		}
		rows = append(rows, row)
	}
	return Table(w, header, rows)
}

func instIntOr(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// ---------------------------------------------------------------------------
// instance set

// instSettings are the instance settings 'instance set' changes.
type instSettings struct {
	Console, Editor, Isolation bool
	Args                       string
	Env                        []string
}

// instPlanSet computes the new settings from the flags and checks them
// against what the instance supports. It returns the new settings and a
// list of human-readable changes (empty for a no-op).
func instPlanSet(c *Ctx, inst Instance) (instSettings, []string, error) {
	m := inst.Meta
	old := instSettings{Console: m.EnableConsole, Editor: m.EnableEditorMode, Isolation: m.EnableIsolation,
		Args: m.LaunchArgs, Env: instEnvLines(m.EnvVars)}
	next := old
	next.Env = append([]string{}, old.Env...)
	var changes []string

	if c.Changed("console") {
		next.Console = c.Bool("console")
	}
	if c.Changed("editor") {
		next.Editor = c.Bool("editor")
		if next.Editor && !versions.SupportsEditorMode(m.GameVersion, inst.Channel) {
			return next, nil, Errorf(ExitUsage, "ERR_UNSUPPORTED", "editor mode needs game %s or newer; this instance is %s",
				versions.EditorMinVersion(inst.Channel), Dash(m.GameVersion))
		}
	}
	ownSet, sharedSet := c.Changed("own-data"), c.Changed("shared-data")
	if ownSet || sharedSet {
		own := c.Bool("own-data")
		if sharedSet {
			if ownSet && own == c.Bool("shared-data") {
				return next, nil, UsageError("--own-data and --shared-data contradict each other")
			}
			own = !c.Bool("shared-data")
		}
		if own && !versions.SupportsIsolation(inst.PackageType, m.GameVersion) {
			return next, nil, Errorf(ExitUsage, "ERR_UNSUPPORTED", "own data on a UWP instance needs game %s or newer; this instance is %s",
				versions.UWPIsolationMinVersion, Dash(m.GameVersion))
		}
		next.Isolation = own
	}
	gdkOnly := []string{"args", "env", "unset-env", "clear-env"}
	for _, f := range gdkOnly {
		if c.Changed(f) && inst.IsUWP() {
			return next, nil, Errorf(ExitUsage, "ERR_UNSUPPORTED", "--%s is not supported on UWP instances: the game is started by Windows without arguments or environment", f)
		}
	}
	if c.Changed("args") {
		next.Args = strings.TrimSpace(c.String("args"))
		if strings.ContainsAny(next.Args, "\r\n") {
			return next, nil, UsageError("--args must be one line")
		}
	}
	if c.Changed("env") || c.Changed("unset-env") || c.Changed("clear-env") {
		env, err := instApplyEnv(next.Env, c.Bool("clear-env"), c.Strings("unset-env"), c.Strings("env"))
		if err != nil {
			return next, nil, err
		}
		next.Env = env
	}

	if old.Console != next.Console {
		changes = append(changes, "console "+instOnOff(old.Console)+" → "+instOnOff(next.Console))
	}
	if old.Editor != next.Editor {
		changes = append(changes, "editor mode "+instOnOff(old.Editor)+" → "+instOnOff(next.Editor))
	}
	if old.Isolation != next.Isolation {
		from, to := "shared", "own"
		if old.Isolation {
			from, to = "own", "shared"
		}
		changes = append(changes, "data "+from+" → "+to)
	}
	if old.Args != next.Args {
		changes = append(changes, fmt.Sprintf("launch args %q → %q", old.Args, next.Args))
	}
	if d := instEnvDiff(old.Env, next.Env); d != "" {
		changes = append(changes, d)
	}
	return next, changes, nil
}

// instEnvDiff summarises environment changes: "env +2 ~1 -1".
func instEnvDiff(old, next []string) string {
	oldBy := map[string]string{}
	for _, l := range old {
		oldBy[strings.ToLower(instEnvKey(l))] = l
	}
	nextBy := map[string]string{}
	for _, l := range next {
		nextBy[strings.ToLower(instEnvKey(l))] = l
	}
	var add, mod, del int
	for k, l := range nextBy {
		switch o, ok := oldBy[k]; {
		case !ok:
			add++
		case o != l:
			mod++
		}
	}
	for k := range oldBy {
		if _, ok := nextBy[k]; !ok {
			del++
		}
	}
	var parts []string
	if add > 0 {
		parts = append(parts, fmt.Sprintf("+%d", add))
	}
	if mod > 0 {
		parts = append(parts, fmt.Sprintf("~%d", mod))
	}
	if del > 0 {
		parts = append(parts, fmt.Sprintf("-%d", del))
	}
	if len(parts) == 0 {
		if strings.Join(old, "\n") != strings.Join(next, "\n") {
			return "env reordered"
		}
		return ""
	}
	return "env " + strings.Join(parts, " ")
}

// instCheckLogo reads the image header to report a non-square logo with its
// size before the backend refuses it.
func instCheckLogo(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NotFoundError("no file %s", path)
		}
		return Errorf(ExitFailure, "ERR_ICON_DECODE", "cannot read %s: %v", path, err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return Backend("ERR_ICON_DECODE").WithDetail(err.Error())
	}
	if cfg.Width <= 0 || cfg.Width != cfg.Height {
		e := Backend("ERR_ICON_NOT_SQUARE")
		e.Message = fmt.Sprintf("the logo must be square (got %d×%d)", cfg.Width, cfg.Height)
		return e
	}
	return nil
}

func runInstanceSet(c *Ctx) error {
	inst, err := c.ResolveInstance(c.Arg(0))
	if err != nil {
		return err
	}
	given := false
	for _, f := range c.Cmd.Flags {
		if c.Changed(f.Name) {
			given = true
			break
		}
	}
	if !given {
		return UsageError("nothing to change: pass at least one setting").
			WithHint("%s help instance set", ProgramName)
	}
	logo := strings.TrimSpace(c.String("logo"))
	if logo != "" && c.Bool("clear-logo") {
		return UsageError("--logo and --clear-logo contradict each other")
	}
	next, changes, err := instPlanSet(c, inst)
	if err != nil {
		return err
	}
	if logo != "" {
		if abs, err := filepath.Abs(logo); err == nil {
			logo = abs
		}
		if err := instCheckLogo(logo); err != nil {
			return err
		}
	}
	if len(changes) > 0 {
		if pids := instRunningPIDs(inst); len(pids) > 0 {
			return instRunningError(inst, pids)
		}
		typ := strings.TrimSpace(inst.Meta.Type)
		if typ == "" {
			typ = "release"
		}
		code := instSaveMeta(inst.Name, inst.Meta.GameVersion, typ, next.Isolation, next.Console, next.Editor,
			next.Args, strings.Join(next.Env, "\n"))
		if e := Backend(code); e != nil {
			return e
		}
	}
	switch {
	case logo != "":
		if e := Backend(instSaveLogo(inst.Name, logo)); e != nil {
			return e
		}
		changes = append(changes, "logo set")
	case c.Bool("clear-logo"):
		if utils.FileExists(filepath.Join(inst.Dir, "LargeLogo.png")) {
			if e := Backend(instRemoveLogo(inst.Name)); e != nil {
				return e
			}
			changes = append(changes, "logo removed")
		}
	}
	if len(changes) == 0 {
		c.Notice("Nothing to change: %s already has these settings", inst.Name)
	} else {
		c.Notice("Updated %s: %s", inst.Name, strings.Join(changes, ", "))
	}
	updated, err := LookupInstance(inst.Name)
	if err != nil {
		updated = inst
	}
	defaultName, _, _ := c.DefaultInstance()
	release, preview := "", ""
	if updated.Meta.Registered {
		release, preview = registeredLocations()
	}
	item := describeInstance(updated, defaultName, release, preview, instRunningPIDs(updated))
	return c.Render(item, []string{item.Name}, nil)
}

// ---------------------------------------------------------------------------
// instance rename

type instRenameResult struct {
	From            string `json:"from"`
	To              string `json:"to"`
	ShortcutUpdated bool   `json:"shortcutUpdated"`
	DefaultUpdated  bool   `json:"defaultUpdated"`
}

func runInstanceRename(c *Ctx) error {
	inst, err := LookupInstance(c.Arg(0))
	if err != nil {
		return err
	}
	newName := strings.TrimSpace(c.Arg(1))
	res := instRenameResult{From: inst.Name, To: newName}
	if newName == inst.Name {
		c.Notice("%s already has that name", inst.Name)
		return c.Render(res, []string{newName}, nil)
	}
	if strings.EqualFold(newName, inst.Name) {
		if code := versions.ValidateFolderName(newName); code != "" {
			return Backend(code)
		}
	} else if code := instValidateName(newName); code != "" {
		e := Backend(code)
		if code == "ERR_NAME_EXISTS" {
			e.Message = fmt.Sprintf("an instance named %q already exists", newName)
		}
		return e
	}
	all, err := ListInstances()
	if err != nil {
		return err
	}
	if pids := instRunningPIDs(inst); len(pids) > 0 {
		return instRunningError(inst, pids)
	}
	if instRegistration(inst, all).Registered {
		return Errorf(ExitConflict, "ERR_INSTANCE_REGISTERED", "%s is registered with Windows as Minecraft (%s); renaming would release it", inst.Name, instSlot(inst)).
			WithHint("%s instance unregister %s", ProgramName, quoteArg(inst.Name))
	}
	oldLnk := instShortcutPath(inst.Name)
	hadShortcut := oldLnk != "" && utils.FileExists(oldLnk)
	if e := Backend(instRenameFolder(inst.Name, newName)); e != nil {
		return e
	}
	// For a case-only change the backend only rewrites the name in the
	// metadata; the folder, which names the instance, keeps its case.
	if base := filepath.Base(inst.Dir); base != newName && strings.EqualFold(base, newName) {
		if err := os.Rename(inst.Dir, filepath.Join(filepath.Dir(inst.Dir), newName)); err != nil {
			return Backend("ERR_RENAME_FAILED").WithDetail(err.Error())
		}
	}
	if strings.EqualFold(strings.TrimSpace(defaultInstanceCfg()), inst.Name) {
		if err := instSetDefault(newName); err != nil {
			c.Warn("the default instance still names %s: %v", inst.Name, err)
		} else {
			res.DefaultUpdated = true
		}
	}
	if hadShortcut {
		if e := Backend(instCreateShortcut(newName)); e != nil {
			c.Warn("could not update the desktop shortcut %s: %s", oldLnk, e.Message)
		} else {
			res.ShortcutUpdated = true
			if newLnk := instShortcutPath(newName); !strings.EqualFold(newLnk, oldLnk) {
				_ = os.Remove(oldLnk)
			}
		}
	}
	note := ""
	if res.ShortcutUpdated {
		note = " (shortcut updated)"
	}
	c.Notice("Renamed %q → %q%s", inst.Name, newName, note)
	return c.Render(res, []string{newName}, nil)
}

// ---------------------------------------------------------------------------
// instance remove

type instRemoveResult struct {
	Name    string `json:"name"`
	Removed bool   `json:"removed"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func runInstanceRemove(c *Ctx) error {
	var targets []Instance
	seen := map[string]bool{}
	for _, a := range c.Args {
		inst, err := LookupInstance(a)
		if err != nil {
			return err
		}
		if k := strings.ToLower(inst.Name); !seen[k] {
			seen[k] = true
			targets = append(targets, inst)
		}
	}
	all, err := ListInstances()
	if err != nil {
		return err
	}
	// Refuse up front what cannot be removed when it is the only target, so
	// the user is not asked first.
	if len(targets) == 1 {
		if err := instRemovable(targets[0], all); err != nil {
			return err
		}
	}
	if !c.G.Yes {
		if err := c.Confirm(instRemoveQuestion(c, targets)); err != nil {
			return err
		}
	}
	results := make([]instRemoveResult, 0, len(targets))
	var failed []*Error
	var skipped []string
	for i, inst := range targets {
		// Ctrl-C stops the batch between instances: the one being deleted
		// is finished, the rest are left alone.
		if c.Context().Err() != nil {
			for _, t := range targets[i:] {
				skipped = append(skipped, t.Name)
			}
			break
		}
		r := instRemoveResult{Name: inst.Name}
		if e := FromError(instRemoveOne(c, inst, all)); e != nil {
			r.Error = &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{e.Code, e.Message}
			failed = append(failed, e)
			if len(targets) > 1 {
				c.errf("error: %s: %s [%s]\n", inst.Name, e.Message, e.Code)
			}
		} else {
			r.Removed = true
		}
		results = append(results, r)
	}
	if len(targets) == 1 && len(failed) == 1 {
		return failed[0]
	}
	var ids []string
	for _, r := range results {
		if r.Removed {
			ids = append(ids, r.Name)
		}
	}
	if err := c.Render(results, ids, func(w io.Writer) error {
		for _, r := range results {
			if r.Removed {
				fmt.Fprintf(w, "Removed %s\n", r.Name)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if len(skipped) > 0 {
		quoted := make([]string, len(skipped))
		for i, n := range skipped {
			quoted[i] = quoteArg(n)
		}
		return Errorf(ExitInterrupted, "ERR_INTERRUPTED", "interrupted; not removed: %s", strings.Join(skipped, ", ")).
			WithHint("%s instance remove %s", ProgramName, strings.Join(quoted, " "))
	}
	return instBatchExit(c, len(targets), len(failed), "instances", "removed")
}

// instBatchExit reports a batch summary and returns its exit: nil when every
// item succeeded, 7 when some failed, 1 when all failed.
func instBatchExit(c *Ctx, total, failed int, noun, verb string) error {
	switch {
	case failed == 0:
		return nil
	case failed < total:
		c.Notice("%s %d of %d %s; %d failed", capitalize(verb), total-failed, total, noun, failed)
		return ExitCode(ExitPartial)
	}
	c.Notice("No %s were %s", noun, verb)
	return ExitCode(ExitFailure)
}

// instRemovable checks the guards the backend lacks.
func instRemovable(inst Instance, all []Instance) error {
	if pids := instRunningPIDs(inst); len(pids) > 0 {
		return instRunningError(inst, pids)
	}
	if instRegistration(inst, all).Registered {
		return Errorf(ExitConflict, "ERR_INSTANCE_REGISTERED", "%s is registered with Windows as Minecraft (%s)", inst.Name, instSlot(inst)).
			WithHint("%s instance unregister %s", ProgramName, quoteArg(inst.Name))
	}
	return nil
}

func instRemoveQuestion(c *Ctx, targets []Instance) string {
	if !c.CanPrompt() {
		names := make([]string, len(targets))
		for i, t := range targets {
			names[i] = t.Name
		}
		return "Permanently delete instance " + strings.Join(names, ", ") + "?"
	}
	var b strings.Builder
	b.WriteString("This permanently deletes:\n")
	for _, t := range targets {
		if instIsLinked(t.Dir) {
			fmt.Fprintf(&b, "  %s  %s (only the link; the linked folder is kept)\n", t.Name, t.Dir)
			continue
		}
		extra := ""
		if t.Meta.EnableIsolation {
			roots := instContentRoots(t)
			worlds := 0
			if t.IsUWP() {
				worlds = instCountDirs(roots.Worlds)
			} else if ps, err := ListPlayers(t); err == nil {
				for _, p := range ps {
					worlds += instCountDirs(instKindPath(t, roots, "worlds", p.ID))
				}
			}
			if worlds > 0 {
				extra = fmt.Sprintf(", includes %d worlds", worlds)
			}
		}
		fmt.Fprintf(&b, "  %s  %s (%s%s)\n", t.Name, t.Dir, Size(utils.DirSize(t.Dir)), extra)
	}
	b.WriteString("Continue?")
	return b.String()
}

func instRemoveOne(c *Ctx, inst Instance, all []Instance) error {
	if err := instRemovable(inst, all); err != nil {
		return err
	}
	if instIsLinked(inst.Dir) {
		if err := os.Remove(inst.Dir); err != nil {
			return Backend("ERR_DELETE_FAILED").WithDetail(err.Error())
		}
	} else if e := Backend(instDeleteFolder(inst.Name)); e != nil {
		return e
	}
	if strings.EqualFold(strings.TrimSpace(defaultInstanceCfg()), inst.Name) {
		if err := instSetDefault(""); err != nil {
			c.Warn("could not clear the default instance: %v", err)
		}
	}
	if lnk := instShortcutPath(inst.Name); lnk != "" && utils.FileExists(lnk) {
		if err := os.Remove(lnk); err != nil {
			c.Warn("could not remove the desktop shortcut %s: %v", lnk, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// instance register / unregister

func runInstanceRegister(c *Ctx) error {
	inst, err := c.ResolveInstance(c.Arg(0))
	if err != nil {
		return err
	}
	res, err := instRegisterFlow(c, inst)
	if err != nil {
		return err
	}
	if res.Changed {
		c.Notice("Registered %s as Minecraft (%s)", inst.Name, res.Channel)
	} else {
		c.Notice("%s is already registered as Minecraft (%s)", inst.Name, res.Channel)
	}
	return c.Render(res, []string{inst.Name}, nil)
}

func runInstanceUnregister(c *Ctx) error {
	inst, err := c.ResolveInstance(c.Arg(0))
	if err != nil {
		return err
	}
	all, err := ListInstances()
	if err != nil {
		return err
	}
	res := instRegisterResult{Instance: inst.Name, Channel: instSlot(inst)}
	if !instRegistration(inst, all).Registered {
		c.Notice("%s is not registered with Windows", inst.Name)
		return c.Render(res, []string{inst.Name}, nil)
	}
	if pids := instRunningPIDs(inst); len(pids) > 0 {
		return instRunningError(inst, pids)
	}
	p := c.Progress()
	p.Phase("Releasing the registration of %s", inst.Name)
	code := instUnregister(c.Context(), inst.Name)
	p.Done()
	if err := c.Context().Err(); err != nil {
		return err
	}
	if bc, _, _ := SplitBackendCode(code); bc != "ERR_NOT_REGISTERED_THIS_VERSION" {
		if e := Backend(code); e != nil {
			return e
		}
	}
	if instRegistration(inst, all).Registered {
		code := "ERR_UNREGISTER_FAILED"
		if inst.IsUWP() {
			code = "ERR_UWP_UNREGISTER"
		}
		return Backend(code).WithDetail("Windows still lists the instance folder as the registered package")
	}
	res.Changed = true
	c.Notice("Unregistered %s; its game data is kept", inst.Name)
	return c.Render(res, []string{inst.Name}, nil)
}

// ---------------------------------------------------------------------------
// instance copy-data

type instCopyDataResult struct {
	Instance string `json:"instance"`
	From     string `json:"from"`
	Files    int64  `json:"files"`
	Bytes    int64  `json:"bytes"`
}

// instGDKDataDir is the game-data folder of a GDK instance with its own data.
func instGDKDataDir(inst Instance) string {
	if inst.IsPreview() {
		return "Minecraft Bedrock Preview"
	}
	return "Minecraft Bedrock"
}

func runInstanceCopyData(c *Ctx) error {
	inst, err := c.ResolveInstance(c.Arg(0))
	if err != nil {
		return err
	}
	from := strings.TrimSpace(c.String("from"))
	if from == "" {
		return UsageError("--from is required: 'system' or an instance name").
			WithHint("%s instance copy-data %s --from system", ProgramName, quoteArg(inst.Name))
	}
	if inst.IsUWP() {
		return Errorf(ExitUsage, "ERR_INHERIT_TYPE_MISMATCH", "%s is a UWP instance; game data can only be copied into GDK instances", inst.Name)
	}
	if !inst.Meta.EnableIsolation {
		return Errorf(ExitUsage, "ERR_NOT_ISOLATED", "%s uses the shared system data, so there is nothing to copy into", inst.Name).
			WithHint("%s instance set %s --own-data", ProgramName, quoteArg(inst.Name))
	}
	var src, label string
	var srcInst *Instance
	if strings.EqualFold(from, "system") {
		label = "the system Minecraft (" + instSlot(inst) + ")"
		src = strings.TrimSpace(instSystemDataPath(inst.IsPreview()))
		if src == "" || !utils.ResolvesToDir(src) {
			return Backend("ERR_NO_GAME_DATA").WithHint("start the Microsoft Store Minecraft (%s) once, or copy from an instance", instSlot(inst))
		}
	} else {
		s, err := LookupInstance(from)
		if err != nil {
			return err
		}
		if strings.EqualFold(s.Name, inst.Name) {
			return UsageError("cannot copy %s into itself", inst.Name)
		}
		if s.IsUWP() || !strings.EqualFold(s.Channel, inst.Channel) {
			return Errorf(ExitUsage, "ERR_INHERIT_TYPE_MISMATCH",
				"%s is a %s %s instance and %s a GDK %s one; game data can only be copied between GDK instances of the same channel",
				s.Name, strings.ToUpper(s.PackageType), Dash(s.Channel), inst.Name, Dash(inst.Channel))
		}
		if !s.Meta.EnableIsolation {
			return Errorf(ExitUsage, "ERR_NOT_ISOLATED", "%s uses the shared system data", s.Name).
				WithHint("%s instance copy-data %s --from system", ProgramName, quoteArg(inst.Name))
		}
		src = filepath.Join(s.Dir, instGDKDataDir(s))
		if !utils.ResolvesToDir(src) {
			return Errorf(ExitNotFound, "ERR_INHERIT_SOURCE_NOT_FOUND", "%s has no game data yet", s.Name)
		}
		label = s.Name
		srcInst = &s
	}
	if pids := instRunningPIDs(inst); len(pids) > 0 {
		return instRunningError(inst, pids)
	}
	if srcInst != nil {
		if pids := instRunningPIDs(*srcInst); len(pids) > 0 {
			return instRunningError(*srcInst, pids)
		}
	}
	dst := filepath.Join(inst.Dir, instGDKDataDir(inst))
	if instDirHasFiles(dst) {
		q := fmt.Sprintf("Merge the game data of %s into %s (%s), overwriting files with the same name?", label, inst.Name, dst)
		if err := c.Confirm(q); err != nil {
			return err
		}
	}
	p := c.Progress()
	p.Phase("Measuring the game data of %s", label)
	totalFiles, totalBytes, err := instMeasureTree(c.Context(), src)
	if err != nil {
		return instCopyError(err)
	}
	p.Phase("Copying game data from %s", label)
	files, bytes, err := instCopyTree(c.Context(), src, dst, func(files, bytes int64) {
		if totalBytes > 0 {
			p.Bytes(bytes, totalBytes)
		} else {
			p.Count(files, totalFiles, "files")
		}
	})
	p.Done()
	if err != nil {
		return instCopyError(err)
	}
	c.Notice("Copied %s of game data (%d files) from %s into %s", Size(bytes), files, label, inst.Name)
	return c.Render(instCopyDataResult{Instance: inst.Name, From: from, Files: files, Bytes: bytes}, []string{dst}, nil)
}

func instCopyError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return Backend("ERR_INHERIT_COPY_FAILED").WithDetail(err.Error()).
		WithHint("run the same command again; files already copied are overwritten")
}

// instDirHasFiles reports whether dir contains any regular file.
func instDirHasFiles(dir string) bool {
	found := errors.New("found")
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			return found
		}
		return nil
	})
	return err == found
}

// instMeasureTree counts the regular files under dir and their bytes.
func instMeasureTree(ctx context.Context, dir string) (files, bytes int64, err error) {
	err = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			files++
			bytes += info.Size()
		}
		return nil
	})
	return files, bytes, err
}

// instCopyTree merges src into dst like utils.CopyDir (folders created, regular
// files overwritten, nothing deleted), reporting progress and stopping when
// ctx ends.
func instCopyTree(ctx context.Context, src, dst string, progress func(files, bytes int64)) (files, bytes int64, err error) {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return 0, 0, err
	}
	buf := make([]byte, 1<<20)
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		n, err := instCopyFile(ctx, path, target, info.Mode(), buf, func(delta int64) {
			bytes += delta
			progress(files, bytes)
		})
		_ = n
		if err != nil {
			return err
		}
		files++
		progress(files, bytes)
		return nil
	})
	return files, bytes, err
}

func instCopyFile(ctx context.Context, src, dst string, mode os.FileMode, buf []byte, onWrite func(int64)) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return 0, err
	}
	var total int64
	for {
		if e := ctx.Err(); e != nil {
			out.Close()
			return total, e
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return total, werr
			}
			total += int64(n)
			onWrite(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			return total, rerr
		}
	}
	return total, out.Close()
}

// ---------------------------------------------------------------------------
// instance shortcut

type instShortcutResult struct {
	Instance string `json:"instance"`
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	Replaced bool   `json:"replaced"`
}

func runInstanceShortcut(c *Ctx) error {
	inst, err := c.ResolveInstance(c.Arg(0))
	if err != nil {
		return err
	}
	lnk := instShortcutPath(inst.Name)
	if lnk == "" {
		return Backend("ERR_SHORTCUT_CREATE_FAILED").WithDetail("the desktop folder cannot be found")
	}
	existed := utils.FileExists(lnk)
	res := instShortcutResult{Instance: inst.Name, Path: lnk}
	if c.Bool("remove") {
		if existed {
			if err := os.Remove(lnk); err != nil {
				return Errorf(ExitFailure, "ERR_SHORTCUT_REMOVE_FAILED", "cannot remove %s: %v", lnk, err)
			}
			c.Notice("Removed the desktop shortcut of %s", inst.Name)
		} else {
			c.Notice("%s has no desktop shortcut", inst.Name)
		}
		return c.Render(res, []string{lnk}, func(w io.Writer) error { _, err := fmt.Fprintln(w, lnk); return err })
	}
	if e := Backend(instCreateShortcut(inst.Name)); e != nil {
		return e
	}
	res.Exists, res.Replaced = true, existed
	if existed {
		c.Notice("Replaced the existing shortcut")
	}
	return c.Render(res, []string{lnk}, func(w io.Writer) error { _, err := fmt.Fprintln(w, lnk); return err })
}

// ---------------------------------------------------------------------------
// instance path

type instPathResult struct {
	Instance string  `json:"instance"`
	Kind     string  `json:"kind"`
	Player   *string `json:"player"`
	Path     string  `json:"path"`
	Exists   bool    `json:"exists"`
}

func instIsPathKind(s string) bool {
	for _, k := range instPathKinds {
		if strings.EqualFold(k, s) {
			return true
		}
	}
	return false
}

func runInstancePath(c *Ctx) error {
	instArg, kind := c.Arg(0), "root"
	switch len(c.Args) {
	case 1:
		// "instance path worlds": a kind word when no instance has that name.
		if instIsPathKind(instArg) {
			if _, err := LookupInstance(instArg); err != nil {
				instArg, kind = "", strings.ToLower(c.Arg(0))
			}
		}
	case 2:
		kind = strings.ToLower(c.Arg(1))
	}
	if !instIsPathKind(kind) {
		return UsageError("unknown folder kind %q", kind).WithHint("use one of: %s", strings.Join(instPathKinds, ", "))
	}
	inst, err := c.ResolveInstance(instArg)
	if err != nil {
		return err
	}
	res := instPathResult{Instance: inst.Name, Kind: kind}
	player := ""
	if instKindIsPerPlayer(inst, kind) {
		p, err := c.ResolvePlayer(inst)
		if err != nil {
			return err
		}
		player = p.ID
		res.Player = &player
	}
	roots := instContentRoots(inst)
	res.Path = instKindPath(inst, roots, kind, player)
	if res.Path == "" {
		return Errorf(ExitNotFound, "ERR_PATH_NOT_FOUND", "%s has no %s folder: its game data folder cannot be located", inst.Name, kind).
			WithHint("start the game once: %s play %s", ProgramName, quoteArg(inst.Name))
	}
	res.Exists = utils.ResolvesToDir(res.Path)
	if !res.Exists {
		if c.G.JSON {
			_ = c.JSON(res)
		}
		what := kind + " folder"
		if player != "" {
			what += " for this player"
		}
		return Errorf(ExitNotFound, "ERR_PATH_NOT_FOUND", "no %s of %s yet: %s", what, inst.Name, res.Path).
			WithHint("start the game once: %s play %s", ProgramName, quoteArg(inst.Name))
	}
	if c.Bool("open") {
		if err := instOpenFolder(res.Path); err != nil {
			c.Warn("could not open Explorer: %v", err)
		}
	}
	return c.Render(res, []string{res.Path}, func(w io.Writer) error { _, err := fmt.Fprintln(w, res.Path); return err })
}

// ---------------------------------------------------------------------------
// use

type instUseResult struct {
	Instance *string `json:"instance"`
	Source   string  `json:"source"` // setting, env, only, none
}

func instSourceWord(source string) string {
	switch source {
	case SourceEnv:
		return "env"
	case SourceSetting:
		return "setting"
	case SourceOnly:
		return "only"
	}
	return "none"
}

func runUse(c *Ctx) error {
	arg := strings.TrimSpace(c.Arg(0))
	if arg == "" {
		arg = strings.TrimSpace(c.G.Instance)
	}
	if c.Bool("clear") {
		if arg != "" {
			return UsageError("--clear takes no instance")
		}
		if strings.TrimSpace(defaultInstanceCfg()) == "" {
			c.Notice("No default instance is set")
		} else {
			if err := instSetDefault(""); err != nil {
				return FromError(err)
			}
			c.Notice("Cleared the default instance")
		}
		return c.Render(instUseResult{Source: "none"}, nil, nil)
	}
	if arg == "" {
		name, source, err := c.DefaultInstance()
		if err != nil {
			return err
		}
		res := instUseResult{Source: instSourceWord(source)}
		if name != "" {
			res.Instance = &name
		}
		return c.Render(res, instNonEmpty(name), func(w io.Writer) error {
			switch source {
			case SourceSetting:
				_, err = fmt.Fprintf(w, "Default instance: %s\n", name)
			case SourceEnv:
				_, err = fmt.Fprintf(w, "Default instance: %s (from %s)\n", name, EnvInstance)
			case SourceOnly:
				_, err = fmt.Fprintf(w, "No default instance (using the only instance: %s)\n", name)
			default:
				_, err = fmt.Fprintln(w, "No default instance")
			}
			return err
		})
	}
	inst, err := LookupInstance(arg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(defaultInstanceCfg()) == inst.Name {
		c.Notice("%s is already the default instance", inst.Name)
	} else {
		if err := instSetDefault(inst.Name); err != nil {
			return FromError(err)
		}
		c.Notice("Default instance: %s", inst.Name)
	}
	if env := strings.TrimSpace(c.Getenv(EnvInstance)); env != "" && !strings.EqualFold(env, inst.Name) {
		c.Warn("%s=%s overrides the default instance in this shell", EnvInstance, env)
	}
	name := inst.Name
	return c.Render(instUseResult{Instance: &name, Source: "setting"}, []string{name}, nil)
}

func instNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
