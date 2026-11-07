package install

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// The Go counterpart of WorkspaceHarness.cs and SoloInstaller: a workspace
// in t.TempDir(), packages built from in-memory sources, and helpers that
// plan and apply one package (or one operation) through a transaction.

// makeDirLink links link to the directory target: a symbolic link, or on
// Windows without the privilege for one, a junction.
func makeDirLink(target, link string) error {
	err := os.Symlink(target, link)
	if err != nil && runtime.GOOS == "windows" {
		if out, jerr := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput(); jerr != nil {
			return fmt.Errorf("%v; mklink /J: %v %s", err, jerr, out)
		}
		return nil
	}
	return err
}

func hiddenAttr(p string) (bool, error) { return txn.OS.Hidden(p) }

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// memSource is InMemorySource of McmodInstallerTests.cs: files in memory,
// in insertion order, recording what was prefetched.
type memSource struct {
	keys  []string
	files map[string]string

	mu           sync.Mutex
	prefetched   []string
	failing      map[string]bool
	failPrefetch bool
	onOpen       func(key string)
	noSize       bool
}

// src makes a source of key/content pairs, in order.
func src(pairs ...string) *memSource {
	s := &memSource{files: map[string]string{}, failing: map[string]bool{}}
	for i := 0; i+1 < len(pairs); i += 2 {
		s.keys = append(s.keys, pairs[i])
		s.files[pairs[i]] = pairs[i+1]
	}
	return s
}

func (s *memSource) Keys() []string { return slices.Clone(s.keys) }

func (s *memSource) Open(key string) (io.ReadCloser, error) {
	if s.onOpen != nil {
		s.onOpen(key)
	}
	if s.failing[key] {
		return nil, fmt.Errorf("cannot read %s", key)
	}
	content, ok := s.files[key]
	if !ok {
		return nil, archive.ErrNoFile
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (s *memSource) Size(key string) (int64, bool) {
	content, ok := s.files[key]
	return int64(len(content)), ok && !s.noSize
}

func (s *memSource) Prefetch(_ context.Context, keys []string) error {
	if s.failPrefetch {
		return errors.New("network down")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefetched = append(s.prefetched, keys...)
	return nil
}

// harness is one workspace with the installer under test.
type harness struct {
	t      *testing.T
	root   string
	assets map[string]archive.Source // by URL
	opened [][]string                // the URLs of every OpenAsset call
	calls  []hookCall
	fail   map[string]bool // commands that fail
	onRun  func(command string)
	rep    *recReporter
	inst   *Installer
	faults *faultyFS
}

type hookCall struct {
	Command, Dir string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root := filepath.Join(t.TempDir(), "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, root: root, assets: map[string]archive.Source{}, fail: map[string]bool{},
		rep: &recReporter{}}
	h.inst = &Installer{Reporter: h.rep, RunCommand: h.run}
	return h
}

func (h *harness) run(ctx context.Context, dir, command string, output io.Writer) error {
	h.calls = append(h.calls, hookCall{command, dir})
	if h.onRun != nil {
		h.onRun(command)
	}
	if h.fail[command] {
		code := 1
		return &CommandFailedError{Msg: "The command exited with code 1.", Command: command, Dir: dir,
			ExitCode: &code}
	}
	fmt.Fprintf(output, "ran %s\n", command)
	return nil
}

func (h *harness) full(rel string) string { return filepath.Join(h.root, filepath.FromSlash(rel)) }

func (h *harness) addFile(rel, content string) {
	h.t.Helper()
	p := h.full(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) addDir(rel string) {
	h.t.Helper()
	if err := os.MkdirAll(h.full(rel), 0o755); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) read(rel string) string {
	h.t.Helper()
	b, err := os.ReadFile(h.full(rel))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(b)
}

func (h *harness) fileExists(rel string) bool { return fileExists(h.full(rel)) }
func (h *harness) dirExists(rel string) bool  { return dirExists(h.full(rel)) }

// snapshot is every file (content and hidden attribute), directory and
// link below the root, except .lip and .lip/lock, whose creation is not a
// change (protocol/workspace.md §7).
func (h *harness) snapshot() map[string]string {
	h.t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(h.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(p, h.root))
		if rel == "" || rel == "/.lip" || rel == "/.lip/lock" {
			return nil
		}
		if target, err := os.Readlink(p); err == nil {
			// A symbolic link or a junction.
			snap[rel] = "<link to " + target + ">"
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case d.IsDir():
			snap[rel] = "<dir>"
		default:
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			hidden, _ := txn.OS.Hidden(p)
			snap[rel] = fmt.Sprintf("%s (hidden=%v)", b, hidden)
		}
		return nil
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return snap
}

func (h *harness) assertUnchanged(before map[string]string) {
	h.t.Helper()
	if after := h.snapshot(); !reflect.DeepEqual(before, after) {
		h.t.Errorf("workspace changed:\nbefore %v\nafter  %v", before, after)
	}
}

// pkg is a package of the given id (version 1.0.0) with the entry and the
// archive self.
func (h *harness) pkg(entry pkgmodel.Entry, self archive.Source) *Package {
	return h.pkgOf("example.com/pack", "1.0.0", entry, self)
}

func (h *harness) pkgOf(id, version string, entry pkgmodel.Entry, self archive.Source) *Package {
	if self == nil {
		self = src()
	}
	entry.Platform, entry.Runtime = pkgmodel.PlatformServer, "win-x64"
	return &Package{
		ID: id, Version: version, Entry: &entry, Archive: self,
		OpenAsset: func(urls []string) (archive.Source, error) {
			h.opened = append(h.opened, slices.Clone(urls))
			for _, u := range urls {
				if s, ok := h.assets[u]; ok {
					return s, nil
				}
			}
			return nil, &archive.UnavailableError{Message: "none of " + strings.Join(urls, ", ")}
		},
	}
}

const legacyMoneyURL = "https://example.com/LegacyMoney.mcmod"

func legacyMoneyFiles() *memSource {
	return src("manifest.json", `{"name":"LegacyMoney"}`, "LegacyMoney.dll", "dll",
		"lang/en_US.json", "{}")
}

// legacyMoney is the mod LegacyMoney (an asset of type "mcmod") with the
// given extra entry members.
func (h *harness) legacyMoney(extra pkgmodel.Entry, files *memSource, version string) *Package {
	if files == nil {
		files = legacyMoneyFiles()
	}
	if version == "" {
		version = "0.22.0"
	}
	h.assets[legacyMoneyURL] = files
	extra.Assets = append([]pkgmodel.Asset{{Type: pkgmodel.AssetMcmod, URLs: []string{legacyMoneyURL}}},
		extra.Assets...)
	return h.pkgOf("example.com/legacymoney", version, extra, nil)
}

func selfAsset(places ...pkgmodel.Placement) []pkgmodel.Asset {
	return []pkgmodel.Asset{{Type: pkgmodel.AssetSelf, Place: places}}
}

func place(src, dest string) pkgmodel.Placement { return pkgmodel.Placement{Src: src, Dest: dest} }

// locked is the workspace file entry of an installed package.
func locked(p *Package, files []string) pkgmodel.LockedPackage {
	m := doc.NewObject()
	m.Set("format", "4")
	m.Set("id", p.ID)
	m.Set("version", p.Version)
	return pkgmodel.LockedPackage{Manifest: m, Variant: p.Variant, Explicit: true, Ref: p.ID,
		Files: slices.Clone(files)}
}

func (h *harness) opts(installed ...pkgmodel.LockedPackage) Options {
	return Options{Root: h.root, Installed: installed}
}

// apply is SoloInstaller.Apply: applies plans through one transaction
// journaled in .lip, and commits it or rolls it back; leaves no .lip that
// was not there before.
func (h *harness) apply(ctx context.Context, layout *LayoutPlan, plans ...*Plan) error {
	h.t.Helper()
	lip := filepath.Join(h.root, ".lip")
	lipExisted := dirExists(lip)
	defer func() {
		if !lipExisted {
			if names, err := readDirNames(lip); err == nil && len(names) == 0 {
				os.Remove(lip)
			}
		}
	}()
	if err := txn.EnsureLipDir(h.root); err != nil {
		h.t.Fatal(err)
	}
	var reserved []string
	for _, p := range plans {
		reserved = append(reserved, p.WrittenPaths()...)
	}
	opts := txn.Options{Context: ctx, Reserved: reserved}
	if h.faults != nil {
		opts.FS = h.faults
	}
	tx, err := txn.Begin(h.root, opts)
	if err != nil {
		h.t.Fatal(err)
	}
	defer tx.Close()
	err = ApplyLayout(tx, layout)
	for _, p := range plans {
		if err != nil {
			break
		}
		err = h.inst.Apply(ctx, tx, p)
	}
	if err != nil {
		if undo := tx.Rollback(func() error { return nil }); len(undo) > 0 {
			h.t.Errorf("rollback errors: %v", undo)
		}
		return err
	}
	if _, err := tx.Commit(nil); err != nil {
		return err
	}
	return nil
}

// install is SoloInstaller.Install: returns the files the package owns.
func (h *harness) install(p *Package, opts Options) ([]string, error) {
	h.t.Helper()
	plan, err := h.inst.PlanInstall(context.Background(), NewView(nil), opts, p)
	if err != nil {
		return nil, err
	}
	if !opts.DryRun {
		if err := h.apply(context.Background(), nil, plan); err != nil {
			return nil, err
		}
	}
	return plan.OwnedFiles, nil
}

func (h *harness) mustInstall(p *Package, opts Options) []string {
	h.t.Helper()
	owned, err := h.install(p, opts)
	if err != nil {
		h.t.Fatalf("install: %v", err)
	}
	return owned
}

// uninstall is SoloInstaller.Uninstall.
func (h *harness) uninstall(p *pkgmodel.LockedPackage, entry *pkgmodel.Entry, opts Options) error {
	h.t.Helper()
	plan, err := h.inst.PlanUninstall(context.Background(), NewView(nil), opts, p, entry)
	if err != nil {
		return err
	}
	if opts.DryRun {
		return nil
	}
	return h.apply(context.Background(), nil, plan)
}

func (h *harness) mustUninstall(p *pkgmodel.LockedPackage, entry *pkgmodel.Entry, opts Options) {
	h.t.Helper()
	if err := h.uninstall(p, entry, opts); err != nil {
		h.t.Fatalf("uninstall: %v", err)
	}
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	sort.Slice(s, func(i, j int) bool { return compareOrdinal(s[i], s[j]) < 0 })
	return s
}

func wantInstallError(t *testing.T, err error, contains string) *Error {
	t.Helper()
	var ie *Error
	if !errors.As(err, &ie) {
		t.Fatalf("err = %v (%T), want *install.Error", err, err)
	}
	if !strings.Contains(ie.Msg, contains) {
		t.Errorf("message %q does not contain %q", ie.Msg, contains)
	}
	return ie
}

// zipBytes is WorkspaceHarness.Zip: a ZIP file holding the files, in order.
func zipBytes(t *testing.T, pairs ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i+1 < len(pairs); i += 2 {
		f, err := w.Create(pairs[i])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(pairs[i+1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// recReporter records messages and progress.
type recReporter struct {
	mu       sync.Mutex
	lines    []string
	progress []report.Progress
}

func (r *recReporter) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, s)
}

func (r *recReporter) Info(m string)                    { r.add("info: " + m) }
func (r *recReporter) Success(m string)                 { r.add("success: " + m) }
func (r *recReporter) Warning(m string)                 { r.add("warning: " + m) }
func (r *recReporter) Error(m string)                   { r.add("error: " + m) }
func (r *recReporter) Progress(string, string, float64) {}
func (r *recReporter) ReportProgress(p report.Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress = append(r.progress, p)
}

func (r *recReporter) infos(contains string) int {
	n := 0
	for _, l := range r.lines {
		if strings.HasPrefix(l, "info: ") && strings.Contains(l, contains) {
			n++
		}
	}
	return n
}

// faultyFS is FaultyFileSystem.cs over the real file system: Create and
// Rename fail on demand.
type faultyFS struct {
	txn.FS
	fail  func(op, path string) bool
	calls int
}

func newFaultyFS() *faultyFS { return &faultyFS{FS: txn.OS} }

// failNthCall fails the n-th (1-based) Create or Rename call from now on.
func (f *faultyFS) failNthCall(n int) {
	target := f.calls + n
	f.fail = func(string, string) bool { return f.calls == target }
}

func (f *faultyFS) check(op, path string) error {
	f.calls++
	if f.fail != nil && f.fail(op, path) {
		return fmt.Errorf("injected failure: %s %s", op, path)
	}
	return nil
}

func (f *faultyFS) Create(name string) (io.WriteCloser, error) {
	if err := f.check("create", name); err != nil {
		return nil, err
	}
	return f.FS.Create(name)
}

func (f *faultyFS) Rename(oldname, newname string) error {
	if err := f.check("move", oldname); err != nil {
		return err
	}
	return f.FS.Rename(oldname, newname)
}
