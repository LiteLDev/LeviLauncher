package fetch

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
}

// TestEscapeKeyMatchesLipCache checks that EscapeKey reproduces the names
// of the entries real lip installations created on this machine.
func TestEscapeKeyMatchesLipCache(t *testing.T) {
	root := DefaultHTTPCache(nil).Root
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
	if p := DefaultHTTPCache(nil).Path("git-blob:abc"); filepath.Base(p) != "git-blob%3Aabc" {
		t.Errorf("Path = %q", p)
	}
}

func TestCleanProgress(t *testing.T) {
	c := NewHTTPCache(filepath.Join(t.TempDir(), "Cache"), nil)
	var got [][2]int64
	record := func(done, total int64) { got = append(got, [2]int64{done, total}) }
	if err := c.CleanProgress(context.Background(), record); err != nil || len(got) != 0 {
		t.Fatalf("missing dir: %v %v", err, got)
	}
	sub := filepath.Join(c.Root, "d", "e")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"a", "b"} {
		if err := os.WriteFile(c.Path(k), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sub, "ro"), []byte("x"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	// root, a, b, d, d/e, d/e/ro
	const total = 6

	// Canceled: nothing is deleted after the cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	got = nil
	err := c.CleanProgress(ctx, func(done, total int64) {
		record(done, total)
		if done == 2 {
			cancel()
		}
	})
	if err != context.Canceled || len(got) != 3 || got[2] != [2]int64{2, total} {
		t.Fatalf("canceled: %v %v", err, got)
	}
	if _, err := os.Stat(c.Root); err != nil {
		t.Fatalf("root removed by a canceled clean: %v", err)
	}

	// The remaining entries are counted again.
	got = nil
	if err := c.CleanProgress(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if len(got) != total-2+1 || got[0] != [2]int64{0, total - 2} || got[len(got)-1] != [2]int64{total - 2, total - 2} {
		t.Fatalf("updates %v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i][0] != got[i-1][0]+1 {
			t.Fatalf("updates %v", got)
		}
	}
	if _, err := os.Stat(c.Root); !os.IsNotExist(err) {
		t.Errorf("cache dir still exists: %v", err)
	}
}
