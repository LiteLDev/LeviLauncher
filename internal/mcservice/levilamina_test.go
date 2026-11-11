package mcservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/lipcore/engine"
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

	fetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{
			"1.21.80.03": {"0.13.0", "0.13.1"},
		}, nil
	}
	lipListPackageStates = func(ctx context.Context, workDir string) (map[string]lip.PackageInstallState, error) {
		return map[string]lip.PackageInstallState{}, nil
	}
	lipInstallPackages = func(ctx context.Context, workDir string, packages []string) error {
		installedWorkDir = workDir
		installedPackages = append([]string{}, packages...)
		return nil
	}
	lipUpdatePackages = func(ctx context.Context, workDir string, packages []string) error {
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

// An installed LeviLamina is updated and uninstalled under the identity
// the workspace records.
func TestLeviLaminaUsesMigratedClientIdentity(t *testing.T) {
	restoreLeviLaminaTestDeps(t)
	versionsDir := setupLeviLaminaTestVersionsDir(t)
	if err := os.MkdirAll(filepath.Join(versionsDir, "Demo"), 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	fetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{"1.21.80.03": {"0.13.0", "0.13.1"}}, nil
	}
	ws := &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/levilamina": migratedState("github.com/LiteLDev/LeviLamina", "client", "0.13.0", true),
		},
	}
	useFakeLipWorkspace(t, ws)
	lipResolveTargetDir = resolveLIPTargetDir

	if got := InstallLeviLamina(context.Background(), "1.21.80.03", "Demo", "0.13.1"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.postStates = map[string]lip.PackageInstallState{}
	if got := UninstallLeviLamina(context.Background(), "Demo"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"update", "uninstall"}, [][]string{
		{"github.com/LiteLDev/LeviLamina#client@0.13.1"},
		{"github.com/LiteLDev/LeviLamina#client"},
	})
}

func TestInstallLeviLaminaSwitchesToUpdateWhenAlreadyExplicit(t *testing.T) {
	restoreLeviLaminaTestDeps(t)
	versionsDir := setupLeviLaminaTestVersionsDir(t)
	if err := os.MkdirAll(filepath.Join(versionsDir, "Demo"), 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	fetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{"1.21.80.03": {"0.13.1"}}, nil
	}
	ws := &fakeLipWorkspace{
		states:     map[string]lip.PackageInstallState{},
		installErr: []error{fmt.Errorf("Cannot install 'github.com/LiteLDev/LeviLamina@0.13.1': package 'github.com/LiteLDev/LeviLamina' is already explicitly installed (version 0.13.0). %w", engine.ErrAlreadyExplicitlyInstalled)},
	}
	useFakeLipWorkspace(t, ws)
	lipResolveTargetDir = resolveLIPTargetDir

	if got := InstallLeviLamina(context.Background(), "1.21.80.03", "Demo", ""); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "update"}, [][]string{
		{"github.com/LiteLDev/LeviLamina#client@0.13.1"},
		{"github.com/LiteLDev/LeviLamina#client@0.13.1"},
	})
}

// A LeviLamina without a client variant (its variant "" has the client
// entries) is installed from its variant "".
func TestInstallLeviLaminaFallsBackToBareRepositoryWithoutClientVariant(t *testing.T) {
	restoreLeviLaminaTestDeps(t)
	versionsDir := setupLeviLaminaTestVersionsDir(t)
	if err := os.MkdirAll(filepath.Join(versionsDir, "Demo"), 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	fetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{"1.21.80.03": {"0.13.1"}}, nil
	}
	ws := &fakeLipWorkspace{
		states:     map[string]lip.PackageInstallState{},
		installErr: []error{wrapNoClientVariant("github.com/LiteLDev/LeviLamina#client@0.13.1", engine.ErrClientVariantNotInLipToml)},
	}
	useFakeLipWorkspace(t, ws)
	lipResolveTargetDir = resolveLIPTargetDir

	if got := InstallLeviLamina(context.Background(), "1.21.80.03", "Demo", ""); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "install"}, [][]string{
		{"github.com/LiteLDev/LeviLamina#client@0.13.1"},
		{"github.com/LiteLDev/LeviLamina@0.13.1"},
	})
}

// Rule 5: an installed LeviLamina#client updated to a version without a
// client variant (a release read from a lip.toml) becomes LeviLamina.
func TestInstallLeviLaminaMovesClientIdentityToLipTomlRelease(t *testing.T) {
	restoreLeviLaminaTestDeps(t)
	versionsDir := setupLeviLaminaTestVersionsDir(t)
	if err := os.MkdirAll(filepath.Join(versionsDir, "Demo"), 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	fetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{"1.21.80.03": {"0.13.0", "0.14.0"}}, nil
	}
	ws := &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/levilamina": migratedState("github.com/LiteLDev/LeviLamina", "client", "0.13.0", true),
		},
		updateErrs: []error{wrapNoClientVariant("github.com/LiteLDev/LeviLamina#client@0.14.0", engine.ErrClientVariantNotInLipToml)},
	}
	useFakeLipWorkspace(t, ws)
	lipResolveTargetDir = resolveLIPTargetDir

	if got := InstallLeviLamina(context.Background(), "1.21.80.03", "Demo", "0.14.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"update", "update"}, [][]string{
		{"github.com/LiteLDev/LeviLamina#client@0.14.0"},
		{"github.com/LiteLDev/LeviLamina@0.14.0"},
	})
}

// Rule 5 the other way: an installed LeviLamina (a release read from a
// lip.toml) changed to a tooth.json release is asked for as it is, then, as
// lip reports, as its client variant;
// a tooth.json release without it (a server build only) is not installed.
func TestInstallLeviLaminaMovesToToothJSONClientVariant(t *testing.T) {
	restoreLeviLaminaTestDeps(t)
	versionsDir := setupLeviLaminaTestVersionsDir(t)
	if err := os.MkdirAll(filepath.Join(versionsDir, "Demo"), 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	fetchLeviLaminaVersionDB = func() (map[string][]string, error) {
		return map[string][]string{"1.21.80.03": {"0.13.0", "0.14.0"}}, nil
	}
	ws := &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/levilamina": migratedState("github.com/LiteLDev/LeviLamina", "", "0.14.0", true),
		},
		updateErrs: []error{clientBuildInRoleVariant("github.com/LiteLDev/LeviLamina@0.13.0")},
	}
	useFakeLipWorkspace(t, ws)
	lipResolveTargetDir = resolveLIPTargetDir
	if got := InstallLeviLamina(context.Background(), "1.21.80.03", "Demo", "0.13.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"update", "update"}, [][]string{{"github.com/LiteLDev/LeviLamina@0.13.0"},
		{"github.com/LiteLDev/LeviLamina#client@0.13.0"}})

	ws = &fakeLipWorkspace{
		states:     map[string]lip.PackageInstallState{},
		installErr: []error{wrapNoClientVariant("github.com/LiteLDev/LeviLamina#client@0.13.0", engine.ErrClientVariantNotInToothJSON)},
	}
	useFakeLipWorkspace(t, ws)
	lipResolveTargetDir = resolveLIPTargetDir
	if got := InstallLeviLamina(context.Background(), "1.21.80.03", "Demo", "0.13.0"); got != "ERR_LIP_INSTALL_FAILED" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install"}, [][]string{{"github.com/LiteLDev/LeviLamina#client@0.13.0"}})
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
	restoreLipQueryDeps(t)
	originalFetch := fetchLeviLaminaVersionDB
	t.Cleanup(func() {
		fetchLeviLaminaVersionDB = originalFetch
		apppath.SetBaseRootOverride("")
	})
}

func setupLeviLaminaTestVersionsDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	apppath.SetBaseRootOverride(filepath.Join(root, "base"))
	versionsDir, err := apppath.InstancesDir()
	if err != nil {
		t.Fatalf("resolve versions dir: %v", err)
	}
	return versionsDir
}
