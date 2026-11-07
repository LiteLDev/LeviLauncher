package lip

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// e2eLeviLamina is a published LeviLamina release, requested as the
// launcher does: the bare repository, whose tooth.json "client" label is the
// variant "" on the client platform. LIP_E2E_PACKAGE overrides it.
var e2eLeviLamina = func() string {
	if pkg := os.Getenv("LIP_E2E_PACKAGE"); pkg != "" {
		return pkg
	}
	return "github.com/LiteLDev/LeviLamina@26.51.9"
}()

// TestE2EInstallLeviLamina installs LeviLamina into a temporary instance
// directory with the real engine. It needs network access and runs only with
// LIP_E2E=1. LeviLamina's install scripts run PeEditor on the instance's
// Minecraft.Windows.exe, so LIP_E2E_GAME_EXE must name one; it is copied into
// the temporary instance and never modified in place.
func TestE2EInstallLeviLamina(t *testing.T) {
	if os.Getenv("LIP_E2E") != "1" {
		t.Skip("set LIP_E2E=1 to run the native lip end-to-end test")
	}
	gameExe := os.Getenv("LIP_E2E_GAME_EXE")
	if gameExe == "" {
		t.Skip("set LIP_E2E_GAME_EXE to a Minecraft.Windows.exe; LeviLamina's install scripts patch it")
	}
	recordEvents(t)

	dir := t.TempDir()
	copyFile(t, gameExe, filepath.Join(dir, "Minecraft.Windows.exe"))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if err := InstallPackages(ctx, dir, []string{e2eLeviLamina}); err != nil {
		t.Fatalf("install %s: %v", e2eLeviLamina, err)
	}
	explicit, err := IsPackageExplicitlyInstalled(ctx, dir, e2eLeviLamina)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !explicit {
		states, _ := ListPackageStates(ctx, dir)
		t.Fatalf("%s not reported as explicitly installed: %+v", e2eLeviLamina, states)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}
