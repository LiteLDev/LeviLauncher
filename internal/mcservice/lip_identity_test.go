package mcservice

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/lipcore/engine"
	"github.com/liteldev/LeviLauncher/internal/types"
)

// fakeLipWorkspace stands in for lip on one instance: List returns states
// (keyed by package id, as lip.ListPackageStates does) and every operation
// is recorded.
type fakeLipWorkspace struct {
	states     map[string]lip.PackageInstallState
	postStates map[string]lip.PackageInstallState // states after a successful operation, when set
	installErr []error                            // returned by successive Install calls
	updateErr  error
	calls      []string
	args       [][]string
}

func useFakeLipWorkspace(t *testing.T, ws *fakeLipWorkspace) {
	t.Helper()
	restoreLipQueryDeps(t)
	lipResolveTargetDir = func(targetName string) (string, string) { return `C:\demo`, "" }
	lipListPackageStates = func(ctx context.Context, workDir string) (map[string]lip.PackageInstallState, error) {
		return ws.states, nil
	}
	done := func() {
		if ws.postStates != nil {
			ws.states = ws.postStates
		}
	}
	lipInstallPackages = func(ctx context.Context, workDir string, packages []string) error {
		ws.calls = append(ws.calls, "install")
		ws.args = append(ws.args, append([]string{}, packages...))
		if len(ws.installErr) > 0 {
			err := ws.installErr[0]
			ws.installErr = ws.installErr[1:]
			if err != nil {
				return err
			}
		}
		done()
		return nil
	}
	lipUpdatePackages = func(ctx context.Context, workDir string, packages []string) error {
		ws.calls = append(ws.calls, "update")
		ws.args = append(ws.args, append([]string{}, packages...))
		if ws.updateErr != nil {
			return ws.updateErr
		}
		done()
		return nil
	}
	lipUninstallPackages = func(ctx context.Context, workDir string, packages []string, noDependencies bool) error {
		if noDependencies {
			t.Fatalf("uninstall must keep dependency handling")
		}
		ws.calls = append(ws.calls, "uninstall")
		ws.args = append(ws.args, append([]string{}, packages...))
		done()
		return nil
	}
}

func (ws *fakeLipWorkspace) expect(t *testing.T, calls []string, args [][]string) {
	t.Helper()
	if !reflect.DeepEqual(ws.calls, calls) || !reflect.DeepEqual(ws.args, args) {
		t.Fatalf("lip calls = %v %v; want %v %v", ws.calls, ws.args, calls, args)
	}
}

func migratedState(id, variant, version string, explicit bool) lip.PackageInstallState {
	identity := id
	if variant != "" {
		identity += "#" + variant
	}
	return lip.PackageInstallState{
		Identity:          identity,
		ID:                id,
		Variant:           variant,
		Installed:         true,
		ExplicitInstalled: explicit,
		InstalledVersion:  version,
	}
}

func TestBuildLIPPackageRequest(t *testing.T) {
	tests := []struct {
		in   string
		want lipPackageRequest
		ok   bool
	}{
		{in: "LiteLDev/Foo", want: lipPackageRequest{identifier: "LiteLDev/Foo", packageID: "github.com/LiteLDev/Foo", requestRef: "github.com/LiteLDev/Foo"}, ok: true},
		{in: " LiteLDev/Foo#client ", want: lipPackageRequest{identifier: "LiteLDev/Foo", packageID: "github.com/LiteLDev/Foo", requestRef: "github.com/LiteLDev/Foo"}, ok: true},
		{in: "github.com/LiteLDev/Foo#client", want: lipPackageRequest{identifier: "LiteLDev/Foo", packageID: "github.com/LiteLDev/Foo", requestRef: "github.com/LiteLDev/Foo"}, ok: true},
		{in: "https://github.com/LiteLDev/Foo/", want: lipPackageRequest{identifier: "LiteLDev/Foo", packageID: "github.com/LiteLDev/Foo", requestRef: "github.com/LiteLDev/Foo"}, ok: true},
		{in: "LiteLDev/Foo#", want: lipPackageRequest{identifier: "LiteLDev/Foo", packageID: "github.com/LiteLDev/Foo", requestRef: "github.com/LiteLDev/Foo"}, ok: true},
		{in: "LiteLDev/Foo#server@1.0.0", want: lipPackageRequest{identifier: "LiteLDev/Foo#server", packageID: "github.com/LiteLDev/Foo", variant: "server", requestRef: "github.com/LiteLDev/Foo#server"}, ok: true},
		{in: "foo", ok: false},
		{in: "gitlab.com/a/b", ok: false},
		{in: "", ok: false},
	}
	for _, tt := range tests {
		got, ok := buildLIPPackageRequest(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("buildLIPPackageRequest(%q) = %+v, %v; want %+v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestInstallLIPPackageRequestsBareRepository(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{}}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo#client", "1.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install"}, [][]string{{"github.com/LiteLDev/Foo@1.0.0"}})
}

// A request for another variant than the installed one installs that
// variant instead of changing the installed one.
func TestInstallLIPPackageOtherVariantIsInstalled(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{
		"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "bar", "1.0.0", true),
	}}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo#extra", "2.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo#bar", "2.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "update"}, [][]string{{"github.com/LiteLDev/Foo#extra@2.0.0"}, {"github.com/LiteLDev/Foo#bar@2.0.0"}})
}

func TestIsLipAlreadyExplicitlyInstalledForMatchesWholeID(t *testing.T) {
	err := fmt.Errorf("Cannot install 'github.com/LiteLDev/LeviLaminaExt@1.0.0': package 'github.com/LiteLDev/LeviLaminaExt' is already explicitly installed (version 1.0.0). %w", engine.ErrAlreadyExplicitlyInstalled)
	if isLipAlreadyExplicitlyInstalledFor(err, leviLaminaPackageID) {
		t.Fatal("LeviLaminaExt must not match LeviLamina")
	}
	if !isLipAlreadyExplicitlyInstalledFor(err, "github.com/LiteLDev/LeviLaminaExt") {
		t.Fatal("expected a match")
	}
	migrated := fmt.Errorf("Cannot install 'github.com/LiteLDev/LeviLamina#client@1.0.0': package 'github.com/LiteLDev/LeviLamina#client' is already explicitly installed (version 1.0.0). %w", engine.ErrAlreadyExplicitlyInstalled)
	if !isLipAlreadyExplicitlyInstalledFor(migrated, leviLaminaPackageID) {
		t.Fatal("expected a match for the #client identity")
	}
}

func TestInstallLIPPackageRequestsNonClientVariant(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{}}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo#extra", "1.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install"}, [][]string{{"github.com/LiteLDev/Foo#extra@1.0.0"}})
}

// A package migrated from tooth_lock.json keeps its "#client" identity and is
// updated in place instead of being installed a second time.
func TestInstallLIPPackageUpdatesMigratedClientIdentity(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{
		"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "client", "1.0.0", true),
	}}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.1.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"update"}, [][]string{{"github.com/LiteLDev/Foo#client@1.1.0"}})
}

// A dependency is installed explicitly under the identity it has, and a
// LeviLamina dependency is pinned under its recorded identity.
func TestInstallLIPPackagePinsMigratedLeviLamina(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{
		"github.com/liteldev/foo":        migratedState("github.com/LiteLDev/Foo", "client", "1.0.0", false),
		"github.com/liteldev/levilamina": migratedState("github.com/LiteLDev/LeviLamina", "client", "1.2.3", false),
	}}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install"}, [][]string{{"github.com/LiteLDev/Foo#client@1.0.0", "github.com/LiteLDev/LeviLamina#client@1.2.3"}})
}

func TestInstallLIPPackageRetriesWithoutPinnedLeviLamina(t *testing.T) {
	ws := &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/levilamina": migratedState("github.com/LiteLDev/LeviLamina", "", "1.2.3", false),
		},
		installErr: []error{fmt.Errorf("Cannot install 'github.com/LiteLDev/LeviLamina@1.2.3': package 'github.com/LiteLDev/LeviLamina' is already explicitly installed (version 1.2.3). %w", engine.ErrAlreadyExplicitlyInstalled)},
	}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "install"}, [][]string{
		{"github.com/LiteLDev/Foo@1.0.0", "github.com/LiteLDev/LeviLamina@1.2.3"},
		{"github.com/LiteLDev/Foo@1.0.0"},
	})
}

func TestInstallLIPPackageFallsBackToUpdateWhenAlreadyExplicit(t *testing.T) {
	ws := &fakeLipWorkspace{
		states:     map[string]lip.PackageInstallState{},
		installErr: []error{fmt.Errorf("Cannot install 'github.com/LiteLDev/Foo@1.0.0': package 'github.com/LiteLDev/Foo' is already explicitly installed (version 0.9.0). %w", engine.ErrAlreadyExplicitlyInstalled)},
	}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "update"}, [][]string{{"github.com/LiteLDev/Foo@1.0.0"}, {"github.com/LiteLDev/Foo@1.0.0"}})
}

func TestInstallLIPPackageFailure(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{}, installErr: []error{errors.New("network down")}}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); got != "ERR_LIP_PACKAGE_INSTALL_FAILED" {
		t.Fatalf("unexpected error code: %q", got)
	}
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", " "); got != "ERR_LIP_PACKAGE_VERSION_REQUIRED" {
		t.Fatalf("unexpected error code: %q", got)
	}
	if got := InstallLIPPackage(context.Background(), "Demo", "nope", "1.0.0"); got != "ERR_LIP_PACKAGE_INVALID_IDENTIFIER" {
		t.Fatalf("unexpected error code: %q", got)
	}
}

func TestUninstallLIPPackageUsesMigratedIdentity(t *testing.T) {
	ws := &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "client", "1.0.0", true),
		},
		postStates: map[string]lip.PackageInstallState{},
	}
	useFakeLipWorkspace(t, ws)

	if got := UninstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"uninstall"}, [][]string{{"github.com/LiteLDev/Foo#client"}})
}

func TestUninstallLIPPackageDependencyStates(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{
		"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "", "1.0.0", false),
	}}
	useFakeLipWorkspace(t, ws)
	if got := UninstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo"); got != "ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, nil, nil)

	ws = &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "", "1.0.0", true),
		},
		postStates: map[string]lip.PackageInstallState{
			"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "", "1.0.0", false),
		},
	}
	useFakeLipWorkspace(t, ws)
	if got := UninstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo"); got != "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"uninstall"}, [][]string{{"github.com/LiteLDev/Foo"}})
}

func TestGetLIPPackageInstallStatesMatchesAnyVariant(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{
		"github.com/liteldev/levilamina": migratedState("github.com/LiteLDev/LeviLamina", "client", "1.2.3", true),
	}}
	useFakeLipWorkspace(t, ws)

	entries := GetLIPPackageInstallStates(context.Background(), "Demo", []string{"LiteLDev/LeviLamina", "liteldev/levilamina#client", "LiteLDev/Other"})
	if len(entries) != 3 {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	for _, entry := range entries[:2] {
		if !entry.State.Installed || !entry.State.ExplicitInstalled || entry.State.InstalledVersion != "1.2.3" ||
			entry.State.PackageRef != "github.com/LiteLDev/LeviLamina#client" {
			t.Fatalf("unexpected state for %s: %+v", entry.IdentifierKey, entry.State)
		}
	}
	if entries[1].IdentifierKey != "liteldev/levilamina#client" || entries[1].State.Identifier != "liteldev/levilamina" {
		t.Fatalf("unexpected entry: %+v", entries[1])
	}
	if other := entries[2].State; other.Installed || other.PackageRef != "github.com/LiteLDev/Other" {
		t.Fatalf("unexpected state: %+v", other)
	}
}

func TestInstanceBackupLIPRestoreIdentifier(t *testing.T) {
	tests := []struct {
		item types.InstanceBackupModsLIPPackage
		want string
	}{
		{item: types.InstanceBackupModsLIPPackage{Identifier: "LiteLDev/Foo#client"}, want: "LiteLDev/Foo#client"},
		{item: types.InstanceBackupModsLIPPackage{Identifier: "LiteLDev/Foo", Identity: "github.com/LiteLDev/Foo#extra"}, want: "github.com/LiteLDev/Foo#extra"},
		{item: types.InstanceBackupModsLIPPackage{Identifier: "LiteLDev/Foo", Identity: "example.com/x"}, want: "LiteLDev/Foo"},
	}
	for _, tt := range tests {
		if got := instanceBackupLIPRestoreIdentifier(tt.item); got != tt.want {
			t.Fatalf("instanceBackupLIPRestoreIdentifier(%+v) = %q, want %q", tt.item, got, tt.want)
		}
		// Old "#client" identifiers and new ones request the same package.
		if req, ok := buildLIPPackageRequest(instanceBackupLIPRestoreIdentifier(tt.item)); !ok || req.packageID != "github.com/LiteLDev/Foo" {
			t.Fatalf("restore identifier of %+v does not name the package: %+v", tt.item, req)
		}
	}
}
