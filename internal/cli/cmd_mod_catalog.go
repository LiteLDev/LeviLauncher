package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipindex"
)

const modSearchPageSize = 20

// modSearchItem is one element of `mod search --json`.
type modSearchItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Author is the repository owner, the same as Owner.
	Author        string            `json:"author"`
	Owner         string            `json:"owner"`
	Tags          []string          `json:"tags"`
	License       string            `json:"license,omitempty"`
	LicenseFamily string            `json:"licenseFamily"`
	Deps          map[string]string `json:"deps"`
	GameVersions  []string          `json:"gameVersions"`
	Latest        string            `json:"latest"`
	Versions      []string          `json:"versions"`
	LoaderRanges  []string          `json:"loaderRanges"`
	Stars         float64           `json:"stars"`
	Updated       string            `json:"updated"`
	ProjectURL    string            `json:"projectUrl"`
}

func init() {
	Register(&Command{
		Path:    "mod search",
		Summary: "search the LeviLamina mod catalogue",
		Args:    "[<words>]",
		Description: `Searches the LeviLamina mod catalogue (levimc.org/mods) as the site does:
every word must appear in a mod's name, id, author, tags or description,
and the best matches come first. Only mods with a client build for this
computer's platform are listed.

Only mods that run on the default instance's game version are shown (the
instance from -i, $LEVILAUNCHER_INSTANCE or 'use'); --game picks another
game version and --any-version turns the filter off. --tag, --license,
--dep and --author narrow the list further (each takes a comma list and
can be repeated). The catalogue is cached for 30 minutes; --refresh fetches
it again.`,
		Examples: []string{"mod search", "mod search economy --any-version", "mod search --game 1.21.50 --sort updated",
			"mod search --tag library,-api --license permissive", "mod search --author LiteLDev --dep LiteLDev/LegacyRemoteCall"},
		Flags: []*Flag{
			StringFlag("game", "", "<version>", "only mods for this game version (\"latest\" or a prefix like 1.21.50)"),
			BoolFlag("any-version", "", "do not filter by game version"),
			StringFlag("loader-version", "", "<version>", "only mods that accept this LeviLamina version"),
			StringsFlag("tag", "", "<tag>", "only mods with every tag; -tag excludes one (featured tags take their aliases: money is economy)"),
			StringsFlag("license", "", "<license>", "only mods under any of these licenses: a family (permissive, weak-copyleft, copyleft, other, none) or an SPDX id"),
			StringsFlag("dep", "", "<package>", "only mods whose latest version depends on every one of these packages"),
			StringsFlag("author", "", "<owner>", "only mods of any of these authors (GitHub owners)"),
			EnumFlag("sort", "", lipindex.Sorts, "sort order (default relevance: the best matches, else the catalogue's order)"),
			IntFlag("limit", "", "<n>", "results per page").WithDefault("20"),
			IntFlag("page", "", "<n>", "page of results").WithDefault("1"),
			BoolFlag("refresh", "", "fetch the mod catalogue again instead of using the cached copy"),
		},
		MaxArgs: Unlimited,
		Run:     runModSearch,
	})
	Register(&Command{
		Path:    "mod show",
		Summary: "show a mod of the catalogue",
		Args:    "<package>",
		Description: `Shows a catalogue mod: description, author, project page, variants and
versions. <package> is owner/repo, github.com/owner/repo or a URL, with an
optional #variant to show another variant ("#" alone: the default one).

When an instance is known (-i, $LEVILAUNCHER_INSTANCE or 'use'), the
installed version there is shown too.`,
		Examples: []string{"mod show LiteLDev/LegacyMoney", "mod show LiteLDev/LegacyMoney --versions", "mod show LiteLDev/LegacyMoney --readme"},
		Flags: []*Flag{
			BoolFlag("versions", "", "list every version with the LeviLamina and game versions it supports"),
			BoolFlag("readme", "", "print the README (Markdown) instead"),
		},
		MinArgs: 1, MaxArgs: 1,
		Run: runModShow,
	})
}

// modListFlag reads a repeatable flag of comma lists ("a,b" = "a" "b").
func modListFlag(c *Ctx, name string) []string {
	var out []string
	for _, v := range c.Strings(name) {
		for _, x := range strings.Split(v, ",") {
			if x = strings.TrimSpace(x); x != "" {
				out = append(out, x)
			}
		}
	}
	return out
}

func runModSearch(c *Ctx) error {
	if c.Changed("game") && c.Bool("any-version") {
		return UsageError("--game and --any-version cannot be used together")
	}
	limit, page := c.Int("limit"), c.Int("page")
	if limit < 1 || page < 1 {
		return UsageError("--limit and --page must be at least 1")
	}
	licenses := modListFlag(c, "license")
	for _, l := range licenses {
		if strings.EqualFold(l, "oss") {
			return UsageError("--license oss is gone; use a family (permissive, weak-copyleft, copyleft, other, none) or an SPDX id")
		}
	}
	deps := modListFlag(c, "dep")
	for i, d := range deps {
		if r, ok := lipindex.ParseReference(d); ok {
			deps[i] = r.ID
		}
	}
	idx, err := modLoadCatalog(c, c.Bool("refresh"))
	if err != nil {
		return modCatalogError(err)
	}
	options := lipindex.Options(lipindex.Visible(idx.Packages))
	// Families in any case, SPDX ids as some mod spells them (the site's license=).
	licenses, unknownLicenses := lipindex.LicenseValues(licenses, options)
	if len(unknownLicenses) > 0 {
		return UsageError("no mod of the catalogue is under license %q; use a family (permissive, weak-copyleft, copyleft, other, none) or an SPDX id a mod has", unknownLicenses[0])
	}
	q := lipindex.Query{Text: strings.Join(c.Args, " "), Sort: c.String("sort"), LoaderVersion: c.String("loader-version"),
		Tags: lipindex.TagFacet(modListFlag(c, "tag")), License: licenses, Deps: deps, Authors: modListFlag(c, "author")}
	if q.Sort == "" {
		q.Sort = lipindex.SortRelevance
	}
	if q.LoaderVersion != "" {
		db, derr := modLoaderDB(c)
		if derr != nil {
			return derr
		}
		q.DB = db
	}
	// The game version filter reads the catalogue's game versions.
	if !c.Bool("any-version") {
		games := options.GameVersions
		if c.Changed("game") {
			g, ok := lipindex.ResolveGameVersion(games, c.String("game"))
			if !ok {
				return Errorf(ExitNotFound, "ERR_LIP_GAME_VERSION_UNSUPPORTED", "no mod of the catalogue runs on Minecraft %s", c.String("game")).
					WithHint("%s mod search --any-version", ProgramName)
			}
			if g != lipindex.NormalizeSemverInput(c.String("game")) {
				c.Notice("Using Minecraft %s", g)
			}
			q.GameVersions = []string{g}
		} else if inst, ok, ierr := modSoftInstance(c); ierr != nil {
			return ierr
		} else if ok {
			g := lipindex.CatalogGameVersion(inst.Meta.GameVersion)
			if slices.Contains(games, g) {
				q.GameVersions = []string{g}
				c.Notice("Showing mods for Minecraft %s (instance %s); add --any-version to see all", g, inst.Name)
			} else {
				c.Warn("no mod of the catalogue runs on Minecraft %s (instance %s); showing mods for every game version", Dash(g), inst.Name)
			}
		}
	}
	found := lipindex.Search(idx.Packages, q)
	total := len(found)
	start := (page - 1) * limit
	pageItems := []*lipindex.Package{}
	if start < total {
		pageItems = found[start:min(start+limit, total)]
	}
	items := make([]modSearchItem, len(pageItems))
	ids := make([]string, len(pageItems))
	for i, p := range pageItems {
		items[i] = modSearchItem{ID: lipShortID(p.Identifier), Name: p.Name, Description: p.Description, Author: p.Owner,
			Owner: p.Owner, Tags: p.Tags, License: p.License, LicenseFamily: p.LicenseFamily, Deps: p.Deps,
			GameVersions: p.GameVersions, Latest: p.LatestVersion(), Versions: p.Versions, LoaderRanges: p.LLRanges,
			Stars: p.Stars, Updated: p.Updated, ProjectURL: p.ProjectURL}
		ids[i] = items[i].ID
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if total == 0 {
			c.Notice("No mods found.")
			return nil
		}
		if len(items) == 0 {
			c.Notice("Page %d is past the end (%d mods, %d per page).", page, total, limit)
			return nil
		}
		rows := make([][]string, len(items))
		for i, it := range items {
			rows[i] = []string{it.ID, it.Name, Dash(it.Latest), fmt.Sprintf("%.0f", it.Stars), modDate(it.Updated), modTruncate(it.Description, 60)}
		}
		if err := Table(w, []string{"PACKAGE", "NAME", "LATEST", "STARS", "UPDATED", "DESCRIPTION"}, rows); err != nil {
			return err
		}
		pages := (total + limit - 1) / limit
		if pages > 1 {
			c.Notice("Page %d of %d (%d mods); next: --page %d", page, pages, total, min(page+1, pages))
		}
		c.Notice("Install with: %s mod install <PACKAGE>", ProgramName)
		return nil
	})
}

// modDate shows the date of an RFC 3339 time.
func modDate(s string) string {
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(s)); err == nil {
		return t.Local().Format("2006-01-02")
	}
	return Dash(s)
}

func modTruncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// modShowResult is `mod show --json`.
type modShowResult struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Author      string           `json:"author"`
	Owner       string           `json:"owner"`
	OwnerURL    string           `json:"ownerUrl"`
	ProjectURL  string           `json:"projectUrl"`
	License     string           `json:"license"`
	// LicenseFamily is the license's family (permissive, weak-copyleft,
	// copyleft, other, none).
	LicenseFamily string   `json:"licenseFamily"`
	Tags          []string `json:"tags"`
	Stars       float64          `json:"stars"`
	Updated     string           `json:"updated"`
	Variant     string           `json:"variant"`
	Variants    []string         `json:"variants"`
	Latest      string           `json:"latest"`
	Versions    []modVersionInfo `json:"versions"`
	Instance    *modShowInstance `json:"instance"`
}

type modVersionInfo struct {
	Version      string            `json:"version"`
	LoaderRanges []string          `json:"loaderRanges"`
	GameVersions []string          `json:"gameVersions"`
	Dependencies map[string]string `json:"dependencies"`
}

type modShowInstance struct {
	Name             string `json:"name"`
	GameVersion      string `json:"gameVersion"`
	Installed        bool   `json:"installed"`
	Explicit         bool   `json:"explicit"`
	InstalledVersion string `json:"installedVersion"`
}

// modVariantName shows a variant name: "" is the default variant.
func modVariantName(key string) string {
	if key == "" {
		return `"" (default)`
	}
	return key
}

// modPickVariant returns the variant a reference names (one with an entry
// for the client on this platform), else the preferred one (rule A2:
// client, then the default "", then by name). named is set when the
// reference names a variant ("owner/repo#" names the default "").
func modPickVariant(pkg *lipindex.Package, key string, named bool) (lipindex.Variant, error) {
	if !named && strings.TrimSpace(key) == "" {
		v, ok := pkg.DefaultVariant()
		if !ok {
			return v, modNotClientMod(pkg.Identifier)
		}
		return v, nil
	}
	if v, ok := pkg.FindVariant(key); ok {
		return v, nil
	}
	var keys []string
	for _, v := range pkg.Variants {
		keys = append(keys, modVariantName(v.Key))
	}
	return lipindex.Variant{}, Errorf(ExitNotFound, "ERR_LIP_VARIANT_NOT_FOUND", "%s has no variant %s for the client on %s (variants: %s)",
		lipShortID(pkg.Identifier), modVariantName(key), lipindex.TargetPlatform, strings.Join(keys, ", "))
}

func runModShow(c *Ctx) error {
	ref, ok := lipindex.ParseReference(c.Arg(0))
	if !ok {
		return modBackendError("ERR_LIP_PACKAGE_INVALID_IDENTIFIER")
	}
	pkg, docs, err := modFetchPackage(c, ref)
	if err != nil {
		return err
	}
	if c.Bool("readme") {
		return modShowReadme(c, pkg, docs)
	}
	variant, err := modPickVariant(pkg, ref.Variant, ref.HasVariant)
	if err != nil {
		return err
	}
	var db lipindex.LoaderDB
	if d, derr := modLoaderDB(c); derr != nil {
		c.Warn("%s; supported game versions are not shown", FromError(derr).Message)
	} else {
		db = d
	}
	res := modShowResult{ID: lipShortID(pkg.Identifier), Name: pkg.Name, Description: modFirstNonEmpty(pkg.FullDescription, pkg.Description), Author: pkg.Owner,
		Owner: pkg.Owner, OwnerURL: lipindex.OwnerURL(pkg.Owner), ProjectURL: pkg.ProjectURL, License: pkg.License,
		LicenseFamily: pkg.LicenseFamily, Tags: pkg.Tags, Stars: pkg.Stars, Updated: pkg.Updated,
		Variant: variant.Key, Variants: []string{}, Latest: lipindex.PickTargetVersion(variant.Versions), Versions: []modVersionInfo{}}
	for _, v := range pkg.Variants {
		res.Variants = append(res.Variants, v.Key)
	}
	for _, f := range lipindex.SortedFiles(variant) {
		res.Versions = append(res.Versions, modVersionInfo{Version: f.Version, LoaderRanges: f.LLRanges,
			GameVersions: lipindex.SupportedGameVersions(f.LLRanges, db), Dependencies: f.Dependencies})
	}
	if inst, ok, ierr := modSoftInstance(c); ierr != nil {
		return ierr
	} else if ok && !inst.IsUWP() {
		states, _ := modQueryStates(c, inst, []string{pkg.Identifier})
		st := states[lipindex.LookupKey(pkg.Identifier)]
		res.Instance = &modShowInstance{Name: inst.Name, GameVersion: inst.Meta.GameVersion, Installed: st.Installed,
			Explicit: st.Explicit, InstalledVersion: st.InstalledVersion}
	}
	ids := []string{res.ID}
	return c.Render(res, ids, func(w io.Writer) error {
		latestGames := "-"
		if len(res.Versions) > 0 {
			for _, v := range res.Versions {
				if v.Version == res.Latest {
					latestGames = modGameList(v.GameVersions, v.LoaderRanges)
				}
			}
		}
		variants := make([]string, len(res.Variants))
		for i, v := range res.Variants {
			variants[i] = modVariantName(v)
		}
		license := res.LicenseFamily
		if res.License != "" {
			license += " (" + res.License + ")"
		}
		author := Dash(res.Owner)
		if res.OwnerURL != "" {
			author += " (" + res.OwnerURL + ")"
		}
		pairs := []string{
			"Package", res.ID,
			"Name", res.Name,
			"Author", author,
			"Description", Dash(res.Description),
			"Project", Dash(res.ProjectURL),
			"License", license,
			"Tags", Dash(strings.Join(modShownTags(res.Tags), ", ")),
			"Stars", fmt.Sprintf("%.0f", res.Stars),
			"Updated", modDate(res.Updated),
			"Variants", strings.Join(variants, ", "),
			"Latest", Dash(res.Latest),
			"Runs on", latestGames,
		}
		if res.Instance != nil {
			state := "not installed"
			switch {
			case res.Instance.Installed && res.Instance.Explicit:
				state = res.Instance.InstalledVersion
			case res.Instance.Installed:
				state = res.Instance.InstalledVersion + " (as a dependency)"
			}
			pairs = append(pairs, "Installed in "+res.Instance.Name, state)
		}
		if err := KV(w, pairs...); err != nil {
			return err
		}
		if c.Bool("versions") {
			fmt.Fprintln(w)
			rows := make([][]string, len(res.Versions))
			for i, v := range res.Versions {
				rows[i] = []string{v.Version, Dash(strings.Join(v.LoaderRanges, " | ")), modGameList(v.GameVersions, v.LoaderRanges)}
			}
			return Table(w, []string{"VERSION", "LEVILAMINA", "GAME VERSIONS"}, rows)
		}
		c.Notice("Install with: %s mod install %s", ProgramName, res.ID)
		return nil
	})
}

// modShownTags are a mod's tags as the catalogue shows them: the featured
// tags first (with the raw tags behind each), then the other visible ones.
func modShownTags(tags []string) []string {
	out := []string{}
	for _, f := range lipindex.FeaturedTagsOf(tags) {
		out = append(out, f.Tag.ID+" ["+strings.Join(f.Raw, " ")+"]")
	}
	return append(out, lipindex.PlainTagsOf(tags)...)
}

func modGameList(games, ranges []string) string {
	switch {
	case len(ranges) == 0:
		return "any (no LeviLamina needed)"
	case len(games) == 0:
		return "-"
	case len(games) > 4:
		return strings.Join(games[:4], ", ") + fmt.Sprintf(" (+%d)", len(games)-4)
	}
	return strings.Join(games, ", ")
}

// showReadme prints the README the catalogue extracted, else the one in
// the repository (as the window's README tab).
func modShowReadme(c *Ctx, pkg *lipindex.Package, docs []lipindex.Readme) error {
	text := ""
	for _, d := range docs {
		if strings.EqualFold(d.File, "README.md") && strings.TrimSpace(d.Text) != "" {
			text = d.Text
			break
		}
	}
	if text == "" && len(docs) > 0 {
		text = docs[0].Text
	}
	if strings.TrimSpace(text) == "" {
		t, err := modOps.readme(pkg.ProjectURL)
		if err != nil {
			return Errorf(ExitNetwork, "ERR_LIP_README_UNAVAILABLE", "cannot download the README of %s", lipShortID(pkg.Identifier)).
				WithDetail(err.Error()).WithHint("open %s", modFirstNonEmpty(pkg.ProjectURL, pkg.Identifier))
		}
		text = t
	}
	if strings.TrimSpace(text) == "" {
		return Errorf(ExitNotFound, "ERR_LIP_README_NOT_FOUND", "%s has no README", lipShortID(pkg.Identifier))
	}
	if c.G.JSON {
		return c.JSON(struct {
			ID     string `json:"id"`
			Readme string `json:"readme"`
		}{lipShortID(pkg.Identifier), text})
	}
	c.Printf("%s", text)
	if !strings.HasSuffix(text, "\n") {
		c.Printf("\n")
	}
	return nil
}
