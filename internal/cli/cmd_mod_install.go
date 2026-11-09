package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipindex"
)

func init() {
	Register(&Command{
		Path:    "mod install",
		Summary: "install, upgrade or downgrade mods from the catalogue",
		Args:    "<package[@version]>...",
		Description: `Installs mods from the LeviLamina mod catalogue into an instance (GDK only).
<package> is owner/repo, github.com/owner/repo or a URL, optionally with
#variant and @version. Without a variant, the one made for the client is
installed: "client", else the default variant, else the first by name that
has a build for the client on this platform ("owner/repo#" names the default
variant). Without a version, the newest one that runs on the instance's game
version is installed.

When the mod needs LeviLamina and the installed one does not fit (or none is
installed), a compatible LeviLamina is installed first: the newest stable
one, or --loader-version. Replacing LeviLamina, downgrading a mod or using a
LeviLamina release candidate asks for confirmation (--yes answers it).

Installing a mod that is present only as a dependency of another mod marks
it as installed explicitly. Installing the version already installed does
nothing unless --reinstall is given.`,
		Examples: []string{
			"mod install LiteLDev/LegacyMoney",
			"mod install LiteLDev/LegacyMoney@0.9.2 -i Survival --yes",
			"mod install LiteLDev/LegacyMoney --no-loader-change",
		},
		Flags: []*Flag{
			StringFlag("loader-version", "", "<version>", "LeviLamina version to install when the mod needs one"),
			BoolFlag("loader-change", "", "install or change LeviLamina when the mod needs it (--no-loader-change fails instead)").WithDefault("true"),
			BoolFlag("reinstall", "", "install again when that version is already installed"),
		},
		MinArgs: 1, MaxArgs: Unlimited,
		Run: runModInstall,
	})
	Register(&Command{
		Path:    "mod update",
		Summary: "update catalogue mods to their newest version",
		Args:    "[<mod>...]",
		Description: `Updates mods installed from the catalogue to the newest stable version the
catalogue offers. Name mods by package (owner/repo), folder or name, or
use --all for every outdated mod. Mods added from files that match a
catalogue package are updated through the package manager too.

The plan is shown and confirmed first (--yes answers it); --dry-run only
shows it.`,
		Examples: []string{"mod update --all", "mod update LiteLDev/LegacyMoney --yes", "mod update --all --dry-run --json"},
		Flags: []*Flag{
			BoolFlag("all", "", "update every outdated mod"),
			BoolFlag("dry-run", "n", "only show what would be updated"),
			BoolFlag("refresh", "", "fetch the mod catalogue again instead of using the cached copy"),
		},
		MaxArgs: Unlimited,
		Run:     runModUpdate,
	})
}

// modInstallStep is one planned package install.
type modInstallStep struct {
	ref     lipindex.Reference
	pkg     *lipindex.Package
	variant lipindex.Variant
	plan    lipindex.Plan
}

func (s modInstallStep) name() string {
	return modFirstNonEmpty(s.pkg.Name, lipShortID(s.pkg.Identifier))
}

func modPlanError(s modInstallStep, inst Instance) *Error {
	p := s.plan
	switch p.Blocker {
	case lipindex.BlockVersionNotFound:
		e := Errorf(ExitNotFound, "ERR_LIP_PACKAGE_VERSION_NOT_FOUND", "%s has no version %s", lipShortID(s.pkg.Identifier), p.Version)
		if len(s.variant.Versions) > 0 {
			return e.WithHint("%s mod show %s --versions", ProgramName, lipShortID(s.pkg.Identifier))
		}
		return e
	case lipindex.BlockLoaderDBUnavailable:
		return modBackendError("ERR_FETCH_LL_DB")
	case lipindex.BlockGameUnsupported:
		games := "none"
		if len(p.SupportedGames) > 0 {
			games = strings.Join(p.SupportedGames, ", ")
		}
		return Errorf(ExitUsage, "ERR_LIP_GAME_VERSION_UNSUPPORTED", "%s %s does not run on Minecraft %s (supported: %s)",
			s.name(), p.Version, Dash(inst.Meta.GameVersion), games).
			WithHint("%s mod show %s --versions", ProgramName, lipShortID(s.pkg.Identifier))
	case lipindex.BlockLoaderCheckFailed:
		return modBackendError("ERR_LIP_PACKAGE_QUERY_FAILED")
	case lipindex.BlockNoCompatibleLoader:
		return Errorf(ExitUsage, "ERR_LL_NO_COMPATIBLE_VERSION", "no LeviLamina release for Minecraft %s fits %s %s (it needs %s)",
			Dash(inst.Meta.GameVersion), s.name(), p.Version, strings.Join(p.File.LLRanges, " or "))
	case lipindex.BlockLoaderVersionInvalid:
		return Errorf(ExitNotFound, "ERR_LL_VERSION_UNSUPPORTED", "that LeviLamina version cannot run %s %s on Minecraft %s (choose from: %s)",
			s.name(), p.Version, Dash(inst.Meta.GameVersion), strings.Join(p.LoaderOptions, ", "))
	}
	return nil
}

func runModInstall(c *Ctx) error {
	refs := make([]lipindex.Reference, 0, len(c.Args))
	for _, a := range c.Args {
		r, ok := lipindex.ParseReference(a)
		if !ok {
			return modBackendError("ERR_LIP_PACKAGE_INVALID_IDENTIFIER").WithDetail(a)
		}
		if lipindex.IsLeviLamina(r.ID) {
			return modErrLoader()
		}
		refs = append(refs, r)
	}
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	if err := modRequireGDK(inst); err != nil {
		return err
	}
	var db lipindex.LoaderDB
	dbLoaded := false
	b := newModBatch(c, len(refs))
	for _, ref := range refs {
		if b.interrupted() {
			break
		}
		item := lipShortID(ref.ID)
		pkg, _, err := modFetchPackage(c, ref)
		if err != nil {
			b.fail(item, err)
			continue
		}
		item = lipShortID(pkg.Identifier)
		variant, err := modPickVariant(pkg, ref.Variant, ref.HasVariant)
		if err != nil {
			b.fail(item, err)
			continue
		}
		if !dbLoaded {
			dbLoaded = true
			if d, derr := modLoaderDB(c); derr == nil {
				db = d
			} else {
				c.Verbosef("%v", derr)
			}
		}
		step := modInstallStep{ref: ref, pkg: pkg, variant: variant}
		states, _ := modQueryStates(c, inst, []string{lipindex.LeviLaminaID, variant.PackageIdentifier})
		step.plan = lipindex.Decide(lipindex.PlanInput{
			Variant: variant, Version: ref.Version, GameVersion: inst.Meta.GameVersion, DB: db,
			Loader: states[lipindex.LookupKey(lipindex.LeviLaminaID)], Pkg: states[lipindex.LookupKey(pkg.Identifier)],
			LoaderVersion: c.String("loader-version"),
		})
		if step.plan.Blocker == lipindex.BlockLoaderVersionInvalid {
			b.fail(item, Errorf(ExitNotFound, "ERR_LL_VERSION_UNSUPPORTED", "LeviLamina %s does not fit %s %s on Minecraft %s (choose from: %s)",
				c.String("loader-version"), step.name(), step.plan.Version, Dash(inst.Meta.GameVersion), strings.Join(step.plan.LoaderOptions, ", ")))
			continue
		}
		if e := modPlanError(step, inst); e != nil {
			b.fail(item, e)
			continue
		}
		res, line, err := runInstallStep(c, inst, step)
		if err != nil {
			b.fail(item, err)
			continue
		}
		res.Item = item
		b.ok(res, item, line)
	}
	return b.done()
}

// runInstallStep confirms and runs one planned install.
func runInstallStep(c *Ctx, inst Instance, s modInstallStep) (modBatchResult, string, error) {
	p := s.plan
	id := lipShortID(s.pkg.Identifier)
	if p.Action == lipindex.ActionReinstall && !p.NeedsLoader && p.InstalledVersion != "" && !c.Bool("reinstall") {
		return modBatchResult{Status: "unchanged", To: p.Version},
			fmt.Sprintf("%s %s is already installed", s.name(), p.Version), nil
	}
	if p.NeedsLoader && !c.Bool("loader-change") {
		what := "install LeviLamina " + p.LoaderVersion
		if p.ReplacesLoader {
			what = fmt.Sprintf("change LeviLamina %s to %s", p.InstalledLoader, p.LoaderVersion)
		}
		return modBatchResult{}, "", ConflictError("%s %s needs to %s, and --no-loader-change was given", s.name(), p.Version, what).
			withCode("ERR_LOADER_CHANGE_REFUSED")
	}
	var plan []string
	var risks []string
	if p.NeedsLoader {
		if p.ReplacesLoader {
			plan = append(plan, fmt.Sprintf("change LeviLamina %s to %s", p.InstalledLoader, p.LoaderVersion))
			risks = append(risks, "other mods may stop working with the new LeviLamina")
		} else {
			plan = append(plan, "install LeviLamina "+p.LoaderVersion)
		}
		if lipindex.IsReleaseCandidate(p.LoaderVersion) {
			risks = append(risks, "LeviLamina "+p.LoaderVersion+" is a release candidate")
		}
	}
	verb := map[string]string{lipindex.ActionInstall: "install", lipindex.ActionUpgrade: "upgrade",
		lipindex.ActionDowngrade: "downgrade", lipindex.ActionReinstall: "reinstall", lipindex.ActionPromote: "install"}[p.Action]
	switch p.Action {
	case lipindex.ActionUpgrade, lipindex.ActionDowngrade:
		plan = append(plan, fmt.Sprintf("%s %s %s -> %s", verb, id, p.InstalledVersion, p.Version))
	default:
		plan = append(plan, fmt.Sprintf("%s %s %s", verb, id, p.Version))
	}
	if p.Action == lipindex.ActionDowngrade {
		risks = append(risks, "this is a downgrade")
	}
	summary := "Will " + strings.Join(plan, ", then ")
	if len(risks) > 0 {
		if err := c.Confirm(fmt.Sprintf("%s in %s (%s). Continue?", summary, inst.Name, strings.Join(risks, "; "))); err != nil {
			return modBatchResult{}, "", err
		}
	} else {
		c.Notice("%s in %s", summary, inst.Name)
	}
	prog := c.Progress()
	if p.NeedsLoader {
		prog.Phase("Installing LeviLamina %s", p.LoaderVersion)
		if r := modOps.installLoader(c.Context(), inst, inst.Meta.GameVersion, p.LoaderVersion); r != "" {
			prog.Done()
			if err := c.Context().Err(); err != nil {
				return modBatchResult{}, "", err
			}
			return modBatchResult{}, "", modBackendError(r)
		}
		prog.Done()
		c.Notice("Installed LeviLamina %s", p.LoaderVersion)
	}
	// An installed package is changed in place, whatever its variant, when
	// the reference names none; otherwise the variant chosen is installed
	// ("<id>#<variant>", "<id>#" for the default variant).
	target := s.variant.PackageIdentifier
	if !s.ref.HasVariant && p.Action != lipindex.ActionInstall {
		target = s.pkg.Identifier
	}
	prog.Phase("Installing %s %s", id, p.Version)
	r := modOps.installPackage(c.Context(), inst, target, p.Version)
	prog.Done()
	if r != "" {
		if err := c.Context().Err(); err != nil {
			return modBatchResult{}, "", err
		}
		return modBatchResult{}, "", modBackendError(r)
	}
	res := modBatchResult{Status: p.Action, From: p.InstalledVersion, To: p.Version}
	switch p.Action {
	case lipindex.ActionUpgrade:
		return res, fmt.Sprintf("upgraded %s %s -> %s", s.name(), p.InstalledVersion, p.Version), nil
	case lipindex.ActionDowngrade:
		return res, fmt.Sprintf("downgraded %s %s -> %s", s.name(), p.InstalledVersion, p.Version), nil
	case lipindex.ActionPromote:
		return res, fmt.Sprintf("installed %s %s (was a dependency only)", s.name(), p.Version), nil
	case lipindex.ActionReinstall:
		return res, fmt.Sprintf("reinstalled %s %s", s.name(), p.Version), nil
	}
	res.From = ""
	return res, fmt.Sprintf("installed %s %s", s.name(), p.Version), nil
}

// modUpdatePlanItem is one entry of `mod update --dry-run --json`.
type modUpdatePlanItem struct {
	Mod     string `json:"mod"`
	Package string `json:"package"`
	From    string `json:"from"`
	To      string `json:"to"`
	id      string
}

// modUpdateWouldDowngrade reports whether updating from to to goes back
// a version, e.g. from an installed pre-release newer than the latest
// stable release. Versions that are not semantic versions never count.
func modUpdateWouldDowngrade(from, to string) bool {
	if _, ok := lipindex.ParseSemver(from); !ok {
		return false
	}
	if _, ok := lipindex.ParseSemver(to); !ok {
		return false
	}
	return lipindex.CompareSemverLike(to, from) < 0
}

func runModUpdate(c *Ctx) error {
	if c.Bool("all") == (len(c.Args) > 0) {
		return UsageError("name the mods to update, or use --all").WithHint("%s mod update --all", ProgramName)
	}
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	if err := modRequireGDK(inst); err != nil {
		return err
	}
	v, err := modLoadView(c, inst, true, c.Bool("refresh"))
	if err != nil {
		return err
	}
	if v.catErr != nil {
		return v.catErr
	}
	if v.stateErr != "" {
		return modBackendError(v.stateErr)
	}
	type cand struct {
		label, id, from string
	}
	var cands []cand
	seen := map[string]bool{}
	add := func(label, id, from string) {
		k := lipindex.LookupKey(id)
		if seen[k] {
			return
		}
		seen[k] = true
		cands = append(cands, cand{label, id, from})
	}
	if c.Bool("all") {
		for _, g := range v.listing.Groups {
			if g.CanUpdate {
				add(g.PackageName, g.Identifier, g.InstalledVersion)
			}
		}
		for _, it := range v.listing.Items {
			if it.State.Source == lipindex.SourceUnique && it.State.CanUpdate {
				add(modFirstNonEmpty(it.Mod.Name, it.Folder), it.State.Identifier, it.Mod.Version)
			}
		}
	} else {
		targets, err := v.resolveMods(c, c.Args)
		if err != nil {
			return err
		}
		for _, t := range targets {
			switch {
			case t.group != nil && t.group.CanUpdate:
				add(t.group.PackageName, t.group.Identifier, t.group.InstalledVersion)
			case t.group != nil:
				c.Notice("%s is already the newest version (%s)", t.label(), Dash(t.group.TargetVersion))
			case t.item != nil && t.item.State.Source == lipindex.SourceUnique && t.item.State.CanUpdate:
				add(t.label(), t.item.State.Identifier, t.item.Mod.Version)
			case t.item != nil && t.item.State.Source == lipindex.SourceUnique:
				c.Notice("%s is already the newest version (%s)", t.label(), Dash(t.item.State.TargetVersion))
			case t.item != nil && t.item.State.Source == lipindex.SourceAmbiguous:
				c.Notice("%s matches several catalogue mods, so it cannot be updated automatically", t.label())
			default:
				c.Notice("%s was not installed from the mod catalogue; nothing to update", t.label())
			}
		}
	}
	// The catalogue may be cached; take the target versions fresh.
	plan := []modUpdatePlanItem{}
	held := 0
	for _, cd := range cands {
		to := ""
		if d, err := modOps.fetchDetail(c.Context(), cd.id); err == nil {
			if p, ok := lipindex.ParsePackage(d); ok {
				to = p.LatestVersion()
			}
		}
		if to == "" {
			if p, ok := v.index.Find(cd.id); ok {
				to = p.LatestVersion()
			}
		}
		if to == "" || lipindex.NormalizeSemverInput(strings.ToLower(to)) == lipindex.NormalizeSemverInput(strings.ToLower(cd.from)) {
			continue
		}
		if modUpdateWouldDowngrade(cd.from, to) {
			// A warning, so --json still says why the mod was not updated.
			c.Warn("%s %s is newer than the catalogue's latest stable %s; to downgrade, run: %s mod install %s@%s",
				cd.label, cd.from, to, ProgramName, lipShortID(cd.id), to)
			held++
			continue
		}
		plan = append(plan, modUpdatePlanItem{Mod: cd.label, Package: lipShortID(cd.id), From: cd.from, To: to, id: cd.id})
	}
	if len(plan) == 0 {
		if c.G.JSON && c.Bool("dry-run") {
			return c.JSON(plan)
		}
		if c.Bool("all") && held == 0 {
			c.Notice("Everything is up to date.")
		}
		if c.G.JSON {
			return c.JSON([]modBatchResult{})
		}
		return nil
	}
	showPlan := func(w io.Writer) error {
		rows := make([][]string, len(plan))
		for i, p := range plan {
			rows[i] = []string{p.Mod, p.Package, Dash(p.From) + " -> " + p.To}
		}
		return Table(w, []string{"MOD", "PACKAGE", "UPDATE"}, rows)
	}
	if c.Bool("dry-run") {
		ids := make([]string, len(plan))
		for i, p := range plan {
			ids[i] = p.Package
		}
		return c.Render(plan, ids, showPlan)
	}
	var buf strings.Builder
	_ = showPlan(&buf)
	c.Notice("%s", buf.String())
	n := "1 mod"
	if len(plan) > 1 {
		n = fmt.Sprintf("%d mods", len(plan))
	}
	if err := c.Confirm(fmt.Sprintf("Update %s in %s?", n, inst.Name)); err != nil {
		return err
	}
	b := newModBatch(c, len(plan))
	for _, p := range plan {
		if b.interrupted() {
			break
		}
		prog := c.Progress()
		prog.Phase("Updating %s to %s", p.Package, p.To)
		r := modOps.installPackage(c.Context(), inst, p.id, p.To)
		prog.Done()
		if r != "" {
			if err := c.Context().Err(); err != nil {
				b.fail(p.Package, err)
				break
			}
			b.fail(p.Package, modBackendError(r))
			continue
		}
		b.ok(modBatchResult{Item: p.Package, Status: "upgraded", From: p.From, To: p.To}, p.Package,
			fmt.Sprintf("updated %s %s -> %s", p.Mod, Dash(p.From), p.To))
	}
	return b.done()
}
