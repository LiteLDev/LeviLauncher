package uwp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

func setupDeployment(t *testing.T) (string, string) {
	t.Helper()
	apppath.SetBaseRootOverride(t.TempDir())
	t.Cleanup(func() { apppath.SetBaseRootOverride("") })
	root, err := apppath.VersionsDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "selected")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AppxManifest.xml"), []byte(testManifest("neutral")), 0644); err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(root, "old")
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "AppxManifest.xml"), []byte(testManifest("neutral")), 0644); err != nil {
		t.Fatal(err)
	}
	old := runPowerShell
	t.Cleanup(func() { runPowerShell = old })
	return root, dir
}

func packageJSON(t *testing.T, dir string, dev bool) []byte {
	t.Helper()
	b, err := json.Marshal(registeredPackage{PackageFullName: "Microsoft.MinecraftUWP_1.21.9301.0_x64__8wekyb3d8bbwe", PackageFamilyName: ReleaseFamilyName, InstallLocation: dir, IsDevelopmentMode: dev})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRegisterStoreConflictNeverUninstalls(t *testing.T) {
	_, dir := setupDeployment(t)
	var commands []string
	runPowerShell = func(_ context.Context, script string) ([]byte, error) {
		commands = append(commands, script)
		if strings.HasPrefix(script, "Get-AppxPackage") {
			return packageJSON(t, `C:\Program Files\WindowsApps\Minecraft`, false), nil
		}
		return []byte("0x80073CFB"), errors.New("deployment failed")
	}
	if err := Register(context.Background(), dir); ErrorCode(err) != "ERR_UWP_PACKAGE_CONFLICT" {
		t.Fatalf("unexpected error %v", err)
	}
	for _, cmd := range commands {
		if strings.Contains(cmd, "Remove-AppxPackage") {
			t.Fatalf("Store package was removed: %s", cmd)
		}
	}
}

func TestRegisterManagedConflictPreservesDataAndVerifies(t *testing.T) {
	root, dir := setupDeployment(t)
	oldDir := filepath.Join(root, "old")
	var commands []string
	addCalls := 0
	queries := 0
	runPowerShell = func(_ context.Context, script string) ([]byte, error) {
		commands = append(commands, script)
		if strings.HasPrefix(script, "Get-AppxPackage") {
			queries++
			if queries == 1 {
				return packageJSON(t, oldDir, true), nil
			}
			return packageJSON(t, dir, true), nil
		}
		if strings.HasPrefix(script, "Add-AppxPackage") {
			addCalls++
			if addCalls == 1 {
				return []byte("0x80073CFB"), errors.New("same version conflict")
			}
		}
		return nil, nil
	}
	if err := Register(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	removed := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "Remove-AppxPackage") {
			removed = true
			if !strings.Contains(cmd, "-PreserveApplicationData") || strings.Contains(cmd, "-PreserveRoamableApplicationData") {
				t.Fatalf("unsafe removal %s", cmd)
			}
		}
	}
	if !removed || queries != 2 {
		t.Fatalf("missing switch or verification: %#v", commands)
	}
}

func TestRegisterManagedDependencyFailureDoesNotRemove(t *testing.T) {
	root, dir := setupDeployment(t)
	runPowerShell = func(_ context.Context, script string) ([]byte, error) {
		if strings.HasPrefix(script, "Get-AppxPackage") {
			return packageJSON(t, filepath.Join(root, "old"), true), nil
		}
		if strings.Contains(script, "Remove-AppxPackage") {
			t.Fatal("must not remove for dependency failure")
		}
		return []byte("0x80073CF3"), errors.New("dependencies missing")
	}
	if err := Register(context.Background(), dir); ErrorCode(err) != "ERR_UWP_DEPENDENCY" {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRegisterFailedSwitchRollsBack(t *testing.T) {
	root, dir := setupDeployment(t)
	oldDir := filepath.Join(root, "old")
	addCalls := 0
	var finalAdd string
	runPowerShell = func(_ context.Context, script string) ([]byte, error) {
		if strings.HasPrefix(script, "Get-AppxPackage") {
			return packageJSON(t, oldDir, true), nil
		}
		if strings.HasPrefix(script, "Add-AppxPackage") {
			addCalls++
			finalAdd = script
			if addCalls == 1 {
				return []byte("0x80073CFB"), errors.New("conflict")
			}
			if addCalls == 2 {
				return []byte("0x80073CF3"), errors.New("dependency")
			}
		}
		return nil, nil
	}
	if err := Register(context.Background(), dir); ErrorCode(err) != "ERR_UWP_DEPENDENCY" {
		t.Fatalf("unexpected error %v", err)
	}
	if addCalls != 3 || !strings.Contains(finalAdd, oldDir) {
		t.Fatalf("original registration not restored: %d %s", addCalls, finalAdd)
	}
}

func TestUnregisterOnlySelectedManagedPackage(t *testing.T) {
	_, dir := setupDeployment(t)
	for _, tc := range []struct {
		name, location string
		dev            bool
		want           string
		remove         bool
	}{
		{"not registered", "", false, "", false},
		{"another directory", filepath.Join(t.TempDir(), "other"), true, "", false},
		{"store registration", dir, false, "ERR_UWP_PACKAGE_CONFLICT", false},
		{"managed registration", dir, true, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			removed := false
			runPowerShell = func(_ context.Context, script string) ([]byte, error) {
				if strings.HasPrefix(script, "Get-AppxPackage") {
					if tc.location == "" {
						return nil, nil
					}
					return packageJSON(t, tc.location, tc.dev), nil
				}
				if strings.HasPrefix(script, "Remove-AppxPackage") {
					removed = true
				}
				return nil, nil
			}
			if err := Unregister(context.Background(), dir); ErrorCode(err) != tc.want {
				t.Fatalf("want %s: %v", tc.want, err)
			}
			if removed != tc.remove {
				t.Fatalf("removed=%v", removed)
			}
		})
	}
}

func TestLaunchUsesSelectedAUMID(t *testing.T) {
	_, dir := setupDeployment(t)
	runPowerShell = func(_ context.Context, script string) ([]byte, error) {
		if !strings.HasPrefix(script, "Get-AppxPackage") {
			t.Fatalf("unexpected deployment %s", script)
		}
		return packageJSON(t, dir, true), nil
	}
	previous := activateApplication
	t.Cleanup(func() { activateApplication = previous })
	activateApplication = func(id string) (int, error) {
		if id != ReleaseFamilyName+"!App" {
			t.Fatalf("unexpected AUMID %s", id)
		}
		return 1234, nil
	}
	if pid, err := Launch(context.Background(), dir); err != nil || pid != 1234 {
		t.Fatalf("pid=%d err=%v", pid, err)
	}
}

func TestPowerShellQuoting(t *testing.T) {
	if got := psQuote("C:\\用户\\O'Brien"); got != "'C:\\用户\\O''Brien'" {
		t.Fatalf("unsafe quoting %q", got)
	}
}

func TestPowerShellQueryIgnoresStderrProgress(t *testing.T) {
	output, err := executePowerShell(context.Background(), `[Console]::Error.WriteLine('#< CLIXML'); [Console]::Error.WriteLine('<Objs><Obj S="progress"/></Objs>'); [Console]::WriteLine('{"InstallLocation":"C:\\用户\\Minecraft"}')`)
	if err != nil {
		t.Fatalf("query process failed: %v %s", err, output)
	}
	var pkg registeredPackage
	if err := json.Unmarshal(output, &pkg); err != nil || pkg.InstallLocation != `C:\用户\Minecraft` {
		t.Fatalf("stderr contaminated JSON or Unicode was lost: %q %v", output, err)
	}
}

func TestPowerShellFailureRetainsDiagnostics(t *testing.T) {
	output, err := executePowerShell(context.Background(), `[Console]::Error.WriteLine('部署失败 0x80073CF3'); exit 1`)
	if err == nil || !strings.Contains(string(output), "部署失败 0x80073CF3") {
		t.Fatalf("missing Windows error: %q %v", output, err)
	}
	if code := ErrorCode(deploymentError("ERR_UWP_REGISTER", output, err)); code != "ERR_UWP_DEPENDENCY" {
		t.Fatalf("Windows failure lost its classification: %s", code)
	}
}

func TestPowerShellTerminatingErrorsAreReadable(t *testing.T) {
	output, err := executePowerShell(context.Background(), `throw 'Windows 部署失败 0x80073CF3'`)
	if err == nil || !strings.Contains(string(output), "Windows 部署失败 0x80073CF3") || strings.Contains(string(output), "#< CLIXML") {
		t.Fatalf("unreadable Windows failure: %q %v", output, err)
	}
}

func TestPrepareLooseRegistrationPreservesOriginalSignature(t *testing.T) {
	_, dir := setupDeployment(t)
	signature := filepath.Join(dir, "AppxSignature.p7x")
	original := []byte("original Store signature")
	if err := os.WriteFile(signature, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareLooseRegistration(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(signature); !os.IsNotExist(err) {
		t.Fatalf("Store signature is still active: %v", err)
	}
	if saved, err := os.ReadFile(signature + ".levilauncher-backup"); err != nil || string(saved) != string(original) {
		t.Fatalf("signature backup: %q %v", saved, err)
	}
	if err := prepareLooseRegistration(dir); err != nil {
		t.Fatalf("prepare was not idempotent: %v", err)
	}
	if err := os.WriteFile(signature, []byte("different signature"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareLooseRegistration(dir); ErrorCode(err) != "ERR_UWP_PREPARE" {
		t.Fatalf("overwrote mismatched backup: %v", err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "AppxSignature.p7x"), original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareLooseRegistration(outside); ErrorCode(err) != "ERR_UWP_PACKAGE_CONFLICT" {
		t.Fatalf("modified outside instance root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "AppxSignature.p7x")); err != nil {
		t.Fatal("external signature modified")
	}
}

func TestLaunchRequiresSelectedRegistration(t *testing.T) {
	_, dir := setupDeployment(t)
	previous := activateApplication
	t.Cleanup(func() { activateApplication = previous })
	activateApplication = func(string) (int, error) {
		t.Fatal("unregistered instance must not activate another package")
		return 0, nil
	}
	for _, registeredDir := range []string{"", filepath.Join(t.TempDir(), "another-instance")} {
		runPowerShell = func(_ context.Context, script string) ([]byte, error) {
			if !strings.HasPrefix(script, "Get-AppxPackage") {
				t.Fatalf("launch implicitly changed registration: %s", script)
			}
			if registeredDir == "" {
				return nil, nil
			}
			return packageJSON(t, registeredDir, true), nil
		}
		if pid, err := Launch(context.Background(), dir); pid != 0 || ErrorCode(err) != "ERR_UWP_NOT_REGISTERED" {
			t.Fatalf("unregistered launch: pid=%d err=%v", pid, err)
		}
	}
}

func TestLiveQueryMinecraftRegistrations(t *testing.T) {
	if os.Getenv("LEVILAUNCHER_UWP_QUERY_TEST") != "1" {
		t.Skip("set LEVILAUNCHER_UWP_QUERY_TEST=1 for read-only Windows package queries")
	}
	for _, name := range []string{ReleasePackageName, PreviewPackageName, "LeviLauncher.Nonexistent.Query.Test"} {
		pkg, err := queryPackage(context.Background(), name)
		if err != nil {
			t.Fatalf("%s query: %v", name, err)
		}
		if name == "LeviLauncher.Nonexistent.Query.Test" && pkg != nil {
			t.Fatalf("unexpected missing-package result: %+v", pkg)
		}
		t.Logf("%s: registered=%t", name, pkg != nil)
	}
}
