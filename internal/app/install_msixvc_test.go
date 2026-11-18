package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

func TestInstallExtractMsixvcReportsInvalidPackage(t *testing.T) {
	root := t.TempDir()
	apppath.SetBaseRootOverride(root)
	t.Cleanup(func() { apppath.SetBaseRootOverride("") })

	archive := filepath.Join(root, "truncated.msixvc")
	if err := os.WriteFile(archive, []byte("truncated"), 0600); err != nil {
		t.Fatal(err)
	}

	launcher := &Minecraft{ctx: context.Background()}
	const want = "ERR_MSIXVC_UNSUPPORTED_FORMAT: input is not a complete MSIXVC"
	if got := launcher.InstallExtractMsixvc(archive, "instance", false); got != want {
		t.Fatalf("installation result = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(root, apppath.InstancesDirName, "instance")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed installation left an instance: %v", err)
	}
}
