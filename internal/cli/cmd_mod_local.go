package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipindex"
)

func init() {
	Register(&Command{
		Path:    "mod import",
		Summary: "add mods from .mcmod, .zip or .dll files",
		Args:    "<file>...",
		Description: `Adds mods to an instance from files: a mod package (.mcmod or .zip with a
manifest.json) or a single .dll. For a .dll, --name, --type and --mod-version
fill in its manifest (defaults: the file name, preload-native and 0.0.0);
they need exactly one .dll.

When a mod folder of that name exists, --overwrite replaces it and
--skip-existing keeps it; otherwise you are asked on a console, and without
one the file fails with exit 4.`,
		Examples: []string{
			`mod import C:\Downloads\BetterChat.mcmod`,
			`mod import Tweaks.dll --name Tweaks --mod-version 1.2.0`,
			`mod import *.zip -i Survival --skip-existing`,
		},
		Flags: []*Flag{
			StringFlag("name", "", "<name>", "mod name (and folder) for a .dll"),
			StringFlag("type", "", "<type>", "mod type for a .dll").WithDefault("preload-native"),
			StringFlag("mod-version", "", "<version>", "mod version for a .dll").WithDefault("0.0.0"),
			BoolFlag("overwrite", "", "replace a mod folder that already exists"),
			BoolFlag("skip-existing", "", "keep a mod folder that already exists"),
		},
		MinArgs: 1, MaxArgs: Unlimited,
		Run: runModImport,
	})
	Register(&Command{
		Path:    "mod remove",
		Summary: "remove mods from an instance",
		Args:    "<mod>...",
		Description: `Removes mods. A mod installed from the catalogue is uninstalled through the
package manager, which also removes dependencies nothing else needs; a mod
added from a file has its folder deleted. Name mods by folder, name or
package (owner/repo).

A mod other mods depend on is not removed (exit 4). Asks for confirmation;
--yes answers it.`,
		Examples: []string{"mod remove LiteLDev/LegacyMoney", "mod rm BetterChat --yes", "mod remove BetterChat Tweaks -i Survival -y"},
		MinArgs:  1, MaxArgs: Unlimited,
		Run: runModRemove,
	})
	for _, on := range []bool{true, false} {
		verb, what := "enable", "turn mods on"
		if !on {
			verb, what = "disable", "turn mods off without removing them"
		}
		enable := on
		Register(&Command{
			Path:    "mod " + verb,
			Summary: what,
			Args:    "<mod>... | --all",
			Description: fmt.Sprintf(`Turns mods %s. Name mods by folder, name or package (owner/repo); a
package turns all of its folders %s. --all does every mod of the instance
(LeviLamina is managed with 'loader').`, map[bool]string{true: "on", false: "off"}[on], map[bool]string{true: "on", false: "off"}[on]),
			Examples: []string{"mod " + verb + " BetterChat", "mod " + verb + " LiteLDev/LegacyMoney -i Survival", "mod " + verb + " --all"},
			Flags:    []*Flag{BoolFlag("all", "", "every mod of the instance")},
			MaxArgs:  Unlimited,
			Run:      func(c *Ctx) error { return runModToggle(c, enable) },
		})
	}
	Register(&Command{
		Path:    "mod set",
		Summary: "change a mod's manifest",
		Args:    "<mod>",
		Description: `Changes fields of a mod's manifest.json. Only the fields you give change;
--author "" removes the author. The folder is not renamed.`,
		Examples: []string{`mod set BetterChat --mod-version 1.1.0`, `mod set Tweaks --name "Better Tweaks" --type preload-native`, `mod set Tweaks --author ""`},
		Flags: []*Flag{
			StringFlag("name", "", "<name>", "new name"),
			StringFlag("mod-version", "", "<version>", "new version"),
			StringFlag("type", "", "<type>", "new type"),
			StringFlag("entry", "", "<file>", "new entry file"),
			StringFlag("author", "", "<author>", `new author ("" removes it)`),
		},
		MinArgs: 1, MaxArgs: 1,
		Run: runModSet,
	})
	Register(&Command{
		Path:    "mod path",
		Summary: "print the folder of a mod or of the instance's mods",
		Args:    "[<mod>]",
		Description: `Prints the folder of a mod, or the instance's mods folder when no mod is
named. --open also opens it in Explorer.`,
		Examples: []string{"mod path", "mod path BetterChat", "mod path -i Survival --open"},
		Flags:    []*Flag{BoolFlag("open", "", "also open the folder in Explorer")},
		MaxArgs:  1,
		Run:      runModPath,
	})
}

// modImportKind classifies a file by extension.
func modImportKind(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mcmod", ".zip":
		return "package", nil
	case ".dll":
		return "dll", nil
	case ".mcpack", ".mcaddon":
		return "", UsageError("%s is a resource or behavior pack, not a mod", filepath.Base(path)).
			WithHint("%s pack import %s", ProgramName, quoteArg(path)).withCode("ERR_UNSUPPORTED_MOD_FILE")
	case ".mcworld":
		return "", UsageError("%s is a world, not a mod", filepath.Base(path)).
			WithHint("%s world import %s", ProgramName, quoteArg(path)).withCode("ERR_UNSUPPORTED_MOD_FILE")
	}
	return "", UsageError("%s is not a mod file (expected .mcmod, .zip or .dll)", filepath.Base(path)).withCode("ERR_UNSUPPORTED_MOD_FILE")
}

func runModImport(c *Ctx) error {
	if c.Bool("overwrite") && c.Bool("skip-existing") {
		return UsageError("--overwrite and --skip-existing cannot be used together")
	}
	kinds := make([]string, len(c.Args))
	dlls := 0
	for i, a := range c.Args {
		k, err := modImportKind(a)
		if err != nil {
			return err
		}
		kinds[i] = k
		if k == "dll" {
			dlls++
		}
	}
	if (c.Changed("name") || c.Changed("type") || c.Changed("mod-version")) && (dlls != 1 || len(c.Args) != 1) {
		return UsageError("--name, --type and --mod-version describe a single .dll; import other files separately")
	}
	// An empty --name falls back to the file name, as without it.
	if name := strings.TrimSpace(c.String("name")); name != "" {
		if err := checkModFolderName(name); err != nil {
			return err
		}
	}
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	b := newModBatch(c, len(c.Args))
	for i, path := range c.Args {
		if b.interrupted() {
			break
		}
		label := filepath.Base(path)
		dllName := strings.TrimSpace(c.String("name"))
		if kinds[i] == "dll" && dllName == "" {
			dllName = strings.TrimSpace(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
			if err := checkModFolderName(dllName); err != nil {
				b.fail(label, err.WithHint("%s mod import %s --name <name>", ProgramName, quoteArg(path)))
				continue
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				b.fail(label, NotFoundError("no file %s", path).withCode("ERR_FILE_NOT_FOUND"))
			} else {
				b.fail(label, Errorf(ExitFailure, "ERR_READ_FILE", "cannot read %s: %v", path, err))
			}
			continue
		}
		run := func(overwrite bool) string {
			if kinds[i] == "dll" {
				return modOps.importDll(inst, filepath.Base(path), data, dllName, c.String("type"), c.String("mod-version"), overwrite)
			}
			return modOps.importZip(inst, data, overwrite)
		}
		r := run(c.Bool("overwrite"))
		status := "imported"
		if code, _, _ := SplitBackendCode(r); code == "ERR_DUPLICATE_FOLDER" {
			switch {
			case c.Bool("skip-existing"):
				b.ok(modBatchResult{Item: label, Status: "skipped"}, label, "skipped "+label+" (already installed)")
				continue
			case !c.CanPrompt() && !c.G.Yes:
				b.fail(label, modBackendError(r))
				continue
			}
			if err := c.ConfirmWith(fmt.Sprintf("A mod folder for %s already exists in %s. Overwrite it?", label, inst.Name), "--overwrite"); err != nil {
				if FromError(err).Exit == ExitCancelled {
					b.ok(modBatchResult{Item: label, Status: "skipped"}, label, "skipped "+label)
					continue
				}
				b.fail(label, err)
				continue
			}
			r, status = run(true), "overwritten"
		}
		if e := modBackendError(r); e != nil {
			b.fail(label, e)
			continue
		}
		b.ok(modBatchResult{Item: label, Status: status}, label, status+" "+label)
	}
	return b.done()
}

// checkModFolderName rejects a .dll mod name that is not one plain folder
// name, so the mod cannot land outside (or nested inside) the mods folder.
func checkModFolderName(name string) *Error {
	bad := name == "" || strings.ContainsAny(name, `<>:"/\|?*`) ||
		strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ")
	for _, r := range name {
		if r < 32 {
			bad = true
		}
	}
	if bad {
		return UsageError(`the mod name %q must be one folder name without < > : " / \ | ? * or a trailing dot or space`, name).
			withCode("ERR_INVALID_NAME")
	}
	return nil
}

// modRemovePlan is how one mod is removed.
type modRemovePlan struct {
	target    modTarget
	packageID string // uninstall through the package manager
}

func runModRemove(c *Ctx) error {
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	v, err := modLoadView(c, inst, !inst.IsUWP(), false)
	if err != nil {
		return err
	}
	targets, err := v.resolveMods(c, c.Args)
	if err != nil {
		return err
	}
	if !inst.IsUWP() && (v.catErr != nil || v.stateErr != "") {
		// Without the catalogue a mod from it would look like a local one and
		// lose only its folder; refuse unless the instance has no catalogue
		// mods at all.
		if err := modRemoveOfflineGuard(c, v); err != nil {
			return err
		}
	}
	var plans []modRemovePlan
	seen := map[string]bool{}
	for _, t := range targets {
		key := strings.ToLower(strings.Join(t.folders, "|"))
		pid := ""
		switch {
		case t.group != nil:
			pid = t.group.Identifier
			key = lipindex.LookupKey(pid)
		case t.packageID != "" && len(t.folders) == 0:
			pid = t.packageID
			key = lipindex.LookupKey(pid)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		plans = append(plans, modRemovePlan{target: t, packageID: pid})
	}
	var lines []string
	for _, p := range plans {
		switch {
		case p.packageID != "":
			lines = append(lines, fmt.Sprintf("%s (uninstall it and its unused dependencies)", p.target.label()))
		default:
			lines = append(lines, fmt.Sprintf("%s (delete its folder)", p.target.label()))
		}
	}
	if err := c.Confirm(fmt.Sprintf("Remove from %s: %s?", inst.Name, strings.Join(lines, ", "))); err != nil {
		return err
	}
	b := newModBatch(c, len(plans))
	for _, p := range plans {
		if b.interrupted() {
			break
		}
		item := p.target.label()
		res, line, err := modRemoveOne(c, inst, p)
		if err != nil {
			b.fail(item, err)
			continue
		}
		res.Item = item
		b.ok(res, item, line)
	}
	return b.done()
}

func modRemoveOne(c *Ctx, inst Instance, p modRemovePlan) (modBatchResult, string, error) {
	item := p.target.label()
	if p.packageID == "" {
		for _, f := range p.target.folders {
			if r := modOps.remove(inst, f); r != "" {
				return modBatchResult{}, "", modBackendError(r)
			}
		}
		return modBatchResult{Status: "removed"}, "removed " + item, nil
	}
	if p.target.group != nil && !p.target.group.Explicit {
		st, _ := modQueryStates(c, inst, []string{p.packageID})
		if s := st[lipindex.LookupKey(p.packageID)]; s.Installed && !s.Explicit {
			return modBatchResult{}, "", modBackendError("ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS").
				WithHint("remove the mods that depend on %s first (see %s mod list)", lipShortID(p.packageID), ProgramName)
		}
	}
	prog := c.Progress()
	prog.Phase("Removing %s", lipShortID(p.packageID))
	r := modOps.uninstallPackage(c.Context(), inst, p.packageID)
	prog.Done()
	code, _, _ := SplitBackendCode(r)
	switch {
	case r == "":
		return modBatchResult{Status: "removed"}, "removed " + item, nil
	case code == "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY":
		c.Warn("%s is still needed by other mods, so it stays installed as a dependency", item)
		return modBatchResult{Status: "kept-as-dependency"}, "kept " + item + " as a dependency of other mods", nil
	case code == "ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS":
		return modBatchResult{}, "", modBackendError(r).WithHint("remove the mods that depend on %s first", lipShortID(p.packageID))
	}
	if err := c.Context().Err(); err != nil {
		return modBatchResult{}, "", err
	}
	return modBatchResult{}, "", modBackendError(r)
}

// modRemoveOfflineGuard allows removing folders without the catalogue only
// when no package other than LeviLamina was installed explicitly.
func modRemoveOfflineGuard(c *Ctx, v *modView) error {
	states, err := modOps.listPackages(c.Context(), v.inst)
	if err == nil {
		explicit := false
		for _, s := range states {
			if s.ExplicitInstalled && !lipindex.IsLeviLamina(s.ID) {
				explicit = true
			}
		}
		if !explicit {
			return nil
		}
	}
	if v.catErr != nil {
		return v.catErr.WithHint("removing needs the mod catalogue to tell catalogue mods from local ones; run the command again when online")
	}
	return modBackendError(v.stateErr)
}

func runModToggle(c *Ctx, enable bool) error {
	if c.Bool("all") == (len(c.Args) > 0) {
		return UsageError("name the mods, or use --all")
	}
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	v, err := modLoadView(c, inst, false, false)
	if err != nil {
		return err
	}
	var targets []modTarget
	if c.Bool("all") {
		for _, m := range v.visible {
			targets = append(targets, modTarget{ref: m.Folder, folders: []string{m.Folder}, mods: []installedMod{m}})
		}
		if len(targets) == 0 {
			c.Notice("No mods in %s.", inst.Name)
			if c.G.JSON {
				return c.JSON([]modBatchResult{})
			}
			return nil
		}
	} else if targets, err = v.resolveMods(c, c.Args); err != nil {
		return err
	}
	past, op := "enabled", modOps.enable
	if !enable {
		past, op = "disabled", modOps.disable
	}
	b := newModBatch(c, len(targets))
	for _, t := range targets {
		item := t.label()
		changed := false
		var failure error
		for _, f := range t.folders {
			if v.enabled[f] == enable {
				continue
			}
			if r := op(inst, f); r != "" {
				failure = modBackendError(r)
				break
			}
			changed = true
		}
		switch {
		case failure != nil:
			b.fail(item, failure)
		case changed:
			b.ok(modBatchResult{Item: item, Status: past}, t.ref, past+" "+item)
		default:
			b.ok(modBatchResult{Item: item, Status: "unchanged"}, t.ref, item+" is already "+past)
		}
	}
	return b.done()
}

func runModSet(c *Ctx) error {
	fields := []string{"name", "version", "type", "entry", "author"}
	flagOf := func(field string) string {
		if field == "version" {
			return "mod-version" // --version is the global flag
		}
		return field
	}
	given := false
	for _, f := range fields {
		if c.Changed(flagOf(f)) {
			given = true
		}
	}
	if !given {
		return UsageError("nothing to change: give at least one of --name, --mod-version, --type, --entry or --author")
	}
	if c.Changed("name") && strings.TrimSpace(c.String("name")) == "" {
		return UsageError("--name cannot be empty")
	}
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	v, err := modLoadView(c, inst, false, false)
	if err != nil {
		return err
	}
	ts, err := v.resolveMods(c, c.Args)
	if err != nil {
		return err
	}
	t := ts[0]
	if len(t.mods) != 1 {
		return UsageError("%s has %d folders; name one folder", t.label(), len(t.folders))
	}
	m := t.mods[0]
	old := map[string]string{"name": m.Name, "version": m.Version, "type": m.Type, "entry": m.Entry, "author": m.Author}
	nw := map[string]string{}
	for _, f := range fields {
		nw[f] = strings.TrimSpace(old[f])
		if c.Changed(flagOf(f)) {
			nw[f] = strings.TrimSpace(c.String(flagOf(f)))
		}
	}
	type change struct {
		Field string `json:"field"`
		From  string `json:"from"`
		To    string `json:"to"`
	}
	changes := []change{}
	for _, f := range fields {
		if nw[f] != strings.TrimSpace(old[f]) {
			changes = append(changes, change{f, old[f], nw[f]})
		}
	}
	if len(changes) > 0 {
		if r := modOps.setManifest(inst, m.Folder, nw["name"], nw["version"], nw["type"], nw["entry"], nw["author"]); r != "" {
			return modBackendError(r)
		}
	}
	return c.Render(changes, []string{m.Folder}, func(w io.Writer) error {
		if len(changes) == 0 {
			c.Notice("Nothing changed: %s already has those values.", t.label())
			return nil
		}
		for _, ch := range changes {
			fmt.Fprintf(w, "%s: %s -> %s\n", ch.Field, modQuoteValue(ch.From), modQuoteValue(ch.To))
		}
		return nil
	})
}

func modQuoteValue(s string) string {
	if s == "" {
		return `""`
	}
	return s
}

func runModPath(c *Ctx) error {
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	dir := modsDir(inst)
	if c.Arg(0) != "" {
		v, err := modLoadView(c, inst, false, false)
		if err != nil {
			return err
		}
		ts, err := v.resolveMods(c, c.Args)
		if err != nil {
			return err
		}
		if len(ts[0].folders) != 1 {
			return UsageError("%s has %d folders: %s", ts[0].label(), len(ts[0].folders), strings.Join(ts[0].folders, ", "))
		}
		dir = filepath.Join(dir, ts[0].folders[0])
	}
	if c.G.JSON {
		if err := c.JSON(struct {
			Path string `json:"path"`
		}{dir}); err != nil {
			return err
		}
	} else {
		c.Printf("%s\n", dir)
	}
	if c.Bool("open") {
		if _, err := os.Stat(dir); err != nil {
			return NotFoundError("the folder %s does not exist yet", dir).WithHint("%s mod import <file>", ProgramName)
		}
		if err := modOps.openFolder(dir); err != nil {
			return Errorf(ExitFailure, "ERR_OPEN_FOLDER", "cannot open %s: %v", dir, err)
		}
	}
	return nil
}
