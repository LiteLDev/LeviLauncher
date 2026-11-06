package lip

import (
	"context"
	"os"
	"testing"
	"time"
)

// e2eLeviLamina is a published LeviLamina client release.
const e2eLeviLamina = "github.com/LiteLDev/LeviLamina#client@26.51.9"

// TestE2EInstallLeviLamina installs LeviLamina into a temporary instance
// directory with the real engine. It needs network access and runs only with
// LIP_E2E=1.
func TestE2EInstallLeviLamina(t *testing.T) {
	if os.Getenv("LIP_E2E") != "1" {
		t.Skip("set LIP_E2E=1 to run the native lip end-to-end test")
	}
	recordEvents(t)

	dir := t.TempDir()
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
