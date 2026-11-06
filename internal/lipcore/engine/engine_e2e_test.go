package engine

// Differential end-to-end test against the real lipd.exe (lip 0.37.0).
//
// Enabled with LIP_E2E=1. Environment:
//
//	LIPD_PATH          path to lipd.exe (default %APPDATA%\levilauncher.exe\bin\lip\lipd.exe)
//	LIP_E2E_PACKAGE    package id (default github.com/LiteLDev/LeviLamina#client)
//	LIP_E2E_FROM       version to install (default 26.51.8)
//	LIP_E2E_TO         version to update to (default 26.51.9)
//	LIP_E2E_REAL_ONLY  =1 runs only the lipd side (checks the RPC driver)
//
// Both sides run in fresh t.TempDir() workspaces; real instance
// directories are never touched. Scripts are skipped on both sides
// (ignoreScripts=true): LeviLamina's scripts patch Minecraft.Windows.exe,
// which a scratch workspace does not have. The download cache
// (%LOCALAPPDATA%\lip\cache) is shared with lip.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func e2eEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func lipdPath() string {
	if p := strings.TrimSpace(os.Getenv("LIPD_PATH")); p != "" {
		return p
	}
	return filepath.Join(os.Getenv("APPDATA"), "levilauncher.exe", "bin", "lip", "lipd.exe")
}

// lipdCall runs one JSON-RPC call against "lipd run" in dir. Callback
// requests (PrintInfo, ReportProgress, ...) are logged and, when they carry
// an id, answered with null.
func lipdCall(t *testing.T, dir, method string, params any) (json.RawMessage, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, lipdPath(), "run")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if root := filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "dotnet"); os.Getenv("DOTNET_ROOT") == "" {
		if _, err := os.Stat(root); err == nil {
			cmd.Env = append(cmd.Env, "DOTNET_ROOT="+root)
		}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() {
		_ = stdin.Close()
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}()

	r := bufio.NewReader(stdout)
	w := bufio.NewWriter(stdin)
	write := func(v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
		return w.Flush()
	}
	if err := write(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for {
		payload, err := readFramed(r)
		if err != nil {
			return nil, fmt.Errorf("read: %w (stderr: %s)", err, stderr.String())
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		if msg.Method != "" {
			if msg.Method != "ReportProgress" {
				t.Logf("lipd %s %s", msg.Method, msg.Params)
			}
			if len(msg.ID) > 0 && string(msg.ID) != "null" {
				if err := write(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": nil}); err != nil {
					return nil, err
				}
			}
			continue
		}
		if string(msg.ID) != "1" {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("rpc %s failed: %s", method, msg.Error.Message)
		}
		return msg.Result, nil
	}
}

func readFramed(r *bufio.Reader) ([]byte, error) {
	n := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), "content-length") {
			n, err = strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, err
			}
		}
	}
	if n < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

// snapshot returns relative path -> SHA-256 of every file under dir except
// tooth_lock.json, plus the lock with dir replaced by a placeholder.
func snapshot(t *testing.T, dir string) (map[string]string, string) {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if rel == "tooth_lock.json" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		files[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := os.ReadFile(filepath.Join(dir, "tooth_lock.json"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	s := string(lock)
	escaped, _ := json.Marshal(dir)
	s = strings.ReplaceAll(s, strings.Trim(string(escaped), `"`), "<WORKSPACE>")
	s = strings.ReplaceAll(s, dir, "<WORKSPACE>")
	return files, s
}

func compareWorkspaces(t *testing.T, step, realDir, nativeDir string) {
	t.Helper()
	rf, rl := snapshot(t, realDir)
	nf, nl := snapshot(t, nativeDir)
	if !reflect.DeepEqual(rf, nf) {
		var diffs []string
		keys := map[string]bool{}
		for k := range rf {
			keys[k] = true
		}
		for k := range nf {
			keys[k] = true
		}
		var sorted []string
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			if rf[k] != nf[k] {
				diffs = append(diffs, fmt.Sprintf("%s: lipd=%.12s native=%.12s", k, rf[k], nf[k]))
			}
		}
		if len(diffs) > 20 {
			diffs = append(diffs[:20], fmt.Sprintf("... %d more", len(diffs)-20))
		}
		t.Errorf("%s: file trees differ:\n%s", step, strings.Join(diffs, "\n"))
	}
	if rl != nl {
		t.Errorf("%s: tooth_lock.json differs\n--- lipd ---\n%s\n--- native ---\n%s", step, rl, nl)
	}
}

type e2eLogger struct{ t *testing.T }

func (l e2eLogger) Info(m string)                    { l.t.Logf("native info: %s", m) }
func (l e2eLogger) Success(m string)                 { l.t.Logf("native success: %s", m) }
func (l e2eLogger) Warning(m string)                 { l.t.Logf("native warning: %s", m) }
func (l e2eLogger) Error(m string)                   { l.t.Logf("native error: %s", m) }
func (l e2eLogger) Progress(string, string, float64) {}

// runNative runs fn, turning a panic from a not yet implemented sibling
// package into a test failure.
func runNative(t *testing.T, step string, fn func() error) {
	t.Helper()
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		return fn()
	}()
	if err != nil {
		t.Fatalf("native %s: %v", step, err)
	}
}

func TestE2EDifferentialAgainstLipd(t *testing.T) {
	if os.Getenv("LIP_E2E") != "1" {
		t.Skip("set LIP_E2E=1 to run the differential test against lipd.exe")
	}
	if _, err := os.Stat(lipdPath()); err != nil {
		t.Skipf("lipd.exe not found: %v", err)
	}
	pkg := e2eEnv("LIP_E2E_PACKAGE", "github.com/LiteLDev/LeviLamina#client")
	from := e2eEnv("LIP_E2E_FROM", "26.51.8")
	to := e2eEnv("LIP_E2E_TO", "26.51.9")
	realOnly := os.Getenv("LIP_E2E_REAL_ONLY") == "1"

	realDir := t.TempDir()
	nativeDir := t.TempDir()

	var eng *Engine
	if !realOnly {
		runNative(t, "setup", func() error {
			var err error
			eng, err = New(nativeDir, "", e2eLogger{t})
			return err
		})
	}
	ctx := context.Background()

	type step struct {
		name   string
		method string
		params any
		native func() error
	}
	steps := []step{
		{"install", "Install", []any{[]string{pkg + "@" + from}, false, true, false},
			func() error { return eng.Install(ctx, []string{pkg + "@" + from}, false, true, false) }},
		{"update", "Update", []any{[]string{pkg + "@" + to}, false, true},
			func() error { return eng.Update(ctx, []string{pkg + "@" + to}, false, true) }},
		{"list", "List", []any{}, nil},
		{"uninstall", "Uninstall", []any{[]string{pkg}, false, true, false},
			func() error { return eng.Uninstall(ctx, []string{pkg}, false, true, false) }},
	}
	for _, s := range steps {
		res, err := lipdCall(t, realDir, s.method, s.params)
		if err != nil {
			t.Fatalf("lipd %s: %v", s.name, err)
		}
		t.Logf("lipd %s result: %s", s.name, res)
		if realOnly {
			continue
		}
		if s.method == "List" {
			// lipd serializes the tuple as {"Item1": [...], "Item2": [...]}.
			var tuple struct{ Item1, Item2 []string }
			if err := json.Unmarshal(res, &tuple); err != nil {
				t.Fatalf("decode List result %s: %v", res, err)
			}
			lists := [2][]string{tuple.Item1, tuple.Item2}
			var ex, im []string
			runNative(t, "list", func() error {
				var err error
				ex, im, err = eng.List(ctx)
				return err
			})
			if !reflect.DeepEqual(nonNil(lists[0]), nonNil(ex)) || !reflect.DeepEqual(nonNil(lists[1]), nonNil(im)) {
				t.Errorf("List differs: lipd=%q native=%q %q", lists, ex, im)
			}
			continue
		}
		runNative(t, s.name, s.native)
		compareWorkspaces(t, s.name, realDir, nativeDir)
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
