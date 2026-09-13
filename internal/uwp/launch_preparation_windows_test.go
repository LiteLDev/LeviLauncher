package uwp

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLaunchPreparationRequiresSelectedDevelopmentRegistration(t *testing.T) {
	for _, registered := range []bool{false, true} {
		_, dir := setupDeployment(t)
		runPowerShell = func(_ context.Context, script string) ([]byte, error) {
			if !strings.HasPrefix(script, "Get-AppxPackage") {
				t.Fatalf("unregistered or Store instance must not be changed: %s", script)
			}
			if registered {
				return packageJSON(t, dir, false), nil
			}
			return nil, nil
		}
		_, err := LaunchWithPreparation(context.Background(), dir, false, func() error {
			t.Fatal("unregistered or Store instance must not be patched")
			return nil
		})
		want := "ERR_UWP_NOT_REGISTERED"
		if registered {
			want = "ERR_UWP_PACKAGE_CONFLICT"
		}
		if ErrorCode(err) != want {
			t.Fatalf("got %v, want %s", err, want)
		}
	}
}

func TestLaunchPreparesAfterFullTrustAndStopsOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		editor bool
		fail   string
	}{
		{"game", false, ""},
		{"game-registration-failure", false, "registration"},
		{"game-preparation-failure", false, "preparation"},
		{"editor", true, ""},
		{"editor-registration-failure", true, "registration"},
		{"editor-preparation-failure", true, "preparation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fail := tc.fail
			_, dir := setupDeployment(t)
			registered, prepared, activated := false, false, false
			runPowerShell = func(_ context.Context, script string) ([]byte, error) {
				if strings.HasPrefix(script, "Get-AppxPackage") {
					return packageJSON(t, dir, true), nil
				}
				if !strings.HasPrefix(script, "Add-AppxPackage") {
					t.Fatalf("unexpected deployment: %s", script)
				}
				if fail == "registration" {
					return []byte("0x80073CF3"), errors.New("missing dependency")
				}
				registered = true
				return nil, nil
			}
			oldActivate, oldProtocol := activateApplication, activateProtocol
			t.Cleanup(func() { activateApplication, activateProtocol = oldActivate, oldProtocol })
			checkActivation := func(id string) (int, error) {
				if !registered || !prepared || id != ReleaseFamilyName+"!App" {
					t.Fatalf("activation before preparation: %s", id)
				}
				activated = true
				return 123, nil
			}
			activateApplication = func(id string) (int, error) {
				if tc.editor {
					t.Fatal("editor must receive protocol activation")
				}
				return checkActivation(id)
			}
			activateProtocol = func(id, uri string) (int, error) {
				if !tc.editor || uri != "minecraft:?Editor=true" {
					t.Fatalf("unexpected protocol activation: %s", uri)
				}
				return checkActivation(id)
			}
			pid, err := LaunchWithPreparation(context.Background(), dir, tc.editor, func() error {
				if !registered {
					t.Fatal("patched before full trust registration")
				}
				prepared = true
				if fail == "preparation" {
					return errors.New("cannot patch executable")
				}
				return nil
			})
			if fail == "" {
				if err != nil || pid != 123 || !activated {
					t.Fatalf("launch: pid=%d error=%v", pid, err)
				}
			} else {
				want := "ERR_UWP_PREPARE"
				if fail == "registration" {
					want = "ERR_UWP_DEPENDENCY"
					if prepared {
						t.Fatal("registration failure did not stop preparation")
					}
				}
				if ErrorCode(err) != want || pid != 0 || activated {
					t.Fatalf("failed launch: pid=%d error=%v activated=%v", pid, err, activated)
				}
			}
		})
	}
}
