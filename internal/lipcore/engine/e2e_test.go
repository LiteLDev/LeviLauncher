package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/config"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// A differential test against the reference lip 0.40 CLI: every scenario
// runs lip in one workspace and the engine (server side, platform
// win-x64) in another, step by step; after every step the file trees (paths,
// contents, hidden attributes; .lip excluded), the bytes of
// .lip/state.json, the list output and the success of the step must be
// the same.
//
//	LIP_E2E=1 LIP_REF=C:\...\lip.exe go test ./internal/lipcore/engine -run E2E -v -timeout 30m
//
// Packages of local directories cannot depend on each other (dependency keys
// are remote repositories), so dependencies are exercised by
// github.com/LiteLDev/LeviLamina, which needs network access.

type e2eStep struct {
	op    string // install, update, uninstall, list, add, mutate
	args  []string
	flags []string
	// files are path, content pairs written into both workspaces (add) or
	// into a fixture (mutate, args[0] its name).
	files []string
}

type e2eFixture struct {
	name  string
	json  string
	files []string // path, content pairs
}

type e2eScenario struct {
	name     string
	fixtures []e2eFixture
	steps    []e2eStep
	network  bool
}

const (
	e2eLL      = "github.com/LiteLDev/LeviLamina"
	e2eLLAsset = "https://github.com/LiteLDev/LeviLamina/releases/download/v26.60.4/levilamina-v26.60.4-server-release-windows-x64.zip"
)

func e2eJSON(id, version, members string) string {
	return fmt.Sprintf(`{ "format": 4, "id": %q, "version": %q, "variants": { "": [ { "side": "server",`+
		` "platform": "win-x64"%s } ] } }`, id, version, members)
}

func e2eScenarios() []e2eScenario {
	fixA := func(version, content string) e2eFixture {
		return e2eFixture{name: "a", json: e2eJSON("example.com/e2e/a", version, `, "assets": [ { "type": "self", "place": [
		  { "src": "a.txt", "dest": "a/a.txt" }, { "src": "dir/", "dest": "d/" }, { "src": "*.cfg", "dest": "cfg/" } ] } ],
		  "preserve": [ "cfg/keep.cfg" ], "purge": [ "a/cache/", "*.log" ],
		  "hooks": { "post_install": [ "echo installed> hook-a.txt" ], "pre_uninstall": [ "echo removing> hook-a-un.txt" ] }`),
			files: []string{"a.txt", content, "dir/x.txt", "x", "dir/sub/y.txt", "y", "keep.cfg", "default", "other.cfg", "o"}}
	}
	fixB := e2eFixture{name: "b", json: e2eJSON("example.com/e2e/b", "1.0.0",
		`, "assets": [ { "type": "self", "place": [ { "src": "b.txt", "dest": "b/b.txt" } ] } ]`),
		files: []string{"b.txt", "b"}}
	fixConflict := e2eFixture{name: "conflict", json: e2eJSON("example.com/e2e/conflict", "1.0.0",
		`, "assets": [ { "type": "self", "place": [ { "src": "c.txt", "dest": "a/a.txt" } ] } ]`),
		files: []string{"c.txt", "c"}}
	fixFail := e2eFixture{name: "fail", json: e2eJSON("example.com/e2e/fail", "1.0.0",
		`, "assets": [ { "type": "self", "place": [ { "src": "f.txt", "dest": "f/f.txt" } ] } ],
		  "hooks": { "post_install": [ "exit 1" ] }`),
		files: []string{"f.txt", "f"}}
	fixVariants := e2eFixture{name: "v", json: `{ "format": 4, "id": "example.com/e2e/v", "version": "2.0.0", "variants": {
		  "": [ { "side": "server", "platform": "win-x64", "assets": [ { "type": "self", "place": [ { "src": "v.txt", "dest": "v/default.txt" } ] } ] } ],
		  "extra": [ { "side": "server", "platform": "win-x64", "assets": [ { "type": "self", "place": [ { "src": "v.txt", "dest": "v/extra.txt" } ] } ] },
		    { "side": "server", "platform": "linux-x64" } ] } }`,
		files: []string{"v.txt", "v"}}
	fixZip := e2eFixture{name: "zipasset", json: e2eJSON("example.com/e2e/zipasset", "1.0.0",
		`, "assets": [ { "type": "self", "place": [ { "src": "s.txt", "dest": "z/s.txt" } ] },
		  { "type": "zip", "urls": [ "`+e2eLLAsset+`" ], "place": [ { "src": "LeviLamina/", "dest": "z/ll/" } ] } ]`),
		files: []string{"s.txt", "s"}}
	fixTooth := e2eFixture{name: "tooth", json: "", files: []string{"tooth.json", `{
	  "format_version": 2, "tooth": "example.com/e2e/tooth", "version": "0.1.0",
	  "files": { "place": [ { "src": "t/*", "dest": "tt" } ], "preserve": [ "tt/keep.txt" ] } }`,
		"t/one.txt", "1", "t/keep.txt", "k"}}
	fixTooth3 := e2eFixture{name: "tooth3", files: []string{"tooth.json", `{
	  "format_version": 3, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
	  "tooth": "example.com/e2e/tooth3", "version": "0.2.0",
	  "variants": [
	    { "label": "", "platform": "win-x64",
	      "assets": [ { "type": "self", "placements": [ { "type": "dir", "src": "t", "dest": "tt" },
	        { "type": "file", "src": "one.txt", "dest": "one/one.txt" } ] } ],
	      "preserve_files": [ "keep.txt" ], "remove_files": [ "tt/cache" ],
	      "scripts": { "install": [ "echo t3> t3.txt" ], "pre_uninstall": [ "echo bye> t3-bye.txt" ] } },
	    { "label": "client", "assets": [ { "type": "self", "placements": [ { "type": "file", "src": "one.txt", "dest": "client.txt" } ] } ] } ] }`,
		"t/a.txt", "a", "t/keep.txt", "k", "one.txt", "1"}}

	in := func(args ...string) e2eStep { return e2eStep{op: "install", args: args} }
	inF := func(flags []string, args ...string) e2eStep { return e2eStep{op: "install", args: args, flags: flags} }
	up := func(args ...string) e2eStep { return e2eStep{op: "update", args: args} }
	un := func(args ...string) e2eStep { return e2eStep{op: "uninstall", args: args} }
	unF := func(flags []string, args ...string) e2eStep {
		return e2eStep{op: "uninstall", args: args, flags: flags}
	}
	list := e2eStep{op: "list"}
	add := func(files ...string) e2eStep { return e2eStep{op: "add", files: files} }
	dry := []string{"--dry-run"}
	noDeps := []string{"--no-dependencies"}
	noScripts := []string{"--ignore-scripts"}
	a2 := fixA("1.0.1", "a2")

	return []e2eScenario{
		{name: "lifecycle", fixtures: []e2eFixture{fixA("1.0.0", "a1")}, steps: []e2eStep{
			add("cfg/keep.cfg", "mine"), in("@a"), list, up(), up("@a"),
			{op: "mutate", args: []string{"a"}, files: append([]string{"lip.toml", a2.json}, a2.files...)},
			up(), list, add("a/cache/tmp", "t", "x.log", "l"), un("example.com/e2e/a"), list,
		}},
		{name: "dryrun", fixtures: []e2eFixture{fixA("1.0.0", "a1"), fixB}, steps: []e2eStep{
			inF(dry, "@a"), in("@a"), inF(dry, "@b"), unF(dry, "example.com/e2e/a"), e2eStep{op: "update", flags: dry}, list,
		}},
		{name: "nodeps", fixtures: []e2eFixture{fixA("1.0.0", "a1"), fixB}, steps: []e2eStep{
			inF(noDeps, "@a"), in("@b"), inF(noDeps, "@b"), unF(noDeps, "example.com/e2e/a", "example.com/e2e/b"), list,
		}},
		{name: "failhook", fixtures: []e2eFixture{fixA("1.0.0", "a1"), fixB, fixFail}, steps: []e2eStep{
			in("@fail"), in("@b"), in("@a", "@fail"), list, inF(noScripts, "@fail"), un("example.com/e2e/fail"),
		}},
		{name: "conflict", fixtures: []e2eFixture{fixA("1.0.0", "a1"), fixConflict}, steps: []e2eStep{
			in("@a"), in("@conflict"), in("@a"), un("example.com/e2e/conflict"), in("@a", "@a"), list,
		}},
		{name: "variants", fixtures: []e2eFixture{fixVariants}, steps: []e2eStep{
			in("@v#extra", "@v"), list, un("example.com/e2e/v"), list, un("@v"), list,
		}},
		{name: "ignorescripts", fixtures: []e2eFixture{fixA("1.0.0", "a1")}, steps: []e2eStep{
			inF(noScripts, "@a"), unF(noScripts, "example.com/e2e/a"),
		}},
		{name: "zipfile", fixtures: []e2eFixture{fixB}, steps: []e2eStep{
			in("@b.zip"), list, up(), un("example.com/e2e/b"),
		}},
		{name: "toothjson", fixtures: []e2eFixture{fixTooth, fixTooth3}, steps: []e2eStep{
			add("tt/keep.txt", "mine"), in("@tooth"), list, un("example.com/e2e/tooth"),
			in("@tooth3"), list, add("tt/cache/x", "x"), up(), un("example.com/e2e/tooth3"), list,
			in("@tooth3#client"), list, un("example.com/e2e/tooth3#client"),
		}},
		{name: "unknown", steps: []e2eStep{
			in("./missing"), in("https://example.invalid/x.zip"), un("example.com/none"), up("example.com/none"), list,
		}},
		{name: "zipasset", network: true, fixtures: []e2eFixture{fixZip}, steps: []e2eStep{
			in("@zipasset"), list, un("example.com/e2e/zipasset"),
		}},
		{name: "toothlock", network: true, fixtures: []e2eFixture{fixB}, steps: []e2eStep{
			add("ll.txt", "ll", "keep/x.txt", "x", "tooth_lock.json", `{ "format_version": 3,
			  "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d", "packages": [ { "files": [ "ll.txt", "keep/x.txt" ],
			  "locked": true, "manifest": { "format_version": 3, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
			  "tooth": "`+e2eLL+`", "version": "1.0.0", "variants": [ { "label": "", "platform": "win-x64",
			  "assets": [ { "type": "self", "placements": [ { "type": "file", "src": "ll.txt", "dest": "ll.txt" } ] } ],
			  "preserve_files": [ "keep/x.txt" ] } ] },
			  "variant": "" } ] }`),
			list, in("@b"), list, un(e2eLL), list,
		}},
		{name: "levilamina", network: true, steps: []e2eStep{
			inF(noScripts, e2eLL), list, inF(noScripts, e2eLL), unF(noScripts, e2eLL), list,
		}},
	}
}

func writeE2EFixture(t *testing.T, dir string, f e2eFixture) {
	t.Helper()
	path := filepath.Join(dir, f.name)
	if f.json != "" {
		writePairsTo(t, path, []string{"lip.toml", f.json})
	}
	writePairsTo(t, path, f.files)
}

func writePairsTo(t *testing.T, root string, pairs []string) {
	t.Helper()
	for i := 0; i+1 < len(pairs); i += 2 {
		p := filepath.Join(root, filepath.FromSlash(pairs[i]))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(tomlFixture(pairs[i], pairs[i+1])), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// zipDir writes a ZIP file of the files below dir.
func zipDir(t *testing.T, dir, output string) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		f, err := w.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		return err
	})
	if err == nil {
		err = w.Close()
	}
	if err == nil {
		err = os.WriteFile(output, buf.Bytes(), 0o644)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// e2eTree is the workspace without .lip: paths, contents and the hidden
// attribute of files; directories.
func e2eTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if strings.EqualFold(rel, ".lip") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			tree[rel] = "<dir>"
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		hidden, _ := txn.OS.Hidden(p)
		tree[rel] = fmt.Sprintf("%q hidden=%v", b, hidden)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func diffTrees(a, b map[string]string) string {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	var sorted []string
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	var sb strings.Builder
	for _, k := range sorted {
		if a[k] != b[k] {
			fmt.Fprintf(&sb, "  %s: lip=%q ours=%q\n", k, a[k], b[k])
		}
	}
	return sb.String()
}

func readOrNone(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return "<none>"
	}
	return string(b)
}

func TestE2EDifferentialAgainstLip(t *testing.T) {
	refLip := os.Getenv("LIP_REF")
	if os.Getenv("LIP_E2E") != "1" || refLip == "" {
		t.Skip("set LIP_E2E=1 and LIP_REF to the lip executable")
	}
	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	proxy := ""
	if cfg.HasGithubProxy {
		proxy = cfg.GithubProxy
	}
	for _, sc := range e2eScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			if sc.network && os.Getenv("LIP_E2E_OFFLINE") == "1" {
				t.Skip("network scenario")
			}
			base := t.TempDir()
			fixtures := filepath.Join(base, "fixtures")
			for _, f := range sc.fixtures {
				writeE2EFixture(t, fixtures, f)
			}
			if _, err := os.Stat(filepath.Join(fixtures, "b")); err == nil {
				zipDir(t, filepath.Join(fixtures, "b"), filepath.Join(fixtures, "b.zip"))
			}
			wsA, wsB := filepath.Join(base, "lip"), filepath.Join(base, "ours")
			for _, ws := range []string{wsA, wsB} {
				if err := os.MkdirAll(ws, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			resolveArgs := func(args []string) []string {
				out := make([]string, len(args))
				for i, a := range args {
					if strings.HasPrefix(a, "@") {
						a = filepath.Join(fixtures, a[1:])
					}
					out[i] = a
				}
				return out
			}

			for i, step := range sc.steps {
				switch step.op {
				case "add":
					writePairsTo(t, wsA, step.files)
					writePairsTo(t, wsB, step.files)
					continue
				case "mutate":
					writePairsTo(t, filepath.Join(fixtures, step.args[0]), step.files)
					continue
				}
				args := resolveArgs(step.args)
				var cliArgs []string
				switch step.op {
				case "list":
					cliArgs = []string{"list", "--json"}
				default:
					cliArgs = append(append([]string{step.op}, args...), step.flags...)
					cliArgs = append(cliArgs, "--platform", "win-x64")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
				cmd := exec.CommandContext(ctx, refLip, cliArgs...)
				cmd.Dir = wsA
				refOut, refErr := cmd.CombinedOutput()
				cancel()
				refOK := refErr == nil

				e := New(wsB, Options{Side: "server", Platform: "win-x64", GithubProxy: proxy}, report.Nop{})
				has := func(f string) bool {
					for _, x := range step.flags {
						if x == f {
							return true
						}
					}
					return false
				}
				dry, noScripts, noDeps := has("--dry-run"), has("--ignore-scripts"), has("--no-dependencies")
				var ourErr error
				var ourList string
				ctx = context.Background()
				switch step.op {
				case "install":
					ourErr = e.Install(ctx, args, dry, noScripts, noDeps)
				case "update":
					ourErr = e.Update(ctx, args, dry, noScripts)
				case "uninstall":
					ourErr = e.Uninstall(ctx, args, dry, noScripts, noDeps)
				case "list":
					explicit, implicit, err := e.List(ctx)
					ourErr = err
					b, _ := json.Marshal(map[string][]string{"ExplicitInstalled": explicit, "ImplicitInstalled": implicit})
					ourList = string(b)
				}
				ourOK := ourErr == nil
				label := fmt.Sprintf("step %d (%s %v %v)", i, step.op, step.args, step.flags)
				if refOK != ourOK {
					t.Errorf("%s: lip ok=%v, ours ok=%v\nours: %s\nlip output:\n%s", label, refOK, ourOK,
						FormatError(ourErr), refOut)
				} else if !refOK {
					squash := func(s string) string { return strings.Join(strings.Fields(s), "") }
					// The workspaces differ; the messages may name them.
					first := strings.ReplaceAll(strings.SplitN(ourErr.Error(), "\n", 2)[0], wsB, wsA)
					if !strings.Contains(squash(string(refOut)), squash(first)) {
						t.Errorf("%s: lip did not print our message %q\nlip output:\n%s", label, first, refOut)
					}
				}
				if step.op == "list" && refOK && ourOK {
					var refList, oursParsed map[string][]string
					start := bytes.IndexByte(refOut, '{')
					if start < 0 || json.Unmarshal(refOut[start:], &refList) != nil {
						t.Errorf("%s: cannot read lip's list: %s", label, refOut)
					} else {
						_ = json.Unmarshal([]byte(ourList), &oursParsed)
						if !reflect.DeepEqual(norm(refList), norm(oursParsed)) {
							t.Errorf("%s: list differs\nlip:  %v\nours: %v", label, refList, oursParsed)
						}
					}
				}
				if a, b := e2eTree(t, wsA), e2eTree(t, wsB); !reflect.DeepEqual(a, b) {
					t.Errorf("%s: trees differ\n%s\nlip output:\n%s", label, diffTrees(a, b), refOut)
				}
				wa := readOrNone(filepath.Join(wsA, ".lip", "state.json"))
				wb := readOrNone(filepath.Join(wsB, ".lip", "state.json"))
				if wa != wb {
					t.Errorf("%s: state.json differs\nlip:\n%s\nours:\n%s", label, wa, wb)
				}
				if testing.Verbose() {
					t.Logf("%s: lip ok=%v, ours err=%v", label, refOK, ourErr)
				}
			}
		})
	}
}

func norm(m map[string][]string) map[string][]string {
	out := map[string][]string{}
	for k, v := range m {
		if v == nil {
			v = []string{}
		}
		out[k] = v
	}
	return out
}
