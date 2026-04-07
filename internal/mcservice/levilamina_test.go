package mcservice

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

func TestResolveSupportedLeviLaminaVersionsUsesExactGameVersionOnly(t *testing.T) {
	db := map[string][]string{
		"1.21.80.03": {"0.13.0", "0.13.1"},
	}

	if got := resolveSupportedLeviLaminaVersions(db, "1.21.80"); got != nil {
		t.Fatalf("expected legacy three-part version to miss, got %v", got)
	}
	if got := resolveSupportedLeviLaminaVersions(db, "1.21.80.04"); got != nil {
		t.Fatalf("expected unmatched four-part version to miss, got %v", got)
	}

	want := []string{"0.13.0", "0.13.1"}
	if got := resolveSupportedLeviLaminaVersions(db, "1.21.80.03"); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact match result: got %v want %v", got, want)
	}
}

func TestInstallLeviLaminaRejectsLegacyThreePartGameVersion(t *testing.T) {
	restoreLeviLaminaTestDeps(t)
	versionsDir := setupLeviLaminaTestVersionsDir(t)
	targetDir := filepath.Join(versionsDir, "Demo")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}

	leviLaminaLipIsInstalled = func() bool { return true }
	fetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{
			"1.21.80.03": {"0.13.0"},
		}, nil
	}

	if got := InstallLeviLamina(context.Background(), "1.21.80", "Demo", "0.13.0"); got != "ERR_LL_NOT_SUPPORTED" {
		t.Fatalf("unexpected error code: %q", got)
	}
}

func TestInstallLeviLaminaInstallsExactMatchedVersion(t *testing.T) {
	restoreLeviLaminaTestDeps(t)
	versionsDir := setupLeviLaminaTestVersionsDir(t)
	targetDir := filepath.Join(versionsDir, "Demo")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}

	var installedWorkDir string
	var installedPackages []string

	leviLaminaLipIsInstalled = func() bool { return true }
	fetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{
			"1.21.80.03": {"0.13.0", "0.13.1"},
		}, nil
	}
	leviLaminaIsPackageExplicitlyInstalledViaDaemon = func(ctx context.Context, workDir string, packageRef string) (bool, error) {
		if packageRef != "github.com/LiteLDev/LeviLamina#client" {
			t.Fatalf("unexpected package ref: %s", packageRef)
		}
		return false, nil
	}
	leviLaminaInstallPackagesViaDaemon = func(ctx context.Context, workDir string, packages []string) error {
		installedWorkDir = workDir
		installedPackages = append([]string{}, packages...)
		return nil
	}
	leviLaminaUpdatePackagesViaDaemon = func(ctx context.Context, workDir string, packages []string) error {
		t.Fatalf("update should not be used when package is not explicitly installed")
		return nil
	}

	if got := InstallLeviLamina(context.Background(), "1.21.80.03", "Demo", "0.13.1"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	if installedWorkDir != targetDir {
		t.Fatalf("unexpected work dir: got %q want %q", installedWorkDir, targetDir)
	}
	wantPackages := []string{"github.com/LiteLDev/LeviLamina#client@0.13.1"}
	if !reflect.DeepEqual(installedPackages, wantPackages) {
		t.Fatalf("unexpected install packages: got %v want %v", installedPackages, wantPackages)
	}
}

func TestInstanceBackupSupportsLeviLaminaRejectsLegacyThreePartVersion(t *testing.T) {
	restoreFetchLLDB := instanceBackupFetchLeviLaminaVersionDB
	instanceBackupFetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{
			"1.21.80.03": {"0.13.0"},
		}, nil
	}
	t.Cleanup(func() {
		instanceBackupFetchLeviLaminaVersionDB = restoreFetchLLDB
	})

	if supported, definitive := instanceBackupSupportsLeviLamina("1.21.80"); !definitive || supported {
		t.Fatalf("expected legacy three-part version to be unsupported definitively, got supported=%t definitive=%t", supported, definitive)
	}
	if supported, definitive := instanceBackupSupportsLeviLamina("1.21.80.03"); !definitive || !supported {
		t.Fatalf("expected exact four-part version to be supported, got supported=%t definitive=%t", supported, definitive)
	}
}

func TestInstanceBackupSupportsLeviLaminaReturnsNonDefinitiveOnFetchFailure(t *testing.T) {
	restoreFetchLLDB := instanceBackupFetchLeviLaminaVersionDB
	instanceBackupFetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() {
		instanceBackupFetchLeviLaminaVersionDB = restoreFetchLLDB
	})

	if supported, definitive := instanceBackupSupportsLeviLamina("1.21.80.03"); !supported || definitive {
		t.Fatalf("expected fetch failure to be non-definitive support, got supported=%t definitive=%t", supported, definitive)
	}
}

func restoreLeviLaminaTestDeps(t *testing.T) {
	t.Helper()
	originalFetch := fetchLeviLaminaVersionDB
	originalLipInstalled := leviLaminaLipIsInstalled
	originalExplicitState := leviLaminaIsPackageExplicitlyInstalledViaDaemon
	originalUpdate := leviLaminaUpdatePackagesViaDaemon
	originalInstall := leviLaminaInstallPackagesViaDaemon
	t.Cleanup(func() {
		fetchLeviLaminaVersionDB = originalFetch
		leviLaminaLipIsInstalled = originalLipInstalled
		leviLaminaIsPackageExplicitlyInstalledViaDaemon = originalExplicitState
		leviLaminaUpdatePackagesViaDaemon = originalUpdate
		leviLaminaInstallPackagesViaDaemon = originalInstall
		apppath.SetBaseRootOverride("")
	})
}

func setupLeviLaminaTestVersionsDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	apppath.SetBaseRootOverride(filepath.Join(root, "base"))
	versionsDir, err := apppath.VersionsDir()
	if err != nil {
		t.Fatalf("resolve versions dir: %v", err)
	}
	return versionsDir
}
