package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestCanonicalWindowsPath(t *testing.T) {
	root := t.TempDir()
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
}
