//go:build darwin

package versionlaunch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInstanceIconPathPriority(t *testing.T) {
	dir := t.TempDir()
	if got := resolveInstanceIconPath(dir); got != "" {
		t.Fatalf("expected empty path for empty dir, got %q", got)
	}
	if got := resolveInstanceIconPath(""); got != "" {
		t.Fatalf("expected empty path for empty input, got %q", got)
	}

	writeFile := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// Empty file should be ignored.
	writeFile("LargeLogo.png", nil)
	ico := writeFile("minecraftIcon.ico", []byte("ico"))
	if got := resolveInstanceIconPath(dir); got != ico {
		t.Fatalf("expected fallback to minecraftIcon.ico %q, got %q", ico, got)
	}

	store := writeFile("StoreLogo.png", []byte("store"))
	if got := resolveInstanceIconPath(dir); got != store {
		t.Fatalf("expected StoreLogo.png %q, got %q", store, got)
	}

	logo := writeFile("Logo.png", []byte("logo"))
	if got := resolveInstanceIconPath(dir); got != logo {
		t.Fatalf("expected Logo.png %q, got %q", logo, got)
	}

	large := writeFile("LargeLogo.png", []byte("large"))
	if got := resolveInstanceIconPath(dir); got != large {
		t.Fatalf("expected LargeLogo.png %q, got %q", large, got)
	}
}
