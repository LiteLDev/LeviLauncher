//go:build windows && wamprobe

package nativeinstall

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

func TestLiveGameLicenses(t *testing.T) {
	if os.Getenv("LEVI_WAM_PROBE") != "1" {
		t.Skip("live check is opt-in")
	}
	ctx := context.Background()
	dir := filepath.Join(config.ConfigDir(), "microsoft-account")
	if err := RestoreSharedAccount(ctx, dir); err != nil {
		t.Fatal("account restoration failed")
	}
	id, err := xbox.GetLocalUserId()
	if err != nil {
		t.Fatal("account identity unavailable")
	}
	result := CheckGameLicenses(ctx, dir, strconv.FormatUint(id, 10))
	t.Logf("release=%s preview=%s", result.Release, result.Preview)
	if result.Release == "error" || result.Preview == "error" {
		t.Fatal("live license check failed")
	}
}
