package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// A differential test against the reference lip 0.38 CLI: every scenario
// runs `lip install/uninstall` in one workspace and this installer, driven
// as McmodService.Execute drives it, in another; the resulting trees
// (paths, contents, hidden attributes; .lip excluded) and the success of
// every step must be the same.
//
//	LIP_E2E=1 LIP_REF=C:\...\lip.exe go test -run TestE2E ./internal/lipcore/install

type e2ePkg struct {
	name, version string
	entry         pkgmodel.Entry
	files         []string // path, content pairs
}

func (p *e2ePkg) id() string { return "example.com/t/" + p.name }

type e2eStep struct {
	install, uninstall *e2ePkg
	add                []string // path, content pairs to create
}

type e2eCase struct {
	name  string
	setup []string // path, content pairs
	steps []e2eStep
}

func e2eHooks(pre, post, preUn, postUn []string) pkgmodel.Hooks {
	return pkgmodel.Hooks{PreInstall: pre, PostInstall: post, PreUninstall: preUn, PostUninstall: postUn}
}

func e2eCases() []e2eCase {
	self := func(places ...pkgmodel.Placement) []pkgmodel.Asset { return selfAsset(places...) }
	install := func(p *e2ePkg) e2eStep { return e2eStep{install: p} }
	uninstall := func(p *e2ePkg) e2eStep { return e2eStep{uninstall: p} }
	add := func(pairs ...string) e2eStep { return e2eStep{add: pairs} }

	file := &e2ePkg{name: "file", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("a.txt", "a.txt"), place("bin/x.dll", "tools/x.dll"))},
		files: []string{"a.txt", "a", "bin/x.dll", "x", "README.md", "readme"}}
	dir := &e2ePkg{name: "dir", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("assets/", "rp/pack/"), place("", "copy/"), place("defaults/", ""))},
		files: []string{"assets/a.txt", "a", "assets/sub/b.txt", "b", "assets/lip.yaml", "nested",
			"defaults/server.properties", "props"}}
	glob := &e2ePkg{name: "glob", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("bin/**/*.dll", "tools/pack/"), place("*.json", "cfg/"),
			place("lib/a*/x.txt", "out/"))},
		files: []string{"bin/a.dll", "a", "bin/x/b.dll", "b", "bin/c.pdb", "c", "tooth.json", "{}", "a.json", "j",
			"lib/ab/x.txt", "x"}}
	later := &e2ePkg{name: "later", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("one.txt", "out.txt"), place("two.txt", "out.txt"),
			place("one.txt", "Case.txt"), place("two.txt", "CASE.txt"))},
		files: []string{"one.txt", "1", "two.txt", "2"}}
	preserve := &e2ePkg{name: "preserve", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("cfg/", "cfg/")), Preserve: []string{"cfg/*.ini"}},
		files: []string{"cfg/a.ini", "default", "cfg/b.ini", "b", "cfg/c.txt", "c"}}
	purge := &e2ePkg{name: "purge", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("a.txt", "a.txt"), place("a.txt", "mods/M/a.txt")),
			Purge: []string{"logs/", "*.log", "cache/", "mods/M/data/", "mods/", "*/"}},
		files: []string{"a.txt", "a"}}
	hooks := &e2ePkg{name: "hooks", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("a.txt", "a.txt")), Hooks: e2eHooks(
			[]string{"echo pre> pre.txt"}, []string{"echo post> post.txt", "if exist a.txt echo yes> post2.txt"},
			[]string{"if exist a.txt echo yes> pre_un.txt"}, []string{"echo done> post_un.txt"})},
		files: []string{"a.txt", "a"}}
	failPost := &e2ePkg{name: "failpost", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("a.txt", "existing.txt"), place("b.txt", "new/dir/b.txt")),
			Hooks: e2eHooks(nil, []string{"echo ran> ran.txt", "exit 1"}, nil, nil)},
		files: []string{"a.txt", "a", "b.txt", "b"}}
	failUn := &e2ePkg{name: "failun", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("", "deep/")), Hooks: e2eHooks(nil, nil, nil, []string{"exit 3"})},
		files: []string{"a.txt", "a", "x/y/z.txt", "z"}}
	owner := &e2ePkg{name: "owner", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("x.txt", "x.txt"))}, files: []string{"x.txt", "owner"}}
	intruder := &e2ePkg{name: "intruder", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("x.txt", "new.txt"), place("x.txt", "x.TXT"), place("x.txt", "X.TXT"))},
		files: []string{"x.txt", "intruder"}}
	dots := &e2ePkg{name: "dots", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("a.txt", "x/./a.txt"), place("a.txt", "lip.yaml"),
			place("a.txt", "tooth.json"))},
		files: []string{"a.txt", "a"}}
	overwrite := &e2ePkg{name: "overwrite", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("a.txt", "foo"), place("b.txt", ".foo.bak"), place("a.txt", "dir/existing.txt"))},
		files: []string{"a.txt", "a", "b.txt", "b"}}
	invalid := func(name, dest string) *e2ePkg {
		return &e2ePkg{name: name, version: "1.0.0",
			entry: pkgmodel.Entry{Assets: self(place("a.txt", "ok.txt"), place("a.txt", dest))},
			files: []string{"a.txt", "a"}}
	}
	missing := &e2ePkg{name: "missing", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("a.txt", "a.txt"), place("missing/", "m/")),
			Hooks: e2eHooks([]string{"echo pre> pre.txt"}, nil, nil, nil)},
		files: []string{"a.txt", "a"}}
	inTheWay := &e2ePkg{name: "intheway", version: "1.0.0",
		entry: pkgmodel.Entry{Assets: self(place("a.txt", "d/a.txt"), place("a.txt", "f/x.txt"))},
		files: []string{"a.txt", "a"}}

	return []e2eCase{
		{name: "file", steps: []e2eStep{install(file), uninstall(file)}},
		{name: "dir", steps: []e2eStep{install(dir), uninstall(dir)}},
		{name: "glob", steps: []e2eStep{install(glob), uninstall(glob)}},
		{name: "later", steps: []e2eStep{install(later), uninstall(later)}},
		{name: "preserve", setup: []string{"cfg/a.ini", "user"}, steps: []e2eStep{install(preserve),
			add("cfg/c.ini", "c"), uninstall(preserve)}},
		{name: "purge", setup: []string{"mods/Other/x.dll", "x"}, steps: []e2eStep{install(purge),
			add("logs/x/1.log", "1", "top.log", "t", "worlds/deep.log", "d", "cache/a/b.bin", "b",
				"mods/M/data/db/m.db", "m", "keep.txt", "k"), uninstall(purge)}},
		{name: "hooks", steps: []e2eStep{install(hooks), uninstall(hooks)}},
		{name: "failpost", setup: []string{"existing.txt", "orig"}, steps: []e2eStep{install(failPost)}},
		{name: "failun", steps: []e2eStep{install(failUn), uninstall(failUn)}},
		{name: "conflict", steps: []e2eStep{install(owner), install(intruder), uninstall(owner), install(intruder)}},
		{name: "dots", steps: []e2eStep{install(dots), uninstall(dots)}},
		{name: "overwrite", setup: []string{"foo", "orig", ".foo.bak", "someone's", "dir/existing.txt", "e"},
			steps: []e2eStep{install(overwrite), uninstall(overwrite)}},
		{name: "trailingspace", steps: []e2eStep{install(invalid("trailingspace", "a.txt "))}},
		{name: "trailingdot", steps: []e2eStep{install(invalid("trailingdot", "dir./a.txt"))}},
		{name: "colon", steps: []e2eStep{install(invalid("colon", "c/a:b.txt"))}},
		{name: "lipdir", steps: []e2eStep{install(invalid("lipdir", ".lip/x.txt"))}},
		{name: "lipdircase", steps: []e2eStep{install(invalid("lipdircase", ".LIP/x.txt"))}},
		{name: "missing", steps: []e2eStep{install(missing)}},
		{name: "intheway", setup: []string{"d/a.txt/z", "dir in the way"}, steps: []e2eStep{install(inTheWay)}},
		{name: "parentfile", setup: []string{"f", "a file"}, steps: []e2eStep{install(inTheWay)}},
	}
}

// writeFixture writes the package as a local directory archive.
func writeFixture(t *testing.T, dir string, p *e2ePkg) string {
	t.Helper()
	path := filepath.Join(dir, p.name+"-"+p.version)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	type placeJSON struct {
		Src  string `json:"src"`
		Dest string `json:"dest"`
	}
	type assetJSON struct {
		Type  string      `json:"type"`
		URLs  []string    `json:"urls,omitempty"`
		Place []placeJSON `json:"place,omitempty"`
	}
	entry := map[string]any{"platform": "server", "runtime": "win-x64"}
	var assets []assetJSON
	for _, a := range p.entry.Assets {
		aj := assetJSON{Type: string(a.Type), URLs: a.URLs}
		for _, pl := range a.Place {
			aj.Place = append(aj.Place, placeJSON{pl.Src, pl.Dest})
		}
		assets = append(assets, aj)
	}
	if len(assets) > 0 {
		entry["assets"] = assets
	}
	if len(p.entry.Preserve) > 0 {
		entry["preserve"] = p.entry.Preserve
	}
	if len(p.entry.Purge) > 0 {
		entry["purge"] = p.entry.Purge
	}
	hooks := map[string][]string{}
	for k, v := range map[string][]string{"pre_install": p.entry.Hooks.PreInstall,
		"post_install": p.entry.Hooks.PostInstall, "pre_uninstall": p.entry.Hooks.PreUninstall,
		"post_uninstall": p.entry.Hooks.PostUninstall} {
		if len(v) > 0 {
			hooks[k] = v
		}
	}
	if len(hooks) > 0 {
		entry["hooks"] = hooks
	}
	doc := map[string]any{"format": 4, "id": p.id(), "version": p.version,
		"variants": map[string]any{"": []any{entry}}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "lip.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	writePairs(t, path, p.files)
	return path
}

func writePairs(t *testing.T, root string, pairs []string) {
	t.Helper()
	for i := 0; i+1 < len(pairs); i += 2 {
		p := filepath.Join(root, filepath.FromSlash(pairs[i]))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(pairs[i+1]), 0o644); err != nil {
			t.Fatal(err)
		}
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
		rel := filepath.ToSlash(strings.TrimPrefix(p, root))
		if rel == "" {
			return nil
		}
		if strings.EqualFold(rel, "/.lip") {
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

// e2eDriver installs and uninstalls with this package as McmodService
// does, keeping the workspace file in memory.
type e2eDriver struct {
	root      string
	installed []pkgmodel.LockedPackage
	entries   map[string]*pkgmodel.Entry
	inst      *Installer
}

func (d *e2eDriver) operation(t *testing.T, plan func(view *View) (*Plan, error)) error {
	t.Helper()
	ctx := context.Background()
	if err := txn.EnsureLipDir(d.root); err != nil {
		return err
	}
	mutex, err := txn.Lock(d.root)
	if err != nil {
		return err
	}
	defer mutex.Close()
	layout, err := PlanLayout(d.root)
	if err != nil {
		return err
	}
	view := NewView(layout)
	p, err := plan(view)
	if err != nil {
		return err
	}
	tx, err := txn.Begin(d.root, txn.Options{Context: ctx, Reserved: p.WrittenPaths()})
	if err != nil {
		return err
	}
	defer tx.Close()
	if err = ApplyLayout(tx, layout); err == nil {
		err = d.inst.Apply(ctx, tx, p)
	}
	if err != nil {
		if undo := tx.Rollback(nil); len(undo) > 0 {
			t.Errorf("rollback: %v", undo)
		}
		return err
	}
	_, err = tx.Commit(func() error {
		return os.WriteFile(filepath.Join(d.root, ".lip", "workspace.yaml.tmp"),
			[]byte("format: 4\nplatform: server\nruntime: win-x64\npackages: []\n"), 0o644)
	})
	return err
}

func (d *e2eDriver) install(t *testing.T, fixture string, p *e2ePkg) error {
	source, err := archive.NewDirectorySource(fixture)
	if err != nil {
		return err
	}
	entry := p.entry
	entry.Platform, entry.Runtime = pkgmodel.PlatformServer, "win-x64"
	pkg := &Package{ID: p.id(), Version: p.version, Entry: &entry, Archive: source,
		OpenAsset: func([]string) (archive.Source, error) {
			return nil, &archive.UnavailableError{Message: "no downloads in this test"}
		}}
	var plan *Plan
	err = d.operation(t, func(view *View) (*Plan, error) {
		var err error
		plan, err = d.inst.PlanInstall(context.Background(), view, Options{Root: d.root, Installed: d.installed}, pkg)
		return plan, err
	})
	if err == nil {
		d.installed = append(d.installed, locked(pkg, plan.OwnedFiles))
		d.entries[p.id()] = &entry
	}
	return err
}

func (d *e2eDriver) uninstall(t *testing.T, p *e2ePkg) error {
	idx := -1
	for i := range d.installed {
		if LockedIdentity(&d.installed[i]) == p.id() {
			idx = i
		}
	}
	if idx < 0 {
		return errors.New("not installed")
	}
	err := d.operation(t, func(view *View) (*Plan, error) {
		return d.inst.PlanUninstall(context.Background(), view, Options{Root: d.root, Installed: d.installed},
			&d.installed[idx], d.entries[p.id()])
	})
	if err == nil {
		d.installed = append(d.installed[:idx], d.installed[idx+1:]...)
	}
	return err
}

func TestE2EDifferentialAgainstLip(t *testing.T) {
	ref := os.Getenv("LIP_REF")
	if os.Getenv("LIP_E2E") != "1" || ref == "" {
		t.Skip("set LIP_E2E=1 and LIP_REF to the lip 0.38 executable")
	}
	for _, c := range e2eCases() {
		t.Run(c.name, func(t *testing.T) {
			base := t.TempDir()
			fixtures := filepath.Join(base, "fixtures")
			wsA, wsB := filepath.Join(base, "a"), filepath.Join(base, "b")
			for _, ws := range []string{wsA, wsB} {
				if err := os.MkdirAll(ws, 0o755); err != nil {
					t.Fatal(err)
				}
				writePairs(t, ws, c.setup)
			}
			driver := &e2eDriver{root: wsB, entries: map[string]*pkgmodel.Entry{}, inst: &Installer{}}

			for i, step := range c.steps {
				var refOK, ourOK bool
				var refOut []byte
				var ourErr error
				switch {
				case step.add != nil:
					writePairs(t, wsA, step.add)
					writePairs(t, wsB, step.add)
					continue
				case step.install != nil:
					fixture := writeFixture(t, fixtures, step.install)
					cmd := exec.Command(ref, "install", fixture, "--rid", "win-x64")
					cmd.Dir = wsA
					var err error
					refOut, err = cmd.CombinedOutput()
					refOK = err == nil
					ourErr = driver.install(t, fixture, step.install)
				case step.uninstall != nil:
					cmd := exec.Command(ref, "uninstall", step.uninstall.id(), "--rid", "win-x64")
					cmd.Dir = wsA
					var err error
					refOut, err = cmd.CombinedOutput()
					refOK = err == nil
					ourErr = driver.uninstall(t, step.uninstall)
				}
				ourOK = ourErr == nil
				if !refOK && !ourOK {
					compareMessage(t, i, ourErr, refOut)
				}
				if refOK != ourOK {
					t.Errorf("step %d: lip ok=%v, ours ok=%v (%v)\nlip output:\n%s", i, refOK, ourOK, ourErr, refOut)
				}
				if testing.Verbose() {
					t.Logf("step %d: lip ok=%v ours err=%v", i, refOK, ourErr)
				}
				a, b := e2eTree(t, wsA), e2eTree(t, wsB)
				if !reflect.DeepEqual(a, b) {
					t.Errorf("step %d: trees differ\n%s\nlip output:\n%s", i, diffTrees(a, b), refOut)
				}
			}
		})
	}
}

func diffTrees(a, b map[string]string) string {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	var sortedKeys []string
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	var sb strings.Builder
	for _, k := range sortedKeys {
		if a[k] != b[k] {
			fmt.Fprintf(&sb, "  %s: lip=%q ours=%q\n", k, a[k], b[k])
		}
	}
	return sb.String()
}

// compareMessage checks that lip printed the message of our error. The
// console wraps lines, so whitespace is ignored.
func compareMessage(t *testing.T, step int, err error, out []byte) {
	t.Helper()
	var ie *Error
	var le *LayoutError
	var msg string
	switch {
	case errors.As(err, &ie):
		msg = ie.Msg
	case errors.As(err, &le):
		msg = le.Msg
	default:
		return
	}
	squash := func(s string) string { return strings.Join(strings.Fields(s), "") }
	if !strings.Contains(squash(string(out)), squash(msg)) {
		t.Errorf("step %d: lip did not print our message %q\nlip output:\n%s", step, msg, out)
	}
}
