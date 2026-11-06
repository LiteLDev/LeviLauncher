package lip

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// e2eLeviLamina is a published LeviLamina client release.
const e2eLeviLamina = "github.com/LiteLDev/LeviLamina#client@26.51.9"

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

	if err := InstallPackagesViaDaemon(ctx, dir, []string{e2eLeviLamina}); err != nil {
		t.Fatalf("install %s: %v", e2eLeviLamina, err)
	}
	explicit, err := IsPackageExplicitlyInstalledViaDaemon(ctx, dir, e2eLeviLamina)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !explicit {
		states, _ := ListPackageStatesViaDaemon(ctx, dir)
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
