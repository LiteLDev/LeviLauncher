package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/gdkversions"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
)

func TestShortVersionAndURLs(t *testing.T) {
	for in, want := range map[string]string{
		"Release 1.21.50.07": "1.21.50.07",
		"Preview 1.21.60.21": "1.21.60.21",
		"preview1.21.60.21":  "1.21.60.21",
		"1.21.50.07":         "1.21.50.07",
		" Release  1.2.3.4 ": "1.2.3.4",
	} {
		if got := ShortVersion(in); got != want {
			t.Errorf("ShortVersion(%q) = %q, want %q", in, got, want)
		}
	}
	got := NormalizeURLs([]string{" `http://assets1.xboxlive.com/a` ", "", "http://assets2.xboxlive.cn/a", "http://assets1.xboxlive.cn/a", "http://x/a"})
	want := []string{"http://assets2.xboxlive.cn/a", "http://assets1.xboxlive.cn/a", "http://assets1.xboxlive.com/a", "http://x/a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NormalizeURLs = %v, want %v", got, want)
	}
}

func sampleBuilds() []Build {
	gdk := FromGDK(gdkversions.Catalog{
		ReleaseVersions: []gdkversions.Version{
			{Version: "Release 1.21.50.07", URLs: []string{"http://a/1"}, Timestamp: 300, MD5: "abc"},
			{Version: "Release 1.21.51.02", Timestamp: 400},
			{Version: "Release 1.21.5.1", Timestamp: 100},
			{Version: "Release 1.20.81.01", Timestamp: 50},
		},
		PreviewVersions: []gdkversions.Version{
			{Version: "Preview 1.21.60.21", Timestamp: 500},
			{Version: "Preview 1.21.50.20", Timestamp: 250},
		},
	})
	uwp := FromUWP([]uwpdownload.Version{
		{Version: "1.21.51.2", UUID: "U1", Type: "release"},
		{Version: "1.21.50.7", UUID: "U0", Type: "release"},
		{Version: "1.16.0.51", UUID: "B1", Type: "beta"},
		{Version: "1.21.60.21", UUID: "P1", Type: "preview"},
	})
	all := append(gdk, uwp...)
	Sort(all)
	return all
}

func TestFromAndSort(t *testing.T) {
	all := sampleBuilds()
	var keys []string
	for _, b := range all {
		keys = append(keys, b.Key())
	}
	want := []string{
		"gdk:preview:1.21.60.21", "gdk:release:1.21.51.02", "gdk:release:1.21.50.07", "gdk:preview:1.21.50.20",
		"gdk:release:1.21.5.1", "gdk:release:1.20.81.01",
		"uwp:preview:1.21.60.21", "uwp:release:1.21.51.2", "uwp:release:1.21.50.7", "uwp:beta:1.16.0.51",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("order\n got %v\nwant %v", keys, want)
	}
	if all[2].MD5 != "abc" || all[2].URLs[0] != "http://a/1" || all[6].UUID != "P1" {
		t.Errorf("fields not carried: %+v %+v", all[2], all[6])
	}
}

func TestResolve(t *testing.T) {
	all := sampleBuilds()
	for _, tc := range []struct {
		spec string
		f    Filter
		want string
		err  error
	}{
		{"latest", Filter{}, "gdk:release:1.21.51.02", nil},
		{"LATEST", Filter{Type: "uwp"}, "uwp:release:1.21.51.2", nil},
		{"latest", Filter{Channels: []string{"preview"}}, "gdk:preview:1.21.60.21", nil},
		{"1.21.50.07", Filter{}, "gdk:release:1.21.50.07", nil},
		{"1.21.50.7", Filter{}, "gdk:release:1.21.50.07", nil},
		{"1.21.50.7", Filter{Type: "uwp"}, "uwp:release:1.21.50.7", nil},
		{"1.21.50", Filter{}, "gdk:release:1.21.50.07", nil},                              // release preferred over 1.21.50.20 preview
		{"1.21.50", Filter{Channels: []string{"preview"}}, "gdk:preview:1.21.50.20", nil}, // channel given
		{"1.21.5", Filter{}, "gdk:release:1.21.5.1", nil},                                 // not 1.21.50.x
		{"1.21", Filter{}, "gdk:release:1.21.51.02", nil},
		{"1.21.60", Filter{}, "gdk:preview:1.21.60.21", nil}, // only previews match
		{"1.16.0.51", Filter{}, "uwp:beta:1.16.0.51", nil},
		{"1.19", Filter{}, "", ErrNoBuild},
		{"1.21.50.07", Filter{Channels: []string{"preview"}}, "", ErrNoBuild},
		{"abc", Filter{}, "", ErrBadSpec},
		{"1.21.", Filter{}, "", ErrBadSpec},
	} {
		got, err := Resolve(all, tc.spec, tc.f)
		if tc.err != nil {
			if !errors.Is(err, tc.err) {
				t.Errorf("Resolve(%q, %+v) err = %v, want %v", tc.spec, tc.f, err, tc.err)
			}
			continue
		}
		if err != nil || got.Key() != tc.want {
			t.Errorf("Resolve(%q, %+v) = %s, %v; want %s", tc.spec, tc.f, got.Key(), err, tc.want)
		}
	}
}

func TestPrefixAndNearest(t *testing.T) {
	if !HasVersionPrefix("1.21.50.07", "1.21.50") || HasVersionPrefix("1.21.50.07", "1.21.5") || !HasVersionPrefix("1.21.50.07", "1.21.50.07") {
		t.Error("HasVersionPrefix")
	}
	if HasVersionPrefix("1.21", "1.21.50") || HasVersionPrefix("1.21.50", "") {
		t.Error("HasVersionPrefix longer/empty")
	}
	if got := Nearest(sampleBuilds(), "1.21.52.01", 3); !reflect.DeepEqual(got, []string{"1.21.60.21", "1.21.51.02", "1.21.51.2"}) {
		t.Errorf("Nearest = %v", got)
	}
	if got := Nearest(sampleBuilds(), "1.21.50.99", 5); !reflect.DeepEqual(got, []string{"1.21.50.20", "1.21.50.07", "1.21.50.7"}) {
		t.Errorf("Nearest = %v", got)
	}
}

func TestFilter(t *testing.T) {
	all := sampleBuilds()
	n := len(Select(all, Filter{Channels: []string{"release"}, Type: "gdk"}))
	if n != 4 {
		t.Errorf("release gdk = %d", n)
	}
	q := Select(all, Filter{Query: "1.21.5"})
	for _, b := range q {
		if !strings.Contains(b.Version, "1.21.5") {
			t.Errorf("query matched %s", b.Version)
		}
	}
	if len(Select(all, Filter{Query: "beta"})) != 1 || len(Select(all, Filter{Query: "Preview 1.21.60"})) != 2 {
		t.Error("query on channel")
	}
}

func TestIdentityFromPath(t *testing.T) {
	for in, want := range map[string]Identity{
		`C:\x\Release 1.21.50.07.msixvc`:                                        {"1.21.50.07", "release", "gdk"},
		"preview 1.21.60.21.MSIXVC":                                             {"1.21.60.21", "preview", "gdk"},
		"minecraft-preview-1.21.60.21.msixvc":                                   {"1.21.60.21", "preview", "gdk"},
		"Minecraft-UWP-Beta-1.16.0.51.appx":                                     {"1.16.0.51", "beta", "uwp"},
		"Release 1.21.0.3.appxbundle":                                           {"1.21.0.3", "release", "uwp"},
		"1.21.50.07.msixvc":                                                     {"1.21.50.07", "", "gdk"},
		"Microsoft.MinecraftUWP_1.21.12001.0_x64__8wekyb3d8bbwe.msixvc":         {"1.21.120.01", "release", "gdk"},
		"Microsoft.MinecraftWindowsBeta_1.21.12021.0_x64__8wekyb3d8bbwe.msixvc": {"1.21.120.21", "preview", "gdk"},
		"Microsoft.MinecraftUWP_1.21.11401.0_x64__8wekyb3d8bbwe.appx":           {"1.21.114.1", "release", "uwp"},
	} {
		got, ok := IdentityFromPath(in)
		if !ok || got != want {
			t.Errorf("IdentityFromPath(%q) = %+v, %v; want %+v", in, got, ok, want)
		}
	}
	for _, in := range []string{"download.msixvc", "foo.zip", "Release.msixvc"} {
		if _, ok := IdentityFromPath(in); ok {
			t.Errorf("IdentityFromPath(%q) matched", in)
		}
	}
	if !IsAppxInstaller("a.MSIXBUNDLE") || IsAppxInstaller("a.msixvc") || !IsInstallerFile("a.msixvc") || IsInstallerFile("a.zip") {
		t.Error("extension checks")
	}
}

func TestFileName(t *testing.T) {
	if n, _ := FileName(Build{Version: "1.21.50.07", Channel: "preview", Type: "gdk"}); n != "Preview 1.21.50.07.msixvc" {
		t.Error(n)
	}
	if n, _ := FileName(Build{Version: "1.21.50.7", Channel: "beta", Type: "uwp"}); n != "Minecraft-UWP-Beta-1.21.50.7.appx" {
		t.Error(n)
	}
	if _, err := FileName(Build{Version: "1.21", Channel: "release", Type: "uwp"}); err == nil {
		t.Error("bad UWP version accepted")
	}
}

func TestScanInstallers(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, size int) {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Release 1.21.50.07.msixvc", 10)
	write("Preview 1.21.60.21.msixvc.partial.tmp", 5)
	write("Release 1.21.51.02.msixvc.partial.tmp", 3)
	write("Release 1.21.51.02.msixvc.lock", 0)
	write("Minecraft-UWP-Release-1.21.50.7.appx", 7)
	write("1.20.81.01.msixvc", 9)
	write("notes.txt", 1)
	write("odd.msixvc", 2)
	files, err := ScanInstallers(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files {
		got = append(got, f.Name+"|"+f.State)
	}
	want := []string{
		"Preview 1.21.60.21.msixvc|partial", "Release 1.21.51.02.msixvc|downloading", "Release 1.21.50.07.msixvc|complete",
		"Minecraft-UWP-Release-1.21.50.7.appx|complete", "1.20.81.01.msixvc|complete", "odd.msixvc|complete",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scan\n got %v\nwant %v", got, want)
	}
	b := Build{Version: "1.21.50.07", Channel: "release", Type: "gdk"}
	if f, ok := FindInstaller(files, b); !ok || f.Name != "Release 1.21.50.07.msixvc" {
		t.Errorf("FindInstaller = %+v %v", f, ok)
	}
	if _, ok := FindInstaller(files, Build{Version: "1.21.60.21", Channel: "preview", Type: "gdk"}); ok {
		t.Error("partial counted as installer")
	}
	if f, ok := PartialOf(files, Build{Version: "1.21.60.21", Channel: "preview", Type: "gdk"}); !ok || f.State != StatePartial {
		t.Error("PartialOf")
	}
	if _, ok := FindInstaller(files, Build{Version: "1.20.81.01", Channel: "preview", Type: "gdk"}); !ok {
		t.Error("bare version file should match any channel")
	}
	if _, ok := FindInstaller(files, Build{Version: "1.21.50.7", Channel: "release", Type: "uwp"}); !ok {
		t.Error("uwp installer")
	}
	if missing, err := ScanInstallers(filepath.Join(dir, "nope")); err != nil || len(missing) != 0 {
		t.Error("missing dir")
	}
}

func TestBestMirror(t *testing.T) {
	if _, ok := BestMirror(nil); ok {
		t.Error("empty")
	}
	rs := []MirrorResult{
		{URL: "a", LatencyMs: 50, OK: false, Measured: true},
		{URL: "b", LatencyMs: 300, OK: true, Measured: true},
		{URL: "c", LatencyMs: 120, OK: true, Measured: true},
		{URL: "d", LatencyMs: 1, Measured: false},
	}
	if b, _ := BestMirror(rs); b.URL != "c" {
		t.Errorf("best = %s", b.URL)
	}
	if b, _ := BestMirror(rs[:1]); b.URL != "a" {
		t.Errorf("fallback best = %s", b.URL)
	}
	var order []string
	for _, r := range RankMirrors(rs) {
		order = append(order, r.URL)
	}
	if !reflect.DeepEqual(order, []string{"c", "b", "a", "d"}) {
		t.Errorf("rank = %v", order)
	}
	if MirrorHost("http://assets1.xboxlive.cn/x") != "assets1.xboxlive.cn" || MirrorHost("%%") != "%%" {
		t.Error("MirrorHost")
	}
}

func TestProbeMirrors(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.Header.Get("X-Test") != "1" {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(200)
	}))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer bad.Close()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer slow.Close()
	rs := ProbeMirrors(context.Background(), nil, []string{ok.URL, bad.URL, slow.URL}, 200*time.Millisecond,
		func(r *http.Request) { r.Header.Set("X-Test", "1") })
	if !rs[0].OK || rs[1].OK || rs[2].OK || !rs[2].Measured || rs[0].URL != ok.URL {
		t.Fatalf("results %+v", rs)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, r := range ProbeMirrors(ctx, nil, []string{ok.URL}, 0, nil) {
		if r.Measured {
			t.Error("cancelled probe measured")
		}
	}
}

func TestLoaderDB(t *testing.T) {
	db := NormalizeLoaderDB(map[string][]string{
		"v1.21.132.01": {"1.9.0", " 1.9.10 ", "1.9.2", "1.9.10", "1.10.0-rc.1", ""},
		"1.21.124.02":  {"1.8.0-rc.1", "1.8.0-rc.2"},
		"1.26.10.04":   {"26.10.0", "weird", "26.10.1"},
		"1.0.0.0":      {""},
	})
	if !reflect.DeepEqual(db.Versions("1.21.132.01"), []string{"1.10.0-rc.1", "1.9.10", "1.9.2", "1.9.0"}) {
		t.Errorf("versions = %v", db.Versions("1.21.132.01"))
	}
	if !reflect.DeepEqual(db.Versions("v1.26.10.04"), []string{"26.10.1", "26.10.0", "weird"}) {
		t.Errorf("non-semver order = %v", db.Versions("1.26.10.04"))
	}
	if db.Supports("1.0.0.0") || db.Supports("1.21.132") {
		t.Error("exact match only")
	}
	if !reflect.DeepEqual(db.GameVersions(), []string{"1.26.10.04", "1.21.132.01", "1.21.124.02"}) {
		t.Errorf("games = %v", db.GameVersions())
	}
	if g, ok := db.ResolveGame("1.21.132"); !ok || g != "1.21.132.01" {
		t.Errorf("ResolveGame prefix = %q", g)
	}
	if g, ok := db.ResolveGame("1.21.132.1"); !ok || g != "1.21.132.01" {
		t.Errorf("ResolveGame numeric = %q", g)
	}
	if _, ok := db.ResolveGame("1.20"); ok {
		t.Error("ResolveGame unknown")
	}
	if LoaderLatest(db.Versions("1.21.132.01")) != "1.10.0-rc.1" || LoaderLatestStable(db.Versions("1.21.132.01")) != "1.9.10" {
		t.Error("latest")
	}
	if LoaderLatestStable(db.Versions("1.21.124.02")) != "" {
		t.Error("only rc")
	}
	if v, ok := FindLoaderVersion(db.Versions("1.21.132.01"), "v1.9.2"); !ok || v != "1.9.2" {
		t.Error("FindLoaderVersion")
	}
	if _, ok := FindLoaderVersion(db.Versions("1.21.132.01"), "1.9"); ok {
		t.Error("FindLoaderVersion prefix")
	}
}

func TestCompareLoaderVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0-rc.1", 1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-1", "1.0.0-alpha", -1},
		{"v1.2.0", "1.10.0", -1},
		{"1.2.0", "x", 1},
		{"a", "b", -1},
		{"1.2.3", "1.2.3", 0},
	} {
		if got := CompareLoaderVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("Compare(%q,%q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
	if !IsPrerelease("1.8.0-rc.1") || !IsPrerelease("1.8.0rc1") || IsPrerelease("1.8.0") {
		t.Error("IsPrerelease")
	}
	for _, tc := range [][3]string{
		{"", "1.0.0", ActionInstall}, {"1.0.0", "1.1.0", ActionUpgrade}, {"1.1.0", "1.0.0", ActionDowngrade},
		{"1.0.0", "v1.0.0", ActionReinstall}, {"1.0.0-rc.1", "1.0.0", ActionUpgrade}, {"?", "1.0.0", ActionChange}, {"x", "x", ActionReinstall},
	} {
		if got := LoaderAction(tc[0], tc[1]); got != tc[2] {
			t.Errorf("LoaderAction(%q,%q) = %s, want %s", tc[0], tc[1], got, tc[2])
		}
	}
}

func TestFreeNameAndInherit(t *testing.T) {
	taken := map[string]bool{"1.21.50.07": true, "1.21.50.07-2": true}
	if n := FreeName("1.21.50.07", func(s string) bool { return taken[s] }); n != "1.21.50.07-3" {
		t.Errorf("FreeName = %q", n)
	}
	if n := FreeName("x", func(string) bool { return false }); n != "x" {
		t.Error(n)
	}
	if n := FreeName(strings.Repeat("a", 64), func(string) bool { return true }); n != "" {
		t.Error("too long")
	}
	all := []InheritSource{
		{Name: "A", Channel: "release", PackageType: "gdk", Isolated: true},
		{Name: "B", Channel: "release", PackageType: "gdk", Isolated: false},
		{Name: "C", Channel: "preview", PackageType: "gdk", Isolated: true},
		{Name: "D", Channel: "release", PackageType: "uwp", Isolated: true},
		{Name: "E", Channel: "Release", PackageType: "", Isolated: true},
		{Name: "F", Channel: "release", PackageType: "gdk", Isolated: true},
	}
	got := InheritCandidates(all, []string{"A", "B", "C", "D", "E"}, "release")
	var names []string
	for _, s := range got {
		names = append(names, s.Name)
	}
	if !reflect.DeepEqual(names, []string{"A", "E"}) {
		t.Errorf("candidates = %v", names)
	}
}

func TestGDKIdentity(t *testing.T) {
	cfg := `<?xml version="1.0" encoding="utf-8"?><Game configVersion="1"><Identity Name="Microsoft.MinecraftWindowsBeta" Publisher="CN=x" Version="1.21.12021.0"/></Game>`
	id, ok := ParseGDKIdentity([]byte(cfg))
	if !ok || id.Channel != "preview" || id.GameVersion != "1.21.120.21" {
		t.Errorf("identity = %+v %v", id, ok)
	}
	if _, ok := ParseGDKIdentity([]byte(`<Game><Identity Name="Other" Version="1.0.0.0"/></Game>`)); ok {
		t.Error("other package accepted")
	}
	dir := t.TempDir()
	if _, ok := ReadGDKIdentity(dir); ok {
		t.Error("missing config")
	}
	os.WriteFile(filepath.Join(dir, "MicrosoftGame.config"), []byte(strings.Replace(cfg, "WindowsBeta", "UWP", 1)), 0o644)
	if id, ok := ReadGDKIdentity(dir); !ok || id.Channel != "release" {
		t.Errorf("read identity = %+v", id)
	}
	if v, ok := PackageToGameVersion("1.26.4501.0"); !ok || v != "1.26.45.01" {
		t.Error(v)
	}
}

func TestLoadCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "c", "gdk.json")
	history := gdkversions.Catalog{Source: "https://x/historical_versions.json", ReleaseVersions: []gdkversions.Version{{Version: "Release 1.21.50.07", Timestamp: 1}}}
	store := gdkversions.Catalog{Source: "https://packagespc.xboxlive.com/GetBasePackage/", ReleaseVersions: []gdkversions.Version{{Version: "Release 1.21.60.10", Timestamp: 2}, {Version: "Release 1.21.50.07", Timestamp: 3}}}
	uwp := func() ([]uwpdownload.Version, error) {
		return []uwpdownload.Version{{Version: "1.2.3.4", Type: "release"}}, nil
	}
	ctx := context.Background()
	src := Source{FetchGDK: func(context.Context) gdkversions.Catalog { return history }, LoadUWP: uwp, CachePath: cache,
		Now: func() time.Time { return time.Unix(1000, 0) }}

	// Live history: cached, not stale.
	got, err := Load(ctx, src, true, false)
	if err != nil || got.GDKStale || got.GDKMissing || len(got.Builds) != 1 {
		t.Fatalf("live: %+v %v", got, err)
	}
	// History down, Store up: cache merged with the Store build.
	src.FetchGDK = func(context.Context) gdkversions.Catalog { return store }
	got, err = Load(ctx, src, true, true)
	if err != nil || !got.GDKStale || got.CachedAt.Unix() != 1000 || len(got.Builds) != 3 {
		t.Fatalf("stale: %+v %v", got, err)
	}
	if got.Builds[0].Version != "1.21.60.10" {
		t.Errorf("merged order: %+v", got.Builds)
	}
	// Nothing reachable, no cache: GDK missing; UWP still listed.
	src.CachePath = filepath.Join(t.TempDir(), "none.json")
	src.FetchGDK = func(context.Context) gdkversions.Catalog { return gdkversions.Catalog{FileVersion: 1} }
	got, err = Load(ctx, src, true, true)
	if err != nil || !got.GDKMissing || len(got.Builds) != 1 {
		t.Fatalf("missing: %+v %v", got, err)
	}
	if _, err := Load(ctx, src, true, false); !errors.Is(err, ErrUnavailable) {
		t.Errorf("unavailable: %v", err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Load(cctx, src, true, false); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
}
