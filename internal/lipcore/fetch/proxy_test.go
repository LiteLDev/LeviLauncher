package fetch

import (
	"context"
	"errors"
	"testing"
)

func TestProxyRewrite(t *testing.T) {
	// Expected values produced by lip's Flurl 4.0.0 GetDownloadUrls logic.
	type c struct{ proxy, in, want string }
	cases := []c{
		{"https://github.bibk.top", "https://github.com/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip", "https://github.bibk.top/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip"},
		{"https://github.bibk.top/", "https://github.com/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip", "https://github.bibk.top/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip"},
		{"https://mirror.example/gh", "https://github.com/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip", "https://mirror.example/gh/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip"},
		{"https://mirror.example/gh/?a=1", "https://github.com/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip", "https://mirror.example/gh/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip?a=1"},
		{"https://github.bibk.top", "https://GitHub.com/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip", "https://github.bibk.top/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip"},
		{"https://github.bibk.top", "https://github.com/a/b/c/?q=1&r=a%20b#frag", "https://github.bibk.top/a/b/c?q=1&r=a%20b#frag"},
		{"https://mirror.example/gh/?a=1", "https://github.com/a/b/c/?q=1&r=a%20b#frag", "https://mirror.example/gh/a/b/c?q=1&r=a%20b#frag"},
		{"https://github.bibk.top", "https://github.com/a/b//c/", "https://github.bibk.top/a/b/c"},
		{"https://github.bibk.top", "https://github.com:443/a/b", "https://github.bibk.top/a/b"},
		{"https://github.bibk.top", "https://github.com/a%2Fb/c d/ü", "https://github.bibk.top/a%2Fb/c%20d/%C3%BC"},
		{"https://github.bibk.top", "https://github.com", "https://github.bibk.top"},
		{"https://github.bibk.top/", "https://github.com", "https://github.bibk.top/"},
		{"https://mirror.example/gh/?a=1", "https://github.com", "https://mirror.example/gh/?a=1"},
	}
	for _, tc := range cases {
		got, ok := NewProxyRewriter(tc.proxy).Rewrite(tc.in)
		if !ok || got != tc.want {
			t.Errorf("Rewrite(%q) via %q = %q, %v; want %q", tc.in, tc.proxy, got, ok, tc.want)
		}
	}
	for _, in := range []string{"https://api.github.com/a/b", "https://example.com/a", "https://raw.githubusercontent.com/a/b"} {
		if got, ok := NewProxyRewriter("https://p").Rewrite(in); ok || got != in {
			t.Errorf("Rewrite(%q) = %q, %v; want unchanged", in, got, ok)
		}
	}
	if got, ok := NewProxyRewriter("").Rewrite("https://github.com/a/b"); ok || got != "https://github.com/a/b" {
		t.Errorf("empty proxy rewrote: %q", got)
	}
	var nilP *ProxyRewriter
	if _, ok := nilP.Rewrite("https://github.com/a/b"); ok {
		t.Error("nil rewriter rewrote")
	}
	// A rewrite producing the same URL is not eligible.
	if _, ok := NewProxyRewriter("https://github.com").Rewrite("https://github.com/a/b"); ok {
		t.Error("identity rewrite reported as eligible")
	}
}

func TestProxyTryFallbackAndSkip(t *testing.T) {
	p := NewProxyRewriter("https://proxy.example")
	ctx := context.Background()
	var calls []string
	failProxy := func(u string) error {
		calls = append(calls, u)
		if u[:len("https://proxy.example")] == "https://proxy.example" {
			return errors.New("proxy down")
		}
		return nil
	}

	if err := p.Try(ctx, "https://github.com/LiteLDev/LeviLamina.git", failProxy); err != nil {
		t.Fatal(err)
	}
	want := []string{"https://proxy.example/LiteLDev/LeviLamina.git", "https://github.com/LiteLDev/LeviLamina.git"}
	if !equal(calls, want) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}

	// Same repository (case-insensitive, with or without .git, sub-URL): direct only.
	for _, u := range []string{"https://github.com/liteldev/levilamina", "https://github.com/LiteLDev/LeviLamina.git/info/refs"} {
		calls = nil
		if err := p.Try(ctx, u, failProxy); err != nil {
			t.Fatal(err)
		}
		if !equal(calls, []string{u}) {
			t.Errorf("calls for %s = %q, want direct only", u, calls)
		}
	}

	// Another repository still tries the proxy first.
	calls = nil
	okAll := func(u string) error { calls = append(calls, u); return nil }
	if err := p.Try(ctx, "https://github.com/LiteLDev/bds.git", okAll); err != nil {
		t.Fatal(err)
	}
	if !equal(calls, []string{"https://proxy.example/LiteLDev/bds.git"}) {
		t.Errorf("calls = %q", calls)
	}

	// Both failing returns both errors.
	errA, errB := errors.New("a"), errors.New("b")
	n := 0
	err := p.Try(ctx, "https://github.com/x/y.git", func(string) error {
		n++
		if n == 1 {
			return errA
		}
		return errB
	})
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Errorf("err = %v", err)
	}

	// Non-github URLs go direct.
	calls = nil
	if err := p.Try(ctx, "https://gitee.com/a/b.git", okAll); err != nil || !equal(calls, []string{"https://gitee.com/a/b.git"}) {
		t.Errorf("calls = %q, err = %v", calls, err)
	}
}

func TestProxyTryCanceled(t *testing.T) {
	p := NewProxyRewriter("https://proxy.example")
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	err := p.Try(ctx, "https://github.com/a/b.git", func(string) error {
		n++
		cancel()
		return context.Canceled
	})
	if !errors.Is(err, context.Canceled) || n != 1 {
		t.Errorf("err = %v, calls = %d", err, n)
	}
	// The repository was not marked as failing.
	var calls []string
	p.Try(context.Background(), "https://github.com/a/b.git", func(u string) error { calls = append(calls, u); return nil })
	if !equal(calls, []string{"https://proxy.example/a/b.git"}) {
		t.Errorf("calls = %q", calls)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
