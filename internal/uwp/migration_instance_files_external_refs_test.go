package uwp

// Regression tests for group "instance-files-external-refs": the Xbox
// authentication key marker of UWP instances and UWP registrations recorded
// at the legacy location, after the storage migration
// (internal/apppath/README.md, "从旧布局迁移").

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/storage"
	"golang.org/x/sys/windows"
)

func migUWPWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func migUWPRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func migUWPTree(t *testing.T, root string) {
	t.Helper()
	var lines []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			lines = append(lines, rel+`\`)
			return nil
		}
		c, _ := os.ReadFile(p)
		if len(c) > 48 {
			c = c[:48]
		}
		lines = append(lines, rel+" = "+strconv.Quote(string(c)))
		return nil
	})
	sort.Strings(lines)
	t.Logf("---- %s ----\n%s", root, strings.Join(lines, "\n"))
}

func migUWPHidden(p string) bool {
	ptr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return false
	}
	a, err := windows.GetFileAttributes(ptr)
	return err == nil && a&windows.FILE_ATTRIBUTE_HIDDEN != 0
}

// migUWPEnv points the launcher's folders at a temporary profile and returns
// the legacy directory.
func migUWPEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(root, "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "Local"))
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	t.Cleanup(func() {
		apppath.SetLegacyMode("")
		apppath.SetBaseRootOverride("")
	})
	return apppath.LegacyDir()
}

func migUWPPrepare(t *testing.T, want storage.Outcome) {
	t.Helper()
	if o, err := storage.Prepare(); err != nil || o != want {
		t.Fatalf("Prepare = %v, %v; want %v", o, err, want)
	}
}

// P4-auth-key-marker: the marker that UWP instances kept at their root moves
// into the hidden .levilauncher without a rescan; when both exist the new
// one is kept and the legacy one is left; an invalid marker is rescanned.
func TestMigration_P4_AuthKeyMarker(t *testing.T) {
	m := legacyAuthManifest()
	legacyName := apppath.LegacyInstanceAuthKeyMarker

	t.Run("legacy only", func(t *testing.T) {
		dir := t.TempDir()
		migUWPWrite(t, filepath.Join(dir, legacyName), "patched\n")
		// No executable: a rescan would fail.
		if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
			t.Fatalf("rescanned instead of using the moved marker: %v", err)
		}
		assertAuthMarker(t, dir, "patched\n")
		if _, err := os.Stat(filepath.Join(dir, legacyName)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("legacy marker not moved: %v", err)
		}
		if !migUWPHidden(filepath.Join(dir, ".levilauncher")) {
			t.Error(".levilauncher not hidden")
		}
		// The next launch uses it as well.
		if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
			t.Fatal(err)
		}
		migUWPTree(t, dir)
	})

	for _, state := range []string{"already-current\n", "not-applicable\n"} {
		t.Run("legacy only "+strings.TrimSpace(state), func(t *testing.T) {
			dir := t.TempDir()
			migUWPWrite(t, filepath.Join(dir, legacyName), state)
			if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
				t.Fatalf("rescanned: %v", err)
			}
			assertAuthMarker(t, dir, state)
		})
	}

	t.Run("both present", func(t *testing.T) {
		dir := t.TempDir()
		migUWPWrite(t, filepath.Join(dir, legacyName), "not-applicable\n")
		migUWPWrite(t, authKeyMarkerPath(dir), "already-current\n")
		if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
			t.Fatal(err)
		}
		assertAuthMarker(t, dir, "already-current\n")
		if got := migUWPRead(t, filepath.Join(dir, legacyName)); got != "not-applicable\n" {
			t.Errorf("legacy marker = %q; want it left in place", got)
		}
	})

	t.Run("legacy invalid", func(t *testing.T) {
		dir := t.TempDir()
		migUWPWrite(t, filepath.Join(dir, legacyName), "garbage")
		migUWPWrite(t, filepath.Join(dir, "Minecraft.Windows.exe"), legacyAuthPublicKey)
		if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
			t.Fatal(err)
		}
		assertAuthMarker(t, dir, "patched\n")
		if got := migUWPRead(t, filepath.Join(dir, "Minecraft.Windows.exe")); got != currentAuthPublicKey {
			t.Error("invalid marker did not cause a rescan")
		}
	})

	t.Run("after storage migration", func(t *testing.T) {
		S := migUWPEnv(t)
		migUWPWrite(t, filepath.Join(S, "config.json"), `{}`)
		migUWPWrite(t, filepath.Join(S, "versions", "U", legacyName), "patched\n")
		migUWPWrite(t, filepath.Join(S, "versions", "U", "version.json"), `{"name":"U","packageType":"uwp"}`)
		migUWPPrepare(t, storage.Migrated)
		inst, err := apppath.InstancesDir()
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(inst, "U")
		if got := migUWPRead(t, filepath.Join(dir, legacyName)); got != "patched\n" {
			t.Fatalf("marker moved by the migration as %q", got)
		}
		if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
			t.Fatalf("rescan after the migration: %v", err)
		}
		assertAuthMarker(t, dir, "patched\n")
		if !migUWPHidden(filepath.Join(dir, ".levilauncher")) {
			t.Error(".levilauncher not hidden")
		}
		migUWPTree(t, dir)
		migUWPPrepare(t, storage.Current)
	})
}

// P7-registered-packages (UWP): a development registration Windows recorded
// at versions\U is recognised as the moved, managed instance; the launch
// renews it at the new location; Unregister removes it with
// -PreserveApplicationData; after a second migration it belongs to U-legacy.
func TestMigration_P7_UWPRegistrationAfterMigration(t *testing.T) {
	S := migUWPEnv(t)
	migUWPWrite(t, filepath.Join(S, "config.json"), `{}`)
	legacyU := filepath.Join(S, "versions", "U")
	migUWPWrite(t, filepath.Join(legacyU, "AppxManifest.xml"), testManifest("neutral"))
	migUWPWrite(t, filepath.Join(legacyU, "Minecraft.Windows.exe"), currentAuthPublicKey)
	migUWPWrite(t, filepath.Join(legacyU, "version.json"), `{"name":"U","packageType":"uwp","registered":true}`)
	migUWPWrite(t, filepath.Join(S, "versions", "Other", "AppxManifest.xml"), testManifest("neutral"))
	migUWPWrite(t, filepath.Join(S, "versions", "Other", "version.json"), `{"name":"Other","packageType":"uwp"}`)
	migUWPPrepare(t, storage.Migrated)
	inst, _ := apppath.InstancesDir()
	newU := filepath.Join(inst, "U")
	other := filepath.Join(inst, "Other")

	for _, loc := range []string{legacyU, `\\?\` + strings.ToUpper(legacyU), legacyU + `\`} {
		pkg := &registeredPackage{PackageFamilyName: ReleaseFamilyName, InstallLocation: loc, IsDevelopmentMode: true}
		if !isMovedRegistration(pkg) {
			t.Errorf("isMovedRegistration(%s) = false", loc)
		}
		if !isManagedDevelopmentPackage(pkg) {
			t.Errorf("isManagedDevelopmentPackage(%s) = false", loc)
		}
		if !samePath(storage.TranslateLegacyPath(loc), newU) || samePath(storage.TranslateLegacyPath(loc), other) {
			t.Errorf("Translate(%s) = %s", loc, storage.TranslateLegacyPath(loc))
		}
	}
	if isMovedRegistration(&registeredPackage{InstallLocation: newU}) {
		t.Error("a registration at the current location is reported as moved")
	}
	store := &registeredPackage{PackageFamilyName: ReleaseFamilyName, InstallLocation: `C:\Program Files\WindowsApps\Microsoft.MinecraftUWP_1.21.9301.0_x64__8wekyb3d8bbwe`}
	if isMovedRegistration(store) || isManagedDevelopmentPackage(store) {
		t.Error("a Store installation is treated as a moved managed registration")
	}

	old := runPowerShell
	t.Cleanup(func() { runPowerShell = old })
	oldAct := activateApplication
	t.Cleanup(func() { activateApplication = oldAct })
	activateApplication = func(string) (int, error) { return 42, nil }

	// Launch: the registration at versions\U is renewed at Instances\U; the
	// first attempt conflicts with the old registration, which is removed
	// keeping its application data.
	var cmds []string
	registered := legacyU
	conflictOnce := true
	runPowerShell = func(_ context.Context, script string) ([]byte, error) {
		cmds = append(cmds, script)
		switch {
		case strings.HasPrefix(script, "Get-AppxPackage"):
			return packageJSON(t, registered, true), nil
		case strings.HasPrefix(script, "Add-AppxPackage"):
			if conflictOnce {
				conflictOnce = false
				return []byte("Deployment failed with HRESULT: 0x80073CFB"), errors.New("exit status 1")
			}
			// Add-AppxPackage -Register '<dir>\AppxManifest.xml' ...
			if _, rest, ok := strings.Cut(script, "-Register '"); ok {
				if manifest, _, ok := strings.Cut(rest, "'"); ok {
					registered = filepath.Dir(manifest)
				}
			}
			return nil, nil
		case strings.HasPrefix(script, "Remove-AppxPackage"):
			registered = ""
			return nil, nil
		}
		return nil, nil
	}
	pid, err := LaunchWithPreparation(context.Background(), newU, false, nil, nil)
	if err != nil || pid != 42 {
		t.Fatalf("launch of the moved instance: pid=%d err=%v cmds=%q", pid, err, cmds)
	}
	var added, removed bool
	for _, c := range cmds {
		lc := strings.ToLower(c)
		if strings.HasPrefix(c, "Add-AppxPackage") && strings.Contains(lc, strings.ToLower(newU)) {
			added = true
		}
		if strings.HasPrefix(c, "Add-AppxPackage") && strings.Contains(lc, strings.ToLower(legacyU)) {
			t.Errorf("registered at the legacy location: %s", c)
		}
		if strings.HasPrefix(c, "Remove-AppxPackage") {
			removed = true
			if !strings.Contains(c, "-PreserveApplicationData") {
				t.Errorf("old registration removed without -PreserveApplicationData: %s", c)
			}
		}
	}
	if !added || !removed {
		t.Errorf("registration not renewed at the new location (added=%v removed=%v): %q", added, removed, cmds)
	}
	assertAuthMarker(t, newU, "already-current\n")

	// Launching another instance does not take over U's registration.
	registered = legacyU
	cmds = nil
	if _, err := LaunchWithPreparation(context.Background(), other, false, nil, nil); ErrorCode(err) != "ERR_UWP_NOT_REGISTERED" {
		t.Errorf("launch of Other with U registered = %v; want ERR_UWP_NOT_REGISTERED", err)
	}
	for _, c := range cmds {
		if !strings.HasPrefix(c, "Get-AppxPackage") {
			t.Errorf("launch of Other changed the registration: %s", c)
		}
	}

	// Unregister of the moved instance removes the legacy registration,
	// keeping its data; Unregister of Other leaves it alone.
	cmds = nil
	registered = legacyU
	if err := Unregister(context.Background(), other); err != nil {
		t.Errorf("Unregister(Other) = %v", err)
	}
	if err := Unregister(context.Background(), newU); err != nil {
		t.Errorf("Unregister(U) = %v", err)
	}
	removes := 0
	for _, c := range cmds {
		if strings.HasPrefix(c, "Remove-AppxPackage") {
			removes++
			if !strings.Contains(c, "-PreserveApplicationData") {
				t.Errorf("removal without -PreserveApplicationData: %s", c)
			}
		}
	}
	if removes != 1 {
		t.Errorf("Unregister removed %d registrations: %q", removes, cmds)
	}

	// Downgrade and upgrade: an older launcher created and registered
	// another versions\U, which the next migration places at U-legacy.
	migUWPWrite(t, filepath.Join(S, "config.json"), `{}`)
	migUWPWrite(t, filepath.Join(legacyU, "AppxManifest.xml"), testManifest("neutral"))
	migUWPWrite(t, filepath.Join(legacyU, "Minecraft.Windows.exe"), currentAuthPublicKey)
	migUWPWrite(t, filepath.Join(legacyU, "version.json"), `{"name":"U","packageType":"uwp","registered":true}`)
	migUWPPrepare(t, storage.Migrated)
	uLegacy := filepath.Join(inst, "U-legacy")
	if _, err := os.Stat(filepath.Join(uLegacy, "AppxManifest.xml")); err != nil {
		t.Fatalf("U-legacy: %v", err)
	}
	if got := storage.TranslateLegacyPath(legacyU); !samePath(got, uLegacy) {
		t.Errorf("Translate(versions\\U) after the second migration = %s, want %s", got, uLegacy)
	}
	registered = legacyU
	conflictOnce = false
	cmds = nil
	if _, err := LaunchWithPreparation(context.Background(), newU, false, nil, nil); ErrorCode(err) != "ERR_UWP_NOT_REGISTERED" {
		t.Errorf("launch of U with U-legacy's registration = %v; want ERR_UWP_NOT_REGISTERED", err)
	}
	registered = legacyU
	cmds = nil
	if pid, err := LaunchWithPreparation(context.Background(), uLegacy, false, nil, nil); err != nil || pid != 42 {
		t.Errorf("launch of U-legacy = %d, %v (%q)", pid, err, cmds)
	}
	if !samePath(registered, uLegacy) {
		t.Errorf("registration now at %s, want %s", registered, uLegacy)
	}
	migUWPTree(t, inst)
	migUWPPrepare(t, storage.Current)
}
