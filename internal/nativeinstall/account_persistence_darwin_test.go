package nativeinstall

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/xbox"
)

func TestBrowserRefreshPersistence(t *testing.T) {
	original := protectedKey
	protectedKey = func(string, bool) ([]byte, error) { return bytes.Repeat([]byte{7}, 32), nil }
	t.Cleanup(func() {
		protectedKey = original
		xbox.ConfigureAccountSelection("", nil)
	})
	dir := t.TempDir()
	const account, initial, rotated = "browser:test-account", "test-initial-refresh", "test-rotated-refresh"
	save := browserRefreshSaver(dir)
	if err := save(account, "", initial); err != nil {
		t.Fatal(err)
	}
	if err := save(account, initial, rotated); err != nil {
		t.Fatal(err)
	}
	if err := save(account, initial, "stale"); !errors.Is(err, xbox.ErrAccountChanged) {
		t.Fatalf("stale rotation was not rejected: %v", err)
	}
	path := filepath.Join(dir, "wam-account.dpapi")
	record, err := os.ReadFile(path)
	if err != nil || bytes.Contains(record, []byte(rotated)) {
		t.Fatal("invalid encrypted credential record")
	}
	var saved wamAccountSelection
	if err := loadProtected(path, &saved); err != nil || saved.AccountID != account || saved.BrowserRefresh != rotated {
		t.Fatal("rotated credential was not restored")
	}

	ctx := context.Background()
	xbox.ConfigureAccountSelection("", nil)
	if err := RestoreSharedAccount(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if id, err := xbox.GetAccountIDForUser(ctx, ""); err != nil || id != account {
		t.Fatal("account identity was not restored")
	}
	if err := ForgetSharedAccount(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("saved account survived sign-out")
	}
	if err := save(account, rotated, "stale"); !errors.Is(err, xbox.ErrAccountChanged) {
		t.Fatalf("stale rotation revived a signed-out account: %v", err)
	}
}
