package mcservice

import (
	"context"
	"errors"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lip"
)

func TestGetLIPPackageInstallStatesBatchQuery(t *testing.T) {
	restoreLipQueryDeps(t)
	lipIsInstalled = func() bool { return true }
	lipResolveTargetDir = func(targetName string) (string, string) {
		if targetName != "Demo" {
			t.Fatalf("unexpected target name: %s", targetName)
		}
		return `C:\demo`, ""
	}
	lipListPackageStatesViaDaemon = func(ctx context.Context, workDir string) (map[string]lip.PackageInstallState, error) {
		if workDir != `C:\demo` {
			t.Fatalf("unexpected work dir: %s", workDir)
		}
		return map[string]lip.PackageInstallState{
			"github.com/liteldev/foo#client": {
				Installed:         true,
				ExplicitInstalled: true,
				InstalledVersion:  "1.2.3",
			},
		}, nil
	}

	entries := GetLIPPackageInstallStates(context.Background(), "Demo", []string{
		"LiteLDev/Foo",
		"LiteLDev/Bar",
	})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].IdentifierKey != "LiteLDev/Foo" {
		t.Fatalf("unexpected first identifier key: %q", entries[0].IdentifierKey)
	}
	if !entries[0].State.Installed || !entries[0].State.ExplicitInstalled {
		t.Fatalf("expected first package to be explicitly installed, got %+v", entries[0].State)
	}
	if entries[0].State.InstalledVersion != "1.2.3" {
		t.Fatalf("unexpected installed version: %q", entries[0].State.InstalledVersion)
	}

	if entries[1].IdentifierKey != "LiteLDev/Bar" {
		t.Fatalf("unexpected second identifier key: %q", entries[1].IdentifierKey)
	}
	if entries[1].State.Installed {
		t.Fatalf("expected second package to be absent, got %+v", entries[1].State)
	}
}

func TestGetLIPPackageInstallStatesLipNotInstalled(t *testing.T) {
	restoreLipQueryDeps(t)
	lipIsInstalled = func() bool { return false }

	entries := GetLIPPackageInstallStates(context.Background(), "Demo", []string{
		"LiteLDev/Foo",
	})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].State.Error != "ERR_LIP_NOT_INSTALLED" {
		t.Fatalf("unexpected error code: %q", entries[0].State.Error)
	}
}

func TestGetLIPPackageInstallStatesTimeoutOrCancellation(t *testing.T) {
	restoreLipQueryDeps(t)
	lipIsInstalled = func() bool { return true }
	lipResolveTargetDir = func(targetName string) (string, string) {
		return `C:\demo`, ""
	}
	lipListPackageStatesViaDaemon = func(ctx context.Context, workDir string) (map[string]lip.PackageInstallState, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	entries := GetLIPPackageInstallStates(ctx, "Demo", []string{"LiteLDev/Foo"})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].State.Error != "ERR_LIP_PACKAGE_QUERY_FAILED" {
		t.Fatalf("unexpected error code: %q", entries[0].State.Error)
	}
}

func TestGetLIPPackageInstallStatesRPCFailure(t *testing.T) {
	restoreLipQueryDeps(t)
	lipIsInstalled = func() bool { return true }
	lipResolveTargetDir = func(targetName string) (string, string) {
		return `C:\demo`, ""
	}
	lipListPackageStatesViaDaemon = func(ctx context.Context, workDir string) (map[string]lip.PackageInstallState, error) {
		return nil, errors.New("rpc exploded")
	}

	entries := GetLIPPackageInstallStates(context.Background(), "Demo", []string{"LiteLDev/Foo"})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].State.Error != "ERR_LIP_PACKAGE_QUERY_FAILED" {
		t.Fatalf("unexpected error code: %q", entries[0].State.Error)
	}
}

func restoreLipQueryDeps(t *testing.T) {
	t.Helper()
	originalInstalled := lipIsInstalled
	originalList := lipListPackageStatesViaDaemon
	originalResolve := lipResolveTargetDir
	t.Cleanup(func() {
		lipIsInstalled = originalInstalled
		lipListPackageStatesViaDaemon = originalList
		lipResolveTargetDir = originalResolve
	})
}
