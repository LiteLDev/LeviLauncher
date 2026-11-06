package fetch

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestEscapeKey(t *testing.T) {
	// Expected values produced by .NET 10 Uri.EscapeDataString.
	cases := map[string]string{
		"https://github.com/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip": "https%3A%2F%2Fgithub.com%2FLiteLDev%2FLeviLamina%2Freleases%2Fdownload%2Fv1.0.0%2Fx.zip",
		"git-blob:abc":    "git-blob%3Aabc",
		"a b+c/~_-.!*'()": "a%20b%2Bc%2F~_-.%21%2A%27%28%29",
		"ü€😀":             "%C3%BC%E2%82%AC%F0%9F%98%80",
		"%41":             "%2541",
		"":                "",
		"AZaz09":          "AZaz09",
		"\x00\x7f":        "%00%7F",
		"bad\xffutf8":     "bad%EF%BF%BDutf8",
		"git+https://github.com/LiteLDev/LeviLamina.git#v26.51.7": "git%2Bhttps%3A%2F%2Fgithub.com%2FLiteLDev%2FLeviLamina.git%23v26.51.7",
	}
	for in, want := range cases {
		if got := EscapeKey(in); got != want {
			t.Errorf("EscapeKey(%q) = %q, want %q", in, got, want)
		}
	}
	if got := EscapeKey(BlobKey("0123456789abcdef0123456789abcdef01234567")); got != "git-blob%3A0123456789abcdef0123456789abcdef01234567" {
		t.Errorf("blob key escaped as %q", got)
	}
}

// TestEscapeKeyMatchesLipCache checks that EscapeKey reproduces the names
// of the entries real lip installations created on this machine.
func TestEscapeKeyMatchesLipCache(t *testing.T) {
	root := DefaultCache().Root
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("no lip cache at %s: %v", root, err)
	}
	checked := map[string]int{}
	for _, e := range entries {
		name := e.Name()
		key, err := url.PathUnescape(name)
		if err != nil {
			t.Errorf("cache entry %q is not percent-encoded: %v", name, err)
			continue
		}
		if got := EscapeKey(key); got != name {
			t.Errorf("EscapeKey(%q) = %q, want existing name %q", key, got, name)
		}
		prefix := key
		if i := strings.IndexAny(key, ":-"); i >= 0 {
			prefix = key[:i]
		}
		checked[prefix]++
	}
	t.Logf("verified %d cache entry names in %s by key prefix: %v", len(entries), root, checked)
	if p := DefaultCache().Path(BlobKey("abc")); filepath.Base(p) != "git-blob%3Aabc" {
		t.Errorf("Path = %q", p)
	}
}

func TestDefaultCache(t *testing.T) {
	c := DefaultCache()
	if filepath.Base(c.Root) != "cache" || filepath.Base(filepath.Dir(c.Root)) != "lip" {
		t.Errorf("Root = %q", c.Root)
	}
	if filepath.Dir(c.TempRoot) != filepath.Dir(c.Root) || filepath.Base(c.TempRoot) != "temp" {
		t.Errorf("TempRoot = %q", c.TempRoot)
	}
	if runtime.GOOS == "windows" {
		if want := os.Getenv("LOCALAPPDATA"); want != "" && !strings.EqualFold(filepath.Dir(filepath.Dir(c.Root)), want) {
			t.Errorf("Root = %q, want under %q", c.Root, want)
		}
	}
}

func testCache(t *testing.T) *Cache {
	d := t.TempDir()
	return &Cache{Root: filepath.Join(d, "lip", "cache"), TempRoot: filepath.Join(d, "lip", "temp")}
}

func TestCachePutGet(t *testing.T) {
	c := testCache(t)
	key := "https://example.com/a b.zip"
	if _, ok := c.Get(key); ok {
		t.Fatal("unexpected hit")
	}
	p, err := c.Put(key, func(w io.Writer) error { _, err := io.WriteString(w, "hello"); return err })
	if err != nil {
		t.Fatal(err)
	}
	if p != c.Path(key) || filepath.Base(p) != "https%3A%2F%2Fexample.com%2Fa%20b.zip" {
		t.Errorf("path = %q", p)
	}
	if got, ok := c.Get(key); !ok || got != p {
		t.Errorf("Get = %q, %v", got, ok)
	}
	if b, _ := os.ReadFile(p); string(b) != "hello" {
		t.Errorf("content = %q", b)
	}
	// Existing entries are never replaced.
	called := false
	if _, err := c.Put(key, func(w io.Writer) error { called = true; return nil }); err != nil || called {
		t.Errorf("Put on existing key: err=%v called=%v", err, called)
	}
	// Temp files are cleaned up.
	if left, _ := os.ReadDir(c.TempRoot); len(left) != 0 {
		t.Errorf("temp dir not empty: %v", left)
	}
}

func TestCacheGetOrCreateLogs(t *testing.T) {
	c := testCache(t)
	var logs []string
	c.Info = func(m string) { logs = append(logs, m) }
	for i := 0; i < 2; i++ {
		if _, err := c.GetOrCreate("k", func(w io.Writer) error { _, err := w.Write([]byte("x")); return err }); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"Caching file for key 'k'...", "Cache hit for key 'k'."}
	if fmt.Sprint(logs) != fmt.Sprint(want) {
		t.Errorf("logs = %q", logs)
	}
}

func TestCachePutFailureLeavesNothing(t *testing.T) {
	c := testCache(t)
	boom := errors.New("boom")
	_, err := c.Put("k", func(w io.Writer) error {
		io.WriteString(w, "partial")
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, ok := c.Get("k"); ok {
		t.Error("entry exists after failed write")
	}
	if left, _ := os.ReadDir(c.TempRoot); len(left) != 0 {
		t.Errorf("temp dir not empty: %v", left)
	}
}

func TestCacheConcurrentPut(t *testing.T) {
	c := testCache(t)
	payload := func(i int) []byte { return bytes.Repeat([]byte{byte('a' + i%26)}, 256*1024) }
	var wg sync.WaitGroup
	paths := make([]string, 16)
	errs := make([]error, 16)
	for i := range paths {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			paths[i], errs[i] = c.Put("same", func(w io.Writer) error {
				_, err := w.Write(payload(i))
				return err
			})
		}(i)
	}
	wg.Wait()
	for i := range paths {
		if errs[i] != nil {
			t.Fatalf("put %d: %v", i, errs[i])
		}
		if paths[i] != c.Path("same") {
			t.Errorf("path %d = %q", i, paths[i])
		}
	}
	b, err := os.ReadFile(c.Path("same"))
	if err != nil {
		t.Fatal(err)
	}
	ok := false
	for i := range paths {
		if bytes.Equal(b, payload(i)) {
			ok = true
		}
	}
	if !ok {
		t.Error("cache entry is not one of the written payloads")
	}
	if left, _ := os.ReadDir(c.TempRoot); len(left) != 0 {
		t.Errorf("temp dir not empty: %v", left)
	}
}

func TestCacheClean(t *testing.T) {
	c := testCache(t)
	if err := c.Clean(); err != nil {
		t.Fatalf("Clean on missing dir: %v", err)
	}
	if _, err := c.Put("k", func(w io.Writer) error { return nil }); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(c.Root, "dir%2F", "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	ro := filepath.Join(sub, "ro.txt")
	if err := os.WriteFile(ro, []byte("x"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(c.Path("k"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := c.Clean(); err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if _, err := os.Stat(c.Root); !os.IsNotExist(err) {
		t.Errorf("cache dir still exists: %v", err)
	}
	if _, err := os.Stat(c.TempRoot); err != nil {
		t.Errorf("temp dir removed: %v", err)
	}
}
