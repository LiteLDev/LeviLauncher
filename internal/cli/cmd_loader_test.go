package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/types"
)

func TestLoaderVersions(t *testing.T) {
	h := newHarness(t)
	newFakeInstall(t, h)
	main := h.add("Main", "1.21.51.02", "release", "gdk")
	writeLoaderManifest(t, main.Dir, "1.9.0")
	h.add("Old", "1.19.60.4", "release", "uwp")
	h.current = "Main"

	code, out, errs := h.run("loader", "versions")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[1], "  1.9.2") || !strings.HasPrefix(lines[2], "* 1.9.0") {
		t.Errorf("table:\n%s", out)
	}
	code, out, _ = h.run("loader", "versions", "--prerelease", "--json")
	var items []loaderVersionItem
	if code != 0 || json.Unmarshal([]byte(out), &items) != nil || len(items) != 3 || !items[0].Prerelease || items[0].Version != "1.10.0-rc.1" {
		t.Errorf("json: %d %s", code, out)
	}
	code, out, errs = h.run("loader", "versions", "--game", "1.21.124")
	if code != 0 || strings.TrimSpace(out) != "" || !strings.Contains(errs, "only has pre-release") || !strings.Contains(errs, "Resolved 1.21.124 to 1.21.124.02") {
		t.Errorf("rc only: %d %q %s", code, out, errs)
	}
	if code, _, errs := h.run("loader", "versions", "--game", "1.20"); code != ExitNotFound || !strings.Contains(errs, "does not support Minecraft 1.20") {
		t.Errorf("unsupported: %d %s", code, errs)
	}
	if code, _, _ := h.run("loader", "versions", "--game", "1.21.51", "-i", "Main"); code != ExitUsage {
		t.Errorf("--game with -i: %d", code)
	}
	if code, _, errs := h.run("loader", "versions", "-i", "Old"); code != ExitConflict || !strings.Contains(errs, "UWP") {
		t.Errorf("uwp: %d %s", code, errs)
	}
}

func TestLoaderInstall(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	main := h.add("Main", "1.21.51.02", "release", "gdk")
	h.current = "Main"

	code, out, errs := h.run("loader", "install")
	if code != 0 || strings.TrimSpace(out) != "LeviLamina 1.9.2 installed on Main" || !f.called("loader-install 1.21.51.02 Main 1.9.2") {
		t.Fatalf("install: %d %q %s %v", code, out, errs, f.calls)
	}
	writeLoaderManifest(t, main.Dir, "1.9.2")

	// Same version: nothing to do.
	f.calls = nil
	if code, out, errs := h.run("loader", "install", "1.9.2"); code != 0 || out != "" || len(f.calls) != 0 || !strings.Contains(errs, "already installed") {
		t.Errorf("same: %d %q %s", code, out, errs)
	}
	// Downgrade, reinstall and rc need confirmation.
	for _, args := range [][]string{{"loader", "install", "1.9.0"}, {"loader", "install", "--reinstall"}, {"loader", "install", "1.10.0-rc.1"}} {
		if code, _, errs := h.run(args...); code != ExitUsage || !strings.Contains(errs, "--yes") {
			t.Errorf("%v: %d %s", args, code, errs)
		}
	}
	code, out, _ = h.run("loader", "install", "1.9.0", "-y", "--json")
	var res loaderInstallResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || res.Action != "downgrade" || res.PreviousVersion == nil || *res.PreviousVersion != "1.9.2" {
		t.Errorf("downgrade: %d %s", code, out)
	}
	if code, _, errs := h.run("loader", "install", "2.0.0"); code != ExitNotFound || !strings.Contains(errs, "loader versions -i Main") {
		t.Errorf("unknown version: %d %s", code, errs)
	}
	h.add("Old", "1.21.50.07", "release", "gdk")
	if code, _, errs := h.run("loader", "install", "-i", "Old"); code != ExitUsage || !strings.Contains(errs, "ERR_LL_NOT_SUPPORTED") {
		t.Errorf("unsupported game: %d %s", code, errs)
	}
	h.add("U", "1.21.51.2", "release", "uwp")
	if code, _, _ := h.run("loader", "install", "-i", "U"); code != ExitConflict {
		t.Errorf("uwp: %d", code)
	}
	f.fail["loader"] = "ERR_LIP_INSTALL_FAILED"
	if code, _, _ := h.run("loader", "install", "1.9.2", "--reinstall", "-y"); code != ExitFailure {
		t.Errorf("lip failure: %d", code)
	}
	delete(f.fail, "loader")
	// The game is running.
	minecraftProcesses = func() []types.ProcessInfo {
		return []types.ProcessInfo{{Pid: 42, VersionName: "Main", IsLauncher: true}}
	}
	if code, _, errs := h.run("loader", "install", "1.9.2", "--reinstall", "-y"); code != ExitConflict || !strings.Contains(errs, "stop Main") {
		t.Errorf("running: %d %s", code, errs)
	}
}

func TestLoaderRemove(t *testing.T) {
	h := newHarness(t)
	f := newFakeInstall(t, h)
	main := h.add("Main", "1.21.51.02", "release", "gdk")
	h.current = "Main"

	if code, out, errs := h.run("loader", "remove"); code != 0 || out != "" || !strings.Contains(errs, "not installed") || f.called("loader-remove") {
		t.Errorf("absent: %d %q %s", code, out, errs)
	}
	writeLoaderManifest(t, main.Dir, "1.9.2")
	ll := lip.PackageKey(giLeviLaminaPackage)
	f.states = map[string]lip.PackageInstallState{ll: {ID: giLeviLaminaPackage, Installed: true, ExplicitInstalled: true}}
	if code, _, errs := h.run("loader", "remove"); code != ExitUsage || !strings.Contains(errs, "--yes") {
		t.Errorf("unconfirmed: %d %s", code, errs)
	}
	if code, out, _ := h.run("loader", "rm", "-y"); code != 0 || strings.TrimSpace(out) != "Removed LeviLamina from Main" || !f.called("loader-remove Main") {
		t.Errorf("remove: %d %q", code, out)
	}
	f.uninst = "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY"
	code, out, errs := h.run("loader", "remove", "-y", "--json")
	var res loaderRemoveResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || !res.Kept || res.Removed || !strings.Contains(errs, "kept") {
		t.Errorf("demoted: %d %s %s", code, out, errs)
	}
	f.uninst = "ERR_LIP_PACKAGE_UNINSTALL_FAILED"
	if code, _, _ := h.run("loader", "remove", "-y"); code != ExitFailure {
		t.Errorf("failure: %d", code)
	}
	f.states = map[string]lip.PackageInstallState{
		ll: {ID: giLeviLaminaPackage, Installed: true},
		lip.PackageKey("github.com/A/LegacyMoney"): {ID: "github.com/A/LegacyMoney", Installed: true, ExplicitInstalled: true},
	}
	if code, _, errs := h.run("loader", "remove", "-y"); code != ExitConflict || !strings.Contains(errs, "A/LegacyMoney") {
		t.Errorf("dependents: %d %s", code, errs)
	}
}
