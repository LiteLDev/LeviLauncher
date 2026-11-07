package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipindex"
)

const modSearchPageSize = 20

// modSearchItem is one element of `mod search --json`.
type modSearchItem struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Author       string   `json:"author"`
	Tags         []string `json:"tags"`
	Latest       string   `json:"latest"`
	Versions     []string `json:"versions"`
	LoaderRanges []string `json:"loaderRanges"`
	Stars        float64  `json:"stars"`
	Updated      string   `json:"updated"`
	ProjectURL   string   `json:"projectUrl"`
}

func init() {
	Register(&Command{
		Path:    "mod search",
		Summary: "search the LeviLamina mod catalogue",
		Args:    "[<words>]",
		Description: `Searches the client mods of the LeviLamina mod catalogue (levimc.org) by
id, name, description, author and tags.

Only mods that run on the default instance's game version are shown (the
instance from -i, $LEVILAUNCHER_INSTANCE or 'use'); --game picks another
game version and --any-version turns the filter off. The catalogue is
cached for 30 minutes; --refresh fetches it again.`,
		Examples: []string{"mod search", "mod search economy --any-version", "mod search --game 1.21.50 --sort updated"},
		Flags: []*Flag{
			StringFlag("game", "", "<version>", "only mods for this game version (\"latest\" or a prefix like 1.21.50)"),
			BoolFlag("any-version", "", "do not filter by game version"),
			StringFlag("loader-version", "", "<version>", "only mods that accept this LeviLamina version"),
			EnumFlag("sort", "", []string{"stars", "updated", "name"}, "sort order (default stars, most first)"),
			BoolFlag("reverse", "", "reverse the sort order"),
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
optional #variant to show another client variant.

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

// modResolveGame matches a game version against the LeviLamina database:
// "latest", an exact version, or a dotted prefix on segment boundaries (the
// newest match).
func modResolveGame(db lipindex.LoaderDB, want string) (string, bool) {
	return db.ResolveGame(want)
}

func runModSearch(c *Ctx) error {
	if c.Changed("game") && c.Bool("any-version") {
		return UsageError("--game and --any-version cannot be used together")
	}
	limit, page := c.Int("limit"), c.Int("page")
	if limit < 1 || page < 1 {
		return UsageError("--limit and --page must be at least 1")
	}
	idx, err := modLoadCatalog(c, c.Bool("refresh"))
	if err != nil {
		return modCatalogError(err)
	}
	q := lipindex.Query{Text: strings.Join(c.Args, " "), Sort: c.String("sort"), LoaderVersion: c.String("loader-version")}
	if q.Sort == "" {
		q.Sort = lipindex.SortStars
	}
	q.Descending = lipindex.DefaultDescending(q.Sort) != c.Bool("reverse")
	if !c.Bool("any-version") || q.LoaderVersion != "" {
		db, derr := modLoaderDB(c)
		switch {
		case derr != nil && (c.Changed("game") || q.LoaderVersion != ""):
			return derr
		case derr != nil:
			c.Warn("%s; showing mods for every game version", FromError(derr).Message)
		default:
			q.DB = db
		}
		if q.DB != nil && !c.Bool("any-version") {
			if c.Changed("game") {
				g, ok := modResolveGame(q.DB, c.String("game"))
				if !ok {
					return Errorf(ExitNotFound, "ERR_LL_NOT_SUPPORTED", "no LeviLamina release supports Minecraft %s", c.String("game")).
						WithHint("%s mod search --any-version", ProgramName)
				}
				if g != lipindex.NormalizeSemverInput(c.String("game")) {
					c.Notice("Using Minecraft %s", g)
				}
				q.GameVersion = g
			} else if inst, ok, ierr := modSoftInstance(c); ierr != nil {
				return ierr
			} else if ok {
				g := lipindex.NormalizeSemverInput(inst.Meta.GameVersion)
				if len(q.DB.Versions(g)) > 0 {
					q.GameVersion = g
					c.Notice("Showing mods for Minecraft %s (instance %s); add --any-version to see all", g, inst.Name)
				}
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
		items[i] = modSearchItem{ID: lipShortID(p.Identifier), Name: p.Name, Description: p.Description, Author: p.Author,
			Tags: p.Tags, Latest: p.LatestVersion(), Versions: p.Versions, LoaderRanges: p.LLRanges,
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
	ProjectURL  string           `json:"projectUrl"`
	License     string           `json:"license"`
	Tags        []string         `json:"tags"`
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

// pickVariant returns the variant a reference names (a client variant),
// else the default one.
func modPickVariant(pkg *lipindex.Package, key string) (lipindex.Variant, error) {
	if strings.TrimSpace(key) == "" {
		v, ok := pkg.DefaultVariant()
		if !ok {
			return v, modNotClientMod(pkg.Identifier)
		}
		return v, nil
	}
	if v, ok := pkg.FindVariant(key); ok && v.IsClientVariant() {
		return v, nil
	}
	var keys []string
	for _, v := range pkg.ClientVariants() {
		keys = append(keys, v.Key)
	}
	return lipindex.Variant{}, Errorf(ExitNotFound, "ERR_LIP_VARIANT_NOT_FOUND", "%s has no client variant %q (client variants: %s)",
		lipShortID(pkg.Identifier), key, strings.Join(keys, ", "))
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
	variant, err := modPickVariant(pkg, ref.Variant)
	if err != nil {
		return err
	}
	var db lipindex.LoaderDB
	if d, derr := modLoaderDB(c); derr != nil {
		c.Warn("%s; supported game versions are not shown", FromError(derr).Message)
	} else {
		db = d
	}
	res := modShowResult{ID: lipShortID(pkg.Identifier), Name: pkg.Name, Description: pkg.Description, Author: pkg.Author,
		ProjectURL: pkg.ProjectURL, License: pkg.License, Tags: pkg.Tags, Stars: pkg.Stars, Updated: pkg.Updated,
		Variant: variant.Key, Variants: []string{}, Latest: lipindex.PickTargetVersion(variant.Versions), Versions: []modVersionInfo{}}
	for _, v := range pkg.ClientVariants() {
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
		pairs := []string{
			"Package", res.ID,
			"Name", res.Name,
			"Author", Dash(res.Author),
			"Description", Dash(res.Description),
			"Project", Dash(res.ProjectURL),
			"License", Dash(res.License),
			"Stars", fmt.Sprintf("%.0f", res.Stars),
			"Updated", modDate(res.Updated),
			"Variants", strings.Join(res.Variants, ", "),
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
