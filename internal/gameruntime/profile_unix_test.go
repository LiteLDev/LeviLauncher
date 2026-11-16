//go:build darwin || linux

package gameruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWinePathsRequireKnownMapping(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "drive_c"), 0700)
	os.MkdirAll(filepath.Join(root, "dosdevices"), 0700)
	p := Profile{Prefix: root}
	if v, e := p.WindowsPath(filepath.Join(root, "drive_c", "Games", "中文", "a.exe")); e != nil || v != `C:\Games\中文\a.exe` {
		t.Fatalf("C mapping: %s %v", v, e)
	}
	if _, e := p.WindowsPath("/tmp/game.exe"); e == nil {
		t.Fatal("missing Z mapping accepted")
	}
	os.Symlink("/", filepath.Join(root, "dosdevices", "z:"))
	if v, e := p.WindowsPath("/tmp/with spaces/game.exe"); e != nil || v != `Z:\tmp\with spaces\game.exe` {
		t.Fatalf("Z mapping: %s %v", v, e)
	}
}
func TestFreeWineEnvironmentDoesNotInheritCrossOver(t *testing.T) {
	t.Setenv("CX_ROOT", "/old/CrossOver")
	t.Setenv("WINEPREFIX", "/old/prefix")
	t.Setenv("WINEDLLPATH", "/old/CrossOver/lib")
	p := Profile{App: "/free/Minecraft Wine.app", Wine: "/free/bin/wine", Prefix: "/free/prefix"}
	for _, entry := range p.Environment() {
		if strings.Contains(entry, "/old/") {
			t.Fatalf("stale runtime environment: %s", entry)
		}
	}
}
