//go:build windows && gdkprobe

package gdkversions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

// Run with -tags gdkprobe and LEVI_GDK_ACCOUNT_DIR pointing to the launcher's
// microsoft-account directory. It restores only the selected identity, uses
// silent WAM authentication and prints no credentials or account identifiers.
func TestStoreLatestLive(t *testing.T) {
	dir := os.Getenv("LEVI_GDK_ACCOUNT_DIR")
	if dir == "" {
		t.Skip("explicit account directory required for live Store probe")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defer xbox.ConfigureAccountSelection("", nil)
	if err := nativeinstall.RestoreSharedAccount(ctx, dir); err != nil {
		t.Fatal(err)
	}
	c := NewClient(httpx.NewClient(5*time.Second), xbox.GetPackageUpdateAuthorization)
	latest := c.fetchLatest(ctx)
	if len(latest.ReleaseVersions) != 1 || len(latest.PreviewVersions) != 1 {
		t.Fatal("live Store discovery did not return both channels")
	}
	// Exercise the complete fetch/cache path with the actual service responses.
	merged := c.Fetch(ctx, false, filepath.Join(t.TempDir(), "pending.json"))
	for i, channel := range [][]Version{merged.ReleaseVersions, merged.PreviewVersions} {
		version := []Version{latest.ReleaseVersions[0], latest.PreviewVersions[0]}[i]
		count := 0
		for _, entry := range channel {
			if versionKey(entry.Version) == versionKey(version.Version) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("live catalog contains %d entries for %s", count, version.Version)
		}
		t.Logf("%s: %d download URLs, exactly one merged entry", version.Version, len(version.URLs))
	}
}
