package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/catalog"
	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/gdkversions"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// fakeInstall is a game-install backend without network, accounts or real
// instances. Instances it creates are added to the harness.
type fakeInstall struct {
	t        *testing.T
	h        *harness
	dir      string // installers folder
	root     string // instances folder
	gdk      gdkversions.Catalog
	uwp      []uwpdownload.Version
	db       map[string][]string
	dbErr    error
	latency  map[string]int64 // URL -> ms; missing = failed
	dlErr    error
	account  bool
	licence  string
	fail     map[string]string // step -> backend code
	identity map[string]string // instance -> MicrosoftGame.config written on extraction
	uwpMeta  versions.VersionMeta
	inherit  []string
	sysData  string
	states   map[string]lip.PackageInstallState
	uninst   string
	calls    []string
	metas    map[string]versions.VersionMeta
}

func (f *fakeInstall) call(format string, a ...any) {
	f.calls = append(f.calls, strings.TrimSpace(fmt.Sprintf(format, a...)))
}

func (f *fakeInstall) called(prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func newFakeInstall(t *testing.T, h *harness) *fakeInstall {
	f := &fakeInstall{
		t: t, h: h, dir: t.TempDir(), root: t.TempDir(), account: true, licence: "authorized",
		fail: map[string]string{}, identity: map[string]string{}, metas: map[string]versions.VersionMeta{},
		gdk: gdkversions.Catalog{
			Source: "https://example/historical_versions.json",
			ReleaseVersions: []gdkversions.Version{
				{Version: "Release 1.21.50.07", URLs: []string{"http://slow/r50", "http://fast/r50", "http://down/r50"}, MD5: "m50", Timestamp: 300},
				{Version: "Release 1.21.51.02", URLs: []string{"http://fast/r51"}, MD5: "m51", Timestamp: 400},
				{Version: "Release 1.21.124.02", URLs: []string{"http://fast/r124"}, Timestamp: 450},
			},
			PreviewVersions: []gdkversions.Version{
				{Version: "Preview 1.21.60.21", URLs: []string{"http://fast/p60"}, Timestamp: 500},
			},
		},
		uwp: []uwpdownload.Version{
			{Version: "1.21.51.2", UUID: "11111111-1111-1111-1111-111111111111", Type: "release"},
			{Version: "1.16.0.51", UUID: "22222222-2222-2222-2222-222222222222", Type: "beta"},
			{Version: "1.19.60.4", UUID: "33333333-3333-3333-3333-333333333333", Type: "release"},
		},
		db: map[string][]string{
			"1.21.51.02":  {"1.9.0", "1.9.2", "1.10.0-rc.1"},
			"1.21.124.02": {"1.8.0-rc.1"},
		},
		latency: map[string]int64{"http://slow/r50": 300, "http://fast/r50": 40},
	}
	f.sysData = t.TempDir()
	saved := gi
	t.Cleanup(func() { gi = saved })
	cache := filepath.Join(t.TempDir(), "gdk.json")
	gi = gameInstallBackend{
		fetchGDK:      func(context.Context) gdkversions.Catalog { return f.gdk },
		loadUWP:       func() ([]uwpdownload.Version, error) { return f.uwp, nil },
		catalogCache:  func() string { return cache },
		installersDir: func() (string, error) { return f.dir, nil },
		probeMirrors: func(_ context.Context, urls []string, _ time.Duration) []catalog.MirrorResult {
			out := make([]catalog.MirrorResult, len(urls))
			for i, u := range urls {
				ms, ok := f.latency[u]
				out[i] = catalog.MirrorResult{URL: u, Host: catalog.MirrorHost(u), LatencyMs: ms, OK: ok, Measured: true}
				if !ok {
					out[i].LatencyMs = 7000
				}
			}
			return out
		},
		download: func(_ context.Context, src, dest string, o downloader.SyncOptions) error {
			f.call("download %s -> %s md5=%s verify=%v", src, filepath.Base(dest), o.MD5, o.Verify != nil)
			if f.dlErr != nil {
				return f.dlErr
			}
			if o.Progress != nil {
				o.Progress(downloader.DownloadProgress{Downloaded: 5, Total: 5, Dest: dest})
			}
			return os.WriteFile(dest, []byte("installer"), 0o644)
		},
		resolveUWP: func(_ context.Context, id string) (string, func(string) error, error) {
			return "http://uwp/" + id, func(string) error { return nil }, nil
		},
		removeFile:     os.Remove,
		restoreAccount: func(context.Context) error { return nil },
		accountSaved:   func() bool { return f.account },
		licenceState:   func(context.Context, string) string { f.call("licence"); return f.licence },
		validateName: func(name string) string {
			if code := versions.ValidateFolderName(name); code != "" {
				return code
			}
			for _, inst := range h.instances {
				if strings.EqualFold(inst.Name, name) {
					return "ERR_NAME_EXISTS"
				}
			}
			return ""
		},
		extractMsixvc: func(_ context.Context, path, name string) string {
			f.call("extract-gdk %s %s", filepath.Base(path), name)
			if code := f.fail["extract"]; code != "" {
				return code
			}
			dir := filepath.Join(f.root, name)
			os.MkdirAll(dir, 0o755)
			if cfg := f.identity[name]; cfg != "" {
				os.WriteFile(filepath.Join(dir, "MicrosoftGame.config"), []byte(cfg), 0o644)
			}
			return ""
		},
		extractAppx: func(_ context.Context, path, name, channel string) string {
			f.call("extract-uwp %s %s %s", filepath.Base(path), name, channel)
			if code := f.fail["extract"]; code != "" {
				return code
			}
			os.MkdirAll(filepath.Join(f.root, name), 0o755)
			m := f.uwpMeta
			m.Name, m.PackageType = name, "uwp"
			f.metas[name] = m
			return ""
		},
		instanceMeta: func(name string) versions.VersionMeta { return f.metas[name] },
		instanceDir:  func(name string) string { return filepath.Join(f.root, name) },
		saveMeta: func(name, gv, channel string, iso bool) string {
			f.call("meta %s %s %s iso=%v", name, gv, channel, iso)
			if code := f.fail["meta"]; code != "" {
				return code
			}
			pkg := "gdk"
			if m, ok := f.metas[name]; ok && m.PackageType == "uwp" {
				pkg = "uwp"
			}
			m := versions.VersionMeta{Name: name, GameVersion: gv, Type: channel, PackageType: pkg, EnableIsolation: iso}
			f.metas[name] = m
			h.instances = append(h.instances, Instance{Name: name, Dir: filepath.Join(f.root, name), Meta: m, PackageType: pkg, Channel: channel})
			return ""
		},
		inheritable:   func(string) []string { return f.inherit },
		systemDataDir: func(bool) string { return f.sysData },
		copyFromSystem: func(preview bool, name string) string {
			f.call("copy-system preview=%v %s", preview, name)
			return f.fail["copy"]
		},
		copyFromInstance: func(src, name string) string {
			f.call("copy-instance %s %s", src, name)
			return f.fail["copy"]
		},
		deleteInstance: func(name string) string {
			f.call("delete %s", name)
			for i, inst := range h.instances {
				if inst.Name == name {
					h.instances = append(h.instances[:i], h.instances[i+1:]...)
					break
				}
			}
			return ""
		},
		setDefault: func(name string) error { h.current = name; return nil },
		fetchLoaderDB: func(context.Context) (map[string][]string, error) {
			return f.db, f.dbErr
		},
		installLoader: func(_ context.Context, gv, name, v string) string {
			f.call("loader-install %s %s %s", gv, name, v)
			if code := f.fail["loader"]; code != "" {
				return code
			}
			writeLoaderManifest(t, filepath.Join(f.root, name), v)
			for _, inst := range h.instances {
				if inst.Name == name && inst.Dir != filepath.Join(f.root, name) {
					writeLoaderManifest(t, inst.Dir, v)
				}
			}
			return ""
		},
		uninstallLoader: func(_ context.Context, name string) string {
			f.call("loader-remove %s", name)
			return f.uninst
		},
		loaderStates: func(context.Context, string) (map[string]lip.PackageInstallState, error) {
			return f.states, nil
		},
	}
	return f
}

func writeLoaderManifest(t *testing.T, instDir, version string) {
	t.Helper()
	dir := filepath.Join(instDir, "mods", "LeviLamina")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]string{"name": "LeviLamina", "version": version})
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fakeInstall) file(name string, size int) string {
	p := filepath.Join(f.dir, name)
	if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func TestInstallerSearch(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	f.file("Release 1.21.50.07.msixvc", 10)
	f.file("Release 1.21.51.02.msixvc.partial.tmp", 3)
	inst := h.add("Main", "1.21.50.07", "release", "gdk")
	_ = inst

	code, out, errs := h.run("installer", "search")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Release only, newest timestamp first, GDK before UWP.
	if len(lines) != 6 || !strings.HasPrefix(lines[0], "VERSION") || !strings.HasPrefix(lines[1], "1.21.124.02") ||
		!strings.HasPrefix(lines[2], "1.21.51.02") || !strings.HasPrefix(lines[3], "1.21.50.07") || !strings.HasPrefix(lines[4], "1.21.51.2 ") {
		t.Fatalf("table:\n%s", out)
	}
	if !strings.Contains(lines[2], "partial") || !strings.Contains(lines[2], "1.9.2") || !strings.Contains(lines[3], "yes") || !strings.Contains(lines[3], "Main") {
		t.Errorf("columns:\n%s", out)
	}
	// rc-only builds show the rc.
	if !strings.Contains(lines[1], "1.8.0-rc.1") {
		t.Errorf("rc column: %s", lines[1])
	}

	code, out, _ = h.run("installer", "search", "--json", "--channel", "preview", "--channel", "beta")
	var items []installerSearchItem
	if code != 0 || json.Unmarshal([]byte(out), &items) != nil || len(items) != 2 {
		t.Fatalf("json: %d %s", code, out)
	}
	if items[0].Version != "1.21.60.21" || items[0].Released == nil || items[1].Channel != "beta" || items[1].UUID == "" || items[1].Released != nil {
		t.Errorf("json items: %+v", items)
	}

	code, out, _ = h.run("installer", "search", "--loader", "-q")
	if code != 0 || strings.TrimSpace(out) != "1.21.124.02\n1.21.51.02" {
		t.Errorf("--loader: %d %q", code, out)
	}
	code, out, _ = h.run("installer", "search", "--downloaded", "-q")
	if code != 0 || strings.TrimSpace(out) != "1.21.50.07" {
		t.Errorf("--downloaded: %q", out)
	}
	code, out, _ = h.run("installer", "search", "1.21.5", "--not-downloaded", "--type", "gdk", "-q")
	if code != 0 || strings.TrimSpace(out) != "1.21.51.02" {
		t.Errorf("query: %q", out)
	}
	code, out, errs = h.run("installer", "search", "--limit", "2")
	if code != 0 || strings.Count(strings.TrimSpace(out), "\n") != 2 || !strings.Contains(errs, "Showing 2 of 5") {
		t.Errorf("--limit: %q %q", out, errs)
	}
	for _, bad := range [][]string{
		{"installer", "search", "--downloaded", "--not-downloaded"},
		{"installer", "search", "--type", "gdk", "--channel", "beta"},
		{"installer", "search", "--loader", "--type", "uwp"},
		{"installer", "search", "--limit", "-1"},
	} {
		if code, _, _ := h.run(bad...); code != ExitUsage {
			t.Errorf("%v: exit %d", bad, code)
		}
	}
	// The loader list is optional unless --loader filters on it.
	f.dbErr = errors.New("offline")
	if code, _, errs := h.run("installer", "search"); code != 0 || !strings.Contains(errs, "LeviLamina version list is unreachable") {
		t.Errorf("db offline: %d %s", code, errs)
	}
	if code, _, _ := h.run("installer", "search", "--loader"); code != ExitNetwork {
		t.Errorf("db offline --loader: %d", code)
	}
}

func TestInstallerSearchCatalogueOffline(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	f.gdk = gdkversions.Catalog{FileVersion: 1}
	code, _, errs := h.run("installer", "search", "--type", "gdk")
	if code != ExitNetwork || !strings.Contains(errs, "ERR_CATALOG_UNAVAILABLE") {
		t.Errorf("offline gdk: %d %s", code, errs)
	}
	code, _, errs = h.run("installer", "search")
	if code != 0 || !strings.Contains(errs, "only UWP builds are listed") {
		t.Errorf("offline both: %d %s", code, errs)
	}
	if code, _, errs := h.run("installer", "download", "latest"); code != ExitNetwork {
		t.Errorf("resolve offline: %d %s", code, errs)
	}
}

func TestInstallerDownload(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)

	code, out, errs := h.run("installer", "download", "1.21.50")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	want := filepath.Join(f.dir, "Release 1.21.50.07.msixvc")
	if strings.TrimSpace(out) != want || !strings.Contains(errs, "Resolved 1.21.50 to 1.21.50.07") {
		t.Errorf("out %q errs %q", out, errs)
	}
	if len(f.calls) != 1 || f.calls[0] != "download http://fast/r50 -> Release 1.21.50.07.msixvc md5=m50 verify=false" {
		t.Errorf("calls %v", f.calls)
	}
	// Already there: nothing downloaded.
	f.calls = nil
	code, out, errs = h.run("installer", "download", "1.21.50.07", "--json")
	var res installerDownloadResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || res.Downloaded || res.Path != want || len(f.calls) != 0 {
		t.Errorf("idempotent: %d %s %v", code, out, f.calls)
	}
	// --force with an explicit mirror.
	code, _, errs = h.run("installer", "download", "1.21.50.07", "--force", "--mirror", "http://other/x")
	if code != 0 || !f.called("download http://other/x") || !strings.Contains(errs, "not one of the listed mirrors") {
		t.Errorf("force: %d %s %v", code, errs, f.calls)
	}
	// UWP: resolved through Windows Update, digest verified.
	f.calls = nil
	code, out, _ = h.run("installer", "download", "1.16.0.51", "-q")
	if code != 0 || filepath.Base(strings.TrimSpace(out)) != "Minecraft-UWP-Beta-1.16.0.51.appx" ||
		!f.called("download http://uwp/22222222-2222-2222-2222-222222222222 -> Minecraft-UWP-Beta-1.16.0.51.appx md5= verify=true") {
		t.Errorf("uwp: %d %q %v", code, out, f.calls)
	}

	for _, tc := range []struct {
		args []string
		exit int
		frag string
	}{
		{[]string{"installer", "download", "1.18"}, ExitNotFound, "ERR_BUILD_NOT_FOUND"},
		{[]string{"installer", "download", "1.21.52.01"}, ExitNotFound, "did you mean"},
		{[]string{"installer", "download", "banana"}, ExitUsage, "not a Minecraft version"},
		{[]string{"installer", "download", "1.16.0.51", "--type", "gdk", "--channel", "beta"}, ExitUsage, "beta builds exist only for UWP"},
		{[]string{"installer", "download", "1.21.51.2", "--type", "uwp", "--mirror", "http://x/y"}, ExitUsage, "--mirror applies to GDK"},
		{[]string{"installer", "download", "1.21.51", "--mirror", "ftp://x"}, ExitUsage, "--mirror must be"},
	} {
		code, _, errs := h.run(tc.args...)
		if code != tc.exit || !strings.Contains(errs, tc.frag) {
			t.Errorf("%v: exit %d (want %d) %s", tc.args, code, tc.exit, errs)
		}
	}

	for _, tc := range []struct {
		err  error
		exit int
		frag string
	}{
		{downloader.ErrBusy, ExitConflict, "ERR_DOWNLOAD_BUSY"},
		{errors.New("HTTP 503 Service Unavailable"), ExitNetwork, "ERR_DOWNLOAD_FAILED"},
		{errors.New("ERR_MD5_MISMATCH"), ExitFailure, "checksum mismatch"},
		{errors.New("ERR_UWP_INTEGRITY"), ExitFailure, "digest mismatch"},
		{context.Canceled, ExitInterrupted, "run the same command to resume"},
	} {
		f.dlErr = tc.err
		code, _, errs := h.run("installer", "download", "1.21.51.02")
		if code != tc.exit || !strings.Contains(errs, tc.frag) {
			t.Errorf("%v: exit %d (want %d) %s", tc.err, code, tc.exit, errs)
		}
	}
	f.dlErr = nil
	// A download another process is writing.
	f.file("Release 1.21.51.02.msixvc.partial.tmp", 1)
	f.file("Release 1.21.51.02.msixvc.lock", 0)
	if code, _, errs := h.run("installer", "download", "1.21.51.02"); code != ExitConflict {
		t.Errorf("busy: %d %s", code, errs)
	}
}

func TestInstallerMirrors(t *testing.T) {
	h := newHarness(t)
	newFakeInstall(t, h)
	code, out, errs := h.run("installer", "mirrors", "1.21.50.07")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[1], "* fast") || !strings.HasPrefix(lines[2], "  slow") || !strings.Contains(lines[3], "no") {
		t.Errorf("table:\n%s", out)
	}
	if code, out, _ := h.run("installer", "mirrors", "1.21.50.07", "-q"); code != 0 || strings.TrimSpace(out) != "http://fast/r50" {
		t.Errorf("-q: %q", out)
	}
	if code, _, _ := h.run("installer", "mirrors", "1.21.50.07", "--timeout", "0s"); code != ExitUsage {
		t.Errorf("timeout 0: %d", code)
	}
	if code, _, _ := h.run("installer", "mirrors", "1.16.0.51"); code != ExitNotFound {
		t.Errorf("uwp build has no mirrors: %d", code)
	}
}

func TestInstallerListAndRemove(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	f.file("Release 1.21.50.07.msixvc", 2048)
	f.file("Preview 1.21.60.21.msixvc.partial.tmp", 10)
	f.file("Minecraft-UWP-Release-1.21.50.7.appx", 5)
	f.file("1.21.51.02.msixvc", 4)

	code, out, _ := h.run("installer", "list", "--json")
	var items []installerFileItem
	if code != 0 || json.Unmarshal([]byte(out), &items) != nil || len(items) != 4 {
		t.Fatalf("list: %d %s", code, out)
	}
	if items[0].Version != "1.21.60.21" || items[0].State != "partial" || items[1].Channel != "" {
		t.Errorf("items %+v", items)
	}

	// Removing needs confirmation without a console.
	if code, _, errs := h.run("installer", "remove", "1.21.51.02"); code != ExitUsage || !strings.Contains(errs, "--yes") {
		t.Errorf("unconfirmed: %d %s", code, errs)
	}
	// 1.21.50.7 exists as GDK and UWP: ambiguous.
	if code, _, errs := h.run("installer", "remove", "1.21.50.7", "-y"); code != ExitUsage || !strings.Contains(errs, "several") {
		t.Errorf("ambiguous: %d %s", code, errs)
	}
	// Only a partial download: hint at --partial.
	if code, _, errs := h.run("installer", "remove", "1.21.60.21", "-y"); code != ExitNotFound || !strings.Contains(errs, "--partial") {
		t.Errorf("partial hint: %d %s", code, errs)
	}
	if code, _, _ := h.run("installer", "remove", "1.21.60", "-y"); code != ExitNotFound {
		t.Errorf("prefix must not match: %d", code)
	}
	code, out, _ = h.run("installer", "remove", "1.21.50.07", "--type", "gdk", "-y")
	if code != 0 || strings.TrimSpace(out) != "Removed Release 1.21.50.07.msixvc (2.0 KiB)" {
		t.Errorf("remove: %d %q", code, out)
	}
	code, out, _ = h.run("installer", "rm", "1.21.60.21", "--partial", "-y", "-q")
	if code != 0 || !strings.HasSuffix(strings.TrimSpace(out), "Preview 1.21.60.21.msixvc.partial.tmp") {
		t.Errorf("remove partial: %d %q", code, out)
	}
	// A batch with one miss: partial success.
	code, out, errs := h.run("installer", "remove", "Minecraft-UWP-Release-1.21.50.7.appx", "9.9.9.9", "-y")
	if code != ExitPartial || !strings.Contains(out, "Removed Minecraft-UWP-Release-1.21.50.7.appx") || !strings.Contains(errs, "9.9.9.9") {
		t.Errorf("batch: %d %q %q", code, out, errs)
	}
	// Paths outside the installers folder are never touched.
	outside := filepath.Join(t.TempDir(), "Release 1.0.0.0.msixvc")
	os.WriteFile(outside, []byte("x"), 0o644)
	if code, _, _ := h.run("installer", "remove", outside, "-y"); code != ExitNotFound {
		t.Errorf("outside path: %d", code)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Error("outside file removed")
	}
	if code, _, _ := h.run("installer", "remove", "latest", "-y"); code != ExitUsage {
		t.Errorf("latest: %d", code)
	}
}
