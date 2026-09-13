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
		_, err := LaunchWithPreparation(context.Background(), dir, func() error {
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
	for _, fail := range []string{"", "registration", "preparation"} {
		t.Run(fail, func(t *testing.T) {
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
			oldActivate := activateApplication
			t.Cleanup(func() { activateApplication = oldActivate })
			activateApplication = func(id string) (int, error) {
				if !registered || !prepared || id != ReleaseFamilyName+"!App" {
					t.Fatalf("activation before preparation: %s", id)
				}
				activated = true
				return 123, nil
			}
			pid, err := LaunchWithPreparation(context.Background(), dir, func() error {
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
