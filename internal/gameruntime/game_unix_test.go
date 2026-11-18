//go:build darwin

package gameruntime

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSikarugirGameUsesWrapperInsteadOfBareWine(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Wine app.app")
	wine := filepath.Join(app, "Contents/SharedSupport/wine/bin/wine")
	wrapper := filepath.Join(app, "Contents/MacOS/wineskinlauncher")
	prefix := filepath.Join(app, "Contents/SharedSupport/prefix")
	for _, path := range []string{wine, wrapper} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("unused"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(prefix, 0700); err != nil {
		t.Fatal(err)
	}
	p := Profile{App: app, Wine: wine, Prefix: prefix}
	game := filepath.Join(t.TempDir(), "中文 instance", "Minecraft.Windows.exe")
	cmd, detached, err := p.GameCommand(context.Background(), game, "-Editor", "true")
	if err != nil || !detached || cmd.Path != wrapper {
		t.Fatalf("wrapper launch: detached=%v command=%v error=%v", detached, cmd, err)
	}
	if !reflect.DeepEqual(cmd.Args, []string{wrapper, game, "-Editor", "true"}) {
		t.Fatalf("game path or arguments changed: %q", cmd.Args)
	}
	for _, entry := range cmd.Env {
		if strings.HasPrefix(entry, "WINELOADER=") || strings.HasPrefix(entry, "WINEDLLPATH=") || strings.HasPrefix(entry, "DYLD_") {
			t.Fatalf("bare engine environment passed to wrapper: %s", entry)
		}
	}
	p.Prefix = t.TempDir()
	if _, _, err := p.GameCommand(context.Background(), game); err == nil {
		t.Fatal("wrapper silently selected a different Wine prefix")
	}
}

func TestGenericWineGameKeepsGuestPathAndProcessMonitoring(t *testing.T) {
	p := Profile{Wine: "/usr/bin/wine", Prefix: t.TempDir()}
	cmd, detached, err := p.GameCommand(context.Background(), filepath.Join(p.Prefix, "drive_c/Games/Minecraft.Windows.exe"))
	if err != nil || detached || cmd.Path != p.Wine || cmd.Args[1] != `C:\Games\Minecraft.Windows.exe` {
		t.Fatalf("direct Wine launch changed: command=%v detached=%v error=%v", cmd, detached, err)
	}
}
