package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
)

// sysSandbox points the launcher's folders (config.json, data root) at a
// temporary directory for the test and loads a fresh config there. It never
// reads or writes the real launcher data.
func sysSandbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(root, "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "Local"))
	apppath.SetBaseRootOverride("")
	if _, err := config.Reload(); err != nil {
		t.Fatal(err)
	}
	savedGUI, savedWrite, savedProxy := cfgGUIRunning, cfgCanWriteDir, cfgEffectiveProxy
	cfgGUIRunning = func() bool { return false }
	cfgEffectiveProxy = func() string { return config.GetLipGithubProxyOverride() }
	t.Cleanup(func() {
		cfgGUIRunning, cfgCanWriteDir, cfgEffectiveProxy = savedGUI, savedWrite, savedProxy
		// Never reload here: the environment is the real one again.
		apppath.SetBaseRootOverride("")
	})
	return root
}

func TestConfigSetGetUnset(t *testing.T) {
	h := newHarness(t)
	sysSandbox(t)

	code, out, errOut := h.run("config", "set", "on-game-launch", "hide")
	if code != 0 || out != "on-game-launch: minimize -> hide\n" {
		t.Fatalf("set: %d %q %q", code, out, errOut)
	}
	if config.GetOnGameLaunch() != "hide" {
		t.Fatalf("saved %q", config.GetOnGameLaunch())
	}
	code, out, _ = h.run("config", "set", "on-game-launch", "HIDE")
	if code != 0 || !strings.Contains(out, "(unchanged)") {
		t.Fatalf("idempotent set: %d %q", code, out)
	}
	code, out, _ = h.run("config", "get", "on-game-launch", "--json")
	var e cfgEntry
	if code != 0 || json.Unmarshal([]byte(out), &e) != nil || e.Value != "hide" || e.Source != cfgSourceSet {
		t.Fatalf("get --json: %d %q", code, out)
	}
	code, out, _ = h.run("config", "unset", "on-game-launch")
	if code != 0 || out != "on-game-launch: hide -> minimize\n" {
		t.Fatalf("unset: %d %q", code, out)
	}
	code, out, _ = h.run("config", "unset", "on-game-launch")
	if code != 0 || !strings.Contains(out, "(unchanged)") {
		t.Fatalf("unset twice: %d %q", code, out)
	}

	if code, _, errOut := h.run("config", "set", "on-game-exit", "explode"); code != ExitUsage || !strings.Contains(errOut, "reopen, keep, close") {
		t.Fatalf("bad enum: %d %q", code, errOut)
	}
	if code, _, _ := h.run("config", "set", "discord", "off"); code != 0 || !cfgLoad().DisableDiscordRPC {
		t.Fatalf("discord off: %d %v", code, cfgLoad().DisableDiscordRPC)
	}
	if code, _, errOut := h.run("config", "set", "discord", "maybe"); code != ExitUsage || !strings.Contains(errOut, "[ERR_INVALID_PARAM]") {
		t.Fatalf("bad bool: %d %q", code, errOut)
	}
	if code, out, _ := h.run("config", "get", "close-to-tray"); code != 0 || out != "off\n" {
		t.Fatalf("get close-to-tray: %d %q", code, out)
	}
	h.run("config", "set", "beta-updates", "yes")
	if !cfgLoad().EnableBetaUpdates {
		t.Fatal("beta-updates yes not saved")
	}
}

func TestConfigMirrorTriState(t *testing.T) {
	h := newHarness(t)
	sysSandbox(t)
	_, out, _ := h.run("config", "get", "mirror", "--json")
	var e cfgEntry
	if json.Unmarshal([]byte(out), &e) != nil || e.Source != cfgSourceAuto {
		t.Fatalf("fresh mirror: %q", out)
	}
	h.run("config", "set", "mirror", "off")
	if v, ok := config.GetMirrorSourceOverride(); !ok || v {
		t.Fatalf("mirror off: %v %v", v, ok)
	}
	h.run("config", "unset", "mirror")
	if _, ok := config.GetMirrorSourceOverride(); ok {
		t.Fatal("unset mirror must return to detection")
	}
}

func TestConfigGithubProxy(t *testing.T) {
	h := newHarness(t)
	sysSandbox(t)
	if code, _, errOut := h.run("config", "set", "github-proxy", "ftp://example.com"); code != ExitUsage || !strings.Contains(errOut, "ERR_LIP_INVALID_GITHUB_PROXY") {
		t.Fatalf("invalid proxy: %d %q", code, errOut)
	}
	if code, out, _ := h.run("config", "set", "github-proxy", "https://proxy.example/"); code != 0 || !strings.HasSuffix(out, "-> https://proxy.example\n") {
		t.Fatalf("set proxy: %d %q", code, out)
	}
	if code, _, _ := h.run("config", "unset", "github-proxy"); code != 0 || config.GetLipGithubProxyOverride() != "" {
		t.Fatalf("unset proxy: %d %q", code, config.GetLipGithubProxyOverride())
	}
	if code, _, _ := h.run("config", "set", "github-proxy", " "); code != ExitUsage {
		t.Fatalf("empty proxy: %d", code)
	}
}

func TestConfigDefaultInstance(t *testing.T) {
	h := newHarness(t)
	sysSandbox(t)
	h.add("Survival", "1.21.50", "release", "gdk")
	if code, out, _ := h.run("config", "set", "default-instance", "survival"); code != 0 || out != "default-instance: - -> Survival\n" {
		t.Fatalf("set: %d %q", code, out)
	}
	if config.GetCurrentInstance() != "Survival" {
		t.Fatalf("saved %q", config.GetCurrentInstance())
	}
	if code, _, errOut := h.run("config", "set", "default-instance", "Creative"); code != ExitNotFound || !strings.Contains(errOut, "ERR_INSTANCE_NOT_FOUND") {
		t.Fatalf("unknown instance: %d %q", code, errOut)
	}
	h.run("config", "unset", "default-instance")
	if config.GetCurrentInstance() != "" {
		t.Fatal("unset default-instance")
	}
}

func TestConfigKeysAndReadOnly(t *testing.T) {
	h := newHarness(t)
	sysSandbox(t)
	if code, _, errOut := h.run("config", "get", "mirrr"); code != ExitUsage || !strings.Contains(errOut, `did you mean "mirror"?`) {
		t.Fatalf("unknown key: %d %q", code, errOut)
	}
	if code, _, errOut := h.run("config", "set", "logs-dir", "x"); code != ExitUsage || !strings.Contains(errOut, "read-only") {
		t.Fatalf("read-only: %d %q", code, errOut)
	}
	code, out, _ := h.run("config", "list", "--json")
	var entries []cfgEntry
	if code != 0 || json.Unmarshal([]byte(out), &entries) != nil || len(entries) != len(cfgKeys) {
		t.Fatalf("list --json: %d %q", code, out)
	}
	if code, out, _ := h.run("config", "ls", "-q"); code != 0 || !strings.HasPrefix(out, "data-root\ndefault-instance\n") {
		t.Fatalf("list -q: %d %q", code, out)
	}
	if code, out, _ := h.run("config", "list"); code != 0 || !strings.Contains(out, "KEY") || !strings.Contains(out, "config-file") {
		t.Fatalf("list: %d %q", code, out)
	}
}

func TestConfigDataRoot(t *testing.T) {
	h := newHarness(t)
	root := sysSandbox(t)
	cfgCanWriteDir = func(string) bool { return false }
	target := filepath.Join(root, "Games")
	if code, _, errOut := h.run("config", "set", "data-root", target); code != ExitConflict || !strings.Contains(errOut, "ERR_WRITE_TARGET") {
		t.Fatalf("not writable: %d %q", code, errOut)
	}
	cfgCanWriteDir = func(string) bool { return true }
	code, out, errOut := h.run("config", "set", "data-root", target)
	if code != 0 || !strings.HasSuffix(out, "-> "+target+"\n") || !strings.Contains(errOut, "were not moved") {
		t.Fatalf("set: %d %q %q", code, out, errOut)
	}
	if apppath.BaseRoot() != target {
		t.Fatalf("base root %q", apppath.BaseRoot())
	}
	if code, _, errOut := h.run("config", "unset", "data-root"); code != ExitUsage || !strings.Contains(errOut, "ERR_CONFIRMATION_REQUIRED") {
		t.Fatalf("unset without --yes: %d %q", code, errOut)
	}
	if code, _, _ := h.run("config", "unset", "data-root", "--yes"); code != 0 || cfgLoad().BaseRoot != "" {
		t.Fatalf("unset --yes: %d %q", code, cfgLoad().BaseRoot)
	}
}

func TestParseOnOff(t *testing.T) {
	for in, want := range map[string]bool{"on": true, "TRUE": true, "1": true, "off": false, "No": false} {
		if got, err := parseOnOff("k", in); err != nil || got != want {
			t.Errorf("parseOnOff(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := parseOnOff("k", "sometimes"); FromError(err).Exit != ExitUsage {
		t.Error("invalid boolean must be a usage error")
	}
}
