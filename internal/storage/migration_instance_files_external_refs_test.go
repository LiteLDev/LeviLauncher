package storage

// Regression tests for the per-instance legacy files, the native loader's
// view of the instance metadata, package registrations, shortcut icons and
// TranslateLegacyPath after the storage migration (internal/apppath/README.md,
// "从旧布局迁移"). Group "instance-files-external-refs".

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/icons"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"golang.org/x/sys/windows"
)

// ---------- helpers (prefixed mig to stay clear of migrate_test.go) ----------

func migTree(root string) string {
	var lines []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			lines = append(lines, rel+`\`)
			return nil
		}
		b, _ := os.ReadFile(p)
		lines = append(lines, rel+" = "+strconv.Quote(string(b)))
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func migLog(t *testing.T, label, root string) {
	t.Helper()
	t.Logf("---- %s (%s) ----\n%s", label, root, migTree(root))
}

func migResetRecord() {
	recordMu.Lock()
	recordLoaded = false
	recordCache = nil
	recordMu.Unlock()
}

func migPrepare(t *testing.T, want Outcome) {
	t.Helper()
	outcome, err := Prepare()
	if outcome != want || (want != Legacy && want != Deferred && err != nil) {
		t.Fatalf("Prepare = %v, %v; want %v", outcome, err, want)
	}
	migResetRecord()
}

// migAssertNoop runs Prepare again: it must find the current layout and
// change nothing under roots.
func migAssertNoop(t *testing.T, roots ...string) {
	t.Helper()
	before := map[string]string{}
	for _, r := range roots {
		before[r] = migTree(r)
	}
	migResetRecord()
	outcome, err := Prepare()
	migResetRecord()
	if err != nil || outcome != Current {
		t.Errorf("second Prepare = %v, %v; want Current", outcome, err)
	}
	for _, r := range roots {
		if after := migTree(r); after != before[r] {
			t.Errorf("second Prepare changed %s:\nbefore:\n%s\nafter:\n%s", r, before[r], after)
		}
	}
}

func migMeta(t *testing.T, m versions.VersionMeta) string {
	t.Helper()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

func migHidden(p string) bool {
	ptr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return false
	}
	a, err := windows.GetFileAttributes(ptr)
	return err == nil && a&windows.FILE_ATTRIBUTE_HIDDEN != 0
}

func migScan(t *testing.T, inst string) map[string]versions.VersionMeta {
	t.Helper()
	metas, err := versions.ScanVersions(inst)
	if err != nil {
		t.Fatalf("ScanVersions(%s): %v", inst, err)
	}
	out := map[string]versions.VersionMeta{}
	for _, m := range metas {
		out[m.Name] = m
	}
	return out
}

func migNames(m map[string]versions.VersionMeta) []string {
	var s []string
	for n := range m {
		s = append(s, n)
	}
	sort.Strings(s)
	return s
}

// ---------- native loader mirror ----------

// migNative mirrors native/levilauncher/src/config/version_config.cpp:
// version_config_path (the newer of .levilauncher\instance.json and
// version.json; instance.json on a tie) and parse_version_config.
type migNative struct {
	path                               string
	hasMetadata, uwp, console, isolate bool
	gameVersion, channel               string
}

func migNativePath(dir string) string {
	current := filepath.Join(dir, ".levilauncher", "instance.json")
	legacy := filepath.Join(dir, "version.json")
	ci, cerr := os.Stat(current)
	if cerr != nil {
		return legacy
	}
	if li, lerr := os.Stat(legacy); lerr == nil && li.ModTime().After(ci.ModTime()) {
		return legacy
	}
	return current
}

func migParseVersion(s string) ([4]uint64, bool) {
	var out [4]uint64
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 4 {
		return out, false
	}
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return out, false
		}
		out[i] = v
	}
	return out, true
}

func migVersionLess(a, b [4]uint64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func migNativeRead(dir string) migNative {
	c := migNative{isolate: true, path: migNativePath(dir)}
	b, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	var raw map[string]any
	if json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &raw) != nil {
		return migNative{isolate: true, path: c.path}
	}
	c.hasMetadata = true
	if v, ok := raw["packageType"].(string); ok {
		c.uwp = strings.ToLower(v) == "uwp"
	}
	if v, ok := raw["enableConsole"].(bool); ok {
		c.console = v
	}
	iso, hasIso := raw["enableIsolation"]
	if hasIso {
		switch v := iso.(type) {
		case bool:
			c.isolate = v
		case string:
			c.isolate = strings.ToLower(v) != "false"
		case float64:
			c.isolate = v != 0
		}
	}
	if v, ok := raw["gameVersion"].(string); ok {
		c.gameVersion = v
	}
	if v, ok := raw["type"].(string); ok {
		c.channel = v
	}
	supportsUWP := false
	if v, ok := migParseVersion(strings.TrimSpace(c.gameVersion)); ok {
		supportsUWP = !migVersionLess(v, [4]uint64{1, 19, 70, 2})
	}
	if c.uwp && (!hasIso || !supportsUWP) {
		c.isolate = false
	}
	return c
}

// migNativeDataDir mirrors legacy_data_directory; "" means no redirect.
func migNativeDataDir(game string, c migNative) string {
	if !c.isolate {
		return ""
	}
	if !c.hasMetadata {
		return game
	}
	if c.gameVersion == "1.26.0.24" {
		return filepath.Join(game, "Minecraft Bedrock Preview")
	}
	if v, ok := migParseVersion(c.gameVersion); ok && !migVersionLess(v, [4]uint64{1, 26, 0, 0}) {
		return game
	}
	if c.channel == "preview" {
		return filepath.Join(game, "Minecraft Bedrock Preview")
	}
	return filepath.Join(game, "Minecraft Bedrock")
}

// ---------- P1 ----------

// P1-version-json-lazy-move-and-native-loader: version.json moves to
// .levilauncher\instance.json at the first read, byte for byte, and the
// native loader keeps reading the same settings.
func TestMigration_P1_VersionJSONLazyMoveAndNativeLoader(t *testing.T) {
	e := setup(t)
	vs := filepath.Join(e.legacy, "versions")
	created := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	raw := map[string]string{
		"A": migMeta(t, versions.VersionMeta{Name: "A", GameVersion: "1.21.120.4", Type: "release", PackageType: "gdk", EnableIsolation: false, CreatedAt: created}),
		"B": migMeta(t, versions.VersionMeta{Name: "B", GameVersion: "1.21.120.4", Type: "release", PackageType: "gdk", EnableIsolation: true, CreatedAt: created}),
		"U": migMeta(t, versions.VersionMeta{Name: "U", GameVersion: "1.21.50.7", Type: "release", PackageType: "uwp", EnableIsolation: true, EnableConsole: true, CreatedAt: created}),
	}
	files := map[string]string{}
	for n, s := range raw {
		files[filepath.Join(n, "version.json")] = s
		files[filepath.Join(n, "Minecraft.Windows.exe")] = "exe-" + n
	}
	files[filepath.Join("B", "MicrosoftGame.config")] = "<Game/>"
	files[filepath.Join("B", "Minecraft Bedrock", "Users", "Shared", "games", "com.mojang", "minecraftWorlds", "w1", "level.dat")] = "world-B"
	files[filepath.Join("U", "AppxManifest.xml")] = "<Package/>"
	for rel, s := range files {
		write(t, filepath.Join(vs, rel), s)
	}
	write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900,"future_key":"kept"}`)

	nativeBefore := map[string]migNative{}
	for n := range raw {
		nativeBefore[n] = migNativeRead(filepath.Join(vs, n))
	}
	if c := nativeBefore["A"]; c.isolate || !c.hasMetadata {
		t.Fatalf("fixture: native A before = %+v", c)
	}

	migPrepare(t, Migrated)
	local := filepath.Join(e.local, "LeviLauncher")
	inst, err := apppath.InstancesDir()
	if err != nil || !samePath(inst, filepath.Join(local, "Instances")) {
		t.Fatalf("InstancesDir = %s, %v", inst, err)
	}
	// Moved whole, nothing rewritten yet: the native loader sees the same.
	for rel, s := range files {
		if got := read(t, filepath.Join(inst, rel)); got != s {
			t.Errorf("%s = %q, want %q", rel, got, s)
		}
	}
	for n := range raw {
		if got := migNativeRead(filepath.Join(inst, n)); got.isolate != nativeBefore[n].isolate || got.console != nativeBefore[n].console {
			t.Errorf("native %s right after the migration = %+v, before %+v", n, got, nativeBefore[n])
		}
	}

	byName := migScan(t, inst)
	if got := migNames(byName); strings.Join(got, ",") != "A,B,U" {
		t.Fatalf("ScanVersions names = %v", got)
	}
	if m := byName["A"]; m.EnableIsolation || m.GameVersion != "1.21.120.4" || m.Type != "release" || !m.CreatedAt.Equal(created) {
		t.Errorf("A meta = %+v", m)
	}
	if m := byName["B"]; !m.EnableIsolation || m.PackageType != versions.PackageTypeGDK {
		t.Errorf("B meta = %+v", m)
	}
	if m := byName["U"]; !m.EnableConsole || !m.EnableIsolation || m.PackageType != versions.PackageTypeUWP {
		t.Errorf("U meta = %+v", m)
	}
	for n, s := range raw {
		dir := filepath.Join(inst, n)
		if got := read(t, filepath.Join(dir, ".levilauncher", "instance.json")); got != s {
			t.Errorf("%s instance.json = %q, want the old bytes %q", n, got, s)
		}
		if exists(filepath.Join(dir, "version.json")) {
			t.Errorf("%s version.json left after its content moved", n)
		}
		if !migHidden(filepath.Join(dir, ".levilauncher")) {
			t.Errorf("%s .levilauncher is not hidden", n)
		}
	}
	migLog(t, "instances after ScanVersions", inst)

	// The native loader (LeviLauncher.dll) reads the moved metadata.
	for n := range raw {
		dir := filepath.Join(inst, n)
		got := migNativeRead(dir)
		if !samePath(got.path, filepath.Join(dir, ".levilauncher", "instance.json")) {
			t.Errorf("native %s reads %s", n, got.path)
		}
		if got.isolate != nativeBefore[n].isolate || got.console != nativeBefore[n].console || got.uwp != nativeBefore[n].uwp {
			t.Errorf("native %s = %+v, before the move %+v", n, got, nativeBefore[n])
		}
	}
	if c := migNativeRead(filepath.Join(inst, "A")); c.isolate {
		t.Error("native: non-isolated A became isolated")
	}
	bDir := filepath.Join(inst, "B")
	if got := migNativeDataDir(bDir, migNativeRead(bDir)); !samePath(got, filepath.Join(bDir, "Minecraft Bedrock")) {
		t.Errorf("native: B data dir = %q, want its Minecraft Bedrock folder", got)
	}
	if c := migNativeRead(filepath.Join(inst, "U")); !c.console || !c.isolate || c.gameVersion != "1.21.50.7" {
		t.Errorf("native: U = %+v, want console and isolation", c)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "native", "levilauncher", "src", "config", "version_config.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`L".levilauncher" / L"instance.json"`, `L"version.json"`, `legacy_time > current_time`, `version_config_path(directory)`} {
		if !bytes.Contains(src, []byte(want)) {
			t.Errorf("version_config.cpp no longer contains %q, which the mirror in this test assumes", want)
		}
	}

	cfg := readJSON(t, apppath.ConfigPath())
	if cfg["window_width"] != float64(900) || cfg["future_key"] != "kept" {
		t.Errorf("config = %v", cfg)
	}
	if _, ok := cfg["base_root"]; ok {
		t.Errorf("default data root got base_root: %v", cfg)
	}
	if !exists(filepath.Join(e.legacy, "config.migrated.json")) || exists(vs) {
		t.Error("legacy layout not retired")
	}
	if got := TranslateLegacyPath(filepath.Join(vs, "A")); !samePath(got, filepath.Join(inst, "A")) {
		t.Errorf("TranslateLegacyPath(A) = %s", got)
	}

	// Reading again changes nothing.
	beforeScan := migTree(inst)
	migScan(t, inst)
	if after := migTree(inst); after != beforeScan {
		t.Errorf("second ScanVersions changed the instances:\n%s\n---\n%s", beforeScan, after)
	}
	migAssertNoop(t, local, e.roaming)
}

// P1 on the legacy layout: when the migration is undone, the instance
// metadata stays in version.json, which an older launcher and the native
// loader read; the next start migrates it.
func TestMigration_P1_LegacyModeKeepsVersionJSON(t *testing.T) {
	e := setup(t)
	vs := filepath.Join(e.legacy, "versions")
	metaA := migMeta(t, versions.VersionMeta{Name: "A", GameVersion: "1.21.120.4", Type: "release", EnableIsolation: false})
	write(t, filepath.Join(vs, "A", "version.json"), metaA)
	exe := filepath.Join(vs, "A", "Minecraft.Windows.exe")
	write(t, exe, "exe")
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)

	// The game runs from the instance: its folder cannot be renamed.
	f, err := os.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	outcome, perr := Prepare()
	migResetRecord()
	if outcome != Legacy || perr == nil {
		f.Close()
		t.Fatalf("Prepare = %v, %v; want Legacy", outcome, perr)
	}
	inst, _ := apppath.InstancesDir()
	if !samePath(inst, vs) {
		f.Close()
		t.Fatalf("legacy mode InstancesDir = %s", inst)
	}
	byName := migScan(t, inst)
	if m, ok := byName["A"]; !ok || m.EnableIsolation {
		t.Errorf("A on the legacy layout = %+v, %v", m, ok)
	}
	if got := read(t, filepath.Join(vs, "A", "version.json")); got != metaA {
		t.Errorf("legacy mode ReadMeta moved or changed version.json: %q", got)
	}
	if exists(filepath.Join(vs, "A", ".levilauncher", "instance.json")) {
		t.Error("legacy mode ReadMeta wrote instance.json")
	}
	// A change on the legacy layout goes to version.json.
	m := byName["A"]
	m.EnableConsole = true
	if err := versions.WriteMeta(filepath.Join(vs, "A"), m); err != nil {
		t.Fatal(err)
	}
	if c := migNativeRead(filepath.Join(vs, "A")); !c.console || c.isolate {
		t.Errorf("native on the legacy layout = %+v", c)
	}
	f.Close()
	migLog(t, "legacy layout", e.legacy)

	apppath.SetLegacyMode("")
	written := read(t, filepath.Join(vs, "A", "version.json"))
	migPrepare(t, Migrated)
	inst, _ = apppath.InstancesDir()
	if got, ok := migScan(t, inst)["A"]; !ok || !got.EnableConsole || got.EnableIsolation {
		t.Errorf("A after the retry = %+v, %v", got, ok)
	}
	if got := read(t, filepath.Join(inst, "A", ".levilauncher", "instance.json")); got != written {
		t.Errorf("instance.json = %q, want %q", got, written)
	}
	if got := read(t, filepath.Join(inst, "A", "Minecraft.Windows.exe")); got != "exe" {
		t.Errorf("exe = %q", got)
	}
	migAssertNoop(t, filepath.Join(e.local, "LeviLauncher"), e.roaming)
}

// ---------- P2 ----------

// P2-both-meta-files: an instance with both instance.json and version.json
// (a downgrade cycle) is read from the newer one, by the launcher and the
// native loader alike.
func TestMigration_P2_BothMetaFiles(t *testing.T) {
	const older = `{"name":"A","enableConsole":false,"packageType":"uwp","gameVersion":"1.21.50.7","enableIsolation":true}`
	const newer = `{"name":"A","enableConsole":true,"packageType":"uwp","gameVersion":"1.21.50.7","enableIsolation":false}`
	for _, tc := range []struct {
		name                string
		instanceJSON        string
		versionJSON         string
		instAge, versionAge time.Duration
		wantConsole         bool
		wantInstanceJSON    string
		wantVersionJSON     string // "" when it must be gone
	}{
		{"version.json newer", older, newer, 48 * time.Hour, time.Hour, true, newer, ""},
		{"instance.json newer", newer, older, time.Hour, 48 * time.Hour, true, newer, older},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := setup(t)
			dir := filepath.Join(e.legacy, "versions", "A")
			ij := filepath.Join(dir, ".levilauncher", "instance.json")
			vj := filepath.Join(dir, "version.json")
			write(t, ij, tc.instanceJSON)
			write(t, vj, tc.versionJSON)
			now := time.Now()
			_ = os.Chtimes(ij, now.Add(-tc.instAge), now.Add(-tc.instAge))
			_ = os.Chtimes(vj, now.Add(-tc.versionAge), now.Add(-tc.versionAge))
			write(t, filepath.Join(e.legacy, "config.json"), `{}`)

			migPrepare(t, Migrated)
			inst, _ := apppath.InstancesDir()
			nd := filepath.Join(inst, "A")
			native := migNativeRead(nd)
			m, err := versions.ReadMeta(nd)
			if err != nil {
				t.Fatal(err)
			}
			migLog(t, "instance A", nd)
			if m.EnableConsole != tc.wantConsole {
				t.Errorf("ReadMeta enableConsole = %v; the newer metadata says %v", m.EnableConsole, tc.wantConsole)
			}
			if native.console != tc.wantConsole || native.isolate {
				t.Errorf("native = %+v; want the newer metadata (console %v, no isolation)", native, tc.wantConsole)
			}
			if got := read(t, filepath.Join(nd, ".levilauncher", "instance.json")); got != tc.wantInstanceJSON {
				t.Errorf("instance.json = %q, want %q", got, tc.wantInstanceJSON)
			}
			if tc.wantVersionJSON == "" {
				if exists(filepath.Join(nd, "version.json")) {
					t.Error("version.json left after its newer content moved")
				}
			} else if got := read(t, filepath.Join(nd, "version.json")); got != tc.wantVersionJSON {
				t.Errorf("version.json = %q, want it left as %q", got, tc.wantVersionJSON)
			}
			// The launcher and the native loader agree after the read too.
			if again := migNativeRead(nd); again.console != m.EnableConsole {
				t.Errorf("native after ReadMeta = %+v, launcher %+v", again, m)
			}
			if names := migNames(migScan(t, inst)); strings.Join(names, ",") != "A" {
				t.Errorf("names = %v", names)
			}
			migAssertNoop(t, filepath.Join(e.local, "LeviLauncher"), e.roaming)
		})
	}

	t.Run("same mtime", func(t *testing.T) {
		dir := t.TempDir()
		ij := filepath.Join(dir, ".levilauncher", "instance.json")
		vj := filepath.Join(dir, "version.json")
		write(t, ij, older)
		write(t, vj, newer)
		ts := time.Now().Add(-time.Hour).Truncate(time.Second)
		_ = os.Chtimes(ij, ts, ts)
		_ = os.Chtimes(vj, ts, ts)
		native := migNativeRead(dir)
		m, err := versions.ReadMeta(dir)
		if err != nil {
			t.Fatal(err)
		}
		if native.console != m.EnableConsole {
			t.Errorf("tie: launcher reads console=%v, native reads console=%v (%s)", m.EnableConsole, native.console, native.path)
		}
		if read(t, ij) != older || read(t, vj) != newer {
			t.Error("a tie changed a metadata file")
		}
	})
}

// ---------- P3 ----------

// P3-meta-variants: metadata of every era, corrupt or odd, never panics,
// is never deleted, and a read-only version.json is tolerated.
func TestMigration_P3_MetaVariants(t *testing.T) {
	e := setup(t)
	vs := filepath.Join(e.legacy, "versions")
	variants := map[string]string{
		"Empty":     "",
		"Truncated": `{"name":"Truncated","gameVer`,
		"BOM":       "\xef\xbb\xbf" + `{"name":"BOM","gameVersion":"1.21.1.1"}`,
		"Spaced":    `{"name        ":"Spaced","type       ":"release"}`,
		"Reg":       `{"name":"Reg","registered":true,"gameVersion":"1.21.1.1"}`,
		"ZeroTime":  `{"name":"ZeroTime","createdAt":"0001-01-01T00:00:00Z"}`,
		"lower":     `{"name":"LOWER"}`,
		"RO":        `{"name":"RO","enableConsole":true}`,
	}
	for n, s := range variants {
		write(t, filepath.Join(vs, n, "version.json"), s)
	}
	if err := os.Chmod(filepath.Join(vs, "RO", "version.json"), 0o444); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	local := filepath.Join(e.local, "LeviLauncher")
	inst := filepath.Join(local, "Instances")
	t.Cleanup(func() {
		_ = os.Chmod(filepath.Join(vs, "RO", "version.json"), 0o666)
		_ = os.Chmod(filepath.Join(inst, "RO", "version.json"), 0o666)
	})

	migPrepare(t, Migrated)
	if got, _ := apppath.InstancesDir(); !samePath(got, inst) {
		t.Fatalf("InstancesDir = %s", got)
	}
	for n, s := range variants {
		if got := read(t, filepath.Join(inst, n, "version.json")); got != s {
			t.Errorf("%s version.json moved as %q", n, got)
		}
	}

	var names map[string]versions.VersionMeta
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ScanVersions panicked: %v", r)
			}
		}()
		names = migScan(t, inst)
	}()
	t.Logf("listed: %v", migNames(names))
	migLog(t, "instances", inst)

	for _, n := range []string{"Empty", "Truncated"} {
		if _, ok := names[n]; ok {
			t.Errorf("corrupt %s listed", n)
		}
	}
	for _, n := range []string{"BOM", "Spaced", "Reg", "ZeroTime", "RO"} {
		if _, ok := names[n]; !ok {
			t.Errorf("%s not listed", n)
		}
	}
	if !names["Reg"].Registered {
		t.Errorf("Reg lost registered: %+v", names["Reg"])
	}
	if !names["RO"].EnableConsole {
		t.Errorf("RO lost enableConsole: %+v", names["RO"])
	}
	lowerOK := false
	for n := range names {
		if strings.EqualFold(n, "lower") {
			lowerOK = true
		}
	}
	if !lowerOK {
		t.Error("an instance whose name differs from its folder in case is not listed")
	}

	// Nothing is deleted: the content of every variant is in the instance,
	// byte for byte, or (when the folder name was written into it) with every
	// member it had.
	for n, s := range variants {
		dir := filepath.Join(inst, n)
		var got string
		if b, err := os.ReadFile(filepath.Join(dir, ".levilauncher", "instance.json")); err == nil {
			got = string(b)
		} else if b, err := os.ReadFile(filepath.Join(dir, "version.json")); err == nil {
			got = string(b)
		} else {
			t.Errorf("%s: metadata deleted", n)
			continue
		}
		if got == s {
			continue
		}
		var want, have map[string]json.RawMessage
		if json.Unmarshal(bytes.TrimPrefix([]byte(s), []byte("\xef\xbb\xbf")), &want) != nil {
			t.Errorf("%s: corrupt metadata changed: %q, want %q", n, got, s)
			continue
		}
		if err := json.Unmarshal([]byte(got), &have); err != nil {
			t.Errorf("%s: metadata became unreadable: %q", n, got)
			continue
		}
		for k, v := range want {
			if k == "name" {
				continue
			}
			if !bytes.Equal(bytes.TrimSpace(have[k]), bytes.TrimSpace(v)) {
				t.Errorf("%s: member %q = %s, want %s", n, k, have[k], v)
			}
		}
	}
	// Read-only version.json: instance.json is written, a failed removal of
	// the legacy file is tolerated, and the result is stable.
	if got := read(t, filepath.Join(inst, "RO", ".levilauncher", "instance.json")); got != variants["RO"] {
		t.Errorf("RO instance.json = %q", got)
	}
	t.Logf("RO version.json still present: %v", exists(filepath.Join(inst, "RO", "version.json")))
	again := migScan(t, inst)
	if strings.Join(migNames(again), ",") != strings.Join(migNames(names), ",") {
		t.Errorf("second scan lists %v, first %v", migNames(again), migNames(names))
	}
	migAssertNoop(t, local, e.roaming)
}

// ---------- P5 ----------

// P5-icons: the shortcut icon is generated into .levilauncher\icon.ico, and
// the legacy LargeLogo.ico, which unrepaired shortcuts may still use, stays.
func TestMigration_P5_Icons(t *testing.T) {
	e := setup(t)
	dir := filepath.Join(e.legacy, "versions", "A")
	png := "\x89PNG\r\n\x1a\nfake-png-bytes"
	write(t, filepath.Join(dir, "LargeLogo.png"), png)
	write(t, filepath.Join(dir, "LargeLogo.ico"), "old-ico")
	write(t, filepath.Join(dir, "version.json"), `{"name":"A"}`)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	oldIcon := filepath.Join(dir, "LargeLogo.ico")

	migPrepare(t, Migrated)
	inst, _ := apppath.InstancesDir()
	nd := filepath.Join(inst, "A")
	if !IsMovedLegacyPath(oldIcon) {
		t.Error("the shortcut's icon path is not a moved legacy path")
	}
	if got := TranslateLegacyPath(oldIcon); !samePath(got, filepath.Join(nd, "LargeLogo.ico")) || read(t, got) != "old-ico" {
		t.Errorf("Translate(icon) = %s", got)
	}
	if got := TranslateLegacyPath(filepath.Dir(oldIcon)); !samePath(got, nd) {
		t.Errorf("Translate(icon dir) = %s", got)
	}
	ico := icons.EnsureVersionIcon(nd, false)
	if !samePath(ico, filepath.Join(nd, ".levilauncher", "icon.ico")) {
		t.Fatalf("EnsureVersionIcon = %q", ico)
	}
	if b := read(t, ico); !strings.HasSuffix(b, png) {
		t.Errorf("icon.ico does not embed LargeLogo.png (%d bytes)", len(b))
	}
	if got := read(t, filepath.Join(nd, "LargeLogo.ico")); got != "old-ico" {
		t.Errorf("LargeLogo.ico = %q", got)
	}
	if got := read(t, filepath.Join(nd, "LargeLogo.png")); got != png {
		t.Error("LargeLogo.png changed")
	}
	if !migHidden(filepath.Join(nd, ".levilauncher")) {
		t.Error(".levilauncher not hidden")
	}
	if again := icons.EnsureVersionIcon(nd, false); !samePath(again, ico) {
		t.Errorf("second EnsureVersionIcon = %q", again)
	}
	if _, ok := migScan(t, inst)["A"]; !ok {
		t.Error("A not listed")
	}
	migLog(t, "A", nd)
	migAssertNoop(t, filepath.Join(e.local, "LeviLauncher"), e.roaming)
}

// ---------- P6 ----------

// P6-translate-legacy-path: legacy paths map to where the migration moved
// them, in every spelling, with prefix boundaries respected, and a child
// that a later migration placed under "-legacy" maps there.
func TestMigration_P6_TranslateLegacyPath(t *testing.T) {
	e := setup(t)
	S := e.legacy
	write(t, filepath.Join(S, "versions", "A", "version.json"), `{"name":"A","gameVersion":"1.21.1.1"}`)
	write(t, filepath.Join(S, "versions", "A", "sub", "f.txt"), "f")
	write(t, filepath.Join(S, "microsoft-account", "device.dpapi"), "dev")
	write(t, filepath.Join(S, "config.json"), `{}`)
	migPrepare(t, Migrated)
	N := filepath.Join(e.local, "LeviLauncher")
	inst := filepath.Join(N, "Instances")

	vA := filepath.Join(S, "versions", "A")
	cases := map[string]string{
		vA:                                filepath.Join(inst, "A"),
		vA + `\`:                          filepath.Join(inst, "A"),
		strings.ToUpper(vA):               filepath.Join(inst, "A"),
		filepath.ToSlash(vA):              filepath.Join(inst, "A"),
		`\\?\` + vA:                       filepath.Join(inst, "A"),
		filepath.Join(vA, "sub", "f.txt"): filepath.Join(inst, "A", "sub", "f.txt"),
		`\\?\` + filepath.Join(vA, "sub"): filepath.Join(inst, "A", "sub"),
		filepath.Join(S, "versions"):      inst,
		filepath.Join(S, "microsoft-account", "device.dpapi"): filepath.Join(N, "Accounts", "device.dpapi"),
	}
	for in, want := range cases {
		got := TranslateLegacyPath(in)
		if !samePath(got, want) {
			t.Errorf("Translate(%s) = %s, want %s", in, got, want)
		}
		if !IsMovedLegacyPath(in) {
			t.Errorf("IsMovedLegacyPath(%s) = false", in)
		}
	}
	if got := TranslateLegacyPath(filepath.Join(vA, "Sub", "F.txt")); got != filepath.Join(inst, "A", "Sub", "F.txt") {
		t.Errorf("case of the remainder not kept: %s", got)
	}
	for _, in := range []string{
		filepath.Join(S, "versionsX", "A"),
		filepath.Join(S, "versions2"),
		filepath.Join(S, "version"),
		`C:\Program Files\WindowsApps\Microsoft.MinecraftUWP_1.21.0.0_x64__8wekyb3d8bbwe`,
		filepath.Join(inst, "A"),
		"",
		"   ",
	} {
		if got := TranslateLegacyPath(in); got != in {
			t.Errorf("unrelated Translate(%q) = %q", in, got)
		}
		if strings.TrimSpace(in) != "" && IsMovedLegacyPath(in) {
			t.Errorf("IsMovedLegacyPath(%s) = true", in)
		}
	}

	// An older launcher runs again (downgrade): it creates another
	// versions\A, which the next migration places at A-legacy, and K.
	write(t, filepath.Join(S, "config.json"), `{}`)
	write(t, filepath.Join(S, "versions", "A", "version.json"), `{"name":"A","gameVersion":"1.20.0.1"}`)
	write(t, filepath.Join(S, "versions", "A", "Minecraft.Windows.exe"), "second-A")
	write(t, filepath.Join(S, "versions", "K", "version.json"), `{"name":"K"}`)
	migPrepare(t, Migrated)
	migLog(t, "instances after the second migration", inst)
	t.Logf("record:\n%s", read(t, apppath.MigrationPath()))
	if got := read(t, filepath.Join(inst, "A-legacy", "Minecraft.Windows.exe")); got != "second-A" {
		t.Fatalf("second A not at A-legacy: %q", got)
	}
	if got := read(t, filepath.Join(inst, "A", "sub", "f.txt")); got != "f" {
		t.Errorf("first A changed: %q", got)
	}
	for in, want := range map[string]string{
		filepath.Join(S, "versions", "K"): filepath.Join(inst, "K"),
		vA:                                filepath.Join(inst, "A-legacy"),
		filepath.Join(vA, "Minecraft.Windows.exe"):                  filepath.Join(inst, "A-legacy", "Minecraft.Windows.exe"),
		`\\?\` + strings.ToUpper(filepath.Join(vA, "version.json")): filepath.Join(inst, "A-legacy", "version.json"),
		filepath.Join(S, "microsoft-account", "device.dpapi"):       filepath.Join(N, "Accounts", "device.dpapi"),
	} {
		if got := TranslateLegacyPath(in); !samePath(got, want) {
			t.Errorf("after the second migration Translate(%s) = %s, want %s", in, got, want)
		}
		if !IsMovedLegacyPath(in) {
			t.Errorf("IsMovedLegacyPath(%s) = false", in)
		}
	}
	// A-legacy is listed under its folder name, reachable by that name.
	names := migScan(t, inst)
	if got := strings.Join(migNames(names), ","); got != "A,A-legacy,K" {
		t.Errorf("names = %s", got)
	}
	if m := names["A-legacy"]; m.GameVersion != "1.20.0.1" {
		t.Errorf("A-legacy = %+v", m)
	}
	if m := names["A"]; m.GameVersion != "1.21.1.1" {
		t.Errorf("A = %+v", m)
	}
	migAssertNoop(t, N, e.roaming)
}

// P6 across volumes: the legacy directory is on another volume, so the data
// root stays there under the current names and the accounts are copied;
// legacy paths of both translate to where the data is now.
func TestMigration_P6_TranslateAcrossVolumes(t *testing.T) {
	base := testSecondVolume
	if _, err := os.Stat(base); err != nil {
		t.Skip("second volume not available")
	}
	uniq, err := os.MkdirTemp(base, "p6-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(uniq) })
	e := setup(t)
	roaming := filepath.Join(uniq, "Roaming")
	if err := os.MkdirAll(roaming, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", roaming)
	S := apppath.LegacyDir()
	write(t, filepath.Join(S, "versions", "A", "version.json"), `{"name":"A"}`)
	write(t, filepath.Join(S, "versions", "A", "Minecraft.Windows.exe"), "exe")
	write(t, filepath.Join(S, "microsoft-account", "device.dpapi"), "dev")
	write(t, filepath.Join(S, "config.json"), `{"window_width":700}`)
	migPrepare(t, Migrated)
	N := filepath.Join(e.local, "LeviLauncher")
	migLog(t, "roaming", roaming)

	cfg := readJSON(t, apppath.ConfigPath())
	if br, _ := cfg["base_root"].(string); !samePath(br, S) || cfg["window_width"] != float64(700) {
		t.Errorf("config = %v; want base_root %s", cfg, S)
	}
	// config.Load hands base_root to apppath; do the same here.
	br, _ := cfg["base_root"].(string)
	apppath.SetBaseRootOverride(br)
	t.Cleanup(func() { apppath.SetBaseRootOverride("") })
	inst, _ := apppath.InstancesDir()
	if !samePath(inst, filepath.Join(S, "Instances")) {
		t.Errorf("InstancesDir = %s, want %s", inst, filepath.Join(S, "Instances"))
	}
	if got := read(t, filepath.Join(S, "Instances", "A", "Minecraft.Windows.exe")); got != "exe" {
		t.Errorf("instance = %q", got)
	}
	if got := read(t, filepath.Join(apppath.AccountsDir(), "device.dpapi")); got != "dev" {
		t.Errorf("accounts copy = %q", got)
	}
	if got := read(t, filepath.Join(S, "microsoft-account", "device.dpapi")); got != "dev" {
		t.Error("accounts original not kept")
	}
	// The original was copied and kept: a path that still exists is not
	// translated.
	if got := TranslateLegacyPath(filepath.Join(S, "microsoft-account", "device.dpapi")); !samePath(got, filepath.Join(S, "microsoft-account", "device.dpapi")) {
		t.Errorf("Translate(accounts) = %s", got)
	}
	if got := TranslateLegacyPath(filepath.Join(S, "versions", "A")); !samePath(got, filepath.Join(S, "Instances", "A")) || !exists(got) {
		t.Errorf("Translate(versions\\A) = %s", got)
	}
	if _, ok := migScan(t, inst)["A"]; !ok {
		t.Error("A not listed")
	}
	if !exists(filepath.Join(S, "versions.migrated")) && exists(filepath.Join(S, "versions")) {
		t.Error("legacy versions still marks the layout as not migrated")
	}
	migAssertNoop(t, N, roaming)
}

// ---------- P7 ----------

// migReconcile is the comparison ReconcileRegisteredFlags
// (mcservice/versions.go) and ListVersionMetasWithRegistered
// (app/service_version.go) make for an InstallLocation Windows reports.
func migReconcile(inst, installLocation string, metas map[string]versions.VersionMeta) (canonical, normalized []string) {
	want := utils.CanonicalWindowsPath(TranslateLegacyPath(installLocation))
	norm := func(p string) string {
		n := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
		n = strings.TrimPrefix(n, `\\?\`)
		n = strings.TrimPrefix(n, `\??\`)
		return strings.TrimSuffix(n, `\`)
	}
	wantNorm := norm(TranslateLegacyPath(installLocation))
	for _, n := range migNames(metas) {
		dir := filepath.Join(inst, strings.TrimSpace(metas[n].Name))
		if want != "" && utils.CanonicalWindowsPath(dir) == want {
			canonical = append(canonical, n)
		}
		if wantNorm != "" && norm(dir) == wantNorm {
			normalized = append(normalized, n)
		}
	}
	return
}

// P7-registered-packages: a registration Windows recorded at the legacy
// location is recognised for the moved instance (so the flag stays and the
// instance is re-registered at the next launch); a registration of a folder
// a pre-PR rename moved away matches nothing.
func TestMigration_P7_RegisteredComparisons(t *testing.T) {
	e := setup(t)
	S := e.legacy
	write(t, filepath.Join(S, "versions", "A", "version.json"), `{"name":"A","registered":true}`)
	write(t, filepath.Join(S, "versions", "A", "MicrosoftGame.config"), `<Game/>`)
	write(t, filepath.Join(S, "versions", "New", "version.json"), `{"name":"New"}`)
	write(t, filepath.Join(S, "config.json"), `{}`)
	migPrepare(t, Migrated)
	inst, _ := apppath.InstancesDir()
	metas := migScan(t, inst)
	if !metas["A"].Registered {
		t.Errorf("A lost its registered flag: %+v", metas["A"])
	}
	legacyA := filepath.Join(S, "versions", "A")
	for _, loc := range []string{legacyA, legacyA + `\`, strings.ToUpper(legacyA), `\\?\` + legacyA} {
		c, n := migReconcile(inst, loc, metas)
		if strings.Join(c, ",") != "A" || strings.Join(n, ",") != "A" {
			t.Errorf("registration at %s matches %v / %v; want A", loc, c, n)
		}
		if !IsMovedLegacyPath(loc) {
			t.Errorf("IsMovedLegacyPath(%s) = false: the launch would not renew the registration", loc)
		}
	}
	// Renamed away before the migration (Old -> New): Windows still records
	// versions\Old.
	if c, n := migReconcile(inst, filepath.Join(S, "versions", "Old"), metas); len(c) != 0 || len(n) != 0 {
		t.Errorf("stale Old registration matches %v / %v", c, n)
	}
	// A registration at the current location is not "moved".
	if IsMovedLegacyPath(filepath.Join(inst, "A")) {
		t.Error("current location reported as moved")
	}
	if c, _ := migReconcile(inst, filepath.Join(inst, "A"), metas); strings.Join(c, ",") != "A" {
		t.Errorf("current registration matches %v", c)
	}
	// A Store installation matches nothing.
	if c, n := migReconcile(inst, `C:\Program Files\WindowsApps\Microsoft.MinecraftUWP_1.21.0.0_x64__8wekyb3d8bbwe`, metas); len(c)+len(n) != 0 {
		t.Errorf("store registration matches %v / %v", c, n)
	}

	// Downgrade, then upgrade: the older launcher created and registered a
	// different versions\A, which the next migration places at A-legacy.
	// The registration at versions\A is now that of A-legacy.
	write(t, filepath.Join(S, "config.json"), `{}`)
	write(t, filepath.Join(S, "versions", "A", "version.json"), `{"name":"A","registered":true,"gameVersion":"1.20.0.1"}`)
	migPrepare(t, Migrated)
	metas = migScan(t, inst)
	if c, n := migReconcile(inst, legacyA, metas); strings.Join(c, ",") != "A-legacy" || strings.Join(n, ",") != "A-legacy" {
		t.Errorf("after the second migration the registration at versions\\A matches %v / %v; want A-legacy", c, n)
	}
	migAssertNoop(t, filepath.Join(e.local, "LeviLauncher"), e.roaming)
}
