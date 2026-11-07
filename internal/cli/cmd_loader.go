package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/catalog"
	"github.com/liteldev/LeviLauncher/internal/lip"
)

// The loader commands manage LeviLamina on a GDK instance: the versions
// that support its game version, installing or changing it, and removing it.

// giLeviLaminaPackage is LeviLamina's package id in lip workspaces.
const giLeviLaminaPackage = "github.com/LiteLDev/LeviLamina"

type loaderVersionItem struct {
	Version     string `json:"version"`
	GameVersion string `json:"gameVersion"`
	Prerelease  bool   `json:"prerelease"`
	Installed   bool   `json:"installed"`
}

func init() {
	Register(&Command{
		Path:    "loader versions",
		Summary: "list the LeviLamina versions available for a game version",
		Description: `Lists the LeviLamina versions that support a Minecraft version, newest
first. The game version is that of the instance (-i, or the default instance)
unless --game names one; --game also accepts a dotted prefix such as 1.21.132.
The version installed on the instance is marked '*'. Pre-releases (release
candidates) are listed only with --prerelease.`,
		Examples: []string{"loader versions", "loader versions -i Survival --prerelease", "loader versions --game 1.21.132 -q"},
		Flags: []*Flag{
			StringFlag("game", "", "<version>", "game version to list for (default: the instance's)"),
			BoolFlag("prerelease", "", "include pre-release versions"),
		},
		Run: runLoaderVersions,
	})
	Register(&Command{
		Path:    "loader install",
		Summary: "install LeviLamina on an instance, or change its version",
		Args:    "[<version>]",
		Description: `Installs LeviLamina on a GDK instance, or changes the installed version.
<version> defaults to the newest stable version for the instance's game
version; a pre-release must be named explicitly. Installing the version that
is already installed does nothing unless --reinstall is given.

A downgrade, a reinstall or a pre-release asks for confirmation; --yes skips
it. The game must not be running.`,
		Examples: []string{"loader install", "loader install 1.9.2 -i Survival", "loader install --reinstall -y"},
		Flags: []*Flag{
			BoolFlag("reinstall", "", "install again when this version is already installed"),
		},
		MaxArgs: 1,
		Run:     runLoaderInstall,
	})
	Register(&Command{
		Path:    "loader remove",
		Summary: "remove LeviLamina from an instance",
		Description: `Removes LeviLamina from a GDK instance. Mods installed from the LIP
catalogue that need it keep it installed: remove them first. Asks for
confirmation; --yes skips it. Removing it when it is not installed does
nothing.`,
		Examples: []string{"loader remove", "loader rm -i Survival -y"},
		Run:      runLoaderRemove,
	})
}

// giLoaderInstance resolves the instance of a loader command; LeviLamina
// runs on GDK instances only.
func giLoaderInstance(c *Ctx) (Instance, error) {
	inst, err := c.ResolveInstance("")
	if err != nil {
		return Instance{}, err
	}
	if inst.IsUWP() {
		e := Backend("ERR_UWP_UNSUPPORTED_FEATURE")
		e.Message = fmt.Sprintf("%s is a UWP instance; LeviLamina supports GDK instances only", inst.Name)
		return Instance{}, e
	}
	return inst, nil
}

func runLoaderVersions(c *Ctx) error {
	game := strings.TrimSpace(c.String("game"))
	if game != "" && strings.TrimSpace(c.G.Instance) != "" {
		return UsageError("--game and --instance exclude each other")
	}
	installed := ""
	var inst Instance
	if game == "" {
		var err error
		if inst, err = giLoaderInstance(c); err != nil {
			return err
		}
		game = strings.TrimSpace(inst.Meta.GameVersion)
		installed = LoaderVersion(inst)
		if game == "" {
			return Errorf(ExitFailure, "ERR_INVALID_VERSION_FORMAT", "the game version of %s is unknown", inst.Name)
		}
	}
	db, err := giLoaderDB(c)
	if err != nil {
		return err
	}
	// An instance's version must match exactly (as the backend requires);
	// --game may also be a dotted prefix.
	key, ok := game, db.Supports(game)
	if !ok && inst.Name == "" {
		key, ok = db.ResolveGame(game)
		if ok && !catalog.VersionEqual(key, game) {
			c.Notice("Resolved %s to %s", game, key)
		}
	}
	if !ok {
		_, err := giPickLoaderVersion(c, db, game, "")
		e := FromError(err)
		e.Exit = ExitNotFound
		return e
	}
	all := db.Versions(key)
	items := []loaderVersionItem{}
	hidden := 0
	for _, v := range all {
		isInstalled := installed != "" && catalog.LoaderAction(installed, v) == catalog.ActionReinstall
		if catalog.IsPrerelease(v) && !c.Bool("prerelease") && !isInstalled {
			hidden++
			continue
		}
		items = append(items, loaderVersionItem{Version: v, GameVersion: key, Prerelease: catalog.IsPrerelease(v), Installed: isInstalled})
	}
	if len(items) == 0 && hidden > 0 {
		c.Notice("Minecraft %s only has pre-release LeviLamina versions; add --prerelease to list them.", key)
	} else if hidden > 0 {
		c.Verbosef("%d pre-release versions hidden; add --prerelease to list them", hidden)
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.Version
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		rows := make([][]string, len(items))
		for i, it := range items {
			mark := " "
			if it.Installed {
				mark = "*"
			}
			rows[i] = []string{mark + " " + it.Version, YesNo(it.Prerelease)}
		}
		return Table(w, []string{"  VERSION", "PRERELEASE"}, rows)
	})
}

type loaderInstallResult struct {
	Instance        string  `json:"instance"`
	GameVersion     string  `json:"gameVersion"`
	Version         string  `json:"version"`
	PreviousVersion *string `json:"previousVersion"`
	Action          string  `json:"action"` // install, upgrade, downgrade, reinstall, change, none
}

func runLoaderInstall(c *Ctx) error {
	inst, err := giLoaderInstance(c)
	if err != nil {
		return err
	}
	game := strings.TrimSpace(inst.Meta.GameVersion)
	if len(strings.Split(game, ".")) < 3 {
		e := Backend("ERR_INVALID_VERSION_FORMAT")
		e.Message = fmt.Sprintf("the game version of %s is not known (%q)", inst.Name, game)
		return e
	}
	db, err := giLoaderDB(c)
	if err != nil {
		return err
	}
	target, err := giPickLoaderVersion(c, db, game, c.Arg(0))
	if err != nil {
		e := FromError(err)
		if e.Code == "ERR_LL_VERSION_UNSUPPORTED" {
			e.Hint = fmt.Sprintf("%s loader versions -i %s --prerelease", ProgramName, quoteArg(inst.Name))
		}
		return e
	}
	current := LoaderVersion(inst)
	action := catalog.LoaderAction(current, target)
	res := loaderInstallResult{Instance: inst.Name, GameVersion: game, Version: target, Action: action}
	if current != "" {
		cur := current
		res.PreviousVersion = &cur
	}
	if action == catalog.ActionReinstall && !c.Bool("reinstall") {
		c.Notice("LeviLamina %s is already installed on %s.", target, inst.Name)
		res.Action = "none"
		return giRenderLoader(c, res)
	}
	var reasons []string
	switch action {
	case catalog.ActionDowngrade:
		reasons = append(reasons, fmt.Sprintf("this downgrades LeviLamina from %s to %s; mods built for %s may stop working", current, target, current))
	case catalog.ActionReinstall:
		reasons = append(reasons, fmt.Sprintf("this reinstalls LeviLamina %s", target))
	}
	if catalog.IsPrerelease(target) {
		reasons = append(reasons, fmt.Sprintf("%s is a pre-release (release candidate): mods may not work with it and it may be unstable", target))
	}
	if err := giNotRunning(inst, "changing LeviLamina"); err != nil {
		return err
	}
	if len(reasons) > 0 {
		if err := c.Confirm(fmt.Sprintf("On %s, %s. Continue?", inst.Name, strings.Join(reasons, "; "))); err != nil {
			return err
		}
	}
	p := c.Progress()
	p.Phase("Installing LeviLamina %s on %s", target, inst.Name)
	code := gi.installLoader(c.Context(), game, inst.Name, target)
	p.Done()
	if err := giCtxErr(c, code); err != nil {
		return err
	}
	if v := LoaderVersion(inst); v != "" && v != "?" {
		res.Version = v
	}
	return giRenderLoader(c, res)
}

func giRenderLoader(c *Ctx, res loaderInstallResult) error {
	return c.Render(res, []string{res.Version}, func(w io.Writer) error {
		switch {
		case res.Action == "none":
			return nil
		case res.PreviousVersion != nil && *res.PreviousVersion != res.Version:
			_, err := fmt.Fprintf(w, "LeviLamina %s installed on %s (was %s)\n", res.Version, res.Instance, *res.PreviousVersion)
			return err
		}
		_, err := fmt.Fprintf(w, "LeviLamina %s installed on %s\n", res.Version, res.Instance)
		return err
	})
}

type loaderRemoveResult struct {
	Instance string  `json:"instance"`
	Version  *string `json:"version"`
	Removed  bool    `json:"removed"`
	// Kept is true when LeviLamina stays installed as a dependency of other
	// packages.
	Kept bool `json:"kept"`
}

func runLoaderRemove(c *Ctx) error {
	inst, err := giLoaderInstance(c)
	if err != nil {
		return err
	}
	current := LoaderVersion(inst)
	states, serr := gi.loaderStates(c.Context(), inst.Dir)
	if serr != nil {
		if c.Context().Err() != nil {
			return FromError(c.Context().Err())
		}
		c.Verbosef("cannot read the installed packages of %s: %v", inst.Name, serr)
	}
	state := states[lip.PackageKey(giLeviLaminaPackage)]
	res := loaderRemoveResult{Instance: inst.Name}
	if current != "" {
		v := current
		res.Version = &v
	}
	if !state.Installed && current == "" {
		c.Notice("LeviLamina is not installed on %s.", inst.Name)
		return giRenderLoaderRemove(c, res)
	}
	if state.Installed && !state.ExplicitInstalled {
		var deps []string
		for k, s := range states {
			if k != lip.PackageKey(giLeviLaminaPackage) && s.ExplicitInstalled {
				deps = append(deps, strings.TrimPrefix(s.ID, "github.com/"))
			}
		}
		sort.Strings(deps)
		e := Backend("ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS")
		if len(deps) > 0 {
			e.Message = fmt.Sprintf("LeviLamina is required by the installed packages %s on %s", strings.Join(deps, ", "), inst.Name)
			e.Hint = fmt.Sprintf("remove them first: %s mod remove <mod> -i %s", ProgramName, quoteArg(inst.Name))
		}
		return e
	}
	if err := giNotRunning(inst, "removing LeviLamina"); err != nil {
		return err
	}
	label := "LeviLamina"
	if current != "" && current != "?" {
		label += " " + current
	}
	if err := c.Confirm(fmt.Sprintf("Remove %s from %s? Mods that need it will no longer load.", label, inst.Name)); err != nil {
		return err
	}
	p := c.Progress()
	p.Phase("Removing LeviLamina from %s", inst.Name)
	code := gi.uninstallLoader(c.Context(), inst.Name)
	p.Done()
	if code == "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY" {
		c.Warn("LeviLamina is still needed by other installed packages and was kept as their dependency")
		res.Kept = true
		return giRenderLoaderRemove(c, res)
	}
	if err := giCtxErr(c, code); err != nil {
		return err
	}
	res.Removed = true
	return giRenderLoaderRemove(c, res)
}

func giRenderLoaderRemove(c *Ctx, res loaderRemoveResult) error {
	var ids []string
	if res.Removed {
		ids = []string{res.Instance}
	}
	return c.Render(res, ids, func(w io.Writer) error {
		if !res.Removed {
			return nil
		}
		_, err := fmt.Fprintf(w, "Removed LeviLamina from %s\n", res.Instance)
		return err
	})
}
