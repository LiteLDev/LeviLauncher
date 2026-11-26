package apppath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDarwinDirsFollowLibraryLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", "")
	t.Setenv("LOCALAPPDATA", "")
	SetBaseRootOverride("")

	support := filepath.Join(home, "Library", "Application Support", AppDirName)
	for name, c := range map[string]struct{ got, want string }{
		"ConfigDir": {ConfigDir(), support},
		"LocalDir":  {LocalDir(), support},
		"BaseRoot":  {BaseRoot(), support},
		"LogsDir":   {LogsDir(), filepath.Join(home, "Library", "Logs", AppDirName)},
		"CacheDir":  {CacheDir(), filepath.Join(home, "Library", "Caches", AppDirName)},
	} {
		if c.got != c.want {
			t.Errorf("%s() = %q, want %q", name, c.got, c.want)
		}
	}

	if err := EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(LogsDir()); err != nil {
		t.Errorf("EnsureDirs did not create LogsDir: %v", err)
	}
	if _, err := os.Stat(WebView2Dir()); !os.IsNotExist(err) {
		t.Errorf("EnsureDirs created %q, which WKWebView does not use", WebView2Dir())
	}
	if _, err := os.Stat(filepath.Join(support, LogsDirName)); !os.IsNotExist(err) {
		t.Errorf("logs written under Application Support")
	}
}

func TestDarwinCacheDirIgnoresBaseRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", "")
	SetBaseRootOverride(filepath.Join(home, "Elsewhere"))
	t.Cleanup(func() { SetBaseRootOverride("") })

	if got, want := CacheDir(), filepath.Join(home, "Library", "Caches", AppDirName); got != want {
		t.Fatalf("CacheDir() = %q, want %q", got, want)
	}
}

func TestDarwinLocalAppDataOverrideKeepsEverythingInside(t *testing.T) {
	home := t.TempDir()
	local := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", local)
	SetBaseRootOverride("")

	root := filepath.Join(local, AppDirName)
	if got, want := LogsDir(), filepath.Join(root, LogsDirName); got != want {
		t.Errorf("LogsDir() = %q, want %q", got, want)
	}
	if got, want := CacheDir(), filepath.Join(root, CacheDirName); got != want {
		t.Errorf("CacheDir() = %q, want %q", got, want)
	}
	if err := EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "Logs")); !os.IsNotExist(err) {
		t.Errorf("an override wrote to ~/Library/Logs")
	}
}
