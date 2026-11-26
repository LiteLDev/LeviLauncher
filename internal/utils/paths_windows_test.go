package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestNormalizeWindowsPath(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"", ""},
		{" \t ", ""},
		{` C:\Games\Version\ `, `c:\games\version`},
		{`C:/Games/Version/../Minecraft.Windows.exe`, `c:\games\minecraft.windows.exe`},
		{`\\?\C:\Games\Minecraft.Windows.exe`, `c:\games\minecraft.windows.exe`},
		{`\??\C:\Games\Minecraft.Windows.exe`, `c:\games\minecraft.windows.exe`},
		{`\\Server\Share\Version`, `\\server\share\version`},
		{`\\?\UNC\Server\Share\Version`, `\\server\share\version`},
		{`\??\UNC\Server\Share\Version`, `\\server\share\version`},
		{`unc\relative`, `unc\relative`},
	} {
		t.Run(tc.input, func(t *testing.T) {
			if got := NormalizeWindowsPath(tc.input); got != tc.want {
				t.Errorf("NormalizeWindowsPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// longPathName is path with its 8.3 short names spelled out; the temporary
// directory has them on some machines (C:\Users\RUNNER~1 on GitHub's).
func longPathName(t *testing.T, path string) string {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetLongPathName(p, &buf[0], uint32(len(buf)))
		if err != nil {
			t.Fatal(err)
		}
		if int(n) < len(buf) {
			return windows.UTF16ToString(buf[:n])
		}
		buf = make([]uint16, n)
	}
}

// shortPathName is path with 8.3 short names where the volume has them.
func shortPathName(t *testing.T, path string) string {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || int(n) >= len(buf) {
		return path
	}
	return windows.UTF16ToString(buf[:n])
}

func TestCanonicalWindowsPath(t *testing.T) {
	root := longPathName(t, t.TempDir())
	longDir := filepath.Join(root, strings.Repeat("long path ", 12)+"folder", strings.Repeat("nested path ", 12)+"folder")
	if err := os.MkdirAll(longDir, 0755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(longDir, "Minecraft.Windows.exe")
	if err := os.WriteFile(exe, nil, 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", "  ", root, longDir, exe, filepath.Join(root, "missing")} {
		if got, want := CanonicalWindowsPath(path), NormalizeWindowsPath(path); got != want {
			t.Errorf("CanonicalWindowsPath(%q) = %q, want %q", path, got, want)
		}
	}
	// A path with short names is the same file as its long spelling.
	short := filepath.Join(shortPathName(t, filepath.Join(root, strings.Repeat("long path ", 12)+"folder")),
		strings.Repeat("nested path ", 12)+"folder")
	if got, want := CanonicalWindowsPath(short), NormalizeWindowsPath(longDir); got != want {
		t.Errorf("CanonicalWindowsPath(%q) = %q, want %q", short, got, want)
	}
}
