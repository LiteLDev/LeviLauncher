package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

type syncMutex = sync.Mutex

// harness runs invocations against fake instances without touching the
// launcher data, the registry of installed packages or running processes.
type harness struct {
	t         *testing.T
	instances []Instance
	current   string
	env       map[string]string
	stdin     string
	console   bool
}

func newHarness(t *testing.T) *harness {
	h := &harness{t: t, env: map[string]string{}}
	saved := []any{listInstances, defaultInstanceCfg, bootstrap, registeredLocations, minecraftProcesses, notifyInterrupt, usersRootOf, gamertagCache, signedInGamertag}
	t.Cleanup(func() {
		listInstances = saved[0].(func() ([]Instance, error))
		defaultInstanceCfg = saved[1].(func() string)
		bootstrap = saved[2].(func() error)
		registeredLocations = saved[3].(func() (string, string))
		minecraftProcesses = saved[4].(func() []types.ProcessInfo)
		notifyInterrupt = saved[5].(func(chan<- os.Signal) func())
		usersRootOf = saved[6].(func(Instance) string)
		gamertagCache = saved[7].(func() map[string]string)
		signedInGamertag = saved[8].(func(context.Context) string)
	})
	listInstances = func() ([]Instance, error) {
		out := append([]Instance(nil), h.instances...)
		sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
		return out, nil
	}
	defaultInstanceCfg = func() string { return h.current }
	bootstrap = func() error { return nil }
	registeredLocations = func() (string, string) { return "", "" }
	minecraftProcesses = func() []types.ProcessInfo { return nil }
	notifyInterrupt = func(chan<- os.Signal) func() { return func() {} }
	gamertagCache = func() map[string]string { return map[string]string{} }
	signedInGamertag = func(context.Context) string { return "" }
	return h
}

func (h *harness) add(name, gameVersion, channel, pkg string) Instance {
	dir := filepath.Join(h.t.TempDir(), name)
	inst := Instance{Name: name, Dir: dir, Channel: channel, PackageType: pkg,
		Meta: versions.VersionMeta{Name: name, GameVersion: gameVersion, Type: channel, PackageType: pkg}}
	h.instances = append(h.instances, inst)
	return inst
}

func (h *harness) run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	s := &Streams{
		Stdin: strings.NewReader(h.stdin), Stdout: &out, Stderr: &errb,
		StdinConsole: h.console, StdoutConsole: false, StderrConsole: h.console,
		Getenv: func(k string) string { return h.env[k] },
	}
	code := run(args, s)
	return code, out.String(), errb.String()
}

func TestWantsKeepsLegacyGUIForms(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{""}, false},
		{[]string{"--debug"}, false},
		{[]string{"-debug"}, false},
		{[]string{"/debug"}, false},
		{[]string{"debug"}, false},
		{[]string{"DEBUG"}, false},
		{[]string{"--launch=1.21.50"}, false},
		{[]string{"--launch=My Instance", "--debug"}, false},
		{[]string{"--self-update=1.0.2"}, false},
		{[]string{"--post-update-restart"}, false},
		{[]string{"--self-update=1.0.2", "--post-update-restart"}, false},
		{[]string{"--webview2-runtime-dir", `D:\wv2`}, false},
		{[]string{"--webview2-runtime-dir=D:\\wv2"}, false},
		{[]string{"--uninstall-cleanup", "--delete-versions=1"}, false},
		{[]string{"--some-future-gui-flag"}, false},
		{[]string{"/unknown"}, false},
		{[]string{"-v", "--launch=x"}, false},
		// Command line.
		{[]string{"version"}, true},
		{[]string{"instance", "list"}, true},
		{[]string{"lanch"}, true},
		{[]string{"--json", "instance", "list"}, true},
		{[]string{"-i", "survival", "world", "list"}, true},
		{[]string{"-i", "debug", "world", "list"}, true},
		{[]string{"--instance=x", "-qy", "mod", "list"}, true},
		{[]string{"--help"}, true},
		{[]string{"-h"}, true},
		{[]string{"/?"}, true},
		{[]string{"--version"}, true},
		{[]string{"--json"}, true},
		{[]string{"--no-json", "version"}, true},
		{[]string{"instance", "list", "--debug"}, true},
	} {
		if got := Wants(tc.args); got != tc.want {
			t.Errorf("Wants(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestRegistryIsConsistent(t *testing.T) {
	tr := registry()
	for _, c := range tr.cmds {
		if strings.TrimSpace(c.Summary) == "" {
			t.Errorf("%s: no summary", c.Path)
		}
		for _, ex := range c.Examples {
			if _, err := resolveCommand(tr, strings.Fields(ex), newFlagValues()); err != nil {
				t.Errorf("%s: example %q does not resolve: %v", c.Path, ex, err)
			}
		}
	}
	if c := tr.find("instance ls"); c == nil || c.Path != "instance list" {
		t.Fatal("automatic ls alias missing")
	}
}

func TestBuildTreeRejectsDuplicates(t *testing.T) {
	noop := func(*Ctx) error { return nil }
	n := &Noun{Name: "thing"}
	if _, err := buildTree([]*Command{{Path: "thing list", Summary: "x", Run: noop}, {Path: "thing list", Summary: "y", Run: noop}}, []*Noun{n}, nil); err == nil {
		t.Error("duplicate verb accepted")
	}
	if _, err := buildTree([]*Command{{Path: "other list", Summary: "x", Run: noop}}, []*Noun{{Name: "thing"}}, nil); err == nil {
		t.Error("unknown noun accepted")
	}
	if _, err := buildTree([]*Command{{Path: "debug", Summary: "x", Run: noop}}, nil, nil); err == nil {
		t.Error("reserved word debug accepted")
	}
	if _, err := buildTree([]*Command{{Path: "x", Summary: "x", Run: noop, Flags: []*Flag{BoolFlag("json", "", "")}}}, nil, nil); err == nil {
		t.Error("flag clashing with a global accepted")
	}
}

func TestFlagParsing(t *testing.T) {
	flags := []*Flag{
		BoolFlag("force", "f", ""),
		StringFlag("name", "n", "<n>", ""),
		StringsFlag("env", "e", "<k=v>", ""),
		EnumFlag("type", "", []string{"gdk", "uwp"}, ""),
		IntFlag("limit", "", "<n>", ""),
		DurationFlag("timeout", "", ""),
	}
	parse := func(args ...string) (*flagValues, []string, error) {
		v := newFlagValues()
		p := &parser{flags: append(append([]*Flag{}, flags...), globalFlags...), values: v}
		pos, _, err := p.parse(args, false)
		return v, pos, err
	}
	v, pos, err := parse("a", "--name", "x y", "-fq", "--env=A=1", "-e", "B=2", "b", "--type", "UWP", "--limit=3", "--timeout", "90s", "--", "--force", "-c")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(pos, "|"); got != "a|b|--force|-c" {
		t.Errorf("positional = %q", got)
	}
	if v.vals["name"] != "x y" || v.vals["force"] != true || v.vals["quiet"] != true || v.vals["type"] != "uwp" ||
		v.vals["limit"] != 3 || v.vals["timeout"] != 90*time.Second {
		t.Errorf("values = %#v", v.vals)
	}
	if e := v.vals["env"].([]string); len(e) != 2 || e[0] != "A=1" || e[1] != "B=2" {
		t.Errorf("env = %q", e)
	}
	v, _, err = parse("--force", "--no-force", "-nvalue", "-ix")
	if err != nil {
		t.Fatal(err)
	}
	if v.vals["force"] != false || !v.set["force"] || v.vals["name"] != "value" || v.vals["instance"] != "x" {
		t.Errorf("values = %#v", v.vals)
	}
	for _, bad := range [][]string{{"--nope"}, {"--name"}, {"--type", "msi"}, {"--limit", "x"}, {"--timeout", "soon"}, {"-z"}, {"--force=maybe"}, {"--no-name"}} {
		if _, _, err := parse(bad...); err == nil {
			t.Errorf("parse(%q) accepted", bad)
		} else if FromError(err).Exit != ExitUsage {
			t.Errorf("parse(%q) exit = %d", bad, FromError(err).Exit)
		}
	}
	_, _, err = parse("--forse")
	if e := FromError(err); !strings.Contains(e.Hint, "--force") {
		t.Errorf("no suggestion for --forse: %+v", e)
	}
}

func TestHelp(t *testing.T) {
	h := newHarness(t)
	code, out, _ := h.run("help")
	if code != 0 || !strings.Contains(out, "Manage an instance:") || !strings.Contains(out, "instance") ||
		!strings.Contains(out, "Global flags:") || !strings.Contains(out, "Exit codes:") {
		t.Fatalf("overview: code %d\n%s", code, out)
	}
	for _, gone := range []string{"--color", "--lang", "--data-root", "completion", "call"} {
		if strings.Contains(out, gone) {
			t.Errorf("overview mentions %q", gone)
		}
	}
	code, out2, _ := h.run("--help")
	if code != 0 || out2 != out {
		t.Errorf("--help differs from help")
	}
	code, out, _ = h.run("help", "instance", "list")
	if code != 0 || !strings.HasPrefix(out, "Usage:\n  LeviLauncher.exe instance list [<filter>] [flags]") || !strings.Contains(out, "Examples:") || !strings.Contains(out, "--channel") {
		t.Fatalf("command help: code %d\n%s", code, out)
	}
	if strings.Index(out, "Examples:") > strings.Index(out, "Flags:") {
		t.Error("examples must come before flags")
	}
	code, out2, _ = h.run("instance", "ls", "-h")
	if code != 0 || out2 != out {
		t.Errorf("-h differs from help <cmd>:\n%s", out2)
	}
	code, out, _ = h.run("help", "instance")
	if code != 0 || !strings.Contains(out, "Commands:") || !strings.Contains(out, "list") {
		t.Fatalf("noun help: code %d\n%s", code, out)
	}
	code, _, errOut := h.run("instance")
	if code != ExitUsage || !strings.Contains(errOut, "Commands:") {
		t.Errorf("bare noun: code %d\n%s", code, errOut)
	}
	code, _, errOut = h.run("--json")
	if code != ExitUsage || !strings.Contains(errOut, "Usage:") {
		t.Errorf("globals alone: code %d", code)
	}
	code, _, errOut = h.run("help", "lanch")
	if code != ExitUsage || !strings.Contains(errOut, "unknown command") {
		t.Errorf("unknown help topic: code %d\n%s", code, errOut)
	}
}

func TestUnknownCommandSuggests(t *testing.T) {
	h := newHarness(t)
	code, out, errOut := h.run("versoin")
	if code != ExitUsage || out != "" {
		t.Fatalf("code %d, stdout %q", code, out)
	}
	if !strings.Contains(errOut, `error: unknown command "versoin"`) || !strings.Contains(errOut, `hint: did you mean "version"?`) || !strings.Contains(errOut, "[ERR_USAGE]") {
		t.Fatalf("stderr:\n%s", errOut)
	}
	code, _, errOut = h.run("instance", "lst")
	if code != ExitUsage || !strings.Contains(errOut, `"instance list"`) {
		t.Fatalf("verb suggestion: %d\n%s", code, errOut)
	}
	code, _, errOut = h.run("version", "--nope")
	if code != ExitUsage || !strings.Contains(errOut, "unknown flag --nope") {
		t.Fatalf("unknown flag: %d\n%s", code, errOut)
	}
	code, _, errOut = h.run("version", "extra")
	if code != ExitUsage || !strings.Contains(errOut, "takes no arguments") {
		t.Fatalf("extra argument: %d\n%s", code, errOut)
	}
}

func TestMistypedVerbUnderDefault(t *testing.T) {
	h := newHarness(t)
	h.add("Alpha123", "1.21.50", "release", "gdk")
	for _, args := range [][]string{
		{"instance", "remov", "foo"},
		{"instance", "lst", "-h"},
		{"help", "instance", "lst"},
	} {
		code, out, errOut := h.run(args...)
		if code != ExitUsage || out != "" || !strings.Contains(errOut, `unknown command "instance `) || !strings.Contains(errOut, "did you mean") {
			t.Errorf("%v: code %d\nstdout %q\nstderr %s", args, code, out, errOut)
		}
	}
	// help takes command words only.
	code, _, errOut := h.run("help", "instance", "Alpha123")
	if code != ExitUsage || !strings.Contains(errOut, `unknown command "instance Alpha123"`) || !strings.Contains(errOut, "help instance") {
		t.Errorf("help with an instance name: code %d\n%s", code, errOut)
	}
	// An instance name with -h still shows the default verb's help.
	code, out, _ := h.run("instance", "Alpha123", "-h")
	if code != 0 || !strings.Contains(out, "instance show") {
		t.Errorf("instance <name> -h: code %d\n%s", code, out)
	}
	// Too many arguments for a word that is no verb keep the usage error.
	code, _, errOut = h.run("instance", "Alpha123", "extra")
	if code != ExitUsage || !strings.Contains(errOut, "too many arguments") {
		t.Errorf("too many arguments: code %d\n%s", code, errOut)
	}
}

func TestJSONErrorAfterBadToken(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{
		{"instance", "list", "--bogus", "--json"},
		{"instnace", "list", "--json"},
		{"versoin", "--json=true"},
	} {
		code, _, errOut := h.run(args...)
		var body struct {
			Error struct{ Code string } `json:"error"`
		}
		if code != ExitUsage || json.Unmarshal([]byte(errOut), &body) != nil || body.Error.Code != "ERR_USAGE" {
			t.Errorf("%v: code %d, stderr %q", args, code, errOut)
		}
	}
	for _, args := range [][]string{
		{"versoin", "--json", "--no-json"},
		{"versoin", "--", "--json"},
	} {
		if _, _, errOut := h.run(args...); !strings.HasPrefix(errOut, "error: ") {
			t.Errorf("%v: stderr %q", args, errOut)
		}
	}
}

func TestVersion(t *testing.T) {
	h := newHarness(t)
	code, out, _ := h.run("version")
	if code != 0 || !strings.HasPrefix(out, "LeviLauncher ") || !strings.Contains(out, repoURL) {
		t.Fatalf("version: %d\n%s", code, out)
	}
	code, out, _ = h.run("--version", "-q")
	if code != 0 || strings.Contains(out, " ") || strings.Count(out, "\n") != 1 {
		t.Fatalf("--version -q: %d %q", code, out)
	}
	code, out, _ = h.run("version", "--json")
	var v versionResult
	if code != 0 || json.Unmarshal([]byte(out), &v) != nil || v.Version == "" || v.Repo != repoURL {
		t.Fatalf("version --json: %d %q", code, out)
	}
}

func TestInstanceList(t *testing.T) {
	h := newHarness(t)
	code, out, errOut := h.run("instance", "list")
	if code != 0 || out != "" || !strings.Contains(errOut, "install latest") {
		t.Fatalf("empty list: %d %q %q", code, out, errOut)
	}
	code, out, _ = h.run("instance", "list", "--json")
	if code != 0 || strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty json list: %d %q", code, out)
	}
	h.add("Old", "1.20.10.01", "release", "gdk")
	pre := h.add("Pre", "1.21.60.21", "preview", "gdk")
	h.add("New", "1.21.60.10", "release", "uwp")
	h.add("Beta", "1.21.60.10", "beta", "uwp")
	h.current = "old"
	registeredLocations = func() (string, string) { return "", strings.ToLower(pre.Dir) }
	minecraftProcesses = func() []types.ProcessInfo {
		return []types.ProcessInfo{{Pid: 42, IsLauncher: true, VersionName: "pre"}, {Pid: 7, VersionName: "Old"}}
	}
	code, out, _ = h.run("ls", "-q")
	if code != ExitUsage {
		t.Fatalf("top-level ls must not exist: %d", code)
	}
	code, out, _ = h.run("instance", "ls", "-q")
	if code != 0 || out != "Pre\nNew\nBeta\nOld\n" {
		t.Fatalf("sorted names: %d %q", code, out)
	}
	code, out, _ = h.run("instance", "list")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if code != 0 || len(lines) != 5 || !strings.Contains(lines[0], "GAME VERSION") ||
		!strings.HasPrefix(lines[4], "* Old") || !strings.Contains(lines[1], "running(42)") {
		t.Fatalf("table: %d\n%s", code, out)
	}
	code, out, _ = h.run("--json", "instance", "list", "--channel", "preview", "--channel", "beta", "--type=gdk")
	var items []instanceListItem
	if code != 0 || json.Unmarshal([]byte(out), &items) != nil || len(items) != 1 {
		t.Fatalf("filtered json: %d %q", code, out)
	}
	it := items[0]
	if it.Name != "Pre" || !it.Registered || !it.Running || len(it.PIDs) != 1 || it.PIDs[0] != 42 || it.IsDefault || it.LoaderVersion != nil {
		t.Fatalf("item = %+v", it)
	}
	code, out, _ = h.run("instance", "list", "UWP", "-q")
	if code != 0 || out != "New\nBeta\n" {
		t.Fatalf("text filter: %d %q", code, out)
	}
	code, _, errOut = h.run("instance", "list", "--type", "msi")
	if code != ExitUsage || !strings.Contains(errOut, "gdk, uwp") {
		t.Fatalf("bad enum: %d %s", code, errOut)
	}
}

func TestLoaderVersionReadsManifest(t *testing.T) {
	dir := t.TempDir()
	mod := filepath.Join(dir, "mods", "LeviLamina")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "manifest.json.close"), []byte(`{"name":"LeviLamina","version":"1.4.2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := LoaderVersion(Instance{Dir: dir}); v != "1.4.2" {
		t.Fatalf("LoaderVersion = %q", v)
	}
	if v := LoaderVersion(Instance{Dir: t.TempDir()}); v != "" {
		t.Fatalf("LoaderVersion without mods = %q", v)
	}
}

func resolveWith(h *harness, globals Globals, arg string) (Instance, string, error) {
	var errb bytes.Buffer
	c := testCtx(h, globals, &errb)
	inst, err := c.ResolveInstance(arg)
	return inst, errb.String(), err
}

func testCtx(h *harness, g Globals, errw *bytes.Buffer) *Ctx {
	s := &Streams{Stdin: strings.NewReader(h.stdin), Stdout: &bytes.Buffer{}, Stderr: errw,
		StdinConsole: h.console, StderrConsole: h.console, Getenv: func(k string) string { return h.env[k] }}
	cmd := &Command{Path: "test"}
	return &Ctx{Cmd: cmd, G: g, Stdout: s.Stdout, Stderr: errw, Stdin: s.Stdin, ctx: context.Background(),
		errw: errw, streams: s, values: newFlagValues(), errMu: new(syncMutex)}
}

func TestResolveInstance(t *testing.T) {
	h := newHarness(t)
	_, _, err := resolveWith(h, Globals{}, "")
	if e := FromError(err); e.Exit != ExitNotFound || !strings.Contains(e.Hint, "install latest") {
		t.Fatalf("no instances: %+v", e)
	}
	h.add("Survival", "1.21.50", "release", "gdk")
	inst, echo, err := resolveWith(h, Globals{}, "")
	if err != nil || inst.Name != "Survival" || !strings.Contains(echo, "Using instance Survival (only instance)") {
		t.Fatalf("only instance: %v %q %v", inst.Name, echo, err)
	}
	_, echo, _ = resolveWith(h, Globals{Quiet: true}, "")
	if echo != "" {
		t.Fatalf("-q must suppress the echo: %q", echo)
	}
	h.add("Creative", "1.21.60", "preview", "gdk")
	_, _, err = resolveWith(h, Globals{}, "")
	if e := FromError(err); e.Exit != ExitUsage || !strings.Contains(e.Message, "Creative, Survival") || !strings.Contains(e.Hint, "LeviLauncher.exe use <name>") {
		t.Fatalf("ambiguous: %+v", e)
	}
	h.current = "creative"
	inst, echo, _ = resolveWith(h, Globals{}, "")
	if inst.Name != "Creative" || !strings.Contains(echo, "(default instance)") {
		t.Fatalf("setting: %q %q", inst.Name, echo)
	}
	h.env[EnvInstance] = "SURVIVAL"
	inst, echo, _ = resolveWith(h, Globals{}, "")
	if inst.Name != "Survival" || !strings.Contains(echo, EnvInstance) {
		t.Fatalf("env: %q %q", inst.Name, echo)
	}
	inst, echo, _ = resolveWith(h, Globals{Instance: "creative"}, "")
	if inst.Name != "Creative" || echo != "" {
		t.Fatalf("flag: %q %q", inst.Name, echo)
	}
	inst, _, _ = resolveWith(h, Globals{Instance: "creative"}, "Creative")
	if inst.Name != "Creative" {
		t.Fatal("argument equal to flag rejected")
	}
	if _, _, err = resolveWith(h, Globals{Instance: "creative"}, "survival"); FromError(err).Exit != ExitUsage {
		t.Fatalf("argument contradicting -i: %v", err)
	}
	_, _, err = resolveWith(h, Globals{}, "Survivl")
	if e := FromError(err); e.Exit != ExitNotFound || e.Code != "ERR_INSTANCE_NOT_FOUND" || !strings.Contains(e.Hint, `"Survival"`) {
		t.Fatalf("near miss: %+v", e)
	}
	if _, _, err = resolveWith(h, Globals{}, "Surv"); FromError(err).Exit != ExitNotFound {
		t.Fatal("prefix matched")
	}
	h.env[EnvInstance] = ""
	h.current = "gone"
	_, _, err = resolveWith(h, Globals{}, "")
	if e := FromError(err); e.Exit != ExitNotFound || !strings.Contains(e.Message, "no longer installed") {
		t.Fatalf("stale default: %+v", e)
	}
}

func TestResolvePlayer(t *testing.T) {
	h := newHarness(t)
	gdk := h.add("G", "1.21.50", "release", "gdk")
	uwp := h.add("U", "1.21.50", "release", "uwp")
	users := t.TempDir()
	usersRootOf = func(Instance) string { return users }
	for _, d := range []string{"Shared", "9556213259376595538", "111"} {
		_ = os.MkdirAll(filepath.Join(users, d), 0o755)
	}
	gamertagCache = func() map[string]string { return map[string]string{"111": "Alex", "222": "Steve"} }
	var errb bytes.Buffer
	c := testCtx(h, Globals{}, &errb)
	p, err := c.ResolvePlayer(gdk)
	if err != nil || p.ID != "111" || p.Gamertag != "Alex" || !strings.Contains(errb.String(), "only profile") {
		t.Fatalf("only player: %+v %v %q", p, err, errb.String())
	}
	_ = os.MkdirAll(filepath.Join(users, "222"), 0o755)
	if _, err := c.ResolvePlayer(gdk); FromError(err).Exit != ExitUsage {
		t.Fatalf("ambiguous players: %v", err)
	}
	signedInGamertag = func(context.Context) string { return "steve" }
	if p, err = c.ResolvePlayer(gdk); err != nil || p.ID != "222" {
		t.Fatalf("signed-in player: %+v %v", p, err)
	}
	c.G.Player = "ALEX"
	if p, err = c.ResolvePlayer(gdk); err != nil || p.ID != "111" {
		t.Fatalf("-p gamertag: %+v %v", p, err)
	}
	c.G.Player = "333"
	if _, err = c.ResolvePlayer(gdk); FromError(err).Exit != ExitNotFound {
		t.Fatalf("unknown player: %v", err)
	}
	if p, err = c.ResolvePlayer(uwp); err != nil || p.ID != "" {
		t.Fatalf("UWP: %+v %v", p, err)
	}
}

func TestConfirm(t *testing.T) {
	h := newHarness(t)
	var errb bytes.Buffer
	c := testCtx(h, Globals{}, &errb)
	err := c.Confirm("Remove instance X?")
	if e := FromError(err); e.Exit != ExitUsage || e.Code != "ERR_CONFIRMATION_REQUIRED" || !strings.Contains(e.Hint, "--yes") {
		t.Fatalf("no console: %+v", e)
	}
	if e := FromError(err); e.Message != "confirmation required: Remove instance X?" {
		t.Fatalf("message: %q", e.Message)
	}
	multi := "Remove LeviLamina 1.0 from Alpha? Mods that need it will no longer load."
	if e := FromError(c.Confirm(multi)); e.Message != "confirmation required: "+multi {
		t.Fatalf("multi-sentence message: %q", e.Message)
	}
	if e := FromError(c.ConfirmWith("Overwrite?", "--overwrite")); !strings.Contains(e.Hint, "--overwrite") {
		t.Fatalf("policy flag hint: %+v", e)
	}
	c.G.Yes = true
	if err := c.Confirm("Remove?"); err != nil {
		t.Fatal(err)
	}
	h.console = true
	h.stdin = "y\nn\n"
	c = testCtx(h, Globals{}, &errb)
	if err := c.Confirm("First?"); err != nil {
		t.Fatalf("yes answer: %v", err)
	}
	if e := FromError(c.Confirm("Second?")); e.Exit != ExitCancelled {
		t.Fatalf("no answer: %+v", e)
	}
	h.env["CI"] = "true"
	c = testCtx(h, Globals{}, &errb)
	if c.CanPrompt() {
		t.Fatal("CI must disable prompts")
	}
}

func TestErrorMapping(t *testing.T) {
	RegisterBackendErrors(map[string]BackendError{"ERR_TEST_THING": {Exit: ExitConflict, Message: "thing is busy", Hint: "wait"}})
	for _, tc := range []struct {
		in   string
		exit int
		code string
		msg  string
	}{
		{"ERR_TEST_THING", ExitConflict, "ERR_TEST_THING", "thing is busy"},
		{"ERR_GAME_ALREADY_RUNNING", ExitConflict, "ERR_GAME_ALREADY_RUNNING", "Minecraft is already running for this instance"},
		{"ERR_LAUNCH_TIMEOUT", ExitTimeout, "ERR_LAUNCH_TIMEOUT", ""},
		{"ERR_UWP_PREPARE: console mods need x64", ExitFailure, "ERR_UWP_PREPARE", "could not prepare the UWP game files for registration"},
		{"ERR_UPDATE_ADMIN_REQUIRED: access denied", ExitSystem, "ERR_UPDATE_ADMIN_REQUIRED", "the launcher folder is not writable"},
		{"ERR_AUTH_SOMETHING_NEW", ExitAuth, "ERR_AUTH_SOMETHING_NEW", ""},
		{"ERR_WORLD_NOT_FOUND", ExitNotFound, "ERR_WORLD_NOT_FOUND", ""},
		{"ERR_NAME_EXISTS", ExitConflict, "ERR_NAME_EXISTS", ""},
		{"ERR_CANCELED", ExitCancelled, "ERR_CANCELED", "cancelled"},
		{"ERR_X::detail text", ExitFailure, "ERR_X", "detail text"},
		{"plain failure", ExitFailure, "ERR_FAILED", "plain failure"},
	} {
		e := Backend(tc.in)
		if e.Exit != tc.exit || e.Code != tc.code || (tc.msg != "" && e.Message != tc.msg) {
			t.Errorf("Backend(%q) = %+v", tc.in, e)
		}
	}
	if Backend("") != nil {
		t.Error("empty result is not an error")
	}
	if e := FromError(context.Canceled); e.Exit != ExitInterrupted {
		t.Errorf("canceled: %+v", e)
	}
	if e := FromError(errors.New("ERR_NOT_FOUND_EXE")); e.Exit != ExitNotFound || e.Hint == "" {
		t.Errorf("wrapped backend: %+v", e)
	}
	if e := FromError(UsageError("x")); e.Exit != ExitUsage {
		t.Error("*Error not passed through")
	}

	var buf bytes.Buffer
	code := printError(&buf, new(syncMutex), Globals{}, Backend("ERR_TEST_THING"))
	if code != ExitConflict || buf.String() != "error: thing is busy\nhint: wait\n[ERR_TEST_THING]\n" {
		t.Errorf("text form: %d %q", code, buf.String())
	}
	buf.Reset()
	printError(&buf, new(syncMutex), Globals{JSON: true}, Backend("ERR_TEST_THING"))
	var doc struct {
		Error struct{ Code, Message, Hint string } `json:"error"`
	}
	if json.Unmarshal(buf.Bytes(), &doc) != nil || doc.Error.Code != "ERR_TEST_THING" || doc.Error.Hint != "wait" {
		t.Errorf("json form: %q", buf.String())
	}
}

func TestExitCodesFromCommands(t *testing.T) {
	h := newHarness(t)
	codes := map[string]error{
		"ok": nil, "exit": ExitCode(7), "usage": UsageError("bad"),
		"backend": Backend("ERR_LAUNCH_TIMEOUT"), "plain": errors.New("boom"), "panic": nil,
	}
	saved := pending
	built = nil
	t.Cleanup(func() { regMu.Lock(); pending, built = saved, nil; regMu.Unlock() })
	for name, err := range codes {
		err := err
		run := func(*Ctx) error { return err }
		if name == "panic" {
			run = func(*Ctx) error { panic("oops") }
		}
		Register(&Command{Path: "zz-" + name, Summary: "test", Hidden: true, Run: run})
	}
	for name, want := range map[string]int{"ok": 0, "exit": 7, "usage": 2, "backend": 9, "plain": 1, "panic": 1} {
		code, _, errOut := h.run("zz-" + name)
		if code != want {
			t.Errorf("%s: exit %d, want %d (%s)", name, code, want, errOut)
		}
	}
	code, out, errOut := h.run("--json", "zz-usage")
	if code != 2 || out != "" || !strings.HasPrefix(errOut, `{"error":`) {
		t.Errorf("json error: %d %q %q", code, out, errOut)
	}
}

func TestProgressModes(t *testing.T) {
	h := newHarness(t)
	var errb bytes.Buffer
	c := testCtx(h, Globals{}, &errb)
	c.Progress().Phase("Downloading")
	c.Progress().Bytes(50, 100)
	if errb.Len() != 0 {
		t.Fatalf("progress without console or -v: %q", errb.String())
	}
	c = testCtx(h, Globals{Verbose: true}, &errb)
	c.Progress().Phase("Downloading")
	c.Progress().Bytes(50, 100)
	if errb.String() != "Downloading\n" {
		t.Fatalf("-v without console: %q", errb.String())
	}
	errb.Reset()
	h.console = true
	c = testCtx(h, Globals{}, &errb)
	p := c.Progress()
	p.width = func() int { return 80 }
	p.Phase("Downloading")
	p.Bytes(100, 100)
	c.Notice("done")
	if s := errb.String(); !strings.Contains(s, "\rDownloading  100%") || !strings.HasSuffix(s, "\rdone\n") {
		t.Fatalf("console progress: %q", s)
	}
	errb.Reset()
	c = testCtx(h, Globals{JSON: true}, &errb)
	c.Progress().Phase("x")
	if errb.Len() != 0 {
		t.Fatal("--json must silence progress")
	}
}

func TestInterruptExits130(t *testing.T) {
	h := newHarness(t)
	sigs := make(chan chan<- os.Signal, 1)
	notifyInterrupt = func(ch chan<- os.Signal) func() { sigs <- ch; return func() {} }
	var forced int
	savedExit := exit
	exit = func(code int) { forced = code }
	saved := pending
	built = nil
	t.Cleanup(func() { exit = savedExit; regMu.Lock(); pending, built = saved, nil; regMu.Unlock() })
	started := make(chan struct{})
	Register(&Command{Path: "zz-block", Summary: "test", Hidden: true, Run: func(c *Ctx) error {
		close(started)
		<-c.Context().Done()
		return c.Context().Err()
	}})
	go func() {
		ch := <-sigs
		<-started
		ch <- os.Interrupt
	}()
	code, _, errOut := h.run("zz-block")
	if code != ExitInterrupted || !strings.Contains(errOut, "[ERR_INTERRUPTED]") || forced != 0 {
		t.Fatalf("exit %d, forced %d, stderr %q", code, forced, errOut)
	}
}

func TestInterruptReplacesExitCode(t *testing.T) {
	h := newHarness(t)
	sigs := make(chan chan<- os.Signal, 1)
	notifyInterrupt = func(ch chan<- os.Signal) func() { sigs <- ch; return func() {} }
	saved := pending
	built = nil
	t.Cleanup(func() { regMu.Lock(); pending, built = saved, nil; regMu.Unlock() })
	started := make(chan struct{})
	Register(&Command{Path: "zz-batch", Summary: "test", Hidden: true, Run: func(c *Ctx) error {
		close(started)
		<-c.Context().Done()
		return ExitCode(7)
	}})
	go func() {
		ch := <-sigs
		<-started
		ch <- os.Interrupt
	}()
	code, _, errOut := h.run("zz-batch")
	if code != ExitInterrupted || !strings.Contains(errOut, "error: interrupted") || strings.Contains(errOut, "exit status") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestInterruptDuringBootstrapSkipsCommand(t *testing.T) {
	h := newHarness(t)
	sigs := make(chan chan<- os.Signal, 1)
	notifyInterrupt = func(ch chan<- os.Signal) func() { sigs <- ch; return func() {} }
	saved := pending
	built = nil
	t.Cleanup(func() { regMu.Lock(); pending, built = saved, nil; regMu.Unlock() })
	ran := false
	Register(&Command{Path: "zz-destroy", Summary: "test", Hidden: true, Run: func(c *Ctx) error {
		ran = true
		return nil
	}})
	// The second Ctrl-C reaches exit only after the first one cancelled the
	// context; exit is a no-op here.
	forced := make(chan struct{})
	savedExit := exit
	exit = func(int) { close(forced) }
	t.Cleanup(func() { exit = savedExit })
	bootstrap = func() error {
		ch := <-sigs
		ch <- os.Interrupt
		ch <- os.Interrupt
		<-forced
		return nil
	}
	code, _, errOut := h.run("zz-destroy")
	if code != ExitInterrupted || ran || !strings.Contains(errOut, "[ERR_INTERRUPTED]") {
		t.Fatalf("exit %d, ran %v, stderr %q", code, ran, errOut)
	}
}
