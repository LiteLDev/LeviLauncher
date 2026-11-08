package cli

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/contentmgr"
	"github.com/liteldev/LeviLauncher/internal/curseforge"
	"github.com/liteldev/LeviLauncher/internal/curseforge/client"
	cftypes "github.com/liteldev/LeviLauncher/internal/curseforge/client/types"
	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/lipindex"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/packages"
)

// curseforgeGameID is Minecraft Bedrock on CurseForge.
const curseforgeGameID = "78022"

// skinPackPlayerMaxVersion: skin packs for games up to this version are
// imported for a player (CurseForgeInstallDialog).
const skinPackPlayerMaxVersion = "1.26.0"

// cfImporter imports downloaded content into an instance.
type cfImporter interface {
	ImportMcworldPath(name, player, path string, overwrite bool) string
	ImportMcaddonPath(name, path string, overwrite bool) string
	ImportMcaddonPathWithPlayer(name, player, path string, overwrite bool) string
	ImportMcpackPath(name, path string, overwrite bool) string
	ImportMcpackPathWithPlayer(name, player, path string, overwrite bool) string
	IsMcpackSkinPackPath(path string) bool
}

// Seams replaced by tests.
var (
	newCurseClient = func() client.CurseClient { return client.NewCurseClient(curseforge.APIKey()) }
	cfDownload     = downloader.Download
	cfDownloadsDir = apppath.DownloadsDir
	newCFImporter  = func() cfImporter {
		return contentmgr.New(contentmgr.Deps{
			PackLoader:      packages.PackManager{},
			GetContentRoots: mcservice.GetContentRoots,
			GetVersionGameInfo: func(name string) string {
				return mcservice.GetVersionMeta(name).GameVersion
			},
			ListDir: mcservice.ListDir,
		})
	}
)

var cfSorts = map[string]int{"featured": 1, "popularity": 2, "updated": 3, "created": 10, "downloads": 6}

func init() {
	Register(&Command{
		Path:    "curseforge search",
		Summary: "search CurseForge for worlds, add-ons, packs and skins",
		Args:    "[<words>]",
		Description: `Searches Minecraft Bedrock content on CurseForge. --class and --category
take a name or an ID from 'curseforge categories'; --category can be
repeated. --game keeps files for a game version ("latest" or a prefix like
1.21).`,
		Examples: []string{"curseforge search castle", "cf search --class worlds --sort downloads", "curseforge search shaders --class \"texture packs\" --page 2 --json"},
		Flags: []*Flag{
			StringFlag("class", "", "<name|id>", "only this class (worlds, addons, ...)"),
			StringsFlag("category", "", "<name|id>", "only this category"),
			StringFlag("game", "", "<version>", "only content for this game version"),
			EnumFlag("sort", "", []string{"featured", "popularity", "updated", "created", "downloads"}, "sort order (default featured)"),
			IntFlag("limit", "", "<n>", "results per page (at most 50)").WithDefault("20"),
			IntFlag("page", "", "<n>", "page of results").WithDefault("1"),
		},
		MaxArgs: Unlimited,
		Run:     runCFSearch,
	})
	Register(&Command{
		Path:     "curseforge categories",
		Summary:  "list the CurseForge classes and categories to filter by",
		Examples: []string{"curseforge categories", "cf categories --json"},
		Run:      runCFCategories,
	})
	Register(&Command{
		Path:    "curseforge show",
		Summary: "show a CurseForge project",
		Args:    "<id>",
		Description: `Shows a CurseForge project: summary, authors, links and the newest game
version. --files lists its files (filtered by --game); --description prints
the full description as plain text.`,
		Examples: []string{"curseforge show 123456", "cf show 123456 --files --game 1.21", "curseforge show 123456 --description"},
		Flags: []*Flag{
			BoolFlag("files", "", "list the project's files"),
			StringFlag("game", "", "<version>", "with --files: only files for this game version"),
			BoolFlag("description", "", "print the description as text"),
		},
		MinArgs: 1, MaxArgs: 1,
		Run: runCFShow,
	})
	Register(&Command{
		Path:    "curseforge install",
		Summary: "download a CurseForge file and import it into an instance",
		Args:    "<id>",
		Description: `Downloads a file of a CurseForge project and imports it into an instance as
a world, add-on or pack. Without --file, the newest release for the
instance's game version is used (then the newest release, then the newest
file). Worlds, and skin packs for games up to 1.26.0, go to a player
profile (-p).

When the content already exists, --overwrite replaces it and
--skip-existing keeps it; otherwise you are asked on a console, and without
one the command fails with exit 4. The downloaded file stays in the
launcher's download cache.`,
		Examples: []string{"curseforge install 123456", "cf install 123456 --file 4567890 -i Survival", "curseforge install 123456 -p Steve --overwrite"},
		Flags: []*Flag{
			IntFlag("file", "", "<file-id>", "file to install (see curseforge show <id> --files)"),
			BoolFlag("overwrite", "", "replace content that already exists"),
			BoolFlag("skip-existing", "", "keep content that already exists"),
		},
		MinArgs: 1, MaxArgs: 1,
		Run: runCFInstall,
	})
}

// cfError converts a CurseForge client error.
func cfError(err error, what string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if status := cfStatus(err); status > 0 {
		switch {
		case status == 404:
			return NotFoundError("%s not found on CurseForge", what).withCode("ERR_CURSEFORGE_NOT_FOUND").WithDetail(err.Error())
		case status == 401 || status == 403:
			return Errorf(ExitAuth, "ERR_CURSEFORGE_FORBIDDEN", "CurseForge refused the request").
				WithHint("set CURSEFORGE_API_KEY to a valid API key").WithDetail(err.Error())
		case status < 500:
			return Errorf(ExitFailure, "ERR_CURSEFORGE_REQUEST", "CurseForge rejected the request").WithDetail(err.Error())
		}
	}
	return Errorf(ExitNetwork, "ERR_CURSEFORGE_UNAVAILABLE", "cannot reach CurseForge").
		WithHint("check the network connection and run the command again").WithDetail(err.Error())
}

// cfStatus finds the HTTP status of a CurseForge API error. The client
// nests its errors without Unwrap, so the chain is walked by hand.
func cfStatus(err error) int {
	for depth := 0; err != nil && depth < 10; depth++ {
		var api cftypes.CurseforgeAPIError
		if errors.As(err, &api) {
			if api.Status >= 400 {
				return api.Status
			}
			err = api.Err
			continue
		}
		var apiPtr *cftypes.CurseforgeAPIError
		if errors.As(err, &apiPtr) && apiPtr != nil {
			if apiPtr.Status >= 400 {
				return apiPtr.Status
			}
			err = apiPtr.Err
			continue
		}
		return 0
	}
	return 0
}

func parseCFID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || id <= 0 {
		return 0, UsageError("%q is not a CurseForge project ID (a positive number)", s).
			WithHint("%s curseforge search <words>", ProgramName)
	}
	return id, nil
}

// cfCategory is one element of `curseforge categories --json`.
type cfCategory struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	IsClass bool   `json:"isClass"`
	ClassID int64  `json:"classId"`
	Class   string `json:"class"`
}

// cfCatalog is the classes (by display order) and categories of the game.
type cfCatalog struct {
	classes    []cftypes.Categories
	categories []cftypes.Categories
}

func loadCFCategories(c *Ctx, cl client.CurseClient) (cfCatalog, error) {
	resp, err := cl.GetCategories(c.Context(), curseforgeGameID)
	if err != nil {
		return cfCatalog{}, cfError(err, "categories")
	}
	var cat cfCatalog
	for _, x := range resp.Data {
		if x.IsClass {
			cat.classes = append(cat.classes, x)
		} else {
			cat.categories = append(cat.categories, x)
		}
	}
	sort.SliceStable(cat.classes, func(i, j int) bool { return cat.classes[i].DisplayIndex < cat.classes[j].DisplayIndex })
	sort.SliceStable(cat.categories, func(i, j int) bool {
		a, b := cat.categories[i], cat.categories[j]
		if a.ClassID != b.ClassID {
			return cat.classIndex(a.ClassID) < cat.classIndex(b.ClassID)
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return cat, nil
}

func (cat cfCatalog) classIndex(id int64) int {
	for i, c := range cat.classes {
		if c.ID == id {
			return i
		}
	}
	return len(cat.classes)
}

func (cat cfCatalog) className(id int64) string {
	for _, c := range cat.classes {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

func cfMatch(c cftypes.Categories, want string) bool {
	want = strings.TrimSpace(want)
	if id, err := strconv.ParseInt(want, 10, 64); err == nil {
		return c.ID == id
	}
	return strings.EqualFold(c.Name, want) || strings.EqualFold(c.Slug, want) ||
		strings.EqualFold(strings.ReplaceAll(c.Name, " ", "-"), want)
}

func (cat cfCatalog) findClass(want string) (cftypes.Categories, error) {
	var names []string
	for _, c := range cat.classes {
		if cfMatch(c, want) {
			return c, nil
		}
		names = append(names, c.Name)
	}
	return cftypes.Categories{}, NotFoundError("no CurseForge class %q (classes: %s)", want, strings.Join(names, ", ")).
		withCode("ERR_CURSEFORGE_CLASS_NOT_FOUND").WithHint("%s curseforge categories", ProgramName)
}

func (cat cfCatalog) findCategory(want string, classID int64) (cftypes.Categories, error) {
	var hits []cftypes.Categories
	for _, c := range cat.categories {
		if (classID == 0 || c.ClassID == classID) && cfMatch(c, want) {
			hits = append(hits, c)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		msg := fmt.Sprintf("no CurseForge category %q", want)
		if classID != 0 {
			var names []string
			for _, c := range cat.categories {
				if c.ClassID == classID {
					names = append(names, c.Name)
				}
			}
			msg = fmt.Sprintf("no category %q in %s (categories: %s)", want, cat.className(classID), strings.Join(names, ", "))
		}
		return cftypes.Categories{}, NotFoundError("%s", msg).withCode("ERR_CURSEFORGE_CATEGORY_NOT_FOUND").
			WithHint("%s curseforge categories", ProgramName)
	}
	var opts []string
	for _, h := range hits {
		opts = append(opts, fmt.Sprintf("%s/%s (%d)", cat.className(h.ClassID), h.Name, h.ID))
	}
	return cftypes.Categories{}, UsageError("category %q is in several classes: %s", want, strings.Join(opts, ", ")).
		WithHint("pass --class, or the category ID").withCode("ERR_AMBIGUOUS_CATEGORY")
}

// resolveCFGame matches a game version against CurseForge's list: an exact
// name, "latest", or a dotted prefix (the newest match).
func resolveCFGame(c *Ctx, cl client.CurseClient, want string) (string, error) {
	resp, err := cl.GetGameVersions(c.Context(), curseforgeGameID)
	if err != nil {
		return "", cfError(err, "game versions")
	}
	want = strings.TrimSpace(want)
	best := ""
	for _, t := range resp.Data {
		for _, v := range t.Versions {
			n := strings.TrimSpace(v.Name)
			if n == "" || (n[0] < '0' || n[0] > '9') {
				continue
			}
			ok := strings.EqualFold(want, "latest") || n == want || strings.HasPrefix(n, want+".")
			if ok && (best == "" || lipindex.CompareVersions(n, best) > 0) {
				best = n
			}
		}
	}
	if best == "" {
		return "", NotFoundError("CurseForge has no game version %q", want).withCode("ERR_CURSEFORGE_GAME_VERSION_NOT_FOUND")
	}
	if best != want {
		c.Notice("Using game version %s", best)
	}
	return best, nil
}

// cfLatestGame is the newest game version a project's latest files list.
func cfLatestGame(m cftypes.ModData) string {
	var versions []string
	for _, x := range m.LatestFilesIndexes {
		versions = append(versions, x.GameVersion)
	}
	for _, f := range m.LatestFiles {
		versions = append(versions, f.GameVersions...)
	}
	best := ""
	for _, v := range versions {
		v = strings.TrimSpace(v)
		if v == "" || v[0] < '0' || v[0] > '9' {
			continue
		}
		if best == "" || cfCompareGame(v, best) > 0 {
			best = v
		}
	}
	return best
}

// cfCompareGame orders game versions numerically (sortGameVersions).
func cfCompareGame(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

// cfSearchItem is one result of `curseforge search --json`.
type cfSearchItem struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Summary    string    `json:"summary"`
	Class      string    `json:"class"`
	ClassID    int       `json:"classId"`
	Authors    []string  `json:"authors"`
	Downloads  int       `json:"downloads"`
	Updated    time.Time `json:"updated"`
	LatestGame string    `json:"latestGame"`
	URL        string    `json:"url"`
}

type cfSearchResult struct {
	Items      []cfSearchItem `json:"items"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalCount int64          `json:"totalCount"`
}

func runCFSearch(c *Ctx) error {
	limit, page := c.Int("limit"), c.Int("page")
	if limit < 1 || limit > 50 || page < 1 {
		return UsageError("--limit must be 1 to 50 and --page at least 1")
	}
	cl := newCurseClient()
	cat, err := loadCFCategories(c, cl)
	if err != nil {
		return err
	}
	var classID int64
	if c.Changed("class") {
		cls, err := cat.findClass(c.String("class"))
		if err != nil {
			return err
		}
		classID = cls.ID
	}
	var catIDs []string
	for _, want := range c.Strings("category") {
		x, err := cat.findCategory(want, classID)
		if err != nil {
			return err
		}
		catIDs = append(catIDs, strconv.FormatInt(x.ID, 10))
	}
	game := ""
	if c.Changed("game") {
		if game, err = resolveCFGame(c, cl, c.String("game")); err != nil {
			return err
		}
	}
	sortKey := c.String("sort")
	if sortKey == "" {
		sortKey = "featured"
	}
	opts := []client.ModsQueryOption{client.WithModsGameID(curseforgeGameID), client.WithModsSortField(cfSorts[sortKey]),
		client.WithModsSortOrder("desc"), client.WithModsPageSize(int64(limit))}
	if game != "" {
		opts = append(opts, client.WithModsGameVersion(game))
	}
	if classID > 0 {
		opts = append(opts, client.WithModsClassID(strconv.FormatInt(classID, 10)))
	}
	if len(catIDs) > 0 {
		opts = append(opts, client.WithModsCategoryIDs("["+strings.Join(catIDs, ",")+"]"))
	}
	if q := strings.TrimSpace(strings.Join(c.Args, " ")); q != "" {
		opts = append(opts, client.WithModsSeatchFilter(q))
	}
	if idx := (page - 1) * limit; idx > 0 {
		opts = append(opts, client.WithModsIndex(int64(idx)))
	}
	prog := c.Progress()
	prog.Phase("Searching CurseForge")
	resp, err := cl.GetMods(c.Context(), opts...)
	prog.Done()
	if err != nil {
		return cfError(err, "search")
	}
	res := cfSearchResult{Items: []cfSearchItem{}, Page: page, PageSize: limit, TotalCount: resp.Pagination.TotalCount}
	ids := []string{}
	for _, m := range resp.Data {
		it := cfSearchItem{ID: m.ID, Name: m.Name, Summary: m.Summary, ClassID: m.ClassID, Class: cat.className(int64(m.ClassID)),
			Authors: []string{}, Downloads: m.DownloadCount, Updated: m.DateModified, LatestGame: cfLatestGame(m), URL: m.Links.WebsiteURL}
		for _, a := range m.Authors {
			it.Authors = append(it.Authors, a.Name)
		}
		res.Items = append(res.Items, it)
		ids = append(ids, strconv.FormatInt(m.ID, 10))
	}
	return c.Render(res, ids, func(w io.Writer) error {
		if len(res.Items) == 0 {
			c.Notice("Nothing found.")
			return nil
		}
		rows := make([][]string, len(res.Items))
		for i, it := range res.Items {
			rows[i] = []string{strconv.FormatInt(it.ID, 10), modTruncate(it.Name, 40), Dash(it.Class), modTruncate(strings.Join(it.Authors, ", "), 24),
				strconv.Itoa(it.Downloads), cfDate(it.Updated), Dash(it.LatestGame)}
		}
		if err := Table(w, []string{"ID", "NAME", "CLASS", "AUTHOR", "DOWNLOADS", "UPDATED", "LATEST-GAME"}, rows); err != nil {
			return err
		}
		if pages := (res.TotalCount + int64(limit) - 1) / int64(limit); pages > 1 {
			c.Notice("Page %d of %d (%d results); next: --page %d", page, pages, res.TotalCount, page+1)
		}
		c.Notice("Install with: %s curseforge install <ID>", ProgramName)
		return nil
	})
}

func cfDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02")
}

func runCFCategories(c *Ctx) error {
	cat, err := loadCFCategories(c, newCurseClient())
	if err != nil {
		return err
	}
	items := []cfCategory{}
	ids := []string{}
	for _, cls := range cat.classes {
		items = append(items, cfCategory{ID: cls.ID, Name: cls.Name, Slug: cls.Slug, IsClass: true, ClassID: cls.ID, Class: cls.Name})
		ids = append(ids, strconv.FormatInt(cls.ID, 10))
		for _, x := range cat.categories {
			if x.ClassID == cls.ID {
				items = append(items, cfCategory{ID: x.ID, Name: x.Name, Slug: x.Slug, ClassID: x.ClassID, Class: cls.Name})
				ids = append(ids, strconv.FormatInt(x.ID, 10))
			}
		}
	}
	return c.Render(items, ids, func(w io.Writer) error {
		rows := make([][]string, len(items))
		for i, it := range items {
			name := it.Name
			if it.IsClass {
				name = "-"
			}
			rows[i] = []string{it.Class, name, strconv.FormatInt(it.ID, 10)}
		}
		return Table(w, []string{"CLASS", "CATEGORY", "ID"}, rows)
	})
}

// cfFile is one file of `curseforge show --files --json`.
type cfFile struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	FileName     string    `json:"fileName"`
	Type         string    `json:"type"`
	GameVersions []string  `json:"gameVersions"`
	Date         time.Time `json:"date"`
	Size         int64     `json:"size"`
	Downloadable bool      `json:"downloadable"`
}

type cfShowResult struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Summary     string    `json:"summary"`
	Class       string    `json:"class"`
	Categories  []string  `json:"categories"`
	Authors     []string  `json:"authors"`
	Downloads   int       `json:"downloads"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	LatestGame  string    `json:"latestGame"`
	Website     string    `json:"website"`
	Source      string    `json:"source"`
	Issues      string    `json:"issues"`
	Files       []cfFile  `json:"files,omitempty"`
	Description string    `json:"description,omitempty"`
}

func cfReleaseType(t int) string {
	switch t {
	case 1:
		return "release"
	case 2:
		return "beta"
	case 3:
		return "alpha"
	}
	return "unknown"
}

func fetchCFProject(c *Ctx, cl client.CurseClient, id int64) (cftypes.ModData, error) {
	prog := c.Progress()
	prog.Phase("Loading CurseForge project %d", id)
	resp, err := cl.GetModsByIDs(c.Context(), &client.GetModsByIdsListRequest{ModIds: []int64{id}})
	prog.Done()
	if err != nil {
		return cftypes.ModData{}, cfError(err, fmt.Sprintf("project %d", id))
	}
	for _, m := range resp.Data {
		if m.ID == id {
			return m, nil
		}
	}
	return cftypes.ModData{}, NotFoundError("no CurseForge project %d", id).withCode("ERR_CURSEFORGE_NOT_FOUND").
		WithHint("%s curseforge search <words>", ProgramName)
}

func fetchCFFiles(c *Ctx, cl client.CurseClient, id int64) ([]cftypes.File, error) {
	prog := c.Progress()
	prog.Phase("Loading the files of project %d", id)
	resp, err := cl.GetModFiles(c.Context(), id)
	prog.Done()
	if err != nil {
		return nil, cfError(err, fmt.Sprintf("project %d", id))
	}
	return resp.Data, nil
}

func runCFShow(c *Ctx) error {
	id, err := parseCFID(c.Arg(0))
	if err != nil {
		return err
	}
	if c.Changed("game") && !c.Bool("files") {
		return UsageError("--game filters the file list; add --files")
	}
	cl := newCurseClient()
	m, err := fetchCFProject(c, cl, id)
	if err != nil {
		return err
	}
	res := cfShowResult{ID: m.ID, Name: m.Name, Summary: m.Summary, Categories: []string{}, Authors: []string{},
		Downloads: m.DownloadCount, Created: m.DateCreated, Updated: m.DateModified, LatestGame: cfLatestGame(m),
		Website: m.Links.WebsiteURL, Source: m.Links.SourceURL, Issues: m.Links.IssuesURL}
	if cat, err := loadCFCategories(c, cl); err == nil {
		res.Class = cat.className(int64(m.ClassID))
	}
	for _, x := range m.Categories {
		res.Categories = append(res.Categories, x.Name)
	}
	for _, a := range m.Authors {
		res.Authors = append(res.Authors, a.Name)
	}
	if c.Bool("files") {
		files, err := fetchCFFiles(c, cl, id)
		if err != nil {
			return err
		}
		game := strings.TrimSpace(c.String("game"))
		res.Files = []cfFile{}
		for _, f := range files {
			if game != "" && !cfFileMatchesGame(f, game) {
				continue
			}
			res.Files = append(res.Files, cfFile{ID: f.ID, Name: modFirstNonEmpty(f.DisplayName, f.FileName), FileName: f.FileName,
				Type: cfReleaseType(f.ReleaseType), GameVersions: append([]string{}, f.GameVersions...), Date: f.FileDate,
				Size: int64(f.FileLength), Downloadable: f.DownloadURL != ""})
		}
		sort.SliceStable(res.Files, func(i, j int) bool { return res.Files[i].Date.After(res.Files[j].Date) })
	}
	if c.Bool("description") {
		resp, err := cl.GetModDescription(c.Context(), id)
		if err != nil {
			return cfError(err, fmt.Sprintf("project %d", id))
		}
		res.Description = cfHTMLToText(resp.Data)
	}
	var ids []string
	if c.Bool("files") {
		for _, f := range res.Files {
			ids = append(ids, strconv.Itoa(f.ID))
		}
	} else {
		ids = []string{strconv.FormatInt(res.ID, 10)}
	}
	return c.Render(res, ids, func(w io.Writer) error {
		if c.Bool("description") && !c.Bool("files") {
			_, err := fmt.Fprintln(w, res.Description)
			return err
		}
		if err := KV(w,
			"ID", strconv.FormatInt(res.ID, 10),
			"Name", res.Name,
			"Summary", Dash(res.Summary),
			"Class", Dash(res.Class),
			"Categories", Dash(strings.Join(res.Categories, ", ")),
			"Authors", Dash(strings.Join(res.Authors, ", ")),
			"Downloads", strconv.Itoa(res.Downloads),
			"Updated", Ago(res.Updated),
			"Latest game", Dash(res.LatestGame),
			"Website", Dash(res.Website),
			"Source", Dash(res.Source),
			"Issues", Dash(res.Issues),
		); err != nil {
			return err
		}
		if c.Bool("files") {
			fmt.Fprintln(w)
			if len(res.Files) == 0 {
				c.Notice("No files match.")
			} else {
				rows := make([][]string, len(res.Files))
				for i, f := range res.Files {
					rows[i] = []string{strconv.Itoa(f.ID), modTruncate(f.Name, 48), f.Type, modTruncate(strings.Join(f.GameVersions, ", "), 30),
						cfDate(f.Date), Size(f.Size)}
				}
				if err := Table(w, []string{"FILE-ID", "NAME", "TYPE", "GAME-VERSIONS", "DATE", "SIZE"}, rows); err != nil {
					return err
				}
			}
		}
		if c.Bool("description") {
			fmt.Fprintln(w)
			fmt.Fprintln(w, res.Description)
		}
		c.Notice("Install with: %s curseforge install %d", ProgramName, res.ID)
		return nil
	})
}

func cfFileMatchesGame(f cftypes.File, game string) bool {
	for _, v := range f.GameVersions {
		if v == game || strings.HasPrefix(v, game+".") {
			return true
		}
	}
	return false
}

var (
	htmlBreaks = regexp.MustCompile(`(?i)<\s*(br|/?p|/?div|/li|/?h[1-6]|/tr)(\s[^>]*)?\s*/?>`)
	htmlItems  = regexp.MustCompile(`(?i)<\s*li[^>]*>`)
	htmlTags   = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlBlank  = regexp.MustCompile(`\n[ \t]*\n([ \t]*\n)+`)
)

// htmlToText renders a CurseForge description as plain text.
func cfHTMLToText(s string) string {
	s = htmlBreaks.ReplaceAllString(s, "\n")
	s = htmlItems.ReplaceAllString(s, "- ")
	s = htmlTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	lines := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	s = htmlBlank.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
	return strings.TrimSpace(s)
}

// pickCFFile is the file installed when none is chosen: the newest release
// for the game version (exact, or the same major.minor), then any file for
// it, then the newest release, then the newest file (pickDefaultFile).
func pickCFFile(files []cftypes.File, gameVersion string) (cftypes.File, bool) {
	var usable []cftypes.File
	for _, f := range files {
		if f.DownloadURL != "" {
			usable = append(usable, f)
		}
	}
	if len(usable) == 0 {
		return cftypes.File{}, false
	}
	sort.SliceStable(usable, func(i, j int) bool { return usable[i].FileDate.After(usable[j].FileDate) })
	parts := strings.Split(gameVersion, ".")
	minor := strings.Join(parts[:min(2, len(parts))], ".")
	matches := func(f cftypes.File) bool {
		if gameVersion == "" {
			return false
		}
		for _, v := range f.GameVersions {
			if v == gameVersion || (minor != "" && strings.HasPrefix(v, minor)) {
				return true
			}
		}
		return false
	}
	for _, f := range usable {
		if matches(f) && f.ReleaseType == 1 {
			return f, true
		}
	}
	for _, f := range usable {
		if matches(f) {
			return f, true
		}
	}
	for _, f := range usable {
		if f.ReleaseType == 1 {
			return f, true
		}
	}
	return usable[0], true
}

// cfContentType classifies a downloaded file by extension.
func cfContentType(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".mcworld":
		return "world"
	case ".mcaddon":
		return "addon"
	case ".mcpack":
		return "pack"
	}
	return "unknown"
}

var cfTypeLabels = map[string]string{"world": "world", "addon": "add-on", "pack": "pack", "skin_pack": "skin pack", "unknown": "pack"}

func cfFileMD5(f cftypes.File) string {
	for _, h := range f.Hashes {
		if h.Algo == cftypes.HashAlgoMD5 {
			return strings.ToLower(strings.TrimSpace(h.Value))
		}
	}
	return ""
}

// cfInstallResult is `curseforge install --json`.
type cfInstallResult struct {
	ProjectID int64  `json:"projectId"`
	FileID    int    `json:"fileId"`
	File      string `json:"file"`
	Path      string `json:"path"`
	Type      string `json:"type"`
	Instance  string `json:"instance"`
	Player    string `json:"player,omitempty"`
	Status    string `json:"status"`
}

func runCFInstall(c *Ctx) error {
	if c.Bool("overwrite") && c.Bool("skip-existing") {
		return UsageError("--overwrite and --skip-existing cannot be used together")
	}
	id, err := parseCFID(c.Arg(0))
	if err != nil {
		return err
	}
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	cl := newCurseClient()
	files, err := fetchCFFiles(c, cl, id)
	if err != nil {
		return err
	}
	var file cftypes.File
	if c.Changed("file") {
		found := false
		for _, f := range files {
			if f.ID == c.Int("file") {
				file, found = f, true
			}
		}
		if !found {
			return NotFoundError("project %d has no file %d", id, c.Int("file")).withCode("ERR_CURSEFORGE_FILE_NOT_FOUND").
				WithHint("%s curseforge show %d --files", ProgramName, id)
		}
		if file.DownloadURL == "" {
			return Errorf(ExitFailure, "ERR_CURSEFORGE_NOT_DOWNLOADABLE", "file %d cannot be downloaded through the API (the author does not allow it)", file.ID).
				WithHint("download it from the website: https://www.curseforge.com")
		}
	} else {
		f, ok := pickCFFile(files, inst.Meta.GameVersion)
		if !ok {
			return Errorf(ExitNotFound, "ERR_CURSEFORGE_NO_FILES", "project %d has no file that can be downloaded through the API", id)
		}
		file = f
	}
	typ := cfContentType(file.FileName)
	// Worlds always go to a player on GDK instances; find it before
	// downloading.
	var player Player
	if typ == "world" && !inst.IsUWP() {
		if player, err = c.ResolvePlayer(inst); err != nil {
			return err
		}
	}
	name := filepath.Base(strings.ReplaceAll(file.FileName, "\\", "/"))
	if name == "" || name == "." || strings.ContainsAny(name, `<>:"|?*`) {
		return Errorf(ExitFailure, "ERR_INVALID_NAME", "CurseForge returned an unusable file name %q", file.FileName)
	}
	dest := filepath.Join(cfDownloadsDir(), name)
	prog := c.Progress()
	prog.Phase("Downloading %s", name)
	err = cfDownload(c.Context(), file.DownloadURL, dest, downloader.SyncOptions{MD5: cfFileMD5(file), Progress: prog.ProgressFunc()})
	prog.Done()
	if err != nil {
		if c.Context().Err() != nil {
			c.Notice("Download stopped; run the same command to resume")
			return c.Context().Err()
		}
		if errors.Is(err, downloader.ErrBusy) {
			return Backend("ERR_DOWNLOAD_BUSY")
		}
		if e := FromError(err); e.Code != "ERR_FAILED" {
			return e
		}
		return Errorf(ExitNetwork, "ERR_DOWNLOAD_FAILED", "downloading %s failed", name).WithDetail(err.Error())
	}
	c.Verbosef("Downloaded to %s", dest)
	imp := newCFImporter()
	if typ == "pack" && imp.IsMcpackSkinPackPath(dest) {
		typ = "skin_pack"
	}
	if typ == "skin_pack" && !inst.IsUWP() &&
		lipindex.CompareVersions(modFirstNonEmpty(inst.Meta.GameVersion, "0"), skinPackPlayerMaxVersion) <= 0 {
		if players, _ := ListPlayers(inst); len(players) > 0 {
			if player, err = c.ResolvePlayer(inst); err != nil {
				return err
			}
		}
	}
	run := func(overwrite bool) string {
		switch {
		case typ == "world":
			return imp.ImportMcworldPath(inst.Name, player.ID, dest, overwrite)
		case typ == "addon" && player.ID != "":
			return imp.ImportMcaddonPathWithPlayer(inst.Name, player.ID, dest, overwrite)
		case typ == "addon":
			return imp.ImportMcaddonPath(inst.Name, dest, overwrite)
		case typ == "skin_pack" && player.ID != "":
			return imp.ImportMcpackPathWithPlayer(inst.Name, player.ID, dest, overwrite)
		}
		return imp.ImportMcpackPath(inst.Name, dest, overwrite)
	}
	label := modFirstNonEmpty(file.DisplayName, file.FileName)
	res := cfInstallResult{ProjectID: id, FileID: file.ID, File: file.FileName, Path: dest, Type: typ, Instance: inst.Name,
		Player: player.ID, Status: "installed"}
	prog.Phase("Importing %s", label)
	r := run(c.Bool("overwrite"))
	prog.Done()
	if code, _, _ := SplitBackendCode(r); code == "ERR_DUPLICATE_FOLDER" || code == "ERR_DUPLICATE_UUID" {
		switch {
		case c.Bool("skip-existing"):
			r, res.Status = "", "skipped"
		case !c.CanPrompt() && !c.G.Yes:
			return ConflictError("%s is already in %s", label, inst.Name).withCode(code).
				WithHint("add --overwrite to replace it or --skip-existing to keep it")
		default:
			if err := c.ConfirmWith(fmt.Sprintf("%s is already in %s. Overwrite it?", label, inst.Name), "--overwrite"); err != nil {
				if FromError(err).Exit != ExitCancelled {
					return err
				}
				r, res.Status = "", "skipped"
			} else {
				r, res.Status = run(true), "overwritten"
			}
		}
	}
	if r != "" {
		e := Backend(r)
		if e.Code == "ERR_OPEN_ZIP" {
			e.Message = "the downloaded file is not a valid Minecraft package"
		}
		return e.WithDetail(modFirstNonEmpty(e.Detail, "file: "+dest))
	}
	return c.Render(res, []string{dest}, func(w io.Writer) error {
		where := inst.Name
		if player.ID != "" {
			where += " for player " + player.label()
		}
		switch res.Status {
		case "skipped":
			_, err := fmt.Fprintf(w, "Kept the existing %s %q in %s\n", cfTypeLabels[typ], label, where)
			return err
		case "overwritten":
			_, err := fmt.Fprintf(w, "Replaced %q (%s) in %s\n", label, cfTypeLabels[typ], where)
			return err
		}
		_, err := fmt.Fprintf(w, "Installed %q (%s) into %s\n", label, cfTypeLabels[typ], where)
		return err
	})
}
