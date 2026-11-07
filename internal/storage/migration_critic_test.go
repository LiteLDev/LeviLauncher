package storage

// Regression tests for the "critic" completeness scenarios (CR1-CR6) of the
// legacy-storage migration, asserting the behaviour internal/apppath/README.md
// ("从旧布局迁移") specifies.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// ---------------------------------------------------------------- helpers

// crTree lists every entry below root, for logging the layout on failure.
func crTree(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if !isNotEmpty(p) {
				out = append(out, rel+`\ (empty)`)
			}
			return nil
		}
		if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			out = append(out, rel+" (link)")
			return nil
		}
		b, _ := os.ReadFile(p)
		s := strings.ReplaceAll(string(b), "\n", " ")
		if len(s) > 60 {
			s = s[:60] + "..."
		}
		out = append(out, rel+" = "+s)
		return nil
	})
	sort.Strings(out)
	return out
}

func crLogTree(t *testing.T, label, root string) {
	t.Helper()
	t.Logf("--- %s (%s)", label, root)
	for _, l := range crTree(root) {
		t.Logf("    %s", l)
	}
}

// crWant asserts that every path holds exactly the given content.
func crWant(t *testing.T, want map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, p := range keys {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("missing %s: %v", p, err)
			continue
		}
		if string(b) != want[p] {
			t.Errorf("%s = %q, want %q", p, b, want[p])
		}
	}
}

// crFreshConfig reads config.json again, as the launcher does after Prepare,
// which also sets the data root override.
func crFreshConfig(t *testing.T) config.AppConfig {
	t.Helper()
	c, err := config.Reload()
	if err != nil {
		t.Fatalf("config.Reload: %v", err)
	}
	return c
}

// crInstanceNames lists the instances the launcher shows: versions.ScanVersions
// of apppath.InstancesDir.
func crInstanceNames(t *testing.T) []string {
	t.Helper()
	dir, err := apppath.InstancesDir()
	if err != nil {
		t.Fatalf("InstancesDir: %v", err)
	}
	metas, err := versions.ScanVersions(dir)
	if err != nil {
		t.Fatalf("ScanVersions(%s): %v", dir, err)
	}
	var names []string
	for _, m := range metas {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return names
}

// crNextStart resets the per-process state as a new launcher process has it.
func crNextStart() {
	releaseLock()
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	resetRecordCache()
}

// crSecondPrepareIsCurrent asserts that the next start finds nothing to do.
func crSecondPrepareIsCurrent(t *testing.T, roots ...string) {
	t.Helper()
	crNextStart()
	snap := func() []string {
		var out []string
		for _, r := range roots {
			out = append(out, crTree(r)...)
		}
		return out
	}
	before := snap()
	out, err := Prepare()
	if err != nil || out != Current {
		t.Errorf("second Prepare = %v, %v; want Current", out, err)
	}
	if apppath.LegacyMode() != "" {
		t.Errorf("second Prepare set legacy mode %q", apppath.LegacyMode())
	}
	after := snap()
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("second Prepare changed the files:\nbefore %v\nafter  %v", before, after)
	}
}

func crWaitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// ---------------------------------------------------- second process helper

const (
	crEnvRole  = "LLMIG_CRITIC_ROLE"
	crEnvOut   = "LLMIG_CRITIC_OUT"
	crEnvReady = "LLMIG_CRITIC_READY"
	crEnvGo    = "LLMIG_CRITIC_GO"
	crEnvIn    = "LLMIG_CRITIC_IN"
)

type crChildResult struct {
	Outcome    Outcome `json:"outcome"`
	Err        string  `json:"err"`
	LegacyMode string  `json:"legacy_mode"`
}

// TestMigration_CriticHelperProcess is a second launcher process: it runs
// Prepare and reports the result. It does nothing in a normal test run.
func TestMigration_CriticHelperProcess(t *testing.T) {
	if os.Getenv(crEnvRole) != "prepare" {
		t.Skip("helper process only")
	}
	if p := os.Getenv(crEnvReady); p != "" {
		_ = os.WriteFile(p, []byte("ready"), 0o644)
	}
	if p := os.Getenv(crEnvGo); p != "" {
		deadline := time.Now().Add(60 * time.Second)
		for !exists(p) {
			if time.Now().After(deadline) {
				t.Fatal("no go signal")
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	if p := os.Getenv(crEnvIn); p != "" {
		_ = os.WriteFile(p, []byte("in Prepare"), 0o644)
	}
	out, err := Prepare()
	res := crChildResult{Outcome: out, LegacyMode: apppath.LegacyMode()}
	if err != nil {
		res.Err = err.Error()
	}
	releaseLock()
	b, _ := json.Marshal(res)
	if werr := os.WriteFile(os.Getenv(crEnvOut), b, 0o644); werr != nil {
		t.Fatal(werr)
	}
}

type crChild struct {
	cmd                   *exec.Cmd
	out, ready, goSig, in string
	done                  chan error
}

// crStartChild starts another launcher process (this test binary) that
// inherits APPDATA and LOCALAPPDATA. With wait, it signals ready and waits for
// crChild.signal before it calls Prepare.
func crStartChild(t *testing.T, wait bool) *crChild {
	t.Helper()
	dir := t.TempDir()
	c := &crChild{out: filepath.Join(dir, "out.json"), done: make(chan error, 1)}
	c.cmd = exec.Command(os.Args[0], "-test.run=^TestMigration_CriticHelperProcess$", "-test.count=1")
	env := append(os.Environ(), crEnvRole+"=prepare", crEnvOut+"="+c.out)
	if wait {
		c.ready = filepath.Join(dir, "ready")
		c.goSig = filepath.Join(dir, "go")
		c.in = filepath.Join(dir, "in")
		env = append(env, crEnvReady+"="+c.ready, crEnvGo+"="+c.goSig, crEnvIn+"="+c.in)
	}
	c.cmd.Env = env
	var buf strings.Builder
	c.cmd.Stdout, c.cmd.Stderr = &buf, &buf
	if err := c.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		err := c.cmd.Wait()
		if err != nil {
			err = fmt.Errorf("%v: %s", err, buf.String())
		}
		c.done <- err
	}()
	t.Cleanup(func() { _ = c.cmd.Process.Kill() })
	return c
}

func (c *crChild) result(t *testing.T) crChildResult {
	t.Helper()
	select {
	case err := <-c.done:
		if err != nil {
			t.Fatalf("second process: %v", err)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("second process did not finish")
	}
	var r crChildResult
	if err := json.Unmarshal([]byte(read(t, c.out)), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// ------------------------------------------------------------------- CR1

// CR1: two launcher processes run Prepare at the same time (a window and a
// command-line run, or two windows racing for the single-instance mutex).
// The second must not undo the live migration of the first: it waits for the
// migration lock and then runs on the migrated layout.
func TestMigration_CR1ConcurrentPrepareDoesNotUndoALiveMigration(t *testing.T) {
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
	local := filepath.Join(e.local, "LeviLauncher")
	root := filepath.Dir(e.roaming)

	// Process B is up and waits for the signal to call Prepare.
	b := crStartChild(t, true)
	crWaitFor(t, "second process start", 30*time.Second, func() bool { return exists(b.ready) })

	// Process A migrates; an open file in EBWebView (one of the last items)
	// stalls it in renameRetry with the journal open and versions already
	// moved to Instances.
	held, err := os.Open(filepath.Join(e.legacy, "EBWebView", "Default", "Local Storage", "leveldb"))
	if err != nil {
		t.Fatal(err)
	}
	heldOpen := true
	defer func() {
		if heldOpen {
			held.Close()
		}
	}()
	type res struct {
		out Outcome
		err error
	}
	aDone := make(chan res, 1)
	go func() {
		out, err := Prepare()
		aDone <- res{out, err}
	}()
	crWaitFor(t, "A to be mid-migration", 2*time.Second, func() bool {
		return exists(apppath.MigrationJournalPath()) && exists(filepath.Join(local, "Instances", "A", "version.json"))
	})

	// B calls Prepare while A's migration is in progress.
	write(t, b.goSig, "go")
	crWaitFor(t, "B to call Prepare", 2*time.Second, func() bool { return exists(b.in) })
	time.Sleep(300 * time.Millisecond)
	if !exists(apppath.MigrationJournalPath()) {
		t.Error("A's journal disappeared while A was migrating")
	}
	if !exists(filepath.Join(local, "Instances", "A", "version.json")) || exists(filepath.Join(e.legacy, "versions")) {
		crLogTree(t, "while A migrates", root)
		t.Error("B undid A's live migration")
	}
	select {
	case <-b.done:
		t.Error("B finished while A still held the migration lock")
	default:
	}

	// A resumes and completes.
	held.Close()
	heldOpen = false
	var ra res
	select {
	case ra = <-aDone:
	case <-time.After(30 * time.Second):
		t.Fatal("A did not finish")
	}
	if ra.err != nil || ra.out != Migrated {
		t.Fatalf("A: Prepare = %v, %v; want Migrated", ra.out, ra.err)
	}
	rb := b.result(t)
	t.Logf("B: Prepare = %v, %q, legacy mode %q", rb.Outcome, rb.Err, rb.LegacyMode)
	if rb.Outcome != Current || rb.Err != "" || rb.LegacyMode != "" {
		t.Errorf("B: Prepare = %v, %q (legacy mode %q); want Current without error", rb.Outcome, rb.Err, rb.LegacyMode)
	}
	if exists(apppath.MigrationJournalPath()) {
		t.Error("journal left behind")
	}

	crWant(t, map[string]string{
		filepath.Join(local, "Instances", "A", "version.json"):                               `{"name":"A"}`,
		filepath.Join(local, "Instances", "A", "Minecraft.Windows.exe"):                      "exe",
		filepath.Join(local, "Backups", "Worlds", "W", "W_1.mcworld"):                        "world",
		filepath.Join(local, "Backups", "Instances", "A", "A_1.zip"):                         "zip",
		filepath.Join(local, "Cache", "Installers", "1.21.msixvc"):                           "pkg",
		filepath.Join(local, "Cache", "Updates", "1.0.8.zip"):                                "update",
		filepath.Join(local, "Cache", "gamertags.json"):                                      `{"1":"Steve"}`,
		filepath.Join(local, "Cache", "gdk-pending-versions.json"):                           `[]`,
		filepath.Join(local, "Accounts", "device.dpapi"):                                     "device",
		filepath.Join(local, "WebView2", "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
		filepath.Join(local, "Logs", "startup.log"):                                          "old log",
		filepath.Join(e.legacy, "config.migrated.json"):                                      `{"window_width":900}`,
	})
	if got := TranslateLegacyPath(filepath.Join(e.legacy, "versions", "A")); !samePath(got, filepath.Join(local, "Instances", "A")) {
		t.Errorf("TranslateLegacyPath = %s", got)
	}

	crNextStart()
	c := crFreshConfig(t)
	if c.WindowWidth != 900 || c.BaseRoot != "" {
		t.Errorf("config = %+v", c)
	}
	if got := crInstanceNames(t); strings.Join(got, ",") != "A" {
		t.Errorf("instances = %v", got)
	}
	if got := apppath.WorldBackupsDir(); !exists(filepath.Join(got, "W", "W_1.mcworld")) {
		t.Errorf("world backup not under WorldBackupsDir %s", got)
	}
	if t.Failed() {
		crLogTree(t, "after both processes", root)
	}
	crSecondPrepareIsCurrent(t, root)
}

// ------------------------------------------------------------------- CR2

// CR2: a window could not migrate (Legacy, the game ran from an instance) and
// keeps running on the legacy layout. Another launcher process that starts
// meanwhile must not migrate the data away from under it: it runs on the
// legacy layout too. What the window then writes through the legacy-mode
// getters is migrated at the next start, settings included.
func TestMigration_CR2NoMigrationUnderARunningLegacyModeLauncher(t *testing.T) {
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
	local := filepath.Join(e.local, "LeviLauncher")
	root := filepath.Dir(e.roaming)

	// The window: the migration is undone, legacy mode.
	held, err := os.Open(filepath.Join(e.legacy, "EBWebView", "Default", "Local Storage", "leveldb"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Prepare()
	held.Close()
	if out != Legacy || err == nil || !samePath(apppath.LegacyMode(), e.legacy) {
		t.Fatalf("window: Prepare = %v, %v (legacy mode %q); want Legacy", out, err, apppath.LegacyMode())
	}
	before := crTree(root)

	// A second process while the window runs.
	rb := crStartChild(t, false).result(t)
	t.Logf("second process: Prepare = %v, %q, legacy mode %q", rb.Outcome, rb.Err, rb.LegacyMode)
	if rb.Outcome != Legacy || !samePath(rb.LegacyMode, e.legacy) {
		t.Errorf("second process: Prepare = %v (legacy mode %q); want Legacy on %s", rb.Outcome, rb.LegacyMode, e.legacy)
	}
	if after := crTree(root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		crLogTree(t, "after the second process", root)
		t.Fatalf("the second process changed files under the running window:\nbefore %v\nafter  %v", before, after)
	}

	// The window keeps working on the legacy layout.
	c := crFreshConfig(t)
	if c.WindowWidth != 900 {
		t.Errorf("window config = %+v", c)
	}
	if got := crInstanceNames(t); strings.Join(got, ",") != "A" {
		t.Fatalf("window sees instances %v; want [A]", got)
	}
	c.WindowWidth, c.DisableDiscordRPC = 1234, true
	if err := config.Save(c); err != nil {
		t.Fatal(err)
	}
	if !samePath(apppath.ConfigPath(), filepath.Join(e.legacy, "config.json")) {
		t.Fatalf("legacy-mode ConfigPath = %s", apppath.ConfigPath())
	}
	dir, _ := apppath.InstancesDir()
	write(t, filepath.Join(dir, "New", "version.json"), `{"name":"New"}`)
	write(t, filepath.Join(apppath.WorldBackupsDir(), "W2", "W2_1.mcworld"), "w2")
	write(t, filepath.Join(apppath.AccountsDir(), "account.dpapi"), "acct")

	// The window exits; the next start migrates everything.
	crNextStart()
	out, err = Prepare()
	if err != nil || out != Migrated {
		crLogTree(t, "after the next start", root)
		t.Fatalf("next start: Prepare = %v, %v; want Migrated", out, err)
	}
	crWant(t, map[string]string{
		filepath.Join(local, "Instances", "A", "version.json"):          `{"name":"A"}`,
		filepath.Join(local, "Instances", "New", "version.json"):        `{"name":"New"}`,
		filepath.Join(local, "Backups", "Worlds", "W", "W_1.mcworld"):   "world",
		filepath.Join(local, "Backups", "Worlds", "W2", "W2_1.mcworld"): "w2",
		filepath.Join(local, "Backups", "Instances", "A", "A_1.zip"):    "zip",
		filepath.Join(local, "Accounts", "device.dpapi"):                "device",
		filepath.Join(local, "Accounts", "account.dpapi"):               "acct",
		filepath.Join(local, "Cache", "gamertags.json"):                 `{"1":"Steve"}`,
	})
	for _, p := range []string{filepath.Join(local, "Backups", "Worlds-legacy"), filepath.Join(local, "Backups", "worlds-legacy")} {
		if exists(p) {
			t.Errorf("world backups split into %s", p)
		}
	}
	crNextStart()
	c = crFreshConfig(t)
	if c.WindowWidth != 1234 || !c.DisableDiscordRPC || c.BaseRoot != "" {
		t.Errorf("settings changed in the legacy-mode window were lost: %+v", c)
	}
	if got := crInstanceNames(t); strings.Join(got, ",") != "A,New" {
		t.Errorf("instances = %v; want [A New]", got)
	}
	if !exists(filepath.Join(apppath.WorldBackupsDir(), "W2", "W2_1.mcworld")) {
		t.Error("W2 backup not reachable through WorldBackupsDir")
	}
	if !exists(filepath.Join(apppath.AccountsDir(), "account.dpapi")) {
		t.Error("account not reachable through AccountsDir")
	}
	if got := TranslateLegacyPath(filepath.Join(e.legacy, "versions", "New")); !samePath(got, filepath.Join(local, "Instances", "New")) {
		t.Errorf("TranslateLegacyPath = %s", got)
	}
	if t.Failed() {
		crLogTree(t, "after the next start", root)
	}
	crSecondPrepareIsCurrent(t, root)
}

// ------------------------------------------------------------------- CR3

// CR3: a run on the legacy layout writes downloads, imports and restore
// temporaries to <legacy data root>\Cache and creates Backups with the new
// name. The next migration merges that Cache into the current Cache, where
// CleanImports reaches it, and leaves nothing behind in the legacy root.
func TestMigration_CR3LegacyModeCacheIsMergedIntoTheCurrentCache(t *testing.T) {
	e := setup(t)
	write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"name":"A"}`)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	local := filepath.Join(e.local, "LeviLauncher")
	root := filepath.Dir(e.roaming)
	// A current Cache already holds a download (deep merge).
	write(t, filepath.Join(local, "Cache", "Downloads", "keep.bin"), "keep")

	apppath.SetLegacyMode(e.legacy)
	if !samePath(apppath.CacheDir(), filepath.Join(e.legacy, "Cache")) {
		t.Fatalf("legacy-mode CacheDir = %s", apppath.CacheDir())
	}
	write(t, filepath.Join(apppath.DownloadsDir(), "pack.mcpack"), "dl")
	write(t, filepath.Join(apppath.ImportsDir(), "world.mcworld"), "imp")
	write(t, filepath.Join(apppath.CacheDir(), "restore.123.tmp", "x"), "restore")
	write(t, filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"), "w")

	crNextStart()
	out, err := Prepare()
	if err != nil || out != Migrated {
		crLogTree(t, "after migration", root)
		t.Fatalf("Prepare = %v, %v; want Migrated", out, err)
	}
	crWant(t, map[string]string{
		filepath.Join(local, "Instances", "A", "version.json"):        `{"name":"A"}`,
		filepath.Join(local, "Backups", "Worlds", "W", "W_1.mcworld"): "w",
		filepath.Join(local, "Cache", "Downloads", "pack.mcpack"):     "dl",
		filepath.Join(local, "Cache", "Downloads", "keep.bin"):        "keep",
		filepath.Join(local, "Cache", "Imports", "world.mcworld"):     "imp",
		filepath.Join(local, "Cache", "restore.123.tmp", "x"):         "restore",
	})
	if exists(filepath.Join(e.legacy, "Cache")) {
		t.Errorf("legacy Cache left behind: %v", crTree(filepath.Join(e.legacy, "Cache")))
	}
	if got := TranslateLegacyPath(filepath.Join(e.legacy, "Cache", "Downloads", "pack.mcpack")); !samePath(got, filepath.Join(local, "Cache", "Downloads", "pack.mcpack")) {
		t.Errorf("TranslateLegacyPath = %s", got)
	}

	crNextStart()
	crFreshConfig(t)
	if got := crInstanceNames(t); strings.Join(got, ",") != "A" {
		t.Errorf("instances = %v", got)
	}
	if !exists(filepath.Join(apppath.DownloadsDir(), "pack.mcpack")) || !exists(filepath.Join(apppath.ImportsDir(), "world.mcworld")) {
		t.Error("migrated downloads and imports not reachable through DownloadsDir and ImportsDir")
	}
	if !exists(filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld")) {
		t.Error("world backup not reachable through WorldBackupsDir")
	}
	if t.Failed() {
		crLogTree(t, "after migration", root)
	}
	crSecondPrepareIsCurrent(t, root)

	// The current layout's cleaner now reaches the stale legacy-mode files.
	old := time.Now().Add(-48 * time.Hour)
	for _, p := range []string{
		filepath.Join(local, "Cache", "Downloads", "pack.mcpack"),
		filepath.Join(local, "Cache", "Imports", "world.mcworld"),
	} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	CleanImports()
	if exists(filepath.Join(local, "Cache", "Downloads", "pack.mcpack")) || exists(filepath.Join(local, "Cache", "Imports", "world.mcworld")) {
		t.Error("CleanImports does not reach the migrated legacy-mode downloads and imports")
	}
	if !exists(filepath.Join(local, "Cache", "Downloads", "keep.bin")) {
		t.Error("CleanImports removed a fresh download")
	}
}

// ------------------------------------------------------------------- CR4

// CR4: nothing writes into the data root before Prepare decides it. The
// runtime checks run after Prepare and config.Load, so a redistributable
// downloaded at the first start lands in the data root's Cache\Installers,
// also when the legacy config names a custom data root. A startup.log opened
// before the migration is merged with the legacy logs.
func TestMigration_CR4RuntimeInstallersGoToTheDecidedDataRoot(t *testing.T) {
	e := setup(t)
	root := filepath.Dir(e.roaming)
	base := filepath.Join(root, "Data")
	seedLegacy(t, e.legacy, base)
	cfg, _ := json.Marshal(map[string]any{"base_root": base, "window_width": 640})
	write(t, filepath.Join(e.legacy, "config.json"), string(cfg))
	local := filepath.Join(e.local, "LeviLauncher")
	// The startup log is opened before Prepare.
	write(t, apppath.StartupLogPath(), "new log")

	out, err := Prepare()
	if err != nil || out != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", out, err)
	}
	c := crFreshConfig(t) // main.go: config.Load after Prepare
	if !samePath(c.BaseRoot, base) || c.WindowWidth != 640 {
		t.Errorf("config = %+v; want base_root %s", c, base)
	}
	// main.go: vcruntime/webview2runtime.EnsureStartupInteractive now.
	dir, err := apppath.InstallersDir()
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(dir, filepath.Join(base, "Cache", "Installers")) {
		t.Errorf("InstallersDir at the runtime checks = %s; want %s", dir, filepath.Join(base, "Cache", "Installers"))
	}
	write(t, filepath.Join(dir, "vc_redist.x64.exe"), "vc")
	crWant(t, map[string]string{
		filepath.Join(base, "Cache", "Installers", "1.21.msixvc"):       "pkg",
		filepath.Join(base, "Cache", "Installers", "vc_redist.x64.exe"): "vc",
		filepath.Join(base, "Instances", "A", "version.json"):           `{"name":"A"}`,
		filepath.Join(local, "Logs", "startup.log"):                     "new log",
		filepath.Join(local, "Logs", "startup-legacy.log"):              "old log",
	})
	if exists(filepath.Join(local, "Cache")) || exists(filepath.Join(local, "Instances")) {
		crLogTree(t, "local", local)
		t.Error("something wrote into the default data root although the data root is custom")
	}
	if got := crInstanceNames(t); strings.Join(got, ",") != "A" {
		t.Errorf("instances = %v", got)
	}
	crSecondPrepareIsCurrent(t, root)

	// The startup order in main.go: Prepare, then config.Load, then the
	// runtime checks that may download into InstallersDir.
	src, err := os.ReadFile(filepath.Join("..", "..", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	order := []string{"storage.Prepare()", "config.Load()", "vcruntime.EnsureStartupInteractive(", "webview2runtime.EnsureStartupInteractive("}
	last := -1
	for _, call := range order {
		i := strings.Index(s, call)
		if i < 0 {
			t.Errorf("main.go has no %s", call)
			continue
		}
		if i < last {
			t.Errorf("main.go calls %s before the call preceding it in %v", call, order)
		}
		last = i
	}
	if i := strings.Index(s, "InstallersDir("); i >= 0 && i < strings.Index(s, "storage.Prepare()") {
		t.Error("main.go reads InstallersDir before storage.Prepare")
	}
}

// ------------------------------------------------------------------- CR5

// CR5: %LOCALAPPDATA%\LeviLauncher, or %LOCALAPPDATA% itself, is a junction
// to another volume while %APPDATA% is on the system volume. The migration
// keeps the data in place (cross-volume fallback, base_root = the legacy
// directory), copies accounts, the WebView2 profile and the logs to the other
// volume keeping the sources, and the next start is Current.
func TestMigration_CR5LocalDirIsAJunctionToAnotherVolume(t *testing.T) {
	dRoot := testSecondVolume
	if !isDir(dRoot) {
		t.Skip("no second volume at " + dRoot)
	}
	for _, variant := range []string{"LeviLauncherJunction", "LocalAppDataJunction"} {
		t.Run(variant, func(t *testing.T) {
			e := setup(t)
			if sameVolume(e.legacy, dRoot) {
				t.Skip("the temporary directory is on the second volume")
			}
			target, err := os.MkdirTemp(dRoot, "critic-cr5-")
			if err != nil {
				t.Fatal(err)
			}
			var link string
			if variant == "LeviLauncherJunction" {
				link = filepath.Join(e.local, "LeviLauncher")
			} else {
				if err := os.Remove(e.local); err != nil {
					t.Fatal(err)
				}
				link = e.local
			}
			if outp, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
				_ = os.RemoveAll(target)
				t.Fatalf("mklink: %v %s", err, outp)
			}
			t.Cleanup(func() {
				releaseLock()
				_ = os.Remove(link)
				if err := os.RemoveAll(target); err != nil {
					t.Logf("cleanup %s: %v", target, err)
				}
			})
			local := filepath.Join(e.local, "LeviLauncher")
			root := filepath.Dir(e.roaming)
			seedLegacy(t, e.legacy, e.legacy)
			write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":800}`)
			if sameVolume(e.legacy, apppath.LocalDir()) {
				t.Fatalf("sameVolume(%s, %s) through a junction to D: = true", e.legacy, apppath.LocalDir())
			}

			out, err := Prepare()
			if err != nil || out != Migrated || apppath.LegacyMode() != "" {
				crLogTree(t, "roaming", e.roaming)
				crLogTree(t, "local", e.local)
				t.Fatalf("start 1: Prepare = %v, %v (legacy mode %q); want Migrated", out, err, apppath.LegacyMode())
			}
			L := e.legacy
			crWant(t, map[string]string{
				// Data kept in place under the current names.
				filepath.Join(L, "Instances", "A", "version.json"):          `{"name":"A"}`,
				filepath.Join(L, "Instances", "A", "Minecraft.Windows.exe"): "exe",
				filepath.Join(L, "Backups", "Worlds", "W", "W_1.mcworld"):   "world",
				filepath.Join(L, "Backups", "Instances", "A", "A_1.zip"):    "zip",
				filepath.Join(L, "Cache", "Installers", "1.21.msixvc"):      "pkg",
				filepath.Join(L, "Cache", "Updates", "1.0.8.zip"):           "update",
				filepath.Join(L, "Cache", "gamertags.json"):                 `{"1":"Steve"}`,
				filepath.Join(L, "Cache", "gdk-pending-versions.json"):      `[]`,
				// Copied to the local directory on D:.
				filepath.Join(local, "Accounts", "device.dpapi"):                                     "device",
				filepath.Join(local, "WebView2", "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
				filepath.Join(local, "Logs", "startup.log"):                                          "old log",
				// Sources of the copies kept.
				filepath.Join(L, "microsoft-account", "device.dpapi"):                "device",
				filepath.Join(L, "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
				filepath.Join(L, "config.migrated.json"):                             `{"window_width":800}`,
			})
			if got := actualName(filepath.Join(L, "Backups")); got != "Backups" {
				t.Errorf("backups kept the name %q", got)
			}
			// The copies are physically on D: behind the junction.
			dLocal := target
			if variant == "LocalAppDataJunction" {
				dLocal = filepath.Join(target, "LeviLauncher")
			}
			crWant(t, map[string]string{
				filepath.Join(dLocal, "Accounts", "device.dpapi"):                                     "device",
				filepath.Join(dLocal, "WebView2", "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
			})
			if !isFile(filepath.Join(dLocal, "storage-migration.json")) {
				t.Errorf("migration record not in %s", dLocal)
			}
			if got := TranslateLegacyPath(filepath.Join(L, "versions", "A")); !samePath(got, filepath.Join(L, "Instances", "A")) {
				t.Errorf("TranslateLegacyPath = %s", got)
			}

			crNextStart()
			c := crFreshConfig(t)
			if !samePath(c.BaseRoot, L) || c.WindowWidth != 800 {
				t.Errorf("config = %+v; want base_root %s", c, L)
			}
			if got := crInstanceNames(t); strings.Join(got, ",") != "A" {
				t.Errorf("instances = %v", got)
			}
			if dir, _ := apppath.InstallersDir(); !exists(filepath.Join(dir, "1.21.msixvc")) {
				t.Errorf("installer not under InstallersDir %s", dir)
			}
			for p, what := range map[string]string{
				filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"):                             "WorldBackupsDir",
				filepath.Join(apppath.AccountsDir(), "device.dpapi"):                                     "AccountsDir",
				filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Local Storage", "leveldb"): "WebView2Dir",
				apppath.GamertagsPath(): "GamertagsPath",
			} {
				if !exists(p) {
					t.Errorf("%s does not reach %s", what, p)
				}
			}
			if t.Failed() {
				crLogTree(t, "roaming", e.roaming)
				crLogTree(t, "local", e.local)
			}
			crSecondPrepareIsCurrent(t, root, target)
		})
	}
}

// ------------------------------------------------------------------- CR6

// CR6: the legacy directory holds only empty data directories (old launchers
// created them at every start) while the current layout names a custom data
// root. Whatever the migration does with the empty directories, the current
// base_root and settings stay, and the instances in the custom root remain
// the ones the launcher lists.
func TestMigration_CR6EmptyLegacyDirsKeepTheCurrentCustomDataRoot(t *testing.T) {
	for _, variant := range []string{"no record", "after an earlier migration"} {
		t.Run(variant, func(t *testing.T) {
			e := setup(t)
			root := filepath.Dir(e.roaming)
			custom := filepath.Join(root, "Custom")
			local := filepath.Join(e.local, "LeviLauncher")
			if variant == "no record" {
				cfg, _ := json.Marshal(map[string]any{"base_root": custom, "window_width": 777, "future_key": "kept"})
				write(t, apppath.ConfigPath(), string(cfg))
				write(t, filepath.Join(custom, "Instances", "Cur", "version.json"), `{"name":"Cur"}`)
			} else {
				cfg, _ := json.Marshal(map[string]any{"base_root": custom, "window_width": 777, "future_key": "kept"})
				write(t, filepath.Join(e.legacy, "config.json"), string(cfg))
				write(t, filepath.Join(custom, "versions", "Cur", "version.json"), `{"name":"Cur"}`)
				if out, err := Prepare(); err != nil || out != Migrated {
					t.Fatalf("earlier migration: Prepare = %v, %v", out, err)
				}
				crNextStart()
			}
			for _, d := range []string{"versions", "installers", "logs"} {
				if err := os.MkdirAll(filepath.Join(e.legacy, d), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			out, err := Prepare()
			if err != nil || (out != Migrated && out != Current) || apppath.LegacyMode() != "" {
				crLogTree(t, "all", root)
				t.Fatalf("Prepare = %v, %v (legacy mode %q)", out, err, apppath.LegacyMode())
			}
			m := readJSON(t, apppath.ConfigPath())
			if s, _ := m["base_root"].(string); !samePath(s, custom) {
				t.Errorf("base_root = %v; want %s (config %v)", m["base_root"], custom, m)
			}
			if m["window_width"] != float64(777) || m["future_key"] != "kept" {
				t.Errorf("config members lost: %v", m)
			}
			crWant(t, map[string]string{filepath.Join(custom, "Instances", "Cur", "version.json"): `{"name":"Cur"}`})
			if exists(filepath.Join(local, "Instances")) {
				t.Errorf("an Instances directory was created in the default data root although the data root is %s", custom)
			}

			crNextStart()
			c := crFreshConfig(t)
			if !samePath(c.BaseRoot, custom) {
				t.Errorf("config.Load base_root = %q", c.BaseRoot)
			}
			if dir, _ := apppath.InstancesDir(); !samePath(dir, filepath.Join(custom, "Instances")) {
				t.Errorf("InstancesDir = %s", dir)
			}
			if got := crInstanceNames(t); strings.Join(got, ",") != "Cur" {
				t.Errorf("instances = %v; want [Cur]", got)
			}
			if t.Failed() {
				crLogTree(t, "all", root)
			}
			crSecondPrepareIsCurrent(t, root)
		})
	}
}
