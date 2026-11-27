// Package gameshell bridges folder-opening and native file/folder picker dialogs
// from Windows mods running under Wine to macOS Finder and AppKit panels over
// an authenticated loopback HTTP server.
package gameshell

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Filter struct {
	Name     string   `json:"name"`
	Patterns []string `json:"patterns"`
}

type DialogRequest struct {
	Kind        string   `json:"kind"` // "open", "save", "folder"
	Title       string   `json:"title,omitempty"`
	InitialDir  string   `json:"initialDir,omitempty"`
	WindowsDir  string   `json:"windowsInitialDir,omitempty"`
	DefaultName string   `json:"defaultName,omitempty"`
	DefaultExt  string   `json:"defaultExt,omitempty"`
	Multi       bool     `json:"multi,omitempty"`
	Filters     []Filter `json:"filters,omitempty"`
}

type DialogResult struct {
	Cancelled bool     `json:"cancelled"`
	Paths     []string `json:"paths"`
}

type Handlers struct {
	OpenDir     func(dir string) error
	RevealPath  func(path string) error
	ShowDialog  func(req DialogRequest) (DialogResult, error)
	FocusDialog func()
}

type Broker struct {
	mu       sync.Mutex
	dialogMu sync.Mutex
	sessions map[string]*Session
	server   *http.Server
	address  string
	handlers Handlers
}

type Session struct {
	broker   *Broker
	id       string
	token    string
	instance string
	prefix   string
}

var (
	defaultMu     sync.RWMutex
	defaultBroker *Broker
)

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func Start(handlers Handlers) (*Broker, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := &Broker{
		sessions: make(map[string]*Session),
		address:  ln.Addr().String(),
		handlers: handlers,
	}
	// Interactive native dialogs can stay open while the user browses Finder,
	// so do not impose a short WriteTimeout on this loopback server.
	b.server = &http.Server{
		Handler:           b,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		MaxHeaderBytes:    4096,
	}
	defaultMu.Lock()
	defaultBroker = b
	defaultMu.Unlock()
	log.Printf("[gameshell] listening on %s", b.address)
	go func() { _ = b.server.Serve(ln) }()
	return b, nil
}

func (b *Broker) Close() {
	if b == nil {
		return
	}
	defaultMu.Lock()
	if defaultBroker == b {
		defaultBroker = nil
	}
	defaultMu.Unlock()
	_ = b.server.Close()
	b.mu.Lock()
	b.sessions = make(map[string]*Session)
	b.mu.Unlock()
}

func Register(instance, prefix string) *Session {
	defaultMu.RLock()
	b := defaultBroker
	defaultMu.RUnlock()
	if b == nil {
		return nil
	}
	s := &Session{
		broker:   b,
		id:       randomHex(16),
		token:    randomHex(24),
		instance: instance,
		prefix:   prefix,
	}
	b.mu.Lock()
	b.sessions[s.id] = s
	b.mu.Unlock()
	log.Printf("[gameshell] session registered instance=%q id=%s", instance, s.id[:8])
	return s
}

func (s *Session) Environment() map[string]string {
	if s == nil || s.broker == nil {
		return nil
	}
	return map[string]string{
		"LEVI_SHELL_BRIDGE_URL":   "http://" + s.broker.address + "/v1/shell/" + s.id,
		"LEVI_SHELL_BRIDGE_TOKEN": s.token,
	}
}

func (s *Session) Close() {
	if s == nil || s.broker == nil {
		return
	}
	s.broker.mu.Lock()
	delete(s.broker.sessions, s.id)
	s.broker.mu.Unlock()
	log.Printf("[gameshell] session closed instance=%q id=%s", s.instance, s.id[:8])
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.Host != b.address {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/v1/shell/")
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	id, action, ok := strings.Cut(rest, "/")
	if !ok || id == "" || (action != "open" && action != "dialog") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	b.mu.Lock()
	s := b.sessions[id]
	b.mu.Unlock()
	if s == nil || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.token)) != 1 {
		log.Printf("[gameshell] unauthorized action=%s id=%s active=%t", action, id, s != nil)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "invalid content type", http.StatusUnsupportedMediaType)
		return
	}

	switch action {
	case "open":
		b.handleOpen(w, r, s)
	case "dialog":
		b.handleDialog(w, r, s)
	}
}

func (b *Broker) handleOpen(w http.ResponseWriter, r *http.Request, s *Session) {
	var req struct {
		Path        string `json:"path"`
		WindowsPath string `json:"windowsPath"`
		Reveal      bool   `json:"reveal"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil || dec.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	target, err := resolveHostPath(req.Path, req.WindowsPath, s.prefix)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		http.Error(w, "path does not exist", http.StatusBadRequest)
		return
	}

	if req.Reveal {
		if b.handlers.RevealPath == nil {
			http.Error(w, "reveal handler unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := b.handlers.RevealPath(target); err != nil {
			log.Printf("[gameshell] reveal failed path=%q err=%v", target, err)
			http.Error(w, "failed to reveal path", http.StatusInternalServerError)
			return
		}
		log.Printf("[gameshell] revealed path=%q", target)
	} else {
		if !info.IsDir() {
			http.Error(w, "target must be a directory", http.StatusBadRequest)
			return
		}
		if b.handlers.OpenDir == nil {
			http.Error(w, "open handler unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := b.handlers.OpenDir(target); err != nil {
			log.Printf("[gameshell] open dir failed path=%q err=%v", target, err)
			http.Error(w, "failed to open directory", http.StatusInternalServerError)
			return
		}
		log.Printf("[gameshell] opened dir=%q", target)
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"ok":true}`)
}

func (b *Broker) handleDialog(w http.ResponseWriter, r *http.Request, s *Session) {
	var req DialogRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024))
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil || dec.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Kind != "open" && req.Kind != "save" && req.Kind != "folder" {
		http.Error(w, "invalid dialog kind", http.StatusBadRequest)
		return
	}
	if strings.ContainsRune(req.Title, 0) || strings.ContainsRune(req.DefaultName, 0) || strings.ContainsRune(req.DefaultExt, 0) {
		http.Error(w, "invalid dialog parameters", http.StatusBadRequest)
		return
	}
	if len(req.Filters) > 64 {
		req.Filters = req.Filters[:64]
	}

	if req.InitialDir != "" || req.WindowsDir != "" {
		if resolved, err := resolveHostPath(req.InitialDir, req.WindowsDir, s.prefix); err == nil {
			if info, statErr := os.Stat(resolved); statErr == nil && info.IsDir() {
				req.InitialDir = resolved
			} else if parent := filepath.Dir(resolved); parent != "" {
				if pinfo, pErr := os.Stat(parent); pErr == nil && pinfo.IsDir() {
					req.InitialDir = parent
				} else {
					req.InitialDir = ""
				}
			} else {
				req.InitialDir = ""
			}
		} else {
			req.InitialDir = ""
		}
	}
	req.WindowsDir = ""
	req.DefaultExt = strings.TrimPrefix(strings.TrimSpace(req.DefaultExt), ".")

	if b.handlers.ShowDialog == nil {
		http.Error(w, "dialog handler unavailable", http.StatusServiceUnavailable)
		return
	}

	if !b.dialogMu.TryLock() {
		log.Printf("[gameshell] dialog already active; focusing existing panel and cancelling duplicate kind=%s", req.Kind)
		if b.handlers.FocusDialog != nil {
			b.handlers.FocusDialog()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"cancelled":true,"paths":[]}`)
		return
	}
	defer b.dialogMu.Unlock()

	log.Printf("[gameshell] dialog open kind=%s dir=%q multi=%t filters=%d", req.Kind, req.InitialDir, req.Multi, len(req.Filters))
	res, err := b.handlers.ShowDialog(req)
	if err != nil {
		log.Printf("[gameshell] dialog failed kind=%s err=%v", req.Kind, err)
		http.Error(w, "dialog failed", http.StatusInternalServerError)
		return
	}
	log.Printf("[gameshell] dialog done kind=%s cancelled=%t count=%d", req.Kind, res.Cancelled, len(res.Paths))
	if res.Paths == nil {
		res.Paths = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func resolveHostPath(posixPath, windowsPath, prefix string) (string, error) {
	if p := strings.TrimSpace(posixPath); p != "" {
		if strings.ContainsRune(p, 0) {
			return "", errors.New("invalid path")
		}
		if !filepath.IsAbs(p) {
			return "", errors.New("path must be absolute")
		}
		return filepath.Clean(p), nil
	}
	return resolveWindowsPath(windowsPath, prefix)
}

func resolveWindowsPath(winPath, prefix string) (string, error) {
	w := strings.TrimSpace(winPath)
	if w == "" || strings.ContainsRune(w, 0) {
		return "", errors.New("empty path")
	}
	w = strings.TrimPrefix(w, `\\?\`)
	if len(w) < 2 || w[1] != ':' {
		return "", errors.New("unsupported Windows path")
	}
	drive := strings.ToLower(w[:1])
	if drive[0] < 'a' || drive[0] > 'z' {
		return "", errors.New("invalid drive letter")
	}
	rest := strings.ReplaceAll(w[2:], `\`, "/")
	rest = strings.TrimLeft(rest, "/")

	if prefix != "" {
		driveLink := filepath.Join(prefix, "dosdevices", drive+":")
		if mapped, err := filepath.EvalSymlinks(driveLink); err == nil && filepath.IsAbs(mapped) {
			return filepath.Clean(filepath.Join(mapped, filepath.FromSlash(rest))), nil
		}
		if drive == "c" {
			return filepath.Clean(filepath.Join(prefix, "drive_c", filepath.FromSlash(rest))), nil
		}
	}
	if drive == "z" {
		return filepath.Clean("/" + filepath.FromSlash(rest)), nil
	}
	return "", errors.New("cannot map Windows drive path")
}

// NormalizeDialogFilters extracts concrete file extensions from Windows filter
// patterns (e.g. "*.txt;*.json") and reports whether wildcard filters ("*.*")
// were included so macOS NSOpenPanel allows selecting other file types.
func NormalizeDialogFilters(filters []Filter) (extensions []string, allowOther bool) {
	seen := make(map[string]struct{})
	for _, f := range filters {
		for _, raw := range f.Patterns {
			for _, part := range strings.Split(raw, ";") {
				p := strings.TrimSpace(part)
				if p == "" {
					continue
				}
				if p == "*" || p == "*.*" {
					allowOther = true
					continue
				}
				ext := strings.TrimPrefix(p, "*.")
				ext = strings.TrimPrefix(ext, ".")
				ext = strings.TrimSpace(ext)
				if ext == "" || strings.ContainsAny(ext, "*?/") {
					allowOther = true
					continue
				}
				key := strings.ToLower(ext)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				extensions = append(extensions, ext)
			}
		}
	}
	return extensions, allowOther
}
