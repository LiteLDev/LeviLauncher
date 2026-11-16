//go:build darwin || linux

package gameruntime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
)

func TestScanFindsWrappersAndDeduplicatesPathAliasesWithoutRunningThem(t *testing.T) {
	root := t.TempDir()
	apps, bin := filepath.Join(root, "Applications"), filepath.Join(root, "bin")
	app := filepath.Join(apps, "Games", "中文 Wine.app")
	wine := filepath.Join(app, "Contents/SharedSupport/wine/bin/wine")
	marker := filepath.Join(root, "executed")
	if err := os.MkdirAll(filepath.Dir(wine), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wine, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app, "Contents/SharedSupport/prefix/drive_c"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wine, filepath.Join(bin, "wine")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wine, filepath.Join(bin, "wine64")); err != nil {
		t.Fatal(err)
	}
	got, err := scanLocations(context.Background(), []string{apps, apps}, []string{bin, bin}, root, types.WineSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want wrapper and one PATH engine, got %+v", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("scanner executed an unselected binary")
	}
	found := false
	for _, item := range got {
		if item.Settings.App == app {
			found = item.Status == "ready" && item.Settings.Prefix == filepath.Join(app, "Contents/SharedSupport/prefix")
		}
	}
	if !found {
		t.Fatalf("initialized wrapper not found: %+v", got)
	}
}

func TestScanLabelsUnsupportedCrossOverAndHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	wine := filepath.Join(root, "CrossOver.app/Contents/SharedSupport/CrossOver/bin/wine")
	if err := os.MkdirAll(filepath.Dir(wine), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wine, []byte("not executed"), 0700); err != nil {
		t.Fatal(err)
	}
	got, err := scanLocations(context.Background(), []string{root}, nil, root, types.WineSettings{})
	if err != nil || len(got) != 1 || got[0].Status != "unsupported" {
		t.Fatalf("unexpected CrossOver classification: %+v %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := scanLocations(ctx, []string{root}, nil, root, types.WineSettings{}); err != context.Canceled {
		t.Fatalf("ignored cancellation: %v", err)
	}
}

func TestInstallerOnlyInstallsTheOfficialPackageAndQuotesPaths(t *testing.T) {
	script := installerScript("/tmp/with space/it's brew")
	if !strings.Contains(script, "'/tmp/with space/it'\\''s brew' install --cask Sikarugir-App/sikarugir/sikarugir") {
		t.Fatalf("invalid installer quoting: %s", script)
	}
	for _, forbidden := range []string{"--no-quarantine", "brew upgrade", "brew trust", "sudo ", "curl ", "eval "} {
		if strings.Contains(script, forbidden) {
			t.Errorf("unexpected installer action %q", forbidden)
		}
	}
}
