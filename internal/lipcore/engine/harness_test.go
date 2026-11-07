package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/install"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// A port of WorkspaceHarness.cs: a workspace with the whole pipeline (the
// real archive provider, resolver, solver, installer and workspace file
// store) over a fake git remote whose repositories are in memory, and a
// fake HTTP server for downloads.

const testRuntime = "win-x64"

type harness struct {
	t         *testing.T
	root      string
	cacheDir  string
	git       *fakeGit
	downloads map[string][]byte
	rec       *recorder
	e         *Engine
	// commands are the hook commands run; failing makes these fail.
	mu       sync.Mutex
	commands []string
	failing  map[string]bool
	platform string
	runtime  string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	base := t.TempDir()
	h := &harness{
		t:         t,
		root:      filepath.Join(base, "ws"),
		cacheDir:  filepath.Join(base, "cache"),
		git:       newFakeGit(),
		downloads: map[string][]byte{},
		rec:       &recorder{},
		failing:   map[string]bool{},
		platform:  pkgmodel.PlatformServer,
		runtime:   testRuntime,
	}
	if err := os.MkdirAll(h.root, 0o755); err != nil {
		t.Fatal(err)
	}
	h.restart()
	return h
}

// restart starts over with a new engine, as a new lip process does.
func (h *harness) restart() {
	rep := h.rec
	client := &http.Client{Transport: roundTripFunc(h.serve)}
	n := 0
	h.e = New(h.root, Options{Platform: h.platform, Runtime: h.runtime, HTTPClient: client}, rep)
	h.e.cache = fetch.NewHTTPCache(h.cacheDir, rep)
	h.e.git = h.git
	h.e.inst = &install.Installer{Reporter: rep, RunCommand: h.runCommand}
	h.e.newID = func() string {
		n++
		return fmt.Sprintf("bar-%d", n)
	}
}

// withRuntime restarts with another runtime override ("" for none).
func (h *harness) withRuntime(rid string) *Engine {
	h.runtime = rid
	h.restart()
	return h.e
}

func (h *harness) runCommand(_ context.Context, dir, command string, _ io.Writer) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.commands = append(h.commands, command)
	if h.failing[command] {
		code := 1
		return &install.CommandFailedError{Msg: "The command '" + command + "' failed.", Command: command,
			Dir: dir, ExitCode: &code}
	}
	return nil
}

func (h *harness) ran(command string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.commands {
		if c == command {
			n++
		}
	}
	return n
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func (h *harness) serve(r *http.Request) (*http.Response, error) {
	h.mu.Lock()
	content, ok := h.downloads[r.URL.String()]
	h.mu.Unlock()
	if !ok {
		return &http.Response{StatusCode: 404, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader("")),
			Request: r, Header: http.Header{}}, nil
	}
	return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(bytes.NewReader(content)),
		ContentLength: int64(len(content)), Request: r, Header: http.Header{}}, nil
}

func (h *harness) addDownload(url string, content []byte) {
	h.mu.Lock()
	h.downloads[url] = content
	h.mu.Unlock()
}

// path is the absolute path of a workspace-relative '/'-separated path.
func (h *harness) path(rel string) string { return filepath.Join(h.root, filepath.FromSlash(rel)) }

func (h *harness) addFile(rel, content string) {
	h.t.Helper()
	p := h.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) addZip(rel string, files ...string) string {
	h.t.Helper()
	p := h.path(rel)
	if err := os.WriteFile(p, zipOf(files...), 0o644); err != nil {
		h.t.Fatal(err)
	}
	return p
}

func (h *harness) exists(rel string) bool {
	info, err := os.Stat(h.path(rel))
	return err == nil && !info.IsDir()
}

func (h *harness) read(rel string) string {
	h.t.Helper()
	b, err := os.ReadFile(h.path(rel))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(b)
}

// lock is the workspace file; it fails the test when there is none.
func (h *harness) lock() *pkgmodel.LockFile {
	h.t.Helper()
	lf, ok, err := pkgfile.LoadWorkspace(h.root)
	if err != nil || !ok {
		h.t.Fatalf("no workspace file: %v", err)
	}
	return lf
}

func (h *harness) hasLock() bool {
	_, err := os.Stat(h.path(pkgmodel.WorkspaceFile))
	return err == nil
}

// snapshot is every file of the workspace with its content, and every
// directory, except .lip and its lock file.
func (h *harness) snapshot() map[string]string {
	h.t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(h.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(h.root, p)
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == ".lip" || rel == ".lip/lock" {
			return nil
		}
		if d.IsDir() {
			out[rel] = "<dir>"
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return out
}

func (h *harness) packages() []InstalledPackage {
	h.t.Helper()
	ps, err := h.e.Packages(context.Background())
	if err != nil {
		h.t.Fatal(err)
	}
	return ps
}

// summary is "<identity>@<version> explicit|dep" of every installed package.
func (h *harness) summary() []string {
	var out []string
	for _, p := range h.packages() {
		kind := "dep"
		if p.Explicit {
			kind = "explicit"
		}
		out = append(out, p.Identity+"@"+p.Version+" "+kind)
	}
	return out
}

func (h *harness) install(args ...string) error {
	return h.e.Install(context.Background(), args, false, false, false)
}

func (h *harness) uninstall(args ...string) error {
	return h.e.Uninstall(context.Background(), args, false, false, false)
}

func (h *harness) update(args ...string) error {
	return h.e.Update(context.Background(), args, false, false)
}

func (h *harness) must(err error) {
	h.t.Helper()
	if err != nil {
		h.t.Fatalf("%v\n%s", err, FormatError(err))
	}
}

// addTag adds (or moves) the tag v<version> of a remote repository to a new
// commit holding the files (path, content pairs).
func (h *harness) addTag(spec, version string, files ...string) string {
	return h.git.setRef(spec, "refs/tags/v"+version, files)
}

// lipYAML is a lip.yaml whose variant "" has one entry for (server,
// win-x64) with the given members (comma-separated JSON members).
func lipYAML(id, version, entry string) string {
	return lipYAMLFor(id, version, pkgmodel.PlatformServer, entry)
}

func lipYAMLFor(id, version, platform, entry string) string {
	if entry != "" {
		entry = ", " + entry
	}
	return fmt.Sprintf(`{ "format": 4, "id": %q, "version": %q, "variants": { "": [ { "platform": %q,`+
		` "runtime": %q%s } ] } }`, id, version, platform, testRuntime, entry)
}

// selfAsset is the member "assets" with one asset of type "self" placing
// src at dest.
func selfAsset(src, dest string) string {
	return fmt.Sprintf(`"assets": [ { "type": "self", "place": [ { "src": %q, "dest": %q } ] } ]`, src, dest)
}

// tooth is a tooth.json of format 3 with one variant (label) placing
// file.dll at dest, with the given dependencies (a JSON object).
func tooth(path, version, dest, deps, label string) string {
	if deps == "" {
		deps = "{}"
	}
	return fmt.Sprintf(`{
  "format_version": 3,
  "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
  "tooth": %q,
  "version": %q,
  "variants": [ { "label": %q, "dependencies": %s,
    "assets": [ { "type": "self", "placements": [ { "type": "file", "src": "file.dll", "dest": %q } ] } ] } ]
}`, path, version, label, deps, dest)
}

// mcmod is the ZIP file of an asset of type "mcmod": a mod manifest named
// name and the file <name>.dll.
func mcmod(name string) []byte {
	return zipOf("manifest.json", fmt.Sprintf(`{ "name": %q, "entry": "%s.dll", "type": "native" }`, name, name),
		name+".dll", name+" binary")
}

// zipOf is a ZIP file holding the files (path, content pairs).
func zipOf(files ...string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i+1 < len(files); i += 2 {
		f, err := w.Create(files[i])
		if err != nil {
			panic(err)
		}
		if _, err := f.Write([]byte(files[i+1])); err != nil {
			panic(err)
		}
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// fakeGit is a git remote whose tags and branches point to in-memory trees
// (WorkspaceHarness.FakeGitRemote).
type fakeGit struct {
	mu          sync.Mutex
	refs        map[string]map[string]string // repo URL -> ref name -> commit
	trees       map[string]*memSource        // commit -> tree
	n           int
	listedTrees []string
	listedRefs  []string
	// failRefs makes ListRefs of a repository fail.
	failRefs map[string]error
}

func newFakeGit() *fakeGit {
	return &fakeGit{refs: map[string]map[string]string{}, trees: map[string]*memSource{},
		failRefs: map[string]error{}}
}

func (g *fakeGit) setRef(spec, refName string, files []string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	repo := repoURL(spec)
	src := &memSource{files: map[string][]byte{}}
	for i := 0; i+1 < len(files); i += 2 {
		src.keys = append(src.keys, files[i])
		src.files[files[i]] = []byte(files[i+1])
	}
	sum := sha1.Sum([]byte(fmt.Sprintf("%s@%s#%d", repo, refName, g.n)))
	g.n++
	commit := hex.EncodeToString(sum[:])
	g.trees[commit] = src
	if g.refs[repo] == nil {
		g.refs[repo] = map[string]string{}
	}
	g.refs[repo][refName] = commit
	return commit
}

func (g *fakeGit) addBranch(spec, branch string, files ...string) string {
	return g.setRef(spec, "refs/heads/"+branch, files)
}

func (g *fakeGit) addTagName(spec, tag string, files ...string) string {
	return g.setRef(spec, "refs/tags/"+tag, files)
}

func (g *fakeGit) ListRefs(ctx context.Context, repo string, prefixes []string) ([]gitproto.RemoteRef, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.listedRefs = append(g.listedRefs, repo+" "+strings.Join(prefixes, " "))
	if err := g.failRefs[repo]; err != nil {
		return nil, err
	}
	var names []string
	for name := range g.refs[repo] {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []gitproto.RemoteRef
	for _, name := range names {
		for _, p := range prefixes {
			if strings.HasPrefix(name, p) {
				out = append(out, gitproto.RemoteRef{Name: name, ObjectID: g.refs[repo][name]})
				break
			}
		}
	}
	return out, nil
}

func (g *fakeGit) OpenTree(ctx context.Context, repo, commit, key string) (archive.Source, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.listedTrees = append(g.listedTrees, commit)
	src, ok := g.trees[commit]
	if !ok {
		return nil, fmt.Errorf("no commit %s", commit)
	}
	return src, nil
}

func (g *fakeGit) treesListed() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.listedTrees...)
}

// memSource is an in-memory archive.
type memSource struct {
	keys  []string
	files map[string][]byte
}

func (m *memSource) Keys() []string { return m.keys }

func (m *memSource) Open(key string) (io.ReadCloser, error) {
	b, ok := m.files[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", archive.ErrNoFile, key)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memSource) Size(key string) (int64, bool) {
	b, ok := m.files[key]
	return int64(len(b)), ok
}

// recorder records messages and progress.
type recorder struct {
	mu       sync.Mutex
	infos    []string
	warnings []string
	progress []report.Progress
}

func (r *recorder) Info(m string)                    { r.mu.Lock(); r.infos = append(r.infos, m); r.mu.Unlock() }
func (r *recorder) Success(string)                   {}
func (r *recorder) Warning(m string)                 { r.mu.Lock(); r.warnings = append(r.warnings, m); r.mu.Unlock() }
func (r *recorder) Error(string)                     {}
func (r *recorder) Progress(string, string, float64) {}
func (r *recorder) ReportProgress(p report.Progress) {
	r.mu.Lock()
	r.progress = append(r.progress, p)
	r.mu.Unlock()
}

func (r *recorder) hasInfo(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.infos {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}

func (r *recorder) hasWarning(substr string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.warnings {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}

func equal[T comparable](t *testing.T, got, want []T) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func equalMaps(t *testing.T, got, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	for k, v := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: unexpected %q", k, v)
		}
	}
}
