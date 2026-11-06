package engine

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/engine/workspace"
	"github.com/liteldev/LeviLauncher/internal/lipcore/lockfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/solver"
)

// ---- versions and ranges ----

// fakeVersion is "X.Y.Z" with an optional "-pre" part.
type fakeVersion struct {
	nums [3]int
	pre  string
}

func parseFakeVersion(s string) (solver.Version, error) {
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("Invalid version: %s", s)
	}
	var v fakeVersion
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("Invalid version: %s", s)
		}
		v.nums[i] = n
	}
	v.pre = pre
	return v, nil
}

func (v fakeVersion) Compare(o solver.Version) int {
	w := o.(fakeVersion)
	for i := range v.nums {
		if v.nums[i] != w.nums[i] {
			if v.nums[i] < w.nums[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case v.pre == w.pre:
		return 0
	case v.pre == "":
		return 1
	case w.pre == "":
		return -1
	}
	return strings.Compare(v.pre, w.pre)
}

func (v fakeVersion) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.nums[0], v.nums[1], v.nums[2])
	if v.pre != "" {
		s += "-" + v.pre
	}
	return s
}

// fakeRange is "*", ">=X" or an exact "X".
type fakeRange struct {
	text string
	min  solver.Version
	any  bool
	gte  bool
}

func parseFakeRange(s string) (solver.Constraint, error) {
	if s == "*" {
		return fakeRange{text: s, any: true}, nil
	}
	if strings.HasPrefix(s, ">=") {
		v, err := parseFakeVersion(s[2:])
		if err != nil {
			return nil, err
		}
		return fakeRange{text: s, min: v, gte: true}, nil
	}
	v, err := parseFakeVersion(s)
	if err != nil {
		return nil, err
	}
	return fakeRange{text: s, min: v}, nil
}

func (r fakeRange) Contains(v solver.Version) bool {
	switch {
	case r.any:
		return true
	case r.gte:
		return v.Compare(r.min) >= 0
	}
	return v.Compare(r.min) == 0
}

func (r fakeRange) String() string { return r.text }

// fakeSolve picks the newest version satisfying every constraint seen so
// far (no backtracking) and returns dependencies first in discovery order.
func fakeSolve(ctx context.Context, reg solver.Registry, roots []solver.Requirement, logf solver.Logf) ([]solver.Selected, error) {
	var names []string
	for _, r := range roots {
		names = append(names, r.String())
	}
	logf("Resolving dependencies for: %s", strings.Join(names, ", "))
	constraints := map[string][]solver.Constraint{}
	var order []string
	selected := map[string]solver.Selected{}
	queue := append([]solver.Requirement(nil), roots...)
	for len(queue) > 0 {
		req := queue[0]
		queue = queue[1:]
		if req.Constraint == nil {
			return nil, fmt.Errorf("nil constraint for %s", req.ID)
		}
		if _, ok := constraints[req.ID]; !ok {
			order = append(order, req.ID)
		}
		constraints[req.ID] = append(constraints[req.ID], req.Constraint)
		vs, err := reg.Versions(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		var best solver.Version
		for _, v := range vs {
			ok := true
			for _, c := range constraints[req.ID] {
				ok = ok && c.Contains(v)
			}
			if ok {
				best = v
			}
		}
		if best == nil {
			return nil, errors.New("Cannot find a valid state to satisfy all dependencies.")
		}
		if cur, ok := selected[req.ID]; ok && cur.Version.Compare(best) == 0 {
			continue
		}
		deps, err := reg.Dependencies(ctx, req.ID, best)
		if err != nil {
			return nil, err
		}
		selected[req.ID] = solver.Selected{ID: req.ID, Version: best, Requirements: deps}
		queue = append(queue, deps...)
	}
	var out []solver.Selected
	done := map[string]bool{}
	var visit func(id string)
	visit = func(id string) {
		if done[id] {
			return
		}
		done[id] = true
		for _, d := range selected[id].Requirements {
			visit(d.ID)
		}
		out = append(out, selected[id])
	}
	for _, id := range order {
		visit(id)
	}
	return out, nil
}

// ---- ids ----

var fakeIDRe = regexp.MustCompile(`^([^#]+)(?:#([^#]*))?$`)
var fakeSpecRe = regexp.MustCompile(`^([^@]+)@([^@]+)$`)

func fakeParseID(s string) (manifest.PackageID, error) {
	m := fakeIDRe.FindStringSubmatch(s)
	if m == nil || strings.ContainsAny(m[1], "@ ") {
		return manifest.PackageID{}, fmt.Errorf("Invalid package id: %s", s)
	}
	return manifest.PackageID{Path: m[1], Variant: m[2]}, nil
}

func fakeParseSpec(s string) (manifest.PackageSpec, error) {
	m := fakeSpecRe.FindStringSubmatch(s)
	if m == nil {
		return manifest.PackageSpec{}, fmt.Errorf("Invalid package spec: %s", s)
	}
	id, err := fakeParseID(m[1])
	if err != nil {
		return manifest.PackageSpec{}, err
	}
	return manifest.PackageSpec{ID: id, Version: m[2]}, nil
}

func fakeResolveVariant(m *manifest.Manifest, label string) (*manifest.Variant, error) {
	for _, v := range m.Variants {
		if v.Label == label {
			out := v
			out.Platform = "test-rid"
			return &out, nil
		}
	}
	return nil, fmt.Errorf("No variant found for label '%s' and platform 'test-rid'.", label)
}

// ---- lock store ----

type fakeLock struct {
	mu    sync.Mutex
	data  map[string][]byte
	saves []lockfile.State
}

func (l *fakeLock) load(dir string) (*lockfile.State, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.data[dir]
	if !ok {
		return lockfile.New(), nil
	}
	s := &lockfile.State{}
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(s); err != nil {
		return lockfile.New(), err
	}
	return s, nil
}

func (l *fakeLock) save(dir string, s *lockfile.State) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(s); err != nil {
		return err
	}
	b := buf.Bytes()
	if l.data == nil {
		l.data = map[string][]byte{}
	}
	l.data[dir] = b
	var snap lockfile.State
	_ = gob.NewDecoder(bytes.NewReader(b)).Decode(&snap)
	l.saves = append(l.saves, snap)
	return nil
}

// ---- package ops ----

type opCall struct {
	op      string // "install" or "uninstall"
	variant string
	files   []string
}

type fakeOps struct {
	dir   string
	calls *[]opCall
	fail  error
}

func (o *fakeOps) InstallPackage(ctx context.Context, variant *manifest.Variant, open workspace.SourceOpener) ([]string, error) {
	if o.fail != nil {
		return nil, o.fail
	}
	var files []string
	for _, a := range variant.Assets {
		src, err := open(ctx, a)
		if err != nil {
			return nil, err
		}
		keys := src.Keys()
		if err := src.Prefetch(ctx, keys); err != nil {
			return nil, err
		}
		for _, k := range keys {
			rc, err := src.Open(k)
			if err != nil {
				return nil, err
			}
			rc.Close()
			files = append(files, filepath.Join(o.dir, filepath.FromSlash(k)))
		}
	}
	*o.calls = append(*o.calls, opCall{op: "install", variant: variant.Label, files: files})
	return files, nil
}

func (o *fakeOps) UninstallPackage(ctx context.Context, variant *manifest.Variant, files []string) error {
	*o.calls = append(*o.calls, opCall{op: "uninstall", variant: variant.Label, files: files})
	return nil
}

// ---- registry ----

type memSource struct {
	files map[string]string
	order []string
}

func (s *memSource) Keys() []string { return append([]string(nil), s.order...) }
func (s *memSource) Open(key string) (io.ReadCloser, error) {
	c, ok := s.files[key]
	if !ok {
		return nil, fmt.Errorf("Key not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader([]byte(c))), nil
}
func (s *memSource) Prefetch(context.Context, []string) error { return nil }

type fakePkg struct {
	m     *manifest.Manifest
	files []string
}

type fakeRegistry struct {
	pkgs      map[string]map[string]fakePkg // path -> version -> pkg
	opened    []string
	versionsN int
}

func newFakeRegistry() *fakeRegistry { return &fakeRegistry{pkgs: map[string]map[string]fakePkg{}} }

// add registers path@version whose variant label has deps ("id": "range").
func (r *fakeRegistry) add(path, version, label string, deps ...string) {
	var d manifest.Dependencies
	for i := 0; i+1 < len(deps); i += 2 {
		d = append(d, manifest.Dependency{ID: deps[i], Range: deps[i+1]})
	}
	name := path[strings.LastIndex(path, "/")+1:]
	files := []string{name + "/" + version + ".txt"}
	m := &manifest.Manifest{
		FormatVersion: manifest.FormatVersion,
		FormatUUID:    manifest.FormatUUID,
		Tooth:         path,
		Version:       version,
		Variants: []manifest.Variant{{
			Label:        label,
			Dependencies: d,
			Assets:       []manifest.Asset{{Type: manifest.AssetSelf}},
		}},
	}
	if r.pkgs[path] == nil {
		r.pkgs[path] = map[string]fakePkg{}
	}
	r.pkgs[path][version] = fakePkg{m: m, files: files}
}

func (r *fakeRegistry) Versions(ctx context.Context, id manifest.PackageID) ([]string, error) {
	r.versionsN++
	vs, ok := r.pkgs[id.Path]
	if !ok {
		return nil, fmt.Errorf("repository %s not found", id.Path)
	}
	var out []string
	for v := range vs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := parseFakeVersion(out[i])
		b, _ := parseFakeVersion(out[j])
		return a.Compare(b) < 0
	})
	return out, nil
}

func (r *fakeRegistry) Manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error) {
	p, ok := r.pkgs[spec.ID.Path][spec.Version]
	if !ok {
		return nil, fmt.Errorf("Package %s not found", spec)
	}
	return p.m, nil
}

func (r *fakeRegistry) OpenAsset(ctx context.Context, spec manifest.PackageSpec, asset manifest.Asset) (workspace.Source, error) {
	p, ok := r.pkgs[spec.ID.Path][spec.Version]
	if !ok {
		return nil, fmt.Errorf("Package %s not found", spec)
	}
	r.opened = append(r.opened, spec.String())
	src := &memSource{files: map[string]string{}}
	for _, f := range p.files {
		src.files[f] = spec.String()
		src.order = append(src.order, f)
	}
	return src, nil
}

// ---- reporter ----

type recReporter struct {
	mu   sync.Mutex
	msgs []string
}

func (r *recReporter) add(s string)     { r.mu.Lock(); r.msgs = append(r.msgs, s); r.mu.Unlock() }
func (r *recReporter) Info(m string)    { r.add("info: " + m) }
func (r *recReporter) Success(m string) { r.add("success: " + m) }
func (r *recReporter) Warning(m string) { r.add("warning: " + m) }
func (r *recReporter) Error(m string)   { r.add("error: " + m) }
func (r *recReporter) Progress(id, m string, p float64) {
	r.add(fmt.Sprintf("progress: %s %.0f", m, p))
}

func (r *recReporter) has(s string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.msgs {
		if m == s {
			return true
		}
	}
	return false
}

var _ report.Reporter = (*recReporter)(nil)

// ---- harness ----

type harness struct {
	e     *Engine
	reg   *fakeRegistry
	lock  *fakeLock
	calls []opCall
	rep   *recReporter
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{reg: newFakeRegistry(), lock: &fakeLock{}, rep: &recReporter{}}
	dir := t.TempDir()
	h.e = &Engine{
		Dir:      dir,
		Registry: h.reg,
		Reporter: h.rep,
		Hooks: Hooks{
			ParseVersion:   parseFakeVersion,
			ParseRange:     parseFakeRange,
			ExactRange:     func(v solver.Version) solver.Constraint { return fakeRange{text: v.String(), min: v} },
			ResolveVariant: fakeResolveVariant,
			LoadState:      h.lock.load,
			SaveState:      h.lock.save,
			Solve:          fakeSolve,
			NewPackageOps: func(dir string, ignoreScripts bool, r report.Reporter) PackageOps {
				return &fakeOps{dir: dir, calls: &h.calls}
			},
			ParseID:   fakeParseID,
			ParseSpec: fakeParseSpec,
		},
	}
	return h
}

func (h *harness) state(t *testing.T) *lockfile.State {
	t.Helper()
	s, err := h.lock.load(h.e.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// summary renders the lock as "spec[!]" (! = locked) in stored order.
func summary(s *lockfile.State) string {
	var parts []string
	for _, p := range s.Packages {
		x := p.Spec().String()
		if p.Locked {
			x += "!"
		}
		parts = append(parts, x)
	}
	return strings.Join(parts, " ")
}

func (h *harness) callSummary() string {
	var parts []string
	for _, c := range h.calls {
		parts = append(parts, c.op+":"+strings.Join(relFiles(h.e.Dir, c.files), ","))
	}
	return strings.Join(parts, " ")
}

func relFiles(dir string, files []string) []string {
	var out []string
	for _, f := range files {
		r, err := filepath.Rel(dir, f)
		if err != nil {
			r = f
		}
		out = append(out, filepath.ToSlash(r))
	}
	return out
}
