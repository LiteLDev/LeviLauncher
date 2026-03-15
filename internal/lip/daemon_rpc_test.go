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
