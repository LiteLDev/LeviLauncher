package lipdirs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Ported from tests/Lip.Core.Tests/Infrastructure/LipDirectoriesTests.cs.

func TestConfigDirIsLipUnderApplicationData(t *testing.T) {
	if runtime.GOOS == "windows" {
		want := filepath.Join(os.Getenv("APPDATA"), "lip")
		if os.Getenv("APPDATA") != "" && !strings.EqualFold(ConfigDir(), want) {
			t.Fatalf("ConfigDir() = %q, want %q", ConfigDir(), want)
		}
		return
	}
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	if got := ConfigDir(); got != "/xdg/config/lip" {
		t.Fatalf("ConfigDir() = %q", got)
	}
	home, _ := os.UserHomeDir()
	for _, v := range []string{"", "relative"} {
		t.Setenv("XDG_CONFIG_HOME", v)
		if got, want := ConfigDir(), filepath.Join(home, ".config", "lip"); got != want {
			t.Fatalf("XDG_CONFIG_HOME=%q: ConfigDir() = %q, want %q", v, got, want)
		}
	}
}

func TestCacheDirFollowsPlatformConvention(t *testing.T) {
	if runtime.GOOS == "windows" {
		want := filepath.Join(os.Getenv("LOCALAPPDATA"), "lip", "Cache")
		if os.Getenv("LOCALAPPDATA") != "" && !strings.EqualFold(CacheDir(), want) {
			t.Fatalf("CacheDir() = %q, want %q", CacheDir(), want)
		}
		if filepath.Base(CacheDir()) != "Cache" {
			t.Fatalf("CacheDir() = %q, want ...\\lip\\Cache", CacheDir())
		}
		return
	}
	t.Setenv("XDG_CACHE_HOME", "/xdg/cache")
	if got := CacheDir(); got != "/xdg/cache/lip" {
		t.Fatalf("CacheDir() = %q", got)
	}
	home, _ := os.UserHomeDir()
	for _, v := range []string{"", "relative/dir"} {
		t.Setenv("XDG_CACHE_HOME", v)
		if got, want := CacheDir(), filepath.Join(home, ".cache", "lip"); got != want {
			t.Fatalf("XDG_CACHE_HOME=%q: CacheDir() = %q, want %q", v, got, want)
		}
	}
}

func TestDirectoriesNoneUnderSystemTemp(t *testing.T) {
	temp := strings.ToLower(os.TempDir())
	for _, d := range []string{ConfigDir(), CacheDir()} {
		if strings.HasPrefix(strings.ToLower(d), temp) {
			t.Errorf("%q is under the system temp directory %q", d, temp)
		}
	}
}
