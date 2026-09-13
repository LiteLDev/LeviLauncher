package uwp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorLaunchTargetsRegisteredPackage(t *testing.T) {
	for _, tc := range []struct {
		name, packageName, familyName, uri string
	}{
		{"release", ReleasePackageName, ReleaseFamilyName, "minecraft:?Editor=true"},
		{"beta", ReleasePackageName, ReleaseFamilyName, "minecraft:?Editor=true"},
		{"preview", PreviewPackageName, PreviewFamilyName, "minecraft-preview:?Editor=true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, dir := setupDeployment(t)
			manifest := strings.ReplaceAll(testManifest("neutral"), ReleasePackageName, tc.packageName)
			if err := os.WriteFile(filepath.Join(dir, "AppxManifest.xml"), []byte(manifest), 0600); err != nil {
				t.Fatal(err)
			}
			runPowerShell = func(_ context.Context, script string) ([]byte, error) {
				if !strings.HasPrefix(script, "Get-AppxPackage -Name '"+tc.packageName+"'") {
					t.Fatalf("launch changed registration or queried the wrong package: %s", script)
				}
				return []byte(strings.ReplaceAll(string(packageJSON(t, dir, true)), ReleasePackageName, tc.packageName)), nil
			}
			oldActivate, oldProtocol := activateApplication, activateProtocol
			t.Cleanup(func() { activateApplication, activateProtocol = oldActivate, oldProtocol })
			activateApplication = func(string) (int, error) {
				t.Fatal("editor URI must be delivered through protocol activation")
				return 0, nil
			}
			calls := 0
			activationErr := errors.New("protocol activation failed")
			activateProtocol = func(id, uri string) (int, error) {
				calls++
				if id != tc.familyName+"!App" || uri != tc.uri {
					t.Fatalf("wrong editor target: %s %s", id, uri)
				}
				if calls == 2 {
					return 0, activationErr
				}
				return 4321, nil
			}
			if pid, err := LaunchWithPreparation(context.Background(), dir, true, nil, nil); err != nil || pid != 4321 || calls != 1 {
				t.Fatalf("editor launch: pid=%d err=%v calls=%d", pid, err, calls)
			}
			if pid, err := LaunchWithPreparation(context.Background(), dir, true, nil, nil); pid != 0 || ErrorCode(err) != "ERR_UWP_LAUNCH" || !errors.Is(err, activationErr) {
				t.Fatalf("activation failure lost: pid=%d err=%v", pid, err)
			}
		})
	}
}

func TestEditorLaunchRequiresSelectedRegistration(t *testing.T) {
	_, dir := setupDeployment(t)
	oldActivate, oldProtocol := activateApplication, activateProtocol
	t.Cleanup(func() { activateApplication, activateProtocol = oldActivate, oldProtocol })
	activateApplication = func(string) (int, error) {
		t.Fatal("unregistered editor must not activate another package")
		return 0, nil
	}
	activateProtocol = func(string, string) (int, error) {
		t.Fatal("unregistered editor must not activate another package")
		return 0, nil
	}
	for _, registeredDir := range []string{"", filepath.Join(t.TempDir(), "other-instance")} {
		runPowerShell = func(_ context.Context, script string) ([]byte, error) {
			if !strings.HasPrefix(script, "Get-AppxPackage") {
				t.Fatalf("editor launch changed registration: %s", script)
			}
			if registeredDir == "" {
				return nil, nil
			}
			return packageJSON(t, registeredDir, true), nil
		}
		pid, err := LaunchWithPreparation(context.Background(), dir, true, func() error {
			t.Fatal("unregistered editor must not prepare another package")
			return nil
		}, nil)
		if pid != 0 || ErrorCode(err) != "ERR_UWP_NOT_REGISTERED" {
			t.Fatalf("unregistered editor launch: pid=%d err=%v", pid, err)
		}
	}
}

func TestEditorLaunchEnforcesManifestMinimumVersion(t *testing.T) {
	for _, tc := range []struct {
		name, packageName, manifestVersion, uri string
		editor                                  bool
	}{
		{"retail-before", ReleasePackageName, "1.21.4999.0", "minecraft:?Editor=true", false},
		{"retail-minimum", ReleasePackageName, "1.21.5000.0", "minecraft:?Editor=true", true},
		{"preview-before", PreviewPackageName, "1.19.8019.0", "minecraft-preview:?Editor=true", false},
		{"preview-minimum", PreviewPackageName, "1.19.8020.0", "minecraft-preview:?Editor=true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, dir := setupDeployment(t)
			manifest := strings.NewReplacer(ReleasePackageName, tc.packageName, "1.21.9301.0", tc.manifestVersion).Replace(testManifest("neutral"))
			if err := os.WriteFile(filepath.Join(dir, "AppxManifest.xml"), []byte(manifest), 0600); err != nil {
				t.Fatal(err)
			}
			runPowerShell = func(_ context.Context, script string) ([]byte, error) {
				if !strings.HasPrefix(script, "Get-AppxPackage") {
					t.Fatalf("unexpected registration change: %s", script)
				}
				return []byte(strings.ReplaceAll(string(packageJSON(t, dir, true)), ReleasePackageName, tc.packageName)), nil
			}
			oldActivate, oldProtocol := activateApplication, activateProtocol
			t.Cleanup(func() { activateApplication, activateProtocol = oldActivate, oldProtocol })
			activateApplication = func(string) (int, error) {
				if tc.editor {
					t.Fatal("minimum supported version did not launch Editor")
				}
				return 321, nil
			}
			activateProtocol = func(_ string, uri string) (int, error) {
				if !tc.editor || uri != tc.uri {
					t.Fatalf("unsupported or incorrect Editor activation: %s", uri)
				}
				return 321, nil
			}
			if pid, err := LaunchWithPreparation(context.Background(), dir, true, nil, nil); err != nil || pid != 321 {
				t.Fatalf("launch: pid=%d err=%v", pid, err)
			}
		})
	}
}
