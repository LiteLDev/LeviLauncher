package lip

import (
	"encoding/json"
	"testing"
)

func TestParsePackageInstallStatesExplicitAndImplicit(t *testing.T) {
	raw := json.RawMessage(`{
		"Explicit": [
			"github.com/LiteLDev/Foo#client@1.2.3"
		],
		"Dependencies": [
			"github.com/LiteLDev/Bar#client@4.5.6"
		]
	}`)

	stateByPackageRef, err := parsePackageInstallStates(raw)
	if err != nil {
		t.Fatalf("parsePackageInstallStates failed: %v", err)
	}

	explicit := stateByPackageRef["github.com/liteldev/foo#client"]
	if !explicit.Installed || !explicit.ExplicitInstalled {
		t.Fatalf("expected explicit package to be installed explicitly, got %+v", explicit)
	}
	if explicit.InstalledVersion != "1.2.3" {
		t.Fatalf("unexpected explicit version: %q", explicit.InstalledVersion)
	}

	implicit := stateByPackageRef["github.com/liteldev/bar#client"]
	if !implicit.Installed || implicit.ExplicitInstalled {
		t.Fatalf("expected implicit package to be installed transitively, got %+v", implicit)
	}
	if implicit.InstalledVersion != "4.5.6" {
		t.Fatalf("unexpected implicit version: %q", implicit.InstalledVersion)
	}
}

// lipd >= 0.35 List result.
func TestParsePackageInstallStatesNestedStringGroups(t *testing.T) {
	raw := json.RawMessage(`[
		["github.com/LiteLDev/LeviLamina#client@26.51.9"],
		["github.com/LiteLDev/CrashLogger#client@1.3.3","github.com/LiteLDev/PeEditor@3.9.2"]
	]`)

	stateByPackageRef, err := parsePackageInstallStates(raw)
	if err != nil {
		t.Fatalf("parsePackageInstallStates failed: %v", err)
	}

	ll := stateByPackageRef["github.com/liteldev/levilamina#client"]
	if !ll.Installed || !ll.ExplicitInstalled || ll.InstalledVersion != "26.51.9" {
		t.Fatalf("unexpected LeviLamina state: %+v", ll)
	}
	dep := stateByPackageRef["github.com/liteldev/crashlogger#client"]
	if !dep.Installed || dep.ExplicitInstalled || dep.InstalledVersion != "1.3.3" {
		t.Fatalf("unexpected dependency state: %+v", dep)
	}
	if _, ok := stateByPackageRef["github.com/liteldev/peeditor"]; !ok {
		t.Fatal("expected variant-less dependency to be indexed")
	}
}

func TestParsePackageInstallStatesNestedSpecGroups(t *testing.T) {
	raw := json.RawMessage(`[[{"Id":{"Path":"github.com/LiteLDev/Foo","Variant":"client"},"Version":"1.0.0"}],[]]`)

	stateByPackageRef, err := parsePackageInstallStates(raw)
	if err != nil {
		t.Fatalf("parsePackageInstallStates failed: %v", err)
	}
	foo := stateByPackageRef["github.com/liteldev/foo#client"]
	if !foo.ExplicitInstalled || foo.InstalledVersion != "1.0.0" {
		t.Fatalf("unexpected state: %+v", foo)
	}
}

func TestParsePackageInstallStatesEmptyResult(t *testing.T) {
	stateByPackageRef, err := parsePackageInstallStates(json.RawMessage(`[]`))
	if err != nil {
		t.Fatalf("parsePackageInstallStates failed: %v", err)
	}
	if len(stateByPackageRef) != 0 {
		t.Fatalf("expected empty state map, got %d entries", len(stateByPackageRef))
	}
}

func TestParsePackageInstallStatesInvalidResult(t *testing.T) {
	_, err := parsePackageInstallStates(json.RawMessage(`{"unexpected":true}`))
	if err == nil {
		t.Fatal("expected parsePackageInstallStates to fail for unsupported payload")
	}
}
