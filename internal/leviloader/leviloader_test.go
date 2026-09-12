package leviloader

import (
	"bytes"
	"context"
	"debug/pe"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

func writeTestVersion(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// Use the bundled x64 PE as an image fixture; tests never execute it.
	if err := os.WriteFile(filepath.Join(dir, minecraftExeName), embeddedLoader, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, legacyProxyName), []byte("legacy proxy"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestUWPCannotReceiveGDKLoader(t *testing.T) {
	for _, metadata := range []bool{false, true} {
		dir := t.TempDir()
		writeTestVersion(t, dir)
		if metadata {
			if err := versions.WriteMeta(dir, versions.VersionMeta{Name: "uwp", PackageType: "uwp"}); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(filepath.Join(dir, "AppxManifest.xml"), []byte("legacy UWP manifest"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := PatchAndActivate(context.Background(), dir); err == nil {
			t.Fatal("UWP loader activation succeeded")
		}
		if fileExists(filepath.Join(dir, LoaderDLLName)) {
			t.Fatal("UWP received GDK loader")
		}
		if data, err := os.ReadFile(filepath.Join(dir, minecraftExeName)); err != nil || !bytes.Equal(data, embeddedLoader) {
			t.Fatal("UWP executable changed")
		}
		if !fileExists(filepath.Join(dir, legacyProxyName)) {
			t.Fatal("UWP runtime DLL removed")
		}
	}
}

func TestRunMigrationSkipsUWP(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	c, err := config.Reload()
	if err != nil {
		t.Fatal(err)
	}
	c.BaseRoot = t.TempDir()
	if err := config.Save(c); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.Reload() })
	dir := filepath.Join(c.BaseRoot, "versions", "uwp")
	writeTestVersion(t, dir)
	if err := versions.WriteMeta(dir, versions.VersionMeta{Name: "uwp", PackageType: "uwp"}); err != nil {
		t.Fatal(err)
	}
	count, err := RunMigration(context.Background())
	if err != nil || count != 0 {
		t.Fatalf("UWP migration count=%d error=%v", count, err)
	}
	if fileExists(filepath.Join(dir, LoaderDLLName)) {
		t.Fatal("migration wrote UWP loader")
	}
	if data, err := os.ReadFile(filepath.Join(dir, minecraftExeName)); err != nil || !bytes.Equal(data, embeddedLoader) {
		t.Fatal("migration changed UWP executable")
	}
}

func TestPatchAndActivateRequiresLoaderDeployment(t *testing.T) {
	dir := t.TempDir()
	writeTestVersion(t, dir)
	if err := os.Mkdir(filepath.Join(dir, LoaderDLLName+".tmp"), 0755); err != nil {
		t.Fatal(err)
	}
	added, err := PatchAndActivate(context.Background(), dir)
	if err == nil || added {
		t.Fatalf("deployment failure must stop activation: added=%v err=%v", added, err)
	}
	exe, err := os.ReadFile(filepath.Join(dir, minecraftExeName))
	if err != nil || !bytes.Equal(exe, embeddedLoader) {
		t.Fatalf("failed deployment changed the executable: %v", err)
	}
	if !fileExists(filepath.Join(dir, legacyProxyName)) {
		t.Fatal("failed deployment removed the legacy proxy")
	}
}

func TestPatchAndActivateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeTestVersion(t, dir)
	added, err := PatchAndActivate(context.Background(), dir)
	if err != nil || !added {
		t.Fatalf("activate: added=%v err=%v", added, err)
	}
	if !fileExists(filepath.Join(dir, LoaderDLLName)) || fileExists(filepath.Join(dir, legacyProxyName)) {
		t.Fatal("activation did not replace the legacy loader")
	}
	exe := filepath.Join(dir, minecraftExeName)
	before, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	f, err := pe.NewFile(bytes.NewReader(before))
	if err != nil {
		t.Fatal(err)
	}
	if f.Section(".levildr") == nil {
		t.Fatal("missing loader import section")
	}
	added, err = PatchAndActivate(context.Background(), dir)
	if err != nil || added {
		t.Fatalf("repeat activation: added=%v err=%v", added, err)
	}
	after, err := os.ReadFile(exe)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("repeat activation changed the executable: %v", err)
	}
}

func TestRunMigrationRetriesFailedVersions(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	c, err := config.Reload()
	if err != nil {
		t.Fatal(err)
	}
	c.BaseRoot = t.TempDir()
	if err := config.Save(c); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(c.BaseRoot, "versions", "a-good")
	second := filepath.Join(c.BaseRoot, "versions", "b-broken")
	writeTestVersion(t, first)
	writeTestVersion(t, second)
	if err := os.WriteFile(filepath.Join(second, minecraftExeName), []byte("invalid PE"), 0644); err != nil {
		t.Fatal(err)
	}
	count, err := RunMigration(context.Background())
	if err == nil || count != 1 {
		t.Fatalf("expected one success followed by failure: count=%d err=%v", count, err)
	}
	c, err = config.Reload()
	if err != nil || c.LoaderMigratedV1 {
		t.Fatalf("failed migration was marked complete: config=%+v err=%v", c, err)
	}
	if !fileExists(filepath.Join(second, legacyProxyName)) {
		t.Fatal("failed patch removed the legacy proxy")
	}
	writeTestVersion(t, second)
	count, err = RunMigration(context.Background())
	if err != nil || count != 2 {
		t.Fatalf("retry: count=%d err=%v", count, err)
	}
	c, err = config.Reload()
	if err != nil || !c.LoaderMigratedV1 {
		t.Fatalf("successful retry did not persist completion: config=%+v err=%v", c, err)
	}
}

func TestRunMigrationReportsConfigWriteFailure(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if _, err := config.Reload(); err != nil {
		t.Fatal(err)
	}
	path := apppath.ConfigPath()
	if err := os.Chmod(path, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0644) })
	if _, err := RunMigration(context.Background()); err == nil {
		t.Fatal("expected the config write failure to reach the caller")
	}
	if !NeedsMigration() {
		t.Fatal("failed config write changed the cached migration marker")
	}
}
