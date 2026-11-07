package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/curseforge/client"
	cftypes "github.com/liteldev/LeviLauncher/internal/curseforge/client/types"
	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

type fakeCurse struct {
	files    []cftypes.File
	searches []string
}

func (f *fakeCurse) GetGameVersions(ctx context.Context, gameID string) (*cftypes.GameVersionsResponse, error) {
	return &cftypes.GameVersionsResponse{Data: []cftypes.GameVersionType{{Versions: cftypes.GameVersions{
		{Name: "1.21.50"}, {Name: "1.21.60"}, {Name: "1.20.80"}, {Name: "Addon"}}}}}, nil
}

func (f *fakeCurse) GetCategories(ctx context.Context, gameID string) (*cftypes.GetCategoriesResponse, error) {
	return &cftypes.GetCategoriesResponse{Data: []cftypes.Categories{
		{ID: 17, Name: "Worlds", Slug: "worlds", IsClass: true, DisplayIndex: 1},
		{ID: 4984, Name: "Addons", Slug: "addons", IsClass: true, DisplayIndex: 0},
		{ID: 200, Name: "Adventure", Slug: "adventure", ClassID: 17},
		{ID: 300, Name: "Adventure", Slug: "adventure-addons", ClassID: 4984},
		{ID: 301, Name: "Mobs", Slug: "mobs", ClassID: 4984},
	}}, nil
}

func (f *fakeCurse) GetMods(ctx context.Context, opts ...client.ModsQueryOption) (*cftypes.ModsResponse, error) {
	q := client.ApiQueryParams{}
	for _, o := range opts {
		o(q)
	}
	f.searches = append(f.searches, q.QueryString())
	return &cftypes.ModsResponse{Data: []cftypes.ModData{{ID: 42, Name: "Castle", ClassID: 17, DownloadCount: 7,
		Authors:     []cftypes.Authors{{Name: "Ann"}},
		LatestFiles: cftypes.Files{{GameVersions: []string{"1.21.50", "Bedrock"}}}}},
		Pagination: cftypes.Pagination{TotalCount: 45}}, nil
}

func (f *fakeCurse) GetModsByIDs(ctx context.Context, req *client.GetModsByIdsListRequest, opts ...client.ModsQueryOption) (*cftypes.ModsResponse, error) {
	if req.ModIds[0] != 42 {
		return &cftypes.ModsResponse{}, nil
	}
	return &cftypes.ModsResponse{Data: []cftypes.ModData{{ID: 42, Name: "Castle", ClassID: 17,
		Links: cftypes.Links{WebsiteURL: "https://www.curseforge.com/minecraft-bedrock/worlds/castle"}}}}, nil
}

func (f *fakeCurse) GetModDescription(ctx context.Context, id int64) (*cftypes.StringResponse, error) {
	return &cftypes.StringResponse{Data: "<p>A <b>big</b> castle &amp; moat.</p><ul><li>towers</li></ul>"}, nil
}

func (f *fakeCurse) GetModFiles(ctx context.Context, id int64) (*cftypes.GetModFilesResponse, error) {
	if id != 42 {
		return nil, cftypes.Wrap(fmt.Errorf("status 404"), "not found", 404)
	}
	return &cftypes.GetModFilesResponse{Data: f.files}, nil
}

type fakeImporter struct {
	calls  []string
	result func(call string) string
	skin   bool
}

func (f *fakeImporter) do(call string) string {
	f.calls = append(f.calls, call)
	if f.result != nil {
		return f.result(call)
	}
	return ""
}

func (f *fakeImporter) ImportMcworldPath(name, player, path string, overwrite bool) string {
	return f.do(fmt.Sprintf("world %s %s %s %v", name, player, filepath.Base(path), overwrite))
}
func (f *fakeImporter) ImportMcaddonPath(name, path string, overwrite bool) string {
	return f.do(fmt.Sprintf("addon %s %s %v", name, filepath.Base(path), overwrite))
}
func (f *fakeImporter) ImportMcaddonPathWithPlayer(name, player, path string, overwrite bool) string {
	return f.do(fmt.Sprintf("addon %s %s %s %v", name, player, filepath.Base(path), overwrite))
}
func (f *fakeImporter) ImportMcpackPath(name, path string, overwrite bool) string {
	return f.do(fmt.Sprintf("pack %s %s %v", name, filepath.Base(path), overwrite))
}
func (f *fakeImporter) ImportMcpackPathWithPlayer(name, player, path string, overwrite bool) string {
	return f.do(fmt.Sprintf("pack %s %s %s %v", name, player, filepath.Base(path), overwrite))
}
func (f *fakeImporter) IsMcpackSkinPackPath(path string) bool { return f.skin }

func cfFixture(t *testing.T) (*harness, *fakeCurse, *fakeImporter, *[]string) {
	h := newHarness(t)
	fc := &fakeCurse{}
	imp := &fakeImporter{}
	var downloads []string
	saved := []any{newCurseClient, cfDownload, cfDownloadsDir, newCFImporter}
	t.Cleanup(func() {
		newCurseClient = saved[0].(func() client.CurseClient)
		cfDownload = saved[1].(func(context.Context, string, string, downloader.SyncOptions) error)
		cfDownloadsDir = saved[2].(func() string)
		newCFImporter = saved[3].(func() cfImporter)
	})
	dir := t.TempDir()
	newCurseClient = func() client.CurseClient { return fc }
	cfDownloadsDir = func() string { return dir }
	cfDownload = func(ctx context.Context, src, dest string, o downloader.SyncOptions) error {
		downloads = append(downloads, src+" -> "+filepath.Base(dest)+" md5="+o.MD5)
		return os.WriteFile(dest, []byte("zip"), 0o644)
	}
	newCFImporter = func() cfImporter { return imp }
	return h, fc, imp, &downloads
}

func cfDay(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }

func TestPickCFFile(t *testing.T) {
	files := []cftypes.File{
		{ID: 1, FileDate: cfDay(1), ReleaseType: 1, GameVersions: []string{"1.21.50"}, DownloadURL: "u"},
		{ID: 2, FileDate: cfDay(5), ReleaseType: 2, GameVersions: []string{"1.21.60"}, DownloadURL: "u"},
		{ID: 3, FileDate: cfDay(3), ReleaseType: 1, GameVersions: []string{"1.20.80"}, DownloadURL: "u"},
		{ID: 4, FileDate: cfDay(9), ReleaseType: 1, GameVersions: []string{"1.21.50"}},
	}
	for game, want := range map[string]int{"1.21.50.07": 1, "1.21.70.1": 1, "1.20.80.05": 3, "1.19.0": 3, "": 3} {
		if f, ok := pickCFFile(files, game); !ok || f.ID != want {
			t.Errorf("pickCFFile(%q) = %d, want %d", game, f.ID, want)
		}
	}
	if _, ok := pickCFFile(files[3:], "1.21.50"); ok {
		t.Error("a file without a download URL cannot be picked")
	}
}

func TestHTMLToText(t *testing.T) {
	got := cfHTMLToText("<p>A <b>big</b> castle &amp; moat.</p><ul><li>towers</li><li>walls</li></ul><br/>")
	if got != "A big castle & moat.\n- towers\n- walls" {
		t.Fatalf("htmlToText = %q", got)
	}
}

func TestCurseForgeSearchAndShow(t *testing.T) {
	h, fc, _, _ := cfFixture(t)
	code, out, errs := h.run("cf", "search", "castle", "--class", "worlds", "--category", "adventure", "--game", "1.21", "--sort", "downloads", "--page", "2")
	if code != 0 || !strings.Contains(out, "Castle") || !strings.Contains(out, "Worlds") || !strings.Contains(out, "1.21.50") {
		t.Fatalf("search = %d %q %q", code, out, errs)
	}
	q := fc.searches[0]
	for _, part := range []string{"classId=17", "categoryIds=%5B200%5D", "gameVersion=1.21.60", "sortField=6", "index=20", "searchFilter=castle", "sortOrder=desc"} {
		if !strings.Contains(q, part) {
			t.Errorf("query %q lacks %s", q, part)
		}
	}
	if code, _, errs = h.run("cf", "search", "--category", "adventure"); code != ExitUsage || !strings.Contains(errs, "several classes") {
		t.Fatalf("ambiguous category = %d %q", code, errs)
	}
	if code, _, _ = h.run("cf", "search", "--class", "skins"); code != ExitNotFound {
		t.Fatalf("unknown class = %d", code)
	}
	code, out, _ = h.run("cf", "search", "--json")
	var res cfSearchResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || res.TotalCount != 45 || res.Items[0].ID != 42 {
		t.Fatalf("json = %d %q", code, out)
	}
	code, out, _ = h.run("curseforge", "categories", "-q")
	if code != 0 || !strings.HasPrefix(out, "4984\n300\n301\n17\n200\n") {
		t.Fatalf("categories = %d %q", code, out)
	}
	fc.files = []cftypes.File{{ID: 7, DisplayName: "Castle v2", FileName: "castle.mcworld", ReleaseType: 1,
		GameVersions: []string{"1.21.50"}, FileDate: cfDay(2), FileLength: 2048, DownloadURL: "https://x/castle.mcworld"}}
	code, out, errs = h.run("cf", "show", "42", "--files", "--description")
	if code != 0 || !strings.Contains(out, "Castle v2") || !strings.Contains(out, "A big castle & moat.") || !strings.Contains(out, "2.0 KiB") {
		t.Fatalf("show = %d %q %q", code, out, errs)
	}
	if code, _, _ = h.run("cf", "show", "43"); code != ExitNotFound {
		t.Fatalf("unknown project = %d", code)
	}
	if code, _, _ = h.run("cf", "show", "abc"); code != ExitUsage {
		t.Fatalf("bad id = %d", code)
	}
}

func TestCurseForgeInstall(t *testing.T) {
	h, fc, imp, downloads := cfFixture(t)
	inst := h.add("Survival", "1.21.50.07", "release", versions.PackageTypeGDK)
	usersRootOf = func(Instance) string { return filepath.Join(inst.Dir, "Users") }
	for _, p := range []string{"111", "222"} {
		if err := os.MkdirAll(filepath.Join(inst.Dir, "Users", p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fc.files = []cftypes.File{
		{ID: 7, DisplayName: "Castle", FileName: "castle.mcworld", ReleaseType: 1, GameVersions: []string{"1.21.50"},
			FileDate: cfDay(2), DownloadURL: "https://x/castle.mcworld", Hashes: []cftypes.Hashes{{Value: "ABC", Algo: cftypes.HashAlgoMD5}}},
		{ID: 8, DisplayName: "Trees", FileName: "trees.mcpack", ReleaseType: 1, GameVersions: []string{"1.21.50"},
			FileDate: cfDay(1), DownloadURL: "https://x/trees.mcpack"},
	}
	// A world needs a player; with two profiles one must be chosen.
	if code, _, errs := h.run("cf", "install", "42"); code != ExitUsage || !strings.Contains(errs, "-p") {
		t.Fatalf("no player = %d %q", code, errs)
	}
	code, out, errs := h.run("cf", "install", "42", "-p", "222")
	if code != 0 || !strings.Contains(out, `Installed "Castle" (world) into Survival for player 222`) {
		t.Fatalf("install = %d %q %q", code, out, errs)
	}
	if imp.calls[0] != "world Survival 222 castle.mcworld false" || (*downloads)[0] != "https://x/castle.mcworld -> castle.mcworld md5=abc" {
		t.Fatalf("calls = %v %v", imp.calls, *downloads)
	}
	// A pack, already present: needs a decision without a console.
	imp.result = func(call string) string {
		if strings.HasSuffix(call, "false") {
			return "ERR_DUPLICATE_UUID"
		}
		return ""
	}
	if code, _, errs = h.run("cf", "install", "42", "--file", "8"); code != ExitConflict || !strings.Contains(errs, "--overwrite") {
		t.Fatalf("duplicate = %d %q", code, errs)
	}
	code, out, _ = h.run("cf", "install", "42", "--file", "8", "--overwrite", "--json")
	var res cfInstallResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || res.Status != "installed" || imp.calls[len(imp.calls)-1] != "pack Survival trees.mcpack true" || res.Type != "pack" || res.Player != "" {
		t.Fatalf("overwrite = %d %q", code, out)
	}
	if code, out, _ = h.run("cf", "install", "42", "--file", "8", "--skip-existing"); code != 0 || !strings.Contains(out, "Kept") {
		t.Fatalf("skip = %d %q", code, out)
	}
	// Skin packs for older games go to a player.
	imp.result, imp.skin = nil, true
	if code, _, errs = h.run("cf", "install", "42", "--file", "8", "-p", "111"); code != 0 || imp.calls[len(imp.calls)-1] != "pack Survival 111 trees.mcpack false" {
		t.Fatalf("skin pack = %d %q %v", code, errs, imp.calls)
	}
	if code, _, _ = h.run("cf", "install", "42", "--file", "99"); code != ExitNotFound {
		t.Fatalf("unknown file = %d", code)
	}
	if code, _, _ = h.run("cf", "install", "43"); code != ExitNotFound {
		t.Fatalf("unknown project = %d", code)
	}
}
