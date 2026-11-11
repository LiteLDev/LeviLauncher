package mcservice

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"fmt"
	"reflect"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/lipcore/engine"
	lipref "github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipindex"
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
	updateErrs []error // returned by successive Update calls before updateErr
	planned    []lip.PlannedPackage // what a dry run (lipPlanPackages) returns
	planErr    error
	calls      []string
	args       [][]string
}

func useFakeLipWorkspace(t *testing.T, ws *fakeLipWorkspace) {
	t.Helper()
	restoreLipQueryDeps(t)
	// The catalogue is unreachable unless a test gives details
	// (useFakeCatalogue).
	savedFetch, savedPlatform := lipFetchDetail, lipindex.TargetPlatform
	t.Cleanup(func() { lipFetchDetail, lipindex.TargetPlatform = savedFetch, savedPlatform })
	lipindex.TargetPlatform = "win-x64"
	lipFetchDetail = func(ctx context.Context, packageID string) (*lipindex.Detail, error) {
		return nil, lipindex.ErrIndexUnavailable
	}
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
		if len(ws.updateErrs) > 0 {
			err := ws.updateErrs[0]
			ws.updateErrs = ws.updateErrs[1:]
			if err != nil {
				return err
			}
		}
		if ws.updateErr != nil {
			return ws.updateErr
		}
		done()
		return nil
	}
	lipPlanPackages = func(ctx context.Context, workDir string, update bool, packages []string) ([]lip.PlannedPackage, error) {
		ws.calls = append(ws.calls, "plan")
		ws.args = append(ws.args, append([]string{}, packages...))
		return ws.planned, ws.planErr
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

// useFakeCatalogue makes the catalogue answer with detail documents
// (levimc.org/api/v4/mods/<id>/index.json) by package id; others are not found.
func useFakeCatalogue(t *testing.T, docs map[string]string) {
	t.Helper()
	lipFetchDetail = func(ctx context.Context, packageID string) (*lipindex.Detail, error) {
		doc, ok := docs[packageID]
		if !ok {
			return nil, lipindex.ErrPackageNotFound
		}
		var d lipindex.Detail
		if err := json.Unmarshal([]byte(doc), &d); err != nil {
			t.Fatal(err)
		}
		return &d, nil
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
		{in: " LiteLDev/Foo#client ", want: lipPackageRequest{identifier: "LiteLDev/Foo#client", packageID: "github.com/LiteLDev/Foo", variant: "client", named: true, requestRef: "github.com/LiteLDev/Foo#client"}, ok: true},
		{in: "github.com/LiteLDev/Foo#client", want: lipPackageRequest{identifier: "LiteLDev/Foo#client", packageID: "github.com/LiteLDev/Foo", variant: "client", named: true, requestRef: "github.com/LiteLDev/Foo#client"}, ok: true},
		{in: "https://github.com/LiteLDev/Foo/", want: lipPackageRequest{identifier: "LiteLDev/Foo", packageID: "github.com/LiteLDev/Foo", requestRef: "github.com/LiteLDev/Foo"}, ok: true},
		{in: "LiteLDev/Foo#", want: lipPackageRequest{identifier: "LiteLDev/Foo", packageID: "github.com/LiteLDev/Foo", named: true, requestRef: "github.com/LiteLDev/Foo"}, ok: true},
		{in: "github.com/LiteLDev/Foo#@1.0.0", want: lipPackageRequest{identifier: "LiteLDev/Foo", packageID: "github.com/LiteLDev/Foo", named: true, requestRef: "github.com/LiteLDev/Foo"}, ok: true},
		{in: "LiteLDev/Foo#server@1.0.0", want: lipPackageRequest{identifier: "LiteLDev/Foo#server", packageID: "github.com/LiteLDev/Foo", variant: "server", named: true, requestRef: "github.com/LiteLDev/Foo#server"}, ok: true},
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

// Catalogue details for the variant chooser (rule A2: "client", then "",
// then the others by name, among the variants with a client entry for the
// platform).
const (
	chooserDefaultOnly = `{"id":"github.com/LiteLDev/Foo","versions":[
		{"version":"1.0.0","variants":{"":[{"side":"server","platform":"win-x64","deps":{}},{"side":"client","platform":"win-x64","deps":{}}]}}]}`
	chooserClientAndDefault = `{"id":"github.com/LiteLDev/Foo","versions":[
		{"version":"1.0.0","variants":{"":[{"side":"client","platform":"win-x64","deps":{}}],"client":[{"side":"client","platform":"win-x64","deps":{}}]}}]}`
	chooserNamed = `{"id":"github.com/LiteLDev/Foo","versions":[
		{"version":"2.0.0","variants":{"":[{"side":"server","platform":"win-x64","deps":{}}],"zeta":[{"side":"client","platform":"win-x64","deps":{}}],"lua":[{"side":"client","platform":"win-x64","deps":{}}]}},
		{"version":"1.0.0","variants":{"client":[{"side":"client","platform":"win-x64","deps":{}}]}}]}`
	chooserServerOnly = `{"id":"github.com/LiteLDev/Foo","versions":[
		{"version":"1.0.0","variants":{"":[{"side":"server","platform":"win-x64","deps":{}}],"client":[{"side":"client","platform":"linux-x64","deps":{}}]}}]}`
)

func TestInstallLIPPackageChoosesVariantFromCatalogue(t *testing.T) {
	for _, tc := range []struct {
		name, doc, version, want string
	}{
		{"default variant for both sides", chooserDefaultOnly, "1.0.0", "github.com/LiteLDev/Foo@1.0.0"},
		{"client before the default", chooserClientAndDefault, "1.0.0", "github.com/LiteLDev/Foo#client@1.0.0"},
		{"the others by name", chooserNamed, "2.0.0", "github.com/LiteLDev/Foo#lua@2.0.0"},
		{"over the version installed", chooserNamed, "1.0.0", "github.com/LiteLDev/Foo#client@1.0.0"},
		{"no client entry for the platform", chooserServerOnly, "1.0.0", "github.com/LiteLDev/Foo@1.0.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{}}
			useFakeLipWorkspace(t, ws)
			useFakeCatalogue(t, map[string]string{"github.com/LiteLDev/Foo": tc.doc})
			if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", tc.version); got != "" {
				t.Fatalf("unexpected error code: %q", got)
			}
			ws.expect(t, []string{"install"}, [][]string{{tc.want}})
		})
	}
}

// A package the catalogue does not list falls back to the conventional
// client variant, then the variant "".
func TestInstallLIPPackageUnlistedPackageFallsBack(t *testing.T) {
	ws := &fakeLipWorkspace{
		states:     map[string]lip.PackageInstallState{},
		installErr: []error{wrapNoClientVariant("github.com/LiteLDev/Foo#client@1.0.0", engine.ErrClientVariantNotInLipToml)},
	}
	useFakeLipWorkspace(t, ws)
	useFakeCatalogue(t, map[string]string{})
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "install"}, [][]string{
		{"github.com/LiteLDev/Foo#client@1.0.0"}, {"github.com/LiteLDev/Foo@1.0.0"}})
}

// An installed package is changed in place whatever its variant, without
// asking the catalogue; naming another variant installs that one.
func TestInstallLIPPackageInstalledIdentityWins(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{
		"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "lua", "1.0.0", true),
	}}
	useFakeLipWorkspace(t, ws)
	lipFetchDetail = func(ctx context.Context, packageID string) (*lipindex.Detail, error) {
		t.Fatal("the catalogue is not asked for an installed package")
		return nil, nil
	}
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "2.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo#", "2.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"update", "install"}, [][]string{{"github.com/LiteLDev/Foo#lua@2.0.0"}, {"github.com/LiteLDev/Foo@2.0.0"}})
}

// Without the catalogue, a request without a variant asks for the client
// variant; one naming it does so anyway.
func TestInstallLIPPackageRequestsClientVariant(t *testing.T) {
	for _, identifier := range []string{"LiteLDev/Foo", "LiteLDev/Foo#client"} {
		ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{}}
		useFakeLipWorkspace(t, ws)

		if got := InstallLIPPackage(context.Background(), "Demo", identifier, "1.0.0"); got != "" {
			t.Fatalf("unexpected error code: %q", got)
		}
		ws.expect(t, []string{"install"}, [][]string{{"github.com/LiteLDev/Foo#client@1.0.0"}})
	}
}

func TestInstallLIPPackageRequestsBareRepositoryForEmptyVariant(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{}}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo#", "1.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install"}, [][]string{{"github.com/LiteLDev/Foo@1.0.0"}})
}

// wrapNoClientVariant is the engine's failure to find a package for a
// request for a client variant, of a kind (engine.ErrClientVariantNot*).
func wrapNoClientVariant(arg string, kind error) error {
	return fmt.Errorf("No package found for '%s' that can be installed in this workspace (client, win-x64). %w%w",
		arg, engine.ErrNoPackageFound, kind)
}

func wrapNoPackageFound(arg string) error {
	return fmt.Errorf("No package found for '%s' that can be installed in this workspace (client, win-x64). %w",
		arg, engine.ErrNoPackageFound)
}

// Without the catalogue, a package without a client variant (its variant ""
// has entries for both sides) is installed from its variant "".
func TestInstallLIPPackageFallsBackToBareRepositoryWithoutClientVariant(t *testing.T) {
	ws := &fakeLipWorkspace{
		states:     map[string]lip.PackageInstallState{},
		installErr: []error{wrapNoClientVariant("github.com/LiteLDev/Foo#client@1.0.0", engine.ErrClientVariantNotInLipToml)},
	}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "install"}, [][]string{
		{"github.com/LiteLDev/Foo#client@1.0.0"}, {"github.com/LiteLDev/Foo@1.0.0"}})
}

// A failure to find another package (a pinned dependency) is not retried.
func TestInstallLIPPackageDoesNotFallBackForAnotherPackage(t *testing.T) {
	ws := &fakeLipWorkspace{
		states:     map[string]lip.PackageInstallState{},
		installErr: []error{wrapNoPackageFound("github.com/LiteLDev/Bar@1.0.0")},
	}
	useFakeLipWorkspace(t, ws)

	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); !strings.HasPrefix(got, "ERR_LIP_PACKAGE_INSTALL_FAILED: ") {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install"}, [][]string{{"github.com/LiteLDev/Foo#client@1.0.0"}})
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
		{"github.com/LiteLDev/Foo#client@1.0.0", "github.com/LiteLDev/LeviLamina@1.2.3"},
		{"github.com/LiteLDev/Foo#client@1.0.0"},
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
	ws.expect(t, []string{"install", "update"}, [][]string{{"github.com/LiteLDev/Foo#client@1.0.0"}, {"github.com/LiteLDev/Foo#client@1.0.0"}})
}

func TestInstallLIPPackageFailure(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{}, installErr: []error{errors.New("network down")}}
	useFakeLipWorkspace(t, ws)

	// The result carries lip's message after the code, for the GUI to explain.
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); got != "ERR_LIP_PACKAGE_INSTALL_FAILED: network down" {
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
	// The state of a package is matched by package id, whatever the variant.
	if len(entries) != 3 {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	for _, entry := range entries[:2] {
		if !entry.State.Installed || !entry.State.ExplicitInstalled || entry.State.InstalledVersion != "1.2.3" ||
			entry.State.PackageRef != "github.com/LiteLDev/LeviLamina#client" {
			t.Fatalf("unexpected state for %s: %+v", entry.IdentifierKey, entry.State)
		}
	}
	if entries[1].IdentifierKey != "liteldev/levilamina#client" || entries[1].State.Identifier != "liteldev/levilamina#client" {
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

// clientBuildInRoleVariant is the engine's refusal of a request "X" over an
// installed lip.toml "X" whose release is a tooth.json with a client label.
func clientBuildInRoleVariant(arg string) error {
	return fmt.Errorf("Cannot use '%s': ... is read from a tooth.json, which keeps its client build in"+
		" '...#client'. %w", arg, engine.ErrClientBuildInRoleVariant)
}

func roleBuildClash() error {
	return fmt.Errorf("Cannot find versions of the requested packages and their dependencies that can all be"+
		" installed together: ... is another build of the same package as ... %w", engine.ErrRoleBuildClash)
}

func TestLipSwitchedIdentity(t *testing.T) {
	const foo = "github.com/LiteLDev/Foo"
	lipToml := func(ref string) error { return wrapNoClientVariant(ref+"@1.0.0", engine.ErrClientVariantNotInLipToml) }
	tooth := func(ref string) error { return wrapNoClientVariant(ref+"@1.0.0", engine.ErrClientVariantNotInToothJSON) }
	for _, tc := range []struct {
		err       error
		ref       string
		orNoLabel bool
		want      string
	}{
		{lipToml(foo + "#client"), foo + "#client", false, foo},
		{lipToml(foo + "#client_lua"), foo + "#client_lua", false, foo + "#lua"},
		{tooth(foo + "#client"), foo + "#client", false, ""},
		{tooth(foo + "#client"), foo + "#client", true, foo},
		{wrapNoPackageFound(foo + "#client@1.0.0"), foo + "#client", true, ""},
		{lipToml(foo + "#client"), foo + "#lua", false, ""},
		{lipToml("github.com/LiteLDev/Bar#client"), foo + "#client", false, ""},
		{clientBuildInRoleVariant(foo + "@1.0.0"), foo, false, foo + "#client"},
		{clientBuildInRoleVariant(foo + "#lua@1.0.0"), foo + "#lua", false, foo + "#client_lua"},
		{roleBuildClash(), foo, false, ""},
		{nil, foo + "#client", true, ""},
	} {
		if got := lipSwitchedIdentity(tc.err, tc.ref+"@1.0.0", tc.ref, tc.orNoLabel); got != tc.want {
			t.Errorf("lipSwitchedIdentity(%v, %q, %v) = %q, want %q", tc.err, tc.ref, tc.orNoLabel, got, tc.want)
		}
	}
}

func TestInstallLIPPackageMovesClientIdentityToLipTomlRelease(t *testing.T) {
	ws := &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "client", "1.0.0", true),
		},
		updateErrs: []error{wrapNoClientVariant("github.com/LiteLDev/Foo#client@2.0.0", engine.ErrClientVariantNotInLipToml)},
	}
	useFakeLipWorkspace(t, ws)
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "2.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"update", "update"}, [][]string{
		{"github.com/LiteLDev/Foo#client@2.0.0"}, {"github.com/LiteLDev/Foo@2.0.0"}})

	ws = &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "client_lua", "1.0.0", false),
		},
		installErr: []error{wrapNoClientVariant("github.com/LiteLDev/Foo#client_lua@2.0.0", engine.ErrClientVariantNotInLipToml)},
	}
	useFakeLipWorkspace(t, ws)
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "2.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "install"}, [][]string{
		{"github.com/LiteLDev/Foo#client_lua@2.0.0"}, {"github.com/LiteLDev/Foo#lua@2.0.0"}})
}

// A release read from a tooth.json keeps "#client": no fallback is tried.
func TestInstallLIPPackageKeepsClientIdentityForToothJSONRelease(t *testing.T) {
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{
		"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "client", "1.0.0", true),
	}}
	useFakeLipWorkspace(t, ws)
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.1.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"update"}, [][]string{{"github.com/LiteLDev/Foo#client@1.1.0"}})
}

// A backup restore requests the recorded identity ("#client" of an old
// record) at the recorded version, and falls back to the bare id only when
// that version has no client variant.
func TestRestoreOldClientRecord(t *testing.T) {
	item := types.InstanceBackupModsLIPPackage{Identifier: "LiteLDev/Foo", PackageID: "github.com/LiteLDev/Foo",
		Identity: "github.com/LiteLDev/Foo#client", Version: "1.0.0", ExplicitInstalled: true}
	ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{}}
	useFakeLipWorkspace(t, ws)
	if got := InstallLIPPackage(context.Background(), "Demo", instanceBackupLIPRestoreIdentifier(item), item.Version); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install"}, [][]string{{"github.com/LiteLDev/Foo#client@1.0.0"}})

	ws = &fakeLipWorkspace{
		states:     map[string]lip.PackageInstallState{},
		installErr: []error{wrapNoClientVariant("github.com/LiteLDev/Foo#client@2.0.0", engine.ErrClientVariantNotInLipToml)},
	}
	useFakeLipWorkspace(t, ws)
	item.Version = "2.0.0"
	if got := InstallLIPPackage(context.Background(), "Demo", instanceBackupLIPRestoreIdentifier(item), item.Version); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "install"}, [][]string{
		{"github.com/LiteLDev/Foo#client@2.0.0"}, {"github.com/LiteLDev/Foo@2.0.0"}})
}

// The fallback to the bare id happens only for a release read from a
// lip.toml: not for a tooth.json release without the client label (its ""
// is a server build) when the client variant is installed or named, and not
// for other failures such as a download error.
func TestInstallLIPPackageFallbackOnlyForLipTomlRelease(t *testing.T) {
	for _, tc := range []struct {
		name, identifier string
		states           map[string]lip.PackageInstallState
		err              error
		wantCode         bool
		wantCalls        [][]string
	}{
		{"installed client, tooth.json without the label", "LiteLDev/Foo",
			map[string]lip.PackageInstallState{"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "client", "1.0.0", false)},
			wrapNoClientVariant("github.com/LiteLDev/Foo#client@1.1.0", engine.ErrClientVariantNotInToothJSON), true,
			[][]string{{"github.com/LiteLDev/Foo#client@1.1.0"}}},
		{"named client, tooth.json without the label", "LiteLDev/Foo#client", map[string]lip.PackageInstallState{},
			wrapNoClientVariant("github.com/LiteLDev/Foo#client@1.1.0", engine.ErrClientVariantNotInToothJSON), true,
			[][]string{{"github.com/LiteLDev/Foo#client@1.1.0"}}},
		{"installed client, download error", "LiteLDev/Foo",
			map[string]lip.PackageInstallState{"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "client", "1.0.0", false)},
			wrapNoPackageFound("github.com/LiteLDev/Foo#client@1.1.0"), true,
			[][]string{{"github.com/LiteLDev/Foo#client@1.1.0"}}},
		{"catalogue unknown, tooth.json without the label", "LiteLDev/Foo", map[string]lip.PackageInstallState{},
			wrapNoClientVariant("github.com/LiteLDev/Foo#client@1.1.0", engine.ErrClientVariantNotInToothJSON), false,
			[][]string{{"github.com/LiteLDev/Foo#client@1.1.0"}, {"github.com/LiteLDev/Foo@1.1.0"}}},
		{"installed client variant of a lip.toml (lip gives no signal)", "LiteLDev/Foo",
			map[string]lip.PackageInstallState{"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "client", "1.0.0", false)},
			fmt.Errorf("No package found for 'github.com/LiteLDev/Foo#client@1.1.0' ... The installed"+
				" 'github.com/LiteLDev/Foo#client' is read from a lip.toml ... %w", engine.ErrNoPackageFound), true,
			[][]string{{"github.com/LiteLDev/Foo#client@1.1.0"}}},
		{"catalogue unknown, download error", "LiteLDev/Foo", map[string]lip.PackageInstallState{},
			wrapNoPackageFound("github.com/LiteLDev/Foo#client@1.1.0"), true,
			[][]string{{"github.com/LiteLDev/Foo#client@1.1.0"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := &fakeLipWorkspace{states: tc.states, installErr: []error{tc.err}}
			useFakeLipWorkspace(t, ws)
			got := InstallLIPPackage(context.Background(), "Demo", tc.identifier, "1.1.0")
			if (got != "") != tc.wantCode {
				t.Fatalf("unexpected error code: %q", got)
			}
			calls := make([]string, len(tc.wantCalls))
			for i := range calls {
				calls[i] = "install"
			}
			ws.expect(t, calls, tc.wantCalls)
		})
	}
}

// Rule 5 the other way: an installed package without a client variant is
// requested as it is (main's behaviour: a third-party lip.toml "X" with its
// own client variant, a tooth.json "X", an installed "X#lua" stay as they
// are), and as "X#client" only when lip reports that the target release is
// a tooth.json keeping its client build there while the installed one is a
// lip.toml; lip replaces it in place.
func TestInstallLIPPackageMovesToToothJSONClientVariant(t *testing.T) {
	for _, variant := range []string{"", "lua"} {
		ws := &fakeLipWorkspace{states: map[string]lip.PackageInstallState{
			"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", variant, "2.0.0", true),
		}}
		useFakeLipWorkspace(t, ws)
		ref := lipref.Identity("github.com/LiteLDev/Foo", variant)
		if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); got != "" {
			t.Fatalf("unexpected error code: %q", got)
		}
		ws.expect(t, []string{"update"}, [][]string{{ref + "@1.0.0"}})
	}

	ws := &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "", "2.0.0", true),
		},
		updateErrs: []error{clientBuildInRoleVariant("github.com/LiteLDev/Foo@1.0.0")},
	}
	useFakeLipWorkspace(t, ws)
	if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo", "1.0.0"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"update", "update"}, [][]string{{"github.com/LiteLDev/Foo@1.0.0"}, {"github.com/LiteLDev/Foo#client@1.0.0"}})

	// A dependency (install), and a backup restore of the recorded identity.
	ws = &fakeLipWorkspace{
		states: map[string]lip.PackageInstallState{
			"github.com/liteldev/foo": migratedState("github.com/LiteLDev/Foo", "", "2.0.0", false),
		},
		installErr: []error{clientBuildInRoleVariant("github.com/LiteLDev/Foo@1.0.0")},
	}
	useFakeLipWorkspace(t, ws)
	item := types.InstanceBackupModsLIPPackage{Identifier: "LiteLDev/Foo", PackageID: "github.com/LiteLDev/Foo",
		Identity: "github.com/LiteLDev/Foo", Version: "1.0.0", ExplicitInstalled: true}
	if got := InstallLIPPackage(context.Background(), "Demo", instanceBackupLIPRestoreIdentifier(item), item.Version); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}
	ws.expect(t, []string{"install", "install"}, [][]string{{"github.com/LiteLDev/Foo@1.0.0"}, {"github.com/LiteLDev/Foo#client@1.0.0"}})
}

// A pinned "LeviLamina#client" is dropped only when lip fails to solve
// because of it and the lip.toml LeviLamina the package needs
// (ErrRoleBuildClash); any other failure (another LeviLamina#client version
// needed, a hook or a download) keeps it and fails.
func TestInstallLIPPackageRetriesWithoutClientVariantPin(t *testing.T) {
	pinned := map[string]lip.PackageInstallState{
		"github.com/liteldev/levilamina": migratedState("github.com/LiteLDev/LeviLamina", "client", "1.2.3", false),
	}
	// The retry runs only when its dry run leaves LeviLamina as pinned or
	// as the bare lip.toml identity; otherwise the pinned failure stands.
	for _, tc := range []struct {
		name    string
		planned []lip.PlannedPackage
		planErr error
		retried bool
	}{
		{"lip.toml LeviLamina", []lip.PlannedPackage{{Identity: leviLaminaPackageID, ID: leviLaminaPackageID, Version: "2.0.0"}}, nil, true},
		{"kept as pinned", []lip.PlannedPackage{{Identity: leviLaminaPackageID + "#client", ID: leviLaminaPackageID, Variant: "client", Version: "1.2.3", FromToothJSON: true}}, nil, true},
		{"another tooth.json version", []lip.PlannedPackage{{Identity: leviLaminaPackageID + "#client", ID: leviLaminaPackageID, Variant: "client", Version: "1.3.0", FromToothJSON: true}}, nil, false},
		{"tooth.json server build", []lip.PlannedPackage{{Identity: leviLaminaPackageID, ID: leviLaminaPackageID, Version: "1.3.0", FromToothJSON: true}}, nil, false},
		{"no LeviLamina", []lip.PlannedPackage{{Identity: "github.com/LiteLDev/Foo", ID: "github.com/LiteLDev/Foo", Version: "1.0.0"}}, nil, false},
		{"dry run fails", nil, errors.New("Cannot find versions"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := &fakeLipWorkspace{states: pinned, installErr: []error{roleBuildClash()}, planned: tc.planned, planErr: tc.planErr}
			useFakeLipWorkspace(t, ws)
			got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo#", "1.0.0")
			calls := []string{"install", "plan"}
			args := [][]string{{"github.com/LiteLDev/Foo@1.0.0", "github.com/LiteLDev/LeviLamina#client@1.2.3"},
				{"github.com/LiteLDev/Foo@1.0.0"}}
			if tc.retried {
				// The retry requests the LeviLamina the dry run planned.
				planned := tc.planned[0].Identity + "@" + tc.planned[0].Version
				calls, args = append(calls, "install"), append(args, []string{"github.com/LiteLDev/Foo@1.0.0", planned})
				if got != "" {
					t.Fatalf("unexpected error code: %q", got)
				}
			} else if !strings.Contains(got, "another build of the same package") {
				t.Fatalf("not the pinned failure: %q", got)
			}
			ws.expect(t, calls, args)
		})
	}

	for _, err := range []error{
		errors.New("Cannot find versions of the requested packages and their dependencies"),
		errors.New("In the post_install hook of github.com/LiteLDev/Foo, the command 'x' failed."),
		wrapNoPackageFound("github.com/LiteLDev/Foo@1.0.0"),
	} {
		ws := &fakeLipWorkspace{states: pinned, installErr: []error{err}}
		useFakeLipWorkspace(t, ws)
		if got := InstallLIPPackage(context.Background(), "Demo", "LiteLDev/Foo#", "1.0.0"); got == "" {
			t.Fatalf("%v: succeeded", err)
		}
		ws.expect(t, []string{"install"}, [][]string{
			{"github.com/LiteLDev/Foo@1.0.0", "github.com/LiteLDev/LeviLamina#client@1.2.3"}})
	}
}
