package uwp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestLaunchMigratesExistingLegacyAuthKey(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "vanilla x86", true: "native loader"}[native], func(t *testing.T) {
			_, dir := setupDeployment(t)
			writeLegacyLaunchFixture(t, dir)
			exe := filepath.Join(dir, "Minecraft.Windows.exe")
			// Editable metadata must not suppress migration of a legacy manifest.
			if err := os.WriteFile(filepath.Join(dir, "version.json"), []byte(`{"gameVersion":"1.21.93.1"}`), 0644); err != nil {
				t.Fatal(err)
			}
			runPowerShell = func(_ context.Context, script string) ([]byte, error) {
				if strings.HasPrefix(script, "Get-AppxPackage") {
					return packageJSON(t, dir, true), nil
				}
				if !native || !strings.HasPrefix(script, "Add-AppxPackage") {
					t.Fatalf("unexpected deployment operation: %s", script)
				}
				return nil, nil
			}
			var prepare func() error
			if native {
				prepare = func() error {
					data, err := os.ReadFile(exe)
					if err != nil || string(data) != currentAuthPublicKey {
						t.Fatalf("key must be patched before native loader preparation: %v", err)
					}
					return nil
				}
			}
			oldActivate := activateApplication
			t.Cleanup(func() { activateApplication = oldActivate })
			activations := 0
			activateApplication = func(id string) (int, error) {
				activations++
				assertAuthMarker(t, dir, "patched\n")
				if id != ReleaseFamilyName+"!App" {
					t.Fatalf("unexpected app ID: %s", id)
				}
				return 123, nil
			}
			if pid, err := LaunchWithPreparation(context.Background(), dir, false, prepare, nil); err != nil || pid != 123 || activations != 1 {
				t.Fatalf("first launch: pid=%d, activations=%d, error=%v", pid, activations, err)
			}
			// An inaccessible executable on the next launch proves that the auth
			// migration uses the marker. The Windows activation is mocked here.
			if err := os.Remove(exe); err != nil {
				t.Fatal(err)
			}
			if native {
				prepare = func() error { return nil }
			}
			if _, err := LaunchWithPreparation(context.Background(), dir, false, prepare, nil); err != nil || activations != 2 {
				t.Fatalf("repeat launch: activations=%d, error=%v", activations, err)
			}
		})
	}
}

func TestLegacyAuthLaunchFailureDoesNotActivateOrMark(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"unregistered", "ERR_UWP_NOT_REGISTERED"},
		{"wrong instance", "ERR_UWP_NOT_REGISTERED"},
		{"Store registration", "ERR_UWP_PACKAGE_CONFLICT"},
		{"missing executable", "ERR_UWP_PREPARE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, dir := setupDeployment(t)
			writeLegacyLaunchFixture(t, dir)
			if tc.name == "missing executable" {
				if err := os.Remove(filepath.Join(dir, "Minecraft.Windows.exe")); err != nil {
					t.Fatal(err)
				}
			}
			runPowerShell = func(_ context.Context, script string) ([]byte, error) {
				if !strings.HasPrefix(script, "Get-AppxPackage") {
					t.Fatalf("unexpected deployment operation: %s", script)
				}
				switch tc.name {
				case "unregistered":
					return nil, nil
				case "wrong instance":
					return packageJSON(t, filepath.Join(filepath.Dir(dir), "old"), true), nil
				case "Store registration":
					return packageJSON(t, dir, false), nil
				default:
					return packageJSON(t, dir, true), nil
				}
			}
			oldActivate := activateApplication
			t.Cleanup(func() { activateApplication = oldActivate })
			activateApplication = func(string) (int, error) {
				t.Fatal("activated after preparation failure")
				return 0, nil
			}
			pid, err := LaunchWithPreparation(context.Background(), dir, false, nil, func() {
				t.Fatal("started observer after preparation failure")
			})
			if pid != 0 || ErrorCode(err) != tc.want {
				t.Fatalf("pid=%d error=%v, want %s", pid, err, tc.want)
			}
			assertAuthMarker(t, dir, "")
			if tc.name != "missing executable" {
				data, err := os.ReadFile(filepath.Join(dir, "Minecraft.Windows.exe"))
				if err != nil || string(data) != legacyAuthPublicKey {
					t.Fatalf("rejected instance was modified: %v", err)
				}
			}
		})
	}
}

func TestLaunchPreservesBothAuthKeys(t *testing.T) {
	_, dir := setupDeployment(t)
	manifest := strings.Replace(testManifest("neutral"), "1.21.9301.0", "1.20.1201.0", 1)
	if err := os.WriteFile(filepath.Join(dir, "AppxManifest.xml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "Minecraft.Windows.exe")
	contents := legacyAuthPublicKey + "\x00" + currentAuthPublicKey
	if err := os.WriteFile(exe, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	runPowerShell = func(_ context.Context, script string) ([]byte, error) {
		if !strings.HasPrefix(script, "Get-AppxPackage") {
			t.Fatalf("unexpected deployment: %s", script)
		}
		return packageJSON(t, dir, true), nil
	}
	oldActivate := activateApplication
	t.Cleanup(func() { activateApplication = oldActivate })
	activateApplication = func(string) (int, error) {
		data, err := os.ReadFile(exe)
		if err != nil || string(data) != contents {
			t.Fatalf("dual-key executable was modified: %v", err)
		}
		assertAuthMarker(t, dir, "already-current\n")
		return 123, nil
	}
	if pid, err := Launch(context.Background(), dir); err != nil || pid != 123 {
		t.Fatalf("dual-key launch: pid=%d error=%v", pid, err)
	}
}

func TestLegacyAuthKeyLockedExecutableRemainsRetryable(t *testing.T) {
	dir := t.TempDir()
	m := legacyAuthManifest()
	exe := filepath.Join(dir, m.Applications[0].Executable)
	if err := os.WriteFile(exe, []byte(legacyAuthPublicKey), 0644); err != nil {
		t.Fatal(err)
	}
	path, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	patchErr := ensureLegacyAuthKey(context.Background(), dir, m)
	windows.CloseHandle(handle)
	if patchErr == nil {
		t.Fatal("patched locked executable")
	}
	assertAuthMarker(t, dir, "")
	if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
		t.Fatal(err)
	}
	assertAuthMarker(t, dir, "patched\n")
}

func TestLegacyAuthKeyMarkerWriteFailureRemainsRetryable(t *testing.T) {
	dir := t.TempDir()
	m := legacyAuthManifest()
	exe := filepath.Join(dir, m.Applications[0].Executable)
	marker := filepath.Join(dir, authKeyMarkerName)
	if err := os.WriteFile(exe, []byte(legacyAuthPublicKey), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("pat"), 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(marker, 0644) })
	if err := ensureLegacyAuthKey(context.Background(), dir, m); err == nil {
		t.Fatal("marker write failure was swallowed")
	}
	assertAuthMarker(t, dir, "pat")
	if err := os.Chmod(marker, 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
		t.Fatal(err)
	}
	assertAuthMarker(t, dir, "already-current\n")
}

func writeLegacyLaunchFixture(t *testing.T, dir string) {
	t.Helper()
	manifest := strings.Replace(testManifest("neutral"), "1.21.9301.0", "1.19.8002.0", 1)
	if err := os.WriteFile(filepath.Join(dir, "AppxManifest.xml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Minecraft.Windows.exe"), []byte(legacyAuthPublicKey), 0644); err != nil {
		t.Fatal(err)
	}
}
