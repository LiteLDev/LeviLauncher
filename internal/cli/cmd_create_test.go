package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/versions"
)

func TestInstanceCreateCatalogueBuild(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	code, out, errs := h.run("install", "1.21.51", "--loader", "--use", "--copy-data-from", "system")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	want := []string{
		"licence",
		"download http://fast/r51 -> Release 1.21.51.02.msixvc md5=m51 verify=false",
		"extract-gdk Release 1.21.51.02.msixvc 1.21.51.02",
		"meta 1.21.51.02 1.21.51.02 release iso=true",
		"copy-system preview=false 1.21.51.02",
		"loader-install 1.21.51.02 1.21.51.02 1.9.2",
	}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls\n got %q\nwant %q", f.calls, want)
	}
	if !strings.Contains(out, `Created instance "1.21.51.02" (Minecraft 1.21.51.02, GDK, release, own data, LeviLamina 1.9.2)`) {
		t.Errorf("out %q", out)
	}
	if h.current != "1.21.51.02" || !strings.Contains(errs, "play 1.21.51.02") {
		t.Errorf("default %q errs %q", h.current, errs)
	}

	// Same build again: the installer is reused, no licence check.
	f.calls = nil
	code, out, errs = h.run("instance", "create", "1.21.51.02", "--name", "Second", "--json")
	var res instanceCreateResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil {
		t.Fatalf("json: %d %s %s", code, out, errs)
	}
	if res.Name != "Second" || res.GameVersion != "1.21.51.02" || !res.Isolated || res.InstallerDownloaded ||
		filepath.Base(res.Installer) != "Release 1.21.51.02.msixvc" || f.called("licence") || f.called("download") {
		t.Errorf("reuse: %+v %v", res, f.calls)
	}

	// The name is taken: exit 4 with a free name.
	code, _, errs = h.run("install", "1.21.51")
	if code != ExitConflict || !strings.Contains(errs, "--name 1.21.51.02-2") {
		t.Errorf("name exists: %d %s", code, errs)
	}
	if code, _, errs := h.run("install", "1.21.51", "--name", "a:b"); code != ExitUsage || !strings.Contains(errs, "must not contain") {
		t.Errorf("bad name: %d %s", code, errs)
	}
}

func TestInstanceCreateRollsBack(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	f.file("Release 1.21.51.02.msixvc", 1)
	for _, step := range []string{"meta", "copy", "loader"} {
		f.calls = nil
		f.fail = map[string]string{step: "ERR_LIP_INSTALL_FAILED"}
		code, _, errs := h.run("install", "1.21.51.02", "--loader", "--copy-data-from", "system")
		if code != ExitFailure || !f.called("delete 1.21.51.02") || !strings.Contains(errs, "Removed the half-created instance") {
			t.Errorf("%s: exit %d calls %v errs %s", step, code, f.calls, errs)
		}
		if len(h.instances) != 0 {
			t.Errorf("%s: instance left behind", step)
		}
	}
	// A failed extraction created nothing: no rollback.
	f.calls = nil
	f.fail = map[string]string{"extract": "ERR_AUTH_INTERACTION_REQUIRED"}
	code, _, errs := h.run("install", "1.21.51.02")
	if code != ExitAuth || f.called("delete") || !strings.Contains(errs, "account login") {
		t.Errorf("extract: %d %v %s", code, f.calls, errs)
	}
}

func TestInstanceCreateAccountAndLicence(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	f.account = false
	if code, _, errs := h.run("install", "latest"); code != ExitAuth || f.called("download") || !strings.Contains(errs, "account login") {
		t.Errorf("no account: %d %v %s", code, f.calls, errs)
	}
	f.account = true
	for _, tc := range []struct {
		state string
		exit  int
	}{{"not_entitled", ExitAuth}, {"trial", ExitAuth}, {"error", 0}, {"authorized", 0}} {
		state, exit := tc.state, tc.exit
		f.licence, f.calls = state, nil
		h.instances = nil
		os.Remove(filepath.Join(f.dir, "Release 1.21.124.02.msixvc"))
		code, _, errs := h.run("install", "latest")
		if exit == 0 && !f.called("licence") {
			t.Errorf("licence %s: not checked", state)
		}
		if code != exit {
			t.Errorf("licence %s: %d %s", state, code, errs)
		}
		if exit != 0 && f.called("download") {
			t.Errorf("licence %s: downloaded anyway", state)
		}
	}
	// UWP builds need no account.
	f.account, f.calls = false, nil
	if code, _, errs := h.run("install", "1.21.51.2", "--type", "uwp", "--name", "U"); code != 0 || f.called("licence") {
		t.Errorf("uwp: %d %v %s", code, f.calls, errs)
	}
}

func TestInstanceCreateOptions(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	for _, tc := range []struct {
		args []string
		exit int
		frag string
	}{
		{[]string{"install", "1.21.51.2", "--type", "uwp", "--loader"}, ExitUsage, "GDK instances only"},
		{[]string{"install", "1.21.51.2", "--type", "uwp", "--copy-data-from", "system"}, ExitUsage, "GDK instances only"},
		{[]string{"install", "1.21.51", "--copy-data-from", "system", "--shared-data"}, ExitUsage, "drop --shared-data"},
		{[]string{"install", "1.21.50.07", "--loader"}, ExitUsage, "does not support Minecraft 1.21.50.07"},
		{[]string{"install", "1.21.51", "--loader-version", "2.0.0"}, ExitNotFound, "LeviLamina 2.0.0 does not support"},
		{[]string{"install", "1.21.124", "--loader"}, ExitUsage, "only has pre-release"},
		{[]string{"install", "1.21.124", "--loader-version", "1.8.0-rc.1"}, ExitUsage, "confirmation required"},
		{[]string{"install", "1.21.51", "--copy-data-from", "Nope"}, ExitNotFound, "no instance named"},
		{[]string{"install", "1.16.0.51", "--type", "gdk", "--channel", "beta"}, ExitUsage, "beta builds exist only for UWP"},
		{[]string{"install", "1.19.60.4", "--no-shared-data"}, ExitUsage, "cannot keep their own game data"},
	} {
		code, _, errs := h.run(tc.args...)
		if code != tc.exit || !strings.Contains(errs, tc.frag) {
			t.Errorf("%v: exit %d (want %d) %s", tc.args, code, tc.exit, errs)
		}
	}
	if f.called("download") || f.called("extract") {
		t.Errorf("rejected options still downloaded or extracted: %v", f.calls)
	}

	// An rc named explicitly installs with --yes.
	f.calls = nil
	if code, _, errs := h.run("install", "1.21.124", "--loader-version", "1.8.0-rc.1", "-y"); code != 0 || !f.called("loader-install 1.21.124.02 1.21.124.02 1.8.0-rc.1") {
		t.Errorf("rc: %d %v %s", code, f.calls, errs)
	}

	// Old UWP builds share the game data.
	f.calls = nil
	f.uwpMeta = versions.VersionMeta{GameVersion: "1.19.60.4", Type: "release"}
	code, _, errs := h.run("install", "1.19.60.4")
	if code != 0 || !f.called("meta 1.19.60.4 1.19.60.4 release iso=false") || !strings.Contains(errs, "shared game data") {
		t.Errorf("old uwp: %d %v %s", code, f.calls, errs)
	}

	// Copy from another instance: it must be an own-data GDK instance of the
	// same channel with game data.
	old := Instance{Name: "Old", Dir: filepath.Join(t.TempDir(), "Old"), Channel: "release", PackageType: "gdk",
		Meta: versions.VersionMeta{Name: "Old", GameVersion: "1.21.50.07", Type: "release", PackageType: "gdk", EnableIsolation: true}}
	h.instances = append(h.instances, old)
	if code, _, errs := h.run("install", "1.21.51", "--name", "New", "--copy-data-from", "old"); code != ExitUsage || !strings.Contains(errs, "no game data yet") {
		t.Errorf("not eligible: %d %s", code, errs)
	}
	f.inherit, f.calls = []string{"Old"}, nil
	if code, _, errs := h.run("install", "1.21.51", "--name", "New", "--copy-data-from", "old"); code != 0 || !f.called("copy-instance Old New") {
		t.Errorf("copy instance: %d %v %s", code, f.calls, errs)
	}
	if code, _, errs := h.run("install", "1.21.60", "--channel", "preview", "--copy-data-from", "Old"); code != ExitUsage || !strings.Contains(errs, "release instance") {
		t.Errorf("channel mismatch: %d %s", code, errs)
	}
	f.sysData = filepath.Join(t.TempDir(), "missing")
	if code, _, errs := h.run("install", "1.21.60", "--copy-data-from", "system"); code != ExitNotFound || !strings.Contains(errs, "no preview Minecraft game data") {
		t.Errorf("no system data: %d %s", code, errs)
	}
}

func TestInstanceCreateLocalFiles(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	dir := t.TempDir()
	appx := filepath.Join(dir, "Minecraft-UWP-Release-1.21.51.2.appx")
	os.WriteFile(appx, []byte("x"), 0o644)
	f.uwpMeta = versions.VersionMeta{GameVersion: "1.21.51.2", Type: "preview"}
	code, out, errs := h.run("install", appx, "-q")
	if code != 0 || strings.TrimSpace(out) != "1.21.51.2" || !f.called("extract-uwp Minecraft-UWP-Release-1.21.51.2.appx 1.21.51.2 release") ||
		!f.called("meta 1.21.51.2 1.21.51.2 preview iso=true") || f.called("download") {
		t.Errorf("appx: %d %q %v %s", code, out, f.calls, errs)
	}

	// A GDK package named by the CDN; its channel comes from the package.
	gdk := filepath.Join(dir, "game.msixvc")
	os.WriteFile(gdk, []byte("x"), 0o644)
	if code, _, errs := h.run("install", gdk); code != ExitUsage || !strings.Contains(errs, "--name") {
		t.Errorf("unnamed: %d %s", code, errs)
	}
	f.identity["Mine"] = `<Game><Identity Name="Microsoft.MinecraftWindowsBeta" Version="1.21.12021.0"/></Game>`
	f.calls = nil
	code, _, errs = h.run("install", gdk, "--name", "Mine", "--channel", "release")
	if code != 0 || !f.called("meta Mine 1.21.120.21 preview iso=true") || !strings.Contains(errs, "preview build") || f.called("licence") {
		t.Errorf("gdk local: %d %v %s", code, f.calls, errs)
	}
	// Without an identity the channel defaults to release, with a warning.
	f.calls = nil
	code, _, errs = h.run("install", gdk, "--name", "Unknown")
	if code != 0 || !f.called("meta Unknown Unknown release iso=true") || !strings.Contains(errs, "channel of game.msixvc is unknown") {
		t.Errorf("gdk unknown: %d %v %s", code, f.calls, errs)
	}

	for _, tc := range []struct {
		args []string
		exit int
	}{
		{[]string{"install", filepath.Join(dir, "missing.msixvc")}, ExitNotFound},
		{[]string{"install", appx, "--type", "gdk"}, ExitUsage},
		{[]string{"install", gdk, "--name", "X", "--mirror", "http://a/b"}, ExitUsage},
		{[]string{"install", appx, "--name", "Y", "--loader"}, ExitUsage},
	} {
		if code, _, errs := h.run(tc.args...); code != tc.exit {
			t.Errorf("%v: exit %d (want %d) %s", tc.args, code, tc.exit, errs)
		}
	}
	txt := filepath.Join(dir, "notes.txt")
	os.WriteFile(txt, []byte("x"), 0o644)
	if code, _, _ := h.run("install", txt); code != ExitUsage {
		t.Errorf("not an installer: %d", code)
	}
}

func TestLooksLikeFile(t *testing.T) {
	for arg, want := range map[string]bool{
		"latest": false, "1.21.50": false, "x.msixvc": true, `D:\a\b`: true, "./a": true, "nonsense": false,
	} {
		if got := giLooksLikeFile(arg); got != want {
			t.Errorf("giLooksLikeFile(%q) = %v", arg, got)
		}
	}
}
